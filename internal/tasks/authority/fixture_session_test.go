//go:build darwin || linux

package authority

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func fixtureMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func fixtureHarness(t *testing.T) (*fixtureSession, *fixture.Repo) {
	t.Helper()
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	fixtureMust(t, err)
	f, err := os.Open(r.Root)
	fixtureMust(t, err)
	_, observed := fixtureObserveMount(f)
	fixtureMust(t, f.Close())
	if observed != nil {
		t.Skipf("NOT_RUN: native fixture host/mount unsupported: %v", observed)
	}
	for _, path := range []string{"staging", "receipts", "evidence", "pinned", "requests/00"} {
		fixtureMust(t, os.MkdirAll(filepath.Join(r.StateDir, path), 0700))
	}
	fixtureMust(t, os.MkdirAll(filepath.Join(r.IntentDir, "tickets"), 0700))
	lock, err := AcquireLock(context.Background(), repo, LockOptions{})
	fixtureMust(t, err)
	s, err := newFixtureSession(repo, lock)
	if err != nil {
		fixtureMust(t, lock.Close())
		t.Fatal(err)
	}
	t.Cleanup(func() { s.before = nil; fixtureMust(t, s.close()) })
	return s, r
}
func fixturePrepared(t *testing.T, s *fixtureSession, role fixtureRole, raw string) *fixtureStage {
	t.Helper()
	stage, err := s.prepare("a00", role, []byte(raw))
	fixtureMust(t, err)
	return stage
}
func fixtureBytes(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	fixtureMust(t, err)
	if string(raw) != want {
		t.Fatalf("%s: got %q want %q", path, raw, want)
	}
}
func fixtureAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected absent %s: %v", path, err)
	}
}
func fixtureEffect(t *testing.T, err error, want bool) {
	t.Helper()
	var effect *fixtureEffectError
	if err == nil || errors.As(err, &effect) != want {
		t.Fatalf("effect=%v: %v", want, err)
	}
}
func fixtureForeign(t *testing.T, path, kind string) {
	t.Helper()
	switch kind {
	case "same":
		fixtureMust(t, os.WriteFile(path, []byte("bytes"), 0640))
	case "different":
		fixtureMust(t, os.WriteFile(path, []byte("foreign"), 0640))
	case "empty":
		fixtureMust(t, os.WriteFile(path, nil, 0640))
	case "symlink":
		fixtureMust(t, os.Symlink("missing", path))
	case "fifo":
		fixtureMust(t, syscall.Mkfifo(path, 0600))
	case "directory":
		fixtureMust(t, os.Mkdir(path, 0700))
	}
}
func fixturePreserved(t *testing.T, path string, before os.FileInfo, raw string) {
	t.Helper()
	after, err := os.Lstat(path)
	fixtureMust(t, err)
	if !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		t.Fatal("foreign name or metadata changed")
	}
	if before.Mode().IsRegular() {
		fixtureBytes(t, path, raw)
	}
	if before.Mode()&os.ModeSymlink != 0 {
		got, err := os.Readlink(path)
		fixtureMust(t, err)
		if got != "missing" {
			t.Fatal("symlink changed")
		}
	}
}

func TestTMV0001_AS10_FixtureSessionIdentity(t *testing.T) {
	for _, kind := range []string{"matching", "nil-repo", "nil-lock", "zero-lock", "wrong-lock", "wrong-repo", "closed-lock", "lock", "common", "state", "staging", "receipts", "primary", "intent"} {
		t.Run(kind, func(t *testing.T) {
			s, r := fixtureHarness(t)
			switch kind {
			case "matching":
				fixtureMust(t, s.operation(func() error { return nil }))
				return
			case "nil-repo", "nil-lock", "zero-lock", "wrong-lock", "wrong-repo", "closed-lock":
				repo := s.repo
				lock := s.lock
				var rp *intent.Repository = &repo
				switch kind {
				case "nil-repo":
					rp = nil
				case "nil-lock":
					lock = nil
				case "zero-lock":
					lock = &Lock{}
				case "wrong-lock":
					other, _ := fixtureHarness(t)
					lock = other.lock
				case "wrong-repo":
					repo.StateDir += "-wrong"
				case "closed-lock":
					fixtureMust(t, lock.Close())
				}
				got, err := newFixtureSession(rp, lock)
				if got != nil || err == nil {
					t.Fatal("invalid constructor accepted")
				}
				return
			}
			path := map[string]string{"lock": s.repo.LockPath, "common": r.CommonDir, "state": r.StateDir, "staging": filepath.Join(r.StateDir, "staging"), "receipts": filepath.Join(r.StateDir, "receipts"), "primary": r.Root, "intent": r.IntentDir}[kind]
			fixtureMust(t, os.Rename(path, path+"-held"))
			if kind == "lock" {
				fixtureMust(t, os.WriteFile(path, []byte("foreign lock"), 0600))
			} else {
				fixtureMust(t, os.Mkdir(path, 0700))
			}
			info, err := os.Lstat(path)
			fixtureMust(t, err)
			if err := s.operation(func() error { t.Fatal("ran under replaced identity"); return nil }); err == nil {
				t.Fatal("identity accepted")
			}
			raw := ""
			if kind == "lock" {
				raw = "foreign lock"
			}
			fixturePreserved(t, path, info, raw)
		})
	}
	var zero fixtureSession
	if _, err := zero.prepare("a00", fixtureEvidence, nil); err == nil {
		t.Fatal("zero session accepted")
	}
	if err := zero.close(); err == nil {
		t.Fatal("zero close accepted")
	}
}

