package cli_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// obligationSpec holds AC-1..AC-5 as whole tokens; AC-8 appears only as a
// prefix of a longer token, so it is not in the source.
const obligationSpec = "import { test } from '@playwright/test';\n" +
	"test('AC-1 login', async () => {});\n" +
	"test('steps', async () => { await test.step('AC-2 soft', async () => {}); await test.step('AC-3 sibling', async () => {}); });\n" +
	"test('AC-4 conflicting', async () => {});\n" +
	"test('AC-5 failing', async () => {});\n" +
	"// AC-80 is another ticket's\n"

const obligationSeed = `{"prefix":"AC","obligations":[` +
	`{"id":"AC-1","title":"login works","core":true},` +
	`{"id":"AC-2","title":"soft step credits on its own","core":true},` +
	`{"id":"AC-3","title":"sibling step","core":false},` +
	`{"id":"AC-4","title":"conflicting projects","core":false},` +
	`{"id":"AC-5","title":"failing test","core":false},` +
	`{"id":"AC-6","title":"outside the repository","core":false},` +
	`{"id":"AC-7","title":"absent at the commit","core":false},` +
	`{"id":"AC-8","title":"forged title","core":false},` +
	`{"id":"AC-9","title":"never matched","core":false}]}`

// obligationRepo is an initialized store with one committed spec file and one
// OPEN native ticket; it returns the repo, the ticket and HEAD.
func obligationRepo(t *testing.T) (*fixture.Repo, string, string) {
	t.Helper()
	return obligationRepoWith(t, nil)
}

// obligationRepoWith edits the policy before init when edit is not nil.
func obligationRepoWith(t *testing.T, edit func(policy wire.Value)) (*fixture.Repo, string, string) {
	t.Helper()
	r := exclusionCLIRepo(t)
	if edit != nil {
		path := filepath.Join(r.IntentDir, "policy.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		pv, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		edit(pv)
		fixture.Write(t, path, wire.EncodeFile(pv))
	}
	fixture.Write(t, filepath.Join(r.Root, "e2e", "login.spec.ts"), []byte(obligationSpec))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "spec")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	t.Setenv("CORVINT_TASKS_ACTOR", "owner")
	id := planTicket(t, r.Root, "ledger", "P2", `["e2e/"]`)
	return r, id, headOID(t, r.Root)
}

func headOID(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// obligationRevision is the ticket's current record revision.
func obligationRevision(t *testing.T, root, id string) string {
	t.Helper()
	x := atm(t, root, nil, "ticket", "show", id)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket show: %s", x.stdout)
	}
	return field(x.res.Items[0], "revision").Str
}

func obligationsShow(t *testing.T, root, id string) wire.Value {
	t.Helper()
	x := atm(t, root, nil, "ticket", "obligations", "show", "--target", id)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("obligations show: %s", x.stdout)
	}
	return x.res.Items[0]
}

// entryStates maps each folded entry id to its state.
func entryStates(v wire.Value) map[string]string {
	out := map[string]string{}
	for _, e := range field(v, "entries").Arr {
		out[field(e, "id").Str] = field(e, "state").Str
	}
	return out
}

func seedObligations(t *testing.T, root, id string) {
	t.Helper()
	x := atm(t, root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, root, id),
		"--request-id", "seed-1", "--issued-at", "2026-10-08T12:00:00Z", "--payload", obligationSeed)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("seed: %s", x.stdout)
	}
}

// TestTOLV0005_SeedPrefixAndDuplicates: the first seed declares the prefix
// and seeds OPEN entries; a later seed must repeat the prefix and may not
// repeat an id; another native ticket cannot declare a held prefix.
func TestTOLV0005_SeedPrefixAndDuplicates(t *testing.T) {
	r, id, _ := obligationRepo(t)
	seedObligations(t, r.Root, id)
	v := obligationsShow(t, r.Root, id)
	for eid, st := range entryStates(v) {
		if st != "OPEN" {
			t.Fatalf("%s seeded %s", eid, st)
		}
	}
	if len(entryStates(v)) != 9 || field(field(v, "reference"), "prefix").Str != "AC" {
		t.Fatalf("seeded ledger: %s", wire.Encode(v))
	}
	refusals := []struct{ name, payload, code string }{
		{"other prefix", `{"prefix":"BD","obligations":[{"id":"BD-1","title":"x","core":false}]}`, wire.CodeMalformed},
		{"duplicate id", `{"prefix":"AC","obligations":[{"id":"AC-1","title":"again","core":false}]}`, wire.CodeDuplicateID},
		{"leading zero", `{"prefix":"AC","obligations":[{"id":"AC-07","title":"x","core":false}]}`, wire.CodeMalformed},
	}
	for i, rc := range refusals {
		x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
			"--request-id", "seed-bad-"+string(rune('a'+i)), "--payload", rc.payload)
		if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), rc.code) {
			t.Fatalf("%s: %s", rc.name, x.stdout)
		}
	}
	other := planTicket(t, r.Root, "other", "P2", `["docs/"]`)
	x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", other, "--expected-revision", obligationRevision(t, r.Root, other),
		"--request-id", "seed-other", "--payload", `{"prefix":"AC","obligations":[{"id":"AC-9","title":"x","core":false}]}`)
	if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), wire.CodeDuplicateID) {
		t.Fatalf("held prefix: %s", x.stdout)
	}
	later := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "seed-2", "--payload", `{"prefix":"AC","obligations":[{"id":"AC-10","title":"added later","core":false}]}`)
	if later.res.Outcome != wire.OutcomeOK || entryStates(obligationsShow(t, r.Root, id))["AC-10"] != "OPEN" {
		t.Fatalf("later seed: %s", later.stdout)
	}
}

// TestTOLV0006_ShowIsReadOnly: show prints the folded ledger, the reference
// and the head's receipt sequence and writes nothing; a ticket without a
// ledger is an empty OK result; a missing chain event is UNKNOWN with a
// typed diagnostic.
func TestTOLV0006_ShowIsReadOnly(t *testing.T) {
	r, id, _ := obligationRepo(t)
	empty := atm(t, r.Root, nil, "ticket", "obligations", "show", "--target", id)
	if empty.res.Outcome != wire.OutcomeOK || len(empty.res.Items) != 0 {
		t.Fatalf("no ledger: %s", empty.stdout)
	}
	seedObligations(t, r.Root, id)
	before := fixture.TreeSnapshot(t, r.StateDir)
	v := obligationsShow(t, r.Root, id)
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("obligations show wrote state")
	}
	ref := field(v, "reference")
	if field(v, "ledger").Str != "KNOWN" || field(ref, "revision").Str != "1" || field(v, "headSeq").Str == "UNKNOWN" || field(v, "headSeq").Str == "" {
		t.Fatalf("show: %s", wire.Encode(v))
	}
	counts := field(ref, "counts")
	if field(counts, "total").Str != "9" || field(counts, "coreTotal").Str != "2" || field(counts, "witnessed").Str != "0" {
		t.Fatalf("counts: %s", wire.Encode(counts))
	}
	head := field(ref, "head").Str
	evidence := filepath.Join(r.StateDir, "evidence", head)
	if _, err := os.Stat(evidence); err != nil {
		t.Fatalf("head event is not in the evidence store: %v", err)
	}
	if err := os.Remove(evidence); err != nil {
		t.Fatal(err)
	}
	u := atm(t, r.Root, nil, "ticket", "obligations", "show", "--target", id)
	if u.res.Outcome != wire.OutcomeOK || len(u.res.Items) != 1 {
		t.Fatalf("show after loss: %s", u.stdout)
	}
	if got := u.res.Items[0]; field(got, "ledger").Str != "UNKNOWN" || field(got, "entries").Kind != wire.KindNull || field(field(got, "diagnostic"), "code").Str == "" {
		t.Fatalf("missing event is not UNKNOWN: %s", u.stdout)
	}
}

