// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
)

func TestIntoContext(t *testing.T) {
	g := NewWithT(t)

	ctx := auth.IntoContext(context.Background(), &auth.Session{UserName: "test-user"})

	sess := auth.FromContext(ctx)

	g.Expect(sess).To(Equal(&auth.Session{UserName: "test-user"}))
}
