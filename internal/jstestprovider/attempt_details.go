package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"reflect"
)

const AttemptExternalProfile = "corvint-playwright-external/3"

// DecodeAttemptReceipt admits the canonical /3 document and recomputes its projections.
func DecodeAttemptReceipt(raw []byte) (Receipt, error) {
	var document struct {
		Receipt Receipt         `json:"receipt"`
		Tests   json.RawMessage `json:"testProjections"`
		Run     json.RawMessage `json:"runProjection"`
	}
	if len(raw) > externalOutputLimit {
		return Receipt{}, errors.New("qualified-document-output-overflow")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&document) != nil || decoder.Decode(new(any)) != io.EOF || document.Receipt.Profile != AttemptExternalProfile {
		return Receipt{}, errors.New("external-attempt-document-invalid")
	}
	canonical, err := EncodeQualified(document.Receipt)
	if err != nil || !bytes.Equal(raw, canonical) {
		return Receipt{}, errors.New("external-attempt-document-invalid")
	}
	return document.Receipt, nil
}

// ValidateAttemptDetails binds every retained detail to the attempt inventory and last summary.
func ValidateAttemptDetails(tests []TestOutcome) error {
	for _, t := range tests {
		if len(t.Attempts) == 0 || len(t.Attempts) > 32 || len(t.AttemptDetails) != len(t.Attempts) || t.Retries != len(t.Attempts)-1 {
			return errors.New("external-attempt-details-invalid")
		}
		hadFailure := false
		for i, d := range t.AttemptDetails {
			a := t.Attempts[i]
			if d.Retry != i || a.Retry != i || d.State != a.State || math.IsNaN(d.DurationMS) || math.IsInf(d.DurationMS, 0) || d.DurationMS < 0 {
				return errors.New("external-attempt-details-invalid")
			}
			switch d.State {
			case StatePassed, StateFailed, StateSkipped, StateTimedOut, StateInterrupted, StateInfrastructure:
			default:
				return errors.New("external-attempt-details-invalid")
			}
			if d.State != StatePassed {
				hadFailure = true
			}
		}
		last := t.AttemptDetails[len(t.AttemptDetails)-1]
		state := last.State
		if state == StatePassed && hadFailure {
			state = StateFlaky
		}
		if state != t.State || last.DurationMS != t.DurationMS || last.FailureMessage != t.FailureMessage || !reflect.DeepEqual(last.Anchor, t.Anchor) || !reflect.DeepEqual(last.Artifacts, t.Artifacts) {
			return errors.New("external-attempt-details-invalid")
		}
	}
	return nil
}
