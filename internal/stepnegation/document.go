// Package stepnegation implements the pure half of step-level negative
// controls by fault injection (docs/specs/live-proof-carrying-verification-v0.md
// LPCV-V0-057 to LPCV-V0-070): the closed corvint-step-negation/0 document and
// its canonical codec, fault derivation from a parsed Playwright baseline
// trace, the single-step and joint pass rules, retention under
// .corvint/strength-evidence, and the strength aggregation a qualified join
// applies. It runs no test and starts no process; the companion provider
// (internal/jstestprovider) executes Playwright and feeds observations here.
//
// Every result is a hypothesis-bound witness: a KILLED step establishes only
// that the named assertion distinguished the injected fault (LPCV-V0-047),
// never test adequacy.
package stepnegation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

const (
	// Schema names the emitted document (LPCV-V0-065).
	Schema = "corvint-step-negation/0"
	// MaxDocumentBytes bounds one encoded document.
	MaxDocumentBytes = 4 << 20

	ModeStep     = "step"
	ModeAllSteps = "all-steps"

	WitnessNetwork      = "network"
	WitnessDOM          = "dom"
	WitnessJointNetwork = "joint-network"
	WitnessJointDOM     = "joint-dom"
	WitnessNone         = "none"

	RequiresManualControl = "manual-control"
	RequiresSingleStepRun = "single-step-run"
)

// Step-level strength reasons (LPCV-V0-058 to LPCV-V0-066).
const (
	ReasonKilled                     = "witnessed-step-fault-kill"
	ReasonUnderivable                = "step-fault-underivable"
	ReasonRunBudgetExhausted         = "run-budget-exhausted"
	ReasonBaselineNotPassing         = "baseline-not-passing"
	ReasonBaselineInfrastructure     = "baseline-infrastructure"
	ReasonFaultNotApplied            = "fault-not-applied"
	ReasonFaultNotStepIsolated       = "fault-not-step-isolated"
	ReasonUnattributedFailure        = "unattributed-failure"
	ReasonFaultedRunInfrastructure   = "faulted-run-infrastructure"
	ReasonFaultedRunTimeout          = "faulted-run-timeout"
	ReasonFaultedRunRetried          = "faulted-run-retried"
	ReasonNetworkFaultSurvived       = "network-fault-survived"
	ReasonRequiresSingleStepRun      = "requires-single-step-run"
	ReasonJointAttributionFailed     = "joint-attribution-failed"
	ReasonStepNotReached             = "step-not-reached"
	ReasonJointSurvivalUnconfirmed   = "joint-survival-unconfirmed"
	ReasonIdentityDrift              = "negate-identity-drift"
	ReasonFaultSurvived              = "witnessed-step-fault-survived"
	ReasonStepNotFound               = "step-not-found"
	ReasonStepTitleAmbiguous         = "step-title-ambiguous"
	ReasonStepHasNoAssertion         = "step-has-no-assertion"
	ReasonNegatedAssertion           = "negated-assertion-unsupported"
	ReasonExpectedValueNotLiteral    = "expected-value-not-literal"
	ReasonNoNetworkDependency        = "no-network-dependency"
	ReasonNetworkDependencyAmbiguous = "network-dependency-ambiguous"
	ReasonOutsideApplicationOrigin   = "dependency-outside-application-origin"
	ReasonFaultDoesNotFalsify        = "fault-does-not-falsify"
	ReasonNoDOMFault                 = "no-dom-fault"
	ReasonFaultNotInstallable        = "fault-not-installable"
)

// Document is the closed corvint-step-negation/0 document.
type Document struct {
	Schema            string    `json:"schema"`
	Binding           Binding   `json:"binding"`
	Mode              string    `json:"mode"`
	Runs              Runs      `json:"runs"`
	Inventory         Inventory `json:"inventory"`
	Steps             []Step    `json:"steps"`
	CleanupIncomplete bool      `json:"cleanupIncomplete,omitempty"`
}

