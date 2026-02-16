// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestNewResourceStatus_ReadyTrue(t *testing.T) {
	g := NewWithT(t)

	obj := makeObj("Kustomization", "app-1", "team-a")
	setCondition(obj, "True", "ReconciliationSucceeded", "applied", "2025-06-15T10:30:00Z")

	rs := NewResourceStatus(obj)
	g.Expect(rs.Status).To(Equal(StatusReady))
	g.Expect(rs.Message).To(Equal("applied"))
	g.Expect(rs.Name).To(Equal("app-1"))
	g.Expect(rs.Kind).To(Equal("Kustomization"))
	g.Expect(rs.Namespace).To(Equal("team-a"))
	g.Expect(rs.LastReconciled.Format("2006-01-02")).To(Equal("2025-06-15"))
}

func TestNewResourceStatus_ReadyFalse(t *testing.T) {
	t.Run("generic failure maps to Failed", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-1", "default")
		setCondition(obj, "False", "ReconciliationFailed", "apply failed", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusFailed))
		g.Expect(rs.Message).To(Equal("apply failed"))
	})

	t.Run("DependencyNotReady maps to Progressing", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-2", "default")
		setCondition(obj, "False", "DependencyNotReady", "dependency 'default/dep' is not ready", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusProgressing))
		g.Expect(rs.Message).To(ContainSubstring("dependency"))
	})

	t.Run("no reason field maps to Failed", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-3", "default")
		setConditionNoReason(obj, "False", "something went wrong", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusFailed))
	})
}

func TestNewResourceStatus_ReadyUnknown(t *testing.T) {
	t.Run("reason Progressing maps to Progressing", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-1", "default")
		setCondition(obj, "Unknown", "Progressing", "reconciliation in progress", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusProgressing))
	})

	t.Run("reason Reconciling maps to Progressing", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("HelmRelease", "nginx", "default")
		setCondition(obj, "Unknown", "Reconciling", "upgrading release", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusProgressing))
	})

	t.Run("other reason maps to Unknown", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-2", "default")
		setCondition(obj, "Unknown", "SomethingElse", "unusual state", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusUnknown))
	})

	t.Run("no reason field maps to Progressing", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-3", "default")
		setConditionNoReason(obj, "Unknown", "working on it", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusProgressing))
	})
}

func TestNewResourceStatus_UnexpectedConditionStatus(t *testing.T) {
	g := NewWithT(t)

	obj := makeObj("Kustomization", "app-1", "default")
	setCondition(obj, "Bogus", "Whatever", "unexpected status", "2025-01-01T00:00:00Z")

	rs := NewResourceStatus(obj)
	g.Expect(rs.Status).To(Equal(StatusUnknown))
}

func TestNewResourceStatus_NoConditions(t *testing.T) {
	g := NewWithT(t)

	obj := makeObj("Kustomization", "app-1", "default")
	// No conditions set.

	rs := NewResourceStatus(obj)
	g.Expect(rs.Status).To(Equal(StatusUnknown))
	g.Expect(rs.Message).To(Equal("No status information available"))
}

func TestNewResourceStatus_NoReadyCondition(t *testing.T) {
	g := NewWithT(t)

	// Object has conditions but none of type "Ready".
	obj := makeObj("Kustomization", "app-1", "default")
	setNestedSlice(obj, []any{
		map[string]any{
			"type":    "Healthy",
			"status":  "True",
			"reason":  "HealthCheckSucceeded",
			"message": "all checks passed",
		},
	}, "status", "conditions")

	rs := NewResourceStatus(obj)
	g.Expect(rs.Status).To(Equal(StatusUnknown))
	g.Expect(rs.Message).To(Equal("No status information available"))
}

