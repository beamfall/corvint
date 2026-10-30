// SPDX-License-Identifier: Apache-2.0
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 26, 7, 23, 0, 0, time.UTC)

func page(artifacts ...map[string]any) string {
	b, err := json.Marshal(map[string]any{"total_count": len(artifacts), "artifacts": artifacts})
	if err != nil {
		panic(err)
	}
	return string(b) + "\n"
}

func art(id int64, name string, created time.Time, run int64) map[string]any {
	return map[string]any{"id": id, "name": name, "size_in_bytes": 512, "created_at": created.Format(time.RFC3339),
		"expired": false, "workflow_run": map[string]any{"id": run, "head_branch": "main"}}
}

func flakyRecord(event string, run, shard int, tests ...string) []byte {
	ft := []map[string]string{}
	pkgs := []string{}
	for _, t := range tests {
		pkg, name, _ := strings.Cut(t, ":")
		ft = append(ft, map[string]string{"package": pkg, "test": name})
		pkgs = append(pkgs, pkg)
	}
	b, err := json.Marshal(map[string]any{"profile": profile, "verdict": "FLAKY", "event": event, "runId": run,
		"runAttempt": 1, "target": "abc", "shard": shard, "shards": 8, "firstStatus": 1, "rerunInvocations": 1,
		"failedPackages": pkgs, "failedTests": ft})
	if err != nil {
		panic(err)
	}
	return b
}

// writeZip stores content as flaky.json in DIR/<id>.zip, the shape `gh api .../artifacts/<id>/zip` serves.
func writeZip(t *testing.T, dir string, id int64, content []byte) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("flaky.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(content)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.zip", id)), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAFPV0530SelectKeepsOnlyTheWindowsFlakyRecords(t *testing.T) {
	dir := t.TempDir()
	listing := filepath.Join(dir, "listing.json")
	day := 24 * time.Hour
	expired := art(9, "ci-flaky-pull_request-3", now.Add(-day), 900)
	expired["expired"] = true
	big := art(10, "ci-flaky-pull_request-3", now.Add(-day), 901)
	big["size_in_bytes"] = 2 << 20
	pages := page(art(1, "ci-flaky-pull_request-3", now.Add(-2*day), 100)) +
		page(art(2, "ci-flaky-merge_group-7", now.Add(-6*day), 200),
			art(1, "ci-flaky-pull_request-3", now.Add(-2*day), 100), // the same id on a second page
			art(3, "ci-flaky-pull_request-3", now.Add(-8*day), 300), // before the window
			art(4, "ci-flaky-pull_request-3", now, 400),             // at the window's end: excluded
			art(5, "ci-shard-costs-3", now.Add(-day), 500),          // a foreign name
			art(6, "ci-flaky-push-3", now.Add(-day), 600),           // not a retained event
			expired, big)
	if err := os.WriteFile(listing, []byte(pages), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := run([]string{"-now", now.Format(time.RFC3339), "select", listing}, &out, &errb); rc != 0 {
		t.Fatalf("select exited %d: %s", rc, errb.String())
	}
	want := "2\tci-flaky-merge_group-7\t2026-10-20T07:23:00Z\t200\n1\tci-flaky-pull_request-3\t2026-10-24T07:23:00Z\t100\n"
	if out.String() != want {
		t.Fatalf("select printed\n%s\nwant\n%s", out.String(), want)
	}
	if rc := run([]string{"select", filepath.Join(dir, "absent.json")}, &out, &errb); rc != 1 {
		t.Fatalf("a missing listing exited %d, want 1", rc)
	}
	if rc := run([]string{"-days", "0", "select", listing}, &out, &errb); rc != 2 {
		t.Fatalf("-days 0 exited %d, want 2", rc)
	}
}

func TestAFPV0530ReportAggregatesPerTestAndNamesUnreadRecords(t *testing.T) {
	dir := t.TempDir()
	listing := filepath.Join(dir, "listing.json")
	day := 24 * time.Hour
	pages := page(
		art(1, "ci-flaky-pull_request-3", now.Add(-6*day), 100),
		art(2, "ci-flaky-merge_group-3", now.Add(-4*day), 200),
		art(3, "ci-flaky-pull_request-5", now.Add(-2*day), 300),
		art(4, "ci-flaky-pull_request-5", now.Add(-day), 400),
		art(5, "ci-flaky-merge_group-1", now.Add(-day), 500),
	)
	if err := os.WriteFile(listing, []byte(pages), 0o644); err != nil {
		t.Fatal(err)
	}
	arts := filepath.Join(dir, "artifacts")
	if err := os.Mkdir(arts, 0o755); err != nil {
		t.Fatal(err)
	}
	writeZip(t, arts, 1, flakyRecord("pull_request", 100, 3, "example.com/m/a:TestOne"))
	writeZip(t, arts, 2, flakyRecord("merge_group", 200, 3, "example.com/m/a:TestOne", "example.com/m/b:TestTwo"))
	writeZip(t, arts, 3, flakyRecord("pull_request", 300, 5, "example.com/m/a:TestOne"))
	// 4 was not downloaded; 5 is not a FLAKY record.
	writeZip(t, arts, 5, []byte(`{"profile":"other/0","verdict":"PASS"}`))
	var out, errb bytes.Buffer
	args := []string{"-now", now.Format(time.RFC3339), "report", "-artifacts", arts,
		"-run-url", "https://example.invalid/run/1", listing}
	if rc := run(args, &out, &errb); rc != 0 {
		t.Fatalf("report exited %d: %s", rc, errb.String())
	}
	got := out.String()
	for _, want := range []string{
		"## CI flake report: 2026-10-19 to 2026-10-26\n",
		"3 FLAKY shard pass(es) in 3 CI run(s): merge_group 1, pull_request 2.",
		"3 of 5 selected record(s) read.",
		"| `example.com/m/a.TestOne` | 3 | 3 | 1 | 2 | 2026-10-20 | 2026-10-24 |\n",
		"| `example.com/m/b.TestTwo` | 1 | 1 | 1 | 0 | 2026-10-22 | 2026-10-22 |\n",
		"Not read (2): the rate above is a lower bound.",
		"- artifact 4 `ci-flaky-pull_request-5` (2026-10-25): archive missing: not downloaded",
		"- artifact 5 `ci-flaky-merge_group-1` (2026-10-25): flaky.json profile \"other/0\" verdict \"PASS\"",
		"(https://example.invalid/run/1) at 2026-10-26T07:23:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if i, j := strings.Index(got, "TestOne"), strings.Index(got, "TestTwo"); i > j {
		t.Errorf("the most frequent test is not first:\n%s", got)
	}
}

func TestAFPV0530EmptyWindowSaysSo(t *testing.T) {
	dir := t.TempDir()
	listing := filepath.Join(dir, "listing.json")
	if err := os.WriteFile(listing, []byte(page()+page()), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := run([]string{"-now", now.Format(time.RFC3339), "report", "-artifacts", dir, listing}, &out, &errb); rc != 0 {
		t.Fatalf("report exited %d: %s", rc, errb.String())
	}
	if !strings.Contains(out.String(), "0 FLAKY shard pass(es) in 0 CI run(s)") || !strings.Contains(out.String(), "No FLAKY record in the window.") {
		t.Fatalf("empty report:\n%s", out.String())
	}
	if rc := run([]string{"report", listing}, &out, &errb); rc != 2 {
		t.Fatalf("report without -artifacts exited %d, want 2", rc)
	}
	if rc := run([]string{"frobnicate"}, &out, &errb); rc != 2 {
		t.Fatalf("an unknown command exited %d, want 2", rc)
	}
}
