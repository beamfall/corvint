package obligation

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// qStep and the other q* types build a json-reporter document of the
// TOL-V0-060 fixture with the shape Playwright 1.61.1 was observed to write.
type qStep struct {
	Title string          `json:"title"`
	Error *map[string]any `json:"error,omitempty"`
	Steps []qStep         `json:"steps"`
}

func qs(title string, failed bool, nested ...qStep) qStep {
	s := qStep{Title: title, Steps: append([]qStep{}, nested...)}
	if failed {
		s.Error = &map[string]any{"message": "expect(received).toBe(expected)"}
	}
	return s
}

type qResult struct {
	Status string  `json:"status"`
	Retry  int     `json:"retry"`
	Steps  []qStep `json:"steps"`
}

type qTest struct {
	ExpectedStatus string           `json:"expectedStatus"`
	ProjectName    string           `json:"projectName"`
	Annotations    []map[string]any `json:"annotations"`
	Results        []qResult        `json:"results"`
}

type qSpec struct {
	Title string  `json:"title"`
	ID    string  `json:"id"`
	File  string  `json:"file"`
	Tests []qTest `json:"tests"`
}

// fixtureSpecs is the expected-outcome report of the fixture; mutate edits
// it before encoding.
func fixtureSpecs() []qSpec {
	soft := []qStep{qs("TOLQ-1 passes", false), qs("TOLQ-2 soft-fails", true), qs("TOLQ-3 passes after a soft failure", false),
		qs("TOLQ-4 parent", true, qs("nested soft failure", true))}
	both := func(status string, steps []qStep) []qResult {
		return []qResult{{Status: status, Retry: 0, Steps: steps}, {Status: status, Retry: 1, Steps: steps}}
	}
	t := func(expected string, rs []qResult) []qTest {
		return []qTest{{ExpectedStatus: expected, ProjectName: "chromium", Annotations: []map[string]any{}, Results: rs}}
	}
	expectedFail := t("failed", []qResult{{Status: "failed", Retry: 0, Steps: []qStep{}}})
	expectedFail[0].Annotations = []map[string]any{{"type": "fail"}}
	return []qSpec{
		{Title: "soft steps", ID: "q1", File: FixtureName, Tests: t("passed", both("failed", soft))},
		{Title: "TOLQ-5 soft failure outside steps", ID: "q2", File: FixtureName, Tests: t("passed", both("failed", []qStep{qs("TOLQ-6 passes", false)}))},
		{Title: "TOLQ-7 fails at retry 0", ID: "q3", File: FixtureName, Tests: t("passed", []qResult{{Status: "failed", Retry: 0, Steps: []qStep{}}, {Status: "passed", Retry: 1, Steps: []qStep{}}})},
		{Title: "TOLQ-8 passes", ID: "q4", File: FixtureName, Tests: t("passed", []qResult{{Status: "passed", Retry: 0, Steps: []qStep{}}})},
		{Title: "TOLQ-9 expected-fail", ID: "q5", File: FixtureName, Tests: expectedFail},
	}
}

