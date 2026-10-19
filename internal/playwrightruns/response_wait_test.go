package playwrightruns

import (
	"strings"
	"testing"
	"time"
)

const waitHAR = `{"log":{"pages":[
 {"id":"page@1","startedDateTime":"2026-10-19T10:00:00.000Z"},
 {"id":"page@2","startedDateTime":"2026-10-19T10:00:05.000Z"}],
"entries":[
 {"pageref":"page@1","startedDateTime":"2026-10-19T10:00:00.100Z","time":12,"request":{"method":"GET","url":"http://app.test/legacy"},"response":{"status":500}},
 {"pageref":"page@2","startedDateTime":"2026-10-19T10:00:05.100Z","time":20,"request":{"method":"GET","url":"http://app.test/orders/new"},"response":{"status":200}},
 {"pageref":"page@2","startedDateTime":"2026-10-19T10:00:05.200Z","time":9,"request":{"method":"GET","url":"http://app.test/api/orders/placeholder?token=abc"},"response":{"status":404}},
 {"pageref":"page@2","startedDateTime":"2026-10-19T10:00:05.300Z","time":30,"request":{"method":"GET","url":"http://app.test/api/session"},"response":{"status":200}}
]}}`

func mustLog(t *testing.T, text string) *RequestLog {
	t.Helper()
	log, err := ReadRequestLog([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func waitAt(t *testing.T, start string, timeout time.Duration) ResponseWait {
	t.Helper()
	at, err := time.Parse(time.RFC3339Nano, start)
	if err != nil {
		t.Fatal(err)
	}
	return ResponseWait{URLContains: "/api/orders/submit", Method: "POST", Start: at, Timeout: timeout}
}

// TestResponseWaitRequestNeverSent: TSA-V0-202 — no matching request in the window is
// REQUEST_NEVER_SENT with the earlier 4xx on the same page; another page's 500 is excluded and
// the query string is dropped.
func TestResponseWaitRequestNeverSent(t *testing.T) {
	report := DiagnoseResponseWait(mustLog(t, waitHAR), waitAt(t, "2026-10-19T10:00:05.250Z", 30*time.Second), "")
	if report.Verdict != WaitRequestNeverSent || report.Matched != nil {
		t.Fatalf("verdict = %+v", report)
	}
	if report.FailureScope != FailureScopePage || report.Page != "page@2" || len(report.PrecedingFailures) != 1 {
		t.Fatalf("failures = %+v", report)
	}
	failed := report.PrecedingFailures[0]
	if failed.Status != 404 || failed.URL != "http://app.test/api/orders/placeholder" || strings.Contains(failed.URL, "token") {
		t.Fatalf("failure = %+v", failed)
	}
}

// TestResponseWaitSlowResponse: TSA-V0-203 — a matching request that answered after the deadline
// is SLOW_RESPONSE with its duration and lateness; one with no response is SLOW_RESPONSE with
// no-response-recorded; one answered in the window is RESPONSE_IN_WAIT.
func TestResponseWaitSlowResponse(t *testing.T) {
	slow := strings.Replace(waitHAR, `]}}`, `,
 {"pageref":"page@2","startedDateTime":"2026-10-19T10:00:06.000Z","time":45000,"request":{"method":"POST","url":"http://app.test/api/orders/submit"},"response":{"status":201}}]}}`, 1)
	report := DiagnoseResponseWait(mustLog(t, slow), waitAt(t, "2026-10-19T10:00:05.250Z", 30*time.Second), "")
	if report.Verdict != WaitSlowResponse || report.Matched == nil || report.Matched.DurationMs == nil || *report.Matched.DurationMs != 45000 {
		t.Fatalf("slow = %+v", report)
	}
	if report.LateByMs == nil || *report.LateByMs != 15750 || len(report.PrecedingFailures) != 0 {
		t.Fatalf("late = %+v", report)
	}
	pending := strings.Replace(slow, `"time":45000`, `"time":-1`, 1)
	pending = strings.Replace(pending, `"status":201`, `"status":0`, 1)
	if r := DiagnoseResponseWait(mustLog(t, pending), waitAt(t, "2026-10-19T10:00:05.250Z", 30*time.Second), ""); r.Verdict != WaitSlowResponse || r.Reason != WaitNoResponse || r.Matched.DurationMs != nil {
		t.Fatalf("pending = %+v", r)
	}
	fast := strings.Replace(slow, `"time":45000`, `"time":800`, 1)
	if r := DiagnoseResponseWait(mustLog(t, fast), waitAt(t, "2026-10-19T10:00:05.250Z", 30*time.Second), ""); r.Verdict != WaitResponseInWait {
		t.Fatalf("fast = %+v", r)
	}
	// A matching request after the deadline, or one of another method, does not satisfy the wait.
	after := strings.Replace(slow, `10:00:06.000Z`, `10:00:40.000Z`, 1)
	if r := DiagnoseResponseWait(mustLog(t, after), waitAt(t, "2026-10-19T10:00:05.250Z", 30*time.Second), ""); r.Verdict != WaitRequestNeverSent {
		t.Fatalf("after deadline = %+v", r)
	}
	get := strings.Replace(slow, `"method":"POST"`, `"method":"GET"`, 1)
	if r := DiagnoseResponseWait(mustLog(t, get), waitAt(t, "2026-10-19T10:00:05.250Z", 30*time.Second), ""); r.Verdict != WaitRequestNeverSent {
		t.Fatalf("other method = %+v", r)
	}
	// A later match answered within the window outranks an earlier slow one.
	retried := strings.Replace(slow, `]}}`, `,
 {"pageref":"page@2","startedDateTime":"2026-10-19T10:00:07.000Z","time":500,"request":{"method":"POST","url":"http://app.test/api/orders/submit"},"response":{"status":201}}]}}`, 1)
	if r := DiagnoseResponseWait(mustLog(t, retried), waitAt(t, "2026-10-19T10:00:05.250Z", 30*time.Second), ""); r.Verdict != WaitResponseInWait {
		t.Fatalf("retried = %+v", r)
	}
	// A response that completed before the wait began cannot satisfy it.
	early := strings.Replace(slow, `10:00:06.000Z","time":45000`, `10:00:05.000Z","time":100`, 1)
	if r := DiagnoseResponseWait(mustLog(t, early), waitAt(t, "2026-10-19T10:00:05.250Z", 30*time.Second), ""); r.Verdict != WaitRequestNeverSent {
		t.Fatalf("early = %+v", r)
	}
}

// TestResponseWaitServerLog: TSA-V0-200, TSA-V0-202 — a JSON-lines server log has no pages, so the
// preceding failures are scoped to the log; 4xx and 5xx count, 2xx and 3xx do not.
func TestResponseWaitServerLog(t *testing.T) {
	text := `{"time":"2026-10-19T10:00:05.100Z","method":"GET","path":"/orders/new","status":302,"durationMs":3}
{"time":"2026-10-19T10:00:05.200Z","method":"GET","path":"/api/orders/placeholder","status":404,"durationMs":2}
{"time":"2026-10-19T10:00:05.210Z","method":"GET","path":"/api/catalog","status":503,"durationMs":2}
{"time":"2026-10-19T10:00:05.220Z","method":"GET","path":"/api/ok","status":200,"durationMs":2}
`
	report := DiagnoseResponseWait(mustLog(t, text), waitAt(t, "2026-10-19T10:00:05.000Z", 5*time.Second), "")
	if report.Verdict != WaitRequestNeverSent || report.FailureScope != FailureScopeLog || report.LogSource != LogSourceServer {
		t.Fatalf("server = %+v", report)
	}
	if len(report.PrecedingFailures) != 2 || report.PrecedingFailures[0].Status != 404 || report.PrecedingFailures[1].Status != 503 {
		t.Fatalf("failures = %+v", report.PrecedingFailures)
	}
	slow := text + `{"time":"2026-10-19T10:00:06.000Z","method":"POST","path":"/api/orders/submit","status":200,"durationMs":9000}` + "\n"
	if r := DiagnoseResponseWait(mustLog(t, slow), waitAt(t, "2026-10-19T10:00:05.000Z", 5*time.Second), ""); r.Verdict != WaitSlowResponse || *r.Matched.DurationMs != 9000 || *r.LateByMs != 5000 {
		t.Fatalf("server slow = %+v", r)
	}
}

// TestResponseWaitAbstains: TSA-V0-204 — a missing, empty, malformed or late-starting log is
// UNKNOWN, never REQUEST_NEVER_SENT.
func TestResponseWaitAbstains(t *testing.T) {
	wait := waitAt(t, "2026-10-19T10:00:05.000Z", 5*time.Second)
	if r := DiagnoseResponseWait(nil, wait, WaitLogUnavailable); r.Verdict != WaitUnknown || r.Reason != WaitLogUnavailable {
		t.Fatalf("missing = %+v", r)
	}
	if r := DiagnoseResponseWait(mustLog(t, `{"log":{"entries":[]}}`), wait, ""); r.Verdict != WaitUnknown || r.Reason != WaitLogEmpty {
		t.Fatalf("empty = %+v", r)
	}
	late := `{"time":"2026-10-19T11:00:00Z","method":"GET","path":"/x","status":200,"durationMs":1}`
	if r := DiagnoseResponseWait(mustLog(t, late), wait, ""); r.Verdict != WaitUnknown || r.Reason != WaitLogNotCovering {
		t.Fatalf("late log = %+v", r)
	}
	noTime := `{"log":{"entries":[{"startedDateTime":"2026-10-19T10:00:05.100Z","request":{"url":"/a"},"response":{"status":200}}]}}`
	hugeTime := strings.Replace(noTime, `"request"`, `"time":1e300,"request"`, 1)
	hugeLine := `{"time":"2026-10-19T10:00:05.100Z","path":"/x","status":200,"durationMs":1e300}`
	for _, bad := range []string{`not json`, `{"time":"yesterday","path":"/x"}`, `{"log":{"entries":[{"startedDateTime":"x","request":{"url":"/a"}}]}}`, noTime, hugeTime, hugeLine} {
		if _, err := ReadRequestLog([]byte(bad)); err != ErrShape {
			t.Fatalf("%q: err = %v", bad, err)
		}
	}
	if _, err := ReadRequestLogFile("/nonexistent/corvint/requests.har"); err != ErrUnreadable {
		t.Fatalf("missing file err = %v", err)
	}
}
