// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

import (
	"fmt"
	"hash/adler32"
)

// Checksum computes the checksum of a string using the Adler-32 algorithm.
func Checksum(s string) string {
	return fmt.Sprintf("%v", adler32.Checksum([]byte(s)))
}