func TestTMV0002_AS10_FixtureStageRolesAndBounds(t *testing.T) {
	roles := []fixtureRole{fixtureReceipt, fixtureEvidence, fixturePin, fixtureRequest, fixtureVersion, fixtureHead, fixtureBarrier, fixtureReservations, fixtureQueue, fixturePolicy, fixtureImportMap, fixtureTicket}
	for _, role := range roles {
		t.Run(fmt.Sprint(role), func(t *testing.T) {
			s, _ := fixtureHarness(t)
			data := []byte("bytes")
			if role == fixtureVersion {
				data = []byte("taskman-state/0\n")
			}
			stage, err := s.prepare("a00", role, data)
			fixtureMust(t, err)
			if stage.role != role || stage.digest != wire.Sum(data) {
				t.Fatal("wrong stage identity")
			}
		})
	}
	s, r := fixtureHarness(t)
	for i := 0; i <= 10; i++ {
		stage, err := s.prepare(fixtureSlot(fmt.Sprintf("a%02d", i)), fixtureEvidence, nil)
		fixtureMust(t, err)
		if stage.size != 0 {
			t.Fatal("empty evidence lost")
		}
	}
	for _, slot := range []fixtureSlot{"active.json", "a11", "a-1", "a0x", "a000", "A00", "a00/", "./a00", "../a00", "a00\x00", "", fixtureSlot(strings.Repeat("x", 256))} {
		if _, err := s.prepare(slot, fixtureEvidence, nil); err == nil {
			t.Fatalf("slot %q accepted", slot)
		}
	}
	for _, size := range []int{0, 2422, 2423} {
		t.Run(fmt.Sprintf("descriptor-%d", size), func(t *testing.T) {
			s, r := fixtureHarness(t)
			stage, err := s.prepare("active.json.tmp", fixtureDescriptor, []byte(strings.Repeat("x", size)))
			if size == 2422 {
				fixtureMust(t, err)
				if stage == nil {
					t.Fatal("no stage")
				}
			} else {
				fixtureEffect(t, err, false)
				fixtureAbsent(t, filepath.Join(r.StateDir, "staging", "active.json.tmp"))
			}
		})
	}
	if _, err := s.prepare("active.json.tmp", fixtureEvidence, nil); err == nil {
		t.Fatal("wrong temp role")
	}
	if _, err := s.prepare("a00", fixtureDescriptor, []byte("x")); err == nil {
		t.Fatal("descriptor in payload")
	}
	// Small head bound exercises exact bound/+1 without large allocations.
	for _, size := range []int{wire.MaxJournalHeadBytes, wire.MaxJournalHeadBytes + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			s, _ := fixtureHarness(t)
			stage, err := s.prepare("a00", fixtureHead, []byte(strings.Repeat("x", size)))
			if size == wire.MaxJournalHeadBytes {
				fixtureMust(t, err)
			} else if err == nil || stage != nil {
				t.Fatal("overcap accepted")
			}
		})
	}
	entries, err := os.ReadDir(filepath.Join(r.StateDir, "staging"))
	fixtureMust(t, err)
	if len(entries) != 11 {
		t.Fatal("refusal created a name")
	}
}

func TestTMV0009_AS10_FixtureStageForeignAndFaults(t *testing.T) {
	for _, kind := range []string{"same", "different", "empty", "symlink", "fifo", "directory"} {
		t.Run(kind, func(t *testing.T) {
			s, r := fixtureHarness(t)
			path := filepath.Join(r.StateDir, "staging", "a00")
			fixtureForeign(t, path, kind)
			info, err := os.Lstat(path)
			fixtureMust(t, err)
			raw := ""
			if kind == "same" {
				raw = "bytes"
			}
			if kind == "different" {
				raw = "foreign"
			}
			stage, err := s.prepare("a00", fixtureEvidence, []byte("bytes"))
			if stage != nil {
				t.Fatal("foreign stage token")
			}
			fixtureEffect(t, err, false)
			fixturePreserved(t, path, info, raw)
		})
	}
	for _, point := range []string{"create", "write", "partial", "short", "file-sync", "sync:staging", "stage-close"} {
		t.Run(point, func(t *testing.T) {
			s, r := fixtureHarness(t)
			boom := errors.New(point)
			s.before = func(at string) error {
				if at == point {
					return boom
				}
				return nil
			}
			if point == "partial" || point == "short" {
				s.write = func(f *os.File, b []byte) (int, error) {
					n, err := f.Write(b[:2])
					if err != nil {
						return n, err
					}
					if point == "partial" {
						return n, boom
					}
					return n, nil
				}
			}
			stage, err := s.prepare("a00", fixtureEvidence, []byte("bytes"))
			if stage != nil {
				t.Fatal("incomplete stage publishable")
			}
			fixtureEffect(t, err, point != "create")
			if point == "short" {
				if !errors.Is(err, io.ErrShortWrite) {
					t.Fatal(err)
				}
			} else if !errors.Is(err, boom) {
				t.Fatal(err)
			}
			fixtureAbsent(t, filepath.Join(r.StateDir, "staging", "a00"))
		})
	}
}

