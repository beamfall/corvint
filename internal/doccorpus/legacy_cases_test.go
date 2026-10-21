package doccorpus

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"
)

const legacyTestRailFixture = `{
 "suites": [{"id": 1, "name": "Billing", "description": "ignored"}],
 "sections": [
  {"id": 10, "suite_id": 1, "parent_id": null, "name": "Invoices"},
  {"id": 11, "suite_id": 1, "parent_id": 10, "name": "Refund approvals"}
 ],
 "cases": [
  {"id": 7, "title": "Manager approves request", "section_id": 11, "suite_id": 1,
   "custom_preconds": "<p>Signed in as <b>manager</b></p>",
   "custom_steps_separated": [
    {"content": "<ol><li>Open the queue</li><li>Click&nbsp;<em>Approve</em></li></ol><script>alert(1)</script>", "expected": "<p>Banner shows SECRET-EXPECTED</p>"}
   ],
   "priority_id": 2},
  {"id": 12, "title": "Totals line up", "section_id": 10, "suite_id": 1,
   "custom_steps": "Add two lines and read STEPWORD total", "custom_expected": "EXPECTEDWORD sum"}
 ]
}`

const legacyCSVFixture = "\xef\xbb\xbfID,Title,Section Hierarchy,Suite,Preconditions,Steps (Step),Steps (Expected Result)\n" +
	"C7,Manager approves request,Invoices > Refund approvals,Billing,<p>Signed in</p>,<b>Open</b> the queue,SECRET-EXPECTED\n" +
	",,,,,Click Approve,Banner\n" +
	"12,Totals line up,Invoices,Billing,,STEPWORD,EXPECTEDWORD\n"

func decodeLegacyRecord(t *testing.T, data []byte) LegacyCasesRecord {
	t.Helper()
	var record LegacyCasesRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

// DCP-V1-070/071/074: both formats join suite and section path, key cases by
// ID and emit manual legacy cases that never count as runtime parity.
func TestImportLegacyCasesFormats(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ format, data string }{{LegacyFormatTestRailJSON, legacyTestRailFixture}, {LegacyFormatCSV, legacyCSVFixture}} {
		t.Run(tc.format, func(t *testing.T) {
			data, err := ImportLegacyCases(tc.format, "", "exports/cases."+tc.format, []byte(tc.data))
			if err != nil {
				t.Fatal(err)
			}
			again, err := ImportLegacyCases(tc.format, LegacyContentIDsPathsTitles, "exports/cases."+tc.format, []byte(tc.data))
			if err != nil || !bytes.Equal(data, again) {
				t.Fatalf("import is not deterministic: %v", err)
			}
			record := decodeLegacyRecord(t, data)
			if record.Schema != LegacyCasesSchema || record.Content != LegacyContentIDsPathsTitles || record.LocalOnly || record.RuntimeParity || len(record.Cases) != 2 {
				t.Fatalf("record header: %+v", record)
			}
			if record.Source.SHA256 != Digest([]byte(tc.data)) || record.Source.Cases != 2 || record.Source.ID != tc.format+":sha256:"+record.Source.SHA256 {
				t.Fatalf("source: %+v", record.Source)
			}
			first := record.Cases[0]
			if first.Legacy.ID != "C7" || first.Legacy.State != "manual" || first.Legacy.Case != "Manager approves request" || first.Legacy.Suite != "Billing > Invoices > Refund approvals" || first.Legacy.Runtime != nil {
				t.Fatalf("case: %+v", first.Legacy)
			}
			if strings.Join(first.Path, "|") != "Billing|Invoices|Refund approvals" || first.Content != nil || len(first.Digest) != 64 {
				t.Fatalf("case path/content: %+v", first)
			}
			if a := first.Legacy.Evidence; a.Region != "case:C7" || a.Authority != "external-provider" || a.Kind != "imported" || a.SHA256 != record.Source.SHA256 || a.SpanSHA256 != first.Digest {
				t.Fatalf("anchor: %+v", a)
			}
			if record.Cases[1].Legacy.ID != "C12" || record.Cases[1].Legacy.Suite != "Billing > Invoices" {
				t.Fatalf("second case: %+v", record.Cases[1].Legacy)
			}
		})
	}
}

// DCP-V1-073: under ids-paths-titles no precondition, step or expected-result
// text appears in import or search output; the digest still changes when a
// step changes.
func TestLegacyCasesIDsPathsTitlesWithholdsSteps(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ format, data string }{{LegacyFormatTestRailJSON, legacyTestRailFixture}, {LegacyFormatCSV, legacyCSVFixture}} {
		imported, err := ImportLegacyCases(tc.format, LegacyContentIDsPathsTitles, "cases", []byte(tc.data))
		if err != nil {
			t.Fatal(err)
		}
		searched, err := SearchLegacyCases(tc.format, "cases", []byte(tc.data), "stepword", 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, out := range [][]byte{imported, searched} {
			for _, secret := range []string{"SECRET-EXPECTED", "EXPECTEDWORD", "STEPWORD", "Signed in", "queue", "Banner"} {
				if bytes.Contains(out, []byte(secret)) {
					t.Fatalf("%s: output leaks %q: %s", tc.format, secret, out)
				}
			}
		}
		edited, err := ImportLegacyCases(tc.format, "", "cases", []byte(strings.Replace(tc.data, "STEPWORD", "OTHERWORD", 1)))
		if err != nil {
			t.Fatal(err)
		}
		if decodeLegacyRecord(t, edited).Cases[1].Digest == decodeLegacyRecord(t, imported).Cases[1].Digest {
			t.Fatalf("%s: step edit did not change the case digest", tc.format)
		}
	}
}