// pwStep, pwResult, pwTest and pwSpec build a synthetic Playwright json
// reporter document with the observed closed top-level shape.
type pwStep struct {
	Title string   `json:"title"`
	Error any      `json:"error,omitempty"`
	Steps []pwStep `json:"steps"`
}

type pwResult struct {
	Status string   `json:"status"`
	Retry  int      `json:"retry"`
	Steps  []pwStep `json:"steps"`
	Stdout []string `json:"stdout"`
}

type pwTest struct {
	ExpectedStatus string     `json:"expectedStatus"`
	ProjectName    string     `json:"projectName"`
	Results        []pwResult `json:"results"`
}

type pwSpec struct {
	Title string   `json:"title"`
	ID    string   `json:"id"`
	File  string   `json:"file"`
	Tests []pwTest `json:"tests"`
}

const pwVersion = "1.63.0-test"

func passed(project string, steps ...pwStep) pwTest {
	return pwTest{ExpectedStatus: "passed", ProjectName: project, Results: []pwResult{{Status: "passed", Steps: steps, Stdout: []string{}}}}
}

func failedTest(project string, steps ...pwStep) pwTest {
	return pwTest{ExpectedStatus: "passed", ProjectName: project, Results: []pwResult{{Status: "failed", Steps: steps, Stdout: []string{}}}}
}

func step(title string, failed bool, nested ...pwStep) pwStep {
	s := pwStep{Title: title, Steps: nested}
	if nested == nil {
		s.Steps = []pwStep{}
	}
	if failed {
		s.Error = map[string]string{"message": "expect failed"}
	}
	return s
}

// writeReport writes the report under dir and returns its path.
func writeReport(t *testing.T, dir, rootDir, version string, specs ...pwSpec) string {
	t.Helper()
	doc := map[string]any{
		"config": map[string]any{"version": version, "rootDir": rootDir},
		"errors": []any{},
		"stats":  map[string]any{"expected": len(specs)},
		"suites": []any{map[string]any{"title": "login.spec.ts", "specs": specs, "suites": []any{}}},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "report.json")
	fixture.Write(t, path, raw)
	return path
}

// obligationCaseReport is the TOL-V0-010..012 case report: AC-1 passes at
// test level; AC-2's step soft-fails while its sibling AC-3 passes; AC-4
// passes in one project and fails in another; AC-5 only fails; AC-6 lies
// outside the repository, AC-7 in a file absent at the commit, AC-8 in a
// title its source does not hold; AC-99 is not in the ledger.
func obligationCaseReport(t *testing.T, root string) string {
	t.Helper()
	spec := "e2e/login.spec.ts"
	return writeReport(t, t.TempDir(), root, pwVersion,
		pwSpec{Title: "AC-1 login", ID: "s1", File: spec, Tests: []pwTest{passed("chromium")}},
		pwSpec{Title: "steps", ID: "s2", File: spec, Tests: []pwTest{failedTest("chromium", step("AC-2 soft", true), step("AC-3 sibling", false))}},
		pwSpec{Title: "AC-4 conflicting", ID: "s3", File: spec, Tests: []pwTest{passed("chromium"), failedTest("firefox")}},
		pwSpec{Title: "AC-5 failing", ID: "s4", File: spec, Tests: []pwTest{failedTest("chromium")}},
		pwSpec{Title: "AC-6 outside", ID: "s5", File: "../outside.spec.ts", Tests: []pwTest{passed("chromium")}},
		pwSpec{Title: "AC-7 absent", ID: "s6", File: "e2e/missing.spec.ts", Tests: []pwTest{passed("chromium")}},
		pwSpec{Title: "AC-8 forged", ID: "s7", File: spec, Tests: []pwTest{passed("chromium")}},
		pwSpec{Title: "AC-99 unknown", ID: "s8", File: spec, Tests: []pwTest{passed("chromium")}},
	)
}

func witnessArgs(id, req, rev, commit string, extra ...string) []string {
	return append([]string{"ticket", "obligations", "witness", "--target", id, "--request-id", req, "--expected-revision", rev,
		"--issued-at", "2026-10-08T13:00:00Z", "--commit", commit}, extra...)
}

func strList(v wire.Value, key string) string {
	var out []string
	for _, x := range field(v, key).Arr {
		if x.Kind == wire.KindObject {
			out = append(out, field(x, "id").Str+":"+field(x, "reason").Str)
			continue
		}
		out = append(out, x.Str)
	}
	return strings.Join(out, ",")
}

// TestTOLV0007_SetTransitions: set moves entries between OPEN, DEFECT,
// BLOCKED and DEFERRED with a reason, never to WITNESSED; demoting a
// WITNESSED entry clears its evidence; an unknown id refuses with
// OBLIGATION_UNKNOWN; a non-OWNER cannot change core.
func TestTOLV0007_SetTransitions(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	set := func(req, changes string, extra ...string) run {
		return atm(t, r.Root, nil, append([]string{"ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
			"--request-id", req, "--payload", `{"changes":` + changes + `}`}, extra...)...)
	}
	if x := set("set-1", `[{"id":"AC-3","state":"BLOCKED","core":null,"reason":"waits on fixture"},{"id":"AC-2","state":"DEFECT","core":true,"reason":"bug"},{"id":"AC-9","state":"DEFERRED","core":null,"reason":"later"}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("set: %s", x.stdout)
	}
	v := obligationsShow(t, r.Root, id)
	st := entryStates(v)
	if st["AC-2"] != "DEFECT" || st["AC-3"] != "BLOCKED" || st["AC-9"] != "DEFERRED" {
		t.Fatalf("states: %v", st)
	}
	if c := field(field(v, "reference"), "counts"); field(c, "total").Str != "8" || field(c, "deferred").Str != "1" {
		t.Fatalf("deferred is not excluded from total: %s", wire.Encode(c))
	}
	for name, changes := range map[string]string{
		"witnessed": `[{"id":"AC-1","state":"WITNESSED","core":null,"reason":"x"}]`,
		"no change": `[{"id":"AC-1","state":null,"core":null,"reason":"x"}]`,
		"no reason": `[{"id":"AC-1","state":"OPEN","core":null,"reason":null}]`,
	} {
		if x := set("set-bad-"+strings.ReplaceAll(name, " ", "-"), changes); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%s accepted: %s", name, x.stdout)
		}
	}
	if x := set("set-unknown", `[{"id":"AC-77","state":"OPEN","core":null,"reason":"x"}]`); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "OBLIGATION_UNKNOWN:") {
		t.Fatalf("unknown id: %s", x.stdout)
	}
	declare := atm(t, r.Root, nil, witnessArgs(id, "w-decl", obligationRevision(t, r.Root, id), head, "--declared", "AC-1",
		"--manifest-sha256", strings.Repeat("a", 64), "--test-id", "manual check", "--reason", "verified by hand")...)
	if declare.res.Outcome != wire.OutcomeOK {
		t.Fatalf("declared witness: %s", declare.stdout)
	}
	if x := set("set-demote", `[{"id":"AC-1","state":"OPEN","core":null,"reason":"regressed"}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("demote: %s", x.stdout)
	}
	for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
		if field(e, "id").Str == "AC-1" && (field(e, "state").Str != "OPEN" || field(e, "evidence").Kind != wire.KindNull || field(e, "reason").Str != "regressed") {
			t.Fatalf("demotion kept evidence: %s", wire.Encode(e))
		}
	}
	if x := set("set-core-op", `[{"id":"AC-4","state":null,"core":true,"reason":"core now"}]`, "--role", "OPERATOR"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("OPERATOR changed core: %s", x.stdout)
	}
}

