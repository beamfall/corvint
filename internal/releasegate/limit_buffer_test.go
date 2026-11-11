// SPDX-License-Identifier: Apache-2.0

package releasegate

import (
	"io"
	"strings"
	"testing"
)

// onlyReader hides every method but Read, as the release gate's io.Copy of a
// Git pipe reaches the destination: io.Copy then prefers dst.ReadFrom.
type onlyReader struct{ io.Reader }

// TestLimitBufferCapHoldsOnEveryWritePath_GPKV09930 drives limitBuffer past
// its limit through io.Copy and io.WriteString (V1-5122, GPK-V0-9930): the
// exceeded signal is set and exactly the cap is retained.
func TestLimitBufferCapHoldsOnEveryWritePath_GPKV09930(t *testing.T) {
	t.Parallel()
	const limit = 16
	over := strings.Repeat("x", 4*limit)
	if _, ok := any(&limitBuffer{}).(io.ReaderFrom); ok {
		t.Fatal("limitBuffer exposes ReadFrom, which io.Copy prefers over the capped Write")
	}
	if _, ok := any(&limitBuffer{}).(io.StringWriter); ok {
		t.Fatal("limitBuffer exposes WriteString, which io.WriteString prefers over the capped Write")
	}
	for name, write := range map[string]func(io.Writer) error{
		"io.Copy from a bare reader": func(w io.Writer) error {
			_, err := io.Copy(w, onlyReader{strings.NewReader(over)})
			return err
		},
		"io.Copy from a WriterTo": func(w io.Writer) error {
			_, err := io.Copy(w, strings.NewReader(over))
			return err
		},
		"io.WriteString": func(w io.Writer) error {
			_, err := io.WriteString(w, over)
			return err
		},
	} {
		buffer := &limitBuffer{limit: limit}
		if err := write(buffer); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !buffer.exceeded || buffer.buffer.Len() != limit {
			t.Fatalf("%s: exceeded=%v retained=%d bytes, want the %d-byte cap", name, buffer.exceeded, buffer.buffer.Len(), limit)
		}
	}
}
