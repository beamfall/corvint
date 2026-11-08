// SPDX-License-Identifier: Apache-2.0
//go:build linux && amd64

package runnerselection

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/testevidence"
)

// RSP-V0-021/012(f): a failed O_DIRECTORY open depends on the existing file,
// even though the syscall returned no descriptor and read no bytes.
func TestRSPV0021CapturedENOTDIRDeletion(t *testing.T) {
	root, toolchain := t.TempDir(), t.TempDir()
	input := filepath.Join(root, "fixture")
	if err := os.WriteFile(input, []byte("regular file"), 0600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(toolchain, "probe.S")
	// Static native ELF uses only open and exit; no libc/startup noise can
	// accidentally force selection and conceal a lost dependency.
	assembly := `.global _start
.text
_start:
 mov $2, %rax
 lea name(%rip), %rdi
 mov $65536, %rsi
 xor %rdx, %rdx
 syscall
 cmp $-20, %rax
 setne %dil
 movzbq %dil, %rdi
 mov $60, %rax
 syscall
.section .rodata
name: .asciz "fixture"
`
	if err := os.WriteFile(source, []byte(assembly), 0600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(toolchain, "probe")
	if out, err := exec.Command("cc", "-nostdlib", "-static", "-no-pie", "-o", exe, source).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v %s", err, out)
	}
	q := proctrace.Request{Executable: exe, Dir: root, Repositories: []proctrace.Repository{{ID: "repo", Root: root}}, Outside: []proctrace.OutsideClass{{Class: "toolchain", Root: toolchain}}, EventCap: 1000, PathCap: 100}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	trace := proctrace.New().Trace(ctx, q)
	if !trace.Outcome.Started || trace.Outcome.ExitCode != 0 || trace.Outcome.Error != "" {
		t.Fatalf("errno assertion failed: %+v", trace)
	}
	for _, state := range []proctrace.State{trace.Reads.State, trace.Listings.State, trace.Lookups.State, trace.Writes.State, trace.Execs.State, trace.External.State} {
		if state != proctrace.Present {
			t.Fatalf("capture not complete: %+v", trace)
		}
	}
	in := fixture()
	// Pin every captured path into the selector's admitted fixture record. The
	// trace's components must be complete above; no fallback can mask the bug.
	for _, p := range trace.Reads.Entries {
		if p.Repository != "repo" || p.Path != p.ResolvedPath {
			t.Fatalf("unexpected path: %+v", p)
		}
		record(&in).Reads.Items = append(record(&in).Reads.Items, blob(&in, p.Path))
	}
	for _, p := range trace.Lookups.Entries {
		record(&in).Lookups.Items = append(record(&in).Lookups.Items, testevidence.Path{Repository: p.Repository, Lexical: p.Path, Resolved: p.ResolvedPath})
	}
	if len(trace.Listings.Entries) != 0 || len(trace.Execs.Entries) != 0 || len(trace.External.Entries) != 0 {
		t.Fatalf("unexpected masking dependency: %+v", trace)
	}
	excluded(t, in) // An unrelated edit is excludable with the same record.
	in.Changes[0] = Change{Repository: "repo", Path: "fixture", Kind: "delete", Decoded: true}
	selected(t, in, "rule:f")
	in.Changes[0] = Change{Repository: "repo", Path: "moved", OldPath: "fixture", Kind: "rename", Decoded: true}
	selected(t, in, "rule:f")
	if err := os.Remove(input); err != nil {
		t.Fatal(err)
	}
	changed := proctrace.New().Trace(ctx, q)
	if !changed.Outcome.Started || changed.Outcome.ExitCode != 1 || changed.Outcome.Error != "" {
		t.Fatalf("deletion did not change errno assertion: %+v", changed)
	}
}
