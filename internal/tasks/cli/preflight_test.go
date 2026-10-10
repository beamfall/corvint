package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// preflightSpec is a committed spec with every TOL-V0-025..027 case:
//   - AC-1 in a test title and AC-2 in a test.step;
//   - a test.fail test (line 8) that names the expected-fail AC-3 and the
//     ordinary AC-4, so AC-4 is mixed;
//   - AC-5 named only through a contract annotation `checkout:AC-5`;
//   - AC-6 only in a comment, so it is unnamed;
//   - AC-7 and AC-8 split one per test in one describe block, while AC-9
//     and AC-10 are split but one of them is annotated isolated.
const preflightSpec = `import { test, expect } from '@playwright/test';

test.describe('checkout', () => {
  test('AC-1 cart totals', async ({ page }) => {
    await test.step('AC-2 discount applies', async () => {});
  });

  test('AC-3 receipt renders', async () => {
    test.fail(true, 'BUG-17 receipt is blank');
    await test.step('AC-3 expected-fail: receipt renders', async () => {});
    await test.step("AC-4 email sent", async () => {});
  });

  test('contract witnesses', async () => {
    await expectContract('checkout:AC-5');
  });
});

// AC-6 is not written yet.
test.describe('profile', () => {
  test('AC-7 avatar upload', async () => {});
  test('AC-8 bio edit', async () => {});
});

test.describe('admin', () => {
  test('AC-9 reset', { annotation: { type: 'isolated', description: 'needs a fresh store' } }, async () => {});
  test('AC-10 audit view', async () => {});
});
`

// cleanSpec names every obligation of the clean ledger once, with no mixed
// test.fail test and no split.
const cleanSpec = `import { test } from '@playwright/test';

test.describe('checkout', () => {
  test('AC-1 cart totals', async () => {
    await test.step('AC-2 discount applies', async () => {});
  });

  test.fail('AC-3 receipt renders (BUG-17)', async () => {
    await test.step('AC-3 expected-fail', async () => {});
  });

  test('contract witnesses', async () => {
    // a string that holds a test( call is not a test: "test('AC-6 fake', () => {})"
    await expectContract(` + "`checkout:AC-5`" + `);
  });
});
`

const preflightSeed = `{"prefix":"AC","obligations":[` +
	`{"id":"AC-1","title":"cart","core":true},{"id":"AC-10","title":"audit","core":false},` +
	`{"id":"AC-2","title":"discount","core":false},{"id":"AC-3","title":"receipt","core":false},` +
	`{"id":"AC-4","title":"email","core":false},{"id":"AC-5","title":"contract","core":false},` +
	`{"id":"AC-6","title":"unwritten","core":false},{"id":"AC-7","title":"avatar","core":false},` +
	`{"id":"AC-8","title":"bio","core":false},{"id":"AC-9","title":"reset","core":false}]}`

const cleanSeed = `{"prefix":"AC","obligations":[` +
	`{"id":"AC-1","title":"cart","core":true},{"id":"AC-2","title":"discount","core":false},` +
	`{"id":"AC-3","title":"receipt","core":false},{"id":"AC-5","title":"contract","core":false}]}`

// preflightRepo is an initialized store whose only spec file is body at
// e2e/checkout.spec.ts, with one native ticket seeded with seed.
func preflightRepo(t *testing.T, body, seed string, gates ...string) (*fixture.Repo, string, string) {
	t.Helper()
	r := exclusionCLIRepo(t)
	if len(gates) > 0 {
		path := filepath.Join(r.IntentDir, "policy.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		pv, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		list := field(pv, "gates").Arr
		for _, g := range gates {
			gv, err := wire.Parse([]byte(g + "\n"))
			if err != nil {
				t.Fatal(err)
			}
			list = append(list, gv)
		}
		pv.Obj.Set("gates", wire.Array(list...))
		fixture.Write(t, path, wire.EncodeFile(pv))
	}
	fixture.Write(t, filepath.Join(r.Root, "e2e", "checkout.spec.ts"), []byte(body))
	fixture.Write(t, filepath.Join(r.Root, "e2e", "helpers.ts"), []byte("// AC-6 helper, not a spec file\n"))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "spec")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	t.Setenv("CORVINT_TASKS_ACTOR", "owner")
	id := planTicket(t, r.Root, "preflight", "P2", `["e2e/"]`)
	x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "seed-1", "--issued-at", "2026-10-09T12:00:00Z", "--payload", seed)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("seed: %s", x.stdout)
	}
	return r, id, headOID(t, r.Root)
}

