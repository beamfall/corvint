package localcompletion

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// TestOCMV0022FinishRetainsAggregateRefusalDetail covers OCM-V0-022: a dogfood
// finish whose OCM aggregate refuses keeps the code and, for a bound refusal,
// the bounded counts on its evidence, instead of reducing it to the bare code.
func TestOCMV0022FinishRetainsAggregateRefusalDetail(t *testing.T) {
	bound := "aggregate OCM obligations exceed the limit: observed 257, bound 256; per scope 1=200, 2=57"
	cases := []struct {
		name   string
		err    error
		code   string
		detail string
	}{
		{name: "OCM-V0-022 bound refusal keeps counts", err: &lrfrepo.Error{Code: "too-many-obligations", Message: bound}, code: "too-many-obligations", detail: bound},
		{name: "OCM-V0-022 other refusal keeps only the code", err: &lrfrepo.Error{Code: "duplicate-requirement", Message: "requirement ID repeats across intent scopes"}, code: "duplicate-requirement"},
		{name: "OCM-V0-022 oversized detail is dropped", err: &lrfrepo.Error{Code: "too-many-claims", Message: strings.Repeat("x", maxAggregateDetail+1)}, code: "too-many-claims"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evaluation := Evaluation{}
			err := aggregateRefusal(&evaluation, tc.err)
			if err == nil || err.Error() != tc.code {
				t.Fatalf("err = %v, want the bare code %q", err, tc.code)
			}
			if len(evaluation.Evidence) != 1 {
				t.Fatalf("evidence = %+v, want one aggregate refusal row", evaluation.Evidence)
			}
			item := evaluation.Evidence[0]
			if item.Tool != "dogfood-ocm-status" || item.Map != ".corvint/change.ocm-intents" || item.Code != tc.code || item.Detail != tc.detail {
				t.Fatalf("evidence row = %+v", item)
			}
			encoded, _ := json.Marshal(item)
			if tc.detail == "" && strings.Contains(string(encoded), `"detail"`) {
				t.Fatalf("detail member present without a bound refusal: %s", encoded)
			}
		})
	}
	t.Run("OCM-V0-022 uncoded error passes through unrecorded", func(t *testing.T) {
		evaluation := Evaluation{}
		plain := errors.New("disk read failed")
		if err := aggregateRefusal(&evaluation, plain); err != plain || len(evaluation.Evidence) != 0 {
			t.Fatalf("err = %v evidence = %+v", err, evaluation.Evidence)
		}
	})
}
