// SPDX-License-Identifier: Apache-2.0
package clitree

import (
	"fmt"
	"slices"
	"strings"
)

// CatalogMarkdown is the checked full-depth LLM index. Detailed flags, inputs
// and examples remain in each linked machine catalog and per-command help.
func CatalogMarkdown(trees []Tree) string {
	trees = slices.Clone(trees)
	slices.SortFunc(trees, func(a, b Tree) int { return strings.Compare(a.Binary, b.Binary) })
	var out strings.Builder
	out.WriteString("| Command | Purpose | Minimal example | Complete help and flags |\n| --- | --- | --- | --- |\n")
	clean := func(s string) string {
		s = strings.SplitN(s, "\n", 2)[0]
		s = strings.ReplaceAll(s, "|", "&#124;")
		s = strings.ReplaceAll(s, "`", "'")
		return s
	}
	for _, tree := range trees {
		for _, node := range tree.Nodes {
			fmt.Fprintf(&out, "| `%s` | %s | `%s` | `%s`; [%s](../internal/capcatalog/trees/%s.json) |\n", node.Command, clean(node.Summary), clean(node.Example), node.Help, tree.Binary, tree.Binary)
		}
	}
	return out.String()
}
