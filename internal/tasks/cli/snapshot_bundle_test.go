package cli_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// snapshotFile writes a queue snapshot of r to a temp file.
func snapshotFile(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "queue-snapshot.json")
	if x := atm(t, root, nil, "queue", "snapshot", "--out", path); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "lastSeq").Str == "NOT_OBSERVED" {
		t.Fatalf("queue snapshot: %s", x.stdout)
	}
	return path
}

// TestTCLV0010_QueueSnapshot: queue snapshot writes the intent files with
// their digests, the tree digest, the source and the journal head, to
// --out or raw to stdout.
func TestTCLV0010_QueueSnapshot(t *testing.T) {
	r, _ := similarRepo(t)
	path := snapshotFile(t, r.Root)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var b struct {
		Profile, SourceHost, PrimaryWorktree, ProducedAt, TreeSha256 string
		Journal                                                      *struct{ LastSeq string }
		Files                                                        []struct{ Path, Sha256, Content string }
	}
	if err := json.Unmarshal(raw, &b); err != nil || b.Profile != "taskman-intent-snapshot/0" || b.PrimaryWorktree != r.Root || b.Journal == nil || len(b.TreeSha256) != 64 || len(b.Files) < 5 {
		t.Fatalf("snapshot: %v %+v", err, b)
	}
	var out bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: r.Root, Args: []string{"queue", "snapshot"}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: io.Discard}); code != 0 || !bytes.Contains(out.Bytes(), []byte(b.TreeSha256)) {
		t.Fatalf("stdout snapshot exit %d: %.200s", code, out.String())
	}
}

// TestTCLV0011_SnapshotReads: under CORVINT_TASKS_SNAPSHOT the inventory
// reads run outside any repository, name the snapshot's source, and a
// tampered snapshot is refused MALFORMED.
func TestTCLV0011_SnapshotReads(t *testing.T) {
	r, ids := similarRepo(t)
	path := snapshotFile(t, r.Root)
	lane := t.TempDir()
	t.Setenv(cli.SnapshotEnv, path)
	for _, args := range [][]string{
		{"ticket", "search", "--status", "OPEN"},
		{"ticket", "show", ids["lease"]},
		{"ticket", "export"},
		{"ticket", "similar", "--text", "lease heartbeat expires during gate runs"},
		{"ticket", "digest"},
		{"roadmap"},
		{"queue", "status"},
	} {
		x := atm(t, lane, nil, args...)
		w := strings.Join(x.res.Warnings, " ")
		if x.res.Outcome != wire.OutcomeOK || !strings.Contains(w, "served read-only from the "+cli.SnapshotEnv) || !strings.Contains(w, r.Root) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	if x := atm(t, lane, nil, "ticket", "search", "--status", "OPEN"); len(x.res.Items) != 3 {
		t.Fatalf("search: %s", x.stdout)
	}
	if x := atm(t, lane, nil, "ticket", "similar", "--text", "lease heartbeat expires during gate runs"); field(x.res.Items[0], "ticketId").Str != ids["lease"] {
		t.Fatalf("similar: %s", x.stdout)
	}
	var text bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: lane, Args: []string{"ticket", "digest", "--format", "text"}, Stdin: bytes.NewReader(nil), Stdout: &text, Stderr: io.Discard}); code != 0 || !strings.Contains(text.String(), "\n# served read-only from the "+cli.SnapshotEnv) {
		t.Fatalf("snapshot text digest exit %d: %s", code, text.String())
	}
	digest := filepath.Join(lane, "digest.txt")
	if err := os.WriteFile(digest, text.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if x := atm(t, lane, nil, "ticket", "similar", "--digest", digest, "--text", "lease heartbeat expires during gate runs"); x.res.Outcome != wire.OutcomeOK || len(x.res.Items) == 0 {
		t.Fatalf("similar against the snapshot digest: %s", x.stdout)
	}
	missing := atm(t, lane, nil, "ticket", "show", "XX-999")
	if missing.res.Outcome == wire.OutcomeOK || !strings.Contains(strings.Join(missing.res.Warnings, " "), "not in the "+cli.SnapshotEnv+" snapshot") {
		t.Fatalf("missing ticket: %s", missing.stdout)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	files := doc["files"].([]any)
	doc["files"] = append(files, files[0])
	repeated, _ := json.Marshal(doc)
	for name, bad := range map[string][]byte{
		"tampered": bytes.Replace(raw, []byte("lease heartbeat expires"), []byte("lease heartbeat EXPIRES"), 1),
		"repeated": repeated,
	} {
		if err := os.WriteFile(path, bad, 0o600); err != nil {
			t.Fatal(err)
		}
		if x := atm(t, lane, nil, "queue", "status"); x.res.Outcome != wire.OutcomeError || x.res.Codes[0] != wire.CodeMalformed || !strings.Contains(strings.Join(x.res.Warnings, " "), path) {
			t.Fatalf("%s snapshot: %s", name, x.stdout)
		}
	}
}

// TestTCLV0012_SnapshotRefusesWrites: every verb the snapshot does not
// serve is refused UNSUPPORTED, naming the owning store; version stays.
func TestTCLV0012_SnapshotRefusesWrites(t *testing.T) {
	r, ids := similarRepo(t)
	path := snapshotFile(t, r.Root)
	lane := t.TempDir()
	t.Setenv(cli.SnapshotEnv, path)
	for _, args := range [][]string{
		{"ticket", "create", "--request-id", "x", "--payload-stdin"},
		{"ticket", "recur", ids["lease"], "--context", "again", "--request-id", "y"},
		{"claim", ids["lease"]},
		{"ticket", "list"},
		{"queue", "snapshot"},
		{"receipt", "audit"},
	} {
		x := atm(t, lane, similarPayload("t", "b"), args...)
		w := strings.Join(x.res.Warnings, " ")
		if x.res.Outcome != wire.OutcomeError || x.res.Codes[0] != wire.CodeUnsupported || !strings.Contains(w, "cd "+r.Root) || !strings.Contains(w, "nothing was written") {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	if x := atm(t, lane, nil, "version"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("version: %s", x.stdout)
	}
}
