// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/git/github"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	reconciler := getResourceSetInputProviderReconciler()
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

func TestResourceSetInputProviderReconciler_reconcile_InvalidDefaultValues(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler()
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
	g.Expect(conditions.GetMessage(obj, meta.ReadyCondition)).To(ContainSubstring("Reconciliation failed terminally due to configuration error"))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring("Reconciliation failed terminally due to configuration error"))
}

func TestResourceSetInputProviderReconciler_reconcile_InvalidSchedule(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			DefaultValues: fluxcdv1.ResourceSetInput{
				"foo": &apix.JSON{
					Raw: []byte(`{"bar": "baz"}`),
				},
			},
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
	g.Expect(conditions.GetMessage(obj, meta.ReadyCondition)).To(ContainSubstring("Reconciliation failed terminally due to configuration error"))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring("Reconciliation failed terminally due to configuration error"))
}

func TestResourceSetInputProviderReconciler_reconcile_SkippedDueToSchedule(t *testing.T) {
	// Disable notifications for the tests as no pod is running.
	// This is required to avoid the 30s retry loop performed by the HTTP client.
	t.Setenv("NOTIFICATIONS_DISABLED", "yes")

	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler()
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

	// Create the ResourceSetInputProvide
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

	// Reconcile and verify schedule
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())

	logObjectStatus(t, result)
	g.Expect(result.Status.NextSchedule).NotTo(BeNil())
	g.Expect(result.Status.NextSchedule.Schedule).To(Equal(fluxcdv1.Schedule{
		Cron:     "0 0 29 2 *",
		TimeZone: "UTC",
		Window:   metav1.Duration{Duration: time.Second},
	}))

	sched, err := schedule.Parse("0 0 29 2 *", "UTC")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sched).NotTo(BeNil())

	expectedRequeueAfter := time.Until(sched.Next(time.Now()))
	g.Expect(r.RequeueAfter).To(BeNumerically("~", expectedRequeueAfter, time.Second))

	untilWhen := time.Until(result.Status.NextSchedule.When.Time)
	g.Expect(untilWhen).To(BeNumerically("~", expectedRequeueAfter, time.Second))

	g.Eventually(func() bool {
		events := getEvents(obj.Name, obj.Namespace)
		if len(events) == 0 {
			return false
		}
		return events[0].Reason == fluxcdv1.ReasonSkippedDueToSchedule &&
			strings.Contains(events[0].Message, "Reconciliation skipped, next scheduled at")
	}, timeout).Should(BeTrue())
}

