package dogfoodflow

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOCMV0880CoverageCountsRequirementsAddedOutsideTheSection reproduces
// V1-2083: test-source-analysis-v0.md defines TSA-V0-410..419 under a later
// `##` section, outside its one `## Requirements` section, so OCM-V0-001
// enumerated none of them and the SUMMARY counted requirements=0. The change
// coverage now counts a requirement the diff defines outside the section,
// traces it by a test naming it, and names it in the outside note; a fenced
// requirement line outside the section and an unchanged one stay uncounted.
func TestOCMV0880CoverageCountsRequirementsAddedOutsideTheSection(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	write := func(path, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) string {
		t.Helper()
		testGit(t, root, "add", "-A", ".")
		testGit(t, root, "commit", "-q", "-m", message)
		return testGit(t, root, "rev-parse", "HEAD")
	}
	spec := "# Spec\n\n## Requirements\n\n- `TSA-V0-001`: one.\n\n## Example receipt join\n\n- `TSA-V0-400`: earlier.\n\n## Boundaries\n\nText.\n"
	write("spec.md", spec)
	write("code.go", "package a\n")
	base := commit("base")
	added := strings.Replace(spec, "- `TSA-V0-400`: earlier.\n", "- `TSA-V0-400`: earlier.\n- `TSA-V0-410`: joined.\n  continued.\n- `TSA-V0-411`: also.\n```\n- `TSA-V0-499`: fenced example.\n```\n", 1)
	write("spec.md", added)
	write("a_test.go", "package a\n// TSA-V0-410 case\n")
	target := commit("adds two requirements outside the section")
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(".corvint/change.cem.json", `{"evidence":[],"hunks":[]}`)
	write(".corvint/change.ocm.001.json", `{"obligations":[]}`)
	run := &flow{ctx: context.Background(), root: root, stderr: io.Discard, base: base, target: target}
	got := run.changeCoverage(filepath.Join(root, ".corvint/change.cem.json"), []string{"spec.md"})
	want := changeCoverage{requirements: 2, linked: 1, unlinked: "TSA-V0-411", outside: "TSA-V0-410,TSA-V0-411"}
	if got == nil || *got != want {
		t.Fatalf("coverage %+v, want %+v", got, want)
	}
	if note := got.outsideNote(); !strings.HasPrefix(note, "dogfood-check: NOTE requirements-outside-ocm-section count=2 ids=TSA-V0-410,TSA-V0-411: ") {
		t.Errorf("outside note = %q", note)
	}
	// Items outside the section end at the next requirement line or heading.
	section := parseRequirements([]byte(added))
	var ids []string
	for _, item := range section.outside {
		ids = append(ids, item.id)
	}
	if strings.Join(ids, " ") != "TSA-V0-400 TSA-V0-410 TSA-V0-411" || len(section.items) != 1 {
		t.Errorf("outside=%q items=%d", ids, len(section.items))
	}
}
