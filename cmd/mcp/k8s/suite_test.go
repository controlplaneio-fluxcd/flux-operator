// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/fluxcd/pkg/runtime/testenv"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var (
	testEnv    *testenv.Environment
	testClient *Client
	testCtx    = context.Background()
)

func NewTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(s))
	utilruntime.Must(apiextensionsv1.AddToScheme(s))
	utilruntime.Must(fluxcdv1.AddToScheme(s))
	return s
}

func TestMain(m *testing.M) {
	testEnv = testenv.New(
		testenv.WithCRDPath(
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
			filepath.Join("testdata", "crds"),
		),
		testenv.WithScheme(NewTestScheme()),
	)

	go func() {
		fmt.Println("Starting the test environment")
		if err := testEnv.Start(testCtx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-testEnv.Manager.Elected()

	httpClient, err := rest.HTTPClientFor(testEnv.Config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test environment HTTP client: %v", err))
	}
	mapper, err := apiutil.NewDynamicRESTMapper(testEnv.Config, httpClient)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test environment REST mapper: %v", err))
	}
	kubeClient, err := ctrlclient.New(testEnv.Config, ctrlclient.Options{Scheme: NewTestScheme(), Mapper: mapper})
	if err != nil {
		panic(fmt.Sprintf("Failed to create test environment client: %v", err))
	}
	testClient = NewClient(kubeClient, testEnv.Config, mapper)

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}
