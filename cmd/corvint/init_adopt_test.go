package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// GPK-V0-001..007, GPK-V0-032.
func TestInitAdoptReceiptModesMatchPythonOracleAndReadNothing(t *testing.T) {
	t.Parallel()
	root := newActivationFixture(t, "sha1")
	for _, activation := range []string{"init", "adopt"} {
		for _, full := range []bool{false, true} {
			name := activation + "-summary"
			if full {
				name = activation + "-full"
			}
			t.Run(name, func(t *testing.T) {
				arguments := activationArguments(root, activation, full)
				before := repositoryBytesDigest(t, root)
				candidate := execute(t, candidateCommand(arguments...))
				after := repositoryBytesDigest(t, root)
				if before != after {
					t.Fatal("activation changed repository or Git bytes")
				}
				if candidate.exit != 0 || len(candidate.stderr) != 0 {
					t.Fatalf("candidate=%#v", candidate)
				}
			})
		}
	}
}

// TestInitAdoptInterruptedRunLeavesNoStateAndRetriesCleanly is V1-0113: init
// and adopt write nothing (`mutates: false`), so a run killed at any point
// leaves the repository and Git bytes unchanged, and the next run answers
// exactly as an uninterrupted one. The kill delays span start-up to late in
// the run; the property holds whichever step each one lands in.
func TestInitAdoptInterruptedRunLeavesNoStateAndRetriesCleanly(t *testing.T) {
	t.Parallel()
	root := newActivationFixture(t, "sha1")
	for _, activation := range []string{"init", "adopt"} {
		arguments := activationArguments(root, activation, true)
		clean := execute(t, candidateCommand(arguments...))
		if clean.exit != 0 {
			t.Fatalf("%s: clean run %#v", activation, clean)
		}
		before := repositoryBytesDigest(t, root)
		for _, delay := range []time.Duration{0, 2 * time.Millisecond, 10 * time.Millisecond, 50 * time.Millisecond, 200 * time.Millisecond} {
			interrupted := candidateCommand(arguments...)
			if err := interrupted.Start(); err != nil {
				t.Fatal(err)
			}
			time.Sleep(delay)
			_ = interrupted.Process.Kill()
			_ = interrupted.Wait()
			if repositoryBytesDigest(t, root) != before {
				t.Fatalf("%s killed after %s changed repository or Git bytes", activation, delay)
			}
			retry := execute(t, candidateCommand(arguments...))
			if retry.exit != clean.exit || !bytes.Equal(retry.stdout, clean.stdout) || len(retry.stderr) != 0 {
				t.Fatalf("%s retry after a kill at %s differs from a clean run: %#v", activation, delay, retry)
			}
		}
	}
}

// GPK-V0-002, GPK-V0-004, GPK-V0-032.
func TestInitAdoptInvalidReceiptsMatchPythonExitOne(t *testing.T) {
	t.Parallel()
	root := newActivationFixture(t, "sha1")
	tests := []struct {
		name string
		args func(string) []string
	}{
		{"invalid authority", func(activation string) []string {
			return []string{"--root", root, activation, "--authority-id", "@invalid"}
		}},
		{"duplicate exclusion", func(activation string) []string {
			return []string{"--root", root, activation, "--exclude-prefix", "vendor", "--exclude-prefix", "vendor"}
		}},
		{"missing revision", func(activation string) []string { return []string{"--root", root, activation, "--revision", "HEAD~99"} }},
	}
	for _, activation := range []string{"init", "adopt"} {
		for _, receiptFlag := range [][]string{nil, {"--full-receipt"}} {
			for _, test := range tests {
				t.Run(activation+"-"+test.name+strings.Join(receiptFlag, ""), func(t *testing.T) {
					arguments := append(test.args(activation), receiptFlag...)
					candidate := execute(t, candidateCommand(arguments...))
					if candidate.exit != 1 || len(candidate.stderr) != 0 || !bytes.Contains(candidate.stdout, []byte(`"ok":false`)) || !bytes.Contains(candidate.stdout, []byte(`"operationalState":"INVALID"`)) {
						t.Fatalf("candidate=%#v", candidate)
					}
				})
			}
		}
	}
}

