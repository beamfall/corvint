package cemcandidate

import (
	"encoding/json"
	"fmt"
	"io"
)

// PlainNextActions names the supported verify route for the bundle a plain
// assembly wrote: cem verify-stable over its stable.json, with the request's
// own repository and revisions and the bundle directory as the artifact root,
// exactly the options the assembler's own verification used (CEM-V1-350).
func PlainNextActions(r PlainRequest, stablePath, bundle string) [][]string {
	return [][]string{{"corvint", "cem", "verify-stable", "--repository", r.Repository, "--map", stablePath, "--expected-base", r.ExpectedBase, "--target", r.Target, "--artifacts", bundle}}
}

// EncodeOK writes result as json.Encoder does, with one leading top-level
// "ok": true member; every other byte is the result's own encoding
// (CEM-V1-341), except that a non-nil next is appended as one trailing
// "nextActions" member (CEM-V1-350).
func EncodeOK(out io.Writer, result any, next [][]string) error {
	encoded, e := json.Marshal(result)
	if e != nil {
		return e
	}
	if len(encoded) < 2 || encoded[0] != '{' {
		return fmt.Errorf("assembly result is not a JSON object")
	}
	envelope := []byte(`{"ok":true`)
	if string(encoded) != "{}" {
		envelope = append(envelope, ',')
	}
	envelope = append(envelope, encoded[1:len(encoded)-1]...)
	if next != nil {
		actions, e := json.Marshal(next)
		if e != nil {
			return e
		}
		envelope = append(append(envelope, `,"nextActions":`...), actions...)
	}
	envelope = append(envelope, '}', '\n')
	_, e = out.Write(envelope)
	return e
}
