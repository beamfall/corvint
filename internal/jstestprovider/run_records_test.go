package jstestprovider

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func annotation(annotationType, description string) playwrightAnnotation {
	return playwrightAnnotation{Type: annotationType, Description: description}
}

// LPCV-V0-120: only exactly one closed, in-bounds annotation is an assertion-operand record; a
// missing or null size is never read as empty.
func TestLPCVV0120AnnotationIsClosed(t *testing.T) {
	valid := `{"assertions":[{"matcher":"toHaveCount","file":"tests/rows.spec.ts","line":12,"actual":0,"expected":0}]}`
	got := assertionOperandRecord([]playwrightAnnotation{{Type: "other"}, annotation(AssertionOperandsAnnotation, valid)})
	if got == nil || len(got.Assertions) != 1 || got.Assertions[0].Matcher != "toHaveCount" || got.Assertions[0].Line != 12 || got.Assertions[0].Actual != 0 {
		t.Fatalf("valid record = %+v", got)
	}
	if got := assertionOperandRecord([]playwrightAnnotation{annotation(AssertionOperandsAnnotation, `{"assertions":[{"matcher":"toBe","actual":3,"expected":3}]}`)}); got == nil || got.Assertions[0].File != "" {
		t.Fatalf("unlocated record = %+v", got)
	}
	many := make([]string, MaxAssertionOperands+1)
	for i := range many {
		many[i] = `{"matcher":"toBe","actual":1,"expected":1}`
	}
	one := func(member string) []playwrightAnnotation {
		return []playwrightAnnotation{annotation(AssertionOperandsAnnotation, `{"assertions":[{`+member+`}]}`)}
	}
	for name, annotations := range map[string][]playwrightAnnotation{
		"absent":            nil,
		"other type":        {annotation(RequestOutcomesAnnotation, valid)},
		"duplicate":         {annotation(AssertionOperandsAnnotation, valid), annotation(AssertionOperandsAnnotation, valid)},
		"malformed":         {annotation(AssertionOperandsAnnotation, `{"assertions":`)},
		"trailing":          {annotation(AssertionOperandsAnnotation, valid+` {}`)},
		"null assertions":   {annotation(AssertionOperandsAnnotation, `{"assertions":null}`)},
		"unknown member":    one(`"matcher":"toBe","actual":0,"expected":0,"value":"x"`),
		"case variant":      one(`"matcher":"toBe","actual":null,"Actual":0,"expected":0`),
		"duplicate size":    one(`"matcher":"toBe","actual":1,"actual":0,"expected":0`),
		"missing actual":    one(`"matcher":"toBe","expected":0`),
		"null expected":     one(`"matcher":"toBe","actual":0,"expected":null`),
		"negative size":     one(`"matcher":"toBe","actual":-1,"expected":0`),
		"fractional size":   one(`"matcher":"toBe","actual":0.5,"expected":0`),
		"unknown matcher":   one(`"matcher":"toContain","actual":0,"expected":0`),
		"line without file": one(`"matcher":"toBe","line":3,"actual":0,"expected":0`),
		"file without line": one(`"matcher":"toBe","file":"a.ts","actual":0,"expected":0`),
		"too many":          {annotation(AssertionOperandsAnnotation, `{"assertions":[`+strings.Join(many, ",")+`]}`)},
		"oversized":         {annotation(AssertionOperandsAnnotation, `{"assertions":[]}`+strings.Repeat(" ", maxAssertionOperandDescription))},
		"non-string":        {{Type: AssertionOperandsAnnotation, Description: valid, malformed: true}},
		"surrogate":         one(`"matcher":"toBe","file":"a\ud800.ts","line":1,"actual":0,"expected":0`),
	} {
		if got := assertionOperandRecord(annotations); got != nil {
			t.Errorf("%s: record = %+v", name, got)
		}
	}
}

