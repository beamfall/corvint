package testplan

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestConsoleQualificationGolden pins the V1-1023 real-repository qualification: 47 labelled
// HTTP-route variations taken from this repository's own console tests
// (internal/console/{console,roadmap,security}_test.go at a0b4c4108) and the plan the planner
// derives for them. The selection rule and the derivation are recorded in
// docs/build-log/2026-10-16-v1-1023-1024-console-qualification.md.
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
	want := Counts{Variations: 47, NewTests: 33, Grouped: 20, Isolated: 27, BaselineOnePerRow: 47}
	if p.Counts != want {
		t.Fatalf("counts = %+v, want %+v", p.Counts, want)
	}
	if p.Status != "COMPLETE" {
		t.Fatalf("status = %q, want COMPLETE", p.Status)
	}
	table := p.Table()
	sum := sha256.Sum256(table)
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != "sha256:6315943581297a42c0e94898b8294e6c8309784ac1a026014b4e96d368ead086" {
		t.Fatalf("table digest = %s", got)
	}
	golden(t, "console-qualification.table", table)
	if err := Check(p, table); err != nil {
		t.Fatalf("check of the plan's own table: %v", err)
	}
}
