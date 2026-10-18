package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// aliasSpec names AC-1 by its alias in a test title (`M001:`), AC-2 by its
// alias in a test.step title, AC-4 by a contract annotation of its alias,
// and AC-3's alias M002 only inside longer tokens, so AC-3 is unnamed.
const aliasSpec = `import { test } from '@playwright/test';

test.describe('rows', () => {
  test('M001: login works', async () => {
    await test.step('ABC-12.V4 export', async () => {});
    await test.step('login_v2 flow', async () => {});
  });

  test('M002-legacy, xM002, M002_x and M002.1', async () => {});

  test('contract witnesses', async () => {
    await expectContract('rows:FLOW-7-register-auth');
  });
});
`

const aliasSeed = `{"prefix":"AC","obligations":[` +
	`{"id":"AC-1","alias":"M001","title":"login","core":true},` +
	`{"id":"AC-2","alias":"ABC-12.V4","title":"export","core":false},` +
	`{"id":"AC-3","alias":"M002","title":"legacy","core":false},` +
	`{"id":"AC-4","alias":"FLOW-7-register-auth","title":"register","core":false},` +
	`{"id":"AC-5","title":"no alias","core":false},` +
	`{"id":"AC-8","alias":"login_v2","title":"underscore","core":false}]}`

// entryAliases maps each folded entry id to its alias, "-" when the entry
// has no alias member.
func entryAliases(v wire.Value) map[string]string {
	out := map[string]string{}
	for _, e := range field(v, "entries").Arr {
		a, ok := e.Obj.Get("alias")
		if !ok {
			out[field(e, "id").Str] = "-"
			continue
		}
		out[field(e, "id").Str] = a.Str
	}
	return out
}

// TestTOLV0034_SeedWithAliases: a seed item may carry an alias, kept on its
// folded entry; an entry seeded without one has no alias member; a later
// seed adds aliased entries; an alias outside the grammar refuses MALFORMED
// and writes nothing.
func TestTOLV0034_SeedWithAliases(t *testing.T) {
	r, id, _ := preflightRepo(t, aliasSpec, aliasSeed)
	got := entryAliases(obligationsShow(t, r.Root, id))
	want := map[string]string{"AC-1": "M001", "AC-2": "ABC-12.V4", "AC-3": "M002", "AC-4": "FLOW-7-register-auth", "AC-5": "-", "AC-8": "login_v2"}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("aliases %v, want %v", got, want)
		}
	}
	bad := []string{"M003.", "-M003", "M 003", "M003/x", strings.Repeat("M", 129)}
	for i, alias := range bad {
		rev := obligationRevision(t, r.Root, id)
		x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", rev,
			"--request-id", "seed-bad-"+string(rune('a'+i)), "--payload", `{"prefix":"AC","obligations":[{"id":"AC-6","alias":"`+alias+`","title":"x","core":false}]}`)
		if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), wire.CodeMalformed) || obligationRevision(t, r.Root, id) != rev {
			t.Fatalf("alias %q: %s", alias, x.stdout)
		}
	}
	later := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "seed-2", "--payload", `{"prefix":"AC","obligations":[{"id":"AC-6","alias":"V7","title":"added later","core":false}]}`)
	if later.res.Outcome != wire.OutcomeOK || entryAliases(obligationsShow(t, r.Root, id))["AC-6"] != "V7" {
		t.Fatalf("later aliased seed: %s", later.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("receipt audit after aliased seeds")
	}
}

// TestTOLV0035_AliasDuplicateAndCollision: an alias repeated in one seed or
// against an earlier seed refuses DUPLICATE_ID OBLIGATION_ALIAS_DUPLICATE:,
// and an alias that is or holds a native id of the ledger prefix refuses
// DUPLICATE_ID OBLIGATION_ALIAS_COLLISION:; none writes.
func TestTOLV0035_AliasDuplicateAndCollision(t *testing.T) {
	r, id, _ := preflightRepo(t, aliasSpec, aliasSeed)
	cases := []struct{ name, payload, detail string }{
		{"in one seed", `{"prefix":"AC","obligations":[{"id":"AC-6","alias":"R1","title":"x","core":false},{"id":"AC-7","alias":"R1","title":"y","core":false}]}`, "OBLIGATION_ALIAS_DUPLICATE:"},
		{"against the ledger", `{"prefix":"AC","obligations":[{"id":"AC-6","alias":"M001","title":"x","core":false}]}`, "OBLIGATION_ALIAS_DUPLICATE:"},
		{"a ledger native id", `{"prefix":"AC","obligations":[{"id":"AC-6","alias":"AC-2","title":"x","core":false}]}`, "OBLIGATION_ALIAS_COLLISION:"},
		{"its own native id", `{"prefix":"AC","obligations":[{"id":"AC-6","alias":"AC-6","title":"x","core":false}]}`, "OBLIGATION_ALIAS_COLLISION:"},
		{"a native id inside", `{"prefix":"AC","obligations":[{"id":"AC-6","alias":"row.AC-9","title":"x","core":false}]}`, "OBLIGATION_ALIAS_COLLISION:"},
	}
	for i, c := range cases {
		rev := obligationRevision(t, r.Root, id)
		x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", rev,
			"--request-id", "seed-dup-"+string(rune('a'+i)), "--payload", c.payload)
		out := string(x.stdout)
		if x.res.Outcome == wire.OutcomeOK || !strings.Contains(out, wire.CodeDuplicateID) || !strings.Contains(out, c.detail) || obligationRevision(t, r.Root, id) != rev {
			t.Fatalf("%s: %s", c.name, out)
		}
	}
	// A defect-shaped or other-prefix alias is not a collision.
	ok := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "seed-other-prefix", "--payload", `{"prefix":"AC","obligations":[{"id":"AC-6","alias":"BD-6","title":"x","core":false}]}`)
	if ok.res.Outcome != wire.OutcomeOK {
		t.Fatalf("other-prefix alias: %s", ok.stdout)
	}
}