// Runs states the run budget and the runs actually used (LPCV-V0-058,
// LPCV-V0-064).
type Runs struct {
	Budget         int `json:"budget"`
	Used           int `json:"used"`
	BaselineRepeat int `json:"baselineRepeat"`
	BaselinePassed int `json:"baselinePassed"`
}

// Inventory is the complete baseline step tree in reporter pre-order.
type Inventory struct {
	Steps                  []InventoryStep `json:"steps"`
	AssertionsOutsideSteps int             `json:"assertionsOutsideSteps"`
}

// InventoryStep is one test.step: its screened title, 1-based pre-order
// ordinal, and the count of expect steps whose innermost test.step it is.
type InventoryStep struct {
	Title      string `json:"title"`
	Ordinal    int    `json:"ordinal"`
	Assertions int    `json:"assertions"`
}

// Step is one measured step entry. PlanDigest is empty when no fault plan
// was derived.
type Step struct {
	PlanDigest string            `json:"planDigest"`
	Title      string            `json:"title"`
	Ordinal    int               `json:"ordinal"`
	Witness    string            `json:"witness"`
	Strength   testvalidity.Axis `json:"strength"`
	Requires   string            `json:"requires,omitempty"`
	PlanReused bool              `json:"planReused,omitempty"`
}

// Binding is the common execution context of every step entry (LPCV-V0-066).
type Binding struct {
	TestRepository        Repository  `json:"testRepository"`
	ConfigFile            string      `json:"configFile"`
	ConfigDigest          string      `json:"configDigest"`
	SpecFile              string      `json:"specFile"`
	SpecDigest            string      `json:"specDigest"`
	Test                  TestBinding `json:"test"`
	Runner                Runner      `json:"runner"`
	Application           Application `json:"application"`
	ReadinessOrigin       string      `json:"readinessOrigin"`
	InjectionModuleDigest string      `json:"injectionModuleDigest"`
}

// Repository is a clean test repository identity.
type Repository struct {
	RootCommit string `json:"rootCommit"`
	Revision   string `json:"revision"`
	Tree       string `json:"tree"`
}

// TestBinding is the full test identity: the canonical key plus the resolved
// browser and device configuration (PWP-V0-003).
type TestBinding struct {
	File      string `json:"file"`
	FullTitle string `json:"fullTitle"`
	Project   string `json:"project"`
	Browser   string `json:"browser"`
	Device    string `json:"device"`
}

// Runner is the runner tuple the runs used.
type Runner struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	NodeVersion string `json:"nodeVersion"`
	Tuple       string `json:"tuple"`
}

// Application is the external application identity: the /0 declared label,
// which stays a caller assertion, or the /1 attested repository and instance.
type Application struct {
	Profile         string `json:"profile"`
	Label           string `json:"label,omitempty"`
	RootCommit      string `json:"rootCommit,omitempty"`
	Revision        string `json:"revision,omitempty"`
	Tree            string `json:"tree,omitempty"`
	InstanceKind    string `json:"instanceKind,omitempty"`
	InstanceID      string `json:"instanceId,omitempty"`
	StartGeneration string `json:"startGeneration,omitempty"`
}

var (
	sha256Pattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	strengthStates    = map[string]bool{testvalidity.StrengthKilled: true, testvalidity.StrengthSurvived: true, testvalidity.StrengthNotMeasured: true, testvalidity.StrengthUnsupported: true}
	unresolvedReasons = map[string]bool{ReasonStepNotFound: true, ReasonStepTitleAmbiguous: true, ReasonBaselineNotPassing: true, ReasonBaselineInfrastructure: true, ReasonIdentityDrift: true}
	witnesses         = map[string]bool{WitnessNetwork: true, WitnessDOM: true, WitnessJointNetwork: true, WitnessJointDOM: true, WitnessNone: true}
)

// Encode returns the canonical bytes: compact encoding/json plus LF.
func Encode(document Document) ([]byte, error) {
	if err := Validate(document); err != nil {
		return nil, err
	}
	data, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if len(data) > MaxDocumentBytes {
		return nil, errors.New("step-negation document exceeds 4 MiB")
	}
	return data, nil
}

