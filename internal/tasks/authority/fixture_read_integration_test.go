//go:build darwin || linux

package authority

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func ftReader(h *ftHarness) journal.Reader {
	q, err := wire.ParseQueueID("", fixture.QueueID)
	fixtureMust(h.t, err)
	return journal.Reader{Source: journal.Native{StateDir: h.r.StateDir, PrimaryWorktree: h.r.Root}, QueueID: q, PrimaryWorktree: h.r.Root}
}

func ftCode(t *testing.T, err error, want string) {
	t.Helper()
	if wire.CodeOf(err) != want {
		t.Fatalf("want %q, got %v", want, err)
	}
}

// Real native readers run while the fixture lock is held. The whole repository
// snapshot includes lock, directories, source, intent and private bytes/metadata.
func ftNativeRead(t *testing.T, h *ftHarness, journalCode, archiveCode string) (*journal.Result, *archive.ExportResult) {
	t.Helper()
	before := fixture.TreeSnapshot(t, h.r.Root)
	defer func() {
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, h.r.Root)) {
			t.Fatal("native read changed source/lock/directory bytes, mode or mtime")
		}
	}()
	audit, err := ftReader(h).Audit()
	ftCode(t, err, journalCode)
	repo, err := intent.Resolve(h.r.Root)
	fixtureMust(t, err)
	outside := fixture.TempDirOutside(t)
	var out bytes.Buffer
	exported, err := archive.Export(archive.ExportOptions{Repo: repo, Staging: outside, Stdout: &out})
	ftCode(t, err, archiveCode)
	names, err := os.ReadDir(outside)
	fixtureMust(t, err)
	if len(names) != 0 {
		t.Fatal("external export temp left behind")
	}
	if archiveCode != "" {
		if out.Len() != 0 || exported != nil {
			t.Fatal("pre-delivery refusal emitted archive")
		}
		return audit, nil
	}
	_, err = archive.Verify(bytes.NewReader(out.Bytes()))
	fixtureMust(t, err)
	// Compare actual tar bodies and inventory, not just manifest assertions.
	disk, err := h.capture()
	fixtureMust(t, err)
	tr := tar.NewReader(bytes.NewReader(out.Bytes()))
	actual := map[string][]byte{}
	for {
		header, e := tr.Next()
		if e == io.EOF {
			break
		}
		fixtureMust(t, e)
		raw, e := io.ReadAll(tr)
		fixtureMust(t, e)
		if header.Name == archive.ManifestName {
			continue
		}
		if strings.HasPrefix(header.Name, "staging/") {
			t.Fatal("staging body exported")
		}
		actual[header.Name] = raw
	}
	if !reflect.DeepEqual(actual, disk.raw) {
		t.Fatal("export omitted/changed actual payload or linked orphan")
	}
	inv, err := disk.inventory()
	fixtureMust(t, err)
	got, want := append([]archive.FileEntry(nil), exported.Manifest.Files...), inv.Files()
	archive.SortFiles(got)
	archive.SortFiles(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("native manifest differs from full observed inventory")
	}
	return audit, exported
}

func TestTMV0022_AS11_FixtureNativeCompletedHistories(t *testing.T) {
	for _, op := range []string{transaction.Init, transaction.Pause, transaction.Unpause, transaction.KeepJournal, transaction.AdoptFile} {
		t.Run(op, func(t *testing.T) {
			h, p := ftFlow(t, op)
			if !ftStopAt(h, p, "durable-head") {
				t.Fatal("head without receipt")
			}
			h.restart()
			audit, _ := ftNativeRead(t, h, "", "")
			if !audit.StagingPresent || audit.Pending || audit.ActorAuthentication != "NOT_OBSERVED" {
				t.Fatal("completed leftovers claims")
			}
			fixtureMust(t, h.recover()) // receives no Plan/cached afterimages
			ftFinal(t, h, p)
			audit, exported := ftNativeRead(t, h, "", "")
			if audit.StagingPresent || audit.Pending {
				t.Fatal("empty persistent staging is active")
			}
			info, err := os.Lstat(filepath.Join(h.r.StateDir, "staging"))
			fixtureMust(t, err)
			if !info.IsDir() {
				t.Fatal("persistent staging missing")
			}
			disk, err := h.capture()
			fixtureMust(t, err)
			cost, err := disk.cost(disk.raw["head.json"])
			fixtureMust(t, err)
			enc, err := archive.MeasureManifestEncoding(exported.Manifest)
			fixtureMust(t, err)
			if cost.Files != uint64(enc.Files) || cost.ManifestBytes != enc.ManifestBytes || cost.TarBytes != enc.TarBytes || cost.PayloadBytes != exported.Bytes {
				t.Fatal("actual archive/final cost mismatch")
			}
		})
	}
}

