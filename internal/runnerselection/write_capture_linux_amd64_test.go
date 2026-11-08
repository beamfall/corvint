// SPDX-License-Identifier: Apache-2.0
//go:build linux && amd64

package runnerselection

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/testevidence"
)

// RSP-V0-007/021/012(f): these probes check only mutation success, never read
// the file, and have no startup reads that could conceal a missing dependency.
func TestRSPV0021CapturedWriteSuccessDependency(t *testing.T) {
	for _, tc := range []struct {
		name      string
		syscall   int
		argument  uint64
		unknown   bool
		canCreate bool
	}{
		{"write-only-open", 2, 1, false, false},
		{"append-open", 2, 1025, false, false},
		{"truncate-open", 2, 513, false, false},
		{"nonexclusive-create", 2, 65, false, true},
		{"creat", 85, 0600, false, true},
		{"truncate", 76, 0, false, false},
		{"unknown-open-flags", 2, 1 | 1<<31, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, toolchain := t.TempDir(), t.TempDir()
			input := filepath.Join(root, "fixture")
			if err := os.WriteFile(input, []byte("tracked fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			assembly := fmt.Sprintf(`.global _start
.text
_start:
 mov $%d, %%rax
 lea name(%%rip), %%rdi
 mov $%d, %%rsi
 mov $384, %%rdx
 syscall
 test %%rax, %%rax
 sets %%dil
 movzbq %%dil, %%rdi
 mov $60, %%rax
 syscall
.section .rodata
name: .asciz "fixture"
`, tc.syscall, tc.argument)
			source, exe := filepath.Join(toolchain, "probe.S"), filepath.Join(toolchain, "probe")
			if err := os.WriteFile(source, []byte(assembly), 0600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("cc", "-nostdlib", "-static", "-no-pie", "-o", exe, source).CombinedOutput(); err != nil {
				t.Fatalf("cc: %v %s", err, out)
			}
			q := proctrace.Request{Executable: exe, Dir: root, Repositories: []proctrace.Repository{{ID: "repo", Root: root}}, Outside: []proctrace.OutsideClass{{Class: "toolchain", Root: toolchain}}, EventCap: 1000, PathCap: 100}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			trace := proctrace.New().Trace(ctx, q)
			if !trace.Outcome.Started || trace.Outcome.ExitCode != 0 || trace.Outcome.Error != "" {
				t.Fatalf("operation failed: %+v", trace)
			}
			wantState := proctrace.Present
			if tc.unknown {
				wantState = proctrace.Incomplete
			}
			for _, state := range []proctrace.State{trace.Reads.State, trace.Listings.State, trace.Lookups.State, trace.Writes.State, trace.Execs.State, trace.External.State} {
				if state != wantState {
					t.Fatalf("unexpected completeness: %+v", trace)
				}
			}
			if len(trace.Reads.Entries) != 1 || trace.Reads.Entries[0].Path != "fixture" {
				t.Fatalf("successful operation lost its dependency: %+v", trace)
			}
			if len(trace.Listings.Entries)+len(trace.Lookups.Entries)+len(trace.Execs.Entries)+len(trace.External.Entries) != 0 {
				t.Fatalf("masking dependency: %+v", trace)
			}
			in := fixture()
			for _, p := range trace.Reads.Entries {
				if p.Repository != "repo" || p.Path != p.ResolvedPath {
					t.Fatalf("unexpected path: %+v", p)
				}
				record(&in).Reads.Items = append(record(&in).Reads.Items, blob(&in, p.Path))
			}
			for _, w := range trace.Writes.Entries {
				record(&in).Writes.Items = append(record(&in).Writes.Items, testevidence.Write{Path: testevidence.Path{Repository: w.Path.Repository, Lexical: w.Path.Path, Resolved: w.Path.ResolvedPath}, Kind: testevidence.WriteKind(w.Kind)})
			}
			if tc.unknown {
				// Preserve the captured uncertainty; even an unrelated diff must
				// not exclude this test when an open form is not understood.
				record(&in).Reads.State = testevidence.Incomplete
				record(&in).Reads.Reason = strings.Join(trace.Reads.Reasons, ",")
				selected(t, in, "component-missing:reads")
			} else {
				excluded(t, in)
			}
			in.Changes[0] = Change{Repository: "repo", Path: "fixture", Kind: "delete", Decoded: true}
			selected(t, in, "rule:f")
			in.Changes[0] = Change{Repository: "repo", Path: "moved", OldPath: "fixture", Kind: "rename", Decoded: true}
			selected(t, in, "rule:f")
			if !tc.canCreate {
				if err := os.Rename(input, filepath.Join(root, "moved")); err != nil {
					t.Fatal(err)
				}
				changed := proctrace.New().Trace(ctx, q)
				if !changed.Outcome.Started || changed.Outcome.ExitCode != 1 || changed.Outcome.Error != "" {
					t.Fatalf("rename did not change success assertion: %+v", changed)
				}
			}
		})
	}
}
