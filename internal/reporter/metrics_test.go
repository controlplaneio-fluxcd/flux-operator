// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestRecordMetrics_FluxResource(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics["FluxResource"])

	repo := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "GitRepository",
			"metadata": map[string]any{
				"uid":       "f252c583-d7b7-4236-b436-618eb5eb3023",
				"name":      "flux-system",
				"namespace": "flux-system",
			},
			"spec": map[string]any{
				"url": "ssh://test/repo",
				"ref": map[string]any{
					"branch": "main",
				},
			},
			"status": map[string]any{
				"artifact": map[string]any{
					"revision": "6.6.3@sha1:b0c487c6b217bed8e6a53fca25f6ee1a7dd573e3",
				},
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "Unknown",
						"reason": "Progressing",
					},
				},
			},
		},
	}

	RecordMetrics(repo)

	ks := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata": map[string]any{
				"uid":       "1a0105c8-1ad2-4c5d-9d25-22096796156f",
				"name":      "flux-system",
				"namespace": "flux-system",
			},
			"spec": map[string]any{
				"sourceRef": map[string]any{
					"kind": "GitRepository",
					"name": "flux-system",
				},
				"path":    "clusters/production",
				"suspend": true,
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "True",
						"reason": "ReconciliationSucceeded",
					},
				},
				"lastAttemptedRevision": "6.6.3@sha1:b0c487c6b217bed8e6a53fca25f6ee1a7dd573e3",
			},
		},
	}

	RecordMetrics(ks)

	metricFamilies, err := reg.Gather()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(metricFamilies).To(HaveLen(1))
	g.Expect(metricFamilies[0].Metric).To(HaveLen(2))

	repoMetric := metricFamilies[0].Metric[0]
	repoLabels := repoMetric.GetLabel()
	g.Expect(repoLabels).To(HaveLen(12))
	g.Expect(repoLabels[0].GetName()).To(Equal("exported_namespace"))
	g.Expect(repoLabels[0].GetValue()).To(Equal("flux-system"))
	g.Expect(repoLabels[1].GetName()).To(Equal("kind"))
	g.Expect(repoLabels[1].GetValue()).To(Equal("GitRepository"))
	g.Expect(repoLabels[2].GetName()).To(Equal("name"))
	g.Expect(repoLabels[2].GetValue()).To(Equal("flux-system"))
	g.Expect(repoLabels[3].GetName()).To(Equal("path"))
	g.Expect(repoLabels[3].GetValue()).To(Equal(""))
	g.Expect(repoLabels[4].GetName()).To(Equal("ready"))
	g.Expect(repoLabels[4].GetValue()).To(Equal("Unknown"))
	g.Expect(repoLabels[5].GetName()).To(Equal("reason"))
	g.Expect(repoLabels[5].GetValue()).To(Equal("Progressing"))
	g.Expect(repoLabels[6].GetName()).To(Equal("ref"))
	g.Expect(repoLabels[6].GetValue()).To(Equal("main"))
	g.Expect(repoLabels[7].GetName()).To(Equal("revision"))
	g.Expect(repoLabels[7].GetValue()).To(Equal("6.6.3@sha1:b0c487c6b217bed8e6a53fca25f6ee1a7dd573e3"))
	g.Expect(repoLabels[8].GetName()).To(Equal("source_name"))
	g.Expect(repoLabels[8].GetValue()).To(Equal(""))
	g.Expect(repoLabels[9].GetName()).To(Equal("suspended"))
	g.Expect(repoLabels[9].GetValue()).To(Equal("False"))
	g.Expect(repoLabels[10].GetName()).To(Equal("uid"))
	g.Expect(repoLabels[10].GetValue()).To(Equal("f252c583-d7b7-4236-b436-618eb5eb3023"))
	g.Expect(repoLabels[11].GetName()).To(Equal("url"))
	g.Expect(repoLabels[11].GetValue()).To(Equal("ssh://test/repo"))

	ksMetric := metricFamilies[0].Metric[1]
	ksLabels := ksMetric.GetLabel()
	g.Expect(ksLabels).To(HaveLen(12))
	g.Expect(ksLabels[0].GetName()).To(Equal("exported_namespace"))
	g.Expect(ksLabels[0].GetValue()).To(Equal("flux-system"))
	g.Expect(ksLabels[1].GetName()).To(Equal("kind"))
	g.Expect(ksLabels[1].GetValue()).To(Equal("Kustomization"))
	g.Expect(ksLabels[2].GetName()).To(Equal("name"))
	g.Expect(ksLabels[2].GetValue()).To(Equal("flux-system"))
	g.Expect(ksLabels[3].GetName()).To(Equal("path"))
	g.Expect(ksLabels[3].GetValue()).To(Equal("clusters/production"))
	g.Expect(ksLabels[4].GetName()).To(Equal("ready"))
	g.Expect(ksLabels[4].GetValue()).To(Equal("True"))
	g.Expect(ksLabels[5].GetName()).To(Equal("reason"))
	g.Expect(ksLabels[5].GetValue()).To(Equal("ReconciliationSucceeded"))
	g.Expect(ksLabels[6].GetName()).To(Equal("ref"))
	g.Expect(ksLabels[6].GetValue()).To(Equal(""))
	g.Expect(ksLabels[7].GetName()).To(Equal("revision"))
	g.Expect(ksLabels[7].GetValue()).To(Equal("6.6.3@sha1:b0c487c6b217bed8e6a53fca25f6ee1a7dd573e3"))
	g.Expect(ksLabels[8].GetName()).To(Equal("source_name"))
	g.Expect(ksLabels[8].GetValue()).To(Equal("flux-system"))
	g.Expect(ksLabels[9].GetName()).To(Equal("suspended"))
	g.Expect(ksLabels[9].GetValue()).To(Equal("True"))
	g.Expect(ksLabels[10].GetName()).To(Equal("uid"))
	g.Expect(ksLabels[10].GetValue()).To(Equal("1a0105c8-1ad2-4c5d-9d25-22096796156f"))
	g.Expect(ksLabels[11].GetName()).To(Equal("url"))
	g.Expect(ksLabels[11].GetValue()).To(Equal(""))

	ResetMetrics("FluxResource")
	metricFamilies, err = reg.Gather()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(metricFamilies).To(BeEmpty())
}

