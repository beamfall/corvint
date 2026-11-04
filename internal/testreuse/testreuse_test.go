// SPDX-License-Identifier: Apache-2.0

package testreuse

import (
	"context"
	json "encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureModule = "example.com/fix"

var fixtureArgv = []string{"-count=1", "-timeout", "5m"}

// fixtureRepo writes a module whose package a is declared with scope
// data/a.txt and imports b; b is declared with an empty scope; c is
// undeclared. a's test reads data/a.txt and wants "ok".
func fixtureRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                         "module " + fixtureModule + "\n\ngo 1.27\n",
		"a/a.go":                         "package a\n\nimport \"" + fixtureModule + "/b\"\n\nfunc A() int { return b.B() }\n",
		"a/a_test.go":                    "package a\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestA(t *testing.T) {\n\tbody, err := os.ReadFile(\"../data/a.txt\")\n\tif err != nil || string(body) != \"ok\\n\" || A() != 1 {\n\t\tt.Fatalf(\"got %q %v\", body, err)\n\t}\n}\n",
		"b/b.go":                         "package b\n\nfunc B() int { return 1 }\n",
		"b/b_test.go":                    "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {\n\tif B() != 1 {\n\t\tt.Fatal()\n\t}\n}\n",
		"c/c.go":                         "package c\n",
		"c/c_test.go":                    "package c\n\nimport \"testing\"\n\nfunc TestC(t *testing.T) {}\n",
		"data/a.txt":                     "ok\n",
		"data/other.txt":                 "other\n",
		"docs/readme.md":                 "# fixture\n",
		".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"a":["data/a.txt"],"b":[]}}` + "\n",
	}
	for name, body := range files {
		writeFixture(t, root, name, body)
	}
	git(t, root, "init", "-q", "-b", "main")
	commitAll(t, root, "c1")
	return root
}

func writeFixture(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, arguments...)...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitAll(t *testing.T, root, message string) {
	t.Helper()
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "--allow-empty", "-m", message)
}

func fixtureEnv() []string {
	return append(os.Environ(), "GOFLAGS=", "GOWORK=off", "GOTOOLCHAIN=local", "GO111MODULE=on")
}

func observe(t *testing.T, root string, unbounded map[string]bool) (*Snapshot, Identity) {
	t.Helper()
	snapshot, err := Observe(context.Background(), ObserveOptions{Root: root, Env: fixtureEnv(), Unbounded: unbounded})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := ObserveIdentity(context.Background(), snapshot, "", fixtureEnv(), fixtureArgv, nil)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, identity
}

func recordAt(t *testing.T, root string, source Source, outcomes map[string]string) []byte {
	t.Helper()
	snapshot, identity := observe(t, root, nil)
	source.Commit, source.Tree = snapshot.Head, snapshot.Tree
	record, err := BuildRecord(snapshot, identity, source, outcomes)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

var allPassed = map[string]string{fixtureModule + "/a": "pass", fixtureModule + "/b": "pass", fixtureModule + "/c": "pass"}

func decideAt(t *testing.T, root string, policy Policy, records ...[]byte) map[string]Unit {
	t.Helper()
	snapshot, identity := observe(t, root, nil)
	var inputs []RecordInput
	for index, body := range records {
		inputs = append(inputs, RecordInput{Name: "record-" + string(rune('0'+index)), Body: body})
	}
	decision, err := Decide(snapshot, identity, policy, inputs, snapshot.Universe)
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]Unit{}
	for _, unit := range decision.Units {
		units[strings.TrimPrefix(unit.Package, fixtureModule+"/")] = unit
	}
	return units
}

var localPolicy = Policy{Trust: map[string]bool{SourceLocal: true, SourceMain: true}}

func wantUnits(t *testing.T, step string, units map[string]Unit, want map[string]string) {
	t.Helper()
	for name, reason := range want {
		if units[name].Reason != reason {
			t.Errorf("%s: package %s: reason %q (%s), want %q", step, name, units[name].Reason, units[name].Detail, reason)
		}
		if (units[name].Decision == DecisionReuse) != (reason == ReasonRecordDigestEqual) {
			t.Errorf("%s: package %s: decision %s with reason %s", step, name, units[name].Decision, units[name].Reason)
		}
	}
}

