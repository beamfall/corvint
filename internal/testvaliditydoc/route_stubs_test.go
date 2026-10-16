package testvaliditydoc

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type routeStubVectors struct {
	Cases []struct {
		Name     string            `json:"name"`
		Receipt  json.RawMessage   `json:"receipt"`
		Expected []json.RawMessage `json:"expected"`
	} `json:"cases"`
}

func routeStubDocument(t *testing.T, receipt []byte) Document {
	t.Helper()
	return projectDocument(t, `{"receipt":`+string(receipt)+`,"testProjections":[],"runProjection":{}}`)
}

// LPCV-V0-073 LPCV-V0-074: every shared vector's routeStubs members, test by test, including the
// required hit, zero-hit and record-absent cases.
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
	for _, name := range []string{"hit", "zero-hit", "record-absent"} {
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
