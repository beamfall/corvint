package testvalidity

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

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
	// RuleRSpecConditionalSkip: the example body skips or is pending behind a trailing if or
	// unless, or inside a nested block, so whether it runs is undecided; it abstains (TSA-V0-450).
	RuleRSpecConditionalSkip = "RSPEC-CONDITIONAL-SKIP"
	// RuleRSpecConfigExcluded: the example or an ancestor group carries metadata that a committed
	// support file's filter_run_excluding removes from a run; it abstains (TSA-V0-450).
	RuleRSpecConfigExcluded = "RSPEC-CONFIG-EXCLUDED"
	// RuleSharedExampleOnly: the example is declared inside a shared example group, so its
	// assertion is reachable only through an including group (TSA-V0-065).
	RuleSharedExampleOnly = "SHARED-EXAMPLE-ONLY"
	// RuleDescribedClassStubbed: the example or a hook on its chain stubs described_class, so an
	// expectation can observe the stub instead of the unit under test (TSA-V0-066).
	RuleDescribedClassStubbed = "DESCRIBED-CLASS-STUBBED"
	// RuleRBBlockNeverYielded: the example passes a block to a helper whose definition takes no
	// block parameter and never yields, so Ruby discards the block (TSA-V0-084).
	RuleRBBlockNeverYielded = "RB-BLOCK-NEVER-YIELDED"
	// RuleRBChangeOnIdentity: a negated change matcher observes a record or its reload; it may
	// compare by identity and miss attribute changes, so it is advisory only (TSA-V0-085).
	RuleRBChangeOnIdentity = "RB-CHANGE-ON-IDENTITY"
	// RuleRBAssignmentInPredicate: a satisfy block ends in an assignment of a truthy literal, so
	// it is always truthy (TSA-V0-086).
	RuleRBAssignmentInPredicate = "RB-ASSIGNMENT-IN-PREDICATE"
	// RuleRBRaiseErrorOnly: every expectation of the example is a negated raise_error; advisory
	// only (TSA-V0-087).
	RuleRBRaiseErrorOnly = "RB-RAISE-ERROR-ONLY"
	// RuleRBHelperUnresolved and RuleRBHelperAmbiguous: the example passes a block to a helper the
	// reader cannot resolve to one bound definition, so it abstains (TSA-V0-088).
	RuleRBHelperUnresolved = "RB-HELPER-UNRESOLVED"
	RuleRBHelperAmbiguous  = "RB-HELPER-AMBIGUOUS"
	// RuleRBDenialWithoutNoEffect: a denial example asserts only the refusal status of a
	// mutating request and nothing about its effect; advisory only (TSA-V0-564).
	RuleRBDenialWithoutNoEffect = "RB-DENIAL-WITHOUT-NO-EFFECT"
	// RuleRBDenialVerbUnresolved and RuleRBDenialMatcherCustom: the reader cannot name the
	// request verb a denial example observes, or its denial matcher is a custom one, so it
	// abstains from TSA-V0-564 with an advisory finding that changes no axis (TSA-V0-565).
	RuleRBDenialVerbUnresolved = "RB-DENIAL-VERB-UNRESOLVED"
	RuleRBDenialMatcherCustom  = "RB-DENIAL-MATCHER-CUSTOM"
)

// The reasons a shared example evaluated through one inclusion site abstains (TSA-V0-091,
// TSA-V0-092).
const (
	InclusionNotFound            = "rspec-inclusion-not-found"
	InclusionInExampleOrHook     = "rspec-inclusion-in-example-or-hook"
	InclusionLoopGenerated       = "rspec-inclusion-loop-generated"
	InclusionNameNotLiteral      = "rspec-inclusion-name-not-literal"
	SharedGroupNotFound          = "rspec-shared-group-not-found"
	SharedGroupAmbiguous         = "rspec-shared-group-ambiguous"
	SharedGroupNested            = "rspec-shared-group-nested"
	SharedGroupUnbound           = "rspec-shared-group-unbound"
	InclusionExampleNotInGroup   = "rspec-inclusion-example-not-in-group"
	InclusionExampleLoop         = "rspec-example-loop-generated"
	InclusionIDUnplaced          = "rspec-inclusion-id-unplaced"
	InclusionDescriptionMismatch = "rspec-inclusion-description-mismatch"
	InclusionSourceUnbound       = "rspec-inclusion-source-unbound"
)

