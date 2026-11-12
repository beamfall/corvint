// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
)

// JST-V1-010: the release binary list ships no Jest provider (owner decision
// R14). Jest is a built-in runner of corvint, so no catalog entry's name or
// package names Jest.
func TestJSTV1010NoJestProviderBinary(t *testing.T) {
	catalog, err := loadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Binaries) == 0 {
		t.Fatal("the embedded catalog lists no binary")
	}
	for _, binary := range catalog.Binaries {
		if strings.Contains(strings.ToLower(binary.Name), "jest") || strings.Contains(strings.ToLower(binary.Package), "jest") {
			t.Errorf("catalog entry %s (%s) is a Jest provider", binary.Name, binary.Package)
		}
	}
}
