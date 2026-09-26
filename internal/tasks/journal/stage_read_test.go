package journal

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
)

func TestTMV0006_AS35_EmptyTicketRequestLookup(t *testing.T) {
	for _, corruption := range []string{"missing", "corrupt"} {
		t.Run(corruption, func(t *testing.T) {
			repo, r := setup(t)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()}, "", true, true, false)
			path, _ := snapshot.RequestPath("R")
			appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes("R", 3, wire.Sum([]byte("R")), false)}, "R", true, true, false)
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets/A.json"), []byte{})
			before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			index := RequestIndex{Reader: r}
			for _, id := range []string{"R", "absent"} {
				_, found, err := index.Lookup(id)
				if err != nil || found != (id == "R") {
					t.Errorf("%s: found=%v err=%v", id, found, err)
				}
				if index.IntentProjectionAgreement != "NOT_OBSERVED" {
					t.Fatal("intent agreement claimed")
				}
			}
			_, err := r.Audit()
			requireCode(t, err, wire.CodeIntentDiverged)
			fixture.AssertUntouched(t, repo, before, intents, "empty D lookup")
			if corruption == "missing" {
				remove(t, filepath.Join(repo.StateDir, path))
			} else {
				fixture.Write(t, filepath.Join(repo.StateDir, path), []byte("{}\n"))
			}
			before, intents = fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			for _, id := range []string{"R", "absent"} {
				if _, ok, e := index.Lookup(id); e == nil || ok {
					t.Fatalf("corrupt index: %v %v", ok, e)
				}
			}
			fixture.AssertUntouched(t, repo, before, intents, "empty D refusal")
		})
	}
}