func TestTMV0008_AS27_FixtureNativeStageParity(t *testing.T) {
	cases := []struct{ name, code string }{
		{"absent", ""}, {"empty", ""}, {"temp-empty", ""}, {"temp-malformed", ""}, {"temp-max", ""}, {"temp-over", wire.CodeLimitExceeded},
		{"active", ""}, {"temp-short", ""}, {"temp-equal", ""}, {"temp-different", wire.CodeJournalForked}, {"temp-long", wire.CodeLimitExceeded},
		{"slot-empty", ""}, {"slot-partial", ""}, {"slot-complete", ""}, {"slot-wrong", wire.CodeJournalForked}, {"slot-long", wire.CodeLimitExceeded},
		{"unknown", wire.CodeMalformed}, {"unassigned", wire.CodeMalformed}, {"active-malformed", wire.CodeMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := ftNew(t, true)
			p := h.plan(h.request(transaction.Pause, "parity"))
			if tc.name != "absent" {
				fixtureMust(t, h.directory("staging"))
			}
			active := p.Descriptor()
			h.era = wire.Sum(active)
			write := func(name string, raw []byte) { fixture.Write(t, filepath.Join(h.r.StateDir, "staging", name), raw) }
			if tc.name != "absent" && tc.name != "empty" && !strings.HasPrefix(tc.name, "temp-m") && tc.name != "temp-empty" && tc.name != "temp-over" {
				write("active.json", active)
			}
			a := p.Artifacts()[0]
			switch tc.name {
			case "temp-empty":
				write("active.json.tmp", []byte{})
			case "temp-malformed":
				write("active.json.tmp", []byte("{"))
			case "temp-max":
				write("active.json.tmp", make([]byte, snapshot.MaxStageDescriptorBytes))
			case "temp-over":
				write("active.json.tmp", make([]byte, snapshot.MaxStageDescriptorBytes+1))
			case "temp-short":
				write("active.json.tmp", active[:len(active)-1])
			case "temp-equal":
				write("active.json.tmp", active)
			case "temp-different":
				write("active.json.tmp", make([]byte, len(active)))
			case "temp-long":
				write("active.json.tmp", append(bytes.Clone(active), 'x'))
			case "slot-empty":
				write(a.Slot, []byte{})
			case "slot-partial":
				write(a.Slot, a.Data[:len(a.Data)-1])
			case "slot-complete":
				write(a.Slot, a.Data)
			case "slot-wrong":
				write(a.Slot, make([]byte, len(a.Data)))
			case "slot-long":
				write(a.Slot, append(bytes.Clone(a.Data), 'x'))
			case "unknown":
				write("foreign", []byte("preserve"))
			case "unassigned":
				write("a10", []byte{})
			case "active-malformed":
				write("active.json", []byte("{}\n"))
				h.era = wire.Sum([]byte("{}\n"))
			}
			// Independent actual-name read into the shared pure structural observer.
			var files []snapshot.StageFile
			if tc.name != "absent" {
				entries, err := os.ReadDir(filepath.Join(h.r.StateDir, "staging"))
				fixtureMust(t, err)
				for _, entry := range entries {
					raw, err := os.ReadFile(filepath.Join(h.r.StateDir, "staging", entry.Name()))
					fixtureMust(t, err)
					files = append(files, snapshot.StageFile{Name: entry.Name(), Raw: raw})
				}
			}
			_, err := snapshot.ObserveStage(files)
			ftCode(t, err, tc.code)
			h.restart()
			before := fixture.TreeSnapshot(t, h.r.Root)
			o, err := h.observe(false)
			if tc.code != "" {
				if err == nil {
					t.Fatal("fixture reopen accepted structural refusal")
				}
			} else {
				fixtureMust(t, err)
				tokens, e := h.reopen(o, true)
				fixtureMust(t, e)
				for _, token := range tokens {
					if h.s.stages[token] {
						t.Fatal("cleanup observation became publication authority")
					}
				}
				if tc.name == "slot-empty" || tc.name == "slot-partial" {
					if _, e = h.reopen(o, false); e == nil {
						t.Fatal("partial publication token")
					}
				}
				if tc.name == "slot-complete" {
					tokens, e = h.reopen(o, false)
					fixtureMust(t, e)
					if !h.s.stages[tokens[a.Slot]] {
						t.Fatal("complete retained token missing")
					}
				}
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, h.r.Root)) {
				t.Fatal("observer/reopen wrote source")
			}
			ftNativeRead(t, h, tc.code, tc.code)
		})
	}
	for _, kind := range []string{"symlink", "fifo", "directory"} {
		t.Run(kind, func(t *testing.T) {
			h := ftNew(t, true)
			fixtureMust(t, h.directory("staging"))
			path := filepath.Join(h.r.StateDir, "staging/a00")
			switch kind {
			case "symlink":
				fixtureMust(t, os.Symlink(filepath.Join(h.r.Root, "source.txt"), path))
			case "fifo":
				fixtureMust(t, syscall.Mkfifo(path, 0600))
			case "directory":
				fixtureMust(t, os.Mkdir(path, 0700))
			}
			ftUnchanged(t, h, func() error { _, err := h.observe(false); return err })
			journalCode := wire.CodeUnsupportedFilesystem
			if kind == "directory" {
				journalCode = wire.CodeMalformed
			}
			ftNativeRead(t, h, journalCode, wire.CodeUnsupportedFilesystem)
		})
	}
}

