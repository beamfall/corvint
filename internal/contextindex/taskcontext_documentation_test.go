package contextindex

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

// documentationFixture builds code files and documentation files that carry
// the same two task words; the documentation bodies are shorter, so BM25
// ranks every documentation file above every code file, and ties inside a
// class fall to the path.
func documentationFixture(t *testing.T, code, documentation int) *Index {
	t.Helper()
	return documentationFixtureWith(t, code, documentation, nil)
}

// documentationFixtureWith adds files that carry no task word, such as a
// governing instruction file, to the documentation fixture.
func documentationFixtureWith(t *testing.T, code, documentation int, extra map[string]string) *Index {
	t.Helper()
	files := map[string]string{"go.mod": "module example.test/share\n\ngo 1.27.0\n"}
	for index := range code {
		files[fmt.Sprintf("code/%02d.go", index+1)] = "package code\n\n// needle signal\n"
	}
	for index := range documentation {
		files[fmt.Sprintf("docs/%c.md", 'a'+index)] = "needle signal\n"
	}
	for path, body := range extra {
		files[path] = body
	}
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func contextUncertainty(t *testing.T, packet map[string]any) []string {
	t.Helper()
	lines := make([]string, 0)
	for _, line := range anySlice(contextCoverage(t, packet)["uncertainty"]) {
		lines = append(lines, line.(string))
	}
	return lines
}

// TestTaskContextDocumentationCompetesByStrength is TCP-V0-013 as amended by
// TCP-V0-059 and V1-0859's reproduction: with the limit binding, four
// documentation rows that outscore the sixth code row are all carried after
// the five-row head. The two-row quota this replaces admitted two of them and
// spent the remaining positions on the weaker code rows.
func TestTaskContextDocumentationCompetesByStrength(t *testing.T) {
	t.Run("TCP-V0-013 TCP-V0-059", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), documentationFixture(t, 12, 4), "needle signal", "", 10)
		if err != nil {
			t.Fatal(err)
		}
		got := contextPairs(t, packet)
		want := []string{
			"lexical code/01.go", "lexical code/02.go", "lexical code/03.go", "lexical code/04.go", "lexical code/05.go",
			"documentation docs/a.md", "documentation docs/b.md", "documentation docs/c.md", "documentation docs/d.md",
			"lexical code/06.go",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
		strengths := lexicalStrengths(t, packet)
		if strengths[5] <= strengths[9] {
			t.Fatalf("fixture must rank documentation above the sixth code row: %v", strengths)
		}
		lines := contextUncertainty(t, packet)
		if !slices.Equal(lines, []string{"6 code and 0 documentation rows the task matched lexically are omitted by the result limit 10"}) {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
	})
}

