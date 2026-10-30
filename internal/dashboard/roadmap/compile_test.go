package roadmap

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// writeFakeAtm writes a tiny POSIX-shell stand-in for `atm` that this test
// package fully controls, so compile.go's envelope parsing, refusal
// handling, and per-ticket failure isolation can be exercised without a
// real planning store. `atm roadmap` reports two tickets in milestone M0;
// `atm ticket show` succeeds for the first and fails for the second, so one
// bad ticket's NOT_OBSERVED blocker never hides the other ticket's real
// evidence. Its behavior for `roadmap` is controlled by $ATM_FAKE_MODE.
func writeFakeAtm(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake atm fixture is a POSIX shell script")
	}
	script := `#!/bin/sh
mode="${ATM_FAKE_MODE:-ok}"
if [ "$1" = "roadmap" ]; then
  case "$mode" in
    refused)
      echo '{"profile":"taskman-command-result/0","outcome":"REFUSED","codes":["X"],"items":[]}'
      ;;
    bad-json)
      echo 'not json'
      ;;
    bad-profile)
      echo '{"profile":"wrong-profile/0","outcome":"OK","items":[]}'
      ;;
    ok-nonzero)
      echo '{"profile":"taskman-command-result/0","outcome":"OK","items":[]}'
      exit 3
      ;;
    ok-stderr)
      echo '{"profile":"taskman-command-result/0","outcome":"OK","items":[]}' >&2
      ;;
    *)
      echo '{"profile":"taskman-command-result/0","outcome":"OK","items":[{"ticketId":"ticket:corvint:planning:T-1","title":"First","milestone":"M0","status":"OPEN","eligibility":"BLOCKED","owner":"owner-a","priority":"P1"},{"ticketId":"ticket:corvint:planning:T-2","title":"Second","milestone":"M0","status":"OPEN","eligibility":"BLOCKED","owner":"owner-b","priority":"P2"}]}'
      ;;
  esac
  exit 0
fi
if [ "$1" = "ticket" ] && [ "$2" = "show" ]; then
  case "$3" in
    *T-2)
      exit 7
      ;;
  esac
  echo '{"profile":"taskman-command-result/0","outcome":"OK","items":[{"blockers":[{"code":"TICKET_STATE","detail":"executionClass MANUAL is never autonomously eligible"}],"record":{"requirementRefs":["TCP-V0-001","UNKNOWN-REF"]}}]}'
  exit 0
fi
exit 1
`
	path := filepath.Join(t.TempDir(), "fake-atm.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fixtureRequirementsPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "REQUIREMENTS.tsv")
	content := "id\tfile\tline\ttitle\nTCP-V0-001\tdocs/specs/task-coordination-protocol-v0.md\t42\tSome requirement title\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func baseOptions(t *testing.T) Options {
	return Options{
		AtmBinary: writeFakeAtm(t), StoreRoot: t.TempDir(),
		RequirementsTSV: fixtureRequirementsPath(t),
		GeneratedAt:     "2026-09-11T00:00:00.000000000Z",
		Timeout:         10 * time.Second,
	}
}

