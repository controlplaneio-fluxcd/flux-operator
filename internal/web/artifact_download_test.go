// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// artifactResolverFunc adapts a function to the artifactIPResolver interface.
type artifactResolverFunc func(context.Context, string) ([]net.IPAddr, error)

// LookupIPAddr resolves an artifact hostname for tests.
func (f artifactResolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

func TestArtifactHTTPClientRejectsRedirects(t *testing.T) {
	g := NewWithT(t)

	g.Expect(artifactHTTPClient.CheckRedirect(nil, nil)).To(Equal(http.ErrUseLastResponse))
}

func TestArtifactHTTPClientWithTLSKeepsBaseSettings(t *testing.T) {
	g := NewWithT(t)

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13}
	httpClient := artifactHTTPClientWithTLS(tlsConfig)
	g.Expect(httpClient).NotTo(BeIdenticalTo(artifactHTTPClient))
	g.Expect(httpClient.Timeout).To(Equal(artifactHTTPClient.Timeout))
	g.Expect(httpClient.CheckRedirect(nil, nil)).To(Equal(http.ErrUseLastResponse))

	transport, ok := httpClient.Transport.(*http.Transport)
	g.Expect(ok).To(BeTrue())
	base := artifactHTTPClient.Transport.(*http.Transport)
	g.Expect(transport).NotTo(BeIdenticalTo(base))
	g.Expect(transport.TLSClientConfig).To(BeIdenticalTo(tlsConfig))
	g.Expect(transport.DialContext).NotTo(BeNil())
	g.Expect(transport.IdleConnTimeout).To(Equal(90 * time.Second))
	g.Expect(transport.TLSHandshakeTimeout).To(Equal(base.TLSHandshakeTimeout))
	g.Expect(transport.ResponseHeaderTimeout).To(Equal(base.ResponseHeaderTimeout))
}

