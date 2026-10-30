package jstestprovider

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func requestOutcomeAnnotation(description string) playwrightAnnotation {
	return playwrightAnnotation{Type: RequestOutcomesAnnotation, Description: description}
}

// LPCV-V0-080: only exactly one closed, in-bounds annotation is a record; anything else is no
// record, never a classification.
func TestLPCVV0080AnnotationIsClosed(t *testing.T) {
	valid := `{"assertion":"rejection-only","actor":"tenant-a","positiveSibling":"owner reads record","requests":[{"method":"GET","route":"/api/records/:id","status":403}]}`
	got := requestOutcomeRecord([]playwrightAnnotation{{Type: "other"}, requestOutcomeAnnotation(valid)})
	if got == nil || got.Assertion != AssertionRejectionOnly || got.Actor != "tenant-a" || len(got.Requests) != 1 || got.Requests[0].Status != 403 {
		t.Fatalf("valid record = %+v", got)
	}
	if got := requestOutcomeRecord([]playwrightAnnotation{requestOutcomeAnnotation(`{"assertion":"positive","requests":[]}`)}); got == nil || got.Requests == nil || len(got.Requests) != 0 {
		t.Fatalf("empty record = %+v", got)
	}
	many := make([]string, MaxRequestOutcomes+1)
	for i := range many {
		many[i] = fmt.Sprintf(`{"method":"GET","route":"/r/%d","status":200}`, i)
	}
	for name, annotations := range map[string][]playwrightAnnotation{
		"absent":                  nil,
		"other type":              {{Type: RouteStubsAnnotation, Description: valid}},
		"duplicate":               {requestOutcomeAnnotation(valid), requestOutcomeAnnotation(valid)},
		"malformed":               {requestOutcomeAnnotation(`{"assertion":`)},
		"trailing":                {requestOutcomeAnnotation(valid + ` {}`)},
		"unknown field":           {requestOutcomeAnnotation(`{"assertion":"positive","requests":[],"extra":1}`)},
		"null requests":           {requestOutcomeAnnotation(`{"assertion":"positive","requests":null}`)},
		"missing requests":        {requestOutcomeAnnotation(`{"assertion":"positive"}`)},
		"bad assertion":           {requestOutcomeAnnotation(`{"assertion":"negative","requests":[]}`)},
		"lowercase method":        {requestOutcomeAnnotation(`{"assertion":"positive","requests":[{"method":"get","route":"/a","status":200}]}`)},
		"empty route":             {requestOutcomeAnnotation(`{"assertion":"positive","requests":[{"method":"GET","route":"","status":200}]}`)},
		"long route":              {requestOutcomeAnnotation(`{"assertion":"positive","requests":[{"method":"GET","route":"` + strings.Repeat("x", 1025) + `","status":200}]}`)},
		"missing status":          {requestOutcomeAnnotation(`{"assertion":"positive","requests":[{"method":"GET","route":"/a"}]}`)},
		"null status":             {requestOutcomeAnnotation(`{"assertion":"positive","requests":[{"method":"GET","route":"/a","status":null}]}`)},
		"fractional status":       {requestOutcomeAnnotation(`{"assertion":"positive","requests":[{"method":"GET","route":"/a","status":200.5}]}`)},
		"status out of range":     {requestOutcomeAnnotation(`{"assertion":"positive","requests":[{"method":"GET","route":"/a","status":600}]}`)},
		"rejection-only with 200": {requestOutcomeAnnotation(`{"assertion":"rejection-only","actor":"a","requests":[{"method":"GET","route":"/a","status":200}]}`)},
		"rejection-only with 500": {requestOutcomeAnnotation(`{"assertion":"rejection-only","actor":"a","requests":[{"method":"GET","route":"/a","status":500}]}`)},
		"duplicate status":        {requestOutcomeAnnotation(`{"assertion":"positive","requests":[{"method":"GET","route":"/a","status":403,"status":200}]}`)},
		"case variant member":     {requestOutcomeAnnotation(`{"assertion":"positive","requests":[{"method":"GET","route":"/a","status":null,"Status":200}]}`)},
		"too many requests":       {requestOutcomeAnnotation(`{"assertion":"positive","requests":[` + strings.Join(many, ",") + `]}`)},
		"oversized":               {requestOutcomeAnnotation(`{"assertion":"positive","requests":[]}` + strings.Repeat(" ", maxRequestOutcomeDescription))},
		"non-string":              {{Type: RequestOutcomesAnnotation, Description: valid, malformed: true}},
	} {
		if got := requestOutcomeRecord(annotations); got != nil {
			t.Errorf("%s: record = %+v", name, got)
		}
	}
}

// LPCV-V0-080: the actor, sibling title and route templates pass the product secret screen.
func TestLPCVV0080RecordIsSecretScreened(t *testing.T) {
	token := "ghp_abcdefghijklmnopqrst"
	got := requestOutcomeRecord([]playwrightAnnotation{requestOutcomeAnnotation(`{"assertion":"positive","actor":"` + token + `","positiveSibling":"` + token + `","requests":[{"method":"GET","route":"/api?key=` + token + `","status":200}]}`)})
	if got == nil || strings.Contains(got.Actor, token) || strings.Contains(got.PositiveSibling, token) || strings.Contains(got.Requests[0].Route, token) {
		t.Fatalf("record not screened: %+v", got)
	}
}

