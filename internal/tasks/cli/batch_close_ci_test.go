package cli_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gbCIPolicy adds to the store's policy, as version 4, the gate full: a
// command that fails if it ever runs, admitted in ciEvidence only for a
// green run of the workflow CI of beamfall/corvint (TCD-V0-154).
func gbCIPolicy(t *testing.T, root string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".taskman", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	gates, _ := v.Obj.Get("gates")
	v.Obj.Set("gates", wire.Array(append(slices.Clone(gates.Arr), commandGate("full", "/bin/sh", "-c", "exit 1"))...))
	v.Obj.Set("policyVersion", wire.String("4"))
	entry := wire.NewObject().Set("gateId", wire.String("full")).Set("repository", wire.String("beamfall/corvint")).Set("requiredChecks", wire.Strings([]string{"CI"}))
	v.Obj.Set("ciEvidence", wire.ObjectValue(wire.NewObject().Set("gates", wire.Array(wire.ObjectValue(entry)))))
	path := filepath.Join(t.TempDir(), "policy-4.json")
	fixture.Write(t, path, wire.EncodeFile(v))
	if x := atm(t, root, nil, "policy", "update", "--request-id", "policy-4", "--expected-policy-version", "3", "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %s", x.stdout)
	}
}

// gbGate is the gates entry of a gated row for gate id, or fails.
func gbGate(t *testing.T, row wire.Value, id string) wire.Value {
	t.Helper()
	for _, g := range field(row, "gates").Arr {
		if field(g, "gateId").Str == id {
			return g
		}
	}
	t.Fatalf("no gate %s in %s", id, wire.Encode(row))
	return wire.Value{}
}

