package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const qualifiedVersion = "1.64.0"

// fixtureRun builds a json-reporter document of the TOL-V0-060 fixture as
// Playwright 1.61.1 was observed to write it; edit may change it first.
func fixtureRun(t *testing.T, version string, edit func([]pwSpec)) string {
	t.Helper()
	file := obligation.FixtureName
	twice := func(status string, steps ...pwStep) pwTest {
		return pwTest{ExpectedStatus: "passed", ProjectName: "chromium", Results: []pwResult{
			{Status: status, Retry: 0, Steps: steps, Stdout: []string{}}, {Status: status, Retry: 1, Steps: steps, Stdout: []string{}}}}
	}
	retried := pwTest{ExpectedStatus: "passed", ProjectName: "chromium", Results: []pwResult{
		{Status: "failed", Retry: 0, Steps: []pwStep{}, Stdout: []string{}}, {Status: "passed", Retry: 1, Steps: []pwStep{}, Stdout: []string{}}}}
	specs := []pwSpec{
		{Title: "soft steps", ID: "q1", File: file, Tests: []pwTest{twice("failed", step("TOLQ-1 passes", false), step("TOLQ-2 soft-fails", true),
			step("TOLQ-3 passes after a soft failure", false), step("TOLQ-4 parent", true, step("nested soft failure", true)))}},
		{Title: "TOLQ-5 soft failure outside steps", ID: "q2", File: file, Tests: []pwTest{twice("failed", step("TOLQ-6 passes", false))}},
		{Title: "TOLQ-7 fails at retry 0", ID: "q3", File: file, Tests: []pwTest{retried}},
		{Title: "TOLQ-8 passes", ID: "q4", File: file, Tests: []pwTest{passed("chromium")}},
		{Title: "TOLQ-9 expected-fail", ID: "q5", File: file, Tests: []pwTest{expectedFail("chromium", "", "failed")}},
	}
	if edit != nil {
		edit(specs)
	}
	return writeReport(t, t.TempDir(), filepath.Join(t.TempDir(), "qualify"), version, specs...)
}

func qualifyVersion(t *testing.T, root, report string) run {
	t.Helper()
	return atm(t, root, nil, "ticket", "obligations", "qualify-version", "--from-playwright-report", report)
}

// TestTOLV0060_PrintFixture: --print-fixture prints the embedded fixture,
// its digest and expected classes, and writes nothing.
func TestTOLV0060_PrintFixture(t *testing.T) {
	dir := t.TempDir()
	x := atm(t, dir, nil, "ticket", "obligations", "qualify-version", "--print-fixture")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("print: %s", x.stdout)
	}
	it := x.res.Items[0]
	text := field(it, "fixture").Str
	if wire.Sum([]byte(text)) != obligation.FixtureSha256() || field(it, "fixtureSha256").Str != string(obligation.FixtureSha256()) ||
		field(it, "fixtureName").Str != obligation.FixtureName || strList(it, "credited") != "TOLQ-1,TOLQ-3,TOLQ-6,TOLQ-8" ||
		strList(it, "failed") != "TOLQ-2,TOLQ-4,TOLQ-5,TOLQ-7" || strList(it, "defectConfirmed") != "TOLQ-9" {
		t.Fatalf("fixture item: %s", x.stdout)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("print-fixture wrote %v %v", entries, err)
	}
	for _, args := range [][]string{{}, {"--print-fixture", "--from-playwright-report", "r.json"}} {
		if y := atm(t, dir, nil, append([]string{"ticket", "obligations", "qualify-version"}, args...)...); y.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%v accepted: %s", args, y.stdout)
		}
	}
}

