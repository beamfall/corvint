package journal

import (
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
	observation := PhysicalObservation{}
	r.writerCache, r.physical = true, &observation
	result, err := r.Audit()
	return result, observation, err
}
