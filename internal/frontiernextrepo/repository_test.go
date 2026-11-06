package frontiernextrepo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/frontiernext"
)

// TestCEMV1210FixtureRecomputeRefusesEveryMap (CEM-V1-210): the experimental
// recompute reads the frozen frontier/0 universe and lrf/0, which admit no map
// since cem/0.x was retired. A cem/1.0 request refuses with unsupported-spec,
// and a retired cem/0.2 map refuses with unsupported-spec and the recovery
// line; neither reaches the edge evaluation.
func TestCEMV1210FixtureRecomputeRefusesEveryMap(t *testing.T) {
	// Admission happens before repository, OCM, enrollment or receipt validation.
	// Reuse a valid wire fixture instead of constructing unreachable authorities.
	cem, err := os.ReadFile(filepath.Join("..", "..", "protocol", "cem-1.0", "stable", "neutral-criterion", "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wire.ParseStable(cem); err != nil {
		t.Fatalf("invalid stable fixture: %v", err)
	}
	r := frontiernext.Request{CEM: cem}
	retired := r
	retired.CEM = bytes.Replace(cem, []byte(`"spec":"cem/1.0"`), []byte(`"spec":"cem/0.2"`), 1)
	if bytes.Equal(retired.CEM, cem) {
		t.Fatal("fixture map lacks the cem/1.0 spec anchor")
	}
	// An empty non-repository directory also witnesses that admission opens no Git repository.
	root := t.TempDir()
	for name, request := range map[string]frontiernext.Request{"cem/1.0": r, "cem/0.2": retired} {
		universe, err := Adapter{root}.Recompute(context.Background(), request, frontiernext.Selection{})
		if cemcode.CodeOf(err) != cemcode.UnsupportedSpec || len(universe.Eligible) != 0 || len(universe.Open) != 0 {
			t.Fatalf("%s: universe %+v err %v", name, universe, err)
		}
		if name == "cem/1.0" && !strings.Contains(err.Error(), "cem/1.0 is admitted only by the stable universe verification") {
			t.Fatalf("%s: did not reach valid stable-map admission: %v", name, err)
		}
		if name == "cem/0.2" && !strings.Contains(err.Error(), wire.RetiredSpecFix) {
			t.Fatalf("%s: refusal lacks the recovery line: %v", name, err)
		}
	}
}