// TestTOLV0061_QualificationRefusals: a report whose classes differ from the
// fixture's refuses GATE_FAILED OBLIGATION_QUALIFICATION_FAILED: naming the
// id; a malformed report refuses OBLIGATION_REPORT:; neither writes the
// allow-list or any store state.
func TestTOLV0061_QualificationRefusals(t *testing.T) {
	r, _, _ := obligationRepo(t)
	before := fixture.TreeSnapshot(t, r.StateDir)
	mismatch := fixtureRun(t, qualifiedVersion, func(s []pwSpec) { s[2].Tests[0].Results[0].Status = "passed" })
	x := qualifyVersion(t, r.Root, mismatch)
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeGateFailed) || !strings.Contains(string(x.stdout), "OBLIGATION_QUALIFICATION_FAILED:") ||
		!strings.Contains(string(x.stdout), "TOLQ-7 expected failed, observed credited") {
		t.Fatalf("mismatch: %s", x.stdout)
	}
	shape := filepath.Join(t.TempDir(), "shape.json")
	fixture.Write(t, shape, []byte(`{"config":{"rootDir":"/x"},"errors":[],"stats":{},"suites":[]}`))
	if y := qualifyVersion(t, r.Root, shape); y.res.Outcome == wire.OutcomeOK || !hasCode(y.res, wire.CodeMalformed) || !strings.Contains(string(y.stdout), "OBLIGATION_REPORT:") {
		t.Fatalf("shape: %s", y.stdout)
	}
	if _, err := os.Lstat(obligation.AllowListPath(r.CommonDir)); !os.IsNotExist(err) {
		t.Fatalf("a refused qualification wrote the allow-list: %v", err)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a refused qualification wrote store state")
	}
}

// TestTOLV0062_AllowListRecorded: a passing qualification records the
// version with its evidence beside the store and writes no store state; a
// repeat replaces the entry; an unreadable allow-list refuses a report
// witness rather than admitting nothing or everything.
func TestTOLV0062_AllowListRecorded(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	before := fixture.TreeSnapshot(t, r.StateDir)
	report := fixtureRun(t, qualifiedVersion, nil)
	x := qualifyVersion(t, r.Root, report)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("qualify: %s", x.stdout)
	}
	raw, _ := os.ReadFile(report)
	it := x.res.Items[0]
	p := obligation.AllowListPath(r.CommonDir)
	if field(it, "version").Str != qualifiedVersion || field(it, "replaced").Bool || field(it, "reportSha256").Str != string(wire.Sum(raw)) ||
		field(it, "fixtureSha256").Str != string(obligation.FixtureSha256()) || field(it, "qualifiedAt").Str == "" {
		t.Fatalf("evidence: %s", x.stdout)
	}
	if got, err := filepath.EvalSymlinks(field(it, "allowList").Str); err != nil || got != mustEval(t, p) {
		t.Fatalf("allow-list path %s: %v", field(it, "allowList").Str, err)
	}
	a, err := obligation.ReadAllowList(p)
	if err != nil || strings.Join(a.Versions(), ",") != qualifiedVersion || a.Entries[qualifiedVersion].ReportSha256 != wire.Sum(raw) {
		t.Fatalf("allow-list: %v %v", a, err)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("qualify-version wrote store state")
	}
	if y := qualifyVersion(t, r.Root, fixtureRun(t, qualifiedVersion, nil)); y.res.Outcome != wire.OutcomeOK || !field(y.res.Items[0], "replaced").Bool {
		t.Fatalf("requalify: %s", y.stdout)
	}
	fixture.Write(t, p, []byte(`{"profile":"taskman-obligation-qualifications/0","versions":[]}`+"\n"))
	w := writeReport(t, t.TempDir(), r.Root, qualifiedVersion, pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	if z := atm(t, r.Root, nil, witnessArgs(id, "w-bad-list", obligationRevision(t, r.Root, id), head, "--from-playwright-report", w)...); z.res.Outcome == wire.OutcomeOK ||
		!hasCode(z.res, wire.CodeMalformed) || !strings.Contains(string(z.stdout), obligation.AllowListFile) {
		t.Fatalf("malformed allow-list admitted: %s", z.stdout)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	got, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestTOLV0063_QualifiedVersionAdmitted: before qualification a report of
// the version refuses exactly as before (UNSUPPORTED_VERSION,
// OBLIGATION_REPORT_VERSION_UNQUALIFIED:); after it the same report credits;
// an unqualified version still refuses.
func TestTOLV0063_QualifiedVersionAdmitted(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	spec := pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}}
	report := writeReport(t, t.TempDir(), r.Root, qualifiedVersion, spec)
	unqualified := func(req, p string) {
		t.Helper()
		x := atm(t, r.Root, nil, witnessArgs(id, req, obligationRevision(t, r.Root, id), head, "--from-playwright-report", p)...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedVersion) || !strings.Contains(string(x.stdout), "OBLIGATION_REPORT_VERSION_UNQUALIFIED:") {
			t.Fatalf("unqualified: %s", x.stdout)
		}
	}
	unqualified("w-before", report)
	if x := qualifyVersion(t, r.Root, fixtureRun(t, qualifiedVersion, nil)); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("qualify: %s", x.stdout)
	}
	unqualified("w-other", writeReport(t, t.TempDir(), r.Root, "1.65.0", spec))
	x := atm(t, r.Root, nil, witnessArgs(id, "w-after", obligationRevision(t, r.Root, id), head, "--from-playwright-report", report)...)
	if x.res.Outcome != wire.OutcomeOK || strList(x.res.Items[0], "credited") != "AC-1" || !auditConsistent(t, r.Root) {
		t.Fatalf("qualified witness: %s", x.stdout)
	}
	for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
		if field(e, "id").Str != "AC-1" {
			continue
		}
		if _, has := field(e, "evidence").Obj.Get("testRepository"); has {
			t.Fatal("a witness without --test-repository gained a testRepository member")
		}
	}
}

