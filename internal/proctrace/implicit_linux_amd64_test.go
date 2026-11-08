// SPDX-License-Identifier: Apache-2.0
//go:build linux && amd64

package proctrace

import (
	"bytes"
	"context"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// The collector only needs the ELF headers to inspect PT_INTERP. Keeping this
// fixture synthetic exercises kernel-implicit dependency handling without
// depending on a compiler or starting a traced process.
func interpreterImage(t *testing.T, directory, interpreter string) string {
	t.Helper()
	var data bytes.Buffer
	h := elf.Header64{Type: uint16(elf.ET_EXEC), Machine: uint16(elf.EM_X86_64), Version: uint32(elf.EV_CURRENT), Phoff: 64, Ehsize: 64, Phentsize: 56, Phnum: 1}
	copy(h.Ident[:], []byte{0x7f, 'E', 'L', 'F', byte(elf.ELFCLASS64), byte(elf.ELFDATA2LSB), byte(elf.EV_CURRENT)})
	p := elf.Prog64{Type: uint32(elf.PT_INTERP), Off: 120, Filesz: uint64(len(interpreter) + 1)}
	for _, v := range []any{h, p} {
		if err := binary.Write(&data, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	data.WriteString(interpreter)
	data.WriteByte(0)
	name := filepath.Join(directory, "image")
	if err := execfile.WriteFile(name, data.Bytes(), 0700); err != nil {
		t.Fatal(err)
	}
	return name
}

func interpreterDependencies(root, image string, cap int) Result {
	r := emptyResult("test")
	c := collector{q: Request{Repositories: []Repository{{ID: "main", Root: root}}, PathCap: cap}, r: &r, paths: map[Path]bool{}}
	c.recordExec(image, image)
	return r
}

func TestRSPV0021InterpreterMiddleLinkRetarget(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"target-a", "target-b"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	first, middle := filepath.Join(root, "first"), filepath.Join(root, "middle")
	if err := os.Symlink("middle", first); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target-a", middle); err != nil {
		t.Fatal(err)
	}
	image := interpreterImage(t, root, first)
	for _, target := range []string{"target-a", "target-b"} {
		if target == "target-b" {
			if err := os.Remove(middle); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, middle); err != nil {
				t.Fatal(err)
			}
		}
		r := interpreterDependencies(root, image, 100)
		for _, want := range []Path{
			{Repository: "main", Path: "first", ResolvedRepository: "main", ResolvedPath: "first"},
			{Repository: "main", Path: "middle", ResolvedRepository: "main", ResolvedPath: "middle"},
			{Repository: "main", Path: "first", ResolvedRepository: "main", ResolvedPath: target},
		} {
			found := false
			for _, got := range r.Reads.Entries {
				found = found || got == want
			}
			if !found {
				t.Fatalf("lost interpreter dependency %+v: %+v", want, r)
			}
		}
		if r.Reads.State != Present {
			t.Fatalf("ordinary interpreter chain incomplete: %+v", r)
		}
	}
}

func TestRSPV0022InterpreterLinkCaps(t *testing.T) {
	for _, kind := range []string{"hops", "paths", "cycle"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "target"), []byte("target"), 0600); err != nil {
				t.Fatal(err)
			}
			count, cap := 2, 100
			if kind == "hops" {
				count = 41
			} else if kind == "paths" {
				cap = 2
			}
			for i := 0; i < count; i++ {
				target := fmt.Sprintf("link-%d", i+1)
				if i == count-1 {
					target = "target"
					if kind == "cycle" {
						target = "link-0"
					}
				}
				if err := os.Symlink(target, filepath.Join(root, fmt.Sprintf("link-%d", i))); err != nil {
					t.Fatal(err)
				}
			}
			image := interpreterImage(t, root, filepath.Join(root, "link-0"))
			r := interpreterDependencies(root, image, cap)
			reason := "symlink-resolution-failed"
			if kind == "paths" {
				reason = "path-cap"
			}
			if r.Reads.State != Incomplete || !hasReason(r.Reads.Reasons, reason) {
				t.Fatalf("unproven interpreter chain admitted: %+v", r)
			}
		})
	}
}

