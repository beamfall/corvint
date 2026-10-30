//go:build darwin || linux

package mobile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

const (
	fakeSession     = "0f1e2d3c-4b5a-4968-8776-655443322110"
	reloadedSession = "1a2b3c4d-5e6f-4a0b-9c1d-2e3f4a5b6c7d"
)

// TestTimeoutDeletesAppiumSession binds TRE-V0-087 for the Appium profile
// (V1-1553): a timed-out run deletes the WebDriver session its fixed hook
// recorded, and each session a reload replaced it with, and records an
// incomplete cleanup when the server refuses the delete or the session
// identity was never recorded.
func TestTimeoutDeletesAppiumSession(t *testing.T) {
	for _, c := range []struct {
		name          string
		status        int
		body          string
		record        []string
		outcome, step string
	}{
		{"deleted", 200, `{"value":null}`, []string{fakeSession}, tr.CleanupClean, "ok"},
		// browser.reloadSession() replaced the first session; both are deleted.
		{"reloaded", 200, `{"value":null}`, []string{fakeSession, reloadedSession}, tr.CleanupClean, "ok"},
		{"already-absent", 404, `{"value":{"error":"invalid session id"}}`, []string{fakeSession}, tr.CleanupClean, "ok"},
		{"refused", 500, `{"value":{"error":"unknown error"}}`, []string{fakeSession}, tr.CleanupIncomplete, "failed"},
		{"unknown-route", 404, `{"value":{"error":"unknown command"}}`, []string{fakeSession}, tr.CleanupIncomplete, "failed"},
		{"unrecorded", 200, `{"value":null}`, nil, tr.CleanupIncomplete, "unobserved"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var mu sync.Mutex
			var calls []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
				mu.Lock()
				calls = append(calls, q.Method+" "+q.URL.Path)
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			root, _ := filepath.EvalSymlinks(t.TempDir())
			record := ""
			for _, id := range c.record {
				record += `printf '%s\n' ` + id + ` >> "${2%/*}/.appium-session"; `
			}
			wdio := "#!/bin/sh\n" + record + "exec /bin/sleep 30\n"
			files := map[string]string{"bin/wdio": wdio, "config.mjs": "export const config={};\n", "reporter.js": "export default class {}\n", "test.cjs": "it('x',()=>{});\n"}
			for name, body := range files {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			digest := func(name string) string { return tr.Digest([]byte(files[name])) }
			r := tr.Request{Runner: Runner, Root: root, Executable: filepath.Join(root, "bin/wdio"), ExecutableSha256: digest("bin/wdio"), Config: "config.mjs", ConfigSha256: digest("config.mjs"), Reporter: filepath.Join(root, "reporter.js"), ReporterSha256: digest("reporter.js"), ReportDir: filepath.Join(root, "report"), Project: srv.URL + "/", Target: "emulator-5580", Selectors: []string{"test.cjs"}, InputFiles: map[string]string{"test.cjs": digest("test.cjs")}, TimeoutSeconds: 1}
			inv, err := Build(r)
			if err != nil {
				t.Fatal(err)
			}
			// A killed WDIO writes no report, so Execute also returns the
			// missing-report error; the receipt still carries the execution.
			out, err := tr.Execute(context.Background(), r, inv)
			if err == nil || !strings.Contains(err.Error(), reportName) {
				t.Fatalf("timed-out run without a report: %v", err)
			}
			d := out.DeviceCleanup
			if !out.Input.TimedOut || d == nil || d.Outcome != c.outcome || len(d.Steps) != 1 || d.Steps[0].Name != "delete-session" || d.Steps[0].Status != c.step {
				t.Fatalf("cleanup %+v timedOut=%v", d, out.Input.TimedOut)
			}
			mu.Lock()
			got := strings.Join(calls, ",")
			mu.Unlock()
			deletes := []string{}
			for _, id := range c.record {
				deletes = append(deletes, "DELETE /session/"+id)
			}
			want := strings.Join(deletes, ",")
			if got != want {
				t.Fatalf("WebDriver calls %q, want %q", got, want)
			}
			problem := false
			for _, p := range out.Input.ExecutionProblems {
				problem = problem || p.Code == "device-cleanup"
			}
			if problem != (c.outcome != tr.CleanupClean) {
				t.Fatalf("cleanup problem %v for %s: %+v", problem, c.outcome, out.Input.ExecutionProblems)
			}
		})
	}
}
