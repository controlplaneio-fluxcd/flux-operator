// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/fluxcd/pkg/ssa"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var (
	operatorSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": "flux-operator",
		},
	}
	controllerSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/part-of": "flux",
		},
	}
)

// UninstallControllers deletes the Kubernetes deployments of Flux Operator
// and the Flux controllers and waits for their removal.
// Returns a list of deleted deployments in the format "Deployment/namespace/name".
func (in *Installer) UninstallControllers(ctx context.Context) ([]string, error) {
	var errs []error
	var deployments []*unstructured.Unstructured

	// List operator deployments
	operatorDeployments := &unstructured.UnstructuredList{}
	operatorDeployments.SetAPIVersion("apps/v1")
	operatorDeployments.SetKind("DeploymentList")

	if err := in.kubeClient.List(ctx, operatorDeployments,
		client.MatchingLabels(operatorSelector.MatchLabels)); err != nil {
		errs = append(errs, err)
	} else {
		for i := range operatorDeployments.Items {
			deployments = append(deployments, &operatorDeployments.Items[i])
		}
	}

	// List controller deployments
	controllerDeployments := &unstructured.UnstructuredList{}
	controllerDeployments.SetAPIVersion("apps/v1")
	controllerDeployments.SetKind("DeploymentList")

	if err := in.kubeClient.List(ctx, controllerDeployments,
		client.MatchingLabels(controllerSelector.MatchLabels)); err != nil {
		errs = append(errs, err)
	} else {
		for i := range controllerDeployments.Items {
			deployments = append(deployments, &controllerDeployments.Items[i])
		}
	}

	// Build list of deployment names in format "Deployment/namespace/name"
	names := make([]string, 0, len(deployments))
	for _, d := range deployments {
		names = append(names, d.GetKind()+"/"+d.GetNamespace()+"/"+d.GetName())
	}

	// Delete all deployments and wait for their removal
	_, err := in.kubeClient.Manager.DeleteAll(ctx, deployments, ssa.DeleteOptions{
		PropagationPolicy: metav1.DeletePropagationBackground,
	})
	if err != nil {
		errs = append(errs, err)
	} else {
		if err := in.kubeClient.Manager.WaitForTermination(deployments, ssa.WaitOptions{
			Interval: 5 * time.Second,
			Timeout:  in.options.TerminationTimeout(),
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return names, errors.Join(errs...)
}

// UninstallCRDs deletes the CustomResourceDefinitions of Flux Operator
// and the Flux controllers and waits for their removal.
// Returns a list of deleted CRD names.
func (in *Installer) UninstallCRDs(ctx context.Context) ([]string, error) {
	var errs []error
	var crds []*unstructured.Unstructured

	// List operator CRDs
	operatorCRDs := &unstructured.UnstructuredList{}
	operatorCRDs.SetAPIVersion("apiextensions.k8s.io/v1")
	operatorCRDs.SetKind("CustomResourceDefinitionList")

	if err := in.kubeClient.List(ctx, operatorCRDs,
		client.MatchingLabels(operatorSelector.MatchLabels)); err != nil {
		errs = append(errs, err)
	} else {
		for i := range operatorCRDs.Items {
			crds = append(crds, &operatorCRDs.Items[i])
		}
	}

	// List controller CRDs
	controllerCRDs := &unstructured.UnstructuredList{}
	controllerCRDs.SetAPIVersion("apiextensions.k8s.io/v1")
	controllerCRDs.SetKind("CustomResourceDefinitionList")

	if err := in.kubeClient.List(ctx, controllerCRDs,
		client.MatchingLabels(controllerSelector.MatchLabels)); err != nil {
		errs = append(errs, err)
	} else {
		for i := range controllerCRDs.Items {
			crds = append(crds, &controllerCRDs.Items[i])
		}
	}

	// Build list of CRD names
	names := make([]string, 0, len(crds))
	for _, crd := range crds {
		names = append(names, crd.GetName())
	}

	// Delete all CRDs and wait for their removal
	_, err := in.kubeClient.Manager.DeleteAll(ctx, crds, ssa.DeleteOptions{
		PropagationPolicy: metav1.DeletePropagationBackground,
	})
	if err != nil {
		errs = append(errs, err)
	} else {
		if err := in.kubeClient.Manager.WaitForTermination(crds, ssa.WaitOptions{
			Interval: 5 * time.Second,
			Timeout:  in.options.TerminationTimeout(),
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return names, errors.Join(errs...)
}

// UninstallRBAC deletes the ClusterRoles and ClusterRoleBindings of Flux Operator
// and the Flux controllers and waits for their removal.
// Returns a list of deleted RBAC resources in the format "Kind/name".
func (in *Installer) UninstallRBAC(ctx context.Context) ([]string, error) {
	var errs []error
	var rbacResources []*unstructured.Unstructured

	// List operator ClusterRoles
	operatorClusterRoles := &unstructured.UnstructuredList{}
	operatorClusterRoles.SetAPIVersion("rbac.authorization.k8s.io/v1")
	operatorClusterRoles.SetKind("ClusterRoleList")

	if err := in.kubeClient.List(ctx, operatorClusterRoles,
		client.MatchingLabels(operatorSelector.MatchLabels)); err != nil {
		errs = append(errs, err)
	} else {
		for i := range operatorClusterRoles.Items {
			rbacResources = append(rbacResources, &operatorClusterRoles.Items[i])
		}
	}

	// List controller ClusterRoles
	controllerClusterRoles := &unstructured.UnstructuredList{}
	controllerClusterRoles.SetAPIVersion("rbac.authorization.k8s.io/v1")
	controllerClusterRoles.SetKind("ClusterRoleList")

	if err := in.kubeClient.List(ctx, controllerClusterRoles,
		client.MatchingLabels(controllerSelector.MatchLabels)); err != nil {
		errs = append(errs, err)
	} else {
		for i := range controllerClusterRoles.Items {
			rbacResources = append(rbacResources, &controllerClusterRoles.Items[i])
		}
	}

	// List operator ClusterRoleBindings
	operatorClusterRoleBindings := &unstructured.UnstructuredList{}
	operatorClusterRoleBindings.SetAPIVersion("rbac.authorization.k8s.io/v1")
	operatorClusterRoleBindings.SetKind("ClusterRoleBindingList")

	if err := in.kubeClient.List(ctx, operatorClusterRoleBindings,
		client.MatchingLabels(operatorSelector.MatchLabels)); err != nil {
		errs = append(errs, err)
	} else {
		for i := range operatorClusterRoleBindings.Items {
			rbacResources = append(rbacResources, &operatorClusterRoleBindings.Items[i])
		}
	}

	// List controller ClusterRoleBindings
	controllerClusterRoleBindings := &unstructured.UnstructuredList{}
	controllerClusterRoleBindings.SetAPIVersion("rbac.authorization.k8s.io/v1")
	controllerClusterRoleBindings.SetKind("ClusterRoleBindingList")

	if err := in.kubeClient.List(ctx, controllerClusterRoleBindings,
		client.MatchingLabels(controllerSelector.MatchLabels)); err != nil {
		errs = append(errs, err)
	} else {
		for i := range controllerClusterRoleBindings.Items {
			rbacResources = append(rbacResources, &controllerClusterRoleBindings.Items[i])
		}
	}

	// Build list of RBAC resource names in format "Kind/name"
	names := make([]string, 0, len(rbacResources))
	for _, r := range rbacResources {
		names = append(names, r.GetKind()+"/"+r.GetName())
	}

	// Delete all RBAC resources and wait for their removal
	_, err := in.kubeClient.Manager.DeleteAll(ctx, rbacResources, ssa.DeleteOptions{
		PropagationPolicy: metav1.DeletePropagationBackground,
	})
	if err != nil {
		errs = append(errs, err)
	} else {
		if err := in.kubeClient.Manager.WaitForTermination(rbacResources, ssa.WaitOptions{
			Interval: 5 * time.Second,
			Timeout:  in.options.TerminationTimeout(),
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return names, errors.Join(errs...)
}

// UninstallNamespace deletes the namespace where Flux Operator and Flux instance are installed
// and waits for its removal.
func (in *Installer) UninstallNamespace(ctx context.Context) error {
	var errs []error

	namespace := &unstructured.Unstructured{}
	namespace.SetAPIVersion("v1")
	namespace.SetKind("Namespace")
	namespace.SetName(in.options.Namespace())

	if err := in.kubeClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}

	_, err := in.kubeClient.Manager.DeleteAll(ctx, []*unstructured.Unstructured{namespace}, ssa.DeleteOptions{
		PropagationPolicy: metav1.DeletePropagationBackground,
	})
	if err != nil {
		errs = append(errs, err)
	} else {
		if err := in.kubeClient.Manager.WaitForTermination([]*unstructured.Unstructured{namespace}, ssa.WaitOptions{
			Interval: 5 * time.Second,
			Timeout:  in.options.TerminationTimeout(),
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// RemoveFinalizers removes the finalizers from the Flux Operator and Flux
// custom resources across all namespaces to allow their deletion.
func (in *Installer) RemoveFinalizers(ctx context.Context) error {
	var errs []error
	versions, err := in.getInstalledGVKs()
	if err != nil {
		errs = append(errs, err)
	}

	for kind, apiVersion := range versions {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return in.removeFinalizersFor(ctx, apiVersion, kind+"List")
		})
		if err != nil {
			if !strings.Contains(err.Error(), "the server could not find the requested resource") &&
				!strings.Contains(err.Error(), "no matches for kind") {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

// removeFinalizersFor is a generic function to remove finalizers from all resources.
func (in *Installer) removeFinalizersFor(ctx context.Context, apiVersion, listKind string) error {
	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion(apiVersion)
	list.SetKind(listKind)

	err := in.kubeClient.List(ctx, list, client.InNamespace(""))
	if err != nil {
		return err
	}

	var errs []error
	for i := range list.Items {
		entry := list.Items[i]
		entry.SetFinalizers([]string{})
		if err := in.kubeClient.Update(ctx, &entry); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// getInstalledGVKs returns a map of installed Flux custom resource kinds to their preferred API versions.
func (in *Installer) getInstalledGVKs() (map[string]string, error) {
	var errs []error
	result := make(map[string]string)

	kinds := slices.Concat(fluxcdv1.FluxOperatorKinds, fluxcdv1.FluxKinds)
	for _, info := range kinds {
		gk, err := fluxcdv1.FluxGroupFor(info.Name)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		mapping, err := in.kubeClient.RESTMapper().RESTMapping(*gk)
		if err != nil {
			if !strings.Contains(err.Error(), "no matches for kind") {
				errs = append(errs, err)
			}
			continue
		}

		result[info.Name] = mapping.GroupVersionKind.GroupVersion().String()
	}

	return result, errors.Join(errs...)
}
