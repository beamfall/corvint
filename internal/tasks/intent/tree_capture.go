// SPDX-License-Identifier: Apache-2.0

package intent

import (
	"os"
	"time"
)

// TreeCapture retains one bounded, process-local capture (TCX-V0-977).
// Capture always lists and stats the closed intent layout. Reuse requires
// native identity and change-time equality; unsupported platforms read all
// bytes. Returned trees and their bytes must be treated as immutable.
// The caller serializes use. This cache is only for planning/modeling:
// binding before effects must use TreeDigest to hash live bytes independently.
type TreeCapture struct {
	root  string
	files map[string]capturedFile
	at    time.Time
}

type capturedFile struct {
	info os.FileInfo
	file File
}

func (c *TreeCapture) Capture(root string) (Tree, error) {
	if c.root != root {
		c.files = nil
		c.root = root
	}
	tree, err := captureDigest(root, nil, nil, c)
	if err != nil {
		c.files = nil
	}
	return tree, err
}

func (c *TreeCapture) retain(tree Tree, plan []plannedFile, at time.Time) {
	if c.files == nil {
		c.files = make(map[string]capturedFile, len(plan))
	} else {
		clear(c.files)
	}
	// Both plan and tree.Files follow the closed layout's byte-sorted path
	// order. Retain metadata alongside those exact bytes without a second
	// inventory map or a per-entry heap allocation.
	for i, f := range tree.Files {
		p := plan[i]
		info := p.opened
		if p.reused.Path != "" {
			info = p.info
		}
		// Phase 1 may have stat'd a different inode than phase 2 opened.
		// Retain only matching metadata from the descriptor actually read.
		if sameCaptureKey(p.info, info) {
			c.files[f.Path] = capturedFile{info, f}
		}
	}
	c.at = at
}

// captureDirsBound checks the phase-1 directory identities before retention.
// Phase 2's pinned descriptors alone cannot detect a swap before they opened.
func captureDirsBound(root *os.Root, listing []capturedInfo) bool {
	for _, entry := range listing {
		if !entry.info.IsDir() {
			continue
		}
		path := entry.path
		if path == "" {
			path = "."
		}
		now, err := root.Lstat(path)
		if err != nil || !os.SameFile(entry.info, now) {
			return false
		}
	}
	return true
}
