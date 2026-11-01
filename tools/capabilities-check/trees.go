// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/clitree"
)

const (
	sectionTrees = "command-trees"
	// treesDir holds one corvint-command-tree/0 snapshot per binary with a command tree
	// (CAP-V0-045); each binary's own test keeps its snapshot equal to what it prints.
	treesDir = "internal/capcatalog/trees"
	// ratchetPath lists the cmd/ binaries that do not yet ship a command tree (CAP-V0-048).
	ratchetPath = "internal/capcatalog/CLI-RATCHET.tsv"
	// ratchetCeiling is the committed size of the ratchet. It may only fall: the check fails
	// when the file grows past it, and when the file shrinks the ceiling must follow, so a
	// removed row cannot quietly be re-added later.
	ratchetCeiling = 40
)

// ratchetBaseline is every binary the ratchet held when it was introduced (V1-2685). A row must
// name one of them, so the ratchet cannot trade a binary that gained its tree for a new binary
// without one. Rows leave the file; names never join this list.
var ratchetBaseline = strings.Fields(`
corvint-analyzer-c-jni corvint-analyzer-dotnet corvint-analyzer-go corvint-analyzer-html-css
corvint-analyzer-java corvint-analyzer-js corvint-analyzer-kotlin-android
corvint-analyzer-objective-c corvint-analyzer-python corvint-analyzer-ruby corvint-analyzer-rust
corvint-analyzer-shader corvint-analyzer-shell corvint-analyzer-sql-native
corvint-analyzer-structured-data corvint-analyzer-swift corvint-behavior-falsify
corvint-cem-candidate corvint-cem-experiments corvint-companion-release corvint-console
corvint-corpus-http corvint-corpus-mcp corvint-corpus-parity corvint-corpus-republish
corvint-dashboard-snapshot corvint-docs-mcp corvint-go-test-provider
corvint-go-toolchain-receipt corvint-intake corvint-js-test-provider corvint-lsp corvint-mcp
corvint-playwright-minimize corvint-postmerge-connect corvint-postmerge-host-launcher
corvint-postmerge-metrics corvint-postmerge-workflow corvint-public-release-check corvint-pulse
corvint-readiness-record corvint-release-candidate corvint-release-gate corvint-release-install
corvint-remote-provider corvint-ruby-test-provider corvint-test-runner corvint-test-validity-mcp
corvint-tests-accept corvint-update corvint-web-flows corvint-work-queue
`)

// CheckTrees is the command-tree half of the drift gate (CAP-V0-043, CAP-V0-047, CAP-V0-048).
// For each snapshot under root/treesDir it requires a well-formed tree for a real binary, and
// every dispatch word and option the snapshot's dispatch sources match (clitree.ScanDirs) to be a
// node word, a flag of some node, a global flag, or a reasoned internal entry. Every cmd/ binary
// needs a snapshot or a ratchet row, and the ratchet may only shrink. It reads files only.
func CheckTrees(root string, binaries []string) []Finding {
	var findings []Finding
	trees, err := readTrees(filepath.Join(root, filepath.FromSlash(treesDir)))
	if err != nil {
		return []Finding{{"malformed", sectionTrees, treesDir, err.Error()}}
	}
	known := map[string]bool{}
	for _, binary := range binaries {
		known[binary] = true
	}
	covered := map[string]bool{}
	for _, tree := range trees {
		covered[tree.Binary] = true
		if !known[tree.Binary] {
			findings = append(findings, Finding{"stale", sectionTrees, tree.Binary, "snapshot names no cmd/ binary"})
		}
		for _, problem := range clitree.Problems(tree) {
			findings = append(findings, Finding{"tree-problem", sectionTrees, tree.Binary, problem})
		}
		findings = append(findings, treeDrift(root, tree)...)
	}
	ratchet, err := readRatchet(filepath.Join(root, filepath.FromSlash(ratchetPath)))
	if err != nil {
		return append(findings, Finding{"malformed", sectionTrees, ratchetPath, err.Error()})
	}
	for _, binary := range binaries {
		if !covered[binary] && !ratchet[binary] {
			findings = append(findings, Finding{"tree-missing", sectionTrees, binary,
				"binary has no command-tree snapshot under " + treesDir + "; ship `help --json` and its snapshot (the ratchet may not grow)"})
		}
	}
	for binary := range ratchet {
		switch {
		case covered[binary]:
			findings = append(findings, Finding{"ratchet-stale", sectionTrees, binary, "binary now has a command tree; remove its ratchet row and lower ratchetCeiling"})
		case !known[binary]:
			findings = append(findings, Finding{"ratchet-stale", sectionTrees, binary, "ratchet row names no cmd/ binary; remove it and lower ratchetCeiling"})
		case !slices.Contains(ratchetBaseline, binary):
			findings = append(findings, Finding{"ratchet-grew", sectionTrees, binary, "binary was not on the ratchet when it was introduced; ship its command tree instead"})
		}
	}
	if len(ratchet) > ratchetCeiling {
		findings = append(findings, Finding{"ratchet-grew", sectionTrees, ratchetPath,
			fmt.Sprintf("%d rows exceed the ceiling of %d; a new binary ships its command tree instead", len(ratchet), ratchetCeiling)})
	}
	return findings
}