// RSP-V0-021/022: errno values outside the lookup-only allowlist retain claims.
func TestRSPV0021UnknownFailureRetainsPath(t *testing.T) {
	for _, errno := range []int64{int64(syscall.ENOTDIR), 4095} {
		root := t.TempDir()
		name := filepath.Join(root, "fixture")
		r := emptyResult("test")
		c := collector{q: Request{Repositories: []Repository{{ID: "main", Root: root}}, PathCap: 100}, r: &r, paths: map[Path]bool{}}
		c.finish(&call{kind: "open", lexical: name, resolved: name}, -errno)
		if !hasPath(r.Reads.Entries, "fixture") {
			t.Fatalf("errno %d dropped dependency: %+v", errno, r)
		}
	}
}

// RSP-V0-021/022: the kernel loads PT_INTERP before observable syscalls,
// regardless of whether the executable itself is a declared outside tool.
func TestRSPV0021OutsideExecutableInterpreter(t *testing.T) {
	root, toolchain := t.TempDir(), t.TempDir()
	interpreter := filepath.Join(root, "interpreter")
	source := filepath.Join(toolchain, "interpreter.S")
	assembly := ".global _start\n.text\n_start:\n mov $60, %rax\n xor %rdi, %rdi\n syscall\n"
	if err := os.WriteFile(source, []byte(assembly), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cc", "-nostdlib", "-static", "-no-pie", "-o", interpreter, source).CombinedOutput(); err != nil {
		t.Fatalf("cc interpreter: %v %s", err, out)
	}
	main := filepath.Join(toolchain, "main.c")
	if err := os.WriteFile(main, []byte("int main(void) { return 99; }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(toolchain, "outside")
	if out, err := exec.Command("cc", "-Wl,--dynamic-linker="+interpreter, "-o", exe, main).CombinedOutput(); err != nil {
		t.Fatalf("cc executable: %v %s", err, out)
	}
	q := Request{Executable: exe, Dir: root, Repositories: []Repository{{ID: "main", Root: root}}, Outside: []OutsideClass{{Class: "toolchain", Root: toolchain}}, EventCap: 1000, PathCap: 100}
	r := traceFixture(t, q)
	if r.Reads.State != Present || r.Execs.State != Present || !hasPath(r.Reads.Entries, "interpreter") || len(r.Execs.Entries) != 0 || r.Outside["toolchain"] == 0 {
		t.Fatalf("outside executable lost interpreter: %+v", r)
	}
	// Replacing only the interpreter changes the result before main can run.
	assembly = ".global _start\n.text\n_start:\n mov $60, %rax\n mov $7, %rdi\n syscall\n"
	if err := os.WriteFile(source, []byte(assembly), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cc", "-nostdlib", "-static", "-no-pie", "-o", interpreter, source).CombinedOutput(); err != nil {
		t.Fatalf("cc replacement: %v %s", err, out)
	}
	changed := New().Trace(context.Background(), q)
	if changed.Outcome.ExitCode != 7 || changed.Outcome.Error != "" {
		t.Fatalf("interpreter replacement did not change outcome: %+v", changed)
	}
}

// RSP-V0-022: an outside declaration never certifies an unknown image format.
func TestRSPV0022OutsideUnknownExecutable(t *testing.T) {
	for _, contents := range []string{"#!/bin/sh\nexit 0\n", "future executable format\n"} {
		root, outside := t.TempDir(), t.TempDir()
		name := filepath.Join(outside, "image")
		if err := execfile.WriteFile(name, []byte(contents), 0700); err != nil {
			t.Fatal(err)
		}
		r := emptyResult("test")
		c := collector{q: Request{Repositories: []Repository{{ID: "main", Root: root}}, Outside: []OutsideClass{{Class: "toolchain", Root: outside}}, PathCap: 100}, r: &r, paths: map[Path]bool{}}
		c.recordExec(name, name)
		if r.Reads.State != Incomplete || r.Execs.State != Incomplete || !hasReason(r.Execs.Reasons, "unsupported-executable-format") {
			t.Fatalf("unknown outside image certified: %+v", r)
		}
	}
}
