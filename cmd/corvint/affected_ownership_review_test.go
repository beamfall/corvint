// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AZU-V0-9900: staging changes membership without changing dirty path names
// or HEAD. Refuse a plan compiled across that change.
func TestOwnershipIndexDriftAfterSources_AZU_V0_9900(t *testing.T) {
	root, base := ratchetSelectionRepo(t)
	cemWrite(t, root, "data/new.txt", "new\n")
	t.Cleanup(func() { affectedPlanSourcesObserved = func() {} })
	affectedPlanSourcesObserved = func() { affectedGit(t, root, "add", "data/new.txt") }
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "affected", "--base", base}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported-affected-drift") || !strings.Contains(stderr.String(), "worktree changed while the plan was compiled") {
		t.Fatalf("membership drift accepted: code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
	affectedPlanSourcesObserved = func() {}
	assertRatchetSelected(t, ratchetPlan(t, root, base))
}

// AZU-V0-9901: text diagnostics guide existing committed unowned edits even
// without the ratchet. JSON compatibility remains covered by the frozen and
// content-only byte regressions.
func TestOwnershipTextGuidance_AZU_V0_9901(t *testing.T) {
	for _, ratchet := range []bool{false, true} {
		t.Run(map[bool]string{false: "without-ratchet", true: "with-ratchet"}[ratchet], func(t *testing.T) {
			root, base := ratchetSelectionRepo(t)
			if !ratchet {
				affectedGit(t, root, "rm", affected.OwnershipAuditTestPath)
				affectedGit(t, root, "commit", "-qm", "without ratchet")
				base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
			}
			cemWrite(t, root, "data/old.txt", "edited committed data\n")
			affectedGit(t, root, "add", "data/old.txt")
			affectedGit(t, root, "commit", "-qm", "content only")
			var stdout, stderr bytes.Buffer
			code := runContext(context.Background(), []string{"--root", root, "affected", "--base", base, "--format", "text"}, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("code=%d stderr=%s", code, &stderr)
			}
			for _, want := range []string{"unknown UNOWNED_DIRTY_PATH data/old.txt", "runtime/test inputs in " + affected.RuntimeReadersPath, "documentation in " + affected.DocumentationPathsPath, "evidence/output in " + affected.NonSourcePathsPath, "AZU-V0-9901"} {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("missing %q: %s", want, &stdout)
				}
			}
		})
	}
}
