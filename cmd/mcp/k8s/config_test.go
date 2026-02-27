// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func writeTestKubeconfig(t *testing.T, config clientcmdapi.Config) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	data, err := clientcmd.Write(config)
	if err != nil {
		t.Fatalf("failed to serialize kubeconfig: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}
	return path
}

func TestKubeConfigLoad(t *testing.T) {
	tests := []struct {
		testName   string
		config     *clientcmdapi.Config
		setEnv     bool
		matchErr   string
		matchLen   int
		currentCtx string
	}{
		{
			testName: "loads contexts from kubeconfig",
			config: &clientcmdapi.Config{
				CurrentContext: "production",
				Clusters: map[string]*clientcmdapi.Cluster{
					"prod-cluster": {Server: "https://prod:6443"},
					"dev-cluster":  {Server: "https://dev:6443"},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"production":  {Cluster: "prod-cluster", AuthInfo: "default"},
					"development": {Cluster: "dev-cluster", AuthInfo: "default"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"default": {},
				},
			},
			setEnv:     true,
			matchLen:   2,
			currentCtx: "production",
		},
		{
			testName: "fails without KUBECONFIG env",
			setEnv:   false,
			matchErr: "KUBECONFIG environment variable not set",
		},
		{
			testName: "loads single context",
			config: &clientcmdapi.Config{
				CurrentContext: "staging",
				Clusters: map[string]*clientcmdapi.Cluster{
					"staging-cluster": {Server: "https://staging:6443"},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"staging": {Cluster: "staging-cluster", AuthInfo: "default"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"default": {},
				},
			},
			setEnv:     true,
			matchLen:   1,
			currentCtx: "staging",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			// Save and restore KUBECONFIG
			if tt.setEnv && tt.config != nil {
				path := writeTestKubeconfig(t, *tt.config)
				t.Setenv("KUBECONFIG", path)
			} else {
				t.Setenv("KUBECONFIG", "")
			}

			kc := NewKubeConfig()
			err := kc.Load()

			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				contexts := kc.Contexts()
				g.Expect(contexts).To(HaveLen(tt.matchLen))

				// Verify current context is set
				found := false
				for _, ctx := range contexts {
					if ctx.CurrentContext {
						g.Expect(ctx.ContextName).To(Equal(tt.currentCtx))
						found = true
					}
				}
				g.Expect(found).To(BeTrue())
			}
		})
	}
}

func TestKubeConfigLoadPreservesCurrentContext(t *testing.T) {
	g := NewWithT(t)

	config := clientcmdapi.Config{
		CurrentContext: "production",
		Clusters: map[string]*clientcmdapi.Cluster{
			"prod-cluster": {Server: "https://prod:6443"},
			"dev-cluster":  {Server: "https://dev:6443"},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"production":  {Cluster: "prod-cluster", AuthInfo: "default"},
			"development": {Cluster: "dev-cluster", AuthInfo: "default"},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"default": {},
		},
	}

	path := writeTestKubeconfig(t, config)
	t.Setenv("KUBECONFIG", path)

	kc := NewKubeConfig()

	// First load picks up "production" as current
	err := kc.Load()
	g.Expect(err).NotTo(HaveOccurred())

	// Switch to "development"
	err = kc.SetCurrentContext("development")
	g.Expect(err).NotTo(HaveOccurred())

	// Reload should preserve "development" as current
	err = kc.Load()
	g.Expect(err).NotTo(HaveOccurred())

	contexts := kc.Contexts()
	for _, ctx := range contexts {
		if ctx.CurrentContext {
			g.Expect(ctx.ContextName).To(Equal("development"))
		}
	}
}

func TestKubeConfigSetCurrentContext(t *testing.T) {
	tests := []struct {
		testName string
		target   string
		matchErr string
	}{
		{
			testName: "sets existing context",
			target:   "development",
		},
		{
			testName: "fails with non-existent context",
			target:   "non-existent",
			matchErr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			kc := NewKubeConfig()
			kc.contexts = []KubeConfigContext{
				{ContextName: "production", ClusterName: "prod-cluster", CurrentContext: true},
				{ContextName: "development", ClusterName: "dev-cluster", CurrentContext: false},
			}

			err := kc.SetCurrentContext(tt.target)

			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				contexts := kc.Contexts()
				for _, ctx := range contexts {
					if ctx.ContextName == tt.target {
						g.Expect(ctx.CurrentContext).To(BeTrue())
					} else {
						g.Expect(ctx.CurrentContext).To(BeFalse())
					}
				}
			}
		})
	}
}

func TestKubeConfigContextsSorted(t *testing.T) {
	g := NewWithT(t)

	kc := NewKubeConfig()
	kc.contexts = []KubeConfigContext{
		{ContextName: "zebra", ClusterName: "z-cluster"},
		{ContextName: "alpha", ClusterName: "a-cluster"},
		{ContextName: "middle", ClusterName: "m-cluster"},
	}

	contexts := kc.Contexts()
	g.Expect(contexts).To(HaveLen(3))
	g.Expect(contexts[0].ContextName).To(Equal("alpha"))
	g.Expect(contexts[1].ContextName).To(Equal("middle"))
	g.Expect(contexts[2].ContextName).To(Equal("zebra"))
}
