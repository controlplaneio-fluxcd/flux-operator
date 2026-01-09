// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"context"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/clusterreader"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/runtime/cel"
	fluxclient "github.com/fluxcd/pkg/runtime/client"
	"github.com/fluxcd/pkg/runtime/statusreaders"
	"github.com/fluxcd/pkg/ssa"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// KubeClient embeds the controller-runtime client
// and the server-side apply resource manager.
type KubeClient struct {
	client.Client

	Config  *rest.Config
	Manager *ssa.ResourceManager
}

// NewKubeClient creates a new server-side apply enabled Kubernetes client
// using the provided rest.Config. The owner parameter is used to set
// the server-side apply field manager for all applied resources.
func NewKubeClient(ctx context.Context, cfg *rest.Config, owner string) (*KubeClient, error) {
	restMapper, err := fluxclient.NewDynamicRESTMapper(cfg)
	if err != nil {
		return nil, err
	}

	kubeClient, err := client.New(cfg, client.Options{Mapper: restMapper, Scheme: NewScheme()})
	if err != nil {
		return nil, err
	}

	kubePoller, err := NewStatusPoller(ctx, kubeClient, restMapper)
	if err != nil {
		return nil, err
	}

	manager := ssa.NewResourceManager(kubeClient, kubePoller, ssa.Owner{
		Field: owner,
		Group: fluxcdv1.GroupVersion.Group,
	})

	return &KubeClient{
		Client:  kubeClient,
		Config:  cfg,
		Manager: manager,
	}, nil
}

// NewScheme returns a new runtime.Scheme with all the
// relevant types needed by the installer client.
func NewScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(apiextensionsv1.AddToScheme(s))
	utilruntime.Must(corev1.AddToScheme(s))
	utilruntime.Must(rbacv1.AddToScheme(s))
	utilruntime.Must(appsv1.AddToScheme(s))
	utilruntime.Must(fluxcdv1.AddToScheme(s))
	return s
}

// NewStatusPoller returns a polling.StatusPoller configured with
// health checks for Flux Operator custom resources.
func NewStatusPoller(ctx context.Context, reader client.Reader, mapper meta.RESTMapper) (*polling.StatusPoller, error) {
	kinds := []string{fluxcdv1.FluxInstanceKind, fluxcdv1.ResourceSetKind, fluxcdv1.ResourceSetInputProviderKind}
	healthChecks := make([]kustomize.CustomHealthCheck, 0, len(kinds))
	for _, kind := range kinds {
		healthChecks = append(healthChecks, kustomize.CustomHealthCheck{
			APIVersion: fluxcdv1.GroupVersion.String(),
			Kind:       kind,
			HealthCheckExpressions: kustomize.HealthCheckExpressions{
				Current: fluxcdv1.HealthCheckExpr,
			},
		})
	}

	statusReader, err := cel.NewStatusReader(healthChecks)
	if err != nil {
		return nil, err
	}

	readers := make([]engine.StatusReader, 0, 1+len(healthChecks))
	readers = append(readers, statusreaders.NewCustomJobStatusReader(mapper))
	if len(healthChecks) > 0 {
		readers = append(readers, statusReader(mapper))
	}

	kubePoller := polling.NewStatusPoller(reader, mapper, polling.Options{
		ClusterReaderFactory: engine.ClusterReaderFactoryFunc(clusterreader.NewDirectClusterReader),
		CustomStatusReaders:  readers,
	})

	return kubePoller, nil
}
