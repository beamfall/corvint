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
// The caller serializes use and binds the capture again before effects.
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
		c.files[f.Path] = capturedFile{plan[i].info, f}
	}
	c.at = at
}
