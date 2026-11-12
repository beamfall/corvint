package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/execfile"
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
		command := fixtureGitCommand(arguments...)
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
	os.Exit(runContext(context.Background(), os.Args[separator:], os.Stdin, os.Stdout, os.Stderr))
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
	stdout, stderr, code, err := runCandidateCapture(stdin, overrides, arguments...)
	if err != nil {
		t.Fatal(err)
	}
	return stdout, stderr, code
}

// runCandidateCapture is runCandidateWithEnvironment without a *testing.T, so
// a worker goroutine can run it: err is set only when the process could not
// run or report an exit code.
func runCandidateCapture(stdin string, overrides []string, arguments ...string) ([]byte, string, int, error) {
	command := candidateCommand(arguments...)
	command.Env = testEnvironment(append([]string{"CORVINT_HELPER_PROCESS=1"}, overrides...)...)
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if err == nil {
		return stdout.Bytes(), stderr.String(), 0, nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return stdout.Bytes(), stderr.String(), 0, err
	}
	return stdout.Bytes(), stderr.String(), exit.ExitCode(), nil
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate module root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
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

// SOL-V0-007 and IDX-SNAP-V0-910: index builds the
// committed index, and a tree listing 135,000,000 source bytes is no longer an
// aggregate-bound refusal: the build reaches the (here missing) blobs and fails
// as an ordinary Git read, and the unsupported ledger records nothing.
func TestIndexBuildingCommandsDoNotRefuseAggregateSize_IDXSNAPV0910(t *testing.T) {
	t.Parallel()
	environment := checkpointGitWrapper(t, `case " $* " in *" ls-tree -r -l -z "*)
 i=0
 while [ "$i" -lt 270 ]; do
  printf '100644 blob 0000000000000000000000000000000000000000 500000\tfile%s.go\000' "$i"
  i=$((i + 1))
 done
 exit 0;; esac`)
	for _, argv := range [][]string{
		{"index"},
	} {
		t.Run(argv[0], func(t *testing.T) {
			root := cliRepository(t)
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			arguments := append([]string{"--root", root}, argv...)
			_, stderr, code := runCandidateWithEnvironment(t, "", environment, arguments...)
			if code != 2 || strings.Contains(stderr, "unsupported-") || strings.Contains(stderr, "aggregate bound") {
				t.Fatalf("exit=%d stderr=%s", code, stderr)
			}
			if ledger, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl")); bytes.Contains(ledger, []byte(`"kind":"unsupported"`)) {
				t.Fatalf("ledger=%q err=%v", ledger, err)
			}
		})
	}
}

// checkpointGitWrapper returns an environment whose PATH runs body before the real git.
func checkpointGitWrapper(t *testing.T, body string) []string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	wrapper := "#!/bin/sh\n" + body + "\nexec " + shellQuote(realGit) + " \"$@\"\n"
	if err := execfile.WriteFile(filepath.Join(directory, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + directory + string(os.PathListSeparator) + os.Getenv("PATH")}
}

func writeFixtureFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// CCF-V1-004 (accepted 2026-09-26, decision 0422; from decision 0398): a repository or
// object-format refusal keeps the oracle's message and adds its optional code.
func TestRepositoryFailureEnvelopeCarriesItsCode(t *testing.T) {
	t.Parallel()
	for _, failure := range []*gokernel.Error{
		{Code: "repository-probe-failed", Message: "Git error: canary failure"},
		{Code: "unsupported-git-object-format", Message: "unsupported Git object format: sha256"},
	} {
		var stderr bytes.Buffer
		emitError(&stderr, failure)
		want := "{\"code\": \"" + failure.Code + "\", \"error\": \"" + failure.Message + "\", \"ok\": false}\n"
		if stderr.String() != want {
			t.Fatalf("stderr=%q, want %q", stderr.String(), want)
		}
	}
}

// The invalid-choice list is the only place an agent learns which verbs exist,
// so it must name every verb runContext dispatches and every verb rootHelp
// documents.
func TestInvalidChoiceNamesEveryDispatchedTopLevelVerb(t *testing.T) {
	t.Parallel()
	// GPK-V0-059: the migration base's argparse verbs lead, and no verb repeats.
	oracleVerbs := []string{"init", "adopt", "impact"}
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
		if !strings.HasPrefix(line, "    ") {
			continue
		}
		names, _, _ := strings.Cut(strings.TrimSpace(line), "  ")
		for _, command := range strings.Split(strings.TrimSuffix(names, " event"), " | ") {
			documentedCommands[command] = true
			if !strings.Contains(message, "'"+command+"'") {
				t.Fatalf("documented command %q is missing from %q", command, message)
			}
		}
	}
	for command := range hiddenCommands {
		documentedCommands[command] = true
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

// DRC-V0-910 (V1-2585): the refusal for a Git that cannot start keeps its
// exact error bytes and adds only the closed start class.
func TestEmitErrorNamesTheGitStartClass_DRCV0910(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	_, err := contextindex.BuildWithGitExecution(context.Background(), root, filepath.Join(t.TempDir(), "missing-git"), os.Environ())
	if err == nil {
		t.Fatal("a missing Git started")
	}
	var stderr bytes.Buffer
	emitError(&stderr, err)
	if want := "{\"error\": \"Git error: cannot start Git\", \"git_start_error\": \"ENOENT\", \"ok\": false}\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	stderr.Reset()
	emitError(&stderr, &contextindex.Error{Message: "Git error: exit status 3"})
	if want := "{\"error\": \"Git error: exit status 3\", \"ok\": false}\n"; stderr.String() != want {
		t.Fatalf("an unrelated envelope changed: %q", stderr.String())
	}
}
