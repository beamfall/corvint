// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/runevidence"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// rvdStore is the fixture repository's run evidence store.
func rvdStore(root string) runevidence.Store {
	return runevidence.Store{Dir: runevidence.Locate(filepath.Join(root, ".git"))}
}

// RVD-V1-004, RVD-V1-001: one `corvint test` run appends exactly one record, the receipt's own
// bytes under their SHA-256; a second run adds a second record and leaves the first unchanged; each
// shard of a sharded run appends its own receipt; a dry run appends nothing.
func TestRVDV1004CLITestAppendsOneRecordPerRun(t *testing.T) {
	root, base, _ := sessionFixture(t, "pass")
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run"); code != 0 {
		t.Fatalf("dry run %d %s", code, stderr)
	}
	if _, err := os.Lstat(rvdStore(root).Dir); !os.IsNotExist(err) {
		t.Fatal("a dry run wrote the run evidence store")
	}
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base)
	if code != 0 || strings.Contains(stderr, runevidence.ReasonWriteFailed) {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	receipt, err := os.ReadFile(filepath.Join(root, ".git", "corvint", "test-run.json"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := rvdStore(root).Read()
	if err != nil || len(snapshot.Records) != 1 || !bytes.Equal(snapshot.Records[0].Data, receipt) ||
		snapshot.Records[0].Entry.ID != runevidence.ID(receipt) || len(snapshot.Records[0].Rows) != 1 ||
		snapshot.Records[0].Rows[0].TestID != "go:example.test/m/a" || snapshot.Records[0].Rows[0].Outcome != "passed" {
		t.Fatalf("store after one run: %+v %v", snapshot, err)
	}
	first := snapshot.Records[0].Entry.ID
	if err := os.WriteFile(filepath.Join(root, "a", "a.go"), []byte("package a\n\nfunc A() int { return 4 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change a again")
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base); code != 0 {
		t.Fatalf("second run %d %s", code, stderr)
	}
	snapshot, err = rvdStore(root).Read()
	if err != nil || len(snapshot.Records) != 2 || snapshot.Records[0].Entry.ID != first || snapshot.Records[0].Entry.Tree == snapshot.Records[1].Entry.Tree {
		t.Fatalf("store after two runs: %+v %v", snapshot, err)
	}
	if data, err := os.ReadFile(filepath.Join(rvdStore(root).Dir, "records", first+".json")); err != nil || runevidence.ID(data) != first {
		t.Fatalf("the first record changed: %v", err)
	}
	for _, shard := range []string{"0/2", "1/2"} {
		if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--shard", shard); code != 0 {
			t.Fatalf("shard %s: %d %s", shard, code, stderr)
		}
	}
	if snapshot, err = rvdStore(root).Read(); err != nil || len(snapshot.Records) != 4 {
		t.Fatalf("two shards appended %d records, want 2 more: %v", len(snapshot.Records)-2, err)
	}
	for index, shard := range []string{"0", "1"} {
		receipt, err := os.ReadFile(filepath.Join(root, ".git", "corvint", "test-run."+shard+".json"))
		if err != nil || !bytes.Equal(snapshot.Records[2+index].Data, receipt) {
			t.Fatalf("shard %s record is not its receipt: %v", shard, err)
		}
	}
	if status := affectedGit(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("a run changed the worktree: %q", status)
	}
}

// RVD-V1-010: when the store cannot be written the run prints run-evidence-write-failed with the
// cause, still writes its receipt and keeps its exit code.
func TestRVDV1010CLIWriteFailureKeepsExitCode(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root, base, _ := sessionFixture(t, "pass")
	store := rvdStore(root).Dir
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(store, 0o700) })
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base)
	if code != 0 || !strings.Contains(stderr, "corvint-test: "+runevidence.ReasonWriteFailed+" ") ||
		!strings.Contains(stderr, "corvint-test: PASS selected units passed: 1 of 1") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "corvint", "test-run.json")); err != nil {
		t.Fatalf("the receipt was not written: %v", err)
	}
	// A linked store directory is refused, never followed.
	if err := os.Chmod(store, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(store); err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	if err := os.Symlink(elsewhere, store); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base)
	if code != 0 || !strings.Contains(stderr, runevidence.ReasonWriteFailed) {
		t.Fatalf("linked store: code %d stderr %q", code, stderr)
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("the linked store received %v", entries)
	}
}

