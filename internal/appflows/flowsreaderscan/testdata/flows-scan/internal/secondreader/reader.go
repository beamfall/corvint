// SPDX-License-Identifier: Apache-2.0

// Package secondreader is the FSV-V1-003 fixture: a package outside
// internal/appflows that opens the flows declaration directly.
package secondreader

import "os"

// Read opens the flows declaration under root without the shared reader.
func Read(root string) ([]byte, error) {
	return os.ReadFile(root + "/.corvint/flows.json")
}
