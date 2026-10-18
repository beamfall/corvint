package journal

import (
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PhysicalFile describes bytes actually consumed from one native path, never
// an inline receipt afterimage. No file body or descriptor is retained.
type PhysicalFile struct {
	Sha256 wire.Digest
	Bytes  int
}

// PhysicalObservation is local to one writer audit. Files is published only
// after a settled full observation and successful native closure. Cleanup is
// independent of the ordinary error so callers cannot retry past failed close.
type PhysicalObservation struct {
	Files   map[string]PhysicalFile
	Cleanup error
	// Walks counts the journal walks this audit ran and their wall time, a
	// diagnostic for the lease timing split (PSR-V0-151). It decides nothing.
	Walks WalkStats
	// capture is the settled attempt's first capture, published with Files;
	// Result.ResumeBase binds a later resumed audit to its stamps.
	capture *observation
}

type physicalReads struct {
	files map[string]PhysicalFile
	limit int
}

func newPhysicalReads() *physicalReads {
	return &physicalReads{files: make(map[string]PhysicalFile), limit: wire.MaxArchiveScanEntries + intent.MaxIntentRootEntries + wire.MaxTicketsPerQueue + wire.MaxReleasesPerQueue}
}

func (p *physicalReads) add(path string, raw []byte) error {
	file := PhysicalFile{Sha256: wire.Sum(raw), Bytes: len(raw)}
	if previous, ok := p.files[path]; ok {
		if previous != file {
			return moved(path)
		}
		return nil
	}
	if len(p.files) >= p.limit {
		return wire.Errorf(wire.CodeLimitExceeded, path, "physical observation entry bound")
	}
	p.files[path] = file
	return nil
}

// AuditForWriteObserved preserves AuditForWrite's full validation and also
// returns bounded physical metadata for same-guard inventory assembly. It is
// not a cross-command cache or authority. Ordinary readers do not collect it.
func (r Reader) AuditForWriteObserved() (*Result, PhysicalObservation, error) {
	if r.PrefixFallback {
		return r.auditPrefixFallback()
	}
	observation := PhysicalObservation{}
	var walks WalkStats
	r.writerCache, r.physical, r.walks = true, &observation, &walks
	if r.StopOnMove {
		switch r.Source.(type) {
		case Native, *Native:
			r.fullAttempts, r.abortMoved = 1, true
		}
	}
	result, err := r.Audit()
	observation.Walks = walks
	return result, observation, err
}

// WalkStats is one audit's walk accounting (PSR-V0-151): for
// AuditForWriteObserved, complete attempts (ModeFull, from receipt 1) and
// prefix walks (ModePrefix, to the head a complete attempt captured), how
// many of each ended SNAPSHOT_MOVED, and their summed wall time; for
// AuditForResume, each attempt it ran from the base, a SNAPSHOT_MOVED retry
// included, and their summed wall time.
type WalkStats struct {
	Complete      int
	CompleteMoved int
	Prefix        int
	PrefixMoved   int
	Resumed       int
	CompleteTime  time.Duration
	PrefixTime    time.Duration
	ResumedTime   time.Duration
}

func (w *WalkStats) record(prefix, resumed bool, took time.Duration, moved bool) {
	if resumed {
		w.Resumed++
		w.ResumedTime += took
		return
	}
	if prefix {
		w.Prefix++
		w.PrefixTime += took
		if moved {
			w.PrefixMoved++
		}
		return
	}
	w.Complete++
	w.CompleteTime += took
	if moved {
		w.CompleteMoved++
	}
}