func TestNewResourceStatus_MultipleConditions(t *testing.T) {
	g := NewWithT(t)

	obj := makeObj("Kustomization", "app-1", "default")
	setNestedSlice(obj, []any{
		map[string]any{
			"type":    "Healthy",
			"status":  "False",
			"reason":  "HealthCheckFailed",
			"message": "health check failed",
		},
		map[string]any{
			"type":               "Ready",
			"status":             "True",
			"reason":             "ReconciliationSucceeded",
			"message":            "applied successfully",
			"lastTransitionTime": "2025-03-01T12:00:00Z",
		},
	}, "status", "conditions")

	rs := NewResourceStatus(obj)
	g.Expect(rs.Status).To(Equal(StatusReady))
	g.Expect(rs.Message).To(Equal("applied successfully"))
}

func TestNewResourceStatus_Message(t *testing.T) {
	t.Run("empty message keeps default", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-1", "default")
		setNestedSlice(obj, []any{
			map[string]any{
				"type":    "Ready",
				"status":  "True",
				"reason":  "Succeeded",
				"message": "",
			},
		}, "status", "conditions")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusReady))
		g.Expect(rs.Message).To(Equal("No status information available"))
	})

	t.Run("missing message field keeps default", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-1", "default")
		setNestedSlice(obj, []any{
			map[string]any{
				"type":   "Ready",
				"status": "True",
				"reason": "Succeeded",
			},
		}, "status", "conditions")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusReady))
		g.Expect(rs.Message).To(Equal("No status information available"))
	})
}

func TestNewResourceStatus_LastReconciled(t *testing.T) {
	t.Run("parsed from lastTransitionTime", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-1", "default")
		setCondition(obj, "True", "Succeeded", "ok", "2025-09-15T14:30:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.LastReconciled.UTC().Format("2006-01-02T15:04:05Z")).To(Equal("2025-09-15T14:30:00Z"))
	})

	t.Run("falls back to creationTimestamp when lastTransitionTime missing", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObjWithCreation("Kustomization", "app-1", "default", "2025-01-10T08:00:00Z")
		setNestedSlice(obj, []any{
			map[string]any{
				"type":   "Ready",
				"status": "True",
				"reason": "Succeeded",
			},
		}, "status", "conditions")

		rs := NewResourceStatus(obj)
		g.Expect(rs.LastReconciled.UTC().Format("2006-01-02")).To(Equal("2025-01-10"))
	})

	t.Run("falls back to creationTimestamp on invalid lastTransitionTime", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObjWithCreation("Kustomization", "app-1", "default", "2025-02-20T00:00:00Z")
		setNestedSlice(obj, []any{
			map[string]any{
				"type":               "Ready",
				"status":             "True",
				"reason":             "Succeeded",
				"lastTransitionTime": "not-a-valid-time",
			},
		}, "status", "conditions")

		rs := NewResourceStatus(obj)
		g.Expect(rs.LastReconciled.UTC().Format("2006-01-02")).To(Equal("2025-02-20"))
	})

	t.Run("falls back to creationTimestamp when no conditions", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObjWithCreation("Kustomization", "app-1", "default", "2025-05-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.LastReconciled.UTC().Format("2006-01-02")).To(Equal("2025-05-01"))
	})
}

func TestNewResourceStatus_AlertKind(t *testing.T) {
	t.Run("Alert without conditions is Ready", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxAlertKind, "slack-alert", "flux-system")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusReady))
		g.Expect(rs.Message).To(Equal("Valid configuration"))
	})

	t.Run("Alert with Ready=True stays Ready", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxAlertKind, "slack-alert", "flux-system")
		setCondition(obj, "True", "Succeeded", "alert configured", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusReady))
		g.Expect(rs.Message).To(Equal("alert configured"))
	})

	t.Run("Alert with Ready=False is Failed not overridden", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxAlertKind, "broken-alert", "flux-system")
		setCondition(obj, "False", "ValidationFailed", "invalid config", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusFailed), "condition status should take precedence over Alert override")
	})
}

func TestNewResourceStatus_ProviderKind(t *testing.T) {
	t.Run("Provider without conditions is Ready", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxAlertProviderKind, "slack", "flux-system")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusReady))
		g.Expect(rs.Message).To(Equal("Valid configuration"))
	})

	t.Run("Provider with Ready=False is Failed not overridden", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxAlertProviderKind, "broken", "flux-system")
		setCondition(obj, "False", "ValidationFailed", "bad creds", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusFailed))
	})
}

