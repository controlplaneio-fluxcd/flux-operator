// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/testutils"
)

// markJobFailed marks the given Job as failed by setting the start time
// and the JobFailureTarget and JobFailed conditions on its status,
// as no Job controller is running in envtest.
func markJobFailed(ctx context.Context, g Gomega, job *batchv1.Job) {
	now := metav1.Now()
	job.Status.StartTime = &now
	job.Status.Conditions = append(job.Status.Conditions,
		batchv1.JobCondition{
			Type:               batchv1.JobFailureTarget,
			Status:             corev1.ConditionTrue,
			Reason:             "BackoffLimitExceeded",
			LastTransitionTime: now,
		},
		batchv1.JobCondition{
			Type:               batchv1.JobFailed,
			Status:             corev1.ConditionTrue,
			Reason:             "BackoffLimitExceeded",
			LastTransitionTime: now,
		})
	g.Expect(testClient.Status().Update(ctx, job)).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_Steps_LifeCycle(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-lifecycle
  namespace: "%[1]s"
spec:
  wait: true
  inputs:
    - tenant: team1
  steps:
    - name: pre-deploy
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: << inputs.tenant >>-pre
          namespace: "%[1]s"
    - name: deploy
      resources:
        - apiVersion: v1
          kind: ConfigMap
          metadata:
            name: << inputs.tenant >>-app
            namespace: "%[1]s"
    - name: post-deploy
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: << inputs.tenant >>-post
          namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
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

	// Check if the steps were applied in order.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))
	g.Expect(conditions.Has(result, meta.ReconcilingCondition)).To(BeFalse())

	// Check if the inventory contains the resources of all steps.
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(3))
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1-pre__ConfigMap", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1-app__ConfigMap", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1-post__ConfigMap", ns.Name),
			Version: "v1",
		},
	))

	// Check if the history was updated with the steps metadata.
	g.Expect(result.Status.LastAppliedRevision).ToNot(BeEmpty())
	g.Expect(result.Status.History).To(HaveLen(1))
	g.Expect(result.Status.History[0].Digest).To(Equal(result.Status.LastAppliedRevision))
	g.Expect(result.Status.History[0].LastReconciledStatus).To(Equal(meta.ReconciliationSucceededReason))
	g.Expect(result.Status.History[0].Metadata).To(HaveKeyWithValue("inputs", "1"))
	g.Expect(result.Status.History[0].Metadata).To(HaveKeyWithValue("resources", "3"))
	g.Expect(result.Status.History[0].Metadata).To(HaveKeyWithValue("steps", "3"))

	// Check if the resources were created.
	for _, name := range []string{"team1-pre", "team1-app", "team1-post"} {
		cm := &corev1.ConfigMap{}
		err = testClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns.Name}, cm)
		g.Expect(err).ToNot(HaveOccurred())
	}

	// Check if events were recorded for each step, with the final step
	// reported by the unprefixed event of the whole reconciliation.
	events := getEvents(result.Name, result.Namespace)
	g.Expect(events).To(HaveLen(4))
	g.Expect(events[0].Reason).To(Equal("ApplySucceeded"))
	g.Expect(events[0].Message).To(HavePrefix(`step "pre-deploy": `))
	g.Expect(events[0].Message).To(ContainSubstring("team1-pre created"))
	g.Expect(events[1].Reason).To(Equal("ApplySucceeded"))
	g.Expect(events[1].Message).To(HavePrefix(`step "deploy": `))
	g.Expect(events[1].Message).To(ContainSubstring("team1-app created"))
	g.Expect(events[2].Reason).To(Equal("ApplySucceeded"))
	g.Expect(events[2].Message).ToNot(HavePrefix("step"))
	g.Expect(events[2].Message).To(ContainSubstring("team1-post created"))
	g.Expect(events[3].Reason).To(Equal(meta.ReconciliationSucceededReason))
	g.Expect(events[3].Message).To(HavePrefix("Reconciliation finished"))

	// Rename the resource of the final step to trigger garbage collection.
	resultP := result.DeepCopy()
	resultP.Spec.Steps[2].ResourcesTemplate = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: << inputs.tenant >>-post2
  namespace: "%s"
