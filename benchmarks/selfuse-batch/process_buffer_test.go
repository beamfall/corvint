// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package main

import (
	"io"
	"strings"
	"testing"
)

// processSnapshotLimit is processBuffer's cap on one /bin/ps snapshot.
const processSnapshotLimit = 4 * 1024 * 1024

// onlyReader hides every method but Read, as os/exec's copy of a child's
// pipe reaches the destination: io.Copy then prefers dst.ReadFrom.
type onlyReader struct{ io.Reader }

// TestProcessBufferCapHoldsOnEveryWritePath_GPKV09930 drives processBuffer
// past its 4 MiB cap through io.Copy and io.WriteString (V1-5122,
// GPK-V0-9930): the write fails and nothing past the cap is kept.
func TestProcessBufferCapHoldsOnEveryWritePath_GPKV09930(t *testing.T) {
	t.Parallel()
	over := strings.Repeat("x", processSnapshotLimit+1)
	if _, ok := any(&processBuffer{}).(io.ReaderFrom); ok {
		t.Fatal("processBuffer exposes ReadFrom, which io.Copy prefers over the capped Write")
	}
	if _, ok := any(&processBuffer{}).(io.StringWriter); ok {
		t.Fatal("processBuffer exposes WriteString, which io.WriteString prefers over the capped Write")
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
		var buffer processBuffer
		if err := write(&buffer); err == nil || err.Error() != "process snapshot output limit" || buffer.buffer.Len() > processSnapshotLimit {
			t.Fatalf("%s: err=%v retained=%d bytes past the %d-byte cap", name, err, buffer.buffer.Len(), processSnapshotLimit)
		}
	}
}
