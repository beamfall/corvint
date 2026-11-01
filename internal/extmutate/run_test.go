package extmutate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
)

const (
	userBase = "class User\n  def name\n    \"x\"\n  end\nend\n"
	userHead = "class User\n  def adult?(age)\n    age >= 18\n  end\n  def name\n    \"x\"\n  end\n  def admin?(role)\n    role == \"admin\"\n  end\nend\n"
	priceTS  = "export const free = (p: number): boolean => p === 0 && true;\n"
)

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// fixture is a base..head change to one Ruby and one TS file plus a spec file, and a clean
// worktree of head to mutate.
type fixture struct{ root, base, head, app, scratch string }

func newFixture(t *testing.T) fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root, scratch := t.TempDir(), t.TempDir()
	gitRun(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "app/models/user.rb"), userBase, 0o644)
	writeFile(t, filepath.Join(root, "README.md"), "fixture\n", 0o644)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")
	base := gitRun(t, root, "rev-parse", "HEAD")
	writeFile(t, filepath.Join(root, "app/models/user.rb"), userHead, 0o644)
	writeFile(t, filepath.Join(root, "app/lib/price.ts"), priceTS, 0o644)
	writeFile(t, filepath.Join(root, "spec/models/user_spec.rb"), "it { expect(true).to be true }\n", 0o644)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "head")
	head := gitRun(t, root, "rev-parse", "HEAD")
	app := filepath.Join(scratch, "app")
	gitRun(t, root, "worktree", "add", "-q", "--detach", app, head)
	return fixture{root: root, base: base, head: head, app: app, scratch: scratch}
}

func (f fixture) options(test ...string) Options {
	return Options{Root: f.root, Base: f.base, Head: f.head, AppDir: f.app, TestCommand: test, RunTimeout: 30 * time.Second, RebuildTimeout: 30 * time.Second}
}

func record(t *testing.T, doc Document, path string, start int, test string) Record {
	t.Helper()
	for _, r := range doc.Records {
		if r.Path == path && r.NewStart == start && r.Test == test {
			return r
		}
	}
	t.Fatalf("no record for %s:%d %s in %+v", path, start, test, doc.Records)
	return Record{}
}

func assertRestored(t *testing.T, f fixture, doc Document) {
	t.Helper()
	if !doc.AppDirRestored || gitRun(t, f.app, "status", "--porcelain") != "" {
		t.Fatalf("app dir not restored: %v %q", doc.AppDirRestored, gitRun(t, f.app, "status", "--porcelain"))
	}
}

// CCE-V0-170, CCE-V0-172..174: changed application hunks are mutated in a clean head worktree,
// rebuilt and tested once per mutant, restored, and folded per (hunk, test) with every run's
// receipt digest pinned beside the app and test revisions.
func TestCCEV0172ExitVerdictKillsSurvivesAndRestores(t *testing.T) {
	f := newFixture(t)
	rebuilds := filepath.Join(f.scratch, "rebuilds.log")
	opts := f.options("sh", "-c", `grep -q "age >= 18" app/models/user.rb && grep -q "p === 0" app/lib/price.ts`)
	opts.RebuildCommand = []string{"sh", "-c", `echo "$CORVINT_EXTMUTATE_RUN" >> ` + rebuilds}
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	assertRestored(t, f, doc)
	if doc.Schema != Schema || doc.OperatorSet != OperatorSet || doc.AppRevision != f.head || doc.BaseRevision != f.base || doc.Tests.Revision != f.head || !doc.Tests.Unchanged || doc.Verdict != VerdictExit || doc.InfrastructureDistinguished {
		t.Fatalf("pins: %+v", doc)
	}
	if len(doc.Hunks) != 3 || doc.Hunks[0].Path != "app/lib/price.ts" || doc.MutantsPlanned != 5 || len(doc.Mutants) != 5 || doc.Outcome != "complete" {
		t.Fatalf("plan: hunks %+v mutants %d outcome %s", doc.Hunks, len(doc.Mutants), doc.Outcome)
	}
	for _, m := range append([]Mutant{}, doc.Mutants...) {
		if m.Run == nil || len(m.Run.ReceiptDigest) != 64 || m.Run.Rebuild != "passed" {
			t.Fatalf("mutant run not pinned: %+v", m)
		}
	}
	if r := record(t, doc, "app/models/user.rb", 2, "command"); r.State != Killed || r.Killed != 1 || r.Mutants != 1 {
		t.Fatalf("tested hunk: %+v", r)
	}
	if r := record(t, doc, "app/models/user.rb", 8, "command"); r.State != Survived || r.Survived != 1 {
		t.Fatalf("untested hunk: %+v", r)
	}
	if r := record(t, doc, "app/lib/price.ts", 1, "command"); r.State != Killed || r.Killed != 1 || r.Survived != 2 {
		t.Fatalf("ts hunk: %+v", r)
	}
	log, _ := os.ReadFile(rebuilds)
	if got := strings.Fields(string(log)); strings.Join(got, ",") != "baseline,m1,m2,m3,m4,m5,restore" || doc.RestoreRebuild != "passed" {
		t.Fatalf("rebuild hook order %v %s", got, doc.RestoreRebuild)
	}
	var round Document
	if err := json.Unmarshal(doc.Encode(), &round); err != nil || round.Records[0].Test != "command" {
		t.Fatalf("document does not round-trip: %v", err)
	}
}

