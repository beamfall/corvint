package doccompiler

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func draftFixture() *contextindex.Index {
	source := func(name, text string) contextindex.Source {
		return contextindex.Source{Path: name, BlobHash: strings.Repeat("a", 40), Data: []byte(text), Mode: "100644"}
	}
	return &contextindex.Index{CommitRevision: strings.Repeat("b", 40), Revision: strings.Repeat("c", 40), Module: "example.test/m", Sources: map[string]contextindex.Source{
		"docs/owner.md":   source("docs/owner.md", "# Compiler\nDelivery status: experimental\n\n## Agent digest\n- Claim: source-bound drafts\n- Blocked on: validation\n\n## Requirements\nFull body omitted.\n"),
		"compiler/api.go": source("compiler/api.go", "package compiler\nfunc Plan() {}\n"),
		"caller/main.go":  source("caller/main.go", "package main\n// \"example.test/m/compiler\" is a decoy\nimport \"example.test/m/compiler\"\n"),
	}, Symbols: []contextindex.Symbol{{Path: "compiler/api.go", BlobHash: strings.Repeat("a", 40), Name: "Plan", Kind: "func", Line: 2}}, Imports: map[string]map[string]struct{}{"caller/main.go": {"example.test/m/compiler": {}}}}
}

// SDD-V0-003: owner spans count physical lines in both sourceEntry and dryRunOwnerEntry.
func TestSourceDraftOwnerPhysicalLines(t *testing.T) {
	const sevenLines = "# Compiler\n\n## Agent digest\n- Claim: source-bound drafts\n- Status: experimental\n- Exists: draft\n- Blocked on: validation"
	for _, tc := range []struct {
		name string
		text string
		end  int
	}{
		{"trailing-newline", sevenLines + "\n", 7},
		{"no-trailing-newline", sevenLines, 7},
		{"blank-final-line", sevenLines + "\n\n", 8},
		{"later-heading", sevenLines + "\n## Details\nOutside excerpt.\n", 7},
	} {
		for _, dryRun := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dry-run=%t", tc.name, dryRun), func(t *testing.T) {
				index := draftFixture()
				source := index.Sources["docs/owner.md"]
				source.Data = []byte(tc.text)
				index.Sources[source.Path] = source
				var draft SourceDraft
				var err error
				if dryRun {
					draft, err = DraftDryRun(index, source.Path, source.Data, "compiler")
				} else {
					draft, err = DraftSources(index, source.Path, "compiler")
				}
				if err != nil {
					t.Fatal(err)
				}
				owner := draft.Entries[0]
				if owner.StartLine != 1 || owner.EndLine != tc.end {
					t.Fatalf("owner span = %d:%d, want 1:%d", owner.StartLine, owner.EndLine, tc.end)
				}
				citations := []string{fmt.Sprintf("source `%q`, lines 1–%d;", source.Path, tc.end)}
				if !dryRun {
					// SDD-V0-631: the owner cites its source-table ID; the table names the path once.
					citations = []string{fmt.Sprintf("\n## S3 `%q`\n\nBlob ", source.Path), fmt.Sprintf("\n`\"owner\"` `\"Agent digest\"`, lines 1–%d:\n", tc.end)}
				}
				for _, citation := range citations {
					if !strings.Contains(string(draft.Markdown), citation) {
						t.Fatalf("draft missing owner citation %q:\n%s", citation, draft.Markdown)
					}
				}
				wantExcerpt := sevenLines
				if tc.end == 8 {
					wantExcerpt += "\n"
				}
				if owner.Excerpt != wantExcerpt {
					t.Fatalf("excerpt = %q, want %q", owner.Excerpt, wantExcerpt)
				}
			})
		}
	}
}

func TestSourceDraftOriginalBindingsAndActualConsumption(t *testing.T) {
	index := draftFixture()
	draft, err := DraftSources(index, "docs/owner.md", "compiler")
	if err != nil {
		t.Fatal(err)
	}
	second, err := DraftSources(index, "docs/owner.md", "compiler")
	if err != nil || !bytes.Equal(draft.Markdown, second.Markdown) {
		t.Fatalf("nondeterministic draft: %v", err)
	}
	t.Run("SDD-V0-002 declarations", func(t *testing.T) {
		if len(draft.Entries) != 3 || draft.Entries[2].StartLine != 3 {
			t.Fatalf("wrong exact import anchor: %+v", draft.Entries)
		}
	})
	t.Run("SDD-V0-003 source bindings", func(t *testing.T) {
		if !strings.Contains(string(draft.Markdown), "GENERATED") || strings.Contains(string(draft.Markdown), "Full body omitted") {
			t.Fatal("missing derivation or excessive owner scope")
		}
		result, err := ConsumeDraft(second, draft.Markdown, "Plan")
		if err != nil {
			t.Fatal(err)
		}
		rows := result["results"].([]DraftEntry)
		if len(rows) != 1 || rows[0].Path != "compiler/api.go" || rows[0].Excerpt != "func Plan() {}" || len(rows[0].SHA256) != 64 {
			t.Fatalf("wrong source rederivation: %+v", result)
		}
		if result["behavior"] != "UNKNOWN" {
			t.Fatalf("authority promotion: %+v", result)
		}
	})

	negative, err := ConsumeDraft(draft, draft.Markdown, "CompileOffline")
	if err != nil || negative["state"] != "NO_CANDIDATES" {
		t.Fatalf("negative query invented evidence: %+v %v", negative, err)
	}
}