func stageDescriptor(t *testing.T, r *fixture.Repo, completed bool) []byte {
	t.Helper()
	headRaw, e := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
	if e != nil {
		t.Fatal(e)
	}
	h, e := snapshot.DecodeHead(headRaw)
	if e != nil {
		t.Fatal(e)
	}
	base := h.LastSeq.Uint64()
	baseHash := *h.LastReceiptSha256
	request := "next"
	requestHash := wire.Sum([]byte("next"))
	receiptRaw := []byte("receipt")
	requestRaw := []byte("request")
	if completed {
		base--
		name, _ := snapshot.ReceiptName(base)
		b, e := os.ReadFile(filepath.Join(r.StateDir, "receipts", name))
		if e != nil {
			t.Fatal(e)
		}
		baseHash = wire.Sum(b)
		name, _ = snapshot.ReceiptName(base + 1)
		receiptRaw, e = os.ReadFile(filepath.Join(r.StateDir, "receipts", name))
		if e != nil {
			t.Fatal(e)
		}
		rc, e := snapshot.DecodeReceipt(receiptRaw)
		if e != nil {
			t.Fatal(e)
		}
		request = *rc.RequestID
		p, _ := snapshot.RequestPath(request)
		requestRaw, e = os.ReadFile(filepath.Join(r.StateDir, p))
		if e != nil {
			t.Fatal(e)
		}
		req, e := snapshot.DecodeRequest(requestRaw)
		if e != nil {
			t.Fatal(e)
		}
		requestHash = req.Entry.MutationSha256
	}
	name, _ := snapshot.ReceiptName(base + 1)
	rp, _ := snapshot.RequestPath(request)
	d := snapshot.StageDescriptor{QueueID: fixture.QueueID, Operation: snapshot.StageUnpause, RequestID: request, RequestSha256: requestHash, RecordedAt: fixture.Timestamp, Base: &snapshot.StageBase{LastSeq: wire.SizeOf(base), LastReceiptSha256: baseHash}, Artifacts: []snapshot.StageDescription{
		{Slot: "a00", Role: "HEAD", Target: "head.json", Sha256: wire.Sum(headRaw), Bytes: wire.SizeOf(uint64(len(headRaw)))},
		{Slot: "a01", Role: "POST", Target: rp, Sha256: wire.Sum(requestRaw), Bytes: wire.SizeOf(uint64(len(requestRaw)))},
		{Slot: "a02", Role: "RECEIPT", Target: "receipts/" + name, Sha256: wire.Sum(receiptRaw), Bytes: wire.SizeOf(uint64(len(receiptRaw)))},
	}}
	raw, e := d.Encode()
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

func TestTMV0008_AS07_JournalStageReadBindings(t *testing.T) {
	for _, mode := range []string{"empty", "temp", "precommit", "completed", "stale", "foreign", "request", "temp-N-1", "temp-Nsame", "temp-Ndifferent", "temp-N+1", "empty-slot"} {
		t.Run(mode, func(t *testing.T) {
			repo, r := setup(t)
			path, _ := snapshot.RequestPath("R")
			appendReceipt(t, repo, "UNPAUSE", map[string][]byte{path: requestBytes("R", 2, wire.Sum([]byte("R")), false)}, "R", true, true, false)
			dir := filepath.Join(repo.StateDir, "staging")
			if e := os.Mkdir(dir, 0700); e != nil {
				t.Fatal(e)
			}
			want := ""
			switch mode {
			case "empty":
			case "temp":
				fixture.Write(t, filepath.Join(dir, "active.json.tmp"), []byte("arbitrary"))
			default:
				raw := stageDescriptor(t, repo, mode == "completed" || mode == "request")
				d, e := snapshot.DecodeStageDescriptor(raw)
				if e != nil {
					t.Fatal(e)
				}
				if mode == "stale" {
					d.Base.LastReceiptSha256 = wire.Sum([]byte("stale"))
					want = wire.CodeJournalForked
				}
				if mode == "foreign" {
					d.QueueID = "queue:other:q"
					want = wire.CodeJournalForked
				}
				if mode == "request" {
					d.RequestSha256 = wire.Sum([]byte("foreign request"))
					want = wire.CodeJournalForked
				}
				raw, e = d.Encode()
				if e != nil {
					t.Fatal(e)
				}
				fixture.Write(t, filepath.Join(dir, "active.json"), raw)
				switch mode {
				case "temp-N-1":
					fixture.Write(t, filepath.Join(dir, "active.json.tmp"), raw[:len(raw)-1])
				case "temp-Nsame":
					fixture.Write(t, filepath.Join(dir, "active.json.tmp"), raw)
				case "temp-Ndifferent":
					fixture.Write(t, filepath.Join(dir, "active.json.tmp"), make([]byte, len(raw)))
					want = wire.CodeJournalForked
				case "temp-N+1":
					fixture.Write(t, filepath.Join(dir, "active.json.tmp"), make([]byte, len(raw)+1))
					want = wire.CodeLimitExceeded
				case "empty-slot":
					fixture.Write(t, filepath.Join(dir, "a00"), nil)
				}
			}
			before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			res, e := r.Audit()
			requireCode(t, e, want)
			if e == nil {
				if res.StagingPresent != (mode != "empty") || res.ActorAuthentication != "NOT_OBSERVED" {
					t.Fatalf("stage claims %+v", res)
				}
			}
			fixture.AssertUntouched(t, repo, before, intents, "journal staging")
		})
	}
}

func swapDirectory(t *testing.T, path string) {
	t.Helper()
	old := path + "-detached"
	if e := os.Rename(path, old); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(old)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range entries {
		if e = os.Rename(filepath.Join(old, f.Name()), filepath.Join(path, f.Name())); e != nil {
			t.Fatal(e)
		}
	}
	if e = os.Remove(old); e != nil {
		t.Fatal(e)
	}
}

func TestTMV0008_AS36_JournalStageMovementFourAttempts(t *testing.T) {
	for _, kind := range []string{"stage", "orphan", "empty-stage", "root", "intent-root", "validation"} {
		for _, persistent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", kind, persistent), func(t *testing.T) {
				repo, r := setup(t)
				dir := filepath.Join(repo.StateDir, "staging")
				if e := os.Mkdir(dir, 0700); e != nil {
					t.Fatal(e)
				}
				tmp := filepath.Join(dir, "active.json.tmp")
				if kind == "stage" {
					fixture.Write(t, tmp, []byte("a"))
				}
				active := filepath.Join(dir, "active.json")
				if kind == "validation" {
					fixture.Write(t, active, []byte("{"))
				}
				calls := 0
				r.afterCapture = func() {
					calls++
					if !persistent && calls > 1 {
						return
					}
					switch kind {
					case "stage":
						info, e := os.Stat(tmp)
						if e != nil {
							t.Fatal(e)
						}
						fixture.Write(t, tmp, []byte{byte('a' + calls)})
						if e = os.Chtimes(tmp, info.ModTime(), info.ModTime()); e != nil {
							t.Fatal(e)
						}
					case "orphan":
						raw := []byte(fmt.Sprint(calls))
						fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum(raw))), raw)
					case "validation":
						if persistent {
							fixture.Write(t, active, bytes.Repeat([]byte("{"), calls+1))
						} else {
							fixture.Write(t, active, stageDescriptor(t, repo, false))
						}
					default:
						path := dir
						if kind == "root" {
							path = repo.StateDir
						}
						if kind == "intent-root" {
							path = repo.IntentDir
						}
						swapDirectory(t, path)
					}
				}
				res, e := r.Audit()
				if persistent {
					if wire.CodeOf(e) != wire.CodeSnapshotMoved || calls != 4 || res != nil {
						t.Fatalf("calls %d res %v err %v", calls, res != nil, e)
					}
				} else if e != nil || calls != 2 {
					t.Fatalf("calls %d err %v", calls, e)
				}
			})
		}
	}
}