// TestTOLV0064_PolicyTestRepositories: obligations.testRepositories decodes
// 1..16 label names to non-empty paths without control characters; a policy
// without it decodes as before.
func TestTOLV0064_PolicyTestRepositories(t *testing.T) {
	decode := func(repos wire.Value) (*intent.Policy, error) {
		p := fixture.PolicyValue()
		o := wire.NewObject().Set("workerWitness", wire.Bool(false))
		if repos.Kind != wire.KindNull {
			o.Set("testRepositories", repos)
		}
		p.Obj.Set("obligations", wire.ObjectValue(o))
		return intent.DecodePolicy(wire.EncodeFile(p))
	}
	pol, err := decode(wire.ObjectValue(wire.NewObject().Set("app", wire.String("../app")).Set("abs", wire.String("/srv/e2e"))))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dir, ok := pol.ObligationTestRepository("app"); !ok || intent.ResolveTestRepository("/w/main", dir) != "/w/app" {
		t.Fatalf("app: %q %v", dir, ok)
	}
	if dir, _ := pol.ObligationTestRepository("abs"); intent.ResolveTestRepository("/w/main", dir) != "/srv/e2e" {
		t.Fatalf("abs: %q", dir)
	}
	if _, ok := pol.ObligationTestRepository("other"); ok {
		t.Fatal("undeclared name resolved")
	}
	plain, err := decode(wire.Value{Kind: wire.KindNull})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plain.ObligationTestRepository("app"); ok {
		t.Fatal("a policy without testRepositories declares one")
	}
	var nilPolicy *intent.Policy
	if _, ok := nilPolicy.ObligationTestRepository("app"); ok {
		t.Fatal("nil policy declares one")
	}
	many := wire.NewObject()
	for i := 0; i <= intent.MaxObligationTestRepositories; i++ {
		many.Set("r"+strings.Repeat("x", i), wire.String("d"))
	}
	for name, c := range map[string]struct {
		v    wire.Value
		code string
	}{
		"empty":     {wire.ObjectValue(wire.NewObject()), wire.CodeLimitExceeded},
		"too many":  {wire.ObjectValue(many), wire.CodeLimitExceeded},
		"empty dir": {wire.ObjectValue(wire.NewObject().Set("app", wire.String(""))), wire.CodeMalformed},
		"newline":   {wire.ObjectValue(wire.NewObject().Set("app", wire.String("a\nb"))), wire.CodeMalformed},
		"not obj":   {wire.String("app"), wire.CodeMalformed},
	} {
		if _, err := decode(c.v); wire.CodeOf(err) != c.code {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// testRepoPolicy declares the test repositories in names.
func testRepoPolicy(names map[string]string) func(wire.Value) {
	return func(p wire.Value) {
		repos := wire.NewObject()
		for k, v := range names {
			repos.Set(k, wire.String(v))
		}
		p.Obj.Set("obligations", wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(false)).Set("testRepositories", wire.ObjectValue(repos))))
	}
}

// separateTestRepo is a Git repository outside the store holding the
// obligation spec; it returns its root and HEAD.
func separateTestRepo(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "app")
	fixture.Write(t, filepath.Join(root, "e2e", "login.spec.ts"), []byte(obligationSpec))
	git(t, root, "init", "-q")
	git(t, root, "add", "e2e")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "app spec")
	return mustEval(t, root), headOID(t, root)
}

