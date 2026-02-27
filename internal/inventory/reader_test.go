// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inventory

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestFromHelmRelease(t *testing.T) {
	g := NewWithT(t)

	// Create mock Helm storage data
	manifestContent := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: default
spec:
  replicas: 1
---
apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: default
spec:
  ports:
  - port: 80
`

	helmStorage := HelmStorage{
		Name:     "test-release",
		Manifest: manifestContent,
	}

	storageData, err := json.Marshal(helmStorage)
	g.Expect(err).NotTo(HaveOccurred())

	encodedStorage := base64.StdEncoding.EncodeToString(storageData)

	// Create mock objects
	mockHelmRelease := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]any{
				"name":      "test-release",
				"namespace": "default",
			},
			"status": map[string]any{
				"storageNamespace": "default",
				"history": []any{
					map[string]any{
						"chartName": "test-chart",
						"version":   int64(1),
						"namespace": "default",
					},
				},
			},
		},
	}
	mockHelmRelease.SetGroupVersionKind(mockHelmRelease.GroupVersionKind())

	mockStorageSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1.test-chart.v1",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"release": []byte(encodedStorage),
		},
	}

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mockHelmRelease, mockStorageSecret).
		Build()

	tests := []struct {
		name              string
		helmRelease       fluxcdv1.ResourceRef
		expectedResources int
		expectError       bool
	}{
		{
			name: "successful inventory extraction",
			helmRelease: fluxcdv1.ResourceRef{
				ID:      "default_test-release_helm.toolkit.fluxcd.io_HelmRelease",
				Version: "v2",
			},
			expectedResources: 2, // Deployment and Service
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := FromHelmRelease(context.Background(), kubeClient, tt.helmRelease)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(HaveLen(tt.expectedResources))

				// Verify specific resources
				resourceIDs := make(map[string]string)
				for _, resource := range result {
					resourceIDs[resource.ID] = resource.Version
				}

				g.Expect(resourceIDs).To(HaveKey("default_test-deployment_apps_Deployment"))
				g.Expect(resourceIDs["default_test-deployment_apps_Deployment"]).To(Equal("v1"))
				g.Expect(resourceIDs).To(HaveKey("default_test-service__Service"))
				g.Expect(resourceIDs["default_test-service__Service"]).To(Equal("v1"))
			}
		})
	}
}

func TestFromHelmRelease_StatusInventory(t *testing.T) {
	g := NewWithT(t)

	// Create a HelmRelease with status.inventory.entries (Flux v2.8+)
	mockHelmRelease := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]any{
				"name":      "test-release",
				"namespace": "default",
			},
			"status": map[string]any{
				"inventory": map[string]any{
					"entries": []any{
						map[string]any{
							"id": "default_test-deployment_apps_Deployment",
							"v":  "v1",
						},
						map[string]any{
							"id": "default_test-service__Service",
							"v":  "v1",
						},
						map[string]any{
							"id": "default_test-config__ConfigMap",
							"v":  "v1",
						},
					},
				},
			},
		},
	}
	mockHelmRelease.SetGroupVersionKind(mockHelmRelease.GroupVersionKind())

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mockHelmRelease).
		Build()

	ref := fluxcdv1.ResourceRef{
		ID:      "default_test-release_helm.toolkit.fluxcd.io_HelmRelease",
		Version: "v2",
	}

	result, err := FromHelmRelease(context.Background(), kubeClient, ref)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(HaveLen(3))

	resourceIDs := make(map[string]string)
	for _, resource := range result {
		resourceIDs[resource.ID] = resource.Version
	}

	g.Expect(resourceIDs).To(HaveKey("default_test-deployment_apps_Deployment"))
	g.Expect(resourceIDs).To(HaveKey("default_test-service__Service"))
	g.Expect(resourceIDs).To(HaveKey("default_test-config__ConfigMap"))
	g.Expect(resourceIDs["default_test-deployment_apps_Deployment"]).To(Equal("v1"))
}

func TestFromHelmRelease_ErrorCases(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	tests := []struct {
		name          string
		helmRelease   *unstructured.Unstructured
		storageSecret *corev1.Secret
		expectNil     bool
		expectError   bool
	}{
		{
			name: "HelmRelease with kubeConfig should return nil",
			helmRelease: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "helm.toolkit.fluxcd.io/v2",
					"kind":       "HelmRelease",
					"metadata": map[string]any{
						"name":      "remote-release",
						"namespace": "default",
					},
					"spec": map[string]any{
						"kubeConfig": map[string]any{
							"secretRef": map[string]any{
								"name": "kubeconfig",
							},
						},
					},
				},
			},
			expectNil: true,
		},
		{
			name: "HelmRelease with no history should return nil",
			helmRelease: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "helm.toolkit.fluxcd.io/v2",
					"kind":       "HelmRelease",
					"metadata": map[string]any{
						"name":      "no-history-release",
						"namespace": "default",
					},
					"status": map[string]any{
						"storageNamespace": "default",
						"history":          []any{},
					},
				},
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.helmRelease).
				Build()

			resourceRef := fluxcdv1.ResourceRef{
				ID:      "default_" + tt.helmRelease.GetName() + "_helm.toolkit.fluxcd.io_HelmRelease",
				Version: "v2",
			}

			result, err := FromHelmRelease(context.Background(), kubeClient, resourceRef)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			if tt.expectNil {
				g.Expect(result).To(BeNil())
			}
		})
	}
}

func TestDecodeHelmStorage(t *testing.T) {
	g := NewWithT(t)

	testManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value`

	testStorage := HelmStorage{
		Name:     "test-release",
		Manifest: testManifest,
	}

	tests := []struct {
		name        string
		setupData   func() []byte
		expectError bool
		validate    func(result *HelmStorage)
	}{
		{
			name: "decode uncompressed base64 data",
			setupData: func() []byte {
				data, _ := json.Marshal(testStorage)
				return []byte(base64.StdEncoding.EncodeToString(data))
			},
			expectError: false,
			validate: func(result *HelmStorage) {
				g.Expect(result.Name).To(Equal("test-release"))
				g.Expect(result.Manifest).To(Equal(testManifest))
			},
		},
		{
			name: "decode gzip compressed base64 data",
			setupData: func() []byte {
				data, _ := json.Marshal(testStorage)

				// Compress with gzip
				var buf bytes.Buffer
				gzipWriter := gzip.NewWriter(&buf)
				_, _ = gzipWriter.Write(data)
				_ = gzipWriter.Close()

				// Base64 encode
				return []byte(base64.StdEncoding.EncodeToString(buf.Bytes()))
			},
			expectError: false,
			validate: func(result *HelmStorage) {
				g.Expect(result.Name).To(Equal("test-release"))
				g.Expect(result.Manifest).To(Equal(testManifest))
			},
		},
		{
			name: "invalid base64 data should return error",
			setupData: func() []byte {
				return []byte("invalid-base64-data!")
			},
			expectError: true,
		},
		{
			name: "valid base64 but invalid JSON should return error",
			setupData: func() []byte {
				return []byte(base64.StdEncoding.EncodeToString([]byte("not-json-data")))
			},
			expectError: true,
		},
		{
			name: "invalid gzip data should return error",
			setupData: func() []byte {
				// Create data that looks like gzip (has magic bytes) but is corrupted
				badGzipData := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00} // gzip magic + corrupted data
				return []byte(base64.StdEncoding.EncodeToString(badGzipData))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			releaseData := tt.setupData()
			result, err := decodeHelmStorage(releaseData)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(result).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).NotTo(BeNil())
				if tt.validate != nil {
					tt.validate(result)
				}
			}
		})
	}
}

