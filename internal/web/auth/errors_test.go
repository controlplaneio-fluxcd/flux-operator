// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestSanitizeErrorMessage(t *testing.T) {
	for _, tt := range []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "internal error returns generic message",
			err:      errInternalError,
			expected: "An internal error occurred. Please try again. Contact your administrator if the problem persists.",
		},
		{
			name:     "wrapped internal error returns generic message",
			err:      fmt.Errorf("something went wrong: %w", errInternalError),
			expected: "An internal error occurred. Please try again. Contact your administrator if the problem persists.",
		},
		{
			name:     "invalid scopes error returns generic message",
			err:      errInvalidOAuth2Scopes,
			expected: "An internal error occurred. Please try again. Contact your administrator if the problem persists.",
		},
		{
			name:     "wrapped invalid scopes error returns generic message",
			err:      fmt.Errorf("scope issue: %w", errInvalidOAuth2Scopes),
			expected: "An internal error occurred. Please try again. Contact your administrator if the problem persists.",
		},
		{
			name:     "user error returns authentication failed",
			err:      errUserError,
			expected: "Authentication failed. Please try again.",
		},
		{
			name:     "random error returns authentication failed",
			err:      errors.New("some random error"),
			expected: "Authentication failed. Please try again.",
		},
		{
			name:     "nil error returns authentication failed",
			err:      nil,
			expected: "Authentication failed. Please try again.",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := sanitizeErrorMessage(tt.err)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}
