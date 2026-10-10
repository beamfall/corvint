package cli_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The TOL-V0-028..033 tests drive run-batch and qualify with a fake test
// command: a shell script that copies a committed Playwright-shaped report
// template, fake/<spec args>.json, to PLAYWRIGHT_JSON_OUTPUT_NAME with the
// checkout's real path as rootDir, and appends one line to a counter file
// per run; like Playwright, it exits 1 when a test failed. No real
// Playwright runs.

type rbResult struct {
	Status string   `json:"status"`
	Retry  int      `json:"retry"`
	Error  any      `json:"error,omitempty"`
	Steps  []pwStep `json:"steps"`
	Stdout []string `json:"stdout"`
}

type rbTest struct {
	ExpectedStatus string     `json:"expectedStatus"`
	ProjectName    string     `json:"projectName"`
	Results        []rbResult `json:"results"`
}

type rbSpec struct {
	Title string   `json:"title"`
	ID    string   `json:"id"`
	File  string   `json:"file"`
	Tests []rbTest `json:"tests"`
}

func rbPass(file, title string, steps ...pwStep) rbSpec {
	return rbSpec{Title: title, ID: title, File: file, Tests: []rbTest{{ExpectedStatus: "passed", ProjectName: "chromium",
		Results: []rbResult{{Status: "passed", Steps: append([]pwStep{}, steps...), Stdout: []string{}}}}}}
}

func rbFail(file, title, msg string, steps ...pwStep) rbSpec {
	return rbSpec{Title: title, ID: title, File: file, Tests: []rbTest{{ExpectedStatus: "passed", ProjectName: "chromium",
		Results: []rbResult{{Status: "failed", Error: map[string]string{"message": msg}, Steps: append([]pwStep{}, steps...), Stdout: []string{}}}}}}
}

// rbFlaky fails at retry 0 and passes at retry 1.
func rbFlaky(file, title string) rbSpec {
	return rbSpec{Title: title, ID: title, File: file, Tests: []rbTest{{ExpectedStatus: "passed", ProjectName: "chromium", Results: []rbResult{
		{Status: "failed", Error: map[string]string{"message": "Error: timing"}, Steps: []pwStep{}, Stdout: []string{}},
		{Status: "passed", Retry: 1, Steps: []pwStep{}, Stdout: []string{}}}}}}
}

// rbTemplate writes the report template the fake command copies for specs.
func rbTemplate(t *testing.T, root string, specs []string, tests ...rbSpec) {
	t.Helper()
	rbTemplateErrors(t, root, specs, []any{}, tests...)
}

// rbTemplateErrors is rbTemplate with top-level report errors.
func rbTemplateErrors(t *testing.T, root string, specs []string, errs []any, tests ...rbSpec) {
	t.Helper()
	doc := map[string]any{
		"config": map[string]any{"version": pwVersion, "rootDir": "@ROOT@"},
		"errors": errs,
		"stats":  map[string]any{"expected": len(tests)},
		"suites": []any{map[string]any{"title": "suite", "specs": tests, "suites": []any{}}},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	key := strings.NewReplacer(" ", "_", "/", "_").Replace(strings.Join(specs, " "))
	fixture.Write(t, filepath.Join(root, "fake", key+".json"), raw)
}

// rbTools writes the fake test command and returns its argv and the counter
// file it appends to.
func rbTools(t *testing.T) ([]string, string) {
	t.Helper()
	dir := t.TempDir()
	counter := filepath.Join(dir, "runs")
	script := "key=$(printf '%s' \"$*\" | tr ' /' '__')\n" +
		"echo run >> '" + counter + "'\n" +
		"sed \"s#@ROOT@#$(pwd -P)#g\" \"fake/$key.json\" > \"$PLAYWRIGHT_JSON_OUTPUT_NAME\" || exit 9\n" +
		"grep -q '\"status\":\"failed\"' \"fake/$key.json\" || exit 0\n" +
		"echo 'Error: some test failed' >&2\nexit 1\n"
	path := filepath.Join(dir, "fake-playwright.sh")
	fixture.Write(t, path, []byte(script))
	return []string{"sh", path}, counter
}

func rbRuns(t *testing.T, counter string) int {
	t.Helper()
	raw, err := os.ReadFile(counter)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(raw), "run\n")
}

