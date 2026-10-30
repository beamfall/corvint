package jstestprovider

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

const routeStubsLive = "route-stubs/live-1.61.1/report.json"

// LPCV-V0-072: the producer's Node unit suite over a synthetic context and pages.
func TestLPCVV0072ProducerNodeUnit(t *testing.T) {
	out, err := exec.Command("node", "--test", "route-stubs_test.cjs").CombinedOutput()
	if err != nil {
		t.Fatalf("route-stubs producer: %v\n%s", err, out)
	}
}

// LPCV-V0-072: the retained live Playwright 1.61.1 run parses into a hit record, a zero-hit
// record naming both registration lines, and no record for the test that took no browser fixture.
func TestLPCVV0072LiveRecordParses(t *testing.T) {
	outcomes := loadPW(t, routeStubsLive)
	byName := map[string]TestOutcome{}
	for _, o := range outcomes {
		byName[o.Name] = o
	}
	stubs := func(name string) *RouteStubRecord {
		o, ok := byName[name]
		if !ok || o.State != StatePassed || len(o.AttemptDetails) != 1 {
			t.Fatalf("%q: %+v", name, o)
		}
		return o.AttemptDetails[0].RouteStubs
	}
	if got, want := stubs("stub is hit"), (&RouteStubRecord{Stubs: []RouteStub{{Target: "page", Kind: "glob", Pattern: "**/api/items", Hits: 1, File: "stubs.spec.cjs", Line: 5}}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("hit record = %+v", got)
	}
	want := &RouteStubRecord{Stubs: []RouteStub{
		{Target: "page", Kind: "glob", Pattern: "**/api/itmes", Hits: 0, File: "stubs.spec.cjs", Line: 11},
		{Target: "context", Kind: "regexp", Pattern: `/\/api\/v2\//`, Hits: 0, File: "stubs.spec.cjs", Line: 12},
	}}
	if got := stubs("stub never matched but the test passes"); !reflect.DeepEqual(got, want) {
		t.Fatalf("zero-hit record = %+v", got)
	}
	if got := stubs("no browser fixture"); got != nil {
		t.Fatalf("absent record = %+v", got)
	}
}

// LPCV-V0-079: the retained Playwright 1.61.1 run records stubs on browser.newContext() and
// browser.newPage() contexts, routeWebSocket and routeFromHAR, with hits and registration lines.
func TestLPCVV0079LiveResidualsParse(t *testing.T) {
	byName := map[string]TestOutcome{}
	for _, o := range loadPW(t, "route-stubs/live-1.61.1-residuals/report.json") {
		byName[o.Name] = o
	}
	want := map[string]*RouteStubRecord{
		"browser.newContext stubs are recorded": {Stubs: []RouteStub{
			{Target: "context", Kind: "glob", Pattern: "**/api/items", Hits: 1, File: "residuals.spec.cjs", Line: 10},
			{Target: "page", Kind: "glob", Pattern: "**/api/never", Hits: 0, File: "residuals.spec.cjs", Line: 12},
		}},
		"browser.newPage stubs are recorded": {Stubs: []RouteStub{
			{Target: "page", Kind: "glob", Pattern: "**/api/itmes", Hits: 0, File: "residuals.spec.cjs", Line: 20},
		}},
		"routeWebSocket stubs are recorded": {Stubs: []RouteStub{
			{Target: "page", API: "routeWebSocket", Kind: "glob", Pattern: "**/live", Hits: 1, File: "residuals.spec.cjs", Line: 27},
			{Target: "page", API: "routeWebSocket", Kind: "glob", Pattern: "**/never-socket", Hits: 0, File: "residuals.spec.cjs", Line: 28},
		}},
		"routeFromHAR stubs are recorded": {Stubs: []RouteStub{
			{Target: "page", API: "routeFromHAR", Kind: "glob", Pattern: "**/api/items", Hits: 1, File: "residuals.spec.cjs", Line: 34},
			{Target: "context", API: "routeFromHAR", Kind: "glob", Pattern: "**/api/never", Hits: 0, File: "residuals.spec.cjs", Line: 35},
		}},
		"no browser fixture": nil,
	}
	if len(byName) != len(want) {
		t.Fatalf("tests = %d, want %d", len(byName), len(want))
	}
	for name, record := range want {
		o, ok := byName[name]
		if !ok || o.State != StatePassed || len(o.AttemptDetails) != 1 {
			t.Fatalf("%q: %+v", name, o)
		}
		if got := o.AttemptDetails[0].RouteStubs; !reflect.DeepEqual(got, record) {
			t.Errorf("%q record = %+v, want %+v", name, got, record)
		}
	}
}

func routeStubAnnotation(description string) playwrightAnnotation {
	return playwrightAnnotation{Type: RouteStubsAnnotation, Description: description}
}

// LPCV-V0-072: only exactly one closed, in-bounds annotation is a record; anything else is no
// record, never an empty one.
func TestLPCVV0072AnnotationIsClosed(t *testing.T) {
	valid := `{"stubs":[{"target":"page","kind":"glob","pattern":"**/a","hits":0,"file":"a.spec.ts","line":3}]}`
	if got := routeStubRecord([]playwrightAnnotation{{Type: "other"}, routeStubAnnotation(valid)}); got == nil || len(got.Stubs) != 1 || got.Stubs[0].Line != 3 {
		t.Fatalf("valid record = %+v", got)
	}
	if got := routeStubRecord([]playwrightAnnotation{routeStubAnnotation(`{"stubs":[]}`)}); got == nil || got.Stubs == nil || len(got.Stubs) != 0 {
		t.Fatalf("empty record = %+v", got)
	}
	many := make([]string, MaxRouteStubs+1)
	for i := range many {
		many[i] = fmt.Sprintf(`{"target":"context","kind":"glob","pattern":"**/%d","hits":1}`, i)
	}
	for name, annotations := range map[string][]playwrightAnnotation{
		"absent":            nil,
		"other type":        {{Type: "corvint-behavior-event", Description: valid}},
		"duplicate":         {routeStubAnnotation(valid), routeStubAnnotation(valid)},
		"malformed":         {routeStubAnnotation(`{"stubs":[`)},
		"trailing":          {routeStubAnnotation(valid + ` {}`)},
		"unknown field":     {routeStubAnnotation(`{"stubs":[],"extra":1}`)},
		"null stubs":        {routeStubAnnotation(`{"stubs":null}`)},
		"bad target":        {routeStubAnnotation(`{"stubs":[{"target":"frame","kind":"glob","pattern":"x","hits":0}]}`)},
		"bad kind":          {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"url","pattern":"x","hits":0}]}`)},
		"empty pattern":     {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"","hits":0}]}`)},
		"long pattern":      {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"` + strings.Repeat("x", 1025) + `","hits":0}]}`)},
		"negative hits":     {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":-1}]}`)},
		"line no file":      {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":0,"line":4}]}`)},
		"file no line":      {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":0,"file":"a"}]}`)},
		"too many stubs":    {routeStubAnnotation(`{"stubs":[` + strings.Join(many, ",") + `]}`)},
		"oversized":         {routeStubAnnotation(`{"stubs":[]}` + strings.Repeat(" ", maxRouteStubDescription))},
		"fractional hits":   {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":0.5}]}`)},
		"missing hits":      {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x"}]}`)},
		"null hits":         {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":null}]}`)},
		"duplicate hits":    {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":3,"hits":0}]}`)},
		"duplicate stubs":   {routeStubAnnotation(valid[:len(valid)-1] + `,"stubs":[]}`)},
		"non-string":        {{Type: RouteStubsAnnotation, Description: valid, malformed: true}},
		"empty api":         {routeStubAnnotation(`{"stubs":[{"target":"page","api":"","kind":"glob","pattern":"x","hits":0}]}`)},
		"null api":          {routeStubAnnotation(`{"stubs":[{"target":"page","api":null,"kind":"glob","pattern":"x","hits":0}]}`)},
		"unknown api":       {routeStubAnnotation(`{"stubs":[{"target":"page","api":"routeFromHTTP","kind":"glob","pattern":"x","hits":0}]}`)},
		"route api":         {routeStubAnnotation(`{"stubs":[{"target":"page","api":"route","kind":"glob","pattern":"x","hits":0}]}`)},
		"case variant api":  {routeStubAnnotation(`{"stubs":[{"target":"page","api":null,"API":"routeWebSocket","kind":"glob","pattern":"x","hits":0}]}`)},
		"case variant hits": {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":null,"Hits":1}]}`)},
	} {
		if got := routeStubRecord(annotations); got != nil {
			t.Errorf("%s: record = %+v", name, got)
		}
	}
}

// LPCV-V0-076: a stub may name routeWebSocket or routeFromHAR; a plain route omits the member.
func TestLPCVV0076AnnotationNamesAPI(t *testing.T) {
	record := routeStubRecord([]playwrightAnnotation{routeStubAnnotation(`{"stubs":[{"target":"page","api":"routeWebSocket","kind":"glob","pattern":"**/ws","hits":1},{"target":"context","api":"routeFromHAR","kind":"glob","pattern":"**/*","hits":0},{"target":"page","kind":"glob","pattern":"**/a","hits":2}]}`)})
	if record == nil || len(record.Stubs) != 3 {
		t.Fatalf("record = %+v", record)
	}
	for i, want := range []string{"routeWebSocket", "routeFromHAR", ""} {
		if record.Stubs[i].API != want {
			t.Errorf("stub %d api = %q, want %q", i, record.Stubs[i].API, want)
		}
	}
	encoded, err := json.Marshal(record.Stubs[2])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"api"`) {
		t.Fatalf("plain route encodes an api member: %s", encoded)
	}
}

// LPCV-V0-214: a stub the test removed with unroute carries unrouted true; the member has one
// spelling, so false, null, a string or a case variant is refused, and a stub never removed
// encodes without it.
func TestLPCVV0214AnnotationMarksUnrouted(t *testing.T) {
	record := routeStubRecord([]playwrightAnnotation{routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"**/a","hits":0,"unrouted":true},{"target":"page","kind":"glob","pattern":"**/b","hits":1}]}`)})
	if record == nil || len(record.Stubs) != 2 || !record.Stubs[0].Unrouted || record.Stubs[1].Unrouted {
		t.Fatalf("record = %+v", record)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"stubs":[{"target":"page","kind":"glob","pattern":"**/a","hits":0,"unrouted":true},{"target":"page","kind":"glob","pattern":"**/b","hits":1}]}`; string(encoded) != want {
		t.Fatalf("encoded = %s, want %s", encoded, want)
	}
	for name, member := range map[string]string{
		"false":        `"unrouted":false`,
		"null":         `"unrouted":null`,
		"string":       `"unrouted":"true"`,
		"number":       `"unrouted":1`,
		"case variant": `"Unrouted":true`,
		"duplicate":    `"unrouted":true,"unrouted":true`,
	} {
		if got := routeStubRecord([]playwrightAnnotation{routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":0,` + member + `}]}`)}); got != nil {
			t.Errorf("%s: record = %+v", name, got)
		}
	}
}

