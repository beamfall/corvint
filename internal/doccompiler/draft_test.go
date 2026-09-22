package doccompiler

import (
	"bytes"
	"fmt"
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
		if _, err := DraftSources(index, "docs/owner.md", "compiler"); err == nil {
			t.Fatal("truncated declaration limit")
		}
	})
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
	if headings != len(draft.Entries)+1 {
		t.Fatalf("metadata injected heading: got %d, want %d", headings, len(draft.Entries)+1)
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
