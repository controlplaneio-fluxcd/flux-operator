// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/schedule"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/testutils"
)

func TestResourceSetInputProviderReconciler_Static(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-static
  namespace: "%[1]s"
spec:
  type: Static
  url: https://gitlab.com/stefanprodan/podinfo
  defaultValues:
    env: "staging"
    foo: "bar"
`, ns.Name)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Create the ResourceSetInputProvider. Should error out due to CEL validation.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.url must be empty when spec.type is 'Static'"))

	// Invert CEL validation error to type not Static and URL empty.
	obj.Spec.Type = fluxcdv1.InputProviderGitHubBranch
	obj.Spec.URL = ""
	err = testEnv.Create(ctx, obj)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.url must not be empty when spec.type is not 'Static'"))

	// Fix object and create.
	obj.Spec.Type = fluxcdv1.InputProviderStatic
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())
	exportedInput := fmt.Sprintf(`
id: "%[1]s"
env: staging
foo: bar `, inputs.Checksum(string(obj.UID)))

	// Initialize the ResourceSetInputProvider.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile and verify exported inputs.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(1))
	b, err := yaml.Marshal(result.Status.ExportedInputs[0])
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(MatchYAML(exportedInput))
	g.Expect(result.Status.LastExportedRevision).To(HavePrefix("sha256:"))
	g.Expect(result.Status.LastExportedRevision).To(HaveLen(71))
	lastExportedRevision := result.Status.LastExportedRevision

	// Reconcile again and verify that revision did not change.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
	result = &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(1))
	b, err = yaml.Marshal(result.Status.ExportedInputs[0])
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(MatchYAML(exportedInput))
	g.Expect(result.Status.LastExportedRevision).To(Equal(lastExportedRevision))
}

func TestResourceSetInputProviderReconciler_MultipleOrderingOptions(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-multiple-ordering-options")
	g.Expect(err).ToNot(HaveOccurred())

	for _, tt := range []struct {
		name   string
		filter fluxcdv1.ResourceSetInputFilter
	}{
		{
			name: "semver and alphabetical",
			filter: fluxcdv1.ResourceSetInputFilter{
				Semver:       ">=1.0.0",
				Alphabetical: "asc",
			},
		},
		{
			name: "alphabetical and numerical",
			filter: fluxcdv1.ResourceSetInputFilter{
				Alphabetical: "desc",
				Numerical:    "asc",
			},
		},
		{
			name: "numerical and semver",
			filter: fluxcdv1.ResourceSetInputFilter{
				Numerical: "asc",
				Semver:    ">=1.0.0",
			},
		},
		{
			name: "all three",
			filter: fluxcdv1.ResourceSetInputFilter{
				Semver:       ">=1.0.0",
				Alphabetical: "asc",
				Numerical:    "desc",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-multiple-ordering-options",
					Namespace: ns.Name,
				},
				Spec: fluxcdv1.ResourceSetInputProviderSpec{
					Type:   fluxcdv1.InputProviderStatic,
					Filter: &tt.filter,
				},
			}

			err := testEnv.Create(ctx, obj)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("cannot specify more than one of semver, alphabetical or numerical"))
		})
	}
}

func TestResourceSetInputProviderReconciler_InvalidDefaultValues(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			DefaultValues: fluxcdv1.ResourceSetInput{
				"foo": &apix.JSON{
					Raw: []byte(`{"bar": "baz"`),
				},
			},
		},
	}

	r, err := reconciler.reconcile(ctx, obj, nil)
	g.Expect(r).To(Equal(reconcile.Result{}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj.Status.Conditions).To(HaveLen(2))
	g.Expect(conditions.IsReady(obj)).To(BeFalse())
	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.GetReason(obj, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonInvalidDefaultValues))
	g.Expect(conditions.GetReason(obj, meta.StalledCondition)).To(Equal(fluxcdv1.ReasonInvalidDefaultValues))
	g.Expect(conditions.GetMessage(obj, meta.ReadyCondition)).To(ContainSubstring(msgTerminalError))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring(msgTerminalError))
}

func TestResourceSetInputProviderReconciler_reconcile_InvalidSchedule(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Schedule: []fluxcdv1.Schedule{{
				Cron: "lalksadlsakd",
			}},
		},
	}

	r, err := reconciler.reconcile(ctx, obj, nil)
	g.Expect(r).To(Equal(reconcile.Result{}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj.Status.Conditions).To(HaveLen(2))
	g.Expect(conditions.IsReady(obj)).To(BeFalse())
	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.GetReason(obj, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonInvalidSchedule))
	g.Expect(conditions.GetReason(obj, meta.StalledCondition)).To(Equal(fluxcdv1.ReasonInvalidSchedule))
	g.Expect(conditions.GetMessage(obj, meta.ReadyCondition)).To(ContainSubstring(msgTerminalError))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring(msgTerminalError))
}

func TestResourceSetInputProviderReconciler_SkippedDueToSchedule(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-skipped-due-to-schedule")
	g.Expect(err).NotTo(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-schedule
  namespace: "%[1]s"
  annotations:
    fluxcd.controlplane.io/reconcileTimeout: 100ms
spec:
  type: Static
  schedule:
  - cron: "0 0 29 2 *" # This cron only happens once every 4 years.
    window: 1s
  defaultValues:
    env: test
`, ns.Name)

	// Create the ResourceSetInputProvider.
	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSetInputProvider.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile and verify schedule.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())

	sched, err := schedule.Parse("0 0 29 2 *", "UTC")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sched).NotTo(BeNil())

	// Verify that the next schedule is set correctly.
	expectedRequeueAfter := time.Until(sched.Next(time.Now()))
	g.Expect(r.RequeueAfter).To(BeNumerically("~", expectedRequeueAfter, time.Second))

	testutils.LogObjectStatus(t, result)

	// Verify that the status contains the next schedule.
	g.Expect(result.Status.NextSchedule).NotTo(BeNil())
	g.Expect(result.Status.NextSchedule.Schedule).To(Equal(fluxcdv1.Schedule{
		Cron:     "0 0 29 2 *",
		TimeZone: "UTC",
		Window:   metav1.Duration{Duration: time.Second},
	}))

	// Verify that the status contains the next schedule time.
	untilWhen := time.Until(result.Status.NextSchedule.When.Time)
	g.Expect(untilWhen).To(BeNumerically("~", expectedRequeueAfter, time.Second))

	// Verify that the status ready condition reason is set to SkippedDueToSchedule.
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonSkippedDueToSchedule))

	// Verify skipped reconciliation event.
	events := getEvents(obj.Name, obj.Namespace)
	g.Expect(events).To(HaveLen(1))
	g.Expect(events[0].Reason).To(Equal(fluxcdv1.ReasonSkippedDueToSchedule))
	g.Expect(events[0].Message).To(ContainSubstring("Reconciliation skipped, next scheduled at"))
}

