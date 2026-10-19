package testsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

const teardownDeclaration = `{"schema": "corvint-side-effect-declaration/0", "version": 3,
 "revision": "0123456789abcdef0123456789abcdef01234567",
 "resources": [
  {"kind": "order", "create": ["createOrder", "create_order"], "delete": ["deleteOrder", "delete_order"]},
  {"kind": "confirmation", "create": ["createConfirmation"], "delete": ["deleteConfirmation", "delete_confirmation"]},
  {"kind": "audit_entry", "create": ["createAuditEntry"], "delete": ["deleteAuditEntry"]}],
 "actions": [
  {"call": "confirmOrder", "parent": "order", "dependent": "confirmation"},
  {"call": "confirm_order", "parent": "order", "dependent": "confirmation"},
  {"call": "archiveOrder", "parent": "order", "dependent": "audit_entry", "cascade": true},
  {"call": "archive_order", "parent": "order", "dependent": "audit_entry", "cascade": true},
  {"call": "renameOrder"},
  {"call": "rename_order"}]}
`

const teardownJS = `import { test, expect } from '@playwright/test';

test.describe('orders', () => {
  let order;
  test.beforeEach(async ({ request }) => {
    order = await createOrder(request);
  });
  test.afterEach(async ({ request }) => {
    await deleteOrder(request, order);
  });

  test('leftover confirmation', async ({ request }) => {
    await confirmOrder(request, order);
    expect(order).toBeTruthy();
  });
});

test('dependent removed first', async ({ request }) => {
  const order = await createOrder(request);
  try {
    await confirmOrder(request, order);
  } finally {
    await deleteConfirmation(request, order);
    await deleteOrder(request, order);
  }
});

test('cascade and inert', async ({ request }) => {
  const order = await createOrder(request);
  try {
    await archiveOrder(request, order);
    await renameOrder(request, order, 'b');
  } finally {
    await deleteOrder(request, order);
  }
});

test('parent first', async ({ request }) => {
  const order = await createOrder(request);
  try {
    await confirmOrder(request, order);
  } finally {
    await deleteOrder(request, order);
    await deleteConfirmation(request, order);
  }
});

test('dependent removed in body', async ({ request }) => {
  const order = await createOrder(request);
  try {
    await confirmOrder(request, order);
    await deleteConfirmation(request, order);
  } finally {
    await deleteOrder(request, order);
  }
});
`

const teardownRSpec = `RSpec.describe 'Orders' do
  let!(:order) { create_order(name: 'a') }

  after do
    delete_order(order)
  end

  it 'leaves a confirmation' do
    confirm_order(order)
    expect(order).to be_present
  end

  it 'removes the confirmation first' do
    confirm_order(order)
    delete_confirmation(order)
  end

  it 'archives with cascade' do
    archive_order(order)
    rename_order(order)
  end

  context 'with a nested cleanup' do
    after do
      delete_confirmation order
    end

    it 'cleans the dependent in an inner hook' do
      confirm_order(order)
    end
  end

  it 'deletes the parent first in ensure' do
    confirm_order(order)
  ensure
    delete_order(order)
    delete_confirmation(order)
  end
end
`

