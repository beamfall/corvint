package evalrepo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// GPK-V0-002: the Python oracle refuses a golden limit that is not a JSON
// integer; an absent limit defaults to 10, and a present malformed one must
// reach the verb's own limit refusal instead of silently scoring at 10.
func TestGoldenMalformedLimitReachesTheVerbRefusal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "golden.json")
	golden := `{"schemaVersion":1,"cases":[{"id":"absent"},{"id":"string","mode":"query","text":"task","limit":"5"},` +
		`{"id":"fraction","limit":2.5},{"id":"null","limit":null},{"id":"bool","limit":true},{"id":"integer","limit":7}]}`
	if err := os.WriteFile(path, []byte(golden), 0o644); err != nil {
		t.Fatal(err)
	}
	_, cases, err := loadGolden(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"absent": 10, "string": 0, "fraction": 0, "null": 0, "bool": 0, "integer": 7}
	for _, item := range cases {
		if item.Limit != want[item.ID] {
			t.Errorf("%s: limit %d, want %d", item.ID, item.Limit, want[item.ID])
		}
	}
	if _, err := evaluateCase(context.Background(), nil, cases[1], nil); err == nil || err.Error() != "limit must be an integer from 1 to 50" {
		t.Fatalf("malformed limit was not refused by the verb: %v", err)
	}
}

// GPK-V0-002: the Python oracle's validate_budget raises "budget_bytes must
// be an integer" for a present, non-null budget_bytes that is not a JSON
// integer -- a distinct message from its own out-of-range refusal. Go's
// integer(value, 0) fallback used to coerce every malformed shape to 0,
// which contextindex's unmodified range check always rejects, but with the
// oracle's out-of-range text instead of its type-error text. caseBudget must
// report the malformed shapes back as unrepresentable so evaluateCase can
// substitute the oracle's own type-error text.
func TestGoldenMalformedBudgetBytesReachesTheOracleTypeRefusal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "golden.json")
	golden := `{"schemaVersion":1,"cases":[` +
		`{"id":"absent","mode":"feature","feature_id":"widget"},` +
		`{"id":"null","mode":"feature","feature_id":"widget","budget_bytes":null},` +
		`{"id":"fraction","mode":"feature","feature_id":"widget","budget_bytes":2.5},` +
		`{"id":"bool","mode":"feature","feature_id":"widget","budget_bytes":true},` +
		`{"id":"string","mode":"feature","feature_id":"widget","budget_bytes":"1300"},` +
		`{"id":"integer","mode":"feature","feature_id":"widget","budget_bytes":1300}]}`
	if err := os.WriteFile(path, []byte(golden), 0o644); err != nil {
		t.Fatal(err)
	}
	_, cases, err := loadGolden(path)
	if err != nil {
		t.Fatal(err)
	}
	wantMalformed := map[string]bool{"absent": false, "null": false, "fraction": true, "bool": true, "string": true, "integer": false}
	wantBudget := map[string]*int{"integer": intPointer(1300)}
	byID := make(map[string]goldenCase, len(cases))
	for _, item := range cases {
		byID[item.ID] = item
		if item.BudgetMalformed != wantMalformed[item.ID] {
			t.Errorf("%s: malformed %v, want %v", item.ID, item.BudgetMalformed, wantMalformed[item.ID])
		}
		want, expectSet := wantBudget[item.ID]
		if expectSet && (item.BudgetBytes == nil || *item.BudgetBytes != *want) {
			t.Errorf("%s: budget %v, want %d", item.ID, item.BudgetBytes, *want)
		}
		if !expectSet && item.BudgetBytes != nil {
			t.Errorf("%s: budget %v, want nil", item.ID, *item.BudgetBytes)
		}
	}

	index := &contextindex.Index{}
	for _, id := range []string{"absent", "null", "fraction", "bool", "string"} {
		item := byID[id]
		_, err := evaluateCase(context.Background(), index, item, nil)
		if id == "fraction" || id == "bool" || id == "string" {
			if err == nil || err.Error() != "budget_bytes must be an integer" {
				t.Errorf("%s: got err %v, want \"budget_bytes must be an integer\"", id, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", id, err)
		}
	}
}

// GPK-V0-002/004: the Python oracle's _clean_impact_path raises "impact
// paths must be non-empty strings" for any impact-paths list entry that is
// not a JSON string. Go's stringsValue used to silently drop such an entry,
// so a malformed case that mixed a bad entry with well-typed ones would
// succeed where the oracle refuses the whole case. casePaths must keep the
// entry as an empty-string sentinel so the "impact" verb's own (unmodified)
// cleanImpactPath refusal fires instead.
func TestGoldenNonStringImpactPathReachesTheVerbRefusal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "golden.json")
	golden := `{"schemaVersion":1,"cases":[` +
		`{"id":"mixed","mode":"impact","paths":["src/a.go",5]},` +
		`{"id":"clean","mode":"impact","paths":["src/a.go"]}]}`
	if err := os.WriteFile(path, []byte(golden), 0o644); err != nil {
		t.Fatal(err)
	}
	_, cases, err := loadGolden(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases[0].Paths) != 2 || cases[0].Paths[0] != "src/a.go" || cases[0].Paths[1] != "" {
		t.Fatalf("mixed paths = %#v, want [\"src/a.go\" \"\"]", cases[0].Paths)
	}
	if _, err := evaluateCase(context.Background(), nil, cases[0], nil); err == nil || err.Error() != "impact paths must be non-empty strings" {
		t.Fatalf("non-string impact path was not refused by the verb: %v", err)
	}
}