func TestRequeueAfterResourceSetInputProvider(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		interval             time.Duration
		schedules            []fluxcdv1.Schedule
		timeout              time.Duration
		reconcileEnd         string
		expectedRequeueAfter time.Duration
		expectedNextSchedule *fluxcdv1.NextSchedule
	}{
		{
			name:                 "disabled",
			interval:             0,
			reconcileEnd:         "2023-10-01T00:00:00Z",
			expectedRequeueAfter: 0,
		},
		{
			name:                 "no schedule",
			interval:             time.Hour,
			reconcileEnd:         "2023-10-01T00:00:00Z",
			expectedRequeueAfter: time.Hour,
		},
		{
			name:     "schedule without window",
			interval: time.Hour,
			schedules: []fluxcdv1.Schedule{{
				Cron: "0 9 * * *",
			}},
			timeout:              5 * time.Minute,
			reconcileEnd:         "2023-10-01T10:00:00Z",
			expectedRequeueAfter: 23 * time.Hour,
			expectedNextSchedule: &fluxcdv1.NextSchedule{
				Schedule: fluxcdv1.Schedule{
					Cron: "0 9 * * *",
				},
				When: metav1.Time{Time: time.Date(2023, 10, 2, 9, 0, 0, 0, time.UTC)},
			},
		},
		{
			name:     "next interval is within the window",
			interval: time.Hour,
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 9 * * *",
				Window: metav1.Duration{Duration: 8 * time.Hour},
			}},
			timeout:              5 * time.Minute,
			reconcileEnd:         "2023-10-01T10:00:00Z",
			expectedRequeueAfter: time.Hour,
		},
		{
			name:     "next interval is outside the window",
			interval: time.Hour,
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 9 * * *",
				Window: metav1.Duration{Duration: 8 * time.Hour},
			}},
			timeout:              5 * time.Minute,
			reconcileEnd:         "2023-10-01T18:00:00Z",
			expectedRequeueAfter: 15 * time.Hour,
			expectedNextSchedule: &fluxcdv1.NextSchedule{
				Schedule: fluxcdv1.Schedule{
					Cron:   "0 9 * * *",
					Window: metav1.Duration{Duration: 8 * time.Hour},
				},
				When: metav1.Time{Time: time.Date(2023, 10, 2, 9, 0, 0, 0, time.UTC)},
			},
		},
		{
			name:     "next interval is later than next schedule",
			interval: time.Hour,
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 9 * * *",
				Window: metav1.Duration{Duration: 8 * time.Hour},
			}},
			timeout:              5 * time.Minute,
			reconcileEnd:         "2023-10-01T08:30:00Z",
			expectedRequeueAfter: 30 * time.Minute,
			expectedNextSchedule: &fluxcdv1.NextSchedule{
				Schedule: fluxcdv1.Schedule{
					Cron:   "0 9 * * *",
					Window: metav1.Duration{Duration: 8 * time.Hour},
				},
				When: metav1.Time{Time: time.Date(2023, 10, 1, 9, 0, 0, 0, time.UTC)},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						fluxcdv1.ReconcileEveryAnnotation: tt.interval.String(),
					},
				},
			}

			scheduler, err := schedule.NewScheduler(tt.schedules, tt.timeout)
			g.Expect(err).NotTo(HaveOccurred())

			reconcileEnd := testutils.ParseTime(t, tt.reconcileEnd)

			res := requeueAfterResourceSetInputProvider(obj, scheduler, reconcileEnd)
			g.Expect(res.RequeueAfter).To(Equal(tt.expectedRequeueAfter))
			g.Expect(obj.Status.NextSchedule).To(Equal(tt.expectedNextSchedule))
		})
	}
}

// getResourceSetInputProviderReconciler returns a new ResourceSetInputProviderReconciler
// configured for testing purposes, with notifications disabled and a test event recorder.
func getResourceSetInputProviderReconciler(t *testing.T) *ResourceSetInputProviderReconciler {
	// Disable notifications for the tests as no pod is running.
	// This is required to avoid the 30s retry loop performed by the HTTP client.
	t.Setenv("NOTIFICATIONS_DISABLED", "yes")
	return &ResourceSetInputProviderReconciler{
		Client:        testClient,
		Scheme:        NewTestScheme(),
		StatusManager: controllerName,
		EventRecorder: testEnv.GetEventRecorderFor(controllerName),
	}
}