// TestTOLV0065_SeparateTestRepository1_64: a Playwright 1.64 report run in
// a separate, policy-declared test repository credits a row whose test
// passed at retry 0; the evidence names the repository; the witness passes
// receipt audit (AC2, AC3).
func TestTOLV0065_SeparateTestRepository1_64(t *testing.T) {
	app, appHead := separateTestRepo(t)
	r, id, _ := obligationRepoWith(t, testRepoPolicy(map[string]string{"app": app}))
	seedObligations(t, r.Root, id)
	if x := qualifyVersion(t, r.Root, fixtureRun(t, qualifiedVersion, nil)); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("qualify: %s", x.stdout)
	}
	report := writeReport(t, t.TempDir(), app, qualifiedVersion,
		pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{{ExpectedStatus: "passed", ProjectName: "chromium",
			Results: []pwResult{{Status: "passed", Retry: 0, Steps: []pwStep{}, Stdout: []string{}}}}}},
		pwSpec{Title: "AC-5 failing", ID: "s4", File: "e2e/login.spec.ts", Tests: []pwTest{{ExpectedStatus: "passed", ProjectName: "chromium",
			Results: []pwResult{{Status: "failed", Retry: 0, Steps: []pwStep{}, Stdout: []string{}}, {Status: "passed", Retry: 1, Steps: []pwStep{}, Stdout: []string{}}}}}})
	x := atm(t, r.Root, nil, witnessArgs(id, "w-app", obligationRevision(t, r.Root, id), appHead, "--from-playwright-report", report, "--test-repository", "app")...)
	if x.res.Outcome != wire.OutcomeOK || strList(x.res.Items[0], "credited") != "AC-1" || strList(x.res.Items[0], "failed") != "AC-5" {
		t.Fatalf("test-repository witness: %s", x.stdout)
	}
	for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
		if field(e, "id").Str != "AC-1" {
			continue
		}
		ev := field(e, "evidence")
		if field(e, "state").Str != "WITNESSED" || field(ev, "testRepository").Str != "app" || field(ev, "commit").Str != appHead ||
			field(ev, "playwrightVersion").Str != qualifiedVersion {
			t.Fatalf("evidence: %s", wire.Encode(e))
		}
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("receipt audit is not consistent after a test-repository witness")
	}
}

// TestTOLV0066_AuditInTestRepository: receipt audit re-checks the retained
// matches in the declared test repository, so removing it makes the audit
// inconsistent.
func TestTOLV0066_AuditInTestRepository(t *testing.T) {
	app, appHead := separateTestRepo(t)
	r, id, _ := obligationRepoWith(t, testRepoPolicy(map[string]string{"app": app}))
	seedObligations(t, r.Root, id)
	if x := qualifyVersion(t, r.Root, fixtureRun(t, qualifiedVersion, nil)); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("qualify: %s", x.stdout)
	}
	report := writeReport(t, t.TempDir(), app, qualifiedVersion, pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-app", obligationRevision(t, r.Root, id), appHead, "--from-playwright-report", report, "--test-repository", "app")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("audit before removal")
	}
	if err := os.RemoveAll(app); err != nil {
		t.Fatal(err)
	}
	if auditConsistent(t, r.Root) {
		t.Fatal("audit stayed consistent without the test repository")
	}
}