func TestSourceDraftTamperAndDriftRefuse(t *testing.T) {
	t.Run("SDD-V0-004", func(t *testing.T) {
		index := draftFixture()
		draft, err := DraftSources(index, "docs/owner.md", "compiler")
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range [][]byte{append(append([]byte{}, draft.Markdown...), []byte("forged")...), bytes.Replace(draft.Markdown, []byte(draftPolicy), []byte("source-orientation/1"), 1)} {
			if _, err := ConsumeDraft(draft, input, "Plan"); err == nil {
				t.Fatal("accepted tampered draft")
			}
		}
		changed := index.Sources["compiler/api.go"]
		changed.Data = []byte("package compiler\nfunc Plan() { panic(1) }\n")
		index.Sources[changed.Path] = changed
		fresh, err := DraftSources(index, "docs/owner.md", "compiler")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ConsumeDraft(fresh, draft.Markdown, "Plan"); err == nil {
			t.Fatal("accepted stale original source")
		}
	})
}

// EAF-V0-004 / SDD-V0-002..005: unchanged old blobs do not hide a new consumer.
func TestSourceDraftAddedImporterInvalidatesPriorDraft(t *testing.T) {
	t.Run("EAF-V0-004", func(t *testing.T) {
		index := draftFixture()
		delete(index.Sources, "caller/main.go")
		delete(index.Imports, "caller/main.go")
		prior, err := DraftSources(index, "docs/owner.md", "compiler")
		if err != nil {
			t.Fatal(err)
		}
		old := index.Sources["compiler/api.go"]
		const added = "consumer/new.go"
		index.Sources[added] = contextindex.Source{Path: added, BlobHash: strings.Repeat("d", 40),
			Mode: "100644", Data: []byte("package consumer\nimport \"example.test/m/compiler\"\n")}
		index.Imports[added] = map[string]struct{}{"example.test/m/compiler": {}}
		fresh, err := DraftSources(index, "docs/owner.md", "compiler")
		if err != nil {
			t.Fatal(err)
		}
		if index.Sources[old.Path].BlobHash != old.BlobHash || !bytes.Equal(index.Sources[old.Path].Data, old.Data) {
			t.Fatal("fixture changed existing source")
		}
		found := false
		for _, entry := range fresh.Entries {
			if entry.Path == added && entry.Blob == strings.Repeat("d", 40) && entry.StartLine == 2 {
				found = true
			}
		}
		if !found {
			t.Fatal("fresh derivation omitted the added import witness")
		}
		if _, err := ConsumeDraft(fresh, prior.Markdown, "Plan"); err == nil {
			t.Fatal("old draft survived new importer with all old blobs unchanged")
		}
		index.Unparsed = []contextindex.Unparsed{{Path: added, Facts: "imports", Reason: "unsupported source"}}
		unparsed, err := DraftSources(index, "docs/owner.md", "compiler")
		if err != nil {
			t.Fatal(err)
		}
		if len(unparsed.Entries) != len(prior.Entries) || !bytes.Contains(unparsed.Markdown, []byte("Zero import entries proves no absence")) {
			t.Fatal("unparsed consumer became evidence or disappeared into an absence claim")
		}
		if _, err := ConsumeDraft(unparsed, fresh.Markdown, "Plan"); err == nil {
			t.Fatal("previous import witness survived failed extraction")
		}
	})
}

func TestSourceDraftUnsupportedCoverageAndLimits(t *testing.T) {
	t.Run("SDD-V0-005", func(t *testing.T) {
		index := draftFixture()
		index.Unparsed = []contextindex.Unparsed{{Path: "caller/main.go", Facts: "imports", Reason: "unknown"}}
		draft, err := DraftSources(index, "docs/owner.md", "compiler")
		if err != nil || len(draft.Entries) != 2 {
			t.Fatalf("unparsed importer leaked: %+v %v", draft, err)
		}
		index.Unparsed = append(index.Unparsed, contextindex.Unparsed{Path: "compiler/api.go", Facts: "symbols", Reason: "unknown"})
		if _, err := DraftSources(index, "docs/owner.md", "compiler"); err == nil {
			t.Fatal("unparsed declaration admitted")
		}
		index = draftFixture()
		for _, directory := range []string{"../compiler", "/compiler", "compiler/../compiler", "."} {
			if _, err := DraftSources(index, "docs/owner.md", directory); err == nil {
				t.Fatalf("accepted invalid directory %q", directory)
			}
		}
		for _, text := range []string{"# No digest\n", "## Agent digest\n## Agent digest\n", strings.Repeat("line\n", 64) + "## Agent digest\n"} {
			source := index.Sources["docs/owner.md"]
			source.Data = []byte(text)
			index.Sources[source.Path] = source
			if _, err := DraftSources(index, source.Path, "compiler"); err == nil {
				t.Fatalf("unsupported owner accepted %q", text)
			}
		}
		index = draftFixture()
		index.Symbols = nil
		for n := range 65 {
			index.Symbols = append(index.Symbols, contextindex.Symbol{Path: "compiler/api.go", Name: fmt.Sprintf("Plan%d", n), Kind: "func", Line: 2})
		}
		if _, err := DraftSourcesWith(index, "docs/owner.md", "compiler", DraftOptions{Page: 3}); err == nil || !strings.Contains(err.Error(), "page 3 is outside the draft's pages 1..2") {
			t.Fatalf("out-of-range page admitted: %v", err)
		}
	})
}

func countKind(entries []DraftEntry, kind string) int {
	count := 0
	for _, entry := range entries {
		if entry.Kind == kind {
			count++
		}
	}
	return count
}

