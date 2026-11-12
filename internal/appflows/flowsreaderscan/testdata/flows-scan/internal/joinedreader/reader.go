// SPDX-License-Identifier: Apache-2.0

// Package joinedreader is the FSV-V1-003 fixture of a second reader that
// builds the path from its parts.
package joinedreader

import (
	"os"
	"path/filepath"
)

// Read opens the flows declaration under root without the shared reader.
func Read(root string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, ".corvint", "flows.json"))
}
