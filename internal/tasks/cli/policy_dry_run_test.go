package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// storeBytes maps every regular file under the given roots to its bytes and
// mode, so a read can be proved byte-identical.
func storeBytes(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if d.IsDir() {
				out[p] = "dir " + info.Mode().String()
				return nil
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out[p] = info.Mode().String() + " " + string(raw)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// TestTCLV0821_PolicyDryRunPreviewsFencesAndWritesNothing: `policy update
// --dry-run` returns the handoff fences the committed update then reports,
// and leaves the journal and intent tree byte-identical (TCL-V0-821).
func TestTCLV0821_PolicyDryRunPreviewsFencesAndWritesNothing(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	current := loaded.Policy.PolicyVersion.Uint64()
	id := planTicket(t, root, "current", "P1", `["src/current/"]`)
	claim := handoffCLI(t, root, "claim", id, "--holder", "current", "--stage", "implement", "--request-id", "claim-current")
	if claim.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", claim.stdout)
	}
	attempt := field(claim.res.Items[0], "attemptId").Str

	v, err := wire.Parse(loaded.Policy.Raw)
	if err != nil {
		t.Fatal(err)
	}
	next := wire.String(string(wire.SizeOf(current + 1)))
	v.Obj.Set("policyVersion", next)
	x, _ := v.Obj.Get("budgets")
	x.Obj.Set("ticketMultiplier", wire.String("5"))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(file, wire.EncodeFile(v), 0o600); err != nil {
		t.Fatal(err)
	}
	expected := string(wire.SizeOf(current))

	before := storeBytes(t, repo.StateDir, filepath.Join(root, intent.Dir))
	dry := handoffCLI(t, root, "policy", "update", "--dry-run", "--expected-policy-version", expected, "--file", file)
	if dry.res.Outcome != wire.OutcomeOK || len(dry.res.Items) != 1 || !field(dry.res.Items[0], "dryRun").Bool {
		t.Fatalf("dry run: %s", dry.stdout)
	}
	fences := field(dry.res.Items[0], "handoffFences")
	if fences.Kind != wire.KindArray || len(fences.Arr) != 1 || field(fences.Arr[0], "attemptId").Str != attempt || field(fences.Arr[0], "handoff").Str != "FENCED" {
		t.Fatalf("dry-run fences: %s", dry.stdout)
	}
	if field(dry.res.Items[0], "newPolicySha256").Str != string(wire.Sum(wire.EncodeFile(v))) {
		t.Fatalf("newPolicySha256: %s", dry.stdout)
	}
	// A wrong version and an undecodable file are refused, also without writing.
	if r := handoffCLI(t, root, "policy", "update", "--dry-run", "--expected-policy-version", string(wire.SizeOf(current+1)), "--file", file); r.res.Outcome == wire.OutcomeOK || !hasCode(r.res, wire.CodeMalformed) {
		t.Fatalf("wrong version: %s", r.stdout)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := handoffCLI(t, root, "policy", "update", "--dry-run", "--expected-policy-version", expected, "--file", bad); r.res.Outcome == wire.OutcomeOK {
		t.Fatalf("undecodable policy: %s", r.stdout)
	}
	after := storeBytes(t, repo.StateDir, filepath.Join(root, intent.Dir))
	if len(before) != len(after) {
		t.Fatalf("dry run changed the file set: %d -> %d", len(before), len(after))
	}
	for p, b := range before {
		if !bytes.Equal([]byte(b), []byte(after[p])) {
			t.Fatalf("dry run changed %s", p)
		}
	}

	// The committed update reports the same fence the dry run previewed.
	real := handoffCLI(t, root, "policy", "update", "--request-id", "budget", "--expected-policy-version", expected, "--file", file)
	got := field(real.res.Items[0], "handoffFences")
	if real.res.Outcome != wire.OutcomeOK || !bytes.Equal(wire.Encode(got), wire.Encode(fences)) {
		t.Fatalf("committed fences differ from the preview: %s", real.stdout)
	}
}

// TestTCLV0821_PolicyDryRunHeadAdvanceWritesNoClosedIndex: another writer
// advancing the head while the dry run previews does not make the dry run
// write the derived closed-ticket index; the store stays byte-identical
// (TCL-V0-821).
func TestTCLV0821_PolicyDryRunHeadAdvanceWritesNoClosedIndex(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	current := loaded.Policy.PolicyVersion.Uint64()
	v, err := wire.Parse(loaded.Policy.Raw)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set("policyVersion", wire.String(string(wire.SizeOf(current+1))))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(file, wire.EncodeFile(v), 0o600); err != nil {
		t.Fatal(err)
	}
	head := filepath.Join(repo.StateDir, "head.json")
	// The closed-index refresher sees a head advance as a changed size or
	// mtime of head.json, which is what a concurrent commit leaves.
	defer cli.SetPolicyDryRunHook(func() {
		later := time.Now().Add(time.Hour)
		if err := os.Chtimes(head, later, later); err != nil {
			t.Error(err)
		}
	})()
	// The refresher rewrites the index atomically even when its bytes are
	// unchanged, so the file's identity is compared as well as its bytes.
	index := intent.ClosedIndexPath(repo.StateDir)
	indexBefore, indexErr := os.Stat(index)
	before := storeBytes(t, repo.StateDir, filepath.Join(root, intent.Dir))
	dry := atm(t, root, nil, "policy", "update", "--dry-run", "--expected-policy-version", string(wire.SizeOf(current)), "--file", file)
	if dry.res.Outcome != wire.OutcomeOK || len(dry.res.Items) != 1 || !field(dry.res.Items[0], "dryRun").Bool {
		t.Fatalf("dry run: %s", dry.stdout)
	}
	indexAfter, err := os.Stat(index)
	if (indexErr == nil) != (err == nil) || indexErr == nil && (!os.SameFile(indexBefore, indexAfter) || !indexBefore.ModTime().Equal(indexAfter.ModTime())) {
		t.Errorf("dry run wrote the closed-ticket index %s", index)
	}
	after := storeBytes(t, repo.StateDir, filepath.Join(root, intent.Dir))
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Errorf("dry run wrote %s", p)
		}
	}
	for p, b := range before {
		if after[p] != b {
			t.Errorf("dry run changed %s", p)
		}
	}
}
