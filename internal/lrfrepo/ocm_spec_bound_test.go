package lrfrepo

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSpecsStayWithinTheOCMObligationBound refuses, at spec-requirements-check
// time, any spec whose ## Requirements section `ocm prepare` would refuse as
// too-many-obligations (OCM-V0-028), so a new requirement added to a full
// section is caught by the doc gates instead of by a later OCM enumeration
// test or a dogfood bind. Define it outside that section and re-home it, as
// the agent-leases spec's re-homed sections do.
//
// overBoundSpecs are the specs already over the bound when this check was
// added; each must stay over it, so the list shrinks as they are restructured.
var overBoundSpecs = map[string]bool{"affected-plan-v0.md": true}

func TestSpecsStayWithinTheOCMObligationBound(t *testing.T) {
	paths, err := filepath.Glob("../../docs/specs/*.md")
	if err != nil || len(paths) == 0 {
		t.Fatalf("specs: %v %v", paths, err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		over := IntentRequirementsRefusal(data) == "too-many-obligations"
		if overBoundSpecs[name] {
			if !over {
				t.Errorf("%s is within the bound now: remove it from overBoundSpecs", name)
			}
			continue
		}
		if over {
			t.Errorf("%s: its ## Requirements section holds more than %d requirement IDs, which `ocm prepare` refuses; define the new requirement in a section of its own or in a related spec", name, MaxOCMObligations)
		}
	}
}
