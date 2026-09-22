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

func depsourceRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// DSE-V0-001: the invocation surface accepts `[--root PATH] depsource
// <module>[@version] [--file REL] [--limit N]` and refuses anything else.
func TestParseDepsourceInvocation(t *testing.T) {
	t.Parallel()
	root := depsourceRoot(t)
	options, isDepsource, err := parseDepsourceInvocation([]string{
		"--root", root, "depsource", "example.com/Dep@v1.2.3", "--file", "dep.go", "--limit", "7",
	})
	if !isDepsource || err != nil {
		t.Fatalf("expected a recognized invocation, got %v, %v", isDepsource, err)
	}
	if options.Root != root || options.Module != "example.com/Dep" || options.Version != "v1.2.3" {
		t.Fatalf("unexpected options: %+v", options)
	}
	if options.File != "dep.go" || options.Limit != 7 {
		t.Fatalf("unexpected bounds: %+v", options)
	}

	if bare, _, err := parseDepsourceInvocation([]string{"--root=" + root, "depsource", "example.com/dep"}); err != nil ||
		bare.Version != "" || bare.Limit != 200 {
		t.Fatalf("expected an unversioned default-limit request, got %+v, %v", bare, err)
	}

	if _, isDepsource, _ := parseDepsourceInvocation([]string{"observations"}); isDepsource {
		t.Fatal("depsource must not claim another verb")
	}

	for _, arguments := range [][]string{
		{"--root", root, "depsource"},
		{"--root", root, "depsource", "example.com/dep", "--limit", "0"},
		{"--root", root, "depsource", "example.com/dep", "--nope"},
	} {
		if _, _, err := parseDepsourceInvocation(arguments); err == nil {
			t.Fatalf("expected %v to be refused", arguments)
		}
	}
}

// DSE-V0-001: an argument defect exits 2 and writes nothing to stdout.
func TestRunDepsourceArgumentFailure(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if status := runDepsource(t.Context(), []string{"--root", depsourceRoot(t), "depsource"}, stdout, stderr); status != 2 {
		t.Fatalf("expected exit status 2, got %d", status)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "module") {
		t.Fatalf("unexpected streams: %q / %q", stdout.String(), stderr.String())
	}
}

// depsourceCommittedRoot commits a go.mod that requires exampleModule at
// exampleVersion with no go.sum, so an uncancelled request abstains
// successfully (reason "missing-go-sum", exit 0) instead of failing for an
// unrelated reason.
const (
	depsourceExampleModule  = "example.com/dep"
	depsourceExampleVersion = "v1.2.3"
)

func depsourceCommittedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	goMod := "module cmd.test/consumer\n\ngo 1.27.0\n\nrequire " + depsourceExampleModule + " " + depsourceExampleVersion + "\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "depsource fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// DSE-V0: ctx is main's signal context, so a cancellation already in flight
// when the verb starts must abort the committed-revision read instead of
// completing the abstention it would otherwise reach.
func TestRunDepsourceHonorsCancellation(t *testing.T) {
	t.Parallel()
	root := depsourceCommittedRoot(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	code := runDepsource(ctx, []string{"--root", root, "depsource", depsourceExampleModule + "@" + depsourceExampleVersion}, &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 {
		t.Fatalf("exit %d stdout %q stderr %q, want exit 2 with no output once cancellation is honored", code, stdout.String(), stderr.String())
	}
}
