package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

var sharedObservationEvents = map[string]struct{ event, input string }{
	"file-change":           {"file-change", `{"paths":["internal/auth/session.go"]}`},
	"compact-session-start": {"session-start", `{"startSource":"compact"}`},
	"stop":                  {"stop", `{}`},
	"startup-session-start": {"session-start", `{"startSource":"startup"}`},
	"post-tool":             {"post-tool", `{"changedPaths":["internal/auth/session.go"]}`},
}

// sharedObservationStates prepare one fixture repository per state the
// proposed GPK-V0-058 parity claim covers.
var sharedObservationStates = map[string]func(t *testing.T, root string){
	"clean-snapshot-absent": func(*testing.T, string) {},
	"clean-snapshot-present": func(t *testing.T, root string) {
		writeFixtureSnapshot(t, root)
	},
	"dirty-snapshot-absent": func(t *testing.T, root string) {
		dirtyFixture(t, root)
	},
	"dirty-snapshot-present": func(t *testing.T, root string) {
		writeFixtureSnapshot(t, root)
		dirtyFixture(t, root)
	},
	"stale-snapshot": func(t *testing.T, root string) {
		writeFixtureSnapshot(t, root)
		dirtyFixture(t, root)
		fixtureGit(t, root, "add", "internal/auth/session.go")
		fixtureGit(t, root, "commit", "-qm", "change tree")
	},
}

func writeFixtureSnapshot(t *testing.T, root string) {
	t.Helper()
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contextindex.WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
}

func dirtyFixture(t *testing.T, root string) {
	t.Helper()
	tracked := filepath.Join(root, "internal", "auth", "session.go")
	if err := os.WriteFile(tracked, []byte("package auth\n\n// feature:session-revocation\nfunc EnforceSessionRevocation() bool { return false }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureGit(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}

func runHarnessEvent(t *testing.T, root, event, input string, environment ...string) []byte {
	t.Helper()
	stdout, stderr, code := runCandidateWithEnvironment(t, input, environment, cliArguments(root, event)...)
	if code != 0 {
		t.Fatalf("%s exit %d: %s", event, code, stderr)
	}
	return stdout
}

// TestHarnessSharedObservationEmitsByteIdenticalReceipts pins proposed
// GPK-V0-058: over every fixture state, the shared-observation path emits the
// same bytes as the default path for both index-building events and the
// no-index stop event.
func TestHarnessSharedObservationEmitsByteIdenticalReceipts(t *testing.T) {
	t.Parallel()
	for stateName, prepare := range sharedObservationStates {
		for eventName, event := range sharedObservationEvents {
			t.Run(stateName+"/"+eventName, func(t *testing.T) {
				root := ledgerRepository(t)
				prepare(t, root)
				defaultBytes := runHarnessEvent(t, root, event.event, event.input, "CORVINT_HARNESS_SHARED_OBSERVATION=")
				sharedBytes := runHarnessEvent(t, root, event.event, event.input, "CORVINT_HARNESS_SHARED_OBSERVATION=1")
				if !bytes.Equal(defaultBytes, sharedBytes) {
					t.Fatalf("GPK-V0-058: receipts differ\ndefault: %s\nshared:  %s", defaultBytes, sharedBytes)
				}
				if !bytes.Contains(defaultBytes, []byte(`"receiptId":"`)) {
					t.Fatalf("no receipt: %s", defaultBytes)
				}
			})
		}
	}
}

// countingGit puts a `git` shim first on PATH that logs every invocation before
// running the real binary, so the spawn count covers the kernel's bracket and
// the index loader alike.
func countingGit(t *testing.T) (string, func() int) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	log := filepath.Join(directory, "spawns.log")
	shim := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	path := "PATH=" + directory + string(os.PathListSeparator) + os.Getenv("PATH")
	return path, func() int {
		raw, err := os.ReadFile(log)
		if err != nil {
			return 0
		}
		count := bytes.Count(raw, []byte("\n"))
		if err := os.Remove(log); err != nil {
			t.Fatal(err)
		}
		return count
	}
}

// TestHarnessSharedObservationSpawnCount pins the proposed GPK-V0-058 counts:
// with a snapshot present, file-change and compact session-start spawn eight
// Git processes on the default path and four on the shared path; stop, which
// loads no index and emits no profile, spawns five and four; the startup
// session-start, the one event that emits the profile, keeps its `ls-tree`
// and spawns five on both. A stale snapshot spawns three fewer on the shared
// path (the loader's identity and status reads and the profile read) while
// the full build's own bracket remains; a repository with no snapshot
// directory spawns one fewer, because the loader already stats the directory
// before it asks Git for anything.
func TestHarnessSharedObservationSpawnCount(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh for the git shim")
	}
	cases := map[string]struct {
		state           string
		event           string
		wantDefault     int
		wantShared      int
		sharedIsFewerBy int
	}{
		"file-change/snapshot-present":           {"clean-snapshot-present", "file-change", 8, 4, 0},
		"compact-session-start/snapshot-present": {"clean-snapshot-present", "compact-session-start", 8, 4, 0},
		"compact-session-start/dirty-snapshot":   {"dirty-snapshot-present", "compact-session-start", 8, 4, 0},
		"stop":                                   {"clean-snapshot-present", "stop", 5, 4, 0},
		"startup-session-start":                  {"clean-snapshot-present", "startup-session-start", 5, 5, 0},
		"file-change/stale-snapshot":             {"stale-snapshot", "file-change", 0, 0, 3},
		"file-change/snapshot-absent":            {"clean-snapshot-absent", "file-change", 10, 9, 0},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			root := ledgerRepository(t)
			sharedObservationStates[test.state](t, root)
			event := sharedObservationEvents[test.event]
			path, spawns := countingGit(t)
			runHarnessEvent(t, root, event.event, event.input, path, "CORVINT_HARNESS_SHARED_OBSERVATION=")
			defaultSpawns := spawns()
			runHarnessEvent(t, root, event.event, event.input, path, "CORVINT_HARNESS_SHARED_OBSERVATION=1")
			sharedSpawns := spawns()
			if test.sharedIsFewerBy != 0 {
				if defaultSpawns-sharedSpawns != test.sharedIsFewerBy {
					t.Fatalf("GPK-V0-058: default %d, shared %d spawns, want shared fewer by %d", defaultSpawns, sharedSpawns, test.sharedIsFewerBy)
				}
				return
			}
			if defaultSpawns != test.wantDefault || sharedSpawns != test.wantShared {
				t.Fatalf("GPK-V0-058: default %d (want %d), shared %d (want %d) Git spawns", defaultSpawns, test.wantDefault, sharedSpawns, test.wantShared)
			}
		})
	}
}
