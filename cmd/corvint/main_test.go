package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
)

func cliRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	commands := [][]string{
		{"init", "-q"},
		{"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"},
		{"config", "gc.auto", "0"},
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commands = append(commands, []string{"add", "."}, []string{"commit", "-qm", "initial"})
	for _, arguments := range commands {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func TestCandidateHelperProcess(t *testing.T) {
	if os.Getenv("CORVINT_HELPER_PROCESS") != "1" {
		return
	}
	separator := 0
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index + 1
			break
		}
	}
	if separator == 0 {
		os.Exit(125)
	}
	os.Exit(run(os.Args[separator:], os.Stdin, os.Stdout, os.Stderr))
}

func testEnvironment(overrides ...string) []string {
	replaced := make(map[string]bool, len(overrides))
	for _, value := range overrides {
		name, _, _ := strings.Cut(value, "=")
		replaced[name] = true
	}
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if !replaced[name] {
			environment = append(environment, value)
		}
	}
	return append(environment, overrides...)
}

func runCandidateWithEnvironment(t *testing.T, stdin string, overrides []string, arguments ...string) ([]byte, string, int) {
	t.Helper()
	command := candidateCommand(arguments...)
	command.Env = testEnvironment(append([]string{"CORVINT_HELPER_PROCESS=1"}, overrides...)...)
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if err == nil {
		return stdout.Bytes(), stderr.String(), 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatal(err)
	}
	return stdout.Bytes(), stderr.String(), exit.ExitCode()
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate module root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func assertPrivateFileMode(t *testing.T, name string, mode os.FileMode, want os.FileMode) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose POSIX file-mode semantics")
		}
		if mode.Perm() != want {
			t.Fatalf("mode=%v, want %v", mode, want)
		}
	})
}

type processResult struct {
	exit   int
	stdout []byte
	stderr []byte
}

func execute(t *testing.T, command *exec.Cmd) processResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	exit := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatalf("execute: %v", err)
		}
		exit = exitError.ExitCode()
	}
	return processResult{exit: exit, stdout: stdout.Bytes(), stderr: stderr.Bytes()}
}

func cliArguments(root, event string) []string {
	return []string{
		"--root", root, "harness", "event", "--host", "codex",
		"--host-version", "unknown", "--surface", "plugin",
		"--adapter-version", "0.1.0", "--event", event, "--input", "-",
	}
}

func TestAHI001ParseHarnessBudgetBoundary(t *testing.T) {
	t.Parallel()
	arguments := cliArguments(".", "stop")[2:]
	if _, err := parse(append(arguments, "--budget-bytes", fmt.Sprint(gokernel.MinOutputBytes-1))); err == nil {
		t.Fatalf("budget %d accepted, want rejection", gokernel.MinOutputBytes-1)
	}
	parsed, err := parse(append(arguments, "--budget-bytes", fmt.Sprint(gokernel.MinOutputBytes)))
	if err != nil {
		t.Fatalf("budget %d rejected: %v", gokernel.MinOutputBytes, err)
	}
	if parsed.budgetBytes != gokernel.MinOutputBytes {
		t.Fatalf("budget = %d, want %d", parsed.budgetBytes, gokernel.MinOutputBytes)
	}
}

func TestAHI011HarnessHostChoicesUseEmbeddedSchema(t *testing.T) {
	t.Parallel()
	if got, want := harnessHostChoices, gokernel.HarnessHosts(); !slices.Equal(got, want) {
		t.Fatalf("CLI host choices = %v, embedded schema = %v", got, want)
	}
	for _, host := range harnessHostChoices {
		if !knownHost(host) {
			t.Fatalf("CLI rejects embedded host %q", host)
		}
	}
}

