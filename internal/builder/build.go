// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/fluxcd/pkg/kustomize"
	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	cp "github.com/otiai10/copy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Build copies the source directory to a temporary directory, generates the
// required manifests and runs kustomize to build the final resources.
// The function returns a slice of unstructured objects.
func Build(srcDir, tmpDir string, options Options) ([]*unstructured.Unstructured, error) {
	if err := cp.Copy(srcDir, tmpDir); err != nil {
		return nil, err
	}

	if err := generate(tmpDir, options); err != nil {
		return nil, err
	}

	resources, err := kustomize.SecureBuild(tmpDir, tmpDir, false)
	if err != nil {
		return nil, err
	}

	data, err := resources.AsYaml()
	if err != nil {
		return nil, err
	}

	objects, err := ssautil.ReadObjects(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	sort.Sort(ssa.SortableUnstructureds(objects))

	return objects, nil
}

func generate(base string, options Options) error {
	if containsItemString(options.Components, options.NotificationController) {
		options.EventsAddr = fmt.Sprintf("http://%s.%s.svc.%s./", options.NotificationController, options.Namespace, options.ClusterDomain)
	}

	if err := execTemplate(options, namespaceTmpl, path.Join(base, "namespace.yaml")); err != nil {
		return fmt.Errorf("generate namespace failed: %w", err)
	}

	if err := execTemplate(options, labelsTmpl, path.Join(base, "labels.yaml")); err != nil {
		return fmt.Errorf("generate labels failed: %w", err)
	}

	if err := execTemplate(options, nodeSelectorTmpl, path.Join(base, "node-selector.yaml")); err != nil {
		return fmt.Errorf("generate node selector failed: %w", err)
	}

	if err := execTemplate(options, kustomizationTmpl, path.Join(base, "kustomization.yaml")); err != nil {
		return fmt.Errorf("generate kustomization failed: %w", err)
	}

	if err := os.MkdirAll(path.Join(base, "roles"), os.ModePerm); err != nil {
		return fmt.Errorf("generate roles failed: %w", err)
	}

	if err := execTemplate(options, kustomizationRolesTmpl, path.Join(base, "roles/kustomization.yaml")); err != nil {
		return fmt.Errorf("generate roles kustomization failed: %w", err)
	}

	rbacFile := filepath.Join(base, "roles", "rbac.yaml")
	if err := copyFile(filepath.Join(base, "rbac.yaml"), rbacFile); err != nil {
		return fmt.Errorf("generate rbac failed: %w", err)
	}

	// workaround for kustomize not being able to patch the SA in ClusterRoleBindings
	defaultNS := MakeDefaultOptions().Namespace
	if defaultNS != options.Namespace {
		rbac, err := os.ReadFile(rbacFile)
		if err != nil {
			return fmt.Errorf("reading rbac file failed: %w", err)
		}
		rbac = bytes.ReplaceAll(rbac, []byte(defaultNS), []byte(options.Namespace))
		if err := os.WriteFile(rbacFile, rbac, os.ModePerm); err != nil {
			return fmt.Errorf("replacing service account namespace in rbac failed: %w", err)
		}
	}
	return nil
}

// MkdirTempAbs creates a tmp dir and returns the absolute path to the dir.
// This is required since certain OSes like MacOS create temporary files in
// e.g. `/private/var`, to which `/var` is a symlink.
func MkdirTempAbs(dir, pattern string) (string, error) {
	tmpDir, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return "", fmt.Errorf("error evaluating symlink: %w", err)
	}
	return tmpDir, nil
}