// rbConfig writes a run configuration with the fake test command.
func rbConfig(t *testing.T, test []string, edit func(map[string]any)) string {
	t.Helper()
	c := map[string]any{"schema": "corvint-tasks-run-config/0", "testCommand": test, "runs": 1, "neighbours": "NONE",
		"env": []string{"PATH"}, "timeoutSeconds": 60}
	if edit != nil {
		edit(c)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	fixture.Write(t, path, raw)
	return path
}

func commitAll(t *testing.T, root, msg string) string {
	t.Helper()
	git(t, root, "add", "-A", "--", "e2e", "fake", "fixtures")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", msg)
	return headOID(t, root)
}

const batchSpec = "import { test } from '@playwright/test';\n" +
	"test('AC-1 login', async () => {});\n" +
	"test('AC-2 checkout', async () => {});\n" +
	"test('flow', async () => { await test.step('AC-3 first', async () => {}); await test.step('AC-4 second', async () => {}); });\n"

const batchSeed = `{"prefix":"AC","obligations":[` +
	`{"id":"AC-1","title":"login","core":true},` +
	`{"id":"AC-2","title":"checkout","core":true},` +
	`{"id":"AC-3","title":"flow first","core":false},` +
	`{"id":"AC-4","title":"flow second","core":false},` +
	`{"id":"AC-5","title":"never named","core":false}]}`

// obligationBatchRepo is an initialized store with a pool db, one committed spec file
// and its report template, and an OPEN ticket seeded with AC-1..AC-5.
func obligationBatchRepo(t *testing.T) (*fixture.Repo, string) {
	t.Helper()
	r := exclusionCLIRepo(t)
	spec := "e2e/batch.spec.ts"
	fixture.Write(t, filepath.Join(r.Root, spec), []byte(batchSpec))
	fixture.Write(t, filepath.Join(r.Root, "fixtures", "data.json"), []byte("{}\n"))
	rbTemplate(t, r.Root, []string{spec},
		rbPass(spec, "AC-1 login"),
		rbFail(spec, "AC-2 checkout", "Error: checkout total was 0"),
		rbFail(spec, "flow", "Error: first step broke the page", step("AC-3 first", true)))
	commitAll(t, r.Root, "spec")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %s", x.stdout)
	}
	t.Setenv("CORVINT_TASKS_ACTOR", "owner")
	id := planTicket(t, r.Root, "batch", "P2", `["e2e/"]`)
	x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--request-id", "seed-1", "--issued-at", "2026-10-10T12:00:00Z", "--payload", batchSeed)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("seed: %s", x.stdout)
	}
	return r, id
}

type batchSummaryDoc struct {
	Schema        string `json:"schema"`
	FixtureDigest string `json:"fixtureDigest"`
	Steps         []struct {
		Step  string `json:"step"`
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	} `json:"steps"`
	Witness struct {
		Outcome string `json:"outcome"`
		Written bool   `json:"written"`
	} `json:"witness"`
	Obligations []struct {
		ID, State, Cause, Error, Test string
	} `json:"obligations"`
}

