package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/witness"
)

func TestParseWitnessInvocationInterceptsAheadOfTheSharedParser(t *testing.T) {
	t.Parallel()
	repository := leaseRoot(t)
	cases := []struct {
		name    string
		input   []string
		match   bool
		rest    []string
		rootSet bool
	}{
		{"bare verb", []string{"witness", "--base", "abc"}, true, []string{"--base", "abc"}, false},
		{"separate root", []string{"--root", repository, "witness", "--base", "abc"}, true, []string{"--base", "abc"}, true},
		{"inline root", []string{"--root=" + repository, "witness"}, true, []string{}, true},
		{"another verb", []string{"impact", "--base", "abc"}, false, nil, false},
		{"verb not first", []string{"impact", "witness"}, false, nil, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root, rest, matched, err := parseWitnessInvocation(testCase.input)
			if matched != testCase.match {
				t.Fatalf("matched = %t, want %t", matched, testCase.match)
			}
			if !matched {
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(rest, " ") != strings.Join(testCase.rest, " ") {
				t.Errorf("rest = %v, want %v", rest, testCase.rest)
			}
			// normalizeRoot resolves symlinks, so the temporary root may arrive under /private.
			if testCase.rootSet && !strings.HasSuffix(root, repository) {
				t.Errorf("root = %q, want a resolved %q", root, repository)
			}
		})
	}
}

func TestParseWitnessOptions(t *testing.T) {
	t.Parallel()
	options, err := parseWitnessOptions([]string{"--base", "abc", "--head=def", "--cem", "x.json", "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if options.base != "abc" || options.head != "def" || options.cem != "x.json" || !options.json {
		t.Fatalf("options = %+v", options)
	}
}

// A bare `--json` must be accepted and only an actual value refused. lane-plan
// once got this wrong by reporting a bare flag as inline; both parsers now
// treat the boolean as "an inline value was supplied" and nothing else.
func TestParseWitnessOptionsAcceptsBareJSON(t *testing.T) {
	t.Parallel()
	options, err := parseWitnessOptions([]string{"--base", "abc", "--json"})
	if err != nil {
		t.Fatalf("--json must be accepted without a value: %v", err)
	}
	if !options.json {
		t.Error("--json did not set the JSON flag")
	}
	if _, err := parseWitnessOptions([]string{"--base", "abc", "--json=1"}); err == nil {
		t.Error("--json=1 must be refused")
	}
}

func TestParseWitnessOptionsRejections(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"missing base":      {"--json"},
		"unknown option":    {"--base", "abc", "--depth", "2"},
		"missing value":     {"--base"},
		"missing cem value": {"--base", "abc", "--cem"},
	}
	for name, arguments := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWitnessOptions(arguments); err == nil {
				t.Errorf("%v must be refused", arguments)
			}
		})
	}
}

