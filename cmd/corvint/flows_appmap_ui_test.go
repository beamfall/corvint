package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
