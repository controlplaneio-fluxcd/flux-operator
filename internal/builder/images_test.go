// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuild_ExtractImages(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	srcDir := filepath.Join("testdata", version)

	images, err := ExtractComponentImages(srcDir, MakeDefaultOptions())
	g.Expect(err).NotTo(HaveOccurred())

	t.Log(images)
	g.Expect(images).To(HaveLen(6))
	g.Expect(images).To(ContainElements(
		ComponentImage{
			Name:       "source-controller",
			Repository: "ghcr.io/fluxcd/source-controller",
			Tag:        "v1.3.0",
			Digest:     "",
		},
		ComponentImage{
			Name:       "kustomize-controller",
			Repository: "ghcr.io/fluxcd/kustomize-controller",
			Tag:        "v1.3.0",
			Digest:     "",
		},
	))
}

func TestBuild_ExtractImagesWithDigest(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	opts := MakeDefaultOptions()
	opts.Version = version
	opts.Registry = "ghcr.io/fluxcd"

	imagePath := filepath.Join("testdata", "flux-images")
	images, err := ExtractComponentImagesWithDigest(imagePath, opts)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(images).To(HaveLen(6))
	g.Expect(images).To(ContainElements(
		ComponentImage{
			Name:       "source-controller",
			Repository: "ghcr.io/fluxcd/source-controller",
			Tag:        "v1.3.0",
			Digest:     "sha256:161da425b16b64dda4b3cec2ba0f8d7442973aba29bb446db3b340626181a0bc",
		},
		ComponentImage{
			Name:       "kustomize-controller",
			Repository: "ghcr.io/fluxcd/kustomize-controller",
			Tag:        "v1.3.0",
			Digest:     "sha256:48a032574dd45c39750ba0f1488e6f1ae36756a38f40976a6b7a588d83acefc1",
		},
	))

	opts.Registry = "registry.local/fluxcd"
	_, err = ExtractComponentImagesWithDigest(imagePath, opts)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unsupported registry"))
}

func TestBuild_ExtractImagesWithDigest_AWS(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	opts := MakeDefaultOptions()
	opts.Version = version
	opts.Registry = "709825985650.dkr.ecr.us-east-1.amazonaws.com/controlplane/fluxcd"

	imagePath := filepath.Join("testdata", "flux-images")
	images, err := ExtractComponentImagesWithDigest(imagePath, opts)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(images).To(HaveLen(6))
	g.Expect(images).To(ContainElements(
		ComponentImage{
			Name:       "source-controller",
			Repository: "709825985650.dkr.ecr.us-east-1.amazonaws.com/controlplane/fluxcd/source-controller",
			Tag:        "v1.3.0",
			Digest:     "sha256:3b34a63a635779b2b3ea67ec02f5925704dc93d39efc4b92243e2170907615af",
		},
	))
}

