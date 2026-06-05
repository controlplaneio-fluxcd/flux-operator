// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// fineGrainedConfig returns an OAuth2 config with fine-grained access enabled.
func fineGrainedConfig() *fluxcdv1.WebConfigSpec {
	return &fluxcdv1.WebConfigSpec{
		Authentication: &fluxcdv1.AuthenticationSpec{
			Type: fluxcdv1.AuthenticationTypeOAuth2,
		},
		UserActions: &fluxcdv1.UserActionsSpec{
			Access: fluxcdv1.UserActionsAccessFineGrained,
		},
	}
}

// grantUserActionVerb creates a ClusterRole/Binding that grants the given user
// the custom action verbs on resourcesets (get, list and the action verbs) but
// deliberately NOT the native patch verb. This models a least-privilege setup where
// a user is granted only the right to trigger an action.
func grantUserActionVerb(t *testing.T, g *WithT, username string, verbs ...string) {
	t.Helper()

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: username + "-role"},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     append([]string{"get", "list"}, verbs...),
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRole)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(ctx, clusterRole) })

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: username + "-binding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{{Kind: "User", Name: username}},
	}
	g.Expect(testClient.Create(ctx, clusterRoleBinding)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(ctx, clusterRoleBinding) })
}

// TestActionHandler_FineGrained_CustomVerbOnly_Success verifies the granted path:
// a user holding only the custom action verb (no native patch) can perform the
// action when fine-grained access is enabled, because the patch is performed
// using the Web UI application's own privileges. The same RBAC is rejected under
// the default impersonated mode, proving the behavioral difference.
func TestActionHandler_FineGrained_CustomVerbOnly_Success(t *testing.T) {
	g := NewWithT(t)

	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fg-custom-verb-only",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// User has the reconcile custom verb but NOT patch.
	username := "fg-custom-verb-only-user"
	grantUserActionVerb(t, g, username, "reconcile")

	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "FG Custom Verb Only User"},
		Impersonation: imp,
	}, userClient)

	newReq := func() *http.Request {
		actionReq := ActionRequest{
			Kind:      "ResourceSet",
			Namespace: "default",
			Name:      "test-fg-custom-verb-only",
			Action:    "reconcile",
		}
		body, _ := json.Marshal(actionReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/resource/action", bytes.NewBuffer(body))
		return req.WithContext(userCtx)
	}

	// Default (impersonated) mode: same RBAC is rejected because the user lacks patch.
	impersonatedHandler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}
	rec := httptest.NewRecorder()
	impersonatedHandler.ActionHandler(rec, newReq())
	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))

	// Fine-grained mode: the same RBAC now succeeds.
	fineGrainedHandler := &Handler{
		conf:          fineGrainedConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}
	rec = httptest.NewRecorder()
	fineGrainedHandler.ActionHandler(rec, newReq())
	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp ActionResponse
	g.Expect(json.NewDecoder(rec.Body).Decode(&resp)).To(Succeed())
	g.Expect(resp.Success).To(BeTrue())

	// Verify the reconcile annotation was actually set on the resource.
	var updated fluxcdv1.ResourceSet
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceSet), &updated)).To(Succeed())
	g.Expect(updated.Annotations).To(HaveKey("reconcile.fluxcd.io/requestedAt"))
}

// TestActionHandler_FineGrained_Suspend_Success verifies the granted path for the
// suspend action, including that the SuspendedBy annotation still records the user
// who initiated the action even though the patch runs with application privileges.
func TestActionHandler_FineGrained_Suspend_Success(t *testing.T) {
	g := NewWithT(t)

	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fg-suspend",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	username := "fg-suspend-user"
	grantUserActionVerb(t, g, username, "suspend")

	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "FG Suspend User"},
		Impersonation: imp,
	}, userClient)

	handler := &Handler{
		conf:          fineGrainedConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-fg-suspend",
		Action:    "suspend",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource/action", bytes.NewBuffer(body)).WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var updated fluxcdv1.ResourceSet
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceSet), &updated)).To(Succeed())
	g.Expect(updated.Annotations).To(HaveKeyWithValue(fluxcdv1.ReconcileAnnotation, fluxcdv1.DisabledValue))
	// SuspendedBy records the display name of the user who initiated the action,
	// which is preserved even though the patch runs with application privileges.
	g.Expect(updated.Annotations).To(HaveKeyWithValue(fluxcdv1.SuspendedByAnnotation, "FG Suspend User"))
}

// TestActionHandler_FineGrained_AppLacksPermission_InternalError verifies the
// non-granted path: when fine-grained access is enabled and the Web UI
// application's own service account lacks the RBAC permissions to perform the
// action, the 403 must bubble up as an internal error (HTTP 500) with a message
// that clearly attributes the missing permissions to the Web UI application and
// not to the user.
func TestActionHandler_FineGrained_AppLacksPermission_InternalError(t *testing.T) {
	g := NewWithT(t)

	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fg-app-no-perms",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// The user IS granted the reconcile custom verb, so the per-action RBAC check passes.
	username := "fg-app-no-perms-user"
	grantUserActionVerb(t, g, username, "reconcile")

	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "FG App No Perms User"},
		Impersonation: imp,
	}, userClient)

	// Build a privileged kubeclient that impersonates a Web UI application identity
	// which can read but NOT patch resourcesets. This models the Web UI service
	// account missing the permissions required to perform the action.
	appIdentity := "flux-operator-web-app-limited"
	appRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "fg-app-limited-role"},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"}, // no patch
			},
		},
	}
	g.Expect(testClient.Create(ctx, appRole)).To(Succeed())
	defer testClient.Delete(ctx, appRole)
	appBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "fg-app-limited-binding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     appRole.Name,
		},
		Subjects: []rbacv1.Subject{{Kind: "User", Name: appIdentity}},
	}
	g.Expect(testClient.Create(ctx, appBinding)).To(Succeed())
	defer testClient.Delete(ctx, appBinding)

	appCfg := rest.CopyConfig(testCluster.GetConfig())
	appCfg.Impersonate = rest.ImpersonationConfig{UserName: appIdentity}
	appClient, err := client.New(appCfg, client.Options{Scheme: testScheme})
	g.Expect(err).NotTo(HaveOccurred())
	limitedKubeClient, err := kubeclient.New(appClient, appClient, appCfg, testScheme, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	handler := &Handler{
		conf:          fineGrainedConfig(),
		kubeClient:    limitedKubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-fg-app-no-perms",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource/action", bytes.NewBuffer(body)).WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	// Must NOT be a 403 user permission error; it is an application RBAC problem.
	g.Expect(rec.Code).To(Equal(http.StatusInternalServerError))
	g.Expect(rec.Body.String()).To(ContainSubstring("Flux Operator Web UI application"))
	g.Expect(rec.Body.String()).To(ContainSubstring("not a problem with your permissions"))
	g.Expect(rec.Body.String()).NotTo(ContainSubstring("Permission denied"))

	// The resource must be unchanged (no reconcile annotation set).
	var updated fluxcdv1.ResourceSet
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceSet), &updated)).To(Succeed())
	g.Expect(updated.Annotations).NotTo(HaveKey("reconcile.fluxcd.io/requestedAt"))
}
