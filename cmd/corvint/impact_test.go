package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func impactCLIRepository(t *testing.T) string {
	t.Helper()
	root := cliRepository(t)
	files := map[string]string{
		"go.mod":                 "module example.test/cli\n\ngo 1.27.0\n",
		"pkg/main.go":            "package main\n\nfunc StableValue() string { return \"clean\" }\n",
		"pkg/main_test.go":       "package main\n\n// scenario:z-last feature:a-first\nfunc TestStableValue() {}\n",
		"testing/features.yaml":  "features:\n  - id: a-first\n    area: auth\n    summary: Scalar fixture.\n    adr: [12]\n    applies: [True, 'server\\nedge']\n    status: shipped\n",
		"testing/scenarios.yaml": "scenarios:\n  - id: z-last\n    area: auth\n    summary: Scenario fixture.\n    features: [a-first]\n    applies: [server]\n    status: shipped\n",
	}
	for relative, content := range files {
		file := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-qm", "add Go fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func addTrackedRustImpactFixture(t *testing.T, root string) string {
	t.Helper()
	relative := "crates/ignore/src/dir.rs"
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("pub fn rust_only() -> bool { true }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", relative)
	affectedGit(t, root, "commit", "-qm", "add Rust impact fixture")
	return relative
}

func runImpactProcess(t *testing.T, root string) processResult {
	t.Helper()
	arguments := []string{"--root", root, "impact", "pkg/main.go", "--limit", "10"}
	candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
	candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
	return execute(t, candidate)
}

func TestFreshProcessImpactMatchesPythonCleanAndMixed(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	candidate := runImpactProcess(t, root)
	if candidate.exit != 0 || len(candidate.stderr) != 0 || !bytes.Contains(candidate.stdout, []byte(`"mode":"impact"`)) {
		t.Fatalf("clean result=%#v", candidate)
	}

	if err := os.WriteFile(
		filepath.Join(root, "pkg", "main.go"),
		[]byte("package main\n\nfunc StableValue() string { return \"mixed\" }\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	candidate = runImpactProcess(t, root)
	if candidate.exit != 0 || len(candidate.stderr) != 0 || !bytes.Contains(candidate.stdout, []byte(`"state":"mixed-worktree"`)) {
		t.Fatalf("mixed result=%#v", candidate)
	}
}

type forbiddenImpactReader struct{ reads int }

func (reader *forbiddenImpactReader) Read(_ []byte) (int, error) {
	reader.reads++
	return 0, fmt.Errorf("impact read stdin")
}

func TestImpactRejectsUnsupportedInputsWithoutReadingStdin(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	for _, test := range []struct {
		name, code string
		arguments  []string
	}{
		// GPK-V0-027 retains the typed refusal for suffixes the index does not
		// admit; admitted-but-unruled suffixes are covered separately below.
		{"unadmitted-text", "unsupported-impact-path-suffix", []string{"--root", root, "impact", "README.csv"}},
		{"budget", "unsupported-impact-option", []string{"--root", root, "impact", "pkg/main.go", "--budget-bytes", "1024"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &forbiddenImpactReader{}
			var stdout, stderr bytes.Buffer
			if exit := run(test.arguments, reader, &stdout, &stderr); exit != 2 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
			}
			if stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte(`"code": "`+test.code+`"`)) {
				t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
			}
			if reader.reads != 0 {
				t.Fatalf("stdin reads=%d", reader.reads)
			}
		})
	}
}

func TestImpactDisclosesUnruledAdmittedSuffixAcrossSurfaces_GPKV0027(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	changed := addTrackedRustImpactFixture(t, root)
	standalone := standaloneContextMember(t, root, "impact", changed)

	var stdout, stderr bytes.Buffer
	input := strings.NewReader(`{"paths":["` + changed + `"]}`)
	if code := runContext(context.Background(), cliArguments(root, "file-change"), input, &stdout, &stderr); code != 0 {
		t.Fatalf("harness exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
	var harness struct {
		Context json.RawMessage `json:"context"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &harness); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(standalone, harness.Context) {
		t.Fatalf("context differs\nimpact  = %s\nharness = %s", standalone, harness.Context)
	}
	if !bytes.Contains(standalone, []byte("outside the native Go impact profile")) {
		t.Fatalf("impact context lacks profile disclosure: %s", standalone)
	}
}

func TestPythonIntegerSemantics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value string
		want  int
		ok    bool
	}{
		{"  +1_0  ", 10, true},
		{"٤٢", 42, true},
		{strings.Repeat("9", 100), maximumImpactLimit + 1, true},
		{"1__0", 0, false},
		{"_10", 0, false},
	} {
		got, ok := pythonInteger(test.value)
		if got != test.want || ok != test.ok {
			t.Fatalf("pythonInteger(%q) = (%d, %v), want (%d, %v)", test.value, got, ok, test.want, test.ok)
		}
	}
}

func TestInvalidImpactLimitUsesPythonReprBytes(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--root", root, "impact", "pkg/main.go", "--limit", "nope"}, &forbiddenImpactReader{}, &stdout, &stderr)
	want := "{\"code\": \"invalid-arguments\", \"error\": \"argument --limit: invalid int value: 'nope'\", \"ok\": false}\n"
	if exit != 2 || stdout.Len() != 0 || stderr.String() != want {
		t.Fatalf("exit=%d stdout=%q stderr=%q want=%q", exit, &stdout, &stderr, want)
	}
	for _, test := range []struct{ value, want string }{
		{"has'quote", `"has'quote"`},
		{"both'\"quotes", `'both\'"quotes'`},
		{"line\n", `'line\n'`},
		{"1\u00a02", `'1\xa02'`},
		{"1\u20282", `'1\u20282'`},
	} {
		if got := pythonRepr(test.value); got != test.want {
			t.Fatalf("pythonRepr(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

// repositoryBytesDigest hashes every path, mode and body under root, including
// .git and .corvint. excluded names a relative, slash-form path (typically
// ".corvint/self-observations.jsonl", the one write AGENTS.md invariant 4 /
// SOL-V0-007 permits a non-query command) to leave out of the digest; callers
// with nothing to exclude pass none.
func repositoryBytesDigest(t *testing.T, root string, excluded ...string) [sha256.Size]byte {
	t.Helper()
	skip := make(map[string]bool, len(excluded))
	for _, path := range excluded {
		skip[path] = true
	}
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(file string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if file != root {
			paths = append(paths, file)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	var snapshot bytes.Buffer
	for _, file := range paths {
		info, err := os.Lstat(file)
		if err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			t.Fatal(err)
		}
		if skip[filepath.ToSlash(relative)] {
			continue
		}
		fmt.Fprintf(&snapshot, "%s\x00%s\x00", filepath.ToSlash(relative), info.Mode())
		switch {
		case info.Mode().IsRegular():
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Write(data)
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(file)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.WriteString(target)
		}
		snapshot.WriteByte(0)
	}
	return sha256.Sum256(snapshot.Bytes())
}

func TestImpactLeavesRepositoryBytesUnchanged(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	before := repositoryBytesDigest(t, root)
	reader := io.LimitReader(strings.NewReader("unread"), 6)
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--root", root, "impact", "pkg/main.go"}, reader, &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
	after := repositoryBytesDigest(t, root)
	if before != after {
		t.Fatal("impact changed repository bytes")
	}
}

func TestWorkingTreeImpactIsExplicitAndDoesNotBroadenDefaultAuthority(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	untracked := "pkg/untracked.go"
	if err := os.WriteFile(filepath.Join(root, untracked), []byte("package main\n\nimport \"fmt\"\n\nfunc Untracked() { fmt.Println(StableValue()) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var defaultStdout, defaultStderr bytes.Buffer
	if exit := run([]string{"--root", root, "impact", untracked, "--limit", "10"}, &forbiddenImpactReader{}, &defaultStdout, &defaultStderr); exit != 2 {
		t.Fatalf("default exit=%d stdout=%q stderr=%q", exit, &defaultStdout, &defaultStderr)
	}
	if defaultStdout.Len() != 0 || !strings.Contains(defaultStderr.String(), "not tracked at revision") {
		t.Fatalf("default stdout=%q stderr=%q", &defaultStdout, &defaultStderr)
	}

	before := repositoryBytesDigest(t, root)
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--root", root, "impact", "--working-tree-untracked", untracked, "--limit", "10"}, &forbiddenImpactReader{}, &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"profile":"corvint-working-tree-impact/0"`)) ||
		!bytes.Contains(stdout.Bytes(), []byte(`"revision_membership":"absent"`)) ||
		!bytes.Contains(stdout.Bytes(), []byte(`"state":"WORKTREE_EVIDENCE"`)) {
		t.Fatalf("stdout=%s", &stdout)
	}
	if after := repositoryBytesDigest(t, root); before != after {
		t.Fatal("explicit working-tree impact changed repository bytes")
	}

	stdout.Reset()
	stderr.Reset()
	exit = run([]string{"--root", root, "impact", "--working-tree-untracked", "pkg/main.go"}, &forbiddenImpactReader{}, &stdout, &stderr)
	if exit != 2 || !strings.Contains(stderr.String(), "only paths absent from the captured revision") {
		t.Fatalf("tracked opt-in exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
}

func TestWorkingTreeImpactRejectsPathNormalizationAndFlagValues(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	for _, arguments := range [][]string{
		{"--root", root, "impact", "--working-tree-untracked=value", "pkg/new.go"},
		{"--root", root, "impact", "--working-tree-untracked", "--working-tree-untracked", "pkg/new.go"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := run(arguments, &forbiddenImpactReader{}, &stdout, &stderr); exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "does not accept a value or repetition") {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", arguments, exit, &stdout, &stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "new.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--root", root, "impact", "pkg/./new.go", "--working-tree-untracked"}, &forbiddenImpactReader{}, &stdout, &stderr)
	if exit != 2 || !strings.Contains(stderr.String(), "must be normalized and repository-relative") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
}

func TestCommittedRangeImpactIsExplicitReadOnlyAndHunkQualified(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	baseCommand := exec.Command("git", "rev-parse", "HEAD")
	baseCommand.Dir = root
	baseRaw, err := baseCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseRaw))
	if err := os.WriteFile(filepath.Join(root, "pkg", "main.go"), []byte("package main\n\n// Changed has no feature marker.\nfunc StableValue() string { return \"range\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-qm", "range target"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	before := repositoryBytesDigest(t, root)
	reader := &forbiddenImpactReader{}
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--root", root, "impact", "--base", base, "--limit", "10"}, reader, &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 || reader.reads != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q stdin=%d", exit, &stdout, &stderr, reader.reads)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"profile":"corvint-range-impact/0"`)) ||
		!bytes.Contains(stdout.Bytes(), []byte(`"baseCommit":"`+base+`"`)) ||
		!bytes.Contains(stdout.Bytes(), []byte(`"id":"pkg/main.go","kind":"path"`)) {
		t.Fatalf("stdout=%s", &stdout)
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"kind":"feature"`)) || bytes.Contains(stdout.Bytes(), []byte(`a-first`)) {
		t.Fatalf("untouched feature marker leaked: %s", &stdout)
	}
	if after := repositoryBytesDigest(t, root); before != after {
		t.Fatal("range impact changed repository bytes")
	}
}

func TestCommittedRangeImpactRejectsAmbiguousCLIForms(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	baseCommand := exec.Command("git", "rev-parse", "HEAD")
	baseCommand.Dir = root
	baseRaw, err := baseCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseRaw))
	for _, arguments := range [][]string{
		{"--root", root, "impact", "--base", base, "pkg/main.go"},
		{"--root", root, "impact", "--base", base, "--base", base},
		{"--root", root, "impact", "--base", base, "--working-tree-untracked"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := run(arguments, &forbiddenImpactReader{}, &stdout, &stderr); exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code": "invalid-arguments"`) {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", arguments, exit, &stdout, &stderr)
		}
	}
}