// TestTOLV0008_DeclaredWitness: a DECLARED witness credits OPEN entries
// with the caller-vouched declaration and null report fields; an entry
// already WITNESSED keeps its evidence and is reported, not repeated; a
// witness that credits nothing writes no receipt and returns written:false;
// an unknown commit refuses.
func TestTOLV0008_DeclaredWitness(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	decl := func(req, ids string) run {
		return atm(t, r.Root, nil, witnessArgs(id, req, obligationRevision(t, r.Root, id), head, "--declared", ids,
			"--manifest-sha256", strings.Repeat("b", 64), "--test-id", "manual", "--reason", "checked")...)
	}
	x := decl("w-1", "AC-2,AC-1")
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool || strList(x.res.Items[0], "credited") != "AC-1,AC-2" {
		t.Fatalf("declared: %s", x.stdout)
	}
	v := obligationsShow(t, r.Root, id)
	var first wire.Value
	for _, e := range field(v, "entries").Arr {
		if field(e, "id").Str == "AC-1" {
			first = field(e, "evidence")
		}
	}
	if field(first, "source").Str != "DECLARED" || field(first, "commit").Str != head || field(first, "reportSha256").Kind != wire.KindNull ||
		field(first, "matches").Kind != wire.KindNull || field(field(first, "declaration"), "testId").Str != "manual" {
		t.Fatalf("declared evidence: %s", wire.Encode(first))
	}
	again := decl("w-2", "AC-1,AC-3")
	if again.res.Outcome != wire.OutcomeOK || strList(again.res.Items[0], "alreadyWitnessed") != "AC-1" || strList(again.res.Items[0], "credited") != "AC-3" {
		t.Fatalf("rewitness: %s", again.stdout)
	}
	for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
		if field(e, "id").Str == "AC-1" && !wire.Equal(field(e, "evidence"), first) {
			t.Fatalf("AC-1 evidence changed: %s", wire.Encode(e))
		}
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	none := decl("w-3", "AC-1")
	if none.res.Outcome != wire.OutcomeOK || field(none.res.Items[0], "written").Bool || strList(none.res.Items[0], "alreadyWitnessed") != "AC-1" {
		t.Fatalf("nothing to credit: %s", none.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a witness that credits nothing wrote state")
	}
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-4", obligationRevision(t, r.Root, id), strings.Repeat("c", 40), "--declared", "AC-4",
		"--manifest-sha256", strings.Repeat("b", 64), "--test-id", "manual", "--reason", "checked")...); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("unknown commit accepted: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-5", obligationRevision(t, r.Root, id), head, "--declared", "AC-4",
		"--manifest-sha256", strings.Repeat("b", 64), "--test-id", "manual", "--reason", "checked", "--role", "OPERATOR")...); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("OPERATOR declared: %s", x.stdout)
	}
	// A DEFERRED entry is not credited: it neither fails a batch that also
	// names a creditable entry nor writes on its own.
	if x := atm(t, r.Root, nil, "ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "set-defer", "--payload", `{"changes":[{"id":"AC-9","state":"DEFERRED","core":null,"reason":"later"}]}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("defer: %s", x.stdout)
	}
	if x := decl("w-6", "AC-4,AC-9"); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool || strList(x.res.Items[0], "credited") != "AC-4" {
		t.Fatalf("deferred in a batch: %s", x.stdout)
	}
	if st := entryStates(obligationsShow(t, r.Root, id)); st["AC-9"] != "DEFERRED" || st["AC-4"] != "WITNESSED" {
		t.Fatalf("states after deferred batch: %v", st)
	}
	before = fixture.TreeSnapshot(t, r.StateDir)
	if x := decl("w-7", "AC-9"); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "written").Bool || strList(x.res.Items[0], "credited") != "" {
		t.Fatalf("deferred only: %s", x.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a deferred-only witness wrote state")
	}
}