// LPCV-V0-123: only exactly one closed, in-bounds annotation is an outbound-call record; a missing
// status or stubbed flag is never read as a server error or an unstubbed call, and a query string
// is never retained.
func TestLPCVV0123AnnotationIsClosed(t *testing.T) {
	valid := `{"responses":[{"method":"POST","route":"/api/orders","status":500,"calls":[{"method":"GET","host":"lookup.example","path":"/v1/customers","stubbed":false}]}]}`
	got := outboundCallRecord([]playwrightAnnotation{annotation(OutboundCallsAnnotation, valid)})
	if got == nil || len(got.Responses) != 1 || got.Responses[0].Status != 500 || got.Responses[0].Calls[0].Host != "lookup.example" || got.Responses[0].Calls[0].Stubbed {
		t.Fatalf("valid record = %+v", got)
	}
	if got := outboundCallRecord([]playwrightAnnotation{annotation(OutboundCallsAnnotation, `{"responses":[]}`)}); got == nil || got.Responses == nil {
		t.Fatalf("empty record = %+v", got)
	}
	many := make([]string, MaxOutboundResponses+1)
	for i := range many {
		many[i] = fmt.Sprintf(`{"method":"GET","route":"/r/%d","status":200,"calls":[]}`, i)
	}
	call := func(member string) []playwrightAnnotation {
		return []playwrightAnnotation{annotation(OutboundCallsAnnotation, `{"responses":[{"method":"POST","route":"/api/orders","status":500,"calls":[{`+member+`}]}]}`)}
	}
	response := func(member string) []playwrightAnnotation {
		return []playwrightAnnotation{annotation(OutboundCallsAnnotation, `{"responses":[{`+member+`}]}`)}
	}
	for name, annotations := range map[string][]playwrightAnnotation{
		"absent":            nil,
		"other type":        {annotation(AssertionOperandsAnnotation, valid)},
		"duplicate":         {annotation(OutboundCallsAnnotation, valid), annotation(OutboundCallsAnnotation, valid)},
		"malformed":         {annotation(OutboundCallsAnnotation, `{"responses":`)},
		"trailing":          {annotation(OutboundCallsAnnotation, valid+` {}`)},
		"null responses":    {annotation(OutboundCallsAnnotation, `{"responses":null}`)},
		"missing status":    response(`"method":"GET","route":"/a","calls":[]`),
		"null status":       response(`"method":"GET","route":"/a","status":null,"calls":[]`),
		"status range":      response(`"method":"GET","route":"/a","status":600,"calls":[]`),
		"null calls":        response(`"method":"GET","route":"/a","status":500,"calls":null`),
		"missing calls":     response(`"method":"GET","route":"/a","status":500`),
		"unknown member":    response(`"method":"GET","route":"/a","status":500,"calls":[],"log":"x"`),
		"missing stubbed":   call(`"method":"GET","host":"h.example","path":"/p"`),
		"null stubbed":      call(`"method":"GET","host":"h.example","path":"/p","stubbed":null`),
		"case variant":      call(`"method":"GET","host":"h.example","path":"/p","stubbed":null,"Stubbed":false`),
		"duplicate stubbed": call(`"method":"GET","host":"h.example","path":"/p","stubbed":true,"stubbed":false`),
		"lowercase method":  call(`"method":"get","host":"h.example","path":"/p","stubbed":false`),
		"query in path":     call(`"method":"GET","host":"h.example","path":"/p?name=Acme","stubbed":false`),
		"fragment in path":  call(`"method":"GET","host":"h.example","path":"/p#x","stubbed":false`),
		"relative path":     call(`"method":"GET","host":"h.example","path":"p","stubbed":false`),
		"url as host":       call(`"method":"GET","host":"https://h.example/p","path":"/p","stubbed":false`),
		"userinfo in host":  call(`"method":"GET","host":"user@h.example","path":"/p","stubbed":false`),
		"empty host":        call(`"method":"GET","host":"","path":"/p","stubbed":false`),
		"too many":          {annotation(OutboundCallsAnnotation, `{"responses":[`+strings.Join(many, ",")+`]}`)},
		"oversized":         {annotation(OutboundCallsAnnotation, `{"responses":[]}`+strings.Repeat(" ", maxOutboundCallDescription))},
		"non-string":        {{Type: OutboundCallsAnnotation, Description: valid, malformed: true}},
	} {
		if got := outboundCallRecord(annotations); got != nil {
			t.Errorf("%s: record = %+v", name, got)
		}
	}
}

