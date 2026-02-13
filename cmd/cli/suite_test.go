// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"bytes"
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
)

var (
	timeout    = 30 * time.Second
	testEnv    *testenv.Environment
	testClient client.Client
	testCtx    = ctrl.SetupSignalHandler()
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

	tmpDir, err := os.MkdirTemp("", "flux-operator-cmd")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFilename := filepath.Join(tmpDir, "kubeconfig")
	if err := os.WriteFile(tmpFilename, kubeConfig, 0644); err != nil {
		panic(err)
	}
	kubeconfigArgs.KubeConfig = &tmpFilename

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}

// executeCommand executes a CLI command with the given args and returns the output and error.
// This helper function can be reused across all CLI command tests.
func executeCommand(args []string) (string, error) {
	defer resetCmdArgs()

	// Capture output
	buf := new(bytes.Buffer)

	// Set up the command
	cmd := rootCmd
	cmd.SetArgs(args)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute command
	err := cmd.Execute()

	return buf.String(), err
}

// resetCmdArgs resets all command-specific flags to their default values.
// This should be called between tests to ensure clean state.
func resetCmdArgs() {
	rootArgs.timeout = timeout
	kubeconfigArgs.Namespace = new("")

	// Version command
	versionArgs = versionFlags{}

	// Build commands
	buildInstanceArgs = buildInstanceFlags{}
	buildResourceSetArgs = buildResourceSetFlags{}

	// Get commands
	getInstanceArgs = getInstanceFlags{allNamespaces: true}
	getResourceSetArgs = getResourceSetFlags{}
	getInputProviderArgs = getInputProviderFlags{}
	getResourcesArgs = getResourcesFlags{output: "table"}

	// Reconcile commands
	reconcileInstanceArgs = reconcileInstanceFlags{}
	reconcileResourceSetArgs = reconcileResourceSetFlags{}
	reconcileInputProviderArgs = reconcileInputProviderFlags{}
	reconcileResourceArgs = reconcileResourceFlags{}

	// Resume commands
	resumeInstanceArgs = resumeInstanceFlags{}
	resumeResourceSetArgs = resumeResourceSetFlags{}
	resumeInputProviderArgs = resumeInputProviderFlags{}
	resumeResourceArgs = resumeResourceFlags{}

	// Delete commands
	deleteInstanceArgs = deleteInstanceFlags{}
	deleteResourceSetArgs = deleteResourceSetFlags{}
	deleteInputProviderArgs = deleteInputProviderFlags{}

	// Create commands
	createSecretBasicAuthArgs = createSecretBasicAuthFlags{}
	createSecretGitHubAppArgs = createSecretGitHubAppFlags{}
	createSecretProxyArgs = createSecretProxyFlags{}
	createSecretTLSArgs = createSecretTLSFlags{}
	createSecretSSHArgs = createSecretSSHFlags{}
	createSecretRegistryArgs = createSecretRegistryFlags{}
	createSecretSOPSArgs = createSecretSOPSFlags{}
	webConfigArgs = webConfigFlags{}

	// Export commands
	exportReportArgs = exportReportFlags{output: "yaml"}
	exportResourceArgs = exportResourceFlags{output: "yaml"}

	// Diff commands
	diffYAMLArgs = diffYAMLFlags{output: "json-patch-yaml"}

	// Distro commands
	distroKeygenSigArgs = distroKeygenSigFlags{}
	distroSignManifestsArgs = distroSignManifestsFlags{}
	distroVerifyManifestsArgs = distroVerifyManifestsFlags{}
	distroSignLicenseKeyArgs = distroSignLicenseKeyFlags{}
	distroVerifyLicenseKeyArgs = distroVerifyLicenseKeyFlags{}
	distroRevokeLicenseKeyArgs = distroRevokeLicenseKeyFlags{}
	distroSignArtifactsArgs = distroSignArtifactsFlags{}
	distroVerifyArtifactsArgs = distroVerifyArtifactsFlags{}
	distroEncryptTokenArgs = distroEncryptTokenFlags{}
	distroDecryptTokenArgs = distroDecryptTokenFlags{}
	distroEncryptManifestsArgs = distroEncryptManifestsFlags{
		ignore: []string{"*.jws"},
	}
	distroDecryptManifestsArgs = distroDecryptManifestsFlags{
		outputPath: ".",
		overwrite:  false,
	}
}