`, ns.Name)
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the stale resource was deleted and the inventory updated.
	resultFinal := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.GetReason(resultFinal, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))
	g.Expect(resultFinal.Status.Inventory.Entries).To(HaveLen(3))
	g.Expect(resultFinal.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1-post2__ConfigMap", ns.Name),
			Version: "v1",
		},
	))
	g.Expect(resultFinal.Status.Inventory.Entries).ToNot(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1-post__ConfigMap", ns.Name),
			Version: "v1",
		},
	))

	cm := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "team1-post", Namespace: ns.Name}, cm)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Check if the final event contains the apply and garbage collection changes.
	events = getEvents(resultFinal.Name, resultFinal.Namespace)
	g.Expect(events).To(HaveLen(6))
	g.Expect(events[4].Reason).To(Equal("ApplySucceeded"))
	g.Expect(events[4].Message).To(ContainSubstring("team1-post2 created"))
	g.Expect(events[4].Message).To(ContainSubstring("team1-post deleted"))

	// Delete the ResourceSet.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the ResourceSet was finalized and the resources deleted.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), &fluxcdv1.ResourceSet{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	for _, name := range []string{"team1-pre", "team1-app", "team1-post2"} {
		cm := &corev1.ConfigMap{}
		err = testClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns.Name}, cm)
		g.Expect(err).To(HaveOccurred())
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}
}

func TestResourceSetReconciler_Steps_FailureBlocksLaterSteps(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-failure
  namespace: "%[1]s"
spec:
  steps:
    - name: db-migration
      timeout: 1s
      resourcesTemplate: |
        apiVersion: batch/v1
        kind: Job
        metadata:
          name: db-migration
          namespace: "%[1]s"
        spec:
          template:
            spec:
              restartPolicy: Never
              containers:
                - name: migrate
                  image: busybox
                  command: ["true"]
    - name: deploy
      resources:
        - apiVersion: v1
          kind: ConfigMap
          metadata:
            name: app-config
            namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile with the first step never becoming ready,
	// as no Job controller is running in envtest.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(`step "db-migration" health check failed`))

	// Check if the failure names the step and blocks the later steps.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationFailedReason))
	g.Expect(conditions.GetMessage(result, meta.ReadyCondition)).To(ContainSubstring(`step "db-migration" health check failed`))

	// Check if the second step resources were not applied.
	cm := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "app-config", Namespace: ns.Name}, cm)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Check if the applied Job was tracked in the inventory
	// by the status patch performed before the health check.
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_db-migration_batch_Job", ns.Name),
			Version: "v1",
		},
	))

	// Delete the ResourceSet and check finalization.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())
}

func TestResourceSetReconciler_Steps_InventoryPreservedOnFailure(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	cmStep := func(name string) string {
		return fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: "%s"
`, name, ns.Name)
	}

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-inventory
  namespace: "%[1]s"
spec:
  steps:
    - name: first
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: cm1
          namespace: "%[1]s"
    - name: second
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: cm2
          namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize and reconcile the first generation.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(2))

	// Make the first step fail and remove the second step resource.
	resultP := result.DeepCopy()
	resultP.Spec.Steps[0].Timeout = &metav1.Duration{Duration: time.Second}
	resultP.Spec.Steps[0].ResourcesTemplate = fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: blocking-job
  namespace: "%s"
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: main
          image: busybox
          command: ["true"]
