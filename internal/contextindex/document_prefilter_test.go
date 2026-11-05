// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Frozen prefilter-free document parser, retained as an independent oracle.
func documentRecordRegexOracle(source Source, text string, sources map[string]Source) (Record, bool) {
	kind := documentKind(source.Path)
	if kind == "" {
		return Record{}, false
	}
	lines := strings.Split(text, "\n")
	title := strings.TrimSuffix(path.Base(source.Path), path.Ext(source.Path))
	titleLine := 1
	titleFound := false
	status := ""
	headings := make([]string, 0)
	summary := ""
	inFrontmatter := false
	fence := ""
	// An instructions file's summary is its preamble, the body under the
	// title before any section, or else its title: the first line of
	// whichever section comes first is not the document's purpose (V1-2074).
	instructions := kind == "instructions"
	inSection := false
	for index, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if match := documentTitle.FindStringSubmatch(raw); match != nil && !titleFound {
			title = strings.TrimSpace(match[1])
			titleLine = index + 1
			titleFound = true
		}
		if match := documentHeading.FindStringSubmatch(raw); match != nil {
			headings = append(headings, match[1])
		}
		// A fenced block is an example, not the document's own field
		// (FPK-V0-029): its lines, fences included, never supply the status.
		open := fence
		fence = nextFence(fence, trimmed)
		if instructions && open == "" && documentHeading.MatchString(raw) && !(titleFound && titleLine == index+1) {
			inSection = true
		}
		if status == "" && open == "" && fence == "" {
			if match := documentStatus.FindStringSubmatch(raw); match != nil {
				status = strings.ToLower(strings.TrimSpace(match[1]))
			}
		}
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if summary == "" && !inSection && !inFrontmatter && trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "```") && !strings.HasPrefix(trimmed, ">") {
			summary = truncateRunes(trimmed, 500)
		}
	}
	if instructions && summary == "" {
		summary = truncateRunes(title, 500)
	}
	referencesSet := make(map[string]struct{})
	for _, candidate := range documentReferences(text) {
		if normalized := normalizedReference(source.Path, candidate, sources); normalized != "" && normalized != source.Path {
			referencesSet[normalized] = struct{}{}
		}
	}
	references := keys(referencesSet)
	if len(references) > 100 {
		references = references[:100]
	}
	headingText := strings.Join(headings, " ")
	headingText = truncateRunes(headingText, 4000)
	return toolchainRecord(Record{kind, source.Path, source.Path, source.BlobHash, titleLine, map[string]any{
		"title": title, "summary": summary, "status": status,
		"headings": headingText, "references": references,
	}}, lines, sources), true
}

func TestGPKV08001DocumentPrefiltersMatchOriginal(t *testing.T) {
	t.Run("GPK-V0-8001 parity", func(t *testing.T) {
		check := func(name, text string, sources map[string]Source) {
			t.Helper()
			source := Source{Path: name, BlobHash: "fixture", Data: []byte(text)}
			got, gok := documentRecord(source, text, sources)
			want, wok := documentRecordRegexOracle(source, text, sources)
			if gok != wok || !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: gated document parser differs from original: %#v != %#v", name, got, want)
			}
			for _, line := range strings.Split(text, "\n") {
				if documentStatus.MatchString(line) && !mayHoldDocumentStatus(line) {
					t.Fatalf("status prefilter lost %q", line)
				}
			}
		}
		edges := []string{"", "# title\nprose\n## section\nStatus: accepted\n", "#\u0085title\nſtatuſ: accepted\n", "#  \n##  \n", "# title\nDate: now. **STATUS**: 'accepted'\n", "# title\n```md\nStatus: rejected\n```\nIntent status: proposed\n", "# title\n\u00a0Status: accepted\n", "# title\nsTaTuS: accepted\n", "# title\n\xffſTATUſ: accepted\n", "---\nstatus: accepted\n---\n# title\n"}
		for _, name := range []string{"docs/specs/fixture.md", "AGENTS.md", "docs/decisions/fixture.md"} {
			for _, text := range edges {
				check(name, text, map[string]Source{})
			}
		}
		if _, err := exec.LookPath("git"); err != nil {
			t.Skipf("tracked document parity requires git on PATH: %v", err)
		}
		root := repositoryRoot(t)
		listed, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
		if err != nil {
			t.Fatal(err)
		}
		sources := map[string]Source{}
		for _, name := range strings.Split(string(listed), "\x00") {
			if name != "" {
				sources[name] = Source{Path: name, BlobHash: "fixture"}
			}
		}
		n := 0
		for name := range sources {
			if documentKind(name) == "" {
				continue
			}
			info, err := os.Stat(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() > maxSourceBytes {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			check(name, string(data), sources)
			n++
		}
		if n < 100 {
			t.Fatalf("self corpus too small: %d documents", n)
		}
		t.Logf("compared %d self-corpus documents with the original regex parser", n)
	})
}

func BenchmarkGPKV08001DocumentRecord(b *testing.B) {
	source := Source{Path: "docs/specs/fixture.md", BlobHash: "fixture"}
	text := "# Feature\n\n" + strings.Repeat("An operator opens the ledger and confirms the entry before saving.\n", 2000) + "\n## Details\n\nStatus: accepted\n"
	for _, legacy := range []bool{true, false} {
		name := "Prefiltered"
		if legacy {
			name = "Original"
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				if legacy {
					documentRecordRegexOracle(source, text, nil)
				} else {
					documentRecord(source, text, nil)
				}
			}
		})
	}
}

func TestGPKV08001DocumentRegexBudget(t *testing.T) {
	t.Run("GPK-V0-8001 backend call budget", func(t *testing.T) {
		oldTitle, oldHeading, oldStatus := findDocumentTitle, findDocumentHeading, findDocumentStatus
		t.Cleanup(func() { findDocumentTitle, findDocumentHeading, findDocumentStatus = oldTitle, oldHeading, oldStatus })
		var titles, headings, statuses int
		findDocumentTitle = func(s string) []string { titles++; return oldTitle(s) }
		findDocumentHeading = func(s string) []string { headings++; return oldHeading(s) }
		findDocumentStatus = func(s string) []string { statuses++; return oldStatus(s) }
		text := "# Feature\n" + strings.Repeat("Ordinary prose about the ledger.\n", 2000) + "## Details\nStatus: accepted\n"
		documentRecord(Source{Path: "docs/specs/fixture.md"}, text, nil)
		if titles != 1 || headings != 2 || statuses != 1 {
			t.Fatalf("regex calls title=%d heading=%d status=%d, want 1/2/1", titles, headings, statuses)
		}
	})
}