type missingDirectoryInfo struct{ Source }

func (s missingDirectoryInfo) List(p string, n int) (Listing, error) {
	v, e := s.Source.List(p, n)
	v.DirectoryInfo = nil
	return v, e
}

func TestTMV0008_AS36_NativeDirectoryObservationLifetime(t *testing.T) {
	t.Run("missing metadata", func(t *testing.T) {
		_, r := setup(t)
		r.Source = missingDirectoryInfo{r.Source}
		_, e := r.Audit()
		requireCode(t, e, wire.CodeMalformed)
	})
	for _, kind := range []string{"empty-root", "empty-parent", "listed-parent", "FIFO", "disappeared", "initially-absent", "IO"} {
		t.Run(kind, func(t *testing.T) {
			repo, r := setup(t)
			s := newNativeRead(r.Source.(Native))
			t.Cleanup(func() {
				if e := s.close(); e != nil {
					t.Error(e)
				}
			})
			if kind == "initially-absent" {
				remove(t, filepath.Join(repo.StateDir, "head.json"))
				dir := filepath.Join(repo.StateDir, "absent")
				s.native.StateDir = dir
				_, e := s.List(".", 20)
				if !os.IsNotExist(e) {
					t.Fatal(e)
				}
				return
			}
			if kind == "empty-root" {
				s.native.StateDir = fixture.TempDirOutside(t)
				if _, e := s.List(".", 0); e != nil {
					t.Fatal(e)
				}
				swapDirectory(t, s.native.StateDir)
				_, e := s.List(".", 0)
				requireCode(t, e, wire.CodeSnapshotMoved)
				return
			}
			dir := filepath.Join(repo.StateDir, "staging")
			if e := os.Mkdir(dir, 0700); e != nil {
				t.Fatal(e)
			}
			if kind == "empty-parent" {
				if _, e := s.List("staging", 0); e != nil {
					t.Fatal(e)
				}
				swapDirectory(t, dir)
				_, e := s.List("staging", 0)
				requireCode(t, e, wire.CodeSnapshotMoved)
				return
			}
			p := filepath.Join(dir, "active.json.tmp")
			fixture.Write(t, p, []byte("foreign bytes"))
			if kind == "disappeared" {
				old := afterReadNames
				t.Cleanup(func() { afterReadNames = old })
				afterReadNames = func(path string) {
					if path == "staging" {
						remove(t, p)
					}
				}
				_, e := s.List("staging", 13)
				requireCode(t, e, wire.CodeSnapshotMoved)
				return
			}
			if _, e := s.List("staging", 13); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "listed-parent":
				if e := os.Rename(dir, dir+"-old"); e != nil {
					t.Fatal(e)
				}
				fixture.Write(t, p, []byte("new foreign bytes"))
				raw, e := s.Read("staging/active.json.tmp", 100)
				if e != nil || string(raw) != "foreign bytes" {
					t.Fatalf("read redirected: %q %v", raw, e)
				}
				_, e = s.List("staging", 13)
				requireCode(t, e, wire.CodeSnapshotMoved)
				raw, e = os.ReadFile(p)
				if e != nil || string(raw) != "new foreign bytes" {
					t.Fatal("foreign bytes changed")
				}
			case "FIFO":
				remove(t, p)
				if e := syscall.Mkfifo(p, 0600); e != nil {
					t.Fatal(e)
				}
				_, e := s.Read("staging/active.json.tmp", 100)
				requireCode(t, e, wire.CodeUnsupportedFilesystem)
			case "IO":
				parent := s.roots["staging"]
				if e := parent.Close(); e != nil {
					t.Fatal(e)
				}
				delete(s.roots, "staging")
				s.roots["staging"] = parent
				_, e := s.Read("staging/active.json.tmp", 100)
				if e == nil || wire.CodeOf(e) == wire.CodeSnapshotMoved {
					t.Fatal(e)
				}
				delete(s.roots, "staging")
			}
		})
	}
}

