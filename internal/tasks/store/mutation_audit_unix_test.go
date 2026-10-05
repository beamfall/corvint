//go:build darwin || linux

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// mapShared maps p shared and writable before the caller starts watching. The
// returned flip replaces the file's last byte through the mapping, without
// msync, and a second call restores it.
func mapShared(t *testing.T, p string) (flip func()) {
	t.Helper()
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		f.Close()
		t.Fatalf("map %s: %v", p, err)
	}
	m, err := syscall.Mmap(int(f.Fd()), 0, int(info.Size()), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if closeErr := f.Close(); err != nil || closeErr != nil {
		t.Fatalf("map %s: %v %v", p, err, closeErr)
	}
	t.Cleanup(func() { syscall.Munmap(m) })
	last := len(m) - 1
	original, replacement := m[last], byte(' ')
	if original == replacement {
		replacement = '\t'
	}
	return func() {
		if m[last] == original {
			m[last] = replacement
		} else {
			m[last] = original
		}
	}
}

// TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit is the CAL-V0-070
// content check. A write through a shared mapping, made after the merged audit
// read the file, raises no inotify event and no kqueue event before msync, so
// only the fresh inventory's digest shows it. Mutate must refuse it with the
// separate passes' code and publish nothing.
func TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit(t *testing.T) {
	repo := historyStore(t, 70)
	requestFile, ticketFile := mutationBoundaryFiles(t, repo)
	for _, tc := range []struct{ name, file, want string }{
		{"request-projection", requestFile, "JOURNAL_FORKED"},
		{"ticket", ticketFile, "INTENT_DIVERGED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flip := mapShared(t, tc.file)
			before, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			// The watch cannot see this write; reads can.
			watch, err := authority.WatchChanges(repo)
			if err != nil {
				t.Fatalf("change watch: %v", err)
			}
			flip()
			changed, readErr := os.ReadFile(tc.file)
			checkErr := watch.Check()
			flip()
			watch.Close()
			if readErr != nil || string(changed) == string(before) {
				t.Fatalf("mapped write not visible to reads: %v", readErr)
			}
			if checkErr != nil {
				t.Fatalf("the change watch saw a mapped write (%v); this case no longer reaches the content check", checkErr)
			}

			id := "mapped-" + tc.name
			published := mutationPublished(t, repo)
			var stages []string
			flipped := false
			ctx := context.WithValue(context.Background(), mutationStageKey{}, func(stage string) {
				stages = append(stages, stage)
				if stage == "audited" {
					flip()
					flipped = true
				}
			})
			rep, err := Mutate(ctx, repo, historyActor, historyCreate(id, "mapped "+tc.name), WallClock())
			if flipped {
				flip()
			}
			code := wire.CodeOf(err)
			oracle := separateMutateAudits(t, repo, id, flip)
			flip()
			if err == nil || code != tc.want || !strings.HasPrefix(oracle.Err, "inventory: "+code) && !strings.HasPrefix(oracle.Err, "audit: "+code) {
				t.Fatalf("Mutate: %v (report %+v); separate passes: %s", err, rep, oracle.Err)
			}
			t.Logf("refused %s, as the separate passes: %s", code, oracle.Err)
			if want := []string{"watched", "audited", "observed", "fresh"}; !reflect.DeepEqual(stages, want) {
				t.Fatalf("stages %v, want %v", stages, want)
			}
			if after := mutationPublished(t, repo); after != published {
				t.Fatalf("published: %s, before %s", after, published)
			}
			if p, _ := snapshot.RequestPath(id); fileExists(filepath.Join(repo.StateDir, p)) {
				t.Fatal("refused request was projected")
			}
			if after, err := os.ReadFile(tc.file); err != nil || string(after) != string(before) {
				t.Fatalf("mapped file not restored: %v", err)
			}
		})
	}
}

const descriptorChildEnv = "CORVINT_CALV0070_DESCRIPTOR_CHILD"

// TestCALV0070_MutateRetriesAuditWithoutWatch runs in a child process with a
// controlled RLIMIT_NOFILE. The watch registers, the merged audit then runs
// out of descriptors, the watch's descriptors are released and the audit is
// taken again: a clean store, given exactly what the audit needs once the
// watch is closed, completes as the separate passes allow, and a corrupt store
// is still refused as they refuse it.
func TestCALV0070_MutateRetriesAuditWithoutWatch(t *testing.T) {
	if mode := os.Getenv(descriptorChildEnv); mode != "" {
		descriptorRetryChild(t, mode)
		return
	}
	for _, mode := range []string{"clean", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			child := exec.Command(os.Args[0], "-test.run=^TestCALV0070_MutateRetriesAuditWithoutWatch$", "-test.count=1", "-test.v")
			child.Env = append(os.Environ(), descriptorChildEnv+"="+mode)
			out, err := child.CombinedOutput()
			marker := "descriptor retry " + mode + ":"
			if err != nil || !strings.Contains(string(out), marker) {
				t.Fatalf("child %s: %v\n%s", mode, err, out)
			}
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, marker) {
					t.Log(strings.TrimSpace(line))
				}
			}
		})
	}
}

