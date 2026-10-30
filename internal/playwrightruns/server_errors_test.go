package playwrightruns

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// OUE-V0-050: the 5xx an error names, with method and path when present, never message text.
func TestHTTPFailure(t *testing.T) {
	cases := []struct {
		msg  string
		want HTTPFailure
		ok   bool
	}{
		{"Error: expect(received).toBe(expected)\n\nExpected: 200\nReceived: 500", HTTPFailure{Status: 500}, true},
		{"apiRequestContext.post: 503 Service Unavailable\nCall log:\n  - → POST https://app.test/api/orders?token=abc", HTTPFailure{Status: 503, Method: "POST", Path: "/api/orders"}, true},
		{"Error: HTTP 502 for GET /api/users/42?x=1 request-id: req-abc123", HTTPFailure{Status: 502, Method: "GET", Path: "/api/users/42", RequestID: "req-abc123"}, true},
		{"\x1b[31mResponse status 500\x1b[39m from PUT http://localhost:3000/items/7", HTTPFailure{Status: 500, Method: "PUT", Path: "/items/7"}, true},
		{"Timeout 5000ms exceeded waiting for locator('#save')", HTTPFailure{}, false},
		{"Expected 404 got 404", HTTPFailure{}, false},
		{"order 5001 not found", HTTPFailure{}, false},
	}
	for _, c := range cases {
		got, ok := httpFailure(c.msg)
		if ok != c.ok || (ok && got != c.want) {
			t.Fatalf("%q: got %+v %v want %+v %v", c.msg, got, ok, c.want, c.ok)
		}
	}
}

