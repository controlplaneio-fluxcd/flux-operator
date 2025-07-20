// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var traceCmd = &cobra.Command{
	Use:   "trace [kind/name]",
	Short: "Trace in-cluster objects throughout the GitOps delivery pipeline",
	Example: `  # Trace a Kubernetes Deployment
  flux-operator -n flux-system trace deploy/source-controller

  # Trace a Kubernetes Pod
  flux-operator -n redis trace pod/redis-master-0

  # Trace a Kubernetes Namespace
  flux-operator trace ns/flux-system

  # Trace a Kubernetes custom resource
  flux-operator -n flux-system trace ResourceSet/apps
`,
	RunE: traceCmdRun,
}

func init() {
	rootCmd.AddCommand(traceCmd)
}

func traceCmdRun(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	obj, err := getObjectByKindName(args)
	if err != nil {
		return err
	}

	managedObj := NewFluxManagedObject(obj)

	reconciler, err := getFluxReconcilerFromOwner(ctx, kubeClient, obj)
	if err != nil {
		return fmt.Errorf("failed to trace '%s/%s': %w", obj.GetKind(), obj.GetName(), err)
	}

	err = managedObj.Compute(ctx, kubeClient, reconciler)
	if err != nil {
		return fmt.Errorf("failed to trace '%s/%s': %w", obj.GetKind(), obj.GetName(), err)
	}

	// Build template from the managed object and print the trace
	tmpl, err := template.New("trace").Parse(traceTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	err = tmpl.Execute(cmd.OutOrStdout(), managedObj)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// getFluxReconciler retrieves the Flux reconciler information
// from the labels of the provided unstructured object.
func getFluxReconciler(obj *unstructured.Unstructured) (*FluxManagedObjectReconciler, error) {
	manager := &FluxManagedObjectReconciler{}

	for k, v := range obj.GetLabels() {
		if k == "app.kubernetes.io/managed-by" && v == "flux-operator" {
			manager.Kind = "FluxInstance"
			manager.Name = obj.GetLabels()["fluxcd.controlplane.io/name"]
			manager.Namespace = obj.GetLabels()["fluxcd.controlplane.io/namespace"]
			break
		}

		if !strings.Contains(k, "fluxcd.") {
			continue
		}

		if strings.HasSuffix(k, "/name") {
			parts := strings.Split(k, ".")
			if len(parts) > 0 {
				switch parts[0] {
				case "kustomize":
					manager.Kind = "Kustomization"
				case "helm":
					manager.Kind = "HelmRelease"
				case "resourceset":
					manager.Kind = "ResourceSet"
				}
			}
			manager.Name = v
		} else if strings.HasSuffix(k, "/namespace") {
			manager.Namespace = v
		}
	}

	if manager.Kind == "" || manager.Name == "" || manager.Namespace == "" {
		return nil, errors.New("object not managed by Flux")
	}
	return manager, nil
}

// getFluxReconcilerFromOwner recursively retrieves the Flux manager
// from the owner references of an object.
func getFluxReconcilerFromOwner(ctx context.Context, kubeClient client.Client, obj *unstructured.Unstructured) (*FluxManagedObjectReconciler, error) {
	if m, err := getFluxReconciler(obj); err == nil {
		return m, nil
	}

	for _, reference := range obj.GetOwnerReferences() {
		owner := &unstructured.Unstructured{}
		gv, err := schema.ParseGroupVersion(reference.APIVersion)
		if err != nil {
			return nil, err
		}

		owner.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gv.Group,
			Version: gv.Version,
			Kind:    reference.Kind,
		})

		ownerName := types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      reference.Name,
		}

		err = kubeClient.Get(ctx, ownerName, owner)
		if err != nil {
			return nil, err
		}

		if m, err := getFluxReconciler(owner); err == nil {
			return m, nil
		}

		if len(owner.GetOwnerReferences()) > 0 {
			return getFluxReconcilerFromOwner(ctx, kubeClient, owner)
		}
	}

	return nil, errors.New("object not managed by Flux")
}

var traceTmpl = `Object:          {{.ID}}
{{- if .Reconciler }}
{{- if eq .Reconciler.Ready "False" }}
Status:          Last reconciliation failed at {{.Reconciler.LastReconciled}}
{{- else }}
Status:          Last reconciled at {{.Reconciler.LastReconciled}}
{{- end }}
Revision:        {{.Reconciler.Revision}}
Reconciler:      {{.Reconciler.Kind}}/{{.Reconciler.Namespace}}/{{.Reconciler.Name}}
{{- if .Source }}
Source:          {{.Source.Kind}}/{{.Source.Namespace}}/{{.Source.Name}}
Source URL:      {{.Source.URL}}
{{- if .Source.OriginURL }}
Origin URL:      {{.Source.OriginURL}}
{{- end }}
{{- if .Source.OriginRevision }}
Origin Revision: {{.Source.OriginRevision}}
{{- end }}
{{- end }}
Message:         {{.Reconciler.ReadyMessage}}
{{- end }}
`