func TestNewResourceStatus_HelmRepositoryOCI(t *testing.T) {
	t.Run("OCI HelmRepository without conditions is Ready", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxHelmRepositoryKind, "ghcr", "flux-system")
		setNestedString(obj, "oci", "spec", "type")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusReady))
		g.Expect(rs.Message).To(Equal("Valid configuration"))
	})

	t.Run("OCI HelmRepository with Ready=True stays Ready", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxHelmRepositoryKind, "ghcr", "flux-system")
		setNestedString(obj, "oci", "spec", "type")
		setCondition(obj, "True", "Succeeded", "chart pulled", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusReady))
		g.Expect(rs.Message).To(Equal("chart pulled"))
	})

	t.Run("OCI HelmRepository with Ready=False is Failed not overridden", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxHelmRepositoryKind, "bad-oci", "flux-system")
		setNestedString(obj, "oci", "spec", "type")
		setCondition(obj, "False", "FetchFailed", "auth error", "2025-01-01T00:00:00Z")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusFailed), "condition should take precedence over OCI override")
	})

	t.Run("non-OCI HelmRepository without conditions stays Unknown", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxHelmRepositoryKind, "bitnami", "flux-system")
		setNestedString(obj, "default", "spec", "type")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusUnknown))
	})

	t.Run("HelmRepository without spec.type stays Unknown", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxHelmRepositoryKind, "bitnami", "flux-system")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusUnknown))
	})
}

func TestNewResourceStatus_SuspendedViaSpecField(t *testing.T) {
	t.Run("spec.suspend=true overrides Ready status", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-1", "default")
		setCondition(obj, "True", "Succeeded", "applied", "2025-01-01T00:00:00Z")
		setNestedBool(obj, true, "spec", "suspend")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusSuspended))
		g.Expect(rs.Message).To(Equal("Reconciliation suspended"))
	})

	t.Run("spec.suspend=true overrides Failed status", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-1", "default")
		setCondition(obj, "False", "ReconciliationFailed", "error", "2025-01-01T00:00:00Z")
		setNestedBool(obj, true, "spec", "suspend")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusSuspended))
	})

	t.Run("spec.suspend=false does not suspend", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-1", "default")
		setCondition(obj, "True", "Succeeded", "applied", "2025-01-01T00:00:00Z")
		setNestedBool(obj, false, "spec", "suspend")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusReady))
	})
}

func TestNewResourceStatus_SuspendedViaAnnotation(t *testing.T) {
	t.Run("reconcile=disabled annotation overrides Ready status", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.FluxInstanceKind, "flux", "flux-system")
		setCondition(obj, "True", "Succeeded", "installed", "2025-01-01T00:00:00Z")
		setAnnotation(obj, fluxcdv1.ReconcileAnnotation, fluxcdv1.DisabledValue)

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusSuspended))
		g.Expect(rs.Message).To(Equal("Reconciliation suspended"))
	})

	t.Run("reconcile=disabled annotation overrides Failed status", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj(fluxcdv1.ResourceSetKind, "rs-1", "default")
		setCondition(obj, "False", "ReconciliationFailed", "error", "2025-01-01T00:00:00Z")
		setAnnotation(obj, fluxcdv1.ReconcileAnnotation, fluxcdv1.DisabledValue)

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusSuspended))
	})

	t.Run("unrelated annotation does not suspend", func(t *testing.T) {
		g := NewWithT(t)
		obj := makeObj("Kustomization", "app-1", "default")
		setCondition(obj, "True", "Succeeded", "applied", "2025-01-01T00:00:00Z")
		setAnnotation(obj, "some.other/annotation", "true")

		rs := NewResourceStatus(obj)
		g.Expect(rs.Status).To(Equal(StatusReady))
	})
}