func descriptorRetryChild(t *testing.T, mode string) {
	repo := historyStore(t, 70)
	requestFile, _ := mutationBoundaryFiles(t, repo)
	if mode == "corrupt" {
		historyWrite(t, requestFile, []byte("{}\n"))
	}
	id := "descriptor-" + mode
	oracle := separateMutateAudits(t, repo, id)
	if (mode == "clean") != (oracle.Err == "") || oracle.Found || mode == "corrupt" && !strings.HasPrefix(oracle.Err, "lookup: JOURNAL_FORKED") {
		t.Fatalf("separate passes on the %s store: %+v", mode, oracle.Err)
	}

	// The descriptors one registration holds.
	idle := openDescriptors(t)
	watch, err := authority.WatchChanges(repo)
	if err != nil {
		t.Fatalf("change watch: %v", err)
	}
	held := len(openDescriptors(t)) - len(idle)
	watch.Close()
	if after := openDescriptors(t); !reflect.DeepEqual(after, idle) || held < 1 {
		t.Fatalf("watch held %d descriptors; open %v after close, %v before", held, after, idle)
	}

	// On the clean store, the fewest free descriptors with which the merged
	// audit completes: it keeps every parent it read pinned until it ends, so
	// the count does not depend on read order. A refusal's count does, since
	// it comes after however many parents were read first, so the corrupt store
	// gets none while the watch is open and no limit once it is closed.
	need := 0
	if mode == "clean" {
		head, err := writerGuards(repo, transaction.Mutate)
		if err != nil {
			t.Fatal(err)
		}
		reader := journalReader(repo, head)
		for free := 0; free <= 512 && need == 0; free++ {
			restore := limitDescriptors(t, free)
			_, err := reader.AuditForMutation(id)
			restore()
			if !descriptorsExhausted(err) {
				if err != nil || free == 0 {
					t.Fatalf("merged audit with %d free: %v", free, err)
				}
				need = free
			}
		}
		if need == 0 {
			t.Fatal("the merged audit needs more than 512 descriptors")
		}
		if after := openDescriptors(t); !reflect.DeepEqual(after, idle) {
			t.Fatalf("exhausted audits leaked descriptors: %v, before %v", after, idle)
		}
	}

	// Inside Mutate, with the watch registered, leave the audit fewer than it
	// needs; on the clean store, closing the watch alone gives it enough.
	var stages []string
	var registered []int
	restore := func() {}
	defer func() { restore() }()
	ctx := context.WithValue(context.Background(), mutationStageKey{}, func(stage string) {
		stages = append(stages, stage)
		switch {
		case stage == "watched":
			restore = limitDescriptors(t, max(0, need-held))
			registered = openDescriptors(t)
		case strings.HasPrefix(stage, "retry: "):
			open := openDescriptors(t)
			released := len(registered) - len(open)
			if !descriptorsExhausted(errors.New(stage)) || released != held || !subset(open, registered) {
				t.Fatalf("retry after %q released %d of the watch's %d descriptors", stage, released, held)
			}
			if mode == "corrupt" {
				restore()
			}
		case stage == "audited":
			restore()
		}
	})
	rep, err := Mutate(ctx, repo, historyActor, historyCreate(id, "descriptor "+mode), WallClock())
	restore()
	if len(stages) < 2 || stages[0] != "watched" || !strings.HasPrefix(stages[1], "retry: ") {
		t.Fatalf("stages %q: registration or the exhausted audit missing", stages)
	}
	switch mode {
	case "clean":
		if err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("Mutate after the retry: %+v %v", rep, err)
		}
		if want := []string{"watched", stages[1], "audited", "fresh"}; !reflect.DeepEqual(stages, want) {
			t.Fatalf("stages %q, want %q", stages, want)
		}
	case "corrupt":
		if err == nil || "lookup: "+err.Error() != oracle.Err {
			t.Fatalf("Mutate after the retry: %v (report %+v); separate passes: %s", err, rep, oracle.Err)
		}
		if len(stages) != 2 {
			t.Fatalf("stages %q, want the refusal after the retry", stages)
		}
		if p, _ := snapshot.RequestPath(id); fileExists(filepath.Join(repo.StateDir, p)) {
			t.Fatal("refused request was projected")
		}
	}
	if after := openDescriptors(t); !reflect.DeepEqual(after, idle) {
		t.Fatalf("Mutate left descriptors open: %v, before %v", after, idle)
	}
	needs := "any"
	if need > 0 {
		needs = fmt.Sprint(need)
	}
	t.Logf("descriptor retry %s: watch held %d, audit needs %s, %s, then %v", mode, held, needs, stages[1], err)
}

// openDescriptors lists open descriptor numbers without opening one.
func openDescriptors(t *testing.T) []int {
	t.Helper()
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		t.Fatal(err)
	}
	bound := min(limit.Cur, 1<<16)
	var open []int
	var st syscall.Stat_t
	for fd := 0; uint64(fd) < bound; fd++ {
		if syscall.Fstat(fd, &st) == nil {
			open = append(open, fd)
		}
	}
	return open
}

// limitDescriptors fills every gap below the highest open descriptor with
// /dev/null and sets the soft limit so exactly free more can open. The
// returned function restores the limit and closes the fillers.
func limitDescriptors(t *testing.T, free int) func() {
	t.Helper()
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
		t.Fatal(err)
	}
	open := openDescriptors(t)
	top := open[len(open)-1]
	var fillers []int
	for {
		fd, err := syscall.Open(os.DevNull, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
		if err != nil {
			t.Fatal(err)
		}
		if fd > top {
			syscall.Close(fd)
			break
		}
		fillers = append(fillers, fd)
	}
	limited := original
	limited.Cur = uint64(top + 1 + free)
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limited); err != nil {
		t.Fatal(err)
	}
	done := false
	return func() {
		if done {
			return
		}
		done = true
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
			t.Error(err)
		}
		for _, fd := range fillers {
			syscall.Close(fd)
		}
	}
}

func descriptorsExhausted(err error) bool {
	return err != nil && (errors.Is(err, syscall.EMFILE) || strings.Contains(err.Error(), syscall.EMFILE.Error()))
}

func subset(small, large []int) bool {
	in := map[int]bool{}
	for _, fd := range large {
		in[fd] = true
	}
	for _, fd := range small {
		if !in[fd] {
			return false
		}
	}
	return true
}