func TestTMV0009_AS11_FixtureExclusiveLink(t *testing.T) {
	for _, kind := range []string{"absent", "same", "different", "empty", "symlink", "fifo", "directory"} {
		t.Run(kind, func(t *testing.T) {
			s, r := fixtureHarness(t)
			stage := fixturePrepared(t, s, fixtureReceipt, "bytes")
			path := filepath.Join(r.StateDir, "receipts", "000000000001.json")
			if kind != "absent" {
				fixtureForeign(t, path, kind)
			}
			info, _ := os.Lstat(path)
			err := s.link(stage, fixtureTarget{fixtureReceipt, "000000000001.json"})
			if kind == "absent" {
				fixtureMust(t, err)
				fixtureBytes(t, path, "bytes")
			} else {
				fixtureEffect(t, err, false)
				raw := ""
				if kind == "same" {
					raw = "bytes"
				}
				if kind == "different" {
					raw = "foreign"
				}
				fixturePreserved(t, path, info, raw)
			}
			fixtureBytes(t, filepath.Join(r.StateDir, "staging", "a00"), "bytes")
		})
	}
	for _, point := range []string{"link", "native:link", "sync:receipts", "source-close"} {
		t.Run(point, func(t *testing.T) {
			s, r := fixtureHarness(t)
			stage := fixturePrepared(t, s, fixtureReceipt, "bytes")
			boom := errors.New(point)
			s.before = func(at string) error {
				if at == point {
					return boom
				}
				return nil
			}
			if point == "source-close" {
				original := s.closeFile
				s.closeFile = func(f *os.File) error {
					err := original(f)
					if f.Name() == "a00" {
						return errors.Join(err, boom)
					}
					return err
				}
			}
			err := s.link(stage, fixtureTarget{fixtureReceipt, "000000000001.json"})
			after := point == "sync:receipts" || point == "source-close"
			fixtureEffect(t, err, after)
			if !errors.Is(err, boom) {
				t.Fatal(err)
			}
			path := filepath.Join(r.StateDir, "receipts", "000000000001.json")
			if after {
				fixtureBytes(t, path, "bytes")
			} else {
				fixtureAbsent(t, path)
			}
			fixtureBytes(t, filepath.Join(r.StateDir, "staging", "a00"), "bytes")
		})
	}
}

func TestTMV0007_AS10_FixtureClosedDestinations(t *testing.T) {
	s, r := fixtureHarness(t)
	stage := fixturePrepared(t, s, fixtureEvidence, "")
	fixtureMust(t, s.link(stage, fixtureTarget{fixtureEvidence, string(wire.Sum(nil))}))
	for _, target := range []fixtureTarget{{fixtureEvidence, "../escape"}, {fixtureEvidence, strings.Repeat("A", 64)}, {fixtureReceipt, "1.json"}, {fixtureReceipt, "000000000000.json"}, {fixtureHead, "taskman.lock"}, {fixtureDescriptor, "active.json.tmp"}, {fixtureTicket, "../x.json"}, {fixtureTicket, "A.json/"}, {fixtureTicket, "A\x00.json"}, {0, "head.json"}} {
		if err := s.link(stage, target); err == nil {
			t.Fatalf("target accepted %+v", target)
		}
	}
	for _, role := range []fixtureRole{fixtureReceipt, fixtureEvidence, fixturePin, fixtureRequest, fixtureVersion, fixtureQueue, fixturePolicy, fixtureImportMap, fixtureTicket, fixtureDescriptor} {
		target := fixtureTarget{role, "head.json"}
		pre := wire.Sum([]byte("bytes"))
		if err := s.replace(stage, target, &pre); err == nil {
			t.Fatal("forbidden replacement")
		}
		if err := s.removeBarrier(target, pre); err == nil {
			t.Fatal("forbidden removal")
		}
	}
	queue, err := s.prepare("a01", fixtureQueue, []byte("queue"))
	fixtureMust(t, err)
	target := fixtureTarget{fixtureQueue, "queue.json"}
	fixtureMust(t, s.link(queue, target))
	info, err := os.Lstat(filepath.Join(r.IntentDir, "queue.json"))
	fixtureMust(t, err)
	if err := s.link(queue, target); err == nil {
		t.Fatal("existing intent overwritten")
	}
	fixturePreserved(t, filepath.Join(r.IntentDir, "queue.json"), info, "queue")
	descriptor, err := s.prepare("active.json.tmp", fixtureDescriptor, []byte("descriptor"))
	fixtureMust(t, err)
	fixtureMust(t, s.link(descriptor, fixtureTarget{fixtureDescriptor, "active.json"}))
	fixtureBytes(t, filepath.Join(r.StateDir, "staging", "active.json.tmp"), "descriptor")
	fixtureMust(t, s.removeStage("active.json"))
	if err := s.removeStage("unowned"); err == nil {
		t.Fatal("unowned removal")
	}
}

