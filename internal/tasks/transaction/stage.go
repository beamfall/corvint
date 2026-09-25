package transaction

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
)

const (
	MaxStageDescriptorBytes = snapshot.MaxStageDescriptorBytes
	UnpauseReceiptBytes     = snapshot.UnpauseReceiptBytes
	UnpauseIndexBytes       = snapshot.UnpauseIndexBytes
	UnpauseOutcomeBytes     = snapshot.UnpauseOutcomeBytes
	UnpauseTemporaryBytes   = snapshot.UnpauseTemporaryBytes
)

type Base = snapshot.StageBase
type Description = snapshot.StageDescription
type Descriptor = snapshot.StageDescriptor

func DecodeDescriptor(raw []byte) (*Descriptor, error) { return snapshot.DecodeStageDescriptor(raw) }
func stageLimits(op string) (int, int)                 { return snapshot.StageLimits(op) }

type Artifact struct {
	Description
	Data []byte
}

func newArtifact(role, path string, raw []byte) Artifact {
	return Artifact{Description: Description{Role: role, Target: path, Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))}, Data: bytes.Clone(raw)}
}
func cloneArtifacts(a []Artifact) []Artifact {
	out := append([]Artifact(nil), a...)
	for i := range out {
		out[i].Data = bytes.Clone(out[i].Data)
	}
	return out
}
func slotName(i int) string { return fmt.Sprintf("a%02d", i) }

// StageFile is a supplied bounded regular-file observation. Type is exactly
// REGULAR; absent entries are omitted. It conveys no physical ownership proof.
type StageFile struct {
	Name, Type string
	Data       []byte
}
type StageObservation struct {
	Inventory        *Inventory // complete hypothetical retained metadata, never physical observation
	Files            []StageFile
	BaseState        string // UNCHANGED only
	ReceiptInventory string // COMPLETE_NO_NEXT, MATCHING_NEXT, MATCHING_COMPLETED
	Premise          string
	// LinkedReceipt and CurrentHead bind pending/completed classifications.
	LinkedReceipt, CurrentHead []byte
	Projections                string // PRE_OR_POST for pending; POST for completed
}
type StageResult struct {
	Kind           string
	Coverage       Coverage
	TemporaryBytes uint64
	Entries        uint64
}

// ClassifyStage recognizes preparation and redo conditionally. A descriptor is
// never a full plan; MATCHING_NEXT additionally requires the supplied frozen plan.
func ClassifyStage(o StageObservation, p *Plan) (StageResult, error) {
	result := StageResult{Coverage: coverage()}
	if (o.Premise != FixtureNoRuntime && o.Premise != LocalOperator) || o.BaseState != "UNCHANGED" {
		return result, malformed("stage base/scope premise")
	}
	if e := stageBase(o, p); e != nil {
		return result, e
	}
	if len(o.Files) > 13 {
		return result, limit("stage entries")
	}
	files := map[string]StageFile{}
	for _, f := range o.Files {
		if _, ok := files[f.Name]; ok {
			return result, malformed("duplicate stage name")
		}
		if f.Type != "REGULAR" {
			return result, malformed("stage nonregular")
		}
		valid := f.Name == "active.json" || f.Name == "active.json.tmp"
		for i := 0; i < 11; i++ {
			valid = valid || f.Name == slotName(i)
		}
		if !valid {
			return result, malformed("unknown staging name")
		}
		files[f.Name] = f
		var e error
		result.TemporaryBytes, e = add(result.TemporaryBytes, uint64(len(f.Data)))
		if e != nil {
			return result, e
		}
	}
	result.Entries = uint64(len(files)) + 1
	active, ok := files["active.json"]
	if !ok {
		if o.ReceiptInventory != "COMPLETE_NO_NEXT" {
			return result, malformed("temp-only receipt inventory")
		}
		if len(files) == 0 {
			result.Kind = "EmptyPreparation"
			return result, nil
		}
		temp, ok := files["active.json.tmp"]
		if !ok || len(files) != 1 || len(temp.Data) > MaxStageDescriptorBytes {
			return result, malformed("descriptor-less artifact or oversized temp")
		}
		result.Kind = "TempPreparationAbortable"
		return result, nil
	}
	d, e := DecodeDescriptor(active.Data)
	if e != nil {
		return result, e
	}
	if o.ReceiptInventory == "COMPLETE_NO_NEXT" {
		if d.Base == nil {
			if len(o.CurrentHead) != 0 {
				return result, malformed("genesis descriptor on initialized base")
			}
		} else {
			h, e := snapshot.DecodeHead(o.CurrentHead)
			if e != nil {
				return result, e
			}
			if h.QueueID.Raw != d.QueueID || h.LastSeq != d.Base.LastSeq || *h.LastReceiptSha256 != d.Base.LastReceiptSha256 {
				return result, malformed("descriptor base identity")
			}
		}
	}
	if temp, ok := files["active.json.tmp"]; ok {
		if len(temp.Data) > len(active.Data) {
			return result, limit("owned descriptor temp exceeds frozen descriptor")
		}
		if len(temp.Data) == len(active.Data) && !bytes.Equal(temp.Data, active.Data) {
			return result, wire.Errorf(wire.CodeJournalForked, "active.json.tmp", "complete descriptor temp differs")
		}
	}
	complete := true
	for _, a := range d.Artifacts {
		f, ok := files[a.Slot]
		if !ok {
			complete = false
			continue
		}
		if uint64(len(f.Data)) > a.Bytes.Uint64() {
			return result, limit("partial artifact too long")
		}
		if uint64(len(f.Data)) < a.Bytes.Uint64() {
			complete = false
			continue
		}
		if wire.Sum(f.Data) != a.Sha256 {
			return result, wire.Errorf(wire.CodeJournalForked, a.Slot, "complete corrupt artifact")
		}
	}
	for name := range files {
		if name == "active.json" || name == "active.json.tmp" {
			continue
		}
		found := false
		for _, a := range d.Artifacts {
			found = found || name == a.Slot
		}
		if !found {
			return result, malformed("unassigned slot")
		}
	}
	if p != nil && !bytes.Equal(p.descriptor, active.Data) {
		return result, malformed("frozen plan descriptor differs")
	}
	switch o.ReceiptInventory {
	case "COMPLETE_NO_NEXT":
		result.Kind = "PreparationPending"
		if complete && p != nil {
			result.Kind = "FullPlanPrepared"
		}
	case "MATCHING_NEXT", "MATCHING_COMPLETED":
		if p == nil || !bytes.Equal(o.LinkedReceipt, p.receipt) {
			return result, malformed("linked receipt is not the frozen plan")
		}
		if o.ReceiptInventory == "MATCHING_NEXT" {
			if p.baseHead == nil {
				if len(o.CurrentHead) != 0 {
					return result, malformed("pending genesis head")
				}
			} else if !bytes.Equal(o.CurrentHead, wire.EncodeFile(p.baseHead.Value())) {
				return result, malformed("pending head")
			}
			if o.Projections != "PRE_OR_POST" {
				return result, malformed("pending projections")
			}
			result.Kind = "RedoPending"
		} else {
			if !bytes.Equal(o.CurrentHead, p.head) || o.Projections != "POST" {
				return result, malformed("completed head/projections")
			}
			result.Kind = "CompletedCleanup"
		}
	default:
		return result, malformed("unknown receipt inventory")
	}
	return result, nil
}