// TestRecordCarriesOnlyEligiblePasses is AFP-V0-2320 and AFP-V0-2323: an
// undeclared package and a failed one are skipped with their reasons.
func TestRecordCarriesOnlyEligiblePasses(t *testing.T) {
	root := fixtureRepo(t)
	outcomes := map[string]string{fixtureModule + "/a": "pass", fixtureModule + "/b": "fail", fixtureModule + "/c": "pass"}
	var record Record
	if err := json.Unmarshal(recordAt(t, root, Source{Kind: SourceLocal}, outcomes), &record); err != nil {
		t.Fatal(err)
	}
	if len(record.Packages) != 1 || record.Packages[0].Package != fixtureModule+"/a" {
		t.Fatalf("packages %+v, want only a", record.Packages)
	}
	reasons := map[string]string{}
	for _, skipped := range record.Skipped {
		reasons[skipped.Package] = skipped.Reason
	}
	if reasons[fixtureModule+"/b"] != ReasonNotPassed || reasons[fixtureModule+"/c"] != ReasonUndeclaredReadScope {
		t.Fatalf("skipped %+v", record.Skipped)
	}
}

// TestDigestNegativeControls is AFP-V0-2321 and AFP-V0-2325: every change to
// a path the digest binds reruns the package (negative controls), and a
// change outside it reuses the pass.
func TestDigestNegativeControls(t *testing.T) {
	root := fixtureRepo(t)
	base := git(t, root, "rev-parse", "HEAD")
	record := recordAt(t, root, Source{Kind: SourceMain}, allPassed)
	steps := []struct {
		name string
		edit func()
		want map[string]string
	}{
		{"unchanged", func() {}, map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonRecordDigestEqual, "c": ReasonUndeclaredReadScope}},
		{"documentation only", func() { writeFixture(t, root, "docs/readme.md", "# changed\n") }, map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonRecordDigestEqual}},
		{"undeclared data", func() { writeFixture(t, root, "data/other.txt", "changed\n") }, map[string]string{"a": ReasonRecordDigestEqual}},
		{"dependency test file", func() {
			writeFixture(t, root, "b/b_test.go", "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n")
		}, map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonDigestChanged}},
		{"declared scope file", func() { writeFixture(t, root, "data/a.txt", "changed\n") }, map[string]string{"a": ReasonDigestChanged, "b": ReasonRecordDigestEqual}},
		{"dependency source", func() { writeFixture(t, root, "b/b.go", "package b\n\nfunc B() int { return 1 + 0 }\n") }, map[string]string{"a": ReasonDigestChanged, "b": ReasonDigestChanged}},
		{"own testdata", func() { writeFixture(t, root, "a/testdata/x.txt", "x\n") }, map[string]string{"a": ReasonDigestChanged, "b": ReasonRecordDigestEqual}},
		{"module file", func() { writeFixture(t, root, "go.mod", "module "+fixtureModule+"\n\ngo 1.27\n\n// changed\n") }, map[string]string{"a": ReasonDigestChanged, "b": ReasonDigestChanged}},
		{"scope declaration", func() {
			writeFixture(t, root, ".corvint/test-read-scopes.json", `{"profile":"corvint-test-read-scopes/0","packages":{"a":["data/a.txt","data/other.txt"],"b":[]}}`+"\n")
		}, map[string]string{"a": ReasonDigestChanged, "b": ReasonRecordDigestEqual}},
		{"declared file created", func() {
			writeFixture(t, root, ".corvint/test-read-scopes.json", `{"profile":"corvint-test-read-scopes/0","packages":{"a":["data/a.txt"],"b":["data/new.txt"]}}`+"\n")
		}, map[string]string{"b": ReasonDigestChanged}},
	}
	for _, step := range steps {
		git(t, root, "checkout", "-q", "--force", base)
		git(t, root, "clean", "-q", "-fdx")
		step.edit()
		commitAll(t, root, step.name)
		wantUnits(t, step.name, decideAt(t, root, localPolicy, record), step.want)
	}
}