func teardownReport(t *testing.T, files map[string]string, options TeardownOptions) TeardownReport {
	t.Helper()
	root := t.TempDir()
	files[".corvint/side-effects.json"] = teardownDeclaration
	commitFiles(t, root, files)
	if options.Declaration == "" {
		options.Declaration = ".corvint/side-effects.json"
	}
	report, err := TeardownDependents(OpenRepo(context.Background(), root), options)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// resourcesByTest indexes a report's resources by test title.
func resourcesByTest(report TeardownReport) map[string]TeardownResource {
	out := map[string]TeardownResource{}
	for _, f := range report.Files {
		for _, test := range f.Tests {
			for _, r := range test.Resources {
				out[test.Test] = r
			}
		}
	}
	return out
}

func findingsByTest(report TeardownReport) map[string][]TeardownFinding {
	out := map[string][]TeardownFinding{}
	for _, f := range report.Findings {
		out[f.Test] = append(out[f.Test], f)
	}
	return out
}

// TestTeardownDeclarationRefusals: the declaration is closed and validated; every defect refuses
// tests-declaration-invalid (TSA-V0-111).
func TestTeardownDeclarationRefusals(t *testing.T) {
	if _, effects, err := decodeSideEffects("d.json", []byte(teardownDeclaration)); err != nil || effects.actions["confirmOrder"].Dependent != "confirmation" || effects.creates["create_order"] != "order" {
		t.Fatalf("valid declaration: %v %+v", err, effects)
	}
	head := `{"schema":"corvint-side-effect-declaration/0","version":1,"revision":"0123456789abcdef0123456789abcdef01234567",`
	order := `{"kind":"order","create":["createOrder"],"delete":["deleteOrder"]}`
	for name, text := range map[string]string{
		"unknown field":       head + `"resources":[` + order + `],"actions":[],"graph":"derived"}`,
		"unknown nested":      head + `"resources":[{"kind":"order","create":["a"],"delete":["b"],"cascade":true}],"actions":[]}`,
		"trailing":            head + `"resources":[` + order + `],"actions":[]} {}`,
		"schema":              strings.Replace(head, "/0", "/1", 1) + `"resources":[` + order + `],"actions":[]}`,
		"version":             strings.Replace(head, `"version":1`, `"version":0`, 1) + `"resources":[` + order + `],"actions":[]}`,
		"revision":            strings.Replace(head, "0123456789abcdef0123456789abcdef01234567", "main", 1) + `"resources":[` + order + `],"actions":[]}`,
		"no resources":        head + `"resources":[],"actions":[]}`,
		"duplicate kind":      head + `"resources":[` + order + `,{"kind":"order","create":["c"],"delete":["d"]}],"actions":[]}`,
		"bad kind":            head + `"resources":[{"kind":"Order","create":["a"],"delete":["b"]}],"actions":[]}`,
		"no delete":           head + `"resources":[{"kind":"order","create":["a"],"delete":[]}],"actions":[]}`,
		"duplicate call":      head + `"resources":[` + order + `],"actions":[{"call":"createOrder"}]}`,
		"bad call":            head + `"resources":[` + order + `],"actions":[{"call":"order.confirm"}]}`,
		"undeclared parent":   head + `"resources":[` + order + `],"actions":[{"call":"x","parent":"invoice"}]}`,
		"dependent no parent": head + `"resources":[` + order + `],"actions":[{"call":"x","dependent":"order"}]}`,
		"cascade no dep":      head + `"resources":[` + order + `],"actions":[{"call":"x","parent":"order","cascade":true}]}`,
	} {
		if _, _, err := decodeSideEffects("d.json", []byte(text)); !hasCode(err, "tests-declaration-invalid") {
			t.Errorf("%s: %v", name, err)
		}
	}
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"e2e/a.spec.ts": teardownJS, "bad.json": `{"schema":"x"}`})
	repo := OpenRepo(context.Background(), root)
	for _, c := range []struct {
		options TeardownOptions
		code    string
	}{
		{TeardownOptions{Files: []string{"e2e/a.spec.ts"}, Declaration: "bad.json"}, "tests-declaration-invalid"},
		{TeardownOptions{Files: []string{"e2e/a.spec.ts"}, Declaration: "missing.json"}, "tests-source-unavailable"},
		{TeardownOptions{Files: []string{"e2e/a.spec.ts"}, Declaration: "../d.json"}, "tests-invalid-arguments"},
	} {
		if _, err := TeardownDependents(repo, c.options); !hasCode(err, c.code) {
			t.Errorf("%+v: %v, want %s", c.options, err, c.code)
		}
	}
}

// TestTeardownPhasesJS: beforeEach is setup, the body is body, finally and afterEach are teardown;
// finally runs before every hook and an inner hook before an outer one (TSA-V0-112).
func TestTeardownPhasesJS(t *testing.T) {
	read := readJSTeardown("e2e/a.spec.ts", `test.describe('outer', () => {
  test.beforeEach(async () => { await createOrder(); });
  test.afterEach(async () => { await deleteOrder(); });
  test.describe('inner', () => {
    test.afterEach(async () => { await deleteConfirmation(); });
    test('one', async () => {
      try { await confirmOrder(); } finally { await cleanUp(); }
    });
  });
  test.beforeAll(async () => { await createAuditEntry(); });
});
`)
	if read.framework != "jsts" || len(read.examples) != 1 {
		t.Fatalf("read %+v", read)
	}
	ex := read.examples[0]
	got := []string{}
	for _, c := range ex.calls {
		got = append(got, c.phase+":"+c.name)
	}
	want := []string{"setup:createOrder", "body:confirmOrder", "teardown:cleanUp", "teardown:deleteConfirmation", "teardown:deleteOrder"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls %v, want %v", got, want)
	}
	for k := 2; k < 4; k++ {
		if before, known := runsBefore(ex.calls[k], ex.calls[k+1]); !before || !known {
			t.Errorf("%s should run before %s", ex.calls[k].name, ex.calls[k+1].name)
		}
	}
	if len(ex.unread) != 1 || ex.unread[0].name != "createAuditEntry" {
		t.Errorf("unread %+v", ex.unread)
	}
}

