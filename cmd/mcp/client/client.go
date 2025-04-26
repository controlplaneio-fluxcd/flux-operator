// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package client

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

type KubeClient struct {
	client.Client
}

func NewClient(flags *cli.ConfigFlags) (*KubeClient, error) {
	cfg, err := flags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	cfg.QPS = 100.0
	cfg.Burst = 300

	restMapper, err := flags.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	scheme := apiruntime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := fluxcdv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	kubeClient, err := client.New(cfg, client.Options{Mapper: restMapper, Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return &KubeClient{Client: client.WithFieldOwner(kubeClient, "flux-operator-mcp")}, nil
}

// ParseGroupVersionKind parses the provided apiVersion and kind into a GroupVersionKind object.
func (k *KubeClient) ParseGroupVersionKind(apiVersion, kind string) (schema.GroupVersionKind, error) {
	var gvk schema.GroupVersionKind
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return gvk, err
	}

	if kind == "" {
		return gvk, errors.New("kind not specified")
	}

	gvk = schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	}
	return gvk, nil
}

// ObjectKeyFromObject returns the ObjectKey given a runtime.Object.
func (k *KubeClient) ObjectKeyFromObject(obj client.Object) client.ObjectKey {
	return client.ObjectKeyFromObject(obj)
}
