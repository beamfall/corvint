package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMigrateTracesDryRunMatchesPythonOracleBytes(t *testing.T) {
	t.Parallel()
	root := newLegacyTraceFixture(t)
	before := traceTreeSnapshot(t, root)
	arguments := []string{"--root", root, "migrate-traces", "--dry-run"}
	candidate := candidateCommand(arguments...)
	candidateResult := execute(t, candidate)
	if candidateResult.exit != 0 || len(candidateResult.stderr) != 0 ||
		!bytes.Contains(candidateResult.stdout, []byte(`"mode":"dry-run"`)) ||
		!bytes.Contains(candidateResult.stdout, []byte(`"legacy_trace_files":1`)) {
		t.Fatalf("dry-run exit=%d stderr=%s", candidateResult.exit, candidateResult.stderr)
	}
	after := traceTreeSnapshot(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("dry-run mutated trace state\nbefore=%#v\nafter=%#v", before, after)
	}
}

func testMigrateTracesApplyIndependentCanonicalFixture(t *testing.T) {
	candidateRoot := newLegacyTraceFixture(t)
	before := traceTreeSnapshot(t, candidateRoot)
	tree := gitOutput(t, candidateRoot, "rev-parse", "HEAD^{tree}")
	commit := gitOutput(t, candidateRoot, "rev-parse", "HEAD")
	candidateDry := execute(t, candidateCommand("--root", candidateRoot, "migrate-traces", "--dry-run"))
	if candidateDry.exit != 0 || len(candidateDry.stderr) != 0 {
		t.Fatalf("dry-run result=%#v", candidateDry)
	}
	digest := planDigest(t, candidateDry.stdout)
	candidateApply := execute(t, candidateCommand("--root", candidateRoot, "migrate-traces", "--apply", "--plan-digest", digest))
	if candidateApply.exit != 0 || len(candidateApply.stderr) != 0 ||
		!bytes.Contains(candidateApply.stdout, []byte(`"mode":"apply"`)) ||
		!bytes.Contains(candidateApply.stdout, []byte(`"mutates":true`)) {
		t.Fatalf("apply exit=%d stderr=%s", candidateApply.exit, candidateApply.stderr)
	}
	state := traceTreeSnapshot(t, candidateRoot)
	if _, exists := state["traces/"+tree+".jsonl"]; exists {
		t.Fatal("legacy source was not removed")
	}
	for _, entry := range []struct{ path, revision string }{
		{"traces/" + commit + ".jsonl", commit},
		{"legacy-traces/" + tree + ".jsonl", tree},
	} {
		path := filepath.Join(candidateRoot, ".context-corvint", entry.path)
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, independentlyAuthoredTraceRow(entry.revision)) {
			t.Fatalf("migration bytes %s: %v %q", entry.path, err, got)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("migration privacy %s: %v", entry.path, err)
		}
	}
	for path, value := range before {
		if path != "traces/"+tree+".jsonl" && state[path] != value {
			t.Fatalf("unrelated state changed: %s", path)
		}
	}
}

func TestMigrateTracesModeArgumentErrorsMatchPythonOracle(t *testing.T) {
	t.Parallel()
	root := newLegacyTraceFixture(t)
	for _, arguments := range [][]string{
		{"--root", root, "migrate-traces"},
		{"--root", root, "migrate-traces", "--dry-run", "--apply"},
	} {
		candidate := execute(t, candidateCommand(arguments...))
		if candidate.exit != 2 || len(candidate.stdout) != 0 || len(candidate.stderr) == 0 {
			t.Fatalf("args=%v candidate=%#v", arguments, candidate)
		}
	}
}

func TestMigrateTracesInvalidRootMatchesPythonOracle(t *testing.T) {
	t.Parallel()
	arguments := []string{"--root", t.TempDir(), "migrate-traces", "--dry-run"}
	candidate := execute(t, candidateCommand(arguments...))
	if candidate.exit != 2 || len(candidate.stdout) != 0 || len(candidate.stderr) == 0 {
		t.Fatalf("candidate=%#v", candidate)
	}
}

func TestMigrateTracesPlanDigestMismatchWritesNothing(t *testing.T) {
	t.Parallel()
	root := bareMigrationRepository(t)
	before := traceTreeSnapshot(t, root)
	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	result := execute(t, candidateCommand("--root", root, "migrate-traces", "--apply", "--plan-digest", wrong))
	if result.exit != 2 || len(result.stdout) != 0 || !bytes.Contains(result.stderr, []byte("trace migration plan drifted after dry run")) {
		t.Fatalf("candidate=%#v", result)
	}
	after := traceTreeSnapshot(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("digest mismatch mutated repository\nbefore=%#v\nafter=%#v", before, after)
	}
}

func newLegacyTraceFixture(t *testing.T) string {
	t.Helper()
	root := bareMigrationRepository(t)
	tree := gitOutput(t, root, "rev-parse", "HEAD^{tree}")
	row := independentlyAuthoredTraceRow(tree)
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, tree+".jsonl"), row, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".context-corvint", "unrelated"), []byte("preserve me\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func independentlyAuthoredTraceRow(revision string) []byte {
	// Literal protocol spelling and independent SHA basis; production writers
	// must not construct either the legacy input or expected migrated bytes.
	basis := fmt.Sprintf(`{"changed_paths":["internal/example/value.go"],"opened_paths":["internal/example/value.go"],"outcome":"passed","revision":"%s","schema_version":1,"task":"verify example trace","verification":["git diff --check"]}`, revision)
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(basis)))
	return []byte(strings.Replace(basis, `,"verification":`, `,"trace_id":"`+id+`","verification":`, 1) + "\n")
}

func bareMigrationRepository(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "example"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{
		".gitignore":                ".context-corvint/\n",
		"go.mod":                    "module example.test/traces\n\ngo 1.27.0\n",
		"internal/example/value.go": "package example\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "initial"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func candidateCommand(arguments ...string) *exec.Cmd {
	command := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
	command.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
	return command
}

func planDigest(t *testing.T, output []byte) string {
	t.Helper()
	var payload struct {
		PlanDigest string `json:"plan_digest"`
	}
	if err := json.Unmarshal(output, &payload); err != nil || payload.PlanDigest == "" {
		t.Fatalf("plan output: %v %s", err, output)
	}
	return payload.PlanDigest
}

func traceTreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	base := filepath.Join(root, ".context-corvint")
	result := make(map[string]string)
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && path == base {
				return fs.SkipDir
			}
			return walkErr
		}
		relative, err := filepath.Rel(base, path)
		if err != nil || relative == "." {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%s:%04o", info.Mode().Type(), info.Mode().Perm())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(":%x", sha256.Sum256(data))
		}
		result[filepath.ToSlash(relative)] = value
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return result
}

func TestMigrateTracesApplyMatchesPythonOracle(t *testing.T) {
	t.Parallel()
	t.Run("GOC-V0-004 independent canonical legacy migration", testMigrateTracesApplyIndependentCanonicalFixture)
}