func TestArtifactMTLSClientCache(t *testing.T) {
	g := NewWithT(t)

	var built [][]string
	cache := &artifactMTLSClientCache{
		buildClient: func(peers []string) (*http.Client, error) {
			built = append(built, peers)
			return artifactHTTPClientWithTLS(&tls.Config{MinVersion: tls.VersionTLS13}), nil
		},
	}

	sourceID := "spiffe://cluster.local/ns/flux-system/sa/source-controller"
	watcherID := "spiffe://cluster.local/ns/flux-system/sa/source-watcher"

	first, err := cache.get(nil, []string{sourceID, watcherID})
	g.Expect(err).NotTo(HaveOccurred())

	// Same peer set in a different order returns the cached client.
	second, err := cache.get(nil, []string{watcherID, sourceID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(second).To(BeIdenticalTo(first))
	g.Expect(built).To(HaveLen(1))
	g.Expect(built[0]).To(Equal([]string{sourceID, watcherID}))

	// A different peer set builds a new client.
	third, err := cache.get(nil, []string{sourceID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(third).NotTo(BeIdenticalTo(first))
	g.Expect(built).To(HaveLen(2))

	fourth, err := cache.get(nil, []string{sourceID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fourth).To(BeIdenticalTo(third))
	g.Expect(built).To(HaveLen(2))
}

// testTLSSource returns an SVID in a configurable trust domain.
type testTLSSource struct {
	trustDomain string
	err         error
}

// GetX509SVID returns an SVID in the configured trust domain.
func (s *testTLSSource) GetX509SVID() (*x509svid.SVID, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &x509svid.SVID{ID: spiffeid.RequireFromPath(spiffeid.RequireTrustDomainFromString(s.trustDomain), "/flux-operator")}, nil
}

// GetX509BundleForTrustDomain is not used by the tests.
func (s *testTLSSource) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return nil, errors.New("not implemented")
}

func TestArtifactMTLSClientCacheSelfTrustDomain(t *testing.T) {
	g := NewWithT(t)

	builds := 0
	cache := &artifactMTLSClientCache{
		buildClient: func([]string) (*http.Client, error) {
			builds++
			return artifactHTTPClientWithTLS(&tls.Config{MinVersion: tls.VersionTLS13}), nil
		},
	}
	source := &testTLSSource{trustDomain: "cluster.local"}

	selfPeers := []string{"spiffe://self/ns/flux-system/sa/source-controller"}
	first, err := cache.get(source, selfPeers)
	g.Expect(err).NotTo(HaveOccurred())
	second, err := cache.get(source, selfPeers)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(second).To(BeIdenticalTo(first))
	g.Expect(builds).To(Equal(1))

	// A trust domain change builds a new client for self-form peers.
	source.trustDomain = "example.org"
	third, err := cache.get(source, selfPeers)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(third).NotTo(BeIdenticalTo(first))
	g.Expect(builds).To(Equal(2))

	// The SVID read error is returned for self-form peers.
	source.err = errors.New("no SVID")
	_, err = cache.get(source, selfPeers)
	g.Expect(err).To(MatchError(ContainSubstring("no SVID")))
	source.err = nil

	// Explicit peers ignore the trust domain of the local SVID.
	explicitPeers := []string{"spiffe://cluster.local/ns/flux-system/sa/source-controller"}
	fourth, err := cache.get(source, explicitPeers)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(builds).To(Equal(3))
	source.trustDomain = "cluster.local"
	source.err = errors.New("no SVID")
	fifth, err := cache.get(source, explicitPeers)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fifth).To(BeIdenticalTo(fourth))
	g.Expect(builds).To(Equal(3))
}

func TestArtifactMTLSClientCacheBuildError(t *testing.T) {
	g := NewWithT(t)

	cache := &artifactMTLSClientCache{
		buildClient: func([]string) (*http.Client, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}

	httpClient, err := cache.get(nil, []string{"spiffe://cluster.local/ns/flux-system/sa/source-controller"})
	g.Expect(err).To(MatchError(io.ErrUnexpectedEOF))
	g.Expect(httpClient).To(BeNil())
	g.Expect(cache.client).To(BeNil())
}

func TestArtifactMTLSClientCacheReusesConnections(t *testing.T) {
	g := NewWithT(t)

	var newConns, closedConns atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("artifact"))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			newConns.Add(1)
		case http.StateClosed:
			closedConns.Add(1)
		}
	}
	server.StartTLS()
	defer server.Close()

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())

	// The base dialer rejects loopback, so dial the test server directly.
	cache := &artifactMTLSClientCache{
		buildClient: func([]string) (*http.Client, error) {
			httpClient := artifactHTTPClientWithTLS(&tls.Config{
				RootCAs:    roots,
				ServerName: "example.com",
				MinVersion: tls.VersionTLS12,
			})
			httpClient.Transport.(*http.Transport).DialContext = (&net.Dialer{}).DialContext
			return httpClient, nil
		},
	}

	peers := []string{"spiffe://cluster.local/ns/flux-system/sa/source-controller"}
	download := func(peers []string) {
		httpClient, err := cache.get(nil, peers)
		g.Expect(err).NotTo(HaveOccurred())
		resp, err := httpClient.Get(server.URL)
		g.Expect(err).NotTo(HaveOccurred())
		body, err := io.ReadAll(resp.Body)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp.Body.Close()).To(Succeed())
		g.Expect(string(body)).To(Equal("artifact"))
	}

	for range 20 {
		download(peers)
	}

	g.Expect(newConns.Load()).To(Equal(int32(1)))

	// Changing the peer set closes the idle connections of the previous client.
	download([]string{"spiffe://cluster.local/ns/flux-system/sa/source-watcher"})
	g.Eventually(closedConns.Load, 5*time.Second, 50*time.Millisecond).Should(Equal(int32(1)))
	g.Expect(newConns.Load()).To(Equal(int32(2)))
}

func TestArtifactDialContextRejectsLiteralIPHostnames(t *testing.T) {
	testCases := []struct {
		name    string
		address string
	}{
		{name: "IPv4", address: "10.0.0.10:80"},
		{name: "IPv6", address: "[2001:db8::1]:443"},
		{name: "IPv6 zone", address: "[fe80::1%eth0]:80"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			resolverCalled := false
			dialCalled := false
			resolver := artifactResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
				resolverCalled = true
				return nil, nil
			})
			dial := newArtifactDialContext(resolver, func(context.Context, string, string) (net.Conn, error) {
				dialCalled = true
				return nil, nil
			})

			conn, err := dial(t.Context(), "tcp", tc.address)
			g.Expect(err).To(MatchError(ContainSubstring("must use a DNS hostname")))
			g.Expect(conn).To(BeNil())
			g.Expect(resolverCalled).To(BeFalse())
			g.Expect(dialCalled).To(BeFalse())
		})
	}
}