// TestTOLV0014_WitnessReplay: a witness retried with the same request id
// and issuedAt replays its receipt even though its credits are now
// WITNESSED (TM-V0-006), for both sources; the same id with other bytes is
// refused rather than reported as written:false.
func TestTOLV0014_WitnessReplay(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	rev := obligationRevision(t, r.Root, id)
	declared := declareArgs(id, "w-decl", rev, head, "AC-2")
	if x := atm(t, r.Root, nil, declared...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool {
		t.Fatalf("declared: %s", x.stdout)
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	if x := atm(t, r.Root, nil, declared...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
		t.Fatalf("declared retry did not replay: %s", x.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a replay wrote state")
	}
	if x := atm(t, r.Root, nil, declareArgs(id, "w-decl", rev, head, "AC-3")...); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("same request id with other bytes accepted: %s", x.stdout)
	}
	report := obligationCaseReport(t, r.Root)
	rev = obligationRevision(t, r.Root, id)
	reported := witnessArgs(id, "w-report", rev, head, "--from-playwright-report", report)
	if x := atm(t, r.Root, nil, reported...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool {
		t.Fatalf("report witness: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, reported...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
		t.Fatalf("report retry did not replay: %s", x.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("replayed witnesses do not audit")
	}
}

// TestTOLV0009_SubsetIgnoresExcludedMatchBound: an id outside --ids whose
// matches exceed the per-id bound does not refuse a witness of the subset.
func TestTOLV0009_SubsetIgnoresExcludedMatchBound(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	spec := "e2e/login.spec.ts"
	specs := []pwSpec{{Title: "AC-1 login", ID: "s1", File: spec, Tests: []pwTest{passed("chromium")}}}
	var many []pwTest
	for i := 0; i < 17; i++ {
		many = append(many, passed("project-"+strconv.Itoa(i)))
	}
	specs = append(specs, pwSpec{Title: "AC-2 many projects", ID: "s2", File: spec, Tests: many})
	report := writeReport(t, t.TempDir(), r.Root, pwVersion, specs...)
	rev := obligationRevision(t, r.Root, id)
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-all", rev, head, "--from-playwright-report", report)...); x.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(x.stdout), "OBLIGATION_EVENT_TOO_LARGE:") {
		t.Fatalf("over-bound id in scope: %s", x.stdout)
	}
	x := atm(t, r.Root, nil, witnessArgs(id, "w-sub", rev, head, "--from-playwright-report", report, "--ids", "AC-1")...)
	if x.res.Outcome != wire.OutcomeOK || strList(x.res.Items[0], "credited") != "AC-1" {
		t.Fatalf("subset witness: %s", x.stdout)
	}
}

// TestTOLV0009_ReportAdmissionAndRetention: an unqualified version refuses
// OBLIGATION_REPORT_VERSION_UNQUALIFIED (the shipped list is empty); a
// missing, non-JSON or wrong-shape report refuses OBLIGATION_REPORT: and
// writes nothing; an admitted report is never retained, only its digest; a
// secret in a credited title refuses SECRET_DETECTED.
func TestTOLV0009_ReportAdmissionAndRetention(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	report := obligationCaseReport(t, r.Root)
	rev := obligationRevision(t, r.Root, id)
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-v", rev, head, "--from-playwright-report", report)...); x.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(x.stdout), "OBLIGATION_REPORT_VERSION_UNQUALIFIED:") || !strings.Contains(string(x.stdout), wire.CodeUnsupportedVersion) {
		t.Fatalf("unqualified version: %s", x.stdout)
	}
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	dir := t.TempDir()
	bad := map[string]string{"non-json": "not json", "shape": `{"config":{"version":"` + pwVersion + `"},"suites":[]}`, "two values": `{} {}`}
	before := fixture.TreeSnapshot(t, r.StateDir)
	for name, body := range bad {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".json")
		fixture.Write(t, p, []byte(body))
		if x := atm(t, r.Root, nil, witnessArgs(id, "w-"+name, rev, head, "--from-playwright-report", p)...); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "OBLIGATION_REPORT:") {
			t.Fatalf("%s: %s", name, x.stdout)
		}
	}
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-missing", rev, head, "--from-playwright-report", filepath.Join(dir, "absent.json"))...); x.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(x.stdout), wire.CodeMissingEvidence) {
		t.Fatalf("missing report: %s", x.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a refused report wrote state")
	}
	secret := writeReport(t, t.TempDir(), r.Root, pwVersion,
		pwSpec{Title: "AC-1 token AKIAABCDEFGHIJKLMNOP", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-secret", rev, head, "--from-playwright-report", secret)...); x.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(x.stdout), wire.CodeSecretDetected) || strings.Contains(string(x.stdout), "AKIAABCDEFGHIJKLMNOP") {
		t.Fatalf("secret title: %s", x.stdout)
	}
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	x := atm(t, r.Root, nil, witnessArgs(id, "w-ok", rev, head, "--from-playwright-report", report)...)
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool {
		t.Fatalf("admitted report: %s", x.stdout)
	}
	sum := string(wire.Sum(raw))
	found := false
	err = filepath.Walk(r.StateDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if bytes.Equal(b, raw) {
			t.Errorf("the report is retained at %s", p)
		}
		found = found || bytes.Contains(b, []byte(sum))
		return nil
	})
	if err != nil || !found {
		t.Fatalf("report digest not retained (%v)", err)
	}
}

// TestTOLV0010_StepOwnErrorCredits, TestTOLV0011_ConflictingMatches and
// TestTOLV0012_SourcePresence share one witness of the case report.
func caseWitness(t *testing.T) (*fixture.Repo, string, wire.Value) {
	t.Helper()
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	x := atm(t, r.Root, nil, witnessArgs(id, "w-case", obligationRevision(t, r.Root, id), head, "--from-playwright-report", obligationCaseReport(t, r.Root))...)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("case witness: %s", x.stdout)
	}
	return r, id, x.res.Items[0]
}

// TestTOLV0010_StepOwnErrorCredits: a step credits its own ids when it has
// no error, whatever its sibling or the test did; a soft-failed step and a
// nested step's parent do not credit; a test-level id credits only on a
// passed result.
func TestTOLV0010_StepOwnErrorCredits(t *testing.T) {
	r, id, item := caseWitness(t)
	if got := strList(item, "credited"); got != "AC-1,AC-3" {
		t.Fatalf("credited %q: %s", got, wire.Encode(item))
	}
	if got := strList(item, "failed"); got != "AC-2,AC-5" {
		t.Fatalf("failed %q", got)
	}
	for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
		if field(e, "id").Str != "AC-3" {
			continue
		}
		ms := field(field(e, "evidence"), "matches").Arr
		if len(ms) != 1 || field(ms[0], "stepTitle").Str != "AC-3 sibling" || field(ms[0], "testId").Str != "s2@chromium" || field(ms[0], "path").Str != "e2e/login.spec.ts" {
			t.Fatalf("AC-3 match: %s", wire.Encode(e))
		}
	}
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	nested := writeReport(t, t.TempDir(), r.Root, pwVersion,
		pwSpec{Title: "nested", ID: "n1", File: "e2e/login.spec.ts", Tests: []pwTest{failedTest("chromium", step("AC-4 conflicting parent", true, step("inner", true)))}},
		pwSpec{Title: "timed", ID: "n2", File: "e2e/login.spec.ts", Tests: []pwTest{{ExpectedStatus: "passed", ProjectName: "chromium",
			Results: []pwResult{{Status: "timedOut", Steps: []pwStep{step("AC-5 failing", false)}, Stdout: []string{}}}}}})
	x := atm(t, r.Root, nil, witnessArgs(id, "w-nested", obligationRevision(t, r.Root, id), headOID(t, r.Root), "--from-playwright-report", nested)...)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "written").Bool || strList(x.res.Items[0], "failed") != "AC-4,AC-5" {
		t.Fatalf("nested parent and timedOut: %s", x.stdout)
	}
}

// TestTOLV0010_Retry0Only: only retry-0 results count; a retry-1 pass after
// a retry-0 failure credits nothing.
func TestTOLV0010_Retry0Only(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	flaky := pwTest{ExpectedStatus: "passed", ProjectName: "chromium", Results: []pwResult{
		{Status: "failed", Retry: 0, Steps: []pwStep{}, Stdout: []string{}}, {Status: "passed", Retry: 1, Steps: []pwStep{}, Stdout: []string{}}}}
	report := writeReport(t, t.TempDir(), r.Root, pwVersion, pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{flaky}})
	x := atm(t, r.Root, nil, witnessArgs(id, "w-retry", obligationRevision(t, r.Root, id), head, "--from-playwright-report", report)...)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "written").Bool || strList(x.res.Items[0], "failed") != "AC-1" {
		t.Fatalf("retry-1 pass credited: %s", x.stdout)
	}
}

