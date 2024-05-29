// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/kustomize"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	gcname "github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// ComponentImage represents a container image used by a component.
type ComponentImage struct {
	Name       string
	Repository string
	Tag        string
	Digest     string
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

// FetchComponentImages fetches the components images from the distribution repository.
func FetchComponentImages(opts Options) (images []ComponentImage, err error) {
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
	default:
		return nil, fmt.Errorf("unsupported registry: %s", registry)
	}

	const ghRepo = "https://raw.githubusercontent.com/controlplaneio-fluxcd/distribution/main/images"
	ghURL := fmt.Sprintf("%s/%s/%s.yaml", ghRepo, opts.Version, distro)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ghURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
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
		component := strings.TrimPrefix(img.Name, registry+"/")
		if containsItemString(opts.Components, component) {
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
