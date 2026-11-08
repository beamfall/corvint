// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testrunner"
)

// TestRSPV0040ShellOutputCapHoldsOnEveryWritePath pins that the shell stream cap
// cannot be bypassed by io.Copy (which prefers ReaderFrom, as os/exec does) or by
// WriteString.
func TestRSPV0040ShellOutputCapHoldsOnEveryWritePath(t *testing.T) {
	limit := testrunner.MaxReportBytes / 4
	over := limit + 4096
	var copied shellOutput
	if _, err := io.Copy(&copied, bytes.NewReader(make([]byte, over))); err != nil {
		t.Fatal(err)
	}
	var written shellOutput
	if _, err := io.WriteString(&written, strings.Repeat("x", over)); err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string]*shellOutput{"io.Copy": &copied, "WriteString": &written} {
		if len(out.Bytes()) != limit || !out.overflow {
			t.Fatalf("%s: kept %d bytes (limit %d), overflow=%v", name, len(out.Bytes()), limit, out.overflow)
		}
	}
}
