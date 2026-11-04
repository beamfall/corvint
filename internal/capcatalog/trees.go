// SPDX-License-Identifier: Apache-2.0
package capcatalog

import (
	"embed"
	"encoding/json"
	"strings"

	"github.com/Beamfall/corvint/internal/clitree"
)

//go:embed trees/*.json
var treeSnapshots embed.FS

// TreeSnapshot returns a shipped, offline command tree. No repository is read.
func TreeSnapshot(binary string) (clitree.Tree, bool) {
	var tree clitree.Tree
	data, err := treeSnapshots.ReadFile("trees/" + binary + ".json")
	if err != nil || json.Unmarshal(data, &tree) != nil {
		return tree, false
	}
	return tree, true
}

// TreeReferences links every companion catalog from the root machine catalog.
func TreeReferences() []CommandTreeRef {
	entries, _ := treeSnapshots.ReadDir("trees")
	var refs []CommandTreeRef
	for _, entry := range entries {
		binary := strings.TrimSuffix(entry.Name(), ".json")
		command := binary + " help --json"
		if binary == "corvint" {
			command = "corvint help --tree"
		}
		refs = append(refs, CommandTreeRef{Binary: binary, Command: command, Snapshot: "internal/capcatalog/trees/" + entry.Name()})
	}
	return refs
}