func TestNewResourceStatus_BothSuspendMechanisms(t *testing.T) {
	g := NewWithT(t)

	// Both spec.suspend and annotation set — should still be Suspended.
	obj := makeObj("Kustomization", "app-1", "default")
	setCondition(obj, "True", "Succeeded", "applied", "2025-01-01T00:00:00Z")
	setAnnotation(obj, fluxcdv1.ReconcileAnnotation, fluxcdv1.DisabledValue)
	setNestedBool(obj, true, "spec", "suspend")

	rs := NewResourceStatus(obj)
	g.Expect(rs.Status).To(Equal(StatusSuspended))
}

func TestNewResourceStatus_MetadataExtraction(t *testing.T) {
	g := NewWithT(t)

	obj := makeObj("HelmRelease", "my-app", "production")
	setCondition(obj, "True", "Succeeded", "upgrade complete", "2025-07-01T00:00:00Z")

	rs := NewResourceStatus(obj)
	g.Expect(rs.Kind).To(Equal("HelmRelease"))
	g.Expect(rs.Name).To(Equal("my-app"))
	g.Expect(rs.Namespace).To(Equal("production"))
}

func TestNewResourceStatus_ConditionStatusFieldMissing(t *testing.T) {
	g := NewWithT(t)

	// Ready condition exists but has no "status" key — should stay Unknown.
	obj := makeObj("Kustomization", "app-1", "default")
	setNestedSlice(obj, []any{
		map[string]any{
			"type":    "Ready",
			"reason":  "Succeeded",
			"message": "some message",
		},
	}, "status", "conditions")

	rs := NewResourceStatus(obj)
	g.Expect(rs.Status).To(Equal(StatusUnknown))
	// Message should still be extracted.
	g.Expect(rs.Message).To(Equal("some message"))
}

func TestNewResourceStatus_EmptyObject(t *testing.T) {
	g := NewWithT(t)

	obj := unstructured.Unstructured{Object: map[string]any{}}

	rs := NewResourceStatus(obj)
	g.Expect(rs.Status).To(Equal(StatusUnknown))
	g.Expect(rs.Message).To(Equal("No status information available"))
	g.Expect(rs.Name).To(BeEmpty())
	g.Expect(rs.Kind).To(BeEmpty())
	g.Expect(rs.Namespace).To(BeEmpty())
	g.Expect(rs.LastReconciled.IsZero()).To(BeTrue())
}

// --- test helpers ---

// makeObj creates a minimal unstructured object.
func makeObj(kind, name, namespace string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "test/v1",
			"kind":       kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

// makeObjWithCreation creates an unstructured object with a creationTimestamp.
func makeObjWithCreation(kind, name, namespace, creationTime string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "test/v1",
			"kind":       kind,
			"metadata": map[string]any{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": creationTime,
			},
		},
	}
}

// setCondition sets a single Ready condition with all standard fields.
func setCondition(obj unstructured.Unstructured, status, reason, message, lastTransitionTime string) {
	setNestedSlice(obj, []any{
		map[string]any{
			"type":               "Ready",
			"status":             status,
			"reason":             reason,
			"message":            message,
			"lastTransitionTime": lastTransitionTime,
		},
	}, "status", "conditions")
}

// setConditionNoReason sets a Ready condition without the reason field.
func setConditionNoReason(obj unstructured.Unstructured, status, message, lastTransitionTime string) {
	setNestedSlice(obj, []any{
		map[string]any{
			"type":               "Ready",
			"status":             status,
			"message":            message,
			"lastTransitionTime": lastTransitionTime,
		},
	}, "status", "conditions")
}

func setNestedSlice(obj unstructured.Unstructured, value []any, fields ...string) {
	_ = unstructured.SetNestedSlice(obj.Object, value, fields...)
}

func setNestedString(obj unstructured.Unstructured, value string, fields ...string) {
	_ = unstructured.SetNestedField(obj.Object, value, fields...)
}

func setNestedBool(obj unstructured.Unstructured, value bool, fields ...string) {
	_ = unstructured.SetNestedField(obj.Object, value, fields...)
}

func setAnnotation(obj unstructured.Unstructured, key, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	obj.SetAnnotations(annotations)
}
