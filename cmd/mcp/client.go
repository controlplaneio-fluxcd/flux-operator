// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func newKubeClient() (client.Client, error) {
	cfg, err := kubeconfigArgs.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	cfg.QPS = 100.0
	cfg.Burst = 300

	restMapper, err := kubeconfigArgs.ToRESTMapper()
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

	return client.WithFieldOwner(kubeClient, "flux-operator-mcp"), nil
}
