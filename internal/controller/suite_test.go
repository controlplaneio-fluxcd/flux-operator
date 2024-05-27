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
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	// +kubebuilder:scaffold:imports

	fluxcdv1alpha1 "github.com/controlplaneio-fluxcd/fluxcd-operator/api/v1alpha1"
)

var (
	controllerName = "fluxcd-operator"
	timeout        = 30 * time.Second
	testEnv        *testenv.Environment
	testClient     client.Client
	testCtx        = ctrl.SetupSignalHandler()
)

func NewTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(s))
	utilruntime.Must(apiextensionsv1.AddToScheme(s))
	utilruntime.Must(fluxcdv1alpha1.AddToScheme(s))
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
	testClient, err = client.New(testEnv.Config, client.Options{Scheme: NewTestScheme()})
	if err != nil {
		panic(fmt.Sprintf("Failed to create test environment client: %v", err))
	}

	go func() {
		fmt.Println("Starting the test environment")
		if err := testEnv.Start(testCtx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-testEnv.Manager.Elected()

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}

func getFluxInstanceReconciler() *FluxInstanceReconciler {
	return &FluxInstanceReconciler{
		Client:        testClient,
		Scheme:        NewTestScheme(),
		StatusManager: controllerName,
		EventRecorder: testEnv.GetEventRecorderFor(controllerName),
	}
}

func logObjectStatus(t *testing.T, obj client.Object) {
	u, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	status, _, _ := unstructured.NestedFieldCopy(u, "status")
	sts, _ := yaml.Marshal(status)
	t.Log(obj.GetName(), "status:\n", string(sts))
}

func checkInstanceReadiness(g *gomega.WithT, obj *fluxcdv1alpha1.FluxInstance) {
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
