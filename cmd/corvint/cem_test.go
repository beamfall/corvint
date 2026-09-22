package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cemGit runs git on the fixture with the host's config files masked rather
// than HOME moved: macOS's git shim keys its toolchain cache on HOME, and a
// foreign HOME under a sandbox costs a full re-resolution plus a cache warning
// on stderr per call. Only stdout is returned, so such a warning can never
// leak into a revision or blob the test hands back to the CLI.
func cemGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s%s", args, err, stdout.String(), stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

func cemWrite(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cemRepo builds a byte-deterministic two-commit fixture repository.
func cemRepo(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	cemWrite(t, root, "docs/rule.txt", "the frozen rule\nsecond line\n")
	cemWrite(t, root, "src/app.txt", "alpha\nbeta\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "src/app.txt", "alpha\nBETA\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "target")
	target := cemGit(t, root, "rev-parse", "HEAD")
	return root, base, target
}

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestCEMHelpSurfaces(t *testing.T) {
	t.Parallel()
	code, out, _ := runCLI(t, "--help")
	if code != 0 || !strings.Contains(out, "cem ACTION") {
		t.Fatalf("root help omits cem: %d", code)
	}
	for _, invocation := range [][]string{{"help", "cem"}, {"cem", "--help"}} {
		code, out, _ := runCLI(t, invocation...)
		if code != 0 {
			t.Fatalf("%v exited %d", invocation, code)
		}
		for _, action := range []string{"begin", "prepare", "cite", "mark", "status", "verify", "report"} {
			if !strings.Contains(out, action) {
				t.Fatalf("%v help omits %q", invocation, action)
			}
		}
	}
}

func TestCEMErrorPrecedence(t *testing.T) {
	t.Parallel()
	root, base, target := cemRepo(t)
	// Stage 1: CLI syntax before anything else.
	code, _, stderr := runCLI(t, "--root", root, "cem")
	if code != 2 || !strings.Contains(stderr, `"code": "invalid-arguments"`) {
		t.Fatalf("missing action: %d %s", code, stderr)
	}
	code, _, stderr = runCLI(t, "--root", root, "cem", "status", "--bogus", "x")
	if code != 2 || !strings.Contains(stderr, "invalid-arguments") {
		t.Fatalf("unknown flag: %d %s", code, stderr)
	}
	// Stage 2 (map read and closed schema) precedes stage 3 (--patch denial):
	// an invalid 0.2 map plus a forbidden --patch reports the schema failure.
	code, _, _ = runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", target)
	if code != 0 {
		t.Fatal("prepare failed")
	}
	mapPath := filepath.Join(root, ".corvint", "change.cem.json")
	data, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := strings.Replace(string(data), `"spec": "cem/0.2"`, `"spec": "cem/0.2",\n  "surplus": true`, 1)
	corrupted = strings.ReplaceAll(corrupted, `\n`, "\n")
	if err := os.WriteFile(mapPath, []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "--root", root, "cem", "verify",
		"--map", ".corvint/change.cem.json", "--patch", "x.patch",
		"--expected-base", base, "--target", "HEAD")
	if code != 2 || !strings.Contains(stderr, "unknown-field") {
		t.Fatalf("schema failure did not precede --patch denial: %d %s", code, stderr)
	}
}

func TestCEMCLIRefusesGitMetadataOutput(t *testing.T) {
	root, base, target := cemRepo(t)
	code, _, stderr := runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", target)
	if code != 0 {
		t.Fatalf("prepare: %d %s", code, stderr)
	}
	configPath := filepath.Join(root, ".git", "config")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "--root", root, "cem", "begin",
		"--patch", ".git/corvint/change.patch", "--output", ".git/config", "--base", base)
	if code != 2 || !strings.Contains(stderr, `"code": "invalid-arguments"`) {
		t.Fatalf("metadata output: %d %s", code, stderr)
	}
	after, err := os.ReadFile(configPath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("metadata output changed Git config: %v", err)
	}
}

