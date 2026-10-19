package testvaliditydoc

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// recordTestJSON is one receipt test whose single attempt carries the given attempt-detail member
// (empty record for none).
func recordTestJSON(name, state, member, record string) string {
	attempt := `"state":"` + state + `","retry":0,"durationMs":1`
	if record != "" {
		attempt += `,"` + member + `":` + record
	}
	return `{"name":"` + name + `","fullName":"` + name + `","state":"` + state + `","attemptDetails":[{` + attempt + `}]}`
}

func operandsTest(name, state, record string) string {
	return recordTestJSON(name, state, "assertionOperands", record)
}

func callsTest(name, state, record string) string {
	return recordTestJSON(name, state, "outboundCalls", record)
}

const (
	emptyCountRecord   = `{"assertions":[{"matcher":"toHaveCount","file":"tests/rows.spec.ts","line":14,"actual":0,"expected":0},{"matcher":"toBe","actual":2,"expected":2}]}`
	seededCountRecord  = `{"assertions":[{"matcher":"toHaveCount","file":"tests/rows.spec.ts","line":14,"actual":3,"expected":3}]}`
	oneSideEmptyRecord = `{"assertions":[{"matcher":"toEqual","actual":0,"expected":4}]}`
	unstubbedLookup    = `{"responses":[{"method":"GET","route":"/api/health","status":200,"calls":[]},{"method":"POST","route":"/api/customers","status":500,"calls":[{"method":"GET","host":"crm.example","path":"/v1/lookup","stubbed":true},{"method":"GET","host":"geo.example","path":"/v2/resolve","stubbed":false}]}]}`
	stubbedLookup      = `{"responses":[{"method":"POST","route":"/api/customers","status":500,"calls":[{"method":"GET","host":"crm.example","path":"/v1/lookup","stubbed":true}]}]}`
	unstubbedOn4xx     = `{"responses":[{"method":"POST","route":"/api/customers","status":422,"calls":[{"method":"GET","host":"geo.example","path":"/v2/resolve","stubbed":false}]}]}`
)

// LPCV-V0-121 LPCV-V0-122: GitHub #1193 run-time reading. A passed test whose count assertion saw
// zero on both sides gets VACUOUS-EMPTY-EQUALITY and blocks credit; a seeded run, a one-sided empty
// and a failed test do not; a test without a usable record states why.
func TestLPCVV0121VacuousEqualityReading(t *testing.T) {
	document := projectDocument(t, `{"receipt":{"kind":"e2e","tests":[`+strings.Join([]string{
		operandsTest("rows match served list", "passed", emptyCountRecord),
		operandsTest("seeded rows match", "passed", seededCountRecord),
		operandsTest("one side empty", "passed", oneSideEmptyRecord),
		operandsTest("failed with empty operands", "failed", emptyCountRecord),
		operandsTest("no record", "passed", ""),
		operandsTest("invalid record", "passed", `{"assertions":[{"matcher":"toHaveCount","line":3,"actual":0,"expected":0}]}`),
		operandsTest("flaky with empty operands", "flaky", emptyCountRecord),
	}, ",")+`]},"testProjections":[],"runProjection":{}}`)
	finding := VacuousEqualityFinding{Code: VacuousEmptyEquality, Matcher: "toHaveCount", File: "tests/rows.spec.ts", Line: 14}
	want := []VacuousEquality{
		{State: VacuousRecorded, BlocksCredit: true, Findings: []VacuousEqualityFinding{finding}},
		{State: VacuousRecorded},
		{State: VacuousRecorded},
		{State: VacuousNotApplicable, Reason: VacuousTestNotPassed},
		{State: VacuousNotProduced, Reason: VacuousRecordAbsent},
		{State: VacuousNotProduced, Reason: VacuousRecordInvalid},
		{State: VacuousRecorded, BlocksCredit: true, Findings: []VacuousEqualityFinding{finding}},
	}
	for i, test := range document.Tests {
		if test.VacuousEquality == nil || !reflect.DeepEqual(*test.VacuousEquality, want[i]) {
			t.Errorf("test %q vacuousEquality = %+v, want %+v", test.Name, test.VacuousEquality, want[i])
		}
	}
}