// GPK-V0-002: the Python oracle's evaluate() applies Python's unconditional
// str() to case.get("id", "") -- a present non-string id (bool/null/number)
// still stringifies, it does not fall back to "". stringValue's plain type
// assertion used to silently coerce every such id to "".
func TestGoldenIDStringifiesLikePythonStr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "golden.json")
	golden := `{"schemaVersion":1,"cases":[{"id":"absent-key-below"},{},` +
		`{"id":true},{"id":false},{"id":null},{"id":1},{"id":1.0},{"id":2.5}]}`
	if err := os.WriteFile(path, []byte(golden), 0o644); err != nil {
		t.Fatal(err)
	}
	_, cases, err := loadGolden(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"absent-key-below", "", "True", "False", "None", "1", "1.0", "2.5"}
	for index, item := range cases {
		if item.ID != want[index] {
			t.Errorf("case %d: id %q, want %q", index, item.ID, want[index])
		}
	}
}

// GPK-V0-002: the oracle's str(case.get("partition", "development")) only
// falls back to "development" when the key is wholly absent; a present
// explicit null still stringifies to "None". stringDefault's type assertion
// used to route both cases to the "development" fallback.
func TestGoldenPartitionStringifiesLikePythonStr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "golden.json")
	golden := `{"schemaVersion":1,"cases":[{"id":"a"},{"id":"b","partition":null},` +
		`{"id":"c","partition":true},{"id":"d","partition":7}]}`
	if err := os.WriteFile(path, []byte(golden), 0o644); err != nil {
		t.Fatal(err)
	}
	_, cases, err := loadGolden(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a": "development", "b": "None", "c": "True", "d": "7"}
	for _, item := range cases {
		if item.Partition != want[item.ID] {
			t.Errorf("%s: partition %q, want %q", item.ID, item.Partition, want[item.ID])
		}
	}
}

// GPK-V0-002: the oracle's dispatch raises CorvintError(f"unknown golden mode:
// {mode!r}") -- Python's repr(), which quotes a string but not a
// bool/None/number. dispatchCase used to format the already-"" -coerced
// Mode field with Go's %q, which always adds double quotes.
func TestGoldenUnknownModeErrorTextMatchesPythonRepr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "golden.json")
	golden := `{"schemaVersion":1,"cases":[{"id":"a","mode":"bogus"},{"id":"b","mode":5},` +
		`{"id":"c","mode":1.5},{"id":"d","mode":true},{"id":"e","mode":null}]}`
	if err := os.WriteFile(path, []byte(golden), 0o644); err != nil {
		t.Fatal(err)
	}
	_, cases, err := loadGolden(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"a": "unknown golden mode: 'bogus'", "b": "unknown golden mode: 5",
		"c": "unknown golden mode: 1.5", "d": "unknown golden mode: True",
		"e": "unknown golden mode: None",
	}
	for _, item := range cases {
		_, err := evaluateCase(context.Background(), nil, item, nil)
		if err == nil || err.Error() != want[item.ID] {
			t.Errorf("%s: err %v, want %q", item.ID, err, want[item.ID])
		}
	}
}

func intPointer(value int) *int { return &value }
