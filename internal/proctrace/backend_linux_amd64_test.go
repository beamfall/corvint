// SPDX-License-Identifier: Apache-2.0
//go:build linux && amd64

package proctrace

import (
	"bytes"
	"context"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRSPV0021Read(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "data"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var out bytes.Buffer
	r := New().Trace(ctx, Request{Executable: "/bin/cat", Args: []string{"data"}, Dir: root, Repositories: []Repository{{ID: "main", Root: root}}, EventCap: 10000, PathCap: 1000, Stdout: &out})
	if !r.Outcome.Started || r.Outcome.ExitCode != 0 || out.String() != "hello" {
		t.Fatalf("trace failed: %+v output=%q", r, out.String())
	}
	for _, p := range r.Reads.Entries {
		if p.Path == "data" {
			return
		}
	}
	t.Fatalf("read missing: %+v", r)
}

// RSP-V0-021: inherited descriptors can expose file size without open or read.
func TestRSPV0021InheritedStdinSeek(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "fixture")
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Fatal("Linux syscall fixtures require cc: ", err)
	}
	build := exec.Command(cc, "-nostdlib", "-static", "-no-pie", "-o", exe, "testdata/lseek.S")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture compiler: %v %s", err, output)
	}
	for _, size := range []int{5, 9} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			name := filepath.Join(root, "data")
			if err := os.WriteFile(name, bytes.Repeat([]byte("x"), size), 0600); err != nil {
				t.Fatal(err)
			}
			stdin, err := os.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			r := New().Trace(ctx, Request{Executable: exe, Dir: root, Stdin: stdin, Repositories: []Repository{{ID: "main", Root: root}}, EventCap: 1000, PathCap: 100})
			if !r.Outcome.Started || r.Outcome.ExitCode != size || r.Outcome.Signal != 0 || r.Outcome.Error != "" {
				t.Fatalf("size-dependent fixture failed: %+v", r)
			}
			if r.Reads.State != Present || !hasPath(r.Reads.Entries, "data") {
				t.Fatalf("inherited stdin size dependency missing: %+v", r)
			}
		})
	}
}

// RSP-V0-007/021/022: retargeting the name used by inherited stdin must either
// match a lexical dependency or leave the trace incomplete.
func TestRSPV0021InheritedStdinSymlinkRetarget(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "fixture")
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(cc, "-nostdlib", "-static", "-no-pie", "-o", exe, "testdata/lseek.S").CombinedOutput(); err != nil {
		t.Fatalf("fixture compiler: %v %s", err, output)
	}
	link := filepath.Join(root, "link")
	for i, target := range []string{"a", "b"} {
		size := 5 + i*4
		if err := os.WriteFile(filepath.Join(root, target), bytes.Repeat([]byte("x"), size), 0600); err != nil {
			t.Fatal(err)
		}
		if i != 0 {
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		stdin, err := os.Open(link)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		r := New().Trace(ctx, Request{Executable: exe, Dir: root, Stdin: stdin, Repositories: []Repository{{ID: "main", Root: root}}, EventCap: 1000, PathCap: 100})
		cancel()
		stdin.Close()
		if !r.Outcome.Started || r.Outcome.ExitCode != size || r.Outcome.Signal != 0 || r.Outcome.Error != "" {
			t.Fatalf("retargeted fixture failed: %+v", r)
		}
		if r.Reads.State != Present || !hasPath(r.Reads.Entries, "link") || !hasPath(r.Reads.Entries, target) {
			t.Fatalf("inherited lexical dependency missing: %+v", r)
		}
	}
	stdin, err := os.Open(link)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	fd, err := syscall.Dup(int(stdin.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	unnamed := os.NewFile(uintptr(fd), "provenance-unavailable")
	defer unnamed.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := New().Trace(ctx, Request{Executable: exe, Dir: root, Stdin: unnamed, Repositories: []Repository{{ID: "main", Root: root}}, EventCap: 1000, PathCap: 100})
	if r.Outcome.ExitCode != 9 || r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "unresolved-descriptor-path") {
		t.Fatalf("unknown inherited provenance did not widen: %+v", r)
	}
}

func fixture(t *testing.T, mode string) Request {
	t.Helper()
	root := t.TempDir()
	exe := filepath.Join(root, "fixture")
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Fatal("Linux syscall fixtures require cc: ", err)
	}
	build := exec.Command(cc, "-O0", "-Wall", "-Wextra", "-Werror", "-o", exe, "testdata/syscalls.c")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture compiler: %v %s", err, output)
	}
	if mode == "calls" {
		b, e := os.ReadFile(exe)
		if e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(root, "exec-target"), b, 0700); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"data", "stat-data", "access-data", "child", "append", "overwrite", "truncate"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("prior"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return Request{Executable: exe, Args: []string{mode}, Dir: root, Repositories: []Repository{{ID: "main", Root: root}}, Outside: []OutsideClass{{"toolchain", "/usr"}, {"toolchain", "/lib"}, {"toolchain", "/etc"}, {"temporary", "/dev"}}, EventCap: 10000, PathCap: 1000}
}
func traceFixture(t *testing.T, q Request) Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := New().Trace(ctx, q)
	if r.Outcome.ExitCode != 0 || r.Outcome.Error != "" {
		t.Fatalf("fixture failed: %+v", r)
	}
	return r
}
func hasPath(paths []Path, name string) bool {
	for _, p := range paths {
		if p.Path == name {
			return true
		}
	}
	return false
}
func hasReason(reasons []string, reason string) bool {
	for _, r := range reasons {
		if r == reason {
			return true
		}
	}
	return false
}

