package corpusbridge

import (
	json "encoding/json/v2"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// frameCost costs the path as the row writes it, a JSON string, not as a Go
// quoted string. A hostile path whose Go and JSON spellings differ (a DEL,
// a quote, a backslash, a hidden or non-ASCII character) must still be costed
// as sent (CodeQL alert 39).
func TestFrameCostEncodesPathAsJSON(t *testing.T) {
	receipt := []byte(`{"ok":true}` + "\n")
	cost := func(quotedPath string) int {
		row := `{"path":` + quotedPath + `,"receipt":` + string(receipt) + `}`
		text, _ := json.Marshal(repoenvelope.EscapeHidden(row))
		return len(row) + 10 + len(text)
	}
	encoded, err := json.Marshal(hostilePath, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := frameCost(hostilePath, receipt), cost(string(encoded)); got != want {
		t.Fatalf("frameCost = %d, want the JSON-encoded row cost %d", got, want)
	}
	if frameCost(hostilePath, receipt) == cost(strconv.Quote(hostilePath)) {
		t.Fatal("fixture no longer separates the Go and JSON spellings")
	}
	plain := "docs/page.md"
	if got, want := frameCost(plain, receipt), len(plain)+len(receipt)+32+len(mustMarshal(t, `{"path":"docs/page.md","receipt":`+string(receipt)+`}`)); got != want {
		t.Fatalf("ordinary path cost changed: %d, want %d", got, want)
	}
}

const hostilePath = "docs/\x7f\"q\\\u00e9\u200b.md"

func mustMarshal(t *testing.T, value string) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