// TestTOLV0067_UndeclaredRefusalsUnchanged: without --test-repository a
// report from another repository refuses or binds nothing exactly as
// before; an undeclared name refuses MALFORMED
// OBLIGATION_TEST_REPOSITORY_UNDECLARED: at the command and at the writer;
// a declared path that is not a Git worktree refuses MISSING_EVIDENCE.
func TestTOLV0067_UndeclaredRefusalsUnchanged(t *testing.T) {
	app, appHead := separateTestRepo(t)
	r, id, head := obligationRepoWith(t, testRepoPolicy(map[string]string{"app": app, "plain": t.TempDir()}))
	seedObligations(t, r.Root, id)
	if x := qualifyVersion(t, r.Root, fixtureRun(t, qualifiedVersion, nil)); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("qualify: %s", x.stdout)
	}
	report := writeReport(t, t.TempDir(), app, qualifiedVersion, pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	before := fixture.TreeSnapshot(t, r.StateDir)
	w := func(req, commit string, extra ...string) run {
		return atm(t, r.Root, nil, witnessArgs(id, req, obligationRevision(t, r.Root, id), commit, append([]string{"--from-playwright-report", report}, extra...)...)...)
	}
	if x := w("w-no-flag-app-commit", appHead); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMissingEvidence) {
		t.Fatalf("app commit in the primary repository: %s", x.stdout)
	}
	if x := w("w-no-flag", head); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "written").Bool || strList(x.res.Items[0], "unbound") != "AC-1:OUTSIDE_REPOSITORY" {
		t.Fatalf("outside repository: %s", x.stdout)
	}
	if x := w("w-undeclared", appHead, "--test-repository", "other"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) ||
		!strings.Contains(string(x.stdout), "OBLIGATION_TEST_REPOSITORY_UNDECLARED:") {
		t.Fatalf("undeclared: %s", x.stdout)
	}
	if x := w("w-plain", appHead, "--test-repository", "plain"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMissingEvidence) {
		t.Fatalf("not a worktree: %s", x.stdout)
	}
	other := "other"
	payload := ticket.ObligationWitnessPayload{Source: ticket.ObligationSourceDeclared, Commit: appHead, TestRepository: &other,
		Declaration: &ticket.ObligationDeclaration{ManifestSha256: wire.Digest(strings.Repeat("d", 64)), TestID: "manual", Reason: "checked"},
		Credits:     []ticket.ObligationCredit{{ID: "AC-1"}}}
	res := cli.SubmitObligationWitness(cli.Env{Cwd: r.Root}, id, obligationRevision(t, r.Root, id), "w-writer", nil, payload.Value())
	if enc := string(wire.Encode(res.Value())); res.Outcome == wire.OutcomeOK || !strings.Contains(enc, "OBLIGATION_TEST_REPOSITORY_UNDECLARED:") {
		t.Fatalf("writer accepted an undeclared repository: %s", enc)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a refused witness wrote state")
	}
}