func TestTMV0009_AS11_FixtureMutableReplace(t *testing.T) {
	t.Run("invalid-pre", func(t *testing.T) {
		s, r := fixtureHarness(t)
		stage := fixturePrepared(t, s, fixtureHead, "post")
		path := filepath.Join(r.StateDir, "head.json")
		fixtureMust(t, os.WriteFile(path, []byte("post"), 0600))
		info, err := os.Lstat(path)
		fixtureMust(t, err)
		bad := wire.Digest("invalid")
		fixtureEffect(t, s.replace(stage, fixtureTarget{fixtureHead, "head.json"}, &bad), false)
		fixturePreserved(t, path, info, "post")
	})
	for _, role := range []fixtureRole{fixtureHead, fixtureBarrier, fixtureReservations} {
		for _, value := range []string{"absent", "pre", "post", "third", "symlink", "fifo", "directory"} {
			t.Run(fmt.Sprintf("%d/%s", role, value), func(t *testing.T) {
				s, r := fixtureHarness(t)
				stage := fixturePrepared(t, s, role, "post")
				name := map[fixtureRole]string{fixtureHead: "head.json", fixtureBarrier: "barrier.json", fixtureReservations: "reservations.json"}[role]
				path := filepath.Join(r.StateDir, name)
				var expected *wire.Digest
				pre := wire.Sum([]byte("pre"))
				if value != "absent" {
					expected = &pre
				}
				switch value {
				case "pre", "post", "third":
					fixtureMust(t, os.WriteFile(path, []byte(value), 0640))
				case "symlink", "fifo", "directory":
					fixtureForeign(t, path, value)
				}
				info, _ := os.Lstat(path)
				err := s.replace(stage, fixtureTarget{role, name}, expected)
				if value == "absent" || value == "pre" || value == "post" {
					fixtureMust(t, err)
					fixtureBytes(t, path, "post")
				} else {
					fixtureEffect(t, err, false)
					fixturePreserved(t, path, info, value)
				}
				source := filepath.Join(r.StateDir, "staging", "a00")
				if value == "pre" {
					fixtureAbsent(t, source)
					if err := s.link(stage, fixtureTarget{role, name}); err == nil {
						t.Fatal("consumed token reused")
					}
				} else {
					fixtureBytes(t, source, "post")
				}
			})
		}
	}
	for _, point := range []string{"rename", "native:rename", "sync:state", "sync:staging", "dest-close", "source-close"} {
		t.Run(point, func(t *testing.T) {
			s, r := fixtureHarness(t)
			stage := fixturePrepared(t, s, fixtureHead, "post")
			path := filepath.Join(r.StateDir, "head.json")
			fixtureMust(t, os.WriteFile(path, []byte("pre"), 0600))
			pre := wire.Sum([]byte("pre"))
			boom := errors.New(point)
			s.before = func(at string) error {
				if at == point {
					return boom
				}
				return nil
			}
			if strings.HasSuffix(point, "close") {
				old := s.closeFile
				s.closeFile = func(f *os.File) error {
					err := old(f)
					if point == "dest-close" && f.Name() == "head.json" || point == "source-close" && f.Name() == "a00" {
						return errors.Join(err, boom)
					}
					return err
				}
			}
			err := s.replace(stage, fixtureTarget{fixtureHead, "head.json"}, &pre)
			after := point != "rename" && point != "native:rename"
			fixtureEffect(t, err, after)
			if !errors.Is(err, boom) {
				t.Fatal(err)
			}
			if after {
				fixtureBytes(t, path, "post")
				fixtureAbsent(t, filepath.Join(r.StateDir, "staging", "a00"))
			} else {
				fixtureBytes(t, path, "pre")
				fixtureBytes(t, filepath.Join(r.StateDir, "staging", "a00"), "post")
			}
		})
	}
}

func TestTMV0009_AS11_FixtureRemove(t *testing.T) {
	for _, value := range []string{"absent", "pre", "third", "symlink", "fifo", "directory"} {
		t.Run(value, func(t *testing.T) {
			s, r := fixtureHarness(t)
			path := filepath.Join(r.StateDir, "barrier.json")
			switch value {
			case "pre", "third":
				fixtureMust(t, os.WriteFile(path, []byte(value), 0640))
			case "symlink", "fifo", "directory":
				fixtureForeign(t, path, value)
			}
			info, _ := os.Lstat(path)
			err := s.removeBarrier(fixtureTarget{fixtureBarrier, "barrier.json"}, wire.Sum([]byte("pre")))
			if value == "absent" || value == "pre" {
				fixtureMust(t, err)
				fixtureAbsent(t, path)
			} else {
				fixtureEffect(t, err, false)
				fixturePreserved(t, path, info, value)
			}
		})
	}
	for _, stageRemoval := range []bool{false, true} {
		for _, point := range []string{"unlink", "native:unlink", "sync", "close"} {
			t.Run(fmt.Sprintf("%v/%s", stageRemoval, point), func(t *testing.T) {
				s, r := fixtureHarness(t)
				path := filepath.Join(r.StateDir, "barrier.json")
				name := "barrier.json"
				boundary := point
				if stageRemoval {
					fixturePrepared(t, s, fixtureEvidence, "pre")
					path = filepath.Join(r.StateDir, "staging", "a00")
					name = "a00"
					if point == "unlink" {
						boundary = "cleanup-unlink"
					}
				} else {
					fixtureMust(t, os.WriteFile(path, []byte("pre"), 0600))
				}
				if point == "sync" {
					boundary = "sync:state"
					if stageRemoval {
						boundary = "sync:staging"
					}
				}
				boom := errors.New(point)
				s.before = func(at string) error {
					if at == boundary {
						return boom
					}
					return nil
				}
				if point == "close" {
					old := s.closeFile
					s.closeFile = func(f *os.File) error {
						err := old(f)
						if f.Name() == name {
							return errors.Join(err, boom)
						}
						return err
					}
				}
				var err error
				if stageRemoval {
					err = s.removeStage("a00")
				} else {
					err = s.removeBarrier(fixtureTarget{fixtureBarrier, "barrier.json"}, wire.Sum([]byte("pre")))
				}
				after := point == "sync" || point == "close"
				fixtureEffect(t, err, after)
				if !errors.Is(err, boom) {
					t.Fatal(err)
				}
				if after {
					fixtureAbsent(t, path)
				} else {
					fixtureBytes(t, path, "pre")
				}
			})
		}
	}
}

func TestTMV0010_AS10_FixtureMkdir(t *testing.T) {
	for _, point := range []string{"success", "mkdir", "native:mkdir", "sync:requests/01", "sync:requests"} {
		t.Run(point, func(t *testing.T) {
			s, r := fixtureHarness(t)
			boom := errors.New(point)
			s.before = func(at string) error {
				if at == point {
					return boom
				}
				return nil
			}
			err := s.mkdir("requests/01")
			after := strings.HasPrefix(point, "sync:")
			if point == "success" {
				fixtureMust(t, err)
			} else {
				fixtureEffect(t, err, after)
				if !errors.Is(err, boom) {
					t.Fatal(err)
				}
			}
			path := filepath.Join(r.StateDir, "requests", "01")
			if after || point == "success" {
				info, err := os.Lstat(path)
				fixtureMust(t, err)
				if !info.IsDir() {
					t.Fatal("missing created dir")
				}
			} else {
				fixtureAbsent(t, path)
			}
		})
	}
	for _, kind := range []string{"same", "empty", "symlink", "fifo", "directory"} {
		t.Run(kind, func(t *testing.T) {
			s, r := fixtureHarness(t)
			path := filepath.Join(r.StateDir, "requests", "01")
			fixtureForeign(t, path, kind)
			info, err := os.Lstat(path)
			fixtureMust(t, err)
			fixtureEffect(t, s.mkdir("requests/01"), false)
			raw := ""
			if kind == "same" {
				raw = "bytes"
			}
			fixturePreserved(t, path, info, raw)
		})
	}
	s, _ := fixtureHarness(t)
	for _, key := range []string{"common", "primary", "requests/../01", "requests/FF", "requests/001", "worktrees", "/tmp", "receipts/x", "requests/01/"} {
		if err := s.mkdir(key); err == nil {
			t.Fatalf("directory key %q accepted", key)
		}
	}
}

