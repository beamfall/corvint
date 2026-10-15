package wire

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestInheritedDiscriminationWitness pins the cem/1.0 hunk discriminates
// member (TCQ-V0-056): three states with counts that agree, candidate-profile
// rejection, and closed keys.
func TestInheritedDiscriminationWitness(t *testing.T) {
	const digest = "3f5c1b2ac0d7e8f9a1b2c3d4e5f60718293a4b5c6d7e8f9012a3b4c5d6e7f809"
	const revision = "4ca153370afd9bd8c6034ad73acc3925150ab681"
	render := func(spec, witness, excluded string) []byte {
		return []byte(`{"spec":"` + spec + `","baseRevision":"` + revision + `",` +
			`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",` + excluded +
			`"evidence":[],"hunks":[{"id":"hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",` +
			`"path":"src/a.go","oldRange":{"start":1,"count":2},"newRange":{"start":4,"count":3},` +
			`"disposition":"unknown","reason":"no-evidence","basis":[]` + witness + `}]}`)
	}
	excluded := `"excludedPath":".corvint/change.cem.json",`
	bounds := `"bounds":{"maxHunks":8,"maxMutants":8,"wallTimeSeconds":600}`
	head := `,"discriminates":{"treeRevision":"` + revision + `","selectionSha256":"` + digest + `",`
	discriminates := head + `"mutants":5,"killed":4,"survived":0,"survivors":[],` + bounds + `,"state":"discriminates","detail":""}`
	survived := head + `"mutants":5,"killed":3,"survived":1,"survivors":[{"operator":"negate-condition","line":5,` +
		`"description":"negate-condition at src/a.go:5 passed every cited test"}],` + bounds + `,"state":"survived","detail":""}`
	notRun := head + `"mutants":0,"killed":0,"survived":0,"survivors":[],` + bounds + `,"state":"not-run","detail":"hunk limit 1 reached"}`
	document, err := parseMechanics(render(StableSpec, survived, excluded), true)
	if err != nil {
		t.Fatalf("survived witness: %v", err)
	}
	witness := document.Hunks[0].Discriminates
	if witness == nil || witness.State != DiscriminationSurvived || witness.TreeRevision != revision ||
		witness.SelectionSha256 != digest || witness.Mutants != 5 || witness.Killed != 3 || witness.Survived != 1 ||
		len(witness.Survivors) != 1 || witness.Survivors[0].Line != 5 || witness.Bounds.WallTimeSeconds != 600 {
		t.Fatalf("witness = %+v", witness)
	}
	for name, member := range map[string]string{"discriminates": discriminates, "not-run": notRun} {
		document, err := parseMechanics(render(StableSpec, member, excluded), true)
		if err != nil || document.Hunks[0].Discriminates == nil || document.Hunks[0].Discriminates.State != name {
			t.Fatalf("%s witness: %v %+v", name, err, document)
		}
	}
	// CEM-V1-090: mutants equal to bounds.maxMutants is admitted; one more is invalid-field (below).
	atBound := strings.Replace(discriminates, `"maxMutants":8`, `"maxMutants":5`, 1)
	if document, err := parseMechanics(render(StableSpec, atBound, excluded), true); err != nil || document.Hunks[0].Discriminates.Bounds.MaxMutants != 5 {
		t.Fatalf("witness at its maxMutants bound: %v %+v", err, document)
	}
	if document, err := parseMechanics(render(StableSpec, "", excluded), true); err != nil || document.Hunks[0].Discriminates != nil {
		t.Fatalf("without a witness: %v %+v", err, document)
	}
	if _, err := parseMechanics(render(StableSpec, discriminates, excluded), false); cemcode.CodeOf(err) != cemcode.UnknownField {
		t.Errorf("candidate with a witness: got %v, want unknown-field", err)
	}
	invalid := map[string]string{
		"discriminates-with-survivor": strings.Replace(survived, `"state":"survived"`, `"state":"discriminates"`, 1),
		"survived-without-survivor":   strings.Replace(discriminates, `"state":"discriminates"`, `"state":"survived"`, 1),
		"not-run-with-mutants":        strings.Replace(discriminates, `"state":"discriminates"`, `"state":"not-run"`, 1),
		"counts-exceed-mutants":       strings.Replace(discriminates, `"killed":4`, `"killed":6`, 1),
		"survivor-count-disagrees":    strings.Replace(survived, `"survived":1`, `"survived":2`, 1),
		"survivor-outside-new-range":  strings.Replace(survived, `"line":5`, `"line":7`, 1),
		"empty-operator":              strings.Replace(survived, `"operator":"negate-condition"`, `"operator":""`, 1),
		"zero-bound":                  strings.Replace(survived, `"maxHunks":8`, `"maxHunks":0`, 1),
		"mutants-exceed-max-mutants":  strings.Replace(discriminates, `"maxMutants":8`, `"maxMutants":4`, 1),
		"bad-state":                   strings.Replace(discriminates, `"state":"discriminates"`, `"state":"killed"`, 1),
		"bad-revision":                strings.Replace(discriminates, revision, "abc", 1),
		"bad-selection":               strings.Replace(discriminates, digest, "abc", 1),
		"control-detail":              strings.Replace(notRun, `"hunk limit 1 reached"`, `"hunk\u0007limit"`, 1),
	}
	for name, member := range invalid {
		_, err := parseMechanics(render(StableSpec, member, excluded), true)
		if err == nil || cemcode.CodeOf(err) != cemcode.InvalidField {
			t.Errorf("%s: got %v, want invalid-field", name, err)
		}
	}
	surplus := map[string]string{
		"witness":  strings.Replace(discriminates, `"mutants":5,`, `"mutants":5,"extra":1,`, 1),
		"survivor": strings.Replace(survived, `"line":5,`, `"line":5,"extra":1,`, 1),
		"bounds":   strings.Replace(discriminates, `"maxHunks":8,`, `"maxHunks":8,"extra":1,`, 1),
	}
	for name, member := range surplus {
		_, err := parseMechanics(render(StableSpec, member, excluded), true)
		if err == nil || cemcode.CodeOf(err) != cemcode.UnknownField {
			t.Errorf("surplus %s key: got %v, want unknown-field", name, err)
		}
	}
}
