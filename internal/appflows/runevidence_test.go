package appflows

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func runHeader() RunHeader {
	digest := strings.Repeat("a", 64)
	return RunHeader{RunID: "run-1", RunnerVersion: "1.50.0", Source: RunSource{Commit: strings.Repeat("1", 40), Tree: strings.Repeat("2", 40), Clean: true},
		BuildArtifactDigest: digest, Environment: RunDigestRef{ID: "ci-linux", Digest: digest}, Fixture: RunDigestRef{ID: "seed-1", Digest: digest}, Cleanup: "done"}
}

func ingest(t *testing.T, format, raw string, header RunHeader) map[string]TestRunEvidence {
	t.Helper()
	got, err := IngestRunEvidence(format, []byte(raw), header)
	if err != nil || got.Incomplete != "" {
		t.Fatalf("ingest: %v %q", err, got.Incomplete)
	}
	byKey := map[string]TestRunEvidence{}
	for _, r := range got.Records {
		byKey[r.TestKey] = r
	}
	return byKey
}

func outcomes(r TestRunEvidence) string {
	names := []string{}
	for _, a := range r.Attempts {
		names = append(names, a.Outcome)
	}
	return strings.Join(names, ",")
}

const playwrightRun = `{"suites":[{"file":"checkout.spec.ts","specs":[
 {"title":"pays","tests":[{"projectName":"chromium","results":[
  {"status":"failed","retry":0,"duration":112.4,"error":{"message":"expected 2","location":{"file":"checkout.spec.ts","line":36}},
   "attachments":[{"name":"screenshot","path":"test-results/pays/test-failed-1.png"},{"name":"stdout","body":"inline"}]},
  {"status":"passed","retry":1,"duration":99,"errors":[],"attachments":[]}]}]},
 {"title":"slow","tests":[{"projectName":"chromium","results":[{"status":"timedOut","retry":0,"duration":30000,"errors":[{"message":"Test timeout of 30000ms exceeded."}],"attachments":[]}]}]},
 {"title":"rejects bad card","tests":[{"projectName":"chromium","annotations":[{"type":"corvint-test-key","description":"control.bad-card"}],"results":[{"status":"passed","retry":0,"duration":5,"attachments":[]}]}]}
]}]}`

