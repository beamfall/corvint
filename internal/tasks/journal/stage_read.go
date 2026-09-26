package journal

import (
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
	for p := range o.files {
		if strings.HasPrefix(p, "staging/a") && p != "staging/active.json" && p != "staging/active.json.tmp" && !assigned[strings.TrimPrefix(p, "staging/")] {
			return files, wire.Errorf(wire.CodeMalformed, p, "unassigned stage slot"), nil
		}
	}
	return files, nil, nil
}

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
