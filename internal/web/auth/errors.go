// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"errors"
	"strings"
)

var (
	errUserError     = errors.New("user error")
	errInternalError = errors.New("internal error")
)

// sanitizeErrorMessage returns a user-friendly error message. It
// avoids exposing internal error details that could aid attackers.
func sanitizeErrorMessage(err error) string {
	switch {
	case errors.Is(err, errInternalError), err != nil && strings.Contains(err.Error(), errInvalidOAuth2Scopes):
		return "An internal error occurred. Please try again. Contact your administrator if the problem persists."
	default:
		return "Authentication failed. Please try again."
	}
}
