package main

import (
	"testing"

	"github.com/Beamfall/corvint/internal/releasegate"
)

func TestParseRequiresCanonicalManifestPath(t *testing.T) {
	if _, err := parse([]string{"--commit", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}); err == nil {
		t.Fatal("missing manifest accepted")
	}
	options, err := parse([]string{"--commit", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--manifest", "release-gate-manifest.json", "--git", "/usr/bin/git", "--git-sha256", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--policy", "/tmp/release-policy.json", "--policy-sha256", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"})
	if err != nil || options.ManifestPath != "release-gate-manifest.json" {
		t.Fatalf("options=%+v err=%v", options, err)
	}
}

func TestBlockingReportHasNonzeroCLIOutcome(t *testing.T) {
	report := releasegate.Report{Checks: releasegate.Checks{Parity: releasegate.NotRun, Safety: releasegate.Pass, Performance: releasegate.NotRun, Packaging: releasegate.NotRun, CorvintDogfood: releasegate.NotRun, BeamfallDogfood: releasegate.NotRun}}
	if !report.Blocking() {
		t.Fatal("incomplete experimental evidence was not blocking")
	}
	if code := reportOutcome(report); code != 1 {
		t.Fatalf("code=%d", code)
	}
}
