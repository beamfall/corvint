// SPDX-License-Identifier: Apache-2.0
package wire

import "testing"

// ACX-V0-7707: process metadata is additive around the unchanged native result.
func TestACXV07707NativeResultMetadata(t *testing.T) {
	raw, err := (&Result{Command: []string{"help"}, Outcome: OutcomeError, Codes: []string{CodeMalformed}, Warnings: []string{"missing input"}}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	v, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]Value{"binary": String("corvint-tasks"), "code": String("MALFORMED"), "error": String("missing input"), "help": String("corvint-tasks --help"), "ok": Bool(false)} {
		v.Obj.Set(key, value)
	}
	// details must be an independent native value, not a self-referential object.
	native, _ := Parse(raw)
	v.Obj.Set("details", native)
	result, err := DecodeResult(EncodeFile(v))
	if err != nil || result.Outcome != OutcomeError || !HasCode(result, CodeMalformed) {
		t.Fatalf("native refusal metadata: %+v %v", result, err)
	}
	v.Obj.Set("ok", Bool(true))
	if _, err := DecodeResult(EncodeFile(v)); err == nil {
		t.Fatal("ok:true admitted as refusal metadata")
	}
}
