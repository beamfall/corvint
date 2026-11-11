// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// envWithout is this process's environment without the named variables and
// without any variable that starts with one of the prefixes.
func envWithout(names []string, prefixes ...string) []string {
	drop := map[string]bool{}
	for _, n := range names {
		drop[n] = true
	}
	var out []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		keep := !drop[name]
		for _, p := range prefixes {
			if strings.HasPrefix(name, p) {
				keep = false
			}
		}
		if keep {
			out = append(out, kv)
		}
	}
	return out
}

// GLS-V0-051: a setting the go command, its compiler or cgo reads that
// `go env -changed` does not report (FC above all: go1.27.3 prints {} for
// FC=x go env -json -changed FC) refuses the identity, whether it comes from
// the environment or the go env -w file; an empty one adds nothing. AR and
// GCCGO are refused once go env -changed reports them.
func TestRunnerIDRefusesUnreportedToolchainEnvironment(t *testing.T) {
	requireGo(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"go.mod": "module example.com/m\n\ngo 1.22\n"})
	names := append(append(append([]string{"GOFLAGS", "GOENV", "GOWORK", "AR", "GCCGO"}, UncarriedEnvVars...), UnreportedGoEnvVars...), UnreportedProcessEnvVars...)
	base := append(envWithout(names, "CGO_"), "GOENV=off")
	plain, _, err := RunnerID(ctx, dir, base, DefaultTestTimeout, "")
	if err != nil {
		t.Fatal(err)
	}
	if empty, _, err := RunnerID(ctx, dir, append(base, "FC=", "GOCOMPILEDEBUG="), DefaultTestTimeout, ""); err != nil || empty != plain {
		t.Fatalf("empty FC and GOCOMPILEDEBUG: %q %v, want %q", empty, err, plain)
	}
	for _, kv := range []string{"FC=gfortran-other", "GO_EXTLINK_ENABLED=1", "GCC=gcc-other", "GOCOMPILEDEBUG=panicnil=1", "GOCLOBBERDEADHASH=1", "GOSSAFUNC=F", "GOSSADIR=/linecov-ssa", "AR=ar-other", "GCCGO=gccgo-other"} {
		name, _, _ := strings.Cut(kv, "=")
		if id, _, err := RunnerID(ctx, dir, append(base, kv), DefaultTestTimeout, ""); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: %q %v, want a refusal naming %s", kv, id, err, name)
		}
	}
	saved := filepath.Join(t.TempDir(), "go.env")
	if err := os.WriteFile(saved, []byte("FC=gfortran-saved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if id, _, err := RunnerID(ctx, dir, append(base, "GOENV="+saved), DefaultTestTimeout, ""); err == nil || !strings.Contains(err.Error(), "FC") {
		t.Errorf("FC saved with go env -w: %q %v, want a refusal naming FC", id, err)
	}
}

