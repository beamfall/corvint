package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// reRegisteredPair is a run registered at 2026-10-05 whose first lane is
// reused from a prior report registered at 2026-10-01 under another scorer
// digest, and whose second lane ran fresh on 2026-10-06.
func reRegisteredPair(t *testing.T) (*report, *report) {
	t.Helper()
	prior := registeredReport("2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z")
	prior.ManifestSHA256 = digestOf("earlier manifest")
	prior.Registration.Commit = strings.Repeat("b", 40)
	prior.Registration.Document.ScorerSHA256 = digestOf("earlier scorer")
	run := registeredReport("2026-10-06T00:00:00Z", "2026-10-05T00:00:00Z")
	run.ManifestSHA256 = digestOf("current manifest")
	return run, prior
}

func reuseFirstLane(run, prior *report) {
	run.Lanes[0].StartedAt = prior.Lanes[0].StartedAt
	run.Lanes[0].ReusedFrom = prior.ManifestSHA256
}

// CRT-V0-016: lanes may be reused across a re-registration that changes only
// the scorer digest; each reused lane answers to the registration it ran
// under, and every other difference is refused.
func TestReuseAcrossAReRegistration(t *testing.T) {
	t.Run("CRT-V0-016/scorer-only re-registration is valid", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		if err := reRegister(run, prior, "prior.json"); err != nil {
			t.Fatal(err)
		}
		reuseFirstLane(run, prior)
		finalize(run)
		if run.Invalid != "" {
			t.Fatalf("invalid = %q", run.Invalid)
		}
	})
	t.Run("CRT-V0-016/without the re-registration the reused lane predates this one", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		reuseFirstLane(run, prior)
		finalize(run)
		if !strings.Contains(run.Invalid, "is newer than the first lane") {
			t.Fatalf("invalid = %q", run.Invalid)
		}
	})
	t.Run("CRT-V0-016/a difference beyond the scorer is refused", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		prior.Registration.Document.Alpha = 0.1
		if err := reRegister(run, prior, "prior.json"); err == nil || !strings.Contains(err.Error(), "beyond the scorer digest") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("CRT-V0-016/the same scorer is not a re-registration", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		prior.Registration.Document.ScorerSHA256 = run.Registration.Document.ScorerSHA256
		if err := reRegister(run, prior, "prior.json"); err == nil || !strings.Contains(err.Error(), "same scorer digest") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("CRT-V0-016/an earlier registration newer than its own lanes is refused", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		prior.Registration.CommittedAt = "2026-10-03T00:00:00Z"
		if err := reRegister(run, prior, "prior.json"); err == nil || !strings.Contains(err.Error(), "newer than that run's first lane") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("CRT-V0-016/a lane from another report is invalid", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		if err := reRegister(run, prior, "prior.json"); err != nil {
			t.Fatal(err)
		}
		reuseFirstLane(run, prior)
		run.Lanes[0].ReusedFrom = digestOf("a third manifest")
		finalize(run)
		if !strings.Contains(run.Invalid, "reused from a report other than the re-registered one") {
			t.Fatalf("invalid = %q", run.Invalid)
		}
	})
	t.Run("CRT-V0-016/a resumed checkpoint lane answers to this registration", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		if err := reRegister(run, prior, "prior.json"); err != nil {
			t.Fatal(err)
		}
		reuseFirstLane(run, prior)
		run.Lanes[1].ReusedFrom = run.ManifestSHA256
		run.Lanes[1].StartedAt = "2026-10-04T00:00:00Z"
		finalize(run)
		if !strings.Contains(run.Invalid, "is newer than the first lane (2026-10-04T00:00:00Z)") {
			t.Fatalf("invalid = %q", run.Invalid)
		}
	})
	t.Run("CRT-V0-016/rescore re-checks the carried registration", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		if err := reRegister(run, prior, "prior.json"); err != nil {
			t.Fatal(err)
		}
		reuseFirstLane(run, prior)
		run.ReRegistered.Registration.Document.Alpha = 0.1
		path := filepath.Join(t.TempDir(), "report.json")
		var stdout, stderr bytes.Buffer
		if code := write(run, path, &stdout, &stderr); code != 0 {
			t.Fatal(stderr.String())
		}
		rebuilt, _, err := rescore([]string{"--report", path})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(rebuilt.Invalid, "beyond the scorer digest") {
			t.Fatalf("rescored invalid = %q", rebuilt.Invalid)
		}
	})
}