func TestRSPV0022Syscalls(t *testing.T) {
	q := fixture(t, "calls")
	r := traceFixture(t, q)
	for _, name := range []string{"data", "stat-data", "access-data", "child"} {
		if !hasPath(r.Reads.Entries, name) {
			t.Errorf("missing read %s: %+v", name, r)
		}
	}
	if !hasPath(r.Listings.Entries, ".") || !hasPath(r.Lookups.Entries, "missing") {
		t.Errorf("missing listing/lookup: %+v", r)
	}
	if hasPath(r.Reads.Entries, "replaced") {
		t.Errorf("own replacement read retained: %+v", r)
	}
	var replaced bool
	for _, w := range r.Writes.Entries {
		if w.Path.Path == "replaced" && w.Kind == "replace" {
			replaced = true
		}
	}
	execObserved := false
	for _, e := range r.Execs.Entries {
		if e.Path.Path == "exec-target" && len(e.Digest) == 64 {
			execObserved = true
		}
	}
	for _, reason := range r.Reads.Reasons {
		if reason != "concurrent-filesystem-path-race" {
			t.Errorf("supported syscall widened unexpectedly: %s", reason)
		}
	}
	if !replaced || !execObserved {
		t.Errorf("missing replace/exec: %+v", r)
	}
	// A failed connection is an attempted outside contact too; it cannot narrow.
	if len(r.External.Entries) == 0 && r.External.State == Present {
		t.Errorf("contact lost: %+v", r)
	}
}
func TestRSPV0022Descendants(t *testing.T) {
	for _, mode := range []string{"fork", "vfork", "clone", "exec"} {
		t.Run(mode, func(t *testing.T) {
			r := traceFixture(t, fixture(t, mode))
			if !hasPath(r.Reads.Entries, "child") {
				t.Fatalf("child read lost: %+v", r)
			}
		})
	}
}
func TestRSPV0022Incomplete(t *testing.T) {
	for _, mode := range []string{"unknown", "malformed", "setuid", "event-cap", "path-cap"} {
		t.Run(mode, func(t *testing.T) {
			action := mode
			if strings.HasSuffix(mode, "cap") {
				action = "child"
			}
			q := fixture(t, action)
			if mode == "setuid" {
				b, err := os.ReadFile(q.Executable)
				if err != nil {
					t.Fatal(err)
				}
				p := filepath.Join(q.Dir, "privileged")
				if err := os.WriteFile(p, b, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(p, 0700|os.ModeSetuid); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "event-cap" {
				q.EventCap = 1
			}
			if mode == "path-cap" {
				q.PathCap = 1
			}
			r := traceFixture(t, q)
			if r.Reads.State != Incomplete || r.Execs.State != Incomplete || len(r.Reads.Reasons) == 0 {
				t.Fatalf("not incomplete: %+v", r)
			}
			reason := map[string]string{"unknown": "unsupported-syscall:304", "malformed": "malformed-open-how", "setuid": "privileged-executable", "event-cap": "event-cap", "path-cap": "path-cap"}[mode]
			if !hasReason(r.Reads.Reasons, reason) {
				t.Fatalf("missing %s: %+v", reason, r)
			}
		})
	}
}
func TestRSPV0007WriteKinds(t *testing.T) {
	r := traceFixture(t, fixture(t, "writes"))
	if hasPath(r.Reads.Entries, "created") {
		t.Fatalf("own create retained: %+v", r)
	}
	for _, name := range []string{"append", "overwrite", "truncate"} {
		if !hasPath(r.Reads.Entries, name) {
			t.Errorf("prior dependency %s lost", name)
		}
		found := false
		for _, w := range r.Writes.Entries {
			if w.Path.Path == name && w.Kind == name {
				found = true
			}
		}
		if !found {
			t.Errorf("write %s lost: %+v", name, r)
		}
	}
}

// RSP-V0-007/021: success depends on an existing file even without a later read.
func TestRSPV0007WriteOnlyOpenDependency(t *testing.T) {
	q := fixture(t, "write-open")
	r := traceFixture(t, q)
	if r.Reads.State != Present || !hasPath(r.Reads.Entries, "data") {
		t.Fatalf("write-only open lost existence dependency: %+v", r)
	}
	if err := os.Remove(filepath.Join(q.Dir, "data")); err != nil {
		t.Fatal(err)
	}
	r = New().Trace(context.Background(), q)
	if !r.Outcome.Started || r.Outcome.ExitCode != 91 || r.Outcome.Error != "" {
		t.Fatalf("deleting the opened file did not change the result: %+v", r)
	}
}

// RSP-V0-021/022: outside classification cannot hide a kernel-loaded repository input.
func TestRSPV0021OutsideExecutableCopiedLoader(t *testing.T) {
	q := fixture(t, "child")
	image, err := elf.Open(q.Executable)
	if err != nil {
		t.Fatal(err)
	}
	defer image.Close()
	var interpreter string
	for _, program := range image.Progs {
		if program.Type == elf.PT_INTERP {
			raw := make([]byte, program.Filesz)
			if _, err := program.ReadAt(raw, 0); err != nil {
				t.Fatal(err)
			}
			interpreter = strings.TrimSuffix(string(raw), "\x00")
		}
	}
	if interpreter == "" {
		t.Fatal("fixture has no ELF interpreter")
	}
	loader, err := os.ReadFile(interpreter)
	if err != nil {
		t.Fatal(err)
	}
	localInterpreter := filepath.Join(q.Dir, "interpreter")
	if err := os.WriteFile(localInterpreter, loader, 0700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	q.Executable = filepath.Join(outside, "fixture")
	build := exec.Command("cc", "-O0", "-Wall", "-Wextra", "-Werror", "-Wl,--dynamic-linker="+localInterpreter, "-o", q.Executable, "testdata/syscalls.c")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture compiler: %v %s", err, output)
	}
	q.Outside = append(q.Outside, OutsideClass{"toolchain", outside})
	r := traceFixture(t, q)
	if r.Reads.State != Present || !hasPath(r.Reads.Entries, "child") || !hasPath(r.Reads.Entries, "interpreter") {
		t.Fatalf("outside executable lost repository interpreter dependency: %+v", r)
	}
	if len(r.Execs.Entries) != 0 {
		t.Fatalf("outside executable retained as repository executable: %+v", r.Execs)
	}
}

func TestRSPV0007Symlink(t *testing.T) {
	q := fixture(t, "symlink")
	if err := os.Symlink("data", filepath.Join(q.Dir, "link")); err != nil {
		t.Fatal(err)
	}
	r := traceFixture(t, q)
	for _, p := range r.Reads.Entries {
		if p.Path == "link" && p.ResolvedPath == "data" && p.Repository == "main" && p.ResolvedRepository == "main" {
			return
		}
	}
	t.Fatalf("link paths missing: %+v", r)
}
func TestRSPV0049Repositories(t *testing.T) {
	for _, declared := range []bool{true, false} {
		t.Run(fmt.Sprint(declared), func(t *testing.T) {
			q := fixture(t, "outside")
			sibling := t.TempDir()
			p := filepath.Join(sibling, "sibling-data")
			if err := os.WriteFile(p, []byte("sibling"), 0600); err != nil {
				t.Fatal(err)
			}
			q.Args = append(q.Args, p)
			if declared {
				q.Repositories = append(q.Repositories, Repository{ID: "sibling", Root: sibling})
			}
			r := traceFixture(t, q)
			if declared {
				for _, path := range r.Reads.Entries {
					if path.ResolvedRepository == "sibling" && path.ResolvedPath == "sibling-data" {
						return
					}
				}
				t.Fatalf("sibling lost: %+v", r)
			}
			if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "undeclared-outside-read") || r.Outside["other"] == 0 {
				t.Fatalf("outside unknown lost: %+v", r)
			}
		})
	}
}
func TestRSPV0024LiveParity(t *testing.T) {
	for _, mode := range []string{"child", "fork", "vfork", "clone", "exec", "signal"} {
		t.Run(mode, func(t *testing.T) {
			q := fixture(t, mode)
			cmd := exec.Command(q.Executable, q.Args...)
			cmd.Dir = q.Dir
			start := time.Now()
			err := cmd.Run()
			duration := time.Since(start)
			outcome := Outcome{Started: cmd.ProcessState != nil, ExitCode: cmd.ProcessState.ExitCode()}
			if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				outcome.Signal = int(status.Signal())
			}
			if err != nil && outcome.ExitCode >= 0 {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			r := New().Trace(ctx, q)
			qualification := Compare("native-fixture/"+mode, outcome, duration, &r)
			if !qualification.Enabled {
				t.Fatalf("parity: %+v trace=%+v", qualification, r)
			}
			t.Logf("runner=%s untraced_ns=%d traced_ns=%d overhead_ns=%d", qualification.Runner, duration, r.Duration, qualification.Overhead)
		})
	}
}
func TestRSPV0021Cancellation(t *testing.T) {
	q := fixture(t, "pause")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r := New().Trace(ctx, q)
	if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "cancelled") || r.Outcome.Signal != int(syscall.SIGKILL) {
		t.Fatalf("cancel failed: %+v", r)
	}
}

