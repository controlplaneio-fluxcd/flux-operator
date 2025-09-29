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

	"github.com/fluxcd/pkg/runtime/testenv"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	// +kubebuilder:scaffold:imports

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

var (
	controllerName = "flux-operator"
	timeout        = 5 * time.Minute
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

func getEvents(objName, objNamespace string) []corev1.Event {
	var result []corev1.Event
	events := &corev1.EventList{}
	_ = testClient.List(context.Background(), events)
	for _, event := range events.Items {
		if event.InvolvedObject.Name == objName && event.InvolvedObject.Namespace == objNamespace {
			result = append(result, event)
		}
	}
	return result
}