// LTPM-V0-011, SOL-V0-007: dogfood-record names the verification-syntax code
// on the wire and, like record, appends exactly one ledger row.
func TestDogfoodRecordUnsupportedVerifySyntaxAppendsOneObservation(t *testing.T) {
	t.Parallel()
	root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repository"))
	base := recordRevision(t, root)
	writeDogfoodChange(t, root, "internal/example/value.go", "package example\n\nconst Changed = true\n")
	commitDogfoodChange(t, root, "verify syntax")
	target := recordRevision(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", ".gitignore"), []byte("*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	arguments := []string{
		"--root", root, "dogfood-record", "--base", base, "--target", target,
		"--task", "dogfood task", "--verify", "cmd-a && cmd-b", "--outcome", "passed",
	}

	result := execute(t, candidateCommand(arguments...))
	var envelope struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(result.stderr, &envelope); err != nil {
		t.Fatal(err)
	}
	if result.exit != 2 || len(result.stdout) != 0 || envelope.Code != "unsupported-verify-syntax" {
		t.Fatalf("result=%#v code=%q", result, envelope.Code)
	}
	ledger, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if want := `{"kind":"unsupported","code":"unsupported-verify-syntax","queryIntent":"unknown"}` + "\n"; err != nil || string(ledger) != want {
		t.Fatalf("ledger=%q err=%v", ledger, err)
	}
}

func TestRunEmitsOneCanonicalResponse(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	var stdout, stderr bytes.Buffer
	if code := run(cliArguments(root, "stop"), strings.NewReader(`{"stopHookActive":true}`), &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, &stderr)
	}
	if stderr.Len() != 0 || !bytes.HasSuffix(stdout.Bytes(), []byte{'\n'}) || bytes.Count(stdout.Bytes(), []byte{'\n'}) != 1 {
		t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
	}
	body := bytes.TrimSuffix(stdout.Bytes(), []byte{'\n'})
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	canonical, err := gokernel.CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, canonical) {
		t.Fatalf("noncanonical stdout: %s", body)
	}
	if value["support"] != "FALLBACK" || value["profile"] != gokernel.Profile {
		t.Fatalf("unexpected response: %#v", value)
	}
}

func TestEmitEscapesNonASCIIWithoutChangingReceiptCanonicalization(t *testing.T) {
	t.Parallel()
	value := map[string]any{"text": "café — 🙂"}
	canonical, err := gokernel.CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte(`{"text":"café — 🙂"}`); !bytes.Equal(canonical, want) {
		t.Fatalf("receipt canonical JSON = %q, want %q", canonical, want)
	}
	var stdout bytes.Buffer
	if err := emit(&stdout, value); err != nil {
		t.Fatal(err)
	}
	if want := []byte("{\"text\":\"caf\\u00e9 \\u2014 \\ud83d\\ude42\"}\n"); !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("stdout = %q, want %q", stdout.Bytes(), want)
	}
}

func TestObservationsCommandIsStdoutOnly(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	directory := filepath.Join(root, ".corvint")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(directory, "self-observations.jsonl")
	row := []byte(`{"kind":"event","event":"session-start","latencyMs":4}` + "\n")
	if err := os.WriteFile(ledger, row, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "observations", "--limit", "1"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	after, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || stderr.Len() != 0 || stdout.Len() == 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDogfoodObserveWritesOnlyBoundedStepOutcome(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	arguments := []string{"--root", root, "dogfood-observe", "--step", "cem-status", "--status", "NOT_PRODUCED", "--reason", "not-ready"}
	if code := run(arguments, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &row); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"kind": "dogfood-step", "step": "cem-status", "status": "NOT_PRODUCED", "reason": "not-ready"}
	if len(row) != len(want) {
		t.Fatalf("row carries unexpected fields: %#v", row)
	}
	for key, value := range want {
		if row[key] != value {
			t.Fatalf("%s=%v, want %q", key, row[key], value)
		}
	}
}

func TestDogfoodObserveRejectsUnboundedOrSensitiveFields(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		field string
		value string
	}{
		{name: "path step", field: "--step", value: "script/dogfood-change.sh"},
		{name: "command reason", field: "--reason", value: "make gate"},
		{name: "oversized reason", field: "--reason", value: strings.Repeat("x", 97)},
		{name: "invalid status", field: "--status", value: "FAILED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := cliRepository(t)
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			arguments := []string{"--root", root, "dogfood-observe", "--step", "cem-status", "--status", "PRODUCED", "--reason", "none"}
			for index, argument := range arguments {
				if argument == test.field {
					arguments[index+1] = test.value
				}
			}
			var stdout, stderr bytes.Buffer
			if code := run(arguments, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, &stdout, &stderr)
			}
			if _, err := os.Stat(filepath.Join(root, ".corvint", "self-observations.jsonl")); !os.IsNotExist(err) {
				t.Fatalf("invalid observation wrote ledger: %v", err)
			}
		})
	}
}

