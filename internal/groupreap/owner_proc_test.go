//go:build darwin || linux

package groupreap

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeProc is a /proc-shaped tree for the PGO-V0-006 classification matrix.
type fakeProc struct {
	t    *testing.T
	root string
}

func newFakeProc(t *testing.T) *fakeProc {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"self", "net", "sys"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &fakeProc{t: t, root: root}
}

func (f *fakeProc) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// process writes pid's stat and a status whose Threads count is the number of
// thread states given (one when none are given).
func (f *fakeProc) process(pid int, comm string, state byte, ppid, pgrp int, threads ...byte) {
	f.t.Helper()
	stat := strconv.Itoa(pid) + " (" + comm + ") " + string(state) + " " + strconv.Itoa(ppid) + " " + strconv.Itoa(pgrp) + " " + strconv.Itoa(pgrp) + " 0 -1 4194304\n"
	f.write(filepath.Join(strconv.Itoa(pid), "stat"), stat)
	f.write(filepath.Join(strconv.Itoa(pid), "status"), "Name:\t"+comm+"\nState:\t"+string(state)+"\nThreads:\t"+strconv.Itoa(max(1, len(threads)))+"\n")
}

func TestProcGroupQuietClassification(t *testing.T) {
	const parent, leader = 50, 100
	cases := []struct {
		name  string
		build func(*fakeProc)
		want  Probe
		err   bool
	}{
		{"zombie-only", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.process(101, "sleep", 'Z', 1, leader)
			f.process(102, "git", 'X', 1, leader)
		}, ProbeQuiet, false},
		{"leader-only", func(f *fakeProc) { f.process(leader, "sh", 'Z', parent, leader) }, ProbeQuiet, false},
		{"unrelated-live-process-ignored", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.process(7, "init", 'S', 0, 7)
			f.process(leader*10, "other", 'R', parent, leader+1)
		}, ProbeQuiet, false},
		{"vanished-entry-skipped", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			if err := os.MkdirAll(filepath.Join(f.root, "103"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, ProbeQuiet, false},
		{"live-member", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.process(101, "sleep", 'S', 1, leader)
		}, ProbeLive, false},
		{"uninterruptible-member", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.process(101, "io", 'D', 1, leader)
		}, ProbeLive, false},
		{"zombie-thread-leader-with-live-thread", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.process(101, "threads", 'Z', 1, leader, 'Z', 'S')
		}, ProbeLive, false},
		{"zombie-member-status-vanished", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.process(101, "sleep", 'Z', 1, leader)
			if err := os.Remove(filepath.Join(f.root, "101", "status")); err != nil {
				t.Fatal(err)
			}
		}, ProbeQuiet, false},
		{"zombie-leader-with-live-thread", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader, 'Z', 'S')
		}, ProbeLive, false},
		{"zombie-member-status-without-thread-count", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.process(101, "sleep", 'Z', 1, leader)
			f.write("101/status", "Name:\tsleep\nState:\tZ\n")
		}, ProbeLive, true},
		{"zombie-member-oversized-status", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.process(101, "sleep", 'Z', 1, leader)
			f.write("101/status", strings.Repeat("x", maxProcStatus)+"\nThreads:\t1\n")
		}, ProbeLive, true},
		{"comm-with-parentheses", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.process(101, "a) Z 1 100 (b", 'R', 1, leader)
		}, ProbeLive, false},
		{"leader-missing", func(f *fakeProc) { f.process(101, "sleep", 'Z', 1, leader) }, ProbeLive, true},
		{"leader-not-exited", func(f *fakeProc) { f.process(leader, "sh", 'S', parent, leader) }, ProbeLive, true},
		{"leader-other-parent", func(f *fakeProc) { f.process(leader, "sh", 'Z', parent+1, leader) }, ProbeLive, true},
		{"leader-other-group", func(f *fakeProc) { f.process(leader, "sh", 'Z', parent, leader+1) }, ProbeLive, true},
		{"malformed-member-stat", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.write("101/stat", "101 sleep S 1 100\n")
		}, ProbeLive, true},
		{"oversized-member-stat", func(f *fakeProc) {
			f.process(leader, "sh", 'Z', parent, leader)
			f.write("101/stat", "101 (x) S 1 100 "+strings.Repeat("0 ", maxProcStat))
		}, ProbeLive, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFakeProc(t)
			c.build(f)
			probe, err := procGroupQuiet(f.root, leader, parent)
			if probe != c.want || (err != nil) != c.err || (err != nil && !errors.Is(err, errProcProof) && !errors.Is(err, os.ErrNotExist)) {
				t.Fatalf("probe = %v, err = %v; want %v, error %v", probe, err, c.want, c.err)
			}
		})
	}
}

func TestProcGroupQuietUnavailableRoot(t *testing.T) {
	probe, err := procGroupQuiet(filepath.Join(t.TempDir(), "absent"), 100, 50)
	if probe != ProbeLive || !errors.Is(err, errProcProof) {
		t.Fatalf("probe = %v, err = %v", probe, err)
	}
	for _, ids := range [][2]int{{1, 50}, {0, 50}, {100, 0}} {
		if _, err := procGroupQuiet(t.TempDir(), ids[0], ids[1]); !errors.Is(err, errProcProof) {
			t.Fatalf("leader %d parent %d: err = %v", ids[0], ids[1], err)
		}
	}
}

func TestQuietProofFallsBackToInjectedProbeGroup(t *testing.T) {
	calls := 0
	probe := func(int) (Probe, error) { calls++; return ProbeQuiet, nil }
	p, err := resolvePrimitives(Primitives{ProbeGroup: probe})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := p.QuietProof(1); got != ProbeQuiet || calls != 1 {
		t.Fatalf("injected ProbeGroup not used as quiet proof: %v, %d", got, calls)
	}
	p, err = resolvePrimitives(Primitives{})
	if err != nil || p.QuietProof == nil || p.RetirementMode != RequirePreReapQuiet {
		t.Fatalf("default primitives = %+v, %v", p, err)
	}
}
