package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// restoreSource is a store holding one attachment-bearing ticket and the
// path of its archive export.
func restoreSource(t *testing.T) (*fixture.Repo, string, []byte) {
	t.Helper()
	r, id := attachStore(t)
	if x := atm(t, r.Root, nil, attachArgs(id, "1", "ev-1", "restored log")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("attach-evidence: %s", x.stdout)
	}
	x := atm(t, r.Root, nil, "archive", "export")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("export: %s", x.stderr)
	}
	path := filepath.Join(fixture.TempDirOutside(t), "queue.tar")
	fixture.Write(t, path, x.stdout)
	return r, path, x.stdout
}

// treeBytes maps every regular file under root to its bytes.
func treeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(p)
		out[strings.TrimPrefix(p, root)] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameBytes(t *testing.T, what string, want, got map[string]string) {
	t.Helper()
	for p, w := range want {
		if g, ok := got[p]; !ok || g != w {
			t.Errorf("%s %s: restored=%t, bytes equal=%t", what, p, ok, g == w)
		}
	}
	for p := range got {
		if _, ok := want[p]; !ok {
			t.Errorf("%s %s: restored but not in the original", what, p)
		}
	}
}

// TestTRLV0871_RestoreRoundTripsByteIdentically (TRL-V0-871, V1-3554): an
// exported store whose state dir and ticket files are gone restores from its
// archive to the same ticket and receipt bytes, re-exports to the same
// archive bytes, and audits CONSISTENT.
func TestTRLV0871_RestoreRoundTripsByteIdentically(t *testing.T) {
	r, path, archived := restoreSource(t)
	state, intentTree := treeBytes(t, r.StateDir), treeBytes(t, r.IntentDir)
	if err := os.RemoveAll(r.StateDir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(r.IntentDir, "tickets")); err != nil {
		t.Fatal(err)
	}
	x := atm(t, r.Root, nil, "archive", "restore", path)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Codes) != 0 || len(x.res.Warnings) != 0 || field(x.res.Items[0], "intentWritten").Str == "0" {
		t.Fatalf("restore: %s", x.stdout)
	}
	sameBytes(t, "state", state, treeBytes(t, r.StateDir))
	sameBytes(t, "intent", intentTree, treeBytes(t, r.IntentDir))
	again := atm(t, r.Root, nil, "archive", "export")
	if again.res.Outcome != wire.OutcomeOK || !bytes.Equal(again.stdout, archived) {
		t.Fatalf("re-export differs from the restored archive (%d vs %d bytes): %s", len(again.stdout), len(archived), again.stderr)
	}
	if a := atm(t, r.Root, nil, "receipt", "audit"); field(a.res.Items[0], "structuralConsistency").Str != "CONSISTENT" ||
		field(a.res.Items[0], "projectionAgreement").Str != "AGREES" {
		t.Fatalf("audit after restore: %s", a.stdout)
	}
}

// TestTRLV0870_RestoreRefusalsWriteNothing (TRL-V0-870, V1-3554): a damaged,
// tampered, absent or misnamed archive, an existing store and an intent root
// that disagrees with the archive each refuse with a typed code in the
// top-level codes, writing nothing and taking no lock.
func TestTRLV0870_RestoreRefusalsWriteNothing(t *testing.T) {
	src, path, archived := restoreSource(t)
	record, err := os.ReadFile(filepath.Join(src.IntentDir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	at := bytes.Index(archived, record)
	if at < 0 {
		t.Fatal("archive does not carry queue.json")
	}
	tampered := bytes.Clone(archived)
	tampered[at+len(record)/2] ^= 0x01
	dir := fixture.TempDirOutside(t)
	write := func(name string, raw []byte) string {
		p := filepath.Join(dir, name)
		fixture.Write(t, p, raw)
		return p
	}
	cases := []struct {
		name  string
		args  []string
		setup func(r *fixture.Repo)
		code  string
	}{
		{"no argument", nil, nil, wire.CodeMalformed},
		{"two arguments", []string{path, path}, nil, wire.CodeMalformed},
		{"flag", []string{"--staging", dir}, nil, wire.CodeMalformed},
		{"absent file", []string{filepath.Join(dir, "absent.tar")}, nil, wire.CodeUnsupportedFilesystem},
		{"truncated", []string{write("short.tar", archived[:len(archived)/2])}, nil, wire.CodeMalformed},
		{"tampered member", []string{write("tampered.tar", tampered)}, nil, wire.CodeMalformed},
		{"existing state dir", []string{path}, func(r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.StateDir, "VERSION"), []byte("x"))
		}, wire.CodeUnsupported},
		{"another queue", []string{path}, func(r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), append(bytes.Clone(record), '\n'))
		}, wire.CodeIntentDiverged},
		{"extra intent file", []string{path}, func(r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "stray.json"), []byte("{}"))
		}, wire.CodeIntentDiverged},
	}
	for _, c := range cases {
		r := fixture.TempRepo(t)
		if c.setup != nil {
			c.setup(r)
		}
		state, intentTree := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
		x := atm(t, r.Root, nil, append([]string{"archive", "restore"}, c.args...)...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, c.code) {
			t.Errorf("%s: want %s in codes: %s", c.name, c.code, x.stdout)
		}
		fixture.AssertUntouched(t, r, state, intentTree, c.name)
	}
	// The source store itself holds a lock file from init; its trees stay put.
	state, intentTree := fixture.TreeSnapshot(t, src.StateDir), fixture.TreeSnapshot(t, src.IntentDir)
	if x := atm(t, src.Root, nil, "archive", "restore", path); !hasCode(x.res, wire.CodeUnsupported) {
		t.Fatalf("restore over a live store: %s", x.stdout)
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, src.StateDir)) || !fixture.SameTree(intentTree, fixture.TreeSnapshot(t, src.IntentDir)) {
		t.Fatal("restore over a live store changed it")
	}
}

// TestTRLV0871_RestoreAtANewPathThenRelocate (TRL-V0-871, V1-3554): an
// archive restored into another repository warns with the relocate step,
// and relocate from the recorded path binds the store there.
func TestTRLV0871_RestoreAtANewPathThenRelocate(t *testing.T) {
	src, path, _ := restoreSource(t)
	old := src.Root
	if err := os.RemoveAll(old); err != nil {
		t.Fatal(err)
	}
	r := fixture.TempRepo(t)
	x := atm(t, r.Root, nil, "archive", "restore", path)
	if x.res.Outcome != wire.OutcomeOK || !warningsContain(x.res.Warnings, "corvint-tasks relocate --from "+old) {
		t.Fatalf("restore at a new path: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "queue", "status"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("unrelocated read: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "relocate", "--from", old, "--request-id", "relocate-1"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("relocate: %s", x.stdout)
	}
	if got := headPrimary(t, r.StateDir); got != r.Root {
		t.Fatalf("head primary %q", got)
	}
	if a := atm(t, r.Root, nil, "receipt", "audit"); field(a.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("audit after relocate: %s", a.stdout)
	}
}
