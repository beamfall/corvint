//go:build darwin || linux

package archive

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

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

func TestTMV0022_AS07_StageReadStableBindingsAndPurity(t *testing.T) {
	for _, mode := range []string{"empty", "temp", "precommit", "completed", "stale", "foreign", "request", "missing-proof", "temp-N-1", "temp-Nsame", "temp-Ndifferent", "temp-N+1"} {
		t.Run(mode, func(t *testing.T) {
			r, repo := repoWithStore(t)
			dir := filepath.Join(r.StateDir, "staging")
			if e := os.Mkdir(dir, 0700); e != nil {
				t.Fatal(e)
			}
			if mode == "completed" || mode == "request" || mode == "missing-proof" {
				setReceiptKind(t, r, "UNPAUSE")
			}

			want := ""
			switch mode {
			case "empty":
			case "temp":
				fixture.Write(t, filepath.Join(dir, "active.json.tmp"), []byte("broken unpublished bytes"))
			default:
				raw := stageDescriptor(t, r, mode == "completed" || mode == "request" || mode == "missing-proof")
				d, e := snapshot.DecodeStageDescriptor(raw)
				if e != nil {
					t.Fatal(e)
				}
				switch mode {
				case "stale":
					d.Base.LastReceiptSha256 = wire.Sum([]byte("stale"))
					want = wire.CodeJournalForked
				case "foreign":
					d.QueueID = "queue:other:q"
					want = wire.CodeJournalForked
				case "request":
					d.RequestSha256 = wire.Sum([]byte("other request"))
					want = wire.CodeJournalForked
				case "missing-proof":
					// Keep actual outer receipt/head proof intact while request binding names
					// another absent afterimage. The strict descriptor remains well formed.
					d.RequestID = "absent"
					d.Artifacts[1].Target, _ = snapshot.RequestPath("absent")
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
				}
			}
			before, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			out, result, e := export(t, repo, fixture.TempDirOutside(t))
			if code(e) != want {
				t.Fatalf("%v want %s", e, want)
			}
			if e != nil {
				if len(out) != 0 || result != nil {
					t.Fatal("pre-delivery output")
				}
			} else {
				if _, e = Verify(bytes.NewReader(out)); e != nil {
					t.Fatal(e)
				}
				for _, f := range result.Manifest.Files {
					if strings.HasPrefix(f.Path, "staging/") {
						t.Fatal("staged payload exported")
					}
				}
				if len(result.Manifest.Files) != 16 {
					t.Fatalf("orphan/payload membership %d", len(result.Manifest.Files))
				}
			}
			fixture.AssertUntouched(t, r, before, intents, "stage read")
		})
	}
}

func TestTMV0022_AS36_StageAndOrphanMovementFourAttempts(t *testing.T) {
	for _, kind := range []string{"stage", "orphan", "empty-stage", "root", "intent-root"} {
		for _, persistent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", kind, persistent), func(t *testing.T) {
				r, repo := repoWithStore(t)
				dir := filepath.Join(r.StateDir, "staging")
				if e := os.Mkdir(dir, 0700); e != nil {
					t.Fatal(e)
				}
				tmp := filepath.Join(dir, "active.json.tmp")
				if kind == "stage" {
					fixture.Write(t, tmp, []byte("A"))
				}
				calls := 0
				var out bytes.Buffer
				result, e := Export(ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t), Stdout: &out, afterCapture: func() {
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
						fixture.Write(t, tmp, []byte{byte('A' + calls)})
						if e = os.Chtimes(tmp, info.ModTime(), info.ModTime()); e != nil {
							t.Fatal(e)
						}
					case "orphan":
						raw := []byte(fmt.Sprint(calls))
						fixture.Write(t, filepath.Join(r.StateDir, "evidence", string(wire.Sum(raw))), raw)
					default:
						path := dir
						if kind == "root" {
							path = r.StateDir
						}
						if kind == "intent-root" {
							path = r.IntentDir
						}
						swapDirectory(t, path)
					}
				}})
				if persistent {
					if code(e) != wire.CodeSnapshotMoved || calls != 4 || result != nil || out.Len() != 0 {
						t.Fatalf("calls %d result %v bytes %d err %v", calls, result != nil, out.Len(), e)
					}
				} else {
					if e != nil || calls != 2 {
						t.Fatalf("calls %d err %v", calls, e)
					}
					if _, e := Verify(bytes.NewReader(out.Bytes())); e != nil {
						t.Fatal(e)
					}
					want := 16
					if kind == "orphan" {
						want++
					}
					if len(result.Manifest.Files) != want {
						t.Fatalf("stale file list %d", len(result.Manifest.Files))
					}
				}
			})
		}
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