// TestTaskContextDocumentationShareStatesTheOmittedClass is TCP-V0-059 and
// TCP-V0-061: documentation never takes more than half of the positions code
// hits compete for, the code head shrinks to the half left for code, and the
// documentation rows the share (not their strength, the head or the limit)
// omitted are stated as uncertainty.
func TestTaskContextDocumentationShareStatesTheOmittedClass(t *testing.T) {
	t.Run("TCP-V0-059 TCP-V0-061", func(t *testing.T) {
		// Six code rows and eight stronger documentation rows at limit 12: the
		// head is five, the share six, and the twelfth position goes to the
		// sixth code row in merged order although docs/g.md and docs/h.md
		// outscore it; without the share docs/g.md would hold it.
		packet, err := TaskContext(context.Background(), documentationFixture(t, 6, 8), "needle signal", "", 12)
		if err != nil {
			t.Fatal(err)
		}
		got := contextPairs(t, packet)
		want := []string{
			"lexical code/01.go", "lexical code/02.go", "lexical code/03.go", "lexical code/04.go", "lexical code/05.go",
			"documentation docs/a.md", "documentation docs/b.md", "documentation docs/c.md",
			"documentation docs/d.md", "documentation docs/e.md", "documentation docs/f.md",
			"lexical code/06.go",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
		lines := contextUncertainty(t, packet)
		if len(lines) != 2 || lines[0] != "0 code and 2 documentation rows the task matched lexically are omitted by the result limit 12" ||
			!strings.HasPrefix(lines[1], "2 documentation rows that outscore a carried code row are omitted by the documentation share (6 of 12 lexical positions); the strongest is `docs/g.md` (bm25 ") {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
		if coverage := contextCoverage(t, packet); coverage["budget_shortage"] != "slots" {
			t.Fatalf("budget_shortage = %v, want slots", coverage["budget_shortage"])
		}
	})
	t.Run("TCP-V0-061 documentation the head displaced is omitted by the limit, not the share", func(t *testing.T) {
		// Three code rows and six stronger documentation rows at limit 6: the
		// head takes three positions and the share the other three, so the
		// merged order carries no code row the deferred documentation lost a
		// position to; removing the share would carry the same packet.
		packet, err := TaskContext(context.Background(), documentationFixture(t, 3, 6), "needle signal", "", 6)
		if err != nil {
			t.Fatal(err)
		}
		got := contextPairs(t, packet)
		want := []string{
			"lexical code/01.go", "lexical code/02.go", "lexical code/03.go",
			"documentation docs/a.md", "documentation docs/b.md", "documentation docs/c.md",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
		lines := contextUncertainty(t, packet)
		if !slices.Equal(lines, []string{"0 code and 3 documentation rows the task matched lexically are omitted by the result limit 6"}) {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
	})
	t.Run("TCP-V0-059 a code head shorter than five yields no position to deferred documentation", func(t *testing.T) {
		// One code row and six documentation rows at limit 4: the head is one,
		// the share two, and the fourth position goes to a deferred
		// documentation row because no code hit remains to take it; the three
		// documentation rows left out are omitted by the limit alone.
		packet, err := TaskContext(context.Background(), documentationFixture(t, 1, 6), "needle signal", "", 4)
		if err != nil {
			t.Fatal(err)
		}
		got := contextPairs(t, packet)
		want := []string{"lexical code/01.go", "documentation docs/a.md", "documentation docs/b.md", "documentation docs/c.md"}
		if !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
		lines := contextUncertainty(t, packet)
		if !slices.Equal(lines, []string{"0 code and 3 documentation rows the task matched lexically are omitted by the result limit 4"}) {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
	})
}

// TestTaskContextLexicalScoreCarriesStrength is TCP-V0-060 (V1-0431's first
// criterion): a lexical or documentation row's score is 300 plus its BM25
// share of the band up to 599, the strongest hit scoring 599, so the packet
// order and the score agree on strength and no lexical row reaches a
// relation tier.
func TestTaskContextLexicalScoreCarriesStrength(t *testing.T) {
	t.Run("TCP-V0-060", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), documentationFixture(t, 3, 2), "needle signal", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		rows := mapsFromAny(packet["results"])
		hits := newTaskContextCompiler(documentationFixture(t, 3, 2), "needle signal", "").lexicalHits()
		exact := make(map[string]float64, len(hits))
		for _, hit := range hits {
			exact[hit.path] = hit.score
		}
		for _, row := range rows {
			want := contextLexicalBase + int(math.Round(299*exact[row["id"].(string)]/hits[0].score))
			if contextIntValue(row["score"]) != want || want > contextLexicalCeiling {
				t.Fatalf("%s %s score = %v, want %d from bm25 %.4f of %.4f", row["kind"], row["id"], row["score"], want, exact[row["id"].(string)], hits[0].score)
			}
		}
		if rows[3]["kind"] != "documentation" || contextIntValue(rows[3]["score"]) != contextLexicalCeiling || contextIntValue(rows[0]["score"]) >= contextLexicalCeiling {
			t.Fatalf("the strongest documentation row must score the ceiling above the code head: %v", contextPairs(t, packet))
		}
		if lexicalScore(0, 1) != contextLexicalBase || lexicalScore(1, 0) != contextLexicalBase || lexicalScore(2, 1) != contextLexicalCeiling {
			t.Fatalf("score band = %d / %d / %d", lexicalScore(0, 1), lexicalScore(1, 0), lexicalScore(2, 1))
		}
	})
}

// TestTaskContextLexicalFillCountsOnlyOpenPositions is TCP-V0-059's fill: a
// hit an earlier slot admitted or a reservation holds takes no share, head or
// fill position, and the reservations reserve prepends are not positions the
// fill can spend; and TCP-V0-061 under TCP-V0-016: hits the verdict withheld
// are stated as withheld, not as omitted by the limit.
func TestTaskContextLexicalFillCountsOnlyOpenPositions(t *testing.T) {
	t.Run("TCP-V0-059 an earlier documentation relation does not spend the share", func(t *testing.T) {
		// docs/a.md is a mentioned row before the fill: the fill of five holds
		// a share of three for docs/b.md to docs/d.md and a head of two, so the
		// third code row, not docs/d.md, is the row the limit omits; no share
		// line, since the fill carried no code row past the head.
		packet, err := TaskContext(context.Background(), documentationFixture(t, 3, 6), "needle signal in docs/a.md", "", 6)
		if err != nil {
			t.Fatal(err)
		}
		got := contextPairs(t, packet)
		want := []string{
			"mentioned docs/a.md", "lexical code/01.go", "lexical code/02.go",
			"documentation docs/b.md", "documentation docs/c.md", "documentation docs/d.md",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
		lines := contextUncertainty(t, packet)
		if !slices.Equal(lines, []string{"1 code and 2 documentation rows the task matched lexically are omitted by the result limit 6"}) {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
	})
	t.Run("TCP-V0-059 a governing reservation is not a fill position", func(t *testing.T) {
		// AGENTS.md carries no task word and is prepended by reserve: at limit
		// 2 the fill is one position, which the stronger documentation hit
		// takes; a fill of two would have given it to the code head.
		fixture := documentationFixtureWith(t, 1, 1, map[string]string{"AGENTS.md": "# Rules\n\nBe brief.\n"})
		packet, err := TaskContext(context.Background(), fixture, "needle signal", "", 2)
		if err != nil {
			t.Fatal(err)
		}
		got := contextPairs(t, packet)
		if want := []string{"governing AGENTS.md", "documentation docs/a.md"}; !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
		lines := contextUncertainty(t, packet)
		if !slices.Equal(lines, []string{"1 code and 0 documentation rows the task matched lexically are omitted by the result limit 2"}) {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
	})
	t.Run("TCP-V0-061 TCP-V0-016 withheld hits are not omitted by the limit", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), documentationFixture(t, 3, 2), "needle signal `absentOne` and `absentTwo`", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		answer := contextCoverage(t, packet)["answerability"].(map[string]any)
		if packet["state"] != "NO_CANDIDATES" || answer["verdict"] != "unsupported-conjunction" {
			t.Fatalf("state = %v, verdict = %v, want a withheld packet", packet["state"], answer["verdict"])
		}
		lines := contextUncertainty(t, packet)
		if !slices.Equal(lines, []string{"3 code and 2 documentation rows the task matched lexically are withheld by the `unsupported-conjunction` verdict, not by the result limit 20"}) {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
	})
}