// DCP-V1-072/073: full content strips HTML (tags, script bodies, entities)
// and is marked local-only.
func TestImportLegacyCasesFullStripsHTML(t *testing.T) {
	t.Parallel()
	data, err := ImportLegacyCases(LegacyFormatTestRailJSON, LegacyContentFull, "cases", []byte(legacyTestRailFixture))
	if err != nil {
		t.Fatal(err)
	}
	record := decodeLegacyRecord(t, data)
	if !record.LocalOnly || record.Content != LegacyContentFull {
		t.Fatalf("full record not local-only: %+v", record)
	}
	content := record.Cases[0].Content
	if content == nil || content.Preconditions != "Signed in as manager" || len(content.Steps) != 1 || content.Steps[0].Content != "Open the queue Click Approve" || content.Steps[0].Expected != "Banner shows SECRET-EXPECTED" {
		t.Fatalf("stripped content: %+v", content)
	}
	if bytes.Contains(data, []byte("alert")) || bytes.Contains(data, []byte("<")) && bytes.Contains(data, []byte("<li>")) {
		t.Fatalf("HTML survived: %s", data)
	}
	for in, want := range map[string]string{
		"a < b &amp;&amp; <!-- note --> c<br/>d":       "a < b && c d",
		`<a title="x > y" data-q='1>2'>Click</a> here`: "Click here",
		"keep <script>LEAK":                            "keep",
		"keep <STYLE type=\"text/css\">LEAK</style":    "keep",
		"a<script>x = '</b>'</script>b":                "a b",
		"keep <b title=\"unterminated>LEAK":            "keep",
		"open <!-- LEAK":                               "open",
		"\u212a<script>LEAK</script>ok":                "\u212a ok",
		strings.Repeat("\u212a", 20) + "<b>x</b>":      strings.Repeat("\u212a", 20) + " x",
		"\u0130<SCRIPT>LEAK</Script>ok":                "\u0130 ok",
		"a<script>x</scripture>LEAK</script >b":        "a b",
		"<style-guide>refund</style-guide>":            "refund",
		"<script\ttype=x>LEAK</SCRIPT\n>ok":            "ok",
	} {
		if got := stripLegacyHTML(in); got != want {
			t.Errorf("strip %q: %q, want %q", in, got, want)
		}
	}
}

