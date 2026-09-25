package transaction

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func validateRedoInventory(p *Plan, i *Inventory) error {
	if p == nil || i == nil {
		return malformed("redo requires frozen plan and complete inventory")
	}
	rc, e := snapshot.DecodeReceipt(p.receipt)
	if e != nil {
		return e
	}
	receiptName, _ := snapshot.ReceiptName(rc.Seq.Uint64())
	if !i.matches("receipts/"+receiptName, p.receipt) {
		return malformed("missing/mismatching pending receipt")
	}
	// Preserve every prior receipt, orphan, projection and directory except the
	// exact post/head targets. No extra receipt or caller-authored post is accepted.
	allowed := map[string]bool{"head.json": true, "receipts/" + receiptName: true}
	for _, a := range p.artifacts {
		allowed[a.Target] = true
	}
	for _, post := range rc.Post {
		allowed[post.Path] = true
	}
	for path, old := range p.base.files {
		if !allowed[path] && i.files[path] != old {
			return malformed("redo changed retained base")
		}
	}
	for path := range i.files {
		if _, old := p.base.files[path]; !old && !allowed[path] {
			return malformed("redo unexpected retained file")
		}
	}
	for d := range p.base.dirs {
		if !i.dirs[d] {
			return malformed("redo deleted directory")
		}
	}
	for _, a := range p.artifacts {
		if strings.HasPrefix(a.Target, "evidence/") && i.files[a.Target] != entry(a) {
			return malformed("committed redo missing promised blob")
		}
	}
	for j, post := range rc.Post {
		current, exists := i.files[post.Path]
		var currentHash *wire.Digest
		if exists {
			d := current.Sha256
			currentHash = &d
		}
		if post.Sha256 == nil {
			if _, e = journal.DeleteRedo(*rc.Pre[j].Sha256, currentHash); e != nil {
				return e
			}
			continue
		}
		if i.matches(post.Path, p.posts[post.Path]) {
			continue
		}
		pre := rc.Pre[j].Sha256
		if pre == nil && !exists {
			continue
		}
		if pre != nil && exists && *pre == current.Sha256 {
			if old, ok := p.base.files[post.Path]; ok && old == current {
				continue
			}
		}
		return wire.Errorf(wire.CodeJournalForked, post.Path, "redo encountered third bytes or size")
	}
	if f, ok := i.files["head.json"]; ok {
		if i.matches("head.json", p.head) {
			for _, post := range rc.Post {
				if post.Sha256 == nil {
					if _, exists := i.files[post.Path]; exists {
						return malformed("advanced head before deletion")
					}
					continue
				}
				if !i.matches(post.Path, p.posts[post.Path]) {
					return malformed("advanced head before post")
				}
			}
			return nil
		}
		if p.baseHead == nil || f != p.base.files["head.json"] {
			return malformed("redo head is neither base nor post")
		}
	} else if p.baseHead != nil {
		return malformed("redo head absent")
	}
	return nil
}

// RedoCapacity models only the remaining publications of the already-linked
// receipt. It never mints a receipt, request identity or second UNPAUSE reserve.
func RedoCapacity(p *Plan, current *Inventory) (Capacity, error) {
	out := Capacity{Coverage: coverage()}
	if e := validateRedoInventory(p, current); e != nil {
		return out, e
	}
	i := current.clone()
	i.dirs["staging"] = true
	h, e := snapshot.DecodeHead(p.head)
	if e != nil {
		return out, e
	}
	promised := uint64(0)
	for _, a := range p.artifacts {
		promised, e = add(promised, a.Bytes.Uint64())
		if e != nil {
			return out, e
		}
	}
	temporary, e := add(promised, 2*uint64(len(p.descriptor)))
	if e != nil {
		return out, e
	}
	names := uint64(len(p.artifacts) + 2)
	record := func(step string) error {
		c, e := measure(i, h, temporary, promised, names)
		if e != nil {
			return e
		}
		if e = check(c, hardLimits()); e != nil {
			return e
		}
		out.Prefixes = append(out.Prefixes, Prefix{Step: step, Cost: c})
		return nil
	}
	if e = record("existing-linked-receipt"); e != nil {
		return out, e
	}
	for _, a := range p.artifacts {
		if a.Role != "POST" || strings.HasPrefix(a.Target, "evidence/") {
			continue
		}
		if e = i.put(entry(a), false); e != nil {
			return out, e
		}
		if e = record("durable-redo-post:" + a.Target); e != nil {
			return out, e
		}
	}
	if raw, ok := p.posts["barrier.json"]; ok && raw == nil {
		delete(i.files, "barrier.json")
		if e = record("durable-redo-barrier-unlink"); e != nil {
			return out, e
		}
	}
	if e = i.put(bytesEntry("head.json", p.head), false); e != nil {
		return out, e
	}
	if e = record("durable-redo-head"); e != nil {
		return out, e
	}
	temporary, promised, names = 0, 0, 0
	if e = record("durable-redo-cleanup"); e != nil {
		return out, e
	}
	if _, barrier := i.files["barrier.json"]; barrier {
		if _, e = unpausePeak(i, h, hardLimits()); e != nil {
			return out, e
		}
	}
	out.Final = i
	return out, nil
}

// AbortCapacity checks a classified precommit cleanup prefix and the successor
// UNPAUSE envelope. Cleanup retains old projections and linked orphans. Its
// FreshDescriptor discriminator stays HELD until required fsyncs are modeled.
type AbortCapacityResult struct {
	Cleanup  CleanupResult
	Current  Cost
	Unpause  Cost
	Coverage Coverage
}

func AbortCapacity(o StageObservation, c CleanupState) (AbortCapacityResult, error) {
	out := AbortCapacityResult{Coverage: coverage()}
	cleanup, e := Cleanup(o, c)
	if e != nil {
		return out, e
	}
	out.Cleanup = cleanup
	h, e := snapshot.DecodeHead(o.CurrentHead)
	if e != nil {
		return out, e
	}
	i := o.Inventory.clone()
	i.dirs["staging"] = true
	out.Current, e = measure(i, h, cleanup.TemporaryBytes, cleanup.PromisedBytes, cleanup.Entries-1)
	if e != nil {
		return out, e
	}
	if e = check(out.Current, hardLimits()); e != nil {
		return out, e
	}
	if _, ok := i.files["barrier.json"]; !ok {
		return out, malformed("abort-to-UNPAUSE requires supplied barrier")
	}
	out.Unpause, e = unpausePeak(i, h, hardLimits())
	return out, e
}
