// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestResourceSetInputProviderReconciler_ExternalService_LifeCycle(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a mock HTTP server that returns inputs.
	inputsResponse := externalServiceResponse{
		Inputs: []map[string]any{
			{"id": "1", "tenant": "tenant1", "version": "2.0.0-rc.1"},
			{"id": "2", "tenant": "tenant2", "version": "1.9.0"},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(inputsResponse)
	}))
	defer server.Close()

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-external-service
  namespace: "%[1]s"
spec:
  type: ExternalService
  url: %[2]s
  insecure: true
`, ns.Name, server.URL)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize the ResourceSetInputProvider.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile and verify exported inputs.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(2))
	g.Expect(result.Status.LastExportedRevision).To(HavePrefix("sha256:"))
	g.Expect(result.Status.LastExportedRevision).To(HaveLen(71))

	// Verify input values.
	b, err := yaml.Marshal(result.Status.ExportedInputs[0])
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(MatchYAML(`
id: "1"
tenant: tenant1
version: 2.0.0-rc.1
`))

	b, err = yaml.Marshal(result.Status.ExportedInputs[1])
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(MatchYAML(`
id: "2"
tenant: tenant2
version: "1.9.0"
`))

	lastRevision := result.Status.LastExportedRevision

	// Reconcile again with same data — revision should not change.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	result = &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Status.LastExportedRevision).To(Equal(lastRevision))

	// Update mock server response — revision should change.
	inputsResponse = externalServiceResponse{
		Inputs: []map[string]any{
			{"id": "1", "tenant": "tenant1", "version": "2.0.0"},
		},
	}

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	result = &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(1))
	g.Expect(result.Status.LastExportedRevision).NotTo(Equal(lastRevision))
}

func TestResourceSetInputProviderReconciler_ExternalService_DefaultValues(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(externalServiceResponse{
			Inputs: []map[string]any{
				{"id": "1", "tenant": "tenant1"},
			},
		})
	}))
	defer server.Close()

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-es-defaults
  namespace: "%[1]s"
spec:
  type: ExternalService
  url: %[2]s
  insecure: true
  defaultValues:
    env: "production"
    region: "us-east-1"
`, ns.Name, server.URL)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Reconcile.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(1))

	// Verify default values are merged.
	b, err := yaml.Marshal(result.Status.ExportedInputs[0])
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(MatchYAML(`
id: "1"
tenant: tenant1
env: production
region: us-east-1
`))
}

func TestResourceSetInputProviderReconciler_ExternalService_BearerToken(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a mock server that validates bearer token.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(externalServiceResponse{
			Inputs: []map[string]any{
				{"id": "1", "feature": "cert-manager"},
			},
		})
	}))
	defer server.Close()

	// Create the auth secret with a bearer token.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bearer-secret",
			Namespace: ns.Name,
		},
		Data: map[string][]byte{
			"token": []byte("my-secret-token"),
		},
	}
	err = testEnv.Create(ctx, secret)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-es-bearer
  namespace: "%[1]s"
spec:
  type: ExternalService
  url: %[2]s
  insecure: true
  secretRef:
    name: bearer-secret
`, ns.Name, server.URL)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Reconcile.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(1))
}

func TestResourceSetInputProviderReconciler_ExternalService_BasicAuth(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a mock server that validates basic auth.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "admin" || password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(externalServiceResponse{
			Inputs: []map[string]any{
				{"id": "1", "feature": "cert-manager"},
			},
		})
	}))
	defer server.Close()

	// Create the auth secret with basic auth.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basic-auth-secret",
			Namespace: ns.Name,
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret"),
		},
	}
	err = testEnv.Create(ctx, secret)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-es-basic
  namespace: "%[1]s"
spec:
  type: ExternalService
  url: %[2]s
  insecure: true
  secretRef:
    name: basic-auth-secret
`, ns.Name, server.URL)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Reconcile.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(1))
}

func TestResourceSetInputProviderReconciler_ExternalService_FilterLimit(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Return 5 inputs.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(externalServiceResponse{
			Inputs: []map[string]any{
				{"id": "1", "name": "a"},
				{"id": "2", "name": "b"},
				{"id": "3", "name": "c"},
				{"id": "4", "name": "d"},
				{"id": "5", "name": "e"},
			},
		})
	}))
	defer server.Close()

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-es-limit
  namespace: "%[1]s"
spec:
  type: ExternalService
  url: %[2]s
  insecure: true
  filter:
    limit: 2
`, ns.Name, server.URL)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Reconcile.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	// Should be limited to 2 despite 5 being returned.
	g.Expect(result.Status.ExportedInputs).To(HaveLen(2))
}

func TestResourceSetInputProviderReconciler_ExternalService_PayloadTooLarge(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create a mock server that returns a response larger than 900Ki.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"inputs": [`))
		// Write a payload larger than maxExternalServicePayloadSize.
		largeValue := strings.Repeat("x", 1024)
		for i := 0; i < 1000; i++ {
			if i > 0 {
				_, _ = w.Write([]byte(","))
			}
			_, _ = fmt.Fprintf(w, `{"id": "%d", "data": "%s"}`, i, largeValue)
		}
		_, _ = w.Write([]byte(`]}`))
	}))
	defer server.Close()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  server.URL,
		},
	}

	_, err := reconciler.callExternalServiceProvider(ctx, obj, nil, nil, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("maximum allowed size"))
}

