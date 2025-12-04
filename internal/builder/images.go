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

const (
	UpstreamAlpine           string = "upstream-alpine"
	EnterpriseAlpine         string = "enterprise-alpine"
	EnterpriseDistroless     string = "enterprise-distroless"
	EnterpriseDistrolessFIPS string = "enterprise-distroless-fips"
)

// ExtractComponentImagesFromObjects extracts the container images from the
// Deployment objects in the provided slice of unstructured objects.
// It returns a slice of ComponentImage containing the component name,
// repository, tag and digest (if available).
func ExtractComponentImagesFromObjects(objects []*unstructured.Unstructured, opts Options) ([]ComponentImage, error) {
	images := make([]ComponentImage, len(opts.Components))
	for i, component := range opts.Components {
		for _, obj := range objects {
			if obj.GetKind() == "Deployment" && obj.GetName() == component {
				containers, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
				if !ok {
					return nil, fmt.Errorf("containers not found in %s", obj.GetName())
				}
				for _, container := range containers {
					cname := container.(map[string]any)["name"].(string)
					if cname != "manager" {
						continue

					}

					img := container.(map[string]any)["image"].(string)
					ref, err := gcname.ParseReference(img, gcname.WeakValidation)
					if err != nil {
						return nil, err
					}

					repo := fmt.Sprintf("%s/%s", ref.Context().RegistryStr(), ref.Context().RepositoryStr())
					tag := "latest"
					digest := ""

					id := strings.SplitN(img, ref.Context().RepositoryStr(), 2)[1]
					if strings.Contains(id, "@") {
						parts := strings.Split(id, "@")
						if len(parts) == 2 {
							digest = parts[1]
							if strings.Contains(parts[0], ":") {
								tag = strings.TrimPrefix(parts[0], ":")
							}
						} else {
							digest = parts[0]
						}
					} else {
						if strings.Contains(id, ":") {
							tag = strings.TrimPrefix(id, ":")
						}
					}

					images[i] = ComponentImage{
						Name:       component,
						Repository: repo,
						Tag:        tag,
						Digest:     digest,
					}
					break
				}
			}
		}
	}

	return images, nil
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
					img := container.(map[string]any)["image"].(string)
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
	variant := opts.Variant

	if variant == "" {
		switch registry {
		case "fluxcd":
			variant = UpstreamAlpine
		case "ghcr.io/fluxcd":
			variant = UpstreamAlpine
		case "ghcr.io/controlplaneio-fluxcd/alpine":
			variant = EnterpriseAlpine
		case "ghcr.io/controlplaneio-fluxcd/distroless":
			variant = EnterpriseDistroless
		case "709825985650.dkr.ecr.us-east-1.amazonaws.com/controlplane/fluxcd":
			variant = EnterpriseDistroless
		case "ghcr.io/controlplaneio-fluxcd/distroless-fips":
			variant = EnterpriseDistrolessFIPS
		default:
			return nil, fmt.Errorf("unsupported registry. consider specifying the distribution variant for registry: %s", registry)
		}
	}

	if variant != UpstreamAlpine && variant != EnterpriseAlpine && variant != EnterpriseDistroless && variant != EnterpriseDistrolessFIPS {
		return nil, fmt.Errorf("unsupported variant: %s", variant)
	}

	imageFile := fmt.Sprintf("%s/%s/%s.yaml", srcDir, opts.Version, variant)

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