func TestTMV0022_AS36_StageBodyErrorRecheck(t *testing.T) {
	for _, moving := range []bool{false, true} {
		t.Run(fmt.Sprint(moving), func(t *testing.T) {
			r, repo := repoWithStore(t)
			tmp := filepath.Join(r.StateDir, "staging/active.json.tmp")
			fixture.Write(t, tmp, []byte("a"))
			boom := wire.Errorf(wire.CodeMalformed, "body", "stable body failure")
			calls := 0
			var out bytes.Buffer
			result, e := Export(ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t), Stdout: &out, afterBody: func() error {
				calls++
				if calls > 1 {
					return nil
				}
				if moving {
					fixture.Write(t, tmp, []byte("bb"))
				}
				return boom
			}})
			if moving {
				if e != nil || calls != 2 || result == nil {
					t.Fatalf("%d %v", calls, e)
				}
			} else {
				if e != boom || calls != 1 || result != nil || out.Len() != 0 {
					t.Fatalf("stable error not retained: %d %v", calls, e)
				}
			}
		})
	}
}

func TestTMV0022_AS10_StageScanAndPayloadBounds(t *testing.T) {
	r, _ := repoWithStore(t)
	dir := filepath.Join(r.StateDir, "staging")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, children := range []int{0, 1, 13} {
		if children == 1 {
			fixture.Write(t, filepath.Join(dir, "active.json.tmp"), nil)
		}
		if children == 13 {
			fixture.Write(t, filepath.Join(dir, "active.json"), stageDescriptor(t, r, false))
			for i := 0; i < 11; i++ {
				fixture.Write(t, filepath.Join(dir, fmt.Sprintf("a%02d", i)), nil)
			}
		}
		for _, delta := range []int{0, -1} {
			files, n, e := stateLayout(r.StateDir, 20+children+delta)
			if delta == 0 {
				if e != nil || n != 20+children || len(files) != 12 {
					t.Fatalf("%d %d %v", len(files), n, e)
				}
			} else if code(e) != wire.CodeLimitExceeded {
				t.Fatal(e)
			}
		}
	}
	// All fourteen names include an unreadable special excess child. Enumeration
	// must refuse the count before any child metadata/read hook can run.
	if e := syscall.Mkfifo(filepath.Join(dir, "z-flood"), 0600); e != nil {
		t.Fatal(e)
	}
	_, _, e := stateLayout(r.StateDir, 100)
	if code(e) != wire.CodeLimitExceeded {
		t.Fatal(e)
	}
}

