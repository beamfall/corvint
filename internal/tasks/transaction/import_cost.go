package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
)

// importCost belongs only to a process-local import sequence. Capacity forks
// share its owned inventory temporarily, record bounded undo, and never escape.
type importCost struct {
	cost     Cost
	encoding archive.FileEncoding
}

func (c *importCost) replace(old archive.FileEntry, exists bool, next archive.FileEntry) error {
	contribution := func(f archive.FileEntry) (Cost, archive.FileEncoding, error) {
		enc, err := archive.MeasureFileEncoding(f)
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
	pe := archive.FileEncoding{}
	if exists {
		prior, pe, err = contribution(old)
		if err != nil {
			return err
		}
	}
	copy := *c
	pairs := [][3]*uint64{{&copy.cost.Files, &prior.Files, &v.Files}, {&copy.cost.ScannedEntries, &prior.ScannedEntries, &v.ScannedEntries}, {&copy.cost.IntentBytes, &prior.IntentBytes, &v.IntentBytes}, {&copy.cost.JournalCount, &prior.JournalCount, &v.JournalCount}, {&copy.cost.JournalBytes, &prior.JournalBytes, &v.JournalBytes}, {&copy.cost.EvidenceBytes, &prior.EvidenceBytes, &v.EvidenceBytes}, {&copy.encoding.Payload, &pe.Payload, &e.Payload}, {&copy.encoding.JSON, &pe.JSON, &e.JSON}, {&copy.encoding.Tar, &pe.Tar, &e.Tar}}
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
	enc, err := archive.MeasureManifestTotals(&m, int(out.Files), c.encoding)
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
