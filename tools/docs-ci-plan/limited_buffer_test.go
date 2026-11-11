// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"strings"
	"testing"
)

// onlyReader hides every method but Read, as os/exec's copy of a child's
// pipe reaches the destination: io.Copy then prefers dst.ReadFrom.
type onlyReader struct{ io.Reader }

// TestLimitedBufferCapHoldsOnEveryWritePath_GPKV09930 drives limitedBuffer
// past maxOutput through io.Copy and io.WriteString (V1-5122, GPK-V0-9930):
// the write fails with git-output-limit and nothing past the cap is kept.
func TestLimitedBufferCapHoldsOnEveryWritePath_GPKV09930(t *testing.T) {
	t.Parallel()
	over := strings.Repeat("x", maxOutput+1)
	if _, ok := any(&limitedBuffer{}).(io.ReaderFrom); ok {
		t.Fatal("limitedBuffer exposes ReadFrom, which io.Copy prefers over the capped Write")
	}
	if _, ok := any(&limitedBuffer{}).(io.StringWriter); ok {
		t.Fatal("limitedBuffer exposes WriteString, which io.WriteString prefers over the capped Write")
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
		var buffer limitedBuffer
		if err := write(&buffer); err == nil || err.Error() != "git-output-limit" || buffer.buffer.Len() > maxOutput {
			t.Fatalf("%s: err=%v retained=%d bytes past the %d-byte cap", name, err, buffer.buffer.Len(), maxOutput)
		}
	}
}