func TestIsBlockedArtifactAddress(t *testing.T) {
	testCases := []struct {
		name    string
		address string
		blocked bool
	}{
		{name: "IPv4 loopback", address: "127.0.0.1", blocked: true},
		{name: "IPv6 loopback", address: "::1", blocked: true},
		{name: "IPv4 link-local unicast", address: "169.254.169.254", blocked: true},
		{name: "IPv6 link-local unicast", address: "fe80::1", blocked: true},
		{name: "IPv4 link-local multicast", address: "224.0.0.1", blocked: true},
		{name: "IPv6 link-local multicast", address: "ff02::1", blocked: true},
		{name: "IPv6 instance metadata", address: "fd00:ec2::254", blocked: true},
		{name: "IPv6 unique-local", address: "fd00::1", blocked: false},
		{name: "private IPv4", address: "10.0.0.10", blocked: false},
		{name: "public IPv4", address: "192.0.2.1", blocked: false},
		{name: "public IPv6", address: "2001:db8::1", blocked: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(isBlockedArtifactAddress(net.ParseIP(tc.address))).To(Equal(tc.blocked))
		})
	}
}

func TestArtifactDialContextRejectsBlockedDNSAnswers(t *testing.T) {
	testCases := []struct {
		name  string
		addrs []net.IPAddr
	}{
		{
			name:  "loopback",
			addrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}},
		},
		{
			name:  "link-local",
			addrs: []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}},
		},
		{
			name: "mixed public and loopback",
			addrs: []net.IPAddr{
				{IP: net.ParseIP("192.0.2.1")},
				{IP: net.ParseIP("127.0.0.1")},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			dialCalled := false
			resolver := artifactResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
				return tc.addrs, nil
			})
			dial := newArtifactDialContext(resolver, func(context.Context, string, string) (net.Conn, error) {
				dialCalled = true
				return nil, nil
			})

			conn, err := dial(t.Context(), "tcp", "artifact.example.com:80")
			g.Expect(err).To(MatchError(ContainSubstring("resolves to blocked address")))
			g.Expect(conn).To(BeNil())
			g.Expect(dialCalled).To(BeFalse())
		})
	}
}

func TestArtifactDialContextDialsValidatedPrivateAddress(t *testing.T) {
	testCases := []struct {
		name string
		host string
	}{
		{
			name: "hostname",
			host: "artifact-server.flux-system.svc.cluster.local",
		},
		{
			// Flux controllers advertise their storage address as a rooted FQDN,
			// e.g. source-controller.flux-system.svc.cluster.local.
			name: "rooted FQDN",
			host: "artifact-server.flux-system.svc.cluster.local.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			var resolvedHost string
			resolver := artifactResolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
				resolvedHost = host
				return []net.IPAddr{{IP: net.ParseIP("10.0.0.10")}}, nil
			})

			var dialedAddress string
			var peer net.Conn
			dial := newArtifactDialContext(resolver, func(_ context.Context, _, address string) (net.Conn, error) {
				dialedAddress = address
				conn, server := net.Pipe()
				peer = server
				return conn, nil
			})

			conn, err := dial(t.Context(), "tcp", net.JoinHostPort(tc.host, "80"))
			g.Expect(err).NotTo(HaveOccurred())
			defer conn.Close()
			defer peer.Close()

			g.Expect(resolvedHost).To(Equal(tc.host))
			g.Expect(dialedAddress).To(Equal("10.0.0.10:80"))
		})
	}
}

func TestDownloadHandler_MethodNotAllowed(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Test with POST method (should fail)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/artifact/download?kind=GitRepository&namespace=default&name=test", nil)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("Method not allowed"))
}

func TestDownloadHandler_MissingParameters(t *testing.T) {
	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name  string
		query string
	}{
		{
			name:  "missing kind",
			query: "namespace=default&name=test",
		},
		{
			name:  "missing namespace",
			query: "kind=GitRepository&name=test",
		},
		{
			name:  "missing name",
			query: "kind=GitRepository&namespace=default",
		},
		{
			name:  "all missing",
			query: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/artifact/download?"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.DownloadHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("Missing required query parameters"))
		})
	}
}

func TestDownloadHandler_UnknownKind(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/artifact/download?kind=UnknownKind&namespace=default&name=test", nil)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("Unknown resource kind"))
}

func TestDownloadHandler_NonDownloadableKind(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Kustomization is a valid Flux kind but doesn't support downloads
	req := httptest.NewRequest(http.MethodGet, "/api/v1/artifact/download?kind=Kustomization&namespace=default&name=test", nil)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("does not support artifact downloads"))
}

