package store

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// WarningExactDirectoryKey prefixes the CAL-V0-211 warning, so a caller can
// match it without parsing the prose that follows.
const WarningExactDirectoryKey = "EXACT_KEY_NAMES_DIRECTORY"

// ExactDirectoryKeyWarnings is one CAL-V0-211 warning, in key order, for
// each PATH key in keys that does not end in "/" and names a directory in
// the tree of rev in the checkout at root. Under CAL-V0-021 such a key covers
// that one path and nothing beneath it. The check only advises: when the tree
// cannot be read it returns no warning rather than an error.
func ExactDirectoryKeyWarnings(root, rev string, keys []string) []string {
	exact := map[string]bool{}
	for _, k := range keys {
		if k != "" && !strings.HasSuffix(k, "/") && !strings.ContainsAny(k, "\n\r") {
			exact[k] = true
		}
	}
	if len(exact) == 0 {
		return nil
	}
	tree, err := resolveObject(root, rev, "tree")
	if err != nil {
		return nil
	}
	sorted := make([]string, 0, len(exact))
	for k := range exact {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	var in bytes.Buffer
	for _, k := range sorted {
		in.WriteString(tree + ":" + k + "\n")
	}
	c := exec.Command("git", "-c", "credential.helper=", "cat-file", "--batch-check=%(objecttype)")
	c.Dir, c.Env, c.Stdin = root, gitEnvironment(), &in
	out, err := c.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != len(sorted) {
		return nil
	}
	var warnings []string
	for i, k := range sorted {
		if lines[i] == "tree" {
			warnings = append(warnings, fmt.Sprintf("%s: PATH key %s names a directory in tree %s but covers only that exact path, not the paths beneath it (CAL-V0-021); use %s/ to cover them",
				WarningExactDirectoryKey, k, tree, k))
		}
	}
	return warnings
}

// effectsPathKeys is every touchPath and PATH resource key effects e name,
// whatever their coverage.
func effectsPathKeys(e ticket.Effects) []string {
	keys := append([]string{}, e.TouchPaths...)
	return append(keys, pathKeys(e.Resources)...)
}

// EffectsDirectoryKeyWarnings is ExactDirectoryKeyWarnings for the PATH
// keys effects e declare, read against rev in the checkout at root.
func EffectsDirectoryKeyWarnings(root, rev string, e ticket.Effects) []string {
	return ExactDirectoryKeyWarnings(root, rev, effectsPathKeys(e))
}

// scopeDirectoryKeyWarnings is ExactDirectoryKeyWarnings for the PATH keys
// of a claimed attempt's scope, read against its base commit.
func scopeDirectoryKeyWarnings(root string, a *snapshot.Attempt) []string {
	if a.Scope == nil {
		return nil
	}
	return ExactDirectoryKeyWarnings(root, a.BaseCommit, pathKeys(a.Scope.Resources))
}

func pathKeys(resources []ticket.Resource) []string {
	var keys []string
	for _, r := range resources {
		if r.Class == "PATH" {
			keys = append(keys, r.Key)
		}
	}
	return keys
}