// findingList renders findings as kind:id:path:line.
func findingList(t *testing.T, v wire.Value) string {
	t.Helper()
	if v.Kind != wire.KindObject {
		t.Fatalf("no preflight item")
	}
	var out []string
	for _, f := range field(v, "findings").Arr {
		if field(f, "remedy").Str == "" {
			t.Fatalf("finding without a remedy: %s", wire.Encode(f))
		}
		out = append(out, field(f, "kind").Str+":"+field(f, "id").Str+":"+field(f, "path").Str+":"+field(f, "line").Str)
	}
	return strings.Join(out, ",")
}

// TestTOLV0025_PreflightRefusesUnnamedMixedAndSplit: preflight reads the
// spec files at the commit and refuses PREFLIGHT_FAILED with an unnamed
// obligation, a test.fail test that names an ordinary obligation (file:line)
// and tests split one per obligation in one describe block; a
// contract-annotated obligation is named, an isolated split is accepted,
// and the read writes no state.
func TestTOLV0025_PreflightRefusesUnnamedMixedAndSplit(t *testing.T) {
	r, id, head := preflightRepo(t, preflightSpec, preflightSeed)
	// A later uncommitted edit is not read: preflight reads the commit.
	fixture.Write(t, filepath.Join(r.Root, "e2e", "checkout.spec.ts"), []byte("// emptied\n"))
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "preflight", id, "--commit", head)
	if x.res.Outcome == wire.OutcomeOK || x.code == 0 || len(x.res.Items) != 1 {
		t.Fatalf("preflight: %s", x.stdout)
	}
	item := x.res.Items[0]
	if field(item, "status").Str != "PREFLIGHT_FAILED" || field(item, "commit").Str != head || field(item, "ok").Bool {
		t.Fatalf("status: %s", x.stdout)
	}
	want := "MIXED_EXPECTED_FAIL:AC-4:e2e/checkout.spec.ts:8," +
		"SPLIT_TESTS:AC-7:e2e/checkout.spec.ts:21,SPLIT_TESTS:AC-8:e2e/checkout.spec.ts:22," +
		"UNNAMED:AC-6::"
	if got := findingList(t, item); got != want {
		t.Fatalf("findings\n got %s\nwant %s\n%s", got, want, x.stdout)
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("preflight wrote state")
	}
}

// TestTOLV0025_PreflightCleanTicketPasses: a ticket whose every open
// obligation is named, with a contract annotation and a whole-test test.fail
// that names only its expected-fail obligation, passes with no finding.
func TestTOLV0025_PreflightCleanTicketPasses(t *testing.T) {
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed)
	x := atm(t, r.Root, nil, "preflight", id)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || !field(x.res.Items[0], "ok").Bool || findingList(t, x.res.Items[0]) != "" {
		t.Fatalf("clean preflight: %s", x.stdout)
	}
}

// TestTOLV0026_PreflightPlanCheck: --plan reads the plan document at the
// commit and adds its TOL-V0-020 findings; a plan path absent at the commit
// is a PLAN_MISSING finding.
func TestTOLV0026_PreflightPlanCheck(t *testing.T) {
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed)
	fixture.Write(t, filepath.Join(r.Root, "e2e", "plan.json"), []byte(`{"profile":"taskman-obligation-plan/0","ticketId":"`+id+
		`","tests":[{"test":"cart","project":"chromium","obligations":["AC-1","AC-2"]},{"test":"receipt","project":"chromium","obligations":["AC-1","AC-3"]}]}`))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "plan")
	x := atm(t, r.Root, nil, "preflight", id, "--plan", "e2e/plan.json")
	if x.res.Outcome == wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("plan findings: %s", x.stdout)
	}
	if got := findingList(t, x.res.Items[0]); got != "SPLIT:AC-1:e2e/plan.json:,UNASSIGNED:AC-5:e2e/plan.json:" {
		t.Fatalf("plan findings %q", got)
	}
	y := atm(t, r.Root, nil, "preflight", id, "--plan", "e2e/absent.json")
	if y.res.Outcome == wire.OutcomeOK || len(y.res.Items) != 1 || findingList(t, y.res.Items[0]) != "PLAN_MISSING::e2e/absent.json:" {
		t.Fatalf("absent plan: %s", y.stdout)
	}
}

