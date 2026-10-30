package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gbRows maps local ticket token to a gated batch-close row.
func gbRows(t *testing.T, x run) map[string]wire.Value {
	t.Helper()
	if len(x.res.Items) != 1 || field(x.res.Items[0], "profile").Str != "taskman-batch-close/1" || field(x.res.Items[0], "mode").Str != "GATED" {
		t.Fatalf("gated batch-close item: %s", x.stdout)
	}
	out := map[string]wire.Value{}
	for _, r := range field(x.res.Items[0], "tickets").Arr {
		out[field(r, "local").Str] = r
	}
	return out
}

func gbDispositions(rows map[string]wire.Value) map[string]string {
	out := map[string]string{}
	for k, r := range rows {
		out[k] = field(r, "disposition").Str
	}
	return out
}

// gbRepo is a store with a required local gate `verify`, a failing gate
// `broken` and a gate `remote` this runner cannot execute, and a batch merge
// M on the intent branch of lane A (src/a.go) and lane B (docs/b.md).
func gbRepo(t *testing.T) (root, base, laneA, merge string) {
	t.Helper()
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ = ergStore(t)
	verify := commandGate("verify", "/bin/sh", "-c", "exit 0")
	verify.Obj.Set("required", wire.Bool(true))
	remote := commandGate("remote", "/bin/sh", "-c", "exit 0")
	remote.Obj.Set("cwd", wire.String("CANDIDATE"))
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, verify, commandGate("broken", "/bin/sh", "-c", "exit 1"), remote)
	for _, d := range []string{"src", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
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
	fixture.Write(t, filepath.Join(root, "src", "keep.txt"), []byte("keep\n"))
	fixture.Write(t, filepath.Join(root, "docs", "keep.md"), []byte("keep\n"))
	base = commit("base")
	git(t, root, "checkout", "-q", "-b", "lane-a")
	fixture.Write(t, filepath.Join(root, "src", "a.go"), []byte("package src\n"))
	laneA = commit("lane a")
	git(t, root, "checkout", "-q", "-b", "lane-b", base)
	fixture.Write(t, filepath.Join(root, "docs", "b.md"), []byte("b\n"))
	laneB := commit("lane b")
	git(t, root, "checkout", "-q", branch)
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "Batch z", laneA, laneB)
	return root, base, laneA, revParse(t, root, "HEAD")
}