func TestStandaloneQueryFailuresDoNotObserveOrMutate(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		argv []string
	}{
		{name: "invalid", argv: []string{"query", "--task", "fix parser", "--limit", "not-an-int"}},
		{name: "out-of-range", argv: []string{"query", "--task", "use codex agent tooling on private task", "--limit", "51"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := cliRepository(t)
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			before := repositoryBytesDigest(t, root)
			var stdout, stderr bytes.Buffer
			arguments := append([]string{"--root", root}, test.argv...)
			if code := run(arguments, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
			if after := repositoryBytesDigest(t, root); before != after {
				t.Fatal("standalone query failure changed repository or Git bytes")
			}
			if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
				t.Fatalf("standalone query wrote observation state: %v", err)
			}
		})
	}
}

func TestNonQueryUnsupportedFailureStillObserves(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "impact", "README.csv"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	ledger, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil || !bytes.Contains(ledger, []byte("unsupported-impact-path-suffix")) {
		t.Fatalf("non-query observation ledger=%s err=%v", ledger, err)
	}
}

// SOL-V0-007: witness, index, and calibrate build the committed
// index, and each records the aggregate-bound refusal exactly once.
func TestIndexBuildingCommandsRecordUnsupportedRefusal(t *testing.T) {
	t.Parallel()
	environment := checkpointGitWrapper(t, `case " $* " in *" ls-tree -r -l -z "*)
 i=0
 while [ "$i" -lt 270 ]; do
  printf '100644 blob 0000000000000000000000000000000000000000 500000\tfile%s.go\000' "$i"
  i=$((i + 1))
 done
 exit 0;; esac`)
	for _, argv := range [][]string{
		{"witness", "--base", ""}, {"index"}, {"calibrate"},
	} {
		t.Run(argv[0], func(t *testing.T) {
			root := cliRepository(t)
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if argv[0] == "witness" {
				argv[2] = strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
			}
			arguments := append([]string{"--root", root}, argv...)
			_, stderr, code := runCandidateWithEnvironment(t, "", environment, arguments...)
			if code != 2 || !strings.Contains(stderr, `"code": "unsupported-impact-repository"`) {
				t.Fatalf("exit=%d stderr=%s", code, stderr)
			}
			ledger, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
			if want := `{"kind":"unsupported","code":"unsupported-impact-repository","queryIntent":"unknown"}` + "\n"; err != nil || string(ledger) != want {
				t.Fatalf("ledger=%q err=%v want %q", ledger, err, want)
			}
		})
	}
}

// user-prompt and file-change are now implemented, so this asserts the surface
// that genuinely remains unimplemented: a compact session start.
func TestRunRejectsInvalidHarnessInputWithExitTwoAndNoStdout(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	var stdout, stderr bytes.Buffer
	code := run(cliArguments(root, "session-start"), strings.NewReader(`{"startSource":"bogus"}`), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, &stdout, &stderr)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["code"] != "invalid-harness-input" || envelope["ok"] != false {
		t.Fatalf("unexpected error: %#v", envelope)
	}
}

// TestFreshProcessCompactStartBytesMatchPythonOracle covers the four rehydration
// branches a compaction can land in. Each dirties the fixture differently, since
// the branch is chosen by the shape of the dirty set rather than by the input.
func TestFreshProcessCompactStartNativeEnvelopes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		dirty func(t *testing.T, root string)
	}{
		{"clean", func(*testing.T, string) {}},
		{"tracked", func(t *testing.T, root string) {
			writeFixtureFile(t, root, "pkg/sample.go", "package sample\n\n// edited\n")
		}},
		{"untracked-only", func(t *testing.T, root string) {
			writeFixtureFile(t, root, "extra.md", "# extra\n")
		}},
		{"mixed", func(t *testing.T, root string) {
			writeFixtureFile(t, root, "pkg/sample.go", "package sample\n\n// edited\n")
			writeFixtureFile(t, root, "extra.md", "# extra\n")
		}},
		{"over-budget", func(t *testing.T, root string) {
			for index := 0; index < maxCompactionPaths+1; index++ {
				writeFixtureFile(t, root, fmt.Sprintf("bulk-%03d.md", index), "# bulk\n")
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := cliGoModuleRepository(t)
			test.dirty(t, root)
			arguments := cliArguments(root, "session-start")
			candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
			candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
			candidate.Stdin = strings.NewReader(`{"startSource":"compact"}`)
			candidateResult := execute(t, candidate)

			if candidateResult.exit != 0 || len(candidateResult.stderr) != 0 ||
				!bytes.Contains(candidateResult.stdout, []byte(`"event":"session-start"`)) {
				t.Fatalf("compact result=%#v", candidateResult)
			}
		})
	}
}