func TestTMV0006_AS35_FixtureNativeEmptyIntentIndex(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			h := ftNew(t, true)
			_, err := h.publish(h.plan(h.request(transaction.Pause, "index-pause")), nil)
			fixtureMust(t, err)
			h.edit([]byte{})
			before := fixture.TreeSnapshot(t, h.r.Root)
			index := journal.RequestIndex{Reader: ftReader(h)}
			for _, id := range []string{"seed", "index-pause", "absent"} {
				entry, found, err := index.Lookup(id)
				fixtureMust(t, err)
				if found != (id != "absent") || index.IntentProjectionAgreement != "NOT_OBSERVED" {
					t.Fatal("request-only lookup agreement/absence")
				}
				if found {
					rp, _ := snapshot.RequestPath(id)
					raw, err := os.ReadFile(filepath.Join(h.r.StateDir, rp))
					fixtureMust(t, err)
					expected, err := snapshot.DecodeRequest(raw)
					fixtureMust(t, err)
					if !reflect.DeepEqual(entry, expected.Entry) {
						t.Fatal("recorded original outcome changed")
					}
				}
			}
			result, err := ftReader(h).Audit(ftTicket)
			ftCode(t, err, wire.CodeIntentDiverged)
			if result == nil || result.ProjectionAgreement != "NOT_OBSERVED" {
				t.Fatal("failed strict audit claimed agreement")
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, h.r.Root)) {
				t.Fatal("empty D reads changed source")
			}
			ftNativeRead(t, h, wire.CodeIntentDiverged, wire.CodeMalformed)
			rp, _ := snapshot.RequestPath("index-pause")
			if mode == "missing" {
				fixtureMust(t, os.Remove(filepath.Join(h.r.StateDir, rp)))
			} else {
				fixture.Write(t, filepath.Join(h.r.StateDir, rp), []byte("{}\n"))
			}
			ftUnchanged(t, h, func() error {
				for _, id := range []string{"seed", "index-pause", "absent"} {
					_, found, e := index.Lookup(id)
					if e == nil || found {
						t.Fatal("private corruption became verified request result")
					}
				}
				return fixtureRefused
			})
		})
	}
}

