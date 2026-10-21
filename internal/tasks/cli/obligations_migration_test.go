package cli_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// migrationAlias is a 78-byte adopter id for row n, the length of the
// longest ids an adopter's 4,562-row ledger carried (GitHub #1071). Its
// dot-joined runs of 10 or more bytes are not JWT-shaped, so the shared
// secret screen admits them (LTA-V0-016, V1-1704).
func migrationAlias(n int) string {
	a := fmt.Sprintf("CHECKOUT-REGRESSION.PAYMENT-FLOW.SAVED-CARD-3DS-CHALLENGE-RETRY-PATH-V2.R%05d", n)
	if len(a) != 78 {
		panic(fmt.Sprintf("alias is %d bytes", len(a)))
	}
	return a
}

// migrationSeed is a 110-row seed with 78-byte aliases; rows 1..10 are core.
func migrationSeed() string {
	var xs []string
	for i := 1; i <= 110; i++ {
		xs = append(xs, fmt.Sprintf(`{"id":"AC-%d","alias":"%s","title":"row %d","core":%t}`, i, migrationAlias(i), i, i <= 10))
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	return `{"prefix":"AC","obligations":[` + strings.Join(xs, ",") + `]}`
}

func migrationRepo(t *testing.T) (*fixture.Repo, string, string) {
	t.Helper()
	r, id, head := obligationRepo(t)
	x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "seed-110", "--issued-at", "2026-10-18T00:00:00Z", "--payload", migrationSeed())
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("110-row seed: %s", x.stdout)
	}
	return r, id, head
}

func setObligations(t *testing.T, root, id, req, changes string) run {
	t.Helper()
	return atm(t, root, nil, "ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, root, id),
		"--request-id", req, "--payload", `{"changes":`+changes+`}`)
}

// TestTOLV0055_AliasLimit128: a 110-row seed with 78-byte aliases folds
// with every alias kept; a 128-byte alias seeds; a 129-byte alias refuses
// MALFORMED naming the 128-byte limit and writes nothing.
func TestTOLV0055_AliasLimit128(t *testing.T) {
	r, id, _ := migrationRepo(t)
	aliases := entryAliases(obligationsShow(t, r.Root, id))
	if len(aliases) != 110 || aliases["AC-110"] != migrationAlias(110) || aliases["AC-1"] != migrationAlias(1) {
		t.Fatalf("aliases: %d entries, AC-110=%q", len(aliases), aliases["AC-110"])
	}
	ok := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "seed-128", "--payload", `{"prefix":"AC","obligations":[{"id":"AC-111","alias":"`+strings.Repeat("M", 128)+`","title":"x","core":false}]}`)
	if ok.res.Outcome != wire.OutcomeOK {
		t.Fatalf("128-byte alias: %s", ok.stdout)
	}
	rev := obligationRevision(t, r.Root, id)
	bad := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", rev,
		"--request-id", "seed-129", "--payload", `{"prefix":"AC","obligations":[{"id":"AC-112","alias":"`+strings.Repeat("M", 129)+`","title":"x","core":false}]}`)
	if bad.res.Outcome == wire.OutcomeOK || !strings.Contains(string(bad.stdout), wire.CodeMalformed) ||
		!strings.Contains(string(bad.stdout), "the alias limit is 1..128 bytes") || obligationRevision(t, r.Root, id) != rev {
		t.Fatalf("129-byte alias: %s", bad.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("audit after the 110-row seed")
	}
}

