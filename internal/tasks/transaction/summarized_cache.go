// SPDX-License-Identifier: Apache-2.0

package transaction

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SummarizedInventoryCache owns the inventory of one serialized writer.
// Build invalidates its previous result, which must no longer be in use.
// Every current entry is compared; only changed entries are validated and
// charged again. key binds the store and immutable audited checkpoint prefix.
// A different prefix, directory set or summary rebuilds normally (TCX-V0-979).
type SummarizedInventoryCache struct {
	key string
	inv *Inventory
}

func (c *SummarizedInventoryCache) Build(key string, files []archive.FileEntry, dirs []string, e Elided) (*Inventory, error) {
	old := c.inv
	c.inv = nil // a failed update never remains reusable
	same := old != nil && key == c.key && old.elided != nil && e.Intent == nil && old.elided.Intent == nil
	if same {
		x := old.elided
		same = x.Receipts == e.Receipts && x.Requests == e.Requests && x.FirstReceiptSha256 == e.FirstReceiptSha256 && x.LastReceiptSha256 == e.LastReceiptSha256 && x.ReceiptBytes == e.ReceiptBytes && x.Cost == e.Cost && e.HasRequest != nil && len(dirs) == len(old.dirs)
	}
	seenDirs := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		if old == nil || !old.dirs[d] || seenDirs[d] {
			same = false
		}
		seenDirs[d] = true
	}
	if !same {
		inv, err := NewSummarizedInventory(files, dirs, e)
		if err == nil {
			c.key, c.inv = key, inv
		}
		return inv, err
	}
	if len(files) > wire.MaxArchiveFiles {
		return nil, limit("inventory entries")
	}
	seen := make(map[string]bool, len(files))
	changed := []archive.FileEntry{}
	tickets := uint64(0)
	for _, f := range files {
		if seen[f.Path] {
			return nil, malformed("duplicate inventory file")
		}
		seen[f.Path] = true
		if strings.HasPrefix(f.Path, "intent/tickets/") {
			tickets++
		}
		if prior, ok := old.files[f.Path]; !ok || prior != f {
			if n, ok := receiptNumber(f.Path); ok && n <= e.Receipts {
				return nil, malformed("explicit receipt inside the elided prefix")
			}
			if strings.HasPrefix(f.Path, "requests/") && e.HasRequest(f.Path) {
				return nil, malformed("explicit request inside the elided prefix")
			}
			changed = append(changed, f)
		}
	}
	if _, err := NewInventory(changed, dirs); err != nil {
		return nil, err
	}
	work := old.clone()
	work.miss = new(bool)
	ok := false
	defer func() {
		if !ok {
			work.rollback()
		}
	}()
	for p := range old.files {
		if !seen[p] {
			if err := work.remove(p); err != nil {
				return nil, err
			}
		}
	}
	for _, f := range changed {
		if err := work.put(f, false); err != nil {
			return nil, err
		}
	}
	cost := work.incremental.cost
	if cost.Files > wire.MaxArchiveFiles || cost.ScannedEntries+uint64(len(dirs)) > wire.MaxArchiveScanEntries || cost.IntentBytes > wire.MaxIntentTreeBytes || tickets > wire.MaxTicketsPerQueue {
		return nil, limit("inventory scan/intent bounds")
	}
	work.undoFiles, work.undoDirs = nil, nil
	c.inv, c.key, ok = work, key, true
	return work, nil
}