func TestTMV0008_AS07_JournalReadHandlesCloseOnEveryExit(t *testing.T) {
	for _, mode := range []string{"success", "body-error", "movement", "close-error"} {
		t.Run(mode, func(t *testing.T) {
			repo, r := setup(t)
			before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			old := closeReadRoot
			var roots []*os.Root
			t.Cleanup(func() { closeReadRoot = old })
			closeReadRoot = func(root *os.Root) error {
				roots = append(roots, root)
				e := old(root)
				if mode == "close-error" {
					return errors.Join(e, syscall.EIO)
				}
				return e
			}
			if mode == "body-error" {
				r.afterCapture = func() { fixture.Write(t, filepath.Join(repo.StateDir, "receipts/000000000001.json"), []byte("{\n")) }
			}
			if mode == "movement" {
				r.afterCapture = func() { swapDirectory(t, repo.StateDir) }
			}
			_, e := r.Audit()
			if mode == "success" && e != nil {
				t.Fatal(e)
			}
			if mode != "success" && e == nil {
				t.Fatal("error lost")
			}
			if mode == "close-error" && !strings.Contains(e.Error(), syscall.EIO.Error()) {
				t.Fatal(e)
			}
			if len(roots) == 0 {
				t.Fatal("no closed roots")
			}
			for _, root := range roots {
				if _, e := root.Stat("."); e == nil {
					t.Fatal("root left open")
				}
			}
			if mode == "success" || mode == "close-error" {
				fixture.AssertUntouched(t, repo, before, intents, "read lifetime closure")
			}
		})
	}
}

func TestTMV0002_AS10_JournalStageScanBounds(t *testing.T) {
	repo, r := setup(t)
	o, e := r.capture(profileLimits)
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for p := range o.files {
		if p != "." && p != "intent" && !strings.HasPrefix(p, "intent/") {
			n++
		}
	}
	dir := filepath.Join(repo.StateDir, "staging")
	fixture.Write(t, filepath.Join(dir, "active.json.tmp"), []byte("x"))
	_, e = r.audit(nil, "", limits{scan: n + 2, selected: wire.MaxIntentTreeBytes}, true)
	if e != nil {
		t.Fatal(e)
	}
	_, e = r.audit(nil, "", limits{scan: n + 1, selected: wire.MaxIntentTreeBytes}, true)
	requireCode(t, e, wire.CodeLimitExceeded)
	for i := 0; i < 14; i++ {
		fixture.Write(t, filepath.Join(dir, fmt.Sprintf("f%d", i)), nil)
	}
	old := afterReadNames
	t.Cleanup(func() { afterReadNames = old })
	afterReadNames = func(p string) {
		if p == "staging" {
			t.Fatal("inspected flooded stage")
		}
	}
	_, e = r.Audit()
	requireCode(t, e, wire.CodeLimitExceeded)
}

