package contextindex

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// recordDataFixture builds one strong code file, five weaker code files and
// record-data files whose BM25 strength lies between them: each body carries
// the two task words, and a longer body ranks lower. Ties inside a class fall
// to the path.
func recordDataFixture(t *testing.T, data int, extra map[string]string) *Index {
	t.Helper()
	files := map[string]string{
		"go.mod":     "module example.test/records\n\ngo 1.27.0\n",
		"code/01.go": "package code\n\n// needle signal\n",
	}
	for index := 2; index <= 6; index++ {
		files[fmt.Sprintf("code/%02d.go", index)] = "package code\n\n// needle signal alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu\n"
	}
	for index := range data {
		files[fmt.Sprintf("data/%c.json", 'a'+index)] = "{\"needle\": \"signal\", \"one\": \"two three four\"}\n"
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

func recordDataPacket(t *testing.T, index *Index, task string, limit int) (map[string]any, []string, []string) {
	t.Helper()
	packet, err := TaskContext(context.Background(), index, task, "", limit)
	if err != nil {
		t.Fatal(err)
	}
	return packet, contextPairs(t, packet), contextUncertainty(t, packet)
}

// TestTaskContextRecordDataIsGatedLikeDocumentation is TCP-V0-063 and the
// record-data flood V1-0859's survey found: record-data hits that outscore
// every code hit but the strongest take no head position, two of them take a
// merged position, and the rest follow every code hit, stated as omitted by
// the gate rather than by their strength. Before the gate data/a.json to
// data/c.json took three of the four head positions at limit 8 and the
// weaker code rows were omitted.
func TestTaskContextRecordDataIsGatedLikeDocumentation(t *testing.T) {
	t.Run("TCP-V0-063 a record-data flood does not displace weaker code", func(t *testing.T) {
		packet, got, lines := recordDataPacket(t, recordDataFixture(t, 6, nil), "needle signal", 8)
		want := []string{
			"lexical code/01.go", "lexical code/02.go", "lexical code/03.go", "lexical code/04.go",
			"lexical data/a.json", "lexical data/b.json", "lexical code/05.go", "lexical code/06.go",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
		strengths := lexicalStrengths(t, packet)
		if !(strengths[0] > strengths[4] && strengths[4] > strengths[7]) {
			t.Fatalf("fixture must rank record data between the strongest and the other code rows: %v", strengths)
		}
		if len(lines) != 2 || lines[0] != "4 code and 0 documentation rows the task matched lexically are omitted by the result limit 8" ||
			!strings.HasPrefix(lines[1], "4 record-data rows that outscore a carried code row are omitted by the record-data gate (2 admitted below the strongest code row); the strongest is `data/c.json` (bm25 ") {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
	})
	t.Run("TCP-V0-063 a record-data hit that outscores every code hit keeps its position and spends no quota", func(t *testing.T) {
		packet, got, _ := recordDataPacket(t, recordDataFixture(t, 4, map[string]string{"data/lead.json": "{\"needle\": \"signal\"}\n"}), "needle signal", 8)
		want := []string{
			"lexical code/01.go", "lexical code/02.go", "lexical code/03.go", "lexical code/04.go",
			"lexical data/lead.json", "lexical data/a.json", "lexical data/b.json", "lexical code/05.go",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
		if strengths := lexicalStrengths(t, packet); strengths[4] <= strengths[0] {
			t.Fatalf("fixture must rank data/lead.json above every code row: %v", strengths)
		}
	})
	t.Run("TCP-V0-063 a deferred record-data row follows the deferred documentation", func(t *testing.T) {
		_, got, _ := recordDataPacket(t, recordDataFixture(t, 4, map[string]string{
			"docs/a.md": "needle signal one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen\n",
			"docs/b.md": "needle signal one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen\n",
			"docs/c.md": "needle signal one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen\n",
		}), "needle signal", 30)
		tail := got[len(got)-3:]
		if want := []string{"documentation docs/c.md", "lexical data/c.json", "lexical data/d.json"}; !slices.Equal(tail, want) {
			t.Fatalf("packet = %v, want it to end %v", got, want)
		}
	})
	t.Run("TCP-V0-063 configuration is not record data", func(t *testing.T) {
		_, got, _ := recordDataPacket(t, recordDataFixture(t, 0, map[string]string{"config/a.yaml": "needle: signal\n"}), "needle signal", 5)
		want := []string{
			"lexical config/a.yaml", "lexical code/01.go", "lexical code/02.go", "lexical code/03.go", "lexical code/04.go",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
	})
	t.Run("TCP-V0-063 a held record-data path spends no quota", func(t *testing.T) {
		// The mention's path words match every record-data path, so the code
		// bodies carry them too and the strengths keep the fixture's order.
		code := map[string]string{"code/01.go": "package code\n\n// needle signal data json\n"}
		for index := 2; index <= 6; index++ {
			code[fmt.Sprintf("code/%02d.go", index)] = "package code\n\n// needle signal data json alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu\n"
		}
		_, got, _ := recordDataPacket(t, recordDataFixture(t, 6, code), "needle signal in data/a.json", 9)
		want := []string{
			"mentioned data/a.json", "lexical code/01.go", "lexical code/02.go", "lexical code/03.go", "lexical code/04.go",
			"lexical data/b.json", "lexical data/c.json", "lexical code/05.go", "lexical code/06.go",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
	})
	t.Run("TCP-V0-063 record data without competing code is ranked by strength", func(t *testing.T) {
		_, got, lines := recordDataPacket(t, documentationFixtureWith(t, 0, 0, map[string]string{
			"data/a.json": "{\"needle\": \"signal\"}\n",
			"data/b.json": "{\"needle\": \"signal\", \"one\": \"two\"}\n",
			"data/c.json": "{\"needle\": \"signal\", \"one\": \"two three\"}\n",
			"data/d.json": "{\"needle\": \"signal\", \"one\": \"two three four\"}\n",
		}), "needle signal", 3)
		if want := []string{"lexical data/a.json", "lexical data/b.json", "lexical data/c.json"}; !slices.Equal(got, want) {
			t.Fatalf("packet = %v, want %v", got, want)
		}
		if !slices.Equal(lines, []string{"1 code and 0 documentation rows the task matched lexically are omitted by the result limit 3"}) {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
	})
	t.Run("TCP-V0-063 TCP-V0-016 withheld record data is not stated as gated", func(t *testing.T) {
		_, _, lines := recordDataPacket(t, recordDataFixture(t, 6, nil), "needle signal `absentOne` and `absentTwo`", 8)
		if !slices.Equal(lines, []string{"12 code and 0 documentation rows the task matched lexically are withheld by the `unsupported-conjunction` verdict, not by the result limit 8"}) {
			t.Fatalf("coverage.uncertainty = %q", lines)
		}
	})
}

func TestIsRecordDataSuffix(t *testing.T) {
	t.Run("TCP-V0-063", func(t *testing.T) {
		for path, want := range map[string]bool{
			"a.json": true, "A.JSON": true, "a.jsonl": true, "a.ndjson": true, "a.csv": true, "a.tsv": true,
			"a.yaml": false, "a.yml": false, "a.toml": false, "a.xml": false, "a.go": false, "a.md": false,
			"Makefile": false, "a.json.go": false, ".json": true,
		} {
			if got := isRecordDataSuffix(path); got != want {
				t.Errorf("isRecordDataSuffix(%q) = %v, want %v", path, got, want)
			}
		}
	})
}
