package appflows

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func sample() Input {
	m := Manifest{Profile: IntentProfile, Application: "fixture", Origin: "http://127.0.0.1:32100", Sources: []string{"app.js"}, Tests: []string{"app.spec.ts"}, BackendSource: "app.js", Fixture: "seed", IdentityPath: "/identity", ResetPath: "/reset", Server: []string{"node", "app.js"}, Scenarios: []Scenario{{ID: "save", Role: "editor", Path: "/editor", Basis: "declared", Actions: []Action{{Kind: "fill", Selector: "#name", Value: "sample"}, {Kind: "click", Selector: "#save"}}, Checks: []Check{{ID: "saved", Kind: "text", Selector: "#status", Want: json.RawMessage(`"saved"`)}}}}}
	return Input{Manifest: m, Binding: Binding{Tree: strings.Repeat("a", 40), ManifestDigest: Digest([]byte("manifest")), Sources: map[string]string{"app.js": Digest([]byte("app")), "app.spec.ts": Digest([]byte("test"))}, FrontendDigest: Digest([]byte("app")), BackendDigest: Digest([]byte("app")), Fixture: "seed"}}
}

func observed(in Input) Evidence {
	a := Anchor{Path: "app.spec.ts", Line: 1, Digest: in.Binding.Sources["app.spec.ts"]}
	actions := slices.Clone(in.Manifest.Scenarios[0].Actions)
	actions[0].Value = Digest([]byte(actions[0].Value))
	return Evidence{Profile: EvidenceProfile, Authority: "CALLER_REPORTED", Binding: in.Binding, RunID: Digest([]byte("run")), Mode: "observe", Parser: "typescript-5.9.3", ProviderDigest: Digest([]byte("provider")), NodeVersion: "v22.23.2", PlaywrightVersion: "1.63.0", Browser: "chromium-fixture", InventoryComplete: true, Cleanup: true, CleanupScope: "owned-process-group-and-owned-browser", BrowserClosed: true, ServerExited: true,
		Tests: []Test{{ID: Digest([]byte("app.spec.ts\x001")), Anchor: a, Route: "/editor", Actions: actions, Complete: true, Assertions: []Assertion{{Kind: "text", Selector: "#status", WantDigest: ValueDigest(json.RawMessage(`"saved"`)), Anchor: a}}}},
		Runs:  []Run{{Scenario: "save", Outcome: "matched", IdentityMatched: true, States: []string{Digest([]byte("state"))}, Checks: []CheckResult{{ID: "saved", Outcome: "matched", Layer: "ui"}}}}}
}

// AFU-V0-003 AFU-V0-004 AFU-V0-005 AFU-V0-012
func TestAFUV0EvidenceSeparation(t *testing.T) {
	t.Run("AFU-V0-003 intent AFU-V0-004 assertions AFU-V0-005 gaps AFU-V0-006 frontier AFU-V0-012 report", func(t *testing.T) {
		in := sample()
		e := observed(in)
		e.Controls = []Control{{ID: Digest([]byte("control")), Kind: "button", Selector: "#help", State: Digest([]byte("state"))}}
		r, err := ReportFor(in, &e)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(r.Frontier, "unexplored-control:"+e.Controls[0].ID) {
			t.Fatal("lost unexplored control")
		}
		if r.Flows[0].RoleIdentity != "caller-declared-unverified" {
			t.Fatal("role identity promoted")
		}
		if r.Complete || r.Flows[0].Coverage[0].State != "assertion-candidate" || r.Flows[0].Runtime != "matched" {
			t.Fatalf("wrong report: %+v", r)
		}
		e.Tests[0].Assertions = nil
		r, err = ReportFor(in, &e)
		if err != nil || r.Flows[0].Coverage[0].State != "action-candidate-without-assertion" {
			t.Fatalf("no-assertion %+v %v", r, err)
		}
		e.Tests[0].Route = "/viewer"
		r, _ = ReportFor(in, &e)
		if r.Flows[0].Coverage[0].State != "no-test-in-declared-inventory" {
			t.Fatal("cross-role mapping")
		}
		e.InventoryComplete = false
		r, _ = ReportFor(in, &e)
		if r.Flows[0].Coverage[0].State != "mapping-unknown" {
			t.Fatal("incomplete inventory claimed absence")
		}
	})
}

// AFU-V0-007 AFU-V0-008 AFU-V0-010 AFU-V0-012
func TestAFUV0ContradictionsAndDrift(t *testing.T) {
	t.Run("AFU-V0-007 contradictions AFU-V0-008 drift AFU-V0-010 cleanup", func(t *testing.T) {
		in := sample()
		e := observed(in)
		e.Runs[0].Outcome = "contradicted"
		e.Runs[0].Checks[0].Outcome = "contradicted"
		r, err := ReportFor(in, &e)
		if err != nil || r.Flows[0].Runtime != "contradicted" {
			t.Fatal(r, err)
		}
		e.Cleanup = false
		r, _ = ReportFor(in, &e)
		if r.Flows[0].Runtime != "inconclusive" {
			t.Fatal("cleanup failure promoted")
		}
		e = observed(in)
		in.Binding.Tree = strings.Repeat("b", 40)
		r, _ = ReportFor(in, &e)
		if r.Freshness != "stale" || r.Flows[0].Runtime != "stale" {
			t.Fatal("old build promoted")
		}
		in = sample()
		in.Manifest.Scenarios[0].Basis = "inferred"
		e = observed(in)
		r, _ = ReportFor(in, &e)
		if r.Flows[0].Runtime != "hypothesis-agreement" {
			t.Fatal("inferred intent accepted")
		}
		e.Runs[0].Checks = nil
		if _, err = ReportFor(in, &e); err == nil {
			t.Fatal("missing postcondition accepted")
		}
	})
}