// TestTOLV0027_PreflightDeepRunsGatesInCleanWorktree: --deep runs each named
// policy COMMAND gate in a temporary clean worktree at the commit, reports a
// failing gate as DEEP_CHECK_FAILED quoting its first actionable line, and
// removes the worktree; the caller's dirty checkout is not what runs.
func TestTOLV0027_PreflightDeepRunsGatesInCleanWorktree(t *testing.T) {
	gate := func(id, script string) string {
		return `{"argv":["sh","-c",` + strings.TrimSpace(string(wire.EncodeFile(wire.String(script)))) + `],"cwd":"WORKTREE","env":[],"evidence":[],` +
			`"expected":{"exitCode":"0","reducer":null},"gateId":"` + id + `","inputs":[],"kind":"COMMAND","required":false,"reusable":true,"sharedResource":null,"timeoutSeconds":"60"}`
	}
	// lint (a committed script) fails on a TODO in the committed spec;
	// contract passes only in a clean checkout of the commit. Gates run with
	// no environment, so the scripts use shell builtins only.
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed,
		gate("contract", `test -f e2e/checkout.spec.ts && test ! -f dirty.txt`), gate("lint", `. ci/lint.sh`))
	fixture.Write(t, filepath.Join(r.Root, "ci", "lint.sh"), []byte(`echo "linting 1 file"
while IFS= read -r l; do case "$l" in *TODO*) echo "e2e/checkout.spec.ts: error: TODO left"; exit 1;; esac; done < e2e/checkout.spec.ts
`))
	git(t, r.Root, "add", "ci")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "lint")
	fixture.Write(t, filepath.Join(r.Root, "dirty.txt"), []byte("uncommitted\n"))
	ok := atm(t, r.Root, nil, "preflight", id, "--deep", "--gate", "contract", "--gate", "lint")
	if ok.res.Outcome != wire.OutcomeOK || len(ok.res.Items) != 1 || !field(ok.res.Items[0], "ok").Bool {
		t.Fatalf("passing deep gates: %s", ok.stdout)
	}
	fixture.Write(t, filepath.Join(r.Root, "e2e", "checkout.spec.ts"), []byte(cleanSpec+"// TODO tidy\n"))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "todo")
	x := atm(t, r.Root, nil, "preflight", id, "--deep", "--gate", "contract", "--gate", "lint")
	if x.res.Outcome == wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("failing deep gate: %s", x.stdout)
	}
	f := field(x.res.Items[0], "findings").Arr
	if len(f) != 1 || field(f[0], "kind").Str != "DEEP_CHECK_FAILED" || field(f[0], "id").Str != "lint" ||
		!strings.Contains(field(f[0], "detail").Str, "error: TODO left") {
		t.Fatalf("deep findings: %s", x.stdout)
	}
	if wt := gitOut(t, r.Root, "worktree", "list", "--porcelain"); strings.Count(wt, "worktree ") != 1 {
		t.Fatalf("temporary worktree left behind:\n%s", wt)
	}
	if y := atm(t, r.Root, nil, "preflight", id, "--deep", "--gate", "absent"); y.res.Outcome == wire.OutcomeOK || !hasCode(y.res, wire.CodeGateUnknown) {
		t.Fatalf("unknown gate: %s", y.stdout)
	}
	if y := atm(t, r.Root, nil, "preflight", id, "--gate", "lint"); y.res.Outcome == wire.OutcomeOK {
		t.Fatalf("--gate without --deep: %s", y.stdout)
	}
}
