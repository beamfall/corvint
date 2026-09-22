package parentverify

import (
	"context"
	json "encoding/json/v2"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

// statusFixture wraps the verifier fixture so one test can control the exact
// `git status` bytes the authority observes, and count how many times it looks.
func statusFixture(t *testing.T, status string) (Config, provider.AuthorityRequest, CommandRunner, *int) {
	t.Helper()
	config, request, base := verifierFixture(t)
	captures := 0
	run := func(ctx context.Context, executable string, argv, environment []string, cwd string, timeout time.Duration) (CommandResult, error) {
		if executable != config.GitExecutable || !strings.Contains(strings.Join(argv, " "), "status --porcelain") {
			return base(ctx, executable, argv, environment, cwd, timeout)
		}
		captures++
		return CommandResult{
			Stdout: []byte(status), ExitCode: 0, Started: true, Exited: true,
			ProcessCleanupDone: true, PipesDrained: true, WaitCompleted: true,
		}, nil
	}
	return config, request, run, &captures
}

// TestPublishedDirtyPathsAreDecodedFromTheHashedStatusBytes is the atomicity
// claim: the published list is what the existing status capture saw, not a
// fresh look at a worktree that may have moved since.
func TestPublishedDirtyPathsAreDecodedFromTheHashedStatusBytes(t *testing.T) {
	status := " M probe.go\x00?? added.go\x00R  moved.go\x00origin.go\x00"
	config, request, run, captures := statusFixture(t, status)
	snapshot, err := Acquire(t.Context(), config, request, run)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Repository.StatusSHA256 != hashRaw([]byte(status)) {
		t.Fatalf("StatusSHA256 does not commit to the fixture status bytes")
	}
	want, err := affected.DecodeStatus([]byte(status))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot.DirtyPaths, want) {
		t.Fatalf("published dirty paths = %v, hashed status decodes to %v", snapshot.DirtyPaths, want)
	}
	// A rename must contribute both endpoints, exactly as affected/dirty.go
	// decodes it: the unit that lost the file changed too.
	if !reflect.DeepEqual(want, []string{"added.go", "moved.go", "origin.go", "probe.go"}) {
		t.Fatalf("rename origin handling drifted: %v", want)
	}
	// Publishing the list must not add an observation. The two captures are the
	// pair that already bracketed the snapshot and are proven equal by the
	// drift refusal in observe.
	if *captures != 2 {
		t.Fatalf("status captures = %d, want the 2 bracketing observations", *captures)
	}
}

// TestPublishedDirtyPathsSurviveTheDriftBracket proves the published list comes
// from an observation the drift check accepted, so a worktree that changes
// between the two captures yields no list at all rather than a stale one.
func TestPublishedDirtyPathsSurviveTheDriftBracket(t *testing.T) {
	config, request, base := verifierFixture(t)
	capture := 0
	run := func(ctx context.Context, executable string, argv, environment []string, cwd string, timeout time.Duration) (CommandResult, error) {
		if executable != config.GitExecutable || !strings.Contains(strings.Join(argv, " "), "status --porcelain") {
			return base(ctx, executable, argv, environment, cwd, timeout)
		}
		capture++
		status := " M probe.go\x00"
		if capture > 1 {
			status = " M probe.go\x00?? raced.go\x00"
		}
		return CommandResult{
			Stdout: []byte(status), ExitCode: 0, Started: true, Exited: true,
			ProcessCleanupDone: true, PipesDrained: true, WaitCompleted: true,
		}, nil
	}
	if _, err := Acquire(t.Context(), config, request, run); err == nil || !strings.Contains(err.Error(), "drift") {
		t.Fatalf("a worktree that moved mid-observation was accepted: %v", err)
	}
}

// TestMalformedStatusRefusesTheSnapshot keeps the capture fail-closed: a status
// stream Corvint cannot decode exactly must not yield a snapshot carrying a
// silently short dirty list.
func TestMalformedStatusRefusesTheSnapshot(t *testing.T) {
	for name, status := range map[string]string{
		"truncated record":      "M\x00",
		"rename without origin": "R  moved.go\x00",
		"absolute path":         " M /etc/passwd\x00",
		"parent traversal":      " M ../escape.go\x00",
	} {
		t.Run(name, func(t *testing.T) {
			config, request, run, _ := statusFixture(t, status)
			if _, err := Acquire(t.Context(), config, request, run); err == nil {
				t.Fatalf("undecodable status %q was accepted", status)
			}
		})
	}
}

// TestRepositoryCommitmentFieldsAreFrozen guards the canonical bytes. Repository
// is marshalled into the workspace-source body that yields the source
// observation identity, so any added or renamed field silently changes every
// lease and receipt digest. The dirty path list is published on Snapshot for
// exactly this reason.
func TestRepositoryCommitmentFieldsAreFrozen(t *testing.T) {
	body, err := json.Marshal(Repository{}, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(decoded))
	for name := range decoded {
		names = append(names, name)
	}
	sort.Strings(names)
	want := []string{
		"configSha256", "gitExeSha256", "gitVersionSha256", "headRevision", "indexSha256",
		"layoutSha256", "objectFormat", "statusSha256", "treeRevision",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("repository canonical fields = %v, frozen set is %v", names, want)
	}
}