// TestTOLV0056_0057_PlanRerunAndNonCore: on the 110-row ledger a plan that
// lists WITNESSED regression rows under rerun, omits every non-core row and
// omits BLOCKED and DEFECT rows passes and reports the reruns; a rerun of an
// unwitnessed id is RERUN_NOT_WITNESSED, a listed BLOCKED row in two tests
// is SPLIT, an omitted core OPEN row is UNASSIGNED, an id under both lists
// refuses MALFORMED, and a plan without rerun keeps its old output.
func TestTOLV0056_0057_PlanRerunAndNonCore(t *testing.T) {
	r, id, head := migrationRepo(t)
	if x := atm(t, r.Root, nil, declareArgs(id, "w-1", obligationRevision(t, r.Root, id), head, "AC-1,AC-2,AC-50")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	if x := setObligations(t, r.Root, id, "set-1", `[{"id":"AC-3","state":"BLOCKED","core":null,"reason":"waits on fixture"},{"id":"AC-4","state":"DEFECT","core":null,"reason":"bug"}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("set: %s", x.stdout)
	}
	dir := t.TempDir()
	n := 0
	plan := func(tests string) run {
		n++
		p := filepath.Join(dir, fmt.Sprintf("plan-%d.json", n))
		fixture.Write(t, p, []byte(`{"profile":"taskman-obligation-plan/0","ticketId":"`+id+`","tests":`+tests+`}`))
		return atm(t, r.Root, nil, "ticket", "obligations", "plan", "--target", id, "--plan", p)
	}
	kinds := func(x run) string {
		var got []string
		for _, f := range field(x.res.Items[0], "findings").Arr {
			got = append(got, field(f, "kind").Str+":"+field(f, "id").Str+":"+field(f, "testCount").Str)
		}
		return strings.Join(got, ",")
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	ok := plan(`[{"test":"a","project":"chromium","obligations":["AC-10","AC-5","AC-6"],"rerun":["AC-1","AC-50"]},` +
		`{"test":"b","project":"chromium","obligations":["AC-7","AC-8","AC-9"]},{"test":"c","project":"chromium","obligations":[],"rerun":["AC-2"]}]`)
	if ok.res.Outcome != wire.OutcomeOK || !field(ok.res.Items[0], "ok").Bool || kinds(ok) != "" {
		t.Fatalf("migration plan: %s", ok.stdout)
	}
	var reruns []string
	for _, f := range field(ok.res.Items[0], "reruns").Arr {
		reruns = append(reruns, field(f, "id").Str+"@"+strings.Join(strList2(field(f, "tests")), "+"))
	}
	if strings.Join(reruns, ",") != "AC-1@chromium/a,AC-2@chromium/c,AC-50@chromium/a" {
		t.Fatalf("reruns: %v", reruns)
	}
	bad := plan(`[{"test":"a","project":"chromium","obligations":["AC-10","AC-3","AC-5"],"rerun":["AC-1","AC-6"]},` +
		`{"test":"b","project":"chromium","obligations":["AC-3","AC-7","AC-8","AC-9"]}]`)
	if bad.res.Outcome == wire.OutcomeOK || bad.code == 0 ||
		kinds(bad) != "SPLIT:AC-3:2,RERUN_NOT_WITNESSED:AC-6:1,UNASSIGNED:AC-6:0" {
		t.Fatalf("bad plan: %s", kinds(bad))
	}
	if x := plan(`[{"test":"a","project":"chromium","obligations":["AC-10","AC-5","AC-6","AC-7","AC-8","AC-9"],"rerun":["AC-9"]}]`); x.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(x.stdout), wire.CodeMalformed) || !strings.Contains(string(x.stdout), "both obligations and rerun") {
		t.Fatalf("id in both lists: %s", x.stdout)
	}
	old := plan(`[{"test":"a","project":"chromium","obligations":["AC-10","AC-5","AC-6","AC-7","AC-8"]}]`)
	if _, has := old.res.Items[0].Obj.Get("reruns"); has || kinds(old) != "UNASSIGNED:AC-9:0" {
		t.Fatalf("plan without rerun: %s", old.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("plan wrote state")
	}
}

func strList2(v wire.Value) []string {
	var out []string
	for _, s := range v.Arr {
		out = append(out, s.Str)
	}
	return out
}

// TestTOLV0058_PartialNeverWitnessed: set moves an obligation to PARTIAL
// with a reason; PARTIAL counts in the total but never as witnessed, stays
// pending for the plan check, and a witness credits it to WITNESSED.
func TestTOLV0058_PartialNeverWitnessed(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := setObligations(t, r.Root, id, "set-p", `[{"id":"AC-1","state":"PARTIAL","core":null,"reason":"two of three steps witnessed"}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("set PARTIAL: %s", x.stdout)
	}
	v := obligationsShow(t, r.Root, id)
	if entryStates(v)["AC-1"] != "PARTIAL" {
		t.Fatalf("states: %v", entryStates(v))
	}
	if c := field(field(v, "reference"), "counts"); field(c, "total").Str != "9" || field(c, "witnessed").Str != "0" || field(c, "coreWitnessed").Str != "0" {
		t.Fatalf("PARTIAL counted as witnessed: %s", wire.Encode(c))
	}
	if x := setObligations(t, r.Root, id, "set-p-noreason", `[{"id":"AC-2","state":"PARTIAL","core":null,"reason":null}]`); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("PARTIAL without a reason: %s", x.stdout)
	}
	p := filepath.Join(t.TempDir(), "plan.json")
	fixture.Write(t, p, []byte(`{"profile":"taskman-obligation-plan/0","ticketId":"`+id+`","tests":[{"test":"a","project":"chromium","obligations":["AC-2"]}]}`))
	if x := atm(t, r.Root, nil, "ticket", "obligations", "plan", "--target", id, "--plan", p); x.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(x.stdout), `"kind":"UNASSIGNED"`) || !strings.Contains(string(x.stdout), `"id":"AC-1"`) {
		t.Fatalf("unplanned core PARTIAL: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, declareArgs(id, "w-p", obligationRevision(t, r.Root, id), head, "AC-1")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness PARTIAL: %s", x.stdout)
	}
	if st := entryStates(obligationsShow(t, r.Root, id)); st["AC-1"] != "WITNESSED" {
		t.Fatalf("states after witness: %v", st)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("audit after PARTIAL")
	}
}