func TestTMV0001_AS10_FixtureReplacementPreservation(t *testing.T) {
	for _, op := range []string{"link", "rename", "unlink", "cleanup-unlink", "cleanup", "write", "stage-close"} {
		for _, kind := range []string{"different", "symlink", "fifo", "directory", "in-place"} {
			t.Run(op+"/"+kind, func(t *testing.T) {
				s, r := fixtureHarness(t)
				sourcePath := filepath.Join(r.StateDir, "staging", "a00")
				targetPath := filepath.Join(r.StateDir, "head.json")
				var stage *fixtureStage
				if op != "cleanup" && op != "write" && op != "stage-close" {
					stage = fixturePrepared(t, s, fixtureHead, "post")
				}
				if op == "rename" {
					fixtureMust(t, os.WriteFile(targetPath, []byte("pre"), 0640))
				}
				if op == "unlink" {
					targetPath = filepath.Join(r.StateDir, "barrier.json")
					fixtureMust(t, os.WriteFile(targetPath, []byte("pre"), 0640))
				}
				path := sourcePath
				if op == "rename" || op == "unlink" {
					path = targetPath
				}
				var preserved os.FileInfo
				fired := false
				s.before = func(at string) error {
					if op == "cleanup" && at == "file-sync" {
						return syscall.EIO
					}
					if at != op || fired {
						return nil
					}
					fired = true
					if kind == "in-place" {
						fixtureMust(t, os.WriteFile(path, []byte("foreign"), 0600))
					} else {
						fixtureMust(t, os.Rename(path, path+"-owned"))
						fixtureForeign(t, path, kind)
					}
					var err error
					preserved, err = os.Lstat(path)
					fixtureMust(t, err)
					return nil
				}
				var err error
				switch op {
				case "link":
					err = s.link(stage, fixtureTarget{fixtureHead, "head.json"})
				case "rename":
					pre := wire.Sum([]byte("pre"))
					err = s.replace(stage, fixtureTarget{fixtureHead, "head.json"}, &pre)
				case "unlink":
					err = s.removeBarrier(fixtureTarget{fixtureBarrier, "barrier.json"}, wire.Sum([]byte("pre")))
				case "cleanup-unlink":
					err = s.removeStage("a00")
				default:
					var got *fixtureStage
					got, err = s.prepare("a00", fixtureHead, []byte("post"))
					if got != nil {
						t.Fatal("foreign stage token")
					}
				}
				if !fired || err == nil {
					t.Fatalf("not refused fired=%v err=%v", fired, err)
				}
				raw := "foreign"
				fixturePreserved(t, path, preserved, raw)
			})
		}
	}
	for _, key := range []string{"common", "state", "staging", "receipts", "intent"} {
		t.Run("parent/"+key, func(t *testing.T) {
			s, r := fixtureHarness(t)
			stage := fixturePrepared(t, s, fixtureReceipt, "bytes")
			paths := map[string]string{"common": r.CommonDir, "state": r.StateDir, "staging": filepath.Join(r.StateDir, "staging"), "receipts": filepath.Join(r.StateDir, "receipts"), "intent": r.IntentDir}
			path := paths[key]
			fired := false
			s.before = func(at string) error {
				if at != "link" || fired {
					return nil
				}
				fired = true
				fixtureMust(t, os.Rename(path, path+"-held"))
				fixtureMust(t, os.Mkdir(path, 0700))
				return nil
			}
			fixtureEffect(t, s.link(stage, fixtureTarget{fixtureReceipt, "000000000001.json"}), false)
			entries, err := os.ReadDir(path)
			fixtureMust(t, err)
			if len(entries) != 0 {
				t.Fatal("foreign parent modified")
			}
		})
	}
}

func TestTMV0010_AS10_FixtureMountBoundaries(t *testing.T) {
	for _, kind := range []string{"device", "filesystem", "mount", "unknown", "error", "late-EXDEV"} {
		t.Run(kind, func(t *testing.T) {
			s, r := fixtureHarness(t)
			stage := fixturePrepared(t, s, fixtureReceipt, "bytes")
			old := s.observe
			if kind == "late-EXDEV" {
				s.before = func(at string) error {
					if at == "native:link" {
						return syscall.EXDEV
					}
					return nil
				}
			} else {
				s.observe = func(f *os.File) (fixtureMount, error) {
					m, err := old(f)
					if err != nil {
						return m, err
					}
					switch kind {
					case "device":
						m.device++
					case "filesystem":
						m.filesystem += "other"
					case "mount":
						m.mount += "other"
					case "unknown":
						m.mount = ""
					case "error":
						return fixtureMount{}, syscall.EIO
					}
					return m, nil
				}
			}
			err := s.link(stage, fixtureTarget{fixtureReceipt, "000000000001.json"})
			fixtureEffect(t, err, false)
			if kind == "late-EXDEV" && !errors.Is(err, syscall.EXDEV) {
				t.Fatal(err)
			}
			fixtureAbsent(t, filepath.Join(r.StateDir, "receipts", "000000000001.json"))
			fixtureBytes(t, filepath.Join(r.StateDir, "staging", "a00"), "bytes")
		})
	}
}