// AFU-V1-011
func TestAFUV1RunEvidenceClosedSchema(t *testing.T) {
	r := ingest(t, FormatPlaywrightJSON, playwrightRun, runHeader())["checkout.spec.ts > pays"]
	raw, err := EncodeRunEvidence(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"schema":"test-run-evidence/0"`, `"authority":"INGESTED"`, `"run_id":"run-1"`, `"runner":{"name":"playwright","version":"1.50.0"}`,
		`"build_artifact_digest"`, `"environment"`, `"fixture"`, `"project":"chromium"`, `"cleanup":"done"`, `"negative_controls":[]`} {
		if !bytes.Contains(raw, []byte(field)) {
			t.Fatalf("encoding lacks %s: %s", field, raw)
		}
	}
	back, err := DecodeRunEvidence(raw)
	if err != nil || back.TestKey != r.TestKey || len(back.Attempts) != 2 {
		t.Fatalf("round trip: %v %+v", err, back)
	}
	refused := map[string][]byte{
		"unknown field": bytes.Replace(raw, []byte(`"cleanup"`), []byte(`"extra":1,"cleanup"`), 1),
		"outcome":       bytes.Replace(raw, []byte(`"outcome":"failed"`), []byte(`"outcome":"broken"`), 1),
		"cleanup":       bytes.Replace(raw, []byte(`"cleanup":"done"`), []byte(`"cleanup":"maybe"`), 1),
		"ordinal":       bytes.Replace(raw, []byte(`"ordinal":2`), []byte(`"ordinal":3`), 1),
		"authority":     bytes.Replace(raw, []byte(`"INGESTED"`), []byte(`"VERIFIED"`), 1),
		"not canonical": append([]byte(" "), raw...),
	}
	for name, bad := range refused {
		if _, err := DecodeRunEvidence(bad); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// AFU-V1-011
func TestAFUV1NegativeControlFailed(t *testing.T) {
	header := runHeader()
	header.Controls = []RunControl{{Subject: "checkout.spec.ts > slow", TestKey: "control.bad-card", Expected: "failed"}, {Subject: "checkout.spec.ts > slow", TestKey: "control.absent", Expected: "failed"}}
	slow := ingest(t, FormatPlaywrightJSON, playwrightRun, header)["checkout.spec.ts > slow"]
	want := []NegativeControl{{TestKey: "control.bad-card", Expected: "failed", Observed: "passed"}, {TestKey: "control.absent", Expected: "failed", Observed: "not-run"}}
	if fmt.Sprint(slow.NegativeControls) != fmt.Sprint(want) {
		t.Fatalf("controls: %+v", slow.NegativeControls)
	}
	pass := ingest(t, FormatPlaywrightJSON, playwrightRun, runHeader())["control.bad-card"]
	pass.NegativeControls = []NegativeControl{{TestKey: "control.x", Expected: "failed", Observed: "passed"}}
	if Verified(pass) {
		t.Fatal("a failed negative control verified its subject")
	}
	pass.NegativeControls[0].Observed = "failed"
	if !Verified(pass) {
		t.Fatal("a negative control that failed as expected blocked verification")
	}
	pass.NegativeControls[0] = NegativeControl{TestKey: "control.x", Expected: "passed", Observed: "passed"}
	if Verified(pass) {
		t.Fatal("a negative control that passed as its declared expectation verified its subject")
	}
}

// AFU-V1-012
func TestAFUV1PlaywrightAdapterKeepsEveryAttempt(t *testing.T) {
	records := ingest(t, FormatPlaywrightJSON, playwrightRun, runHeader())
	pays := records["checkout.spec.ts > pays"]
	if outcomes(pays) != "failed,passed" || Classify(pays.Attempts) != "flaky" {
		t.Fatalf("retry-passed test: %s %s", outcomes(pays), Classify(pays.Attempts))
	}
	first := pays.Attempts[0]
	if first.DurationMS != 112 || first.Failure != "expected 2" || fmt.Sprint(first.AssertionAnchors) != "[{checkout.spec.ts 36}]" {
		t.Fatalf("first attempt: %+v", first)
	}
	if fmt.Sprint(first.Attachments) != "[{screenshot test-results/pays/test-failed-1.png}]" {
		t.Fatalf("first attempt attachments (inline body must be dropped): %+v", first.Attachments)
	}
	if pays.Attempts[1].DurationMS != 99 || pays.Attempts[1].Ordinal != 2 {
		t.Fatalf("second attempt: %+v", pays.Attempts[1])
	}
	if slow := records["checkout.spec.ts > slow"]; outcomes(slow) != "timedOut" || Classify(slow.Attempts) != "timedOut" || slow.Attempts[0].DurationMS != 30000 {
		t.Fatalf("timed-out attempt: %+v", slow)
	}
}

const junitRun = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites><testsuite name="api">
 <testcase classname="api.Orders" name="creates" time="0.5">
  <flakyFailure message="timeout" time="1.25"><stackTrace>at Orders.java:40</stackTrace><system-out>[[ATTACHMENT|target/orders-1.png]]</system-out></flakyFailure>
  <system-out>[[ATTACHMENT|target/orders-2.png]]</system-out>
 </testcase>
 <testcase classname="api.Orders" name="cancels" time="0.2">
  <failure message="expected 204">at Orders.java:77</failure>
  <rerunFailure message="expected 204 again"><stackTrace>at Orders.java:77</stackTrace></rerunFailure>
 </testcase>
 <testcase classname="api.Orders" name="refunds" time="0"><skipped/></testcase>
</testsuite></testsuites>`

// AFU-V1-012
func TestAFUV1JUnitAdapterKeepsEveryAttempt(t *testing.T) {
	records := ingest(t, FormatJUnitXML, junitRun, runHeader())
	creates := records["api.Orders > creates"]
	if outcomes(creates) != "failed,passed" || Classify(creates.Attempts) != "flaky" {
		t.Fatalf("flakyFailure test: %s", outcomes(creates))
	}
	if first := creates.Attempts[0]; first.DurationMS != 1250 || first.Failure != "timeout\nat Orders.java:40" || fmt.Sprint(first.Attachments) != "[{orders-1.png target/orders-1.png}]" {
		t.Fatalf("flaky attempt: %+v", first)
	}
	if last := creates.Attempts[1]; last.DurationMS != 500 || fmt.Sprint(last.Attachments) != "[{orders-2.png target/orders-2.png}]" {
		t.Fatalf("final attempt: %+v", last)
	}
	cancels := records["api.Orders > cancels"]
	if outcomes(cancels) != "failed,failed" || cancels.Attempts[0].Failure != "expected 204\nat Orders.java:77" || cancels.Attempts[1].Failure != "expected 204 again\nat Orders.java:77" {
		t.Fatalf("rerunFailure test: %+v", cancels.Attempts)
	}
	if outcomes(records["api.Orders > refunds"]) != "skipped" {
		t.Fatalf("skipped test: %+v", records["api.Orders > refunds"])
	}
	nested := ingest(t, FormatJUnitXML, `<testcase name="n"><failure message="m">before<stackTrace>st</stackTrace>after</failure></testcase>`, runHeader())["n"]
	if nested.Attempts[0].Failure != "m\nbefore\nst\nafter" {
		t.Fatalf("text around a nested element: %q", nested.Attempts[0].Failure)
	}
	if _, err := IngestRunEvidence(FormatJUnitXML, []byte(`<!DOCTYPE x><testsuite/>`), runHeader()); err == nil {
		t.Fatal("DTD accepted")
	}
}

const goTestStream = `{"Action":"run","Package":"shop","Test":"TestPay"}
{"Action":"output","Package":"shop","Test":"TestPay","Output":"    pay_test.go:12: declined\n"}
{"Action":"fail","Package":"shop","Test":"TestPay","Elapsed":0.25}
{"Action":"run","Package":"shop","Test":"TestPay"}
{"Action":"pass","Package":"shop","Test":"TestPay","Elapsed":0.1}
{"Action":"run","Package":"shop","Test":"TestSlow"}
{"Action":"output","Package":"shop","Test":"TestSlow","Output":"panic: test timed out after 1s\n"}
{"Action":"fail","Package":"shop","Elapsed":1.2}
`

// AFU-V1-012
func TestAFUV1GoTestAdapterKeepsEveryAttempt(t *testing.T) {
	records := ingest(t, FormatGoTestJSON, goTestStream, runHeader())
	pay := records["shop > TestPay"]
	if outcomes(pay) != "failed,passed" || Classify(pay.Attempts) != "flaky" {
		t.Fatalf("repeated run: %s", outcomes(pay))
	}
	if first := pay.Attempts[0]; first.DurationMS != 250 || first.Failure != "pay_test.go:12: declined" || fmt.Sprint(first.AssertionAnchors) != "[{pay_test.go 12}]" {
		t.Fatalf("first attempt: %+v", first)
	}
	if slow := records["shop > TestSlow"]; outcomes(slow) != "timedOut" {
		t.Fatalf("timed-out test: %+v", slow)
	}
	cut := strings.Replace(goTestStream, "panic: test timed out after 1s", "killed", 1)
	if slow := ingest(t, FormatGoTestJSON, cut, runHeader())["shop > TestSlow"]; outcomes(slow) != "interrupted" {
		t.Fatalf("unfinished test: %+v", slow)
	}
}

// AFU-V1-013
func TestAFUV1FailedAttemptThenPassIsFlaky(t *testing.T) {
	cases := map[string]string{"failed,passed": "flaky", "timedOut,passed": "flaky", "failed,failed,passed": "flaky", "passed": "passed",
		"passed,failed": "failed", "skipped": "skipped", "interrupted": "interrupted", "": "not-run"}
	for sequence, want := range cases {
		attempts := []RunAttempt{}
		for _, outcome := range strings.Split(sequence, ",") {
			if outcome != "" {
				attempts = append(attempts, RunAttempt{Outcome: outcome})
			}
		}
		if got := Classify(attempts); got != want {
			t.Fatalf("%s: got %s, want %s", sequence, got, want)
		}
	}
	if flaky := ingest(t, FormatPlaywrightJSON, playwrightRun, runHeader())["checkout.spec.ts > pays"]; Verified(flaky) {
		t.Fatal("a flaky test verified")
	}
}

// AFU-V1-013
func TestAFUV1PlaywrightUnexpectedNeverPassed(t *testing.T) {
	report := `{"suites":[{"file":"known.spec.ts","specs":[
 {"title":"known bug","tests":[{"expectedStatus":"failed","status":"unexpected","results":[{"status":"passed","duration":3,"attachments":[]}]}]},
 {"title":"odd","tests":[{"expectedStatus":"passed","status":"unexpected","results":[{"status":"passed","duration":3,"attachments":[]}]}]}]}]}`
	for key, r := range ingest(t, FormatPlaywrightJSON, report, runHeader()) {
		if Classify(r.Attempts) != "failed" || Verified(r) || r.Attempts[0].Failure == "" {
			t.Fatalf("%s: an unexpected pass classified %s: %+v", key, Classify(r.Attempts), r.Attempts)
		}
	}
}

// AFU-V1-014
func TestAFUV1StaticNeverVerified(t *testing.T) {
	static := TestRunEvidence{Schema: RunEvidenceSchema, Authority: AuthorityStatic, TestKey: "checkout.spec.ts > pays", Attempts: []RunAttempt{}, Cleanup: "not-declared", NegativeControls: []NegativeControl{}}
	if _, err := EncodeRunEvidence(static); err != nil {
		t.Fatalf("static mapping refused: %v", err)
	}
	if Verified(static) {
		t.Fatal("a STATIC record verified")
	}
	static.Attempts = []RunAttempt{{Ordinal: 1, Outcome: "passed", AssertionAnchors: []RunAnchor{}, Attachments: []RunAttachment{}}}
	if _, err := EncodeRunEvidence(static); err == nil {
		t.Fatal("a STATIC record carried a passed attempt")
	}
	for _, authority := range []string{AuthorityIngested, AuthorityLocallyObserved} {
		r := ingest(t, FormatGoTestJSON, goTestStream, runHeader())["shop > TestSlow"]
		r.Authority = authority
		if _, err := EncodeRunEvidence(r); err != nil {
			t.Fatalf("%s refused: %v", authority, err)
		}
	}
	passed := ingest(t, FormatPlaywrightJSON, playwrightRun, runHeader())["control.bad-card"]
	if !Verified(passed) {
		t.Fatal("a valid passed record did not verify")
	}
	for _, authority := range []string{"", "VERIFIED"} {
		passed.Authority = authority
		if Verified(passed) {
			t.Fatalf("authority %q verified", authority)
		}
	}
}

// AFU-V1-037
func TestAFUV1RunEvidenceBoundsIncomplete(t *testing.T) {
	var records, attempts, links strings.Builder
	for i := 0; i <= maxRunRecords; i++ {
		fmt.Fprintf(&records, `{"Action":"run","Package":"p","Test":"T%d"}`+"\n", i)
	}
	for i := 0; i <= maxRunAttempts; i++ {
		attempts.WriteString(`{"Action":"run","Package":"p","Test":"T"}` + "\n" + `{"Action":"pass","Package":"p","Test":"T"}` + "\n")
	}
	links.WriteString(`{"Action":"run","Package":"p","Test":"T"}` + "\n")
	for i := 0; i <= maxRunLinks; i++ {
		links.WriteString(`{"Action":"output","Package":"p","Test":"T","Output":"    a_test.go:1: x\n"}` + "\n")
	}
	links.WriteString(`{"Action":"fail","Package":"p","Test":"T"}` + "\n")
	playwright := func(specs, results, attachments int) string {
		attachment := strings.TrimSuffix(strings.Repeat(`{"name":"a","path":"a.png"},`, attachments), ",")
		result := strings.TrimSuffix(strings.Repeat(`{"status":"passed","attachments":[`+attachment+`]},`, results), ",")
		spec := strings.TrimSuffix(strings.Repeat(`{"title":"t","tests":[{"results":[`+result+`]}]},`, specs), ",")
		return `{"suites":[{"file":"f.spec.ts","specs":[` + spec + `]}]}`
	}
	escaping := `<testcase name="e"><failure>` + strings.Repeat(`"`, MaxBytes*3/5) + `</failure></testcase>`
	cases := []struct{ format, raw, code string }{
		{FormatGoTestJSON, strings.Repeat(" ", MaxBytes+1), BoundBytes},
		{FormatJUnitXML, escaping, BoundBytes},
		{FormatPlaywrightJSON, playwright(maxRunRecords+1, 1, 0), BoundRecords},
		{FormatPlaywrightJSON, playwright(1, maxRunAttempts+1, 0), BoundAttempts},
		{FormatPlaywrightJSON, playwright(1, 1, maxRunLinks+1), BoundLinks},
		{FormatGoTestJSON, records.String(), BoundRecords},
		{FormatGoTestJSON, attempts.String(), BoundAttempts},
		{FormatGoTestJSON, links.String(), BoundLinks},
		{FormatPlaywrightJSON, strings.Repeat("[", maxRunDepth+1) + strings.Repeat("]", maxRunDepth+1), BoundTraversal},
		{FormatJUnitXML, strings.Repeat("<a>", maxRunDepth+1) + strings.Repeat("</a>", maxRunDepth+1), BoundTraversal},
	}
	for _, c := range cases {
		got, err := IngestRunEvidence(c.format, []byte(c.raw), runHeader())
		if err != nil || got.Incomplete != c.code || len(got.Records) != 0 {
			t.Fatalf("%s: got %q with %d records, err %v", c.code, got.Incomplete, len(got.Records), err)
		}
	}
}

// AFU-V1-038
func TestAFUV1RunEvidenceSecretsDropped(t *testing.T) {
	report := `{"suites":[{"file":"login.spec.ts","specs":[{"title":"logs in","tests":[{"results":[{"status":"failed","duration":1,
 "error":{"message":"request failed\nCookie: sid=abc123\nAuthorization: Bearer abc.def\nx-csrf-token: 42\nResponse body:\n{\"user\":\"jo\"}"},
 "attachments":[{"name":"screenshot","path":"shot.png"},{"name":"cookies","path":"c.json"},{"name":"network","path":"run.har"},{"name":"response","path":"r.json"}]}]}]}]}]}`
	r := ingest(t, FormatPlaywrightJSON, report, runHeader())["login.spec.ts > logs in"]
	failure := r.Attempts[0].Failure
	for _, leaked := range []string{"sid=abc123", "Bearer", "x-csrf-token", "jo"} {
		if strings.Contains(failure, leaked) {
			t.Fatalf("failure kept %q: %s", leaked, failure)
		}
	}
	if !strings.HasPrefix(failure, "request failed\n"+droppedMarker) || fmt.Sprint(r.Attempts[0].Attachments) != "[{screenshot shot.png}]" {
		t.Fatalf("scrubbed attempt: %+v", r.Attempts[0])
	}
	// Each input survived the first, pattern-precise screen verbatim.
	bypasses := map[string]string{
		`headers {"cookie":"sid=abc123","authorization":"Basic dXNlcjpwYXNz"}`: "",
		`got {'set-cookie': 'sid=abc123'}`:                                     "",
		`body was {"access_token":"eyJhbGciOi.abc.def"}`:                       "",
		`Authorization Bearer abc123def456`:                                    "",
		`POST /login failed. Response body: {"user":"jo","ssn":"123"}`:         "POST /login failed. ",
	}
	for line, prefix := range bypasses {
		if got := scrubFailure(line + "\nnext"); !strings.HasPrefix(got, prefix+droppedMarker) || strings.Contains(got, "abc") || strings.Contains(got, "ssn") {
			t.Fatalf("%s: scrubbed to %q", line, got)
		}
	}
	stream := `{"Action":"run","Package":"api","Test":"TestLogin"}
{"Action":"output","Package":"api","Test":"TestLogin","Output":"    api_test.go:20: Response body: {\"session\":\"s3cr3t-value\"}\n"}
{"Action":"fail","Package":"api","Test":"TestLogin","Elapsed":0.1}
`
	login := ingest(t, FormatGoTestJSON, stream, runHeader())["api > TestLogin"]
	if login.Attempts[0].Failure != "api_test.go:20: "+droppedMarker || fmt.Sprint(login.Attempts[0].AssertionAnchors) != "[{api_test.go 20}]" {
		t.Fatalf("go test body kept: %+v", login.Attempts[0])
	}
	secret := strings.Replace(report, "request failed", "leaked ghp_abcdefghijklmnopqrstuvwxyz0123456789", 1)
	if _, err := IngestRunEvidence(FormatPlaywrightJSON, []byte(secret), runHeader()); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("secret-shaped failure accepted: %v", err)
	}
}