// SDD-V0-632: 110 declarations draft over two pages; nothing off a page is invented.
func TestSDDV0632PagesOverSixtyFourEntries(t *testing.T) {
	index := draftFixture()
	index.Symbols = nil
	for n := range 110 {
		index.Symbols = append(index.Symbols, contextindex.Symbol{Path: "compiler/api.go", Name: fmt.Sprintf("Plan%03d", n), Kind: "func", Line: 2})
	}
	seen := map[string]bool{}
	for page, want := range map[int]int{1: 64, 2: 46} {
		draft, err := DraftSourcesWith(index, "docs/owner.md", "compiler", DraftOptions{Page: page})
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if got := countKind(draft.Entries, "declaration"); got != want || draft.Page != page || draft.Pages != 2 {
			t.Fatalf("page %d: %d declarations page %d of %d", page, got, draft.Page, draft.Pages)
		}
		for _, entry := range draft.Entries {
			if entry.Kind == "declaration" {
				if seen[entry.ID] {
					t.Fatalf("declaration %s on two pages", entry.ID)
				}
				seen[entry.ID] = true
			}
		}
		first := (page-1)*64 + 1
		line := fmt.Sprintf("TRUNCATED: declarations %d–%d of 110 on page %d of 2; the other %d entries are UNKNOWN on this page.", first, first+want-1, page, 110-want)
		if !slices.Contains(draft.Limitations, line) || !strings.Contains(string(draft.Markdown), fmt.Sprintf("Page: %d of 2\n", page)) {
			t.Fatalf("page %d missing TRUNCATED/page header: %q", page, draft.Limitations)
		}
	}
	if len(seen) != 110 {
		t.Fatalf("pages covered %d of 110 declarations", len(seen))
	}
	if _, err := DraftSourcesWith(index, "docs/owner.md", "compiler", DraftOptions{Page: 0}); err != nil {
		t.Fatalf("page 0 is page 1: %v", err)
	}
	if _, err := DraftSourcesWith(index, "docs/owner.md", "compiler", DraftOptions{Page: -1}); err == nil {
		t.Fatal("negative page admitted")
	}
	small, err := DraftSources(draftFixture(), "docs/owner.md", "compiler")
	if err != nil || small.Pages != 1 || strings.Contains(string(small.Markdown), "Page: ") {
		t.Fatalf("single-page draft printed a page header: %v", err)
	}
}