// AFU-V0-001 AFU-V0-010 AFU-V0-011
func TestAFUV0DecodeAndAdmission(t *testing.T) {
	t.Run("AFU-V0-001 scope AFU-V0-011 screening", func(t *testing.T) {
		for _, raw := range []string{`{"profile":"a","profile":"b"}`, `{"unknown":true}`, `[]`, strings.Repeat(" ", MaxBytes+1)} {
			var m Manifest
			if Decode([]byte(raw), &m) == nil {
				t.Fatalf("accepted malformed input %.80s", raw)
			}
		}
		for _, origin := range []string{"https://example.com", "http://localhost:80", "http://127.0.0.1:80/path", "http://user:pass@127.0.0.1:80"} {
			in := sample()
			in.Manifest.Origin = origin
			if ValidateManifest(in.Manifest) == nil {
				t.Fatal("unsafe origin", origin)
			}
		}
		in := sample()
		if err := ValidateManifest(in.Manifest); err != nil {
			t.Fatal(err)
		}
		in.Manifest.Sources[0] = "../outside"
		if ValidateManifest(in.Manifest) == nil {
			t.Fatal("escaped source")
		}
		in = sample()
		in.Manifest.Scenarios[0].Actions[0].Kind = "evaluate"
		if ValidateManifest(in.Manifest) == nil {
			t.Fatal("script execution accepted")
		}
		in = sample()
		in.Manifest.Scenarios[0].Checks[0].Want = json.RawMessage(`{"unbounded":"object"}`)
		if ValidateManifest(in.Manifest) == nil {
			t.Fatal("nonscalar oracle accepted")
		}
		t.Run("AFU-V0-008 conflicting route roles", func(t *testing.T) {
			in := sample()
			s := in.Manifest.Scenarios[0]
			s.ID, s.Role = "viewer-copy", "viewer"
			in.Manifest.Scenarios = append(in.Manifest.Scenarios, s)
			if ValidateManifest(in.Manifest) == nil {
				t.Fatal("ambiguous role accepted")
			}
		})
	})
}

func fixture(t *testing.T) (string, Input) {
	t.Helper()
	root := t.TempDir()
	in := sample()
	raw, _ := json.Marshal(in.Manifest)
	for p, b := range map[string][]byte{"app.js": []byte("app"), "app.spec.ts": []byte("test"), "flows.json": raw} {
		if err := os.WriteFile(filepath.Join(root, p), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	gitTest(t, root, "init", "-q")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	got, err := Capture(context.Background(), root, "flows.json")
	if err != nil {
		t.Fatal(err)
	}
	return root, got
}

func gitTest(t *testing.T, root string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git: %v: %s", err, out)
	}
}

// AFU-V0-001 AFU-V0-002 AFU-V0-008 AFU-V0-009 AFU-V0-011
func TestAFUV0CaptureAndRecord(t *testing.T) {
	t.Run("AFU-V0-002 sources AFU-V0-009 record", func(t *testing.T) {
		root, in := fixture(t)
		e := observed(in)
		raw, _ := json.Marshal(e)
		output := filepath.Join(root, "learned.json")
		if err := Record(in, raw, output); err != nil {
			t.Fatal(err)
		}
		if err := Record(in, raw, output); err == nil {
			t.Fatal("overwrote prior observation")
		}
		st, err := os.Stat(output)
		if err != nil || st.Mode().Perm() != 0600 {
			t.Fatal("record is not private", err)
		}
		if err = os.WriteFile(filepath.Join(root, "app.js"), []byte("changed"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = Capture(context.Background(), root, "flows.json"); err == nil {
			t.Fatal("dirty source bound as immutable")
		}
		e.Authority = "ACCEPTED"
		raw, _ = json.Marshal(e)
		if Record(in, raw, filepath.Join(root, "false-authority.json")) == nil {
			t.Fatal("authority elevated")
		}
		outside := filepath.Join(t.TempDir(), "private")
		if err = os.WriteFile(outside, []byte("outside"), 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
			t.Fatal(err)
		}
		if _, err = readSource(root, "escape"); err == nil {
			t.Fatal("followed escaping symlink")
		}
	})
}

// AFU-V0-004 AFU-V0-007
func TestAFUV0InvalidEvidenceDoesNotBecomeCoverage(t *testing.T) {
	in := sample()
	changes := []func(*Evidence){
		func(e *Evidence) { e.Tests[0].Anchor.Digest = Digest([]byte("different")) },
		func(e *Evidence) { e.Tests[0].Assertions[0].Anchor.Path = "other.spec.ts" },
		func(e *Evidence) { e.Runs[0].Checks[0].Layer = "backend" },
		func(e *Evidence) { e.Runs = append(e.Runs, e.Runs[0]) },
		func(e *Evidence) { e.Tests[0].Actions[0].Value = "raw input" },
	}
	for i, change := range changes {
		e := observed(in)
		change(&e)
		if _, err := ReportFor(in, &e); err == nil {
			t.Fatalf("accepted invalid case %d", i)
		}
	}
}