func TestCompileJoinsRequirementsAndIsolatesPerTicketFailure(t *testing.T) {
	options := baseOptions(t)
	snapshot, _, err := Compile(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Outcome != "OK" {
		t.Fatalf("outcome = %q, reason = %q", snapshot.Outcome, snapshot.Reason)
	}
	if len(snapshot.Milestones) != 1 || snapshot.Milestones[0].Milestone != "M0" {
		t.Fatalf("milestones = %+v", snapshot.Milestones)
	}
	tickets := snapshot.Milestones[0].Tickets
	if len(tickets) != 2 {
		t.Fatalf("tickets = %+v", tickets)
	}
	// Sorted by LocalID: T-1 before T-2.
	first, second := tickets[0], tickets[1]
	if first.LocalID != "T-1" || second.LocalID != "T-2" {
		t.Fatalf("ticket order = %q, %q", first.LocalID, second.LocalID)
	}
	if len(first.Evidence) != 2 || !first.Evidence[0].Resolved || first.Evidence[0].File == "" {
		t.Fatalf("first evidence = %+v", first.Evidence)
	}
	if first.Evidence[1].Resolved {
		t.Fatalf("unknown requirement ref resolved unexpectedly: %+v", first.Evidence[1])
	}
	if len(second.Blockers) != 1 || second.Blockers[0].Code != notObserved {
		t.Fatalf("second (failed ticket show) blockers = %+v", second.Blockers)
	}
}

func TestCompileWholeSnapshotNotObservedWhenAtmRefuses(t *testing.T) {
	options := baseOptions(t)
	snapshot, _, err := Compile(context.Background(), withFakeMode(t, options, "refused"))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Outcome != notObserved || snapshot.Reason == "" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if len(snapshot.Milestones) != 0 {
		t.Fatalf("milestones = %+v, want none", snapshot.Milestones)
	}
}

func TestCompileWholeSnapshotNotObservedOnMalformedEnvelope(t *testing.T) {
	options := baseOptions(t)
	snapshot, _, err := Compile(context.Background(), withFakeMode(t, options, "bad-json"))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Outcome != notObserved || snapshot.Reason == "" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestCompileWholeSnapshotNotObservedOnWrongProfile(t *testing.T) {
	options := baseOptions(t)
	snapshot, _, err := Compile(context.Background(), withFakeMode(t, options, "bad-profile"))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Outcome != notObserved || snapshot.Reason == "" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

// LOD-V0-030: an OK envelope is admitted only on stdout with a zero exit.
func TestCompileWholeSnapshotNotObservedOnOKEnvelopeWithoutCleanExit(t *testing.T) {
	for _, mode := range []string{"ok-nonzero", "ok-stderr"} {
		t.Run(mode, func(t *testing.T) {
			snapshot, _, err := Compile(context.Background(), withFakeMode(t, baseOptions(t), mode))
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Outcome != notObserved || snapshot.Reason == "" {
				t.Fatalf("snapshot = %+v", snapshot)
			}
		})
	}
}

func TestCompileWholeSnapshotNotObservedWhenAtmBinaryAbsent(t *testing.T) {
	options := baseOptions(t)
	options.AtmBinary = filepath.Join(t.TempDir(), "does-not-exist-atm")
	snapshot, _, err := Compile(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Outcome != notObserved || snapshot.Reason == "" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestCompileIsDeterministic(t *testing.T) {
	options := baseOptions(t)
	_, first, err := Compile(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := Compile(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("compile is not deterministic:\n%s\nvs\n%s", first, second)
	}
}

// withFakeMode is a small helper so each refusal-mode test gets its own
// fixture binary carrying $ATM_FAKE_MODE baked into its own wrapper, since
// runBinary passes through the process environment rather than per-call
// overrides.
func withFakeMode(t *testing.T, options Options, mode string) Options {
	t.Helper()
	wrapper := filepath.Join(t.TempDir(), "fake-atm-wrapper.sh")
	content := "#!/bin/sh\nexport ATM_FAKE_MODE=" + mode + "\nexec \"" + options.AtmBinary + "\" \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	options.AtmBinary = wrapper
	return options
}

// TestCompileDirtyCheckUsesRepoRootNotStore is the dogfood repair for
// LOD-V0-032: the dirty check must run in the repository whose tree
// TreeDigest names, not in the planning store (which is a separate, usually
// clean, directory). A dirty repository with a clean store is dirty.
func TestCompileDirtyCheckUsesRepoRootNotStore(t *testing.T) {
	repo := initScratchRepo(t)
	options := baseOptions(t)
	options.RepoRoot = repo
	snapshot, _, err := Compile(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.WorktreeDirty {
		t.Fatal("clean repository reported dirty")
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot, _, err = Compile(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.WorktreeDirty {
		t.Fatal("dirty repository reported clean because the check ran in the store")
	}
}

// TestCompileEvidenceNotObservedWhenRequirementsTableUnreadable pins
// LOD-V0-031's missing-evidence rule: an unreadable REQUIREMENTS.tsv is not
// an empty table, so a ref must carry a reason instead of reading as an ID
// the table lacks.
func TestCompileEvidenceNotObservedWhenRequirementsTableUnreadable(t *testing.T) {
	options := baseOptions(t)
	options.RequirementsTSV = filepath.Join(t.TempDir(), "missing.tsv")
	snapshot, _, err := Compile(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Outcome != "OK" || len(snapshot.Milestones) != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	evidence := snapshot.Milestones[0].Tickets[0].Evidence
	if len(evidence) != 2 {
		t.Fatalf("evidence = %+v", evidence)
	}
	for _, link := range evidence {
		if link.Resolved || link.Reason != requirementsNotObserved {
			t.Fatalf("link = %+v, want unresolved with reason %q", link, requirementsNotObserved)
		}
	}
}
