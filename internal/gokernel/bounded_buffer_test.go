// SPDX-License-Identifier: Apache-2.0

package gokernel

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// onlyReader hides every method but Read, as os/exec's copy of a child's
// pipe reaches the destination: io.Copy then prefers dst.ReadFrom.
type onlyReader struct{ io.Reader }

// TestBoundedBufferCapHoldsOnEveryWritePath_GPKV09930 drives boundedBuffer
// past its limit through io.Copy and io.WriteString (V1-5122, GPK-V0-9930):
// the exceeded signal is set and exactly the cap is retained.
func TestBoundedBufferCapHoldsOnEveryWritePath_GPKV09930(t *testing.T) {
	t.Parallel()
	const limit = 16
	over := strings.Repeat("x", 4*limit)
	if _, ok := any(&boundedBuffer{}).(io.ReaderFrom); ok {
		t.Fatal("boundedBuffer exposes ReadFrom, which io.Copy prefers over the capped Write")
	}
	if _, ok := any(&boundedBuffer{}).(io.StringWriter); ok {
		t.Fatal("boundedBuffer exposes WriteString, which io.WriteString prefers over the capped Write")
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
		buffer := &boundedBuffer{limit: limit}
		if err := write(buffer); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !buffer.exceeded || buffer.buffer.Len() != limit {
			t.Fatalf("%s: exceeded=%v retained=%d bytes, want the %d-byte cap", name, buffer.exceeded, buffer.buffer.Len(), limit)
		}
	}
}

// TestGitRawCapsOutputCopiedByExec_GPKV09930 runs a real Git whose stdout
// os/exec copies into boundedBuffer with io.Copy: output past the limit
// refuses repository-probe-too-large rather than returning every byte
// (V1-5122, GPK-V0-9930).
func TestGitRawCapsOutputCopiedByExec_GPKV09930(t *testing.T) {
	t.Parallel()
	output, err := gitRaw(context.Background(), t.TempDir(), 4, "version")
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "repository-probe-too-large" {
		t.Fatalf("output %q err %v, want repository-probe-too-large", output, err)
	}
}