func TestTMV0009_AS10_FixtureJoinedFailures(t *testing.T) {
	s, r := fixtureHarness(t)
	primary := errors.New("primary write")
	cleanup := errors.New("cleanup unlink")
	closeErr := errors.New("close")
	s.write = func(f *os.File, b []byte) (int, error) { n, err := f.Write(b[:2]); return n, errors.Join(err, primary) }
	s.before = func(at string) error {
		if at == "cleanup-unlink" {
			return cleanup
		}
		return nil
	}
	old := s.closeFile
	s.closeFile = func(f *os.File) error {
		err := old(f)
		if f.Name() == "a00" {
			return errors.Join(err, closeErr)
		}
		return err
	}
	stage, err := s.prepare("a00", fixtureEvidence, []byte("bytes"))
	if stage != nil {
		t.Fatal("failed stage token")
	}
	fixtureEffect(t, err, true)
	for _, want := range []error{primary, cleanup, closeErr} {
		if !errors.Is(err, want) {
			t.Fatalf("lost %v: %v", want, err)
		}
	}
	fixtureBytes(t, filepath.Join(r.StateDir, "staging", "a00"), "by")
	// Both rename directory sync failures must survive along with source close.
	s2, r2 := fixtureHarness(t)
	post := fixturePrepared(t, s2, fixtureHead, "post")
	fixtureMust(t, os.WriteFile(filepath.Join(r2.StateDir, "head.json"), []byte("pre"), 0600))
	pre := wire.Sum([]byte("pre"))
	s2.before = func(at string) error {
		switch at {
		case "sync:state":
			return primary
		case "sync:staging":
			return cleanup
		}
		return nil
	}
	close2 := s2.closeFile
	s2.closeFile = func(f *os.File) error {
		err := close2(f)
		if f.Name() == "a00" {
			return errors.Join(err, closeErr)
		}
		return err
	}
	err = s2.replace(post, fixtureTarget{fixtureHead, "head.json"}, &pre)
	fixtureEffect(t, err, true)
	for _, want := range []error{primary, cleanup, closeErr} {
		if !errors.Is(err, want) {
			t.Fatalf("lost %v: %v", want, err)
		}
	}
	fixtureBytes(t, filepath.Join(r2.StateDir, "head.json"), "post")
	fixtureAbsent(t, filepath.Join(r2.StateDir, "staging", "a00"))
}

func TestTMV0009_AS10_FixtureLifetimeAndClose(t *testing.T) {
	for _, closer := range []string{"lock", "session", "both", "lock-late", "session-late", "both-late"} {
		t.Run(closer, func(t *testing.T) {
			s, r := fixtureHarness(t)
			stage := fixturePrepared(t, s, fixtureReceipt, "bytes")
			entered, release := make(chan struct{}), make(chan struct{})
			operationDone := make(chan error, 1)
			s.before = func(at string) error {
				point := "native:link"
				if strings.HasSuffix(closer, "-late") {
					point = "sync:receipts"
				}
				if at == point {
					close(entered)
					<-release
				}
				return nil
			}
			go func() { operationDone <- s.link(stage, fixtureTarget{fixtureReceipt, "000000000001.json"}) }()
			<-entered
			closeDone := make(chan error, 2)
			count := 0
			if !strings.HasPrefix(closer, "session") {
				count++
				go func() { closeDone <- s.lock.Close() }()
			}
			if !strings.HasPrefix(closer, "lock") {
				count++
				go func() { closeDone <- s.close() }()
			}
			// Independent open description cannot acquire flock while the operation
			// is paused, even with Close pending. No child processes are needed.
			f, err := os.OpenFile(s.repo.LockPath, os.O_RDWR, 0)
			fixtureMust(t, err)
			lockErr := withFD(f, flockExclusiveNB)
			if !isWouldBlock(lockErr) {
				_ = withFD(f, flockUnlock)
			}
			fixtureMust(t, f.Close())
			early := false
			select {
			case <-closeDone:
				early = true
				count--
			case <-time.After(10 * time.Millisecond):
			}
			close(release)
			fixtureMust(t, <-operationDone)
			for i := 0; i < count; i++ {
				fixtureMust(t, <-closeDone)
			}
			if early || !isWouldBlock(lockErr) {
				t.Fatal("Close released exclusion during operation")
			}
			fixtureBytes(t, filepath.Join(r.StateDir, "receipts", "000000000001.json"), "bytes")
			if err := s.link(stage, fixtureTarget{fixtureReceipt, "000000000002.json"}); err == nil {
				t.Fatal("closed session/lock accepted")
			}
			fixtureMust(t, s.close())
			fixtureMust(t, s.close())
			for _, p := range s.parents {
				if _, err := p.file.Stat(); err == nil {
					t.Fatal("file handle leaked")
				}
				if _, err := p.root.Stat("."); err == nil {
					t.Fatal("root handle leaked")
				}
			}
			lock, err := AcquireLock(context.Background(), &s.repo, LockOptions{Wait: time.Second})
			fixtureMust(t, err)
			fixtureMust(t, lock.Close())
		})
	}
	t.Run("joined-close", func(t *testing.T) {
		s, _ := fixtureHarness(t)
		fileErr := errors.New("file close")
		rootErr := errors.New("root close")
		oldFile, oldRoot := s.closeFile, s.closeRoot
		s.closeFile = func(f *os.File) error { return errors.Join(oldFile(f), fileErr) }
		s.closeRoot = func(r *os.Root) error { return errors.Join(oldRoot(r), rootErr) }
		restoreHooks(t)
		flockRelease = func(int) error { return syscall.EIO }
		err := s.close()
		if !errors.Is(err, fileErr) || !errors.Is(err, rootErr) || !strings.Contains(err.Error(), "LOCK_UN") {
			t.Fatalf("lost close/release error: %v", err)
		}
		if s.close() != err {
			t.Fatal("close failure not sticky")
		}
		// Cleanup already verified the expected error explicitly.
		s.closeErr = nil
		for _, p := range s.parents {
			if _, err := p.file.Stat(); err == nil {
				t.Fatal("failed close leaked handle")
			}
		}
	})
}