func TestTMV0022_AS36_StageStreamAndCleanupSticky(t *testing.T) {
	for _, kind := range []string{"stream", "close"} {
		t.Run(kind, func(t *testing.T) {
			r, repo := repoWithStore(t)
			tmp := filepath.Join(r.StateDir, "staging/active.json.tmp")
			fixture.Write(t, tmp, []byte("a"))
			opts := ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t)}
			var out bytes.Buffer
			opts.Stdout = &out
			calls := 0
			opts.afterBody = func() error {
				calls++
				if calls == 1 {
					fixture.Write(t, tmp, []byte("bb"))
				}
				return nil
			}
			if kind == "stream" {
				baseline, _, e := export(t, repo, fixture.TempDirOutside(t))
				if e != nil {
					t.Fatal(e)
				}
				raw := bytes.Repeat([]byte("large orphan"), 2048)
				orphan := filepath.Join(r.StateDir, "evidence", string(wire.Sum(raw)))
				fixture.Write(t, orphan, raw)
				opts.streamLimit = uint64(len(baseline))
				opts.afterBody = func() error {
					calls++
					if calls == 1 {
						if e := os.Remove(orphan); e != nil {
							t.Fatal(e)
						}
						fixture.Write(t, tmp, []byte("bb"))
					}
					return nil
				}
			} else {
				old := closeStaging
				t.Cleanup(func() { closeStaging = old })
				closeStaging = func(f *os.File) error { return errors.Join(old(f), syscall.EIO) }
			}
			result, e := Export(opts)
			if e == nil || result != nil || out.Len() != 0 {
				t.Fatalf("sticky %s lost: %v", kind, e)
			}
			if kind == "stream" && (code(e) != wire.CodeLimitExceeded || calls != 2) {
				t.Fatalf("later smaller attempt: calls=%d err=%v", calls, e)
			}
			if kind == "close" && !strings.Contains(e.Error(), syscall.EIO.Error()) {
				t.Fatal(e)
			}
		})
	}
}

func TestTMV0022_AS36_ArchiveCoordinatorOneShotAndReprobe(t *testing.T) {
	for _, mode := range []string{"callback", "header", "pending", "fork"} {
		t.Run(mode, func(t *testing.T) {
			r, repo := repoWithStore(t)
			calls := 0
			rd := snapshot.Reader{StateDir: repo.StateDir} // default retries must be overridden
			_, e := readArchive(rd, func(*snapshot.Snapshot) error {
				calls++
				switch mode {
				case "callback":
					return archiveMoved("body")
				case "header":
					fixture.Commit(t, r, "MUTATION")
				case "pending", "fork":
					head, e := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
					if e != nil {
						t.Fatal(e)
					}
					fixture.Commit(t, r, "MUTATION")
					if mode == "fork" {
						fixture.Commit(t, r, "MUTATION")
					}
					fixture.Write(t, filepath.Join(r.StateDir, "head.json"), head)
				}
				return nil
			})
			want := wire.CodeSnapshotMoved
			n := 4
			if mode == "pending" {
				want = wire.CodeRedoPending
				n = 1
			}
			if mode == "fork" {
				want = wire.CodeJournalForked
				n = 1
			}
			if code(e) != want || calls != n {
				t.Fatalf("%s callbacks=%d err=%v", mode, calls, e)
			}
		})
	}
}

func TestTMV0022_AS36_ArchiveStageValidationAndFailedAfterCapture(t *testing.T) {
	for _, mode := range []string{"validation-stable", "validation-moves", "after-failure", "missing-head", "pending", "fork"} {
		t.Run(mode, func(t *testing.T) {
			r, repo := repoWithStore(t)
			active := filepath.Join(r.StateDir, "staging/active.json")
			want := wire.CodeMalformed
			if strings.HasPrefix(mode, "validation") {
				fixture.Write(t, active, []byte("{"))
			} else {
				fixture.Write(t, filepath.Join(r.StateDir, "staging/active.json.tmp"), []byte("{"))
			}
			if mode == "missing-head" {
				if e := os.Remove(filepath.Join(r.StateDir, "head.json")); e != nil {
					t.Fatal(e)
				}
				want = wire.CodeUninitialized
			}
			if mode == "pending" || mode == "fork" {
				head, e := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
				if e != nil {
					t.Fatal(e)
				}
				fixture.Commit(t, r, "MUTATION")
				want = wire.CodeRedoPending
				if mode == "fork" {
					fixture.Commit(t, r, "MUTATION")
					want = wire.CodeJournalForked
				}
				fixture.Write(t, filepath.Join(r.StateDir, "head.json"), head)
			}
			calls := 0
			var out bytes.Buffer
			before, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			result, e := Export(ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t), Stdout: &out, afterCapture: func() {
				calls++
				if mode == "validation-moves" && calls == 1 {
					fixture.Write(t, active, stageDescriptor(t, r, false))
				}
			}, afterBody: func() error {
				if mode == "after-failure" {
					fixture.Write(t, filepath.Join(r.StateDir, "staging/unknown"), nil)
				}
				return nil
			}})
			if mode == "validation-moves" {
				if e != nil || calls != 2 || result == nil {
					t.Fatalf("calls %d err %v", calls, e)
				}
				return
			}
			if code(e) != want || result != nil || out.Len() != 0 {
				t.Fatalf("%s calls %d out %d err %v", mode, calls, out.Len(), e)
			}
			if mode != "after-failure" {
				fixture.AssertUntouched(t, r, before, intents, "staging refusal")
			}
		})
	}
}

