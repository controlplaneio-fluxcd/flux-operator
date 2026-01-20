// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package user

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/gomega"
)

func TestImpersonation_SanitizeAndValidate(t *testing.T) {
	for _, tt := range []struct {
		name         string
		imp          Impersonation
		wantErr      string
		wantUsername string
		wantGroups   []string
	}{
		{
			name: "valid username only",
			imp: Impersonation{
				Username: "user@example.com",
			},
			wantUsername: "user@example.com",
			wantGroups:   []string{},
		},
		{
			name: "valid groups only",
			imp: Impersonation{
				Groups: []string{"group1", "group2"},
			},
			wantUsername: "",
			wantGroups:   []string{"group1", "group2"},
		},
		{
			name: "valid username and groups",
			imp: Impersonation{
				Username: "user@example.com",
				Groups:   []string{"admin", "developer"},
			},
			wantUsername: "user@example.com",
			wantGroups:   []string{"admin", "developer"},
		},
		{
			name: "trims whitespace from username",
			imp: Impersonation{
				Username: "  user@example.com  ",
				Groups:   []string{"group1"},
			},
			wantUsername: "user@example.com",
			wantGroups:   []string{"group1"},
		},
		{
			name: "trims whitespace from groups",
			imp: Impersonation{
				Username: "user@example.com",
				Groups:   []string{"  group1  ", "  group2  "},
			},
			wantUsername: "user@example.com",
			wantGroups:   []string{"group1", "group2"},
		},
		{
			name: "sorts groups alphabetically",
			imp: Impersonation{
				Username: "user@example.com",
				Groups:   []string{"zebra", "alpha", "middle"},
			},
			wantUsername: "user@example.com",
			wantGroups:   []string{"alpha", "middle", "zebra"},
		},
		{
			name: "nil groups becomes empty slice",
			imp: Impersonation{
				Username: "user@example.com",
				Groups:   nil,
			},
			wantUsername: "user@example.com",
			wantGroups:   []string{},
		},
		{
			name: "missing both username and groups fails",
			imp: Impersonation{
				Username: "",
				Groups:   []string{},
			},
			wantErr: "at least one of 'username' or 'groups' must be set for user impersonation",
		},
		{
			name: "whitespace-only username with no groups fails",
			imp: Impersonation{
				Username: "   ",
				Groups:   []string{},
			},
			wantErr: "at least one of 'username' or 'groups' must be set for user impersonation",
		},
		{
			name: "empty string in groups fails",
			imp: Impersonation{
				Username: "user@example.com",
				Groups:   []string{"group1", "", "group2"},
			},
			wantErr: "group[0] is an empty string",
		},
		{
			name: "whitespace-only group becomes empty string and fails",
			imp: Impersonation{
				Username: "user@example.com",
				Groups:   []string{"group1", "   "},
			},
			wantErr: "group[0] is an empty string",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := tt.imp.SanitizeAndValidate()
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(tt.imp.Username).To(Equal(tt.wantUsername))
				g.Expect(tt.imp.Groups).To(Equal(tt.wantGroups))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestSessionKey(t *testing.T) {
	t.Run("nil session returns privileged-user", func(t *testing.T) {
		g := NewWithT(t)

		var s *session
		g.Expect(s.Key()).To(Equal("privileged-user"))
	})

	t.Run("returns formatted key with username only", func(t *testing.T) {
		g := NewWithT(t)

		s := &session{
			Details: Details{
				Impersonation: Impersonation{
					Username: "test-user",
					Groups:   []string{},
				},
			},
		}
		g.Expect(s.Key()).To(Equal("username=test-user"))
	})

	t.Run("returns formatted key with username and groups", func(t *testing.T) {
		g := NewWithT(t)

		s := &session{
			Details: Details{
				Impersonation: Impersonation{
					Username: "test-user",
					Groups:   []string{"group1", "group2"},
				},
			},
		}
		expected := "username=test-user\ngroup=group1\ngroup=group2"
		g.Expect(s.Key()).To(Equal(expected))
	})
}

func TestSessionKubeClient(t *testing.T) {
	t.Run("nil session returns nil", func(t *testing.T) {
		g := NewWithT(t)

		var s *session
		g.Expect(s.KubeClient()).To(BeNil())
	})

	t.Run("returns kubeClient from session", func(t *testing.T) {
		g := NewWithT(t)

		mockClient := "mock-kube-client"
		s := &session{
			kubeClient: mockClient,
		}
		g.Expect(s.KubeClient()).To(Equal(mockClient))
	})
}

func TestKey(t *testing.T) {
	for _, tt := range []struct {
		name     string
		imp      Impersonation
		expected string
	}{
		{
			name: "username only",
			imp: Impersonation{
				Username: "user@example.com",
				Groups:   []string{},
			},
			expected: "username=user@example.com",
		},
		{
			name: "username with single group",
			imp: Impersonation{
				Username: "user@example.com",
				Groups:   []string{"admins"},
			},
			expected: "username=user@example.com\ngroup=admins",
		},
		{
			name: "username with multiple groups",
			imp: Impersonation{
				Username: "user@example.com",
				Groups:   []string{"admins", "developers", "viewers"},
			},
			expected: "username=user@example.com\ngroup=admins\ngroup=developers\ngroup=viewers",
		},
		{
			name: "empty username",
			imp: Impersonation{
				Username: "",
				Groups:   []string{"group1"},
			},
			expected: "username=\ngroup=group1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(Key(tt.imp)).To(Equal(tt.expected))
		})
	}
}

func TestStoreAndLoadSession(t *testing.T) {
	t.Run("stores and retrieves session", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		details := Details{
			Profile: Profile{Name: "Test User"},
			Impersonation: Impersonation{
				Username: "test-user",
				Groups:   []string{"group1"},
			},
		}
		mockClient := "mock-client"

		ctx = StoreSession(ctx, details, mockClient)
		session := LoadSession(ctx)

		g.Expect(session).NotTo(BeNil())
		g.Expect(session.Name).To(Equal("Test User"))
		g.Expect(session.Username).To(Equal("test-user"))
		g.Expect(session.Groups).To(Equal([]string{"group1"}))
		g.Expect(session.kubeClient).To(Equal(mockClient))
	})

	t.Run("LoadSession returns nil for context without session", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		session := LoadSession(ctx)

		g.Expect(session).To(BeNil())
	})

	t.Run("StoreSession sorts groups alphabetically", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		details := Details{
			Impersonation: Impersonation{
				Username: "test-user",
				Groups:   []string{"zebra", "alpha", "middle"},
			},
		}

		ctx = StoreSession(ctx, details, nil)
		session := LoadSession(ctx)

		g.Expect(session.Groups).To(Equal([]string{"alpha", "middle", "zebra"}))
	})
}