// LPCV-V0-072: patterns and registration paths pass the product secret screen, including a
// credential written with regular-expression escapes.
func TestLPCVV0072RecordIsSecretScreened(t *testing.T) {
	token := "ghp_abcdefghijklmnopqrst"
	got := routeStubRecord([]playwrightAnnotation{routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"**/api?key=` + token + `","hits":0,"file":"` + token + `.spec.ts","line":1}]}`)})
	if got == nil || strings.Contains(got.Stubs[0].Pattern, token) || strings.Contains(got.Stubs[0].File, token) {
		t.Fatalf("record not screened: %+v", got)
	}
	regexp := routeStubRecord([]playwrightAnnotation{routeStubAnnotation(`{"stubs":[{"target":"page","kind":"regexp","pattern":"/https:\\/\\/alice:hunter2@example\\.test\\/api/","hits":0}]}`)})
	if regexp == nil || strings.Contains(regexp.Stubs[0].Pattern, "hunter2") {
		t.Fatalf("escaped credential not screened: %+v", regexp)
	}
	plain := routeStubRecord([]playwrightAnnotation{routeStubAnnotation(`{"stubs":[{"target":"page","kind":"regexp","pattern":"/\\/api\\/v2\\//","hits":0}]}`)})
	if plain == nil || plain.Stubs[0].Pattern != `/\/api\/v2\//` {
		t.Fatalf("clean regexp rewritten: %+v", plain)
	}
}