func TestTMV0022_AS07_ArchiveNativeHandlesAndFIFO(t *testing.T) {
	for _, mode := range []string{"success", "body-error", "movement", "close-error", "FIFO", "disappeared", "empty-root", "listed-parent"} {
		t.Run(mode, func(t *testing.T) {
			r, repo := repoWithStore(t)
			dir := filepath.Join(r.StateDir, "staging")
			tmp := filepath.Join(dir, "active.json.tmp")
			fixture.Write(t, tmp, []byte("foreign"))
			if mode == "FIFO" || mode == "disappeared" || mode == "empty-root" || mode == "listed-parent" {
				life := newArchiveRead(archiveNative{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree})
				defer func() {
					if e := life.close(); e != nil {
						t.Error(e)
					}
				}()
				if mode == "empty-root" {
					life.native.StateDir = fixture.TempDirOutside(t)
					if _, e := life.List(".", 0); e != nil {
						t.Fatal(e)
					}
					swapDirectory(t, life.native.StateDir)
					_, e := life.List(".", 0)
					if code(e) != wire.CodeSnapshotMoved {
						t.Fatal(e)
					}
					return
				}
				if mode == "disappeared" {
					old := afterReadNames
					t.Cleanup(func() { afterReadNames = old })
					afterReadNames = func(p string) {
						if p == "staging" {
							if e := os.Remove(tmp); e != nil {
								t.Fatal(e)
							}
						}
					}
					_, e := life.List("staging", 13)
					if code(e) != wire.CodeSnapshotMoved {
						t.Fatal(e)
					}
					return
				}
				if _, e := life.List("staging", 13); e != nil {
					t.Fatal(e)
				}
				if mode == "FIFO" {
					if e := os.Remove(tmp); e != nil {
						t.Fatal(e)
					}
					if e := syscall.Mkfifo(tmp, 0600); e != nil {
						t.Fatal(e)
					}
					_, e := life.Read("staging/active.json.tmp", 100)
					if code(e) != wire.CodeUnsupportedFilesystem {
						t.Fatal(e)
					}
					return
				}
				if e := os.Rename(dir, dir+"-old"); e != nil {
					t.Fatal(e)
				}
				fixture.Write(t, tmp, []byte("new foreign"))
				raw, e := life.Read("staging/active.json.tmp", 100)
				if e != nil || string(raw) != "foreign" {
					t.Fatalf("redirected: %q %v", raw, e)
				}
				_, e = life.List("staging", 13)
				if code(e) != wire.CodeSnapshotMoved {
					t.Fatal(e)
				}
				return
			}
			old := closeReadRoot
			var closed []*os.Root
			t.Cleanup(func() { closeReadRoot = old })
			closeReadRoot = func(root *os.Root) error {
				closed = append(closed, root)
				e := old(root)
				if mode == "close-error" {
					return errors.Join(e, syscall.EIO)
				}
				return e
			}
			opts := ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t)}
			var out bytes.Buffer
			opts.Stdout = &out
			if mode == "body-error" {
				opts.afterBody = func() error { return syscall.EIO }
			}
			if mode == "movement" {
				opts.afterCapture = func() { swapDirectory(t, r.StateDir) }
			}
			res, e := Export(opts)
			if mode == "success" {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || res != nil || out.Len() != 0 {
				t.Fatal("error/closure lost")
			}
			if len(closed) == 0 {
				t.Fatal("no roots closed")
			}
			for _, root := range closed {
				if _, e := root.Stat("."); e == nil {
					t.Fatal("root left open")
				}
			}
		})
	}
}

