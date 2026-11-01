package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// distinctRegistrations gives the two registrations of reRegisteredPair the
// distinct file digests real files with different scorer digests have.
func distinctRegistrations(run, prior *report) {
	prior.Registration.SHA256 = digestOf("earlier registration")
	run.Registration.SHA256 = digestOf("current registration")
}

// V1-2624: the seven Codex round findings on the CRT-V0-016 reuse path, each
// failing before the fix.
func TestReuseCodexRoundFindings(t *testing.T) {
	t.Run("CRT-V0-016/registrations committed together are still compared", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		distinctRegistrations(run, prior)
		prior.Registration.Commit = run.Registration.Commit
		prior.Registration.Document.Alpha = 0.1
		if err := reRegister(run, prior, "prior.json"); err == nil || !strings.Contains(err.Error(), "beyond the scorer digest") {
			t.Fatalf("same-commit registrations differing beyond the scorer: err = %v", err)
		}
		run, prior = reRegisteredPair(t)
		distinctRegistrations(run, prior)
		prior.Registration.Commit = run.Registration.Commit
		if err := reRegister(run, prior, "prior.json"); err != nil || run.ReRegistered == nil {
			t.Fatalf("same-commit scorer-only re-registration not recorded: err = %v", err)
		}
	})
	t.Run("CRT-V0-016/a stray re_registered block on an unregistered report is checked", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		if err := reRegister(run, prior, "prior.json"); err != nil {
			t.Fatal(err)
		}
		run.Registration, run.Partition = nil, "pilot"
		finalize(run)
		if !strings.Contains(run.Invalid, "a re-registered run carries no registration") {
			t.Fatalf("invalid = %q", run.Invalid)
		}
	})
	t.Run("CRT-V0-016/reuse under an unchanged manifest is told apart from resume", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		distinctRegistrations(run, prior)
		prior.ManifestSHA256 = run.ManifestSHA256
		if err := reRegister(run, prior, "prior.json"); err != nil {
			t.Fatal(err)
		}
		record := &lane{ID: run.Lanes[0].ID, Arm: run.Lanes[0].Arm, Repeat: run.Lanes[0].Repeat, PromptSHA256: prior.Lanes[0].PromptSHA256}
		if !newReuseSource(prior).apply(record) || record.ReusedRegistration != prior.Registration.SHA256 {
			t.Fatalf("reused lane provenance: %+v", record)
		}
		*run.Lanes[0] = *record
		finalize(run)
		if run.Invalid != "" {
			t.Fatalf("reused lane checked against this registration: invalid = %q", run.Invalid)
		}
		if ownLane(run, run.Lanes[0]) || !ownLane(run, run.Lanes[1]) {
			t.Fatal("ownLane does not separate the reused lane from the run's own")
		}
	})
	t.Run("CRT-V0-016/resume restores the checkpoint's re_registered record", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		distinctRegistrations(run, prior)
		checkpoint, _ := reRegisteredPair(t)
		distinctRegistrations(checkpoint, prior)
		if err := reRegister(checkpoint, prior, "prior.json"); err != nil {
			t.Fatal(err)
		}
		if err := resumeReRegistration(run, checkpoint, "out.partial.json"); err != nil || run.ReRegistered == nil ||
			run.ReRegistered.Registration.SHA256 != prior.Registration.SHA256 {
			t.Fatalf("checkpoint record not restored: err = %v, %+v", err, run.ReRegistered)
		}
		other, _ := reRegisteredPair(t)
		other.ReRegistered = &reRegistration{ManifestSHA256: digestOf("another"), Registration: *prior.Registration}
		if err := resumeReRegistration(run, other, "out.partial.json"); err == nil {
			t.Fatal("a second, different earlier registration was accepted")
		}
	})
	t.Run("CRT-V0-016/an ineligible reuse lane does not shadow an eligible checkpoint lane", func(t *testing.T) {
		key := lane{ID: "c01", Arm: "control", Repeat: 1, PromptSHA256: "p"}
		stale := key
		stale.PromptSHA256, stale.WallMs, stale.ExitCode = "older prompt", int64(5), 0
		good := key
		good.WallMs, good.ExitCode, good.Reply, good.StartedAt = int64(7), 0, "reply", "2026-10-06T00:00:00Z"
		reused := &report{ManifestSHA256: digestOf("reused"), Lanes: []*lane{&stale}}
		checkpoint := &report{ManifestSHA256: digestOf("current"), Lanes: []*lane{&good}}
		record := key
		if !newReuseSource(reused).merge(checkpoint).apply(&record) || record.Reply != "reply" || record.ReusedFrom != checkpoint.ManifestSHA256 {
			t.Fatalf("checkpoint lane shadowed: %+v", record)
		}
	})
	t.Run("CRT-V0-016/never-started pending lanes do not block a re-registration", func(t *testing.T) {
		run, prior := reRegisteredPair(t)
		prior.Lanes[1].StartedAt, prior.Lanes[1].WallMs, prior.Lanes[1].ExitCode = "", notObserved, notObserved
		if err := reRegister(run, prior, "prior.json"); err != nil {
			t.Fatalf("pending lane counted as unstamped: %v", err)
		}
		run, prior = reRegisteredPair(t)
		prior.Lanes[1].StartedAt = ""
		if err := reRegister(run, prior, "prior.json"); err == nil || !strings.Contains(err.Error(), "order is unobserved") {
			t.Fatalf("completed unstamped lane: err = %v", err)
		}
	})
}

// V1-2624: select presents the patch gitauth CanonicalDiff derives, byte for
// byte, under a repository configured with diff.noprefix=true and an
// environment carrying GIT_DIFF_OPTS=--unified=7.
func TestSelectPatchMatchesCanonicalDiffUnderHostileConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	ctx := context.Background()
	gitIn := func(arguments ...string) string {
		t.Helper()
		out, err := git(ctx, root, arguments...)
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(arguments, " "), err)
		}
		return strings.TrimSpace(out)
	}
	gitIn("init", "-q", "-b", "main")
	writeFixtureFile(t, root, "internal/play/play.go", baseSource)
	gitIn("add", "-A")
	gitIn("commit", "-q", "-m", "base")
	writeFixtureFile(t, root, "internal/play/play.go", changedSource)
	gitIn("add", "-A")
	gitIn("commit", "-q", "-m", "change")
	for _, setting := range [][2]string{{"diff.noprefix", "true"}, {"diff.mnemonicPrefix", "true"}, {"diff.context", "7"}} {
		gitIn("config", setting[0], setting[1])
	}
	parent, commit := gitIn("rev-parse", "HEAD~1"), gitIn("rev-parse", "HEAD")
	// A hostile environment too: GIT_DIFF_OPTS widens the context and a
	// GIT_CONFIG_PARAMETERS entry asks for no prefix (Codex round 2).
	t.Setenv("GIT_DIFF_OPTS", "--unified=7")
	t.Setenv("GIT_CONFIG_PARAMETERS", "'diff.noprefix'='true'")
	presented, err := canonicalPatch(ctx, root, parent, commit, []string{"internal/play/play.go"})
	if err != nil {
		t.Fatal(err)
	}
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := repository.CanonicalDiff(ctx, parent, commit)
	if err != nil {
		t.Fatal(err)
	}
	if presented != string(canonical) {
		t.Fatalf("presented patch differs from CanonicalDiff:\n--- presented\n%s\n--- canonical\n%s", presented, canonical)
	}
	if !strings.Contains(presented, "--- a/internal/play/play.go") {
		t.Fatalf("hostile diff.noprefix leaked into the patch:\n%s", presented)
	}
}