func TestTMV0002_AS10_FixtureDestinationRoles(t *testing.T) {
	for _, role := range []fixtureRole{fixtureReceipt, fixtureEvidence, fixturePin, fixtureRequest, fixtureVersion, fixtureHead, fixtureBarrier, fixtureReservations, fixtureQueue, fixturePolicy, fixtureImportMap, fixtureTicket} {
		t.Run(fmt.Sprint(role), func(t *testing.T) {
			s, _ := fixtureHarness(t)
			raw := "bytes"
			if role == fixtureVersion {
				raw = "taskman-state/0\n"
			}
			stage := fixturePrepared(t, s, role, raw)
			names := map[fixtureRole]string{fixtureReceipt: "000000000001.json", fixtureEvidence: string(stage.digest), fixturePin: string(stage.digest) + ".json", fixtureRequest: strings.Repeat("0", 64) + ".json", fixtureVersion: "VERSION", fixtureHead: "head.json", fixtureBarrier: "barrier.json", fixtureReservations: "reservations.json", fixtureQueue: "queue.json", fixturePolicy: "policy.json", fixtureImportMap: "import-map.json", fixtureTicket: "AT-01.json"}
			target := fixtureTarget{role, names[role]}
			fixtureMust(t, s.link(stage, target))
			// A same-byte retry remains exclusive even for content-addressed targets.
			fixtureEffect(t, s.link(stage, target), false)
			key, name, _, err := target.location()
			fixtureMust(t, err)
			f, err := s.parents[key].root.Open(name)
			fixtureMust(t, err)
			b, err := io.ReadAll(f)
			fixtureMust(t, err)
			fixtureMust(t, f.Close())
			if string(b) != raw {
				t.Fatal("wrong destination bytes")
			}
		})
	}
	// Request-index files have the accepted J1 64 KiB bound, not the receipt bound.
	for _, slot := range []fixtureSlot{"a00", "a10"} {
		n, err := fixtureStageLimit(slot, fixtureRequest)
		fixtureMust(t, err)
		if n != wire.MaxAttemptRecordBytes {
			t.Fatal("request bound drift")
		}
	}
	s, _ := fixtureHarness(t)
	stage := fixturePrepared(t, s, fixturePin, "bytes")
	if err := s.link(stage, fixtureTarget{fixturePin, strings.Repeat("0", 64) + ".json"}); err == nil {
		t.Fatal("pin digest mismatch accepted")
	}
	other, _ := fixtureHarness(t)
	if err := other.link(stage, fixtureTarget{fixturePin, string(stage.digest) + ".json"}); err == nil {
		t.Fatal("foreign token accepted")
	}
	copied := *stage
	if err := s.link(&copied, fixtureTarget{fixturePin, string(stage.digest) + ".json"}); err == nil {
		t.Fatal("copied token accepted")
	}
}

func TestTMV0010_AS10_FixtureActualSyncFaultsAndAlreadyApplied(t *testing.T) {
	for _, kind := range []string{"file", "directory", "already-file", "already-dir", "absent-remove"} {
		t.Run(kind, func(t *testing.T) {
			s, r := fixtureHarness(t)
			restoreHooks(t)
			if kind == "file" {
				syncFile = func(*os.File) error { return syscall.ENOTSUP }
				got, err := s.prepare("a00", fixtureEvidence, []byte("bytes"))
				fixtureEffect(t, err, true)
				if got != nil || !errors.Is(err, syscall.ENOTSUP) {
					t.Fatal(err)
				}
				return
			}
			if kind == "directory" {
				syncDirectory = func(*os.File) error { return syscall.EIO }
				got, err := s.prepare("a00", fixtureEvidence, []byte("bytes"))
				fixtureEffect(t, err, true)
				if got != nil || !errors.Is(err, syscall.EIO) {
					t.Fatal(err)
				}
				return
			}
			if kind == "absent-remove" {
				syncDirectory = func(*os.File) error { return syscall.EIO }
				err := s.removeBarrier(fixtureTarget{fixtureBarrier, "barrier.json"}, wire.Sum([]byte("pre")))
				fixtureEffect(t, err, false)
				return
			}
			stage := fixturePrepared(t, s, fixtureHead, "post")
			path := filepath.Join(r.StateDir, "head.json")
			fixtureMust(t, os.WriteFile(path, []byte("post"), 0600))
			info, err := os.Lstat(path)
			fixtureMust(t, err)
			if kind == "already-file" {
				syncFile = func(*os.File) error { return syscall.ENOTSUP }
			} else {
				syncDirectory = func(*os.File) error { return syscall.EIO }
			}
			pre := wire.Sum([]byte("pre"))
			fixtureEffect(t, s.replace(stage, fixtureTarget{fixtureHead, "head.json"}, &pre), false)
			fixturePreserved(t, path, info, "post")
		})
	}
}

