// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/opencontainers/go-digest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/kubeconfig"
)

const (
	kindConfigMap = "ConfigMap"
	kindSecret    = "Secret"
)

// copyResources copies data from ConfigMaps and Secrets based on the
// annotations set on the resources template.
func (r *ResourceSetReconciler) copyResources(ctx context.Context,
	kubeClient client.Client, objects []*unstructured.Unstructured) error {
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

// convertKubeConfigResources converts kubeconfig data stored in Secrets
// into ConfigMap fields by extracting the server and CA certificate.
// The conversion is triggered using a specific annotation on the ConfigMap.
// The annotation value must be in the format 'namespace/name' or 'namespace/name:key'.
// When no key is specified, the function looks for 'kubeconfig' first, then 'value'.
func (r *ResourceSetReconciler) convertKubeConfigResources(
	ctx context.Context,
	kubeClient client.Client,
	objects []*unstructured.Unstructured,
) error {

	for i := range objects {
		if objects[i].GetAPIVersion() != "v1" || objects[i].GetKind() != kindConfigMap {
			continue
		}

		source, found := objects[i].GetAnnotations()[fluxcdv1.ConvertKubeConfigFromAnnotation]
		if !found {
			continue
		}

		// Parse the annotation value to extract namespace/name and optional key.
		// Supported formats: 'namespace/name' or 'namespace/name:key'.
		var customKey string
		nameRef := source
		if colonIdx := strings.LastIndex(source, ":"); colonIdx > 0 {
			if slashIdx := strings.Index(source, "/"); slashIdx > 0 && colonIdx > slashIdx {
				customKey = source[colonIdx+1:]
				nameRef = source[:colonIdx]
			}
		}

		sourceParts := strings.Split(nameRef, "/")
		if len(sourceParts) != 2 {
			return fmt.Errorf("invalid %s annotation value '%s' must be in the format 'namespace/name' or 'namespace/name:key'", fluxcdv1.ConvertKubeConfigFromAnnotation, source)
		}

		sourceName := types.NamespacedName{
			Namespace: sourceParts[0],
			Name:      sourceParts[1],
		}

		secret := &corev1.Secret{}
		if err := kubeClient.Get(ctx, sourceName, secret); err != nil {
			return fmt.Errorf("failed to get kubeconfig Secret/%s: %w", nameRef, err)
		}

		var data []byte
		var exists bool
		if customKey != "" {
			data, exists = secret.Data[customKey]
			if !exists {
				return fmt.Errorf("kubeconfig Secret/%s does not have '%s' field", nameRef, customKey)
			}
		} else {
			data, exists = secret.Data["kubeconfig"]
			if !exists {
				data, exists = secret.Data["value"]
			}
			if !exists {
				return fmt.Errorf("kubeconfig Secret/%s does not have 'kubeconfig' or 'value' field", nameRef)
			}
		}

		kubeconfigYAML := string(data)

		server, caCert, err := kubeconfig.ExtractFluxFields(kubeconfigYAML)
		if err != nil {
			return fmt.Errorf("failed to extract fields from kubeconfig Secret/%s: %w", source, err)
		}

		existingData, _, err := unstructured.NestedStringMap(objects[i].Object, "data")
		if err != nil {
			return fmt.Errorf("failed to get existing data from ConfigMap: %w", err)
		}
		if existingData == nil {
			existingData = make(map[string]string)
		}

		if _, exists := existingData["address"]; !exists {
			existingData["address"] = server
		}
		if _, exists := existingData["ca.crt"]; !exists {
			existingData["ca.crt"] = caCert
		}

		if err := unstructured.SetNestedStringMap(objects[i].Object, existingData, "data"); err != nil {
			return fmt.Errorf("failed to set data on ConfigMap: %w", err)
		}
	}

	return nil
}

// computeChecksumsFromAnnotations walks every rendered object tree looking
// for metadata.annotations maps that contain the checksumFrom annotation.
// For each occurrence, it resolves the comma-separated list of
// ConfigMap/Secret references, computes a combined SHA256 digest over the
// canonicalized data, and sets the checksum annotation next to checksumFrom.
// The original checksumFrom annotation is preserved.
//
// References to ConfigMaps/Secrets also rendered by this ResourceSet are
// resolved from the in-memory slice so the checksum reflects the
// about-to-be-applied data rather than any absent or stale cluster state.
// Repeated references to the same source within a single reconciliation
// are fetched at most once.
//
// It returns the sorted list of external references resolved from the
// cluster, so the caller can persist them on the ResourceSet status and
// trigger reconciliation when any of them changes. In-set references are
// omitted because their changes already change the applied digest.
func (r *ResourceSetReconciler) computeChecksumsFromAnnotations(ctx context.Context,
	kubeClient client.Client, objects []*unstructured.Unstructured) ([]string, error) {
	cr := newChecksumResolver(ctx, kubeClient, objects)
	for i := range objects {
		if err := cr.walk(objects[i].Object); err != nil {
			return nil, err
		}
	}
	return cr.externalRefs(), nil
}

// checksumResolver resolves checksumFrom references during a single
// reconciliation pass. It indexes in-set ConfigMaps/Secrets so references
// to objects rendered by the same ResourceSet are resolved from pending
// data (not from an absent or stale cluster copy), and caches canonical
// data per reference so repeated references fetch at most once.
type checksumResolver struct {
	ctx       context.Context
	client    client.Client
	rendered  map[string]*unstructured.Unstructured
	canonical map[string]map[string][]byte
	external  map[string]struct{}
}

func newChecksumResolver(ctx context.Context, c client.Client,
	objects []*unstructured.Unstructured) *checksumResolver {
	rendered := make(map[string]*unstructured.Unstructured)
	for _, o := range objects {
		if kind := o.GetKind(); kind == kindConfigMap || kind == kindSecret {
			rendered[checksumRefKey(kind, o.GetNamespace(), o.GetName())] = o
		}
	}
	return &checksumResolver{
		ctx:       ctx,
		client:    c,
		rendered:  rendered,
		canonical: make(map[string]map[string][]byte),
		external:  make(map[string]struct{}),
	}
}

// externalRefs returns the sorted list of references that were resolved
// from the cluster (not from the in-set rendered slice). Used by the
// caller to persist the dependency set on the ResourceSet status.
func (cr *checksumResolver) externalRefs() []string {
	if len(cr.external) == 0 {
		return nil
	}
	out := make([]string, 0, len(cr.external))
	for k := range cr.external {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// checksumRefKey builds the canonical "Kind/namespace/name" string used to
// index both the in-set rendered objects and the canonical-data cache and
// as the per-reference header in the hash input.
func checksumRefKey(kind, namespace, name string) string {
	return kind + "/" + namespace + "/" + name
}

// walk recursively descends through maps and slices. Whenever it finds a
// map named "metadata" whose "annotations" subfield contains the
// checksumFrom annotation, it resolves the references and writes the
// resulting checksum alongside it.
// Anchoring on "metadata" (not "annotations") ensures we only touch
// Kubernetes ObjectMeta locations such as metadata.annotations,
// spec.template.metadata.annotations and
// spec.jobTemplate.spec.template.metadata.annotations.
func (cr *checksumResolver) walk(node any) error {
	switch v := node.(type) {
	case map[string]any:
		if metaMap, ok := v["metadata"].(map[string]any); ok {
			if ann, ok := metaMap["annotations"].(map[string]any); ok {
				if raw, ok := ann[fluxcdv1.ChecksumFromAnnotation].(string); ok && raw != "" {
					sum, err := cr.resolve(raw)
					if err != nil {
						return err
					}
					ann[fluxcdv1.ChecksumAnnotation] = sum
				}
			}
		}
		for _, val := range v {
			if err := cr.walk(val); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := cr.walk(item); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolve parses a comma-separated list of ConfigMap/Secret references,
// resolves each one via data(), and returns a "sha256:<hex>" digest over
// the combined null-delimited input.
func (cr *checksumResolver) resolve(refs string) (string, error) {
	var buf bytes.Buffer
	for ref := range strings.SplitSeq(refs, ",") {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		parts := strings.Split(ref, "/")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return "", fmt.Errorf("invalid %s reference '%s' must be in the format 'Kind/namespace/name'",
				fluxcdv1.ChecksumFromAnnotation, ref)
		}
		kind, ns, name := parts[0], parts[1], parts[2]
		if kind != kindConfigMap && kind != kindSecret {
			return "", fmt.Errorf("unsupported kind '%s' in %s, only ConfigMap and Secret are allowed",
				kind, fluxcdv1.ChecksumFromAnnotation)
		}

		data, err := cr.data(kind, ns, name)
		if err != nil {
			return "", err
		}

		buf.WriteString(checksumRefKey(kind, ns, name))
		buf.WriteByte(0)
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			buf.WriteString(k)
			buf.WriteByte(0)
			buf.Write(data[k])
			buf.WriteByte(0)
		}
	}
	return digest.FromBytes(buf.Bytes()).String(), nil
}

// data returns the canonical map[string][]byte form of the referenced
// ConfigMap or Secret, pulling from the in-set rendered slice when
// available and falling back to the cluster. Results are cached so
// repeated references within a single reconcile resolve at most once.
func (cr *checksumResolver) data(kind, ns, name string) (map[string][]byte, error) {
	key := checksumRefKey(kind, ns, name)
	if d, ok := cr.canonical[key]; ok {
		return d, nil
	}
	var (
		d   map[string][]byte
		err error
	)
	if u, ok := cr.rendered[key]; ok {
		d, err = canonicalFromUnstructured(u)
	} else {
		d, err = cr.fetchCanonical(kind, ns, name)
		cr.external[key] = struct{}{}
	}
	if err != nil {
		return nil, err
	}
	cr.canonical[key] = d
	return d, nil
}

// fetchCanonical issues a cluster Get for the referenced ConfigMap or
// Secret and returns its data as a canonical map[string][]byte.
func (cr *checksumResolver) fetchCanonical(kind, ns, name string) (map[string][]byte, error) {
	nn := types.NamespacedName{Namespace: ns, Name: name}
	switch kind {
	case kindConfigMap:
		cm := &corev1.ConfigMap{}
		if err := cr.client.Get(cr.ctx, nn, cm); err != nil {
			return nil, fmt.Errorf("failed to resolve %s reference ConfigMap/%s/%s: %w",
				fluxcdv1.ChecksumFromAnnotation, ns, name, err)
		}
		out := make(map[string][]byte, len(cm.Data)+len(cm.BinaryData))
		for k, v := range cm.Data {
			out[k] = []byte(v)
		}
		maps.Copy(out, cm.BinaryData)
		return out, nil
	case kindSecret:
		secret := &corev1.Secret{}
		if err := cr.client.Get(cr.ctx, nn, secret); err != nil {
			return nil, fmt.Errorf("failed to resolve %s reference Secret/%s/%s: %w",
				fluxcdv1.ChecksumFromAnnotation, ns, name, err)
		}
		out := make(map[string][]byte, len(secret.Data))
		maps.Copy(out, secret.Data)
		return out, nil
	}
	return nil, fmt.Errorf("unsupported kind '%s' in %s", kind, fluxcdv1.ChecksumFromAnnotation)
}

// canonicalFromUnstructured decodes the unstructured form of a rendered
// ConfigMap or Secret into map[string][]byte of raw bytes.
//
// For a Secret, stringData overlays data on shared keys to mirror the
// kube-apiserver's merge behavior when the object is applied.
func canonicalFromUnstructured(u *unstructured.Unstructured) (map[string][]byte, error) {
	out := map[string][]byte{}
	switch u.GetKind() {
	case kindConfigMap:
		if data, ok, _ := unstructured.NestedStringMap(u.Object, "data"); ok {
			for k, v := range data {
				out[k] = []byte(v)
			}
		}
		if bin, ok, _ := unstructured.NestedStringMap(u.Object, "binaryData"); ok {
			for k, v := range bin {
				decoded, err := base64.StdEncoding.DecodeString(v)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 in ConfigMap/%s/%s binaryData[%q]: %w",
						u.GetNamespace(), u.GetName(), k, err)
				}
				out[k] = decoded
			}
		}
	case kindSecret:
		if data, ok, _ := unstructured.NestedStringMap(u.Object, "data"); ok {
			for k, v := range data {
				decoded, err := base64.StdEncoding.DecodeString(v)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 in Secret/%s/%s data[%q]: %w",
						u.GetNamespace(), u.GetName(), k, err)
				}
				out[k] = decoded
			}
		}
		if strData, ok, _ := unstructured.NestedStringMap(u.Object, "stringData"); ok {
			for k, v := range strData {
				out[k] = []byte(v)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported kind %q", u.GetKind())
	}
	return out, nil
}