// TestTOLV0011_ConflictingMatches: an id with a passing and a failing match
// is conflicting and not credited; an id outside the ledger is unknown; a
// ledger id with no match is unmatched; nothing becomes DEFECT.
func TestTOLV0011_ConflictingMatches(t *testing.T) {
	r, id, item := caseWitness(t)
	if strList(item, "conflicting") != "AC-4" || strList(item, "unknown") != "AC-99" || strList(item, "unmatched") != "AC-9" {
		t.Fatalf("lists: %s", wire.Encode(item))
	}
	for eid, st := range entryStates(obligationsShow(t, r.Root, id)) {
		if st == "DEFECT" || (eid == "AC-4" && st != "OPEN") {
			t.Fatalf("%s is %s", eid, st)
		}
	}
}

// TestTOLV0012_SourcePresence: a match outside the repository, in a file
// absent at the commit or in a source that lacks the id is unbound with its
// reason and not credited; the same report at a commit that lacks the spec
// file binds nothing.
func TestTOLV0012_SourcePresence(t *testing.T) {
	r, id, item := caseWitness(t)
	if got := strList(item, "unbound"); got != "AC-6:OUTSIDE_REPOSITORY,AC-7:ABSENT_AT_COMMIT,AC-8:ID_NOT_IN_SOURCE" {
		t.Fatalf("unbound %q", got)
	}
	st := entryStates(obligationsShow(t, r.Root, id))
	for _, eid := range []string{"AC-6", "AC-7", "AC-8"} {
		if st[eid] != "OPEN" {
			t.Fatalf("%s credited", eid)
		}
	}
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	base := strings.TrimSpace(gitOut(t, r.Root, "rev-list", "--max-parents=0", "HEAD"))
	report := writeReport(t, t.TempDir(), r.Root, pwVersion, pwSpec{Title: "AC-5 failing", ID: "s4", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	x := atm(t, r.Root, nil, witnessArgs(id, "w-old", obligationRevision(t, r.Root, id), base, "--from-playwright-report", report)...)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "written").Bool || strList(x.res.Items[0], "unbound") != "AC-5:ABSENT_AT_COMMIT" {
		t.Fatalf("wrong commit: %s", x.stdout)
	}
}

func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func declareArgs(id, req, rev, commit, ids string, extra ...string) []string {
	return witnessArgs(id, req, rev, commit, append([]string{"--declared", ids, "--manifest-sha256", strings.Repeat("d", 64),
		"--test-id", "manual", "--reason", "checked"}, extra...)...)
}

func auditConsistent(t *testing.T, root string) bool {
	t.Helper()
	a := atm(t, root, nil, "receipt", "audit")
	return a.res.Outcome == wire.OutcomeOK && field(a.res.Items[0], "structuralConsistency").Str == "CONSISTENT" &&
		field(a.res.Items[0], "projectionAgreement").Str == "AGREES"
}

// TestTOLV0013_CreditMismatchAndAudit: a report-witness payload the writer
// cannot recompute (no report, or credits that differ) refuses
// VALIDATION_FAILED MALFORMED OBLIGATION_CREDIT_MISMATCH: and writes
// nothing; the admitted witness passes receipt audit, which re-checks the
// retained matches against the commit.
func TestTOLV0013_CreditMismatchAndAudit(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	match := ticket.ObligationMatch{Path: "e2e/login.spec.ts", TestID: "s4@chromium", TitlePath: []string{"AC-5 failing"}, StepPath: []string{}}
	sum, version := wire.Sum([]byte("report")), pwVersion
	forged := ticket.ObligationWitnessPayload{Source: ticket.ObligationSourceReport, Commit: head, ReportSha256: &sum, PlaywrightVersion: &version,
		Credits: []ticket.ObligationCredit{{ID: "AC-5", Matches: []ticket.ObligationMatch{match}}}}
	env := cli.Env{Cwd: r.Root, Stdout: io.Discard, Stderr: io.Discard}
	honest := &mutation.ObligationReportCheck{ReportSha256: sum, PlaywrightVersion: version, Commit: head,
		Eligible: map[string][]ticket.ObligationMatch{"AC-1": {{Path: "e2e/login.spec.ts", TestID: "s1@chromium", TitlePath: []string{"AC-1 login"}, StepPath: []string{}}}}}
	before := fixture.TreeSnapshot(t, r.StateDir)
	for name, check := range map[string]*mutation.ObligationReportCheck{"no report": nil, "differing credits": honest} {
		res := cli.SubmitObligationWitness(env, id, obligationRevision(t, r.Root, id), "w-forged-"+strings.ReplaceAll(name, " ", "-"), check, forged.Value())
		enc := string(wire.Encode(res.Value()))
		if res.Outcome == wire.OutcomeOK || !strings.Contains(enc, mutation.OutcomeValidationFailed) || !strings.Contains(enc, wire.CodeMalformed) ||
			!strings.Contains(enc, ticket.ObligationCreditMismatchDetail) {
			t.Fatalf("%s: %s", name, enc)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a refused mismatch wrote state")
	}
	x := atm(t, r.Root, nil, witnessArgs(id, "w-ok", obligationRevision(t, r.Root, id), head, "--from-playwright-report", obligationCaseReport(t, r.Root))...)
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool {
		t.Fatalf("honest witness: %s", x.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("receipt audit refused the honest witness")
	}
}

// TestTOLV0013_AuditAfterReportDeleted: the report is never an input to
// the audit, so deleting it leaves receipt audit consistent; deleting the
// retained ledger event does not.
func TestTOLV0013_AuditAfterReportDeleted(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	report := obligationCaseReport(t, r.Root)
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-ok", obligationRevision(t, r.Root, id), head, "--from-playwright-report", report)...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	if err := os.Remove(report); err != nil {
		t.Fatal(err)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("receipt audit depends on the deleted report")
	}
	ev := field(obligationsShow(t, r.Root, id), "reference")
	if err := os.Remove(filepath.Join(r.StateDir, "evidence", field(ev, "head").Str)); err != nil {
		t.Fatal(err)
	}
	if auditConsistent(t, r.Root) {
		t.Fatal("receipt audit accepted a missing ledger event")
	}
}

// TestTOLV0013_DeclaredWitnessAudit: a DECLARED witness, a set and a later
// seed all pass receipt audit from their retained events alone.
func TestTOLV0013_DeclaredWitnessAudit(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := atm(t, r.Root, nil, declareArgs(id, "w-1", obligationRevision(t, r.Root, id), head, "AC-1,AC-2")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("declared: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "set-1", "--payload", `{"changes":[{"id":"AC-1","state":"DEFECT","core":null,"reason":"regressed"}]}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("set: %s", x.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("receipt audit refused declared ledger writes")
	}
}

// TestTOLV0014_RevisionOnlyWrite: each write moves the record revision and
// obligations.revision by one and leaves acceptanceRevision; the CAS is
// required; an identical retry replays; HELD admits writes and COMPLETED
// refuses TICKET_STATE.
func TestTOLV0014_RevisionOnlyWrite(t *testing.T) {
	r, id, head := obligationRepo(t)
	show := func() wire.Value { return atm(t, r.Root, nil, "ticket", "show", id).res.Items[0] }
	before := show()
	seedObligations(t, r.Root, id)
	after := show()
	if field(after, "revision").Str != "2" || field(after, "acceptanceRevision").Str != field(before, "acceptanceRevision").Str {
		t.Fatalf("seed moved %s -> %s", wire.Encode(before), wire.Encode(after))
	}
	args := []string{"ticket", "obligations", "seed", "--target", id, "--expected-revision", "1",
		"--request-id", "seed-1", "--issued-at", "2026-10-08T12:00:00Z", "--payload", obligationSeed}
	if x := atm(t, r.Root, nil, args...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
		t.Fatalf("identical retry: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, declareArgs(id, "w-stale", "1", head, "AC-1")...); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("stale expected revision: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "obligations", "show", "--target", id); field(field(x.res.Items[0], "reference"), "revision").Str != "1" {
		t.Fatalf("ledger revision: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "hold", "--target", id, "--expected-revision", "2", "--request-id", "hold-1",
		"--payload", `{"holdId":"h1","reason":"wait"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("hold: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, declareArgs(id, "w-held", "3", head, "AC-1")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("HELD refused a write: %s", x.stdout)
	}
	if v := obligationsShow(t, r.Root, id); field(field(v, "reference"), "revision").Str != "2" {
		t.Fatalf("ledger revision after witness: %s", wire.Encode(v))
	}
	done := planTicket(t, r.Root, "done", "P2", `["docs/"]`)
	if x := atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", "complete-done", "--target", done, "--expected-revision", "1",
		"--payload", `{"evidence":[],"reason":"manual"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete: %s", x.stdout)
	}
	x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", done, "--expected-revision", "2", "--request-id", "seed-done",
		"--payload", `{"prefix":"DN","obligations":[{"id":"DN-1","title":"x","core":false}]}`)
	if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), wire.CodeTicketState) {
		t.Fatalf("COMPLETED admitted a write: %s", x.stdout)
	}
}

// TestTOLV0015_RoleMatrix: OWNER has all three verbs by default; OPERATOR
// only through an explicit row and never DECLARED or core; WORKER only a
// report witness under obligations.workerWitness; REVIEWER is refused and no
// other role's row may name the verbs.
func TestTOLV0015_RoleMatrix(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedArgs := func(req, role string) []string {
		return []string{"ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
			"--request-id", req, "--role", role, "--payload", obligationSeed}
	}
	for _, role := range []string{"OPERATOR", "WORKER", "REVIEWER"} {
		if x := atm(t, r.Root, nil, seedArgs("seed-"+role, role)...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%s seeded without a grant: %s", role, x.stdout)
		}
	}
	seedObligations(t, r.Root, id)
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	report := obligationCaseReport(t, r.Root)
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-worker", obligationRevision(t, r.Root, id), head, "--from-playwright-report", report, "--role", "WORKER")...); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("WORKER witnessed without the policy key: %s", x.stdout)
	}
	ops := []string{"ARCHIVE", "COMPLETE_MANUAL", "CREATE", "GRANT_APPROVAL", "HOLD", "OBLIGATIONS_SEED", "OBLIGATIONS_SET", "OBLIGATIONS_WITNESS",
		"PRIORITIZE", "REFINE", "RELEASE_CANDIDATE", "RELEASE_CREATE", "RELEASE_EXTERNAL_ATTEST", "RELEASE_HOLD", "RELEASE_UPDATE", "REOPEN",
		"RESTORE", "REVOKE_APPROVAL", "SET_DEPENDENCIES", "SET_EFFECTS", "SET_GATES"}
	r2, id2, head2 := obligationRepoWith(t, func(p wire.Value) {
		p.Obj.Set("roles", wire.ObjectValue(wire.NewObject().Set("OPERATOR", wire.Strings(ops))))
	})
	op := func(args ...string) run { return atm(t, r2.Root, nil, append(args, "--role", "OPERATOR")...) }
	if x := op("ticket", "obligations", "seed", "--target", id2, "--expected-revision", "1", "--request-id", "seed-op", "--payload", obligationSeed); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("OPERATOR seed under an explicit row: %s", x.stdout)
	}
	if x := op("ticket", "obligations", "set", "--target", id2, "--expected-revision", "2", "--request-id", "set-op",
		"--payload", `{"changes":[{"id":"AC-2","state":"BLOCKED","core":null,"reason":"waits"}]}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("OPERATOR set: %s", x.stdout)
	}
	if x := op(declareArgs(id2, "w-op", "3", head2, "AC-1")...); x.res.Outcome == wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeUnauthorized {
		t.Fatalf("OPERATOR declared: %s", x.stdout)
	}
	if x := op(witnessArgs(id2, "w-op-report", "3", head2, "--from-playwright-report", obligationCaseReport(t, r2.Root))...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("OPERATOR report witness: %s", x.stdout)
	}
	for _, role := range []string{"WORKER", "REVIEWER", "IMPORTER", "SYSTEM"} {
		p := fixture.PolicyValue()
		p.Obj.Set("roles", wire.ObjectValue(wire.NewObject().Set(role, wire.Strings([]string{"OBLIGATIONS_WITNESS"}))))
		if _, err := intent.DecodePolicy(wire.EncodeFile(p)); err == nil {
			t.Fatalf("a %s row naming OBLIGATIONS_WITNESS decoded", role)
		}
	}
	p := fixture.PolicyValue()
	plain := wire.EncodeFile(p)
	if pol, err := intent.DecodePolicy(plain); err != nil || pol.WorkerObligationWitness() || bytes.Contains(plain, []byte(`"obligations"`)) {
		t.Fatalf("default policy: %v", err)
	}
	p.Obj.Set("obligations", wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(true)).Set("extra", wire.Bool(true))))
	if _, err := intent.DecodePolicy(wire.EncodeFile(p)); err == nil {
		t.Fatal("an open obligations policy key decoded")
	}
}

// workerObligationRepo is a seeded ledger under obligations.workerWitness
// whose ticket is claimed by agent; it returns the attempt and generation.
func workerObligationRepo(t *testing.T) (r *fixture.Repo, id, head, attempt, gen string) {
	t.Helper()
	r, id, head = obligationRepoWith(t, func(p wire.Value) {
		p.Obj.Set("obligations", wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(true))))
	})
	seedObligations(t, r.Root, id)
	t.Setenv("CORVINT_TASKS_ACTOR", "agent")
	c := atm(t, r.Root, nil, "claim", id, "--holder", "agent", "--request-id", "claim-1")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	return r, id, head, field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
}

// TestTOLV0015_WorkerStaleGenerationFenced: under the policy key a WORKER
// witnesses a report under its live attempt generation and that write sets
// lastRaise (TOL-V0-018); a WORKER cannot declare, seed or set; once the
// attempt is released and the same actor claims again, the old generation
// is FENCED; an unrecorded generation is PROVENANCE_UNVERIFIED.
func TestTOLV0015_WorkerStaleGenerationFenced(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head, attempt, gen := workerObligationRepo(t)
	report := obligationCaseReport(t, r.Root)
	worker := func(req string, extra ...string) run {
		return atm(t, r.Root, nil, witnessArgs(id, req, obligationRevision(t, r.Root, id), head, append([]string{"--role", "WORKER"}, extra...)...)...)
	}
	if x := worker("w-none", "--from-playwright-report", report); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("WORKER witness without an attempt: %s", x.stdout)
	}
	n, _ := strconv.Atoi(gen)
	if x := worker("w-unrecorded", "--from-playwright-report", report, "--attempt", attempt, "--generation", strconv.Itoa(n+1)); x.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(x.stdout), wire.CodeProvenanceUnverified) {
		t.Fatalf("unrecorded generation: %s", x.stdout)
	}
	if x := worker("w-decl", "--declared", "AC-1", "--manifest-sha256", strings.Repeat("d", 64), "--test-id", "m", "--reason", "r",
		"--attempt", attempt, "--generation", gen); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("WORKER declared: %s", x.stdout)
	}
	x := worker("w-ok", "--from-playwright-report", report, "--attempt", attempt, "--generation", gen)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("WORKER witness: %s", x.stdout)
	}
	raise := field(field(obligationsShow(t, r.Root, id), "reference"), "lastRaise")
	if field(raise, "attempt").Str != attempt || field(raise, "generation").Str != gen {
		t.Fatalf("lastRaise: %s", wire.Encode(raise))
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("receipt audit refused the WORKER witness")
	}
	if x := atm(t, r.Root, nil, "release", "--attempt", attempt, "--generation", gen, "--request-id", "release-1"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release: %s", x.stdout)
	}
	if c := atm(t, r.Root, nil, "claim", id, "--holder", "agent", "--request-id", "claim-2"); c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("reclaim: %s", c.stdout)
	}
	later := writeReport(t, t.TempDir(), r.Root, pwVersion, pwSpec{Title: "AC-5 failing", ID: "s4", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	if x := worker("w-stale", "--from-playwright-report", later, "--attempt", attempt, "--generation", gen); x.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(x.stdout), wire.CodeFenced) {
		t.Fatalf("stale generation after reclamation: %s", x.stdout)
	}
}

// TestTOLV0016_HighWaterMonotone: demotion lowers counts.witnessed but not
// highWater within one acceptance revision, so a rewitness is not a raise;
// an acceptance change resets the mark to the current count.
func TestTOLV0016_HighWaterMonotone(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	ref := func() wire.Value { return field(obligationsShow(t, r.Root, id), "reference") }
	hw := func() (string, string) {
		v := ref()
		return field(field(v, "counts"), "witnessed").Str, field(field(v, "highWater"), "witnessed").Str
	}
	if x := atm(t, r.Root, nil, declareArgs(id, "w-1", obligationRevision(t, r.Root, id), head, "AC-1,AC-2")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	set := func(req, state string) {
		if x := atm(t, r.Root, nil, "ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
			"--request-id", req, "--payload", `{"changes":[{"id":"AC-1","state":"`+state+`","core":null,"reason":"churn"}]}`); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("set: %s", x.stdout)
		}
	}
	set("set-1", "OPEN")
	if w, h := hw(); w != "1" || h != "2" {
		t.Fatalf("after demotion witnessed=%s highWater=%s", w, h)
	}
	if x := atm(t, r.Root, nil, declareArgs(id, "w-2", obligationRevision(t, r.Root, id), head, "AC-1")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("rewitness: %s", x.stdout)
	}
	if w, h := hw(); w != "2" || h != "2" {
		t.Fatalf("after rewitness witnessed=%s highWater=%s", w, h)
	}
	if x := atm(t, r.Root, nil, "ticket", "refine", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id), "--request-id", "refine-1",
		"--payload", `{"acceptanceCriteria":["it exists","it is witnessed"]}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("refine: %s", x.stdout)
	}
	set("set-2", "OPEN")
	acc := field(atm(t, r.Root, nil, "ticket", "show", id).res.Items[0], "acceptanceRevision").Str
	if w, h := hw(); w != "1" || h != "1" || field(field(ref(), "highWater"), "acceptanceRevision").Str != acc {
		t.Fatalf("after an acceptance change witnessed=%s highWater=%s: %s", w, h, wire.Encode(ref()))
	}
}

// TestTOLV0019_QueueStatusLegacyIdentity: without a ledger ticket queue
// status and its summary carry no obligations key.
func TestTOLV0019_QueueStatusLegacyIdentity(t *testing.T) {
	r, id, _ := obligationRepo(t)
	for _, args := range [][]string{{"queue", "status"}, {"queue", "status", "--summary"}} {
		if x := atm(t, r.Root, nil, args...); strings.Contains(string(x.stdout), `"obligations"`) {
			t.Fatalf("%v carries obligations: %s", args, x.stdout)
		}
	}
	for _, args := range [][]string{{"ticket", "show", id}, {"ticket", "list"}} {
		if x := atm(t, r.Root, nil, args...); strings.Contains(string(x.stdout), `"obligations"`) {
			t.Fatalf("%v carries obligations: %s", args, x.stdout)
		}
	}
}

// TestTOLV0019_ObligationSummary: queue status and its summary sum the OPEN
// and HELD ledger tickets; ticket show and list items carry the counts.
func TestTOLV0019_ObligationSummary(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := atm(t, r.Root, nil, declareArgs(id, "w-1", obligationRevision(t, r.Root, id), head, "AC-1,AC-3")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	want := `"obligations":{"core":{"total":"2","witnessed":"1"},"deferred":"0","tickets":"1","total":"9","witnessed":"2"}`
	for _, args := range [][]string{{"queue", "status"}, {"queue", "status", "--summary"}} {
		if x := atm(t, r.Root, nil, args...); !strings.Contains(string(x.stdout), want) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	for _, args := range [][]string{{"ticket", "show", id}, {"ticket", "list"}} {
		if x := atm(t, r.Root, nil, args...); !strings.Contains(string(x.stdout), `"obligations":{`) {
			t.Fatalf("%v lacks obligations: %s", args, x.stdout)
		}
	}
}

// TestTOLV0020_PlanCheck: the plan check reports UNASSIGNED, SPLIT,
// UNKNOWN_OBLIGATION and ALREADY_CLOSED, refuses until every open
// obligation has exactly one planned test, and writes nothing.
func TestTOLV0020_PlanCheck(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := atm(t, r.Root, nil, declareArgs(id, "w-1", obligationRevision(t, r.Root, id), head, "AC-9")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	dir := t.TempDir()
	plan := func(tests string) string {
		p := filepath.Join(dir, "plan.json")
		fixture.Write(t, p, []byte(`{"profile":"taskman-obligation-plan/0","ticketId":"`+id+`","tests":`+tests+`}`))
		return p
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	x := atm(t, r.Root, nil, "ticket", "obligations", "plan", "--target", id, "--plan",
		plan(`[{"test":"a","project":"chromium","obligations":["AC-1","AC-2","AC-3","AC-4","AC-5","AC-6","AC-7","AC-77"]},`+
			`{"test":"b","project":"chromium","obligations":["AC-1","AC-9"]}]`))
	if x.res.Outcome == wire.OutcomeOK || x.code == 0 {
		t.Fatalf("incomplete plan passed: %s", x.stdout)
	}
	var got []string
	for _, f := range field(x.res.Items[0], "findings").Arr {
		got = append(got, field(f, "kind").Str+":"+field(f, "id").Str+":"+field(f, "testCount").Str)
	}
	if strings.Join(got, ",") != "SPLIT:AC-1:2,UNKNOWN_OBLIGATION:AC-77:1,UNASSIGNED:AC-8:0,ALREADY_CLOSED:AC-9:1" {
		t.Fatalf("findings: %v", got)
	}
	ok := atm(t, r.Root, nil, "ticket", "obligations", "plan", "--target", id, "--plan",
		plan(`[{"test":"a","project":"chromium","obligations":["AC-1","AC-2","AC-3","AC-4"]},{"test":"b","project":"firefox","obligations":["AC-5","AC-6","AC-7","AC-8"]}]`))
	if ok.res.Outcome != wire.OutcomeOK || !field(ok.res.Items[0], "ok").Bool {
		t.Fatalf("complete plan: %s", ok.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("plan wrote state")
	}
}

// TestTOLV0002_OversizedWriteRefused: a write whose event would exceed the
// 65,536-byte derived-event slot refuses LIMIT_EXCEEDED
// OBLIGATION_EVENT_TOO_LARGE: and writes nothing; the same obligations split
// across two seeds are admitted.
func TestTOLV0002_OversizedWriteRefused(t *testing.T) {
	r, id, _ := obligationRepo(t)
	items := func(from, to int) string {
		var xs []string
		for i := from; i <= to; i++ {
			xs = append(xs, `{"core":false,"id":"AC-`+strconv.Itoa(i)+`","title":"`+strings.Repeat("x", 230)+`"}`)
		}
		sort.Strings(xs)
		return `{"obligations":[` + strings.Join(xs, ",") + `],"prefix":"AC"}`
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", "1", "--request-id", "seed-big", "--payload", items(1, 256))
	if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), wire.CodeLimitExceeded) || !strings.Contains(string(x.stdout), ticket.ObligationEventTooLargeDetail) {
		t.Fatalf("oversized seed: %s", x.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a refused oversized write wrote state")
	}
	for i, half := range []string{items(1, 128), items(129, 256)} {
		if x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", strconv.Itoa(i+1),
			"--request-id", "seed-half-"+strconv.Itoa(i), "--payload", half); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("split seed %d: %s", i, x.stdout)
		}
	}
	if c := field(field(obligationsShow(t, r.Root, id), "reference"), "counts"); field(c, "total").Str != "256" {
		t.Fatalf("counts: %s", wire.Encode(c))
	}
}

// TestTOLV0002_FoldFromRetainedRequestsAfterRestart: every write posts
// exactly one ledger event, and a fresh process folds the ledger from the
// retained events alone: the fold of the chain read from the evidence store
// equals what show reports, and a missing event makes show UNKNOWN.
func TestTOLV0002_FoldFromRetainedRequestsAfterRestart(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := atm(t, r.Root, nil, declareArgs(id, "w-1", obligationRevision(t, r.Root, id), head, "AC-1,AC-3")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "set-1", "--payload", `{"changes":[{"id":"AC-9","state":"DEFERRED","core":null,"reason":"later"}]}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("set: %s", x.stdout)
	}
	receipts, err := os.ReadDir(filepath.Join(r.StateDir, "receipts"))
	if err != nil {
		t.Fatal(err)
	}
	events := 0
	for _, e := range receipts {
		raw, err := os.ReadFile(filepath.Join(r.StateDir, "receipts", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		rv, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, p := range field(rv, "post").Arr {
			if strings.HasPrefix(field(p, "path").Str, "evidence/") {
				n++
			}
		}
		if n > 1 {
			t.Fatalf("receipt %s posts %d events", e.Name(), n)
		}
		events += n
	}
	shown := obligationsShow(t, r.Root, id)
	refV := field(shown, "reference")
	if events != 3 || field(refV, "revision").Str != "3" || field(shown, "ledger").Str != "KNOWN" {
		t.Fatalf("events=%d show: %s", events, wire.Encode(shown))
	}
	ref := ticket.ReadObligationsReference(wire.NewReader(refV, "/obligations"))
	tid, err := wire.ParseTicketID("/id", id)
	if err != nil || ref == nil {
		t.Fatalf("reference: %v", err)
	}
	read := func(d wire.Digest) ([]byte, error) {
		raw, err := os.ReadFile(filepath.Join(r.StateDir, "evidence", string(d)))
		if os.IsNotExist(err) {
			return nil, nil
		}
		return raw, err
	}
	l, err := ticket.FoldObligationChain(tid, *ref, read)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire.Encode(l.EntriesValue()), wire.Encode(field(shown, "entries"))) {
		t.Fatalf("fold differs from show:\n%s\n%s", wire.Encode(l.EntriesValue()), wire.Encode(field(shown, "entries")))
	}
	if err := os.Remove(filepath.Join(r.StateDir, "evidence", string(l.Events[0]))); err != nil {
		t.Fatal(err)
	}
	if v := obligationsShow(t, r.Root, id); field(v, "ledger").Str != "UNKNOWN" {
		t.Fatalf("show with a missing event: %s", wire.Encode(v))
	}
}

// TestTOLV0003_UpdatedAtIsIssuedAtNotRecordedAt: a folded entry's updatedAt
// is the retained request's issuedAt and updatedBy its actor, never the
// receipt's recordedAt (the wall clock of the write, not 2026-10-08T13:00Z).
func TestTOLV0003_UpdatedAtIsIssuedAtNotRecordedAt(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := atm(t, r.Root, nil, declareArgs(id, "w-1", obligationRevision(t, r.Root, id), head, "AC-1")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	got := map[string]string{}
	for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
		got[field(e, "id").Str] = field(e, "updatedAt").Str + "/" + field(field(e, "updatedBy"), "id").Str
	}
	if got["AC-1"] != "2026-10-08T13:00:00Z/owner" || got["AC-2"] != "2026-10-08T12:00:00Z/owner" {
		t.Fatalf("updatedAt: %v", got)
	}
}
