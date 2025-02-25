// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	kcheck "github.com/fluxcd/pkg/runtime/conditions/check"
	"github.com/fluxcd/pkg/runtime/testenv"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/yaml"

	// +kubebuilder:scaffold:imports

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

var (
	controllerName = "flux-operator"
	timeout        = 30 * time.Second
	testEnv        *testenv.Environment
	testClient     client.Client
	testCtx        = ctrl.SetupSignalHandler()
	testKubeConfig []byte
)

func NewTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(s))
	utilruntime.Must(rbacv1.AddToScheme(s))
	utilruntime.Must(appsv1.AddToScheme(s))
	utilruntime.Must(apiextensionsv1.AddToScheme(s))
	utilruntime.Must(fluxcdv1.AddToScheme(s))
	return s
}

func TestMain(m *testing.M) {
	testEnv = testenv.New(
		testenv.WithCRDPath(
			filepath.Join("..", "..", "config", "crd", "bases"),
		),
		testenv.WithScheme(NewTestScheme()),
	)

	var err error
	testClient, err = client.New(testEnv.Config, client.Options{Scheme: NewTestScheme(), Cache: nil})
	if err != nil {
		panic(fmt.Sprintf("Failed to create test environment client: %v", err))
	}

	reporter.MustRegisterMetrics()

	go func() {
		fmt.Println("Starting the test environment")
		if err := testEnv.Start(testCtx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-testEnv.Manager.Elected()

	// Generate a kubeconfig for the testenv-admin user.
	user, err := testEnv.AddUser(envtest.User{
		Name:   "testenv-admin",
		Groups: []string{"system:masters"},
	}, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create testenv-admin user: %v", err))
	}

	kubeConfig, err := user.KubeConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to create the testenv-admin user kubeconfig: %v", err))
	}
	testKubeConfig = kubeConfig

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}

func getFluxInstanceReconciler(t *testing.T) *FluxInstanceReconciler {
	tmpDir := t.TempDir()
	err := os.WriteFile(fmt.Sprintf("%s/kubeconfig", tmpDir), testKubeConfig, 0644)
	if err != nil {
		panic(fmt.Sprintf("failed to create the testenv-admin user kubeconfig: %v", err))
	}

	// Set the kubeconfig environment variable for the impersonator.
	t.Setenv("KUBECONFIG", fmt.Sprintf("%s/kubeconfig", tmpDir))

	return &FluxInstanceReconciler{
		Client:        testClient,
		Scheme:        NewTestScheme(),
		StoragePath:   filepath.Join("..", "..", "config", "data"),
		StatusManager: controllerName,
		EventRecorder: testEnv.GetEventRecorderFor(controllerName),
	}
}

func getFluxInstanceArtifactReconciler() *FluxInstanceArtifactReconciler {
	return &FluxInstanceArtifactReconciler{
		Client:        testClient,
		EventRecorder: testEnv.GetEventRecorderFor(controllerName),
		StatusManager: controllerName,
	}
}

func logObjectStatus(t *testing.T, obj client.Object) {
	u, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	status, _, _ := unstructured.NestedFieldCopy(u, "status")
	sts, _ := yaml.Marshal(status)
	t.Log(obj.GetName(), "status:\n", string(sts))
}

func logObject(t *testing.T, obj interface{}) {
	sts, _ := yaml.Marshal(obj)
	t.Log("object:\n", string(sts))
}

func checkInstanceReadiness(g *WithT, obj *fluxcdv1.FluxInstance) {
	statusCheck := kcheck.NewInProgressChecker(testClient)
	statusCheck.DisableFetch = true
	statusCheck.WithT(g).CheckErr(context.Background(), obj)
	g.Expect(conditions.IsTrue(obj, meta.ReadyCondition)).To(BeTrue())
}

func getEvents(objName string) []corev1.Event {
	var result []corev1.Event
	events := &corev1.EventList{}
	_ = testClient.List(context.Background(), events)
	for _, event := range events.Items {
		if event.InvolvedObject.Name == objName {
			result = append(result, event)
		}
	}
	return result
}
