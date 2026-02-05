// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// DownloadableKinds lists the Flux source kinds that have downloadable artifacts.
var DownloadableKinds = []string{
	fluxcdv1.FluxBucketKind,
	fluxcdv1.FluxGitRepositoryKind,
	fluxcdv1.FluxOCIRepositoryKind,
	fluxcdv1.FluxHelmChartKind,
	fluxcdv1.FluxExternalArtifactKind,
}

// isDownloadableKind checks if the given kind supports artifact downloads.
func isDownloadableKind(kind string) bool {
	return slices.Contains(DownloadableKinds, kind)
}

// artifactHTTPClient is a dedicated HTTP client for fetching artifacts from source-controller.
// The timeout is set to 59 seconds, just under the web server's 60 second write timeout.
var artifactHTTPClient = &http.Client{
	Timeout: 59 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

// DownloadHandler handles GET /api/v1/artifact/download requests to download artifacts from Flux sources.
func (h *Handler) DownloadHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if actions are enabled.
	if !h.conf.UserActionsEnabled() {
		http.Error(w, "User actions are disabled", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters.
	kind := req.URL.Query().Get("kind")
	namespace := req.URL.Query().Get("namespace")
	name := req.URL.Query().Get("name")

	// Validate required fields.
	if kind == "" || namespace == "" || name == "" {
		http.Error(w, "Missing required query parameters: kind, namespace, name", http.StatusBadRequest)
		return
	}

	// Find the FluxKindInfo for validation.
	kindInfo, err := fluxcdv1.FindFluxKindInfo(kind)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unknown resource kind: %s", kind), http.StatusBadRequest)
		return
	}

	// Check if the kind supports downloads.
	if !isDownloadableKind(kindInfo.Name) {
		http.Error(w, fmt.Sprintf("Resource kind %s does not support artifact downloads", kindInfo.Name), http.StatusBadRequest)
		return
	}

	// Get the preferred GVK for the kind.
	gvk, err := h.preferredFluxGVK(req.Context(), kindInfo.Name)
	if err != nil {
		log.FromContext(req.Context()).Error(err, "failed to get GVK for kind", "kind", kindInfo.Name)
		http.Error(w, fmt.Sprintf("Unable to get resource type for kind %s", kindInfo.Name), http.StatusInternalServerError)
		return
	}

	ctx := req.Context()

	// Check custom RBAC for download action.
	if allowed, err := h.kubeClient.CanActOnResource(ctx,
		fluxcdv1.UserActionDownload, gvk.Group, kindInfo.Plural, namespace, name); err != nil {
		log.FromContext(req.Context()).Error(err, "failed to check custom RBAC for download",
			"kind", kind, "name", name, "namespace", namespace)
		http.Error(w, "Unable to verify permissions", http.StatusInternalServerError)
		return
	} else if !allowed {
		perms := user.Permissions(req.Context())
		http.Error(w, fmt.Sprintf("Permission denied. User %s does not have access to download %s/%s/%s",
			perms.Username, kind, namespace, name), http.StatusForbidden)
		return
	}

	// Fetch the resource.
	kubeClient := h.kubeClient.GetClient(ctx)
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(*gvk)
	resource.SetName(name)
	resource.SetNamespace(namespace)

	key := client.ObjectKey{Namespace: namespace, Name: name}
	if err := kubeClient.Get(ctx, key, resource); err != nil {
		log.FromContext(ctx).Error(err, "failed to fetch resource",
			"kind", kind, "name", name, "namespace", namespace)
		http.Error(w, fmt.Sprintf("Resource %s/%s not found", namespace, name), http.StatusNotFound)
		return
	}

	// Extract the artifact URL from status.artifact.url.
	artifactURL, found, err := unstructured.NestedString(resource.Object, "status", "artifact", "url")
	if err != nil || !found || artifactURL == "" {
		http.Error(w, fmt.Sprintf("Artifact not available for %s/%s", namespace, name), http.StatusNotFound)
		return
	}

	// Fetch the artifact from the source-controller.
	artifactReq, err := http.NewRequestWithContext(ctx, http.MethodGet, artifactURL, nil)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to create artifact request",
			"kind", kind, "name", name, "namespace", namespace, "url", artifactURL)
		http.Error(w, "Failed to download artifact from source", http.StatusBadGateway)
		return
	}

	artifactResp, err := artifactHTTPClient.Do(artifactReq)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to fetch artifact",
			"kind", kind, "name", name, "namespace", namespace, "url", artifactURL)
		http.Error(w, "Failed to download artifact from source", http.StatusBadGateway)
		return
	}
	defer artifactResp.Body.Close()

	if artifactResp.StatusCode != http.StatusOK {
		log.FromContext(ctx).Error(nil, "artifact fetch returned non-OK status",
			"kind", kind, "name", name, "namespace", namespace, "url", artifactURL, "status", artifactResp.StatusCode)
		http.Error(w, "Failed to download artifact from source", http.StatusBadGateway)
		return
	}

	// Enforce maximum artifact size of 50MB.
	const maxArtifactSize int64 = 50 * 1024 * 1024
	if artifactResp.ContentLength > maxArtifactSize {
		log.FromContext(ctx).Error(nil, "artifact size exceeds maximum allowed",
			"kind", kind, "name", name, "namespace", namespace,
			"size", artifactResp.ContentLength, "maxSize", maxArtifactSize)
		http.Error(w, fmt.Sprintf("Artifact size %d bytes exceeds maximum allowed size of 50MB", artifactResp.ContentLength), http.StatusBadRequest)
		return
	}

	// Set response headers for cross-browser compatible downloads.
	filename := fmt.Sprintf("%s-%s-%s.tar.gz", kind, namespace, name)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	// Content-Length allows browser to show download progress.
	if artifactResp.ContentLength > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", artifactResp.ContentLength))
	}

	w.WriteHeader(http.StatusOK)

	// Use chunked writing with flush to keep connection alive through proxies.
	// Track total bytes to enforce size limit even when Content-Length is missing or incorrect.
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 32*1024) // 32KB chunks
	var totalBytes int64
	for {
		n, err := artifactResp.Body.Read(buf)
		if n > 0 {
			totalBytes += int64(n)
			if totalBytes > maxArtifactSize {
				log.FromContext(ctx).Error(nil, "artifact exceeded maximum size during streaming",
					"kind", kind, "name", name, "namespace", namespace,
					"bytesRead", totalBytes, "maxSize", maxArtifactSize)
				return // Abort streaming, response will be truncated
			}
			if _, wErr := w.Write(buf[:n]); wErr != nil {
				return // Connection closed by client
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				log.FromContext(ctx).Error(err, "failed to stream artifact to client",
					"kind", kind, "name", name, "namespace", namespace)
			}
			break
		}
	}
}