func readSummary(t *testing.T, file string) batchSummaryDoc {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var s batchSummaryDoc
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func causeLine(s batchSummaryDoc) string {
	var out []string
	for _, o := range s.Obligations {
		out = append(out, o.ID+"="+o.Cause+"|"+o.Error)
	}
	return strings.Join(out, "; ")
}

func runBatchArgs(id, config, out, request string, extra ...string) []string {
	return append([]string{"run-batch", id, "--config", config, "--spec", "e2e/batch.spec.ts", "--out", out, "--request-id", request}, extra...)
}

// TOL-V0-029: one batch job, with no agent session, runs the named specs
// once, credits the ledger and writes a summary giving each obligation its
// cause: witnessed, failed with the error, not reached because an earlier
// test failed, and not named.
func TestTOLV0029_RunBatchCreditsAndWritesCauses(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id := obligationBatchRepo(t)
	test, counter := rbTools(t)
	out := filepath.Join(t.TempDir(), "out")
	x := atm(t, r.Root, nil, runBatchArgs(id, rbConfig(t, test, nil), out, "b1")...)
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool {
		t.Fatalf("run-batch: %s", x.stdout)
	}
	if got := entryStates(obligationsShow(t, r.Root, id)); got["AC-1"] != "WITNESSED" || got["AC-2"] != "OPEN" || got["AC-4"] != "OPEN" {
		t.Fatalf("ledger after the batch: %v", got)
	}
	s := readSummary(t, filepath.Join(out, "b1", "summary.json"))
	want := "AC-1=WITNESSED|; " +
		"AC-2=FAILED|Error: checkout total was 0; " +
		"AC-3=FAILED|expect failed; " +
		"AC-4=NOT_REACHED|Error: first step broke the page; " +
		"AC-5=NOT_NAMED|"
	if got := causeLine(s); got != want || s.Schema != "corvint-tasks-batch-summary/0" || !s.Witness.Written {
		t.Fatalf("causes\n got %s\nwant %s\n%+v", got, want, s)
	}
	if s.Obligations[3].Test != "e2e/batch.spec.ts > flow" || rbRuns(t, counter) != 1 {
		t.Fatalf("not-reached test %q, runs %d", s.Obligations[3].Test, rbRuns(t, counter))
	}
	// The same request id never overwrites earlier evidence.
	if again := atm(t, r.Root, nil, runBatchArgs(id, rbConfig(t, test, nil), out, "b1")...); !hasCode(again.res, wire.CodeRequestIDConflict) || rbRuns(t, counter) != 1 {
		t.Fatalf("reused request id: %s", again.stdout)
	}
}

// TOL-V0-029: a failed prep step leaves every in-scope obligation NOT_REACHED
// with the prep error, credits nothing and refuses GATE_FAILED.
func TestTOLV0029_RunBatchPrepFailureIsNotReached(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id := obligationBatchRepo(t)
	test, counter := rbTools(t)
	config := rbConfig(t, test, func(c map[string]any) {
		c["prepCommand"] = []string{"sh", "-c", "echo 'Error: database seed failed' >&2; exit 3"}
	})
	out := filepath.Join(t.TempDir(), "out")
	x := atm(t, r.Root, nil, runBatchArgs(id, config, out, "b1", "--ids", "AC-1,AC-2")...)
	if x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeGateFailed) || rbRuns(t, counter) != 0 {
		t.Fatalf("prep failure: %s", x.stdout)
	}
	s := readSummary(t, filepath.Join(out, "b1", "summary.json"))
	want := "AC-1=NOT_REACHED|prep: Error: database seed failed; AC-2=NOT_REACHED|prep: Error: database seed failed; " +
		"AC-3=OUT_OF_SCOPE|; AC-4=OUT_OF_SCOPE|; AC-5=OUT_OF_SCOPE|"
	if got := causeLine(s); got != want {
		t.Fatalf("causes\n got %s\nwant %s", got, want)
	}
	if got := entryStates(obligationsShow(t, r.Root, id)); got["AC-1"] != "OPEN" {
		t.Fatalf("prep failure credited: %v", got)
	}
}

// TOL-V0-030: the job runs only a clean checkout of --commit.
func TestTOLV0030_RunBatchRefusesAStaleOrDirtyCheckout(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id := obligationBatchRepo(t)
	test, counter := rbTools(t)
	config := rbConfig(t, test, nil)
	out := filepath.Join(t.TempDir(), "out")
	base := headOID(t, r.Root)
	fixture.Write(t, filepath.Join(r.Root, "fixtures", "data.json"), []byte("{\"x\":1}\n"))
	if x := atm(t, r.Root, nil, runBatchArgs(id, config, out, "b1")...); !hasCode(x.res, wire.CodeDirtyWorktree) {
		t.Fatalf("dirty: %s", x.stdout)
	}
	commitAll(t, r.Root, "fixture")
	if x := atm(t, r.Root, nil, runBatchArgs(id, config, out, "b2", "--commit", base)...); !hasCode(x.res, wire.CodeStaleTree) {
		t.Fatalf("stale: %s", x.stdout)
	}
	if rbRuns(t, counter) != 0 {
		t.Fatal("a refused job ran the test command")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a refused job created its results directory: %v", err)
	}
}