// treeDrift scans the tree's dispatch sources and reports every dispatched word or option the
// tree does not name.
func treeDrift(root string, tree clitree.Tree) []Finding {
	if len(tree.Sources) == 0 {
		return []Finding{{"tree-problem", sectionTrees, tree.Binary, "snapshot names no dispatch sources"}}
	}
	dirs := make([]string, 0, len(tree.Sources))
	for _, source := range tree.Sources {
		dirs = append(dirs, filepath.Join(root, filepath.FromSlash(source)))
	}
	scan, err := clitree.ScanDirs(dirs...)
	if err != nil {
		return []Finding{{"malformed", sectionTrees, tree.Binary, err.Error()}}
	}
	words, flags := tree.Words(), tree.AllFlags()
	var findings []Finding
	for _, word := range scan.Words {
		if !words[word.Value] {
			findings = append(findings, Finding{"subcommand-unlisted", sectionTrees, tree.Binary + " " + word.Value,
				"dispatched at " + word.At + " but no node, help synopsis or internal entry names it"})
		}
	}
	for _, flag := range scan.Flags {
		if !flags[flag.Value] {
			findings = append(findings, Finding{"flag-unlisted", sectionTrees, tree.Binary + " " + flag.Value,
				"matched at " + flag.At + " but no node's help, global flag or internal entry names it"})
		}
	}
	return findings
}

func readTrees(dir string) ([]clitree.Tree, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var trees []clitree.Tree
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var tree clitree.Tree
		if err := json.Unmarshal(data, &tree); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if tree.Schema != clitree.Schema || tree.Binary+".json" != entry.Name() {
			return nil, fmt.Errorf("%s: want schema %s for binary %s", entry.Name(), clitree.Schema, strings.TrimSuffix(entry.Name(), ".json"))
		}
		trees = append(trees, tree)
	}
	slices.SortFunc(trees, func(a, b clitree.Tree) int { return strings.Compare(a.Binary, b.Binary) })
	return trees, nil
}

// readRatchet reads binary<TAB>reason rows after a header; a row without a reason is malformed.
func readRatchet(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows := map[string]bool{}
	for index, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if index == 0 {
			if line != "binary\treason" {
				return nil, fmt.Errorf("header %q, want %q", line, "binary\treason")
			}
			continue
		}
		binary, reason, ok := strings.Cut(line, "\t")
		if !ok || binary == "" || strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("line %d: want binary<TAB>reason", index+1)
		}
		if rows[binary] {
			return nil, fmt.Errorf("line %d: duplicate row for %s", index+1, binary)
		}
		rows[binary] = true
	}
	return rows, nil
}

// treeSubcommands reads the committed corvint command tree and returns its two-word node paths
// by verb and the verbs that have a node (CAP-V0-960). A missing corvint snapshot is an error.
func treeSubcommands(root string) (map[string][]string, map[string]bool, error) {
	trees, err := readTrees(filepath.Join(root, filepath.FromSlash(treesDir)))
	if err != nil {
		return nil, nil, err
	}
	for _, tree := range trees {
		if tree.Binary != "corvint" {
			continue
		}
		subs, verbs := map[string][]string{}, map[string]bool{}
		for _, node := range tree.Nodes {
			if len(node.Path) > 0 {
				verbs[node.Path[0]] = true
			}
			if len(node.Path) == 2 {
				subs[node.Path[0]] = append(subs[node.Path[0]], node.Path[1])
			}
		}
		return subs, verbs, nil
	}
	return nil, nil, fmt.Errorf("no corvint command tree under %s", treesDir)
}

// MergeSubcommands adds the corvint command tree's two-word nodes to the declared subcommand
// tables and records the verbs the tree knows (CAP-V0-960, CAP-V0-961).
func MergeSubcommands(inv *Inventory, root string) error {
	subs, verbs, err := treeSubcommands(root)
	if err != nil {
		return err
	}
	if inv.Subcommands == nil {
		inv.Subcommands = map[string][]string{}
	}
	for verb, list := range subs {
		merged := append(inv.Subcommands[verb], list...)
		slices.Sort(merged)
		inv.Subcommands[verb] = slices.Compact(merged)
	}
	inv.TreeVerbs = verbs
	return nil
}
