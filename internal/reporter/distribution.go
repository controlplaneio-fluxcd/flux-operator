// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/entitlement"
)

func (r *FluxStatusReporter) getDistributionStatus(ctx context.Context) fluxcdv1.FluxDistributionStatus {
	result := fluxcdv1.FluxDistributionStatus{
		Status:      "Unknown",
		Entitlement: "Unknown",
	}

	crdMeta := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "gitrepositories.source.toolkit.fluxcd.io",
		},
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(crdMeta), crdMeta); err == nil {
		result.Status = "Installed"

		if version, found := crdMeta.Labels["app.kubernetes.io/version"]; found {
			result.Version = version
		}

		if manager, ok := crdMeta.Labels["app.kubernetes.io/managed-by"]; ok {
			result.ManagedBy = manager
		} else if _, ok := crdMeta.Labels["kustomize.toolkit.fluxcd.io/name"]; ok {
			result.ManagedBy = "flux bootstrap"
		}
	} else {
		result.Status = "Not Installed"
	}

	entitlementSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: r.namespace,
		Name:      fmt.Sprintf("%s-entitlement", r.manager),
	}, entitlementSecret)
	if err == nil {
		if _, found := entitlementSecret.Data[entitlement.TokenKey]; found {
			result.Entitlement = "Issued"
			if vendor, found := entitlementSecret.Data[entitlement.VendorKey]; found {
				result.Entitlement += " by " + string(vendor)
			}
		}
	}

	return result
}