// TOL-V0-031: an obligation that failed twice with the same error at the
// same fixture evidence is refused before any lane is acquired; leaving it
// out with --ids or changing a fixture path admits the next job. With a
// pool, the job acquires a member and releases it after the capture.
func TestTOLV0031_RepeatFailureRefusedBeforeLaneAcquire(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id := obligationBatchRepo(t)
	test, counter := rbTools(t)
	config := rbConfig(t, test, func(c map[string]any) {
		c["fixturePaths"] = []string{"fixtures"}
		c["captureCommand"] = []string{"sh", "-c", "echo 'server: 500 on /api/cart' > \"$CORVINT_RUN_DIR/server.log\""}
	})
	claim := atm(t, r.Root, nil, "claim", id, "--holder", "builder", "--request-id", "claim-1", "--stage", "implement")
	if claim.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim %s", claim.stdout)
	}
	lane := []string{"--pool", "db", "--attempt", field(claim.res.Items[0], "attemptId").Str, "--generation", field(claim.res.Items[0], "generation").Str}
	out := filepath.Join(t.TempDir(), "out")
	for _, request := range []string{"b1", "b2"} {
		if x := atm(t, r.Root, nil, runBatchArgs(id, config, out, request)...); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%s: %s", request, x.stdout)
		}
	}
	x := atm(t, r.Root, nil, runBatchArgs(id, config, out, "b3", lane...)...)
	if !hasCode(x.res, wire.CodeLoopDetected) || !strings.Contains(strings.Join(x.res.Warnings, " "), "OBLIGATION_REPEAT_FAILURE: AC-2 (b1, b2: Error: checkout total was 0)") {
		t.Fatalf("repeat failure: %s", x.stdout)
	}
	if rbRuns(t, counter) != 2 {
		t.Fatalf("the refused job ran the test command: %d runs", rbRuns(t, counter))
	}
	for name, m := range poolStatusMembers(t, r.Root) {
		if field(m, "attemptId").Str != "" {
			t.Fatalf("the refused job acquired %s: %s", name, wire.Encode(m))
		}
	}
	if _, err := os.Stat(filepath.Join(out, "b3")); !os.IsNotExist(err) {
		t.Fatalf("the refused job wrote evidence: %v", err)
	}
	// Leaving the repeated failures out admits the job, on a pool member.
	x = atm(t, r.Root, nil, runBatchArgs(id, config, out, "b4", append(lane, "--ids", "AC-4,AC-5")...)...)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("subset: %s", x.stdout)
	}
	s := readSummary(t, filepath.Join(out, "b4", "summary.json"))
	var steps []string
	for _, st := range s.Steps {
		steps = append(steps, st.Step)
	}
	if got := strings.Join(steps, ","); got != "acquire,test,capture,release" {
		t.Fatalf("lane steps %s", got)
	}
	if raw, err := os.ReadFile(filepath.Join(out, "b4", "server.log")); err != nil || !strings.Contains(string(raw), "500 on /api/cart") {
		t.Fatalf("capture: %q %v", raw, err)
	}
	// New fixture evidence admits the full job again.
	fixture.Write(t, filepath.Join(r.Root, "fixtures", "data.json"), []byte("{\"cart\":[1]}\n"))
	commitAll(t, r.Root, "fixture")
	if x := atm(t, r.Root, nil, runBatchArgs(id, config, out, "b5")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("new fixture evidence: %s", x.stdout)
	}
}

// qualifyRepo commits a candidate spec and a neighbour spec in one directory
// at base, then changes the candidate spec; the neighbour report template
// differs between base and the candidate as the case says.
func qualifyRepo(t *testing.T, cand []rbSpec, nbBase, nbCand rbSpec, nbErrs ...any) (root, base string) {
	t.Helper()
	root = t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	c, nb := "e2e/a/cand.spec.ts", "e2e/a/nb.spec.ts"
	fixture.Write(t, filepath.Join(root, c), []byte("test('cand one', async () => {});\n"))
	fixture.Write(t, filepath.Join(root, nb), []byte("test('nb one', async () => {});\n"))
	fixture.Write(t, filepath.Join(root, "fixtures", "data.json"), []byte("{}\n"))
	rbTemplate(t, root, []string{nb}, nbBase)
	base = commitAll(t, root, "base")
	fixture.Write(t, filepath.Join(root, c), []byte("test('cand one', async () => { /* changed */ });\n"))
	rbTemplate(t, root, []string{c}, cand...)
	rbTemplateErrors(t, root, []string{nb}, append([]any{}, nbErrs...), nbCand)
	commitAll(t, root, "candidate")
	return root, base
}

type verdictDoc struct {
	Verdict    string   `json:"verdict"`
	Reasons    []string `json:"reasons"`
	Runs       []any    `json:"runs"`
	Neighbours []struct {
		Test, Base, Verdict string
	} `json:"neighbours"`
}

