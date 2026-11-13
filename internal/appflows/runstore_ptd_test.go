// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"testing"

	"github.com/Beamfall/corvint/internal/runevidence"
)

// PTD-V1-003, PTD-V1-008: a stored receipt row with no durationMs becomes an attempt whose zero
// DurationMS is marked not measured; a row with a time keeps it as measured.
func TestPTDV1008StoredRowWithoutDurationIsNotMeasured(t *testing.T) {
	seven := int64(7)
	record := runevidence.Record{
		Entry: runevidence.Entry{ID: "r", Profile: runevidence.ProfileReceipt},
		Rows: []runevidence.Row{
			{TestID: "go:m/a", Outcome: "passed", DurationMs: &seven, DurationSource: "go-test2json"},
			{TestID: "go:m/b", Outcome: "passed", DurationSource: "NOT_MEASURED"},
		},
	}
	records, err := storedRecords(record)
	if err != nil || len(records) != 2 {
		t.Fatalf("stored records %+v %v", records, err)
	}
	measured, unmeasured := records[0].Attempts[0], records[1].Attempts[0]
	if !measured.DurationMeasured() || measured.DurationMS != 7 {
		t.Fatalf("measured attempt %+v", measured)
	}
	if unmeasured.DurationMeasured() || unmeasured.DurationMS != 0 {
		t.Fatalf("a row with no durationMs was read as measured: %+v", unmeasured)
	}
}