// LPCV-V0-080: ParsePlaywrightJSON reads the attempt's record beside a route-stub record; an
// unrelated malformed annotation never fails the report.
func TestLPCVV0080ParsedFromPlaywrightReport(t *testing.T) {
	record := `{\"assertion\":\"rejection-only\",\"actor\":\"tenant-a\",\"requests\":[{\"method\":\"GET\",\"route\":\"/r\",\"status\":403}]}`
	annotations := `[{"type":"note","description":7},{"type":"corvint-route-stubs","description":"{\"stubs\":[]}"},{"type":"corvint-request-outcomes","description":"` + record + `"}]`
	report := `{"suites":[{"title":"a.spec.ts","file":"a.spec.ts","specs":[{"title":"t","ok":true,"tests":[{"status":"expected","results":[{"status":"passed","retry":0,"duration":1,"annotations":` + annotations + `}]}]}]}],"errors":[]}`
	outcomes, _, err := ParsePlaywrightJSON([]byte(report))
	if err != nil || len(outcomes) != 1 {
		t.Fatalf("outcomes=%+v err=%v", outcomes, err)
	}
	detail := outcomes[0].AttemptDetails[0]
	if detail.RouteStubs == nil || detail.RequestOutcomes == nil || detail.RequestOutcomes.Requests[0].Status != 403 {
		t.Fatalf("detail = %+v", detail)
	}
}

// LPCV-V0-080: the record is unprofiled-only, and a receipt without it keeps every byte.
func TestLPCVV0080ProfiledReceiptsRefuseRequestOutcomes(t *testing.T) {
	o := attemptTestOutcome()
	o.AttemptDetails[1].RequestOutcomes = &RequestOutcomeRecord{Assertion: AssertionPositive, Requests: []RequestOutcome{}}
	for _, profile := range []string{ExternalProfile, AttestedExternalProfile, SensitiveExternalProfile, AttemptExternalProfile} {
		if err := qualifiedProfileShapeError(Receipt{Profile: profile, Tests: []TestOutcome{o}}); err == nil {
			t.Fatalf("%s accepted request outcomes", profile)
		}
	}
	if _, err := EncodeQualified(Receipt{Profile: AttemptExternalProfile, Kind: "e2e", Tests: []TestOutcome{o}}); err == nil || err.Error() != "external-profile-has-request-outcomes" {
		t.Fatalf("/3 request outcomes: %v", err)
	}
	raw, err := json.Marshal(attemptTestOutcome())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "requestOutcomes") {
		t.Fatalf("absent record serialized: %s", raw)
	}
}

// LPCV-V0-080: text that is not valid UTF-8 is refused before decoding can substitute U+FFFD,
// whether it arrives as raw bytes in the report or as a lone-surrogate escape in the description.
func TestLPCVV0080InvalidUTF8IsRefused(t *testing.T) {
	for name, description := range map[string]string{
		"lone surrogate escape": `{"assertion":"positive","actor":"a\ud800","requests":[]}`,
		"raw invalid byte":      "{\"assertion\":\"positive\",\"actor\":\"a\xff\",\"requests\":[]}",
		"replacement character": "{\"assertion\":\"positive\",\"actor\":\"a�\",\"requests\":[]}",
		"route surrogate":       `{"assertion":"positive","requests":[{"method":"GET","route":"/\udc00","status":200}]}`,
	} {
		if got := requestOutcomeRecord([]playwrightAnnotation{requestOutcomeAnnotation(description)}); got != nil {
			t.Errorf("%s: record = %+v", name, got)
		}
	}
	report := "{\"suites\":[{\"title\":\"a.spec.ts\",\"file\":\"a.spec.ts\",\"specs\":[{\"title\":\"t\",\"ok\":true,\"tests\":[{\"status\":\"expected\",\"results\":[{\"status\":\"passed\",\"retry\":0,\"duration\":1,\"annotations\":[{\"type\":\"corvint-request-outcomes\",\"description\":\"{\\\"assertion\\\":\\\"positive\\\",\\\"actor\\\":\\\"a\xff\\\",\\\"requests\\\":[]}\"}]}]}]}]}],\"errors\":[]}"
	outcomes, _, err := ParsePlaywrightJSON([]byte(report))
	if err != nil || len(outcomes) != 1 {
		t.Fatalf("outcomes=%+v err=%v", outcomes, err)
	}
	if got := outcomes[0].AttemptDetails[0].RequestOutcomes; got != nil {
		t.Fatalf("raw invalid byte in report produced %+v", got)
	}
	var record RequestOutcomeRecord
	if err := json.Unmarshal([]byte("{\"assertion\":\"positive\",\"actor\":\"a\xff\",\"requests\":[]}"), &record); err == nil {
		t.Fatalf("retained record with invalid UTF-8 decoded: %+v", record)
	}
}

// LPCV-V0-080 LPCV-V0-082: screening collapses distinct secret-bearing actors to one placeholder,
// so screened text, and text the screen would change, is marked as never joinable.
func TestLPCVV0080ScreeningCollapsesDistinctActors(t *testing.T) {
	first := requestOutcomeRecord([]playwrightAnnotation{requestOutcomeAnnotation(`{"assertion":"positive","actor":"ghp_aaaaaaaaaaaaaaaaaaaaaaaa","requests":[]}`)})
	second := requestOutcomeRecord([]playwrightAnnotation{requestOutcomeAnnotation(`{"assertion":"positive","actor":"ghp_bbbbbbbbbbbbbbbbbbbbbbbb","requests":[]}`)})
	if first == nil || second == nil || first.Actor != second.Actor {
		t.Fatalf("screened actors differ or absent: %+v %+v", first, second)
	}
	for value, want := range map[string]bool{first.Actor: true, "ghp_aaaaaaaaaaaaaaaaaaaaaaaa": true, "/r?key=[REDACTED]": true, "tenant-a": false, "/api/records/:id": false} {
		if got := ScreenedRequestOutcomeText(value); got != want {
			t.Errorf("ScreenedRequestOutcomeText(%q) = %v, want %v", value, got, want)
		}
	}
}