// GLS-V0-052: the variables the Go runtime reads from its own copy of the
// environment (GOTRACEBACK, a GOROOT naming another toolchain) and those the
// dynamic loader and C library read (LD_*, DYLD_*, GLIBC_TUNABLES) are in
// runtime=, loader names sorted after the runtime's; none set leaves
// runtime="". A GOROOT spelling exactly the go command's own root leaves the
// identity unchanged; a symbolic-link or trailing-separator alias is bound,
// because runtime.GOROOT returns that spelling (PR 194 review).
func TestRunnerIDBindsRuntimeReadEnvironment(t *testing.T) {
	requireGo(t)
	ctx := context.Background()
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := envWithout(append(append([]string{}, RuntimeEnvVars...), LoaderEnvVars...), LoaderEnvPrefixes...)
	id := func(env ...string) string {
		t.Helper()
		got, _, err := RunnerID(ctx, t.TempDir(), append(append([]string{}, base...), env...), DefaultTestTimeout, "")
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	root := "GOROOT=" + strings.TrimSpace(string(goroot))
	plain := id()
	seen := map[string]string{plain: "none"}
	alias := filepath.Join(t.TempDir(), "goroot-alias")
	if err := os.Symlink(strings.TrimSpace(string(goroot)), alias); err != nil {
		t.Fatal(err)
	}
	if got := id(root); got != plain {
		t.Errorf("%s names the go command's own GOROOT, but changes the identity %q to %q", root, plain, got)
	}
	for _, kv := range []string{"GOROOT=" + alias, "GOROOT=" + strings.TrimSpace(string(goroot)) + string(filepath.Separator)} {
		if got := id(kv); got == plain || !strings.Contains(got, kv) {
			t.Errorf("%s is another spelling runtime.GOROOT reports, but the identity %q does not bind it", kv, got)
		}
	}
	for _, kv := range []string{"GOTRACEBACK=all", "LD_LIBRARY_PATH=/linecov-absent", "GLIBC_TUNABLES=glibc.malloc.check=0", "DYLD_LIBRARY_PATH=/linecov-absent"} {
		got := id(kv)
		if !strings.Contains(got, kv) {
			t.Errorf("%s: identity %q does not name it", kv, got)
		}
		if other, dup := seen[got]; dup {
			t.Errorf("%s and %s share identity %q", kv, other, got)
		}
		seen[got] = kv
	}
	if !strings.Contains(plain, `runtime=""`) {
		t.Fatalf("no runtime-read variable set: %q", plain)
	}
	if got := RuntimeSettings([]string{"LD_PRELOAD=/b.so", "GOGC=50", "DYLD_INSERT_LIBRARIES=/a.dylib", "GOTRACEBACK=crash", "HOME=/h"}); got != "GOGC=50 GOTRACEBACK=crash DYLD_INSERT_LIBRARIES=/a.dylib LD_PRELOAD=/b.so" {
		t.Fatalf("RuntimeSettings = %q", got)
	}
	// A GOROOT naming another directory than the default stays bound.
	other := t.TempDir()
	if got := RuntimeSettings(dropDefaultGOROOT([]string{"GOROOT=" + other, "GOGC=50"}, strings.TrimSpace(string(goroot)))); got != "GOGC=50 GOROOT="+other {
		t.Fatalf("another GOROOT: RuntimeSettings = %q", got)
	}
	if got := RuntimeSettings(dropDefaultGOROOT([]string{"GOROOT=" + alias, "GOGC=50"}, strings.TrimSpace(string(goroot)))); got != "GOGC=50 GOROOT="+alias {
		t.Fatalf("an alias GOROOT: RuntimeSettings = %q", got)
	}
	if got := RuntimeSettings(dropDefaultGOROOT([]string{"GOROOT=" + strings.TrimSpace(string(goroot)), "GOGC=50"}, strings.TrimSpace(string(goroot)))); got != "GOGC=50" {
		t.Fatalf("the default GOROOT: RuntimeSettings = %q", got)
	}
}

// cFixture links C code three ways: cc compiles it, usecc imports cc, and
// e2e.TestTool builds and runs a module binary that imports cc. plain links
// none and starts no child.
var cFixture = map[string]string{
	"go.mod": "module example.com/m\n\ngo 1.22\n",
	"cc/cc.go": `package cc

// int seven(void) { return 7; }
import "C"

func Seven() int { return int(C.seven()) }
`,
	"cc/cc_test.go": `package cc

import "testing"

func TestSeven(t *testing.T) {
	if Seven() != 7 {
		t.Fatal("seven")
	}
}
`,
	"usecc/usecc.go": `package usecc

import "example.com/m/cc"

func Use() int { return cc.Seven() + 1 }
`,
	"usecc/usecc_test.go": `package usecc

import "testing"

func TestUse(t *testing.T) {
	if Use() != 8 {
		t.Fatal("use")
	}
}
`,
	"plain/plain.go": `package plain

func Plain() int { return 1 }
`,
	"plain/plain_test.go": `package plain

import "testing"

func TestPlain(t *testing.T) {
	if Plain() != 1 {
		t.Fatal("plain")
	}
}
`,
	"cmd/tool/main.go": `package main

import (
	"fmt"

	"example.com/m/cc"
)

func main() { fmt.Println(cc.Seven()) }
`,
	"e2e/e2e_test.go": `package e2e

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTool(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "tool")
	if out, err := exec.Command("go", "build", "-o", bin, "../cmd/tool").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	out, err := exec.Command(bin).Output()
	if err != nil || strings.TrimSpace(string(out)) != "7" {
		t.Fatalf("tool: %v %q", err, out)
	}
}
`,
}

// GLS-V0-053: C code reads the environment through getenv, outside the os
// audit, so every record of a package whose test binary links non-standard
// C code, and every record that started an instrumented child of a module
// that has such code, binds the whole environment as collector-modified;
// a package that links none and starts no child binds only what it read.
func TestCollectBindsWholeEnvironmentForCCode(t *testing.T) {
	requireGo(t)
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("needs a C compiler for cgo")
	}
	t.Setenv("CGO_ENABLED", "1")
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, cFixture)
	outcomes, err := Collect(ctx, dir, CollectOptions{Module: "example.com/m", Packages: []string{"cc", "e2e", "plain", "usecc"}, Work: t.TempDir(), Parallel: 4, Timeout: 5 * time.Minute, SuiteTimeout: 5 * time.Minute, TestTimeout: DefaultTestTimeout})
	if err != nil {
		t.Fatal(err)
	}
	whole := map[string]bool{}
	for _, o := range outcomes {
		if o.Record == nil {
			t.Fatalf("%s.%s diverged: %s", o.Package, o.Name, o.Divergent)
		}
		for _, b := range o.Record.Env {
			if b.Name == EnvAll {
				if b.Value != EnvModified {
					t.Errorf("%s.%s binds the whole environment as %q", o.Package, o.Name, b.Value)
				}
				whole[o.Package+"."+o.Name] = true
			}
		}
	}
	for _, key := range []string{"cc.TestSeven", "cc." + InOrder, "usecc.TestUse", "usecc." + InOrder, "e2e.TestTool", "e2e." + InOrder} {
		if !whole[key] {
			t.Errorf("%s does not bind the whole environment", key)
		}
	}
	for _, key := range []string{"plain.TestPlain", "plain." + InOrder} {
		if whole[key] {
			t.Errorf("%s binds the whole environment, but links no C code and starts no child", key)
		}
	}
}
