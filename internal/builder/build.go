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
	"strings"

	"github.com/fluxcd/pkg/kustomize"
	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	gcname "github.com/google/go-containerregistry/pkg/name"
	"github.com/opencontainers/go-digest"
	cp "github.com/otiai10/copy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		Version:  options.Version,
		Objects:  objects,
		Digest:   d.String(),
		Revision: fmt.Sprintf("%s@%s", options.Version, d.String()),
	}, nil
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
	return nil
}

// ComponentImage represents a container image used by a component.
type ComponentImage struct {
	Component   string
	ImageName   string
	ImageTag    string
	ImageDigest string
}

// ExtractComponentImages reads the source directory and extracts the container images
// from the components manifests.
func ExtractComponentImages(srcDir string, opts Options) ([]ComponentImage, error) {
	images := make([]ComponentImage, len(opts.Components))
	for i, component := range opts.Components {
		d, err := os.ReadFile(filepath.Join(srcDir, fmt.Sprintf("/%s.yaml", component)))
		if err != nil {
			return nil, err
		}
		objects, err := ssautil.ReadObjects(bytes.NewReader(d))
		if err != nil {
			return nil, err
		}
		for _, obj := range objects {
			if obj.GetKind() == "Deployment" {
				containers, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
				if !ok {
					return nil, fmt.Errorf("containers not found in %s", obj.GetName())
				}
				for _, container := range containers {
					img := container.(map[string]interface{})["image"].(string)
					tag, err := gcname.NewTag(img, gcname.WeakValidation)
					if err != nil {
						return nil, err
					}

					images[i] = ComponentImage{
						Component: component,
						ImageName: fmt.Sprintf("%s/%s", strings.TrimSuffix(opts.Registry, "/"), component),
						ImageTag:  tag.Identifier(),
					}
				}
			}
		}
	}

	return images, nil
}
