// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Package copier provides shared Kubernetes resource data-copy transforms.
package copier

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	kindConfigMap = "ConfigMap"
	kindSecret    = "Secret"
)

// CopyResources copies data from ConfigMaps and Secrets referenced by the
// fluxcd.controlplane.io/copyFrom annotation. It intentionally does not process
// checksumFrom or convertKubeConfigFrom annotations.
func CopyResources(ctx context.Context, kubeClient client.Client, objects []*unstructured.Unstructured) error {
	for i := range objects {
		if objects[i].GetAPIVersion() == "v1" {
			source, found := objects[i].GetAnnotations()[fluxcdv1.CopyFromAnnotation]
			if !found {
				continue
			}

			sourceParts := strings.Split(source, "/")
			if len(sourceParts) != 2 {
				return fmt.Errorf("invalid %s annotation value '%s' must be in the format 'namespace/name'",
					fluxcdv1.CopyFromAnnotation, source)
			}

			sourceName := types.NamespacedName{
				Namespace: sourceParts[0],
				Name:      sourceParts[1],
			}

			switch objects[i].GetKind() {
			case kindConfigMap:
				cm := &corev1.ConfigMap{}
				if err := kubeClient.Get(ctx, sourceName, cm); err != nil {
					return fmt.Errorf("failed to copy data from ConfigMap/%s: %w", source, err)
				}
				if err := unstructured.SetNestedStringMap(objects[i].Object, cm.Data, "data"); err != nil {
					return fmt.Errorf("failed to copy data from ConfigMap/%s: %w", source, err)
				}
				if len(cm.BinaryData) > 0 {
					binaryData := make(map[string]string, len(cm.BinaryData))
					for k, v := range cm.BinaryData {
						binaryData[k] = base64.StdEncoding.EncodeToString(v)
					}
					if err := unstructured.SetNestedStringMap(objects[i].Object, binaryData, "binaryData"); err != nil {
						return fmt.Errorf("failed to copy binaryData from ConfigMap/%s: %w", source, err)
					}
				}
			case kindSecret:
				secret := &corev1.Secret{}
				if err := kubeClient.Get(ctx, sourceName, secret); err != nil {
					return fmt.Errorf("failed to copy data from Secret/%s: %w", source, err)
				}
				_, ok, err := unstructured.NestedString(objects[i].Object, "type")
				if err != nil {
					return fmt.Errorf("type field of Secret/%s is not a string: %w", source, err)
				}
				if !ok {
					if secret.Type == "" {
						secret.Type = corev1.SecretTypeOpaque
					}
					if err := unstructured.SetNestedField(objects[i].Object, string(secret.Type), "type"); err != nil {
						return fmt.Errorf("failed to copy type from Secret/%s: %w", source, err)
					}
				}
				data := make(map[string]string, len(secret.Data))
				for k, v := range secret.Data {
					data[k] = string(v)
				}
				if err := unstructured.SetNestedStringMap(objects[i].Object, data, "stringData"); err != nil {
					return fmt.Errorf("failed to copy data from Secret/%s: %w", source, err)
				}
			}
		}
	}
	return nil
}