`, ns.Name)
	resultP.Spec.Steps[1].ResourcesTemplate = cmStep("cm3")
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(`step "first"`))

	// Check if the inventory kept the union of the old entries
	// with the resources applied before the failure.
	resultFailed := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFailed)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultFailed)
	g.Expect(resultFailed.Status.Inventory.Entries).To(HaveLen(3))
	g.Expect(resultFailed.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm1__ConfigMap", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm2__ConfigMap", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_blocking-job_batch_Job", ns.Name),
			Version: "v1",
		},
	))

	// Check if garbage collection was skipped for the removed resource
	// and the later step resource was not created.
	cm := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "cm2", Namespace: ns.Name}, cm)
	g.Expect(err).ToNot(HaveOccurred())
	err = testClient.Get(ctx, client.ObjectKey{Name: "cm3", Namespace: ns.Name}, &corev1.ConfigMap{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Fix the first step and check if garbage collection removes the
	// stale resources tracked by the inventory union.
	resultP = resultFailed.DeepCopy()
	resultP.Spec.Steps[0].Timeout = nil
	resultP.Spec.Steps[0].ResourcesTemplate = cmStep("cm1")
	err = testClient.Patch(ctx, resultP, client.MergeFrom(resultFailed))
	g.Expect(err).ToNot(HaveOccurred())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	resultFinal := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.GetReason(resultFinal, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))
	g.Expect(resultFinal.Status.Inventory.Entries).To(HaveLen(2))
	g.Expect(resultFinal.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm1__ConfigMap", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm3__ConfigMap", ns.Name),
			Version: "v1",
		},
	))

	err = testClient.Get(ctx, client.ObjectKey{Name: "cm2", Namespace: ns.Name}, &corev1.ConfigMap{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	err = testClient.Get(ctx, client.ObjectKey{Name: "blocking-job", Namespace: ns.Name}, &batchv1.Job{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	err = testClient.Get(ctx, client.ObjectKey{Name: "cm3", Namespace: ns.Name}, &corev1.ConfigMap{})
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_Steps_RecreatesFailedJobs(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-jobs
  namespace: "%[1]s"
spec:
  steps:
    - name: jobs
      resourcesTemplate: |
        apiVersion: batch/v1
        kind: Job
        metadata:
          name: job-recreate
          namespace: "%[1]s"
          annotations:
            fluxcd.controlplane.io/recreateOnFailure: enabled
        spec:
          template:
            spec:
              restartPolicy: Never
              containers:
                - name: main
                  image: busybox
                  command: ["true"]
        ---
        apiVersion: batch/v1
        kind: Job
        metadata:
          name: job-keep
          namespace: "%[1]s"
        spec:
          template:
            spec:
              restartPolicy: Never
              containers:
                - name: main
                  image: busybox
                  command: ["true"]
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize and reconcile the Jobs.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Mark both Jobs as failed, as no Job controller is running in envtest.
	jobUID := make(map[string]string)
	for _, name := range []string{"job-recreate", "job-keep"} {
		job := &batchv1.Job{}
		err = testClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns.Name}, job)
		g.Expect(err).ToNot(HaveOccurred())
		jobUID[name] = string(job.UID)

		markJobFailed(ctx, g, job)
	}

	// Reconcile in the background as the controller waits for the
	// foreground deletion of the failed Job to complete.
	done := make(chan error, 1)
	go func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(obj),
		})
		done <- err
	}()

	// Remove the foregroundDeletion finalizer of the deleted Job,
	// as no garbage collector controller is running in envtest.
	g.Eventually(func() bool {
		job := &batchv1.Job{}
		err := testClient.Get(ctx, client.ObjectKey{Name: "job-recreate", Namespace: ns.Name}, job)
		if apierrors.IsNotFound(err) {
			return true
		}
		if err != nil {
			return false
		}
		if string(job.UID) != jobUID["job-recreate"] {
			// The Job was already recreated.
			return true
		}
		if job.DeletionTimestamp.IsZero() {
			return false
		}
		if len(job.Finalizers) > 0 {
			job.Finalizers = nil
			if err := testClient.Update(ctx, job); err != nil {
				return false
			}
		}
		return true
	}, time.Minute, 100*time.Millisecond).Should(BeTrue())

	select {
	case err = <-done:
	case <-time.After(2 * time.Minute):
		t.Fatal("timeout waiting for reconciliation to complete")
	}
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the annotated Job was recreated.
	job := &batchv1.Job{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "job-recreate", Namespace: ns.Name}, job)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(job.UID)).ToNot(Equal(jobUID["job-recreate"]))
	g.Expect(job.Status.Conditions).To(BeEmpty())

	// Check if the Job without the annotation was left untouched.
	job = &batchv1.Job{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "job-keep", Namespace: ns.Name}, job)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(job.UID)).To(Equal(jobUID["job-keep"]))

	// Delete the ResourceSet and check finalization.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())
}

func TestResourceSetReconciler_RecreatesFailedJobsWithoutSteps(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: stepless-jobs
  namespace: "%[1]s"
spec:
  resourcesTemplate: |
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: job-recreate
      namespace: "%[1]s"
      annotations:
        fluxcd.controlplane.io/recreateOnFailure: enabled
    spec:
      template:
        spec:
          restartPolicy: Never
          containers:
            - name: main
              image: busybox
              command: ["true"]
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize and reconcile the Job.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Mark the Job as failed, as no Job controller is running in envtest.
	job := &batchv1.Job{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "job-recreate", Namespace: ns.Name}, job)
	g.Expect(err).ToNot(HaveOccurred())
	jobUID := string(job.UID)

	markJobFailed(ctx, g, job)

	// Reconcile in the background as the controller waits for the
	// foreground deletion of the failed Job to complete.
	done := make(chan error, 1)
	go func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(obj),
		})
		done <- err
	}()

	// Remove the foregroundDeletion finalizer of the deleted Job,
	// as no garbage collector controller is running in envtest.
	g.Eventually(func() bool {
		job := &batchv1.Job{}
		err := testClient.Get(ctx, client.ObjectKey{Name: "job-recreate", Namespace: ns.Name}, job)
		if apierrors.IsNotFound(err) {
			return true
		}
		if err != nil {
			return false
		}
		if string(job.UID) != jobUID {
			// The Job was already recreated.
			return true
		}
		if job.DeletionTimestamp.IsZero() {
			return false
		}
		if len(job.Finalizers) > 0 {
			job.Finalizers = nil
			if err := testClient.Update(ctx, job); err != nil {
				return false
			}
		}
		return true
	}, time.Minute, 100*time.Millisecond).Should(BeTrue())

	select {
	case err = <-done:
	case <-time.After(2 * time.Minute):
		t.Fatal("timeout waiting for reconciliation to complete")
	}
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the annotated Job was recreated even though
	// the ResourceSet does not define steps.
	job = &batchv1.Job{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "job-recreate", Namespace: ns.Name}, job)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(job.UID)).ToNot(Equal(jobUID))
	g.Expect(job.Status.Conditions).To(BeEmpty())

	// Delete the ResourceSet and check finalization.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())
}

func TestResourceSetReconciler_Steps_KeepsForeignFailedJobs(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a failed Job out-of-band, annotated for recreation
	// but without this ResourceSet's owner labels.
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-foreign",
			Namespace: ns.Name,
			Annotations: map[string]string{
				fluxcdv1.RecreateOnFailureAnnotation: fluxcdv1.EnabledValue,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "main",
						Image:   "busybox",
						Command: []string{"true"},
					}},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, job)).ToNot(HaveOccurred())
	jobUID := string(job.UID)

	// Mark the Job as failed, as no Job controller is running in envtest.
	markJobFailed(ctx, g, job)

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-foreign-job
  namespace: "%[1]s"
spec:
  steps:
    - name: jobs
      resourcesTemplate: |
        apiVersion: batch/v1
        kind: Job
        metadata:
          name: job-foreign
          namespace: "%[1]s"
          annotations:
            fluxcd.controlplane.io/recreateOnFailure: enabled
        spec:
          template:
            spec:
              restartPolicy: Never
              containers:
                - name: main
                  image: busybox
                  command: ["true"]
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize and reconcile the ResourceSet.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the failed Job not owned by the ResourceSet was adopted
	// by the server-side apply instead of being deleted and recreated.
	resultJob := &batchv1.Job{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "job-foreign", Namespace: ns.Name}, resultJob)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(resultJob.UID)).To(Equal(jobUID))
	g.Expect(resultJob.Status.Conditions).ToNot(BeEmpty())
}

func TestResourceSetReconciler_Steps_PartialApplyKeepsInventory(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a service account which is allowed to apply Namespaces but
	// not ConfigMaps, to make the apply fail mid-step after the Namespace
	// stage has been applied.
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "partial-sa",
			Namespace: ns.Name,
		},
	}
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-partial-apply", ns.Name),
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "list", "watch", "create", "patch", "update"},
		}},
	}
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-partial-apply", ns.Name),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "partial-sa",
			Namespace: ns.Name,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: fmt.Sprintf("%s-partial-apply", ns.Name),
		},
	}
	g.Expect(testClient.Create(ctx, sa)).ToNot(HaveOccurred())
	g.Expect(testClient.Create(ctx, clusterRole)).ToNot(HaveOccurred())
	g.Expect(testClient.Create(ctx, clusterRoleBinding)).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-partial-apply
  namespace: "%[1]s"
spec:
  serviceAccountName: partial-sa
  steps:
    - name: first
      resourcesTemplate: |
        apiVersion: v1
        kind: Namespace
        metadata:
          name: "%[1]s-partial"
        ---
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: cm1
          namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile with the ConfigMap apply denied by RBAC, after the
	// Namespace stage of the same step has been applied.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(`step "first" apply failed`))
	g.Expect(err.Error()).To(ContainSubstring("forbidden"))

	// Check if the Namespace applied before the in-step failure
	// was tracked in the inventory.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationFailedReason))
	g.Expect(result.Status.Inventory).ToNot(BeNil())
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("_%s-partial__Namespace", ns.Name),
			Version: "v1",
		},
	))

	// Check if the Namespace was created on the cluster.
	err = testClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-partial", ns.Name)}, &corev1.Namespace{})
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetSteps_Validation(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	cmTemplate := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  namespace: "%s"
`, ns.Name)

	// Duplicate step names are rejected by the CRD CEL rule.
	err = testEnv.Create(ctx, &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dup-names",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			Steps: []fluxcdv1.ResourceSetStep{
				{Name: "deploy", ResourcesTemplate: cmTemplate},
				{Name: "deploy", ResourcesTemplate: cmTemplate},
			},
		},
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("step names must be unique"))

	// Invalid step names are rejected by the CRD pattern.
	err = testEnv.Create(ctx, &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-name",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			Steps: []fluxcdv1.ResourceSetStep{
				{Name: "Not_Valid", ResourcesTemplate: cmTemplate},
			},
		},
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("should match"))

	// Invalid step timeouts are rejected by the CRD pattern.
	err = testEnv.Create(ctx, &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "fluxcd.controlplane.io/v1",
			"kind":       "ResourceSet",
			"metadata": map[string]any{
				"name":      "bad-timeout",
				"namespace": ns.Name,
			},
			"spec": map[string]any{
				"steps": []any{
					map[string]any{
						"name":              "deploy",
						"timeout":           "5x",
						"resourcesTemplate": cmTemplate,
					},
				},
			},
		},
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.steps[0].timeout"))

	// More than 20 steps are rejected by the CRD.
	manySteps := make([]fluxcdv1.ResourceSetStep, 21)
	for i := range manySteps {
		manySteps[i] = fluxcdv1.ResourceSetStep{
			Name:              fmt.Sprintf("step-%d", i),
			ResourcesTemplate: cmTemplate,
		}
	}
	err = testEnv.Create(ctx, &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "too-many-steps",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			Steps: manySteps,
		},
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must have at most 20 items"))

	// reconcileTerminal initializes the object and asserts that the
	// reconciliation fails terminally with the build failed reason.
	reconcileTerminal := func(obj *fluxcdv1.ResourceSet, msg string) {
		err = testEnv.Create(ctx, obj)
		g.Expect(err).ToNot(HaveOccurred())

		r, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(obj),
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(r.Requeue).To(BeTrue())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(obj),
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeTrue())

		result := &fluxcdv1.ResourceSet{}
		err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
		g.Expect(err).ToNot(HaveOccurred())

		testutils.LogObjectStatus(t, result)
		g.Expect(conditions.IsStalled(result)).To(BeTrue())
		g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.BuildFailedReason))
		g.Expect(conditions.GetMessage(result, meta.ReadyCondition)).To(ContainSubstring(msg))
	}

	// Steps with resources are rejected by the controller.
	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-and-resources
  namespace: "%[1]s"