func TestResourceSetInputProviderReconciler_GitLabBranch_LifeCycle(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler()
	rsetReconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test
  namespace: "%[1]s"
spec:
  type: GitLabBranch
  url: "https://gitlab.com/stefanprodan/podinfo"
  filter:
    includeBranch: "^patch-[1|2]$"
`, ns.Name)

	setDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: test
  namespace: "%[1]s"
spec:
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: test
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: test-<< inputs.id >>
        namespace: "%[1]s"
      data:
        branch: << inputs.branch | quote >>
        sha: << inputs.sha | quote >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Create the ResourceSetInputProvider.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSetInputProvider.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Retrieve the inputs.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	// Check if the ResourceSetInputProvider was marked as ready.
	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	logObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))
	g.Expect(result.Status.LastExportedRevision).To(BeIdenticalTo("sha256:be31afc5e49da21b12fdca6a2cad6916cad26f4bbde8c16e5822359f75c1d46a"))

	// Create a ResourceSet referencing the ResourceSetInputProvider.
	rset := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(setDef), rset)
	g.Expect(err).ToNot(HaveOccurred())
	err = testEnv.Create(ctx, rset)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSet instance.
	_, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile the ResourceSet instance.
	_, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the ResourceSet generated the resources.
	result1CM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-183501423",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(result1CM), result1CM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result1CM.Data).To(HaveKeyWithValue("branch", "patch-1"))
	g.Expect(result1CM.Data).To(HaveKeyWithValue("sha", "cebef2d870bc83b37f43c470bae205fca094bacc"))

	result2CM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-183566960",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(result2CM), result2CM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result2CM.Data).To(HaveKeyWithValue("branch", "patch-2"))
	g.Expect(result2CM.Data).To(HaveKeyWithValue("sha", "a275fb0322466eaa1a74485a4f79f88d7c8858e8"))

	// Update the filter to exclude all results.
	resultP := result.DeepCopy()
	resultP.Spec.Filter.ExcludeBranch = "^patch-.*$"
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the exported inputs were updated.
	resultFinal := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultFinal.Status.ExportedInputs).To(BeEmpty())
	g.Expect(resultFinal.Status.LastExportedRevision).To(BeIdenticalTo("sha256:38e0b9de817f645c4bec37c0d4a3e58baecccb040f5718dc069a72c7385a0bed"))

	// Reconcile the ResourceSet to remove the generated resources.
	_, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the generated resources were removed.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(result1CM), result1CM)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	err = testClient.Get(ctx, client.ObjectKeyFromObject(result2CM), result2CM)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Delete the ResourceSetInputProvider.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the ResourceSetInputProvider was finalized.
	result = &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Delete the ResourceSet.
	err = testClient.Delete(ctx, rset)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())
}

func TestResourceSetInputProviderReconciler_GitHubPullRequest_LifeCycle(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler()
	rsetReconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test
  namespace: "%[1]s"
spec:
  type: GitHubPullRequest
  url: "https://github.com/fluxcd-testing/pr-testing"
  defaultValues:
    env: "staging"
  filter:
    limit: 2
    includeBranch: "^stefanprodan-patch-.*$"
    labels:
      - "enhancement"
`, ns.Name)

	exportedInputs := `
- author: stefanprodan
  branch: stefanprodan-patch-4
  env: staging
  id: "4"
  sha: 80332195632fe293564ff563344032cf4c75af45
  title: 'test4: Update README.md'
  labels:
  - documentation
  - enhancement
- author: stefanprodan
  branch: stefanprodan-patch-2
  env: staging
  id: "2"
  sha: 1e5aef14d38a8c67e5240308adf2935d6cdc2ec8
  title: 'test2: Update README.md'
  labels:
  - enhancement
`

	setDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: test
  namespace: "%[1]s"
spec:
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: test
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: test-<< inputs.id >>
        namespace: "%[1]s"
      data:
        branch: << inputs.branch | quote >>
        sha: << inputs.sha | quote >>
        title: << inputs.title | quote >>
        author: << inputs.author | quote >>
        env: << inputs.env | quote >>
        labels: << inputs.labels | join "," >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSetInputProvider.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Check if the finalizer was added.
	resultInit := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultInit)
	g.Expect(err).ToNot(HaveOccurred())

	logObjectStatus(t, resultInit)
	g.Expect(resultInit.Finalizers).To(ContainElement(fluxcdv1.Finalizer))

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	// Check if the ResourceSetInputProvider was marked as ready.
	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	logObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Check if the exported inputs are correct.
	inputsData, err := yaml.Marshal(result.Status.ExportedInputs)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(inputsData)).To(MatchYAML(exportedInputs))

	// Create a ResourceSet referencing the ResourceSetInputProvider.
	rset := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(setDef), rset)
	g.Expect(err).ToNot(HaveOccurred())
	err = testEnv.Create(ctx, rset)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSet instance.
	_, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile the ResourceSet instance.
	_, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the ResourceSet generated the resources.
	resultCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-2",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultCM), resultCM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultCM.Data).To(HaveKeyWithValue("branch", "stefanprodan-patch-2"))
	g.Expect(resultCM.Data).To(HaveKeyWithValue("sha", "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8"))
	g.Expect(resultCM.Data).To(HaveKeyWithValue("title", "test2: Update README.md"))
	g.Expect(resultCM.Data).To(HaveKeyWithValue("author", "stefanprodan"))
	g.Expect(resultCM.Data).To(HaveKeyWithValue("env", "staging"))
	g.Expect(resultCM.Data).To(HaveKeyWithValue("labels", "enhancement"))

	// Update the filter to exclude all results.
	resultP := result.DeepCopy()
	resultP.Spec.Filter.ExcludeBranch = "^stefanprodan-.*$"
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the exported inputs were updated.
	resultFinal := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultFinal.Status.ExportedInputs).To(BeEmpty())

	// Reconcile the ResourceSet to remove the generated resources.
	_, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the generated resources were removed.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultCM), resultCM)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Delete the ResourceSetInputProvider.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the ResourceSetInputProvider was finalized.
	result = &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Reconcile the ResourceSet and expect a provider not found error.
	_, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Delete the ResourceSet.
	err = testClient.Delete(ctx, rset)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = rsetReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(rset),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())
}

