// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func serveArtifact(addr string) {
	files := []struct {
		name    string
		content string
	}{
		{
			name: "kustomization.yaml",
			content: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - configmap.yaml
`,
		},
		{
			name: "configmap.yaml",
			content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: spiffe-e2e-static
  namespace: default
data:
  source: envoy
`,
		},
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, file := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: file.name,
			Mode: 0o600,
			Size: int64(len(file.content)),
		}); err != nil {
			fmt.Fprintf(os.Stderr, "artifact server failed to write header: %v\n", err)
			os.Exit(1)
		}
		if _, err := tw.Write([]byte(file.content)); err != nil {
			fmt.Fprintf(os.Stderr, "artifact server failed to write content: %v\n", err)
			os.Exit(1)
		}
	}
	if err := tw.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "artifact server failed to close tar: %v\n", err)
		os.Exit(1)
	}
	if err := gz.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "artifact server failed to close gzip: %v\n", err)
		os.Exit(1)
	}
	artifact := buf.Bytes()

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/gzip")
			w.Header().Set("Content-Length", strconv.Itoa(len(artifact)))
			_, _ = w.Write(artifact)
		}),
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "artifact server failed: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	addr := flag.String("addr", ":8080", "event sink listen address")
	artifactAddr := flag.String("artifact-addr", "", "artifact server listen address")
	flag.Parse()

	if *artifactAddr != "" {
		serveArtifact(*artifactAddr)
		return
	}

	server := &http.Server{
		Addr:              *addr,
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var event struct {
				ReportingController string `json:"reportingController"`
				Reason              string `json:"reason"`
				Message             string `json:"message"`
				InvolvedObject      struct {
					Kind      string `json:"kind"`
					Namespace string `json:"namespace"`
					Name      string `json:"name"`
				} `json:"involvedObject"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&event); err != nil {
				http.Error(w, "invalid event payload", http.StatusBadRequest)
				return
			}
			fmt.Printf("event received reportingController=%s kind=%s namespace=%s name=%s reason=%s message=%q\n",
				event.ReportingController,
				event.InvolvedObject.Kind,
				event.InvolvedObject.Namespace,
				event.InvolvedObject.Name,
				event.Reason,
				event.Message,
			)
			w.WriteHeader(http.StatusAccepted)
		}),
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "event sink failed: %v\n", err)
		os.Exit(1)
	}
}
