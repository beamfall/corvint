package sqlnative

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const reviewCorpusSHA256 = "f7b71491c7442d0b164ac5827435bab0813b7eaed4980579c8e7e8535812fce3"
const writeReturningCorpusClass = "write-returning-cross-product"
const writeReturningCorpusSeed uint64 = 202608260351
const sqliteOracleVersion = "3.51.0 2025-06-12 13:14:41 f0ca7bba1c5e232e5d279fad6338121ab55af0c8c68c84cdfb18ba5114dcaapl (64-bit)\n"

type reviewCase struct {
	Name  string `json:"name"`
	Class string `json:"class"`
	SQL   string `json:"sql"`
	Want  string `json:"want"`
}
type reviewClass struct {
	Name  string `json:"name"`
	Seed  uint64 `json:"seed"`
	Cases int    `json:"cases"`
}
type reviewCorpus struct {
	Version string        `json:"version"`
	Classes []reviewClass `json:"classes"`
	Cases   []reviewCase  `json:"cases"`
}

func loadReviewCorpus(t *testing.T) reviewCorpus {
	t.Helper()
	raw, err := os.ReadFile("testdata/review-adversarial-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != reviewCorpusSHA256 {
		t.Fatalf("review corpus digest=%s", got)
	}
	var corpus reviewCorpus
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != "sqlite-review-adversarial/v1" || len(corpus.Cases) == 0 {
		t.Fatalf("review corpus=%#v", corpus)
	}
	return corpus
}

func candidateSQL(source string) bool {
	output := string(AnalyzeFrame(EncodeRequest(fixture("internal/store/query.sql", "sqlite.query", source))))
	return strings.Contains(output, `"status":"CANDIDATE"`)
}

func TestReviewAdversarialCorpus(t *testing.T) {
	for _, tc := range loadReviewCorpus(t).Cases {
		t.Run(tc.Name, func(t *testing.T) {
			if got, want := candidateSQL(tc.SQL), tc.Want == "candidate"; got != want {
				t.Fatalf("candidate=%v want=%v sql=%q", got, want, tc.SQL)
			}
		})
	}
}

func TestSQLite351WriteReturningCorpusCrossProduct(t *testing.T) {
	corpus := loadReviewCorpus(t)
	classes := map[string]reviewClass{}
	for _, class := range corpus.Classes {
		classes[class.Name] = class
	}
	class, ok := classes[writeReturningCorpusClass]
	if !ok || class.Seed != writeReturningCorpusSeed || class.Cases != 92 {
		t.Fatalf("write returning class=%#v", class)
	}
	results := []string{
		"id", "(id)", "+id", "-id", "NOT id", "id + 1", "id - 1", "id * 1", "id / 1", "id % 1", "id || 'v'", "id | 1", "id & 1", "id << 1", "id >> 1", "id = 1", "id == 1", "id != 1", "id <> 1", "id < 1", "id > 1", "id <= 1", "id >= 1",
	}
	want := map[string]struct{}{}
	for _, prefix := range []string{"UPDATE t SET id = 8", "DELETE FROM t"} {
		for _, where := range []string{"", " WHERE id = 8"} {
			for _, result := range results {
				want[prefix+where+" RETURNING "+result+";"] = struct{}{}
			}
		}
	}
	got := map[string]struct{}{}
	for _, tc := range corpus.Cases {
		if tc.Class != writeReturningCorpusClass {
			continue
		}
		if tc.Want != "candidate" {
			t.Fatalf("write returning wants %q for %q", tc.Want, tc.Name)
		}
		if _, duplicate := got[tc.SQL]; duplicate {
			t.Fatalf("duplicate write returning SQL %q", tc.SQL)
		}
		got[tc.SQL] = struct{}{}
	}
	if len(got) != len(want) {
		t.Fatalf("write returning count=%d want=%d", len(got), len(want))
	}
	for source := range want {
		if _, ok := got[source]; !ok {
			t.Fatalf("missing write returning cross-product source %q", source)
		}
	}
}