// TestTeardownPhasesRSpec: let/before are setup, ensure and after hooks are teardown, an inner
// after runs before an outer one and two after hooks of one group have no known order (TSA-V0-112).
func TestTeardownPhasesRSpec(t *testing.T) {
	_, effects, err := decodeSideEffects("d.json", []byte(teardownDeclaration))
	if err != nil {
		t.Fatal(err)
	}
	read := readRSpecTeardown(`RSpec.describe Order do
  let(:order) { create_order }
  after { delete_order(order) }
  after(:each) { delete_audit_trail(order) }
  around { |example| example.run }
  context 'inner' do
    after { delete_confirmation order }
    it 'confirms' do
      confirm_order(order)
    ensure
      log_out
    end
  end
end
`, effects)
	if read.framework != "rspec" || len(read.examples) != 1 {
		t.Fatalf("read %+v", read)
	}
	ex := read.examples[0]
	phases := map[string]string{}
	index := map[string]int{}
	for k, c := range ex.calls {
		phases[c.name], index[c.name] = c.phase, k
	}
	for name, phase := range map[string]string{"create_order": phaseSetup, "confirm_order": phaseBody, "log_out": phaseTeardown, "delete_confirmation": phaseTeardown, "delete_order": phaseTeardown, "delete_audit_trail": phaseTeardown} {
		if phases[name] != phase {
			t.Errorf("%s phase %q, want %q (calls %+v)", name, phases[name], phase, ex.calls)
		}
	}
	if ex.calls[index["create_order"]].assigned != "order" {
		t.Errorf("let binding %+v", ex.calls[index["create_order"]])
	}
	order := func(a, b string) (bool, bool) { return runsBefore(ex.calls[index[a]], ex.calls[index[b]]) }
	if before, known := order("log_out", "delete_confirmation"); !before || !known {
		t.Error("ensure should run before the hooks")
	}
	if before, known := order("delete_confirmation", "delete_order"); !before || !known {
		t.Error("the inner after should run before the outer one")
	}
	if _, known := order("delete_order", "delete_audit_trail"); known {
		t.Error("two after hooks of one group have no known order")
	}
	if len(ex.unread) != 0 {
		t.Errorf("around without a declared call should read nothing: %+v", ex.unread)
	}
}

// TestTeardownLeftoverJS: a declared action whose dependent is not removed, or removed after the
// parent, is a declared finding with its lines; a dependent removed first, a cascade and an inert
// action are not (TSA-V0-113, TSA-V0-115, TSA-V0-116, TSA-V0-118).
func TestTeardownLeftoverJS(t *testing.T) {
	report := teardownReport(t, map[string]string{"e2e/orders.spec.ts": teardownJS}, TeardownOptions{Files: []string{"e2e/orders.spec.ts"}})
	if report.Schema != TeardownSchema || report.GraphSource != "declaration" || report.Declaration.Version != 3 || len(report.Files) != 1 || report.Files[0].Framework != "jsts" || report.Files[0].Examples != 5 {
		t.Fatalf("report header %+v", report)
	}
	resources := resourcesByTest(report)
	findings := findingsByTest(report)
	leftover := findings["leftover confirmation"]
	if len(leftover) != 1 || leftover[0].Cause != causeNotRemoved || leftover[0].Basis != basisDeclared || leftover[0].ActionLine != 13 || leftover[0].DeleteLine != 9 || leftover[0].Dependent != "confirmation" || leftover[0].Kind != "order" {
		t.Errorf("leftover %+v", leftover)
	}
	if r := resources["leftover confirmation"]; r.Variable != "order" || r.CreateLine != 6 || r.Verdict != tdVerdictLeftover || len(r.Actions) != 1 || r.Actions[0].Line != 13 {
		t.Errorf("leftover resource %+v", r)
	}
	first := findings["parent first"]
	if len(first) != 1 || first[0].Cause != causeParentFirst || first[0].Basis != basisDeclared || first[0].ActionLine != 41 || first[0].DeleteLine != 43 || first[0].DependentDeleteLine != 44 {
		t.Errorf("parent first %+v", first)
	}
	for _, title := range []string{"dependent removed first", "cascade and inert", "dependent removed in body"} {
		if len(findings[title]) != 0 || resources[title].Verdict != tdVerdictNone || len(resources[title].Reasons) != 0 {
			t.Errorf("%s: %+v %+v", title, findings[title], resources[title])
		}
	}
	actions := resources["cascade and inert"].Actions
	if len(actions) != 2 || !actions[0].Cascade || actions[1].Call != "renameOrder" || !actions[1].Declared || actions[1].Dependent != "" {
		t.Errorf("cascade and inert actions %+v", actions)
	}
	if report.Declared() != 2 {
		t.Errorf("declared findings %d", report.Declared())
	}
}

