// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// actionClientOptions returns the kubeclient options used when performing
// GitOps actions and the reads that support them (e.g. fetching the object to
// patch or to audit).
//
// When fine-grained access is enabled, these operations are performed using
// the Flux Operator Web UI's own privileges instead of impersonating the user.
// The per-action custom RBAC verb is always checked against the impersonated
// user beforehand via Client.CanActOnResource, so the user still needs to be
// explicitly granted the action; they just no longer need the native
// Kubernetes verbs (e.g. patch) on top of it.
//
// When fine-grained access is disabled (the default), no options are returned
// and the user is impersonated as before.
func (h *Handler) actionClientOptions() []kubeclient.Option {
	if h.conf.FineGrainedAccessEnabled() {
		return []kubeclient.Option{kubeclient.WithPrivileges()}
	}
	return nil
}

// writeActionForbiddenError writes the appropriate HTTP error for a 403
// returned while performing a GitOps action.
//
// Under the default (impersonated) access mode, a 403 means the user lacks the
// native Kubernetes permissions required by the action, so it is surfaced as a
// user permission error (HTTP 403).
//
// Under fine-grained access, the action is performed using the Flux Operator
// Web UI application's own service account, not the user's identity. A 403
// therefore means the application itself is missing RBAC permissions, which is
// a cluster configuration problem and not the user's fault. It is surfaced as
// an internal error (HTTP 500) with a message addressed to the cluster
// administrator, making it unambiguous that the missing permissions belong to
// the Web UI application and not to the user.
func (h *Handler) writeActionForbiddenError(ctx context.Context, w http.ResponseWriter,
	err error, action, namespace, name string) {

	if h.conf.FineGrainedAccessEnabled() {
		log.FromContext(ctx).Error(err,
			"GitOps action forbidden by the Flux Operator Web UI application RBAC",
			"action", action, "namespace", namespace, "name", name)
		http.Error(w, fmt.Sprintf(
			"Action failed: the Flux Operator Web UI application is not allowed to %s %s/%s. "+
				"Fine-grained access is enabled (.spec.userActions.access: FineGrained), so this "+
				"action is performed using the Web UI application's own service account, not your "+
				"user identity. This is not a problem with your permissions: the cluster "+
				"administrator must grant the Flux Operator Web UI service account the Kubernetes "+
				"RBAC permissions required to perform this action.",
			action, namespace, name), http.StatusInternalServerError)
		return
	}

	perms := user.Permissions(ctx)
	http.Error(w, fmt.Sprintf("Permission denied. User %s does not have access to %s %s/%s",
		perms.Username, action, namespace, name), http.StatusForbidden)
}
