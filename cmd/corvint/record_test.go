package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// GPK-V0-001, GPK-V0-002, GPK-V0-005, GPK-V0-008.
func TestRecordSuccessMatchesPythonOracleBytesAndStore(t *testing.T) {
	t.Parallel()
	fixture := filepath.Join(t.TempDir(), "repository")
	fixture = newRecordFixtureAt(t, fixture)
	arguments := recordArguments(fixture, " explicit record ", "passed")
	candidate := execute(t, candidateCommand(arguments...))
	if candidate.exit != 0 || len(candidate.stderr) != 0 || !bytes.Contains(candidate.stdout, []byte(`"outcome":"passed"`)) {
		t.Fatalf("candidate=%#v", candidate)
	}
	candidateState := traceTreeSnapshot(t, fixture)
	if len(candidateState) == 0 {
		t.Fatal("record produced no private trace state")
	}
}

// GPK-V0-001, GPK-V0-002, GPK-V0-004.
func TestRecordRequiredArgumentErrorsMatchPythonOracle(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "repository")
	root = newRecordFixtureAt(t, root)
	tests := []struct {
		name string
		args []string
	}{
		{"missing changed", []string{"--root", root, "record", "--task", "task", "--verify", "go test ./...", "--outcome", "passed"}},
		{"missing verify", []string{"--root", root, "record", "--task", "task", "--changed", "internal/example/value.go", "--outcome", "passed"}},
		{"missing task", []string{"--root", root, "record", "--changed", "internal/example/value.go", "--verify", "go test ./...", "--outcome", "passed"}},
		{"missing outcome", []string{"--root", root, "record", "--task", "task", "--changed", "internal/example/value.go", "--verify", "go test ./..."}},
		{"invalid outcome", []string{"--root", root, "record", "--task", "task", "--changed", "internal/example/value.go", "--verify", "go test ./...", "--outcome", "nope"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := execute(t, candidateCommand(test.args...))
			if candidate.exit != 2 || len(candidate.stdout) != 0 || len(candidate.stderr) == 0 {
				t.Fatalf("candidate=%#v", candidate)
			}
		})
	}
}

// GPK-V0-002, GPK-V0-004, GPK-V0-008.
func TestRecordValidationErrorsMatchOracleAndWriteNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args func(string) []string
	}{
		{"empty task", func(root string) []string { return recordArguments(root, " ", "passed") }},
		{"untracked changed path", func(root string) []string {
			return []string{"--root", root, "record", "--task", "task", "--changed", "missing.go", "--verify", "go test ./...", "--outcome", "passed"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := filepath.Join(t.TempDir(), "repository")
			fixture = newRecordFixtureAt(t, fixture)
			arguments := test.args(fixture)
			candidate := execute(t, candidateCommand(arguments...))
			if candidate.exit != 2 || len(candidate.stdout) != 0 {
				t.Fatalf("candidate=%#v", candidate)
			}
			if state := traceTreeSnapshot(t, fixture); len(state) != 0 {
				t.Fatalf("failed validation mutated trace state: %#v", state)
			}
		})
	}
}

