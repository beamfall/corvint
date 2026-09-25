package transaction

import (
	"archive/tar"
	"bytes"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestTMV0014_AS10_HardCapsInsideReserveAndOverflow(t *testing.T) {
	l := hardLimits()
	for _, row := range []struct {
		count, bytes uint64
		barrier, ok  bool
	}{{999999, MaxJournalBytes - 1683, true, true}, {1000000, MaxJournalBytes - 1683, true, false}, {999999, MaxJournalBytes - 1682, true, false}, {1000000, MaxJournalBytes, false, true}, {1000001, 0, false, false}, {0, MaxJournalBytes + 1, false, false}, {math.MaxUint64, 0, true, false}, {0, math.MaxUint64, true, false}} {
		e := journalReserve(Cost{JournalCount: row.count, JournalBytes: row.bytes}, row.barrier, l)
		if (e == nil) != row.ok {
			t.Fatalf("%+v: %v", row, e)
		}
	}
	for _, field := range []string{"files", "scan", "manifest", "tar", "evidence", "intent"} {
		c := Cost{}
		var cap uint64
		switch field {
		case "files":
			cap = l.files
			c.Files = cap
		case "scan":
			cap = l.scan
			c.ScannedEntries = cap
		case "manifest":
			cap = l.manifest
			c.ManifestBytes = cap
		case "tar":
			cap = l.tar
			c.TarBytes = cap
		case "evidence":
			cap = l.evidence
			c.EvidenceBytes = cap
		case "intent":
			cap = l.intent
			c.IntentBytes = cap
		}
		if e := check(c, l); e != nil {
			t.Fatal(field, e)
		}
		switch field {
		case "files":
			c.Files++
		case "scan":
			c.ScannedEntries++
		case "manifest":
			c.ManifestBytes++
		case "tar":
			c.TarBytes++
		case "evidence":
			c.EvidenceBytes++
		case "intent":
			c.IntentBytes++
		}
		if e := check(c, l); e == nil {
			t.Fatal(field, "cap+1")
		}
	}
	if _, e := add(math.MaxUint64, 1); e == nil {
		t.Fatal("overflow")
	}
	if e := check(Cost{TemporaryBytes: math.MaxUint64, PromisedBytes: 1}, l); e == nil {
		t.Fatal("temporary promise overflow")
	}
}
func TestTMV0022_AS10_ExactInventoryCostsAndEncoder(t *testing.T) {
	in, _ := initialized(t)
	h, _ := snapshot.DecodeHead(in.Head)
	c, e := measure(in.Inventory, h, 17, 31, 2)
	if e != nil {
		t.Fatal(e)
	}
	files := in.Inventory.Files()
	var payload uint64
	for _, f := range files {
		payload += f.Bytes.Uint64()
	}
	if c.PayloadBytes != payload || c.Files != uint64(len(files)) || c.TemporaryBytes != 17 || c.PromisedBytes != 31 {
		t.Fatal(c)
	}
	// Compare J2b accounting with actual modest manifest and standard-library tar
	// encodings. Archive metadata digests have fixed widths, so this independent
	// envelope's unrelated valid digest values have exactly the same length.
	m := archive.Manifest{QueueID: h.QueueID, ExportedAtSeq: h.LastSeq, HeadSha256: wire.Sum(in.Head), VersionSha256: h.VersionSha256, PrimaryWorktree: h.PrimaryWorktree, IntentTreeSha256: wire.Sum(nil), Files: files, ReceiptCount: h.LastSeq, LastReceiptSha256: *h.LastReceiptSha256, HeadGeneration: h.Generation, Complete: true}
	raw := m.Encode()
	if c.ManifestBytes != uint64(len(raw)) {
		t.Fatal(c.ManifestBytes, len(raw))
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	write := func(name string, n uint64) {
		t.Helper()
		if e := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(n), Typeflag: tar.TypeReg, Format: tar.FormatPAX}); e != nil {
			t.Fatal(e)
		}
		if _, e := tw.Write(make([]byte, int(n))); e != nil {
			t.Fatal(e)
		}
	}
	write("manifest.json", uint64(len(raw)))
	for _, f := range files {
		write(f.Path, f.Bytes.Uint64())
	}
	if e = tw.Close(); e != nil {
		t.Fatal(e)
	}
	if c.TarBytes != uint64(buf.Len()) {
		t.Fatal(c.TarBytes, buf.Len())
	}
	before := in.Inventory.Files()
	p := Model(admin(Pause, "pause"), in)
	if p.Kind != "Transaction" {
		t.Fatal(p)
	}
	cost, e := CheckCapacity(p.Plan)
	if e != nil {
		t.Fatal(e)
	}
	if len(before) != len(in.Inventory.Files()) || len(cost.Final.Files()) != len(before)+3 {
		t.Fatal("immutable exact path transform")
	}
	// Directories are supplied and independently counted; intent files do not
	// consume state scan entries, while empty directories do.
	clone := in.Inventory.clone()
	clone.dirs["attempts"] = true
	after, e := measure(clone, h, 17, 31, 2)
	if e != nil {
		t.Fatal(e)
	}
	if after.ScannedEntries != c.ScannedEntries+1 || after.Files != c.Files {
		t.Fatal("D versus F")
	}
	if _, e = NewInventory(files, nil); e == nil {
		t.Fatal("missing directories")
	}
	f := files[0]
	f.Bytes = "18446744073709551615"
	if _, e = NewInventory([]archive.FileEntry{f}, in.Inventory.Directories()); e == nil {
		t.Fatal("oversized file")
	}
}
func paused(t *testing.T) Input {
	t.Helper()
	in, _ := initialized(t)
	r := Model(admin(Pause, "pause"), in)
	if r.Kind != "Transaction" {
		t.Fatal(r)
	}
	return commitModel(t, in, r.Plan)
}
func TestTMV0014_AS27_MissingShardBarrierPeakAndPostBeforeHead(t *testing.T) {
	in := paused(t)
	r := Model(admin(Unpause, "unpause"), in)
	if r.Kind != "Transaction" {
		t.Fatal(r)
	}
	cost, e := CheckCapacity(r.Plan)
	if e != nil {
		t.Fatal(e)
	}
	var receiptCount uint64
	postIndex, deleteIndex, headIndex := -1, -1, -1
	for j, p := range cost.Prefixes {
		if p.Step == "durable-receipt-link" {
			receiptCount = p.Cost.Files
		}
		if strings.HasPrefix(p.Step, "durable-post:requests/") {
			postIndex = j
			if p.Cost.Files != receiptCount+1 {
				t.Fatal("barrier retained while adding request")
			}
		}
		if p.Step == "durable-barrier-unlink" {
			deleteIndex = j
			if p.Cost.Files != receiptCount {
				t.Fatal("deletion charged at durable step")
			}
		}
		if p.Step == "durable-head" {
			headIndex = j
		}
	}
	if !(postIndex < deleteIndex && deleteIndex < headIndex) {
		t.Fatal("head ahead of a required post", postIndex, deleteIndex, headIndex)
	}
	h, _ := snapshot.DecodeHead(in.Head)
	peak, e := unpausePeak(in.Inventory, h, hardLimits())
	if e != nil {
		t.Fatal(e)
	}
	final := cost.Prefixes[len(cost.Prefixes)-1].Cost
	if peak.Files != final.Files+1 || peak.TemporaryBytes != 8538 {
		t.Fatal("retained barrier conservative envelope", peak, final)
	}
	// All 256 existing shard directories remove precisely one missing-directory
	// reserve charge; retained file/manifest/tar costs otherwise match.
	filled := in.Inventory.clone()
	for x := 0; x < 256; x++ {
		filled.dirs["requests/"+hexByte(x)] = true
	}
	all, e := unpausePeak(filled, h, hardLimits())
	if e != nil {
		t.Fatal(e)
	}
	baseDirectories := len(in.Inventory.dirs)
	if all.ScannedEntries-peak.ScannedEntries != uint64(len(filled.dirs)-baseDirectories-1) {
		t.Fatal("missing request shard")
	}
	// Private scaled limit exercises the same production journal arithmetic.
	l := hardLimits()
	l.journalCount = 3
	if _, e = checkCapacity(r.Plan, l); e != nil {
		t.Fatal("inside last slot UNPAUSE", e)
	}
	in = commitModel(t, in, r.Plan)
	newPause := Model(admin(Pause, "fresh-pause"), in)
	if newPause.Kind != "Transaction" {
		t.Fatal(newPause)
	}
	if _, e = checkCapacity(newPause.Plan, l); e == nil {
		t.Fatal("PAUSE cannot borrow nonexistent reserve")
	}
}
func hexByte(n int) string { const h = "0123456789abcdef"; return string([]byte{h[n/16], h[n%16]}) }
func TestTMV0014_AS27_NoShrinkCreditOrphanAccumulationAndAbort(t *testing.T) {
	in := paused(t)
	c := fixture.Ticket("T-1")
	discard := []byte(strings.Repeat("x", 70000))
	in = withTicket(t, in, c, discard)
	r := admin(KeepJournal, "keep")
	r.TargetID = c.TicketID.Raw
	r.CanonicalSha256 = c.FileDigest()
	r.File = discard
	result := Model(r, in)
	if result.Kind != "Transaction" {
		t.Fatal(result)
	}
	p := result.Plan
	cost, e := CheckCapacity(p)
	if e != nil {
		t.Fatal(e)
	}
	sawEvidence := false
	for _, pref := range cost.Prefixes {
		if pref.AbortToUnpause == nil {
			continue
		}
		if pref.AbortToUnpause.IntentBytes < uint64(len(discard)) {
			t.Fatal("abort credited shrink")
		}
		if strings.HasPrefix(pref.Step, "durable-link:evidence/") {
			sawEvidence = true
			if pref.AbortToUnpause.EvidenceBytes < uint64(len(discard)) {
				t.Fatal("orphan dropped")
			}
		}
	}
	if !sawEvidence {
		t.Fatal("missing abort evidence prefix")
	}
	final := cost.Prefixes[len(cost.Prefixes)-2].Cost
	l := hardLimits()
	l.intent = final.IntentBytes
	if _, e = checkCapacity(p, l); e == nil {
		t.Fatal("abandoned shrink funded transaction")
	}
	o := stageObservation(p)
	o.Inventory = p.base.clone()
	for _, a := range p.Artifacts() {
		if strings.HasPrefix(a.Target, "evidence/") {
			if e = o.Inventory.put(entry(a), true); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, a := range p.Artifacts() {
		o.Files = append(o.Files, StageFile{Name: a.Slot, Type: "REGULAR", Data: a.Data[:len(a.Data)/2]})
	}
	removed := []string{}
	for _, f := range o.Files {
		if f.Name != "active.json" {
			removed = append(removed, f.Name)
		}
	}
	for _, synced := range []bool{false, true} {
		a, e := AbortCapacity(o, CleanupState{Removed: removed, PayloadSynced: true, DescriptorUnlinked: true, DescriptorSynced: synced})
		if e != nil {
			t.Fatal(e)
		}
		if a.Unpause.IntentBytes < uint64(len(discard)) || a.Unpause.EvidenceBytes < uint64(len(discard)) {
			t.Fatal(a)
		}
		if synced != (a.Cleanup.FreshDescriptor == "MODEL_READY") {
			t.Fatal(a)
		}
	}
	// Descriptor-less malformed temp follows the selected abort-to-UNPAUSE path.
	o.Files = []StageFile{{Name: "active.json.tmp", Type: "REGULAR", Data: bytes.Repeat([]byte("!"), 2422)}}
	a, e := AbortCapacity(o, CleanupState{Removed: []string{"active.json.tmp"}})
	if e != nil || a.Current.TemporaryBytes != 2422 || a.Cleanup.FreshDescriptor != "HELD" {
		t.Fatal(a, e)
	}
	a, e = AbortCapacity(o, CleanupState{Removed: []string{"active.json.tmp"}, PayloadSynced: true})
	if e != nil || a.Current.TemporaryBytes != 0 || a.Cleanup.FreshDescriptor != "MODEL_READY" {
		t.Fatal(a, e)
	}
	// A second abandoned reconciliation adds another orphan; no cleanup removes
	// either. The physical projection remains the operator's edited bytes.
	second := []byte(strings.Repeat("y", 71000))
	o.Inventory.files["intent/tickets/T-1.json"] = bytesEntry("intent/tickets/T-1.json", second)
	orphan := "evidence/" + string(wire.Sum(second))
	if e = o.Inventory.put(bytesEntry(orphan, second), true); e != nil {
		t.Fatal(e)
	}
	a, e = AbortCapacity(o, CleanupState{Removed: []string{"active.json.tmp"}, PayloadSynced: true})
	if e != nil || a.Unpause.EvidenceBytes < uint64(len(discard)+len(second)) {
		t.Fatal(a, e)
	}
}
func TestTMV0014_AS27_AbortCleanupPermitsFreshUnpauseAcrossDivergence(t *testing.T) {
	for _, op := range []string{KeepJournal, AdoptFile} {
		t.Run(op, func(t *testing.T) {
			in := paused(t)
			canonical := fixture.Ticket("T-1")
			physical := []byte{}
			if op == AdoptFile {
				body := strings.Repeat("c", 65000)
				canonical.Body = &body
				offered := fixture.Ticket("T-1")
				offered.Body = &body
				offered.Title = "adopted title"
				physical = offered.Encode()
			}
			in.Inventory = in.Inventory.clone()
			selectedPath := "intent/tickets/" + canonical.TicketID.Local + ".json"
			in.Inventory.files[selectedPath] = bytesEntry(selectedPath, physical)
			in.CanonicalTickets = [][]byte{canonical.Encode()}

			r := admin(op, "abandoned-"+strings.ToLower(op))
			r.TargetID = canonical.TicketID.Raw
			r.File = physical
			if op == KeepJournal {
				r.CanonicalSha256 = canonical.FileDigest()
			}
			abandoned := Model(r, in)
			if abandoned.Kind != "Transaction" {
				t.Fatal("abandoned reconciliation", abandoned)
			}
			o := stageObservation(abandoned.Plan)
			o.Inventory = abandoned.Plan.base.clone()
			divergent := map[*ticket.Record][]byte{canonical: physical}
			for rec, projection := range map[*ticket.Record][]byte{
				fixture.Ticket("T-2"): {},
				fixture.Ticket("T-3"): []byte("unrelated malformed projection"),
			} {
				path := "intent/tickets/" + rec.TicketID.Local + ".json"
				o.Inventory.files[path] = bytesEntry(path, projection)
				in.CanonicalTickets = append(in.CanonicalTickets, rec.Encode())
				divergent[rec] = projection
			}
			sort.Slice(in.CanonicalTickets, func(i, j int) bool { return bytes.Compare(in.CanonicalTickets[i], in.CanonicalTickets[j]) < 0 })
			linkedOrphans := []archive.FileEntry{}
			removed := []string{}
			for _, a := range abandoned.Plan.Artifacts() {
				o.Files = append(o.Files, StageFile{Name: a.Slot, Type: "REGULAR", Data: a.Data[:len(a.Data)/2]})
				removed = append(removed, a.Slot)
				if strings.HasPrefix(a.Target, "evidence/") {
					orphan := entry(a)
					if e := o.Inventory.put(orphan, true); e != nil {
						t.Fatal(e)
					}
					linkedOrphans = append(linkedOrphans, orphan)
				}
			}
			if len(linkedOrphans) == 0 {
				t.Fatal("fixture did not publish an orphan")
			}
			abort, e := AbortCapacity(o, CleanupState{Removed: removed, PayloadSynced: true, DescriptorUnlinked: true, DescriptorSynced: true})
			if e != nil || abort.Cleanup.FreshDescriptor != "MODEL_READY" {
				t.Fatal("abort cleanup", abort, e)
			}

			in.Inventory = o.Inventory
			unpauseRequest := admin(Unpause, "fresh-unpause-"+strings.ToLower(op))
			unpause := Model(unpauseRequest, in)
			if unpause.Kind != "Transaction" {
				t.Fatal("fresh UNPAUSE after abort", unpause)
			}
			capacity, e := CheckCapacity(unpause.Plan)
			if e != nil {
				t.Fatal(e)
			}
			for rec, projection := range divergent {
				path := "intent/tickets/" + rec.TicketID.Local + ".json"
				if !capacity.Final.matches(path, projection) {
					t.Fatal("UNPAUSE changed divergent projection", path)
				}
			}
			for _, orphan := range linkedOrphans {
				if capacity.Final.files[orphan.Path] != orphan {
					t.Fatal("UNPAUSE dropped linked orphan", orphan.Path)
				}
			}

			in = commitModel(t, in, unpause.Plan)
			requestPath, _ := snapshot.RequestPath(unpauseRequest.RequestID)
			in.Replay = ReplayObservation{State: "FOUND", Record: unpause.Plan.posts[requestPath]}
			assertZero(t, Model(unpauseRequest, in), "Replay")
			in.Replay = ReplayObservation{State: "ABSENT"}
			unpauseRequest.RequestID += "-again"
			assertZero(t, Model(unpauseRequest, in), "NoChange")
		})
	}
}
func TestTMV0009_AS11_RedoExistingReceiptAndRetry(t *testing.T) {
	in := paused(t)
	r := admin(Unpause, "unpause")
	result := Model(r, in)
	if result.Kind != "Transaction" {
		t.Fatal(result)
	}
	p := result.Plan
	current := in.Inventory.clone()
	for _, a := range p.Artifacts() {
		if a.Role == "RECEIPT" {
			if e := current.put(entry(a), true); e != nil {
				t.Fatal(e)
			}
		}
	}
	redo, e := RedoCapacity(p, current)
	if e != nil {
		t.Fatal(e)
	}
	for _, pref := range redo.Prefixes {
		if pref.Cost.JournalCount != 3 {
			t.Fatal("redo minted receipt")
		}
	}
	expected, e := CheckCapacity(p)
	if e != nil {
		t.Fatal(e)
	}
	if len(redo.Final.Files()) != len(expected.Final.Files()) {
		t.Fatal("redo membership")
	}
	again, e := RedoCapacity(p, redo.Final)
	if e != nil {
		t.Fatal(e)
	}
	if again.Prefixes[0].Cost.JournalCount != 3 {
		t.Fatal("repeated redo")
	}
	corrupt := current.clone()
	corrupt.files["barrier.json"] = bytesEntry("barrier.json", []byte("third"))
	if _, e = RedoCapacity(p, corrupt); e == nil {
		t.Fatal("third-value deletion")
	}
	missing := current.clone()
	delete(missing.files, "receipts/000000000001.json")
	if _, e = RedoCapacity(p, missing); e == nil {
		t.Fatal("history deletion")
	}
	in = commitModel(t, in, p)
	rp, _ := snapshot.RequestPath(r.RequestID)
	in.Replay = ReplayObservation{State: "FOUND", Record: p.posts[rp]}
	assertZero(t, Model(r, in), "Replay")
	in.Replay = ReplayObservation{State: "ABSENT"}
	r.RequestID = "fresh"
	assertZero(t, Model(r, in), "NoChange")
}
func TestTMV0002_AS10_HeadDigitGrowthAndStageBinding(t *testing.T) {
	in := paused(t)
	h, _ := snapshot.DecodeHead(in.Head)
	h.LastSeq = "9"
	old := wire.EncodeFile(h.Value())
	h.LastSeq = "10"
	next := wire.EncodeFile(h.Value())
	if len(next) != len(old)+1 {
		t.Fatal("head digit width")
	}
	r := Model(admin(Unpause, "unpause"), in)
	o := stageObservation(r.Plan)
	o.Inventory = o.Inventory.clone()
	o.Inventory.files["receipts/000000000003.json"] = bytesEntry("receipts/000000000003.json", []byte("extra"))
	if _, e := ClassifyStage(o, r.Plan); e == nil {
		t.Fatal("extra receipt not absent")
	}
	o = stageObservation(r.Plan)
	o.CurrentHead = next
	if _, e := ClassifyStage(o, r.Plan); e == nil {
		t.Fatal("changed head binding")
	}
}

func TestTMV0009_AS11_HeadCannotAdvanceBeforeRequiredPost(t *testing.T) {
	in := paused(t)
	result := Model(admin(Unpause, "unpause"), in)
	p := result.Plan
	current := in.Inventory.clone()
	for _, a := range p.Artifacts() {
		if a.Role == "RECEIPT" || a.Role == "HEAD" || a.Role == "POST" {
			if e := current.put(entry(a), false); e != nil {
				t.Fatal(e)
			}
		}
	}
	if _, e := RedoCapacity(p, current); e == nil {
		t.Fatal("advanced head with barrier still present")
	}
	o := stageObservation(p)
	o.Inventory = current
	o.ReceiptInventory = "MATCHING_COMPLETED"
	o.LinkedReceipt = p.Receipt()
	o.CurrentHead = p.Head()
	o.Projections = "POST"
	if _, e := ClassifyStage(o, p); e == nil {
		t.Fatal("caller projection claim hid missing deletion")
	}
}

func TestTMV0014_AS27_ExhaustiveReconcileCleanupToUnpause(t *testing.T) {
	in := paused(t)
	c := fixture.Ticket("T-1")
	body := strings.Repeat("c", 65000)
	c.Body = &body
	discarded := []byte("malformed discarded projection")
	in = withTicket(t, in, c, discarded)
	r := admin(KeepJournal, "keep-large")
	r.TargetID = c.TicketID.Raw
	r.CanonicalSha256 = c.FileDigest()
	r.File = discarded
	result := Model(r, in)
	if result.Kind != "Transaction" {
		t.Fatal(result)
	}
	p := result.Plan
	if len(p.artifacts) != 6 {
		t.Fatal("maximal reconciliation slots", len(p.artifacts))
	}
	// Each subset of two new blobs is an interrupted evidence publication
	// prefix under some preparation order. Every original receipt stays present.
	evidence := []Artifact{}
	for _, a := range p.Artifacts() {
		if strings.HasPrefix(a.Target, "evidence/") {
			evidence = append(evidence, a)
		}
	}
	for linked := 0; linked < 1<<len(evidence); linked++ {
		inventory := p.base.clone()
		for j, a := range evidence {
			if linked&(1<<j) != 0 {
				if e := inventory.put(entry(a), true); e != nil {
					t.Fatal(e)
				}
			}
		}
		for mask := 0; mask < 1<<len(p.artifacts); mask++ {
			o := stageObservation(p)
			o.Inventory = inventory
			names := []string{}
			for j, a := range p.Artifacts() {
				if mask&(1<<j) != 0 {
					o.Files = append(o.Files, StageFile{Name: a.Slot, Type: "REGULAR", Data: a.Data[:len(a.Data)/2]})
					names = append(names, a.Slot)
				}
			}
			// Enumerate every removed subset, not just one deletion ordering.
			for removedMask := 0; removedMask < 1<<len(names); removedMask++ {
				removed := []string{}
				for j, name := range names {
					if removedMask&(1<<j) != 0 {
						removed = append(removed, name)
					}
				}
				a, e := AbortCapacity(o, CleanupState{Removed: removed})
				if e != nil {
					t.Fatal(linked, mask, removedMask, e)
				}
				if a.Cleanup.FreshDescriptor != "HELD" || a.Current.TemporaryBytes == 0 || a.Unpause.JournalCount != 3 {
					t.Fatal(a)
				}
			}
			for _, synced := range []bool{false, true} {
				a, e := AbortCapacity(o, CleanupState{Removed: names, PayloadSynced: true, DescriptorUnlinked: true, DescriptorSynced: synced})
				if e != nil {
					t.Fatal(e)
				}
				if synced != (a.Cleanup.FreshDescriptor == "MODEL_READY") {
					t.Fatal(a)
				}
				if a.Unpause.IntentBytes < uint64(len(discarded)) {
					t.Fatal("old projection lost")
				}
			}
		}
	}
}
