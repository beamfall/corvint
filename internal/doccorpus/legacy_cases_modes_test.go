package doccorpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"
)

// legacyModesFixture is the #1246 reproduction export plus distractors: a
// tie on one title word (C200, C500), a weaker title-plus-steps match (C300)
// and a non-match (C400).
const legacyModesFixture = `{
 "suites": [{"id": 1, "name": "Travel"}],
 "sections": [{"id": 10, "suite_id": 1, "parent_id": null, "name": "Cancellation"}, {"id": 11, "suite_id": 1, "parent_id": null, "name": "Payments"}],
 "cases": [
  {"id": 12345, "title": "Cancel a paid booking", "section_id": 10, "custom_steps": "Open the booking and press Cancel"},
  {"id": 500, "title": "Booking list", "section_id": 11},
  {"id": 300, "title": "Refund after cancel", "section_id": 11, "custom_steps": "Cancel the booking"},
  {"id": 200, "title": "Pay for a booking", "section_id": 11},
  {"id": 400, "title": "Login works", "section_id": 11}
 ]
}`

func searchLegacyModes(t *testing.T, q LegacyCaseQuery) LegacyCaseSearch {
	t.Helper()
	data, err := SearchLegacyCasesQuery(LegacyFormatTestRailJSON, "exports/cases.json", []byte(legacyModesFixture), q)
	if err != nil {
		t.Fatalf("%+v: %v", q, err)
	}
	var receipt LegacyCaseSearch
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	return receipt
}

func legacyRefused(t *testing.T, q LegacyCaseQuery) {
	t.Helper()
	_, err := SearchLegacyCasesQuery(LegacyFormatTestRailJSON, "exports/cases.json", []byte(legacyModesFixture), q)
	var corpusErr *Error
	if !errors.As(err, &corpusErr) || corpusErr.Code != "corpus-refused" {
		t.Fatalf("%+v: want corpus-refused, got %v", q, err)
	}
}

// DCP-V1-165: an id lookup (C12345 or 12345) returns exactly that case; an
// unknown id is none-found; id with a query or a match mode is refused.
func TestDCPV1165LegacyCaseIDLookup(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"C12345", "12345"} {
		r := searchLegacyModes(t, LegacyCaseQuery{ID: id, Limit: 10})
		if r.State != "found" || r.Total != 1 || len(r.Results) != 1 || r.Results[0].ID != "C12345" || r.Results[0].Title != "Cancel a paid booking" || r.Mode != "id" || r.ID != id || strings.Join(r.Fields, ",") != "id" || strings.Join(r.Results[0].Matched, ",") != "id" || r.Results[0].Score != 0 {
			t.Fatalf("id %s: %+v", id, r)
		}
	}
	if r := searchLegacyModes(t, LegacyCaseQuery{ID: "C99", Limit: 10}); r.State != "none-found" || r.Total != 0 || len(r.Results) != 0 || r.Mode != "id" || len(r.Searched) != 1 {
		t.Fatalf("unknown id: %+v", r)
	}
	for _, q := range []LegacyCaseQuery{
		{ID: "C12345", Query: "cancel", Limit: 10},
		{ID: "C12345", Match: LegacyMatchAll, Limit: 10},
		{ID: "C12345", Match: LegacyMatchAny, Limit: 10},
		{ID: "c12345", Limit: 10},
		{ID: "C012", Limit: 10},
		{ID: " 12345", Limit: 10},
		{ID: "C12345", Limit: 0},
	} {
		legacyRefused(t, q)
	}
}

// DCP-V1-166: match any ranks by weighted matched-token score, drops stop
// words, reports per-hit scores and breaks ties by export order; the issue's
// natural-language query ranks the target first while match all finds none.
func TestDCPV1166LegacyCaseAnyMatchRanks(t *testing.T) {
	t.Parallel()
	query := "how do I cancel a paid booking"
	if r := searchLegacyModes(t, LegacyCaseQuery{Query: query, Limit: 10}); r.State != "none-found" {
		t.Fatalf("match all should find none: %+v", r)
	}
	r := searchLegacyModes(t, LegacyCaseQuery{Query: query, Match: LegacyMatchAny, Limit: 10})
	ids, scores := []string{}, []int{}
	for _, hit := range r.Results {
		ids, scores = append(ids, hit.ID), append(scores, hit.Score)
	}
	// C12345: cancel/paid/booking in title (3x4). C300: cancel title 4 +
	// booking steps 1. C200 and C500 tie on booking in title and keep export
	// (case-ID) order. C400 does not match.
	if r.State != "found" || r.Mode != LegacyMatchAny || r.Total != 4 || strings.Join(ids, ",") != "C12345,C300,C200,C500" || len(scores) != 4 || scores[0] != 12 || scores[1] != 5 || scores[2] != 4 || scores[3] != 4 {
		t.Fatalf("ranking: %v %v %+v", ids, scores, r)
	}
	if strings.Join(r.Tokens, " ") != "cancel paid booking" || strings.Join(r.Dropped, " ") != "how do i a" || !strings.Contains(r.Match, "title 4, path 2") {
		t.Fatalf("receipt tokens: %+v", r)
	}
	if limited := searchLegacyModes(t, LegacyCaseQuery{Query: query, Match: LegacyMatchAny, Limit: 1}); limited.Total != 4 || len(limited.Results) != 1 || limited.Results[0].ID != "C12345" {
		t.Fatalf("limit: %+v", limited)
	}
	// A path-only word outranks a step-only word.
	if p := searchLegacyModes(t, LegacyCaseQuery{Query: "payments open", Match: LegacyMatchAny, Limit: 10}); p.Results[0].ID != "C200" || p.Results[0].Score != 2 || p.Results[len(p.Results)-1].ID != "C12345" || p.Results[len(p.Results)-1].Score != 1 {
		t.Fatalf("path weight: %+v", p)
	}
	legacyRefused(t, LegacyCaseQuery{Query: "how do I", Match: LegacyMatchAny, Limit: 10})
	legacyRefused(t, LegacyCaseQuery{Query: "cancel", Match: "fuzzy", Limit: 10})
}