// DCP-V1-073: oversized fields are bounded with an explicit marker naming the
// field, its stripped size and the bytes kept.
func TestImportLegacyCasesBoundsOversized(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("é", legacyStepBytes)
	steps := []map[string]string{}
	for range legacySteps + 2 {
		steps = append(steps, map[string]string{"content": long, "expected": "ok"})
	}
	export := map[string]any{"suites": []any{}, "sections": []any{}, "cases": []any{map[string]any{"id": 3, "title": strings.Repeat("t", legacyTitleBytes+5), "custom_steps_separated": steps}}}
	raw, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ImportLegacyCases(LegacyFormatTestRailJSON, LegacyContentFull, "cases", raw)
	if err != nil {
		t.Fatal(err)
	}
	c := decodeLegacyRecord(t, data).Cases[0]
	fields := map[string]LegacyCaseBound{}
	for _, b := range c.Bounded {
		fields[b.Field] = b
	}
	if b := fields["title"]; b.Bytes != legacyTitleBytes+5 || b.Kept != legacyTitleBytes {
		t.Fatalf("title bound: %+v", c.Bounded)
	}
	if b := fields["steps[0].content"]; b.Bytes != 2*legacyStepBytes || b.Kept != legacyStepBytes || len(c.Content.Steps[0].Content) != legacyStepBytes {
		t.Fatalf("step bound: %+v", b)
	}
	if b := fields["steps"]; b.Bytes != legacySteps+2 || b.Kept != legacySteps || len(c.Content.Steps) != legacySteps {
		t.Fatalf("step-count bound: %+v", b)
	}
	search, err := SearchLegacyCases(LegacyFormatTestRailJSON, "cases", raw, "zzz", 5)
	if err != nil {
		t.Fatal(err)
	}
	var receipt LegacyCaseSearch
	if err := json.Unmarshal(search, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.State != "none-found" || len(receipt.Bounded) != 1 || receipt.Bounded[0].ID != "C3" {
		t.Fatalf("search does not name bounded cases: %+v", receipt)
	}
}

// DCP-V1-075/076/077: search matches path-only feature words, returns ID,
// path and title, and a none-found names its query and searched source.
func TestSearchLegacyCasesReceipt(t *testing.T) {
	t.Parallel()
	data, err := SearchLegacyCases(LegacyFormatTestRailJSON, "exports/cases.json", []byte(legacyTestRailFixture), "Refund", 10)
	if err != nil {
		t.Fatal(err)
	}
	var receipt LegacyCaseSearch
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.State != "found" || receipt.Total != 1 || len(receipt.Results) != 1 {
		t.Fatalf("path-only feature word not found: %+v", receipt)
	}
	hit := receipt.Results[0]
	if hit.ID != "C7" || hit.Title != "Manager approves request" || strings.Join(hit.Path, "|") != "Billing|Invoices|Refund approvals" || strings.Join(hit.Matched, ",") != "path" {
		t.Fatalf("hit: %+v", hit)
	}
	if strings.Contains(strings.ToLower(hit.Title), "refund") {
		t.Fatal("fixture must keep the feature word out of the title")
	}
	for query, want := range map[string]string{"approve queue": "C7", "signed manager": "C7", "totals invoices": "C12"} {
		data, err := SearchLegacyCases(LegacyFormatCSV, "cases.csv", []byte(legacyCSVFixture), query, 10)
		if err != nil {
			t.Fatal(err)
		}
		var r LegacyCaseSearch
		if err := json.Unmarshal(data, &r); err != nil || len(r.Results) != 1 || r.Results[0].ID != want {
			t.Fatalf("%q: %+v %v", query, r, err)
		}
	}
	none, err := SearchLegacyCases(LegacyFormatTestRailJSON, "exports/cases.json", []byte(legacyTestRailFixture), "Chargeback dispute", 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(none, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.State != "none-found" || receipt.Query != "Chargeback dispute" || strings.Join(receipt.Tokens, " ") != "chargeback dispute" || len(receipt.Searched) != 1 || receipt.Searched[0].Path != "exports/cases.json" || receipt.Searched[0].SHA256 != Digest([]byte(legacyTestRailFixture)) || receipt.Searched[0].Cases != 2 || strings.Join(receipt.Fields, ",") != "title,path,preconditions,steps" || len(receipt.Results) != 0 {
		t.Fatalf("none-found receipt is not auditable: %+v", receipt)
	}
}

// DCP-V1-078: malformed exports, unknown sections, cycles, duplicate IDs and
// bad options are named refusals.
func TestLegacyCasesRefusals(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ format, content, data, query string }{
		"format":           {"xlsx", "", legacyCSVFixture, ""},
		"content":          {LegacyFormatCSV, "steps", legacyCSVFixture, ""},
		"not json":         {LegacyFormatTestRailJSON, "", "[]", ""},
		"unknown section":  {LegacyFormatTestRailJSON, "", `{"suites":[],"sections":[],"cases":[{"id":1,"title":"x","section_id":9}]}`, ""},
		"unknown suite":    {LegacyFormatTestRailJSON, "", `{"suites":[{"id":1,"name":"s"}],"sections":[],"cases":[{"id":1,"title":"x","suite_id":2}]}`, ""},
		"suite, no suites": {LegacyFormatTestRailJSON, "", `{"suites":[],"sections":[],"cases":[{"id":1,"title":"x","suite_id":9}]}`, ""},
		"cycle":            {LegacyFormatTestRailJSON, "", `{"suites":[{"id":1,"name":"s"}],"sections":[{"id":1,"suite_id":1,"parent_id":2,"name":"a"},{"id":2,"suite_id":1,"parent_id":1,"name":"b"}],"cases":[{"id":1,"title":"x","section_id":1}]}`, ""},
		"section suite":    {LegacyFormatTestRailJSON, "", `{"suites":[{"id":1,"name":"s"}],"sections":[{"id":10,"suite_id":9,"name":"a"}],"cases":[{"id":1,"title":"x","suite_id":1,"section_id":10}]}`, ""},
		"duplicate id":     {LegacyFormatCSV, "", "ID,Title\nC1,a\n1,b\n", ""},
		"bad id":           {LegacyFormatCSV, "", "ID,Title\nC01,a\n", ""},
		"no title":         {LegacyFormatCSV, "", "ID,Title\nC1, \n", ""},
		"no columns":       {LegacyFormatCSV, "", "Name\nx\n", ""},
		"empty":            {LegacyFormatCSV, "", "ID,Title\n", ""},
		"empty query":      {LegacyFormatCSV, "", legacyCSVFixture, " - "},
	} {
		t.Run(name, func(t *testing.T) {
			var err error
			if tc.query != "" {
				_, err = SearchLegacyCases(tc.format, "cases", []byte(tc.data), tc.query, 10)
			} else {
				_, err = ImportLegacyCases(tc.format, tc.content, "cases", []byte(tc.data))
			}
			var corpusErr *Error
			if !errors.As(err, &corpusErr) || corpusErr.Code != "corpus-refused" {
				t.Fatalf("want corpus-refused, got %v", err)
			}
		})
	}
}
