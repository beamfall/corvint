// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	sectionEntry = "llms-entry-point"
	// llmsPath is the repository's single LLM entry point (CAP-V0-049).
	llmsPath = "llms.txt"
)

// entryPointLinks are the views llms.txt must name besides each command tree's command and
// snapshot: the capability catalog page, the command catalog and the guide.
var entryPointLinks = []string{"docs/CAPABILITIES.md", "corvint help --json", "corvint help guide"}

// CheckEntryPoint requires root/llms.txt to exist and to name the capability catalog, the command
// catalog, the guide, and every committed command tree's printing command and snapshot path, so a
// new binary's tree is reachable from the entry point (CAP-V0-049). It reads files only.
func CheckEntryPoint(root string) []Finding {
	data, err := os.ReadFile(filepath.Join(root, llmsPath))
	if err != nil {
		return []Finding{{"entry-missing", sectionEntry, llmsPath, err.Error()}}
	}
	text := string(data)
	want := append([]string{}, entryPointLinks...)
	trees, err := readTrees(filepath.Join(root, filepath.FromSlash(treesDir)))
	if err != nil {
		return []Finding{{"malformed", sectionTrees, treesDir, err.Error()}}
	}
	for _, tree := range trees {
		want = append(want, tree.HelpJSON, treesDir+"/"+tree.Binary+".json")
	}
	var findings []Finding
	for _, link := range want {
		if !strings.Contains(text, link) {
			findings = append(findings, Finding{"entry-unlinked", sectionEntry, link, llmsPath + " does not name it"})
		}
	}
	return findings
}