// DCP-V1-167: a query that is exactly one C<digits> word naming a case is an
// id lookup; when no case has that id, the query searches as before.
func TestDCPV1167LegacyCaseQueryIDRule(t *testing.T) {
	t.Parallel()
	r := searchLegacyModes(t, LegacyCaseQuery{Query: "C12345", Limit: 10})
	if r.Mode != "id" || r.Query != "C12345" || r.ID != "" || r.Total != 1 || r.Results[0].ID != "C12345" {
		t.Fatalf("query id: %+v", r)
	}
	for _, query := range []string{"C99", "12345", " C12345", "C12345 cancel", "c12345"} {
		if got := searchLegacyModes(t, LegacyCaseQuery{Query: query, Limit: 10}); got.Mode != "" {
			t.Fatalf("%q treated as an id lookup: %+v", query, got)
		}
	}
}

// DCP-V1-168: the default and explicit all-token receipts are byte-identical
// to the pre-change search (digests recorded at 99dac5234 before this
// change), carry no mode or score, and an absent exact-id query falls back to
// those bytes.
func TestDCPV1168LegacyCaseAllModeBytesUnchanged(t *testing.T) {
	t.Parallel()
	golden := map[string]string{
		"testrail-json|Refund":             "35429caaf9828abf4a0651c90c161859fc20d9692c646f5253c2b7fa6afded38",
		"testrail-json|approve queue":      "f19067d817832823e8dfe6a41f38755735582f3d8af651db3188f0569fbb8743",
		"testrail-json|Chargeback dispute": "9c990ee01563798960ed463b9c025110b0d11b8846413f08f1cb4208f22d58fa",
		"testrail-json|c7":                 "52a8eb4629ca0bb4aa4f5026e7efde2a691f5c06604aea3572aa22b4fb13b5ef",
		"testrail-json|7":                  "3c8b9ab6f61e9d3da071e50449158769dd57f91080697b4b5049af5bf74f1906",
		"testrail-json|stepword":           "23afbca18e45933fc57e827e49f73de9a79109a52c2cc0f51c19c21e4d1edccc",
		"testrail-json|a":                  "22fb0bbeb735886002e9a76b0e955831a293bc77fc028e2e8880639601408e07",
		"testrail-json|C99":                "103d350b569a5a07c9728961268798b82bb08dfa009d7f070279d8e2bd8d3d7a",
		"testrail-json|C7 queue":           "3a8d702c258576e7a305682207085bbc7eea9a322327aa34202963ce6973e0a1",
		"testrail-json| C7":                "559186d51927bfd37b9211b17305e1ea7a1e9a242bf20ed6f855839508c733dd",
		"csv|signed manager":               "db0928e4533a6cac4cffd3da37290e6ce33af198031585ed77362c382659bba8",
		"csv|totals invoices":              "d869e73b7c2807ae63c917dff151aaa7e2e68e386474755406fe994bb33e6277",
		"csv|C99":                          "80813e5bdd6ef8eb93805697d502a2c42cf5da81e66fcb61c5c2742d16a52e0e",
		"csv|7":                            "84414c991d4b70246af9875fa8e5a05f9378cb406c693514fb083c0a2cab7e31",
	}
	fixtures := map[string]string{LegacyFormatTestRailJSON: legacyTestRailFixture, LegacyFormatCSV: legacyCSVFixture}
	for key, want := range golden {
		format, query, _ := strings.Cut(key, "|")
		def, err := SearchLegacyCases(format, "cases", []byte(fixtures[format]), query, 10)
		if err != nil {
			t.Fatal(err)
		}
		explicit, err := SearchLegacyCasesQuery(format, "cases", []byte(fixtures[format]), LegacyCaseQuery{Query: query, Match: LegacyMatchAll, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(def)
		if hex.EncodeToString(sum[:]) != want || !bytes.Equal(def, explicit) {
			t.Errorf("%s: all-mode bytes changed:\n%s", key, def)
		}
		if bytes.Contains(def, []byte(`"mode"`)) || bytes.Contains(def, []byte(`"score"`)) || bytes.Contains(def, []byte(`"stop_words_dropped"`)) {
			t.Errorf("%s: all-mode receipt gained a field: %s", key, def)
		}
	}
}