func TestRSPV0021CompleteSingleProcess(t *testing.T) {
	r := traceFixture(t, fixture(t, "child"))
	if r.Reads.State != Present || r.Listings.State != Present || r.Lookups.State != Present || r.Writes.State != Present || r.Execs.State != Present || r.External.State != Present {
		t.Fatalf("simple trace incomplete: %+v", r)
	}
}
func TestRSPV0022RefusedStart(t *testing.T) {
	q := fixture(t, "child")
	q.Executable = filepath.Join(q.Dir, "nonexistent")
	r := New().Trace(context.Background(), q)
	if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "trace-start-refused") || r.Outcome.Started {
		t.Fatalf("refused start fabricated evidence: %+v", r)
	}
}
func TestRSPV0022InvalidConfiguration(t *testing.T) {
	for _, mutate := range []func(*Request){
		func(q *Request) { q.EventCap = 0 }, func(q *Request) { q.PathCap = 0 },
		func(q *Request) { q.Executable = "fixture" }, func(q *Request) { q.Repositories = nil },
		func(q *Request) { q.Repositories = append(q.Repositories, q.Repositories[0]) },
		func(q *Request) { q.Outside = []OutsideClass{{"unrecognized", "/"}} },
	} {
		q := fixture(t, "child")
		mutate(&q)
		r := New().Trace(context.Background(), q)
		if r.Reads.State != Incomplete || r.Outcome.Started {
			t.Fatal(r)
		}
	}
}

func TestRSPV0022GoChild(t *testing.T) {
	if os.Getenv("RSP_TRACE_GO_CHILD") != "1" {
		return
	}
	data, err := os.ReadFile("child")
	if err != nil || string(data) != "prior" {
		os.Exit(97)
	}
	os.Exit(0)
}
func TestRSPV0024GoRuntimeParity(t *testing.T) {
	q := fixture(t, "child")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	q.Executable = executable
	q.Args = []string{"-test.run=^TestRSPV0022GoChild$"}
	q.Env = append(os.Environ(), "RSP_TRACE_GO_CHILD=1")
	q.Outside = append(q.Outside, OutsideClass{"temporary", filepath.Dir(executable)})
	cmd := exec.Command(q.Executable, q.Args...)
	cmd.Dir = q.Dir
	cmd.Env = q.Env
	start := time.Now()
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	duration := time.Since(start)
	r := traceFixture(t, q)
	if !hasPath(r.Reads.Entries, "child") {
		t.Fatalf("Go read missing: %+v", r)
	}
	parity := Compare("go-runtime-fixture", Outcome{Started: true, ExitCode: 0}, duration, &r)
	if !parity.Enabled {
		t.Fatal(parity)
	}
	t.Logf("runner=%s untraced_ns=%d traced_ns=%d overhead_ns=%d", parity.Runner, duration, r.Duration, parity.Overhead)
}

func TestRSPV0022FilterCoverage(t *testing.T) {
	required := map[uint32]bool{0: true, 1: true, 9: true, 42: true, 56: true, 57: true, 58: true, 59: true, 200: true, 202: true, 217: true, 234: true, 257: true, 316: true, 322: true, 332: true, 437: true, 439: true, 0xffffffff: true}
	for _, call := range safeCalls {
		if required[call] {
			t.Errorf("observed or unknown syscall %d bypasses filter", call)
		}
	}
}

func TestRSPV0007DirectorySymlink(t *testing.T) {
	q := fixture(t, "outside")
	q.Args = append(q.Args, "alias/data")
	if err := os.Symlink(".", filepath.Join(q.Dir, "alias")); err != nil {
		t.Fatal(err)
	}
	r := traceFixture(t, q)
	if !hasPath(r.Reads.Entries, "alias") || !hasPath(r.Reads.Entries, "alias/data") {
		t.Fatalf("intermediate symlink dependency lost: %+v", r)
	}
}
func TestRSPV0007UnsupportedMatching(t *testing.T) {
	if matchingReason(0xef53, 0) != "" {
		t.Fatal("known case-sensitive filesystem refused")
	}
	if matchingReason(0xef53, 0x40000000) == "" || matchingReason(0, 0) == "" {
		t.Fatal("unknown matching admitted")
	}
}