func fixtureReport(t *testing.T, version string, specs []qSpec) []byte {
	t.Helper()
	root := filepath.Join(t.TempDir(), "qualify")
	doc := map[string]any{
		"config": map[string]any{"version": version, "rootDir": root},
		"errors": []any{},
		"stats":  map[string]any{"expected": 1},
		"suites": []any{map[string]any{"title": FixtureName, "specs": []any{}, "suites": []any{map[string]any{"title": "qualification", "specs": specs, "suites": []any{}}}}},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func qualifyRaw(t *testing.T, raw []byte) (*Qualification, error) {
	t.Helper()
	r, err := parseReport(raw, nil, true)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Qualify(r)
}

// TestTOLV0060_FixtureExpectedClasses: the embedded fixture names each
// TOLQ id as a whole token, and the expected classes cover every id once.
func TestTOLV0060_FixtureExpectedClasses(t *testing.T) {
	c, f, d := FixtureExpected()
	if strings.Join(c, ",") != "TOLQ-1,TOLQ-3,TOLQ-6,TOLQ-8" || strings.Join(f, ",") != "TOLQ-2,TOLQ-4,TOLQ-5,TOLQ-7" || strings.Join(d, ",") != "TOLQ-9" {
		t.Fatalf("classes %v %v %v", c, f, d)
	}
	for _, e := range fixtureExpected {
		if !strings.Contains(string(Fixture()), "'"+e.id+" ") {
			t.Fatalf("fixture lacks %s", e.id)
		}
	}
	if FixtureSha256() != wire.Sum(Fixture()) {
		t.Fatal("fixture digest differs")
	}
}

// TestTOLV0061_QualifyMatchesFixture: the expected-outcome report qualifies
// with its evidence; every class difference refuses GATE_FAILED with
// OBLIGATION_QUALIFICATION_FAILED: naming the id; a report of another file
// leaves every id missing.
func TestTOLV0061_QualifyMatchesFixture(t *testing.T) {
	raw := fixtureReport(t, "1.64.0", fixtureSpecs())
	q, err := qualifyRaw(t, raw)
	if err != nil {
		t.Fatalf("qualify: %v", err)
	}
	if q.Version != "1.64.0" || q.ReportSha256 != wire.Sum(raw) || q.FixtureSha256 != FixtureSha256() || strings.Join(q.Credited, ",") != "TOLQ-1,TOLQ-3,TOLQ-6,TOLQ-8" {
		t.Fatalf("evidence %+v", q)
	}
	cases := map[string]struct {
		edit func([]qSpec)
		want string
	}{
		"soft parent passes": {func(s []qSpec) { s[0].Tests[0].Results[0].Steps[3].Error = nil }, "TOLQ-4 expected failed, observed credited"},
		"retry 0 credited":   {func(s []qSpec) { s[2].Tests[0].Results[0].Status = "passed" }, "TOLQ-7 expected failed, observed credited"},
		"sibling fails":      {func(s []qSpec) { s[0].Tests[0].Results[0].Steps[2] = qs("TOLQ-3 passes after a soft failure", true) }, "TOLQ-3 expected credited, observed failed"},
		"no expected fail":   {func(s []qSpec) { s[4].Tests[0].ExpectedStatus = "passed"; s[4].Tests[0].Annotations = nil }, "TOLQ-9 expected defectConfirmed"},
		"other file":         {func(s []qSpec) { s[3].File = "other.spec.ts" }, "TOLQ-8 expected credited, observed unbound"},
	}
	for name, c := range cases {
		specs := fixtureSpecs()
		c.edit(specs)
		_, err := qualifyRaw(t, fixtureReport(t, "1.64.0", specs))
		if wire.CodeOf(err) != wire.CodeGateFailed || !strings.Contains(err.Error(), "OBLIGATION_QUALIFICATION_FAILED:") || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// TestTOLV0062_AllowListCodec: the allow-list round-trips canonically, a
// later profile refuses UNSUPPORTED_VERSION, an unknown member or a 65th
// version refuses, and a missing file is an empty list.
func TestTOLV0062_AllowListCodec(t *testing.T) {
	q, err := qualifyRaw(t, fixtureReport(t, "1.64.0", fixtureSpecs()))
	if err != nil {
		t.Fatal(err)
	}
	q.QualifiedAt = "2026-10-18T12:00:00Z"
	p := AllowListPath(t.TempDir())
	a, err := ReadAllowList(p)
	if err != nil || len(a.Entries) != 0 {
		t.Fatalf("missing file: %v %v", a, err)
	}
	if err := a.Put(*q); err != nil {
		t.Fatal(err)
	}
	if err := WriteAllowList(p, a); err != nil {
		t.Fatal(err)
	}
	b, err := ReadAllowList(p)
	if err != nil || strings.Join(b.Versions(), ",") != "1.64.0" || string(wire.Encode(b.Value())) != string(wire.Encode(a.Value())) {
		t.Fatalf("round trip: %v %v", b, err)
	}
	enc := string(wire.EncodeFile(a.Value()))
	for name, c := range map[string]struct{ doc, code string }{
		"newer profile":  {strings.Replace(enc, ProfileQualifications, "taskman-obligation-qualifications/1", 1), wire.CodeUnsupportedVersion},
		"unknown member": {strings.Replace(enc, `"qualifiedAt"`, `"extra":"x","qualifiedAt"`, 1), wire.CodeMalformed},
		"bad timestamp":  {strings.Replace(enc, "2026-10-18T12:00:00Z", "yesterday", 1), wire.CodeMalformed},
	} {
		if _, err := DecodeAllowList([]byte(c.doc)); wire.CodeOf(err) != c.code {
			t.Fatalf("%s: %v", name, err)
		}
	}
	full := &AllowList{Entries: map[string]Qualification{}}
	for i := 0; i < MaxQualifiedVersions; i++ {
		v := *q
		v.Version = "1." + strings.Repeat("9", i+1)
		if err := full.Put(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := full.Put(*q); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("65th version: %v", err)
	}
}
