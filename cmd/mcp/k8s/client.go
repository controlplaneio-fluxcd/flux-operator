// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"errors"
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/pkg/ssa"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
)

// Client embeds the controller-runtime client to provide
// extended functionality for interacting with Kubernetes resources.
type Client struct {
	ctrlclient.Client
	cfg *rest.Config
	rm  *ssa.ResourceManager
}

// NewClient creates a new Kubernetes client using the provided cli.ConfigFlags,
// configuring QPS, Burst, and custom schemes.
func NewClient(ctx context.Context, flags *cli.ConfigFlags) (*Client, error) {
	cfg, err := flags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	if sess := auth.FromContext(ctx); sess != nil {
		cfg.Impersonate = rest.ImpersonationConfig{
			UserName: sess.UserName,
			Groups:   sess.Groups,
		}
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

	kubeClient, err := ctrlclient.New(cfg, ctrlclient.Options{Mapper: restMapper, Scheme: scheme})
	if err != nil {
		return nil, err
	}

	kubePoller := polling.NewStatusPoller(kubeClient, restMapper, polling.Options{})
	rm := ssa.NewResourceManager(kubeClient, kubePoller, ssa.Owner{
		Field: "kubectl-flux-mcp",
		Group: fluxcdv1.GroupVersion.Group,
	})

	return &Client{
		Client: ctrlclient.WithFieldOwner(kubeClient, "flux-operator-mcp"),
		cfg:    cfg,
		rm:     rm,
	}, nil
}

// ParseGroupVersionKind parses the provided apiVersion and kind into a GroupVersionKind object.
func (k *Client) ParseGroupVersionKind(apiVersion, kind string) (schema.GroupVersionKind, error) {
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
func (k *Client) ObjectKeyFromObject(obj ctrlclient.Object) ctrlclient.ObjectKey {
	return ctrlclient.ObjectKeyFromObject(obj)
}