// RSP-V0-022: pathname/peer calls and unknown forms must never bypass observation.
func TestRSPV0022ClosedSyscallForms(t *testing.T) {
	for _, tt := range []struct{ mode, reason string }{
		{"utimensat", "unsupported-syscall:280"},
		{"sigqueue", "unsupported-syscall:129"},
		{"future", "unsupported-syscall:1073741823"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			q := fixture(t, tt.mode)
			r := traceFixture(t, q)
			if r.Reads.State != Incomplete || r.External.State != Incomplete || !hasReason(r.Reads.Reasons, tt.reason) {
				t.Fatalf("unsupported effect admitted: %+v", r)
			}
			if tt.mode == "utimensat" {
				if err := os.Remove(filepath.Join(q.Dir, "data")); err != nil {
					t.Fatal(err)
				}
				r = New().Trace(context.Background(), q)
				if r.Outcome.ExitCode != 1 || r.Reads.State != Incomplete {
					t.Fatalf("missing path: %+v", r)
				}
			}
		})
	}
	// A known syscall number with an unknown selector is also unproven.
	for _, regs := range []syscall.PtraceRegs{
		{Orig_rax: 158, Rdi: 0xffff},
		{Orig_rax: 302, Rdi: 1},
		{Orig_rax: 273, Rsi: 32},
		{Orig_rax: 318, Rdx: 8},
		{Orig_rax: 334, Rsi: 64},
	} {
		r := emptyResult("test")
		c := collector{r: &r}
		regs.Cs = 0x33
		c.enter(0, regs)
		if r.Reads.State != Incomplete {
			t.Fatalf("unknown form admitted: %+v", regs)
		}
	}
}

