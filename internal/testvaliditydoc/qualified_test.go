package testvaliditydoc

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

func TestQualifiedDecodeRejectsNoncanonicalAndPreservesUnknowns(t *testing.T) {
	r := jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown"}, Tests: []jstestprovider.TestOutcome{{Name: "unknown-project", State: jstestprovider.StatePassed}}}
	data, err := jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	input, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	doc := Project(input)
	if doc.Tests[0].Projection.Execution.State == testvalidity.ExecutionPassed || doc.Playwright.External.ServerDescendants != "unknown" {
		t.Fatal("unknown qualified observation became green")
	}
	for _, malformed := range [][]byte{append([]byte(" "), data...), bytes.Replace(data, []byte(`"ownership":"external"`), []byte(`"ownership":"external","unexpected":true`), 1), append(data, []byte("{}")...)} {
		if _, err := Decode(malformed); err == nil {
			t.Fatal("malformed qualified receipt admitted")
		}
	}
	mixed := bytes.Replace(data, []byte(`"profile":"corvint-playwright-external/0"`), []byte(`"profile":"corvint-playwright-external/0","applicationAttestation":{}`), 1)
	if _, err := Decode(mixed); err == nil {
		t.Fatal("legacy profile admitted attested identity fields")
	}
	v1 := bytes.Replace(data, []byte(`"profile":"corvint-playwright-external/0"`), []byte(`"profile":"corvint-playwright-external/1"`), 1)
	input, err = Decode(v1)
	if err != nil || Project(input).Tests[0].Projection.Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("incomplete /1 receipt did not remain explicit infrastructure")
	}
	r.Profile = ""
	data, err = jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Decode(data); err == nil {
		t.Fatal("stripped profile downgraded external metadata to legacy")
	}
	for _, raw := range []string{`{"receipt":{"kind":"e2e","tests":[{"name":"pass","state":"passed","project":{"name":"p"}}]}}`, `{"receipt":{"kind":"e2e","tests":[{"name":"pass","state":"passed","attempts":[{"state":"passed"}]}]}}`} {
		if _, err = Decode([]byte(raw)); err == nil {
			t.Fatal("legacy decoder accepted qualified metadata")
		}
	}
}
