// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/runevidence"
)

// rvdJUnit is a JUnit report from another system whose one case is the shop fixture's
// profile.view test, keyed "profile.spec.ts > views" by the classname > name rule.
const rvdJUnit = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites><testsuite name="e2e" tests="1" failures="0"><testcase classname="profile.spec.ts" name="views" time="0.25"></testcase></testsuite></testsuites>
`

func rvdIngestArgs(t *testing.T, fx shopFixture, report string, extra ...string) []string {
	t.Helper()
	digest := strings.Repeat("a", 64)
	args := []string{"ingest", "--format", "junit-xml", "--from", report, "--run-id", "ci-1", "--runner-version", "junit",
		"--source-commit", fx.head, "--source-tree", shopGit(t, fx.root, "rev-parse", "HEAD^{tree}"), "--source-clean",
		"--build-artifact-digest", digest, "--environment-id", "ci", "--environment-digest", digest,
		"--fixture-id", "seed", "--fixture-digest", digest}
	return append(args, extra...)
}

func rvdStore(root string) runevidence.Store {
	return runevidence.Store{Dir: runevidence.Locate(filepath.Join(root, ".git"))}
}

// rvdStoreDigest names every file under the store with its mode, size, time and bytes.
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

// RVD-V1-005: without --record, flows ingest prints as before and writes no store; with --record it
// appends exactly the document it prints as one record, names the record on stderr, and the
// ingested test then appears in flows status with no --evidence.
func TestRVDV1005IngestRecord(t *testing.T) {
	fx := newShopFixture(t)
	report := filepath.Join(t.TempDir(), "report.xml")
	if err := os.WriteFile(report, []byte(rvdJUnit), 0o600); err != nil {
		t.Fatal(err)
	}
	code, plain, diagnostic := runFlowsCLI(fx.root, rvdIngestArgs(t, fx, report)...)
	if code != 0 || diagnostic != "" || !strings.Contains(plain, `"test_key":"profile.spec.ts > views"`) {
		t.Fatalf("ingest %d %q %s", code, diagnostic, plain)
	}
	if _, err := os.Lstat(rvdStore(fx.root).Dir); !os.IsNotExist(err) {
		t.Fatal("flows ingest without --record wrote the store")
	}
	code, recorded, diagnostic := runFlowsCLI(fx.root, rvdIngestArgs(t, fx, report, "--record")...)
	if code != 0 || recorded != plain {
		t.Fatalf("ingest --record %d %s; stdout changed:\n%s", code, diagnostic, recorded)
	}
	id := runevidence.ID([]byte(plain))
	if diagnostic != "flows ingest: recorded run evidence "+id+"\n" {
		t.Fatalf("diagnostic %q", diagnostic)
	}
	snapshot, err := rvdStore(fx.root).Read()
	if err != nil || len(snapshot.Records) != 1 || snapshot.Records[0].Entry.ID != id || string(snapshot.Records[0].Data) != plain ||
		snapshot.Records[0].Entry.Profile != runevidence.ProfileIngested {
		t.Fatalf("store %+v %v", snapshot, err)
	}
	report2, _ := decodeStatus(t, fx.root)
	view := statusVariation(t, report2, "profile", "profile.view")
	if len(view.Tests) != 1 || view.Tests[0].Result != "passed" || view.Tests[0].Stale || view.Tests[0].RunID != "ci-1" ||
		view.Tests[0].SourceTree != shopGit(t, fx.root, "rev-parse", "HEAD^{tree}") || report2.RunEvidence == nil || report2.RunEvidence.Records != 1 {
		t.Fatalf("profile.view from the store: %+v %+v", view.Tests, report2.RunEvidence)
	}
	// A failed append exits 2 with run-evidence-write-failed and prints nothing.
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		records := filepath.Join(rvdStore(fx.root).Dir, "records")
		if err := os.Chmod(records, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(records, 0o700) })
		other := filepath.Join(t.TempDir(), "other.xml")
		if err := os.WriteFile(other, []byte(strings.Replace(rvdJUnit, `time="0.25"`, `time="0.5"`, 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		code, out, diagnostic := runFlowsCLI(fx.root, rvdIngestArgs(t, fx, other, "--record")...)
		if code != 2 || out != "" || !strings.Contains(diagnostic, runevidence.ReasonWriteFailed) {
			t.Fatalf("failed append %d %q %s", code, out, diagnostic)
		}
	}
}

// RVD-V1-007, RVD-V1-012: status, map, gaps and impact read the store by default; --evidence adds
// files to it and --no-store reads only the files; the store's newest record gives the result; no
// reader changes a byte of the store.
func TestRVDV1007FlowsReadersUseStore(t *testing.T) {
	fx := newShopFixture(t)
	report := filepath.Join(t.TempDir(), "report.xml")
	if err := os.WriteFile(report, []byte(rvdJUnit), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, diagnostic := runFlowsCLI(fx.root, rvdIngestArgs(t, fx, report, "--record")...); code != 0 {
		t.Fatalf("ingest --record %d %s", code, diagnostic)
	}
	failing := filepath.Join(t.TempDir(), "failing.xml")
	if err := os.WriteFile(failing, []byte(strings.Replace(rvdJUnit, `time="0.25"></testcase>`, `time="0.25"><failure message="boom"/></testcase>`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, diagnostic := runFlowsCLI(fx.root, append(rvdIngestArgs(t, fx, failing, "--record"), "--run-id", "ci-2")...); code != 0 {
		t.Fatalf("second ingest --record %d %s", code, diagnostic)
	}
	before := rvdStoreDigest(t, fx.root)
	stored, _ := decodeStatus(t, fx.root)
	if view := statusVariation(t, stored, "profile", "profile.view"); len(view.Tests) != 1 || view.Tests[0].Result != "failed" || view.Tests[0].RunID != "ci-2" {
		t.Fatalf("the newest record does not give the result: %+v", view.Tests)
	}
	// --evidence adds: the file's checkout result joins the store's profile result.
	both, _ := decodeStatus(t, fx.root, "--evidence", fx.evidence)
	if v := statusVariation(t, both, "checkout", "checkout.happy"); v.Tests[0].Result != "passed" {
		t.Fatalf("--evidence did not add: %+v", v.Tests)
	}
	if v := statusVariation(t, both, "profile", "profile.view"); v.Tests[0].Result != "failed" {
		t.Fatalf("--evidence replaced the store: %+v", v.Tests)
	}
	// --no-store reads only the named files.
	only, _ := decodeStatus(t, fx.root, "--no-store", "--evidence", fx.evidence)
	if v := statusVariation(t, only, "profile", "profile.view"); v.Tests[0].Result != appflows.ResultNotObserved || only.RunEvidence != nil {
		t.Fatalf("--no-store read the store: %+v %+v", v.Tests, only.RunEvidence)
	}
	// map sees the stored runs of profile.view; with --no-store its evidence is missing again. gaps
	// reads the same records: with the store it differs from the report of no evidence.
	missing := `"test_key":"profile.spec.ts > views","state":"missing"`
	code, withStore, diagnostic := runFlowsCLI(fx.root, "map", "--flows", "flows")
	if code != 0 || strings.Contains(withStore, missing) || !strings.Contains(withStore, `"test_key":"profile.spec.ts > views","state":"flaky","authority":"INGESTED"`) {
		t.Fatalf("map did not read the store: %d %s %s", code, diagnostic, withStore)
	}
	if _, noStore, _ := runFlowsCLI(fx.root, "map", "--flows", "flows", "--no-store"); !strings.Contains(noStore, missing) {
		t.Fatalf("map --no-store read the store: %s", noStore)
	}
	_, gapsStore, _ := runFlowsCLI(fx.root, "gaps", "--flows", "flows")
	_, gapsNone, _ := runFlowsCLI(fx.root, "gaps", "--flows", "flows", "--no-store")
	if gapsStore == "" || gapsStore == gapsNone {
		t.Fatalf("gaps did not read the store:\n%s", gapsStore)
	}
	for _, verb := range []string{"map", "gaps"} {
		if _, none, _ := runFlowsCLI(fx.root, verb, "--flows", "flows", "--no-store", "--evidence", filepath.Join(t.TempDir(), "absent.jsonl")); none != "" {
			t.Fatalf("%s with an absent --evidence printed %s", verb, none)
		}
	}
	// impact adds the last stored result per reached test key; --no-store keeps the golden bytes.
	code, impact, diagnostic := runFlowsCLI(fx.root, "impact", "--flows", "flows", "--base", fx.b)
	if code != 0 || !strings.Contains(impact, `"last_results"`) {
		t.Fatalf("impact did not read the store: %d %s %s", code, diagnostic, impact)
	}
	golden, err := os.ReadFile(filepath.Join("testdata", "flows", "impact.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, plain, _ := runFlowsCLI(fx.root, "impact", "--flows", "flows", "--base", fx.b, "--no-store"); plain != string(golden) {
		t.Fatalf("impact --no-store differs from its golden:\n%s", plain)
	}
	if after := rvdStoreDigest(t, fx.root); after != before {
		t.Fatalf("a flows reader changed the store:\n%s\n%s", before, after)
	}
}

// RVD-V1-010: a hand-edited record is skipped; status lists it and marks the test only it held
// run-evidence-corrupt.
func TestRVDV1010StatusNamesCorruptRecord(t *testing.T) {
	fx := newShopFixture(t)
	report := filepath.Join(t.TempDir(), "report.xml")
	if err := os.WriteFile(report, []byte(rvdJUnit), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, diagnostic := runFlowsCLI(fx.root, rvdIngestArgs(t, fx, report, "--record")...)
	if code != 0 {
		t.Fatalf("ingest --record %d %s", code, diagnostic)
	}
	id := runevidence.ID([]byte(out))
	name := filepath.Join(rvdStore(fx.root).Dir, "records", id+".json")
	if err := os.WriteFile(name, bytes.Replace([]byte(out), []byte(`"passed"`), []byte(`"failed"`), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	status, _ := decodeStatus(t, fx.root)
	view := statusVariation(t, status, "profile", "profile.view")
	if len(view.Tests) != 1 || view.Tests[0].Result != appflows.ResultNotObserved || view.Tests[0].Reason != runevidence.ReasonCorrupt ||
		status.RunEvidence == nil || !slices.Equal(status.RunEvidence.CorruptRecords, []string{id}) || !slices.Equal(status.RunEvidence.Reasons, []string{runevidence.ReasonCorrupt}) {
		t.Fatalf("corrupt record: %+v %+v", view.Tests, status.RunEvidence)
	}
	if edit := statusVariation(t, status, "profile", "profile.edit"); edit.Tests[0].Reason != "" {
		t.Fatalf("a test the corrupt record never held is marked: %+v", edit.Tests)
	}
}

// RVD-V1-007, RVD-V1-012: the MCP flows tools read the store as their CLI verbs do and leave it
// unchanged.
func TestRVDV1007FlowsToolsReadStore(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the repository probe is qualified only on Darwin and Linux")
	}
	fx := newShopFixture(t)
	fx.root = resolvedRoot(t, fx.root)
	report := filepath.Join(t.TempDir(), "report.xml")
	if err := os.WriteFile(report, []byte(rvdJUnit), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, diagnostic := runFlowsCLI(fx.root, rvdIngestArgs(t, fx, report, "--record")...); code != 0 {
		t.Fatalf("ingest --record %d %s", code, diagnostic)
	}
	before := rvdStoreDigest(t, fx.root)
	registry := flowsRegistry(t, fx.root)
	assertFlowsParity(t, fx.root, registry, bridge.ToolFlowsStatus, map[string]any{"flows": "flows"}, "status", "--flows", "flows")
	assertFlowsParity(t, fx.root, registry, bridge.ToolFlowsMap, map[string]any{"flows": "flows"}, "map", "--flows", "flows")
	assertFlowsParity(t, fx.root, registry, bridge.ToolFlowsGaps, map[string]any{"flows": "flows"}, "gaps", "--flows", "flows")
	assertFlowsParity(t, fx.root, registry, bridge.ToolFlowsImpact, map[string]any{"flows": "flows", "base": fx.b}, "impact", "--flows", "flows", "--base", fx.b)
	raw, _ := json.Marshal(map[string]any{"flows": "flows"})
	result, callErr := registry.Call(t.Context(), bridge.ToolFlowsStatus, raw)
	if callErr != nil {
		t.Fatal(callErr)
	}
	receipt, _ := json.Marshal(result.Receipt)
	if !strings.Contains(string(receipt), `"run_id":"ci-1"`) {
		t.Fatalf("the status tool did not read the store: %s", receipt)
	}
	if after := rvdStoreDigest(t, fx.root); after != before {
		t.Fatal("an MCP flows tool changed the store")
	}
}