func TestFromStatusOf(t *testing.T) {

	tests := []struct {
		name              string
		setupObject       func() *unstructured.Unstructured
		resourceRef       fluxcdv1.ResourceRef
		expectedResources int
		expectedIDs       []string
		expectError       bool
	}{
		{
			name: "extract inventory from Kustomization status",
			setupObject: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
						"kind":       "Kustomization",
						"metadata": map[string]any{
							"name":      "test-kustomization",
							"namespace": "flux-system",
						},
						"status": map[string]any{
							"inventory": map[string]any{
								"entries": []any{
									map[string]any{
										"id": "default_test-deployment_apps_Deployment",
										"v":  "v1",
									},
									map[string]any{
										"id": "default_test-service__Service",
										"v":  "v1",
									},
								},
							},
						},
					},
				}
			},
			resourceRef: fluxcdv1.ResourceRef{
				ID:      "flux-system_test-kustomization_kustomize.toolkit.fluxcd.io_Kustomization",
				Version: "v1",
			},
			expectedResources: 2,
			expectedIDs:       []string{"default_test-deployment_apps_Deployment", "default_test-service__Service"},
			expectError:       false,
		},
		{
			name: "extract inventory from ResourceSet status",
			setupObject: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "fluxcd.controlplane.io/v1",
						"kind":       "ResourceSet",
						"metadata": map[string]any{
							"name":      "test-resourceset",
							"namespace": "flux-system",
						},
						"status": map[string]any{
							"inventory": map[string]any{
								"entries": []any{
									map[string]any{
										"id": "flux-system_source-controller_apps_Deployment",
										"v":  "v1",
									},
									map[string]any{
										"id": "flux-system_source-controller__Service",
										"v":  "v1",
									},
									map[string]any{
										"id": "flux-system_source-controller__ServiceAccount",
										"v":  "v1",
									},
								},
							},
						},
					},
				}
			},
			resourceRef: fluxcdv1.ResourceRef{
				ID:      "flux-system_test-resourceset_fluxcd.controlplane.io_ResourceSet",
				Version: "v1",
			},
			expectedResources: 3,
			expectedIDs: []string{
				"flux-system_source-controller_apps_Deployment",
				"flux-system_source-controller__Service",
				"flux-system_source-controller__ServiceAccount",
			},
			expectError: false,
		},
		{
			name: "object with no inventory status returns empty result",
			setupObject: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
						"kind":       "Kustomization",
						"metadata": map[string]any{
							"name":      "empty-kustomization",
							"namespace": "flux-system",
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{
									"type":   "Ready",
									"status": "True",
								},
							},
						},
					},
				}
			},
			resourceRef: fluxcdv1.ResourceRef{
				ID:      "flux-system_empty-kustomization_kustomize.toolkit.fluxcd.io_Kustomization",
				Version: "v1",
			},
			expectedResources: 0,
			expectError:       false,
		},
		{
			name: "object with empty inventory entries returns empty result",
			setupObject: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
						"kind":       "Kustomization",
						"metadata": map[string]any{
							"name":      "empty-inventory-kustomization",
							"namespace": "flux-system",
						},
						"status": map[string]any{
							"inventory": map[string]any{
								"entries": []any{},
							},
						},
					},
				}
			},
			resourceRef: fluxcdv1.ResourceRef{
				ID:      "flux-system_empty-inventory-kustomization_kustomize.toolkit.fluxcd.io_Kustomization",
				Version: "v1",
			},
			expectedResources: 0,
			expectError:       false,
		},
		{
			name: "skip malformed inventory entries",
			setupObject: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
						"kind":       "Kustomization",
						"metadata": map[string]any{
							"name":      "malformed-kustomization",
							"namespace": "flux-system",
						},
						"status": map[string]any{
							"inventory": map[string]any{
								"entries": []any{
									map[string]any{
										"id": "valid_entry_apps_Deployment",
										"v":  "v1",
									},
									map[string]any{
										"id": "missing_version_entry_apps_Service",
										// missing "v" field - should be skipped
									},
									"not-a-map", // invalid entry type - should be skipped
									map[string]any{
										"id": "another_valid_entry__ConfigMap",
										"v":  "v1",
									},
								},
							},
						},
					},
				}
			},
			resourceRef: fluxcdv1.ResourceRef{
				ID:      "flux-system_malformed-kustomization_kustomize.toolkit.fluxcd.io_Kustomization",
				Version: "v1",
			},
			expectedResources: 2, // Only entries with valid types and fields should be extracted
			expectedIDs:       []string{"valid_entry_apps_Deployment", "another_valid_entry__ConfigMap"},
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := tt.setupObject()
			obj.SetGroupVersionKind(obj.GroupVersionKind())

			scheme := runtime.NewScheme()
			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(obj).
				Build()

			result, err := FromStatusOf(context.Background(), kubeClient, tt.resourceRef)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(HaveLen(tt.expectedResources))

				if tt.expectedResources > 0 {
					resultIDs := make([]string, len(result))
					for i, resource := range result {
						resultIDs[i] = resource.ID
						g.Expect(resource.Version).To(Equal("v1"))
					}

					for _, expectedID := range tt.expectedIDs {
						g.Expect(resultIDs).To(ContainElement(expectedID))
					}
				}
			}
		})
	}
}

func TestFromStatusOf_ErrorCases(t *testing.T) {

	tests := []struct {
		name        string
		resourceRef fluxcdv1.ResourceRef
		expectError bool
	}{
		{
			name: "object not found should return error",
			resourceRef: fluxcdv1.ResourceRef{
				ID:      "flux-system_nonexistent-kustomization_kustomize.toolkit.fluxcd.io_Kustomization",
				Version: "v1",
			},
			expectError: true,
		},
		{
			name: "invalid resource ref ID should return error",
			resourceRef: fluxcdv1.ResourceRef{
				ID:      "invalid-format",
				Version: "v1",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			result, err := FromStatusOf(context.Background(), kubeClient, tt.resourceRef)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(result).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
