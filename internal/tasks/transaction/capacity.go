package transaction

import (
	"crypto/sha256"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const MaxReceipts uint64 = snapshot.MaxStageReceiptSeq
const MaxJournalBytes uint64 = 4 * wire.GiB

// Inventory privately owns complete supplied metadata, including actual state
// directories (without the state root). It cannot verify physical existence or
// authentication. Only regular retained files belong here; staging is separate.
type Inventory struct {
	files map[string]archive.FileEntry
	dirs  map[string]bool
}

func NewInventory(files []archive.FileEntry, stateDirectories []string) (*Inventory, error) {
	if len(files) > wire.MaxArchiveFiles || len(stateDirectories) > wire.MaxArchiveScanEntries {
		return nil, limit("inventory entries")
	}
	inv := &Inventory{files: map[string]archive.FileEntry{}, dirs: map[string]bool{}}
	for _, d := range stateDirectories {
		if !directory(d) || inv.dirs[d] {
			return nil, malformed("unknown/duplicate state directory")
		}
		inv.dirs[d] = true
	}
	for _, f := range files {
		if _, ok := inv.files[f.Path]; ok {
			return nil, malformed("duplicate inventory file")
		}
		bound, e := retainedBound(f.Path)
		if e != nil {
			return nil, e
		}
		n, e := wire.ParseSize(f.Path, string(f.Bytes))
		if e != nil {
			return nil, e
		}
		if n.Uint64() > bound {
			return nil, limit(f.Path)
		}
		if _, e = wire.ParseDigest(f.Path, string(f.Sha256)); e != nil {
			return nil, e
		}
		if strings.HasPrefix(f.Path, "evidence/") && f.Path != "evidence/"+string(f.Sha256) {
			return nil, malformed("evidence metadata identity")
		}
		if strings.HasPrefix(f.Path, "pinned/") && f.Path != "pinned/"+string(f.Sha256)+".json" {
			return nil, malformed("pinned metadata identity")
		}
		if !strings.HasPrefix(f.Path, "intent/") {
			parent := path.Dir(f.Path)
			if parent != "." && !inv.dirs[parent] {
				return nil, malformed("missing scanned parent")
			}
		}
		inv.files[f.Path] = f
	}
	for d := range inv.dirs {
		parent := path.Dir(d)
		if parent != "." && !inv.dirs[parent] {
			return nil, malformed("missing directory parent")
		}
	}
	scanned := uint64(len(inv.dirs))
	var intentBytes, tickets uint64
	for p, f := range inv.files {
		if strings.HasPrefix(p, "intent/") {
			var e error
			intentBytes, e = add(intentBytes, f.Bytes.Uint64())
			if e != nil {
				return nil, e
			}
		} else {
			scanned++
		}
		if strings.HasPrefix(p, "intent/tickets/") {
			tickets++
		}
	}
	if scanned > wire.MaxArchiveScanEntries || intentBytes > wire.MaxIntentTreeBytes || tickets > wire.MaxTicketsPerQueue {
		return nil, limit("inventory scan/intent bounds")
	}
	return inv, nil
}
func directory(d string) bool {
	switch d {
	case "receipts", "requests", "evidence", "pinned", "attempts", "effects", "worktrees", "staging":
		return true
	}
	if len(d) == 11 && strings.HasPrefix(d, "requests/") {
		_, e := wire.ParseDigest(d, strings.Repeat(strings.TrimPrefix(d, "requests/"), 32))
		return e == nil
	}
	return false
}
func retainedBound(p string) (uint64, error) {
	if p == "head.json" {
		return 4096, nil
	}
	if strings.HasPrefix(p, "receipts/") {
		name := strings.TrimPrefix(p, "receipts/")
		if len(name) != 17 || !strings.HasSuffix(name, ".json") {
			return 0, malformed("receipt filename")
		}
		var n uint64
		for _, c := range name[:12] {
			if c < '0' || c > '9' {
				return 0, malformed("receipt filename")
			}
			n = n*10 + uint64(c-'0')
		}
		if n < 1 || n > MaxReceipts {
			return 0, limit("receipt number")
		}
		return wire.MaxReceiptFileBytes, nil
	}
	if strings.HasPrefix(p, "attempts/") || strings.HasPrefix(p, "effects/") || strings.HasPrefix(p, "worktrees/") || p == "intent/import-map.json" {
		return 0, wire.Errorf(wire.CodeUnsupported, p, "outside no-runtime fixture subset")
	}
	n, e := snapshot.PostBound(p)
	return uint64(n), e
}
func (i *Inventory) Files() []archive.FileEntry {
	out := make([]archive.FileEntry, 0, len(i.files))
	for _, f := range i.files {
		out = append(out, f)
	}
	archive.SortFiles(out)
	return out
}
func (i *Inventory) Directories() []string {
	out := make([]string, 0, len(i.dirs))
	for d := range i.dirs {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
func (i *Inventory) matches(p string, raw []byte) bool {
	f, ok := i.files[p]
	return ok && f.Sha256 == wire.Sum(raw) && f.Bytes.Uint64() == uint64(len(raw))
}
func (i *Inventory) clone() *Inventory {
	out := &Inventory{files: map[string]archive.FileEntry{}, dirs: map[string]bool{}}
	for p, f := range i.files {
		out.files[p] = f
	}
	for d := range i.dirs {
		out.dirs[d] = true
	}
	return out
}
func (i *Inventory) chain(h *snapshot.Head) error {
	count := uint64(0)
	for p := range i.files {
		if strings.HasPrefix(p, "receipts/") {
			count++
		}
	}
	if count != h.LastSeq.Uint64() {
		return malformed("complete receipt inventory differs from head")
	}
	for n := uint64(1); n <= count; n++ {
		name, _ := snapshot.ReceiptName(n)
		if _, ok := i.files["receipts/"+name]; !ok {
			return malformed("receipt gap/extra")
		}
	}
	name, _ := snapshot.ReceiptName(count)
	if i.files["receipts/"+name].Sha256 != *h.LastReceiptSha256 {
		return malformed("head receipt digest")
	}
	if h.VersionSha256 != wire.Sum([]byte(snapshot.VersionBytes)) || !i.matches("VERSION", []byte(snapshot.VersionBytes)) {
		return malformed("VERSION binding")
	}
	first, _ := snapshot.ReceiptName(1)
	if i.files["receipts/"+first].Sha256 != h.InitSha256 {
		return malformed("genesis head binding")
	}
	return nil
}
func (i *Inventory) put(f archive.FileEntry, exclusive bool) error {
	old, exists := i.files[f.Path]
	if exists && exclusive {
		if old == f {
			return nil
		}
		return malformed("non-overwriting publication conflict")
	}
	i.files[f.Path] = f
	if !strings.HasPrefix(f.Path, "intent/") {
		for d := path.Dir(f.Path); d != "."; d = path.Dir(d) {
			i.dirs[d] = true
		}
	}
	return nil
}
func entry(a Artifact) archive.FileEntry {
	return archive.FileEntry{Path: a.Target, Sha256: a.Sha256, Bytes: a.Bytes}
}
func bytesEntry(p string, b []byte) archive.FileEntry {
	return archive.FileEntry{Path: p, Sha256: wire.Sum(b), Bytes: wire.SizeOf(uint64(len(b)))}
}

// Cost separates exact retained encoder cost from logical temporary/promised
// bytes. Pending-head measurements are hypothetical redo envelopes, not exports.
type Cost struct {
	Files, ScannedEntries, PayloadBytes, ManifestBytes, TarBytes uint64
	JournalCount, JournalBytes, EvidenceBytes, IntentBytes       uint64
	TemporaryBytes, PromisedBytes                                uint64
}
type limits struct{ files, scan, manifest, tar, journalCount, journalBytes, evidence, intent uint64 }

func hardLimits() limits {
	return limits{wire.MaxArchiveFiles, wire.MaxArchiveScanEntries, wire.MaxArchiveManifestBytes, wire.MaxArchiveBytes, MaxReceipts, MaxJournalBytes, wire.MaxEvidenceStoreBytes, wire.MaxIntentTreeBytes}
}
func add(a, b uint64) (uint64, error) {
	if b > math.MaxUint64-a {
		return 0, limit("uint64 overflow")
	}
	return a + b, nil
}
func check(c Cost, l limits) error {
	for _, v := range [][2]uint64{{c.Files, l.files}, {c.ScannedEntries, l.scan}, {c.ManifestBytes, l.manifest}, {c.TarBytes, l.tar}, {c.JournalCount, l.journalCount}, {c.JournalBytes, l.journalBytes}, {c.EvidenceBytes, l.evidence}, {c.IntentBytes, l.intent}} {
		if v[0] > v[1] {
			return limit("aggregate capacity")
		}
	}
	_, e := add(c.TemporaryBytes, c.PromisedBytes)
	return e
}
func journalReserve(c Cost, barrier bool, l limits) error {
	if !barrier {
		return check(c, l)
	}
	n, e := add(c.JournalCount, 1)
	if e != nil {
		return e
	}
	b, e := add(c.JournalBytes, UnpauseReceiptBytes)
	if e != nil {
		return e
	}
	if n > l.journalCount || b > l.journalBytes {
		return limit("barrier lacks inside-hard-caps UNPAUSE reserve")
	}
	return check(c, l)
}
func measure(i *Inventory, h *snapshot.Head, temporary, promised, stageNames uint64) (Cost, error) {
	c := Cost{TemporaryBytes: temporary, PromisedBytes: promised, ScannedEntries: uint64(len(i.dirs))}
	var e error
	intentHash := sha256.New()
	files := i.Files()
	for _, f := range files {
		n := f.Bytes.Uint64()
		if strings.HasPrefix(f.Path, "intent/") {
			c.IntentBytes, e = add(c.IntentBytes, n)
			if e != nil {
				return c, e
			}
			fmt.Fprintf(intentHash, "%s%c%s\n", strings.TrimPrefix(f.Path, "intent/"), byte(0), f.Sha256)
		} else {
			c.ScannedEntries, e = add(c.ScannedEntries, 1)
			if e != nil {
				return c, e
			}
		}
		if strings.HasPrefix(f.Path, "receipts/") {
			c.JournalCount++
			c.JournalBytes, e = add(c.JournalBytes, n)
			if e != nil {
				return c, e
			}
		}
		if strings.HasPrefix(f.Path, "evidence/") {
			c.EvidenceBytes, e = add(c.EvidenceBytes, n)
			if e != nil {
				return c, e
			}
		}
	}
	c.ScannedEntries, e = add(c.ScannedEntries, stageNames)
	if e != nil {
		return c, e
	}
	if h == nil {
		return c, malformed("measurement requires hypothetical head metadata")
	}
	last := *h.LastReceiptSha256
	var barrier *wire.Digest
	if f, ok := i.files["barrier.json"]; ok {
		d := f.Sha256
		barrier = &d
	}
	m := archive.Manifest{QueueID: h.QueueID, ExportedAtSeq: wire.SizeOf(c.JournalCount), HeadSha256: wire.Sum(wire.EncodeFile(h.Value())), VersionSha256: h.VersionSha256, BarrierSha256: barrier, PrimaryWorktree: h.PrimaryWorktree, IntentTreeSha256: wire.Digest(fmt.Sprintf("%x", intentHash.Sum(nil))), Files: files, ReceiptCount: wire.SizeOf(c.JournalCount), LastReceiptSha256: last, HeadGeneration: h.Generation, Complete: true}
	enc, e := archive.MeasureManifestEncoding(&m)
	if e != nil {
		return c, e
	}
	c.Files = uint64(enc.Files)
	c.PayloadBytes = enc.PayloadBytes
	c.ManifestBytes = enc.ManifestBytes
	c.TarBytes = enc.TarBytes
	return c, nil
}

// Prefix is a bounded durable publication point. Before a listed durable point,
// the preceding cost remains charged; shrink/unlink cannot finance its producer.
type Prefix struct {
	Step           string
	Cost           Cost
	AbortToUnpause *Cost
}
type Capacity struct {
	Prefixes []Prefix
	Final    *Inventory
	Coverage Coverage
}

func CheckCapacity(p *Plan) (Capacity, error) { return checkCapacity(p, hardLimits()) }
func checkCapacity(p *Plan, l limits) (Capacity, error) {
	out := Capacity{Coverage: coverage()}
	if p == nil {
		return out, malformed("missing frozen plan")
	}
	h, e := snapshot.DecodeHead(p.head)
	if e != nil {
		return out, e
	}
	working := p.base.clone()
	working.dirs["staging"] = true
	baseHead := p.baseHead
	if baseHead == nil {
		baseHead = h
	}
	baseCost, e := measure(working, baseHead, 0, 0, 0)
	if e != nil {
		return out, e
	}
	_, barrierBefore := working.files["barrier.json"]
	if e = journalReserve(baseCost, barrierBefore, l); e != nil {
		return out, e
	}
	promised := uint64(0)
	for _, a := range p.artifacts {
		promised, e = add(promised, a.Bytes.Uint64())
		if e != nil {
			return out, e
		}
	}
	temp, e := add(promised, 2*uint64(len(p.descriptor)))
	if e != nil {
		return out, e
	}
	stageNames := uint64(len(p.artifacts) + 2)
	record := func(step string, precommit bool) error {
		c, e := measure(working, h, temp, promised, stageNames)
		if e != nil {
			return e
		}
		if e = check(c, l); e != nil {
			return e
		}
		pref := Prefix{Step: step, Cost: c}
		if precommit && barrierBefore {
			// All cleanup subsets are dominated by this maximal stage cost. After
			// durable cleanup, UNPAUSE starts from these retained files, including
			// every linked orphan and the unchanged old projection.
			u, e := unpausePeak(working, baseHead, l)
			if e != nil {
				return e
			}
			pref.AbortToUnpause = &u
		}
		out.Prefixes = append(out.Prefixes, pref)
		return nil
	}
	if e = record("descriptor-and-all-promised-artifacts", true); e != nil {
		return out, e
	}
	// Evidence POST is the one cross-role dedup. All evidence publishes before
	// receipt; only it may survive an abandoned preparation.
	for _, a := range p.artifacts {
		if !strings.HasPrefix(a.Target, "evidence/") {
			continue
		}
		if e = working.put(entry(a), true); e != nil {
			return out, e
		}
		if e = record("durable-link:"+a.Target, true); e != nil {
			return out, e
		}
	}
	for _, a := range p.artifacts {
		if a.Role == "RECEIPT" {
			if _, ok := working.files[a.Target]; ok {
				return out, malformed("receipt already linked")
			}
			if e = working.put(entry(a), true); e != nil {
				return out, e
			}
		}
	}
	if e = record("durable-receipt-link", false); e != nil {
		return out, e
	}
	// Additions before shrinking replacements/deletions covers the maximal
	// retained peak even if a future publisher chooses a different post order.
	ordered := []Artifact{}
	for _, a := range p.artifacts {
		if a.Role == "POST" && !strings.HasPrefix(a.Target, "evidence/") {
			ordered = append(ordered, a)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		oldI := working.files[ordered[i].Target].Bytes.Uint64()
		oldJ := working.files[ordered[j].Target].Bytes.Uint64()
		growI := ordered[i].Bytes.Uint64() >= oldI
		growJ := ordered[j].Bytes.Uint64() >= oldJ
		if growI != growJ {
			return growI
		}
		return ordered[i].Target < ordered[j].Target
	})
	for _, a := range ordered {
		if e = working.put(entry(a), strings.HasPrefix(a.Target, "requests/") || strings.HasPrefix(a.Target, "pinned/")); e != nil {
			return out, e
		}
		if e = record("durable-post:"+a.Target, false); e != nil {
			return out, e
		}
	}
	if raw, ok := p.posts["barrier.json"]; ok && raw == nil {
		delete(working.files, "barrier.json")
		if e = record("durable-barrier-unlink", false); e != nil {
			return out, e
		}
	}
	if e = working.put(bytesEntry("head.json", p.head), false); e != nil {
		return out, e
	}
	if e = record("durable-head", false); e != nil {
		return out, e
	}
	temp, promised, stageNames = 0, 0, 0
	if e = record("durable-staging-cleanup", false); e != nil {
		return out, e
	}
	finalCost := out.Prefixes[len(out.Prefixes)-1].Cost
	_, barrierAfter := working.files["barrier.json"]
	if e = journalReserve(finalCost, barrierAfter, l); e != nil {
		return out, e
	}
	if barrierAfter {
		u, e := unpausePeak(working, h, l)
		if e != nil {
			return out, e
		}
		out.Prefixes = append(out.Prefixes, Prefix{Step: "reserved-fresh-UNPAUSE", Cost: u})
	}
	out.Final = working
	return out, nil
}

// unpausePeak uses the codec-proved maximal receipt/index promises and exact
// derived head encoding. Request paths all have equal wire/PAX size; choose an
// absent shard if available, then an unoccupied fixed-length digest path.
func unpausePeak(base *Inventory, h *snapshot.Head, l limits) (Cost, error) {
	i := base.clone()
	i.dirs["staging"] = true
	c, e := measure(i, h, 0, 0, 0)
	if e != nil {
		return c, e
	}
	if e = journalReserve(c, true, l); e != nil {
		return c, e
	}
	n, e := add(h.LastSeq.Uint64(), 1)
	if e != nil {
		return c, e
	}
	if n > MaxReceipts {
		return c, limit("UNPAUSE sequence")
	}
	name, _ := snapshot.ReceiptName(n)
	hash := wire.Sum([]byte("hypothetical reserved bytes; not authentication"))
	if _, ok := i.files["receipts/"+name]; ok {
		return c, malformed("reserve receipt occupied")
	}
	if e = i.put(archive.FileEntry{Path: "receipts/" + name, Sha256: hash, Bytes: wire.SizeOf(UnpauseReceiptBytes)}, true); e != nil {
		return c, e
	}
	shard := "00"
	for x := 0; x < 256; x++ {
		candidate := fmt.Sprintf("%02x", x)
		if !i.dirs["requests/"+candidate] {
			shard = candidate
			break
		}
	}
	requestPath := ""
	for x := 0; x <= len(i.files); x++ {
		candidate := "requests/" + shard + "/" + shard + fmt.Sprintf("%062x", x) + ".json"
		if _, ok := i.files[candidate]; !ok {
			requestPath = candidate
			break
		}
	}
	if requestPath == "" {
		return c, limit("reserve request path")
	}
	if e = i.put(archive.FileEntry{Path: requestPath, Sha256: hash, Bytes: wire.SizeOf(UnpauseIndexBytes)}, true); e != nil {
		return c, e
	}
	next := *h
	next.LastSeq = wire.SizeOf(n)
	next.LastReceiptSha256 = &hash
	head := wire.EncodeFile(next.Value())
	if _, e = snapshot.DecodeHead(head); e != nil {
		return c, e
	}
	// Charge the old barrier and whichever head is larger until replacement
	// durability; this includes decimal head growth and never credits shrink.
	old := i.files["head.json"]
	if uint64(len(head)) >= old.Bytes.Uint64() {
		if e = i.put(bytesEntry("head.json", head), false); e != nil {
			return c, e
		}
	}
	c, e = measure(i, &next, UnpauseTemporaryBytes, 6342, 5)
	if e != nil {
		return c, e
	}
	if e = check(c, l); e != nil {
		return c, e
	}
	return c, nil
}