func TestTMV0009_AS11_FixtureNativeInterruptedStatuses(t *testing.T) {
	for _, op := range []string{transaction.Init, transaction.Pause, transaction.Unpause, transaction.KeepJournal, transaction.AdoptFile} {
		t.Run(op, func(t *testing.T) {
			h, p := ftFlow(t, op)
			if !ftStopAt(h, p, "durable-receipt-link") {
				t.Fatal("missing commit")
			}
			h.restart()
			archiveCode := wire.CodeRedoPending
			if op == transaction.Init {
				archiveCode = wire.CodeUninitialized
				for _, path := range []string{filepath.Join(h.r.StateDir, "head.json"), filepath.Join(h.r.IntentDir, "queue.json")} {
					if _, err := os.Lstat(path); !os.IsNotExist(err) {
						t.Fatal("earliest INIT already materialized head/queue")
					}
				}
			}
			ftNativeRead(t, h, wire.CodeRedoPending, archiveCode)
			if op == transaction.Init {
				for _, a := range p.Artifacts() {
					if a.Target == "intent/queue.json" {
						fixtureMust(t, os.Remove(filepath.Join(h.r.StateDir, "staging", a.Slot)))
					}
				}
				ftNativeRead(t, h, wire.CodeRedoPending, archiveCode)
			}
			fixtureMust(t, h.recover())
			ftNativeRead(t, h, "", "")
		})
	}
	for _, mode := range []string{"headless-temp", "init-missing-blob", "init-corrupt-blob", "fork"} {
		t.Run(mode, func(t *testing.T) {
			op := transaction.Init
			if mode == "fork" {
				op = transaction.Pause
			}
			h, p := ftFlow(t, op)
			stop := "durable-receipt-link"
			if mode == "headless-temp" {
				stop = "descriptor-temp"
			}
			ftStopAt(h, p, stop)
			jc, ac := wire.CodeUninitialized, wire.CodeUninitialized
			switch mode {
			case "init-missing-blob", "init-corrupt-blob":
				rc := mustReceipt(t, p.Receipt())
				var name string
				for _, post := range rc.Post {
					if post.Path == "VERSION" {
						name = filepath.Join(h.r.StateDir, "evidence", string(*post.BlobSha256))
					}
				}
				if mode == "init-missing-blob" {
					fixtureMust(t, os.Remove(name))
				} else {
					fixture.Write(t, name, []byte("broken"))
				}
				jc = wire.CodeJournalForked
			case "fork":
				fixture.PlantReceipt(t, h.r, 4)
				jc, ac = wire.CodeJournalForked, wire.CodeJournalForked
			}
			ftNativeRead(t, h, jc, ac)
		})
	}
}

