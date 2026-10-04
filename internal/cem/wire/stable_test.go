package wire

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func stableFixtureRoot(t *testing.T) string {
	t.Helper()
	p := os.Getenv("CEM_STABLE_FIXTURE_ROOT")
	if p == "" {
		t.Skip("explicit literal fixture root required")
	}
	return p
}
func stableFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(stableFixtureRoot(t), "maps", name+".json"))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestStableWireLiteralCorpus(t *testing.T) {
	p := stableFixtureRoot(t)
	b, e := os.ReadFile(filepath.Join(p, "manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Cases []struct {
			ID, Map            string
			ExpectedProjection struct{ Stage string }
		}
	}
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	for _, c := range m.Cases {
		t.Run(c.ID, func(t *testing.T) {
			b, e := os.ReadFile(filepath.Join(p, c.Map))
			if e != nil {
				t.Fatal(e)
			}
			d, e := ParseStable(b)
			bad := c.ExpectedProjection.Stage == "wire" || c.ExpectedProjection.Stage == "references"
			if bad != (e != nil) {
				t.Fatalf("stage %s parse error %v", c.ExpectedProjection.Stage, e)
			}
			if c.ExpectedProjection.Stage == "references" && d == nil {
				t.Fatal("lost complete wire progress")
			}
			if !bad {
				encoded, e := EncodeStable(d)
				if e != nil {
					t.Fatal(e)
				}
				again, e := ParseStable(encoded)
				if e != nil {
					t.Fatal(e)
				}
				d.original = nil
				again.original = nil
				if !reflect.DeepEqual(d, again) {
					t.Fatal("lossy stable round trip")
				}
			}
		})
	}
}
func TestStableLegacyAdmissionAndOriginalBytes(t *testing.T) {
	raw := stableFixture(t, "sha1-positive-sealed")
	saved := bytes.Clone(raw)
	d, e := ParseStable(raw)
	if e != nil {
		t.Fatal(e)
	}
	raw[0] = '!'
	if !bytes.Equal(d.OriginalBytes(), saved) {
		t.Fatal("caller mutated stored original")
	}
	copyOut := d.OriginalBytes()
	copyOut[0] = '!'
	if !bytes.Equal(d.OriginalBytes(), saved) {
		t.Fatal("getter exposed mutable original")
	}
	if _, e = ParseMap(saved); e == nil {
		t.Fatal("legacy parser admitted stable")
	}
	if _, e = ParseCandidate(saved); e == nil {
		t.Fatal("candidate parser admitted stable")
	}
	if Canonical(StableSpec) || MechanicalReason(StableSpec, "rename") {
		t.Fatal("legacy capabilities widened")
	}
	encoded, e := EncodeStable(d)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.HasSuffix(encoded, []byte{'\n'}) {
		t.Fatal("missing canonical LF")
	}
	if !bytes.Equal(d.OriginalBytes(), saved) {
		t.Fatal("encoder replaced original bytes")
	}
	d.Links[0].HunkIDs[0] = HunkPrefix + strings.Repeat("0", 64)
	if _, e = EncodeStable(d); e == nil {
		t.Fatal("edited dangling links must refuse")
	}
}
func TestStableClosedScalarAndDepthBound(t *testing.T) {
	raw := stableFixture(t, "sha1-positive-sealed")
	cases := [][]byte{bytes.Replace(raw, []byte(`"criterionIndex": 0`), []byte(`"criterionIndex": true`), 1), bytes.Replace(raw, []byte(`"criterionIndex": 0`), []byte(`"criterionIndex": 1e0`), 1), []byte(strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65))}
	for _, b := range cases {
		if bytes.Equal(raw, b) {
			t.Fatal("fixture mutation did not apply")
		}
		if _, e := ParseStable(b); e == nil {
			t.Fatal("invalid scalar/depth accepted")
		}
	}
}

func TestStableArtifactReservedGitComponents(t *testing.T) {
	raw := stableFixture(t, "sha1-positive-sealed")
	for _, path := range []string{".git/receipt", ".GiT/receipt", "nested/.GIT/receipt"} {
		t.Run(path, func(t *testing.T) {
			var value map[string]any
			if e := json.Unmarshal(raw, &value); e != nil {
				t.Fatal(e)
			}
			value["artifacts"].([]any)[0].(map[string]any)["path"] = path
			b, e := json.Marshal(value)
			if e != nil {
				t.Fatal(e)
			}
			if d, e := ParseStable(b); e == nil || d != nil {
				t.Fatalf("reserved wire path admitted: %v", e)
			}
			d, e := ParseStable(raw)
			if e != nil {
				t.Fatal(e)
			}
			d.Artifacts[0].Path = path
			if _, e = EncodeStable(d); e == nil {
				t.Fatal("encoder admitted reserved component")
			}
		})
	}
	// Historical path grammar stays unchanged; only the stable artifact contract
	// reserves this component.
	if e := ValidatePath(".GiT/receipt"); e != nil {
		t.Fatalf("legacy grammar widened: %v", e)
	}
}