// GPK-V0-002, GPK-V0-004, GPK-V0-008.
func TestRecordExistingStoreRefusalsMatchPythonOracle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data func(string) []byte
	}{
		{"malformed JSON", func(string) []byte { return []byte("{\n") }},
		{"invalid encoding", func(string) []byte { return []byte{0xff, '\n'} }},
		{"stored untracked path", func(revision string) []byte {
			return []byte(fmt.Sprintf("{\"changed_paths\":[\"missing.go\"],\"opened_paths\":[],\"outcome\":\"passed\",\"revision\":%q,\"schema_version\":1,\"task\":\"task\",\"trace_id\":%q,\"verification\":[]}\n", revision, strings.Repeat("0", 64)))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := filepath.Join(t.TempDir(), "repository")
			fixture = newRecordFixtureAt(t, fixture)
			writeRecordStore(t, fixture, test.data(recordRevision(t, fixture)))
			before := traceTreeSnapshot(t, fixture)
			arguments := recordArguments(fixture, "task", "passed")
			candidate := execute(t, candidateCommand(arguments...))
			if candidate.exit != 2 || len(candidate.stdout) != 0 {
				t.Fatalf("candidate=%#v", candidate)
			}
			if after := traceTreeSnapshot(t, fixture); !reflect.DeepEqual(after, before) {
				t.Fatalf("validation mutated candidate store\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}
}

// GPK-V0-002, GPK-V0-004.
func TestRecordRuntimeValidationPrecedenceMatchesPythonOracle(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "repository")
	root = newRecordFixtureAt(t, root)
	arguments := []string{
		"--root", root, "record", "--task", " ", "--changed", "missing.go",
		"--verify", "go test; false", "--outcome", "passed",
	}
	candidate := execute(t, candidateCommand(arguments...))
	if candidate.exit != 2 || len(candidate.stdout) != 0 || !bytes.Contains(candidate.stderr, []byte("verification command contains unsupported shell syntax")) {
		t.Fatalf("candidate=%#v", candidate)
	}
}

// LTPM-V0-011, SOL-V0-007: the verification-syntax refusal names its code on
// the wire and appends exactly one ledger row.
func TestRecordUnsupportedVerifySyntaxAppendsOneObservation(t *testing.T) {
	t.Parallel()
	root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repository"))
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", ".gitignore"), []byte("*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, "--root", root, "record", "--task", "task", "--changed",
		"internal/example/value.go", "--verify", "go test; false", "--outcome", "passed")
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, `{"code": "unsupported-verify-syntax", "error": `) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	ledger, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if want := `{"kind":"unsupported","code":"unsupported-verify-syntax","queryIntent":"unknown"}` + "\n"; err != nil || string(ledger) != want {
		t.Fatalf("ledger=%q err=%v", ledger, err)
	}
}

// GPK-V0-002, GPK-V0-004, GPK-V0-008.
func TestRecordGitErrorsMatchPythonOracleAndWriteNothing(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX executable script")
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name            string
		candidateScript string
		oracleScript    string
	}{
		{"index identity stderr", recordGitFailureScript(realGit, "rev-parse", "  identity first\\nidentity second  ", 7), ""},
		{"index identity empty stderr", recordGitFailureScript(realGit, "rev-parse", "", 7), ""},
		{"index identity start", "#!/missing-git-interpreter\n", recordGitIndexIdentityStartScript(realGit)},
		{"status start", recordGitStatusStartScript(realGit), ""},
		{"status stderr", recordGitFailureScript(realGit, "status", "  status first\\nstatus second  ", 7), ""},
		{"status Python whitespace", recordGitFailureScript(realGit, "status", "\x1cstatus detail\x1f", 7), ""},
		{"status empty stderr", recordGitFailureScript(realGit, "status", "", 7), ""},
		{"historical log start", recordGitHistoryStartScript(realGit), ""},
		{"historical log stderr", recordGitFailureScript(realGit, "log", "  history first\\nhistory second  ", 7), ""},
		{"historical log Python whitespace", recordGitFailureScript(realGit, "log", "\x1chistory detail\x1f", 7), ""},
		{"historical log empty stderr", recordGitFailureScript(realGit, "log", "", 7), ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := filepath.Join(t.TempDir(), "repository")
			fixture = newRecordFixtureAt(t, fixture)
			if strings.HasPrefix(test.name, "historical log") {
				writeRecordStore(t, fixture, nil)
			}
			before := traceTreeSnapshot(t, fixture)
			candidateBin := t.TempDir()
			writeExecutable(t, filepath.Join(candidateBin, "git"), test.candidateScript)
			arguments := recordArguments(fixture, "task", "passed")
			candidateCommand := candidateCommand(arguments...)
			candidateCommand.Env = append(candidateCommand.Env, "PATH="+candidateBin)
			candidate := execute(t, candidateCommand)
			if candidate.exit != 2 || len(candidate.stdout) != 0 {
				t.Fatalf("candidate=%#v", candidate)
			}
			if after := traceTreeSnapshot(t, fixture); !reflect.DeepEqual(after, before) {
				t.Fatalf("failed Git operation mutated candidate trace state: before=%#v after=%#v", before, after)
			}

		})
	}
}

// GPK-V0-004, GPK-V0-008.
func TestRecordRejectsSymlinkedTraceRootWithoutOutsideWrite(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "repository")
	root = newRecordFixtureAt(t, root)
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".context-corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".context-corvint", "traces")); err != nil {
		t.Fatal(err)
	}
	result := execute(t, candidateCommand(recordArguments(root, "task", "passed")...))
	if result.exit != 2 || len(result.stdout) != 0 || !bytes.Contains(result.stderr, []byte("unsafe local trace store")) {
		t.Fatalf("result=%#v", result)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside entries=%v error=%v", entries, err)
	}
}

func recordArguments(root, task, outcome string) []string {
	return []string{
		"--root", root, "record", "--task", task,
		"--opened", "internal/example/value.go", "--opened", "go.mod",
		"--changed", "internal/example/value.go",
		"--verify", "go test ./...", "--verify", "git diff --check", "--outcome", outcome,
	}
}

