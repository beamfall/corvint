package testplan

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestConsoleQualificationGolden pins the V1-1023 real-repository qualification: 44 labelled
// HTTP-route variations taken from this repository's own console tests
// (internal/console/{console,roadmap,security}_test.go at a0b4c4108) and the plan the planner
// derives for them. The selection rule and the derivation are recorded in
// https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-16-v1-1023-1024-console-qualification.md.
func TestConsoleQualificationGolden(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "console-qualification-input.json"))
	if err != nil {
		t.Fatal(err)
	}
	in, err := DecodeInput(data)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Evidence{}, DefaultMaxSteps)
	if err != nil {
		t.Fatal(err)
	}
	want := Counts{Variations: 44, NewTests: 31, Grouped: 19, Isolated: 25, BaselineOnePerRow: 44}
	if p.Counts != want {
		t.Fatalf("counts = %+v, want %+v", p.Counts, want)
	}
	if p.Status != "COMPLETE" {
		t.Fatalf("status = %q, want COMPLETE", p.Status)
	}
	table := p.Table()
	sum := sha256.Sum256(table)
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != "sha256:be92a7dedcfa6abac41a2006db1d9c02b1b49184ed7990ceb8bbdf713a55c240" {
		t.Fatalf("table digest = %s", got)
	}
	golden(t, "console-qualification.table", table)
	if err := Check(p, table); err != nil {
		t.Fatalf("check of the plan's own table: %v", err)
	}
}
