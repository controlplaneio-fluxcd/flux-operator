// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

// Credentials represents authentication credentials extracted from the request.
type Credentials struct {
	Username string
	Password string
	Token    string
}
