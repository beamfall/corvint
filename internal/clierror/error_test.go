// SPDX-License-Identifier: Apache-2.0
package clierror

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCAPV07707NativeEnvelopes(t *testing.T) {
	for _, raw := range []string{
		`{"ok":false,"error":"cannot read file"}`,
		`{"error":{"code":"not-found","message":"file does not exist"}}`,
		`{"outcome":"REFUSED","codes":["MALFORMED"],"warnings":["ticket does not exist"],"items":[]}`,
		`{"code":"proof-missing","error":"proof-missing","subject":"claim","evidence":[],"supported_fixes":[]}`,
	} {
		var out, errout bytes.Buffer
		s := &Streams{Binary: "corvint-x", Out: &out, Err: &errout}
		_, _ = s.Stdout().Write([]byte(raw))
		s.Status(1)
		var e Envelope
		if out.Len() != 0 || json.Unmarshal(errout.Bytes(), &e) != nil || Validate(e) != nil || e.Details == nil {
			t.Fatalf("%s => out=%s err=%s", raw, out.Bytes(), errout.Bytes())
		}
	}
}

func TestCAPV07707PlainFailureIsOneEnvelope(t *testing.T) {
	var out, errout bytes.Buffer
	s := &Streams{Binary: "tool", Out: &out, Err: &errout}
	_, _ = s.Stderr().Write([]byte("missing input\nUsage: tool --file FILE\n"))
	if status := s.Status(2); status != 2 {
		t.Fatal(status)
	}
	s.Finish()
	var e Envelope
	if json.Unmarshal(errout.Bytes(), &e) != nil || Validate(e) != nil || bytes.Count(errout.Bytes(), []byte("\n")) != 1 {
		t.Fatalf("not one error: %s", errout.Bytes())
	}
}
