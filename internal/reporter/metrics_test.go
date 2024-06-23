package reporter

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRecordMetrics_FluxResource(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics["FluxResource"])

	repo := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "GitRepository",
			"metadata": map[string]interface{}{
				"uid":       "f252c583-d7b7-4236-b436-618eb5eb3023",
				"name":      "flux-system",
				"namespace": "flux-system",
			},
			"spec": map[string]interface{}{
				"url": "ssh://test/repo",
				"ref": map[string]interface{}{
					"branch": "main",
				},
			},
			"status": map[string]interface{}{
				"artifact": map[string]interface{}{
					"revision": "6.6.3@sha1:b0c487c6b217bed8e6a53fca25f6ee1a7dd573e3",
				},
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "Unknown",
					},
				},
			},
		},
	}

	RecordMetrics(repo)

	ks := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata": map[string]interface{}{
				"uid":       "1a0105c8-1ad2-4c5d-9d25-22096796156f",
				"name":      "flux-system",
				"namespace": "flux-system",
			},
			"spec": map[string]interface{}{
				"sourceRef": map[string]interface{}{
					"kind": "GitRepository",
					"name": "flux-system",
				},
				"path":    "clusters/production",
				"suspend": true,
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
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
	g.Expect(repoLabels).To(HaveLen(11))
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
	g.Expect(repoLabels[5].GetName()).To(Equal("ref"))
	g.Expect(repoLabels[5].GetValue()).To(Equal("main"))
	g.Expect(repoLabels[6].GetName()).To(Equal("revision"))
	g.Expect(repoLabels[6].GetValue()).To(Equal("6.6.3@sha1:b0c487c6b217bed8e6a53fca25f6ee1a7dd573e3"))
	g.Expect(repoLabels[7].GetName()).To(Equal("source_name"))
	g.Expect(repoLabels[7].GetValue()).To(Equal(""))
	g.Expect(repoLabels[8].GetName()).To(Equal("suspended"))
	g.Expect(repoLabels[8].GetValue()).To(Equal("False"))
	g.Expect(repoLabels[9].GetName()).To(Equal("uid"))
	g.Expect(repoLabels[9].GetValue()).To(Equal("f252c583-d7b7-4236-b436-618eb5eb3023"))
	g.Expect(repoLabels[10].GetName()).To(Equal("url"))
	g.Expect(repoLabels[10].GetValue()).To(Equal("ssh://test/repo"))

	ksMetric := metricFamilies[0].Metric[1]
	ksLabels := ksMetric.GetLabel()
	g.Expect(ksLabels).To(HaveLen(11))
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
	g.Expect(ksLabels[5].GetName()).To(Equal("ref"))
	g.Expect(ksLabels[5].GetValue()).To(Equal(""))
	g.Expect(ksLabels[6].GetName()).To(Equal("revision"))
	g.Expect(ksLabels[6].GetValue()).To(Equal("6.6.3@sha1:b0c487c6b217bed8e6a53fca25f6ee1a7dd573e3"))
	g.Expect(ksLabels[7].GetName()).To(Equal("source_name"))
	g.Expect(ksLabels[7].GetValue()).To(Equal("flux-system"))
	g.Expect(ksLabels[8].GetName()).To(Equal("suspended"))
	g.Expect(ksLabels[8].GetValue()).To(Equal("True"))
	g.Expect(ksLabels[9].GetName()).To(Equal("uid"))
	g.Expect(ksLabels[9].GetValue()).To(Equal("1a0105c8-1ad2-4c5d-9d25-22096796156f"))
	g.Expect(ksLabels[10].GetName()).To(Equal("url"))
	g.Expect(ksLabels[10].GetValue()).To(Equal(""))

	ResetMetrics("FluxResource")
	metricFamilies, err = reg.Gather()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(metricFamilies).To(BeEmpty())
}
