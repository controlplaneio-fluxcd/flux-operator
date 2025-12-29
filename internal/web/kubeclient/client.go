// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package kubeclient

import (
	"context"
	"fmt"
	"slices"
	"time"

	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/fluxcd/pkg/cache"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// Client exposes RBAC-aware methods to
// talk to the Kubernetes API server.
type Client struct {
	reader                 client.Reader
	client                 client.Client
	config                 *rest.Config
	scheme                 *runtime.Scheme
	userClientCache        *cache.LRU[*userClient]
	userNamespacesCache    *cache.LRU[*userNamespaces]
	namespaceCacheDuration time.Duration
}

// userClient is a Kubernetes API client scoped
// to a specific user for RBAC impersonation.
type userClient struct {
	reader client.Reader
	client client.Client
	config *rest.Config
}

// userNamespaces holds a list of namespaces along
// with the timestamp they were cached at.
type userNamespaces struct {
	namespaces    []string
	timestamp     time.Time
	allNamespaces bool
}

// Option defines a functional option for calling the
// Client methods.
type Option func(*options)

type options struct {
	withPrivileges bool
}

// WithPrivileges is a ClientOption that indicates
// the Client method should use a privileged client
// to talk to the Kubernetes API server.
func WithPrivileges() Option {
	return func(o *options) {
		o.withPrivileges = true
	}
}

// New returns a new Client wrapping the given cluster.Cluster.
func New(c cluster.Cluster, userCacheSize int, namespaceCacheDuration time.Duration) (*Client, error) {
	userClientCache, err := cache.NewLRU[*userClient](userCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create user client cache: %w", err)
	}

	userNamespacesCache, err := cache.NewLRU[*userNamespaces](userCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create user namespace cache: %w", err)
	}

	return &Client{
		reader:                 c.GetAPIReader(),
		client:                 c.GetClient(),
		config:                 c.GetConfig(),
		scheme:                 c.GetScheme(),
		userClientCache:        userClientCache,
		userNamespacesCache:    userNamespacesCache,
		namespaceCacheDuration: namespaceCacheDuration,
	}, nil
}

// GetScheme returns the client's scheme.
func (c *Client) GetScheme() *runtime.Scheme {
	return c.scheme
}

// GetAPIReader returns a client.Reader that will be configured to hit the API server directly.
func (c *Client) GetAPIReader(ctx context.Context, opts ...Option) client.Reader {
	return c.getUserClientFromContext(ctx, opts...).reader
}

// GetClient returns a client.Client that will be configured with a cache for reads.
func (c *Client) GetClient(ctx context.Context, opts ...Option) client.Client {
	return c.getUserClientFromContext(ctx, opts...).client
}

// GetConfig returns a *rest.Config for creating specialized clients like *metricsclientset.Clientset.
func (c *Client) GetConfig(ctx context.Context, opts ...Option) *rest.Config {
	return c.getUserClientFromContext(ctx, opts...).config
}

// getUserClientFromContext returns a userClient based on the context and options.
func (c *Client) getUserClientFromContext(ctx context.Context, opts ...Option) *userClient {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if uc := user.KubeClient(ctx); uc != nil && !o.withPrivileges {
		return uc.(*userClient)
	}

	return &userClient{
		reader: c.reader,
		client: c.client,
		config: c.config,
	}
}

// GetUserClientFromCache retrieves a userClient from the cache or creates and caches a new one.
func (c *Client) GetUserClientFromCache(imp user.Impersonation) (*userClient, error) {
	ctx := context.Background() // fetch does not use the context
	key := user.Key(imp)
	condition := func(*userClient) bool { return true } // always valid
	fetch := func(context.Context) (*userClient, error) { return c.newUserClient(imp) }
	uc, _, err := c.userClientCache.GetIfOrSet(ctx, key, condition, fetch)
	return uc, err
}

// newUserClient creates a new userClient for the given username and groups.
func (c *Client) newUserClient(imp user.Impersonation) (*userClient, error) {
	// Create user impersonated REST kubeConfig.
	kubeConfig := rest.CopyConfig(c.config)
	kubeConfig.Impersonate = rest.ImpersonationConfig{
		UserName: imp.Username,
		Groups:   imp.Groups,
	}

	// Create user HTTP client.
	httpClient, err := rest.HTTPClientFor(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create user HTTP client: %w", err)
	}

	// Create user REST mapper.
	restMapper, err := apiutil.NewDynamicRESTMapper(kubeConfig, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create user REST mapper: %w", err)
	}

	// Create userreader without cache.
	kubeReader, err := client.New(kubeConfig, client.Options{
		HTTPClient: httpClient,
		Scheme:     c.scheme,
		Mapper:     restMapper,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user reader: %w", err)
	}

	// Create user kubeClient with cache, excluding Secrets and ConfigMaps.
	kubeClient, err := client.New(kubeConfig, client.Options{
		HTTPClient: httpClient,
		Scheme:     c.scheme,
		Mapper:     restMapper,
		Cache: &client.CacheOptions{
			DisableFor: []client.Object{&corev1.Secret{}, &corev1.ConfigMap{}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user client: %w", err)
	}

	return &userClient{
		reader: kubeReader,
		client: kubeClient,
		config: kubeConfig,
	}, nil
}

// ListUserNamespaces lists the namespaces the user has access to and returns their names sorted
// in alphabetical order. Since this operation is expensive, it has a cache per user.
// The boolean return value indicates whether the user has access to all namespaces in the cluster.
func (c *Client) ListUserNamespaces(ctx context.Context) ([]string, bool, error) {
	key := user.LoadSession(ctx).Key()

	fetch := func(ctx context.Context) (*userNamespaces, error) {
		// List and sort all namespaces.
		var namespaceList corev1.NamespaceList
		if err := c.client.List(ctx, &namespaceList); err != nil {
			return nil, err
		}
		namespaces := make([]string, 0, len(namespaceList.Items))
		for _, ns := range namespaceList.Items {
			namespaces = append(namespaces, ns.Name)
		}
		slices.Sort(namespaces)

		// Filter namespaces by access.
		namespaces, allNamespaces, err := c.filterNamespacesByAccess(ctx, namespaces)
		if err != nil {
			return nil, err
		}

		return &userNamespaces{
			namespaces:    namespaces,
			allNamespaces: allNamespaces,
			timestamp:     time.Now(),
		}, nil
	}

	// Here we explicitly implement the cache-aside pattern because GetIfOrSet is
	// atomic and hence would block concurrent requests. The fetch logic here is
	// expensive so we need to allow concurrent fetches.
	un, err := c.userNamespacesCache.Get(key)
	if err == nil && time.Since(un.timestamp) < c.namespaceCacheDuration {
		return un.namespaces, un.allNamespaces, nil
	}
	un, err = fetch(ctx)
	if err != nil {
		return nil, false, err
	}
	_ = c.userNamespacesCache.Set(key, un) // Set() does not return errors.

	return un.namespaces, un.allNamespaces, nil
}

// filterNamespacesByAccess filters the given list of namespaces
// and returns only those the user has access to. It first checks
// for cluster-wide access to avoid per-namespace checks when possible.
// Access is determined by performing a SelfSubjectAccessReview
// checking the "get" verb on the "resourcesets.fluxcd.controlplane.io" resource.
// The boolean return value indicates whether the user has access to all namespaces.
func (c *Client) filterNamespacesByAccess(ctx context.Context, namespaces []string) ([]string, bool, error) {
	kubeClient := c.GetClient(ctx)
	if kubeClient == c.client {
		// Privileged client has access to all namespaces.
		return namespaces, true, nil
	}

	// Look up the plural for ResourceSet from FluxOperatorKinds.
	var resourceSetPlural string
	for _, kind := range fluxcdv1.FluxOperatorKinds {
		if kind.Name == fluxcdv1.ResourceSetKind {
			resourceSetPlural = kind.Plural
			break
		}
	}

	// Check for cluster-wide access first in case the user has a ClusterRoleBinding.
	clusterSSAR := &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authzv1.ResourceAttributes{
				Verb:     "get",
				Group:    fluxcdv1.GroupVersion.Group,
				Resource: resourceSetPlural,
			},
		},
	}
	if err := kubeClient.Create(ctx, clusterSSAR); err != nil {
		return nil, false, fmt.Errorf("failed to create cluster-wide SelfSubjectAccessReview: %w", err)
	}
	if clusterSSAR.Status.Allowed {
		return namespaces, true, nil
	}

	// Check access per namespace, the user probably has at least one RoleBinding.
	filteredNamespaces := make([]string, 0)
	for _, ns := range namespaces {
		ssar := &authzv1.SelfSubjectAccessReview{
			Spec: authzv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authzv1.ResourceAttributes{
					Verb:      "get",
					Group:     fluxcdv1.GroupVersion.Group,
					Resource:  resourceSetPlural,
					Namespace: ns,
				},
			},
		}

		if err := kubeClient.Create(ctx, ssar); err != nil {
			return nil, false, fmt.Errorf("failed to create SelfSubjectAccessReview for namespace %s: %w", ns, err)
		}

		if ssar.Status.Allowed {
			filteredNamespaces = append(filteredNamespaces, ns)
		}
	}

	return filteredNamespaces, false, nil
}

// CanPatchResource checks if the user has permission to patch a resource
// by performing a SelfSubjectAccessReview with the "patch" verb.
func (c *Client) CanPatchResource(ctx context.Context, group, resource, namespace, name string) (bool, error) {
	kubeClient := c.GetClient(ctx)
	if kubeClient == c.client {
		// Privileged client has access to all resources.
		return true, nil
	}

	ssar := &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authzv1.ResourceAttributes{
				Verb:      "patch",
				Group:     group,
				Resource:  resource,
				Namespace: namespace,
				Name:      name,
			},
		},
	}

	if err := kubeClient.Create(ctx, ssar); err != nil {
		return false, fmt.Errorf("failed to create SelfSubjectAccessReview: %w", err)
	}

	return ssar.Status.Allowed, nil
}
