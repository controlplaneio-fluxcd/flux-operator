// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTrace(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	appsNS := createTraceNamespace(t)
	fluxNS := createTraceNamespace(t)

	createTraceObject(t, map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "OCIRepository",
		"metadata":   map[string]any{"name": "apps-repo", "namespace": fluxNS},
		"spec":       map[string]any{"interval": "1m", "url": "oci://ghcr.io/org/apps"},
	}, map[string]any{
		"conditions": traceReadyCondition("True", "Succeeded", "stored artifact"),
		"artifact":   map[string]any{"revision": "latest@sha256:abc123"},
	})

	createTraceObject(t, map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata": map[string]any{
			"name":      "apps",
			"namespace": fluxNS,
			// The self-referencing labels exercise the cycle guard.
			"labels": map[string]any{
				"kustomize.toolkit.fluxcd.io/name":      "apps",
				"kustomize.toolkit.fluxcd.io/namespace": fluxNS,
			},
		},
		"spec": map[string]any{
			"interval": "1m",
			"path":     "./",
			"prune":    true,
			"sourceRef": map[string]any{
				"kind": "OCIRepository",
				"name": "apps-repo",
			},
		},
	}, map[string]any{
		"conditions":          traceReadyCondition("True", "ReconciliationSucceeded", "Applied revision latest@sha256:abc123"),
		"lastAppliedRevision": "latest@sha256:abc123",
	})

	createTraceObject(t, map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata": map[string]any{
			"name":      "backend",
			"namespace": appsNS,
			"labels": map[string]any{
				"kustomize.toolkit.fluxcd.io/name":      "apps",
				"kustomize.toolkit.fluxcd.io/namespace": fluxNS,
			},
		},
		"spec": map[string]any{"interval": "1m"},
	}, map[string]any{
		"conditions":          traceReadyCondition("False", "UpgradeFailed", "Helm upgrade failed for release backend"),
		"lastAppliedRevision": "6.9.0",
	})

	deploy := createTraceObject(t, map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "backend",
			"namespace": appsNS,
			"labels": map[string]any{
				"helm.toolkit.fluxcd.io/name":      "backend",
				"helm.toolkit.fluxcd.io/namespace": appsNS,
			},
		},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "backend"}},
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]any{"app": "backend"}},
				"spec": map[string]any{"containers": []any{
					map[string]any{"name": "app", "image": "app"},
				}},
			},
		},
	}, nil)

	controller := true
	rs := createTraceObject(t, map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "ReplicaSet",
		"metadata": map[string]any{
			"name":      "backend-7f9d4c",
			"namespace": appsNS,
			"ownerReferences": []any{map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"name":       deploy.GetName(),
				"uid":        string(deploy.GetUID()),
				"controller": true,
			}},
		},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "backend"}},
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]any{"app": "backend"}},
				"spec": map[string]any{"containers": []any{
					map[string]any{"name": "app", "image": "app"},
				}},
			},
		},
	}, nil)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend-7f9d4c-abcde",
			Namespace: appsNS,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       rs.GetName(),
				UID:        rs.GetUID(),
				Controller: &controller,
			}},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "app"}},
		},
	}
	g.Expect(testClient.Client.Create(ctx, pod)).To(Succeed())

	t.Run("traces pod through owners and labels to the source", func(t *testing.T) {
		g := NewWithT(t)
		result, err := testClient.Trace(ctx, TraceOptions{
			APIVersion: "v1", Kind: "Pod", Name: pod.Name, Namespace: appsNS,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Object).To(Equal("Pod/" + appsNS + "/" + pod.Name))
		g.Expect(result.Unmanaged).To(BeFalse())
		g.Expect(result.ManagedBy).NotTo(BeNil())

		deployment := result.ManagedBy
		g.Expect(deployment.Object).To(Equal("Deployment/" + appsNS + "/backend"))
		g.Expect(deployment.Status).To(Equal("InProgress"))

		helmRelease := deployment.ManagedBy
		g.Expect(helmRelease).NotTo(BeNil())
		g.Expect(helmRelease.Object).To(Equal("HelmRelease/" + appsNS + "/backend"))
		g.Expect(helmRelease.Status).To(Equal("UpgradeFailed"))
		g.Expect(helmRelease.Message).To(ContainSubstring("Helm upgrade failed"))
		g.Expect(helmRelease.LastAppliedRevision).To(Equal("6.9.0"))

		kustomization := helmRelease.ManagedBy
		g.Expect(kustomization).NotTo(BeNil())
		g.Expect(kustomization.Object).To(Equal("Kustomization/" + fluxNS + "/apps"))
		g.Expect(kustomization.Status).To(Equal("ReconciliationSucceeded"))
		g.Expect(kustomization.Message).To(BeEmpty())
		g.Expect(kustomization.LastAppliedRevision).To(Equal("latest@sha256:abc123"))
		// The self-referencing ownership labels must not add another manager.
		g.Expect(kustomization.ManagedBy).To(BeNil())

		g.Expect(result.Source).NotTo(BeNil())
		g.Expect(result.Source.ResolvedFor).To(Equal("Kustomization/" + fluxNS + "/apps"))
		g.Expect(result.Source.Object).To(Equal("OCIRepository/" + fluxNS + "/apps-repo"))
		g.Expect(result.Source.Status).To(Equal("Succeeded"))
		g.Expect(result.Source.URL).To(Equal("oci://ghcr.io/org/apps"))
		g.Expect(result.Source.Revision).To(Equal("latest@sha256:abc123"))
		g.Expect(result.Source.ProducedBy).To(BeNil())
		g.Expect(result.Source.BuiltFrom).To(BeEmpty())
	})

	t.Run("traces flux object to its manager and source", func(t *testing.T) {
		g := NewWithT(t)
		result, err := testClient.Trace(ctx, TraceOptions{
			APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Name: "backend", Namespace: appsNS,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Object).To(Equal("HelmRelease/" + appsNS + "/backend"))
		g.Expect(result.Unmanaged).To(BeFalse())
		g.Expect(result.ManagedBy).NotTo(BeNil())
		g.Expect(result.ManagedBy.Object).To(Equal("Kustomization/" + fluxNS + "/apps"))
		g.Expect(result.ManagedBy.ManagedBy).To(BeNil())
		g.Expect(result.Source).NotTo(BeNil())
		g.Expect(result.Source.ResolvedFor).To(Equal("Kustomization/" + fluxNS + "/apps"))
		g.Expect(result.Source.Object).To(Equal("OCIRepository/" + fluxNS + "/apps-repo"))
	})

	t.Run("resolves the source of an unmanaged helmrelease through the chart", func(t *testing.T) {
		g := NewWithT(t)
		createTraceObject(t, map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "HelmRepository",
			"metadata":   map[string]any{"name": "charts", "namespace": fluxNS},
			"spec":       map[string]any{"interval": "1m", "url": "https://charts.example.com"},
		}, map[string]any{
			"conditions": traceReadyCondition("True", "Succeeded", "stored artifact"),
			"artifact":   map[string]any{"revision": "sha256:d4e5f6"},
		})
		createTraceObject(t, map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "HelmChart",
			"metadata":   map[string]any{"name": "frontend-chart", "namespace": fluxNS},
			"spec": map[string]any{
				"chart": "frontend",
				"sourceRef": map[string]any{
					"kind": "HelmRepository",
					"name": "charts",
				},
			},
		}, nil)
		createTraceObject(t, map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata":   map[string]any{"name": "frontend", "namespace": appsNS},
			"spec": map[string]any{
				"interval": "1m",
				"chartRef": map[string]any{
					"kind":      "HelmChart",
					"name":      "frontend-chart",
					"namespace": fluxNS,
				},
			},
		}, nil)

		result, err := testClient.Trace(ctx, TraceOptions{
			APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Name: "frontend", Namespace: appsNS,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Unmanaged).To(BeTrue())
		g.Expect(result.ManagedBy).To(BeNil())
		g.Expect(result.Source).NotTo(BeNil())
		g.Expect(result.Source.ResolvedFor).To(Equal("HelmRelease/" + appsNS + "/frontend"))
		g.Expect(result.Source.Object).To(Equal("HelmRepository/" + fluxNS + "/charts"))
		g.Expect(result.Source.URL).To(Equal("https://charts.example.com"))
		g.Expect(result.Source.Revision).To(Equal("sha256:d4e5f6"))

		// Tracing the HelmChart itself resolves its repository source.
		result, err = testClient.Trace(ctx, TraceOptions{
			APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "HelmChart", Name: "frontend-chart", Namespace: fluxNS,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Unmanaged).To(BeTrue())
		g.Expect(result.ManagedBy).To(BeNil())
		g.Expect(result.Source).NotTo(BeNil())
		g.Expect(result.Source.ResolvedFor).To(Equal("HelmChart/" + fluxNS + "/frontend-chart"))
		g.Expect(result.Source.Object).To(Equal("HelmRepository/" + fluxNS + "/charts"))
	})

	t.Run("follows external artifact to its generator", func(t *testing.T) {
		g := NewWithT(t)
		createTraceObject(t, map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "HelmRepository",
			"metadata":   map[string]any{"name": "gen-charts", "namespace": fluxNS},
			"spec":       map[string]any{"interval": "1m", "url": "https://gen-charts.example.com"},
		}, map[string]any{
			"conditions": traceReadyCondition("True", "Succeeded", "stored artifact"),
		})
		createTraceObject(t, map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "HelmChart",
			"metadata":   map[string]any{"name": "gen-chart", "namespace": fluxNS},
			"spec": map[string]any{
				"chart": "gen",
				"sourceRef": map[string]any{
					"kind": "HelmRepository",
					"name": "gen-charts",
				},
			},
		}, nil)
		createTraceObject(t, map[string]any{
			"apiVersion": "source.extensions.fluxcd.io/v1",
			"kind":       "ArtifactGenerator",
			"metadata":   map[string]any{"name": "gen", "namespace": fluxNS},
			"spec": map[string]any{
				"sources": []any{
					map[string]any{
						"alias": "repo",
						"kind":  "OCIRepository",
						"name":  "apps-repo",
					},
					map[string]any{
						"alias": "chart",
						"kind":  "HelmChart",
						"name":  "gen-chart",
					},
				},
			},
		}, map[string]any{
			"conditions": traceReadyCondition("True", "ReconciliationSucceeded", "stored artifact"),
		})
		createTraceObject(t, map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "ExternalArtifact",
			"metadata":   map[string]any{"name": "gen-artifact", "namespace": fluxNS},
			"spec": map[string]any{
				"sourceRef": map[string]any{
					"apiVersion": "source.extensions.fluxcd.io/v1",
					"kind":       "ArtifactGenerator",
					"name":       "gen",
					"namespace":  fluxNS,
				},
			},
		}, map[string]any{
			"conditions": traceReadyCondition("True", "Succeeded", "stored artifact"),
			"artifact": map[string]any{
				"revision": "sha256:1a2b3c",
				"url":      "http://source-watcher.flux-system/ea.tar.gz",
			},
		})
		createTraceObject(t, map[string]any{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]any{"name": "gen-apps", "namespace": fluxNS},
			"spec": map[string]any{
				"interval": "1m",
				"path":     "./",
				"prune":    true,
				"sourceRef": map[string]any{
					"kind": "ExternalArtifact",
					"name": "gen-artifact",
				},
			},
		}, nil)

		result, err := testClient.Trace(ctx, TraceOptions{
			APIVersion: "kustomize.toolkit.fluxcd.io/v1", Kind: "Kustomization", Name: "gen-apps", Namespace: fluxNS,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Unmanaged).To(BeTrue())
		g.Expect(result.ManagedBy).To(BeNil())
		g.Expect(result.Source).NotTo(BeNil())
		g.Expect(result.Source.ResolvedFor).To(Equal("Kustomization/" + fluxNS + "/gen-apps"))
		g.Expect(result.Source.Object).To(Equal("ExternalArtifact/" + fluxNS + "/gen-artifact"))
		g.Expect(result.Source.URL).To(Equal("http://source-watcher.flux-system/ea.tar.gz"))
		g.Expect(result.Source.Revision).To(Equal("sha256:1a2b3c"))
		g.Expect(result.Source.BuiltFrom).To(BeEmpty())
		g.Expect(result.Source.ProducedBy).NotTo(BeNil())
		g.Expect(result.Source.ProducedBy.Object).To(Equal("ArtifactGenerator/" + fluxNS + "/gen"))
		g.Expect(result.Source.ProducedBy.Status).To(Equal("ReconciliationSucceeded"))
		g.Expect(result.Source.ProducedBy.BuiltFrom).To(HaveLen(2))
		g.Expect(result.Source.ProducedBy.BuiltFrom[0].Object).To(Equal("OCIRepository/" + fluxNS + "/apps-repo"))
		g.Expect(result.Source.ProducedBy.BuiltFrom[0].URL).To(Equal("oci://ghcr.io/org/apps"))
		g.Expect(result.Source.ProducedBy.BuiltFrom[0].Revision).To(Equal("latest@sha256:abc123"))
		// A HelmChart upstream is resolved to its HelmRepository.
		g.Expect(result.Source.ProducedBy.BuiltFrom[1].Object).To(Equal("HelmRepository/" + fluxNS + "/gen-charts"))
		g.Expect(result.Source.ProducedBy.BuiltFrom[1].URL).To(Equal("https://gen-charts.example.com"))

		// Tracing the ExternalArtifact itself resolves the generator and its upstreams.
		result, err = testClient.Trace(ctx, TraceOptions{
			APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "ExternalArtifact", Name: "gen-artifact", Namespace: fluxNS,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Unmanaged).To(BeTrue())
		g.Expect(result.ManagedBy).To(BeNil())
		g.Expect(result.Source).NotTo(BeNil())
		g.Expect(result.Source.ResolvedFor).To(Equal("ExternalArtifact/" + fluxNS + "/gen-artifact"))
		g.Expect(result.Source.Object).To(Equal("ArtifactGenerator/" + fluxNS + "/gen"))
		g.Expect(result.Source.ProducedBy).To(BeNil())
		g.Expect(result.Source.BuiltFrom).To(HaveLen(2))
		g.Expect(result.Source.BuiltFrom[0].Object).To(Equal("OCIRepository/" + fluxNS + "/apps-repo"))
		g.Expect(result.Source.BuiltFrom[1].Object).To(Equal("HelmRepository/" + fluxNS + "/gen-charts"))
	})

	t.Run("reports unmanaged objects", func(t *testing.T) {
		g := NewWithT(t)
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "unmanaged", Namespace: appsNS},
		}
		g.Expect(testClient.Client.Create(ctx, cm)).To(Succeed())

		result, err := testClient.Trace(ctx, TraceOptions{
			APIVersion: "v1", Kind: "ConfigMap", Name: cm.Name, Namespace: appsNS,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Object).To(Equal("ConfigMap/" + appsNS + "/unmanaged"))
		g.Expect(result.Unmanaged).To(BeTrue())
		g.Expect(result.ManagedBy).To(BeNil())
		g.Expect(result.Source).To(BeNil())
	})

	t.Run("errors on not found objects", func(t *testing.T) {
		g := NewWithT(t)
		_, err := testClient.Trace(ctx, TraceOptions{
			APIVersion: "v1", Kind: "ConfigMap", Name: "not-found", Namespace: appsNS,
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("unable to get ConfigMap"))
	})

	t.Run("marks suspended links", func(t *testing.T) {
		g := NewWithT(t)
		ks := &unstructured.Unstructured{}
		ks.SetAPIVersion("kustomize.toolkit.fluxcd.io/v1")
		ks.SetKind("Kustomization")
		g.Expect(testClient.Client.Get(ctx, testClient.ObjectKeyFromObject(&metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{Name: "apps", Namespace: fluxNS},
		}), ks)).To(Succeed())
		g.Expect(unstructured.SetNestedField(ks.Object, true, "spec", "suspend")).To(Succeed())
		g.Expect(testClient.Client.Update(ctx, ks)).To(Succeed())

		result, err := testClient.Trace(ctx, TraceOptions{
			APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Name: "backend", Namespace: appsNS,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.ManagedBy).NotTo(BeNil())
		g.Expect(result.ManagedBy.Object).To(Equal("Kustomization/" + fluxNS + "/apps"))
		g.Expect(result.ManagedBy.Suspended).To(BeTrue())
	})
}

// createTraceNamespace creates a namespace with a generated trace- prefixed name.
func createTraceNamespace(t *testing.T) string {
	t.Helper()
	g := NewWithT(t)
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "trace-"}}
	g.Expect(testClient.Client.Create(context.Background(), namespace)).To(Succeed())
	return namespace.Name
}

// createTraceObject creates an unstructured object and optionally sets its status subresource.
func createTraceObject(t *testing.T, obj map[string]any, statusFields map[string]any) *unstructured.Unstructured {
	t.Helper()
	g := NewWithT(t)
	u := &unstructured.Unstructured{Object: obj}
	g.Expect(testClient.Client.Create(context.Background(), u)).To(Succeed())
	if statusFields != nil {
		u.Object["status"] = statusFields
		g.Expect(testClient.Client.Status().Update(context.Background(), u)).To(Succeed())
	}
	return u
}

// traceReadyCondition renders a Ready condition slice for status fixtures.
func traceReadyCondition(status, reason, message string) []any {
	return []any{map[string]any{
		"type":               "Ready",
		"status":             status,
		"reason":             reason,
		"message":            message,
		"lastTransitionTime": "2026-01-01T00:00:00Z",
	}}
}
