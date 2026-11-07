// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// bigBatchRepo is gbRepo's store and gates with a batch merge M whose diff
// against its first parent changes exactly 712 paths: lane A adds 711 files
// under src/a/dNN/ and lane B adds docs/b.md.
func bigBatchRepo(t *testing.T) (root, base, laneA, merge string) {
	t.Helper()
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ = ergStore(t)
	verify := commandGate("verify", "/bin/sh", "-c", "exit 0")
	verify.Obj.Set("required", wire.Bool(true))
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, verify)
	commit := func(msg string) string {
		t.Helper()
		git(t, root, "add", "-A", "--", "src", "docs")
		git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", msg)
		return revParse(t, root, "HEAD")
	}
	out, err := exec.Command("git", "-C", root, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	branch := strings.TrimSpace(string(out))
	for _, d := range []string{"src", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fixture.Write(t, filepath.Join(root, "src", "keep.txt"), []byte("keep\n"))
	fixture.Write(t, filepath.Join(root, "docs", "keep.md"), []byte("keep\n"))
	base = commit("base")
	git(t, root, "checkout", "-q", "-b", "lane-a")
	for d := 0; d < 9; d++ {
		dir := filepath.Join(root, "src", "a", fmt.Sprintf("d%02d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < 79; f++ {
			fixture.Write(t, filepath.Join(dir, fmt.Sprintf("f%02d.txt", f)), []byte("x\n"))
		}
	}
	laneA = commit("lane a")
	git(t, root, "checkout", "-q", "-b", "lane-b", base)
	fixture.Write(t, filepath.Join(root, "docs", "b.md"), []byte("b\n"))
	laneB := commit("lane b")
	git(t, root, "checkout", "-q", branch)
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "Batch big", laneA, laneB)
	return root, base, laneA, revParse(t, root, "HEAD")
}

// TestTCDV01101_GatedBatchCloseBoundsA712PathDiff covers V1-4266. A batch
// merge of 712 changed paths is over the 256-path claim limit, so the gated
// close derives a bounded scope for each ticket instead of failing every
// claim LIMIT_EXCEEDED (TCD-V0-1101). An undeclared ticket claims the diff
// collapsed to directory keys, which the attempt (and so its claim receipt)
// records as its REQUESTED scope, and closes VERIFIED. A declared scope that
// covers the diff is claimed as is; one the diff exceeds is UNRUNNABLE
// OUT_OF_SCOPE before any write, naming --seal and --manual, never the
// claim --scope-from-diff recovery. Each row reports claimScope
// (TCD-V0-1102).
func TestTCDV01101_GatedBatchCloseBoundsA712PathDiff(t *testing.T) {
	root, base, laneA, merge := bigBatchRepo(t)
	wideID, _ := effectsTicket(t, root, "wide", "P1", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":["elsewhere/"]}`)
	coverID, _ := effectsTicket(t, root, "cover", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["docs/","src/"]}`)
	narrowID, _ := effectsTicket(t, root, "narrow", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["docs/"]}`)
	local := func(id string) string { return id[strings.LastIndex(id, ":")+1:] }
	wide, cover, narrow := local(wideID), local(coverID), local(narrowID)

	prs := tdsExport(t, "prs.json", `[
{"number":40,"state":"MERGED","title":"lane a","body":"`+tdsJSON("Refs "+wide+"\nRefs "+cover+"\nRefs "+narrow)+`","headRefOid":"`+laneA+`","labels":[]},
{"number":50,"state":"MERGED","title":"Batch big: #40","body":"- #40","headRefOid":"`+laneA+`","mergeCommit":{"oid":"`+merge+`"},"mergedAt":"2026-11-01T05:00:00Z","url":"https://github.com/beamfall/corvint/pull/50","labels":[]}]`)
	runs := tdsExport(t, "runs.json", `[{"databaseId":7,"headSha":"`+merge+`","conclusion":"success","url":"https://github.com/beamfall/corvint/actions/runs/7"}]`)
	closeArgs := []string{"ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs}
	diff, err := exec.Command("git", "-C", root, "diff-tree", "-r", "--name-only", "--no-renames", base, merge).Output()
	if n := len(strings.Fields(string(diff))); err != nil || n != 712 {
		t.Fatalf("fixture diff has %d paths, want 712: %v", n, err)
	}

	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	dry := atm(t, root, nil, append(closeArgs, "--dry-run")...)
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("a dry-run gated batch-close changed the store")
	}
	want := map[string]string{wide: "CLOSE", cover: "CLOSE", narrow: "UNRUNNABLE"}
	if got := gbDispositions(gbRows(t, dry)); !sameRows(got, want) {
		t.Fatalf("dry run %v: %s", got, dry.stdout)
	}
	wantKeys := "src/a/d00/ src/a/d01/ src/a/d02/ src/a/d03/ src/a/d04/ src/a/d05/ src/a/d06/ src/a/d07/ src/a/d08/"
	scopeOf := func(r wire.Value) (string, string, string) {
		sc := field(r, "claimScope")
		var keys []string
		for _, k := range field(sc, "keys").Arr {
			keys = append(keys, k.Str)
		}
		return field(sc, "source").Str, field(sc, "changedPaths").Str, strings.Join(keys, " ")
	}
	rows := gbRows(t, dry)
	if src, n, keys := scopeOf(rows[wide]); src != "COLLAPSED" || n != "711" || keys != wantKeys {
		t.Fatalf("collapsed claimScope %s %s %q: %s", src, n, keys, wire.Encode(rows[wide]))
	}
	if src, n, keys := scopeOf(rows[cover]); src != "DECLARED" || n != "711" || keys != "docs/ src/" {
		t.Fatalf("declared claimScope %s %s %q", src, n, keys)
	}
	if r := rows[narrow]; field(r, "code").Str != wire.CodeOutOfScope || !strings.Contains(field(r, "recovery").Str, "--seal "+narrow+"=") ||
		!strings.Contains(field(r, "recovery").Str, "--manual") || strings.Contains(field(r, "recovery").Str, "scope-from-diff") ||
		!strings.Contains(field(r, "detail").Str, "711 of the 711") {
		t.Fatalf("narrow row: %s", wire.Encode(r))
	}

	x := atm(t, root, nil, closeArgs...)
	rows = gbRows(t, x)
	want = map[string]string{wide: "CLOSED", cover: "CLOSED", narrow: "UNRUNNABLE"}
	if got := gbDispositions(rows); x.code != 1 || !sameRows(got, want) || hasCode(x.res, wire.CodeLimitExceeded) {
		t.Fatalf("gated close %v: exit %d %s", got, x.code, x.stdout)
	}
	for _, id := range []string{wideID, coverID} {
		if rec := field(bcShow(t, root, id), "record"); field(field(rec, "completion"), "kind").Str != "VERIFIED" {
			t.Fatalf("%s not VERIFIED: %s", id, wire.Encode(rec))
		}
	}
	if st := field(bcShow(t, root, narrowID), "status").Str; st != "OPEN" || field(rows[narrow], "attemptId").Kind != wire.KindNull {
		t.Fatalf("narrow ticket %s or claimed: %s", st, wire.Encode(rows[narrow]))
	}
	// The collapsed keys are the attempt's REQUESTED scope, committed by the
	// claim receipt (ATR-V0-211).
	scopeKeys := func(attempt string) (string, string) {
		t.Helper()
		a := atm(t, root, nil, "attempt", "show", attempt)
		if a.res.Outcome != wire.OutcomeOK {
			t.Fatalf("attempt show %s: %s", attempt, a.stdout)
		}
		sc := field(a.res.Items[0], "scope")
		var keys []string
		for _, r := range field(sc, "resources").Arr {
			keys = append(keys, field(r, "key").Str)
		}
		return field(sc, "source").Str, strings.Join(keys, " ")
	}
	if src, keys := scopeKeys(field(rows[wide], "attemptId").Str); src != "REQUESTED" || keys != wantKeys {
		t.Fatalf("wide attempt scope %s %q", src, keys)
	}
	if src, keys := scopeKeys(field(rows[cover], "attemptId").Str); src != "DECLARED" || keys != "docs/ src/" {
		t.Fatalf("cover attempt scope %s %q", src, keys)
	}
	if audit := atm(t, root, nil, "receipt", "audit"); audit.code != 0 || field(field(audit.res.Items[0], "completions"), "verified").Str != "2" {
		t.Fatalf("receipt audit: %s", audit.stdout)
	}
}
