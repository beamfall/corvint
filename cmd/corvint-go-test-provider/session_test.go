package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/session"
)

// TestReadModulePathMatchesGoModEdit pins readModulePath to the module path
// `go mod edit -json` reports for the same go.mod.
func TestReadModulePathMatchesGoModEdit(t *testing.T) {
	cases := map[string]string{
		"module example.com/plain\n\ngo 1.27.0\n":             "example.com/plain",
		"module \"example.com/quoted\"\n\ngo 1.27.0\n":        "example.com/quoted",
		"module example.com/commented // note\n\ngo 1.27.0\n": "example.com/commented",
	}
	for content, want := range cases {
		path := filepath.Join(t.TempDir(), "go.mod")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := readModulePath(path); got != want {
			t.Errorf("readModulePath(%q) = %q, want %q", content, got, want)
		}
	}
}

// TestResolveWatchedFilesWalksDotNamedRoot pins that the default walk skips
// dot directories below the repository root but never the root itself, even
// when the root's own base name starts with a dot.
func TestResolveWatchedFilesWalksDotNamedRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".repo")
	for _, dir := range []string{root, filepath.Join(root, ".hidden")} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(root, "a_test.go"), filepath.Join(root, ".hidden", "b.go")} {
		if err := os.WriteFile(path, []byte("package a\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files, testFiles, err := resolveWatchedFiles(root, nil)
	want := filepath.Join(root, "a_test.go")
	if err != nil || len(files) != 1 || files[0] != want || !testFiles[want] {
		t.Fatalf("files=%v testFiles=%v err=%v", files, testFiles, err)
	}
}

// LPCV-V0-055: session --retain writes each completed event line's exact
// stdout bytes into .corvint/test-evidence, never a running event, prunes only
// its own names to the newest 32, and refuses a symlinked location while
// still emitting the line.
func TestSessionRetainsCompletedEventBytesPrunesAndRefusesSymlink(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, ".corvint", "test-evidence")
	if err := os.MkdirAll(evidence, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a_test.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for index := range 40 {
		name := filepath.Join(evidence, fmt.Sprintf("corvint-go-test-provider-%019d-%016x.json", index+1, index))
		if err := os.WriteFile(name, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(evidence, "operator.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	retention := &sessionRetention{root: root, stderr: &stderr}
	bundle := authorityBundle{RepositoryRoot: root, GoExecutable: filepath.Join(root, "missing-go")}
	cfg, err := buildSessionConfig(bundle, nil, nil, time.Second, time.Second, &stdout, retention)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Publish(session.Event{State: session.StateRunning, Sequence: 1})
	running := stdout.Len()
	cfg.Publish(session.Event{State: session.StatePassed, Identity: "identity", Sequence: 2})
	retained, _ := filepath.Glob(filepath.Join(evidence, "corvint-go-test-provider-*.json"))
	sort.Strings(retained)
	data, readErr := os.ReadFile(retained[len(retained)-1])
	if readErr != nil || !bytes.Equal(data, stdout.Bytes()[running:]) || len(retained) != 32 || retention.failed.Load() {
		t.Fatalf("retained=%d equal=%v failed=%v stderr=%s", len(retained), bytes.Equal(data, stdout.Bytes()[running:]), retention.failed.Load(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evidence, "operator.json")); err != nil {
		t.Fatalf("pruned a foreign entry: %v", err)
	}

	linked, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".corvint"), filepath.Join(linked, ".corvint")); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	refused := &sessionRetention{root: linked, stderr: &stderr}
	refused.keep(session.StateFailed, []byte("{}\n"))
	if !refused.failed.Load() || !strings.Contains(stderr.String(), "retention failure") {
		t.Fatalf("symlinked location was not refused: stderr=%s", stderr.String())
	}
	if after, _ := filepath.Glob(filepath.Join(evidence, "corvint-go-test-provider-*.json")); len(after) != 32 {
		t.Fatalf("a refused retention wrote through the symlink: %d entries", len(after))
	}
}

// TestSessionIgnoresTheUserGoEnvFile pins GLTP-V0-048's environment: a
// `go env -w` file under the inherited HOME must not reach the session's go
// command, or a GOFLAGS -run filter there turns a failing test into a pass.
func TestSessionIgnoresTheUserGoEnvFile(t *testing.T) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go executable not found")
	}
	goPath, err = filepath.EvalSymlinks(goPath)
	if err != nil {
		t.Skip("go executable is not resolvable")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(root, "home"))
	// os.UserConfigDir prefers XDG_CONFIG_HOME on Linux (GitHub runners set it); without
	// this the poisoned go env file lands in the real user config and outlives the test.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "home", ".config"))
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(configDir, "go"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "go", "env"), []byte("GOFLAGS=-run=^NoSuchTest$\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(root, "module")
	if err := os.MkdirAll(module, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module fixture\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(module, "fixture_test.go")
	if err := os.WriteFile(testFile, []byte("package fixture\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) { t.Fatal(\"broken\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bundle := authorityBundle{RepositoryRoot: module, GoExecutable: goPath, TemporaryParent: filepath.Join(root, "tmp"), Packages: []string{"./..."}}
	cfg, err := buildSessionConfig(bundle, nil, []string{testFile}, 50*time.Millisecond, 50*time.Millisecond, io.Discard, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var final session.State
	cfg.Publish = func(event session.Event) {
		if event.State != session.StateIdle && event.State != session.StateRunning {
			final = event.State
			cancel()
		}
	}
	_ = session.Run(ctx, cfg)
	if final != session.StateFailed {
		t.Fatalf("session state = %q, want failed", final)
	}
}
