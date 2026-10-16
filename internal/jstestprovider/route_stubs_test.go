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
		"absent":          nil,
		"other type":      {{Type: "corvint-behavior-event", Description: valid}},
		"duplicate":       {routeStubAnnotation(valid), routeStubAnnotation(valid)},
		"malformed":       {routeStubAnnotation(`{"stubs":[`)},
		"trailing":        {routeStubAnnotation(valid + ` {}`)},
		"unknown field":   {routeStubAnnotation(`{"stubs":[],"extra":1}`)},
		"null stubs":      {routeStubAnnotation(`{"stubs":null}`)},
		"bad target":      {routeStubAnnotation(`{"stubs":[{"target":"frame","kind":"glob","pattern":"x","hits":0}]}`)},
		"bad kind":        {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"url","pattern":"x","hits":0}]}`)},
		"empty pattern":   {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"","hits":0}]}`)},
		"long pattern":    {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"` + strings.Repeat("x", 1025) + `","hits":0}]}`)},
		"negative hits":   {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":-1}]}`)},
		"line no file":    {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":0,"line":4}]}`)},
		"file no line":    {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":0,"file":"a"}]}`)},
		"too many stubs":  {routeStubAnnotation(`{"stubs":[` + strings.Join(many, ",") + `]}`)},
		"oversized":       {routeStubAnnotation(`{"stubs":[]}` + strings.Repeat(" ", maxRouteStubDescription))},
		"fractional hits": {routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"x","hits":0.5}]}`)},
	} {
		if got := routeStubRecord(annotations); got != nil {
			t.Errorf("%s: record = %+v", name, got)
		}
	}
}

// LPCV-V0-072: patterns and registration paths pass the product secret screen.
func TestLPCVV0072RecordIsSecretScreened(t *testing.T) {
	token := "ghp_abcdefghijklmnopqrst"
	got := routeStubRecord([]playwrightAnnotation{routeStubAnnotation(`{"stubs":[{"target":"page","kind":"glob","pattern":"**/api?key=` + token + `","hits":0,"file":"` + token + `.spec.ts","line":1}]}`)})
	if got == nil || strings.Contains(got.Stubs[0].Pattern, token) || strings.Contains(got.Stubs[0].File, token) {
		t.Fatalf("record not screened: %+v", got)
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
