package witness

import (
	"strings"
	"testing"
)

func sampleReport() *Report {
	return &Report{
		Profile: Profile,
		Range: Range{
			Base: "b1", BaseTree: "t1", Head: "h1", HeadTree: "t2",
			ChangedPaths: 2, Admission: "COMPUTED", Closure: "COMPUTED",
		},
		Summary:     Summary{Opened: 2, Analysed: 1, Unproven: 1, NotRun: 1, DeterminablePerMille: 500},
		Authorities: authorities,
		Sources:     []Source{{Name: "cem", Status: "BOUND", Path: ".corvint/change.cem.json", Hunks: 1, Basis: 1}},
		Preconditions: []Precondition{
			{ID: "WITNESS-P1-NO-CLOSING-AUTHORITY", Statement: "no authority closes"},
		},
		Obligations: []Obligation{
			{ID: "change:a.go", Kind: KindChange, Path: "internal/a/a.go", Relation: "changed-path", Coverage: "DEEP",
				Verdict: Unproven, Reason: "NO_CLOSING_AUTHORITY", Witnesses: []Witness{
					{Source: "cem", HunkID: "h", Relation: "test-claim", Authority: "CALLER_REPORTED", EvidencePath: "docs/x.md"},
				}},
			{ID: "change:a.md", Kind: KindChange, Path: "docs/a.md", Relation: "changed-path", Coverage: "NONE",
				Verdict: NotRun, Reason: "LANGUAGE_COVERAGE_NONE", Witnesses: []Witness{}},
		},
	}
}

func TestRenderIsByteIdentical(t *testing.T) {
	first, second := Render(sampleReport()), Render(sampleReport())
	if first != second {
		t.Fatal("identical reports must render identical bytes")
	}
}

func TestRenderShowsTheCredibilityRatioAndEveryObligation(t *testing.T) {
	output := Render(sampleReport())
	for _, want := range []string{
		"WITNESS " + Profile,
		"closed-by-other-party  0",
		"not-run                1",
		"determinable           1/2 (50.0%)",
		"proven                 0/1 (0.0%)",
		"AUTHORITY TABLE",
		"CALLER_REPORTED    closing=false",
		"WITNESS-P1-NO-CLOSING-AUTHORITY",
		"UNPROVEN CHANGE cov=DEEP",
		"NOT_RUN  CHANGE cov=NONE",
		"[LANGUAGE_COVERAGE_NONE]",
		"witness cem hunk=h relation=test-claim authority=CALLER_REPORTED closing=false",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered report is missing %q\n---\n%s", want, output)
		}
	}
}

// An empty result must never read as "nothing outstanding".
func TestRenderNamesAnEmptyRangeRatherThanShowingSilence(t *testing.T) {
	report := sampleReport()
	report.Obligations = nil
	report.Preconditions = nil
	output := Render(report)
	if !strings.Contains(output, "(none: the range changes no path)") {
		t.Errorf("an empty obligation list must say so explicitly:\n%s", output)
	}
	if !strings.Contains(output, "PRECONDITIONS\n  (none)") {
		t.Errorf("an empty precondition list must say so explicitly:\n%s", output)
	}
}

func TestShortIdentity(t *testing.T) {
	cases := map[string]string{
		"hunk:sha256:bb93392edc22c3339b7d96e8a8bf5b0e": "bb93392edc22",
		"hunk:sha256:abc": "abc",
		"":                "-",
		"plain":           "plain",
	}
	for input, want := range cases {
		if got := shortIdentity(input); got != want {
			t.Errorf("shortIdentity(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPercentUsesNoFloatingPoint(t *testing.T) {
	cases := map[int]string{0: "0.0%", 5: "0.5%", 500: "50.0%", 763: "76.3%", 1000: "100.0%"}
	for perMille, want := range cases {
		if got := percent(perMille); got != want {
			t.Errorf("percent(%d) = %s, want %s", perMille, got, want)
		}
	}
}