func TestExtractComponentImagesFromObjects(t *testing.T) {
	tests := []struct {
		name        string
		objects     []*unstructured.Unstructured
		opts        Options
		expected    []ComponentImage
		expectError bool
	}{
		{
			name: "standard image format",
			objects: []*unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "source-controller",
						},
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "manager",
											"image": "ghcr.io/fluxcd/source-controller:v1.3.0",
										},
									},
								},
							},
						},
					},
				},
			},
			opts: Options{
				Components: []string{"source-controller"},
			},
			expected: []ComponentImage{
				{
					Name:       "source-controller",
					Repository: "ghcr.io/fluxcd/source-controller",
					Tag:        "v1.3.0",
					Digest:     "",
				},
			},
			expectError: false,
		},

		{
			name: "image with no tag and no digest",
			objects: []*unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "kustomize-controller",
						},
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "manager",
											"image": "my.registry/kustomize-controller",
										},
									},
								},
							},
						},
					},
				},
			},
			opts: Options{
				Components: []string{"kustomize-controller"},
			},
			expected: []ComponentImage{
				{
					Name:       "kustomize-controller",
					Repository: "my.registry/kustomize-controller",
					Tag:        "latest",
					Digest:     "",
				},
			},
			expectError: false,
		},
		{
			name: "image with digest",
			objects: []*unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "kustomize-controller",
						},
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "manager",
											"image": "ghcr.io/fluxcd/kustomize-controller:v1.3.0@sha256:e4cb9731b4db9e98d8eda886e16ced9896861e10cfbbf1153a2ec181bd68f770",
										},
									},
								},
							},
						},
					},
				},
			},
			opts: Options{
				Components: []string{"kustomize-controller"},
			},
			expected: []ComponentImage{
				{
					Name:       "kustomize-controller",
					Repository: "ghcr.io/fluxcd/kustomize-controller",
					Tag:        "v1.3.0",
					Digest:     "sha256:e4cb9731b4db9e98d8eda886e16ced9896861e10cfbbf1153a2ec181bd68f770",
				},
			},
			expectError: false,
		},
		{
			name: "localhost registry",
			objects: []*unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "helm-controller",
						},
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "manager",
											"image": "localhost:5000/fluxcd/helm-controller:v0.37.4",
										},
									},
								},
							},
						},
					},
				},
			},
			opts: Options{
				Components: []string{"helm-controller"},
			},
			expected: []ComponentImage{
				{
					Name:       "helm-controller",
					Repository: "localhost:5000/fluxcd/helm-controller",
					Tag:        "v0.37.4",
					Digest:     "",
				},
			},
			expectError: false,
		},
		{
			name: "multiple components",
			objects: []*unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "source-controller",
						},
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "manager",
											"image": "fluxcd/source-controller:v1.3.0",
										},
									},
								},
							},
						},
					},
				},
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "kustomize-controller",
						},
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "manager",
											"image": "ghcr.io/fluxcd/kustomize-controller:v1.3.0@sha256:e7487a8ef09c4f584b5e2620665950aef7814b95c366a9b88f9fbacdd7eb3269",
										},
									},
								},
							},
						},
					},
				},
			},
			opts: Options{
				Components: []string{"source-controller", "kustomize-controller"},
			},
			expected: []ComponentImage{
				{
					Name:       "source-controller",
					Repository: "index.docker.io/fluxcd/source-controller",
					Tag:        "v1.3.0",
					Digest:     "",
				},
				{
					Name:       "kustomize-controller",
					Repository: "ghcr.io/fluxcd/kustomize-controller",
					Tag:        "v1.3.0",
					Digest:     "sha256:e7487a8ef09c4f584b5e2620665950aef7814b95c366a9b88f9fbacdd7eb3269",
				},
			},
			expectError: false,
		},
		{
			name: "container with non-manager name should be skipped",
			objects: []*unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "source-controller",
						},
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "sidecar",
											"image": "my.registry/sidecar:v1.0.0",
										},
										map[string]any{
											"name":  "manager",
											"image": "my.registry/source-controller@sha256:d5d97fb756e42c453f2f9a3d4e37cd5482c6e473437169016adbf50f8b495a37",
										},
									},
								},
							},
						},
					},
				},
			},
			opts: Options{
				Components: []string{"source-controller"},
			},
			expected: []ComponentImage{
				{
					Name:       "source-controller",
					Repository: "my.registry/source-controller",
					Tag:        "latest",
					Digest:     "sha256:d5d97fb756e42c453f2f9a3d4e37cd5482c6e473437169016adbf50f8b495a37",
				},
			},
			expectError: false,
		},
		{
			name: "missing containers should return error",
			objects: []*unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "source-controller",
						},
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{},
							},
						},
					},
				},
			},
			opts: Options{
				Components: []string{"source-controller"},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "invalid digest should return error",
			objects: []*unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "source-controller",
						},
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "manager",
											"image": "reg.internal/source-controller@sha256:abc123",
										},
									},
								},
							},
						},
					},
				},
			},
			opts: Options{
				Components: []string{"source-controller"},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			images, err := ExtractComponentImagesFromObjects(tt.objects, tt.opts)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(images).To(Equal(tt.expected))
			}
		})
	}
}
