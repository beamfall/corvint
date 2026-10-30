package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCLITraceZip(t *testing.T, dir string, lines []string) string {
	t.Helper()
	name := filepath.Join(dir, "trace.zip")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("trace.trace")
	if err == nil {
		_, err = w.Write([]byte(strings.Join(lines, "\n") + "\n"))
	}
	if err != nil || zw.Close() != nil || f.Close() != nil {
		t.Fatalf("zip: %v", err)
	}
	return name
}

// OUE-V0-001, 030, 042: the three observed-UI verbs are wired; without a map every screen has no
// linked path, so staleness reads UNKNOWN for every record and never current.
func TestFlowsAppmapObservedUIVerbs(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "one")
	rev := strings.TrimSpace(cemGit(t, root, "rev-parse", "HEAD"))
	zipName := writeCLITraceZip(t, root, []string{
		`{"type":"before","callId":"call@1","pageId":"page@1","method":"goto","params":{"url":"http://app.test/users/42?token=SECRET"},"stack":[{"file":"/ci/e2e/a.spec.ts","line":3,"column":1}]}`,
		`{"type":"frame-snapshot","snapshot":{"callId":"call@1","snapshotName":"after@call@1","frameUrl":"http://app.test/users/42?token=SECRET","pageId":"page@1","isMainFrame":true,"timestamp":2}}`,
	})
	code, out, errText := runCLI(t, "--root", root, "flows", "appmap", "observe", "--records", "--revision", rev, "--trace", "spec-a="+zipName)
	if code != 0 || strings.Contains(out, "SECRET") || !strings.Contains(out, `"observed-ui-records/0"`) || !strings.Contains(out, "/users/:param") {
		t.Fatalf("observe --records: %d %s %s", code, out, errText)
	}
	records := filepath.Join(root, "records.json")
	if err := os.WriteFile(records, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errText = runCLI(t, "--root", root, "flows", "appmap", "structure-diff", "--base", records, "--head", records)
	if code != 0 || !strings.Contains(out, `"signal":"candidate"`) {
		t.Fatalf("structure-diff: %d %s %s", code, out, errText)
	}
	code, out, errText = runCLI(t, "--root", root, "flows", "appmap", "staleness", "--records", records, "--at", "HEAD")
	var rep struct {
		Schema string
		Counts struct{ Total, Current, Unknown int }
	}
	if code != 0 || json.Unmarshal([]byte(out), &rep) != nil || rep.Schema != "observed-staleness/0" || rep.Counts.Total == 0 ||
		rep.Counts.Unknown != rep.Counts.Total || rep.Counts.Current != 0 {
		t.Fatalf("staleness: %d %s %s", code, out, errText)
	}
	for _, bad := range [][]string{
		{"flows", "appmap", "observe", "--records", "--trace", "k=" + zipName},
		{"flows", "appmap", "observe", "--records", "--revision", "abc", "--trace", "k=" + zipName},
		{"flows", "appmap", "staleness", "--records", records, "--retire-after", "-1"},
		{"flows", "appmap", "structure-diff", "--base", records},
	} {
		if code, _, _ := runCLI(t, append([]string{"--root", root}, bad...)...); code == 0 {
			t.Fatalf("%v must be refused", bad)
		}
	}
}

// OUE-V0-057: runs server-errors reads the reports and the server log and refuses a missing log.
func TestRunsServerErrorsCLI(t *testing.T) {
	root := t.TempDir()
	report := `{"config":{"version":"1.63.0","rootDir":"/w","workers":1,"metadata":{},"projects":[{"name":"chromium"}]},"suites":[{"title":"a.spec.ts","file":"a.spec.ts","specs":[{"title":"t","file":"a.spec.ts","line":1,"tests":[{"projectName":"chromium","expectedStatus":"passed","status":"unexpected","annotations":[],"results":[{"workerIndex":0,"status":"failed","duration":20,"retry":0,"startTime":"2026-10-25T10:00:00.000Z","errors":[{"message":"Error: HTTP 500 for POST /api/orders"}]}]}]}],"suites":[]}],"errors":[],"stats":{}}`
	log := `{"time":"2026-10-25T10:00:00.010Z","method":"POST","path":"/api/orders","status":500,"exception":{"class":"NoMethodError","message":"undefined method","frames":[{"file":"app/models/order.rb","line":4}]}}` + "\n"
	for name, body := range map[string]string{"report.json": report, "server.jsonl": log} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errText := runCLI(t, "--root", root, "runs", "server-errors", "--runs", "report.json", "--server-log", "server.jsonl")
	if code != 0 || !strings.Contains(out, `"state":"ATTACHED"`) || !strings.Contains(out, `"file":"app/models/order.rb"`) {
		t.Fatalf("server-errors: %d %s %s", code, out, errText)
	}
	code, _, errText = runCLI(t, "--root", root, "runs", "server-errors", "--runs", "report.json", "--server-log", "absent.jsonl")
	if code != 2 || !strings.Contains(errText, "runs-server-log-unavailable") {
		t.Fatalf("missing log: %d %s", code, errText)
	}
}
