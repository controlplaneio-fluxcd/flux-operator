// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package kubeclient_test

import (
	"context"
	"testing"

	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	ctx         context.Context
	cancel      context.CancelFunc
	testEnv     = &envtest.Environment{}
	testEnvConf *rest.Config
	testScheme  *runtime.Scheme
	testClient  client.Client
)

func init() {
	testScheme = runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(testScheme))
	utilruntime.Must(rbacv1.AddToScheme(testScheme))
	utilruntime.Must(authzv1.AddToScheme(testScheme))
}

func TestMain(m *testing.M) {
	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	var err error
	testEnvConf, err = testEnv.Start()
	if err != nil {
		panic(err)
	}

	testClient, err = client.New(testEnvConf, client.Options{Scheme: testScheme})
	if err != nil {
		panic(err)
	}

	m.Run()

	cancel()
	err = testEnv.Stop()
	if err != nil {
		panic(err)
	}
}