// TestDirtyOrUntrackedInputRuns is AFP-V0-2321: a worktree path the commit
// does not pin, at or below an input, is a DIGEST_ERROR, never a reuse.
func TestDirtyOrUntrackedInputRuns(t *testing.T) {
	root := fixtureRepo(t)
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	writeFixture(t, root, "data/a.txt", "dirty\n")
	units := decideAt(t, root, localPolicy, record)
	if units["a"].Reason != ReasonDigestError || !strings.HasPrefix(units["a"].Detail, ErrorDirtyInput) {
		t.Fatalf("dirty scope file: %+v", units["a"])
	}
	git(t, root, "checkout", "-q", "--", "data/a.txt")
	writeFixture(t, root, "a/untracked.txt", "x\n")
	units = decideAt(t, root, localPolicy, record)
	if units["a"].Reason != ReasonDigestError {
		t.Fatalf("untracked file in the package directory: %+v", units["a"])
	}
	os.Remove(filepath.Join(root, "a/untracked.txt"))
	writeFixture(t, root, "docs/untracked.md", "x\n")
	wantUnits(t, "untracked outside every input", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonRecordDigestEqual})
	writeFixture(t, root, ".corvint/test-read-scopes.json", `{"profile":"corvint-test-read-scopes/0","packages":{"a":["data/a.txt"]}}`+"\n")
	wantUnits(t, "dirty declaration", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonDigestError, "b": ReasonDigestError})
}