// TestTeardownLeftoverRSpec: the RSpec reading gives the same findings through let!, after hooks
// and ensure (TSA-V0-113, TSA-V0-115, TSA-V0-116).
func TestTeardownLeftoverRSpec(t *testing.T) {
	report := teardownReport(t, map[string]string{"spec/requests/orders_spec.rb": teardownRSpec}, TeardownOptions{Files: []string{"spec/requests/orders_spec.rb"}})
	if len(report.Files) != 1 || report.Files[0].Framework != "rspec" || report.Files[0].Examples != 5 {
		t.Fatalf("file %+v", report.Files)
	}
	findings := findingsByTest(report)
	resources := resourcesByTest(report)
	if f := findings["leaves a confirmation"]; len(f) != 1 || f[0].Cause != causeNotRemoved || f[0].Basis != basisDeclared || f[0].ActionLine != 9 || f[0].DeleteLine != 5 {
		t.Errorf("leaves %+v", f)
	}
	if f := findings["deletes the parent first in ensure"]; len(f) != 1 || f[0].Cause != causeParentFirst || f[0].ActionLine != 34 || f[0].DeleteLine != 36 || f[0].DependentDeleteLine != 37 {
		t.Errorf("ensure %+v", f)
	}
	for _, title := range []string{"removes the confirmation first", "archives with cascade", "cleans the dependent in an inner hook"} {
		if len(findings[title]) != 0 || resources[title].Verdict != tdVerdictNone {
			t.Errorf("%s: %+v %+v", title, findings[title], resources[title])
		}
	}
	if r := resources["leaves a confirmation"]; r.Variable != "order" || r.CreateLine != 2 {
		t.Errorf("resource %+v", r)
	}
}

