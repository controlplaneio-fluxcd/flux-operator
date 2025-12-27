// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/pkg/ssa"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var (
	// Scheme is the global scheme for Kubernetes resources required by the MCP server.
	Scheme = apiruntime.NewScheme()
)

func init() {
	_ = corev1.AddToScheme(Scheme)
	_ = apiextensionsv1.AddToScheme(Scheme)
	_ = fluxcdv1.AddToScheme(Scheme)
}

// Client embeds the controller-runtime client to provide
// extended functionality for interacting with Kubernetes resources.
type Client struct {
	ctrlclient.Client
	cfg *rest.Config
	rm  *ssa.ResourceManager
}

// NewClient creates a new Kubernetes client using the provided objects.
func NewClient(kubeClient ctrlclient.Client, cfg *rest.Config, restMapper meta.RESTMapper) *Client {
	return &Client{
		Client: kubeClient,
		cfg:    cfg,
		rm:     newResourceManager(kubeClient, restMapper),
	}
}

// newClientFromFlags creates a new Kubernetes client using the provided cli.ConfigFlags,
// configuring QPS, Burst, and custom schemes.
func newClientFromFlags(flags *cli.ConfigFlags) (*Client, error) {
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

	kubeClient, err := ctrlclient.New(cfg, ctrlclient.Options{Mapper: restMapper, Scheme: Scheme})
	if err != nil {
		return nil, err
	}

	return &Client{
		Client: ctrlclient.WithFieldOwner(kubeClient, "flux-operator-mcp"),
		cfg:    cfg,
		rm:     newResourceManager(kubeClient, restMapper),
	}, nil
}

func newResourceManager(kubeClient ctrlclient.Client, restMapper meta.RESTMapper) *ssa.ResourceManager {
	kubePoller := polling.NewStatusPoller(kubeClient, restMapper, polling.Options{})
	return ssa.NewResourceManager(kubeClient, kubePoller, ssa.Owner{
		Field: "kubectl-flux-mcp",
		Group: fluxcdv1.GroupVersion.Group,
	})
}

// GetConfig returns the REST config used by the Kubernetes client.
func (k *Client) GetConfig() *rest.Config {
	return rest.CopyConfig(k.cfg)
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

// IntoContext adds the Kubernetes client to the provided context.Context.
func (k *Client) IntoContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, clientContextKey{}, k)
}

// clientContextKey is the context key for storing and retrieving the Kubernetes client
// from a context.Context.
type clientContextKey struct{}

// ClientFactory provides an interface to get Kubernetes clients
// and setting the current context.
type ClientFactory struct {
	flags *cli.ConfigFlags
	mu    sync.RWMutex
}

// NewClientFactory creates a new ClientFactory with the given cli.ConfigFlags.
func NewClientFactory(flags *cli.ConfigFlags) *ClientFactory {
	return &ClientFactory{
		flags: flags,
	}
}

// GetClient creates and returns a new Kubernetes client.
func (f *ClientFactory) GetClient(ctx context.Context) (*Client, error) {
	if v := ctx.Value(clientContextKey{}); v != nil {
		return v.(*Client), nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	return newClientFromFlags(f.flags)
}

// SetCurrentContext sets the current context in the kubeconfig to the specified context name.
func (f *ClientFactory) SetCurrentContext(contextName string) {
	f.mu.Lock()
	f.flags.Context = &contextName
	f.mu.Unlock()
}