func TestTMV0022_AS07_ArchiveStageFixtureRestriction(t *testing.T) {
	r, repo := repoWithStore(t)
	raw, e := os.ReadFile(filepath.Join(r.IntentDir, "queue.json"))
	if e != nil {
		t.Fatal(e)
	}
	q, e := intent.DecodeQueue(raw)
	if e != nil {
		t.Fatal(e)
	}
	q.Fixture = false
	fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/queue.json": wire.EncodeFile(q.Value())})
	dir := filepath.Join(r.StateDir, "staging")
	if e = os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	if _, _, e = export(t, repo, fixture.TempDirOutside(t)); e != nil {
		t.Fatal(e)
	}
	fixture.Write(t, filepath.Join(dir, "active.json.tmp"), nil)
	out, res, e := export(t, repo, fixture.TempDirOutside(t))
	if code(e) != wire.CodeUnsupported || len(out) != 0 || res != nil {
		t.Fatal(e)
	}
}

func TestTMV0022_AS07_ArchiveFileClosureErrorsSurviveMovement(t *testing.T) {
	r, repo := repoWithStore(t)
	tmp := filepath.Join(r.StateDir, "staging/active.json.tmp")
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
	out, res, e := export(t, repo, fixture.TempDirOutside(t))
	if res != nil || len(out) != 0 || e == nil || !strings.Contains(e.Error(), syscall.EBUSY.Error()) {
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

func TestTMV0022_AS10_ArchiveStageEmptyRaw(t *testing.T) {
	repo, r := repoWithStore(t)
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
	out, res, e := export(t, r, fixture.TempDirOutside(t))
	if e != nil || res == nil || len(out) == 0 {
		t.Fatal(e)
	}
	fixture.AssertUntouched(t, repo, before, intents, "declared empty raw stage bytes")
}

func setReceiptKind(t *testing.T, r *fixture.Repo, kind string) {
	t.Helper()
	hp := filepath.Join(r.StateDir, "head.json")
	raw, e := os.ReadFile(hp)
	if e != nil {
		t.Fatal(e)
	}
	h, e := snapshot.DecodeHead(raw)
	if e != nil {
		t.Fatal(e)
	}
	name, _ := snapshot.ReceiptName(h.LastSeq.Uint64())
	path := filepath.Join(r.StateDir, "receipts", name)
	raw, e = os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	v, e := wire.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	v.Obj.Set("kind", wire.String(kind))
	raw = wire.EncodeFile(v)
	fixture.Write(t, path, raw)
	hash := wire.Sum(raw)
	h.LastReceiptSha256 = &hash
	if h.LastSeq == "1" {
		h.InitSha256 = hash
	}
	fixture.Write(t, hp, wire.EncodeFile(h.Value()))
}

func TestTMV0022_AS07_ArchiveCompletedStageLabels(t *testing.T) {
	for _, mode := range []string{"wrong-label", "timestamp"} {
		t.Run(mode, func(t *testing.T) {
			r, repo := repoWithStore(t)
			if mode == "wrong-label" {
				barrier := wire.NewObject()
				for k, v := range map[string]string{"profile": snapshot.ProfileBarrier, "queueId": fixture.QueueID, "scope": "ADMISSION", "reason": "OPERATOR", "actor": fixture.Actor, "sinceSeq": "4", "since": fixture.Timestamp} {
					barrier.Set(k, wire.String(v))
				}
				seq := wire.Size("4")
				outcome := mutation.Outcome{RequestID: "pause", Outcome: mutation.OutcomeCompleted, ReceiptSeq: &seq}
				req := wire.NewObject()
				req.Set("requestId", wire.String("pause"))
				req.Set("seq", wire.String("4"))
				req.Set("mutationSha256", wire.String(string(wire.Sum([]byte("pause")))))
				req.Set("outcome", outcome.Value())
				rp, _ := snapshot.RequestPath("pause")
				fixture.CommitPosts(t, r, "PAUSE", "pause", map[string][]byte{"barrier.json": wire.EncodeFile(wire.ObjectValue(barrier)), rp: wire.EncodeFile(wire.ObjectValue(req))})
			} else {
				setReceiptKind(t, r, "UNPAUSE")
			}
			if _, _, e := export(t, repo, fixture.TempDirOutside(t)); e != nil {
				t.Fatal("invalid underlying fixture", e)
			}
			raw := stageDescriptor(t, r, true)
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
			fixture.Write(t, filepath.Join(r.StateDir, "staging/active.json"), raw)
			before, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			out, res, e := export(t, repo, fixture.TempDirOutside(t))
			if code(e) != wire.CodeJournalForked || len(out) != 0 || res != nil {
				t.Fatalf("label mismatch accepted: %v", e)
			}
			fixture.AssertUntouched(t, r, before, intents, "completed-stage labels")
		})
	}
}

func TestTMV0022_AS07_ArchiveCompletedStageRequestBlob(t *testing.T) {
	r, repo := repoWithStore(t)
	setReceiptKind(t, r, "UNPAUSE")
	hp := filepath.Join(r.StateDir, "head.json")
	raw, e := os.ReadFile(hp)
	if e != nil {
		t.Fatal(e)
	}
	h, e := snapshot.DecodeHead(raw)
	if e != nil {
		t.Fatal(e)
	}
	name, _ := snapshot.ReceiptName(h.LastSeq.Uint64())
	rp := filepath.Join(r.StateDir, "receipts", name)
	raw, e = os.ReadFile(rp)
	if e != nil {
		t.Fatal(e)
	}
	v, e := wire.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	posts, _ := v.Obj.Get("post")
	post := posts.Arr[0]
	record, _ := post.Obj.Get("record")
	request := wire.EncodeFile(record)
	hash := wire.Sum(request)
	post.Obj.Set("record", wire.Null())
	post.Obj.Set("blobSha256", wire.String(string(hash)))
	raw = wire.EncodeFile(v)
	fixture.Write(t, rp, raw)
	receiptHash := wire.Sum(raw)
	h.LastReceiptSha256 = &receiptHash
	fixture.Write(t, hp, wire.EncodeFile(h.Value()))
	blob := filepath.Join(r.StateDir, "evidence", string(hash))
	fixture.Write(t, blob, request)
	fixture.Write(t, filepath.Join(r.StateDir, "staging/active.json"), stageDescriptor(t, r, true))
	for _, missing := range []bool{false, true} {
		if missing {
			if e := os.Remove(blob); e != nil {
				t.Fatal(e)
			}
		}
		before, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
		out, res, e := export(t, repo, fixture.TempDirOutside(t))
		if missing {
			if code(e) != wire.CodeJournalForked || res != nil || len(out) != 0 {
				t.Fatalf("missing request proof: %v", e)
			}
		} else if e != nil {
			t.Fatal(e)
		}
		fixture.AssertUntouched(t, r, before, intents, "completed request blob")
	}
}

func TestTMV0022_AS10_StageExcludedFromFileCapBeforeBodies(t *testing.T) {
	r, repo := repoWithStore(t)
	fixture.Write(t, filepath.Join(r.StateDir, "staging/active.json.tmp"), bytes.Repeat([]byte("x"), 2422))
	for _, cap := range []int{16, 15} {
		life := newArchiveRead(archiveNative{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree})
		opened := 0
		old := closeReadFile
		closeReadFile = func(f *os.File) error {
			if info, e := f.Stat(); e == nil && info.Mode().IsRegular() {
				opened++
			}
			return old(f)
		}
		files, _, _, e := life.captureLayout(100, cap)
		closeReadFile = old
		if ce := life.close(); ce != nil {
			t.Fatal(ce)
		}
		if cap == 16 {
			if e != nil || len(files) != 16 || opened == 0 {
				t.Fatalf("stage charged to F: %d %v", len(files), e)
			}
		} else if code(e) != wire.CodeLimitExceeded || opened != 0 {
			t.Fatalf("body opened before cap refusal: %d %v", opened, e)
		}
	}
}
