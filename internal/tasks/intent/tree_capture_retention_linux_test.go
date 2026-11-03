// SPDX-License-Identifier: Apache-2.0

//go:build linux

package intent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TCX-V0-977: even when the parent directory stays bound, retention must not
// associate bytes from a replacement child with the previous inode's stat key.
func TestTCXV0977_CaptureFileSwapDoesNotPoisonRetention(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, Dir, TicketsDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "T.json")
	peer := filepath.Join(root, "replacement")
	for p, raw := range map[string]string{path: "A", peer: "B"} {
		if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	saved := beforeTreeCapture
	beforeTreeCapture = func() {
		if err := os.Rename(peer, path); err != nil {
			t.Fatal(err)
		}
	}
	defer func() { beforeTreeCapture = saved }()
	var cache TreeCapture
	tree, err := cache.Capture(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 || tree.Files[0].Sha256 != wire.Sum([]byte("B")) {
		t.Fatal("capture did not read the replacement inode")
	}
	if _, retained := cache.files["tickets/T.json"]; retained {
		t.Fatal("retained replacement bytes despite differing phase-1 and opened-file keys")
	}
}