// TestTOLV0063_ForgedAllowListEntryNotAdmitted: an allow-list entry whose
// evidence is not the shipped fixture's (another fixtureSha256, or empty
// class lists) admits nothing, so its version still refuses
// UNSUPPORTED_VERSION; qualify-version replaces it.
func TestTOLV0063_ForgedAllowListEntryNotAdmitted(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := qualifyVersion(t, r.Root, fixtureRun(t, qualifiedVersion, nil)); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("qualify: %s", x.stdout)
	}
	p := obligation.AllowListPath(r.CommonDir)
	report := writeReport(t, t.TempDir(), r.Root, qualifiedVersion, pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	for name, forge := range map[string]func(*obligation.Qualification){
		"digest": func(q *obligation.Qualification) { q.FixtureSha256 = wire.Digest(strings.Repeat("0", 64)) },
		"empty": func(q *obligation.Qualification) {
			q.Credited, q.Failed, q.DefectConfirmed = []string{}, []string{}, []string{}
		},
	} {
		a, err := obligation.ReadAllowList(p)
		if err != nil {
			t.Fatal(err)
		}
		q := a.Entries[qualifiedVersion]
		forge(&q)
		if err := a.Put(q); err != nil {
			t.Fatal(err)
		}
		if err := obligation.WriteAllowList(p, a); err != nil {
			t.Fatal(err)
		}
		x := atm(t, r.Root, nil, witnessArgs(id, "w-forged-"+name, obligationRevision(t, r.Root, id), head, "--from-playwright-report", report)...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedVersion) || !strings.Contains(string(x.stdout), "OBLIGATION_REPORT_VERSION_UNQUALIFIED:") {
			t.Fatalf("%s entry admitted: %s", name, x.stdout)
		}
		if y := qualifyVersion(t, r.Root, fixtureRun(t, qualifiedVersion, nil)); y.res.Outcome != wire.OutcomeOK || !field(y.res.Items[0], "replaced").Bool {
			t.Fatalf("requalify: %s", y.stdout)
		}
	}
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-requalified", obligationRevision(t, r.Root, id), head, "--from-playwright-report", report)...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("requalified witness: %s", x.stdout)
	}
}

// TestTOLV0064_DeclaredDirectoryMustBeWorktreeRoot: a declared test
// repository directory that is not itself a worktree root, such as a
// directory inside the store's repository, refuses MISSING_EVIDENCE rather
// than resolving to the enclosing repository.
func TestTOLV0064_DeclaredDirectoryMustBeWorktreeRoot(t *testing.T) {
	r, id, head := obligationRepoWith(t, testRepoPolicy(map[string]string{"nested": "nested"}))
	seedObligations(t, r.Root, id)
	nested := filepath.Join(r.Root, "nested")
	fixture.Write(t, filepath.Join(nested, "e2e", "login.spec.ts"), []byte(obligationSpec))
	if x := qualifyVersion(t, r.Root, fixtureRun(t, qualifiedVersion, nil)); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("qualify: %s", x.stdout)
	}
	report := writeReport(t, t.TempDir(), mustEval(t, nested), qualifiedVersion, pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	before := fixture.TreeSnapshot(t, r.StateDir)
	x := atm(t, r.Root, nil, witnessArgs(id, "w-nested", obligationRevision(t, r.Root, id), head, "--from-playwright-report", report, "--test-repository", "nested")...)
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMissingEvidence) || !strings.Contains(string(x.stdout), "is not the root of a Git worktree") {
		t.Fatalf("nested directory resolved to the enclosing repository: %s", x.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a refused witness wrote state")
	}
}