// TestTCDV0185_GatedBatchCloseRunsRequiredGates covers V1-2984. Without
// --manual, batch-close closes each delivered ticket through claim, submit,
// every required gate and complete, so the completion is VERIFIED; it never
// records COMPLETE_MANUAL (TCD-V0-185). A sealed lane's own diff is the
// candidate and a declared scope is never widened (TCD-V0-186). A gate that
// cannot run here refuses its row UNRUNNABLE before any write, and a failing
// gate leaves the ticket OPEN with its attempt released (TCD-V0-187,
// TCD-V0-189). The batch merge identity is attached as the
// taskman-batch-merge/0 document's digest (TCD-V0-188). A rerun skips the
// closed tickets and claims afresh; --manual still closes the rest manually
// and the receipt audit counts the two kinds apart (TCD-V0-190).
func TestTCDV0185_GatedBatchCloseRunsRequiredGates(t *testing.T) {
	root, base, laneA, merge := gbRepo(t)
	scopedID, _ := effectsTicket(t, root, "scoped", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`)
	wholeID, _ := effectsTicket(t, root, "whole", "P1", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":["elsewhere/"]}`)
	narrowID, _ := effectsTicket(t, root, "narrow", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["docs/"]}`)
	gated := func(title, gate string) string {
		t.Helper()
		payload := strings.NewReplacer(`"title":"Console ticket"`, `"title":"`+title+`"`, `"requiredGates":[]`, `"requiredGates":["`+gate+`"]`,
			`"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]}`,
			`"effects":{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":["elsewhere/"]}`).Replace(createPayloadJSON)
		x := atm(t, root, nil, "ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-10-08T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %s: %s", title, x.stdout)
		}
		return field(x.res.Items[0], "ticketId").Str
	}
	unrunID, brokenID := gated("unrun", "remote"), gated("broken", "broken")
	local := func(id string) string { return id[strings.LastIndex(id, ":")+1:] }
	scoped, whole, narrow, unrun, broken := local(scopedID), local(wholeID), local(narrowID), local(unrunID), local(brokenID)

	prURL, runURL := "https://github.com/beamfall/corvint/pull/50", "https://github.com/beamfall/corvint/actions/runs/7"
	prs := tdsExport(t, "prs.json", `[
{"number":40,"state":"MERGED","title":"lane a","body":"`+tdsJSON("Refs "+scoped+"\nRefs "+whole)+`","headRefOid":"`+laneA+`","labels":[]},
{"number":41,"state":"MERGED","title":"lane b","body":"`+tdsJSON("Refs "+narrow+"\nRefs "+unrun+"\nRefs "+broken)+`","headRefOid":"`+base+`","labels":[]},
{"number":50,"state":"MERGED","title":"Batch z: #40 #41","body":"- #40\n- #41","headRefOid":"`+laneA+`","mergeCommit":{"oid":"`+merge+`"},"mergedAt":"2026-10-28T05:00:00Z","url":"`+prURL+`","labels":[]}]`)
	runs := tdsExport(t, "runs.json", `[{"databaseId":7,"headSha":"`+merge+`","conclusion":"success","url":"`+runURL+`"}]`)
	closeArgs := []string{"ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs, "--seal", scoped + "=" + laneA}

	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	for _, bad := range [][]string{
		{"ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs, "--seal", "nobody=" + laneA},
		{"ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs, "--seal", scoped},
		{"ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs, "--manual", "--seal", scoped + "=" + laneA},
	} {
		if x := atm(t, root, nil, bad...); !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("%v: %s", bad, x.stdout)
		}
	}
	dry := atm(t, root, nil, append(closeArgs, "--dry-run")...)
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("a refused or dry-run gated batch-close changed the store")
	}
	want := map[string]string{scoped: "CLOSE", whole: "CLOSE", narrow: "CLOSE", unrun: "UNRUNNABLE", broken: "CLOSE"}
	if got := gbDispositions(gbRows(t, dry)); !sameRows(got, want) {
		t.Fatalf("dry run %v: %s", got, dry.stdout)
	}
	if item := dry.res.Items[0]; field(item, "prUrl").Str != prURL || field(item, "mergedAt").Str != "2026-10-28T05:00:00Z" {
		t.Fatalf("header: %s", dry.stdout)
	}

	x := atm(t, root, nil, closeArgs...)
	rows := gbRows(t, x)
	want = map[string]string{scoped: "CLOSED", whole: "CLOSED", narrow: "FAILED", unrun: "UNRUNNABLE", broken: "FAILED"}
	if got := gbDispositions(rows); x.code != 1 || !sameRows(got, want) || !hasCode(x.res, wire.CodeOutOfScope) || !hasCode(x.res, wire.CodeGateFailed) {
		t.Fatalf("gated close %v: exit %d %s", got, x.code, x.stdout)
	}
	if r := rows[scoped]; field(r, "candidateCommit").Str != laneA || field(r, "baseCommit").Str != base || field(r, "sealCommit").Str != laneA || field(r, "completionKind").Str != "VERIFIED" || field(r, "requestId").Str != "batch-close-50-"+scoped+"-a0-complete" {
		t.Fatalf("sealed row: %s", wire.Encode(r))
	}
	if r := rows[whole]; field(r, "candidateCommit").Str != merge || field(r, "baseCommit").Str != base || field(r, "sealCommit").Kind != wire.KindNull {
		t.Fatalf("merge row: %s", wire.Encode(r))
	}
	if r := rows[narrow]; field(r, "step").Str != "claim" || !strings.Contains(field(r, "recovery").Str, "--seal "+narrow+"=") {
		t.Fatalf("out-of-scope row: %s", wire.Encode(r))
	}
	if r := rows[unrun]; !strings.Contains(field(r, "detail").Str, "remote") || !strings.Contains(field(r, "recovery").Str, "--manual") || field(r, "receipt").Kind != wire.KindNull {
		t.Fatalf("unrunnable row: %s", wire.Encode(r))
	}
	if r := rows[broken]; field(r, "step").Str != "gate" || field(r, "code").Str != wire.CodeGateFailed || !strings.Contains(field(r, "detail").Str, "released") {
		t.Fatalf("failed-gate row: %s", wire.Encode(r))
	}
	for _, id := range []string{scopedID, wholeID} {
		rec := field(bcShow(t, root, id), "record")
		if field(field(rec, "completion"), "kind").Str != "VERIFIED" {
			t.Fatalf("%s not VERIFIED: %s", id, wire.Encode(rec))
		}
		ev := rows[local(id)]
		me := field(ev, "mergeEvidence")
		doc := field(me, "document")
		if string(wire.Sum(wire.EncodeFile(doc))) != field(me, "sha256").Str || field(doc, "prUrl").Str != prURL || field(doc, "mergeCommit").Str != merge || field(doc, "runUrl").Str != runURL || field(doc, "lanePr").Str != "40" {
			t.Fatalf("merge evidence: %s", wire.Encode(me))
		}
		if !strings.Contains(string(wire.Encode(field(rec, "attachedEvidence"))), field(me, "sha256").Str) {
			t.Fatalf("merge evidence not attached: %s", wire.Encode(rec))
		}
	}
	for _, id := range []string{narrowID, unrunID, brokenID} {
		if st := field(bcShow(t, root, id), "status").Str; st != "OPEN" {
			t.Fatalf("%s is %s after a refused gated close", id, st)
		}
	}

	again := atm(t, root, nil, closeArgs...)
	want[scoped], want[whole] = "ALREADY_COMPLETED", "ALREADY_COMPLETED"
	if got, r := gbDispositions(gbRows(t, again)), gbRows(t, again)[broken]; again.code != 1 || !sameRows(got, want) || field(r, "code").Str != wire.CodeGateFailed ||
		!strings.HasPrefix(field(r, "requestId").Str, "batch-close-50-"+broken+"-a1-") {
		t.Fatalf("rerun %v: %s", got, again.stdout)
	}

	manual := atm(t, root, nil, "ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs, "--manual")
	if got := bcRows(t, manual); manual.code != 0 || !sameRows(got, map[string]string{scoped: "ALREADY_COMPLETED", whole: "ALREADY_COMPLETED", narrow: "CLOSED", unrun: "CLOSED", broken: "CLOSED"}) {
		t.Fatalf("--manual %v: %s", got, manual.stdout)
	}
	audit := atm(t, root, nil, "receipt", "audit")
	c := field(audit.res.Items[0], "completions")
	if audit.code != 0 || field(c, "verified").Str != "2" || field(c, "manual").Str != "3" {
		t.Fatalf("receipt audit completions: %s", audit.stdout)
	}
}