func TestTMV0009_AS11_JournalStageGenesisAndNext(t *testing.T) {
	for _, mode := range []string{"empty-no-head", "stage-no-head", "genesis", "next", "fork"} {
		t.Run(mode, func(t *testing.T) {
			repo, r := setup(t)
			if mode == "empty-no-head" {
				repo = fixture.TempRepo(t)
				r.Source = Native{StateDir: repo.StateDir, PrimaryWorktree: repo.Root}
				r.PrimaryWorktree = repo.Root
				if e := os.MkdirAll(repo.IntentDir, 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.MkdirAll(repo.StateDir, 0700); e != nil {
					t.Fatal(e)
				}
			}

			want := wire.CodeUninitialized
			dir := filepath.Join(repo.StateDir, "staging")
			if e := os.Mkdir(dir, 0700); e != nil {
				t.Fatal(e)
			}
			if mode != "empty-no-head" {
				fixture.Write(t, filepath.Join(dir, "active.json.tmp"), []byte("{"))
			}
			switch mode {
			case "stage-no-head":
				remove(t, filepath.Join(repo.StateDir, "head.json"))
				remove(t, filepath.Join(repo.StateDir, "receipts/000000000001.json"))
			case "genesis":
				remove(t, filepath.Join(repo.StateDir, "head.json"))
				want = wire.CodeRedoPending
			case "next", "fork":
				appendReceipt(t, repo, "MUTATION", nil, "", false, false, false)
				want = wire.CodeRedoPending
				if mode == "fork" {
					fixture.Write(t, filepath.Join(repo.StateDir, "receipts/000000000003.json"), []byte("{}\n"))
					want = wire.CodeJournalForked
				}
			}
			before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			res, e := r.Audit()
			requireCode(t, e, want)
			if mode == "empty-no-head" && res.StagingPresent {
				t.Fatal("empty stage active")
			}
			fixture.AssertUntouched(t, repo, before, intents, "genesis and next staging")
		})
	}
}

type stageReadFailure struct {
	Source
	boom   error
	reads  int
	change func()
}

func (s *stageReadFailure) Read(p string, n int) ([]byte, error) {
	if p == "receipts/000000000001.json" {
		s.reads++
		if s.reads == 1 {
			if s.change != nil {
				s.change()
			}
			return nil, s.boom
		}
	}
	return s.Source.Read(p, n)
}

type afterCaptureFailure struct {
	Source
	lists int
}

func (s *afterCaptureFailure) List(p string, n int) (Listing, error) {
	if p == "." {
		s.lists++
		if s.lists == 2 {
			return Listing{}, syscall.EIO
		}
	}
	return s.Source.List(p, n)
}
func TestTMV0008_AS36_JournalStageReadErrorRecheck(t *testing.T) {
	for _, moving := range []bool{false, true} {
		t.Run(fmt.Sprint(moving), func(t *testing.T) {
			repo, r := setup(t)
			tmp := filepath.Join(repo.StateDir, "staging/active.json.tmp")
			fixture.Write(t, tmp, []byte("a"))
			boom := errors.New("actual read error")
			source := &stageReadFailure{Source: r.Source, boom: boom}
			if moving {
				source.change = func() { fixture.Write(t, tmp, []byte("bb")) }
			}
			r.Source = source
			res, e := r.Audit()
			if moving {
				if e != nil || source.reads != 2 || res == nil {
					t.Fatalf("read-error movement: calls=%d err=%v", source.reads, e)
				}
			} else if e != boom || source.reads != 1 {
				t.Fatalf("stable read failure %v", e)
			}
		})
	}
	t.Run("after capture IO", func(t *testing.T) {
		repo, r := setup(t)
		s := &afterCaptureFailure{Source: r.Source}
		r.Source = s
		r.afterCapture = func() { fixture.Write(t, filepath.Join(repo.StateDir, "staging/active.json.tmp"), []byte("new")) }
		_, e := r.Audit()
		if e != syscall.EIO || s.lists != 2 {
			t.Fatalf("unrelated after IO retried: %d %v", s.lists, e)
		}
	})
}

func TestTMV0006_AS35_EmptyTicketStrictReadBoundary(t *testing.T) {
	for _, path := range []string{"intent/tickets/A.json", "intent/queue.json", "intent/policy.json"} {
		t.Run(path, func(t *testing.T) {
			repo, r := setup(t)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()}, "", true, true, false)
			physical := filepath.Join(repo.IntentDir, strings.TrimPrefix(path, "intent/"))
			fixture.Write(t, physical, nil)
			if path == "intent/tickets/A.json" {
				boom := errors.New("ticket bytes unavailable")
				r.Source = failingSource{Source: r.Source, path: path, err: boom}
				_, _, e := (&RequestIndex{Reader: r}).Lookup("absent")
				if e != boom {
					t.Fatalf("read error ignored: %v", e)
				}
			} else {
				_, _, e := (&RequestIndex{Reader: r}).Lookup("absent")
				requireCode(t, e, wire.CodeMalformed)
			}
		})
	}
}

func TestTMV0008_AS07_JournalStageFixtureRestriction(t *testing.T) {
	repo, r := setup(t)
	q, e := intent.DecodeQueue(read(t, filepath.Join(repo.IntentDir, "queue.json")))
	if e != nil {
		t.Fatal(e)
	}
	q.Fixture = false
	appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/queue.json": wire.EncodeFile(q.Value())}, "", true, true, false)
	dir := filepath.Join(repo.StateDir, "staging")
	if e = os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e = r.Audit(); e != nil {
		t.Fatal(e)
	}
	fixture.Write(t, filepath.Join(dir, "active.json.tmp"), nil)
	_, e = r.Audit()
	requireCode(t, e, wire.CodeUnsupported)
}

func TestTMV0008_AS07_JournalFileClosureErrorsSurviveMovement(t *testing.T) {
	repo, r := setup(t)
	tmp := filepath.Join(repo.StateDir, "staging/active.json.tmp")
	fixture.Write(t, tmp, []byte("a"))
	old := closeReadFile
	var files []*os.File
	t.Cleanup(func() { closeReadFile = old })
	closeReadFile = func(f *os.File) error {
		files = append(files, f)
		e := old(f)
		if strings.HasSuffix(f.Name(), "000000000001.json") {
			fixture.Write(t, tmp, []byte("changed"))
			return errors.Join(e, syscall.EBUSY)
		}
		return e
	}
	res, e := r.Audit()
	if res != nil || e == nil || !strings.Contains(e.Error(), syscall.EBUSY.Error()) {
		t.Fatalf("close lost: %v", e)
	}
	if len(files) == 0 {
		t.Fatal("no files closed")
	}
	for _, f := range files {
		if _, e := f.Stat(); e == nil {
			t.Fatal("file left open")
		}
	}
}

