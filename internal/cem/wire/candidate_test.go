package wire

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func candidateFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("..", "..", "..", "protocol", "cem-1.0", "maps", name))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestCandidateClosedProfile(t *testing.T) {
	raw := candidateFixture(t, "sha1-valid.json")
	c, e := ParseCandidate(raw)
	if e != nil {
		t.Fatal(e)
	}
	if c.Change.Spec != CandidateSpec || Canonical(CandidateSpec) || len(c.Links) != 1 || len(c.Artifacts) != 5 {
		t.Fatalf("candidate silently promoted: %+v", c)
	}
	if _, e = ParseMap(raw); cemcode.CodeOf(e) != cemcode.UnsupportedSpec {
		t.Fatalf("legacy reader admitted candidate: %v", e)
	}
	for _, name := range []string{"unknown-stable-profile.json", "unknown-field-killed.json", "criterion-hash-tamper.json", "duplicate-criterion.json", "duplicate-link-reference.json", "dangling-hunk.json", "unknown-receipt-profile.json", "invented-source-assurance.json", "missing-artifact-declaration.json", "duplicate-artifact-path.json", "artifact-path-escape.json", "artifact-role-conflict.json", "unsupported-coverage.json", "duplicate-json-key.json"} {
		t.Run(name, func(t *testing.T) {
			if _, e := ParseCandidate(candidateFixture(t, name)); e == nil {
				t.Fatal("unsafe candidate admitted")
			}
		})
	}
}
func TestCandidateAdditionalBoundaries(t *testing.T) {
	raw := candidateFixture(t, "sha1-valid.json")
	for name, change := range map[string]func(map[string]any){
		"null-array":           func(m map[string]any) { m["criterionBindings"] = nil },
		"unknown-nested-field": func(m map[string]any) { m["runnerReceipts"].([]any)[0].(map[string]any)["satisfied"] = true },
		"case-aliased-field": func(m map[string]any) {
			m["runnerReceipts"].([]any)[0].(map[string]any)["Profile"] = "corvint-test-runner-receipt/0"
		},
		"artifact-git-path": func(m map[string]any) { m["artifacts"].([]any)[0].(map[string]any)["path"] = "x/.GiT/file" },
		"unused-receipt": func(m map[string]any) {
			a := m["runnerReceipts"].([]any)
			x := map[string]any{}
			for k, v := range a[0].(map[string]any) {
				x[k] = v
			}
			x["sha256"] = strings.Repeat("0", 64)
			m["runnerReceipts"] = append(a, x)
		},
		"duplicate-hunk":       func(m map[string]any) { a := m["hunks"].([]any); m["hunks"] = append(a, a[0]) },
		"fractional-criterion": func(m map[string]any) { m["criterionBindings"].([]any)[0].(map[string]any)["criterionIndex"] = 0.5 },
	} {
		t.Run(name, func(t *testing.T) {
			var m map[string]any
			if e := json.Unmarshal(raw, &m); e != nil {
				t.Fatal(e)
			}
			change(m)
			b, _ := json.Marshal(m)
			if _, e := ParseCandidate(b); e == nil {
				t.Fatal("invalid candidate accepted")
			}
		})
	}
	// Candidate integer tokens are closed; historical portable 0.1 is unchanged.
	if _, e := ParseCandidate([]byte(strings.Replace(string(raw), `"criterionIndex": 0`, `"criterionIndex": 0.0`, 1))); e == nil {
		t.Fatal("candidate fractional token accepted")
	}
}