// cliGoModuleRepository adds the slash-qualified module that the Go impact
// profile requires, so the rehydration branches that run impact are inside it.
func cliGoModuleRepository(t *testing.T) string {
	t.Helper()
	root := cliRepository(t)
	writeFixtureFile(t, root, "go.mod", "module example.test/compact\n\ngo 1.27.0\n")
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, root, "pkg/sample.go", "package sample\n")
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-qm", "module"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func writeFixtureFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunContextCancelsAnOpenStdinPipe(t *testing.T) {
	root := cliRepository(t)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan int, 1)
	var stdout, stderr bytes.Buffer
	go func() {
		result <- runContext(ctx, cliArguments(root, "stop"), reader, &stdout, &stderr)
	}()
	cancel()
	select {
	case code := <-result:
		if code != 2 || !bytes.Contains(stderr.Bytes(), []byte(`"code": "harness-input-cancelled"`)) {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, &stdout, &stderr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled stdin read did not terminate")
	}
}

func TestRunUsesLastRepeatedArgumentAndRejectsOversizeInput(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	t.Run("arguments", func(t *testing.T) {
		arguments := append(cliArguments(root, "session-start"), "--event", "stop")
		var stdout, stderr bytes.Buffer
		if code := run(arguments, strings.NewReader(`{}`), &stdout, &stderr); code != 0 {
			t.Fatalf("exit=%d stderr=%q", code, &stderr)
		}
		if !bytes.Contains(stdout.Bytes(), []byte(`"event":"stop"`)) || stderr.Len() != 0 {
			t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
		}
	})
	t.Run("input", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		input := bytes.NewReader(bytes.Repeat([]byte{' '}, gokernel.MaxInputBytes+1))
		if code := run(cliArguments(root, "stop"), input, &stdout, &stderr); code != 2 {
			t.Fatalf("exit=%d stderr=%q", code, &stderr)
		}
		if !bytes.Contains(stderr.Bytes(), []byte(`"code": "harness-input-too-large"`)) {
			t.Fatalf("stderr=%q", &stderr)
		}
	})
}

func TestFreshProcessSuccessNativeEnvelopes(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	digest := strings.Repeat("a", 64)
	cases := []struct {
		event, input string
	}{
		{"session-start", `{}`},
		{"post-tool", `{"changedPaths":["README.md"]}`},
		{"stop", `{"stopHookActive":true}`},
		{"session-end", `{"changedPaths":["README.md"],"outcome":"passed","taskSha256":"` + digest + `","verification":[{"commandSha256":"` + digest + `","status":"passed"}]}`},
	}
	for _, test := range cases {
		t.Run(test.event, func(t *testing.T) {
			arguments := cliArguments(root, test.event)
			candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
			candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
			candidate.Stdin = strings.NewReader(test.input)
			candidateResult := execute(t, candidate)

			if candidateResult.exit != 0 || len(candidateResult.stderr) != 0 ||
				!bytes.Contains(candidateResult.stdout, []byte(`"event":"`+test.event+`"`)) {
				t.Fatalf("fresh-process result=%#v", candidateResult)
			}
		})
	}
}

