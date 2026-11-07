// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-9010: the filesystem proof admits regular files and directories
// only. Unknown modes and read failures are observing, not just symlinks.
func TestTestdataUnknownEntryFailsClosed_AFP_V0_9010(t *testing.T) {
	const body = "package probe\nimport \"os\"\nvar _, _ = os.ReadFile(\"testdata/fixture\")\n"
	for _, mode := range []fs.FileMode{0, fs.ModeSymlink, fs.ModeNamedPipe, fs.ModeDevice, fs.ModeIrregular} {
		t.Run(mode.String(), func(t *testing.T) {
			source := affected.FSSource(fstest.MapFS{"probe/testdata/fixture": {Data: []byte("fixture"), Mode: mode}})
			if got := observesSource(source, "probe/probe_test.go", []byte(body), nil); got != (mode != 0) {
				t.Fatalf("mode %v: observing=%v", mode, got)
			}
		})
	}
	if !observesSource(affected.FSSource(failingReaderFS{}), "probe/probe_test.go", []byte(body), nil) {
		t.Fatal("unknown filesystem error was certified")
	}
}

type failingReaderFS struct{}

func (failingReaderFS) Open(string) (fs.File, error) { return nil, fs.ErrPermission }
