// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func makeHelmRelease(namespace, apiVersion, storageNs string, history []any, kubeConfig bool) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": apiVersion,
		"kind":       "HelmRelease",
		"metadata": map[string]any{
			"name":      "my-release",
			"namespace": namespace,
		},
		"status": map[string]any{},
	}

	if storageNs != "" {
		obj["status"].(map[string]any)["storageNamespace"] = storageNs
	}
	if len(history) > 0 {
		obj["status"].(map[string]any)["history"] = history
	}
	if kubeConfig {
		obj["spec"] = map[string]any{
			"kubeConfig": map[string]any{
				"secretRef": map[string]any{"name": "remote-kubeconfig"},
			},
		}
	}

	return &unstructured.Unstructured{Object: obj}
}

func encodeHelmStorage(t *testing.T, manifest string, useGzip bool) []byte {
	t.Helper()
	storage := HelmStorage{
		Name:     "test-release",
		Manifest: manifest,
	}
	data, err := json.Marshal(storage)
	if err != nil {
		t.Fatalf("failed to marshal helm storage: %v", err)
	}

	if useGzip {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			t.Fatalf("failed to gzip: %v", err)
		}
		_ = w.Close()
		data = buf.Bytes()
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return []byte(encoded)
}

func TestGetHelmInventory(t *testing.T) {
	apiVersion := "helm.toolkit.fluxcd.io/v2"
	namespace := "flux-system"

	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: flux-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deploy
  namespace: flux-system
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx:1.25
      initContainers:
      - name: init
        image: busybox:1.36
`

	history := []any{
		map[string]any{
			"chartName": "test-chart",
			"version":   int64(1),
			"namespace": namespace,
		},
	}

	tests := []struct {
		testName    string
		objects     []client.Object
		matchErr    string
		matchLen    int
		nilResult   bool
		matchImages []string
	}{
		{
			testName: "extracts inventory from helm storage",
			objects: []client.Object{
				makeHelmRelease(namespace, apiVersion, namespace, history, false),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sh.helm.release.v1.test-chart.v1",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"release": encodeHelmStorage(t, manifest, false),
					},
				},
			},
			matchLen:    2,
			matchImages: []string{"nginx:1.25", "busybox:1.36"},
		},
		{
			testName: "extracts inventory from gzipped helm storage",
			objects: []client.Object{
				makeHelmRelease(namespace, apiVersion, namespace, history, false),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sh.helm.release.v1.test-chart.v1",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"release": encodeHelmStorage(t, manifest, true),
					},
				},
			},
			matchLen:    2,
			matchImages: []string{"nginx:1.25", "busybox:1.36"},
		},
		{
			testName:  "skips release with remote kubeConfig",
			objects:   []client.Object{makeHelmRelease(namespace, apiVersion, namespace, history, true)},
			nilResult: true,
		},
		{
			testName:  "skips release without storage namespace",
			objects:   []client.Object{makeHelmRelease(namespace, apiVersion, "", history, false)},
			nilResult: true,
		},
		{
			testName:  "skips release without history",
			objects:   []client.Object{makeHelmRelease(namespace, apiVersion, namespace, nil, false)},
			nilResult: true,
		},
		{
			testName: "returns error when release not found",
			objects:  []client.Object{},
			matchErr: "not found",
		},
		{
			testName: "returns error when secret has no release data",
			objects: []client.Object{
				makeHelmRelease(namespace, apiVersion, namespace, history, false),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sh.helm.release.v1.test-chart.v1",
						Namespace: namespace,
					},
					Data: map[string][]byte{},
				},
			},
			matchErr: "failed to decode",
		},
		{
			testName: "skips when storage secret not found",
			objects: []client.Object{
				makeHelmRelease(namespace, apiVersion, namespace, history, false),
			},
			nilResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			kubeClient := Client{
				Client: fake.NewClientBuilder().
					WithScheme(NewTestScheme()).
					WithObjects(tt.objects...).
					Build(),
			}

			objectKey := client.ObjectKey{
				Name:      "my-release",
				Namespace: namespace,
			}

			inventory, err := kubeClient.GetHelmInventory(
				context.Background(),
				apiVersion,
				objectKey,
			)

			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
			} else if tt.nilResult {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(inventory).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(inventory).To(HaveLen(tt.matchLen))

				// Check container images are extracted
				allImages := make([]string, 0)
				for _, item := range inventory {
					allImages = append(allImages, item.ContainerImages...)
				}
				for _, img := range tt.matchImages {
					g.Expect(allImages).To(ContainElement(img))
				}
			}
		})
	}
}