// Decode accepts exactly one closed canonical document: unknown fields,
// trailing data, noncanonical bytes and oversized input are refused.
func Decode(data []byte) (Document, error) {
	if len(data) > MaxDocumentBytes {
		return Document{}, errors.New("step-negation document exceeds 4 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document Document
	if err := decoder.Decode(&document); err != nil {
		return Document{}, fmt.Errorf("step-negation document does not decode: %w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return Document{}, errors.New("step-negation document carries data after the document")
	}
	canonical, err := Encode(document)
	if err != nil {
		return Document{}, err
	}
	if !bytes.Equal(canonical, data) {
		return Document{}, errors.New("step-negation document is not canonical")
	}
	return document, nil
}

// Validate checks the closed vocabulary and internal consistency.
func Validate(document Document) error {
	if document.Schema != Schema {
		return errors.New("step-negation schema is not " + Schema)
	}
	if document.Mode != ModeStep && document.Mode != ModeAllSteps {
		return errors.New("step-negation mode is neither step nor all-steps")
	}
	runs := document.Runs
	if runs.Budget < 1 || runs.Used < 0 || runs.Used > runs.Budget || runs.BaselineRepeat < 1 || runs.BaselineRepeat > 5 || runs.BaselinePassed < 0 || runs.BaselinePassed > runs.BaselineRepeat {
		return errors.New("step-negation runs are inconsistent")
	}
	if document.Binding.InjectionModuleDigest != "" && !sha256Pattern.MatchString(document.Binding.InjectionModuleDigest) {
		return errors.New("step-negation injection module digest is not SHA-256")
	}
	if document.Inventory.AssertionsOutsideSteps < 0 {
		return errors.New("step-negation inventory count is negative")
	}
	titles := map[int]string{}
	for index, step := range document.Inventory.Steps {
		if step.Ordinal != index+1 || step.Assertions < 0 {
			return errors.New("step-negation inventory ordinals are not 1..n")
		}
		titles[step.Ordinal] = step.Title
	}
	if document.Steps == nil {
		return errors.New("step-negation steps must be an array")
	}
	previous := -1
	for _, step := range document.Steps {
		if step.Ordinal <= previous {
			return errors.New("step-negation steps are not strictly ordered by ordinal")
		}
		previous = step.Ordinal
		if step.Ordinal == 0 {
			// A requested step title the baseline tree does not resolve to
			// exactly one step has no ordinal; it carries only its reason.
			if step.PlanDigest != "" || step.Witness != WitnessNone || step.Strength.State != testvalidity.StrengthNotMeasured || !unresolvedReasons[step.Strength.Reason] {
				return errors.New("step-negation unresolved step entry is malformed")
			}
		} else if title, ok := titles[step.Ordinal]; !ok || title != step.Title {
			return errors.New("step-negation step is not in the inventory")
		}
		if step.PlanDigest != "" && !sha256Pattern.MatchString(step.PlanDigest) {
			return errors.New("step-negation plan digest is not SHA-256")
		}
		if !witnesses[step.Witness] {
			return errors.New("step-negation witness is outside the closed set")
		}
		if step.Requires != "" && step.Requires != RequiresManualControl && step.Requires != RequiresSingleStepRun {
			return errors.New("step-negation requires is outside the closed set")
		}
		if !strengthStates[step.Strength.State] || step.Strength.Reason == "" || step.Strength.Anchors == nil {
			return errors.New("step-negation strength axis is malformed")
		}
		if step.Strength.State == testvalidity.StrengthKilled && (step.Strength.Reason != ReasonKilled || step.Witness == WitnessNone || step.PlanDigest == "") {
			return errors.New("step-negation KILLED step lacks its witness")
		}
		if step.Strength.State == testvalidity.StrengthSurvived && (step.Witness == WitnessNone || step.PlanDigest == "") {
			return errors.New("step-negation SURVIVED step lacks its witness")
		}
	}
	return nil
}