func TestTMV0010_AS10_FixtureAbsentHierarchyAndUnsupported(t *testing.T) {
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	fixtureMust(t, err)
	f, err := os.Open(r.Root)
	fixtureMust(t, err)
	_, observeErr := fixtureObserveMount(f)
	fixtureMust(t, f.Close())
	if observeErr != nil {
		t.Skipf("NOT_RUN: host unsupported: %v", observeErr)
	}
	lock, err := AcquireLock(context.Background(), repo, LockOptions{})
	fixtureMust(t, err)
	s, err := newFixtureSession(repo, lock)
	if err != nil {
		fixtureMust(t, lock.Close())
		t.Fatal(err)
	}
	t.Cleanup(func() { fixtureMust(t, s.close()) })
	for _, key := range []string{"state", "staging", "receipts", "evidence", "pinned", "requests", "requests/aa", "intent", "tickets"} {
		fixtureMust(t, s.mkdir(key))
	}
	// Inject platform refusal before a constructor can create/open owned handles.
	old := supportedPlatform
	supportedPlatform = false
	t.Cleanup(func() { supportedPlatform = old })
	before := fixture.TreeSnapshot(t, r.Root)
	got, err := newFixtureSession(repo, lock)
	if got != nil || !errors.Is(err, fixtureUnsupported) {
		t.Fatal("unsupported platform accepted")
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.Root)) {
		t.Fatal("unsupported constructor changed tree")
	}
}

func TestTMV0009_AS10_FixtureTransientHandlesAndCleanupRetention(t *testing.T) {
	for _, point := range []string{"write", "file-sync", "stage-close", "cleanup-unlink", "link", "sync:receipts", "check-close"} {
		t.Run(point, func(t *testing.T) {
			s, r := fixtureHarness(t)
			var stage *fixtureStage
			if point == "link" || point == "sync:receipts" {
				stage = fixturePrepared(t, s, fixtureReceipt, "bytes")
			}
			before, err := os.ReadDir("/dev/fd")
			fixtureMust(t, err)
			s.before = func(at string) error {
				if at == point || point == "cleanup-unlink" && at == "file-sync" {
					return syscall.EIO
				}
				return nil
			}
			if point == "check-close" {
				closeRoot := s.closeRoot
				s.closeRoot = func(root *os.Root) error { return errors.Join(closeRoot(root), syscall.EIO) }
				defer func() { s.closeRoot = closeRoot }()
			}
			if stage != nil {
				err = s.link(stage, fixtureTarget{fixtureReceipt, "000000000001.json"})
			} else {
				stage, err = s.prepare("a00", fixtureEvidence, []byte("bytes"))
				if stage != nil {
					t.Fatal("failed preparation token")
				}
			}
			if err == nil {
				t.Fatal("fault ignored")
			}
			after, err := os.ReadDir("/dev/fd")
			fixtureMust(t, err)
			if len(after) != len(before) {
				t.Fatalf("transient descriptor leak: before %d after %d", len(before), len(after))
			}
			if point == "cleanup-unlink" {
				fixtureBytes(t, filepath.Join(r.StateDir, "staging", "a00"), "bytes")
			}
		})
	}
	s, r := fixtureHarness(t)
	stage := fixturePrepared(t, s, fixtureEvidence, "bytes")
	path := filepath.Join(r.StateDir, "evidence", string(stage.digest))
	fixtureMust(t, s.link(stage, fixtureTarget{fixtureEvidence, string(stage.digest)}))
	info, err := os.Lstat(path)
	fixtureMust(t, err)
	fixtureMust(t, s.removeStage("a00"))
	fixtureAbsent(t, filepath.Join(r.StateDir, "staging", "a00"))
	fixturePreserved(t, path, info, "bytes")
}

func TestTMV0009_AS11_FixtureSyncBoundaryPreservesForeignNames(t *testing.T) {
	for _, op := range []string{"link", "rename", "remove", "cleanup", "already-post", "already-absent"} {
		t.Run(op, func(t *testing.T) {
			s, r := fixtureHarness(t)
			role := fixtureHead
			if op == "link" {
				role = fixtureReceipt
			}
			var stage *fixtureStage
			if op != "remove" && op != "already-absent" {
				stage = fixturePrepared(t, s, role, "post")
			}
			path := filepath.Join(r.StateDir, "head.json")
			point := "sync:state"
			switch op {
			case "link":
				path = filepath.Join(r.StateDir, "receipts", "000000000001.json")
				point = "sync:receipts"
			case "rename":
				fixtureMust(t, os.WriteFile(path, []byte("pre"), 0600))
			case "already-post":
				fixtureMust(t, os.WriteFile(path, []byte("post"), 0600))
			case "remove":
				path = filepath.Join(r.StateDir, "barrier.json")
				fixtureMust(t, os.WriteFile(path, []byte("pre"), 0600))
			case "already-absent":
				path = filepath.Join(r.StateDir, "barrier.json")
			case "cleanup":
				path = filepath.Join(r.StateDir, "staging", "a00")
				point = "sync:staging"
			}
			var foreign os.FileInfo
			fired := false
			s.before = func(at string) error {
				if at != point || fired {
					return nil
				}
				fired = true
				if _, err := os.Lstat(path); err == nil {
					fixtureMust(t, os.Rename(path, path+"-owned"))
				} else if !os.IsNotExist(err) {
					t.Fatal(err)
				}
				fixtureMust(t, os.WriteFile(path, []byte("foreign"), 0640))
				var err error
				foreign, err = os.Lstat(path)
				fixtureMust(t, err)
				return nil
			}
			pre := wire.Sum([]byte("pre"))
			var err error
			switch op {
			case "link":
				err = s.link(stage, fixtureTarget{fixtureReceipt, "000000000001.json"})
			case "rename", "already-post":
				err = s.replace(stage, fixtureTarget{fixtureHead, "head.json"}, &pre)
			case "remove", "already-absent":
				err = s.removeBarrier(fixtureTarget{fixtureBarrier, "barrier.json"}, pre)
			case "cleanup":
				err = s.removeStage("a00")
			}
			if !fired {
				t.Fatal("sync boundary not exercised")
			}
			fixtureEffect(t, err, !strings.HasPrefix(op, "already"))
			fixturePreserved(t, path, foreign, "foreign")
		})
	}
}
