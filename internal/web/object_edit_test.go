// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestObjectEditHandler_GetConfigMap_Success(t *testing.T) {
	g := NewWithT(t)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-get-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "value",
		},
	}
	g.Expect(testClient.Create(ctx, configMap)).To(Succeed())
	defer testClient.Delete(ctx, configMap)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/object?apiVersion=v1&kind=ConfigMap&namespace=default&name=test-get-configmap", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ObjectEditHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))
	var resp map[string]any
	g.Expect(json.NewDecoder(rec.Body).Decode(&resp)).To(Succeed())
	g.Expect(resp["kind"]).To(Equal("ConfigMap"))
	g.Expect(resp["data"]).To(HaveKeyWithValue("key", "value"))
}

func TestObjectEditHandler_UpdateConfigMap_Success(t *testing.T) {
	g := NewWithT(t)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-edit-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "old-value",
		},
	}
	g.Expect(testClient.Create(ctx, configMap)).To(Succeed())
	defer testClient.Delete(ctx, configMap)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	body, _ := json.Marshal(ObjectEditRequest{
		YAML: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-edit-configmap
  namespace: default
data:
  key: new-value
`,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/object", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ObjectEditHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp ObjectEditResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp.Success).To(BeTrue())
	g.Expect(resp.Message).To(ContainSubstring("Updated ConfigMap default/test-edit-configmap"))

	var updated corev1.ConfigMap
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), &updated)).To(Succeed())
	g.Expect(updated.Data).To(HaveKeyWithValue("key", "new-value"))
}

func TestObjectEditHandler_InvalidYAML(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	body, _ := json.Marshal(ObjectEditRequest{YAML: "apiVersion: ["})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/object", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ObjectEditHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("Invalid YAML"))
}

func TestObjectEditHandler_IdentityMismatch(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	body, _ := json.Marshal(ObjectEditRequest{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Namespace:  "default",
		Name:       "expected-name",
		YAML: `apiVersion: v1
kind: ConfigMap
metadata:
  name: different-name
  namespace: default
`,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/object", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ObjectEditHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("metadata.name cannot be changed"))
}

func TestObjectEditHandler_MissingIdentity(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	body, _ := json.Marshal(ObjectEditRequest{
		YAML: `apiVersion: v1
kind: ConfigMap
data:
  key: value
`,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/object", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ObjectEditHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("metadata.name is required"))
}
