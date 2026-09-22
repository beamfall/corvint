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
