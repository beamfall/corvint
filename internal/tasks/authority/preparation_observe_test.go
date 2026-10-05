//go:build darwin || linux

package authority

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// preparationNamespace captures every fixed coordination file's identity,
// bytes, mode and modification time, plus which names exist.
func preparationNamespace(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "taskman.prepare") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		st, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = st.Mode().String() + "|" + st.ModTime().Format(time.RFC3339Nano) + "|" + string(raw)
	}
	return out
}

// heldSlot publishes a record into a slot through an owned flock, as a
// registrant's slot descriptor does.
func heldSlot(t *testing.T, dir string, i int, record []byte) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, preparationSlotName(i)), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	if err = withFD(f, flockExclusiveNB); err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteAt(record, 0); err != nil {
		t.Fatal(err)
	}
}

func TestCALV0095_PreparationQueueObservation(t *testing.T) {
	assumeCompleteLockTable(t)
	t.Run("absent-namespace-creates-nothing", func(t *testing.T) {
		_, repo := preparationOpenFixture(t)
		before := preparationNamespace(t, repo.CommonDir)
		q := ObservePreparationQueue(repo)
		if q.NotObserved != "" || q.Method == "" || q.Registered != 0 || q.Unpublished != 0 || q.RegistryActive {
			t.Fatalf("%+v", q)
		}
		if r, ok := q.WouldBeRank(); !ok || r != 1 {
			t.Fatalf("would-be rank %d %t", r, ok)
		}
		if after := preparationNamespace(t, repo.CommonDir); len(before) != 0 || len(after) != 0 {
			t.Fatalf("observation created coordination files: %v -> %v", before, after)
		}
	})

	t.Run("registered-waiters-rank-and-no-write", func(t *testing.T) {
		admissionRestore(t)
		_, repo := preparationOpenFixture(t)
		const waiters = 4
		holder, err := AcquirePreparation(context.Background(), repo, LockOptions{Poll: time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		var mu sync.Mutex
		queued := map[uint64]bool{}
		admissionReached = func(event string, rank uint64) {
			if event == "queued" {
				mu.Lock()
				queued[rank] = true
				mu.Unlock()
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		errs := make(chan error, waiters)
		for i := 0; i < waiters; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				l, e := AcquirePreparation(ctx, repo, LockOptions{Poll: 5 * time.Millisecond})
				if l != nil {
					e = errors.Join(errors.New("waiter entered while holder served"), l.Close())
				}
				errs <- e
			}()
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			mu.Lock()
			n := len(queued)
			mu.Unlock()
			if n == waiters {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("only %d waiters queued", n)
			}
			time.Sleep(time.Millisecond)
		}
		before := preparationNamespace(t, repo.CommonDir)
		q := ObservePreparationQueue(repo)
		after := preparationNamespace(t, repo.CommonDir)
		cancel()
		wg.Wait()
		close(errs)
		for e := range errs {
			if !errors.Is(e, context.Canceled) {
				t.Errorf("waiter: %v", e)
			}
		}
		if e := holder.Close(); e != nil {
			t.Fatal(e)
		}
		// The holder and every waiter are live registrations, ranks 1..5.
		if q.NotObserved != "" || q.Registered != waiters+1 || q.Unpublished != 0 || q.MaxRank != waiters+1 {
			t.Fatalf("%+v", q)
		}
		if r, ok := q.WouldBeRank(); !ok || r != waiters+2 {
			t.Fatalf("would-be rank %d %t", r, ok)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("observation changed coordination files:\n%v\n%v", before, after)
		}
		if _, ok := before[preparationLockFileName]; !ok || len(before) != waiters+3 {
			t.Fatalf("unexpected namespace %v", before)
		}
		// Retired registrations leave stale records that are not live.
		stale := preparationNamespace(t, repo.CommonDir)
		q = ObservePreparationQueue(repo)
		if q.NotObserved != "" || q.Registered != 0 || q.MaxRank != 0 || q.RegistryActive {
			t.Fatalf("stale slots counted: %+v", q)
		}
		if r, ok := q.WouldBeRank(); !ok || r != 1 {
			t.Fatalf("would-be rank after retirement %d %t", r, ok)
		}
		if !reflect.DeepEqual(stale, preparationNamespace(t, repo.CommonDir)) {
			t.Fatal("observation of stale slots changed them")
		}
	})

	t.Run("unpublished-live-slot-hides-rank", func(t *testing.T) {
		_, repo := preparationOpenFixture(t)
		heldSlot(t, repo.CommonDir, 0, admissionRecord(7))
		heldSlot(t, repo.CommonDir, 1, []byte("CPA1"))
		q := ObservePreparationQueue(repo)
		if q.NotObserved != "" || q.Registered != 1 || q.Unpublished != 1 || q.MaxRank != 7 {
			t.Fatalf("%+v", q)
		}
		if _, ok := q.WouldBeRank(); ok {
			t.Fatal("rank reported despite an unreadable live slot")
		}
	})

	t.Run("unsafe-object-not-observed", func(t *testing.T) {
		_, repo := preparationOpenFixture(t)
		if err := os.Mkdir(filepath.Join(repo.CommonDir, preparationSlotName(3)), 0o755); err != nil {
			t.Fatal(err)
		}
		q := ObservePreparationQueue(repo)
		if q.NotObserved == "" || q.Registered != 0 {
			t.Fatalf("%+v", q)
		}
		if _, ok := q.WouldBeRank(); ok {
			t.Fatal("rank reported without observation")
		}
	})

	t.Run("drift-after-stat-not-observed", func(t *testing.T) {
		for _, replace := range []bool{false, true} {
			_, repo := preparationOpenFixture(t)
			heldSlot(t, repo.CommonDir, 0, admissionRecord(1))
			target := preparationSlotName(2)
			p := filepath.Join(repo.CommonDir, target)
			if err := os.WriteFile(p, admissionRecord(4), 0o644); err != nil {
				t.Fatal(err)
			}
			observeAfterLstat = func(name string) {
				if name != target {
					return
				}
				if !replace {
					if err := os.Remove(p); err != nil {
						t.Error(err)
					}
					return
				}
				// Create the replacement while the original still holds its
				// inode, so the two identities differ on every filesystem.
				tmp := p + ".replacement"
				if err := os.WriteFile(tmp, admissionRecord(4), 0o644); err != nil {
					t.Error(err)
				}
				if err := os.Rename(tmp, p); err != nil {
					t.Error(err)
				}
			}
			q := ObservePreparationQueue(repo)
			observeAfterLstat = nil
			if q.NotObserved != "preparation file "+target+" identity drift" || q.Registered != 0 {
				t.Fatalf("replace=%t: %+v", replace, q)
			}
			if _, ok := q.WouldBeRank(); ok {
				t.Fatalf("replace=%t: rank reported after drift", replace)
			}
		}
	})

	t.Run("drift-after-open-not-observed", func(t *testing.T) {
		for _, held := range []bool{false, true} {
			for _, replace := range []bool{false, true} {
				_, repo := preparationOpenFixture(t)
				heldSlot(t, repo.CommonDir, 0, admissionRecord(1))
				target := preparationSlotName(2)
				p := filepath.Join(repo.CommonDir, target)
				if held {
					heldSlot(t, repo.CommonDir, 2, admissionRecord(4))
				} else if err := os.WriteFile(p, admissionRecord(4), 0o644); err != nil {
					t.Fatal(err)
				}
				// The unlink or replacement lands after the open and before
				// the descriptor stat, so both compared identities describe
				// the original inode; only the post-observation revalidation
				// can see the drift.
				observeAfterOpen = func(name string) {
					if name != target {
						return
					}
					if !replace {
						if err := os.Remove(p); err != nil {
							t.Error(err)
						}
						return
					}
					tmp := p + ".replacement"
					if err := os.WriteFile(tmp, admissionRecord(4), 0o644); err != nil {
						t.Error(err)
					}
					if err := os.Rename(tmp, p); err != nil {
						t.Error(err)
					}
				}
				q := ObservePreparationQueue(repo)
				observeAfterOpen = nil
				if q.NotObserved != "preparation file "+target+" identity drift" || q.Registered != 0 {
					t.Fatalf("held=%t replace=%t: %+v", held, replace, q)
				}
				if _, ok := q.WouldBeRank(); ok {
					t.Fatalf("held=%t replace=%t: rank reported after drift", held, replace)
				}
			}
		}
	})

	t.Run("registry-drift-after-open-not-observed", func(t *testing.T) {
		_, repo := preparationOpenFixture(t)
		heldSlot(t, repo.CommonDir, 0, admissionRecord(1))
		p := filepath.Join(repo.CommonDir, preparationRegistryName)
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		observeAfterOpen = func(name string) {
			if name == preparationRegistryName {
				if err := os.Remove(p); err != nil {
					t.Error(err)
				}
			}
		}
		q := ObservePreparationQueue(repo)
		observeAfterOpen = nil
		if q.NotObserved != "preparation file "+preparationRegistryName+" identity drift" || q.Registered != 0 {
			t.Fatalf("%+v", q)
		}
	})

	t.Run("lock-query-unavailable-not-observed", func(t *testing.T) {
		_, repo := preparationOpenFixture(t)
		old := preparationLockViewLoad
		t.Cleanup(func() { preparationLockViewLoad = old })
		preparationLockViewLoad = func() (preparationLockView, error) { return nil, errors.New("no lock query") }
		heldSlot(t, repo.CommonDir, 0, admissionRecord(1))
		q := ObservePreparationQueue(repo)
		if q.NotObserved != "no lock query" || q.Registered != 0 || q.Method != "" {
			t.Fatalf("%+v", q)
		}
		if _, ok := q.WouldBeRank(); ok {
			t.Fatal("rank reported without observation")
		}
	})
}

// The helper holds one real registration in a separate process until its
// stdin closes, so the lock query is proven against another process's owner.
func TestCALV0095ProcessHolderHelper(t *testing.T) {
	root := os.Getenv("CAL095_HOLD_ROOT")
	if root == "" {
		t.Skip("helper process only")
	}
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	l, err := AcquirePreparation(context.Background(), repo, LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout.WriteString("held\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCALV0095_PreparationQueueObservationAcrossProcesses(t *testing.T) {
	assumeCompleteLockTable(t)
	r, repo := preparationOpenFixture(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestCALV0095ProcessHolderHelper$")
	cmd.Env = append(os.Environ(), "CAL095_HOLD_ROOT="+r.Root)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || line != "held\n" {
		in.Close()
		_ = cmd.Wait()
		t.Fatalf("helper did not hold: %q %v", line, err)
	}
	before := preparationNamespace(t, repo.CommonDir)
	q := ObservePreparationQueue(repo)
	after := preparationNamespace(t, repo.CommonDir)
	in.Close()
	go io.Copy(io.Discard, out)
	if err = cmd.Wait(); err != nil {
		t.Fatalf("helper: %v", err)
	}
	if r, ok := q.WouldBeRank(); q.NotObserved != "" || q.Registered != 1 || !ok || r != 2 {
		t.Fatalf("live foreign registration: %+v", q)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("observation changed coordination files")
	}
	if q = ObservePreparationQueue(repo); q.NotObserved != "" || q.Registered != 0 {
		t.Fatalf("after helper exit: %+v", q)
	}
}

// The helper holds one non-flock record lock on a path until stdin closes.
func TestCALV0095RecordLockHelper(t *testing.T) {
	path, kind := os.Getenv("CAL095_RECORD_PATH"), os.Getenv("CAL095_RECORD_KIND")
	if path == "" {
		t.Skip("helper process only")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = holdRecordLock(f, kind); err != nil {
		t.Fatal(err)
	}
	os.Stdout.WriteString("held\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

// TestCALV0095_RecordLockIsNotARegistration: another process's POSIX or OFD
// record lock on a retired slot never reads as a registered writer. Linux
// ignores it (the lock families do not interact there); Darwin abstains,
// because its query cannot then rule out a coexisting flock.
func TestCALV0095_RecordLockIsNotARegistration(t *testing.T) {
	assumeCompleteLockTable(t)
	for _, kind := range recordLockKinds {
		_, repo := preparationOpenFixture(t)
		p := filepath.Join(repo.CommonDir, preparationSlotName(5))
		if err := os.WriteFile(p, admissionRecord(9), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestCALV0095RecordLockHelper$")
		cmd.Env = append(os.Environ(), "CAL095_RECORD_PATH="+p, "CAL095_RECORD_KIND="+kind)
		in, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		line, err := bufio.NewReader(out).ReadString('\n')
		if err != nil || line != "held\n" {
			in.Close()
			_ = cmd.Wait()
			t.Fatalf("%s helper did not hold: %q %v", kind, line, err)
		}
		before := preparationNamespace(t, repo.CommonDir)
		q := ObservePreparationQueue(repo)
		after := preparationNamespace(t, repo.CommonDir)
		in.Close()
		go io.Copy(io.Discard, out)
		if err = cmd.Wait(); err != nil {
			t.Fatalf("%s helper: %v", kind, err)
		}
		checkRecordLockObservation(t, kind, q)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("%s: observation changed coordination files", kind)
		}
	}
}