func TestTMV0008_AS10_FixtureNativeAccountingAndSequentialReads(t *testing.T) {
	h := ftNew(t, true)
	_, beforeExport := ftNativeRead(t, h, "", "")
	absent, err := h.capture()
	fixtureMust(t, err)
	absentCost, err := absent.cost(absent.raw["head.json"])
	fixtureMust(t, err)
	fixtureMust(t, h.directory("staging"))
	_, emptyExport := ftNativeRead(t, h, "", "")
	empty, err := h.capture()
	fixtureMust(t, err)
	emptyCost, err := empty.cost(empty.raw["head.json"])
	fixtureMust(t, err)
	if emptyCost.ScannedEntries != absentCost.ScannedEntries+1 || emptyCost.Files != absentCost.Files || !reflect.DeepEqual(beforeExport.Manifest.Files, emptyExport.Manifest.Files) {
		t.Fatal("empty staging directory D/F charge")
	}
	_, err = h.publish(h.plan(h.request(transaction.Pause, "history-pause")), nil)
	fixtureMust(t, err)
	_, err = h.publish(h.plan(h.request(transaction.Unpause, "history-unpause")), nil)
	fixtureMust(t, err)
	h.edit([]byte{})
	_, err = h.publish(h.plan(h.request(transaction.KeepJournal, "history-keep")), nil)
	fixtureMust(t, err)
	ftNativeRead(t, h, "", "")
	canonical, err := ftReader(h).Audit(ftTicket)
	fixtureMust(t, err)
	edited, err := wire.Parse(canonical.Records[ftTicket].Raw)
	fixtureMust(t, err)
	edited.Obj.Set("title", wire.String("combined history edit"))
	h.edit(wire.EncodeFile(edited))
	_, err = h.publish(h.plan(h.request(transaction.AdoptFile, "history-adopt")), nil)
	fixtureMust(t, err)
	first, exported := ftNativeRead(t, h, "", "")
	disk, err := h.capture()
	fixtureMust(t, err)
	baseCost, err := disk.cost(disk.raw["head.json"])
	fixtureMust(t, err)
	// Both orphans come from the common modeled KEEP sequence, before receipt.
	// Each valid D remains the physical projection after abort; exports retain it.
	for i := 0; i < 2; i++ {
		offered, err := wire.Parse(disk.raw[ftTicket])
		fixtureMust(t, err)
		offered.Obj.Set("title", wire.String(fmt.Sprintf("aborted valid edit %d", i)))
		discarded := wire.EncodeFile(offered)
		h.edit(discarded)
		p := h.plan(h.request(transaction.KeepJournal, fmt.Sprintf("orphan-%d", i)))
		if ftStopAt(h, p, "durable-link:evidence/"+string(wire.Sum(discarded))) {
			t.Fatal("orphan prefix committed")
		}
		if i == 0 {
			h.restart()
			fixtureMust(t, h.cleanup(true))
			ftNativeRead(t, h, wire.CodeIntentDiverged, "")
		}
	}
	second, secondExport := ftNativeRead(t, h, wire.CodeIntentDiverged, "")
	if first.Identity.HeadSha256 != second.Identity.HeadSha256 || first.Identity.InventorySha256 == second.Identity.InventorySha256 {
		t.Fatal("unchanged-head read reused old inventory")
	}
	disk, err = h.capture()
	fixtureMust(t, err)
	stagedCost, err := disk.cost(disk.raw["head.json"])
	fixtureMust(t, err)
	if stagedCost.ScannedEntries != baseCost.ScannedEntries+uint64(len(disk.stage))+2 || stagedCost.Files != baseCost.Files+2 || len(secondExport.Manifest.Files) != len(exported.Manifest.Files)+2 {
		t.Fatal("staging D/orphan F accounting")
	}
	// A same-size temp edit with restored mtime changes the next native identity.
	temp := filepath.Join(h.r.StateDir, "staging/active.json.tmp")
	fixture.Write(t, temp, []byte("abc"))
	third, _ := ftNativeRead(t, h, wire.CodeIntentDiverged, "")
	info, err := os.Stat(temp)
	fixtureMust(t, err)
	fixtureMust(t, os.WriteFile(temp, []byte("xyz"), 0600))
	fixtureMust(t, os.Chtimes(temp, info.ModTime(), info.ModTime()))
	fourth, _ := ftNativeRead(t, h, wire.CodeIntentDiverged, "")
	if third.Identity.HeadSha256 != fourth.Identity.HeadSha256 || third.Identity.InventorySha256 == fourth.Identity.InventorySha256 {
		t.Fatal("sequential read missed restored-mtime stage bytes")
	}
	h.restart()
	fixtureMust(t, h.cleanup(true))
	final, finalExport := ftNativeRead(t, h, wire.CodeIntentDiverged, "")
	if final.Identity.HeadSha256 != first.Identity.HeadSha256 || len(finalExport.Manifest.Files) != len(secondExport.Manifest.Files) {
		t.Fatal("abort removed linked orphan or moved head")
	}
}