// CleanupCost keeps names/bytes and promised afterimages charged until their
// respective directory fsync. Removed is a subset of existing payload/temp
// names; descriptor removal is ordered after payload fsync. No I/O is performed.
type CleanupState struct {
	Removed                                             []string
	PayloadSynced, DescriptorUnlinked, DescriptorSynced bool
}
type CleanupResult struct {
	TemporaryBytes, Entries uint64
	PromisedBytes           uint64
	FreshDescriptor         string
	Coverage                Coverage
}

func Cleanup(o StageObservation, c CleanupState) (CleanupResult, error) {
	classified, e := ClassifyStage(o, nil)
	out := CleanupResult{FreshDescriptor: "HELD", Coverage: coverage()}
	if e != nil {
		return out, e
	}
	if o.ReceiptInventory != "COMPLETE_NO_NEXT" {
		return out, malformed("precommit cleanup only")
	}
	tempOnly := classified.Kind == "TempPreparationAbortable" || classified.Kind == "EmptyPreparation"
	files := map[string]StageFile{}
	for _, f := range o.Files {
		files[f.Name] = f
	}
	removed := map[string]bool{}
	for _, n := range c.Removed {
		if n == "active.json" || removed[n] {
			return out, malformed("cleanup name/order")
		}
		if _, ok := files[n]; !ok {
			return out, malformed("cleanup unowned name")
		}
		removed[n] = true
	}
	for name := range files {
		if name != "active.json" && !removed[name] && c.PayloadSynced {
			return out, malformed("payload sync before all payload removal")
		}
	}
	if c.DescriptorUnlinked && (!c.PayloadSynced || tempOnly) {
		return out, malformed("descriptor unlink order")
	}
	if c.DescriptorSynced && !c.DescriptorUnlinked {
		return out, malformed("descriptor fsync order")
	}
	out.Entries = 1
	for _, f := range files {
		durableRemoved := removed[f.Name] && c.PayloadSynced
		if f.Name == "active.json" {
			durableRemoved = c.DescriptorSynced
		}
		if durableRemoved {
			continue
		}
		out.Entries++
		out.TemporaryBytes += uint64(len(f.Data))
	}
	if !tempOnly && !c.DescriptorSynced {
		d, _ := DecodeDescriptor(files["active.json"].Data)
		for _, a := range d.Artifacts {
			out.PromisedBytes, e = add(out.PromisedBytes, a.Bytes.Uint64())
			if e != nil {
				return out, e
			}
		}
	}
	if (tempOnly && c.PayloadSynced) || c.DescriptorSynced {
		out.FreshDescriptor = "MODEL_READY"
	}
	return out, nil
}

func stageBase(o StageObservation, p *Plan) error {
	if o.Inventory == nil {
		return malformed("complete stage inventory required")
	}
	if o.ReceiptInventory != "COMPLETE_NO_NEXT" {
		if p == nil {
			return malformed("pending/completed stage requires frozen plan")
		}
		if len(o.CurrentHead) == 0 {
			if _, ok := o.Inventory.files["head.json"]; ok {
				return malformed("head observation absent")
			}
		} else if !o.Inventory.matches("head.json", o.CurrentHead) {
			return malformed("head observation differs from inventory")
		}
		return validateRedoInventory(p, o.Inventory)
	}
	if len(o.CurrentHead) == 0 {
		for path := range o.Inventory.files {
			if !strings.HasPrefix(path, "evidence/") {
				return malformed("headless preparation has non-evidence state")
			}
		}
		return nil
	}
	h, e := snapshot.DecodeHead(o.CurrentHead)
	if e != nil {
		return e
	}
	if e = canonical(o.CurrentHead); e != nil {
		return e
	}
	if !o.Inventory.matches("head.json", o.CurrentHead) {
		return malformed("stage head inventory binding")
	}
	return o.Inventory.chain(h)
}