// TestIdentityAndTrust is AFP-V0-2322 and AFP-V0-2324.
func TestIdentityAndTrust(t *testing.T) {
	root := fixtureRepo(t)
	pr7 := recordAt(t, root, Source{Kind: SourcePullRequest, PullRequest: 7, RunID: 11, RunAttempt: 1}, allPassed)
	snapshot, identity := observe(t, root, nil)
	decision, err := Decide(snapshot, identity, Policy{Trust: map[string]bool{SourcePullRequest: true}, PullRequest: 8}, []RecordInput{{Name: "pr7", Body: pr7}}, snapshot.Universe)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Records[0].Verdict != RecordUntrusted || len(decision.Reused) != 0 {
		t.Fatalf("another pull request's record: %+v reused %v", decision.Records, decision.Reused)
	}
	decision, _ = Decide(snapshot, identity, Policy{Trust: map[string]bool{SourcePullRequest: true}, PullRequest: 7}, []RecordInput{{Name: "pr7", Body: pr7}}, snapshot.Universe)
	if len(decision.Reused) != 2 || decision.Units[0].Witness == nil && decision.Units[1].Witness == nil {
		t.Fatalf("the same pull request's record: reused %v", decision.Reused)
	}
	decision, _ = Decide(snapshot, identity, localPolicy, []RecordInput{{Name: "pr7", Body: pr7}}, snapshot.Universe)
	if decision.Records[0].Verdict != RecordUntrusted {
		t.Fatalf("a pull_request record under a local,main policy: %+v", decision.Records)
	}
	forged := strings.Replace(string(pr7), `"-count=1","-timeout","5m"`, `"-count=1","-timeout","9m"`, 1)
	decision, _ = Decide(snapshot, identity, Policy{Trust: map[string]bool{SourcePullRequest: true}, PullRequest: 7}, []RecordInput{{Name: "forged", Body: []byte(forged)}}, snapshot.Universe)
	if decision.Records[0].Verdict != RecordIdentityForged || len(decision.Reused) != 0 {
		t.Fatalf("an edited identity: %+v", decision.Records)
	}
	other, err := ObserveIdentity(context.Background(), snapshot, "", fixtureEnv(), []string{"-race", "-count=1", "-timeout", "5m"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	decision, _ = Decide(snapshot, other, Policy{Trust: map[string]bool{SourcePullRequest: true}, PullRequest: 7}, []RecordInput{{Name: "pr7", Body: pr7}}, snapshot.Universe)
	for _, unit := range decision.Units {
		if strings.HasSuffix(unit.Package, "/a") && unit.Reason != ReasonIdentityChanged {
			t.Fatalf("another argv: %+v", unit)
		}
	}
	decision, _ = Decide(snapshot, identity, localPolicy, []RecordInput{{Name: "missing", Err: os.ErrNotExist}}, snapshot.Universe)
	if decision.Records[0].Verdict != RecordInvalid || len(decision.Reused) != 0 {
		t.Fatalf("an unreadable record: %+v", decision.Records)
	}
}

// TestUnboundedAndSlicedUnitsRun is AFP-V0-2320.
func TestUnboundedAndSlicedUnitsRun(t *testing.T) {
	root := fixtureRepo(t)
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	snapshot, identity := observe(t, root, map[string]bool{fixtureModule + "/a": true})
	decision, err := Decide(snapshot, identity, localPolicy, []RecordInput{{Name: "r", Body: record}}, snapshot.Universe)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range decision.Units {
		if strings.HasSuffix(unit.Package, "/a") && unit.Reason != ReasonUnboundedReader {
			t.Fatalf("an unbounded reader: %+v", unit)
		}
	}
	writeFixture(t, root, SplitAllowPath, `{"profile":"corvint-ci-test-split-allow/0","packages":{"`+fixtureModule+`/b":"split"}}`+"\n")
	commitAll(t, root, "split b")
	wantUnits(t, "split package", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonSlicedUnit})
}

// TestFaultInjectionIsSelectedAndFails is the AFP-V0-2321 fault injection: a
// record made from a real passing go test -json run, then a change the test
// detects in a declared scope file; the decision runs the package and the
// package's test fails, while the untouched dependency is reused.
func TestFaultInjectionIsSelectedAndFails(t *testing.T) {
	root := fixtureRepo(t)
	stream := goTest(t, root, true, "-json", "./a", "./b")
	outcomes, err := ParseTestJSON(stream)
	if err != nil {
		t.Fatal(err)
	}
	if outcomes[fixtureModule+"/a"] != "pass" || outcomes[fixtureModule+"/b"] != "pass" {
		t.Fatalf("baseline outcomes %v", outcomes)
	}
	record := recordAt(t, root, Source{Kind: SourceLocal}, outcomes)
	writeFixture(t, root, "data/a.txt", "broken\n")
	commitAll(t, root, "inject")
	wantUnits(t, "injected", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonDigestChanged, "b": ReasonRecordDigestEqual})
	failed := goTest(t, root, false, "-json", "./a")
	after, err := ParseTestJSON(failed)
	if err != nil || after[fixtureModule+"/a"] != "fail" {
		t.Fatalf("the injected change must fail the selected package: %v %v", after, err)
	}
}

func goTest(t *testing.T, root string, wantPass bool, arguments ...string) []byte {
	t.Helper()
	command := exec.Command("go", append([]string{"test", "-count=1"}, arguments...)...)
	command.Dir = root
	command.Env = fixtureEnv()
	out, err := command.Output()
	if (err == nil) != wantPass {
		t.Fatalf("go test %v: err %v, want pass %v\n%s", arguments, err, wantPass, out)
	}
	return out
}

func TestCheckArgv(t *testing.T) {
	for _, argv := range [][]string{{"-count=1"}, {"-race", "-count=1", "-timeout", "50m", "-p", "1"}, {"-test.count=1"}} {
		if err := CheckArgv(argv); err != nil {
			t.Errorf("%v: %v", argv, err)
		}
	}
	for _, argv := range [][]string{{}, {"-timeout", "5m"}, {"-count=1", "-run", "TestX"}, {"-count=1", "-skip=TestY"}, {"-count", "1"}, {"-count=2"}, {"-count=1", "-short"}, {"-count=1", "-test.run=X"}} {
		if err := CheckArgv(argv); err == nil {
			t.Errorf("%v: accepted", argv)
		}
	}
}

