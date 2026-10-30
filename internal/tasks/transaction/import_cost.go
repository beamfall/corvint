package transaction

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strconv"
	"strings"
	"sync/atomic"
)

// importCost belongs only to a process-local import sequence. Capacity forks
// share its owned inventory temporarily, record bounded undo, and never escape.
// encoding carries the CAL-V0-117 order-independent file-set cost of every
// listed file; its Files count is taken from cost.Files when measured.
// entryMeasurements counts the per-entry encoder measurements capacity makes,
// incremental or whole; JSG-V0-014 asserts it. Only tests read it.
var entryMeasurements atomic.Uint64

type importCost struct {
	cost        Cost
	encoding    archive.FileSetCost
	reserveNext [256]uint64
}

// remove uncharges an explicit file.
func (c *importCost) remove(old archive.FileEntry) error {
	return c.replace(old, true, archive.FileEntry{})
}

// replace charges next in place of old (when exists); a zero next path
// charges nothing, which remove uses.
func (c *importCost) replace(old archive.FileEntry, exists bool, next archive.FileEntry) error {
	contribution := func(f archive.FileEntry) (Cost, archive.FileSetCost, error) {
		if f.Path == "" {
			return Cost{}, archive.FileSetCost{}, nil
		}
		entryMeasurements.Add(1)
		enc, err := archive.EntryCost(f.Path, f.Bytes.Uint64())
		if err != nil {
			return Cost{}, enc, err
		}
		v := Cost{Files: 1}
		if strings.HasPrefix(f.Path, "intent/") {
			v.IntentBytes = f.Bytes.Uint64()
		} else {
			v.ScannedEntries = 1
		}
		if strings.HasPrefix(f.Path, "receipts/") {
			v.JournalCount = 1
			v.JournalBytes = f.Bytes.Uint64()
		}
		if strings.HasPrefix(f.Path, "evidence/") {
			v.EvidenceBytes = f.Bytes.Uint64()
		}
		return v, enc, nil
	}
	v, e, err := contribution(next)
	if err != nil {
		return err
	}
	prior := Cost{}
	pe := archive.FileSetCost{}
	if exists {
		prior, pe, err = contribution(old)
		if err != nil {
			return err
		}
	}
	copy := *c
	pairs := [][3]*uint64{{&copy.cost.Files, &prior.Files, &v.Files}, {&copy.cost.ScannedEntries, &prior.ScannedEntries, &v.ScannedEntries}, {&copy.cost.IntentBytes, &prior.IntentBytes, &v.IntentBytes}, {&copy.cost.JournalCount, &prior.JournalCount, &v.JournalCount}, {&copy.cost.JournalBytes, &prior.JournalBytes, &v.JournalBytes}, {&copy.cost.EvidenceBytes, &prior.EvidenceBytes, &v.EvidenceBytes}, {&copy.encoding.PayloadBytes, &pe.PayloadBytes, &e.PayloadBytes}, {&copy.encoding.EntryBytes, &pe.EntryBytes, &e.EntryBytes}, {&copy.encoding.TarBytes, &pe.TarBytes, &e.TarBytes}}
	for _, p := range pairs {
		if *p[0] < *p[1] {
			return malformed("incremental cost underflow")
		}
		*p[0], err = add(*p[0]-*p[1], *p[2])
		if err != nil {
			return err
		}
	}
	*c = copy
	return nil
}

func (c *importCost) measure(i *Inventory, h *snapshot.Head, temp, promised, names uint64) (Cost, error) {
	out := c.cost
	out.TemporaryBytes = temp
	out.PromisedBytes = promised
	var err error
	out.ScannedEntries, err = add(out.ScannedEntries, uint64(len(i.dirs)))
	if err != nil {
		return out, err
	}
	out.ScannedEntries, err = add(out.ScannedEntries, names)
	if err != nil {
		return out, err
	}
	if h == nil {
		return out, malformed("measurement requires hypothetical head metadata")
	}
	var barrier *wire.Digest
	if f, ok := i.files["barrier.json"]; ok {
		v := f.Sha256
		barrier = &v
	}
	// Digest text has fixed encoded width. This size-only placeholder is never
	// emitted as a manifest, journal identity, or content observation.
	m := archive.Manifest{QueueID: h.QueueID, ExportedAtSeq: wire.SizeOf(out.JournalCount), HeadSha256: wire.Sum(wire.EncodeFile(h.Value())), VersionSha256: h.VersionSha256, BarrierSha256: barrier, PrimaryWorktree: h.PrimaryWorktree, IntentTreeSha256: wire.Sum(nil), ReceiptCount: wire.SizeOf(out.JournalCount), LastReceiptSha256: *h.LastReceiptSha256, HeadGeneration: h.Generation, Complete: true}
	prefix := c.encoding
	prefix.Files = int(out.Files)
	enc, err := archive.MeasureManifestEncodingWith(&m, prefix)
	if err != nil {
		return out, err
	}
	out.PayloadBytes = enc.PayloadBytes
	out.ManifestBytes = enc.ManifestBytes
	out.TarBytes = enc.TarBytes
	return out, nil
}
func (i *Inventory) stageDirectory() {
	if i.undoDirs != nil {
		if _, ok := i.undoDirs["staging"]; !ok {
			i.undoDirs["staging"] = i.dirs["staging"]
		}
	}
	i.dirs["staging"] = true
}
func (i *Inventory) rollback() {
	for p, f := range i.undoFiles {
		if f == nil {
			delete(i.files, p)
		} else {
			i.files[p] = *f
		}
	}
	for p, exists := range i.undoDirs {
		if exists {
			i.dirs[p] = true
		} else {
			delete(i.dirs, p)
		}
	}
}

func (i *Inventory) advanceReserve(shard int) {
	for {
		candidate := fmt.Sprintf("requests/%02x/%02x%062x.json", shard, shard, i.incremental.reserveNext[shard])
		if _, exists := i.files[candidate]; !exists && !i.isElided(candidate) {
			return
		}
		i.incremental.reserveNext[shard]++
	}
}
func (i *Inventory) updateReserve(p string) {
	if strings.HasPrefix(p, "requests/") && len(p) > 12 {
		shard, err := strconv.ParseUint(p[9:11], 16, 8)
		if err == nil {
			i.advanceReserve(int(shard))
		}
	}
}
