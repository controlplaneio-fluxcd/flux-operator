// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	authzv1 "k8s.io/api/authorization/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
)

var (
	ctx             context.Context
	cancel          context.CancelFunc
	testEnv         *envtest.Environment
	testScheme      *runtime.Scheme
	testClient      client.Client
	testCluster     cluster.Cluster
	kubeClient      *kubeclient.Client
	kubeClientCache *kubeclient.Client
)

func NewTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(s))
	utilruntime.Must(rbacv1.AddToScheme(s))
	utilruntime.Must(appsv1.AddToScheme(s))
	utilruntime.Must(batchv1.AddToScheme(s))
	utilruntime.Must(authzv1.AddToScheme(s))
	utilruntime.Must(fluxcdv1.AddToScheme(s))
	return s
}

// setupActionRBAC creates RBAC that grants custom action verbs (reconcile, suspend, resume)
// to all authenticated users. This is needed for tests that use the privileged context.
func setupActionRBAC(ctx context.Context, c client.Client) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-action-verbs",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"*"},
				Verbs:     []string{"reconcile", "suspend", "resume", "download"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"cronjobs", "jobs"},
				Verbs:     []string{"get", "list", "create", "restart"},
			},
		},
	}
	if err := c.Create(ctx, clusterRole); err != nil {
		return fmt.Errorf("failed to create ClusterRole: %w", err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-action-verbs-binding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				// Only grant to system:masters (the admin group used by the privileged test context)
				// This ensures user session tests with custom RBAC are not affected
				Kind: "Group",
				Name: "system:masters",
			},
		},
	}
	if err := c.Create(ctx, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create ClusterRoleBinding: %w", err)
	}

	return nil
}

func TestMain(m *testing.M) {
	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	testScheme = NewTestScheme()

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(fmt.Sprintf("Failed to start test environment: %v", err))
	}

	testClient, err = client.New(cfg, client.Options{Scheme: testScheme})
	if err != nil {
		panic(fmt.Sprintf("Failed to create test client: %v", err))
	}

	testCluster, err = cluster.New(cfg, func(o *cluster.Options) {
		o.Scheme = testScheme
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create test cluster: %v", err))
	}

	// Index Jobs by CronJob owner for the workload handler tests
	if err := testCluster.GetFieldIndexer().IndexField(ctx, &batchv1.Job{}, JobOwnerCronJobField, func(obj client.Object) []string {
		job := obj.(*batchv1.Job)
		for _, ref := range job.OwnerReferences {
			if ref.APIVersion == "batch/v1" && ref.Kind == "CronJob" {
				return []string{ref.Name}
			}
		}
		return nil
	}); err != nil {
		panic(fmt.Sprintf("Failed to create field index for Jobs: %v", err))
	}

	// Start the cluster in a goroutine
	go func() {
		if err := testCluster.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start test cluster: %v", err))
		}
	}()

	// Wait for the cache to sync
	syncCtx, syncCancel := context.WithTimeout(ctx, 30*time.Second)
	if !testCluster.GetCache().WaitForCacheSync(syncCtx) {
		panic("Failed to sync test cluster cache")
	}
	syncCancel()

	// Create the kubeclient
	kubeClient, err = kubeclient.New(testClient, testClient, cfg, testScheme, 100, 5*time.Minute)
	if err != nil {
		panic(fmt.Sprintf("Failed to create kubeclient: %v", err))
	}

	// Create a kubeclient with the manager cache
	kubeClientCache, err = kubeclient.New(testClient, testCluster.GetClient(), cfg, testScheme, 100, 5*time.Minute)
	if err != nil {
		panic(fmt.Sprintf("Failed to create kubeclient with cache: %v", err))
	}

	// Create RBAC for custom action verbs (reconcile, suspend, resume)
	// This is needed because the action handler now checks custom RBAC verbs
	if err := setupActionRBAC(ctx, testClient); err != nil {
		panic(fmt.Sprintf("Failed to setup action RBAC: %v", err))
	}

	// Register metrics (needed for reporter)
	reporter.MustRegisterMetrics()

	code := m.Run()

	cancel()
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop test environment: %v", err))
	}

	os.Exit(code)
}
