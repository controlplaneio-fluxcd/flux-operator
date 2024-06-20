// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"errors"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *FluxStatusReporter) listCRDs(ctx context.Context) ([]metav1.GroupVersionKind, error) {
	var list apiextensionsv1.CustomResourceDefinitionList
	if err := r.List(ctx, &list, client.InNamespace(""), r.labelSelector); err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	if len(list.Items) == 0 {
		return nil, errors.New("no Flux CRDs found")
	}

	gvkList := make([]metav1.GroupVersionKind, len(list.Items))
	for i, crd := range list.Items {
		gvk := metav1.GroupVersionKind{
			Group: crd.Spec.Group,
			Kind:  crd.Spec.Names.Kind,
		}
		versions := crd.Status.StoredVersions
		if len(versions) > 0 {
			gvk.Version = versions[len(versions)-1]
		} else {
			return nil, fmt.Errorf("no stored versions found for CRD %s", crd.Name)
		}
		gvkList[i] = gvk
	}

	return gvkList, nil
}

func gvkFor(kind string, crds []metav1.GroupVersionKind) *metav1.GroupVersionKind {
	for _, gvk := range crds {
		if gvk.Kind == kind {
			return &gvk
		}
	}
	return nil
}