func qualifyRun(t *testing.T, root, config string) (run, verdictDoc, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out")
	x := atm(t, root, nil, "qualify", "--config", config, "--spec", "e2e/a/cand.spec.ts", "--base", "HEAD~1", "--out", out, "--request-id", "q1")
	raw, err := os.ReadFile(filepath.Join(out, "q1", "verdict.json"))
	if err != nil {
		t.Fatalf("verdict: %v: %s", err, x.stdout)
	}
	var v verdictDoc
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return x, v, filepath.Join(out, "q1")
}

// TOL-V0-032: the final is NOT_QUALIFIED for a retry above 0, a failed
// post-check or a new neighbour failure, and the verdict names the reason;
// a neighbour failure the base shares is PRE_EXISTING and does not
// disqualify; static checks run first.
func TestTOLV0032_QualifyVerdicts(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	c, nb := "e2e/a/cand.spec.ts", "e2e/a/nb.spec.ts"
	pass, nbPass, nbFail := []rbSpec{rbPass(c, "cand one")}, rbPass(nb, "nb one"), rbFail(nb, "nb one", "Error: nb broke")
	cases := []struct {
		name          string
		cand          []rbSpec
		nbBase, nbNew rbSpec
		edit          func(map[string]any)
		verdict       string
		reason        string
		runs          int
		neighbour     string
		nbErrs        []any
	}{
		{"qualified", pass, nbPass, nbPass, nil, "QUALIFIED", "", 3, "", nil},
		{"retry", []rbSpec{rbFlaky(c, "cand one")}, nbPass, nbPass, nil, "NOT_QUALIFIED", "RETRY: run 1 retried e2e/a/cand.spec.ts > cand one [chromium]", 1, "", nil},
		{"candidate failure", []rbSpec{rbFail(c, "cand one", "Error: nope")}, nbPass, nbPass, nil, "NOT_QUALIFIED", "CANDIDATE_FAILURE: run 1 failed e2e/a/cand.spec.ts > cand one [chromium] (Error: nope)", 1, "", nil},
		{"post-check", pass, nbPass, nbPass, func(m map[string]any) {
			m["postCheck"] = []string{"sh", "-c", "echo 'contract: GET /api/cart returned 500' >&2; exit 1"}
		}, "NOT_QUALIFIED", "POST_CHECK_FAILED: contract: GET /api/cart returned 500", 2, "", nil},
		{"new neighbour failure", pass, nbPass, nbFail, nil, "NOT_QUALIFIED", "NEW_NEIGHBOUR_FAILURE: e2e/a/nb.spec.ts > nb one [chromium] (base PASSED)", 4, "NEW_NEIGHBOUR_FAILURE", nil},
		{"pre-existing neighbour failure", pass, nbFail, nbFail, nil, "QUALIFIED", "", 4, "PRE_EXISTING", nil},
		// A retried neighbour disqualifies even when its failure is
		// pre-existing; top-level report errors disqualify with or without a
		// failing test (Codex round 1, finding 2).
		{"retried pre-existing neighbour", pass, nbFail, rbFlaky(nb, "nb one"), nil, "NOT_QUALIFIED", "RETRY: neighbour run retried e2e/a/nb.spec.ts > nb one [chromium]", 4, "PRE_EXISTING", nil},
		{"neighbour errors with a pre-existing failure", pass, nbFail, nbFail, nil, "NOT_QUALIFIED", "NEW_NEIGHBOUR_FAILURE: the neighbour run reported 1 top-level errors", 4, "PRE_EXISTING", []any{map[string]string{"message": "Error: e2e/a/other.spec.ts failed to load"}}},
		{"neighbour errors without a failure", pass, nbPass, nbPass, nil, "NOT_QUALIFIED", "NEW_NEIGHBOUR_FAILURE: the neighbour run reported 1 top-level errors", 3, "", []any{map[string]string{"message": "Error: e2e/a/other.spec.ts failed to load"}}},
		{"static check", pass, nbPass, nbPass, func(m map[string]any) {
			m["staticChecks"] = [][]string{{"sh", "-c", "echo 'lint: unused import' >&2; exit 2"}}
		}, "NOT_QUALIFIED", "STATIC_CHECK_FAILED: static/0: lint: unused import", 0, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := qualifyRepo(t, tc.cand, tc.nbBase, tc.nbNew, tc.nbErrs...)
			test, counter := rbTools(t)
			config := rbConfig(t, test, func(m map[string]any) {
				m["runs"], m["neighbours"] = 2, "CHANGED_DIRECTORIES"
				if tc.edit != nil {
					tc.edit(m)
				}
			})
			x, v, dir := qualifyRun(t, root, config)
			if v.Verdict != tc.verdict || field(x.res.Items[0], "verdict").Str != tc.verdict || rbRuns(t, counter) != tc.runs {
				t.Fatalf("verdict %s, runs %d: %s", v.Verdict, rbRuns(t, counter), x.stdout)
			}
			if tc.verdict == "QUALIFIED" {
				if x.res.Outcome != wire.OutcomeOK || len(v.Reasons) != 0 {
					t.Fatalf("qualified: %s", x.stdout)
				}
			} else if x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeGateFailed) || len(v.Reasons) != 1 || v.Reasons[0] != tc.reason ||
				!strings.Contains(strings.Join(x.res.Warnings, " "), "QUALIFY_NOT_QUALIFIED: "+tc.reason) {
				t.Fatalf("reasons %q: %s", v.Reasons, x.stdout)
			}
			if tc.neighbour != "" && (len(v.Neighbours) != 1 || v.Neighbours[0].Verdict != tc.neighbour) {
				t.Fatalf("neighbours %+v", v.Neighbours)
			}
			if wts := strings.Count(gitOut(t, root, "worktree", "list", "--porcelain"), "worktree "); wts != 1 {
				t.Fatalf("the base worktree was not removed: %d worktrees", wts)
			}
			manifest, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
			if err != nil || !strings.Contains(string(manifest), "\"verdict.json\"") || (tc.runs > 0 && !strings.Contains(string(manifest), "runs/1/report.json")) {
				t.Fatalf("manifest %s %v", manifest, err)
			}
		})
	}
}

