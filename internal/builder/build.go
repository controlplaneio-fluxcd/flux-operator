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
	"github.com/opencontainers/go-digest"
	cp "github.com/otiai10/copy"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/notifier"
)

// Build copies the source directory to a temporary directory, generates the
// required manifests and runs kustomize to build the final resources.
// The function returns a slice of unstructured objects.
func Build(srcDir, tmpDir string, options Options) (*Result, error) {
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

	d := digest.FromBytes(data)

	return &Result{
		Version:         options.Version,
		Objects:         objects,
		Digest:          d.String(),
		Revision:        fmt.Sprintf("%s@%s", options.Version, d.String()),
		ComponentImages: options.ComponentImages,
	}, nil
}

func generate(base string, options Options) error {
	if options.HasNotificationController() {
		options.EventsAddr = notifier.Address(options.Namespace, options.ClusterDomain)
	}

	if err := execTemplate(options, namespaceTmpl, path.Join(base, "namespace.yaml")); err != nil {
		return fmt.Errorf("generate namespace failed: %w", err)
	}

	if err := execTemplate(options, annotationsTmpl, path.Join(base, "annotations.yaml")); err != nil {
		return fmt.Errorf("generate annotations failed: %w", err)
	}

	if err := execTemplate(options, labelsTmpl, path.Join(base, "labels.yaml")); err != nil {
		return fmt.Errorf("generate labels failed: %w", err)
	}

	if err := execTemplate(options, nodeSelectorTmpl, path.Join(base, "node-selector.yaml")); err != nil {
		return fmt.Errorf("generate node selector failed: %w", err)
	}

	if options.ArtifactStorage != nil {
		if err := execTemplate(options, pvcTmpl, path.Join(base, "pvc.yaml")); err != nil {
			return fmt.Errorf("generate pvc failed: %w", err)
		}
	}

	if options.Sync != nil {
		if err := execTemplate(options, syncTmpl, path.Join(base, "sync.yaml")); err != nil {
			return fmt.Errorf("generate sync failed: %w", err)
		}
	}

	if err := execTemplate(options, kustomizationTmpl, path.Join(base, "kustomization.yaml")); err != nil {
		return fmt.Errorf("generate kustomization failed: %w", err)
	}

	rbacFile := filepath.Join(base, "roles", "rbac.yaml")
	if err := cp.Copy(filepath.Join(base, "rbac.yaml"), rbacFile); err != nil {
		return fmt.Errorf("generate rbac failed: %w", err)
	}

	if err := execTemplate(options, kustomizationRolesTmpl, path.Join(base, "roles", "kustomization.yaml")); err != nil {
		return fmt.Errorf("generate roles kustomization failed: %w", err)
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

	for _, shard := range options.Shards {
		options.ShardName = shard
		if err := os.MkdirAll(path.Join(base, shard), os.ModePerm); err != nil {
			return fmt.Errorf("generate shard dir failed: %w", err)
		}
		if err := execTemplate(options, kustomizationShardTmpl, path.Join(base, shard, "kustomization.yaml")); err != nil {
			return fmt.Errorf("generate shard kustomization failed: %w", err)
		}
	}

	return nil
}
