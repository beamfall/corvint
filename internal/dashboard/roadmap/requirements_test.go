package roadmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRequirementsFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "REQUIREMENTS.tsv")
	content := "id\tfile\tline\ttitle\n" +
		"TCP-V0-001\tdocs/specs/task-coordination-protocol-v0.md\t42\tSome requirement title\n" +
		"\n" +
		"malformed-row-with-too-few-fields\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRequirementsResolvesKnownID(t *testing.T) {
	path := writeRequirementsFixture(t)
	requirements, err := LoadRequirements(path)
	if err != nil {
		t.Fatal(err)
	}
	link := ResolveRequirement(requirements, "TCP-V0-001")
	if !link.Resolved || link.File != "docs/specs/task-coordination-protocol-v0.md" || link.Line != "42" {
		t.Fatalf("resolved link = %+v", link)
	}
}

func TestResolveRequirementUnresolvedForUnknownID(t *testing.T) {
	path := writeRequirementsFixture(t)
	requirements, err := LoadRequirements(path)
	if err != nil {
		t.Fatal(err)
	}
	link := ResolveRequirement(requirements, "TCP-02")
	if link.Resolved || link.File != "" || link.Line != "" || link.Title != "" {
		t.Fatalf("unresolved link = %+v", link)
	}
}

func TestLoadRequirementsMissingFile(t *testing.T) {
	if _, err := LoadRequirements(filepath.Join(t.TempDir(), "missing.tsv")); err == nil {
		t.Fatal("want error for missing requirements file")
	}
}

func TestLoadRequirementsRefusesOversizeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "REQUIREMENTS.tsv")
	row := "X-V0-001\tdocs/specs/x.md\t1\t" + strings.Repeat("t", 1024) + "\n"
	content := "id\tfile\tline\ttitle\n" + strings.Repeat(row, maxInputFileBytes/len(row)+1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRequirements(path); err == nil {
		t.Fatal("want error for a requirements table past maxInputFileBytes")
	}
}

// TestSRGV0017RoadmapLocatesCommittedLayoutLines pins SRG-V0-017 for the roadmap reader: the
// committed index carries no line, so a resolved link takes its line from the spec's definition,
// and a spec that no longer defines the id, or a path outside the root, leaves the line empty.
func TestSRGV0017RoadmapLocatesCommittedLayoutLines(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := "# Spec\n\n| `ONE-V0-001` | table |\n\n- `ONE-V0-001`: Defined here.\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "specs", "one.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "docs", "specs", "REQUIREMENTS.tsv")
	content := "id\tfile\ttitle\tacceptance\tacceptedBy\n" +
		"ONE-V0-001\tdocs/specs/one.md\tDefined here.\tunrecorded\t-\n" +
		"ONE-V0-002\tdocs/specs/one.md\tGone.\tunrecorded\t-\n" +
		"ESC-V0-001\t../outside.md\tOutside.\tunrecorded\t-\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	requirements, err := LoadRequirements(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := newSpecLines(root)
	if link := lines.fill(ResolveRequirement(requirements, "ONE-V0-001")); !link.Resolved || link.Line != "5" || link.Title != "Defined here." {
		t.Fatalf("committed-layout link = %+v, want line 5", link)
	}
	for _, id := range []string{"ONE-V0-002", "ESC-V0-001"} {
		if link := lines.fill(ResolveRequirement(requirements, id)); !link.Resolved || link.Line != "" {
			t.Fatalf("%s link = %+v, want resolved with no line", id, link)
		}
	}
	if _, err := LoadRequirements(writeFile(t, "not\tan\tindex\n")); err == nil {
		t.Fatal("want error for a table whose header is not an index header")
	}
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "REQUIREMENTS.tsv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
