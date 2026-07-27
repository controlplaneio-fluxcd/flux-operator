// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleInstallFluxInstance(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		kubeClient: k8s.NewClientFactory(cli.NewConfigFlags(false)),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "install_flux_instance",
		},
	}

	tests := []struct {
		testName  string
		arguments map[string]any
		matchErr  string
	}{
		{
			testName: "fails without instance_url",
			arguments: map[string]any{
				"instance_url": "",
			},
			matchErr: "The instance URL cannot be empty",
		},
		{
			testName: "fails with invalid timeout",
			arguments: map[string]any{
				"instance_url": "https://example.com/instance.yaml",
				"timeout":      "invalid",
			},
			matchErr: "The timeout is not a valid duration",
		},
		{
			testName: "fails with invalid kubeconfig",
			arguments: map[string]any{
				"instance_url": "https://example.com/instance.yaml",
			},
			matchErr: "failed to fetch instance manifest",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input installFluxInstanceInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, content, err := m.HandleInstallFluxInstance(context.Background(), request, input)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())

			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			_ = content
		})
	}
}

func TestFetchOperatorManifest_DigestPinnedHandoff(t *testing.T) {
	g := NewWithT(t)
	const (
		mutableRef = "oci://ghcr.io/example/manifests:latest"
		pinnedRef  = "oci://ghcr.io/example/manifests@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)
	instance := &fluxcdv1.FluxInstance{
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{Artifact: mutableRef},
		},
	}

	originalResolve := resolveMCPInstallArtifact
	originalDownload := downloadMCPInstallArtifact
	defer func() {
		resolveMCPInstallArtifact = originalResolve
		downloadMCPInstallArtifact = originalDownload
	}()

	resolveMCPInstallArtifact = func(_ context.Context, ref string, _ authn.Keychain) (string, error) {
		g.Expect(ref).To(Equal(mutableRef))
		return pinnedRef, nil
	}
	downloadMCPInstallArtifact = func(_ context.Context, ref, path string, _ authn.Keychain) ([]byte, error) {
		g.Expect(ref).To(Equal(pinnedRef))
		g.Expect(path).To(Equal("flux-operator/install.yaml"))
		return []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: flux-system\n"), nil
	}

	objects, err := (&Manager{}).fetchOperatorManifest(context.Background(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(operatorArtifactURL(instance)).To(Equal(mutableRef))
}

func TestFetchOperatorManifest_ResolveFailurePreventsDownload(t *testing.T) {
	g := NewWithT(t)
	instance := &fluxcdv1.FluxInstance{}

	originalResolve := resolveMCPInstallArtifact
	originalDownload := downloadMCPInstallArtifact
	defer func() {
		resolveMCPInstallArtifact = originalResolve
		downloadMCPInstallArtifact = originalDownload
	}()

	downloadCalled := false
	resolveMCPInstallArtifact = func(context.Context, string, authn.Keychain) (string, error) {
		return "", errors.New("digest resolution failed")
	}
	downloadMCPInstallArtifact = func(context.Context, string, string, authn.Keychain) ([]byte, error) {
		downloadCalled = true
		return nil, nil
	}

	_, err := (&Manager{}).fetchOperatorManifest(context.Background(), instance)
	g.Expect(err).To(MatchError(ContainSubstring("digest resolution failed")))
	g.Expect(downloadCalled).To(BeFalse())
}