// LPCV-V0-124 LPCV-V0-125: GitHub #1192 run-time classification. A failed test whose record ties a
// 5xx to an unstubbed outbound call is ENV-UNSTUBBED-CALL and names the call; a 5xx whose calls were
// all stubbed, an unstubbed call under a 4xx, a passed test and a missing record are not.
func TestLPCVV0124UnstubbedCallClassification(t *testing.T) {
	document := projectDocument(t, `{"receipt":{"kind":"e2e","tests":[`+strings.Join([]string{
		callsTest("customer lookup", "failed", unstubbedLookup),
		callsTest("stubbed lookup", "failed", stubbedLookup),
		callsTest("validation error", "failed", unstubbedOn4xx),
		callsTest("passed lookup", "passed", unstubbedLookup),
		callsTest("timed out lookup", "timedOut", unstubbedLookup),
		callsTest("no record", "failed", ""),
		callsTest("invalid record", "failed", `{"responses":[{"method":"GET","route":"/a","status":500,"calls":[{"method":"GET","host":"h","path":"/p?name=Acme","stubbed":false}]}]}`),
	}, ",")+`]},"testProjections":[],"runProjection":{}}`)
	classified := UnstubbedCall{State: UnstubbedClassified, Reason: UnstubbedCallOnServerErr, Code: EnvUnstubbedCall, Calls: []UnstubbedCallEntry{{Method: "GET", Host: "geo.example", Path: "/v2/resolve", ResponseMethod: "POST", ResponseRoute: "/api/customers", ResponseStatus: 500}}}
	want := []UnstubbedCall{
		classified,
		{State: UnstubbedNotClassified, Reason: UnstubbedAllCallsStubbed},
		{State: UnstubbedNotClassified, Reason: UnstubbedNoServerError},
		{State: UnstubbedNotApplicable, Reason: UnstubbedTestNotFailed},
		classified,
		{State: UnstubbedNotProduced, Reason: UnstubbedRecordAbsent},
		{State: UnstubbedNotProduced, Reason: UnstubbedRecordInvalid},
	}
	for i, test := range document.Tests {
		if test.UnstubbedCall == nil || !reflect.DeepEqual(*test.UnstubbedCall, want[i]) {
			t.Errorf("test %q unstubbedCall = %+v, want %+v", test.Name, test.UnstubbedCall, want[i])
		}
	}
}

// LPCV-V0-121 LPCV-V0-124: a receipt without either record keeps every document byte, and the members
// change no other member.
func TestLPCVV0121ReceiptWithoutRecordsKeepsBytes(t *testing.T) {
	legacy := projectDocument(t, `{"receipt":{"kind":"e2e","tests":[`+operandsTest("a", "failed", "")+`]},"testProjections":[],"runProjection":{}}`)
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "vacuousEquality") || strings.Contains(string(encoded), "unstubbedCall") {
		t.Fatalf("legacy document gained a run-record member: %s", encoded)
	}
	both := `{"name":"a","fullName":"a","state":"failed","attemptDetails":[{"state":"failed","retry":0,"durationMs":1,"assertionOperands":` + emptyCountRecord + `,"outboundCalls":` + unstubbedLookup + `}]}`
	withRecords := projectDocument(t, `{"receipt":{"kind":"e2e","tests":[`+both+`]},"testProjections":[],"runProjection":{}}`)
	if withRecords.Tests[0].VacuousEquality == nil || withRecords.Tests[0].UnstubbedCall == nil {
		t.Fatalf("members absent: %+v", withRecords.Tests[0])
	}
	withRecords.Tests[0].VacuousEquality = nil
	withRecords.Tests[0].UnstubbedCall = nil
	again, err := json.Marshal(withRecords)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatalf("run-record members changed another member:\n%s\n%s", encoded, again)
	}
}

// LPCV-V0-122 LPCV-V0-125: neither reading rewrites the five-axis projection of the test or the run.
func TestLPCVV0122ReadingsLeaveAxesUnchanged(t *testing.T) {
	body := func(passed, failed string) string {
		return `{"receipt":{"kind":"e2e","tests":[` + operandsTest("rows", "passed", passed) + `,` + callsTest("lookup", "failed", failed) + `]},"testProjections":[],"runProjection":{}}`
	}
	plain := projectDocument(t, body("", ""))
	read := projectDocument(t, body(emptyCountRecord, unstubbedLookup))
	if !read.Tests[0].VacuousEquality.BlocksCredit || read.Tests[1].UnstubbedCall.Code != EnvUnstubbedCall {
		t.Fatalf("readings absent: %+v %+v", read.Tests[0].VacuousEquality, read.Tests[1].UnstubbedCall)
	}
	for i := range plain.Tests {
		if !reflect.DeepEqual(plain.Tests[i].Projection, read.Tests[i].Projection) {
			t.Fatalf("test %d projection rewritten:\n%+v\n%+v", i, plain.Tests[i].Projection, read.Tests[i].Projection)
		}
	}
	if !reflect.DeepEqual(plain.Run, read.Run) {
		t.Fatalf("run projection rewritten:\n%+v\n%+v", plain.Run, read.Run)
	}
}

// LPCV-V0-120 LPCV-V0-123: a retained receipt's records decode closed, so a missing size or stubbed
// flag is refused rather than read as empty or unstubbed.
func TestLPCVV0120RetainedRecordsAreClosed(t *testing.T) {
	for name, test := range map[string]string{
		"missing actual":   operandsTest("a", "passed", `{"assertions":[{"matcher":"toBe","expected":0}]}`),
		"unknown operand":  operandsTest("a", "passed", `{"assertions":[{"matcher":"toBe","actual":0,"expected":0,"value":"x"}]}`),
		"missing stubbed":  callsTest("a", "failed", `{"responses":[{"method":"GET","route":"/a","status":500,"calls":[{"method":"GET","host":"h","path":"/p"}]}]}`),
		"duplicate status": callsTest("a", "failed", `{"responses":[{"method":"GET","route":"/a","status":200,"status":500,"calls":[]}]}`),
	} {
		body := `{"receipt":{"kind":"e2e","tests":[` + test + `]},"testProjections":[],"runProjection":{}}`
		if _, err := Decode([]byte(body)); err == nil {
			t.Errorf("%s: retained record accepted", name)
		}
	}
}