func TestTMV0002_AS10_JournalStageEmptyRaw(t *testing.T) {
	repo, r := setup(t)
	raw := stageDescriptor(t, repo, false)
	d, e := snapshot.DecodeStageDescriptor(raw)
	if e != nil {
		t.Fatal(e)
	}
	d.Operation = snapshot.StageKeepJournal
	d.Artifacts = append(d.Artifacts, snapshot.StageDescription{Role: "POST", Target: "evidence/" + string(wire.Sum(nil)), Sha256: wire.Sum(nil), Bytes: "0"}, snapshot.StageDescription{Role: "POST", Target: "intent/tickets/A.json", Sha256: wire.Sum([]byte("ticket")), Bytes: "6"})
	sort.Slice(d.Artifacts, func(i, j int) bool {
		a, b := d.Artifacts[i], d.Artifacts[j]
		return a.Role < b.Role || a.Role == b.Role && a.Target < b.Target
	})
	emptySlot := ""
	for i := range d.Artifacts {
		a := &d.Artifacts[i]
		a.Slot = fmt.Sprintf("a%02d", i)
		if a.Bytes == "0" {
			emptySlot = a.Slot
		}
	}
	raw, e = d.Encode()
	if e != nil {
		t.Fatal(e)
	}
	fixture.Write(t, filepath.Join(repo.StateDir, "staging/active.json"), raw)
	fixture.Write(t, filepath.Join(repo.StateDir, "staging", emptySlot), nil)
	before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
	res, e := r.Audit()
	if e != nil || !res.StagingPresent {
		t.Fatal(e)
	}
	fixture.AssertUntouched(t, repo, before, intents, "declared empty raw stage bytes")
}

func TestTMV0008_AS07_JournalCompletedStageLabels(t *testing.T) {
	for _, mode := range []string{"wrong-label", "timestamp"} {
		t.Run(mode, func(t *testing.T) {
			repo, r := setup(t)
			barrier := wire.EncodeFile(object("profile", str(snapshot.ProfileBarrier), "queueId", str(fixture.QueueID), "scope", str("ADMISSION"), "reason", str("OPERATOR"), "actor", str(fixture.Actor), "sinceSeq", str("2"), "since", str(fixture.Timestamp)))
			rp, _ := snapshot.RequestPath("pause")
			appendReceipt(t, repo, "PAUSE", map[string][]byte{"barrier.json": barrier, rp: requestBytes("pause", 2, wire.Sum([]byte("pause")), false)}, "pause", true, true, false)
			if mode == "timestamp" {
				rp, _ = snapshot.RequestPath("unpause")
				appendReceipt(t, repo, "UNPAUSE", map[string][]byte{"barrier.json": nil, rp: requestBytes("unpause", 3, wire.Sum([]byte("unpause")), false)}, "unpause", true, true, false)
			}
			if _, e := r.Audit(); e != nil {
				t.Fatal("invalid underlying fixture", e)
			}
			raw := stageDescriptor(t, repo, true)
			if mode == "timestamp" {
				d, e := snapshot.DecodeStageDescriptor(raw)
				if e != nil {
					t.Fatal(e)
				}
				d.RecordedAt = "2026-09-07T12:00:00Z"
				raw, e = d.Encode()
				if e != nil {
					t.Fatal(e)
				}
			}
			fixture.Write(t, filepath.Join(repo.StateDir, "staging/active.json"), raw)
			before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			_, e := r.Audit()
			requireCode(t, e, wire.CodeJournalForked)
			fixture.AssertUntouched(t, repo, before, intents, "completed-stage labels")
		})
	}
}

func TestTMV0008_AS07_JournalCompletedStageRequestBlob(t *testing.T) {
	repo, r := setup(t)
	path, _ := snapshot.RequestPath("R")
	raw := requestBytes("R", 2, wire.Sum([]byte("R")), false)
	appendReceipt(t, repo, "UNPAUSE", map[string][]byte{path: raw}, "R", true, true, true)
	fixture.Write(t, filepath.Join(repo.StateDir, "staging/active.json"), stageDescriptor(t, repo, true))
	for _, missing := range []bool{false, true} {
		if missing {
			remove(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum(raw))))
		}
		before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
		_, e := r.Audit()
		want := ""
		if missing {
			want = wire.CodeJournalForked
		}
		requireCode(t, e, want)
		fixture.AssertUntouched(t, repo, before, intents, "completed request blob")
	}
}