// RSP-V0-021: both operands and ancestors matter for ambiguous failed lookups.
func TestRSPV0021RenameDestinationParent(t *testing.T) {
	for _, variant := range []string{"rename", "renameat", "renameat2"} {
		t.Run(variant, func(t *testing.T) {
			q := fixture(t, "rename-parent")
			q.Args = append(q.Args, variant)
			r := New().Trace(context.Background(), q)
			if r.Outcome.ExitCode != 1 || r.Lookups.State != Present {
				t.Fatalf("expected ENOENT: %+v", r)
			}
			for _, name := range []string{"data", "missing/out", "missing"} {
				if !hasPath(r.Lookups.Entries, name) {
					t.Errorf("missing operand/ancestor %s: %+v", name, r)
				}
			}
			if err := os.Mkdir(filepath.Join(q.Dir, "missing"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(q.Dir, "missing/anything"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			traceFixture(t, q) // The sibling addition changes failure into success.
		})
	}
}

// RSP-V0-007/021: replacing a link does not create the untouched target.
func TestRSPV0007RenameOverSymlink(t *testing.T) {
	for _, variant := range []string{"rename", "renameat", "renameat2"} {
		t.Run(variant, func(t *testing.T) {
			q := fixture(t, "rename-symlink")
			q.Args = append(q.Args, variant)
			if err := os.Symlink("data", filepath.Join(q.Dir, "dest")); err != nil {
				t.Fatal(err)
			}
			r := traceFixture(t, q)
			if r.Reads.State != Present || !hasPath(r.Reads.Entries, "data") {
				t.Fatalf("read of untouched symlink target lost: %+v", r)
			}
			// The created source does not make the existing destination
			// irrelevant: a later directory at dest would fail the rename.
			if !hasPath(r.Reads.Entries, "dest") {
				t.Fatalf("existing rename destination dependency lost: %+v", r)
			}
			var replaced bool
			for _, w := range r.Writes.Entries {
				if w.Kind == "replace" {
					if w.Path.Path != "dest" || w.Path.ResolvedPath != "dest" {
						t.Fatalf("rename followed destination link: %+v", w)
					}
					replaced = true
				}
			}
			if !replaced {
				t.Fatal("rename not observed")
			}
			if b, err := os.ReadFile(filepath.Join(q.Dir, "data")); err != nil || string(b) != "prior" {
				t.Fatalf("target was not preserved: %q %v", b, err)
			}
		})
	}
}

// RSP-V0-007: a created source renamed over an existing regular file keeps
// the destination as a dependency; a later directory at dest would fail it.
func TestRSPV0007RenameOverRegularDestination(t *testing.T) {
	for _, variant := range []string{"rename", "renameat", "renameat2"} {
		t.Run(variant, func(t *testing.T) {
			q := fixture(t, "rename-symlink")
			q.Args = append(q.Args, variant)
			if err := os.WriteFile(filepath.Join(q.Dir, "dest"), []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			r := traceFixture(t, q)
			if r.Reads.State != Present || !hasPath(r.Reads.Entries, "dest") {
				t.Fatalf("existing regular rename destination dependency lost: %+v", r)
			}
			var replaced bool
			for _, w := range r.Writes.Entries {
				if w.Kind == "replace" && w.Path.Path == "dest" {
					replaced = true
				}
			}
			if !replaced {
				t.Fatal("rename not observed")
			}
		})
	}
}

// RSP-V0-007: every hop remains a dependency when only the middle link changes.
func TestRSPV0007ChainedSymlink(t *testing.T) {
	q := fixture(t, "byte")
	for _, name := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(q.Dir, name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(q.Dir, name, "data"), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("bridge", filepath.Join(q.Dir, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if err := os.Symlink(name, filepath.Join(q.Dir, "bridge")); err != nil {
			t.Fatal(err)
		}
		r := New().Trace(context.Background(), q)
		if r.Outcome.ExitCode != int(name[0]) || r.Reads.State != Present {
			t.Fatalf("wrong bytes: %+v", r)
		}
		for _, link := range []string{"alias", "bridge", "alias/data"} {
			if !hasPath(r.Reads.Entries, link) {
				t.Errorf("lost hop %s: %+v", link, r)
			}
		}
		if err := os.Remove(filepath.Join(q.Dir, "bridge")); err != nil {
			t.Fatal(err)
		}
	}
}

// RSP-V0-007/012(e)/021: additions behind a dangling link remain selectable.
func TestRSPV0007DanglingSymlink(t *testing.T) {
	for _, form := range []string{"relative", "absolute", "chained"} {
		t.Run(form, func(t *testing.T) {
			q := fixture(t, "lookup")
			target := "missing/data"
			if form == "absolute" {
				target = filepath.Join(q.Dir, target)
			} else if form == "chained" {
				if err := os.Symlink(target, filepath.Join(q.Dir, "bridge")); err != nil {
					t.Fatal(err)
				}
				target = "bridge"
			}
			if err := os.Symlink(target, filepath.Join(q.Dir, "alias")); err != nil {
				t.Fatal(err)
			}
			r := New().Trace(context.Background(), q)
			if r.Outcome.ExitCode != 1 || r.Lookups.State != Present || r.Reads.State != Present {
				t.Fatalf("expected complete failed lookup: %+v", r)
			}
			for _, name := range []string{"missing/data", "missing", "."} {
				want := Path{Repository: "main", Path: name, ResolvedRepository: "main", ResolvedPath: name}
				found := false
				for _, path := range r.Lookups.Entries {
					found = found || path == want
				}
				if !found {
					t.Errorf("missing expanded lookup %s: %+v", name, r)
				}
			}
			if !hasPath(r.Reads.Entries, "alias") || (form == "chained" && !hasPath(r.Reads.Entries, "bridge")) {
				t.Fatalf("lost link dependency: %+v", r)
			}
			if err := os.Mkdir(filepath.Join(q.Dir, "missing"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(q.Dir, "missing/data"), []byte("new"), 0600); err != nil {
				t.Fatal(err)
			}
			traceFixture(t, q) // Adding only the expanded target changes the outcome.
		})
	}
}

// RSP-V0-007/022: a suffix outside the ordinary-name allowlist is unproven.
func TestRSPV0007UnknownMissingSuffix(t *testing.T) {
	q := fixture(t, "lookup")
	if err := os.Symlink("missing/../data", filepath.Join(q.Dir, "alias")); err != nil {
		t.Fatal(err)
	}
	r := New().Trace(context.Background(), q)
	if r.Outcome.ExitCode != 1 || r.Reads.State != Incomplete || r.Lookups.State != Incomplete ||
		!hasReason(r.Lookups.Reasons, "symlink-resolution-failed") {
		t.Fatalf("uncertified suffix admitted: %+v", r)
	}
	if err := os.Mkdir(filepath.Join(q.Dir, "missing"), 0700); err != nil {
		t.Fatal(err)
	}
	traceFixture(t, q) // Lexically cleaning the target would have missed this input.
}

func TestRSPV0007UnprovenLinkForms(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("cycle", filepath.Join(root, "cycle")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "special"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{filepath.Join(root, "cycle"), filepath.Join(root, "special"), "/proc/self/exe", "relative"} {
		r := emptyResult("test")
		c := collector{q: Request{Repositories: []Repository{{ID: "main", Root: root}}, PathCap: 100}, r: &r, paths: map[Path]bool{}}
		c.linkDependencies(name)
		if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "symlink-resolution-failed") {
			t.Fatalf("unproven form admitted %s: %+v", name, r)
		}
	}
}

// RSP-V0-007/022: ignored or unknown open flags cannot establish own-write provenance.
func TestRSPV0007OpenProvenance(t *testing.T) {
	q := fixture(t, "path-create")
	for _, value := range []byte{'a', 'b'} {
		if err := os.WriteFile(filepath.Join(q.Dir, "data"), []byte{value}, 0600); err != nil {
			t.Fatal(err)
		}
		r := New().Trace(context.Background(), q)
		if r.Outcome.ExitCode != int(value) || r.Reads.State != Present || !hasPath(r.Reads.Entries, "data") {
			t.Fatalf("existing-file read suppressed: %+v", r)
		}
		for _, w := range r.Writes.Entries {
			if w.Path.Path == "data" {
				t.Fatalf("O_PATH invented a write: %+v", w)
			}
		}
	}
	q = fixture(t, "open-future")
	r := traceFixture(t, q)
	if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "unsupported-syscall:2") || !hasPath(r.Reads.Entries, "made") {
		t.Fatalf("unknown open form established ownership: %+v", r)
	}
	for _, flags := range []uint64{syscall.O_ACCMODE, syscall.O_EXCL, syscall.O_RDONLY | syscall.O_TRUNC,
		syscall.O_CREAT | syscall.O_DIRECTORY, 0x400000 | syscall.O_DIRECTORY | syscall.O_RDWR, 1 << 63} {
		if supportedOpenFlags(flags) {
			t.Errorf("unproven flags admitted: %#x", flags)
		}
	}
}

// RSP-V0-007/022: access mode is not creation provenance, including unknown forms.
func TestRSPV0007OpenExistenceProvenance(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "data")
	for _, tc := range []struct {
		name                 string
		flags                uint64
		wantRead, incomplete bool
	}{
		{"write-only", syscall.O_WRONLY, true, false},
		{"nonexclusive-create", syscall.O_WRONLY | syscall.O_CREAT, true, false},
		{"exclusive-create", syscall.O_WRONLY | syscall.O_CREAT | syscall.O_EXCL, false, false},
		{"unknown-create-form", syscall.O_WRONLY | syscall.O_CREAT | syscall.O_EXCL | 1<<63, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := emptyResult("test")
			c := collector{q: Request{Repositories: []Repository{{ID: "main", Root: root}}, PathCap: 100}, r: &r, paths: map[Path]bool{}, created: map[string]bool{}}
			c.finish(&call{kind: "open", lexical: path, resolved: path, flags: tc.flags}, 3)
			if hasPath(r.Reads.Entries, "data") != tc.wantRead || (r.Reads.State == Incomplete) != tc.incomplete {
				t.Fatalf("open existence/provenance: %+v", r)
			}
			if tc.incomplete && c.created[path] {
				t.Fatal("unknown form established creation provenance")
			}
		})
	}
}

// RSP-V0-007: deletion and rename affect a successful open without any read.
func TestRSPV0007WriteOnlyOpenExistence(t *testing.T) {
	for _, mutation := range []string{"delete", "rename"} {
		t.Run(mutation, func(t *testing.T) {
			q := fixture(t, "write-open")
			r := traceFixture(t, q)
			if r.Reads.State != Present || !hasPath(r.Reads.Entries, "data") {
				t.Fatalf("write-only existence dependency missing: %+v", r)
			}
			path := filepath.Join(q.Dir, "data")
			var err error
			if mutation == "delete" {
				err = os.Remove(path)
			} else {
				err = os.Rename(path, filepath.Join(q.Dir, "renamed"))
			}
			if err != nil {
				t.Fatal(err)
			}
			after := New().Trace(context.Background(), q)
			if !after.Outcome.Started || after.Outcome.ExitCode == 0 || !hasPath(after.Lookups.Entries, "data") {
				t.Fatalf("mutation did not expose existence dependence: %+v", after)
			}
		})
	}
}

// RSP-V0-007/021/022: successful mutations observe existence without a later read.
// Only understood exclusive creation or prior tree-owned creation exempts a path.
func TestRSPV0007WriteSuccessDependencies(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags uint64
	}{
		{"open", syscall.O_WRONLY},
		{"append", syscall.O_WRONLY | syscall.O_APPEND},
		{"open-truncate", syscall.O_WRONLY | syscall.O_TRUNC},
		{"truncate", 0},
		{"unknown", syscall.O_WRONLY | 0x40000000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := fixture(t, "write-dependency")
			q.Args = append(q.Args, tc.name, fmt.Sprint(tc.flags), "base")
			r := traceFixture(t, q)
			wantState := Present
			if tc.name == "unknown" {
				wantState = Incomplete
			}
			if r.Reads.State != wantState || !hasPath(r.Reads.Entries, "data") {
				t.Fatalf("success dependency lost without subsequent read: %+v", r)
			}
			if err := os.Remove(filepath.Join(q.Dir, "data")); err != nil {
				t.Fatal(err)
			}
			r = New().Trace(context.Background(), q)
			if !r.Outcome.Started || r.Outcome.ExitCode != 1 || !hasPath(r.Lookups.Entries, "data") {
				t.Fatalf("deletion must change outcome and retain lookup: %+v", r)
			}
			if tc.name != "unknown" {
				q.Args[len(q.Args)-1] = "owned"
				r = traceFixture(t, q)
				if r.Reads.State != Present || hasPath(r.Reads.Entries, "made") {
					t.Fatalf("prior owned creation should prove independence: %+v", r)
				}
			}
		})
	}
}

// RSP-V0-007/022: outside classification cannot certify implicit kernel loads.
func TestRSPV0022OutsideExecutableInterpreter(t *testing.T) {
	q := fixture(t, "child")
	image, err := elf.Open(q.Executable)
	if err != nil {
		t.Fatal(err)
	}
	defer image.Close()
	var loader string
	for _, program := range image.Progs {
		if program.Type == elf.PT_INTERP {
			raw, err := io.ReadAll(io.LimitReader(program.Open(), 4097))
			if err != nil || len(raw) < 2 || len(raw) > 4096 {
				t.Fatalf("fixture interpreter: %q %v", raw, err)
			}
			loader = strings.TrimSuffix(string(raw), "\x00")
		}
	}
	if loader == "" {
		t.Fatal("fixture must have an ELF interpreter")
	}
	contents, err := os.ReadFile(loader)
	if err != nil {
		t.Fatal(err)
	}
	interpreter := filepath.Join(q.Dir, "interpreter")
	if err := os.WriteFile(interpreter, contents, 0700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	q.Outside = append(q.Outside, OutsideClass{"toolchain", outside})
	q.Executable = filepath.Join(outside, "launcher")
	build := exec.Command("cc", "-x", "c", "-Wl,--dynamic-linker="+interpreter, "-o", q.Executable, "-")
	build.Stdin = strings.NewReader("int main(void) { return 0; }\n")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("outside launcher compiler: %v %s", err, output)
	}
	t.Run("collector", func(t *testing.T) {
		r := emptyResult("test")
		c := collector{q: q, r: &r, paths: map[Path]bool{}, created: map[string]bool{}}
		c.recordExec(q.Executable, q.Executable)
		if r.Reads.State != Present || r.Execs.State != Present || !hasPath(r.Reads.Entries, "interpreter") || len(r.Execs.Entries) != 0 {
			t.Fatalf("outside implicit interpreter dependency: %+v", r)
		}
	})
	r := traceFixture(t, q)
	if r.Reads.State != Present || r.Execs.State != Present || !hasPath(r.Reads.Entries, "interpreter") || len(r.Execs.Entries) != 0 {
		t.Fatalf("outside executable lost repository interpreter: %+v", r)
	}
	if err := os.WriteFile(interpreter, []byte("unknown interpreter format\n"), 0700); err != nil {
		t.Fatal(err)
	}
	after := New().Trace(context.Background(), q)
	if after.Outcome.Started && after.Outcome.ExitCode == 0 {
		t.Fatalf("interpreter change did not affect outcome: %+v", after)
	}

}

func TestRSPV0022UnknownOutsideExecutableFormat(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	q := Request{Repositories: []Repository{{ID: "main", Root: root}}, Outside: []OutsideClass{{"toolchain", outside}}, PathCap: 100}
	q.Executable = filepath.Join(outside, "launcher")
	if err := os.WriteFile(q.Executable, []byte("future executable format\n"), 0700); err != nil {
		t.Fatal(err)
	}
	r := emptyResult("test")
	c := collector{q: q, r: &r, paths: map[Path]bool{}, created: map[string]bool{}}
	c.recordExec(q.Executable, q.Executable)
	if r.Execs.State != Incomplete || !hasReason(r.Execs.Reasons, "unsupported-executable-format") {
		t.Fatalf("unknown outside format certified: %+v", r)
	}
}

// RSP-V0-007/021/022: startup paths have the same dependency and matching obligations.
func TestRSPV0007StartupLinks(t *testing.T) {
	for _, kind := range []string{"executable", "directory"} {
		t.Run(kind, func(t *testing.T) {
			q := fixture(t, "cwd-byte")
			root := q.Dir
			for i, name := range []string{"first", "second"} {
				p := filepath.Join(root, name)
				if kind == "executable" {
					// Tiny executables avoid unrelated host utility IPC/runtime setup.
					build := exec.Command("cc", "-x", "c", "-o", p, "-")
					build.Stdin = strings.NewReader(fmt.Sprintf("int main(void) { return %d; }\n", i))
					if output, err := build.CombinedOutput(); err != nil {
						t.Fatalf("startup fixture compiler: %v %s", err, output)
					}
				} else {
					if err := os.Mkdir(p, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(p, "data"), []byte{byte(i)}, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := os.Symlink("bridge", filepath.Join(root, "launcher")); err != nil {
				t.Fatal(err)
			}
			if kind == "executable" {
				q.Executable = filepath.Join(root, "launcher")
			} else {
				q.Dir = filepath.Join(root, "launcher")
			}
			for i, name := range []string{"first", "second"} {
				if err := os.Symlink(name, filepath.Join(root, "bridge")); err != nil {
					t.Fatal(err)
				}
				r := New().Trace(context.Background(), q)
				if r.Outcome.ExitCode != i || r.Reads.State != Present || r.Execs.State != Present {
					t.Fatalf("startup result: %+v", r)
				}
				for _, link := range []string{"launcher", "bridge"} {
					if !hasPath(r.Reads.Entries, link) {
						t.Errorf("startup dependency lost: %s: %+v", link, r)
					}
				}
				if err := os.Remove(filepath.Join(root, "bridge")); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestRSPV0022UnknownStartupPaths(t *testing.T) {
	for _, kind := range []string{"executable", "directory"} {
		q := fixture(t, "outside")
		q.Args = append(q.Args, filepath.Join(q.Dir, "data"))
		// procfs magic links are outside the ordinary-textual-link allowlist.
		if kind == "executable" {
			q.Executable = "/proc/self/root" + q.Executable
		} else {
			q.Dir = "/proc/self/root" + q.Dir
		}
		r := traceFixture(t, q)
		if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "symlink-resolution-failed") {
			t.Fatalf("unknown startup %s admitted: %+v", kind, r)
		}
	}
}

// RSP-V0-021/022: a pre-existing peer can exchange repository-dependent bytes
// solely through mappings, even when the backing file is declared temporary.
func TestRSPV0021SharedMemoryPeer(t *testing.T) {
	for _, flags := range []int{syscall.MAP_SHARED, 3 /* MAP_SHARED_VALIDATE */, syscall.MAP_PRIVATE} {
		q := fixture(t, "shared-memory")
		outside := t.TempDir()
		name := filepath.Join(outside, "channel")
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		if err := f.Truncate(4096); err != nil {
			t.Fatal(err)
		}
		memory, err := syscall.Mmap(int(f.Fd()), 0, 4096, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { syscall.Munmap(memory) })
		q.Outside = append(q.Outside, OutsideClass{"temporary", outside})
		q.Args = append(q.Args, name, fmt.Sprint(flags))
		for _, value := range []byte{5, 6} {
			if err := os.WriteFile(filepath.Join(q.Dir, "data"), []byte{value}, 0600); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(q.Dir, "data"))
			if err != nil {
				t.Fatal(err)
			}
			memory[0], memory[1] = data[0], 0 // Outside peer publishes its repository-derived response.
			r := New().Trace(context.Background(), q)
			if r.Outcome.ExitCode != int(value) || hasPath(r.Reads.Entries, "data") {
				t.Fatalf("unexpected IPC outcome/dependency: %+v", r)
			}
			if flags == syscall.MAP_PRIVATE {
				if memory[1] != 0 || r.External.State != Present {
					t.Fatalf("private mapping unexpectedly shares writes: %+v", r)
				}
			} else if memory[1] != value+1 || r.External.State != Incomplete || !hasReason(r.External.Reasons, "descriptor-peer-unknown") {
				t.Fatalf("shared-memory peer was invisible: %+v, reply=%d", r, memory[1])
			}
		}
	}
}

func TestRSPV0022UnknownMappingForms(t *testing.T) {
	for _, flags := range []uint64{syscall.MAP_ANON, syscall.MAP_PRIVATE | syscall.MAP_ANON | 1<<63} {
		r := emptyResult("test")
		c := collector{r: &r}
		c.enter(0, syscall.PtraceRegs{Cs: 0x33, Orig_rax: 9, Rdx: syscall.PROT_READ, R10: flags})
		if r.External.State != Incomplete || !hasReason(r.External.Reasons, "unsupported-syscall:9") {
			t.Fatalf("unknown mapping admitted: %#x %+v", flags, r)
		}
	}
}

// RSP-V0-007/021/022: the kernel loads PT_INTERP even for an outside executable.
func TestRSPV0021OutsideExecutableRepositoryInterpreter(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		filepath.Join(outside, "loader"): filepath.Join(root, "alias"),
		filepath.Join(root, "alias"):     "bridge",
		filepath.Join(root, "bridge"):    "interpreter",
	} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(outside, "program")
	build := exec.Command(cc, "-x", "c", "-", "-Wl,--dynamic-linker,"+filepath.Join(outside, "loader"), "-o", exe)
	build.Stdin = strings.NewReader("int main(void) { return 0; }\n")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile executable: %v %s", err, output)
	}
	for _, exit := range []int{7, 9} {
		build := exec.Command(cc, "-nostdlib", "-static", "-no-pie", "-x", "assembler", "-", "-o", filepath.Join(root, "interpreter"))
		build.Stdin = strings.NewReader(fmt.Sprintf(".text\n.global _start\n_start:\n mov $%d, %%edi\n mov $60, %%eax\n syscall\n.section .note.GNU-stack,\"\",@progbits\n", exit))
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("compile interpreter: %v %s", err, output)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		r := New().Trace(ctx, Request{Executable: exe, Dir: root, Repositories: []Repository{{ID: "main", Root: root}}, Outside: []OutsideClass{{"toolchain", outside}}, EventCap: 1000, PathCap: 100})
		cancel()
		if !r.Outcome.Started || r.Outcome.ExitCode != exit || r.Outcome.Error != "" || r.Reads.State != Present {
			t.Fatalf("interpreter outcome not observed: %+v", r)
		}
		for _, name := range []string{"alias", "bridge"} {
			if !hasPath(r.Reads.Entries, name) {
				t.Errorf("implicit link dependency %s lost: %+v", name, r)
			}
		}
		found := false
		for _, read := range r.Reads.Entries {
			found = found || read.ResolvedRepository == "main" && read.ResolvedPath == "interpreter"
		}
		if !found {
			t.Fatalf("implicit interpreter target lost: %+v", r)
		}
	}
}

// RSP-V0-022: the executable allowlist applies to declared outside roots too.
func TestRSPV0022OutsideUnknownExecutableFormatIncomplete(t *testing.T) {
	for _, contents := range []string{"unknown future executable format", "#!/bin/sh\nexit 0\n"} {
		outside := t.TempDir()
		exe := filepath.Join(outside, "program")
		if err := os.WriteFile(exe, []byte(contents), 0700); err != nil {
			t.Fatal(err)
		}
		r := emptyResult("test")
		c := collector{q: Request{Repositories: []Repository{{ID: "main", Root: t.TempDir()}}, Outside: []OutsideClass{{"toolchain", outside}}, PathCap: 100}, r: &r, paths: map[Path]bool{}}
		c.recordExec(exe, exe)
		if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "unsupported-executable-format") {
			t.Fatalf("unknown executable certified: %+v", r)
		}
	}
}

// RSP-V0-007/022: opening for writing alone depends on existing data.
func TestRSPV0007WriteOnlyOpen(t *testing.T) {
	q := fixture(t, "write-open")
	r := traceFixture(t, q)
	if r.Reads.State != Present || !hasPath(r.Reads.Entries, "data") {
		t.Fatalf("write-only existence dependency missing: %+v", r)
	}
	if err := os.Remove(filepath.Join(q.Dir, "data")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	missing := New().Trace(ctx, q)
	if !missing.Outcome.Started || missing.Outcome.ExitCode != 91 || !hasPath(missing.Lookups.Entries, "data") {
		t.Fatalf("deletion did not reach failing open: %+v", missing)
	}
}

// RSP-V0-007/021/022: PT_INTERP is an implicit read with the same closed
// path-walking obligations as a traced open, including intermediate links.
func TestRSPV0007InterpreterLinks(t *testing.T) {
	root := t.TempDir()
	interpreter := filepath.Join(root, "interp")
	if err := os.Symlink("bridge", interpreter); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"loaderA", "loaderB"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	// A minimal ELF image isolates the kernel's implicit interpreter dependency
	// from ptrace availability and the host's dynamic loader/runtime behavior.
	var image bytes.Buffer
	header := elf.Header64{Ident: [16]byte{0x7f, 'E', 'L', 'F', 2, 1, 1}, Type: uint16(elf.ET_EXEC), Machine: uint16(elf.EM_X86_64), Version: 1, Phoff: 64, Ehsize: 64, Phentsize: 56, Phnum: 1}
	program := elf.Prog64{Type: uint32(elf.PT_INTERP), Off: 120, Filesz: uint64(len(interpreter) + 1)}
	for _, v := range []any{header, program} {
		if err := binary.Write(&image, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	image.WriteString(interpreter)
	image.WriteByte(0)
	executable := filepath.Join(root, "program")
	if err := os.WriteFile(executable, image.Bytes(), 0700); err != nil {
		t.Fatal(err)
	}
	parsed, err := elf.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Progs) != 1 || parsed.Progs[0].Type != elf.PT_INTERP {
		t.Fatal("invalid ELF fixture")
	}
	if err := parsed.Close(); err != nil {
		t.Fatal(err)
	}
	bridge := filepath.Join(root, "bridge")
	for _, target := range []string{"loaderA", "loaderB", "bridge", "missing/../loaderA"} {
		if err := os.Symlink(target, bridge); err != nil {
			t.Fatal(err)
		}
		r := emptyResult("test")
		c := collector{q: Request{Repositories: []Repository{{ID: "main", Root: root}}, PathCap: 100}, r: &r, paths: map[Path]bool{}}
		c.recordExec(executable, executable)
		if target == "loaderA" || target == "loaderB" {
			if r.Reads.State != Present || r.Execs.State != Present {
				t.Fatalf("valid interpreter capture incomplete: %+v", r)
			}
			for _, name := range []string{"interp", "bridge"} {
				if !hasPath(r.Reads.Entries, name) {
					t.Errorf("interpreter dependency lost after retarget to %s: %s: %+v", target, name, r)
				}
			}
			found := false
			for _, path := range r.Reads.Entries {
				if path.Path == "interp" && path.ResolvedPath == target {
					found = true
				}
			}
			if !found {
				t.Fatalf("retarget did not reach %s: %+v", target, r)
			}
		} else if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "symlink-resolution-failed") {
			t.Fatalf("unsupported interpreter resolution admitted: %+v", r)
		}
		if err := os.Remove(bridge); err != nil {
			t.Fatal(err)
		}
	}
}

// RSP-V0-007/021/022: successful non-exclusive opens observe the file even
// without a read syscall; flags outside the closed allowlist stay incomplete.
func TestRSPV0021WriteOnlyOpenClassification(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "data")
	for _, flags := range []uint64{syscall.O_WRONLY, syscall.O_WRONLY | syscall.O_APPEND,
		syscall.O_WRONLY | syscall.O_TRUNC, syscall.O_WRONLY | syscall.O_CREAT,
		syscall.O_WRONLY | 0x40000000, syscall.O_WRONLY | syscall.O_CREAT | syscall.O_EXCL} {
		t.Run(fmt.Sprintf("%#x", flags), func(t *testing.T) {
			r := emptyResult("test")
			c := collector{q: Request{Repositories: []Repository{{ID: "main", Root: root}}, PathCap: 100}, r: &r, paths: map[Path]bool{}, created: map[string]bool{}}
			c.finish(&call{kind: "open", lexical: name, resolved: name, flags: flags}, 3)
			exclusive := flags&(syscall.O_CREAT|syscall.O_EXCL) == syscall.O_CREAT|syscall.O_EXCL
			if hasPath(r.Reads.Entries, "data") == exclusive || c.created[name] != exclusive {
				t.Fatalf("incorrect dependency/provenance for %#x: %+v", flags, r)
			}
			if flags&0x40000000 != 0 {
				if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "unsupported-syscall:2") {
					t.Fatalf("unknown form certified: %+v", r)
				}
			} else if r.Reads.State != Present || len(r.Writes.Entries) != 1 {
				t.Fatalf("ordinary open classification: %+v", r)
			}
		})
	}
}

func TestRSPV0021WriteOnlyOpenDependency(t *testing.T) {
	for _, flags := range []uint64{syscall.O_WRONLY, syscall.O_WRONLY | syscall.O_APPEND,
		syscall.O_WRONLY | syscall.O_TRUNC, syscall.O_WRONLY | syscall.O_CREAT,
		syscall.O_WRONLY | 0x40000000} {
		t.Run(fmt.Sprintf("%#x", flags), func(t *testing.T) {
			q := fixture(t, "open-write-only")
			q.Args = append(q.Args, fmt.Sprint(flags))
			r := traceFixture(t, q)
			if !hasPath(r.Reads.Entries, "data") {
				t.Fatalf("open success dependency missing: %+v", r)
			}
			if flags&0x40000000 != 0 {
				if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, "unsupported-syscall:2") {
					t.Fatalf("unknown open form certified: %+v", r)
				}
				return
			}
			if r.Reads.State != Present {
				t.Fatalf("ordinary open incomplete: %+v", r)
			}
			if flags&syscall.O_CREAT == 0 {
				if err := os.Remove(filepath.Join(q.Dir, "data")); err != nil {
					t.Fatal(err)
				}
				missing := New().Trace(context.Background(), q)
				if missing.Outcome.ExitCode != 1 || !hasPath(missing.Lookups.Entries, "data") {
					t.Fatalf("deletion must change outcome and retain lookup: %+v", missing)
				}
			}
		})
	}
}
