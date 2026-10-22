//go:build darwin || linux

package verify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The native names of the sealed REPOSITORY-ENVELOPE.md recipe (V1-2136,
// CEM-V1-242): CEM_STABLE_REPOSITORY_CASES and
// TestStableRepositoryEnvelope{Ordinary,Cancelled,DescendantCleanup,RootIdentity}
// in internal/cem/verify. The sealed document is left unchanged; these names
// make it true.

// stableRepositoryCasesEnv names the materialized case array that the sealed
// recipe passes to the native tests.
const stableRepositoryCasesEnv = "CEM_STABLE_REPOSITORY_CASES"

// stableRepositoryCase is one element of the materialized array, with the
// field names of the sealed recipe. Paths are absolute; Expected names the
// frozen full JSON result object.
type stableRepositoryCase struct {
	ID           string
	Repository   string
	Map          string
	ExpectedBase string
	Target       string
	ArtifactRoot string
	Expected     string
	Exit         int
	Env          map[string]string
}

// readStableRepositoryCases reads and validates the materialized array.
func readStableRepositoryCases(t *testing.T, path string) []stableRepositoryCase {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", stableRepositoryCasesEnv, err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var cases []stableRepositoryCase
	if err := decoder.Decode(&cases); err != nil {
		t.Fatalf("%s %s: %v", stableRepositoryCasesEnv, path, err)
	}
	if len(cases) == 0 {
		t.Fatalf("%s %s names no cases", stableRepositoryCasesEnv, path)
	}
	seen := map[string]bool{}
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] {
			t.Fatalf("%s %s: empty or repeated case ID %q", stableRepositoryCasesEnv, path, c.ID)
		}
		seen[c.ID] = true
		for name, value := range map[string]string{"Repository": c.Repository, "Map": c.Map, "Expected": c.Expected} {
			if !filepath.IsAbs(value) {
				t.Fatalf("case %s: %s %q is not an absolute path", c.ID, name, value)
			}
		}
		if c.ArtifactRoot != "" && !filepath.IsAbs(c.ArtifactRoot) {
			t.Fatalf("case %s: ArtifactRoot %q is not an absolute path", c.ID, c.ArtifactRoot)
		}
	}
	return cases
}

// runStableRepositoryCases verifies every materialized case natively and
// compares the complete closed result and the exit code with its frozen
// expectation, bound to the running toolchain only through an admitted tuple.
func runStableRepositoryCases(t *testing.T, packet *s0ePacket, path string) {
	t.Helper()
	for _, c := range readStableRepositoryCases(t, path) {
		t.Run(c.ID, func(t *testing.T) {
			raw, err := os.ReadFile(c.Map)
			if err != nil {
				t.Fatal(err)
			}
			frozen, err := os.ReadFile(c.Expected)
			if err != nil {
				t.Fatal(err)
			}
			var expected map[string]any
			if err := json.Unmarshal(frozen, &expected); err != nil {
				t.Fatalf("expected %s: %v", c.Expected, err)
			}
			bound := s0eBindRuntime(t, packet, s0eCase{ID: c.ID, ExpectedExit: c.Exit, ExpectedResult: expected})
			for key, value := range c.Env {
				t.Setenv(key, value)
			}
			result, exit := Stable(context.Background(), raw, StableOptions{Repository: c.Repository, ExpectedBase: c.ExpectedBase, Target: c.Target, ArtifactRoot: c.ArtifactRoot})
			status := "PASS"
			if !requireS0EResult(t, bound, result, exit) {
				status = "FAIL"
			}
			t.Logf("REPOSITORY-ENVELOPE-CASE %s %s exit=%d stage=%s", c.ID, status, exit, result.Stage)
		})
	}
}

// stableRepositoryOrdinaryCells are the ordinary, origin and linked cells of
// the sealed recipe, the ones a materialized array carries.
var stableRepositoryOrdinaryCells = []string{
	"ordinary-sha1", "ordinary-sha256", "origin-sha1", "origin-sha256",
	"linked-sha1", "linked-sha256", "relative-sha1", "relative-sha256",
}

