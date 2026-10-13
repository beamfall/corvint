package main

import (
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/localauthority"
)

// TestCEMV1167CapsuleRefusesStableEnrollment (CEM-V1-167, CEM-V1-210): the
// protected consumer holds only a Git object capsule, so it cannot run the
// stable verifier; an enrollment whose cem.json is an exact cem/1.0 map with a
// matching base is refused before any object is requested, and a retired
// cem/0.x map refuses with unsupported-spec and the fix line.
func TestCEMV1167CapsuleRefusesStableEnrollment(t *testing.T) {
	change := stablefixture.NewChange(t)
	stable := change.Stable(t)
	in := enrolledInput{
		enrollment: localauthority.Enrollment{Binding: localauthority.Binding{Base: change.Base, Target: stable.Target}},
		raw:        map[string][]byte{"cem.json": stable.Map},
	}
	if err := (objectSet{}).requireEvidencePaths(in); err == nil || err.Error() != "capsule CEM base" {
		t.Fatalf("stable enrollment: %v, want the capsule CEM base refusal", err)
	}
	in.raw["cem.json"] = []byte(`{"spec":"cem/0.2","hunks":[]}`)
	err := (objectSet{}).requireEvidencePaths(in)
	if cemcode.CodeOf(err) != cemcode.UnsupportedSpec || !strings.Contains(err.Error(), wire.RetiredSpecFix) {
		t.Fatalf("0.x enrollment: %v, want unsupported-spec with the fix line", err)
	}
}