// SDD-V0-632: importers page independently of declarations.
func TestSDDV0632ImportersCountSeparately(t *testing.T) {
	index := draftFixture()
	for n := range 70 {
		path := fmt.Sprintf("caller%02d/main.go", n)
		index.Sources[path] = contextindex.Source{Path: path, BlobHash: strings.Repeat("a", 40), Data: []byte("package main\nimport \"example.test/m/compiler\"\n"), Mode: "100644"}
		index.Imports[path] = map[string]struct{}{"example.test/m/compiler": {}}
	}
	draft, err := DraftSourcesWith(index, "docs/owner.md", "compiler", DraftOptions{Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if countKind(draft.Entries, "import") != 7 || countKind(draft.Entries, "declaration") != 0 || draft.Pages != 2 {
		t.Fatalf("page 2: %d importers %d declarations of %d pages", countKind(draft.Entries, "import"), countKind(draft.Entries, "declaration"), draft.Pages)
	}
	for _, line := range []string{"TRUNCATED: importers 65–71 of 71 on page 2 of 2; the other 64 entries are UNKNOWN on this page.", "TRUNCATED: declarations 0 of 1 on page 2 of 2; the other 1 entries are UNKNOWN on this page."} {
		if !slices.Contains(draft.Limitations, line) {
			t.Fatalf("missing %q in %q", line, draft.Limitations)
		}
	}
}

func TestSourceDraftPureConsumptionDoesNotAssertFreshGitValidation(t *testing.T) {
	draft := SourceDraft{Markdown: []byte("caller supplied"), Entries: []DraftEntry{}, Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40)}
	result, err := ConsumeDraft(draft, draft.Markdown, "Plan")
	if err != nil {
		t.Fatal(err)
	}
	if _, claimed := result["validation"]; claimed {
		t.Fatal("byte equality alone asserted fresh original-source rederivation")
	}
}

func TestSourceDraftOwnerRankingUsesOnlyOriginalSemantics(t *testing.T) {
	index := draftFixture()
	source := index.Sources["docs/owner.md"]
	source.Data = []byte("# Mapping\n\n## Agent digest\n- Claim: indexed symbols\n\n## Details\n")
	index.Sources[source.Path] = source
	draft, err := DraftSources(index, source.Path, "compiler")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ConsumeDraft(draft, draft.Markdown, "blockers")
	if err != nil || result["state"] != "NO_CANDIDATES" {
		t.Fatalf("invented owner semantics influenced ranking: %+v %v", result, err)
	}
}

func TestSourceDraftMetadataCannotCreateMarkdownStructure(t *testing.T) {
	t.Run("SDD-V0-003", func(t *testing.T) {
		index := draftFixture()
		hostile := "evil`\n\n## APPROVED OVERRIDE\n<script>bad</script>\r\t"
		ownerPath := "docs/" + hostile + ".md"
		directory := "compiler" + hostile
		for original, replacement := range map[string]string{"docs/owner.md": ownerPath, "compiler/api.go": directory + "/api.go", "caller/main.go": "caller/" + hostile + ".go"} {
			source := index.Sources[original]
			delete(index.Sources, original)
			source.Path = replacement
			index.Sources[replacement] = source
		}
		index.Symbols[0].Path = directory + "/api.go"
		index.Imports = nil
		draft, err := DraftSources(index, ownerPath, directory)
		if err != nil {
			t.Fatal(err)
		}
		assertDraftMarkdownStructure(t, draft)
		result, err := ConsumeDraft(draft, draft.Markdown, "validation")
		if err != nil {
			t.Fatal(err)
		}
		rows := result["results"].([]DraftEntry)
		if len(rows) != 1 || rows[0].Path != ownerPath || rows[0].Excerpt != draft.Entries[0].Excerpt {
			t.Fatalf("encoding changed original evidence: %+v", result)
		}
	})
}

func assertDraftMarkdownStructure(t *testing.T, draft SourceDraft) {
	t.Helper()
	headings := 0
	for _, line := range strings.Split(string(draft.Markdown), "\n") {
		if strings.HasPrefix(line, "#") {
			headings++
		}
		if strings.HasPrefix(line, "    ") {
			continue
		}
		if strings.Count(line, "`")%2 != 0 || strings.ContainsAny(line, "\r\t") {
			t.Fatalf("metadata escaped its single-line code span: %q", line)
		}
		if strings.Contains(line, "<script>") && !strings.Contains(line, "`\"") {
			t.Fatalf("raw HTML escaped metadata: %q", line)
		}
	}
	// SDD-V0-631: one title heading, one heading per cited source file, and one per blobless entry.
	want, files := 1, map[string]bool{}
	for _, entry := range draft.Entries {
		if entry.Blob == "" {
			want++
		} else if !files[entry.Path] {
			files[entry.Path] = true
			want++
		}
	}
	if headings != want {
		t.Fatalf("metadata injected heading: got %d, want %d", headings, want)
	}
}

func TestSourceDraftAgentDigestIgnoresFencedHeadings(t *testing.T) {
	t.Run("SDD-V0-001", func(t *testing.T) {
		for _, fence := range []string{"```", "~~~~", "   ````"} {
			t.Run(fence, func(t *testing.T) {
				index := draftFixture()
				source := index.Sources["docs/owner.md"]
				marker := strings.TrimSpace(fence)
				preamble := "# Owner\n" + fence + "markdown\n## Agent digest\n" + marker + "\n"
				digest := "## Agent digest\n" + fence + "markdown\n## Example\n## Agent digest\n" + marker[:2] + "\n~~~not a close\n" + marker + "not a close\n" + marker + marker[:1] + " \t\n- Blocked on: CriticalUnreviewedCondition\n"
				source.Data = []byte(preamble + digest + "## Details\nFull body omitted.\n")
				index.Sources[source.Path] = source
				draft, err := DraftSources(index, source.Path, "compiler")
				if err != nil {
					t.Fatal(err)
				}
				if draft.Entries[0].Excerpt != strings.TrimSuffix(preamble+digest, "\n") {
					t.Fatalf("digest was not copied exactly: %q", draft.Entries[0].Excerpt)
				}
				result, err := ConsumeDraft(draft, draft.Markdown, "CriticalUnreviewedCondition")
				if err != nil || result["state"] != "READY" {
					t.Fatalf("blocker unavailable: %+v %v", result, err)
				}
			})
		}
	})
}

func TestSourceDraftAgentDigestRejectsUnclosedFence(t *testing.T) {
	t.Run("SDD-V0-001", func(t *testing.T) {
		for _, opener := range []string{"```", "~~~"} {
			index := draftFixture()
			source := index.Sources["docs/owner.md"]
			source.Data = []byte("## Agent digest\n" + opener + "\n## Details\n")
			index.Sources[source.Path] = source
			if _, err := DraftSources(index, source.Path, "compiler"); err == nil || !strings.Contains(err.Error(), "unclosed") {
				t.Fatalf("unclosed fence did not explicitly refuse: %v", err)
			}
		}
	})
}

func TestSourceDraftOriginalRouteRejectsNonObjectBlob(t *testing.T) {
	t.Run("SDD-V0-003", func(t *testing.T) {
		for _, blob := range []string{"-p", "$(touch unwanted)", strings.Repeat("A", 40), strings.Repeat("a", 39)} {
			index := draftFixture()
			source := index.Sources["docs/owner.md"]
			source.BlobHash = blob
			index.Sources[source.Path] = source
			if _, err := DraftSources(index, source.Path, "compiler"); err == nil {
				t.Fatalf("unsafe original route admitted %q", blob)
			}
		}
	})
}

func TestSourceDraftAgentDigestKeepsProseAfterFencedHeading(t *testing.T) {
	t.Run("SDD-V0-001", func(t *testing.T) {
		for _, fence := range []string{"```", "~~~"} {
			t.Run(fence, func(t *testing.T) {
				index := draftFixture()
				source := index.Sources["docs/owner.md"]
				digest := "## Agent digest\n" + fence + "markdown\n## Example\n" + fence + "\n- Blocked on: CriticalUnreviewedCondition\n"
				source.Data = []byte(digest + "## Details\nOmitted full body.\n")
				index.Sources[source.Path] = source
				draft, err := DraftSources(index, source.Path, "compiler")
				if err != nil {
					t.Fatal(err)
				}
				if draft.Entries[0].Excerpt != strings.TrimSuffix(digest, "\n") {
					t.Errorf("digest truncated: %q", draft.Entries[0].Excerpt)
				}
				result, err := ConsumeDraft(draft, draft.Markdown, "CriticalUnreviewedCondition")
				if err != nil || result["state"] != "READY" {
					t.Fatalf("post-fence blocker unavailable: %+v %v", result, err)
				}
			})
		}
	})
}

// TestSDDV0018DraftLanguagesExportRules covers every language the native index
// extracts declarations from: each drafts its exported names, omits its
// unexported names and its test files, and a Go-only package keeps the
// Go-only limitations (V1-3072).
func TestSDDV0018DraftLanguagesExportRules(t *testing.T) {
	for _, test := range []struct {
		file, text, exported, hidden, test, language string
	}{
		{"m/api.go", "package m\nfunc Plan() {}\nfunc plan() {}\n", "Plan", "plan", "m/api_test.go", ""},
		{"m/api.py", "def plan():\n    pass\ndef _plan():\n    pass\n", "plan", "_plan", "m/test_api.py", "Python"},
		{"m/api.ts", "export function plan() {}\nfunction hidden() {}\n", "plan", "hidden", "m/api.spec.ts", "TypeScript"},
		{"m/api.mjs", "export const plan = 1\nconst hidden = 2\n", "plan", "hidden", "m/api.test.mjs", "JavaScript"},
		{"m/api.rs", "pub fn plan() {}\npub(crate) fn hidden() {}\n", "plan", "hidden", "m/api_test.rs", "Rust"},
		{"m/Api.cs", "public class Plan {}\ninternal class Hidden {}\n", "Plan", "Hidden", "m/ApiTests.cs", "C#"},
		{"m/Api.swift", "public func plan() {}\nfunc hidden() {}\n", "plan", "hidden", "m/ApiTests.swift", "Swift"},
		{"m/Api.kt", "fun plan() {}\nprivate fun hidden() {}\n", "plan", "hidden", "m/ApiTest.kt", "Kotlin"},
		{"m/api.rb", "def plan\nend\ndef _hidden\nend\n", "plan", "_hidden", "m/api_spec.rb", "Ruby"},
	} {
		t.Run(test.file, func(t *testing.T) {
			index := draftFixture()
			hash := strings.Repeat("a", 40)
			index.Tracked = map[string]struct{}{"docs/owner.md": {}, test.file: {}, test.test: {}}
			for _, name := range []string{test.file, test.test} {
				index.Sources[name] = contextindex.Source{Path: name, BlobHash: hash, Data: []byte(test.text), Mode: "100644"}
			}
			lines := strings.Split(test.text, "\n")
			for _, name := range []string{test.file, test.test} {
				for number, line := range lines {
					for _, symbol := range []string{test.exported, test.hidden} {
						if strings.Contains(line, " "+symbol) {
							index.Symbols = append(index.Symbols, contextindex.Symbol{Path: name, BlobHash: hash, Name: symbol, Kind: "func", Line: number + 1})
						}
					}
				}
			}
			draft, err := DraftSources(index, "docs/owner.md", "m")
			if err != nil {
				t.Fatal(err)
			}
			if len(draft.Entries) != 2 || draft.Entries[1].Title != "func "+test.exported || draft.Entries[1].Path != test.file {
				t.Fatalf("entries: %+v", draft.Entries)
			}
			drafted := strings.Contains(string(draft.Markdown), "Non-Go languages drafted: "+test.language+".")
			if drafted != (test.language != "") {
				t.Fatalf("language limitation for %q:\n%s", test.language, draft.Markdown)
			}
		})
	}
	t.Run("Go package beside cgo and assembly keeps Go-only bytes", func(t *testing.T) {
		index := draftFixture()
		want, err := DraftSources(index, "docs/owner.md", "compiler")
		if err != nil {
			t.Fatal(err)
		}
		index.Tracked = map[string]struct{}{"compiler/api.go": {}, "compiler/shim.c": {}, "compiler/shim.h": {}, "compiler/asm_amd64.s": {}, "compiler/testdata": {}}
		got, err := DraftSources(index, "docs/owner.md", "compiler")
		if err != nil || !bytes.Equal(got.Markdown, want.Markdown) {
			t.Fatalf("Go-only draft bytes changed (%v):\n%s", err, got.Markdown)
		}
	})
	t.Run("unsupported language named", func(t *testing.T) {
		index := draftFixture()
		index.Tracked = map[string]struct{}{"lib/Main.java": {}, "lib/run.zig": {}, "lib/README.md": {}}
		_, err := DraftSources(index, "docs/owner.md", "lib")
		if err == nil || err.Error() != "unsupported-documentation-source: unsupported language: the native index extracts no declarations from Java, .zig" {
			t.Fatalf("err = %v", err)
		}
	})
}

// TestSDDV0018VisibilityReadsOnlyTheDeclarationPrefix: a modifier word in a
// body, string literal or comment never decides visibility (V1-3074).
func TestSDDV0018VisibilityReadsOnlyTheDeclarationPrefix(t *testing.T) {
	for _, test := range []struct {
		suffix, name, line string
		exported           bool
	}{
		{".kt", "mode", `fun mode() = "private"`, true},
		{".kt", "mode", `fun mode() {} // internal`, true},
		{".kt", "mode", `fun mode() = 1 /* protected */`, true},
		{".kt", "hidden", `private fun hidden() = "public"`, false},
		{".kt", "Box.mode", `    internal fun mode() = 0`, false},
		{".cs", "Mode", `void Mode() { Log("public"); }`, false},
		{".cs", "Count", `internal int Count; // public`, false},
		{".cs", "Mode", `[Doc("public")] void Mode() {}`, false},
		{".cs", "Mode", `public void Mode() { Log("private"); }`, true},
		{".cs", "Api.Mode", `    public static void Mode() {}`, true},
		{".swift", "mode", `func mode() -> String { "public" }`, false},
		{".swift", "mode", `func mode() /* open */ {}`, false},
		{".swift", "mode", `public func mode() -> String { "private" }`, true},
		{".swift", "Mode", `open class Mode {}`, true},
		// V1-3077: a same-name attribute, a C# verbatim string ending in a
		// backslash, and Kotlin's nested block comments.
		{".cs", "Mode", `[Mode] public void Mode() {}`, true},
		{".cs", "Mode", `[Mode] void Mode() {}`, false},
		{".cs", "Mode", `[Doc(@"C:\")] public void Mode() {}`, true},
		{".cs", "Mode", `[Doc(@"say ""public""")] void Mode() {}`, false},
		{".kt", "mode", `/* outer /* inner */ private */ fun mode() {}`, true},
		{".kt", "mode", `/* outer /* inner */ note */ private fun mode() {}`, false},
		{".swift", "mode", `/* outer /* inner */ public */ func mode() {}`, false},
		{".cs", "Mode", `/* outer /* inner */ public void Mode() {}`, true},
	} {
		if got := draftLanguages[test.suffix].exported(test.name, test.line); got != test.exported {
			t.Errorf("%s %q exported = %v, want %v", test.suffix, test.line, got, test.exported)
		}
	}
	t.Run("Kotlin module whose only declaration returns \"private\"", func(t *testing.T) {
		index := draftFixture()
		hash := strings.Repeat("a", 40)
		index.Tracked = map[string]struct{}{"docs/owner.md": {}, "kt/Mode.kt": {}}
		index.Sources["kt/Mode.kt"] = contextindex.Source{Path: "kt/Mode.kt", BlobHash: hash, Data: []byte("fun mode() = \"private\"\n"), Mode: "100644"}
		index.Symbols = append(index.Symbols, contextindex.Symbol{Path: "kt/Mode.kt", BlobHash: hash, Name: "mode", Kind: "func", Line: 1})
		draft, err := DraftSources(index, "docs/owner.md", "kt")
		if err != nil || len(draft.Entries) != 2 || draft.Entries[1].Title != "func mode" {
			t.Fatalf("draft = %+v, err = %v", draft.Entries, err)
		}
	})
}

// TestSDDV0019DryRunDraftIsMarkedAndNeverConsumed: an untracked or inline
// owner source drafts only under DraftDryRun, the page says NOT_ADMISSIBLE and
// cites no Git blob for the owner, and ConsumeDraft refuses both the dry-run
// draft and its bytes; DraftSources still refuses the untracked owner.
func TestSDDV0019DryRunDraftIsMarkedAndNeverConsumed(t *testing.T) {
	index := draftFixture()
	owner := []byte("# New\n\n## Agent digest\n- Claim: first draft\n\n## Requirements\nOmitted.\n")
	if _, err := DraftSources(index, "docs/new.md", "compiler"); err == nil || !strings.Contains(err.Error(), "admitted tracked Markdown") {
		t.Fatalf("normal path accepted an untracked owner: %v", err)
	}
	draft, err := DraftDryRun(index, "docs/new.md", owner, "compiler")
	if err != nil {
		t.Fatal(err)
	}
	markdown := string(draft.Markdown)
	for _, want := range []string{"\nProfile: corvint-source-documentation-draft-dry-run/0\nAdmissibility: NOT_ADMISSIBLE\nAuthority: NONE\n", "DRY RUN: NOT_ADMISSIBLE", "no Git blob", "    - Claim: first draft"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("dry-run draft lacks %q:\n%s", want, markdown)
		}
	}
	if !draft.DryRun || draft.Entries[0].Kind != "dry-run-owner" || draft.Entries[0].Blob != "" || strings.Contains(markdown, "Omitted.") || strings.Count(markdown, "git --no-pager --no-replace-objects show") != len(draft.Entries)-1 {
		t.Fatalf("dry-run owner entry: %+v", draft.Entries[0])
	}
	if !IsDryRunDraft(draft.Markdown) {
		t.Fatal("dry-run bytes not recognised")
	}
	admitted, err := DraftSources(index, "docs/owner.md", "compiler")
	if err != nil || IsDryRunDraft(admitted.Markdown) {
		t.Fatalf("admitted draft: %v", err)
	}
	for _, attempt := range []struct {
		draft    SourceDraft
		provided []byte
	}{{draft, draft.Markdown}, {admitted, draft.Markdown}} {
		if _, err := ConsumeDraft(attempt.draft, attempt.provided, "Plan"); err == nil || !strings.Contains(err.Error(), "dry-run draft is not admissible") {
			t.Fatalf("consume accepted a dry-run draft: %v", err)
		}
	}
	if _, err := DraftDryRun(index, "docs/new.txt", owner, "compiler"); err == nil {
		t.Fatal("dry run accepted a non-Markdown owner path")
	}
	if _, err := DraftDryRun(index, "docs/new.md", bytes.Repeat([]byte("x"), DraftMaxBytes+1), "compiler"); err == nil || !strings.Contains(err.Error(), "64 KiB") {
		t.Fatalf("oversized dry-run owner: %v", err)
	}
}