func TestParseTestJSON(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"run","Package":"p/ok","Test":"TestA"}`,
		`{"Action":"pass","Package":"p/ok","Test":"TestA"}`,
		`{"Action":"pass","Package":"p/ok"}`,
		`{"Action":"fail","Package":"p/bad","Test":"TestB"}`,
		`{"Action":"pass","Package":"p/bad"}`,
		`{"Action":"skip","Package":"p/none"}`,
		`not json`,
	}, "\n")
	outcomes, err := ParseTestJSON([]byte(stream))
	if err != nil {
		t.Fatal(err)
	}
	if outcomes["p/ok"] != "pass" || outcomes["p/bad"] != "fail" || outcomes["p/none"] != "skip" {
		t.Fatalf("outcomes %v", outcomes)
	}
}

func TestMergeOutcomesRefusesAnotherCommit(t *testing.T) {
	commit, tree := strings.Repeat("a", 40), strings.Repeat("b", 40)
	other := strings.Repeat("c", 40)
	if _, err := MergeOutcomes([]Outcomes{{Target: other, Outcomes: map[string]string{}}}, commit, tree); err == nil {
		t.Fatal("another target was accepted")
	}
	if _, err := MergeOutcomes([]Outcomes{{Target: commit, Tree: &other, Outcomes: map[string]string{}}}, commit, tree); err == nil {
		t.Fatal("another tree was accepted")
	}
	merged, err := MergeOutcomes([]Outcomes{{Target: commit, Outcomes: map[string]string{"p": "pass"}}, {Target: commit, Outcomes: map[string]string{"p": "fail"}}}, commit, tree)
	if err != nil || merged["p"] == "pass" {
		t.Fatalf("a package one shard failed: %v %v", merged, err)
	}
}

func observeArgv(t *testing.T, root string, argv []string) (*Snapshot, Identity) {
	t.Helper()
	snapshot, err := Observe(context.Background(), ObserveOptions{Root: root, Env: fixtureEnv(), BuildFlags: BuildFlags(argv)})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := ObserveIdentity(context.Background(), snapshot, "", fixtureEnv(), argv, nil)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, identity
}

func reasonFor(t *testing.T, snapshot *Snapshot, identity Identity, record []byte, name string) Unit {
	t.Helper()
	decision, err := Decide(snapshot, identity, localPolicy, []RecordInput{{Name: "r", Body: record}}, snapshot.Universe)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range decision.Units {
		if unit.Package == fixtureModule+"/"+name {
			return unit
		}
	}
	t.Fatalf("no unit %s", name)
	return Unit{}
}

// TestBuildFlagsReachTheClosure is AFP-V0-2321 with AFP-V0-2322: a file only a
// -race build compiles is an input of a -race identity, so changing it reruns
// the package, while the same change is outside a non-race closure (control).
func TestBuildFlagsReachTheClosure(t *testing.T) {
	root := fixtureRepo(t)
	writeFixture(t, root, "b/b_race.go", "//go:build race\n\npackage b\n\nvar raced = 1\n")
	commitAll(t, root, "race file")
	race := []string{"-race", "-count=1", "-timeout", "5m"}
	records := map[string][]byte{}
	for name, argv := range map[string][]string{"race": race, "plain": fixtureArgv} {
		snapshot, identity := observeArgv(t, root, argv)
		record, err := BuildRecord(snapshot, identity, Source{Kind: SourceLocal, Commit: snapshot.Head, Tree: snapshot.Tree}, allPassed)
		if err != nil {
			t.Fatal(err)
		}
		if records[name], err = json.Marshal(record); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture(t, root, "b/b_race.go", "//go:build race\n\npackage b\n\nvar raced = 2\n")
	commitAll(t, root, "race edit")
	snapshot, identity := observeArgv(t, root, race)
	if unit := reasonFor(t, snapshot, identity, records["race"], "a"); unit.Reason != ReasonDigestChanged {
		t.Fatalf("race-only dependency file under -race: %+v", unit)
	}
	snapshot, identity = observeArgv(t, root, fixtureArgv)
	if unit := reasonFor(t, snapshot, identity, records["plain"], "a"); unit.Reason != ReasonRecordDigestEqual {
		t.Fatalf("race-only dependency file without -race: %+v", unit)
	}
}

// TestHiddenIndexFlagsAreDirty is AFP-V0-2321: a tracked input marked
// assume-unchanged or skip-worktree may differ from its blob while status says
// nothing, so it is a dirty input.
func TestHiddenIndexFlagsAreDirty(t *testing.T) {
	for _, flag := range []string{"--assume-unchanged", "--skip-worktree"} {
		root := fixtureRepo(t)
		record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
		git(t, root, "update-index", flag, "data/a.txt")
		writeFixture(t, root, "data/a.txt", "hidden\n")
		units := decideAt(t, root, localPolicy, record)
		if units["a"].Reason != ReasonDigestError || !strings.HasPrefix(units["a"].Detail, ErrorDirtyInput) {
			t.Fatalf("%s: %+v", flag, units["a"])
		}
	}
}

func TestArgvFlagsAreClassified(t *testing.T) {
	got := BuildFlags([]string{"-race", "-count=1", "-tags", "x y", "-timeout", "5m", "-p", "1", "-v"})
	if strings.Join(got, "|") != "-race|-tags=x y" {
		t.Fatalf("build flags %q", got)
	}
	for _, argv := range [][]string{{"-count=1", "-foo"}, {"-count=1", "-timeout"}, {"-count=1", "./pkg"}, {"-count=1", "-race=false"}, {"-count=1", "-overlay=x.json"}, {"-count=1", "-pgo=x.pprof"}, {"-count=1", "-gcflags=-importcfg=x"}} {
		if err := CheckArgv(argv); err == nil {
			t.Errorf("%v: accepted", argv)
		}
	}
	for _, flag := range []string{"-tags=", "-covermode=invalid", "-timeout=-1s", "-timeout=invalid", "-p=0", "-parallel=no", "-cpu=1,0"} {
		if err := CheckArgv([]string{"-count=1", flag}); err == nil {
			t.Errorf("invalid flag %s: accepted", flag)
		}
	}
	if err := CheckArgv([]string{"-count=1", "-timeout=0", "-cpu=1,2", "-covermode=atomic"}); err != nil {
		t.Fatal(err)
	}
}

// TestUnsupportedGOFLAGSFailClosed is AFP-V0-2322: a file-valued build
// override cannot be accepted on the identity of its path alone.
func TestUnsupportedGOFLAGSFailClosed(t *testing.T) {
	root := fixtureRepo(t)
	_, err := Observe(context.Background(), ObserveOptions{Root: root, Env: append(fixtureEnv(), "GOFLAGS=-overlay=missing.json")})
	if err == nil || !strings.Contains(err.Error(), "GOFLAGS:") {
		t.Fatalf("overlay: %v", err)
	}
}

// TestHarnessDirectoryReevaluation is AFP-V0-2322/AFP-V0-2326: reopening a
// saved identity must preserve the directory scope and notice new harness files.
func TestHarnessDirectoryReevaluation(t *testing.T) {
	root := fixtureRepo(t)
	writeFixture(t, root, "harness/runner.sh", "exit 0\n")
	commitAll(t, root, "harness")
	snapshot, _ := observe(t, root, nil)
	identity, err := ObserveIdentity(context.Background(), snapshot, "", fixtureEnv(), fixtureArgv, []string{"harness"})
	if err != nil {
		t.Fatal(err)
	}
	record, err := BuildRecord(snapshot, identity, Source{Kind: SourceLocal, Commit: snapshot.Head, Tree: snapshot.Tree}, allPassed)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, input := range identity.Harness {
		paths = append(paths, input.Path)
	}
	check := func(want string) {
		t.Helper()
		snapshot, _ := observe(t, root, nil)
		current, err := ObserveIdentity(context.Background(), snapshot, "", fixtureEnv(), fixtureArgv, paths)
		if err != nil {
			t.Fatal(err)
		}
		if unit := reasonFor(t, snapshot, current, body, "a"); unit.Reason != want {
			t.Fatalf("saved harness paths %v: %+v, want %s", paths, unit, want)
		}
	}
	check(ReasonRecordDigestEqual)
	writeFixture(t, root, "harness/extra.sh", "exit 1\n")
	commitAll(t, root, "new harness input")
	check(ReasonIdentityChanged)
}