// RSpecInclusion names one shared example read through one inclusion site (TSA-V0-089): the
// including spec, the shared group's file and the example's declaring line in it, and either the
// inclusion's line (IncludeLine) or the receipt's scoped id path and full description
// (ScopedID, Description; TSA-V0-092). Support is the bound support set (TSA-V0-088).
type RSpecInclusion struct {
	SpecPath, SpecText     string
	IncludeLine            int
	ScopedID               []int
	Description            string
	SharedPath, SharedText string
	ExampleLine            int
	Support                RubySupport
}

// Ref is the qualified reference `SPEC:INCLUDE_LINE>SHARED:EXAMPLE_LINE`, with `same` for a shared
// group in the including spec (TSA-V0-089).
func (q RSpecInclusion) Ref(includeLine int) string {
	shared := q.SharedPath
	if shared == q.SpecPath {
		shared = "same"
	}
	return fmt.Sprintf("%s:%d>%s:%d", q.SpecPath, includeLine, shared, q.ExampleLine)
}

// RubySupport is the bound support Ruby source an RSpec example can reach without a require the
// reader follows: spec/spec_helper.rb, spec/rails_helper.rb and spec/support/**/*.rb, keyed by
// path. Unbound is the reason the set could not be bound, or "" (TSA-V0-088).
type RubySupport struct {
	Files   map[string]string
	Unbound string
	// Flags is the flag lookup list stubs are classified against (TSA-V0-435); nil uses
	// BuiltinFlagLookups.
	Flags *FlagLookups
	// App is the committed application source a never-written assertion target is resolved in
	// (TSA-V0-448); nil reads none, and RB-ASSERT-TARGET-NEVER-WRITTEN is never reported.
	App *RubyApp
}

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
	// Cause is the file behind an abstention, when one file is: a support file that did not read
	// and could define the shared group (TSA-V0-397). It is anchored after the example.
	Cause string `json:"-"`
	// EndLine is a JS/TS example's last declaring line, so a failure frame inside it can be
	// placed (TSA-V0-082). The RSpec reader leaves it zero.
	EndLine int `json:"-"`
	// TitlePath is the example's literal title path, outermost group first and its own title
	// last (LPCV-V0-147). It is nil when any title on the path is not a literal string.
	TitlePath []string `json:"-"`
	// Conditions are the flag values the example forces, in stub line order (TSA-V0-437).
	Conditions []FlagCondition `json:"conditions,omitempty"`
}

// anchors is the example's anchor followed by the abstention's cause, when it has one.
func (source ExampleSource) anchors(anchor string) []string {
	if source.Cause == "" {
		return []string{anchor}
	}
	return []string{anchor, source.Cause}
}

// ApplyExampleSource returns p with hygiene and strength recomputed from one example's source
// facts (TSA-V0-068, TSA-V0-069). anchor is the example's `path:line`. An INELIGIBLE hygiene axis
// is never relaxed. A located example becomes ELIGIBLE and then takes its findings through
// ApplySourceFindings; an unlocated one is ABSTAINED with the read reason. Strength stays
// NOT_MEASURED: only a mutation run decides it.
func ApplyExampleSource(p Projection, source ExampleSource, anchor string) Projection {
	if source.Read != ExampleSourceLocated {
		if p.Hygiene.State != HygieneIneligible {
			p.Hygiene = Axis{State: HygieneAbstained, Reason: source.Read, Anchors: source.anchors(anchor)}
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

// rspecScopedID is an RSpec receipt id: `./PATH[1:2:3]`.
var rspecScopedID = regexp.MustCompile(`^(?:\./)?([^\[\]]+)\[([1-9][0-9]{0,5}(?::[1-9][0-9]{0,5}){0,31})\]$`)

// ParseRSpecScopedID splits an RSpec receipt id into its project-relative path and scoped id path
// (TSA-V0-092).
func ParseRSpecScopedID(id string) (string, []int, bool) {
	m := rspecScopedID.FindStringSubmatch(id)
	if m == nil {
		return "", nil, false
	}
	ids := []int{}
	for _, part := range strings.Split(m[2], ":") {
		n, _ := strconv.Atoi(part)
		ids = append(ids, n)
	}
	return m[1], ids, true
}
