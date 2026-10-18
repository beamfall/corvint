package testvalidity

// RSpec example hygiene and strength source facts (docs/specs/test-source-analysis-v0.md,
// TSA-V0-062..072). internal/testsource reads them from committed spec source; this file holds the
// vocabulary and the one place they reach a projection. An invalid rule here uses its rule ID as
// the hygiene reason (TSA-V0-051).

// RSpec example rule IDs.
const (
	// RuleRSpecSkipped: the example is skipped or pending by declaration, metadata, an x-prefixed
	// ancestor group or an unconditional skip/pending statement (TSA-V0-063).
	RuleRSpecSkipped = "RSPEC-SKIPPED"
	// RuleRSpecFocused: the example or an ancestor group is focused (TSA-V0-064).
	RuleRSpecFocused = "RSPEC-FOCUSED"
	// RuleSharedExampleOnly: the example is declared inside a shared example group, so its
	// assertion is reachable only through an including group (TSA-V0-065).
	RuleSharedExampleOnly = "SHARED-EXAMPLE-ONLY"
	// RuleDescribedClassStubbed: the example or a hook on its chain stubs described_class, so an
	// expectation can observe the stub instead of the unit under test (TSA-V0-066).
	RuleDescribedClassStubbed = "DESCRIBED-CLASS-STUBBED"
)

// Hygiene and strength reasons of the RSpec example facts (TSA-V0-068..070).
const (
	HygieneReasonRSpecPending         = "rspec-example-pending"
	HygieneReasonRSpecSourceUnbound   = "rspec-source-not-bound"
	HygieneReasonRSpecExampleUnplaced = "rspec-example-not-located"
	StrengthReasonDescribedClassStub  = "described-class-stubbed"
	StrengthReasonNoBodyExpectation   = "no-expectation-in-example-body"
	StrengthReasonBodyExpectations    = "example-body-expectations"
	// ExampleSourceLocated is ExampleSource.Read for an example the reader placed.
	ExampleSourceLocated = "located"
)

// ExampleSource is the static source reading of one cited RSpec example. Read is
// ExampleSourceLocated or the abstention reason. Expectations counts the expectations in the
// example body; ExpectationAnchors holds their distinct `path:line` locations. Findings are every
// source finding naming the example.
type ExampleSource struct {
	Read               string          `json:"read"`
	Expectations       int             `json:"expectations"`
	Findings           []SourceFinding `json:"findings"`
	ExpectationAnchors []string        `json:"-"`
}

// ApplyExampleSource returns p with hygiene and strength recomputed from one example's source
// facts (TSA-V0-068, TSA-V0-069). anchor is the example's `path:line`. An INELIGIBLE hygiene axis
// is never relaxed. A located example becomes ELIGIBLE and then takes its findings through
// ApplySourceFindings; an unlocated one is ABSTAINED with the read reason. Strength stays
// NOT_MEASURED: only a mutation run decides it.
func ApplyExampleSource(p Projection, source ExampleSource, anchor string) Projection {
	if source.Read != ExampleSourceLocated {
		if p.Hygiene.State != HygieneIneligible {
			p.Hygiene = Axis{State: HygieneAbstained, Reason: source.Read, Anchors: []string{anchor}}
		}
		return p
	}
	if p.Hygiene.State != HygieneIneligible {
		p.Hygiene = Axis{State: HygieneEligible, Reason: "rspec-example-source-read", Anchors: []string{anchor}}
	}
	p = ApplySourceFindings(p, source.Findings)
	if p.Strength.State != StrengthNotMeasured {
		return p
	}
	stubs := []string{}
	for _, f := range source.Findings {
		if f.Rule == RuleDescribedClassStubbed {
			stubs = append(stubs, f.Location)
		}
	}
	switch {
	case len(stubs) > 0:
		p.Strength = Axis{State: StrengthNotMeasured, Reason: StrengthReasonDescribedClassStub, Anchors: stubs}
	case source.Expectations == 0:
		p.Strength = Axis{State: StrengthNotMeasured, Reason: StrengthReasonNoBodyExpectation, Anchors: []string{anchor}}
	default:
		p.Strength = Axis{State: StrengthNotMeasured, Reason: StrengthReasonBodyExpectations, Anchors: append([]string{}, source.ExpectationAnchors...)}
	}
	return p
}
