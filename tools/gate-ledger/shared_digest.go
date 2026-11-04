// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// sharedDigestEnv names the per-run worktree digest file (GL-V0-786, V1-4194).
// It is outside the GO*, CGO_* and CORVINT_* prefixes batch-precheck salts a
// step with, so the run-specific path never enters a key.
const sharedDigestEnv = "GATE_LEDGER_SHARED_DIGEST"

const sharedDigestSchema = "corvint-gate-ledger-digest/1"

type sharedDigest struct {
	Schema  string      `json:"schema"`
	Root    string      `json:"root"`
	Tree    string      `json:"tree"`
	Err     string      `json:"err,omitempty"`
	Entries [][2]string `json:"entries"`
}

// openShared returns the ledger view from the shared digest file when the
// invocation's env names one whose root is its working directory; any doubt (no
// file, another root, an unreadable or foreign file) returns false and the
// caller digests the worktree itself.
func openShared(inv invocation, stdout, stderr io.Writer) (*ledger, bool) {
	file := inv.getenv(sharedDigestEnv)
	if file == "" {
		return nil, false
	}
	d, err := readSharedDigest(file)
	if err != nil {
		return nil, false
	}
	if inv.dir == "" || !sameDirectory(inv.dir, d.Root) {
		return nil, false
	}
	l := &ledger{inv: inv, stdout: stdout, stderr: stderr, root: d.Root, tree: d.Tree, digestErr: d.Err}
	for _, e := range d.Entries {
		l.entries = append(l.entries, treeEntry{path: e[0], line: e[1]})
	}
	l.identity = inv.toolIdentity()
	l.dir, l.dirErr = inv.ledgerDirectory()
	return l, true
}

func sameDirectory(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ra == rb
}

func readSharedDigest(file string) (sharedDigest, error) {
	var d sharedDigest
	data, err := os.ReadFile(file)
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return d, err
	}
	if d.Schema != sharedDigestSchema || d.Root == "" || (d.Tree == "" && d.Err == "") {
		return d, fmt.Errorf("%s is not a %s file", file, sharedDigestSchema)
	}
	return d, nil
}

// digestCommand is `gate-ledger digest FILE`, which writes this worktree's
// digest for the run's steps to share, and `gate-ledger digest --verify FILE`,
// which digests the worktree again and, when it moved, removes every record
// written under the shared tree since FILE was written and exits 1, so no step
// keeps a pass keyed on content it may not have read (GL-V0-786).
func digestCommand(inv invocation, args []string, stdout, stderr io.Writer) int {
	verify := len(args) == 2 && args[0] == "--verify"
	if !verify && len(args) != 1 {
		return usage(stderr)
	}
	file := args[len(args)-1]
	// The digest is always the worktree's own, never a shared file's.
	inv.env = append(slices.Clone(inv.env), sharedDigestEnv+"=")
	l := open(inv, stdout, stderr)
	if l.root == "" {
		fmt.Fprintf(stderr, "%sDIGEST unavailable: %s\n", prefixOut, l.dirErr)
		return 1
	}
	if !verify {
		d := sharedDigest{Schema: sharedDigestSchema, Root: l.root, Tree: l.tree, Err: l.digestErr}
		for _, e := range l.entries {
			d.Entries = append(d.Entries, [2]string{e.path, e.line})
		}
		data, err := json.Marshal(d)
		if err == nil {
			err = os.WriteFile(file, data, 0o600)
		}
		if err != nil {
			fmt.Fprintf(stderr, "%sDIGEST not written: %s\n", prefixOut, err)
			return 1
		}
		fmt.Fprintf(stdout, "%sDIGEST %s shared through %s\n", prefixOut, describeTree(l.tree, l.digestErr), sharedDigestEnv)
		return 0
	}
	d, err := readSharedDigest(file)
	if err != nil {
		fmt.Fprintf(stderr, "%sDIGEST unreadable: %s\n", prefixOut, err)
		return 1
	}
	if d.Tree == l.tree && d.Err == l.digestErr {
		fmt.Fprintf(stdout, "%sDIGEST unchanged %s\n", prefixOut, describeTree(l.tree, l.digestErr))
		return 0
	}
	removed := 0
	if info, err := os.Stat(file); err == nil && l.dirErr == "" && d.Tree != "" {
		removed = forgetSince(l.dir, d.Tree, info.ModTime().Unix())
	}
	fmt.Fprintf(stdout, "%sDIGEST changed during the run (%s -> %s); removed %d records written under the shared tree\n", prefixOut, describeTree(d.Tree, d.Err), describeTree(l.tree, l.digestErr), removed)
	return 1
}

func describeTree(tree, err string) string {
	if tree == "" {
		return "undigested: " + err
	}
	return "tree " + tree
}

// forgetSince removes the records in dir whose tree is tree and that were
// written at or after since (Unix seconds).
func forgetSince(dir, tree string, since int64) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil || info.ModTime().Unix() < since {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var rec record
		if json.Unmarshal(data, &rec) == nil && rec.Tree == tree && os.Remove(name) == nil {
			removed++
		}
	}
	return removed
}

// labelEnv names the human-readable step name a caller records beside a
// pass (ASC-V0-776, V1-4242): batch-precheck keys every non-make step as
// `batch-precheck+SALT`, so the label is what tells those records apart. It is
// outside the salted prefixes and never enters a key, so a pass recorded
// without a label still hits.
const labelEnv = "GATE_LEDGER_LABEL"

// stepLabel is the invocation's GATE_LEDGER_LABEL as one line of at most 200 bytes.
func (l *ledger) stepLabel() string {
	label := strings.Join(strings.Fields(l.inv.getenv(labelEnv)), " ")
	if len(label) > 200 {
		label = strings.ToValidUTF8(label[:200], "")
	}
	return label
}
