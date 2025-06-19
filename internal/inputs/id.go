// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

import (
	"fmt"
	"hash/adler32"
)

// ID returns a short, opaque ID for input sets.
func ID(s string) string {
	return fmt.Sprintf("%v", adler32.Checksum([]byte(s)))
}
