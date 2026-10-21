package rbmutate

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// The strong spec's examples sit on these lines of fixtureStrongSpec.
const (
	bandsAtThreeLine = 4
	bandsAtFourLine  = 7
	bandsAtEightLine = 13
)

// TestFPKV0075NamedSourceRunsOnlyTheCitedExamples pins FPK-V0-075 and
// FPK-V0-076: an unchanged source judged against two cited examples, with no
// spans, plans over the whole file, keeps the 12-mutant cap, runs every
// mutant through `SPEC:L1:L2` so only those examples run, gives every mutant a
// verdict and a receipt, and writes nothing to the repository.
func TestFPKV0075NamedSourceRunsOnlyTheCitedExamples(t *testing.T) {
	request, root := scoringRequest(t, fixtureStrongSpec, nil)
	request.Examples = []int{bandsAtEightLine, bandsAtThreeLine, bandsAtThreeLine}
	request.LineBasis = LineBasisWholeFile
	before := treeDigest(t, root)
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Target != "spec/models/scoring_spec.rb:4:13" || report.LineBasis != LineBasisWholeFile {
		t.Fatalf("target %q basis %q", report.Target, report.LineBasis)
	}
	if report.Stage != StageMutants || report.Planned != 15 || report.Mutants != defaultMaxMutants || len(report.Results) != defaultMaxMutants {
		t.Fatalf("whole-file plan under the cap: %+v", report)
	}
	if report.Baseline == nil || report.Baseline.Examples != 2 || report.Baseline.Failures != 0 {
		t.Fatalf("baseline must run the two cited examples only: %+v", report.Baseline)
	}
	if report.Killed == 0 || report.Survived == 0 {
		t.Fatalf("two examples must kill some band mutants and miss others: %v", outcomes(report))
	}
	for _, result := range report.Results {
		switch result.Outcome {
		case OutcomeKilled, OutcomeSurvived, OutcomeUnloadable, OutcomeTimedOut:
		default:
			t.Fatalf("mutant without a verdict: %+v", result)
		}
		if result.Receipt.SourceSHA256 == "" || result.Receipt.SourceSHA256 == report.Baseline.SourceSHA256 || result.Receipt.Examples != 2 {
			t.Fatalf("mutant receipt: %+v", result)
		}
	}
	if !strings.Contains(report.Detail, "survived spec/models/scoring_spec.rb:4:13") || !strings.HasSuffix(report.Detail, "; 12 of 15 planned sites selected") {
		t.Fatalf("detail: %q", report.Detail)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("the run wrote the repository")
	}
}

// TestFPKV0076CallerSpansConfineTheNamedSource: caller spans confine the
// mutants exactly as changed lines do, and the band mutants the two cited
// examples cannot see survive.
func TestFPKV0076CallerSpansConfineTheNamedSource(t *testing.T) {
	request, _ := scoringRequest(t, fixtureStrongSpec, bandLines)
	request.Examples = []int{bandsAtThreeLine, bandsAtFourLine}
	request.LineBasis = LineBasisCallerSpans
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	got := outcomes(report)
	if report.Planned != 14 || report.Mutants != 12 || report.LineBasis != LineBasisCallerSpans {
		t.Fatalf("report: %+v", report)
	}
	if got["negate-condition count >= 4->!(count >= 4) @6"] != OutcomeKilled || got["constant-increment 3->4 @5"] != OutcomeSurvived {
		t.Fatalf("outcomes: %v", got)
	}
	for _, result := range report.Results {
		if result.Line < 4 || result.Line > 9 {
			t.Fatalf("mutant outside the caller spans: %+v", result)
		}
	}
}

// TestFPKV0075ExampleSelectionThatRunsNothingIsNotRun: a cited line no
// example or group sits on runs no example, so the baseline is unusable.
func TestFPKV0075ExampleSelectionThatRunsNothingIsNotRun(t *testing.T) {
	request, _ := scoringRequest(t, fixtureStrongSpec, bandLines)
	request.Examples = []int{2}
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != mutate.Unsupported || report.Stage != StageBaseline || report.Detail != "spec file runs no examples: spec/models/scoring_spec.rb:2" {
		t.Fatalf("report: %+v", report)
	}
}

func TestFPKV0075RequestRejectsExampleMistakes(t *testing.T) {
	many := make([]int, maxExamples+1)
	for index := range many {
		many[index] = index + 1
	}
	for _, request := range []Request{
		{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb", Examples: []int{0}},
		{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb", Examples: many},
		{ChangedPath: "lib/a.rb", TestPath: "spec/a_spec.rb", LineBasis: "executed-lines"},
	} {
		if _, err := Judge(context.Background(), fakeCopy{dir: t.TempDir()}, request); err == nil {
			t.Errorf("accepted %+v", request)
		}
	}
}
