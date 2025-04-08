// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fluxcd/pkg/apis/kustomize"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	gcname "github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

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
						Name:       component,
						Repository: fmt.Sprintf("%s/%s", strings.TrimSuffix(opts.Registry, "/"), component),
						Tag:        tag.Identifier(),
					}
				}
			}
		}
	}

	return images, nil
}

// ExtractComponentImagesWithDigest reads the source directory and extracts
// the container images with digest from the kustomize images patches.
func ExtractComponentImagesWithDigest(srcDir string, opts Options) (images []ComponentImage, err error) {
	registry := strings.TrimSuffix(opts.Registry, "/")
	var distro string

	switch registry {
	case "fluxcd":
		distro = "upstream-alpine"
	case "ghcr.io/fluxcd":
		distro = "upstream-alpine"
	case "ghcr.io/controlplaneio-fluxcd/alpine":
		distro = "enterprise-alpine"
	case "ghcr.io/controlplaneio-fluxcd/distroless":
		distro = "enterprise-distroless"
	case "709825985650.dkr.ecr.us-east-1.amazonaws.com/controlplane/fluxcd":
		distro = "enterprise-distroless"
	default:
		return nil, fmt.Errorf("unsupported registry: %s", registry)
	}

	imageFile := fmt.Sprintf("%s/%s/%s.yaml", srcDir, opts.Version, distro)

	data, err := os.ReadFile(imageFile)
	if err != nil {
		return nil, fmt.Errorf("read body: %v", err)
	}

	var kc struct {
		Images []kustomize.Image `yaml:"images"`
	}
	err = yaml.Unmarshal(data, &kc)
	if err != nil {
		return nil, err
	}

	for _, img := range kc.Images {
		name := img.Name
		component := name[strings.LastIndex(name, "/")+1:]
		if slices.Contains(opts.Components, component) {
			images = append(images, ComponentImage{
				Name:       component,
				Repository: fmt.Sprintf("%s/%s", registry, component),
				Tag:        img.NewTag,
				Digest:     img.Digest,
			})
		}
	}

	if len(images) != len(opts.Components) {
		return nil, fmt.Errorf("missing images for components: %v", opts.Components)
	}
	return images, nil
}