// TestSDDV0019ReadDryRunOwnerIsConfined: the working-tree owner read refuses
// non-Markdown, Git metadata, symlinked and oversized paths, and writes nothing.
func TestSDDV0019ReadDryRunOwnerIsConfined(t *testing.T) {
	root := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/new.md", []byte("owner\n"))
	write(".git/x.md", []byte("meta\n"))
	write("big.md", bytes.Repeat([]byte("x"), DraftMaxBytes+1))
	if err := os.Symlink(filepath.Join(root, "docs/new.md"), filepath.Join(root, "link.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if data, err := ReadDryRunOwner(root, "docs/new.md"); err != nil || string(data) != "owner\n" {
		t.Fatalf("read: %q %v", data, err)
	}
	for name, code := range map[string]string{"docs/new.txt": "unsupported-documentation-source", ".git/x.md": "unsupported-documentation-source", "link.md": "unsupported-documentation-source", "missing.md": "unsupported-documentation-source", "big.md": "documentation-limit-exceeded"} {
		var problem *Error
		if _, err := ReadDryRunOwner(root, name); !errors.As(err, &problem) || problem.Code != code {
			t.Fatalf("%s: %v, want %s", name, err, code)
		}
	}
}

// SDD-V0-630 (V1-3719): an owner whose excerpt does not name the package is
// labelled OWNER_UNRELATED; naming it as a whole path token is NAMES_PACKAGE.
func TestSDDV0630OwnerRelationLabelsUnrelatedOwner(t *testing.T) {
	for _, tc := range []struct {
		name, digest, want string
	}{
		{"unrelated", "- Claim: drafts for internal/docviews", OwnerUnrelated}, //corvint:fixture-path
		{"prefix-only", "- Implementation: `compilerx/api.go`", OwnerUnrelated},
		{"suffix-of-longer-path", "- Implementation: `internal/compiler/api.go`", OwnerUnrelated},
		{"file-under-package", "- Implementation: `compiler/api.go`", OwnerNamesPackage},
		{"bare-directory", "- Package: compiler.", OwnerNamesPackage},
		{"sibling-file", "- Implementation: `compiler.go`", OwnerUnrelated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := draftFixture()
			source := index.Sources["docs/owner.md"]
			source.Data = []byte("# Owner\n\n## Agent digest\n" + tc.digest + "\n")
			index.Sources[source.Path] = source
			draft, err := DraftSources(index, source.Path, "compiler")
			if err != nil {
				t.Fatal(err)
			}
			if draft.OwnerRelation != tc.want {
				t.Fatalf("relation = %q, want %q", draft.OwnerRelation, tc.want)
			}
			if !strings.Contains(string(draft.Markdown), "\nOwner relation: "+tc.want+"\n") {
				t.Fatalf("draft missing relation line:\n%s", draft.Markdown)
			}
			labelled := slices.Contains(draft.Limitations, ownerUnrelatedLimitation)
			if labelled != (tc.want == OwnerUnrelated) {
				t.Fatalf("unrelated limitation present = %t for %s", labelled, tc.want)
			}
		})
	}
}