func TestResourceSetInputProviderReconciler_ExternalService_MissingID(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(externalServiceResponse{
			Inputs: []map[string]any{
				{"tenant": "tenant1", "version": "1.0.0"},
			},
		})
	}))
	defer server.Close()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  server.URL,
		},
	}

	_, err := reconciler.callExternalServiceProvider(ctx, obj, nil, nil, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("missing the required 'id' field"))
}

func TestResourceSetInputProviderReconciler_ExternalService_DuplicateID(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(externalServiceResponse{
			Inputs: []map[string]any{
				{"id": "1", "tenant": "tenant1"},
				{"id": "1", "tenant": "tenant2"},
			},
		})
	}))
	defer server.Close()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  server.URL,
		},
	}

	_, err := reconciler.callExternalServiceProvider(ctx, obj, nil, nil, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("duplicate 'id' value '1'"))
}

func TestResourceSetInputProviderReconciler_ExternalService_InvalidJSON(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  server.URL,
		},
	}

	_, err := reconciler.callExternalServiceProvider(ctx, obj, nil, nil, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to parse JSON response"))
}

func TestResourceSetInputProviderReconciler_ExternalService_HTTPError(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  server.URL,
		},
	}

	_, err := reconciler.callExternalServiceProvider(ctx, obj, nil, nil, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("returned HTTP 500"))
}

func TestResourceSetInputProviderReconciler_ExternalService_MissingInputsField(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "1"}]}`))
	}))
	defer server.Close()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  server.URL,
		},
	}

	_, err := reconciler.callExternalServiceProvider(ctx, obj, nil, nil, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("does not contain an 'inputs' field"))
}

func TestResourceSetInputProviderReconciler_ExternalService_NestedValues(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Return inputs with nested/complex values (arrays, objects).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(externalServiceResponse{
			Inputs: []map[string]any{
				{
					"id":      "1",
					"tenant":  "tenant1",
					"version": "2.0.0",
					"featureFlags": []string{
						"feature1",
						"feature2",
					},
				},
			},
		})
	}))
	defer server.Close()

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-es-nested
  namespace: "%[1]s"
spec:
  type: ExternalService
  url: %[2]s
  insecure: true
`, ns.Name, server.URL)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Reconcile.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.ExportedInputs).To(HaveLen(1))

	b, err := yaml.Marshal(result.Status.ExportedInputs[0])
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(MatchYAML(`
id: "1"
tenant: tenant1
version: "2.0.0"
featureFlags:
  - feature1
  - feature2
`))
}

func TestResourceSetInputProviderReconciler_ExternalService_NonStringID(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return numeric ID — should be rejected.
		_, _ = w.Write([]byte(`{"inputs": [{"id": 123, "name": "test"}]}`))
	}))
	defer server.Close()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  server.URL,
		},
	}

	_, err := reconciler.callExternalServiceProvider(ctx, obj, nil, nil, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("non-string 'id' field"))
}

func TestResourceSetInputProviderReconciler_ExternalService_UserAgent(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	reconciler.Version = "1.2.3-test"
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var capturedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(externalServiceResponse{
			Inputs: []map[string]any{
				{"id": "1", "name": "test"},
			},
		})
	}))
	defer server.Close()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  server.URL,
		},
	}

	_, err := reconciler.callExternalServiceProvider(ctx, obj, nil, nil, nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(capturedUserAgent).To(Equal("flux-operator/1.2.3-test"))
}

func TestResourceSetInputProviderReconciler_ExternalService_InvalidURL(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  "oci://registry.example.com/repo",
		},
	}

	r, err := reconciler.reconcile(ctx, obj, nil)
	g.Expect(r).To(Equal(reconcile.Result{}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(obj)).To(BeFalse())
	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.GetReason(obj, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonInvalidSpec))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring("spec.url must start with"))
}

func TestResourceSetInputProviderReconciler_ExternalService_HTTPWithoutInsecure(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalService,
			URL:  "http://example.com/api",
		},
	}

	r, err := reconciler.reconcile(ctx, obj, nil)
	g.Expect(r).To(Equal(reconcile.Result{}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(obj)).To(BeFalse())
	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.GetReason(obj, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonInvalidSpec))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring("spec.insecure is true"))
}

func TestResourceSetInputProviderReconciler_ExternalService_InsecureOnNonExternalService(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type:     fluxcdv1.InputProviderGitHubBranch,
			URL:      "https://github.com/example/repo",
			Insecure: true,
		},
	}

	r, err := reconciler.reconcile(ctx, obj, nil)
	g.Expect(r).To(Equal(reconcile.Result{}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(obj)).To(BeFalse())
	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.GetReason(obj, meta.ReadyCondition)).To(Equal(fluxcdv1.ReasonInvalidSpec))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring("spec.insecure can only be set when spec.type is 'ExternalService' or 'OCIArtifactTag'"))
}