func TestResourceSetInputProviderReconciler_FailureRecovery(t *testing.T) {
	// Disable notifications for the tests as no pod is running.
	// This is required to avoid the 30s retry loop performed by the HTTP client.
	t.Setenv("NOTIFICATIONS_DISABLED", "yes")

	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-failure-recovery
  namespace: "%[1]s"
spec:
  type: GitLabBranch
  url: "https://gitlab.com/stefanprodan/podinfo-not-found"
  filter:
    includeBranch: "^patch-[1|2]$"
`, ns.Name)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Create the ResourceSetInputProvider.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSetInputProvider.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Try to reconcile the inputs with upstream.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())

	// Check if the ResourceSetInputProvider was marked as failed.
	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeFalse())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationFailedReason))
	g.Expect(conditions.GetMessage(result, meta.ReadyCondition)).To(ContainSubstring("404 Not Found"))
	g.Expect(result.Status.ExportedInputs).To(BeEmpty())

	// Check if the failure event was recorded.
	events := getEvents(result.Name, result.Namespace)
	g.Expect(events[0].Reason).To(Equal(meta.ReconciliationFailedReason))
	g.Expect(events[0].Message).To(ContainSubstring("failed to list branches"))

	// Update the URL to a valid repository.
	resultP := result.DeepCopy()
	resultP.Spec.URL = "https://gitlab.com/stefanprodan/podinfo"
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile the inputs with upstream.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the exported inputs were updated and marked as ready.
	resultFinal := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.IsReady(resultFinal)).To(BeTrue())
	g.Expect(resultFinal.Status.ExportedInputs).ToNot(BeEmpty())
	g.Expect(resultFinal.Status.LastExportedRevision).To(BeIdenticalTo("sha256:be31afc5e49da21b12fdca6a2cad6916cad26f4bbde8c16e5822359f75c1d46a"))

	// Delete the ResourceSetInputProvider.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the ResourceSetInputProvider was finalized.
	result = &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestResourceSetInputProviderReconciler_getGitHubToken_cached(t *testing.T) {
	const key = "dd2ce27f135e666c946a3bd4657f4ffaf1d2c97d9a35b93336f467dcdd93a56b"

	g := NewWithT(t)

	ctx := context.Background()

	tokenCache, err := cache.NewTokenCache(1)
	g.Expect(err).NotTo(HaveOccurred())

	r := &ResourceSetInputProviderReconciler{
		TokenCache: tokenCache,
	}

	_, _, err = r.TokenCache.GetOrSet(ctx, key, func(context.Context) (cache.Token, error) {
		return &github.AppToken{
			Token:     "my-gh-app-token",
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil
	})
	g.Expect(err).NotTo(HaveOccurred())

	privateKeyPEM, err := os.ReadFile("testdata/rsa-private-key.pem")
	g.Expect(err).NotTo(HaveOccurred())

	token, err := r.getGitHubToken(ctx, &fluxcdv1.ResourceSetInputProvider{}, map[string][]byte{
		"githubAppID":             []byte("123"),
		"githubAppInstallationID": []byte("123456"),
		"githubAppBaseURL":        []byte("https://github.com"),
		"githubAppPrivateKey":     privateKeyPEM,
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal("my-gh-app-token"))
}

func getResourceSetInputProviderReconciler() *ResourceSetInputProviderReconciler {
	return &ResourceSetInputProviderReconciler{
		Client:        testClient,
		Scheme:        NewTestScheme(),
		StatusManager: controllerName,
		EventRecorder: testEnv.GetEventRecorderFor(controllerName),
	}
}

func TestResourceSetInputProviderReconciler_SkipExportedInputsUpdate_LifeCycle(t *testing.T) {
	defaultStatus := `
conditions:
- lastTransitionTime: "2025-03-28T09:53:36Z"
  message: Reconciliation finished in 331ms
  observedGeneration: 1
  reason: ReconciliationSucceeded
  status: "True"
  type: Ready
exportedInputs:
- author: stefanprodan
  branch: stefanprodan-patch-4
  env: staging
  id: "4"
  sha: 342db2d64746fedf3a8768d351621b7fda2362f3
  title: 'test4: Update README.md'
  labels:
  - documentation
  - enhancement
  - test
- author: stefanprodan
  branch: stefanprodan-patch-2
  env: staging
  id: "2"
  sha: 381635adf7bfa06e48cb958531cc1d44e03a744c
  title: 'test2: Update README.md'
  labels:
  - enhancement
`
	objDef := `
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test
  namespace: %[1]s
spec:
  type: GitHubPullRequest
  url: "https://github.com/fluxcd-testing/pr-testing"
  defaultValues:
    env: "staging"
  filter:
    includeBranch: "^stefanprodan-patch-.*$"
    labels:
    - "enhancement"
  skip:
    labels:
    %[2]s
`

	tests := []struct {
		name           string
		skipDef        string
		statusDef      string // fake status to simulate previous exported inputs
		expectedInputs string
	}{
		{
			name:      "Skip label",
			skipDef:   `- "documentation"`,
			statusDef: defaultStatus,
			expectedInputs: `
- author: stefanprodan
  branch: stefanprodan-patch-4
  env: staging
  id: "4"
  sha: 342db2d64746fedf3a8768d351621b7fda2362f3
  title: 'test4: Update README.md'
  labels:
  - documentation
  - enhancement
  - test
- author: stefanprodan
  branch: stefanprodan-patch-2
  env: staging
  id: "2"
  sha: 1e5aef14d38a8c67e5240308adf2935d6cdc2ec8
  title: 'test2: Update README.md'
  labels:
  - enhancement
- author: stefanprodan
  branch: stefanprodan-patch-1
  env: staging
  id: "1"
  sha: 2dd3a8d2088457e5cf991018edf13e25cbd61380
  title: 'test1: Update README.md'
  labels:
  - enhancement
`,
		},
		{
			name:      "Skip label with reverse",
			skipDef:   `- "!documentation"`,
			statusDef: defaultStatus,
			expectedInputs: `
- author: stefanprodan
  branch: stefanprodan-patch-4
  env: staging
  id: "4"
  sha: 80332195632fe293564ff563344032cf4c75af45
  title: 'test4: Update README.md'
  labels:
  - documentation
  - enhancement
- author: stefanprodan
  branch: stefanprodan-patch-2
  env: staging
  id: "2"
  sha: 381635adf7bfa06e48cb958531cc1d44e03a744c
  title: 'test2: Update README.md'
  labels:
  - enhancement
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := getResourceSetInputProviderReconciler()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			ns, err := testEnv.CreateNamespace(ctx, "test")
			g.Expect(err).ToNot(HaveOccurred())

			objDef := fmt.Sprintf(objDef, ns.Name, tt.skipDef)

			obj := &fluxcdv1.ResourceSetInputProvider{}
			err = yaml.Unmarshal([]byte(objDef), obj)
			g.Expect(err).ToNot(HaveOccurred())

			// Initialize the ResourceSetInputProvider.
			err = testEnv.Create(ctx, obj)
			g.Expect(err).ToNot(HaveOccurred())

			err = yaml.Unmarshal([]byte(tt.statusDef), &obj.Status)
			g.Expect(err).ToNot(HaveOccurred())

			// Manually update the exportedInputs to simulate previous exportedInputs
			err = testEnv.Status().Update(ctx, obj)
			g.Expect(err).ToNot(HaveOccurred())

			r, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(obj),
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(r.Requeue).To(BeTrue())

			r, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(obj),
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(r.Requeue).To(BeFalse())

			// Check if the ResourceSetInputProvider was marked as ready.
			result := &fluxcdv1.ResourceSetInputProvider{}
			err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
			g.Expect(err).ToNot(HaveOccurred())

			logObjectStatus(t, result)

			// Check if the exported inputs are correct.
			inputsData, err := yaml.Marshal(result.Status.ExportedInputs)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(string(inputsData)).To(MatchYAML(tt.expectedInputs))
		})
	}
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