// TestTOLV0059_WitnessProvenance: a witness may name testCommit,
// environmentTree, evidenceRef and the test title, and show reports them on
// each credited entry's evidence; a witness without them keeps evidence
// without a provenance member; a malformed OID or an explicitly empty value of
// any provenance flag refuses MALFORMED and a secret in the provenance refuses
// SECRET_DETECTED, none writing.
func TestTOLV0059_WitnessProvenance(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	tree := strings.TrimSpace(gitOut(t, r.Root, "rev-parse", "HEAD^{tree}"))
	x := atm(t, r.Root, nil, declareArgs(id, "w-prov", obligationRevision(t, r.Root, id), head, "AC-1,AC-2",
		"--test-commit", head, "--environment-tree", tree, "--evidence-ref", "ci/run/4211#shard-3", "--test-title", "checkout: AC-1 AC-2 saved card")...)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness with provenance: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, declareArgs(id, "w-plain", obligationRevision(t, r.Root, id), head, "AC-3")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("plain witness: %s", x.stdout)
	}
	want := `{"environmentTree":"` + tree + `","evidenceRef":"ci/run/4211#shard-3","testCommit":"` + head + `","testTitle":"checkout: AC-1 AC-2 saved card"}`
	for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
		if field(e, "state").Str != "WITNESSED" {
			continue
		}
		prov, has := field(e, "evidence").Obj.Get("provenance")
		switch field(e, "id").Str {
		case "AC-1", "AC-2":
			if !has || string(wire.Encode(prov)) != want {
				t.Fatalf("%s provenance: %v", field(e, "id").Str, has)
			}
		case "AC-3":
			if has {
				t.Fatal("a witness without provenance gained a provenance member")
			}
		}
	}
	for name, extra := range map[string][]string{
		"bad oid":                {"--test-commit", "not-an-oid"},
		"secret":                 {"--evidence-ref", "token AKIAABCDEFGHIJKLMNOP"},
		"empty test-commit":      {"--test-commit", ""},
		"empty environment-tree": {"--environment-tree", ""},
		"empty evidence-ref":     {"--evidence-ref", ""},
		"empty test-title":       {"--test-title", ""},
	} {
		rev := obligationRevision(t, r.Root, id)
		x := atm(t, r.Root, nil, declareArgs(id, "w-"+strings.ReplaceAll(name, " ", "-"), rev, head, "AC-4", extra...)...)
		want := wire.CodeMalformed
		if name == "secret" {
			want = wire.CodeSecretDetected
		}
		if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), want) || obligationRevision(t, r.Root, id) != rev {
			t.Fatalf("%s: %s", name, x.stdout)
		}
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("audit after provenance witness")
	}
}
