// SPDX-License-Identifier: Apache-2.0
package main

import (
	_ "embed"
	"encoding/json"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/clitree"
)

// Keep this binary's help independent of the global catalog's embedded data:
// catalog-only changes do not rebuild the observer consumed by the Node drivers.
// TestCAPV07700ShippedHelp compares its machine help with the catalog snapshot.
// CAP-V0-7700 / AFP-V0-762.
//
//go:embed help.json
var offlineHelp []byte

func answerHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	if len(args) > 0 && args[0] == "--version" {
		return 0, false
	}
	var tree clitree.Tree
	if json.Unmarshal(offlineHelp, &tree) != nil {
		return 1, true
	}
	var root strings.Builder
	for _, node := range tree.Nodes {
		if len(node.Path) == 0 {
			root.WriteString(clitree.Render(tree, node))
		}
	}
	return (clitree.Help{Tree: tree, Title: "Experimental local companion", Intro: func(w io.Writer) {
		io.WriteString(w, root.String())
	}}).Serve(args, stdout, stderr)
}
