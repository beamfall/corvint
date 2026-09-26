package snapshot

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
)

const MaxStageChildren = 13

// StageFile is a present regular child, including present-empty bytes. Native
// callers establish type, bounded enumeration and read lifetime before calling.
type StageFile struct {
	Name string
	Raw  []byte
}

// StageObservation describes structure only. It grants no cleanup or execution.
type StageObservation struct {
	Descriptor *StageDescriptor
	Files      map[string]wire.Digest
}

func StageName(name string) bool {
	if name == "active.json" || name == "active.json.tmp" {
		return true
	}
	for i := 0; i < 11; i++ {
		if name == stageSlot(i) {
			return true
		}
	}
	return false
}

func ObserveStage(files []StageFile) (*StageObservation, error) {
	if len(files) > MaxStageChildren {
		return nil, stageLimit("stage children")
	}
	raw := map[string][]byte{}
	o := &StageObservation{Files: map[string]wire.Digest{}}
	for _, f := range files {
		if !StageName(f.Name) {
			return nil, stageMalformed("unexpected child")
		}
		if _, ok := raw[f.Name]; ok {
			return nil, stageMalformed("duplicate child")
		}
		raw[f.Name] = f.Raw
	}
	active, ok := raw["active.json"]
	if ok {
		d, e := DecodeStageDescriptor(active)
		if e != nil {
			return nil, e
		}
		o.Descriptor = d
	}
	cap := MaxStageDescriptorBytes
	if ok {
		cap = len(active)
	}
	if temp, present := raw["active.json.tmp"]; present {
		if len(temp) > cap {
			return nil, stageLimit("descriptor temp exceeds actual bound")
		}
		if ok && len(temp) == len(active) && !bytes.Equal(temp, active) {
			return nil, stageFork("complete temp differs from active")
		}
	}
	assigned := map[string]StageDescription{}
	if o.Descriptor != nil {
		for _, a := range o.Descriptor.Artifacts {
			assigned[a.Slot] = a
		}
	}
	for name, b := range raw {
		if name == "active.json" || name == "active.json.tmp" {
			continue
		}
		a, ok := assigned[name]
		if !ok {
			return nil, stageMalformed("unassigned payload slot")
		}
		n := a.Bytes.Uint64()
		if uint64(len(b)) > n {
			return nil, stageLimit("payload exceeds declared bytes")
		}
		if uint64(len(b)) == n && wire.Sum(b) != a.Sha256 {
			return nil, stageFork("complete payload digest differs")
		}
	}
	for _, f := range files {
		o.Files[f.Name] = wire.Sum(f.Raw)
	}
	return o, nil
}

// StageBinding uses actual current bytes. RequestRaw is the receipt's retained
// request afterimage (inline or bounded blob); callers verify blob identity.
// Receipt inventory/pending precedence remains with each command's existing check.
type StageBinding struct {
	QueueID                                   string
	QueueRaw, HeadRaw, ReceiptRaw, RequestRaw []byte
}

func (o *StageObservation) Bind(b StageBinding) error {
	if o == nil || len(o.Files) == 0 {
		return nil
	}
	q, e := intent.DecodeQueue(b.QueueRaw)
	if e != nil {
		return e
	}
	if q.QueueID.Raw != b.QueueID {
		return stageFork("observed queue differs")
	}
	if !q.Fixture {
		return wire.Errorf(wire.CodeUnsupported, "staging", "nonempty staging is fixture-only")
	}
	d := o.Descriptor
	if d == nil {
		return nil
	}
	if d.QueueID != b.QueueID {
		return stageFork("descriptor queue differs")
	}
	if b.HeadRaw == nil {
		if d.Base == nil {
			return nil
		}
		return stageFork("base head absent")
	}
	h, e := DecodeHead(b.HeadRaw)
	if e != nil {
		return e
	}
	if h.QueueID.Raw != b.QueueID {
		return stageFork("head queue differs")
	}
	if d.Base != nil && h.LastSeq == d.Base.LastSeq && *h.LastReceiptSha256 == d.Base.LastReceiptSha256 {
		return nil
	}
	seq := uint64(1)
	if d.Base != nil {
		seq = d.Base.LastSeq.Uint64() + 1
	}
	if h.LastSeq.Uint64() != seq {
		return stageFork("stale descriptor base")
	}
	var head, receipt *StageDescription
	for i := range d.Artifacts {
		a := &d.Artifacts[i]
		if a.Role == "HEAD" {
			head = a
		}
		if a.Role == "RECEIPT" {
			receipt = a
		}
	}
	if head == nil || receipt == nil || uint64(len(b.HeadRaw)) != head.Bytes.Uint64() || uint64(len(b.ReceiptRaw)) != receipt.Bytes.Uint64() || wire.Sum(b.HeadRaw) != head.Sha256 || wire.Sum(b.ReceiptRaw) != receipt.Sha256 || *h.LastReceiptSha256 != receipt.Sha256 {
		return stageFork("completed head/receipt bytes unavailable or different")
	}
	rc, e := DecodeReceipt(b.ReceiptRaw)
	if e != nil {
		return e
	}
	kind := d.Operation
	if kind == StageKeepJournal || kind == StageAdoptFile {
		kind = "RECONCILE"
	}
	if rc.Kind != kind || rc.RecordedAt != d.RecordedAt {
		return stageFork("completed operation or timestamp differs")
	}
	if rc.Seq != h.LastSeq || rc.HeadGeneration != h.Generation || rc.RequestID == nil || *rc.RequestID != d.RequestID {
		return stageFork("completed receipt binding")
	}
	if d.Base == nil {
		if rc.Prev != nil {
			return stageFork("genesis prev")
		}
	} else if rc.Prev == nil || *rc.Prev != d.Base.LastReceiptSha256 {
		return stageFork("completed receipt prev")
	}
	requestPath, _ := RequestPath(d.RequestID)
	var post *PostEntry
	for i := range rc.Post {
		if rc.Post[i].Path == requestPath {
			post = &rc.Post[i]
		}
	}
	var requestArtifact *StageDescription
	for i := range d.Artifacts {
		a := &d.Artifacts[i]
		if a.Role == "POST" && a.Target == requestPath {
			requestArtifact = a
		}
	}
	if requestArtifact == nil || requestArtifact.Bytes.Uint64() != uint64(len(b.RequestRaw)) || requestArtifact.Sha256 != wire.Sum(b.RequestRaw) {
		return stageFork("completed request artifact differs")
	}
	if post == nil || post.Sha256 == nil || wire.Sum(b.RequestRaw) != *post.Sha256 {
		return stageFork("request afterimage proof absent")
	}
	req, e := DecodeRequest(b.RequestRaw)
	if e != nil {
		return e
	}
	if req.Entry.RequestID != d.RequestID || req.Seq != rc.Seq || req.Entry.MutationSha256 != d.RequestSha256 || req.Entry.Outcome.Outcome != rc.Outcome || strings.Join(req.Entry.Outcome.Codes, "\x00") != strings.Join(rc.Codes, "\x00") {
		return stageFork("completed request binding")
	}
	return nil
}
func stageFork(detail string) error {
	return wire.Errorf(wire.CodeJournalForked, "staging", "%s", detail)
}