func serverLog(t *testing.T, recs ...map[string]any) []ServerError {
	t.Helper()
	var b strings.Builder
	for _, r := range recs {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	errs, err := ReadServerErrorLog([]byte(b.String()))
	if err != nil {
		t.Fatalf("ReadServerErrorLog: %v", err)
	}
	return errs
}

func serverRec(at, method, path string, status int, class, msg string, frames ...ServerFrame) map[string]any {
	return map[string]any{"time": at, "method": method, "path": path, "status": status,
		"exception": map[string]any{"class": class, "message": msg, "frames": frames}}
}

// OUE-V0-051..056: attach, classify, not-found, ambiguous, missing start time, grouping.
func TestAttachServerErrors(t *testing.T) {
	failed := func(start, msg string) fxResult {
		r := fail(msg, "", 0)
		r.start = start
		return r
	}
	run1 := mustParse(t, reportJSON(t, "1.63.0", nil,
		fxTest{file: "orders.spec.ts", title: "creates an order", status: "unexpected", results: []fxResult{
			failed("2026-10-25T10:00:00.000Z", "Error: HTTP 500 for POST /api/orders?debug=1")}},
		fxTest{file: "seed.spec.ts", title: "seeds a user", status: "unexpected", results: []fxResult{
			failed("2026-10-25T10:00:05.000Z", "apiRequestContext.post: 500 Internal Server Error\n → POST http://app.test/api/users")}},
		fxTest{file: "lost.spec.ts", title: "nothing logged", status: "unexpected", results: []fxResult{
			failed("2026-10-25T10:01:00.000Z", "Error: HTTP 500 for GET /api/missing")}},
		fxTest{file: "vague.spec.ts", title: "two candidates", status: "unexpected", results: []fxResult{
			failed("2026-10-25T10:02:00.000Z", "Expected: 200\nReceived: 500")}},
		fxTest{file: "nostart.spec.ts", title: "no start", status: "unexpected", results: []fxResult{
			fail("Error: HTTP 500 for GET /api/x", "", 0)}},
		fxTest{file: "ok.spec.ts", title: "passes", status: "expected", results: []fxResult{pass()}},
	))
	run2 := mustParse(t, reportJSON(t, "1.63.0", nil,
		fxTest{file: "orders.spec.ts", title: "creates an order", status: "unexpected", results: []fxResult{
			failed("2026-10-26T09:00:00.000Z", "Error: HTTP 500 for POST /api/orders")}},
	))
	errs := serverLog(t,
		serverRec("2026-10-25T10:00:00.300Z", "POST", "/api/orders", 500, "NoMethodError", "undefined method `total' for nil (order 1234)",
			ServerFrame{File: "/usr/local/bundle/gems/actionpack-7.1/lib/x.rb", Line: 9}, ServerFrame{File: "app/models/order.rb", Line: 42, Function: "total"}),
		serverRec("2026-10-26T09:00:00.200Z", "POST", "/api/orders", 500, "NoMethodError", "undefined method `total' for nil (order 98)",
			ServerFrame{File: "app/models/order.rb", Line: 42, Function: "total"}),
		serverRec("2026-10-25T10:00:05.100Z", "POST", "/api/users", 500, "ActiveRecord::RecordInvalid", "Validation failed: email taken",
			ServerFrame{File: "spec/support/factories/users.rb", Line: 7}),
		serverRec("2026-10-25T10:02:00.010Z", "GET", "/api/a", 500, "KeyError", "a", ServerFrame{File: "app/a.rb", Line: 1}),
		serverRec("2026-10-25T10:02:00.020Z", "GET", "/api/b", 500, "TypeError", "b", ServerFrame{File: "app/b.rb", Line: 2}),
		serverRec("2026-10-25T12:00:00.000Z", "GET", "/api/missing", 500, "Late", "outside the window"),
	)
	rep := AttachServerErrors([]*Run{run1, run2}, errs, nil, ServerErrorOptions{SupportPaths: []string{"spec/support"}})
	byFile := map[string]ServerErrorFailure{}
	for _, f := range rep.Failures {
		byFile[f.File] = f
	}
	if f := byFile["orders.spec.ts"]; f.State != ServerErrorAttached || f.Classification != ClassProduct || f.Exception.Frame == nil ||
		f.Exception.Frame.File != "app/models/order.rb" || f.Exception.Class != "NoMethodError" || f.Path != "/api/orders" {
		t.Fatalf("orders: %+v", f)
	}
	if f := byFile["seed.spec.ts"]; f.State != ServerErrorAttached || f.Classification != ClassEnvironmentFixture {
		t.Fatalf("seed: %+v", f)
	}
	if f := byFile["lost.spec.ts"]; f.State != ServerErrorNotFound || f.Reason != ServerErrorNotFoundReason {
		t.Fatalf("lost: %+v", f)
	}
	if f := byFile["vague.spec.ts"]; f.State != ServerErrorUnknown || !strings.HasPrefix(f.Reason, "ambiguous") || f.MatchBasis != "status-time" {
		t.Fatalf("vague: %+v", f)
	}
	if f := byFile["nostart.spec.ts"]; f.State != ServerErrorUnknown || !strings.Contains(f.Reason, "start time") {
		t.Fatalf("nostart: %+v", f)
	}
	c := rep.Counts
	if c.Failures != 6 || c.Attached != 3 || c.EnvironmentFixture != 1 || c.NotFound != 1 || c.Unknown != 2 || c.Attached+c.NotFound+c.Unknown != c.Failures {
		t.Fatalf("counts: %+v", c)
	}
	// The two order failures in two runs share one group despite different order numbers.
	if c.Groups != 2 || rep.Groups[0].Occurrences != 2 || len(rep.Groups[0].Runs) != 2 || len(rep.Groups[0].Tests) != 1 {
		t.Fatalf("groups: %+v", rep.Groups)
	}
	raw, _ := json.Marshal(rep)
	for _, leak := range []string{"debug=1", "gems/actionpack"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("report leaks %q", leak)
		}
	}

	// A request log names the method and path a bare status assertion leaves out.
	logRaw := `{"time":"2026-10-25T10:02:00.005Z","method":"GET","url":"http://app.test/api/b?q=1","status":500,"durationMs":15}` + "\n"
	reqs, err := ReadRequestLog([]byte(logRaw))
	if err != nil {
		t.Fatalf("ReadRequestLog: %v", err)
	}
	// A known method still constrains the match when the path is unknown: a lone GET
	// exception never attaches to a failed POST.
	run3 := mustParse(t, reportJSON(t, "1.63.0", nil,
		fxTest{file: "post.spec.ts", title: "posts", status: "unexpected", results: []fxResult{
			failed("2026-10-25T10:02:00.000Z", "apiRequestContext.post: 500 Internal Server Error")}},
	))
	lone := serverLog(t, serverRec("2026-10-25T10:02:00.010Z", "GET", "/api/a", 500, "KeyError", "a", ServerFrame{File: "app/a.rb", Line: 1}))
	if f := AttachServerErrors([]*Run{run3}, lone, nil, ServerErrorOptions{}).Failures[0]; f.State != ServerErrorNotFound || f.MatchBasis != "method-time" {
		t.Fatalf("method-only: %+v", f)
	}
	rep = AttachServerErrors([]*Run{run1}, errs, reqs, ServerErrorOptions{Window: time.Second})
	for _, f := range rep.Failures {
		if f.File == "vague.spec.ts" && (f.State != ServerErrorAttached || f.Exception.Class != "TypeError" || f.MatchBasis != "request-log-method-path-time") {
			t.Fatalf("vague with request log: %+v", f)
		}
	}
}

func TestReadServerErrorLogRefusals(t *testing.T) {
	for _, bad := range []string{
		`{"time":"x","method":"GET","path":"/a","status":500,"exception":{"class":"E"}}`,
		`{"time":"2026-10-25T10:00:00Z","method":"GET","path":"/a","status":500,"exception":{"class":""}}`,
		`{"time":"2026-10-25T10:00:00Z","method":"GET","path":"/a","status":500,"exception":{"class":"E"},"body":"secret"}`,
		`{"time":"2026-10-25T10:00:00Z","method":"GET","path":"/a","exception":{"class":"E"}}`,
		`not json`,
	} {
		if _, err := ReadServerErrorLog([]byte(bad + "\n")); err != ErrShape {
			t.Fatalf("%s: got %v", bad, err)
		}
	}
}