// GPK-V0-002, GPK-V0-006, GPK-V0-032.
func TestInitAdoptMixedNonHeadRevisionMatchesPython(t *testing.T) {
	t.Parallel()
	root := newActivationFixture(t, "sha1")
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("mixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, activation := range []string{"init", "adopt"} {
		arguments := activationArguments(root, activation, false)
		candidate := execute(t, candidateCommand(arguments...))
		if !bytes.Contains(candidate.stdout, []byte(`"dirtyState":"DIRTY"`)) {
			t.Fatalf("stdout=%s", candidate.stdout)
		}
	}
}

// GPK-V0-002, GPK-V0-032.
func TestInitAdoptOutputIsIndependentOfRepositoryLocation(t *testing.T) {
	t.Parallel()
	firstRoot := newActivationFixture(t, "sha1")
	secondRoot := filepath.Join(t.TempDir(), "different", "root")
	if err := os.MkdirAll(filepath.Dir(secondRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, "", nil, "clone", "-q", firstRoot, secondRoot)
	for _, activation := range []string{"init", "adopt"} {
		for _, full := range []bool{false, true} {
			firstArgs := activationArguments(firstRoot, activation, full)
			secondArgs := activationArguments(secondRoot, activation, full)
			first := execute(t, candidateCommand(firstArgs...))
			second := execute(t, candidateCommand(secondArgs...))
			if first.exit != second.exit || !bytes.Equal(first.stdout, second.stdout) || !bytes.Equal(first.stderr, second.stderr) {
				t.Fatalf("location-dependent output\nfirst=%q\nsecond=%q", first.stdout, second.stdout)
			}
			if bytes.Contains(first.stdout, []byte(firstRoot)) || bytes.Contains(second.stdout, []byte(secondRoot)) {
				t.Fatal("activation stdout contains an absolute repository root")
			}
		}
	}
}

// GPK-V0-005, GPK-V0-032.
func TestInitAdoptSHA256AndLinkedWorktreeMatchPython(t *testing.T) {
	t.Parallel()
	sha256Root := newActivationFixture(t, "sha256")
	for _, activation := range []string{"init", "adopt"} {
		arguments := activationArguments(sha256Root, activation, false)
		result := execute(t, candidateCommand(arguments...))
		if result.exit != 0 || len(result.stderr) != 0 || len(result.stdout) == 0 {
			t.Fatalf("%s sha256 result=%#v", activation, result)
		}
	}

	root := newActivationFixture(t, "sha1")
	linked := filepath.Join(t.TempDir(), "linked")
	gitCommand(t, root, nil, "worktree", "add", "-q", "--detach", linked, "HEAD")
	for _, activation := range []string{"init", "adopt"} {
		arguments := activationArguments(linked, activation, true)
		result := execute(t, candidateCommand(arguments...))
		if result.exit != 0 || len(result.stderr) != 0 || len(result.stdout) == 0 {
			t.Fatalf("%s linked result=%#v", activation, result)
		}
	}
}

// GPK-V0-005, GPK-V0-009, GPK-V0-032.
func TestInitAdoptSanitizesHostileGitEnvironment(t *testing.T) {
	t.Parallel()
	root := newActivationFixture(t, "sha1")
	arguments := activationArguments(root, "init", false)
	candidateCommand := candidateCommand(arguments...)
	hostile := []string{"GIT_ALTERNATE_OBJECT_DIRECTORIES=/missing", "GIT_CONFIG_GLOBAL=/missing", "GIT_NO_LAZY_FETCH=0", "GIT_PROTOCOL_FROM_USER=1"}
	candidateCommand.Env = append(candidateCommand.Env, hostile...)
	result := execute(t, candidateCommand)
	if result.exit != 0 || len(result.stderr) != 0 || len(result.stdout) == 0 {
		t.Fatalf("hostile environment result=%#v", result)
	}
}

func TestInitAdoptRejectsObjectAlternatesLikePython(t *testing.T) {
	t.Parallel()
	root := newActivationFixture(t, "sha1")
	alternates := filepath.Join(root, ".git", "objects", "info", "alternates")
	if err := os.WriteFile(alternates, []byte("/private/tmp/untrusted-objects\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, activation := range []string{"init", "adopt"} {
		arguments := activationArguments(root, activation, true)
		candidate := execute(t, candidateCommand(arguments...))
		if candidate.exit != 1 || !bytes.Contains(candidate.stdout, []byte(`"code":"invalid-repository"`)) {
			t.Fatalf("candidate=%#v", candidate)
		}
	}
}

func TestInitAdoptMissingTreeObjectMatchesPythonWithoutFetch(t *testing.T) {
	t.Parallel()
	root := newActivationFixture(t, "sha1")
	tree := gitOutput(t, root, "rev-parse", "HEAD^^{tree}")
	object := filepath.Join(root, ".git", "objects", tree[:2], tree[2:])
	if err := os.Remove(object); err != nil {
		t.Fatal(err)
	}
	arguments := activationArguments(root, "init", false)
	candidate := execute(t, candidateCommand(arguments...))
	if candidate.exit != 1 || len(candidate.stderr) != 0 || !bytes.Contains(candidate.stdout, []byte(`"operationalState":"INVALID"`)) ||
		!bytes.Contains(candidate.stdout, []byte(`"code":"git-read-failed"`)) {
		t.Fatalf("missing tree result=%#v", candidate)
	}
}

func TestActivationArgumentErrorsAndHelp(t *testing.T) {
	t.Parallel()
	root := newActivationFixture(t, "sha1")
	for _, arguments := range [][]string{
		{"--root", root, "init", "--revision"},
		{"--root", root, "adopt", "--unknown"},
	} {
		candidate := execute(t, candidateCommand(arguments...))
		if candidate.exit != 2 || len(candidate.stdout) != 0 {
			t.Fatalf("candidate=%#v", candidate)
		}
	}
	for _, activation := range []string{"init", "adopt"} {
		result := execute(t, candidateCommand(activation, "--help"))
		if result.exit != 0 || len(result.stderr) != 0 || !bytes.Contains(result.stdout, []byte("--full-receipt")) {
			t.Fatalf("%s help=%#v", activation, result)
		}
	}
}

func TestActivationUnsupportedPlatformRefusesBeforeRepositoryAccess(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	exit := runActivationForPlatform(context.Background(), filepath.Join(t.TempDir(), "missing"), "init", nil, "windows", &stdout, &stderr)
	want := "{\"code\": \"unsupported-genesis-platform\", \"error\": \"native Go Genesis inventory is qualified only on Darwin and Linux\", \"evidence\": [{\"name\": \"platform\", \"value\": \"windows\"}], \"ok\": false, \"subject\": {\"kind\": \"host-capability\", \"value\": \"native-platform\"}, \"supported_fixes\": [], \"terminal\": \"unsupported-platform\"}\n"
	if exit != 2 || stdout.Len() != 0 || stderr.String() != want {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
}

func activationArguments(root, activation string, full bool) []string {
	arguments := []string{
		"--root", root, activation, "--authority-id", "repo:fixture", "--revision", "HEAD^",
		"--exclude-prefix", "vendor", "--exclude-prefix", "docs/private",
	}
	if full {
		arguments = append(arguments, "--full-receipt")
	}
	return arguments
}

func newActivationFixture(t *testing.T, objectFormat string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repository")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "private"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"README.md": []byte("# fixture\n"), "src/main.go": []byte("package main\r\n"),
		"src/café.py": []byte("def café():\n    return 'ok'\n"), "vendor/lib.js": []byte("export const x = 1\n"),
		"docs/private/note.md": []byte("private\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	initArguments := []string{"init", "-q"}
	if objectFormat == "sha256" {
		initArguments = append(initArguments, "--object-format=sha256")
	}
	command := exec.Command("git", initArguments...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		if objectFormat == "sha256" {
			t.Skipf("Git lacks SHA-256 repositories: %v\n%s", err, output)
		}
		t.Fatalf("git init: %v\n%s", err, output)
	}
	environment := []string{"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z"}
	gitCommand(t, root, environment, "config", "user.email", "corvint@example.test")
	gitCommand(t, root, environment, "config", "user.name", "Corvint Test")
	if runtime.GOOS != "windows" {
		if err := os.Symlink("README.md", filepath.Join(root, "readme-link")); err != nil {
			t.Fatal(err)
		}
	}
	gitCommand(t, root, environment, "add", ".")
	gitCommand(t, root, environment, "commit", "-qm", "initial")
	if err := os.WriteFile(filepath.Join(root, "CURRENT.md"), []byte("current\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, environment, "add", ".")
	gitCommand(t, root, environment, "commit", "-qm", "current")
	return root
}

func gitCommand(t *testing.T, directory string, environment []string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	if directory != "" {
		command.Dir = directory
	}
	command.Env = append(os.Environ(), environment...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}

func decodeActivationOutput(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}
