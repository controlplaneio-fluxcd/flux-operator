// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	crtlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// Registerer returns the metrics registerer.
func Registerer() prometheus.Registerer {
	return crtlmetrics.Registry
}

// MustRegisterMetrics attempts to register the metrics collectors
// in the controller-runtime metrics registry.
func MustRegisterMetrics() {
	collectors := make([]prometheus.Collector, len(metrics))
	i := 0
	for _, metric := range metrics {
		collectors[i] = metric
		i++
	}
	crtlmetrics.Registry.MustRegister(collectors...)
}

// RecordMetrics records the metrics for the given object.
func RecordMetrics(obj unstructured.Unstructured) {
	kind := obj.GetKind()
	labels := commonLabelsToValues(obj)
	switch kind {
	case fluxcdv1.FluxInstanceKind:
		registry, _, _ := unstructured.NestedString(obj.Object, "spec", "distribution", "registry")
		labels["registry"] = registry

		applyRev, _, _ := unstructured.NestedString(obj.Object, "status", "lastAppliedRevision")
		labels["revision"] = applyRev

		labels["suspended"] = falseValue
		val, ok := obj.GetAnnotations()[fluxcdv1.ReconcileAnnotation]
		if ok && strings.ToLower(val) == fluxcdv1.DisabledValue {
			labels["suspended"] = trueValue
		}

		metrics[kind].Reset()
		metrics[kind].With(labels).Set(1)
	case fluxcdv1.ResourceSetKind:
		applyRev, _, _ := unstructured.NestedString(obj.Object, "status", "lastAppliedRevision")
		labels["revision"] = applyRev

		labels["suspended"] = falseValue
		val, ok := obj.GetAnnotations()[fluxcdv1.ReconcileAnnotation]
		if ok && strings.ToLower(val) == fluxcdv1.DisabledValue {
			labels["suspended"] = trueValue
		}

		metrics[kind].DeletePartialMatch(map[string]string{
			"name":               labels["name"],
			"exported_namespace": labels["exported_namespace"],
		})
		metrics[kind].With(labels).Set(1)
	case fluxcdv1.ResourceSetInputProviderKind:
		sourceURL, _, _ := unstructured.NestedString(obj.Object, "spec", "url")
		labels["url"] = sourceURL

		labels["suspended"] = falseValue
		val, ok := obj.GetAnnotations()[fluxcdv1.ReconcileAnnotation]
		if ok && strings.ToLower(val) == fluxcdv1.DisabledValue {
			labels["suspended"] = trueValue
		}

		metrics[kind].DeletePartialMatch(map[string]string{
			"name":               labels["name"],
			"exported_namespace": labels["exported_namespace"],
		})
		metrics[kind].With(labels).Set(1)
	default:
		metrics["FluxResource"].With(fluxLabelsToValues(obj)).Set(1)
	}
}

// ResetMetrics resets the metrics for the given kind.
func ResetMetrics(kind string) {
	metrics[kind].Reset()
}

func DeleteMetricsFor(kind, name, namespace string) {
	metrics[kind].DeletePartialMatch(map[string]string{
		"name":               name,
		"exported_namespace": namespace,
	})
}

const (
	trueValue  = "True"
	falseValue = "False"
)

var commonLabels = []string{"uid", "kind", "name", "exported_namespace", "ready", "reason", "suspended"}

var metrics = map[string]*prometheus.GaugeVec{
	fluxcdv1.FluxInstanceKind: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "flux_instance_info",
			Help: "The current status of a Flux instance.",
		},
		append(commonLabels, "registry", "revision"),
	),
	fluxcdv1.ResourceSetKind: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "flux_resourceset_info",
			Help: "The current status of a Flux Operator ResourceSet.",
		},
		append(commonLabels, "revision"),
	),
	fluxcdv1.ResourceSetInputProviderKind: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "flux_resourcesetinputprovider_info",
			Help: "The current status of a Flux Operator ResourceSetInputProvider.",
		},
		append(commonLabels, "url"),
	),
	"FluxResource": prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "flux_resource_info",
			Help: "The current status of a Flux resource reconciliation.",
		},
		append(commonLabels,
			"revision",
			"url",
			"ref",
			"source_name",
			"path",
		),
	),
}