// TestTCDV0770_BatchCloseRecordsCIGateFromGreenPushRun covers NEW-D2 (the
// 2026-10-21 owner ruling: a batch checked in the cloud needs no local gate).
// A required gate the policy admits in ciEvidence is recorded from the --runs
// export: the highest-databaseId push run at the batch merge of its one
// required workflow (TCD-V0-770). A merge-queue run, a run of another
// workflow, an export without event and workflowName, another repository's
// run and a failed push run never satisfy it: the row is UNRUNNABLE before
// any write and the gate never runs locally (TCD-V0-771). Each gates entry
// names its ciRunId, and the record the store retains is exactly the derived
// taskman-ci-run/0 bytes (TCD-V0-772). The gate's command exits 1, so a
// VERIFIED close proves it never executed.
func TestTCDV0770_BatchCloseRecordsCIGateFromGreenPushRun(t *testing.T) {
	parallelTest(t) // seam-carried actor; no env, cwd, hook, package-variable or in-process executable writes
	root, _, laneA, merge := gbRepo(t)
	gbCIPolicy(t, root)
	payload := strings.NewReplacer(`"title":"Console ticket"`, `"title":"cloud"`, `"requiredGates":[]`, `"requiredGates":["full"]`,
		`"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]}`,
		`"effects":{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":["elsewhere/"]}`).Replace(createPayloadJSON)
	created := atm(t, root, nil, "ticket", "create", "--request-id", "create-cloud", "--issued-at", "2026-10-08T12:00:00Z", "--payload", payload)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %s", created.stdout)
	}
	ticketID := field(created.res.Items[0], "ticketId").Str
	local := ticketID[strings.LastIndex(ticketID, ":")+1:]
	prs := tdsExport(t, "prs.json", `[
{"number":40,"state":"MERGED","title":"lane a","body":"`+tdsJSON("Refs "+local)+`","headRefOid":"`+laneA+`","labels":[]},
{"number":50,"state":"MERGED","title":"Batch z: #40","body":"- #40","headRefOid":"`+laneA+`","mergeCommit":{"oid":"`+merge+`"},"mergedAt":"2026-10-28T05:00:00Z","url":"https://github.com/beamfall/corvint/pull/50","labels":[]}]`)
	run := func(id, sha, conclusion, workflow, event, repo string) string {
		return `{"databaseId":` + id + `,"headSha":"` + sha + `","conclusion":"` + conclusion + `","url":"https://github.com/` + repo + `/actions/runs/` + id + `","workflowName":"` + workflow + `","event":"` + event + `","updatedAt":"2026-10-28T05:10:0` + id[len(id)-1:] + `Z"}`
	}
	intake := run("11", merge, "success", "Intake containment", "push", "beamfall/corvint")
	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	tmpBefore, _ := filepath.Glob(filepath.Join(os.TempDir(), "corvint-batch-close-ci-*"))

	for _, c := range []struct{ name, runs, code string }{
		{"merge-queue run only", `[` + run("9", merge, "success", "CI", "merge_group", "beamfall/corvint") + `,` + intake + `]`, wire.CodePrerequisiteUnsatisfied},
		{"pull-request run only", `[` + run("8", merge, "success", "CI", "pull_request", "beamfall/corvint") + `,` + intake + `]`, wire.CodePrerequisiteUnsatisfied},
		{"export without event", `[{"databaseId":7,"headSha":"` + merge + `","conclusion":"success","url":"https://github.com/beamfall/corvint/actions/runs/7"}]`, wire.CodePrerequisiteUnsatisfied},
		{"another repository", `[` + run("7", merge, "success", "CI", "push", "fork/corvint") + `]`, wire.CodeGateStale},
		{"failed push run", `[` + run("7", merge, "success", "CI", "push", "beamfall/corvint") + `,` + run("12", merge, "failure", "CI", "push", "beamfall/corvint") + `,` + intake + `]`, wire.CodeGateFailed},
		{"push run of another commit", `[` + run("7", laneA, "success", "CI", "push", "beamfall/corvint") + `,` + intake + `]`, wire.CodePrerequisiteUnsatisfied},
	} {
		beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
		x := atm(t, root, nil, "ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", tdsExport(t, "runs.json", c.runs))
		r := gbRows(t, x)[local]
		g := gbGate(t, r, "full")
		if x.code != 1 || field(r, "disposition").Str != "UNRUNNABLE" || field(r, "code").Str != c.code ||
			field(g, "via").Str != "NONE" || field(g, "state").Str != "UNRUNNABLE" || field(g, "ciRunId").Kind != wire.KindNull ||
			!strings.Contains(field(r, "recovery").Str, "workflowName,event,updatedAt") || !strings.Contains(field(r, "recovery").Str, "never run locally") {
			t.Fatalf("%s: exit %d %s", c.name, x.code, x.stdout)
		}
		if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
			t.Fatalf("%s: an UNRUNNABLE gated batch-close changed the store", c.name)
		}
	}

	runs := tdsExport(t, "runs.json", `[`+run("7", merge, "success", "CI", "push", "beamfall/corvint")+`,`+run("9", merge, "success", "CI", "merge_group", "beamfall/corvint")+`,`+intake+`]`)
	closeArgs := []string{"ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs}
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	dry := atm(t, root, nil, append(closeArgs, "--dry-run")...)
	r := gbRows(t, dry)[local]
	if g, v := gbGate(t, r, "full"), gbGate(t, r, "verify"); dry.code != 0 || field(r, "disposition").Str != "CLOSE" || field(g, "via").Str != "CI" || field(g, "state").Str != "PLANNED" || field(g, "ciRunId").Str != "7" ||
		field(v, "via").Str != "LOCAL" || field(v, "ciRunId").Kind != wire.KindNull {
		t.Fatalf("dry run: exit %d %s", dry.code, dry.stdout)
	}
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("a dry-run gated batch-close changed the store")
	}

	x := atm(t, root, nil, closeArgs...)
	r = gbRows(t, x)[local]
	if g, v := gbGate(t, r, "full"), gbGate(t, r, "verify"); x.code != 0 || field(r, "disposition").Str != "CLOSED" || field(r, "completionKind").Str != "VERIFIED" ||
		field(g, "via").Str != "CI" || field(g, "state").Str != "REUSED" || field(g, "ciRunId").Str != "7" || field(v, "state").Str != "PASSED" {
		t.Fatalf("gated close: exit %d %s", x.code, x.stdout)
	}
	if doc := field(field(r, "mergeEvidence"), "document"); field(doc, "runUrl").Str != "https://github.com/beamfall/corvint/actions/runs/11" {
		t.Fatalf("the merge document's cited run changed: %s", wire.Encode(doc))
	}
	want, err := (&snapshot.CIRun{Repository: "beamfall/corvint", RunID: "7", HeadSha: merge, Conclusion: "success", PassedChecks: []string{"CI"}, CompletedAt: "2026-10-28T05:10:07Z"}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(stateDir, "evidence", string(wire.Sum(want)))); err != nil || string(got) != string(want) {
		t.Fatalf("retained CI run record: %q %v", got, err)
	}
	if rec := field(bcShow(t, root, ticketID), "record"); field(field(rec, "completion"), "kind").Str != "VERIFIED" {
		t.Fatalf("not VERIFIED: %s", wire.Encode(rec))
	}
	if after, _ := filepath.Glob(filepath.Join(os.TempDir(), "corvint-batch-close-ci-*")); !slices.Equal(tmpBefore, after) {
		t.Fatalf("derived CI run records left behind: %v", after)
	}
	if audit := atm(t, root, nil, "receipt", "audit"); audit.code != 0 {
		t.Fatalf("receipt audit: %s", audit.stdout)
	}
}