// TestTOLV0065_WriterBindsDeclaredPath: the writer refuses STALE_POLICY,
// writing nothing, when the testRepository's declared path at write time is
// not the one the witness resolved, or when no resolved path is bound.
func TestTOLV0065_WriterBindsDeclaredPath(t *testing.T) {
	app, appHead := separateTestRepo(t)
	other, _ := separateTestRepo(t)
	r, id, _ := obligationRepoWith(t, testRepoPolicy(map[string]string{"app": app}))
	seedObligations(t, r.Root, id)
	before := fixture.TreeSnapshot(t, r.StateDir)
	name := "app"
	payload := ticket.ObligationWitnessPayload{Source: ticket.ObligationSourceDeclared, Commit: appHead, TestRepository: &name,
		Declaration: &ticket.ObligationDeclaration{ManifestSha256: wire.Digest(strings.Repeat("d", 64)), TestID: "manual", Reason: "checked"},
		Credits:     []ticket.ObligationCredit{{ID: "AC-1"}}}
	for label, check := range map[string]*mutation.ObligationReportCheck{
		"changed mapping": {TestRepository: name, TestRepositoryPath: other},
		"unbound":         nil,
	} {
		res := cli.SubmitObligationWitness(cli.Env{Cwd: r.Root}, id, obligationRevision(t, r.Root, id), "w-"+strings.ReplaceAll(label, " ", "-"), check, payload.Value())
		if enc := string(wire.Encode(res.Value())); res.Outcome == wire.OutcomeOK || !strings.Contains(enc, wire.CodeStalePolicy) {
			t.Fatalf("%s: %s", label, enc)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a refused witness wrote state")
	}
	res := cli.SubmitObligationWitness(cli.Env{Cwd: r.Root}, id, obligationRevision(t, r.Root, id), "w-bound",
		&mutation.ObligationReportCheck{TestRepository: name, TestRepositoryPath: app}, payload.Value())
	if enc := string(wire.Encode(res.Value())); strings.Contains(enc, wire.CodeStalePolicy) {
		t.Fatalf("bound path refused: %s", enc)
	}
}

// TestTOLV0066_AuditRequiresExactTestRepositoryRoot: receipt audit resolves
// the declared directory only when it is itself a worktree root; a linked
// worktree that loses its .git, so Git would resolve its directory to the
// enclosing repository at the same commit, makes the audit inconsistent.
func TestTOLV0066_AuditRequiresExactTestRepositoryRoot(t *testing.T) {
	encl := filepath.Join(t.TempDir(), "encl")
	fixture.Write(t, filepath.Join(encl, "e2e", "login.spec.ts"), []byte(obligationSpec))
	git(t, encl, "init", "-q")
	git(t, encl, "add", "e2e")
	git(t, encl, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "app spec")
	encl = mustEval(t, encl)
	app := filepath.Join(encl, "app")
	git(t, encl, "worktree", "add", "-q", "--detach", app)
	appHead := headOID(t, app)
	r, id, _ := obligationRepoWith(t, testRepoPolicy(map[string]string{"app": app}))
	seedObligations(t, r.Root, id)
	if x := qualifyVersion(t, r.Root, fixtureRun(t, qualifiedVersion, nil)); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("qualify: %s", x.stdout)
	}
	report := writeReport(t, t.TempDir(), app, qualifiedVersion, pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	if x := atm(t, r.Root, nil, witnessArgs(id, "w-app", obligationRevision(t, r.Root, id), appHead, "--from-playwright-report", report, "--test-repository", "app")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("audit before the worktree lost its .git")
	}
	if err := os.Remove(filepath.Join(app, ".git")); err != nil {
		t.Fatal(err)
	}
	if auditConsistent(t, r.Root) {
		t.Fatal("audit resolved the declared directory to the enclosing repository")
	}
}

// TestTOLV0126_ShippedVersionAdmitted: the shipped TOL-V0-009 list holds
// exactly 1.63.0, whose live fixture run is retained, so a 1.63.0 report
// credits in a store with no allow-list, and another 1.63 patch still refuses.
func TestTOLV0126_ShippedVersionAdmitted(t *testing.T) {
	if got := strings.Join(obligation.QualifiedVersions(), ","); got != "1.63.0" {
		t.Fatalf("shipped qualified versions %q, want 1.63.0", got)
	}
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	spec := pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}}
	other := atm(t, r.Root, nil, witnessArgs(id, "w-other", obligationRevision(t, r.Root, id), head, "--from-playwright-report",
		writeReport(t, t.TempDir(), r.Root, "1.63.1", spec))...)
	if other.res.Outcome == wire.OutcomeOK || !hasCode(other.res, wire.CodeUnsupportedVersion) {
		t.Fatalf("1.63.1 admitted: %s", other.stdout)
	}
	x := atm(t, r.Root, nil, witnessArgs(id, "w-shipped", obligationRevision(t, r.Root, id), head, "--from-playwright-report",
		writeReport(t, t.TempDir(), r.Root, "1.63.0", spec))...)
	if x.res.Outcome != wire.OutcomeOK || strList(x.res.Items[0], "credited") != "AC-1" || !auditConsistent(t, r.Root) {
		t.Fatalf("shipped 1.63.0 witness: %s", x.stdout)
	}
	if _, err := os.Lstat(obligation.AllowListPath(r.CommonDir)); !os.IsNotExist(err) {
		t.Fatalf("a shipped-version witness touched the allow-list: %v", err)
	}
}