func TestKubeClientFromContext(t *testing.T) {
	t.Run("returns kubeClient from session", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		mockClient := "mock-kube-client"
		details := Details{
			Impersonation: Impersonation{Username: "user"},
		}

		ctx = StoreSession(ctx, details, mockClient)
		result := KubeClient(ctx)

		g.Expect(result).To(Equal(mockClient))
	})

	t.Run("returns nil when no session in context", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		result := KubeClient(ctx)

		g.Expect(result).To(BeNil())
	})
}

func TestPermissions(t *testing.T) {
	t.Run("returns Impersonation from session", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		details := Details{
			Impersonation: Impersonation{
				Username: "test-user",
				Groups:   []string{"group1", "group2"},
			},
		}

		ctx = StoreSession(ctx, details, nil)
		result := Permissions(ctx)

		g.Expect(result.Username).To(Equal("test-user"))
		g.Expect(result.Groups).To(Equal([]string{"group1", "group2"}))
	})

	t.Run("returns empty Impersonation when no session", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		result := Permissions(ctx)

		g.Expect(result.Username).To(BeEmpty())
		g.Expect(result.Groups).To(BeNil())
	})
}

func TestUsernameAndRole(t *testing.T) {
	t.Run("returns kubeconfig dev when no session and no HOSTNAME", func(t *testing.T) {
		g := NewWithT(t)

		// Ensure HOSTNAME is not set (t.Setenv restores original value after test)
		t.Setenv("HOSTNAME", "")
		os.Unsetenv("HOSTNAME") //nolint:errcheck

		ctx := context.Background()
		username, role := UsernameAndRole(ctx)

		g.Expect(username).To(Equal("kubeconfig (dev)"))
		g.Expect(role).To(BeEmpty())
	})

	t.Run("returns hostname when no session but HOSTNAME set", func(t *testing.T) {
		g := NewWithT(t)

		t.Setenv("HOSTNAME", "flux-operator-pod-abc123")

		ctx := context.Background()
		username, role := UsernameAndRole(ctx)

		g.Expect(username).To(Equal("flux-operator-pod-abc123"))
		g.Expect(role).To(BeEmpty())
	})

	t.Run("returns profile name with empty role when session has name", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		details := Details{
			Profile: Profile{Name: "John Doe"},
			Impersonation: Impersonation{
				Username: "john@example.com",
				Groups:   []string{"admins"},
			},
		}

		ctx = StoreSession(ctx, details, nil)
		username, role := UsernameAndRole(ctx)

		g.Expect(username).To(Equal("John Doe"))
		g.Expect(role).To(BeEmpty())
	})

	t.Run("returns username and groups as role when session has no name", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		details := Details{
			Impersonation: Impersonation{
				Username: "john@example.com",
				Groups:   []string{"admins", "developers"},
			},
		}

		ctx = StoreSession(ctx, details, nil)
		username, role := UsernameAndRole(ctx)

		g.Expect(username).To(Equal("john@example.com"))
		g.Expect(role).To(Equal("admins, developers"))
	})

	t.Run("returns username with empty role when no groups", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		details := Details{
			Impersonation: Impersonation{
				Username: "john@example.com",
				Groups:   []string{},
			},
		}

		ctx = StoreSession(ctx, details, nil)
		username, role := UsernameAndRole(ctx)

		g.Expect(username).To(Equal("john@example.com"))
		g.Expect(role).To(BeEmpty())
	})
}