// TestTOLV0036_PreflightMatchesAliases: preflight counts an alias as naming
// its obligation in a test title, a test.step title and a contract
// annotation, but never inside a longer token (M002-legacy, xM002, M002_x, M002.1).
func TestTOLV0036_PreflightMatchesAliases(t *testing.T) {
	r, id, head := preflightRepo(t, aliasSpec, aliasSeed)
	x := atm(t, r.Root, nil, "preflight", id, "--commit", head)
	if len(x.res.Items) != 1 {
		t.Fatalf("preflight: %s", x.stdout)
	}
	if got, want := findingList(t, x.res.Items[0]), "UNNAMED:AC-3::,UNNAMED:AC-5::"; got != want {
		t.Fatalf("findings %q, want %q\n%s", got, want, x.stdout)
	}
}

// TestTOLV0036_WitnessCreditsByAliasKeyedToNativeID: a report that names
// obligations only by alias credits them under their native ids, binds the
// source by the alias, passes receipt audit, ignores an alias inside a
// longer token, and an alias shaped like a defect id is not read as a
// defect inside an expected-failure test.
func TestTOLV0036_WitnessCreditsByAliasKeyedToNativeID(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := preflightRepo(t, aliasSpec, aliasSeed)
	spec := "e2e/checkout.spec.ts"
	xfail := writeReport(t, t.TempDir(), r.Root, pwVersion,
		pwSpec{Title: "export", ID: "f1", File: spec, Tests: []pwTest{expectedFail("chromium", "", "failed",
			stepErr("ABC-12.V4 expected-fail", "Timeout 5000ms exceeded"))}})
	y := atm(t, r.Root, nil, witnessArgs(id, "w-xfail", obligationRevision(t, r.Root, id), head, "--from-playwright-report", xfail)...)
	if y.res.Outcome != wire.OutcomeOK || defectList(y.res.Items[0]) != "AC-2::Timeout 5000ms exceeded" {
		t.Fatalf("expected-fail alias: %s", y.stdout)
	}
	report := writeReport(t, t.TempDir(), r.Root, pwVersion,
		pwSpec{Title: "M001: login works", ID: "s1", File: spec, Tests: []pwTest{passed("chromium", step("ABC-12.V4 export", false), step("login_v2 flow", false))}},
		pwSpec{Title: "M002-legacy, xM002, M002_x and M002.1", ID: "s2", File: spec, Tests: []pwTest{passed("chromium")}})
	x := atm(t, r.Root, nil, witnessArgs(id, "w-alias", obligationRevision(t, r.Root, id), head, "--from-playwright-report", report)...)
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool {
		t.Fatalf("witness: %s", x.stdout)
	}
	item := x.res.Items[0]
	if strList(item, "credited") != "AC-1,AC-2,AC-8" || strList(item, "unmatched") != "AC-3,AC-4,AC-5" || strList(item, "unknown") != "" || strList(item, "unbound") != "" {
		t.Fatalf("lists: %s", wire.Encode(item))
	}
	v := obligationsShow(t, r.Root, id)
	st := entryStates(v)
	if st["AC-1"] != "WITNESSED" || st["AC-2"] != "WITNESSED" || st["AC-8"] != "WITNESSED" || st["AC-3"] != "OPEN" {
		t.Fatalf("states %v", st)
	}
	for _, e := range field(v, "entries").Arr {
		if field(e, "id").Str != "AC-2" {
			continue
		}
		ms := field(field(e, "evidence"), "matches").Arr
		if len(ms) != 1 || field(ms[0], "stepTitle").Str != "ABC-12.V4 export" || field(ms[0], "path").Str != spec {
			t.Fatalf("AC-2 evidence: %s", wire.Encode(e))
		}
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("receipt audit does not bind the alias-named source")
	}
}