spec:
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: legacy
        namespace: "%[1]s"
  steps:
    - name: deploy
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: stepped
          namespace: "%[1]s"
`, ns.Name)
	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())
	reconcileTerminal(obj, "spec.steps is mutually exclusive with spec.resources and spec.resourcesTemplate")

	// Steps with resourcesTemplate are rejected by the controller.
	obj = &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "steps-and-template",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			ResourcesTemplate: cmTemplate,
			Steps: []fluxcdv1.ResourceSetStep{
				{Name: "deploy", ResourcesTemplate: cmTemplate},
			},
		},
	}
	reconcileTerminal(obj, "spec.steps is mutually exclusive with spec.resources and spec.resourcesTemplate")

	// Steps without resources and resourcesTemplate are rejected by the controller.
	obj = &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "step-empty",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			Steps: []fluxcdv1.ResourceSetStep{
				{Name: "migrate"},
			},
		},
	}
	reconcileTerminal(obj, `step "migrate": at least one of resources or resourcesTemplate must be set`)
}

func TestResourceSetReconciler_Steps_GCFailureKeepsInventory(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a service account which is not allowed to delete ConfigMaps,
	// to make garbage collection fail while apply succeeds.
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gc-sa",
			Namespace: ns.Name,
		},
	}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gc-role",
			Namespace: ns.Name,
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch", "create", "patch", "update"},
		}},
	}
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gc-role",
			Namespace: ns.Name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "gc-sa",
			Namespace: ns.Name,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "gc-role",
		},
	}
	g.Expect(testClient.Create(ctx, sa)).ToNot(HaveOccurred())
	g.Expect(testClient.Create(ctx, role)).ToNot(HaveOccurred())
	g.Expect(testClient.Create(ctx, rb)).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-gc-failure
  namespace: "%[1]s"
spec:
  serviceAccountName: gc-sa
  steps:
    - name: first
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: cm1
          namespace: "%[1]s"
    - name: second
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: cm2
          namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize and reconcile the first generation.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(2))

	// Rename the second step resource to trigger garbage collection.
	resultP := result.DeepCopy()
	resultP.Spec.Steps[1].ResourcesTemplate = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
  namespace: "%s"
`, ns.Name)
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile with garbage collection denied by RBAC.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("forbidden"))

	// Check if the inventory kept the union of the old and new entries.
	resultFailed := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFailed)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultFailed)
	g.Expect(conditions.GetReason(resultFailed, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationFailedReason))
	g.Expect(resultFailed.Status.Inventory.Entries).To(HaveLen(3))
	g.Expect(resultFailed.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm2__ConfigMap", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm3__ConfigMap", ns.Name),
			Version: "v1",
		},
	))

	// Check if the stale resource is still on the cluster.
	err = testClient.Get(ctx, client.ObjectKey{Name: "cm2", Namespace: ns.Name}, &corev1.ConfigMap{})
	g.Expect(err).ToNot(HaveOccurred())

	// Allow the service account to delete ConfigMaps and check if the
	// stale resource is deleted and dropped from the inventory.
	roleP := role.DeepCopy()
	roleP.Rules[0].Verbs = append(roleP.Rules[0].Verbs, "delete")
	err = testClient.Patch(ctx, roleP, client.MergeFrom(role))
	g.Expect(err).ToNot(HaveOccurred())

	g.Eventually(func() error {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(obj),
		})
		return err
	}, time.Minute, time.Second).Should(Succeed())

	resultFinal := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.GetReason(resultFinal, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))
	g.Expect(resultFinal.Status.Inventory.Entries).To(HaveLen(2))
	g.Expect(resultFinal.Status.Inventory.Entries).ToNot(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm2__ConfigMap", ns.Name),
			Version: "v1",
		},
	))

	err = testClient.Get(ctx, client.ObjectKey{Name: "cm2", Namespace: ns.Name}, &corev1.ConfigMap{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestResourceSetReconciler_Steps_EmptyInputs(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	rsipReconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create and reconcile a static input provider.
	rsipDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: static
  namespace: "%[1]s"
  labels:
    steps: tenant
spec:
  type: Static
  defaultValues:
    tenant: team1
`, ns.Name)

	rsip := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(rsipDef), rsip)
	g.Expect(err).ToNot(HaveOccurred())
	err = testEnv.Create(ctx, rsip)
	g.Expect(err).ToNot(HaveOccurred())
	for range 2 {
		_, err = rsipReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(rsip),
		})
		g.Expect(err).ToNot(HaveOccurred())
	}

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-empty-inputs
  namespace: "%[1]s"
spec:
  inputsFrom:
    - selector:
        matchLabels:
          steps: tenant
  steps:
    - name: pre-deploy
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: << inputs.tenant >>-pre
          namespace: "%[1]s"
    - name: deploy
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: << inputs.tenant >>-app
          namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize and reconcile with the provider inputs.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(2))

	// Remove the provider label so the selector matches no providers
	// and the ResourceSet reconciles an empty set.
	rsipResult := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(rsip), rsipResult)
	g.Expect(err).ToNot(HaveOccurred())
	rsipP := rsipResult.DeepCopy()
	rsipP.SetLabels(map[string]string{})
	err = testClient.Patch(ctx, rsipP, client.MergeFrom(rsipResult))
	g.Expect(err).ToNot(HaveOccurred())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if all the resources were garbage collected.
	resultFinal := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultFinal)
	g.Expect(conditions.IsReady(resultFinal)).To(BeTrue())
	g.Expect(resultFinal.Status.Inventory.Entries).To(BeEmpty())

	for _, name := range []string{"team1-pre", "team1-app"} {
		err = testClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns.Name}, &corev1.ConfigMap{})
		g.Expect(err).To(HaveOccurred())
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}
}

func TestResourceSetReconciler_Steps_InvalidSpecBlocksEmptyInputsGC(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	rsipReconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create and reconcile a static input provider.
	rsipDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: static
  namespace: "%[1]s"
  labels:
    steps: invalid-spec
spec:
  type: Static
  defaultValues:
    tenant: team1
`, ns.Name)

	rsip := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(rsipDef), rsip)
	g.Expect(err).ToNot(HaveOccurred())
	err = testEnv.Create(ctx, rsip)
	g.Expect(err).ToNot(HaveOccurred())
	for range 2 {
		_, err = rsipReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(rsip),
		})
		g.Expect(err).ToNot(HaveOccurred())
	}

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-invalid-spec
  namespace: "%[1]s"