// LPCV-V0-120 LPCV-V0-123: assertion paths, routes, hosts and outbound paths pass the product secret
// screen.
func TestLPCVV0120RunRecordsAreSecretScreened(t *testing.T) {
	token := "ghp_abcdefghijklmnopqrst"
	operands := assertionOperandRecord([]playwrightAnnotation{annotation(AssertionOperandsAnnotation, `{"assertions":[{"matcher":"toBe","file":"tests/`+token+`.ts","line":1,"actual":0,"expected":0}]}`)})
	if operands == nil || strings.Contains(operands.Assertions[0].File, token) {
		t.Fatalf("operand record not screened: %+v", operands)
	}
	calls := outboundCallRecord([]playwrightAnnotation{annotation(OutboundCallsAnnotation, `{"responses":[{"method":"GET","route":"/api/`+token+`","status":500,"calls":[{"method":"GET","host":"`+token+`.example","path":"/v1/`+token+`","stubbed":false}]}]}`)})
	if calls == nil {
		t.Fatal("outbound record absent")
	}
	if encoded, _ := json.Marshal(calls); strings.Contains(string(encoded), token) {
		t.Fatalf("outbound record not screened: %s", encoded)
	}
}

// LPCV-V0-120 LPCV-V0-123: ParsePlaywrightJSON reads both records beside the other declared records,
// and raw invalid UTF-8 in either description is refused rather than replaced.
func TestLPCVV0120ParsedFromPlaywrightReport(t *testing.T) {
	operands := `{\"assertions\":[{\"matcher\":\"toHaveCount\",\"actual\":0,\"expected\":0}]}`
	calls := `{\"responses\":[{\"method\":\"GET\",\"route\":\"/r\",\"status\":502,\"calls\":[]}]}`
	annotations := `[{"type":"note","description":7},{"type":"corvint-assertion-operands","description":"` + operands + `"},{"type":"corvint-outbound-calls","description":"` + calls + `"}]`
	report := `{"suites":[{"title":"a.spec.ts","file":"a.spec.ts","specs":[{"title":"t","ok":true,"tests":[{"status":"expected","results":[{"status":"passed","retry":0,"duration":1,"annotations":` + annotations + `}]}]}]}],"errors":[]}`
	outcomes, _, err := ParsePlaywrightJSON([]byte(report))
	if err != nil || len(outcomes) != 1 {
		t.Fatalf("outcomes=%+v err=%v", outcomes, err)
	}
	detail := outcomes[0].AttemptDetails[0]
	if detail.AssertionOperands == nil || detail.OutboundCalls == nil || detail.OutboundCalls.Responses[0].Status != 502 {
		t.Fatalf("detail = %+v", detail)
	}
	invalid := strings.Replace(report, `\"route\":\"/r\"`, "\\\"route\\\":\\\"/r\xff\\\"", 1)
	outcomes, _, err = ParsePlaywrightJSON([]byte(invalid))
	if err != nil || len(outcomes) != 1 || outcomes[0].AttemptDetails[0].OutboundCalls != nil {
		t.Fatalf("raw invalid byte accepted: %+v err=%v", outcomes, err)
	}
	var record OutboundCallRecord
	if err := json.Unmarshal([]byte("{\"responses\":[{\"method\":\"GET\",\"route\":\"/r\xff\",\"status\":500,\"calls\":[]}]}"), &record); err == nil {
		t.Fatalf("retained record with invalid UTF-8 decoded: %+v", record)
	}
}

// LPCV-V0-120 LPCV-V0-123: both records are unprofiled-only, and a receipt without them keeps every
// byte.
func TestLPCVV0120ProfiledReceiptsRefuseRunRecords(t *testing.T) {
	withOperands := attemptTestOutcome()
	withOperands.AttemptDetails[1].AssertionOperands = &AssertionOperandRecord{Assertions: []AssertionOperands{}}
	withCalls := attemptTestOutcome()
	withCalls.AttemptDetails[1].OutboundCalls = &OutboundCallRecord{Responses: []OutboundResponse{}}
	for code, outcome := range map[string]TestOutcome{"external-profile-has-assertion-operands": withOperands, "external-profile-has-outbound-calls": withCalls} {
		for _, profile := range []string{ExternalProfile, AttestedExternalProfile, SensitiveExternalProfile, AttemptExternalProfile} {
			if err := qualifiedProfileShapeError(Receipt{Profile: profile, Tests: []TestOutcome{outcome}}); err == nil {
				t.Fatalf("%s accepted %s", profile, code)
			}
		}
		if _, err := EncodeQualified(Receipt{Profile: AttemptExternalProfile, Kind: "e2e", Tests: []TestOutcome{outcome}}); err == nil || err.Error() != code {
			t.Fatalf("/3 %s: %v", code, err)
		}
	}
	raw, err := json.Marshal(attemptTestOutcome())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "assertionOperands") || strings.Contains(string(raw), "outboundCalls") {
		t.Fatalf("absent record serialized: %s", raw)
	}
}