// LPCV-V0-072: an annotation of an unexpected shape never fails the report; only the route-stub
// record it would carry is absent.
func TestLPCVV0072MalformedAnnotationKeepsReport(t *testing.T) {
	const annotations = `[{"type":"note","description":7},{"type":"corvint-route-stubs","description":"{\"stubs\":[]}"}]`
	report := `{"suites":[{"title":"a.spec.ts","file":"a.spec.ts","specs":[{"title":"t","ok":true,"tests":[{"status":"expected","results":[{"status":"passed","retry":0,"duration":1,"annotations":` + annotations + `}]}]}]}],"errors":[]}`
	parse := func(name, report string) *RouteStubRecord {
		outcomes, _, err := ParsePlaywrightJSON([]byte(report))
		if err != nil || len(outcomes) != 1 || outcomes[0].State != StatePassed {
			t.Fatalf("%s: outcomes=%+v err=%v", name, outcomes, err)
		}
		return outcomes[0].AttemptDetails[0].RouteStubs
	}
	if got := parse("unrelated malformed", report); got == nil || got.Stubs == nil {
		t.Fatalf("an unrelated malformed annotation dropped the record: %+v", got)
	}
	if got := parse("object description", strings.Replace(report, `"description":"{\"stubs\":[]}"`, `"description":{"stubs":[]}`, 1)); got != nil {
		t.Fatalf("object description read as a record: %+v", got)
	}
	if got := parse("non-array", strings.Replace(report, annotations, `5`, 1)); got != nil {
		t.Fatalf("non-array annotations: %+v", got)
	}
}

// LPCV-V0-072: the record is unprofiled-only; an external profile gains it only through another
// profile revision, and an unprofiled receipt without the producer keeps every byte.
func TestLPCVV0072ProfiledReceiptsRefuseRouteStubs(t *testing.T) {
	o := attemptTestOutcome()
	o.AttemptDetails[1].RouteStubs = &RouteStubRecord{Stubs: []RouteStub{}}
	for _, profile := range []string{ExternalProfile, AttestedExternalProfile, SensitiveExternalProfile, AttemptExternalProfile} {
		if err := qualifiedProfileShapeError(Receipt{Profile: profile, Tests: []TestOutcome{o}}); err == nil {
			t.Fatalf("%s accepted route stubs", profile)
		}
	}
	if _, err := EncodeQualified(Receipt{Profile: AttemptExternalProfile, Kind: "e2e", Tests: []TestOutcome{o}}); err == nil || err.Error() != "external-profile-has-route-stubs" {
		t.Fatalf("/3 route stubs: %v", err)
	}
	plain := attemptTestOutcome()
	raw, err := json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "routeStubs") {
		t.Fatalf("absent record serialized: %s", raw)
	}
}
