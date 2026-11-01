package specindex

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// TestSRGV0979SupersededSpecRegeneratesToACheckedIndex supersedes a live spec in a fixture
// repository, regenerates INDEX.json and README.md with script/gen-spec-requirements.sh, and runs
// the superseded-spec check on the result: no hand edit is needed, and supersededBy names the
// Successor, a bare file name resolving to docs/specs/ (SRG-V0-979, V1-3797).
func TestSRGV0979SupersededSpecRegeneratesToACheckedIndex(t *testing.T) {
	for _, tool := range []string{"git", "perl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("NOT_RUN: the projection needs %s, which is not on PATH", tool)
		}
	}
	generator, err := filepath.Abs(filepath.Join("..", "..", "script", "gen-spec-requirements.sh"))
	if err != nil {
		t.Fatal(err)
	}
	claim := "The fixture specification keeps its index claim, Agent digest and README row in agreement."
	root := indexFixture(t, claim, claim)
	spec := filepath.Join(root, "docs", "specs", "fixture-v0.md")
	if err := os.WriteFile(spec, []byte("# Fixture V0\n\nIntent status: proposed\nDelivery status: superseded\n\n"+
		"## Agent digest\n- Successor: `fixture-v1.md` replaces it.\n- Why: retired by the fixture.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The generator projects onto the README table under its own header row.
	readme := "| Scope | Contract | Intent | Delivery | Binding evidence or next gate |\n|---|---|---|---|---|\n" +
		"| Fixture V0 | [`fixture-v0.md`](fixture-v0.md) | proposed | experimental | " + claim + " |\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "specs", "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "script"), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(generator)
	if err != nil {
		t.Fatal(err)
	}
	if err := execfile.WriteFile(filepath.Join(root, "script", "gen-spec-requirements.sh"), body, 0o755); err != nil {
		t.Fatal(err)
	}
	git("add", "--", "docs/specs")
	for _, projection := range []struct{ mode, file string }{{"--index", "INDEX.json"}, {"--readme", "README.md"}} {
		cmd := exec.Command("sh", "script/gen-spec-requirements.sh", projection.mode)
		cmd.Dir = root
		out, err := cmd.Output()
		if ee, ok := err.(*exec.ExitError); ok {
			t.Logf("stderr: %s", ee.Stderr)
		}
		if err != nil {
			t.Fatalf("%s: %v", projection.mode, err)
		}
		if err := os.WriteFile(filepath.Join(root, "docs", "specs", projection.file), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var entries []entry
	if err := json.Unmarshal(mustRead(t, filepath.Join(root, "docs", "specs", "INDEX.json")), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].SupersededBy == nil || *entries[0].SupersededBy != "docs/specs/fixture-v1.md" || entries[0].Delivery != "superseded" {
		t.Fatalf("projected entry %+v, want supersededBy docs/specs/fixture-v1.md", entries)
	}
	var findings []string
	checkIndex(root, func(format string, args ...any) { findings = append(findings, fmt.Sprintf(format, args...)) })
	if len(findings) != 0 {
		t.Fatalf("the regenerated superseded spec failed the check:\n%s", strings.Join(findings, "\n"))
	}
}
