package tcq

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"
)

// The expectations in testdata/oracle-junit.json are captured from
// src/context_corvint_test_claim_junit.py, including every row ID.

type oracleJUnit struct {
	Status map[string]struct {
		Raw   string `json:"raw"`
		Error string `json:"error"`
		Rows  []struct {
			ExecutionKey string `json:"executionKey"`
			ID           string `json:"id"`
			Status       string `json:"status"`
		} `json:"rows"`
		Unkeyed         int    `json:"unkeyed"`
		HasNonPassing   bool   `json:"hasNonPassing"`
		ObservationRows string `json:"observationRows"`
	} `json:"status"`
	Grammar map[string]struct {
		Raw     string `json:"raw"`
		Error   string `json:"error"`
		Keyed   int    `json:"keyed"`
		Unkeyed int    `json:"unkeyed"`
	} `json:"grammar"`
	ExecutionKey string `json:"executionKey"`
}

// junitFixtureRepository is the same tree the oracle vectors were generated
// against: one tracked blob, one symlink-mode entry, one directory.
func junitFixtureRepository() *fixtureRepository {
	return &fixtureRepository{
		blobs: map[string][]byte{},
		entries: map[string]TreeEntry{
			"pkg/a_test.go":    {Mode: "100644", Type: "blob", Found: true},
			"pkg/link_test.go": {Mode: "120000", Type: "blob", Found: true},
			"pkg":              {Mode: "040000", Type: "tree", Found: true},
		},
	}
}

func loadJUnitVectors(t *testing.T) oracleJUnit {
	t.Helper()
	raw, err := os.ReadFile("testdata/oracle-junit.json")
	if err != nil {
		t.Fatalf("read oracle junit vectors: %v", err)
	}
	var vectors oracleJUnit
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("decode oracle junit vectors: %v", err)
	}
	return vectors
}

const junitTarget = "2222222222222222222222222222222222222222"

// TestJUnitStatusTable covers the complete TCQ-V0-029 status table plus the
// TCQ-V0-028 keying rules, including the paths that must stay unkeyed.
func TestJUnitStatusTable(t *testing.T) {
	vectors := loadJUnitVectors(t)
	repository := junitFixtureRepository()
	for name, expected := range vectors.Status {
		t.Run(name, func(t *testing.T) {
			report, err := parseJUnit(repository, junitTarget, []byte(expected.Raw))
			if expected.Error != "" {
				requireCode(t, err, expected.Error)
				return
			}
			if err != nil {
				t.Fatalf("parseJUnit: %v", err)
			}
			if len(report.keyed) != len(expected.Rows) {
				t.Fatalf("keyed rows = %d, oracle %d", len(report.keyed), len(expected.Rows))
			}
			for index, row := range report.keyed {
				want := expected.Rows[index]
				if row.executionKey != want.ExecutionKey || row.id != want.ID || row.status != want.Status {
					t.Errorf("row[%d] = %+v, oracle %+v", index, row, want)
				}
			}
			if report.unkeyedCount != expected.Unkeyed {
				t.Errorf("unkeyed = %d, oracle %d", report.unkeyedCount, expected.Unkeyed)
			}
			if report.hasNonPassing != expected.HasNonPassing {
				t.Errorf("hasNonPassing = %v, oracle %v", report.hasNonPassing, expected.HasNonPassing)
			}
			got := string(canonicalValue(jsonArray(observationRows(report))))
			if got != expected.ObservationRows {
				t.Errorf("observation rows = %s, oracle %s", got, expected.ObservationRows)
			}
		})
	}
}

// TestJUnitGrammar covers TCQ-V0-026's byte preflight and TCQ-V0-027's closed
// element grammar: DTD, entity, second processing instruction, namespace,
// unknown root, and misplaced children are all refused before any row exists.
func TestJUnitGrammar(t *testing.T) {
	vectors := loadJUnitVectors(t)
	repository := junitFixtureRepository()
	for name, expected := range vectors.Grammar {
		t.Run(name, func(t *testing.T) {
			report, err := parseJUnit(repository, junitTarget, []byte(expected.Raw))
			if expected.Error != "" {
				requireCode(t, err, expected.Error)
				return
			}
			if err != nil {
				t.Fatalf("parseJUnit: %v", err)
			}
			if len(report.keyed) != expected.Keyed || report.unkeyedCount != expected.Unkeyed {
				t.Errorf("keyed/unkeyed = %d/%d, oracle %d/%d",
					len(report.keyed), report.unkeyedCount, expected.Keyed, expected.Unkeyed)
			}
		})
	}
}

