// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
)

func TestRunServer_ConfigWatcher(t *testing.T) {
	g := NewWithT(t)

	// Create a cancellable context for the server.
	serverCtx, cancelServer := context.WithCancel(ctx)
	defer cancelServer()

	// Create the test namespace for the config secret.
	testNamespace := "test-server-config-ns"
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	g.Expect(testClient.Create(ctx, ns)).To(Succeed())
	defer testClient.Delete(ctx, ns)

	// Create the initial config secret without authentication configured.
	secretName := "test-web-config"
	initialConfigYAML := `apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec: {}
`
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"config.yaml": []byte(initialConfigYAML),
		},
	}
	g.Expect(testClient.Create(ctx, secret)).To(Succeed())
	defer testClient.Delete(ctx, secret)

	// Inject a logger into the context for the watcher.
	l := logr.Discard()
	watcherCtx := log.IntoContext(serverCtx, l)

	// Start watching the config secret.
	confChannel, confWatcherStopped, firstConf, err := config.WatchSecret(
		watcherCtx, secretName, testNamespace, testEnv.Config)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(firstConf).NotTo(BeNil())
	g.Expect(firstConf.Authentication).To(BeNil(), "initial config should not have authentication")

	// Initialize server components with the first configuration.
	firstComponents, err := InitializeServerComponents(firstConf, testCluster, l)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(firstComponents).NotTo(BeNil())

	// Create a listener on a random port.
	lis, err := net.Listen("tcp", ":0")
	g.Expect(err).NotTo(HaveOccurred())
	defer lis.Close()

	// Get the random port.
	addr := lis.Addr().(*net.TCPAddr)
	port := fmt.Sprintf("%d", addr.Port)

	// Start the server in a goroutine.
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- RunServer(serverCtx,
			testCluster,
			confChannel,
			"test-version",
			"test-status-manager",
			testNamespace,
			firstComponents,
			lis,
			l)
	}()

	// Wait for the server to start.
	g.Eventually(func() error {
		_, err := http.Get("http://localhost:" + port + "/")
		return err
	}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

	// Make an HTTP call to /oauth2/authorize - should return 200
	// (serving index page) because OAuth2 is not configured.
	// The "/" handler is a catch-all that serves index.html.
	resp, err := http.Get("http://localhost:" + port + "/oauth2/authorize")
	g.Expect(err).NotTo(HaveOccurred())
	_ = resp.Body.Close()
	g.Expect(resp.StatusCode).To(Equal(http.StatusOK),
		"should return 200 (index page) when OAuth2 is not configured")

	// Update the secret with OAuth2 configuration.
	oauth2ConfigYAML := `apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: http://localhost:` + port + `
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: flux-operator-web
      clientSecret: flux-ui-secret
      issuerURL: https://auth.example.com
`
	secret.Data["config.yaml"] = []byte(oauth2ConfigYAML)
	g.Expect(testClient.Update(ctx, secret)).To(Succeed())

	// Wait for the server to reconfigure and return 303 on /oauth2/authorize.
	// We need to use a custom HTTP client that does not follow redirects.
	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	g.Eventually(func() int {
		resp, err := httpClient.Get("http://localhost:" + port + "/oauth2/authorize")
		if err != nil {
			return 0
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}, 30*time.Second, 500*time.Millisecond).Should(Equal(http.StatusSeeOther),
		"should return 303 redirect when OAuth2 is configured")

	// Now switch back to no authentication and verify.
	secret.Data["config.yaml"] = []byte(initialConfigYAML)
	g.Expect(testClient.Update(ctx, secret)).To(Succeed())
	g.Eventually(func() int {
		resp, err := http.Get("http://localhost:" + port + "/oauth2/authorize")
		if err != nil {
			return 0
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}, 30*time.Second, 500*time.Millisecond).Should(Equal(http.StatusOK),
		"should return 200 (index page) when OAuth2 is removed from config")

	// Shutdown the server.
	cancelServer()

	// Wait for server to stop gracefully.
	gracefulShutdownDeadline := time.After(10 * time.Second)
	select {
	case <-confWatcherStopped:
		select {
		case err := <-serverErrCh:
			g.Expect(err).NotTo(HaveOccurred())
		case <-gracefulShutdownDeadline:
			t.Fatal("timed out waiting for web server to stop")
		}
	case <-gracefulShutdownDeadline:
		t.Fatal("timed out waiting for web server configuration watcher to stop")
	}
}