// materializeStableRepositoryCases builds the ordinary cells as fresh real
// repositories under the test's temporary directory and writes the array the
// sealed recipe describes, so the env contract is exercised without an
// external packet.
func materializeStableRepositoryCases(t *testing.T, packet *s0ePacket) string {
	t.Helper()
	cells := loadEnvelopeCells(t, packet)
	dir := t.TempDir()
	var cases []stableRepositoryCase
	for _, id := range stableRepositoryOrdinaryCells {
		c := envelopeCellByID(t, cells, id)
		f := newEnvelopeFixture(t, packet, c)
		if strings.HasPrefix(id, "relative-") && !envelopeRelativePaths(t, f.repo) {
			t.Logf("REPOSITORY-ENVELOPE-CASE %s NOT_RUN: host Git has no `git worktree add --relative-paths` (CEM-V1-361)", id)
			continue
		}
		if recipe := envelopeCellRecipes[id]; recipe != nil {
			if env := recipe(f); len(env) != 0 {
				t.Fatalf("cell %s recipe sets environment; the ordinary cells set none", id)
			}
		}
		mapPath := filepath.Join(dir, id+".map.json")
		expectedPath := filepath.Join(dir, id+".expected.json")
		raw := packet.records["fixtures/"+c.Map]
		if raw == nil {
			t.Fatalf("packet has no map %s", c.Map)
		}
		expected, err := json.Marshal(c.ExpectedResult)
		if err != nil {
			t.Fatal(err)
		}
		for path, data := range map[string][]byte{mapPath: raw, expectedPath: expected} {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cases = append(cases, stableRepositoryCase{ID: id, Repository: f.verified, Map: mapPath, ExpectedBase: c.ExpectedBase,
			Target: c.Target, ArtifactRoot: f.artifacts, Expected: expectedPath, Exit: c.ExpectedExit})
	}
	encoded, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "cases.json")
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestStableRepositoryEnvelopeOrdinary runs the materialized array named by
// CEM_STABLE_REPOSITORY_CASES. Unset, it materializes the packet's ordinary,
// origin and linked cells itself and runs them through the same path.
func TestStableRepositoryEnvelopeOrdinary(t *testing.T) {
	packet := loadS0EPacket(t)
	requireStableLifecyclePlatform(t)
	path := os.Getenv(stableRepositoryCasesEnv)
	if path == "" {
		path = materializeStableRepositoryCases(t, packet)
	}
	runStableRepositoryCases(t, packet, path)
}

// runS0ECasesWhere runs the public full-result cases selected by keep through
// the same comparison as TestStableS0EPublicCases and requires at least want.
func runS0ECasesWhere(t *testing.T, want int, keep func(id string) bool) {
	t.Helper()
	packet := loadS0EPacket(t)
	requireStableLifecyclePlatform(t)
	selected := 0
	for _, c := range packet.cases {
		if !keep(c.ID) {
			continue
		}
		selected++
		t.Run(c.ID, func(t *testing.T) {
			t.Parallel()
			c := s0eBindRuntime(t, packet, c)
			result, exit, run := runS0E(t, packet, c, s0eRecipes[c.ID])
			ok := requireS0EResult(t, c, result, exit)
			if want, bound := s0eLedger[c.ID]; bound && run.attempted != want {
				ok = false
				t.Errorf("attempted logical operations = %d, want %d", run.attempted, want)
			}
			if !ok && run.diagnose != nil {
				run.diagnose()
			}
		})
	}
	if selected < want {
		t.Fatalf("selected %d public cases, want at least %d", selected, want)
	}
}

// TestStableRepositoryEnvelopeCancelled runs the public cancellation cases:
// context cancellation maps to verification-timeout at its observed stage.
func TestStableRepositoryEnvelopeCancelled(t *testing.T) {
	t.Parallel()
	runS0ECasesWhere(t, 11, func(id string) bool {
		return strings.HasPrefix(id, "cancel-") || strings.Contains(id, "-cancel")
	})
}

// TestStableRepositoryEnvelopeDescendantCleanup runs the public cases that
// prove owned-group teardown: a contained lingering descendant and an
// unproved owner failure.
func TestStableRepositoryEnvelopeDescendantCleanup(t *testing.T) {
	t.Parallel()
	runS0ECasesWhere(t, 3, func(id string) bool {
		return strings.Contains(id, "lingering-descendant") || strings.HasPrefix(id, "owner-failure-")
	})
}

// TestStableRepositoryEnvelopeRootIdentity runs the public case whose
// repository root is replaced before the final identity check.
func TestStableRepositoryEnvelopeRootIdentity(t *testing.T) {
	t.Parallel()
	runS0ECasesWhere(t, 1, func(id string) bool { return id == "root-replaced-before-final-check" })
}