// rvdFlowFixture is the session fixture with one committed flow whose variation links the Go
// package test "go:example.test/m/a", the key a corvint-test-run/0 unit row carries, and a later
// change to package a measured from the flow commit.
func rvdFlowFixture(t *testing.T) (root, base string) {
	t.Helper()
	root, _, _ = sessionFixture(t, "pass")
	intent := appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: "compute", Revision: 1, Kind: "ui", Actor: "developer",
		Preconditions: []string{}, Steps: []appflows.FlowStep{{StepID: "run", Action: "run A"}}, Outcomes: []appflows.FlowOutcome{},
		Variations: []appflows.FlowVariation{{VariationID: "compute.basic", Preconditions: []string{}, Steps: []string{"run"}, ObservableFacts: []string{}, Outcomes: []string{}, Projects: []string{}}},
		Links:      []appflows.FlowLink{{From: "compute.basic", Basis: "declared", Target: appflows.LinkTarget{Type: "test", Path: "a/a_test.go", TestKey: "go:example.test/m/a"}}}}
	raw, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "flows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "flows", "compute.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "add the compute flow")
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "a", "a.go"), []byte("package a\n\nfunc A() int { return 5 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change a after the flow")
	return root, base
}

// rvdStoreDigest names every file under the fixture store with its mode, size, time and bytes.
func rvdStoreDigest(t *testing.T, root string) string {
	t.Helper()
	var out strings.Builder
	err := filepath.WalkDir(rvdStore(root).Dir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "%s %v %d %d\n", name, info.Mode(), info.Size(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			fmt.Fprintf(&out, "%s\n", runevidence.ID(data))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// PROD-TEST-020, RVD-V1-007, RVD-V1-012: after `corvint test`, `flows status` (served by
// corvint-docs, run here in process) with no --evidence shows that run's outcome with its tree at HEAD, and `corvint test-evidence` names it;
// --no-store leaves it out; no reader (flows status, map, gaps, impact, test-evidence, flows
// test-evidence) changes a byte of the store.
func TestRVDV1007ProdTest020TestThenFlowsStatus(t *testing.T) {
	root, base := rvdFlowFixture(t)
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base); code != 0 {
		t.Fatalf("corvint test %d %s", code, stderr)
	}
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD^{tree}"))
	before := rvdStoreDigest(t, root)
	code, out, stderr := runFlowsCLI(root, "status", "--flows", "flows")
	if code != 0 {
		t.Fatalf("flows status %d %s", code, stderr)
	}
	var report appflows.StatusReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Flows) != 1 || len(report.Flows[0].Variations) != 1 || len(report.Flows[0].Variations[0].Tests) != 1 {
		t.Fatalf("status %s", out)
	}
	got := report.Flows[0].Variations[0].Tests[0]
	if got.TestKey != "go:example.test/m/a" || got.Result != "passed" || got.Stale || got.SourceCommit != head || got.SourceTree != tree ||
		got.Authority != appflows.AuthorityLocallyObserved || report.RunEvidence == nil || report.RunEvidence.Records != 1 {
		t.Fatalf("flows status after corvint test: %+v %+v", got, report.RunEvidence)
	}
	code, out, _ = runFlowsCLI(root, "status", "--flows", "flows", "--no-store")
	if code != 0 || !strings.Contains(out, `"result":"NOT_OBSERVED"`) || strings.Contains(out, "run_evidence") {
		t.Fatalf("flows status --no-store %d %s", code, out)
	}
	code, out, stderr = runCLI(t, "--root", root, "test-evidence", "--test", "go:example.test/m/a")
	var evidence struct {
		Runs *struct {
			Results []struct {
				Test, Outcome, Tree string
				AtRevision          bool `json:"at_revision"`
			} `json:"results"`
		} `json:"runs"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &evidence) != nil || evidence.Runs == nil || len(evidence.Runs.Results) != 1 ||
		evidence.Runs.Results[0].Outcome != "passed" || evidence.Runs.Results[0].Tree != tree || !evidence.Runs.Results[0].AtRevision {
		t.Fatalf("test-evidence %d %s %s", code, stderr, out)
	}
	if code, out, _ := runCLI(t, "--root", root, "test-evidence", "--test", "go:example.test/m/a", "--no-store"); code != 0 || strings.Contains(out, `"runs"`) {
		t.Fatalf("test-evidence --no-store %d %s", code, out)
	}
	// corvint flows forwards to corvint-docs; runFlowsCLI runs the same handlers in process.
	for _, args := range [][]string{{"map", "--flows", "flows"}, {"gaps", "--flows", "flows"}, {"impact", "--flows", "flows", "--base", base}} {
		if code, _, stderr := runFlowsCLI(root, args...); code != 0 {
			t.Fatalf("flows %v %d %s", args, code, stderr)
		}
	}
	for _, args := range [][]string{{"test-evidence", "a/a.go"}, {"flows", "test-evidence", "--flows", "flows", "--test", "go:example.test/m/a"}} {
		runCLI(t, append([]string{"--root", root}, args...)...)
	}
	if after := rvdStoreDigest(t, root); after != before {
		t.Fatalf("a reader changed the store:\n%s\n%s", before, after)
	}
}

// RVD-V1-004, RVD-V1-012: one corvint watch save that runs tests appends exactly one record, a
// corvint-test-run/0 receipt the strict receipt reader accepts (the shape corvint test writes),
// naming tool "corvint watch", the planned package's outcome and its measured time, and the dirty
// working tree. The save is injected on --events-stdin, so no file-system watch or sleep runs. A
// --dry-run save appends nothing.
func TestRVDV1004WatchAppendsOneRecordPerSave(t *testing.T) {
	root, index := watchModule(t)
	if err := os.WriteFile(filepath.Join(root, "p", "p.go"), []byte("package p\nfunc F() int {\n return 2\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := rvdStore(root)
	if code, _, stderr := watchCLI(t, "save\n", "--root", root, "watch", "--index", index, "--events-stdin", "--dry-run", "--packages", "p"); code != 0 {
		t.Fatalf("dry-run watch %d %s", code, stderr)
	}
	if _, err := os.Lstat(store.Dir); !os.IsNotExist(err) {
		t.Fatalf("a dry-run watch wrote the store: %v", err)
	}
	code, stdout, stderr := watchCLI(t, "save\n", "--root", root, "watch", "--index", index, "--events-stdin", "--packages", "p")
	if events := watchEvents(t, stdout); code != 0 || len(events) != 2 || events[1].Kind != "result" || events[1].ExitCode != 0 {
		t.Fatalf("watch %d %s %s", code, stdout, stderr)
	}
	if strings.Contains(stderr, "run-evidence-write-failed") {
		t.Fatalf("watch append failed: %s", stderr)
	}
	snapshot, err := store.Read()
	if err != nil || len(snapshot.Records) != 1 || len(snapshot.Reasons) != 0 {
		t.Fatalf("store after one save: %d records %v %v", len(snapshot.Records), snapshot.Reasons, err)
	}
	record := snapshot.Records[0]
	receipt, err := sessiontest.ReadReceipt(filepath.Join(store.Dir, "records", record.Entry.ID+".json"))
	if err != nil {
		t.Fatalf("the watch record is not a corvint-test-run/0 receipt: %v", err)
	}
	head := strings.TrimSpace(watchGit(t, root, "rev-parse", "HEAD"))
	if receipt.Tool != sessiontest.ToolWatch || receipt.Source.Head != head || len(receipt.Source.Paths) != 1 || receipt.Source.Paths[0].Path != "p/p.go" ||
		receipt.Result != sessiontest.ResultPass || len(receipt.Invocations) != 1 {
		t.Fatalf("watch receipt %+v", receipt)
	}
	if len(record.Rows) != 1 {
		t.Fatalf("rows %+v", record.Rows)
	}
	row := record.Rows[0]
	if row.TestID != "go:example.com/m/p" || row.Outcome != "passed" || row.Slice != sessiontest.SliceSelected || row.Clean ||
		row.DurationSource != "go-test2json" || row.DurationMs == nil || record.Entry.Profile != runevidence.ProfileReceipt {
		t.Fatalf("watch row %+v entry %+v", row, record.Entry)
	}
}

// RVD-V1-008: the store adds no verb. Only the writers and readers RVD-V1-004, RVD-V1-005 and
// RVD-V1-007 name mention it in the command catalog; `corvint runs`, which change source-removal
// deletes, does not read it.
func TestRVDV1008NoListingVerb(t *testing.T) {
	code, out, stderr := runCLI(t, "help", "--json")
	if code != 0 {
		t.Fatalf("help --json %d %s", code, stderr)
	}
	var catalog struct {
		Commands []json.RawMessage `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &catalog); err != nil || len(catalog.Commands) == 0 {
		t.Fatalf("catalog %v", err)
	}
	allowed := map[string]bool{"test": true, "watch": true, "flows": true, "test-evidence": true}
	for _, raw := range catalog.Commands {
		var command struct{ Name string }
		if err := json.Unmarshal(raw, &command); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(raw)), "run evidence store") && !allowed[command.Name] {
			t.Errorf("verb %q mentions the run evidence store", command.Name)
		}
	}
	if code, out, _ := runCLI(t, "runs", "--help"); strings.Contains(strings.ToLower(out), "evidence store") {
		t.Fatalf("corvint runs reads the store: %d %s", code, out)
	}
}

// rvdV1008Gate is why the RVD-V1-008 "removed verb" scenario cannot hold on this tree.
const rvdV1008Gate = "gated on change source-removal: `corvint runs` still exists until that change deletes it"

// RVD-V1-008 scenario "removed verb": after delivery `corvint runs` is unknown, as after change
// source-removal. It is gated on that change, which run-evidence precedes (R38), so it is skipped
// until source-removal lands; the check below is what it then proves.
func TestRVDV1008RunsVerbIsUnknown(t *testing.T) {
	t.Skip(rvdV1008Gate)
	code, _, stderr := runCLI(t, "runs")
	if code != 2 || !strings.Contains(stderr, "unknown") {
		t.Fatalf("corvint runs exited %d: %s", code, stderr)
	}
}
