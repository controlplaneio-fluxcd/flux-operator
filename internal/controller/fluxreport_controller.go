// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlrun "k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// FluxReportReconciler reconciles a FluxReport object
type FluxReportReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	Scheme            *ctrlrun.Scheme
	StatusManager     string
	WatchNamespace    string
	ReportingInterval time.Duration
	Version           string
}

// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=fluxreports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=fluxreports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=fluxreports/finalizers,verbs=update

// Reconcile computes the report of the Flux instance.
func (r *FluxReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	reconcileStart := time.Now()

	obj := &fluxcdv1.FluxReport{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if errors.IsNotFound(err) {
			// Initialize the FluxReport if it doesn't exist.
			err = r.initReport(ctx, fluxcdv1.DefaultInstanceName, r.WatchNamespace)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to initialize FluxReport: %w", err)
			}
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Pause reconciliation if the object has the reconcile annotation set to 'disabled'.
	if obj.IsDisabled() {
		log.Info("Reconciliation in disabled, cannot proceed with the report computation.")
		return ctrl.Result{}, nil
	}

	// Initialize the runtime patcher with the current version of the object.
	patcher := patch.NewSerialPatcher(obj, r.Client)

	// Compute the status of the Flux instance.
	rep := reporter.NewFluxStatusReporter(r.Client, fluxcdv1.DefaultInstanceName, r.StatusManager, obj.Namespace)
	report, err := rep.Compute(ctx)
	if err != nil {
		log.Error(err, "report computed with errors")
	}

	// Set the operator info.
	report.Operator = r.getInfo()

	// Update the FluxReport with the computed spec.
	obj.Spec = report

	// Update the report timestamp.
	msg := fmt.Sprintf("Reporting finished in %s", fmtDuration(reconcileStart))
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.SucceededReason,
		"%s", msg)

	// Patch the FluxReport with the computed spec.
	err = patcher.Patch(ctx, obj, patch.WithFieldOwner(r.StatusManager))
	if err != nil {
		return ctrl.Result{}, err
	}

	log.V(1).Info(msg)
	return r.requeueAfter(obj), nil
}

// FluxReportReconcilerOptions contains options for the reconciler.
type FluxReportReconcilerOptions struct {
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// SetupWithManager sets up the controller with the Manager.
func (r *FluxReportReconciler) SetupWithManager(mgr ctrl.Manager, opts FluxReportReconcilerOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := r.initReport(ctx, fluxcdv1.DefaultInstanceName, r.WatchNamespace); err != nil {
		return fmt.Errorf("failed to initialize FluxReport: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&fluxcdv1.FluxReport{}).
		WithEventFilter(predicate.AnnotationChangedPredicate{}).
		WithOptions(controller.Options{RateLimiter: opts.RateLimiter}).
		Complete(r)
}

func (r *FluxReportReconciler) getInfo() *fluxcdv1.OperatorInfo {
	return &fluxcdv1.OperatorInfo{
		APIVersion: fluxcdv1.GroupVersion.String(),
		Version:    r.Version,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

func (r *FluxReportReconciler) initReport(ctx context.Context, name, namespace string) error {
	report := &fluxcdv1.FluxReport{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fluxcdv1.GroupVersion.String(),
			Kind:       fluxcdv1.FluxReportKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: fluxcdv1.FluxReportSpec{
			Distribution: fluxcdv1.FluxDistributionStatus{
				Status:      "Unknown",
				Entitlement: "Unknown",
			},
			Operator: r.getInfo(),
		},
	}

	if err := r.Client.Patch(ctx, report, client.Apply, client.FieldOwner(r.StatusManager)); err != nil {
		if !errors.IsConflict(err) {
			return err
		}
	}
	return nil
}

// requeueAfter returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotations. If the annotation is not set,
// the global reporting interval is used.
func (r *FluxReportReconciler) requeueAfter(obj *fluxcdv1.FluxReport) ctrl.Result {
	result := ctrl.Result{}
	if obj.GetInterval() > 0 {
		result.RequeueAfter = obj.GetInterval()
	} else {
		result.RequeueAfter = r.ReportingInterval
	}

	return result
}
