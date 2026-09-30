// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"bytes"
	"context"
)

// Immutable committed and current index gitlinks are refused at admission.
// Subsequent index-only staging is not a provider input closure claim; committed
// changes are caught by the resolved commit/tree brackets on every request.
func admitWorkspace(ctx context.Context, root string) (workspaceStamp, error) {
	before, err := observeWorkspaceGit(ctx, root)
	if err != nil {
		return workspaceStamp{}, err
	}
	for _, args := range [][]string{{"ls-tree", "-r", "-z", "--full-tree", before.tree}, {"ls-files", "--stage", "-z"}} {
		raw, err := boundedGitOutput(ctx, root, 8<<20, args...)
		if err != nil || !admittedGitModes(raw) {
			return workspaceStamp{}, errWorkspaceObservation
		}
	}
	stamp, err := observeWorkspace(ctx, root)
	if err != nil || !sameWorkspaceGit(before, stamp.repository) {
		return workspaceStamp{}, errWorkspaceObservation
	}
	return stamp, nil
}
func admittedGitModes(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	if raw[len(raw)-1] != 0 {
		return false
	}
	records := bytes.Split(raw[:len(raw)-1], []byte{0})
	if len(records) > 100000 {
		return false
	}
	for _, record := range records {
		tab := bytes.IndexByte(record, '\t')
		if tab < 0 || tab == len(record)-1 || tab < 7 {
			return false
		}
		mode := string(record[:6])
		if record[6] != ' ' || (mode != "100644" && mode != "100755" && mode != "120000") {
			return false
		}
	}
	return true
}
