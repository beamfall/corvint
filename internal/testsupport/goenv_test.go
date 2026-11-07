package testsupport

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestGoEnvAddsBuildVCSFalse_AFPV07920 pins how GoEnv rewrites GOFLAGS.
func TestGoEnvAddsBuildVCSFalse_AFPV07920(t *testing.T) {
	cases := map[string]struct{ in, want []string }{
		"no GOFLAGS":    {[]string{"HOME=/h"}, []string{"HOME=/h", "GOFLAGS=-buildvcs=false"}},
		"empty GOFLAGS": {[]string{"GOFLAGS="}, []string{"GOFLAGS=-buildvcs=false"}},
		"last wins":     {[]string{"GOFLAGS=-mod=mod", "X=1", "GOFLAGS=-count=1"}, []string{"GOFLAGS=-mod=mod", "X=1", "GOFLAGS=-count=1 -buildvcs=false"}},
		"caller choice": {[]string{"GOFLAGS=-buildvcs=true"}, []string{"GOFLAGS=-buildvcs=true"}},
	}
	for name, c := range cases {
		in := append([]string(nil), c.in...)
		if got := GoEnv(in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: GoEnv(%q) = %q, want %q", name, c.in, got, c.want)
		}
		if !reflect.DeepEqual(in, c.in) {
			t.Errorf("%s: GoEnv modified its input to %q", name, in)
		}
	}
}

// TestGoEnvBuildsUnderStrayGitDirectory_AFPV07920 reproduces V1-4501: a main
// package in a temporary fixture under an empty .git directory fails go
// build's VCS stamping, and GoEnv's environment builds it.
func TestGoEnvBuildsUnderStrayGitDirectory_AFPV07920(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(root, "fixture")
	if err := os.Mkdir(module, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"go.mod": "module fixture\n\ngo 1.22\n", "main.go": "package main\n\nfunc main() {}\n"}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(module, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := append(os.Environ(), "GOFLAGS=", "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off")
	build := func(env []string) (string, error) {
		command := exec.Command("go", "build", "-o", filepath.Join(t.TempDir(), "fixture"), ".")
		command.Dir = module
		command.Env = env
		output, err := command.CombinedOutput()
		return string(output), err
	}
	if output, err := build(base); err == nil || !strings.Contains(output, "VCS status") {
		t.Skipf("this host's go does not fail VCS stamping under an empty .git (err=%v): %s", err, output)
	}
	if output, err := build(GoEnv(base)); err != nil {
		t.Fatalf("go build with GoEnv: %v\n%s", err, output)
	}
}