func testFreshProcessCLICompatibilityEdgesHaveStableNativeResults(t *testing.T) {
	root := cliRepository(t)
	deepInput := `{"unknown":` + strings.Repeat("[", 260) + strings.Repeat("]", 260) + `}`
	deepDuplicate := `{"unknown":` + strings.Repeat("[", 257) + `{"x":1,"x":2}` + strings.Repeat("]", 257) + `}`
	veryDeep := strings.Repeat("[", 2_000) + strings.Repeat("]", 2_000)
	cases := []struct {
		name  string
		args  []string
		input string
		cwd   string
		// wantMessage, when set, is the exact stderr envelope "error" text a
		// boundary/precedence case must produce: proof that a specific check
		// fired first, not just that the process exited non-zero (GOC-V0-003).
		wantMessage string
	}{
		{
			name: "semantic-error",
			args: cliArguments(root, "stop"), input: `{"stopHookActive":1}`,
		},
		{
			name: "repeated-last-wins",
			args: append(cliArguments(root, "session-start"), "--event", "stop"), input: `{}`,
		},
		{
			name: "default-root",
			args: cliArguments(root, "session-start")[2:], input: `{}`, cwd: root,
		},
		{
			name: "version",
			args: []string{"--version"}, input: `{}`,
		},
		{
			name: "equals-options",
			args: []string{
				"--root=" + root, "harness", "event", "--host=codex",
				"--host-version=unknown", "--surface=plugin", "--adapter-version=0.1.0",
				"--event=stop", "--input=-", "--budget-bytes=8000",
			},
			input: `{}`,
		},
		{
			name: "unhashable-start-source",
			args: cliArguments(root, "session-start"), input: `{"startSource":[]}`,
		},
		{
			name: "unhashable-outcome",
			args: cliArguments(root, "session-end"), input: `{"outcome":[]}`,
		},
		{
			name:  "unhashable-verification-status",
			args:  cliArguments(root, "post-tool"),
			input: `{"verification":[{"commandSha256":"` + strings.Repeat("a", 64) + `","status":[]}]}`,
		},
		{
			name: "malformed-nan-key",
			args: cliArguments(root, "post-tool"), input: `{NaN:1}`,
		},
		{
			name: "malformed-nan-array",
			args: cliArguments(root, "post-tool"), input: `[NaN:1]`,
		},
		{
			name: "malformed-nan-key-and-value",
			args: cliArguments(root, "post-tool"), input: `{NaN:NaN}`,
		},
		{
			name: "semantic-field-with-unpaired-surrogate",
			args: cliArguments(root, "session-start"), input: `{"startSource":"\ud800"}`,
		},
		{
			name: "bounded-nesting",
			args: cliArguments(root, "post-tool"), input: deepInput,
			wantMessage: "harness input exceeds its nesting limit",
		},
		{
			name: "nesting-precedes-duplicate-key",
			args: cliArguments(root, "post-tool"), input: deepDuplicate,
			wantMessage: "harness input exceeds its nesting limit",
		},
		{
			name: "nesting-precedes-parser-recursion",
			args: cliArguments(root, "post-tool"), input: veryDeep,
			wantMessage: "harness input exceeds its nesting limit",
		},
		{
			name: "utf8-precedes-invalid-constant",
			args: cliArguments(root, "post-tool"), input: "{\"x\":NaN}\xff",
			wantMessage: "harness input must be one JSON object",
		},
		{
			name: "utf8-precedes-unpaired-surrogate",
			args: cliArguments(root, "post-tool"), input: "{\"x\":\"\\ud800\"}\xff",
			wantMessage: "harness input must be one JSON object",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, test.args...)...)
			candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
			candidate.Stdin = strings.NewReader(test.input)
			candidate.Dir = test.cwd
			candidateResult := execute(t, candidate)

			if candidateResult.exit != 0 && candidateResult.exit != 2 {
				t.Fatalf("unexpected exit: %#v", candidateResult)
			}
			if (candidateResult.exit == 0) == (len(candidateResult.stdout) == 0) {
				t.Fatalf("invalid success/error stream shape: %#v", candidateResult)
			}
			if test.wantMessage != "" {
				var envelope struct {
					Error string `json:"error"`
				}
				if candidateResult.exit != 2 {
					t.Fatalf("boundary case must be refused: %#v", candidateResult)
				}
				if err := json.Unmarshal(candidateResult.stderr, &envelope); err != nil {
					t.Fatalf("stderr envelope: %v\n%s", err, candidateResult.stderr)
				}
				if envelope.Error != test.wantMessage {
					t.Fatalf("error precedence: got %q, want %q", envelope.Error, test.wantMessage)
				}
			}
		})
	}
}

