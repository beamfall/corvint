// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCXV0971_BatchCloseRefreshesIndexBeforeFirstRow: a gated batch-close
// writes the derived closed-ticket index before its first row, so the rows'
// lease writes model against it (CAL-V0-971); a --dry-run writes none. The
// required gate itself proves it: it passes only while the index file exists.
func TestTCXV0971_BatchCloseRefreshesIndexBeforeFirstRow(t *testing.T) {
	parallelTest(t) // seam-carried actor and qualified versions; no env, cwd, hook, package-variable or in-process executable writes (V1-4208)
	setActor(t, "tester")
	root, _ := ergStore(t)
	indexPath := intent.ClosedIndexPath(filepath.Join(root, ".git", "taskman"))
	verify := commandGate("verify", "/bin/sh", "-c", "test -f "+indexPath)
	verify.Obj.Set("required", wire.Bool(true))
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, verify)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	commit := func(msg string) string {
		t.Helper()
		git(t, root, "add", "-A", "--", "src")
		git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", msg)
		return revParse(t, root, "HEAD")
	}
	fixture.Write(t, filepath.Join(root, "src", "keep.txt"), []byte("keep\n"))
	commit("base")
	branch := strings.TrimSpace(gitOut(t, root, "symbolic-ref", "--short", "HEAD"))
	git(t, root, "checkout", "-q", "-b", "lane-a")
	fixture.Write(t, filepath.Join(root, "src", "a.go"), []byte("package src\n"))
	laneA := commit("lane a")
	git(t, root, "checkout", "-q", branch)
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "Batch z", laneA)
	merge := revParse(t, root, "HEAD")
	wholeID, _ := effectsTicket(t, root, "whole", "P1", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":["elsewhere/"]}`)
	whole := wholeID[strings.LastIndex(wholeID, ":")+1:]
	prs := tdsExport(t, "prs.json", `[
{"number":40,"state":"MERGED","title":"lane a","body":"`+tdsJSON("Refs "+whole)+`","headRefOid":"`+laneA+`","labels":[]},
{"number":50,"state":"MERGED","title":"Batch z: #40","body":"- #40","headRefOid":"`+laneA+`","mergeCommit":{"oid":"`+merge+`"},"mergedAt":"2026-10-28T05:00:00Z","url":"https://github.com/beamfall/corvint/pull/50","labels":[]}]`)
	runs := tdsExport(t, "runs.json", `[{"databaseId":7,"headSha":"`+merge+`","conclusion":"success","url":"https://github.com/beamfall/corvint/actions/runs/7"}]`)
	closeArgs := []string{"ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs}
	// The create above refreshed the index after it committed; the batch
	// close must not depend on that.
	if err := os.Remove(indexPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	dry := atm(t, root, nil, append(closeArgs, "--dry-run")...)
	if got := gbDispositions(gbRows(t, dry)); got[whole] != "CLOSE" {
		t.Fatalf("dry run %v: %s", got, dry.stdout)
	}
	if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
		t.Fatalf("a --dry-run wrote the index: %v", err)
	}
	x := atm(t, root, nil, closeArgs...)
	rows := gbRows(t, x)
	if d := field(rows[whole], "disposition").Str; d != "CLOSED" || field(rows[whole], "completionKind").Str != "VERIFIED" {
		t.Fatalf("the gate did not find the index before the first row: %s", x.stdout)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("index after the close: %v", err)
	}
}
