package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
)

const overlayIntent = "# Reviewer intent\n\n## Requirements\n\n- `REV-001`: The parser refuses an empty name.\n- `REV-002`: The parser keeps the input order.\n\n## Notes\n\nNot in scope.\n"

// TestCEMV1642ObligationsReviewerOverlay (CEM-V1-642): an out-of-tree reviewer
// intent adds only the advisory reviewer_overlay member. Every other member,
// the id and the top-level state are those of the run without it, and each
// derived obligation stays UNKNOWN. A relative, in-tree, symlinked, missing or
// non-intent overlay refuses with exit 2 and no stdout, as does the flag
// outside the stable mode.
func TestCEMV1642ObligationsReviewerOverlay(t *testing.T) {
	change := stablefixture.NewChange(t)
	stable := change.Stable(t)
	scratch := t.TempDir()
	cemPath, impactPath := filepath.Join(scratch, "stable.cem.json"), filepath.Join(scratch, "impact.json")
	overlayPath, notIntent := filepath.Join(scratch, "intent.md"), filepath.Join(scratch, "notes.md")
	inTree := filepath.Join(change.Root, "reviewer-intent.md")
	receipt := `{"ok":true,"tool":"impact","context":{"profile":"impact"}}`
	for path, content := range map[string]string{
		cemPath: string(stable.Map), impactPath: receipt, overlayPath: overlayIntent, notIntent: "# Notes\n\nNo requirements here.\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	symlink := filepath.Join(scratch, "link.md")
	if err := os.Symlink(overlayPath, symlink); err != nil {
		t.Fatal(err)
	}
	arguments := func(extra ...string) []string {
		return append([]string{"obligations", "--cem", cemPath, "--impact", impactPath, "--repository", change.Root,
			"--expected-base", change.Base, "--target", stable.Target, "--stable-artifact-root", stable.ArtifactRoot}, extra...)
	}
	decode := func(out string) map[string]any {
		var document map[string]any
		if err := json.Unmarshal([]byte(out), &document); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return document
	}
	code, plain, stderr := runCLI(t, arguments()...)
	if code != 0 || stderr != "" || strings.Contains(plain, "reviewer_overlay") {
		t.Fatalf("without overlay: exit %d stderr %q out %s", code, stderr, plain)
	}
	code, out, stderr := runCLI(t, arguments("--intent-overlay", overlayPath)...)
	if code != 0 || stderr != "" {
		t.Fatalf("with overlay: exit %d stderr %q", code, stderr)
	}
	withOverlay := decode(out)
	overlay, ok := withOverlay["reviewer_overlay"].(map[string]any)
	if !ok {
		t.Fatalf("no reviewer_overlay member: %s", out)
	}
	delete(withOverlay, "reviewer_overlay")
	if without := decode(plain); !reflect.DeepEqual(withOverlay, without) {
		t.Fatalf("the overlay changed the sidecar:\n%s\n%s", out, plain)
	}
	if withOverlay["state"] != "unknown" {
		t.Fatalf("state = %v, want the unchanged unknown", withOverlay["state"])
	}
	obligations, _ := overlay["obligations"].([]any)
	if overlay["authority"] != "reviewer-supplied" || overlay["authoritative"] != false || len(obligations) != 2 ||
		!strings.Contains(overlay["note"].(string), "not project authority") {
		t.Fatalf("overlay = %v", overlay)
	}
	for index, want := range []string{"REV-001", "REV-002"} {
		obligation := obligations[index].(map[string]any)
		if obligation["requirement"] != want || obligation["state"] != "UNKNOWN" {
			t.Fatalf("obligation %d = %v", index, obligation)
		}
	}
	// The in-tree file is written only after the admitted runs, so it cannot
	// disturb their repository reads.
	if err := os.WriteFile(inTree, []byte(overlayIntent), 0o644); err != nil {
		t.Fatal(err)
	}
	refusals := map[string][]string{
		"relative":   arguments("--intent-overlay", "intent.md"),
		"in tree":    arguments("--intent-overlay", inTree),
		"symlink":    arguments("--intent-overlay", symlink),
		"missing":    arguments("--intent-overlay", filepath.Join(scratch, "absent.md")),
		"not intent": arguments("--intent-overlay", notIntent),
		"no stable":  {"obligations", "--cem", cemPath, "--impact", impactPath, "--intent-overlay", overlayPath},
	}
	for name, invalid := range refusals {
		if code, out, stderr := runCLI(t, invalid...); code != 2 || out != "" || !strings.Contains(stderr, "--intent-overlay") {
			t.Errorf("%s: exit %d out %q stderr %q", name, code, out, stderr)
		}
	}
}