func TestTMV0009_AS11_StageLinkedGenesisBeforeIntentProjection(t *testing.T) {
	for _, mode := range []string{"temp", "active", "missing-genesis-blob"} {
		t.Run(mode, func(t *testing.T) {
			repo, r := setup(t)
			if mode == "temp" {
				fixture.Write(t, filepath.Join(repo.StateDir, "staging/active.json.tmp"), []byte("interrupted preparation"))
			} else {
				prepareIndexedInitStage(t, repo, r)
				path, _ := snapshot.RequestPath("init")
				remove(t, filepath.Join(repo.StateDir, path))
			}
			remove(t, filepath.Join(repo.StateDir, "head.json"))
			remove(t, filepath.Join(repo.IntentDir, "queue.json"))
			remove(t, filepath.Join(repo.IntentDir, "policy.json"))
			want := wire.CodeRedoPending
			if mode == "missing-genesis-blob" {
				remove(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum([]byte(snapshot.VersionBytes)))))
				want = wire.CodeJournalForked
			}
			before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			res, e := r.Audit()
			requireCode(t, e, want)
			if want == wire.CodeRedoPending && (res == nil || !res.Pending) {
				t.Fatal("validated linked genesis lost")
			}
			fixture.AssertUntouched(t, repo, before, intents, "genesis before intent projection")
		})
	}
}

func prepareIndexedInitStage(t *testing.T, repo *fixture.Repo, r Reader) *snapshot.Receipt {
	t.Helper()
	path, _ := snapshot.RequestPath("init")
	request := requestBytes("init", 1, wire.Sum([]byte("INIT request")), false)
	rewrite(t, repo, 1, func(v wire.Value) { v.Obj.Set("requestId", str("init")); addPost(v, path, request) })
	fixture.Write(t, filepath.Join(repo.StateDir, path), request)
	d := snapshot.StageDescriptor{QueueID: fixture.QueueID, Operation: snapshot.StageInit, RequestID: "init", RequestSha256: wire.Sum([]byte("INIT request")), RecordedAt: fixture.Timestamp}
	add := func(role, path string, raw []byte) {
		d.Artifacts = append(d.Artifacts, snapshot.StageDescription{Role: role, Target: path, Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))})
	}
	receipt := read(t, filepath.Join(repo.StateDir, "receipts/000000000001.json"))
	rc, e := snapshot.DecodeReceipt(receipt)
	if e != nil {
		t.Fatal(e)
	}
	add("HEAD", "head.json", read(t, filepath.Join(repo.StateDir, "head.json")))
	add("RECEIPT", "receipts/000000000001.json", receipt)
	for _, p := range rc.Post {
		raw, e := r.postBytes(p)
		if e != nil {
			t.Fatal(e)
		}
		add("POST", p.Path, raw)
	}
	sort.Slice(d.Artifacts, func(i, j int) bool {
		a, b := d.Artifacts[i], d.Artifacts[j]
		return a.Role < b.Role || a.Role == b.Role && a.Target < b.Target
	})
	for i := range d.Artifacts {
		d.Artifacts[i].Slot = fmt.Sprintf("a%02d", i)
	}
	raw, e := d.Encode()
	if e != nil {
		t.Fatal(e)
	}
	fixture.Write(t, filepath.Join(repo.StateDir, "staging/active.json"), raw)
	return rc
}

