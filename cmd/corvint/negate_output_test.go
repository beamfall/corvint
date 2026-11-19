// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/jsonend"
)

const negateRefusalSchema = "corvint-step-negation-refusal/0"

// negateOutput is a bounded stdout capture; its buffer is a named field so no promoted
// write path skips the cap (GPK-V0-9930).
type negateOutput struct {
	buffer  bytes.Buffer
	limit   int
	overrun bool
}

func (o *negateOutput) Write(data []byte) (int, error) {
	if o.buffer.Len()+len(data) > o.limit {
		o.overrun = true
		return len(data), nil
	}
	return o.buffer.Write(data)
}

// decodeNegateRefusal accepts exactly one closed refusal document and returns
// its typed code; anything else is a provider failure.
func decodeNegateRefusal(data []byte) error {
	var refusal struct {
		Schema  string `json:"schema"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&refusal); err != nil || !jsonend.Done(decoder) ||
		refusal.Schema != negateRefusalSchema || refusal.Code == "" {
		return &gokernel.Error{
			Code: "negate-provider-failed", Message: "provider exited 2 without a refusal document",
		}
	}
	return &gokernel.Error{Code: refusal.Code, Message: refusal.Message}
}