func TestTMV0008_AS11_FixtureNativeStageBindings(t *testing.T) {
	for _, mode := range []string{"queue", "base", "request", "request-id", "timestamp", "operation", "head", "receipt", "actual-queue"} {
		t.Run(mode, func(t *testing.T) {
			h, p := ftFlow(t, transaction.Pause)
			ftStopAt(h, p, "durable-head")
			// Keep only active, a valid completed cleanup subset. Wrong-label descriptors
			// remain structurally valid, so receipt-kind binding is the causal check.
			entries, err := os.ReadDir(filepath.Join(h.r.StateDir, "staging"))
			fixtureMust(t, err)
			for _, entry := range entries {
				if entry.Name() != "active.json" {
					fixtureMust(t, os.Remove(filepath.Join(h.r.StateDir, "staging", entry.Name())))
				}
			}
			d, err := snapshot.DecodeStageDescriptor(p.Descriptor())
			fixtureMust(t, err)
			switch mode {
			case "queue":
				d.QueueID = "queue:other:q"
			case "base":
				d.Base.LastReceiptSha256 = wire.Sum([]byte("wrong base"))
			case "request":
				d.RequestSha256 = wire.Sum([]byte("wrong request"))
			case "request-id":
				d.RequestID = "other-request"
				rp, _ := snapshot.RequestPath(d.RequestID)
				for i := range d.Artifacts {
					if strings.HasPrefix(d.Artifacts[i].Target, "requests/") {
						d.Artifacts[i].Target = rp
					}
				}
			case "timestamp":
				d.RecordedAt = "2026-09-07T12:00:00Z"
			case "operation":
				d.Operation = transaction.Unpause
				kept := []snapshot.StageDescription{}
				for _, a := range d.Artifacts {
					if a.Target != "barrier.json" {
						kept = append(kept, a)
					}
				}
				d.Artifacts = kept
			case "head", "receipt":
				for i := range d.Artifacts {
					if strings.EqualFold(d.Artifacts[i].Role, mode) {
						d.Artifacts[i].Sha256 = wire.Sum([]byte("wrong actual bytes"))
					}
				}
			case "actual-queue":
				v, err := wire.Parse(fixture.QueueBytes())
				fixtureMust(t, err)
				v.Obj.Set("queueId", wire.String("queue:acme:other"))
				fixture.Write(t, filepath.Join(h.r.IntentDir, "queue.json"), wire.EncodeFile(v))
			}
			// Canonical ordering also assigns the descriptor's session-local slot names.
			for i := range d.Artifacts {
				d.Artifacts[i].Slot = fmt.Sprintf("a%02d", i)
			}
			raw, err := d.Encode()
			fixtureMust(t, err)
			fixture.Write(t, filepath.Join(h.r.StateDir, "staging/active.json"), raw)
			h.era = wire.Sum(raw)
			shared, err := snapshot.ObserveStage([]snapshot.StageFile{{Name: "active.json", Raw: raw}})
			fixtureMust(t, err)
			head, err := os.ReadFile(filepath.Join(h.r.StateDir, "head.json"))
			fixtureMust(t, err)
			queue, err := os.ReadFile(filepath.Join(h.r.IntentDir, "queue.json"))
			fixtureMust(t, err)
			linkedHead, err := snapshot.DecodeHead(head)
			fixtureMust(t, err)
			receiptName, err := snapshot.ReceiptName(linkedHead.LastSeq.Uint64())
			fixtureMust(t, err)
			receiptRaw, err := os.ReadFile(filepath.Join(h.r.StateDir, "receipts", receiptName))
			fixtureMust(t, err)
			rc := mustReceipt(t, receiptRaw)
			var req []byte
			for _, post := range rc.Post {
				if strings.HasPrefix(post.Path, "requests/") {
					disk, err := h.capture()
					fixtureMust(t, err)
					req, err = ftPost(disk, post)
					fixtureMust(t, err)
				}
			}
			ftCode(t, shared.Bind(snapshot.StageBinding{QueueID: fixture.QueueID, QueueRaw: queue, HeadRaw: head, ReceiptRaw: receiptRaw, RequestRaw: req}), wire.CodeJournalForked)
			h.restart()
			ftUnchanged(t, h, func() error { _, err := h.observe(true); return err })
			ftNativeRead(t, h, wire.CodeJournalForked, wire.CodeJournalForked)
		})
	}
}