// CCE-V0-172: the count and wall-time bounds leave named not-run records, never silence.
func TestCCEV0172BoundsAreVisible(t *testing.T) {
	f := newFixture(t)
	opts := f.options("true")
	opts.MaxMutants = 1
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	assertRestored(t, f, doc)
	if doc.MutantsPlanned != 5 || doc.MutantsOmitted != 4 || doc.Outcome != "partial" {
		t.Fatalf("bound: %+v", doc)
	}
	if r := record(t, doc, "app/models/user.rb", 8, "command"); r.State != NotRun || r.Reason != "mutant-bound" {
		t.Fatalf("bounded hunk: %+v", r)
	}
	// The TS hunk has three sites and only its first ran and survived: still not survived.
	if r := record(t, doc, "app/lib/price.ts", 1, "command"); r.State != NotRun || r.Reason != "mutant-bound" || r.Survived != 1 {
		t.Fatalf("partly bounded hunk: %+v", r)
	}
	// The baseline finishes; the first mutant run outlives the wall time and is cut at it, and
	// the later mutants never start.
	opts = f.options("sh", "-c", `[ "$CORVINT_EXTMUTATE_RUN" = baseline ] || sleep 20`)
	opts.WallTime = 2 * time.Second
	doc, err = Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if m := doc.Mutants[0]; m.Run == nil || m.Run.Infrastructure != WallTimeExhausted || m.Results[0].Reason != "infrastructure: "+WallTimeExhausted {
		t.Fatalf("cut mutant: %+v", m)
	}
	if r := record(t, doc, "app/models/user.rb", 2, "command"); r.State != NotRun || r.Reason != WallTimeExhausted {
		t.Fatalf("wall time: %+v", r)
	}
}

// CCE-V0-173: a mutant whose rebuild fails is not-run, and a baseline the suite cannot run marks
// every mutant not-run without touching the app.
func TestCCEV0173RebuildAndBaselineFailuresAreNotRun(t *testing.T) {
	f := newFixture(t)
	opts := f.options("false")
	opts.RebuildCommand = []string{"sh", "-c", `! grep -q "age > 18" app/models/user.rb`}
	opts.TestCommand = []string{"sh", "-c", "exit 1"}
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if r := record(t, doc, "app/models/user.rb", 2, "command"); r.State != NotRun || r.Reason != "baseline-not-passing" {
		t.Fatalf("baseline failing test: %+v", r)
	}
	opts.TestCommand = []string{"true"}
	doc, err = Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	assertRestored(t, f, doc)
	if r := record(t, doc, "app/models/user.rb", 2, "command"); r.State != NotRun || r.Reason != "rebuild-failed" {
		t.Fatalf("rebuild failure: %+v", r)
	}
	opts.RebuildCommand = nil
	opts.TestCommand = []string{filepath.Join(f.scratch, "no-such-suite")}
	doc, err = Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Outcome != "baseline-infrastructure" || doc.Baseline.Infrastructure != "command-start-failed" || len(doc.Records) != 0 {
		t.Fatalf("baseline infrastructure: %+v", doc)
	}
	for _, m := range doc.Mutants {
		if m.Run != nil {
			t.Fatal("a mutant ran after baseline infrastructure")
		}
	}
}