// TOL-V0-028: the configuration file is closed and bounded.
func TestTOLV0028_RunConfigIsClosed(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	fixture.Write(t, filepath.Join(root, "e2e", "a.spec.ts"), []byte("test('a', async () => {});\n"))
	git(t, root, "add", "e2e")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "a")
	for name, edit := range map[string]func(map[string]any){
		"unknown field":   func(m map[string]any) { m["retries"] = 2 },
		"runs above 20":   func(m map[string]any) { m["runs"] = 21 },
		"bad neighbours":  func(m map[string]any) { m["neighbours"] = "ALL" },
		"no test command": func(m map[string]any) { delete(m, "testCommand") },
		"absolute path":   func(m map[string]any) { m["fixturePaths"] = []string{"/etc"} },
	} {
		config := rbConfig(t, []string{"true"}, edit)
		x := atm(t, root, nil, "qualify", "--config", config, "--spec", "e2e", "--base", "HEAD", "--out", t.TempDir(), "--request-id", "q1")
		if !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("%s: %s", name, x.stdout)
		}
	}
}

// markerInterrupt replaces the job context with one that ends when marker
// appears, as SIGINT would, and returns the restore function.
func markerInterrupt(marker string) func() {
	return cli.SetJobInterruptContext(func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			for ctx.Err() == nil {
				if _, err := os.Stat(marker); err == nil {
					cancel()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
		return ctx, cancel
	})
}

// TOL-V0-033 (Codex round 1, finding 1): an interrupt seen after the report
// is read but before the witness mutation is submitted credits nothing and
// refuses with LANE_FAILED.
func TestTOLV0033_InterruptBeforeWitnessSubmitCreditsNothing(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id := obligationBatchRepo(t)
	test, _ := rbTools(t)
	var cancel context.CancelFunc
	defer cli.SetJobInterruptContext(func() (context.Context, context.CancelFunc) {
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		return ctx, cancel
	})()
	defer cli.SetWitnessEvidenceReadHook(func() { cancel() })()
	out := filepath.Join(t.TempDir(), "out")
	x := atm(t, r.Root, nil, runBatchArgs(id, rbConfig(t, test, nil), out, "b1")...)
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeGateFailed) || field(x.res.Items[0], "written").Bool ||
		!strings.Contains(strings.Join(x.res.Warnings, " "), "LANE_FAILED: interrupted before the witness was submitted") {
		t.Fatalf("interrupted witness: %s", x.stdout)
	}
	if got := entryStates(obligationsShow(t, r.Root, id)); got["AC-1"] != "OPEN" {
		t.Fatalf("an interrupted job credited: %v", got)
	}
	if s := readSummary(t, filepath.Join(out, "b1", "summary.json")); s.Obligations[0].Cause != "UNCREDITED" || s.Witness.Written {
		t.Fatalf("summary: %s", causeLine(s))
	}
}

