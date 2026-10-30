package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
)

// The real OCM producer and verifier must join only the explicitly linked hunk.
// This exercises the outer CLI adapter rather than a substituted callback.
func TestCEMReviewNativeOCMAdapter(t *testing.T) {
	t.Run("CEM-PILOT-006 exact selective hunk joins", func(t *testing.T) {
		fixture := newOCMLinkValidationFixture(t, "TM-V0-008 exact anchor")
		cemWrite(t, fixture.root, "src/unrelated.txt", "independent addition\n")
		cemGit(t, fixture.root, "add", "src/unrelated.txt")
		cemGit(t, fixture.root, "commit", "-qm", "unrelated hunk")
		fixture.target = cemGit(t, fixture.root, "rev-parse", "HEAD")
		// The fixture map is cem/1.0; `cem report --ocm` reads it through the
		// stable route (CEM-V1-210, CEM-V1-211).
		for _, args := range [][]string{
			{"cem", "prepare", "--base", fixture.base, "--target", fixture.target, "--replace"},
			{"cem", "cite", "--map", fixture.mapPath, "--hunk", "1", "--evidence-path", "docs/rule.txt", "--lines", "1:1", "--relation", "specification"},
			{"ocm", "prepare", "--map", "change.ocm.json", "--cem", fixture.mapPath, "--intent", "docs/intent.md", "--expected-base", fixture.base, "--target", fixture.target, "--replace"},
		} {
			code, _, diagnostic := runCLI(t, append([]string{"--root", fixture.root}, args...)...)
			if code != 0 {
				t.Fatalf("prepare %v: %d %s", args, code, diagnostic)
			}
		}
		code, _, diagnostic := runCLI(t, ocmLinkValidationArguments(fixture, "TM-V0-008 exact anchor", "change.ocm.json")...)
		if code != 0 {
			t.Fatalf("native OCM link: %d %s", code, diagnostic)
		}
		// Stable OCM admission reads the exact sidecar committed at the target:
		// commit the map and rebind the OCM to that bind commit.
		mapRaw, err := os.ReadFile(filepath.Join(fixture.root, filepath.FromSlash(fixture.mapPath)))
		if err != nil {
			t.Fatal(err)
		}
		ocmRaw, err := os.ReadFile(filepath.Join(fixture.root, "change.ocm.json"))
		if err != nil {
			t.Fatal(err)
		}
		cemGit(t, fixture.root, "add", "-f", fixture.mapPath)
		cemGit(t, fixture.root, "commit", "-qm", "bind")
		fixture.target = cemGit(t, fixture.root, "rev-parse", "HEAD")
		cemWrite(t, fixture.root, "change.ocm.json", string(stablefixture.RebindOCM(t, ocmRaw, fixture.target, digestBytes(mapRaw))))
		args := []string{"--root", fixture.root, "cem", "report", "--map", fixture.mapPath, "--format", "json", "--expected-base", fixture.base, "--target", fixture.target, "--artifacts", fixture.root}
		read := func(extra ...string) map[string]any {
			t.Helper()
			code, out, diagnostic := runCLI(t, append(append([]string(nil), args...), extra...)...)
			if code > 1 {
				t.Fatalf("report: %d %s", code, diagnostic)
			}
			var envelope map[string]any
			if err := json.Unmarshal([]byte(out), &envelope); err != nil {
				t.Fatalf("report JSON: %v %s", err, out)
			}
			return envelope["review"].(map[string]any)
		}
		joined := read("--ocm", "change.ocm.json")
		if joined["ocmValid"] != true {
			t.Fatalf("native verifier not wired: %v", joined["ocm"])
		}
		hunks := joined["hunks"].([]any)
		if len(hunks) != 2 {
			t.Fatalf("hunk denominator = %d", len(hunks))
		}
		for _, raw := range hunks {
			hunk := raw.(map[string]any)
			want := 0
			if hunk["path"] == "src/app.txt" {
				want = 1
			}
			if got := len(hunk["obligations"].([]any)); got != want {
				t.Fatalf("%v obligations = %d, want %d", hunk["path"], got, want)
			}
		}
		absent := read()
		if absent["ocm"].(map[string]any)["reason"] != "ocm-not-supplied" {
			t.Fatalf("absent OCM: %v", absent["ocm"])
		}
		raw, err := os.ReadFile(filepath.Join(fixture.root, "change.ocm.json"))
		if err != nil {
			t.Fatal(err)
		}
		var invalid map[string]any
		if err := json.Unmarshal(raw, &invalid); err != nil {
			t.Fatal(err)
		}
		invalid["targetRevision"] = fixture.base
		raw, _ = json.Marshal(invalid)
		cemWrite(t, fixture.root, "invalid.ocm.json", string(raw))
		rejected := read("--ocm", "invalid.ocm.json")
		if rejected["ocmValid"] != false {
			t.Fatalf("invalid OCM accepted: %v", rejected["ocm"])
		}
		for _, projection := range []map[string]any{absent, rejected} {
			for _, raw := range projection["hunks"].([]any) {
				// The stable review omits an empty obligation join.
				if obligations, _ := raw.(map[string]any)["obligations"].([]any); len(obligations) != 0 {
					t.Fatal("missing/invalid OCM created an obligation join")
				}
			}
		}
	})
}
