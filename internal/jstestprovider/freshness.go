package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// FreshnessProfile is jstestreceipt.FreshnessProfile, kept under its old name.
const FreshnessProfile = jstestreceipt.FreshnessProfile

// FreshCommandIdentity is jstestreceipt.FreshCommandIdentity, kept under its old name.
type FreshCommandIdentity = jstestreceipt.FreshCommandIdentity

// FreshCommandBinding is jstestreceipt.FreshCommandBinding, kept under its old name.
type FreshCommandBinding = jstestreceipt.FreshCommandBinding

// FreshProcessIdentity is jstestreceipt.FreshProcessIdentity, kept under its old name.
type FreshProcessIdentity = jstestreceipt.FreshProcessIdentity

// FreshServeObservation is jstestreceipt.FreshServeObservation, kept under its old name.
type FreshServeObservation = jstestreceipt.FreshServeObservation

// FreshnessBinding is jstestreceipt.FreshnessBinding, kept under its old name.
type FreshnessBinding = jstestreceipt.FreshnessBinding

// FreshnessCurrency is jstestreceipt.FreshnessCurrency, kept under its old name.
func FreshnessCurrency(r Receipt, t TestOutcome) string { return jstestreceipt.FreshnessCurrency(r, t) }

type freshnessDocument struct {
	Receipt     Receipt                   `json:"receipt"`
	Projections []testvalidity.Projection `json:"testProjections"`
	Run         testvalidity.Projection   `json:"runProjection"`
}

// EncodeFreshness and DecodeFreshness form a separate closed canonical codec.
// Legacy retention remains unsupported for this profile (PTF-V0-005).
func EncodeFreshness(r Receipt) ([]byte, error) {
	if r.Profile != FreshnessProfile || r.Freshness == nil || len(r.Tests) > 10000 || jstestreceipt.HasAttemptDetails(r) || jstestreceipt.HasSensitiveInputEvidence(r) {
		return nil, errors.New("freshness-profile-shape")
	}
	if r.Freshness.Mutation != nil {
		if _, err := ResponseMutation(r.Freshness.Source, r.Freshness.ArtifactPath, *r.Freshness.Mutation); err != nil {
			return nil, err
		}
	}
	d := freshnessDocument{Receipt: r, Run: ReceiptRunProjection(r), Projections: make([]testvalidity.Projection, 0, len(r.Tests))}
	for _, t := range r.Tests {
		d.Projections = append(d.Projections, ReceiptTestProjection(r, t))
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	if len(b) >= jstestreceipt.ExternalOutputLimit || secretscreen.MatchString(string(b)) {
		return nil, errors.New("freshness-output-bound-or-secret")
	}
	for _, observed := range []*FreshServeObservation{r.Freshness.ServedBefore, r.Freshness.ServedAfter} {
		if observed != nil && secretscreen.MatchString(string(observed.Body)) {
			return nil, errors.New("freshness-source-secret")
		}
	}
	if secretscreen.MatchString(string(r.Freshness.Source)) {
		return nil, errors.New("freshness-source-secret")
	}
	return append(b, '\n'), nil
}

func DecodeFreshness(data []byte) (Receipt, error) {
	if len(data) >= jstestreceipt.ExternalOutputLimit {
		return Receipt{}, errors.New("freshness-output-bound")
	}
	var d freshnessDocument
	if err := decodeFreshCanonical(data, &d, jstestreceipt.ExternalOutputLimit); err != nil {
		return Receipt{}, err
	}
	encoded, err := EncodeFreshness(d.Receipt)
	if err != nil {
		return Receipt{}, err
	}
	if !reflect.DeepEqual(d.Projections, freshnessProjections(d.Receipt)) || string(encoded) != string(data) {
		return Receipt{}, errors.New("freshness-projection-or-canonical-drift")
	}
	for _, t := range d.Receipt.Tests {
		if t.ID != "" && t.ID != jstestreceipt.QualifiedTestID(d.Receipt.Identity, t) {
			return Receipt{}, errors.New("freshness-native-identity-drift")
		}
	}
	return d.Receipt, nil
}

func decodeFreshCanonical(data []byte, out any, limit int) error {
	if len(data) == 0 || len(data) >= limit || secretscreen.MatchString(string(data)) {
		return errors.New("freshness-input-bound-or-secret")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("freshness-trailing-data")
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if !bytes.Equal(append(b, '\n'), data) {
		return errors.New("freshness-noncanonical-input")
	}
	return nil
}

func freshnessProjections(r Receipt) []testvalidity.Projection {
	out := make([]testvalidity.Projection, 0, len(r.Tests))
	for _, t := range r.Tests {
		out = append(out, ReceiptTestProjection(r, t))
	}
	return out
}