// TOL-V0-033 (Codex round 1, finding 4): an interrupt during the tests still
// runs the capture, on a fresh context, so server-side errors are kept.
func TestTOLV0033_CaptureRunsAfterAnInterruptDuringTests(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id := obligationBatchRepo(t)
	out := filepath.Join(t.TempDir(), "out")
	defer markerInterrupt(filepath.Join(out, "b1", "interrupt.now"))()
	config := rbConfig(t, []string{"sh", "-c", "touch \"$CORVINT_RUN_DIR/interrupt.now\"; exec sleep 30"}, func(c map[string]any) {
		c["captureCommand"] = []string{"sh", "-c", "sleep 0.3; echo 'server: 500 on /api/cart' > \"$CORVINT_RUN_DIR/server.log\""}
	})
	x := atm(t, r.Root, nil, runBatchArgs(id, config, out, "b1")...)
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeGateFailed) || !strings.Contains(strings.Join(x.res.Warnings, " "), "LANE_FAILED: interrupted") {
		t.Fatalf("interrupted job: %s", x.stdout)
	}
	if raw, err := os.ReadFile(filepath.Join(out, "b1", "server.log")); err != nil || !strings.Contains(string(raw), "500 on /api/cart") {
		t.Fatalf("capture after the interrupt: %q %v", raw, err)
	}
	s := readSummary(t, filepath.Join(out, "b1", "summary.json"))
	if s.Obligations[0].Cause != "UNCREDITED" || len(s.Steps) != 2 || s.Steps[1].Step != "capture" || !s.Steps[1].OK {
		t.Fatalf("summary: %s %+v", causeLine(s), s.Steps)
	}
}

// laneRepo is obligationBatchRepo with a claimed attempt and the lane flags
// for pool db, plus a passing spec e2e/ok.spec.ts.
func laneRepo(t *testing.T) (*fixture.Repo, string, []string) {
	t.Helper()
	r, id := obligationBatchRepo(t)
	fixture.Write(t, filepath.Join(r.Root, "e2e", "ok.spec.ts"), []byte("test('AC-1 login', async () => {});\n"))
	rbTemplate(t, r.Root, []string{"e2e/ok.spec.ts"}, rbPass("e2e/ok.spec.ts", "AC-1 login"))
	commitAll(t, r.Root, "ok spec")
	claim := atm(t, r.Root, nil, "claim", id, "--holder", "builder", "--request-id", "claim-1", "--stage", "implement")
	if claim.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim %s", claim.stdout)
	}
	return r, id, []string{"--pool", "db", "--attempt", field(claim.res.Items[0], "attemptId").Str, "--generation", field(claim.res.Items[0], "generation").Str}
}

// TOL-V0-030 (Codex round 1, finding 3): a refused lane release fails the
// job: run-batch witnesses nothing and returns the release refusal, and
// qualify is NOT_QUALIFIED with LANE_FAILED.
func TestTOLV0030_RefusedReleaseFailsTheJob(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	defer cli.FailJobPoolRelease(wire.CodeResourceCollision)()
	r, id, lane := laneRepo(t)
	test, _ := rbTools(t)
	config := rbConfig(t, test, nil)
	out := filepath.Join(t.TempDir(), "out")
	x := atm(t, r.Root, nil, append([]string{"run-batch", id, "--config", config, "--spec", "e2e/ok.spec.ts", "--out", out, "--request-id", "b1"}, lane...)...)
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeResourceCollision) || field(x.res.Items[0], "written").Bool ||
		!strings.Contains(strings.Join(x.res.Warnings, " "), "LANE_FAILED: pool release was refused") {
		t.Fatalf("run-batch with a refused release: %s", x.stdout)
	}
	if got := entryStates(obligationsShow(t, r.Root, id)); got["AC-1"] != "OPEN" {
		t.Fatalf("a job whose release was refused credited: %v", got)
	}
	// The refused release left that allocation held; qualify uses a fresh lane.
	r2, _, lane2 := laneRepo(t)
	q := atm(t, r2.Root, nil, append([]string{"qualify", "--config", config, "--spec", "e2e/ok.spec.ts", "--base", "HEAD", "--out", out, "--request-id", "q1"}, lane2...)...)
	v := field(q.res.Items[0], "verdict").Str
	reasons := field(q.res.Items[0], "reasons").Arr
	if v != "NOT_QUALIFIED" || len(reasons) != 1 || !strings.HasPrefix(reasons[0].Str, "LANE_FAILED: release: ") {
		t.Fatalf("qualify with a refused release: %s", q.stdout)
	}
}

