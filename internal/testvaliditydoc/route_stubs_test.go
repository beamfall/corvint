package testvaliditydoc

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

type routeStubVectors struct {
	Cases []struct {
		Name                string            `json:"name"`
		RouteStubsRequested bool              `json:"routeStubsRequested"`
		Receipt             json.RawMessage   `json:"receipt"`
		Expected            []json.RawMessage `json:"expected"`
	} `json:"cases"`
}

func routeStubDocument(t *testing.T, receipt []byte) Document {
	t.Helper()
	return projectDocument(t, `{"receipt":`+string(receipt)+`,"testProjections":[],"runProjection":{}}`)
}

func requestedRouteStubDocument(t *testing.T, receipt []byte) Document {
	t.Helper()
	input, err := Decode([]byte(`{"receipt":` + string(receipt) + `,"testProjections":[],"runProjection":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	return ProjectRouteStubs(input)
}

// LPCV-V0-073 LPCV-V0-074 LPCV-V0-076 LPCV-V0-077: every shared vector's routeStubs members, test
// by test, including the required hit, zero-hit, record-absent, websocket-and-har and
// producer-absent-requested cases.
func TestLPCVV0074RouteStubVectors(t *testing.T) {
	raw, err := os.ReadFile("../../conformance/test-validity-v0/route-stubs.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors routeStubVectors
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, vector := range vectors.Cases {
		seen[vector.Name] = true
		t.Run(vector.Name, func(t *testing.T) {
			document := routeStubDocument(t, vector.Receipt)
			if vector.RouteStubsRequested {
				document = requestedRouteStubDocument(t, vector.Receipt)
			}
			if len(document.Tests) != len(vector.Expected) {
				t.Fatalf("tests=%d expected=%d", len(document.Tests), len(vector.Expected))
			}
			for i, test := range document.Tests {
				got, err := json.Marshal(test.RouteStubs)
				if err != nil {
					t.Fatal(err)
				}
				var want bytes.Buffer
				if err := json.Compact(&want, vector.Expected[i]); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want.Bytes()) {
					t.Errorf("test %d routeStubs = %s, want %s", i, got, want.Bytes())
				}
			}
		})
	}
	for _, name := range []string{"hit", "zero-hit", "record-absent", "websocket-and-har", "producer-absent-requested", "unrouted-before-hit"} {
		if !seen[name] {
			t.Errorf("route-stubs.json is missing the required case %q", name)
		}
	}
}

// LPCV-V0-073: a receipt without any producer record keeps every document byte; the routeStubs
// member appears only once some attempt carries a record.
func TestLPCVV0073ReceiptWithoutProducerKeepsBytes(t *testing.T) {
	legacy := `{"kind":"e2e","tests":[{"name":"a","state":"passed","attemptDetails":[{"state":"passed","retry":0,"durationMs":1}]}]}`
	encoded, err := json.Marshal(routeStubDocument(t, []byte(legacy)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "routeStubs") {
		t.Fatalf("legacy document gained routeStubs: %s", encoded)
	}
	recorded := strings.Replace(legacy, `"durationMs":1}`, `"durationMs":1,"routeStubs":{"stubs":[]}}`, 1)
	withRecord := routeStubDocument(t, []byte(recorded))
	withRecord.Tests[0].RouteStubs = nil
	again, err := json.Marshal(withRecord)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatalf("routeStubs changed another member:\n%s\n%s", encoded, again)
	}
}

// LPCV-V0-073: a finding never rewrites the five-axis projection of the test or the run.
func TestLPCVV0073FindingLeavesAxesUnchanged(t *testing.T) {
	base := `{"kind":"e2e","tests":[{"name":"a","state":"passed","attemptDetails":[{"state":"passed","retry":0,"durationMs":1}]}]}`
	zero := strings.Replace(base, `"durationMs":1}`, `"durationMs":1,"routeStubs":{"stubs":[{"target":"page","kind":"glob","pattern":"**/x","hits":0}]}}`, 1)
	plain, stubbed := routeStubDocument(t, []byte(base)), routeStubDocument(t, []byte(zero))
	if stubbed.Tests[0].RouteStubs == nil || len(stubbed.Tests[0].RouteStubs.Findings) != 1 {
		t.Fatalf("no finding: %+v", stubbed.Tests[0].RouteStubs)
	}
	if !reflect.DeepEqual(plain.Tests[0].Projection, stubbed.Tests[0].Projection) || !reflect.DeepEqual(plain.Run, stubbed.Run) {
		t.Fatalf("finding rewrote the projection:\n%+v\n%+v", plain.Tests[0].Projection, stubbed.Tests[0].Projection)
	}
}

// LPCV-V0-073: a retained receipt whose record omits a hit count, sets it null or repeats a member
// is refused, never read as zero hits.
func TestLPCVV0073RetainedRecordIsClosed(t *testing.T) {
	for name, stub := range map[string]string{
		"missing hits":   `{"target":"page","kind":"glob","pattern":"x"}`,
		"null hits":      `{"target":"page","kind":"glob","pattern":"x","hits":null}`,
		"duplicate hits": `{"target":"page","kind":"glob","pattern":"x","hits":3,"hits":0}`,
	} {
		body := `{"receipt":{"kind":"e2e","tests":[{"name":"a","state":"passed","attemptDetails":[{"state":"passed","retry":0,"durationMs":1,"routeStubs":{"stubs":[` + stub + `]}}]}]},"testProjections":[],"runProjection":{}}`
		if _, err := Decode([]byte(body)); err == nil {
			t.Errorf("%s: retained record accepted", name)
		}
	}
}

// LPCV-V0-077: requesting the member states the producer-absent case explicitly, while the default
// projection of the same receipt keeps every legacy byte and the request changes no other member.
func TestLPCVV0077RequestedMemberKeepsDefaultBytes(t *testing.T) {
	legacy := []byte(`{"kind":"e2e","tests":[{"name":"a","state":"passed","attemptDetails":[{"state":"passed","retry":0,"durationMs":1}]}]}`)
	plain, err := json.Marshal(routeStubDocument(t, legacy))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "routeStubs") {
		t.Fatalf("default projection gained routeStubs: %s", plain)
	}
	requested := requestedRouteStubDocument(t, legacy)
	if got := requested.Tests[0].RouteStubs; got == nil || got.State != RouteStubsNotProduced || got.Reason != RouteStubProducerAbsent {
		t.Fatalf("requested member = %+v", got)
	}
	requested.Tests[0].RouteStubs = nil
	again, err := json.Marshal(requested)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plain, again) {
		t.Fatalf("request changed another member:\n%s\n%s", plain, again)
	}
}

// LPCV-V0-077 LPCV-V0-078: a profiled receipt never carries the record, so a request states it as
// NOT_PRODUCED with route-stub-external-profile; the default projection still omits the member.
func TestLPCVV0078ProfiledReceiptRequestedIsExternalProfile(t *testing.T) {
	receipt := jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", Tests: []jstestprovider.TestOutcome{{Name: "a", State: jstestprovider.StatePassed}}}
	if got := Project(Input{js: &receipt}).Tests[0].RouteStubs; got != nil {
		t.Fatalf("default projection of a profiled receipt carries routeStubs: %+v", got)
	}
	got := ProjectRouteStubs(Input{js: &receipt}).Tests[0].RouteStubs
	if got == nil || got.State != RouteStubsNotProduced || got.Reason != RouteStubExternalProfile {
		t.Fatalf("requested member = %+v", got)
	}
}