spec:
  inputsFrom:
    - selector:
        matchLabels:
          steps: invalid-spec
  steps:
    - name: pre-deploy
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: << inputs.tenant >>-pre
          namespace: "%[1]s"
    - name: deploy
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: << inputs.tenant >>-app
          namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize and reconcile with the provider inputs.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(2))

	// Make a step invalid and remove the provider label in the same
	// change, so that the spec validation must reject the object before
	// the empty-inputs branch can garbage collect the inventory.
	resultP := result.DeepCopy()
	resultP.Spec.Steps[1].ResourcesTemplate = ""
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	rsipResult := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(rsip), rsipResult)
	g.Expect(err).ToNot(HaveOccurred())
	rsipP := rsipResult.DeepCopy()
	rsipP.SetLabels(map[string]string{})
	err = testClient.Patch(ctx, rsipP, client.MergeFrom(rsipResult))
	g.Expect(err).ToNot(HaveOccurred())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(`step "deploy": at least one of resources or resourcesTemplate must be set`))

	// Check that the object is stalled and nothing was garbage collected.
	resultFinal := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultFinal)
	g.Expect(conditions.IsReady(resultFinal)).To(BeFalse())
	g.Expect(conditions.GetReason(resultFinal, meta.ReadyCondition)).To(Equal(meta.BuildFailedReason))
	g.Expect(conditions.IsStalled(resultFinal)).To(BeTrue())
	g.Expect(resultFinal.Status.Inventory.Entries).To(HaveLen(2))

	for _, name := range []string{"team1-pre", "team1-app"} {
		err = testClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns.Name}, &corev1.ConfigMap{})
		g.Expect(err).ToNot(HaveOccurred())
	}
}

func TestResourceSetReconciler_Steps_ChecksumFrom(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// The consumer in the second step references the ConfigMap rendered by
	// the first step, which does not exist on the cluster at build time,
	// proving the checksum is resolved from the in-memory flattened slice.
	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: steps-checksum
  namespace: "%[1]s"
spec:
  steps:
    - name: data
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: app-data
          namespace: "%[1]s"
        data:
          foo: bar
    - name: consumer
      resourcesTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: app-consumer
          namespace: "%[1]s"
          annotations:
            fluxcd.controlplane.io/checksumFrom: ConfigMap/%[1]s/app-data
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize and reconcile the ResourceSet.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())

	// Check if the checksum annotation was computed from the in-memory
	// data and the in-set reference was not tracked as external.
	cm := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "app-consumer", Namespace: ns.Name}, cm)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cm.Annotations).To(HaveKey(fluxcdv1.ChecksumAnnotation))
	g.Expect(cm.Annotations[fluxcdv1.ChecksumAnnotation]).To(HavePrefix("sha256:"))
	g.Expect(result.Status.ExternalChecksumRefs).To(BeEmpty())
}