func commonLabelsToValues(obj unstructured.Unstructured) prometheus.Labels {
	labels := prometheus.Labels{}
	labels["uid"] = string(obj.GetUID())
	labels["kind"] = obj.GetKind()
	labels["name"] = obj.GetName()
	labels["exported_namespace"] = obj.GetNamespace()
	labels["ready"] = "Unknown"
	labels["reason"] = ""

	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if found {
		for _, condition := range conditions {
			conditionMap := condition.(map[string]any)
			if conditionMap["type"] == "Ready" {
				labels["ready"] = conditionMap["status"].(string)
				labels["reason"] = conditionMap["reason"].(string)
				break
			}
		}
	}

	return labels
}

// nolint: gocyclo
func fluxLabelsToValues(obj unstructured.Unstructured) prometheus.Labels {
	labels := commonLabelsToValues(obj)
	labels["suspended"] = falseValue
	labels["revision"] = ""
	labels["url"] = ""
	labels["ref"] = ""
	labels["source_name"] = ""
	labels["path"] = ""

	if suspended, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend"); suspended {
		labels["suspended"] = trueValue
	}

	if source, found, _ := unstructured.NestedString(obj.Object, "spec", "sourceRef", "name"); found {
		labels["source_name"] = source
	}

	if sourceRev, found, _ := unstructured.NestedString(obj.Object, "status", "artifact", "revision"); found {
		labels["revision"] = sourceRev
	}

	if applyRev, found, _ := unstructured.NestedString(obj.Object, "status", "lastAttemptedRevision"); found {
		labels["revision"] = applyRev
	}

	if sourceURL, found, _ := unstructured.NestedString(obj.Object, "spec", "url"); found {
		labels["url"] = sourceURL
	}

	switch obj.GetKind() {
	case "Kustomization":
		if sourcePath, found, _ := unstructured.NestedString(obj.Object, "spec", "path"); found {
			labels["path"] = sourcePath
		}
	case "GitRepository":
		if sourceRef, found, _ := unstructured.NestedString(obj.Object, "spec", "ref", "branch"); found {
			labels["ref"] = sourceRef
		}
		if sourceRef, found, _ := unstructured.NestedString(obj.Object, "spec", "ref", "tag"); found {
			labels["ref"] = sourceRef
		}
		if sourceRef, found, _ := unstructured.NestedString(obj.Object, "spec", "ref", "semver"); found {
			labels["ref"] = sourceRef
		}
		if sourceRef, found, _ := unstructured.NestedString(obj.Object, "spec", "ref", "name"); found {
			labels["ref"] = sourceRef
		}
	case "OCIRepository":
		if sourceRef, found, _ := unstructured.NestedString(obj.Object, "spec", "ref", "tag"); found {
			labels["ref"] = sourceRef
		}
		if sourceRef, found, _ := unstructured.NestedString(obj.Object, "spec", "ref", "semver"); found {
			labels["ref"] = sourceRef
		}
	case "Bucket":
		if sourceURL, found, _ := unstructured.NestedString(obj.Object, "spec", "endpoint"); found {
			labels["url"] = sourceURL
		}
		if sourceRef, found, _ := unstructured.NestedString(obj.Object, "spec", "bucketName"); found {
			labels["ref"] = sourceRef
		}
	case "HelmRelease":
		if source, found, _ := unstructured.NestedString(obj.Object, "spec", "chartRef", "name"); found {
			labels["source_name"] = source
		}
		if source, found, _ := unstructured.NestedString(obj.Object, "spec", "chart", "spec", "sourceRef", "name"); found {
			labels["source_name"] = source
		}
	case "HelmRepository":
		if t, _, _ := unstructured.NestedString(obj.Object, "spec", "type"); t == "oci" {
			labels["ready"] = trueValue
		}
	case "Alert":
		labels["ready"] = trueValue
	case "Provider":
		labels["ready"] = trueValue
	case "Receiver":
		if url, found, _ := unstructured.NestedString(obj.Object, "status", "webhookPath"); found {
			labels["url"] = url
		}
	case "ImageRepository":
		if image, found, _ := unstructured.NestedString(obj.Object, "spec", "image"); found {
			labels["url"] = image
		}
	case "ImagePolicy":
		if source, found, _ := unstructured.NestedString(obj.Object, "spec", "imageRepositoryRef", "name"); found {
			labels["source_name"] = source
		}
	}

	return labels
}
