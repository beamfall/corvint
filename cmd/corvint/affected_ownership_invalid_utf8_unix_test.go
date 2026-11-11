//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// requireRawFilenames skips when the temp filesystem refuses non-UTF-8 names
// (APFS on macOS returns EILSEQ), so no such Git path can exist there (V1-4799).
// Any other failure still fails the test, and Linux, which accepts the name,
// runs it.
func requireRawFilenames(t *testing.T) {
	t.Helper()
	err := os.WriteFile(filepath.Join(t.TempDir(), "probe\xff"), nil, 0o600)
	if reason := rawFilenameSkipReason(err); reason != "" {
		t.Skip(reason)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// rawFilenameSkipReason names why a raw-filename test skips, or returns ""
// when err is not the filesystem refusing a non-UTF-8 name with EILSEQ.
func rawFilenameSkipReason(err error) string {
	if !errors.Is(err, syscall.EILSEQ) {
		return ""
	}
	return "the filesystem rejects non-UTF-8 file names with EILSEQ (APFS on macOS does), so no raw Git filename can be checked out here: " + err.Error()
}

// V1-4799: only EILSEQ skips; another error or none runs the test.
func TestRawFilenameSkipReasonOnlyForEILSEQ(t *testing.T) {
	rejected := &os.PathError{Op: "open", Path: "probe\xff", Err: syscall.EILSEQ}
	if reason := rawFilenameSkipReason(rejected); !strings.Contains(reason, "EILSEQ") || !strings.Contains(reason, rejected.Error()) {
		t.Fatalf("EILSEQ skip reason = %q", reason)
	}
	for _, err := range []error{nil, &os.PathError{Op: "open", Path: "probe\xff", Err: syscall.EACCES}, errors.New("illegal byte sequence")} {
		if reason := rawFilenameSkipReason(err); reason != "" {
			t.Fatalf("%v skips with %q, want the test to run", err, reason)
		}
	}
}

// AZU-V0-9900: Git names retain raw bytes while dirty paths replace invalid
// UTF-8. Hidden paths reach membership selection without a source-walk refusal.
func TestOwnershipInvalidUTF8Membership_AZU_V0_9900(t *testing.T) {
	requireRawFilenames(t)
	for _, change := range []string{"add", "stage", "delete", "worktree-delete", "replace", "content-only"} {
		t.Run(change, func(t *testing.T) {
			root, base := ratchetSelectionRepo(t)
			raw := ".metadata/name\xff.txt"
			display := strings.ToValidUTF8(raw, "\uFFFD")
			if change != "add" && change != "stage" {
				cemWrite(t, root, raw, "old\n")
				affectedGit(t, root, "add", ".")
				affectedGit(t, root, "commit", "-qm", "raw filename base")
				base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
			}
			switch change {
			case "delete", "worktree-delete", "replace":
				if err := os.Remove(filepath.Join(root, raw)); err != nil {
					t.Fatal(err)
				}
				if change == "replace" {
					// Both names normalize to the same dirty path, but raw
					// membership still changes.
					cemWrite(t, root, ".metadata/name\xfe.txt", "new\n")
				}
			default:
				cemWrite(t, root, raw, "new\n")
			}
			if change != "worktree-delete" {
				affectedGit(t, root, "add", ".")
				if change != "stage" {
					affectedGit(t, root, "commit", "-qm", "raw filename change")
				}
			}
			p := ratchetPlan(t, root, base)
			if !slices.Contains(p.Dirty, display) {
				t.Fatalf("normalized dirty path absent: %q", p.Dirty)
			}
			if change == "content-only" {
				if slices.ContainsFunc(p.Selected, func(s affected.Selection) bool {
					return slices.Contains(s.Tests, affected.OwnershipAuditTestPath)
				}) {
					t.Fatalf("content-only edit selected ratchet: %+v", p.Selected)
				}
				return
			}
			assertRatchetSelected(t, p)
			if !slices.ContainsFunc(p.Selected, func(s affected.Selection) bool {
				return slices.Contains(s.Tests, affected.OwnershipAuditTestPath) && s.Witness.DirtyPath == display
			}) {
				t.Fatalf("ratchet trigger does not name dirty path %q: %+v", display, p.Selected)
			}
		})
	}
}

// AZU-V0-9900: a raw index rename that preserves the displayed dirty path
// still invalidates the membership observation.
func TestOwnershipInvalidUTF8IndexDrift_AZU_V0_9900(t *testing.T) {
	requireRawFilenames(t)
	root, _ := ratchetSelectionRepo(t)
	raw := ".metadata/name\xff.txt"
	cemWrite(t, root, raw, "old\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "raw filename base")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	cemWrite(t, root, raw, "new\n")
	t.Cleanup(func() { affectedPlanSourcesObserved = func() {} })
	affectedPlanSourcesObserved = func() {
		if err := os.Rename(filepath.Join(root, raw), filepath.Join(root, ".metadata/name\xfe.txt")); err != nil {
			t.Fatal(err)
		}
		affectedGit(t, root, "add", ".")
	}
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "affected", "--base", base}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported-affected-drift") || !strings.Contains(stderr.String(), "worktree changed while the plan was compiled") {
		t.Fatalf("raw membership drift accepted: code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
}