func TestRecordMetrics_ResourceSet(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics[fluxcdv1.ResourceSetKind])

	rs := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "toolkit.fluxcd.io/v1",
			"kind":       "ResourceSet",
			"metadata": map[string]any{
				"uid":       "f252c583-d7b7-4236-b436-618eb5eb3023",
				"name":      "test",
				"namespace": "flux-system",
			},
			"spec": map[string]any{
				"resources": []any{
					map[string]any{
						"kind": "GitRepository",
						"name": "flux-system",
					},
				},
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "True",
						"reason": "ReconciliationSucceeded",
					},
				},
				"lastAppliedRevision": "b0c487c6b217bed8e6a53fca25f6ee1a7dd573e3",
			},
		},
	}

	RecordMetrics(rs)

	rsNew := rs.DeepCopy()
	rsNew.SetName("test2")
	RecordMetrics(*rsNew)

	metricFamilies, err := reg.Gather()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(metricFamilies).To(HaveLen(1))
	g.Expect(metricFamilies[0].Metric).To(HaveLen(2))

	rsMetric := metricFamilies[0].Metric[0]
	rsLabels := rsMetric.GetLabel()
	g.Expect(rsLabels).To(HaveLen(8))
	g.Expect(rsLabels[0].GetName()).To(Equal("exported_namespace"))
	g.Expect(rsLabels[0].GetValue()).To(Equal("flux-system"))
	g.Expect(rsLabels[1].GetName()).To(Equal("kind"))
	g.Expect(rsLabels[1].GetValue()).To(Equal("ResourceSet"))
	g.Expect(rsLabels[2].GetName()).To(Equal("name"))
	g.Expect(rsLabels[2].GetValue()).To(Equal("test"))
	g.Expect(rsLabels[3].GetName()).To(Equal("ready"))
	g.Expect(rsLabels[3].GetValue()).To(Equal("True"))
	g.Expect(rsLabels[4].GetName()).To(Equal("reason"))
	g.Expect(rsLabels[4].GetValue()).To(Equal("ReconciliationSucceeded"))
	g.Expect(rsLabels[5].GetName()).To(Equal("revision"))
	g.Expect(rsLabels[5].GetValue()).To(Equal("b0c487c6b217bed8e6a53fca25f6ee1a7dd573e3"))

	DeleteMetricsFor(fluxcdv1.ResourceSetKind, rs.GetName(), rs.GetNamespace())
	metricFamilies, err = reg.Gather()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(metricFamilies).To(HaveLen(1))
	g.Expect(metricFamilies[0].Metric).To(HaveLen(1))
}