// richFixture is one Go package with a package doc, a documented struct, an
// interface, a grouped constant block and a function with a multi-line doc.
func richFixture() *contextindex.Index {
	index := draftFixture()
	api := "// Package compiler plans offline builds.\npackage compiler\n\n// Plan builds a plan.\n// It never runs a build.\nfunc Plan() {\n\treturn\n}\n\n// Options configures Plan.\ntype Options struct {\n\t// Name is the target.\n\tName string\n\tlimit int\n}\n\n// Runner runs plans.\ntype Runner interface {\n\tRun() error\n}\n\nconst (\n\t// Fast is quick.\n\tFast = \"fast\"\n\tSlow = \"slow\"\n)\n"
	extra := "package compiler\n\n// Extra is a second-file declaration.\nvar Extra = 1\n"
	index.Sources["compiler/api.go"] = contextindex.Source{Path: "compiler/api.go", BlobHash: strings.Repeat("a", 40), Data: []byte(api), Mode: "100644"}
	index.Sources["compiler/extra.go"] = contextindex.Source{Path: "compiler/extra.go", BlobHash: strings.Repeat("d", 40), Data: []byte(extra), Mode: "100644"}
	index.Symbols = []contextindex.Symbol{
		{Path: "compiler/api.go", Name: "Plan", Kind: "func", Line: 6},
		{Path: "compiler/api.go", Name: "Options", Kind: "type", Line: 11},
		{Path: "compiler/api.go", Name: "Runner", Kind: "type", Line: 18},
		{Path: "compiler/api.go", Name: "Fast", Kind: "const", Line: 24},
		{Path: "compiler/api.go", Name: "Slow", Kind: "const", Line: 25},
		{Path: "compiler/extra.go", Name: "Extra", Kind: "var", Line: 4},
	}
	return index
}