// TestJUnitResourceBounds covers TCQ-V0-041: the report ceiling and the XML
// depth bound are enforced during the single traversal.
func TestJUnitResourceBounds(t *testing.T) {
	repository := junitFixtureRepository()
	oversized := make([]byte, maxReportBytes+1)
	requireCode(t, mustFail(parseJUnit(repository, junitTarget, oversized)), CodeResourceExhausted)

	deep := []byte("<testsuites>")
	for index := 0; index < maxXMLDepth+2; index++ {
		deep = append(deep, []byte(`<testsuite name="s">`)...)
	}
	requireCode(t, mustFail(parseJUnit(repository, junitTarget, deep)), CodeResourceExhausted)
}

// TestJUnitBoundsAttributesBeforeDecoding covers TCQ-V0-027/041: a start tag
// with more than 64 attributes is refused from the bytes, before Go's decoder
// allocates one attribute per `name=""` pair of a 4 MiB report.
func TestJUnitBoundsAttributesBeforeDecoding(t *testing.T) {
	repository := junitFixtureRepository()
	raw := []byte("<testsuite" + strings.Repeat(` a=""`, (maxReportBytes-16)/5) + "/>")
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := parseJUnit(repository, junitTarget, raw)
	runtime.ReadMemStats(&after)
	requireCode(t, err, CodeResourceExhausted)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 2*maxReportBytes {
		t.Fatalf("parseJUnit allocated %d bytes before refusing the attribute count", allocated)
	}
	quoted := []byte(`<testsuite name="a=b>c"><system-out><![CDATA[<x a=1>]]>` + strings.Repeat("=", 2*maxXMLAttributes) + `></system-out><testcase file="pkg/a_test.go" name="TestA"/></testsuite>`)
	if _, err := parseJUnit(repository, junitTarget, quoted); err != nil {
		t.Fatalf("quoted and CDATA equals signs: %v", err)
	}
}

// TestJUnitRefusesAmbiguousStatusAttributes covers TCQ-V0-027/029: Go's
// decoder admits a repeated attribute that XML forbids, and a prefixed
// attribute is not the raw `status`, so neither may coerce a failing testcase
// into a passing row.
func TestJUnitRefusesAmbiguousStatusAttributes(t *testing.T) {
	repository := junitFixtureRepository()
	duplicate := []byte(`<testsuite><testcase file="pkg/a_test.go" name="TestA" status="failed" status="passed"/></testsuite>`)
	requireCode(t, mustFail(parseJUnit(repository, junitTarget, duplicate)), CodeInvalidJUnit)

	prefixed := []byte(`<testsuite xmlns:x="urn:x"><testcase file="pkg/a_test.go" name="TestA" x:status="skipped"><failure/></testcase></testsuite>`)
	report, err := parseJUnit(repository, junitTarget, prefixed)
	if err != nil {
		t.Fatalf("parseJUnit: %v", err)
	}
	if len(report.keyed) != 1 || report.keyed[0].status != "FAILED" {
		t.Fatalf("keyed rows = %+v, want one FAILED row", report.keyed)
	}
}

func TestJUnitRefusesContentOutsideTheSingleRoot(t *testing.T) {
	repository := junitFixtureRepository()
	for _, raw := range []string{
		`<testsuite><testcase file="pkg/a_test.go" name="TestA"/></testsuite><testsuite/>`,
		`<testsuite><testcase file="pkg/a_test.go" name="TestA"/></testsuite>junk`,
		`junk<testsuite><testcase file="pkg/a_test.go" name="TestA"/></testsuite>`,
	} {
		requireCode(t, mustFail(parseJUnit(repository, junitTarget, []byte(raw))), CodeInvalidJUnit)
	}
	if _, err := parseJUnit(repository, junitTarget, []byte("\n<testsuite/><!-- end -->\n")); err != nil {
		t.Fatalf("whitespace and comments around the root: %v", err)
	}
}

func mustFail(_ junitReport, err error) error { return err }

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got success", code)
	}
	typed, ok := err.(*Error)
	if !ok || typed.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}
