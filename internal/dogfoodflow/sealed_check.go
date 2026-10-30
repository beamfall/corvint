// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gitancestry"
)

// SealBinding distinguishes the current seal from the unchanged evidence at its parent.
// It is derived from Git; callers cannot choose an arbitrary evidence target.
type SealBinding struct {
	Target         string `json:"target"`
	EvidenceTarget string `json:"evidenceTarget"`
	MapPath        string `json:"mapPath"`
	Blob           string `json:"blob"`
}

// ResolveSealBinding admits only a single-parent, byte- and mode-preserving move.
func ResolveSealBinding(ctx context.Context, root, target string) (SealBinding, error) {
	var binding SealBinding
	f := &flow{ctx: ctx, root: root, stderr: io.Discard}
	code, err := boundary(ctx, func() int {
		if !wire.IsGitOid(target) {
			return 2
		}
		common, status := f.git(true, "rev-parse", "--git-common-dir")
		if status != 0 {
			return 2
		}
		if !strings.HasPrefix(strings.TrimSpace(common), "/") {
			common = root + "/" + strings.TrimSpace(common)
		}
		if gitancestry.RewrittenAt(strings.TrimSpace(common)) != "" {
			return 2
		}
		parents, status := f.git(true, "rev-list", "--parents", "-n", "1", target)
		fields := strings.Fields(parents)
		if status != 0 || len(fields) != 2 || fields[0] != target || !wire.IsGitOid(fields[1]) {
			return 2
		}
		parent := fields[1]
		archive := ".corvint/changes/" + parent + ".cem.json"
		delta, status := f.git(true, "diff-tree", "-r", "--no-renames", "--no-commit-id", "--name-status", parent, target)
		if status != 0 || strings.TrimSpace(delta) != "D\t.corvint/change.cem.json\nA\t"+archive {
			return 2
		}
		old, status := f.git(true, "ls-tree", parent, "--", ".corvint/change.cem.json")
		if status != 0 {
			return 2
		}
		now, status := f.git(true, "ls-tree", target, "--", archive)
		a, b := strings.Fields(old), strings.Fields(now)
		if status != 0 || len(a) != 4 || len(b) != 4 || (a[0] != "100644" && a[0] != "100755") || a[0] != b[0] || a[1] != "blob" || b[1] != "blob" || a[2] != b[2] {
			return 2
		}
		binding = SealBinding{Target: target, EvidenceTarget: parent, MapPath: archive, Blob: a[2]}
		return 0
	}, func() {})
	if err != nil {
		return SealBinding{}, err
	}
	if code != 0 {
		return SealBinding{}, errors.New("sealed-binding-required")
	}
	return binding, nil
}

// CheckSealed runs the real checker on a strictly derived parent evidence binding.
// Ordinary Check retains its sealed-head refusal. Cached check passes are not reused.
func CheckSealed(ctx context.Context, options CheckOptions, stdout, stderr io.Writer) (int, error) {
	options.ReusePass = false
	run := newCheck(ctx, "dogfood-check", options, stdout, stderr)
	run.sealed = true
	defer run.releaseOperation()
	return boundary(ctx, run.run, func() {})
}