// TOL-V0-030 (Codex round 1, finding 6): a refused acquire is the job's
// result code, not GATE_FAILED.
func TestTOLV0030_AcquireRefusalCodeIsTheJobs(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, lane := laneRepo(t)
	// The attempt already holds a member, so the job's acquire is refused.
	held := atm(t, r.Root, nil, "pool", "acquire", lane[2], lane[3], lane[4], lane[5], lane[0], lane[1], "--request-id", "held-1")
	if held.res.Outcome != wire.OutcomeOK {
		t.Fatalf("pool acquire: %s", held.stdout)
	}
	again := atm(t, r.Root, nil, "pool", "acquire", lane[2], lane[3], lane[4], lane[5], lane[0], lane[1], "--request-id", "held-2")
	if again.res.Outcome == wire.OutcomeOK || len(again.res.Codes) == 0 {
		t.Fatalf("second acquire: %s", again.stdout)
	}
	test, counter := rbTools(t)
	out := filepath.Join(t.TempDir(), "out")
	x := atm(t, r.Root, nil, append([]string{"run-batch", id, "--config", rbConfig(t, test, nil), "--spec", "e2e/ok.spec.ts", "--out", out, "--request-id", "b1"}, lane...)...)
	if x.res.Outcome == wire.OutcomeOK || hasCode(x.res, wire.CodeGateFailed) || !hasCode(x.res, again.res.Codes[0]) || rbRuns(t, counter) != 0 {
		t.Fatalf("run-batch with a refused acquire (want %v): %s", again.res.Codes, x.stdout)
	}
}

// TOL-V0-031 (Codex round 1, finding 5): a fixture path "." digests the
// commit's root tree, so a change anywhere clears the repeat refusal.
func TestTOLV0031_RootFixturePathIsDigested(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id := obligationBatchRepo(t)
	test, _ := rbTools(t)
	config := rbConfig(t, test, func(c map[string]any) { c["fixturePaths"] = []string{"."} })
	out := filepath.Join(t.TempDir(), "out")
	for _, request := range []string{"b1", "b2"} {
		if x := atm(t, r.Root, nil, runBatchArgs(id, config, out, request)...); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%s: %s", request, x.stdout)
		}
	}
	if x := atm(t, r.Root, nil, runBatchArgs(id, config, out, "b3")...); !hasCode(x.res, wire.CodeLoopDetected) {
		t.Fatalf("repeat failure: %s", x.stdout)
	}
	fixture.Write(t, filepath.Join(r.Root, "README.md"), []byte("root fixture change\n"))
	git(t, r.Root, "add", "README.md")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "root change")
	if x := atm(t, r.Root, nil, runBatchArgs(id, config, out, "b4")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("a root-level fixture change did not clear the refusal: %s", x.stdout)
	}
}

// TOL-V0-033 (Codex round 1, finding 7): the manifest streams each file's
// hash, so an artifact far larger than the report bound is listed without
// being read into memory, and the oversized report is still refused.
func TestTOLV0033_ManifestStreamsAnOversizedArtifact(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	c := "e2e/a/cand.spec.ts"
	root, _ := qualifyRepo(t, []rbSpec{rbPass(c, "cand one")}, rbPass("e2e/a/nb.spec.ts", "nb one"), rbPass("e2e/a/nb.spec.ts", "nb one"))
	const size = 512 << 20
	config := rbConfig(t, []string{"sh", "-c", "dd if=/dev/zero of=\"$PLAYWRIGHT_JSON_OUTPUT_NAME\" bs=1048576 seek=512 count=0 2>/dev/null"}, nil)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, v, dir := qualifyRun(t, root, config)
	runtime.ReadMemStats(&after)
	if v.Verdict != "NOT_QUALIFIED" || len(v.Reasons) != 1 || !strings.HasPrefix(v.Reasons[0], "CANDIDATE_FAILURE: run 1 produced no admissible report") {
		t.Fatalf("oversized report: %+v", v)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil || !strings.Contains(string(raw), "\"path\": \"runs/1/report.json\"") || !strings.Contains(string(raw), "\"size\": 536870912") {
		t.Fatalf("manifest %s %v", raw, err)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew >= size*3/4 {
		t.Fatalf("qualify allocated %d bytes for a %d-byte artifact; the manifest must stream it", grew, size)
	}
}