// TestTeardownIncompleteGraph: an undeclared action, an unbound create, an unread hook calling a
// declared name, an undeclared teardown call before the parent delete and an unknown teardown
// order each keep the resource's findings advisory with that reason; the verdict is unknown when
// there is no finding, never safe (TSA-V0-114, TSA-V0-118).
func TestTeardownIncompleteGraph(t *testing.T) {
	js := `test('undeclared action', async ({ request }) => {
  const order = await createOrder(request);
  try {
    await shipOrder(request, order);
  } finally {
    await deleteOrder(request, order);
  }
});

test('unbound create', async ({ request }) => {
  await createOrder(request);
  try {
    await confirmOrder(request);
  } finally {
    await deleteOrder(request);
  }
});

test.describe('closing', () => {
  let order;
  test.beforeAll(async () => { await createAuditEntry(); });
  test.beforeEach(async ({ request }) => { order = await createOrder(request); });
  test.afterEach(async ({ page, request }) => { await page.close(); await deleteOrder(request, order); });
  test('undeclared teardown', async ({ request }) => {
    await confirmOrder(request, order);
  });
});
`
	rb := `RSpec.describe 'Orders' do
  let(:order) { create_order }
  after { delete_confirmation(order) }
  after { delete_order(order) }

  it 'has two hooks of one group' do
    confirm_order(order)
  end
end
`
	report := teardownReport(t, map[string]string{"e2e/incomplete.spec.ts": js, "spec/incomplete_spec.rb": rb}, TeardownOptions{Files: []string{"e2e/incomplete.spec.ts", "spec/incomplete_spec.rb"}})
	resources := resourcesByTest(report)
	findings := findingsByTest(report)
	if r := resources["undeclared action"]; r.Verdict != tdVerdictUnknown || !reflect.DeepEqual(r.Reasons, []string{reasonActionNotDeclared}) || len(r.Actions) != 1 || r.Actions[0].Declared {
		t.Errorf("undeclared action %+v", r)
	}
	if f := findings["unbound create"]; len(f) != 1 || f[0].Basis != basisAdvisory || !reflect.DeepEqual(f[0].Reasons, []string{reasonNotBound}) || !strings.Contains(f[0].Message, "advisory") {
		t.Errorf("unbound create %+v", f)
	}
	if f := findings["undeclared teardown"]; len(f) != 1 || f[0].Basis != basisAdvisory || !reflect.DeepEqual(f[0].Reasons, []string{reasonHookNotRead, reasonTeardownNotDeclared}) {
		t.Errorf("undeclared teardown %+v", f)
	}
	if f := findings["has two hooks of one group"]; len(f) != 1 || f[0].Cause != causeParentFirst || f[0].Basis != basisAdvisory || !reflect.DeepEqual(f[0].Reasons, []string{reasonOrderUnknown}) || f[0].DependentDeleteLine != 0 {
		t.Errorf("two hooks %+v", f)
	}
	if report.Declared() != 0 {
		t.Errorf("advisory findings must not count as declared: %d", report.Declared())
	}
	encoded, _ := json.Marshal(report)
	if strings.Contains(strings.ToLower(string(encoded)), "safe") {
		t.Error("the report must never call a teardown safe")
	}
}

// TestTeardownReceiptJoin: a JS receipt bound to the committed blob attaches a failed attempt at
// the parent delete line with its DELETE 409/422 status and dependent kind; a recorded conflict on
// a resource with no other finding is an advisory finding; other digests, missing files and non-JS
// inputs abstain (TSA-V0-117).
func TestTeardownReceiptJoin(t *testing.T) {
	sum := sha256.Sum256([]byte(teardownJS))
	digest := hex.EncodeToString(sum[:])
	failed := func(title string, line, status int) jstestprovider.TestOutcome {
		return jstestprovider.TestOutcome{Name: title, FullName: "orders " + title, State: jstestprovider.StateFailed,
			AttemptDetails: []jstestprovider.AttemptDetail{{State: jstestprovider.StateFailed, Retry: 0, Anchor: &jstestprovider.Anchor{File: "/work/e2e/orders.spec.ts", Line: line},
				RequestOutcomes: &jstestprovider.RequestOutcomeRecord{Requests: []jstestprovider.RequestOutcome{{Method: "delete", Route: "/orders/:id", Status: status}}}}}}
	}
	bound := &jstestprovider.Receipt{Identity: jstestprovider.Identity{TestFileDigests: map[string]string{"/work/e2e/orders.spec.ts": digest}},
		Tests: []jstestprovider.TestOutcome{failed("leftover confirmation", 9, 409), failed("dependent removed first", 24, 422), failed("parent first", 99, 409)}}
	other := &jstestprovider.Receipt{Identity: jstestprovider.Identity{TestFileDigests: map[string]string{"/work/e2e/orders.spec.ts": strings.Repeat("0", 64)}}, Tests: bound.Tests}
	missing := &jstestprovider.Receipt{Identity: jstestprovider.Identity{TestFileDigests: map[string]string{"/work/e2e/other.spec.ts": digest}}, Tests: bound.Tests}
	ambiguous := &jstestprovider.Receipt{Identity: jstestprovider.Identity{TestFileDigests: map[string]string{"/a/e2e/orders.spec.ts": digest, "/b/e2e/orders.spec.ts": digest}}, Tests: bound.Tests}
	report := teardownReport(t, map[string]string{"e2e/orders.spec.ts": teardownJS}, TeardownOptions{Files: []string{"e2e/orders.spec.ts"}, Receipts: []TeardownReceipt{
		{Name: "bound.json", JS: bound}, {Name: "other.json", JS: other}, {Name: "missing.json", JS: missing}, {Name: "ambiguous.json", JS: ambiguous}, {Name: "ruby.json"},
	}})
	reasons := []string{}
	for _, b := range report.Receipts {
		reasons = append(reasons, b.Reason)
	}
	if want := []string{"", "digest-mismatch", "file-not-in-receipt", "file-ambiguous", "receipt-not-js"}; !reflect.DeepEqual(reasons, want) || !report.Receipts[0].Bound || report.Receipts[0].Rows != 3 {
		t.Fatalf("bindings %+v", report.Receipts)
	}
	findings := findingsByTest(report)
	if f := findings["leftover confirmation"]; len(f) != 1 || len(f[0].Recorded) != 1 || f[0].Recorded[0].Status != 409 || f[0].Recorded[0].Dependent != "confirmation" || f[0].Recorded[0].Receipt != "bound.json" {
		t.Errorf("leftover recorded %+v", f)
	}
	if f := findings["dependent removed first"]; len(f) != 1 || f[0].Cause != causeRecordedFailure || f[0].Basis != basisAdvisory || f[0].Recorded[0].Status != 422 || f[0].DeleteLine != 24 {
		t.Errorf("recorded conflict %+v", f)
	}
	if f := findings["parent first"]; len(f) != 1 || len(f[0].Recorded) != 0 {
		t.Errorf("a row at another line must not join: %+v", f)
	}
}