func TestTMV0009_AS11_StageReceiptOnlyGenesisValidation(t *testing.T) {
	for _, mode := range []string{"receipt-only", "partial", "missing-blob", "corrupt-blob", "invalid-queue", "wrong-queue", "nonfixture", "nonfixture+missing-blob", "invalid-genesis", "foreign-stage", "physical-invalid", "read-error", "late-read-error", "extra", "gap", "no-receipt"} {
		t.Run(mode, func(t *testing.T) {
			repo, r := setup(t)
			rc := prepareIndexedInitStage(t, repo, r)
			// Keep only linked receipt, required immutable evidence and transient stage.
			// No VERSION, queue, policy, reservations, request or pin projection remains.
			for _, p := range rc.Post {
				physical := filepath.Join(repo.StateDir, p.Path)
				if strings.HasPrefix(p.Path, "intent/") {
					physical = filepath.Join(repo.IntentDir, strings.TrimPrefix(p.Path, "intent/"))
				}
				remove(t, physical)
			}
			remove(t, filepath.Join(repo.StateDir, "head.json"))
			receiptPath := filepath.Join(repo.StateDir, "receipts/000000000001.json")
			blob := filepath.Join(repo.StateDir, "evidence", string(wire.Sum([]byte(snapshot.VersionBytes))))
			want := wire.CodeRedoPending
			switch mode {
			case "partial":
				fixture.Write(t, filepath.Join(repo.IntentDir, "queue.json"), fixture.QueueBytes())
			case "missing-blob":
				remove(t, blob)
				want = wire.CodeJournalForked
			case "corrupt-blob":
				fixture.Write(t, blob, []byte("bad"))
				want = wire.CodeJournalForked
			case "invalid-queue", "wrong-queue", "nonfixture", "nonfixture+missing-blob":
				v := value(t, receiptPath)
				posts, _ := v.Obj.Get("post")
				for _, p := range posts.Arr {
					path, _ := p.Obj.Get("path")
					if path.Str != "intent/queue.json" {
						continue
					}
					q, _ := p.Obj.Get("record")
					switch mode {
					case "invalid-queue":
						q.Obj.Set("extra", str("bad"))
						want = wire.CodeMalformed
					case "wrong-queue":
						q.Obj.Set("queueId", str("queue:acme:other"))
						want = wire.CodeJournalForked
					default:
						q.Obj.Set("fixture", wire.Bool(false))
						want = wire.CodeUnsupported
					}
					p.Obj.Set("sha256", str(string(wire.Sum(wire.EncodeFile(q)))))
				}
				fixture.Write(t, receiptPath, wire.EncodeFile(v))
				if mode == "nonfixture+missing-blob" {
					remove(t, blob)
					want = wire.CodeJournalForked
				}
			case "invalid-genesis":
				v := value(t, receiptPath)
				for _, field := range []string{"pre", "post"} {
					entries, _ := v.Obj.Get(field)
					var kept []wire.Value
					for _, p := range entries.Arr {
						path, _ := p.Obj.Get("path")
						if path.Str != "reservations.json" {
							kept = append(kept, p)
						}
					}
					v.Obj.Set(field, wire.Array(kept...))
				}
				fixture.Write(t, receiptPath, wire.EncodeFile(v))
				want = wire.CodeJournalForked
			case "foreign-stage":
				p := filepath.Join(repo.StateDir, "staging/active.json")
				d, e := snapshot.DecodeStageDescriptor(read(t, p))
				if e != nil {
					t.Fatal(e)
				}
				d.QueueID = "queue:acme:other"
				raw, e := d.Encode()
				if e != nil {
					t.Fatal(e)
				}
				fixture.Write(t, p, raw)
				want = wire.CodeJournalForked
			case "physical-invalid":
				fixture.Write(t, filepath.Join(repo.IntentDir, "queue.json"), []byte("{}\n"))
				want = wire.CodeIntentDiverged
			case "read-error":
				r.Source = failingSource{Source: r.Source, path: "intent/queue.json", err: syscall.EIO}
				want = wire.CodeMalformed
			case "late-read-error":
				r.Source = &lateGenesisQueueError{Source: r.Source}
				want = wire.CodeMalformed
			case "extra", "gap":
				fixture.Write(t, filepath.Join(repo.StateDir, "receipts/000000000002.json"), read(t, receiptPath))
				if mode == "gap" {
					remove(t, receiptPath)
				}
				want = wire.CodeJournalForked
			case "no-receipt":
				remove(t, receiptPath)
				want = wire.CodeUninitialized
			}
			before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			res, e := r.Audit()
			requireCode(t, e, want)
			if (mode == "read-error" || mode == "late-read-error") && e != syscall.EIO {
				t.Fatalf("read error replaced: %v", e)
			}
			if want == wire.CodeRedoPending && (res == nil || !res.Pending || res.StructuralConsistency != "CONSISTENT" || res.ProjectionAgreement != "PRE_OR_POST") {
				t.Fatalf("genesis not fully proved: %+v", res)
			}
			fixture.AssertUntouched(t, repo, before, intents, "receipt-only genesis")
		})
	}
}

type lateGenesisQueueError struct {
	Source
	reads int
}

func (s *lateGenesisQueueError) Read(p string, n int) ([]byte, error) {
	if p == "intent/queue.json" {
		s.reads++
		if s.reads == 2 {
			return nil, syscall.EIO
		}
	}
	return s.Source.Read(p, n)
}
