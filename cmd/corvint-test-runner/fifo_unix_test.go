//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

const fifoHelperEnv = "CORVINT_TEST_RUNNER_FIFO_HELPER"

// TestFIFOHelperProcess is the re-executed companion process; it is inert
// unless the process-level FIFO tests below select it through the environment.
func TestFIFOHelperProcess(t *testing.T) {
	if os.Getenv(fifoHelperEnv) != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(command(ctx, args, os.Stdout, os.Stderr))
}

// runCompanion runs the companion as a real child process with a hard deadline,
// so a blocking FIFO open fails the test instead of hanging it.
func runCompanion(t *testing.T, args ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestFIFOHelperProcess$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), fifoHelperEnv+"=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("companion blocked on a nonregular document for %s: %v", time.Since(start), args)
	}
	code := 0
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return code, stderr.String()
}

func fifo(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	return path
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

// V1-0624: request, plan and tools documents that name a FIFO without a writer
// are refused before any blocking open, document validation or execution.
func TestNonregularRunnerDocumentsRefusedBeforeBlockingOpen(t *testing.T) {
	dir := t.TempDir()
	pipe := fifo(t, dir, "document.json")
	request := filepath.Join(dir, "request.json")
	writeJSON(t, request, tr.Request{Runner: "go-test", Project: "example.invalid/p", ReportDir: filepath.Join(dir, "reports")})
	identity := []string{"--executable", "/independent/go", "--executable-sha256", strings.Repeat("0", 64)}
	for name, args := range map[string][]string{
		"request": append([]string{"plan", "--request", pipe}, identity...),
		"tools":   append([]string{"plan", "--request", request, "--tools", pipe}, identity...),
		"plan":    append([]string{"run", "--plan", pipe, "--approve", strings.Repeat("0", 64), "--experimental", "--trusted-local"}, identity...),
	} {
		code, stderr := runCompanion(t, args...)
		if code != 1 || !strings.Contains(stderr, "runner refused: regular document required") {
			t.Fatalf("%s FIFO: exit=%d stderr=%q", name, code, stderr)
		}
	}
	// Directories and other nonregular paths take the same refusal.
	if code, stderr := runCompanion(t, append([]string{"plan", "--request", dir}, identity...)...); code != 1 || !strings.Contains(stderr, "regular document required") {
		t.Fatalf("directory request: exit=%d stderr=%q", code, stderr)
	}
	// Regular documents keep their normal admission.
	if code, stderr := runCompanion(t, append([]string{"plan", "--request", request}, identity...)...); code != 0 || !strings.Contains(stderr, "planSha256:") {
		t.Fatalf("regular request: exit=%d stderr=%q", code, stderr)
	}
}

// V1-0624: the independently pinned Gradle build manifest is refused without a
// blocking open when it becomes a FIFO after the plan was approved.
func TestPinnedGradleManifestFIFORefusedBeforeExecution(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "src")
	project := filepath.Join(root, "app")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(project, "build.gradle")
	manifestBytes := []byte("plugins { id 'java' }\n")
	source := []byte("class A {}\n")
	for path, b := range map[string][]byte{manifest: manifestBytes, filepath.Join(project, "A.java"): source} {
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	tools := map[string]tr.Tool{}
	for name, path := range map[string]string{"gradle": filepath.Join(dir, "gradle", "bin", "gradle"), "java": filepath.Join(dir, "jdk", "bin", "java")} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		b := []byte("#!/bin/sh\nexit 3\n")
		if err := os.WriteFile(path, b, 0700); err != nil {
			t.Fatal(err)
		}
		tools[name] = tr.Tool{Executable: path, Sha256: tr.Digest(b)}
	}
	toolsPath := filepath.Join(dir, "tools.json")
	writeJSON(t, toolsPath, map[string]tr.Tool{"java": tools["java"]})
	r := tr.Request{Runner: "gradle-junit", Root: root, Project: "app", Config: manifest, ConfigSha256: tr.Digest(manifestBytes), Target: ":test", ReportDir: filepath.Join(dir, "reports"), TimeoutSeconds: 60, InputFiles: map[string]string{"app/A.java": tr.Digest(source)}}
	requestPath := filepath.Join(dir, "request.json")
	writeJSON(t, requestPath, r)
	identity := []string{"--executable", tools["gradle"].Executable, "--executable-sha256", tools["gradle"].Sha256, "--tools", toolsPath}
	planPath := filepath.Join(dir, "plan.json")
	if code, stderr := runCompanion(t, append([]string{"plan", "--request", requestPath, "--out", planPath}, identity...)...); code != 0 {
		t.Fatalf("plan: exit=%d stderr=%q", code, stderr)
	}
	var p plan
	b, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = tr.DecodeDocument(b, &p); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	fifo(t, project, "build.gradle")
	receiptPath := filepath.Join(dir, "receipt.json")
	code, stderr := runCompanion(t, append([]string{"run", "--plan", planPath, "--approve", tr.Identity(p), "--out", receiptPath, "--experimental", "--trusted-local"}, identity...)...)
	if code != 1 {
		t.Fatalf("FIFO manifest run: exit=%d stderr=%q", code, stderr)
	}
	var result receipt
	if b, err = os.ReadFile(receiptPath); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	// The manifest is pinned only through Config, so the refusal comes from the
	// pinned-file reader rather than the source input inventory.
	if !strings.HasPrefix(result.Error, "config: nonregular file refused") {
		t.Fatalf("manifest refusal not retained: %q", result.Error)
	}
	if _, err = os.Stat(r.ReportDir); !os.IsNotExist(err) {
		t.Fatal("refused manifest reached runner execution")
	}
}