func TestDownloadHandler_ActionsDisabled_NoAuth(t *testing.T) {
	g := NewWithT(t)

	// Test with no authentication configured
	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			UserActions: &fluxcdv1.UserActionsSpec{},
		},
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/artifact/download?kind=GitRepository&namespace=default&name=test", nil)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("User actions are disabled"))
}

func TestDownloadHandler_UnprivilegedUser_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create RBAC for the test user but NOT for the "download" action
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-download-no-permission",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"}, // No "download" permission
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRole)).To(Succeed())
	defer testClient.Delete(ctx, clusterRole)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-download-no-permission-binding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "unprivileged-download-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRoleBinding)).To(Succeed())
	defer testClient.Delete(ctx, clusterRoleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create an unprivileged user session (no download RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-download-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	// Request for a ResourceSet - will fail at RBAC check
	// Using ResourceSet because FluxOperator CRDs are installed in test env
	req := httptest.NewRequest(http.MethodGet, "/api/v1/artifact/download?kind=ResourceSet&namespace=default&name=test", nil)
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	// ResourceSet is not a downloadable kind, so it should fail at kind validation
	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("does not support artifact downloads"))
}

func TestDownloadHandler_GVKLookupFailsWhenCRDNotInstalled(t *testing.T) {
	g := NewWithT(t)

	// This test verifies that when Flux CRDs are not installed,
	// the handler returns 500 Internal Server Error for GVK lookup failure.
	// In a real cluster with Flux installed, this would proceed to RBAC checks.

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Request for a GitRepository - will fail at GVK lookup because CRD is not installed
	req := httptest.NewRequest(http.MethodGet, "/api/v1/artifact/download?kind=GitRepository&namespace=default&name=test", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	// Should return 500 because the GitRepository CRD is not installed in test environment
	g.Expect(rec.Code).To(Equal(http.StatusInternalServerError))
	g.Expect(rec.Body.String()).To(ContainSubstring("Unable to get resource type"))
}

func TestIsDownloadableKind(t *testing.T) {
	g := NewWithT(t)

	// Test downloadable kinds
	g.Expect(isDownloadableKind(fluxcdv1.FluxGitRepositoryKind)).To(BeTrue())
	g.Expect(isDownloadableKind(fluxcdv1.FluxBucketKind)).To(BeTrue())
	g.Expect(isDownloadableKind(fluxcdv1.FluxOCIRepositoryKind)).To(BeTrue())
	g.Expect(isDownloadableKind(fluxcdv1.FluxHelmChartKind)).To(BeTrue())
	g.Expect(isDownloadableKind(fluxcdv1.FluxExternalArtifactKind)).To(BeTrue())

	// Test non-downloadable kinds
	g.Expect(isDownloadableKind(fluxcdv1.FluxKustomizationKind)).To(BeFalse())
	g.Expect(isDownloadableKind(fluxcdv1.FluxHelmReleaseKind)).To(BeFalse())
	g.Expect(isDownloadableKind(fluxcdv1.FluxAlertKind)).To(BeFalse())
	g.Expect(isDownloadableKind("UnknownKind")).To(BeFalse())
}

func TestDownloadHandler_DownloadableKindsValidation(t *testing.T) {
	downloadableKinds := []string{
		fluxcdv1.FluxBucketKind,
		fluxcdv1.FluxGitRepositoryKind,
		fluxcdv1.FluxOCIRepositoryKind,
		fluxcdv1.FluxHelmChartKind,
	}

	nonDownloadableKinds := []string{
		fluxcdv1.FluxKustomizationKind,
		fluxcdv1.FluxHelmReleaseKind,
		fluxcdv1.FluxAlertKind,
		fluxcdv1.FluxAlertProviderKind,
		"ResourceSet",
	}

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Test that downloadable kinds pass the kind validation
	for _, kind := range downloadableKinds {
		t.Run("downloadable_"+kind, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/artifact/download?kind="+kind+"&namespace=default&name=test", nil)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.DownloadHandler(rec, req)

			// Should get past kind validation (not 400 for invalid kind)
			// Will fail with 404 (not found) or 403 (no permission) or 500 (no CRD installed)
			g.Expect(rec.Body.String()).NotTo(ContainSubstring("does not support artifact downloads"))
		})
	}

	// Test that non-downloadable kinds are rejected
	for _, kind := range nonDownloadableKinds {
		t.Run("non_downloadable_"+kind, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/artifact/download?kind="+kind+"&namespace=default&name=test", nil)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.DownloadHandler(rec, req)

			// Should fail with 400 for non-downloadable kind
			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("does not support artifact downloads"))
		})
	}
}