func TestTMV0002_AS27_FixtureNativeAssignedEmptyEvidence(t *testing.T) {
	h, p := ftFlow(t, transaction.KeepJournal)
	// Complete the KEEP first, preserving present-empty discarded D. Its leftover
	// empty evidence slot remains an assigned complete slot, never absence.
	ftStopAt(h, p, "durable-head")
	h.restart()
	o, err := h.observe(true)
	fixtureMust(t, err)
	shared, err := o.disk.sharedStage()
	fixtureMust(t, err)
	tokens, err := h.reopen(o, false)
	fixtureMust(t, err)
	found := false
	for _, a := range o.desc.Artifacts {
		if a.Target == "evidence/"+string(wire.Sum([]byte{})) {
			found = true
			f, present := o.disk.stage[a.Slot]
			if !present || len(f.raw) != 0 || shared.Files[a.Slot] != wire.Sum([]byte{}) || tokens[a.Slot] == nil || !h.s.stages[tokens[a.Slot]] {
				t.Fatal("assigned complete empty slot lost")
			}
		}
	}
	if !found {
		t.Fatal("empty evidence artifact absent")
	}
	ftNativeRead(t, h, "", "")
}

func TestTMV0022_AS35_FixtureNativePrecommitIntent(t *testing.T) {
	for _, mode := range []string{"empty", "valid-divergent"} {
		t.Run(mode, func(t *testing.T) {
			h := ftNew(t, true)
			discarded := []byte{}
			if mode == "valid-divergent" {
				record := fixture.Ticket("AT-01")
				record.Title = "offered but not adopted"
				discarded = record.Encode()
			}
			h.edit(discarded)
			p := h.plan(h.request(transaction.KeepJournal, "precommit-keep"))
			stop := "durable-link:evidence/" + string(wire.Sum(discarded))
			if ftStopAt(h, p, stop) {
				t.Fatal("precommit fixture committed")
			}
			archiveCode := ""
			if mode == "empty" {
				archiveCode = wire.CodeMalformed
			}
			ftNativeRead(t, h, wire.CodeIntentDiverged, archiveCode)
			h.restart()
			fixtureMust(t, h.cleanup(true))
			raw, err := os.ReadFile(filepath.Join(h.r.IntentDir, "tickets/AT-01.json"))
			fixtureMust(t, err)
			if !bytes.Equal(raw, discarded) {
				t.Fatal("export/abort substituted canonical C for actual D")
			}
			ftNativeRead(t, h, wire.CodeIntentDiverged, archiveCode)
		})
	}
}

func TestTMV0007_AS35_FixtureNativeOrdinaryIntentRefusal(t *testing.T) {
	h := ftNew(t, true)
	h.edit([]byte("{"))
	before := fixture.TreeSnapshot(t, h.r.Root)
	_, err := intent.Load(h.r.Root)
	ftCode(t, err, wire.CodeMalformed)
	_, err = ftReader(h).Audit(ftTicket)
	ftCode(t, err, wire.CodeIntentDiverged)
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, h.r.Root)) {
		t.Fatal("ordinary malformed-D refusal changed source")
	}
}