// SDD-V0-631 (V1-3717): Go entries carry doc comments, struct fields,
// interface methods and the package doc, each as an exact source span.
func TestSDDV0631DeclarationsCarryDocCommentsAndFields(t *testing.T) {
	draft, err := DraftSources(richFixture(), "docs/owner.md", "compiler")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		start, end int
		excerpt    string
	}{
		"package compiler": {1, 2, "// Package compiler plans offline builds.\npackage compiler"},
		"func Plan":        {4, 6, "// Plan builds a plan.\n// It never runs a build.\nfunc Plan() {"},
		"type Options":     {10, 15, "// Options configures Plan.\ntype Options struct {\n\t// Name is the target.\n\tName string\n\tlimit int\n}"},
		"type Runner":      {17, 20, "// Runner runs plans.\ntype Runner interface {\n\tRun() error\n}"},
		"const Fast":       {23, 24, "\t// Fast is quick.\n\tFast = \"fast\""},
		"const Slow":       {25, 25, "\tSlow = \"slow\""},
		"var Extra":        {3, 4, "// Extra is a second-file declaration.\nvar Extra = 1"},
	}
	seen := 0
	for _, entry := range draft.Entries {
		expected, ok := want[entry.Title]
		if !ok {
			continue
		}
		seen++
		if entry.StartLine != expected.start || entry.EndLine != expected.end || entry.Excerpt != expected.excerpt {
			t.Errorf("%s = %d–%d %q, want %d–%d %q", entry.Title, entry.StartLine, entry.EndLine, entry.Excerpt, expected.start, expected.end, expected.excerpt)
		}
		if entry.Title == "package compiler" && entry.Kind != "package-doc" {
			t.Errorf("package doc kind = %q", entry.Kind)
		}
	}
	if seen != len(want) {
		t.Fatalf("saw %d of %d expected entries: %+v", seen, len(want), draft.Entries)
	}
	assertDraftMarkdownStructure(t, draft)
	result, err := ConsumeDraft(draft, draft.Markdown, "target name")
	if err != nil {
		t.Fatal(err)
	}
	if rows := result["results"].([]DraftEntry); len(rows) == 0 || rows[0].Title != "type Options" {
		t.Fatalf("field doc comment not consumable: %+v", rows)
	}
}