func TestRepositoryFailureEnvelopeMatchesPythonShape(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	emitError(&stderr, &gokernel.Error{Code: "repository-probe-failed", Message: "Git error: canary failure"})
	want := "{\"error\": \"Git error: canary failure\", \"ok\": false}\n"
	if stderr.String() != want {
		t.Fatalf("stderr=%q, want %q", stderr.String(), want)
	}
}

func TestFreshProcessHostileGitFailuresReturnNativeErrors(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX executable script")
	}
	root := cliRepository(t)
	bin := t.TempDir()
	fakeGit := filepath.Join(bin, "git")
	cases := []struct{ name, script string }{
		{"no-stderr-exit", "#!/bin/sh\nexit 7\n"},
		{
			"empty-object-format",
			"#!/bin/sh\nprintf '\\n%s\\n%s\\n' '" + strings.Repeat("a", 40) + "' '" + strings.Repeat("b", 40) + "'\n",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(fakeGit, []byte(test.script), 0o755); err != nil {
				t.Fatal(err)
			}
			arguments := cliArguments(root, "session-start")
			environment := append(os.Environ(), "PATH="+bin)
			candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
			candidate.Env = append(environment, "CORVINT_HELPER_PROCESS=1")
			candidate.Stdin = strings.NewReader(`{}`)
			candidateResult := execute(t, candidate)

			if candidateResult.exit != 2 || len(candidateResult.stdout) != 0 || len(candidateResult.stderr) == 0 {
				t.Fatalf("hostile Git result=%#v", candidateResult)
			}
		})
	}
}

// The invalid-choice list is the only place an agent learns which verbs exist,
// so it must name every verb runContext dispatches and every verb rootHelp
// documents.
func TestInvalidChoiceNamesEveryDispatchedTopLevelVerb(t *testing.T) {
	t.Parallel()
	// GPK-V0-059: the migration base's argparse verbs lead, and no verb repeats.
	oracleVerbs := []string{"init", "adopt", "query", "feature", "impact", "eval", "lrf",
		"record", "migrate-traces", "harness", "cem", "ocm", "work"}
	if !slices.Equal(topLevelCommands[:len(oracleVerbs)], oracleVerbs) {
		t.Fatalf("leading verbs = %q, want %q", topLevelCommands[:len(oracleVerbs)], oracleVerbs)
	}
	seen := map[string]bool{}
	for _, command := range topLevelCommands {
		if seen[command] {
			t.Fatalf("verb %q listed twice", command)
		}
		seen[command] = true
	}
	for _, command := range topLevelCommands {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			arguments := []string{"--root", "/definitely/not/a/repository", command}
			if command == "adapter" {
				arguments = []string{"adapter", "unsupported"}
			}
			run(arguments, strings.NewReader("{}"), &stdout, &stderr)
			if strings.Contains(stderr.String(), "invalid choice") {
				t.Fatalf("listed verb %q is not dispatched: stderr=%s", command, &stderr)
			}
		})
	}
	message := invalidChoice("command", "version", topLevelCommands).Error()
	documented, _, _ := strings.Cut(strings.SplitN(rootHelp, "\nCommands:\n", 2)[1], "\nGlobal options:")
	documentedCommands := map[string]bool{}
	for _, line := range strings.Split(documented, "\n") {
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
			continue
		}
		command := strings.Fields(line)[0]
		documentedCommands[command] = true
		if !strings.Contains(message, "'"+command+"'") {
			t.Fatalf("documented command %q is missing from %q", command, message)
		}
	}
	for _, command := range topLevelCommands {
		if !documentedCommands[command] {
			t.Errorf("dispatched command %q is absent from root help", command)
		}
		topic, ok := publicHelpTopic(command)
		if !ok || helpText(topic) == "" {
			t.Errorf("dispatched command %q has no help topic", command)
		}
	}
}

func TestFreshProcessCLICompatibilityEdgesHaveStableNativeResults(t *testing.T) {
	t.Parallel()
	t.Run("GOC-V0-003 independent CLI boundary expectations", testFreshProcessCLICompatibilityEdgesHaveStableNativeResults)
}