// TestCEMPrepareKeepsInheritedMapReplaceGuidance pins the bounded recovery line
// that the untyped map-read collapse used to discard: an inherited map from an
// earlier range names --replace, while a genuinely absent map keeps the fixed
// oracle text byte-for-byte.
func TestCEMPrepareKeepsInheritedMapReplaceGuidance(t *testing.T) {
	t.Parallel()
	root, base, target := cemRepo(t)
	code, _, stderr := runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", target)
	if code != 0 {
		t.Fatalf("first prepare: %d %s", code, stderr)
	}
	cemWrite(t, root, "docs/rule.txt", "the frozen rule\nthird line\n")
	cemGit(t, root, "add", "docs/rule.txt")
	cemGit(t, root, "commit", "-qm", "third")
	third := cemGit(t, root, "rev-parse", "HEAD")
	// The inherited map records the first range, so this one is refused.
	code, _, stderr = runCLI(t, "--root", root, "cem", "prepare", "--base", target, "--target", third)
	if code != 2 {
		t.Fatalf("inherited map was not refused: %d %s", code, stderr)
	}
	expected := "{\"error\": \"cannot read CEM map: the existing map records a different base or patch; " +
		"pass --replace to regenerate\", \"ok\": false}\n"
	if stderr != expected {
		t.Fatalf("guidance lost:\n got %q\nwant %q", stderr, expected)
	}
	// CEM-PILOT-018: an invalid map carries its own line without parser detail.
	cemWrite(t, root, ".corvint/change.cem.json", "not a CEM document\n")
	code, _, stderr = runCLI(t, "--root", root, "cem", "prepare", "--base", target, "--target", third)
	expected = "{\"error\": \"cannot read CEM map: the existing map is not a valid CEM document; " +
		"pass --replace to regenerate\", \"ok\": false}\n"
	if code != 2 || stderr != expected {
		t.Fatalf("invalid-map guidance: %d\n got %q\nwant %q", code, stderr, expected)
	}
	// An absent map carries no guidance and keeps the fixed untyped text.
	code, _, stderr = runCLI(t, "--root", root, "cem", "verify", "--map", ".corvint/nope.json")
	if code != 2 || stderr != "{\"error\": \"cannot read CEM map\", \"ok\": false}\n" {
		t.Fatalf("absent map text moved: %d %q", code, stderr)
	}
}

// TestCEMEndToEndAndGoldenEnvelope drives the full two-phase flow through the
// CLI and freezes the canonical status envelope byte-for-byte (roots
// normalized; every hash is deterministic from the fixed fixture).
func TestCEMEndToEndAndGoldenEnvelope(t *testing.T) {
	t.Parallel()
	root, base, target := cemRepo(t)
	code, _, stderr := runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", target)
	if code != 0 {
		t.Fatalf("prepare: %s", stderr)
	}
	code, _, stderr = runCLI(t, "--root", root, "cem", "cite",
		"--map", ".corvint/change.cem.json", "--hunk", "1",
		"--evidence-path", "docs/rule.txt", "--lines", "1:1", "--relation", "specification")
	if code != 0 {
		t.Fatalf("cite: %s", stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "candidate")
	code, out, stderr := runCLI(t, "--root", root, "cem", "status",
		"--map", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD",
		"--max-unknown", "0", "--max-mechanical", "0")
	if code != 0 {
		t.Fatalf("status: %d %s", code, stderr)
	}
	normalized := strings.ReplaceAll(out, root, "$ROOT")
	golden := filepath.Join("testdata", "cem-status-canonical.golden")
	if os.Getenv("UPDATE_CEM_GOLDENS") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(normalized), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("golden missing (set UPDATE_CEM_GOLDENS=1 to record): %v", err)
	}
	if normalized != string(want) {
		t.Fatalf("canonical status envelope drifted:\n got: %s\nwant: %s", normalized, want)
	}
	// A failing policy still exits 1 with an envelope on stdout.
	code, out, _ = runCLI(t, "--root", root, "cem", "mark",
		"--map", ".corvint/change.cem.json", "--hunk", "1",
		"--disposition", "unknown", "--reason", "insufficient-evidence")
	if code != 0 {
		t.Fatalf("mark: %s", out)
	}
	code, out, _ = runCLI(t, "--root", root, "cem", "status",
		"--map", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD",
		"--max-unknown", "0")
	if code != 1 || !strings.Contains(out, "max-unknown-exceeded") {
		t.Fatalf("policy failure: %d %s", code, out)
	}
}