// SDD-V0-631 (V1-3717): blob, sha256 and the immutable route print once per
// file, however many entries cite that file.
func TestSDDV0631ProvenanceOncePerFile(t *testing.T) {
	draft, err := DraftSources(richFixture(), "docs/owner.md", "compiler")
	if err != nil {
		t.Fatal(err)
	}
	markdown := string(draft.Markdown)
	files := map[string]DraftEntry{}
	for _, entry := range draft.Entries {
		files[entry.Path] = entry
	}
	if len(files) != 4 {
		t.Fatalf("fixture cites %d files, want 4", len(files))
	}
	if got := strings.Count(markdown, "git --no-pager --no-replace-objects show "); got != len(files) {
		t.Fatalf("route printed %d times, want once per file (%d)", got, len(files))
	}
	for sourcePath, entry := range files {
		if got := strings.Count(markdown, "sha256 `\""+entry.SHA256+"\"`"); got != 1 {
			t.Errorf("%s sha256 printed %d times", sourcePath, got)
		}
		if got := strings.Count(markdown, "`"+fmt.Sprintf("%q", sourcePath)+"`\n\nBlob "); got != 1 {
			t.Errorf("%s path printed %d times", sourcePath, got)
		}
	}
}

// SDD-V0-633: a foreign owner drafts from a named section without any Agent digest.
func TestSDDV0633OwnerSectionDraftsWithoutAgentDigest(t *testing.T) {
	const owner = "# Upstream\n\nIntro prose that stays out.\n\n## Overview\nThe compiler package plans.\n\n## Install\nnot excerpted\n"
	index := draftFixture()
	source := index.Sources["docs/owner.md"]
	source.Data = []byte(owner)
	index.Sources[source.Path] = source
	if _, err := DraftSources(index, source.Path, "compiler"); err == nil {
		t.Fatal("default heading admitted an owner without an Agent digest")
	}
	for _, dryRun := range []bool{false, true} {
		var draft SourceDraft
		var err error
		if dryRun {
			draft, err = DraftDryRunWith(index, source.Path, source.Data, "compiler", DraftOptions{OwnerSection: "Overview"})
		} else {
			draft, err = DraftSourcesWith(index, source.Path, "compiler", DraftOptions{OwnerSection: "Overview"})
		}
		if err != nil {
			t.Fatalf("dry-run=%t: %v", dryRun, err)
		}
		entry := draft.Entries[0]
		if entry.StartLine != 5 || entry.EndLine != 7 || entry.Excerpt != "## Overview\nThe compiler package plans.\n" || entry.Title != "Overview" {
			t.Fatalf("dry-run=%t owner entry %+v", dryRun, entry)
		}
		markdown := string(draft.Markdown)
		if !strings.Contains(markdown, "\nOwner section: `\"Overview\"`\n") || strings.Contains(markdown, "Intro prose") || strings.Contains(markdown, "not excerpted") {
			t.Fatalf("dry-run=%t section draft:\n%s", dryRun, markdown)
		}
	}
	plain, err := DraftSources(draftFixture(), "docs/owner.md", "compiler")
	named, namedErr := DraftSourcesWith(draftFixture(), "docs/owner.md", "compiler", DraftOptions{OwnerSection: "Agent digest"})
	if err != nil || namedErr != nil || !bytes.Equal(plain.Markdown, named.Markdown) || strings.Contains(string(plain.Markdown), "Owner section:") {
		t.Fatalf("default section changed bytes: %v %v", err, namedErr)
	}
	for _, section := range []string{"Missing", "Install\n", " Overview", strings.Repeat("x", 201), "Tab\there"} {
		if _, err := DraftSourcesWith(index, source.Path, "compiler", DraftOptions{OwnerSection: section}); err == nil {
			t.Fatalf("section %q admitted", section)
		}
	}
	source.Data = []byte("## Overview\n" + strings.Repeat("line\n", 64) + "## Next\n")
	index.Sources[source.Path] = source
	if _, err := DraftSourcesWith(index, source.Path, "compiler", DraftOptions{OwnerSection: "Overview"}); err == nil || !strings.Contains(err.Error(), "owner section exceeds the limit of 64 lines") {
		t.Fatalf("65-line section admitted: %v", err)
	}
}
