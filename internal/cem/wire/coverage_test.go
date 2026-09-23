package wire

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestSpec03CoverageWitness is TCQ-V0-052: a cem/0.3 hunk may carry one
// coverage witness, cem/0.1 and cem/0.2 reject the key as unknown, and a
// witness whose state and ranges disagree or whose ranges leave the hunk's
// new-side range is an invalid field.
func TestSpec03CoverageWitness(t *testing.T) {
	const digest = "3f5c1b2ac0d7e8f9a1b2c3d4e5f60718293a4b5c6d7e8f9012a3b4c5d6e7f809"
	render := func(spec, coverage, excluded string) []byte {
		return []byte(`{"spec":"` + spec + `","baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",` +
			`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",` + excluded +
			`"evidence":[],"hunks":[{"id":"hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",` +
			`"path":"src/a.go","oldRange":{"start":1,"count":2},"newRange":{"start":4,"count":3},` +
			`"disposition":"unknown","reason":"no-evidence","basis":[]` + coverage + `}]}`)
	}
	excluded := `"excludedPath":".corvint/change.cem.json",`
	covered := `,"coverage":{"profileSha256":"` + digest + `","testRun":"go test ./pkg/...","mode":"set",` +
		`"state":"covered","covered":[{"start":4,"count":1},{"start":6,"count":1}]}`
	uncovered := `,"coverage":{"profileSha256":"` + digest + `","testRun":"go test ./pkg/...","mode":"count",` +
		`"state":"uncovered","covered":[]}`
	document, err := ParseMap(render(Spec03, covered, excluded))
	if err != nil {
		t.Fatalf("cem/0.3 covered witness: %v", err)
	}
	witness := document.Hunks[0].Coverage
	if witness == nil || witness.State != CoverageCovered || witness.TestRun != "go test ./pkg/..." ||
		witness.ProfileSha256 != digest || witness.Mode != "set" || len(witness.Covered) != 2 || witness.Covered[1].Start != 6 {
		t.Fatalf("witness = %+v", witness)
	}
	document, err = ParseMap(render(Spec03, uncovered, excluded))
	if err != nil || document.Hunks[0].Coverage == nil || document.Hunks[0].Coverage.State != CoverageUncovered {
		t.Fatalf("cem/0.3 uncovered witness: %v %+v", err, document)
	}
	if document, err := ParseMap(render(Spec03, "", excluded)); err != nil || document.Hunks[0].Coverage != nil {
		t.Fatalf("cem/0.3 without a witness: %v %+v", err, document)
	}
	for spec, path := range map[string]string{Spec01: "", Spec02: excluded} {
		_, err := ParseMap(render(spec, covered, path))
		if err == nil || cemcode.CodeOf(err) != cemcode.UnknownField {
			t.Errorf("%s with a witness: got %v, want unknown-field", spec, err)
		}
	}
	invalid := map[string]string{
		"state-disagrees":   strings.Replace(covered, `"state":"covered"`, `"state":"uncovered"`, 1),
		"empty-but-covered": strings.Replace(uncovered, `"state":"uncovered"`, `"state":"covered"`, 1),
		"outside-new-range": strings.Replace(covered, `{"start":6,"count":1}`, `{"start":6,"count":2}`, 1),
		"overlapping":       strings.Replace(covered, `{"start":6,"count":1}`, `{"start":4,"count":1}`, 1),
		"adjacent":          strings.Replace(covered, `{"start":6,"count":1}`, `{"start":5,"count":1}`, 1),
		"bad-digest":        strings.Replace(covered, digest, "abc", 1),
		"empty-test-run":    strings.Replace(covered, `"go test ./pkg/..."`, `""`, 1),
		"control-test-run":  strings.Replace(covered, `"go test ./pkg/..."`, `"go\u0007test"`, 1),
		"bad-mode":          strings.Replace(covered, `"mode":"set"`, `"mode":"lines"`, 1),
		"bad-state":         strings.Replace(covered, `"state":"covered"`, `"state":"partial"`, 1),
	}
	for name, coverage := range invalid {
		_, err := ParseMap(render(Spec03, coverage, excluded))
		if err == nil || cemcode.CodeOf(err) != cemcode.InvalidField {
			t.Errorf("%s: got %v, want invalid-field", name, err)
		}
	}
	_, err = ParseMap(render(Spec03, strings.Replace(covered, `"mode":"set",`, `"mode":"set","extra":1,`, 1), excluded))
	if err == nil || cemcode.CodeOf(err) != cemcode.UnknownField {
		t.Errorf("surplus witness key: got %v, want unknown-field", err)
	}
}