// TestTeardownFrameworksAndFiles: a _spec.rb file is RSpec, a JS/TS test file is jsts, another file
// is uncertain; paths and bounds are validated and the report is deterministic (TSA-V0-110,
// TSA-V0-119).
func TestTeardownFrameworksAndFiles(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"e2e/orders.spec.ts": teardownJS, "spec/orders_spec.rb": teardownRSpec, "lib/orders.ts": "export {};\n", ".corvint/side-effects.json": teardownDeclaration})
	repo := OpenRepo(context.Background(), root)
	options := TeardownOptions{Files: []string{"spec/orders_spec.rb", "e2e/orders.spec.ts", "lib/orders.ts", "e2e/orders.spec.ts"}, Declaration: ".corvint/side-effects.json"}
	report, err := TeardownDependents(repo, options)
	if err != nil {
		t.Fatal(err)
	}
	frameworks := []string{}
	for _, f := range report.Files {
		frameworks = append(frameworks, f.Path+"="+f.Framework+"/"+f.Reason)
	}
	if want := []string{"spec/orders_spec.rb=rspec/", "e2e/orders.spec.ts=jsts/", "lib/orders.ts=/not-a-test-file"}; !reflect.DeepEqual(frameworks, want) {
		t.Errorf("files %v", frameworks)
	}
	again, err := TeardownDependents(repo, options)
	first, _ := json.Marshal(report)
	second, _ := json.Marshal(again)
	if err != nil || string(first) != string(second) {
		t.Error("the report must be deterministic")
	}
	many := make([]string, MaxValidityFiles+1)
	for i := range many {
		many[i] = "spec/" + strings.Repeat("x", i+1) + "_spec.rb"
	}
	for _, c := range []struct {
		options TeardownOptions
		code    string
	}{
		{TeardownOptions{Declaration: ".corvint/side-effects.json"}, "tests-invalid-arguments"},
		{TeardownOptions{Files: []string{".git/config"}, Declaration: ".corvint/side-effects.json"}, "tests-invalid-arguments"},
		{TeardownOptions{Files: []string{"spec/missing_spec.rb"}, Declaration: ".corvint/side-effects.json"}, "tests-source-unavailable"},
		{TeardownOptions{Files: many, Declaration: ".corvint/side-effects.json"}, "tests-bound-exceeded"},
		{TeardownOptions{Files: []string{"spec/orders_spec.rb"}, Declaration: ".corvint/side-effects.json", Receipts: make([]TeardownReceipt, MaxTeardownReceipts+1)}, "tests-bound-exceeded"},
		{TeardownOptions{Files: []string{"spec/orders_spec.rb"}, Declaration: ".corvint/side-effects.json", Revision: "no-such-rev"}, "tests-invalid-revision"},
	} {
		if _, err := TeardownDependents(repo, c.options); !hasCode(err, c.code) {
			t.Errorf("%+v: %v, want %s", c.options.Files, err, c.code)
		}
	}
}