// TestCEMSeamsDependOnlyOnStdlibAndGit enforces the native-cem-adapter
// dependency claim: the CEM seams import nothing beyond the standard library
// and each other.
func TestCEMSeamsDependOnlyOnStdlibAndGit(t *testing.T) {
	t.Parallel()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool unavailable")
	}
	command := exec.Command(goTool, "list", "-deps", "./internal/cem/...")
	command.Dir = filepath.Join("..", "..")
	out, err := command.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, dependency := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if !strings.Contains(strings.SplitN(dependency, "/", 2)[0], ".") {
			continue // standard library: no dot in the first path segment
		}
		if strings.HasPrefix(dependency, "github.com/Beamfall/corvint/internal/cem") {
			continue
		}
		t.Errorf("CEM seams depend on %s", dependency)
	}
}

// TestCEMExplicitRootKeepsStagePrecedence: an explicit --root must get only
// syntactic normalization — with no .git present at all, stage-2 map defects
// and stage-4 missing inputs still report their own codes, and only a clean
// stage 1–4 pass reaches the repository validation refusal.
func TestCEMExplicitRootKeepsStagePrecedence(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	valid := `{"spec":"cem/0.2","baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",` +
		`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",` +
		`"excludedPath":".corvint/change.cem.json","evidence":[],"hunks":[]}`
	corrupt := strings.Replace(valid, `"spec":"cem/0.2"`, `"spec":"cem/0.2","surplus":true`, 1)
	cemWrite(t, root, ".corvint/change.cem.json", corrupt)
	// Stage 2 beats both the stage-3 --patch denial and any repository check.
	code, _, stderr := runCLI(t, "--root", root, "cem", "verify",
		"--map", ".corvint/change.cem.json", "--patch", "x.patch",
		"--expected-base", "4ca153370afd9bd8c6034ad73acc3925150ab681", "--target", "HEAD")
	if code != 2 || !strings.Contains(stderr, "unknown-field") {
		t.Fatalf("stage 2 vs missing .git: %d %s", code, stderr)
	}
	cemWrite(t, root, ".corvint/change.cem.json", valid)
	// Stage 4 beats the repository check.
	code, _, stderr = runCLI(t, "--root", root, "cem", "verify",
		"--map", ".corvint/change.cem.json", "--target", "HEAD")
	if code != 2 || !strings.Contains(stderr, "expected-base-required") {
		t.Fatalf("stage 4 vs missing .git: %d %s", code, stderr)
	}
	// Clean stages 1–4 finally reach the stage-5 repository refusal.
	code, _, stderr = runCLI(t, "--root", root, "cem", "verify",
		"--map", ".corvint/change.cem.json",
		"--expected-base", "4ca153370afd9bd8c6034ad73acc3925150ab681", "--target", "HEAD")
	if code != 2 || !strings.Contains(stderr, "repository-object-unavailable") {
		t.Fatalf("stage 5 refusal: %d %s", code, stderr)
	}
}

// TestRootHelpMutationBoundary freezes the corrected support-boundary text:
// the read-only claim is scoped to the read-only slices and the CEM mutation
// surfaces are named.
func TestRootHelpMutationBoundary(t *testing.T) {
	t.Parallel()
	code, out, _ := runCLI(t, "--help")
	if code != 0 {
		t.Fatalf("help exited %d", code)
	}
	for _, required := range []string{"query, impact, docs, harness, and lrf read without mutating", "write local CEM artifacts", "cem\n  report writes the local review report"} {
		if !strings.Contains(out, required) {
			t.Fatalf("support boundary lost %q", required)
		}
	}
	if strings.Contains(out, "source\n  mutation request") {
		t.Fatal("support boundary reverted to the blanket non-mutation claim")
	}
}
