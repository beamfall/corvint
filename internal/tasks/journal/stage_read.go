package journal

import (
	"os"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
)

type stageDigest struct {
	Name string
	Sum  wire.Digest
}

// Read only declared bounded bytes. Malformed descriptors become body errors;
// their consumed bytes remain in the tuple and are rechecked before refusal.
func (r Reader) readStage(o *observation) (files []snapshot.StageFile, validation error, err error) {
	var d *snapshot.StageDescriptor
	read := func(name string, max int) error {
		raw, e := requiredRead(r.Source, "staging/"+name, max)
		if e != nil {
			return e
		}
		files = append(files, snapshot.StageFile{Name: name, Raw: raw})
		return nil
	}
	if o.files["staging/active.json"] != nil {
		if e := read("active.json", snapshot.MaxStageDescriptorBytes); e != nil {
			return nil, nil, e
		}
		d, validation = snapshot.DecodeStageDescriptor(files[0].Raw)
	}
	if o.files["staging/active.json.tmp"] != nil {
		cap := snapshot.MaxStageDescriptorBytes
		if d != nil {
			cap = len(files[0].Raw)
		}
		if e := read("active.json.tmp", cap); e != nil {
			return files, nil, e
		}
	}
	if validation != nil {
		return files, validation, nil
	}
	assigned := map[string]bool{}
	if d != nil {
		for _, a := range d.Artifacts {
			assigned[a.Slot] = true
			if o.files["staging/"+a.Slot] != nil {
				if e := read(a.Slot, int(a.Bytes.Uint64())); e != nil {
					return files, nil, e
				}
			}
		}
	}
	descriptorless := d == nil && o.files["staging/active.json.tmp"] == nil
	var inFlight *slotsInFlight
	for _, p := range sortedPaths(o.files) {
		if strings.HasPrefix(p, "staging/a") && p != "staging/active.json" && p != "staging/active.json.tmp" && !assigned[strings.TrimPrefix(p, "staging/")] {
			if !descriptorless {
				return files, wire.Errorf(wire.CodeMalformed, p, "unassigned stage slot"), nil
			}
			if granted := r.orphanSlots[p]; granted != nil && sameSlot(granted, o.files[p]) {
				// A free writer lock proved this exact slot a killed
				// writer's orphan (CTS-V0-026): set it aside unread. Its
				// metadata stays in the inventory and staging stays present.
				o.staging = true
				o.orphanSlots = append(o.orphanSlots, p)
				continue
			}
			if inFlight == nil {
				// A native writer holds descriptor-less slots under its lock
				// while it publishes; audit waits a bounded time (CTS-V0-008).
				inFlight = &slotsInFlight{error: wire.Errorf(wire.CodeMalformed, p, "unassigned stage slot"), head: o.headRaw, slots: map[string]os.FileInfo{}}
			}
			inFlight.slots[p] = o.files[p]
		}
	}
	if inFlight != nil {
		return files, *inFlight, nil
	}
	return files, nil, nil
}

// slotsInFlight marks an unassigned-slot refusal observed with no stage
// descriptor or descriptor temp: the shape a native writer's publish leaves
// for its lifetime and a killed writer leaves until the next writer clears
// it. audit waits for it (snapshot.StageSlotWait) and then returns the wrapped
// MALFORMED unchanged (CTS-V0-008), unless the head the observation read moved
// while it waited, which warrants another bounded observation but does not
// prove the slots' holders are live (CAL-V0-340). head is that observation's
// head.json. slots binds every descriptor-less slot the observation listed to
// its listed metadata, for the CTS-V0-026 orphan proof.
type slotsInFlight struct {
	error
	head  []byte
	slots map[string]os.FileInfo
}

// sameSlot reports that a granted orphan is still the very file the refused
// observation listed: same file identity, mode, size and modification time.
func sameSlot(granted, now os.FileInfo) bool {
	return now != nil && os.SameFile(granted, now) && granted.Mode() == now.Mode() && granted.Size() == now.Size() && granted.ModTime().Equal(now.ModTime())
}

func (e slotsInFlight) Unwrap() error { return e.error }

func (r Reader) validateStage(o *observation, genesisQueue []byte) error {
	if o.stageErr != nil {
		return o.stageErr
	}
	stage := o.stage
	if len(stage.Files) == 0 {
		return nil
	}
	var queue []byte
	var e error
	if o.head == nil {
		// Supplied only after walk has proved the complete linked genesis,
		// retained blobs, request binding and every pending projection.
		var present bool
		queue, present, e = optionalRead(r.Source, "intent/queue.json", wire.MaxQueueFileBytes)
		if e != nil {
			return e
		}
		if !present {
			queue = genesisQueue
		}
		if queue == nil {
			return wire.Errorf(wire.CodeJournalForked, "receipts/000000000001.json", "validated genesis queue proof absent")
		}
	} else {
		queue, e = requiredRead(r.Source, "intent/queue.json", wire.MaxQueueFileBytes)
		if e != nil {
			return e
		}
	}
	binding := snapshot.StageBinding{QueueID: r.QueueID.Raw, QueueRaw: queue, HeadRaw: o.headRaw}
	if d := stage.Descriptor; d != nil && o.head != nil && (d.Base == nil || o.head.LastSeq != d.Base.LastSeq) {
		name, _ := snapshot.ReceiptName(o.head.LastSeq.Uint64())
		binding.ReceiptRaw, e = requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
		if e != nil {
			return e
		}
		rc, e := snapshot.DecodeReceipt(binding.ReceiptRaw)
		if e != nil {
			return e
		}
		path, _ := snapshot.RequestPath(d.RequestID)
		for _, p := range rc.Post {
			if p.Path == path {
				binding.RequestRaw, e = r.postBytes(p)
				if e != nil {
					return e
				}
			}
		}
	}
	return stage.Bind(binding)
}