func jsDocument(t *testing.T, tests ...jstestprovider.TestOutcome) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{"receipt": jstestprovider.Receipt{Kind: "e2e", Tests: tests}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func preflightFailureDocument(t *testing.T) []byte {
	t.Helper()
	failure := &jstestprovider.InfrastructureFailure{Reason: jstestprovider.ReasonPreflightFailed, Detail: "api: http-unreachable", Phase: jstestprovider.InfrastructurePhasePreflight, Probe: "api", Artefact: "http://127.0.0.1:3000/healthz"}
	r := jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", Tests: []jstestprovider.TestOutcome{},
		External: &jstestprovider.ExternalLifecycle{ReadyURL: "http://127.0.0.1:3000/", DeclaredAppIdentity: "fixture", Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ConfigOverride: "config"},
		Preflight: &jstestprovider.PreflightReceipt{Schema: jstestprovider.PreflightSchema, DeclarationFile: "/p.json", DeclarationDigest: strings.Repeat("d", 64), BaseURL: "http://127.0.0.1:3000", Outcome: jstestprovider.PreflightFailed,
			Probes: []jstestprovider.PreflightProbeResult{{Name: "api", Kind: jstestprovider.PreflightHTTP, State: jstestprovider.PreflightFailed, Reason: "http-unreachable", Artefact: "http://127.0.0.1:3000/healthz"}}},
		Infrastructure: failure}
	data, err := jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// CCE-V0-173: a JS provider document decides per test; a provider infrastructure failure, a
// preflight failure included, is not-run for every test and never a kill.
func TestCCEV0173JSProviderVerdictSeparatesInfrastructure(t *testing.T) {
	f := newFixture(t)
	docs := filepath.Join(f.scratch, "docs")
	writeFile(t, filepath.Join(docs, "pass.json"), string(jsDocument(t, jstestprovider.TestOutcome{Name: "login", FullName: "login", State: jstestprovider.StatePassed}, jstestprovider.TestOutcome{Name: "checkout", FullName: "checkout", State: jstestprovider.StatePassed})), 0o644)
	writeFile(t, filepath.Join(docs, "login-fails.json"), string(jsDocument(t, jstestprovider.TestOutcome{Name: "login", FullName: "login", State: jstestprovider.StateFailed}, jstestprovider.TestOutcome{Name: "checkout", FullName: "checkout", State: jstestprovider.StatePassed})), 0o644)
	writeFile(t, filepath.Join(docs, "preflight.json"), string(preflightFailureDocument(t)), 0o644)
	script := `if ! grep -q "p === 0" app/lib/price.ts; then cat ` + docs + `/preflight.json; exit 1; fi
if ! grep -q "age >= 18" app/models/user.rb; then cat ` + docs + `/login-fails.json; exit 1; fi
cat ` + docs + `/pass.json`
	opts := f.options("sh", "-c", script)
	opts.Verdict = VerdictProvider
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	assertRestored(t, f, doc)
	if !doc.InfrastructureDistinguished || doc.Outcome != "partial" {
		t.Fatalf("doc: %+v", doc)
	}
	if r := record(t, doc, "app/models/user.rb", 2, "login"); r.State != Killed {
		t.Fatalf("login: %+v", r)
	}
	if r := record(t, doc, "app/models/user.rb", 2, "checkout"); r.State != Survived {
		t.Fatalf("checkout: %+v", r)
	}
	// m1 is the strict-eq mutant of price.ts: the preflight fails, so nothing is killed there.
	if m := doc.Mutants[0]; m.Operator != "strict-eq-negate" || m.Results[0].Result != NotRun || m.Results[0].Reason != "infrastructure: preflight: api" || m.Run.Infrastructure != "preflight: api" {
		t.Fatalf("preflight mutant: %+v", m)
	}
	if r := record(t, doc, "app/lib/price.ts", 1, "login"); r.State != NotRun || r.Killed != 0 || r.NotRun != 1 || r.Survived != 2 {
		t.Fatalf("ts record: %+v", r)
	}
}

// CCE-V0-173: a Ruby provider document decides per example.
func TestCCEV0173RubyProviderVerdict(t *testing.T) {
	f := newFixture(t)
	repository := &jstestprovider.ApplicationRepositoryIdentity{RootCommit: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40), DirtyState: "clean"}
	helpers := []rubytestprovider.BoundFile{{Path: "spec/spec_helper.rb", Blob: rubytestprovider.BlobID("sha1", []byte("RSpec.configure { }\n"))}}
	document := func(adult string) string {
		d := rubytestprovider.Document{Profile: rubytestprovider.Profile, Runner: "rspec", ProjectDirectory: ".", ObjectFormat: "sha1", RepositoryBefore: repository, RepositoryAfter: repository,
			HelperBasis: rubytestprovider.HelperBasis, Helpers: helpers, HelperSet: rubytestprovider.HelperSetDigest(helpers), JUnit: rubytestprovider.JUnit{State: "NOT_REQUESTED"}}
		for i, status := range []string{adult, "passed"} {
			d.Examples = append(d.Examples, rubytestprovider.Example{ID: "spec/user_spec.rb:" + string(rune('2'+i)), File: "spec/user_spec.rb", Line: 2 + i, Description: "user " + status,
				Status: status, SpecBlob: rubytestprovider.BlobID("sha1", []byte("spec\n")), Helpers: d.HelperSet})
		}
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	docs := filepath.Join(f.scratch, "ruby")
	writeFile(t, filepath.Join(docs, "pass.json"), document("passed"), 0o644)
	writeFile(t, filepath.Join(docs, "fail.json"), document("failed"), 0o644)
	opts := f.options("sh", "-c", `if grep -q "age >= 18" app/models/user.rb; then cat `+docs+`/pass.json; else cat `+docs+`/fail.json; fi`)
	opts.Verdict = VerdictProvider
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	assertRestored(t, f, doc)
	if r := record(t, doc, "app/models/user.rb", 2, "spec/user_spec.rb:2"); r.State != Killed {
		t.Fatalf("adult example: %+v", r)
	}
	if r := record(t, doc, "app/models/user.rb", 2, "spec/user_spec.rb:3"); r.State != Survived {
		t.Fatalf("other example: %+v", r)
	}
	if r := record(t, doc, "app/models/user.rb", 8, "spec/user_spec.rb:2"); r.State != Survived {
		t.Fatalf("admin hunk: %+v", r)
	}
}

// CCE-V0-174: a test revision that changes during the run downgrades every result to not-run.
func TestCCEV0174TestRevisionDriftDowngrades(t *testing.T) {
	f := newFixture(t)
	tests := filepath.Join(f.scratch, "suite")
	if err := os.MkdirAll(tests, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tests, "init", "-q")
	writeFile(t, filepath.Join(tests, "suite.txt"), "suite\n", 0o644)
	gitRun(t, tests, "add", "-A")
	gitRun(t, tests, "commit", "-q", "-m", "suite")
	opts := f.options("sh", "-c", `[ "$CORVINT_EXTMUTATE_RUN" = baseline ] || echo drift >> suite.txt; grep -q "age >= 18" `+filepath.Join(f.app, "app/models/user.rb"))
	opts.TestsDir = tests
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Tests.Unchanged || doc.Tests.Revision != gitRun(t, tests, "rev-parse", "HEAD") || doc.Outcome != "partial" {
		t.Fatalf("drift not recorded: %+v", doc.Tests)
	}
	if r := record(t, doc, "app/models/user.rb", 2, "command"); r.State != NotRun || r.Reason != "test-revision-drift" {
		t.Fatalf("drift record: %+v", r)
	}
}

// CCE-V0-174: a tests directory already dirty at start that changes again keeps its HEAD and
// dirty flag, so only the digest of its tracked changes shows the drift.
func TestCCEV0174DirtyTestDriftDowngrades(t *testing.T) {
	f := newFixture(t)
	tests := filepath.Join(f.scratch, "suite")
	if err := os.MkdirAll(tests, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tests, "init", "-q")
	writeFile(t, filepath.Join(tests, "suite.txt"), "suite\n", 0o644)
	gitRun(t, tests, "add", "-A")
	gitRun(t, tests, "commit", "-q", "-m", "suite")
	writeFile(t, filepath.Join(tests, "suite.txt"), "suite\ndirty\n", 0o644)
	opts := f.options("sh", "-c", `[ "$CORVINT_EXTMUTATE_RUN" = baseline ] || echo drift >> suite.txt; grep -q "age >= 18" `+filepath.Join(f.app, "app/models/user.rb"))
	opts.TestsDir = tests
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Tests.Clean || doc.Tests.Unchanged || len(doc.Tests.ChangesDigest) != 64 || doc.Outcome != "partial" {
		t.Fatalf("dirty drift not recorded: %+v outcome %s", doc.Tests, doc.Outcome)
	}
	if r := record(t, doc, "app/models/user.rb", 2, "command"); r.State != NotRun || r.Reason != "test-revision-drift" {
		t.Fatalf("drift record: %+v", r)
	}
}

// CCE-V0-684 (V1-2882): editing an untracked test file between the baseline and the mutant runs
// leaves HEAD, the tracked-clean flag and the tracked diff unchanged, so only the untracked
// digest shows the drift.
func TestCCEV0684UntrackedTestDriftDowngrades(t *testing.T) {
	f := newFixture(t)
	tests := filepath.Join(f.scratch, "suite")
	if err := os.MkdirAll(tests, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tests, "init", "-q")
	writeFile(t, filepath.Join(tests, "suite.txt"), "suite\n", 0o644)
	gitRun(t, tests, "add", "-A")
	gitRun(t, tests, "commit", "-q", "-m", "suite")
	writeFile(t, filepath.Join(tests, "new_spec.rb"), "it { }\n", 0o644)
	opts := f.options("sh", "-c", `[ "$CORVINT_EXTMUTATE_RUN" = baseline ] || echo drift >> new_spec.rb; grep -q "age >= 18" `+filepath.Join(f.app, "app/models/user.rb"))
	opts.TestsDir = tests
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Tests.Clean || doc.Tests.Unchanged || len(doc.Tests.ChangesDigest) != 64 || doc.Tests.NotEstablished != "" || doc.Outcome != "partial" {
		t.Fatalf("untracked drift not recorded: %+v outcome %s", doc.Tests, doc.Outcome)
	}
	if r := record(t, doc, "app/models/user.rb", 2, "command"); r.State != NotRun || r.Reason != "test-revision-drift" {
		t.Fatalf("drift record: %+v", r)
	}

	// An untracked file that does not change keeps the run unchanged.
	f = newFixture(t)
	tests = filepath.Join(f.scratch, "suite")
	if err := os.MkdirAll(tests, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tests, "init", "-q")
	writeFile(t, filepath.Join(tests, "suite.txt"), "suite\n", 0o644)
	gitRun(t, tests, "add", "-A")
	gitRun(t, tests, "commit", "-q", "-m", "suite")
	writeFile(t, filepath.Join(tests, "new_spec.rb"), "it { }\n", 0o644)
	opts = f.options("sh", "-c", `grep -q "age >= 18" `+filepath.Join(f.app, "app/models/user.rb"))
	opts.TestsDir = tests
	if doc, err = Discriminate(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if !doc.Tests.Unchanged || len(doc.Tests.ChangesDigest) != 64 {
		t.Fatalf("steady untracked file read as drift: %+v", doc.Tests)
	}
}

// CCE-V0-684 (V1-2882): a tests directory that is not a Git worktree has no revision to compare,
// so the test side is not established and no kill or survival stands.
func TestCCEV0684NonGitTestsDirIsNotEstablished(t *testing.T) {
	f := newFixture(t)
	tests := filepath.Join(t.TempDir(), "suite")
	if err := os.MkdirAll(tests, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := f.options("sh", "-c", `grep -q "age >= 18" `+filepath.Join(f.app, "app/models/user.rb"))
	opts.TestsDir = tests
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Tests.Unchanged || doc.Tests.Revision != "" || doc.Tests.NotEstablished != "not-a-git-worktree" || doc.Outcome != "partial" {
		t.Fatalf("non-Git tests dir: %+v outcome %s", doc.Tests, doc.Outcome)
	}
	if r := record(t, doc, "app/models/user.rb", 2, "command"); r.State != NotRun || r.Reason != "test-revision-not-established" {
		t.Fatalf("not-established record: %+v", r)
	}
}

// CCE-V0-172: --wall-time bounds the commands themselves: a baseline that outlives it is cut
// at the wall time, not at the longer per-run timeout, and the final rebuild gets only what is
// left.
func TestCCEV0172WallTimeBoundsRunningCommands(t *testing.T) {
	f := newFixture(t)
	opts := f.options("sleep", "20")
	opts.WallTime = time.Second
	started := time.Now()
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("baseline ran %s past a 1s wall time", elapsed)
	}
	assertRestored(t, f, doc)
	if doc.Baseline.Infrastructure != WallTimeExhausted || doc.Outcome != "baseline-infrastructure" {
		t.Fatalf("baseline: %+v outcome %s", doc.Baseline, doc.Outcome)
	}
	opts = f.options("true")
	opts.RebuildCommand = []string{"sh", "-c", `[ "$CORVINT_EXTMUTATE_RUN" != restore ] || sleep 20`}
	opts.WallTime = 2 * time.Second
	started = time.Now()
	doc, err = Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("restore rebuild ran %s past a 2s wall time", elapsed)
	}
	assertRestored(t, f, doc)
	if doc.RestoreRebuild != WallTimeExhausted {
		t.Fatalf("restore rebuild %q", doc.RestoreRebuild)
	}
}

// CCE-V0-175: every refusal happens before the app directory is touched.
func TestCCEV0175Refusals(t *testing.T) {
	f := newFixture(t)
	other := filepath.Join(f.scratch, "base-tree")
	gitRun(t, f.root, "worktree", "add", "-q", "--detach", other, f.base)
	for code, mutate := range map[string]func(*Options){
		"discriminate-input-required":    func(o *Options) { o.AppDir = "" },
		"discriminate-command-invalid":   func(o *Options) { o.TestCommand = []string{"run", "--password=hunter2hunter2"} },
		"discriminate-verdict-invalid":   func(o *Options) { o.Verdict = "junit" },
		"discriminate-bound-invalid":     func(o *Options) { o.MaxMutants = MaxMutants + 1 },
		"discriminate-revision-invalid":  func(o *Options) { o.Head = "no-such-revision" },
		"discriminate-app-dir-invalid":   func(o *Options) { o.AppDir = other },
		"discriminate-command-invalid/2": func(o *Options) { o.RebuildCommand = []string{""} },
	} {
		opts := f.options("true")
		mutate(&opts)
		_, err := Discriminate(context.Background(), opts)
		var refusal *Refusal
		if !errors.As(err, &refusal) || refusal.Code != strings.TrimSuffix(code, "/2") {
			t.Fatalf("%s: %v", code, err)
		}
	}
	writeFile(t, filepath.Join(f.app, "app/models/user.rb"), "dirty\n", 0o644)
	_, err := Discriminate(context.Background(), f.options("true"))
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != "discriminate-app-dir-invalid" {
		t.Fatalf("dirty app dir: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(f.app, "app/models/user.rb")); string(data) != "dirty\n" {
		t.Fatal("refusal touched the app dir")
	}
}

// recommit adds a commit on top of the fixture's head and moves the app worktree to it.
func (f fixture) recommit(t *testing.T, files map[string]string, deletes ...string) fixture {
	t.Helper()
	for name, body := range files {
		writeFile(t, filepath.Join(f.root, filepath.FromSlash(name)), body, 0o644)
	}
	for _, name := range deletes {
		gitRun(t, f.root, "rm", "-q", name)
	}
	gitRun(t, f.root, "add", "-A")
	gitRun(t, f.root, "commit", "-q", "-m", "more")
	f.head = gitRun(t, f.root, "rev-parse", "HEAD")
	gitRun(t, f.app, "checkout", "-q", "--detach", f.head)
	return f
}

func excludedRules(doc Document) map[string]string {
	rules := map[string]string{}
	for _, e := range doc.Excluded {
		rules[e.Path] = e.Rule
	}
	return rules
}

// CCE-V0-236: every changed file without a hunk is listed with its rule, so an application file
// under an excluded directory name is never a silent drop.
func TestCCEV0236ExcludedFilesAreListed(t *testing.T) {
	f := newFixture(t).recommit(t, map[string]string{
		"app/services/e2e/seed.rb": "class Seed\n  def ok?(n)\n    n == 1\n  end\nend\n",
		"README.md":                "changed\n",
		"types/global.d.ts":        "declare const x: boolean;\n",
	})
	f.base = gitRun(t, f.root, "rev-parse", f.head+"~2")
	gitRun(t, f.root, "rm", "-q", "app/lib/price.ts")
	writeFile(t, filepath.Join(f.root, "app/models/user.rb"), strings.Replace(userHead, "    \"x\"\n", "", 1), 0o644)
	gitRun(t, f.root, "add", "-A")
	gitRun(t, f.root, "commit", "-q", "-m", "drop")
	f.head = gitRun(t, f.root, "rev-parse", "HEAD")
	gitRun(t, f.app, "checkout", "-q", "--detach", f.head)
	opts := f.options("true")
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"README.md": "language", "app/services/e2e/seed.rb": "directory:e2e", "spec/models/user_spec.rb": "directory:spec", "types/global.d.ts": "declaration"}
	if got := excludedRules(doc); !reflect.DeepEqual(got, want) || doc.ExcludedOmitted != 0 {
		t.Fatalf("excluded %v, want %v", got, want)
	}
	if doc.Exclusion.DirectoryMatch != DirectoryMatchAny || len(doc.Exclusion.Directories) != len(DefaultExcludedDirectories) {
		t.Fatalf("exclusion %+v", doc.Exclusion)
	}
	// A name with a space or a quote is parsed from the hunk header, not mislabelled.
	named := map[string]string{"app/My Widget.ts": "export const w = (n: number) => n === 1;\n"}
	if runtime.GOOS != "windows" {
		named[`app/q"t.ts`] = "export const q = (n: number) => n === 2;\n"
	}
	h := f.recommit(t, named)
	h.base = f.head
	doc, err = Discriminate(context.Background(), h.options("true"))
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, hunk := range doc.Hunks {
		paths[hunk.Path] = named[hunk.Path]
	}
	if !reflect.DeepEqual(paths, named) || len(doc.Excluded) != 0 {
		t.Fatalf("quoted names: hunks %+v excluded %v", doc.Hunks, doc.Excluded)
	}
	gitRun(t, f.app, "checkout", "-q", "--detach", f.head)
	// A deleted file and an application file whose only change is a deletion are listed too.
	g := f
	g.base = gitRun(t, f.root, "rev-parse", f.head+"~1")
	doc, err = Discriminate(context.Background(), g.options("true"))
	if err != nil {
		t.Fatal(err)
	}
	if got := excludedRules(doc); !reflect.DeepEqual(got, map[string]string{"app/lib/price.ts": "deleted", "app/models/user.rb": "no-added-lines"}) || len(doc.Hunks) != 0 {
		t.Fatalf("deletions: excluded %v hunks %+v", got, doc.Hunks)
	}
}

// CCE-V0-237: the directory-name exclusion can match the first segment only, and names can be
// added or removed; the effective rule is recorded.
func TestCCEV0237DirectoryExclusionIsAdjustable(t *testing.T) {
	f := newFixture(t).recommit(t, map[string]string{
		"app/services/e2e/seed.rb": "class Seed\n  def ok?(n)\n    n == 1\n  end\nend\n",
		"e2e/support/login.ts":     "export const ok = (n: number) => n === 1;\n",
	})
	f.base = gitRun(t, f.root, "rev-parse", f.head+"~1")
	opts := f.options("true")
	opts.DirectoryMatch = DirectoryMatchLeading
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Hunks) != 1 || doc.Hunks[0].Path != "app/services/e2e/seed.rb" || excludedRules(doc)["e2e/support/login.ts"] != "directory:e2e" || doc.Exclusion.DirectoryMatch != DirectoryMatchLeading {
		t.Fatalf("leading: hunks %+v excluded %v", doc.Hunks, doc.Excluded)
	}
	opts = f.options("true")
	opts.KeepDirectories = []string{"e2e"}
	opts.ExcludeDirectories = []string{"services"}
	doc, err = Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Hunks) != 1 || doc.Hunks[0].Path != "e2e/support/login.ts" || excludedRules(doc)["app/services/e2e/seed.rb"] != "directory:services" {
		t.Fatalf("keep/exclude: hunks %+v excluded %v", doc.Hunks, doc.Excluded)
	}
	for _, d := range doc.Exclusion.Directories {
		if d == "e2e" {
			t.Fatalf("kept name still excluded: %v", doc.Exclusion.Directories)
		}
	}
	for _, bad := range []Options{{DirectoryMatch: "prefix"}, {KeepDirectories: []string{"app"}}, {ExcludeDirectories: []string{"a/b"}}} {
		o := f.options("true")
		o.DirectoryMatch, o.KeepDirectories, o.ExcludeDirectories = bad.DirectoryMatch, bad.KeepDirectories, bad.ExcludeDirectories
		var refusal *Refusal
		if _, err := Discriminate(context.Background(), o); !errors.As(err, &refusal) || refusal.Code != "discriminate-exclusion-invalid" {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
}

// CCE-V0-238: --map joins each hunk and record to the containing CEM hunk ID of a cem/1.0 map of
// the same base and head, and refuses a map of another change before any write.
func TestCCEV0238CEMHunkJoin(t *testing.T) {
	f := newFixture(t)
	session, err := workflow.Open(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{Base: f.base, Target: f.head}); err != nil {
		t.Fatal(err)
	}
	mapPath := filepath.Join(f.root, ".corvint", "change.cem.json")
	raw, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	stable, err := wire.ParseStable(raw)
	if err != nil {
		t.Fatal(err)
	}
	opts := f.options("true")
	opts.MapPath = mapPath
	doc, err := Discriminate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if doc.CEMMap == nil || doc.CEMMap.BaseRevision != f.base || doc.CEMMap.PatchSha256 != stable.Change.PatchSha256 || len(doc.CEMMap.Sha256) != 64 {
		t.Fatalf("map pin %+v", doc.CEMMap)
	}
	for _, h := range doc.Hunks {
		found := false
		for _, c := range stable.Change.Hunks {
			if c.ID == h.CEMHunk && c.Path == h.Path && c.NewRange.Start <= int64(h.NewStart) && int64(h.NewStart+h.NewCount) <= c.NewRange.Start+c.NewRange.Count {
				found = true
			}
		}
		if !found || h.CEMJoin != CEMJoined {
			t.Fatalf("hunk %+v not joined to a containing CEM hunk of %+v", h, stable.Change.Hunks)
		}
	}
	for _, r := range doc.Records {
		if r.CEMHunk != doc.Hunks[r.Hunk].CEMHunk {
			t.Fatalf("record %+v lost its CEM hunk", r)
		}
	}
	// An unreadable map, a non-map and a map of another range refuse before the app dir is written.
	writeFile(t, filepath.Join(f.scratch, "bad.json"), "{}\n", 0o644)
	for _, c := range []struct {
		opts Options
		code string
	}{
		{func() Options { o := f.options("true"); o.MapPath = filepath.Join(f.scratch, "missing.json"); return o }(), "discriminate-map-invalid"},
		{func() Options { o := f.options("true"); o.MapPath = filepath.Join(f.scratch, "bad.json"); return o }(), "discriminate-map-invalid"},
	} {
		var refusal *Refusal
		if _, err := Discriminate(context.Background(), c.opts); !errors.As(err, &refusal) || refusal.Code != c.code {
			t.Fatalf("%s: %v", c.code, err)
		}
	}
	g := f.recommit(t, map[string]string{"app/lib/extra.ts": "export const y = 1 === 1;\n"})
	o := g.options("true")
	o.MapPath = mapPath
	var refusal *Refusal
	if _, err := Discriminate(context.Background(), o); !errors.As(err, &refusal) || refusal.Code != "discriminate-map-mismatch" {
		t.Fatalf("mismatched head: %v", err)
	}
	if status := gitRun(t, g.app, "status", "--porcelain"); status != "" {
		t.Fatalf("refusal wrote the app dir: %q", status)
	}
}