func newRecordFixtureAt(t *testing.T, root string) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(filepath.Dir(root))
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(parent, filepath.Base(root))
	if err := os.MkdirAll(filepath.Join(root, "internal", "example"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{
		".gitignore":                ".context-corvint/\n",
		"go.mod":                    "module example.test/record\n\ngo 1.27.0\n",
		"internal/example/value.go": "package example\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"add", "."}, {"commit", "-qm", "initial"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != root {
		t.Fatalf("fixture root=%q resolved=%q error=%v", root, resolved, err)
	}
	return root
}

func recordRevision(t *testing.T, root string) string {
	t.Helper()
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

func writeRecordStore(t *testing.T, root string, data []byte) {
	t.Helper()
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("%s.jsonl", recordRevision(t, root))), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path, data string) {
	t.Helper()
	for _, suffix := range []string{".counter", ".identities", ".statuses", ".lock"} {
		if err := os.Remove(path + suffix); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte(data), 0o755); err != nil {
		t.Fatal(err)
	}
}

func recordGitIndexIdentityStartScript(realGit string) string {
	return fmt.Sprintf(`#!/bin/sh
identity=
for argument in "$@"; do
  if [ "$argument" = "rev-parse" ]; then identity=1; fi
done
if [ -n "$identity" ] && [ -n "${GIT_CONFIG_COUNT-}" ]; then
  count=0
  if [ -f "$0.counter" ]; then IFS= read -r count < "$0.counter"; fi
  count=$((count + 1))
  printf '%%s\n' "$count" > "$0.counter"
  if [ "$count" -eq 2 ]; then /bin/rm -- "$0"; fi
fi
exec %s "$@"
`, shellQuote(realGit))
}

func recordGitHistoryStartScript(realGit string) string {
	return fmt.Sprintf(`#!/bin/sh
direct=
if [ "$1" = "-C" ]; then direct=1; fi
identity=
status=
for argument in "$@"; do
  if [ "$argument" = "rev-parse" ]; then identity=1; fi
  if [ "$argument" = "status" ]; then status=1; fi
done
relevant=
if [ -z "$direct" ]; then
  if [ -n "${GIT_CONFIG_COUNT-}" ]; then
    if [ -n "$identity" ]; then relevant=1; fi
  else
    if [ -n "$identity$status" ]; then relevant=1; fi
  fi
fi
if [ -z "$relevant" ]; then
  exec %s "$@"
fi
while ! /bin/mkdir "$0.lock" 2>/dev/null; do /bin/sleep 0.01; done
trap '/bin/rmdir "$0.lock" 2>/dev/null' EXIT HUP INT TERM
%s "$@"
result=$?
identities=0
if [ -f "$0.identities" ]; then IFS= read -r identities < "$0.identities"; fi
statuses=0
if [ -f "$0.statuses" ]; then IFS= read -r statuses < "$0.statuses"; fi
if [ -n "$identity" ]; then
  identities=$((identities + 1))
  printf '%%s\n' "$identities" > "$0.identities"
fi
if [ -n "$status" ]; then
  statuses=$((statuses + 1))
  printf '%%s\n' "$statuses" > "$0.statuses"
fi
if [ -n "${GIT_CONFIG_COUNT-}" ]; then
  if [ "${identities-0}" -eq 4 ]; then /bin/rm -- "$0"; fi
else
  if [ "${identities-0}" -eq 2 ] && [ "${statuses-0}" -eq 2 ]; then /bin/rm -- "$0"; fi
fi
/bin/rmdir "$0.lock"
trap - EXIT HUP INT TERM
exit "$result"
`, shellQuote(realGit), shellQuote(realGit))
}

func recordGitFailureScript(realGit, operation, stderr string, exit int) string {
	return fmt.Sprintf(`#!/bin/sh
direct=
if [ "$1" = "-C" ]; then direct=1; fi
matched=
for argument in "$@"; do
  if [ "$argument" = %s ]; then matched=1; fi
done
if [ -n "$matched" ] && { [ -n "$direct" ] || [ -z "${GIT_CONFIG_COUNT-}" ]; }; then
  printf %s >&2
  exit %d
fi
exec %s "$@"
`, shellQuote(operation), shellQuote(stderr), exit, shellQuote(realGit))
}

func recordGitStatusStartScript(realGit string) string {
	return fmt.Sprintf(`#!/bin/sh
direct=
if [ "$1" = "-C" ]; then direct=1; fi
identity=
for argument in "$@"; do
  if [ "$argument" = "rev-parse" ]; then identity=1; fi
done
if [ -n "$identity" ] && { [ -n "$direct" ] || [ -z "${GIT_CONFIG_COUNT-}" ]; }; then
  /bin/rm -- "$0"
fi
exec %s "$@"
`, shellQuote(realGit))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
