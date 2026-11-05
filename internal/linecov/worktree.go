// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// WorkingDiff uses the recorded commit as the old side, including staged,
// unstaged and untracked inputs without writing Git objects (LPCV-V0-7607).
func WorkingDiff(ctx context.Context, root, commit string) ([]FileDiff, error) {
	raw, err := gitOut(ctx, root, "diff", "--raw", "-p", "-U0", "--no-renames", "--no-ext-diff", "--no-abbrev", "--full-index", "--no-color", commit, "--")
	if err != nil {
		return nil, err
	}
	diffs, err := parseDiff(raw)
	if err != nil {
		return nil, err
	}
	others, err := gitOut(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	// Ignored Go source still participates in go test, so it must not hide
	// working-tree initialization or newly declared tests from selection.
	ignoredGo, err := gitOut(ctx, root, "ls-files", "--others", "--ignored", "--exclude-standard", "-z", "--", "*.go")
	if err != nil {
		return nil, err
	}
	others = append(others, ignoredGo...)
	for _, p := range strings.Split(string(others), "\x00") {
		if p != "" {
			diffs = append(diffs, FileDiff{Path: p})
		}
	}
	for i := range diffs {
		d := &diffs[i]
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(d.Path)))
		if os.IsNotExist(err) {
			d.NewOID = ""
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			d.NewOID = "non-regular"
			continue
		}
		oid, err := gitOut(ctx, root, "hash-object", "--", d.Path)
		if err != nil {
			return nil, err
		}
		d.NewOID = strings.TrimSpace(string(oid))
	}
	return diffs, nil
}