func TestSQLite351WriteReturningClosedClause(t *testing.T) {
	forms := []struct {
		name    string
		valid   string
		invalid []string
	}{
		{name: "insert", valid: "INSERT INTO t VALUES (8) RETURNING id;", invalid: []string{"INSERT INTO t VALUES (8) RETURNING;"}},
		{name: "update", valid: "UPDATE t SET id = 8 RETURNING id;", invalid: []string{"UPDATE t SET id = 8 RETURNING;", "UPDATE t SET id = 8 RETURNING id RETURNING id;", "UPDATE t SET id = 8 WHERE id = 8 RETURNING id FROM t;"}},
		{name: "delete", valid: "DELETE FROM t RETURNING id;", invalid: []string{"DELETE FROM t RETURNING;", "DELETE FROM t RETURNING id WHERE id = 8;"}},
	}
	parser, err := os.ReadFile("analyzer.go")
	if err != nil || strings.Count(string(parser), `kind, fact = "sqlite.query.write"`) != len(forms) {
		t.Fatalf("write dispatch=%d forms=%d err=%v", strings.Count(string(parser), `kind, fact = "sqlite.query.write"`), len(forms), err)
	}
	for _, form := range forms {
		if form.valid == "" || len(form.invalid) == 0 {
			t.Fatalf("write form %q bypasses returning coverage", form.name)
		}
		if !candidateSQL(form.valid) {
			t.Fatalf("rejected shared returning form %q", form.valid)
		}
		for _, source := range form.invalid {
			if candidateSQL(source) {
				t.Fatalf("accepted malformed returning form %q", source)
			}
		}
	}
	source := "UPDATE t SET id = 8 RETURNING id, id + 1;"
	tokens, err := lexSQL([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	returning := 0
	for i, token := range tokens {
		if token.keyword("RETURNING") {
			returning = i
			break
		}
	}
	clause, ok := returningClauseFor(tokens[returning : len(tokens)-1])
	if !ok || string([]byte(source)[clause[0].start:clause[len(clause)-1].end]) != "RETURNING id, id + 1" || len(clause) != 6 {
		t.Fatalf("returning clause=%#v ok=%v", clause, ok)
	}
	for _, forged := range [][]token{
		{{kind: tokQuoted, text: "RETURNING", norm: "returning", start: 0, end: 9}, {kind: tokIdent, text: "id", norm: "id", start: 10, end: 12}},
		{{kind: tokIdent, text: "RETURNING", norm: "wrong", start: 0, end: 9}, {kind: tokIdent, text: "id", norm: "id", start: 10, end: 12}},
		{{kind: tokIdent, text: "RETURNING", norm: "returning", start: 0, end: 9}},
	} {
		if _, ok := returningClauseFor(forged); ok {
			t.Fatalf("accepted forged returning clause %#v", forged)
		}
	}
}

func TestGeneratedSQLiteKeywordNameAliasMatrix(t *testing.T) {
	sum := sha256.Sum256([]byte(kw))
	if got := hex.EncodeToString(sum[:]); got != "105feccd4ef384e0fcc3defa44d6400b9ef9e6516a87b64d77c883ebffaba6bc" {
		t.Fatalf("frozen SQLite keyword table digest=%s", got)
	}
	words := strings.Split(strings.Trim(kw, ","), ",")
	if len(words) != 147 {
		t.Fatalf("keywords=%d want=147", len(words))
	}
	for _, word := range words {
		for _, source := range []string{"CREATE TABLE " + word + " (id INTEGER);", "SELECT 1 AS " + word + ";"} {
			if candidateSQL(source) {
				t.Errorf("accepted unquoted keyword %q", source)
			}
		}
		for _, source := range []string{"CREATE TABLE \"" + word + "\" (id INTEGER);", "SELECT 1 AS \"" + word + "\";"} {
			if !candidateSQL(source) {
				t.Errorf("rejected quoted keyword %q", source)
			}
		}
	}
	for _, source := range []string{"CREATE TABLE strict (id INTEGER);", "SELECT 1 AS strict;", "INSERT INTO t VALUES (1) RETURNING id;"} {
		if !candidateSQL(source) {
			t.Errorf("rejected contextual syntax %q", source)
		}
	}
}

func TestGeneratedClosedPragmaValueMatrix(t *testing.T) {
	for _, value := range []string{"0", "1", "ON", "OFF", "TRUE", "FALSE", "YES", "NO", "'on'", "'off'", "'true'", "'false'", "'yes'", "'no'"} {
		if !candidateSQL("PRAGMA foreign_keys=" + value + ";") {
			t.Errorf("rejected supported pragma value %q", value)
		}
	}
	for _, value := range []string{"2", "-1", "'2'", "SELECT", "(SELECT)", "NULL", "X'01'", "ON OFF"} {
		if candidateSQL("PRAGMA foreign_keys=" + value + ";") {
			t.Errorf("accepted unsupported pragma value %q", value)
		}
	}
	if candidateSQL("PRAGMA cache_size=1;") {
		t.Error("accepted arbitrary pragma")
	}
}

func TestGeneratedNumericTokenAdjacencyMatrix(t *testing.T) {
	for _, literal := range []string{"0x0", "0X1", "0xDeAdBeEf", "0x8000000000000000", "0xFFFFFFFFFFFFFFFF"} {
		if !candidateSQL("SELECT " + literal + ";") {
			t.Errorf("rejected supported literal %q", literal)
		}
	}
	for _, literal := range []string{"0x", "0xg", "0x1g", "0x1_", "0x1$", "0x10000000000000000"} {
		if candidateSQL("SELECT " + literal + ";") {
			t.Errorf("accepted invalid literal %q", literal)
		}
	}
}

func TestPinnedSQLiteOracleForAcceptedSubset(t *testing.T) {
	bin := os.Getenv("SQLITE3_BIN")
	if bin == "" {
		t.Skip("pinned SQLite oracle not requested")
	}
	got, err := exec.Command(bin, "--version").Output()
	if err != nil || string(got) != sqliteOracleVersion {
		t.Fatalf("oracle version=%q err=%v", got, err)
	}
	for _, tc := range loadReviewCorpus(t).Cases {
		if tc.Want != "candidate" {
			continue
		}
		command := exec.Command(bin, ":memory:")
		command.Stdin = strings.NewReader("CREATE TABLE t (id INTEGER);" + tc.SQL)
		if output, err := command.CombinedOutput(); err != nil {
			t.Errorf("oracle rejected %q: %v: %s", tc.Name, err, output)
		}
	}
}

func TestPinnedSQLiteOracleForTriggerRestrictions(t *testing.T) {
	bin := os.Getenv("SQLITE3_BIN")
	if bin == "" {
		t.Skip("pinned SQLite oracle not requested")
	}
	got, err := exec.Command(bin, "--version").Output()
	if err != nil || string(got) != sqliteOracleVersion {
		t.Fatalf("oracle version=%q err=%v", got, err)
	}
	setup := "CREATE TABLE t (id INTEGER); CREATE TABLE log (id INTEGER);"
	rejected := []string{
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN INSERT INTO main.log VALUES (1); END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN UPDATE main.log SET id=1; END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN DELETE FROM main.log; END;",
		"CREATE TRIGGER tr AFTER INSERT ON t BEGIN INSERT INTO log DEFAULT VALUES; END;",
	}
	for _, source := range rejected {
		command := exec.Command(bin, ":memory:")
		command.Stdin = strings.NewReader(setup + source)
		if output, err := command.CombinedOutput(); err == nil {
			t.Errorf("oracle accepted trigger restriction witness %q: %s", source, output)
		}
	}
	profileExcluded := "CREATE TRIGGER tr AFTER INSERT ON t BEGIN WITH c AS (SELECT 1) SELECT * FROM c; END;"
	command := exec.Command(bin, ":memory:")
	command.Stdin = strings.NewReader(setup + profileExcluded)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("oracle rejected profile-excluded trigger CTE: %v: %s", err, output)
	}
	accepted := "CREATE TRIGGER tr AFTER INSERT ON t BEGIN INSERT INTO log VALUES (1); UPDATE log SET id=2; DELETE FROM log WHERE id=2; SELECT 1; END;"
	command = exec.Command(bin, ":memory:")
	command.Stdin = strings.NewReader(setup + accepted)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("oracle rejected trigger complement: %v: %s", err, output)
	}
}