// AGW-V0-008: --evidence is repeatable, resolves to an absolute path, and is
// bounded; a document the report cannot read refuses with exit 2 naming it.
func TestParseWitnessEvidence(t *testing.T) {
	t.Parallel()
	options, err := parseWitnessOptions([]string{"--base", "abc", "--evidence", "one.json", "--evidence=/two.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	first, _ := filepath.Abs("one.json")
	second, _ := filepath.Abs("/two.json")
	if len(options.evidence) != 2 || options.evidence[0] != first || options.evidence[1] != second {
		t.Fatalf("evidence = %v", options.evidence)
	}
	arguments := []string{"--base", "abc"}
	for range witness.MaxEvidenceFiles + 1 {
		arguments = append(arguments, "--evidence", "x.json")
	}
	if _, err := parseWitnessOptions(arguments); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("err = %v, want the --evidence bound", err)
	}
	if _, err := parseWitnessOptions([]string{"--base", "abc", "--evidence"}); err == nil {
		t.Fatal("--evidence without a value must be refused")
	}
	root := witnessCLIRange(t)
	missing := filepath.Join(t.TempDir(), "absent.json")
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := runWitness(t.Context(), root, []string{"--base", "HEAD~1", "--evidence", missing}, stdout, stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--evidence "+missing) {
		t.Fatalf("exit = %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}

// AGW-V0-011: --evidence-in-repo values are kept as given for the reader's
// path admission, and share the --evidence bound.
func TestParseWitnessRepositoryEvidence(t *testing.T) {
	t.Parallel()
	options, err := parseWitnessOptions([]string{"--base", "abc", "--evidence-in-repo", ".corvint/run.json", "--evidence-in-repo=../x.json", "--evidence", "one.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options.repositoryEvidence) != 2 || options.repositoryEvidence[0] != ".corvint/run.json" || options.repositoryEvidence[1] != "../x.json" || len(options.evidence) != 1 {
		t.Fatalf("repository evidence = %v, evidence = %v", options.repositoryEvidence, options.evidence)
	}
	arguments := []string{"--base", "abc", "--evidence", "x.json"}
	for range witness.MaxEvidenceFiles {
		arguments = append(arguments, "--evidence-in-repo", "x.json")
	}
	if _, err := parseWitnessOptions(arguments); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("err = %v, want the combined bound", err)
	}
	root := witnessCLIRange(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := runWitness(t.Context(), root, []string{"--base", "HEAD~1", "--evidence-in-repo", "../x.json"}, stdout, stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--evidence-in-repo ../x.json: the path leaves the checkout through ..") {
		t.Fatalf("exit = %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}

// AGW-V0-005 and AGW-V0-007: --head resolves like --base; a head other than
// the checked-out HEAD is a historical head compiled from the object store and
// bound COMMIT-PINNED, the checked-out HEAD carries no head binding, and a head
// that names no commit or does not descend from --base refuses.
func TestRunWitnessCompilesAHistoricalHead(t *testing.T) {
	t.Parallel()
	root := witnessCLIRange(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# later\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "commit", "-qam", "later")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, output)
	}
	command = exec.Command("git", "rev-parse", "HEAD~1", "HEAD")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	commits := strings.Fields(string(output))
	for head, binding := range map[string]string{"HEAD~1": "COMMIT-PINNED", "HEAD": ""} {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		code := runWitness(t.Context(), root, []string{"--base", "HEAD~2", "--head", head, "--json"}, stdout, stderr)
		if code != 0 {
			t.Fatalf("--head %s: exit = %d; stderr %s", head, code, stderr.String())
		}
		var report struct {
			Range struct {
				Head        string `json:"head"`
				HeadBinding string `json:"headBinding"`
			} `json:"range"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("--head %s: %v: %s", head, err, stdout.String())
		}
		wantHead := commits[1]
		if head == "HEAD~1" {
			wantHead = commits[0]
		}
		if report.Range.Head != wantHead || report.Range.HeadBinding != binding {
			t.Fatalf("--head %s: range %+v, want head %s binding %q\n%s", head, report.Range, wantHead, binding, stdout.String())
		}
		if binding == "" && strings.Contains(stdout.String(), "headBinding") {
			t.Fatalf("the checked-out head must keep its report bytes: %s", stdout.String())
		}
	}
	for head, want := range map[string]string{
		strings.Repeat("b", 40): "does not resolve to a commit",
		"HEAD~2":                "ancestor",
	} {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		code := runWitness(t.Context(), root, []string{"--base", "HEAD~1", "--head", head}, stdout, stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), want) {
			t.Errorf("--head %s: exit = %d, stdout %q, stderr %s; want 2 naming %q", head, code, stdout.String(), stderr.String(), want)
		}
	}
}

// witnessCLIRange commits a second revision on cliRepository, so HEAD~1..HEAD
// is a one-path range.
func witnessCLIRange(t *testing.T) string {
	t.Helper()
	root := cliRepository(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "commit", "-qam", "change")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, output)
	}
	return root
}

// witnessGoModuleRange builds a Go module with a slash-qualified module path
// in which b imports a, then commits a change to a/a.go, so HEAD~1..HEAD opens
// a change obligation on a/a.go (AGW-V0-009 (5)).
func witnessGoModuleRange(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/cli\n\ngo 1.22\n",
		"a/a.go": "package a\n\n// A is used by b.\nfunc A() int { return 1 }\n",
		"b/b.go": "package b\n\nimport \"example.test/cli/a\"\n\n// B calls A.\nfunc B() int { return a.A() }\n",
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(arguments ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "user.email=corvint@example.test", "-c", "user.name=Corvint Test", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, arguments...)...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-qm", "initial")
	if err := os.WriteFile(filepath.Join(root, "a", "a.go"), []byte("package a\n\n// A is used by b.\nfunc A() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("commit", "-qam", "change a")
	return root
}

// TestRunWitnessAttachesABoundReceipt pins AGW-V0-008 and AGW-V0-009 end to
// end through the CLI: a receipt shaped as the qualified reporter writes it
// (absolute checkout paths, a test-directory-relative anchor, head-blob
// digests) and bound to the head commit and tree attaches a non-closing
// CALLER_REPORTED witness to the changed file.
func TestRunWitnessAttachesABoundReceipt(t *testing.T) {
	t.Parallel()
	root := witnessGoModuleRange(t)
	checkout, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "rev-parse", "HEAD", "HEAD^{tree}")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	identity := strings.Fields(string(output))
	digest := func(file string) string {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	at := &jstestprovider.ApplicationRepositoryIdentity{RootCommit: identity[0], Revision: identity[0], Tree: identity[1], DirtyState: "clean"}
	publish := *at
	prefix := filepath.ToSlash(checkout)
	receipt := jstestprovider.Receipt{
		Kind: "unit", TestRepositoryAtStart: at, TestRepositoryAtPublish: &publish,
		Identity: jstestprovider.Identity{TestFileDigests: map[string]string{prefix + "/b/b.go": digest("b/b.go")}},
		Tests: []jstestprovider.TestOutcome{{
			Name: "adds", FullName: "b adds", State: jstestprovider.StatePassed,
			Anchor:        &jstestprovider.Anchor{File: "b.go", Line: 3},
			ImportedFiles: map[string]string{prefix + "/a/a.go": digest("a/a.go")},
		}},
	}
	data, err := json.Marshal(map[string]any{"receipt": receipt})
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(evidence, data, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := runWitness(t.Context(), root, []string{"--base", "HEAD~1", "--json", "--evidence", evidence}, stdout, stderr); code != 0 {
		t.Fatalf("exit = %d; stderr %s", code, stderr.String())
	}
	var report struct {
		Sources []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
			Basis  int    `json:"basis"`
		} `json:"sources"`
		Obligations []struct {
			Path      string `json:"path"`
			Witnesses []struct {
				Source       string `json:"source"`
				Authority    string `json:"authority"`
				EvidencePath string `json:"evidencePath"`
				Closing      bool   `json:"closing"`
			} `json:"witnesses"`
		} `json:"obligations"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("%v: %s", err, stdout.String())
	}
	if len(report.Sources) != 2 || report.Sources[1].Status != "BOUND" || report.Sources[1].Basis < 1 {
		t.Fatalf("sources = %+v", report.Sources)
	}
	attached := false
	for _, item := range report.Obligations {
		for _, candidate := range item.Witnesses {
			attached = attached || item.Path == "a/a.go" && candidate.Source == "evidence" && candidate.Authority == "CALLER_REPORTED" && !candidate.Closing && candidate.EvidencePath == "b/b.go"
		}
	}
	if !attached {
		t.Fatalf("no CALLER_REPORTED witness on a/a.go: %s", stdout.String())
	}
	assertWitnessCheckModes(t, root, evidence, stdout.Bytes())
}

// assertWitnessCheckModes pins AGW-V0-034 on a report whose every obligation
// is analysed and carries a candidate witness while none closes: the
// valueless --check exits 1, and --check=admitted and --check=candidate exit
// 0 with the report bytes unchanged; without the evidence, candidate exits 1.
func assertWitnessCheckModes(t *testing.T, root, evidence string, want []byte) {
	t.Helper()
	var report struct {
		Range   struct{ Admission, Closure string } `json:"range"`
		Summary struct {
			Opened, Closed, NotRun, Unwitnessed int
		} `json:"summary"`
	}
	if err := json.Unmarshal(want, &report); err != nil {
		t.Fatal(err)
	}
	if report.Range.Admission != "COMPUTED" || report.Range.Closure != "COMPUTED" || report.Summary.NotRun != 0 ||
		report.Summary.Unwitnessed != 0 || report.Summary.Closed >= report.Summary.Opened {
		t.Fatalf("fixture must be analysed, candidate-witnessed and unclosed: %+v", report)
	}
	for _, item := range []struct {
		check    string
		evidence bool
		exit     int
	}{
		{"--check", true, 1},
		{"--check=admitted", true, 0},
		{"--check=candidate", true, 0},
		{"--check=admitted", false, 0},
		{"--check=candidate", false, 1},
	} {
		arguments := []string{"--base", "HEAD~1", "--json", item.check}
		if item.evidence {
			arguments = append(arguments, "--evidence", evidence)
		}
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		if code := runWitness(t.Context(), root, arguments, stdout, stderr); code != item.exit {
			t.Fatalf("%v: exit = %d, want %d; stderr %s", arguments, code, item.exit, stderr.String())
		}
		if item.evidence && !bytes.Equal(stdout.Bytes(), want) {
			t.Fatalf("%v changed the report bytes", arguments)
		}
	}
}

// AGW-V0-005 and AGW-V0-006: --base takes any revision Git resolves and the
// report carries the full commit object ID; without --check a report exits 0,
// and with --check an unclosed obligation exits 1 after printing the report.
func TestRunWitnessResolvesRevisionsAndChecks(t *testing.T) {
	t.Parallel()
	root := witnessCLIRange(t)
	command := exec.Command("git", "rev-parse", "HEAD~1", "HEAD")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	commits := strings.Fields(string(output))
	for _, check := range []bool{false, true} {
		arguments := []string{"--base", "HEAD~1", "--head", "HEAD", "--json"}
		if check {
			arguments = append(arguments, "--check")
		}
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		code := runWitness(t.Context(), root, arguments, stdout, stderr)
		want := 0
		if check {
			want = 1
		}
		if code != want {
			t.Fatalf("%v: exit = %d, want %d; stderr %s", arguments, code, want, stderr.String())
		}
		var report struct {
			Range struct {
				Base string `json:"base"`
				Head string `json:"head"`
			} `json:"range"`
			Summary struct {
				Opened int `json:"opened"`
				Closed int `json:"closed"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("%v: %v: %s", arguments, err, stdout.String())
		}
		if report.Range.Base != commits[0] || report.Range.Head != commits[1] {
			t.Fatalf("range %+v, want base %s head %s", report.Range, commits[0], commits[1])
		}
		if report.Summary.Opened == 0 || report.Summary.Closed >= report.Summary.Opened {
			t.Fatalf("fixture must open an unclosed obligation: %+v", report.Summary)
		}
	}
	for _, arguments := range [][]string{{"--check=1"}, {"--check=closed"}, {"--check=admitted", "--check=candidate"}, {"--check", "--check=admitted"}, {"--check=admitted", "--check"}} {
		if _, err := parseWitnessOptions(append([]string{"--base", "x"}, arguments...)); err == nil {
			t.Errorf("%v must be refused", arguments)
		}
	}
	if options, err := parseWitnessOptions([]string{"--base", "x", "--check=candidate", "--check=candidate"}); err != nil || !options.check || options.checkMode != "candidate" {
		t.Errorf("repeated --check=candidate = %+v, %v", options, err)
	}
}

// TestWitnessAnalysedAdmitsAnEmptyClosure pins AGW-V0-034: a range that
// admitted no path has an EMPTY closure and passes the analysis gate, while a
// TRUNCATED closure fails it.
func TestWitnessAnalysedAdmitsAnEmptyClosure(t *testing.T) {
	t.Parallel()
	for closure, want := range map[string]bool{"COMPUTED": true, "EMPTY": true, "TRUNCATED": false, "REFUSED": false} {
		for _, mode := range []string{witnessCheckAdmitted, witnessCheckCandidate} {
			if got := witnessAnalysed(witness.Range{Admission: "COMPUTED", Closure: closure}, witness.Summary{}, mode); got != want {
				t.Errorf("closure %s mode %s analysed = %v, want %v", closure, mode, got, want)
			}
		}
	}
}

// TestWitnessCheckNamesTheAdmissionPacketBound pins AGW-V0-038 (V1-2959):
// --check=admitted over a PARTIAL admission exits 1 and names the bound on
// stderr, while a passing gate writes nothing there.
func TestWitnessCheckNamesTheAdmissionPacketBound(t *testing.T) {
	t.Parallel()
	detail := "10 admitted paths fell outside the 50-result admission packet bound (ADMISSION_PACKET_BOUND) and each is a NOT_RUN obligation: lib/f51.rb"
	report := &witness.Report{Range: witness.Range{Admission: "PARTIAL", AdmissionDetail: detail, Closure: "COMPUTED"}}
	stderr := &bytes.Buffer{}
	if code := witnessExit(report, witnessOptions{check: true, checkMode: witnessCheckAdmitted}, stderr); code != 1 ||
		!strings.Contains(stderr.String(), "admission is PARTIAL: "+detail) {
		t.Fatalf("exit %d stderr %q", code, stderr.String())
	}
	report.Range = witness.Range{Admission: "COMPUTED", Closure: "COMPUTED"}
	stderr.Reset()
	if code := witnessExit(report, witnessOptions{check: true, checkMode: witnessCheckAdmitted}, stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr.String())
	}
}

func TestRunWitnessRefusesAnUnknownBase(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := runWitness(t.Context(), root, []string{"--base", strings.Repeat("a", 40)}, stdout, stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("a refused range must print no report, got %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "does not resolve to a commit") {
		t.Errorf("stderr = %s", stderr.String())
	}
}

// An unknown option is named as unknown even when no value follows it.
func TestParseWitnessOptionsNamesAnUnknownOptionBeforeItsValue(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{{"--base", "abc", "--bogus"}, {"--bogus", "--json"}, {"--bogus", "x"}} {
		_, err := parseWitnessOptions(arguments)
		if err == nil || err.Error() != "unrecognized witness option: --bogus" {
			t.Errorf("%v: err = %v", arguments, err)
		}
	}
}

// AGW-V0-001: an explicit --root that is not a Git repository is refused as an
// argument error before the index builds, as the sibling native verbs do.
func TestParseWitnessInvocationRefusesARootThatIsNotARepository(t *testing.T) {
	t.Parallel()
	_, _, matched, err := parseWitnessInvocation([]string{"--root", filepath.Join(t.TempDir(), "missing"), "witness", "--base", "abc"})
	var kernelError *gokernel.Error
	if !matched || !errors.As(err, &kernelError) || kernelError.Code != "invalid-arguments" ||
		!strings.HasPrefix(kernelError.Message, "not a Git repository: ") {
		t.Fatalf("matched = %t err = %v", matched, err)
	}
}
