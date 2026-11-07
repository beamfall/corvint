// SPDX-License-Identifier: Apache-2.0
package clierror

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// ACX-V0-7707: native JSON refusals keep their stream, shape, bytes and status.
func TestCAPV07707NativeEnvelopes(t *testing.T) {
	for _, raw := range []string{`{"ok":false,"error":"cannot read file"}`, `{"error":{"code":"not-found","message":"file does not exist"}}`, `{"outcome":"REFUSED","codes":["MALFORMED"],"warnings":["ticket does not exist"],"items":[]}`} {
		for _, status := range []int{0, 1, 2, 7} {
			var out, errout bytes.Buffer
			s := &Streams{Binary: "tool", Out: &out, Err: &errout}
			io.WriteString(s.Stdout(), raw)
			io.WriteString(s.Stderr(), raw)
			if s.Status(status) != status || out.String() != raw || errout.String() != raw {
				t.Fatalf("changed native refusal: out=%q err=%q status=%d", out.String(), errout.String(), status)
			}
		}
	}
}

func TestCAPV07707PlainFailurePassesThrough(t *testing.T) {
	for _, raw := range []string{"missing input\nUsage: tool --file FILE\n", strings.Repeat("x", (1<<20)+1), ""} {
		var out, errout bytes.Buffer
		s := &Streams{Binary: "tool", Out: &out, Err: &errout}
		for _, part := range []string{raw[:len(raw)/2], raw[len(raw)/2:]} {
			io.WriteString(s.Stderr(), part)
		}
		if errout.String() != raw {
			t.Fatal("diagnostic buffered or truncated")
		}
		if s.Status(2) != 2 {
			t.Fatal("status changed")
		}
		s.Finish()
		if errout.String() != raw || out.Len() != 0 {
			t.Fatal("diagnostic changed on finish")
		}
	}
}
