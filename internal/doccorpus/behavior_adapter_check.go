package doccorpus

import (
	"bytes"
	"errors"
	"strconv"
)

// BehaviorAdapterCheckSchema identifies the DCP-V1-044 check report. A check
// validates a whole behavior-adapter request and lists every refusal it can
// determine without producing an adapter result.
const BehaviorAdapterCheckSchema = "corvint-behavior-adapter-check/1"

const (
	behaviorCheckPassed        = "passed"
	behaviorCheckRefused       = "refused"
	behaviorCheckNotEvaluated  = "not-evaluated"
	behaviorCheckNotApplicable = "not-applicable"
)

type BehaviorAdapterCheck struct {
	Schema      string                      `json:"schema"`
	Accepted    bool                        `json:"accepted"`
	Stages      []BehaviorAdapterCheckStage `json:"stages"`
	Refusals    []BehaviorAdapterRefusal    `json:"refusals"`
	Fallback    string                      `json:"fallback"`
	Limitations []string                    `json:"limitations"`
}

// BehaviorAdapterCheckStage reports one build stage in build order.
// BlockedBy names the earlier stages that kept it from being evaluated.
type BehaviorAdapterCheckStage struct {
	Stage     string   `json:"stage"`
	State     string   `json:"state"`
	BlockedBy []string `json:"blocked_by"`
}

// BehaviorAdapterRefusal is one refusal, or one check that could not be
// evaluated because an earlier stage refused.
type BehaviorAdapterRefusal struct {
	Stage     string   `json:"stage"`
	State     string   `json:"state"`
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	BlockedBy []string `json:"blocked_by"`
}

type behaviorAdapterChecker struct {
	report BehaviorAdapterCheck
	states map[string]string
	items  []BehaviorAdapterRefusal
	start  int
}

// CheckBehaviorAdapter evaluates the stages of BuildBehaviorAdapter in the
// same order and reports every refusal. The report is accepted exactly when
// BuildBehaviorAdapter would succeed, and its first refusal is the error that
// BuildBehaviorAdapter would return.
func CheckBehaviorAdapter(requestRaw, previousRaw []byte) BehaviorAdapterCheck {
	c := &behaviorAdapterChecker{states: map[string]string{}, report: BehaviorAdapterCheck{
		Schema: BehaviorAdapterCheckSchema, Stages: []BehaviorAdapterCheckStage{}, Refusals: []BehaviorAdapterRefusal{},
		Fallback: "full-relevant-suite", Limitations: behaviorAdapterCheckLimitations(),
	}}
	var request BehaviorAdapterRequest
	c.stage("request", nil, nil, func() error { return decode(requestRaw, &request) })
	var previous *BehaviorAdapterResult
	if len(bytes.TrimSpace(previousRaw)) == 0 {
		c.skip("previous")
	} else {
		c.stage("previous", nil, nil, func() error {
			var decoded BehaviorAdapterResult
			if err := decode(previousRaw, &decoded); err != nil {
				return fail("invalid previous behavior adapter result")
			}
			previous = &decoded
			return nil
		})
	}
	a := newBehaviorAdapterState(request)
	a.checking = true
	run := func(name string, needs []string, check func() error) {
		c.stage(name, needs, a, check)
	}
	run("identity", []string{"request"}, func() error { return behaviorAdapterIdentityError(request) })
	run("revisions", []string{"request"}, func() error { return behaviorAdapterRevisionError(request) })
	for _, bound := range behaviorAdapterBounds {
		run(bound+"-bound", []string{"request"}, func() error { return behaviorAdapterBoundError(request, bound) })
	}
	run("inputs", []string{"inputs-bound"}, func() error {
		for index, input := range request.Inputs {
			if err := a.addInput(input); err != nil {
				a.keepItem("/inputs/"+strconv.Itoa(index), err)
			}
		}
		return nil
	})
	declared := map[string]bool{}
	declaredInputs := map[string]BehaviorAdapterInput{}
	for _, input := range request.Inputs {
		declared[input.ID] = true
		if _, seen := declaredInputs[input.ID]; !seen {
			declaredInputs[input.ID] = input
		}
	}
	refusedInput := func(id string) bool { return declared[id] && a.inputs[id].ID == "" }
	run("required-inputs", []string{"inputs-bound"}, func() error {
		if !declared[request.MigrationInput] || !declared[request.DiscoveryInput] {
			return a.requiredInputError()
		}
		for _, id := range []string{request.MigrationInput, request.DiscoveryInput} {
			if refusedInput(id) {
				c.notEvaluated("required-inputs", "required input "+id+" not evaluated because the input was refused", "inputs")
			}
		}
		return nil
	})
	run("mappings", []string{"inputs-bound"}, func() error {
		for index, mapping := range request.Mappings {
			item := "/mappings/" + strconv.Itoa(index)
			var refused *BehaviorAdapterInput
			if refusedInput(mapping.Input) {
				input := declaredInputs[mapping.Input]
				refused = &input
			}
			if err := a.addMapping(mapping, refused); err != nil {
				a.keepItem(item, err)
				continue
			}
			if refused != nil {
				c.notEvaluated("mappings", "record checks for "+item+" not evaluated because input "+mapping.Input+" was refused", "inputs")
			}
		}
		return nil
	})
	run("observations", []string{"identity", "revisions", "observations-bound"}, func() error {
		if err := a.validateObservations(); err != nil {
			a.keep(err)
		}
		return nil
	})
	if len(bytes.TrimSpace(previousRaw)) == 0 {
		c.skip("previous-lineage")
	} else {
		run("previous-lineage", []string{"previous", "identity", "revisions"}, func() error {
			return validatePreviousBehaviorAdapterResult(request, *previous)
		})
	}
	identityInput := func(name, id string, needs []string, check func() error) {
		switch {
		case c.states["required-inputs"] == behaviorCheckRefused && !declared[id]:
			c.block(name, "required-inputs")
		case refusedInput(id):
			c.block(name, "inputs")
		default:
			run(name, append(needs, "inputs-bound"), check)
		}
	}
	identityInput("migration", request.MigrationInput, []string{"identity", "revisions"}, a.migrationIdentity)
	var discovery BehaviorDiscovery
	identityInput("discovery", request.DiscoveryInput, []string{"revisions"}, func() (err error) {
		discovery, err = a.discoveryIdentity()
		return err
	})
	var flows []BehaviorFlow
	var variations []BehaviorAdapterVariation
	var behaviors []BehaviorSource
	var tests []BehaviorTest
	records := func(kind string, check func() error) {
		if _, ok := a.mappings[kind]; !ok {
			c.block(kind, "mappings")
			return
		}
		run(kind, nil, check)
	}
	records("flows", func() (err error) { flows, err = a.flows(); return err })
	records("variations", func() (err error) { variations, err = a.variations(); return err })
	records("candidates", func() (err error) { behaviors, err = a.candidates(); return err })
	records("tests", func() (err error) { tests, err = a.tests(); return err })
	var registry BehaviorRegistry
	run("declarations", []string{"flows", "variations", "candidates", "tests"}, func() (err error) {
		flows = a.attachBehaviorVariations(flows, variations)
		if err := a.validateMappedBehaviorAdapterDeclarations(flows, behaviors, tests); err != nil {
			return err
		}
		if c.stageRefused(a) {
			return nil
		}
		registry, err = a.registry(flows, behaviors, tests)
		return err
	})
	run("observation-subjects", []string{"identity", "observations", "tests"}, func() error {
		return a.validateObservationSubjects(tests)
	})
	var result BehaviorAdapterResult
	run("artifacts", []string{"identity", "revisions", "migration", "discovery", "declarations", "observation-subjects"}, func() (err error) {
		result, err = a.result(registry, discovery, variations)
		return err
	})
	run("delta", []string{"artifacts", "previous-lineage"}, func() error {
		_, err := behaviorAdapterDelta(previous, result)
		return err
	})
	c.report.Accepted = len(c.report.Refusals) == 0
	return c.report
}

// stageRefused reports whether the stage currently running has kept a
// refusal; the checker records the refusal count when the stage starts.
func (c *behaviorAdapterChecker) stageRefused(a *behaviorAdapter) bool {
	return len(a.refused) > c.start
}

func (c *behaviorAdapterChecker) stage(name string, needs []string, a *behaviorAdapter, check func() error) {
	blocked := []string{}
	for _, need := range needs {
		if state := c.states[need]; state != behaviorCheckPassed && state != behaviorCheckNotApplicable {
			blocked = append(blocked, need)
		}
	}
	if len(blocked) > 0 {
		c.block(name, blocked...)
		return
	}
	c.items = nil
	refusals := []error{}
	if a != nil {
		c.start = len(a.refused)
	}
	err := check()
	if a != nil {
		refusals = append(refusals, a.refused[c.start:]...)
		for _, detail := range a.unevaluated {
			c.notEvaluated(name, detail, name)
		}
		a.unevaluated = nil
	}
	if err != nil {
		refusals = append(refusals, err)
	}
	state := behaviorCheckPassed
	switch {
	case len(refusals) > 0:
		state = behaviorCheckRefused
	case len(c.items) > 0:
		state = behaviorCheckNotEvaluated
	}
	c.states[name] = state
	c.report.Stages = append(c.report.Stages, BehaviorAdapterCheckStage{Stage: name, State: state, BlockedBy: []string{}})
	for _, refusal := range refusals {
		code, message := "corpus-refused", refusal.Error()
		var typed *Error
		if errors.As(refusal, &typed) {
			code, message = typed.Code, typed.Message
		}
		c.report.Refusals = append(c.report.Refusals, BehaviorAdapterRefusal{Stage: name, State: behaviorCheckRefused, Code: code, Message: message, BlockedBy: []string{}})
	}
	c.report.Refusals = append(c.report.Refusals, c.items...)
	c.items = nil
}

// block records a stage that cannot be evaluated because the named earlier
// stages refused or were themselves not evaluated.
func (c *behaviorAdapterChecker) block(name string, blockedBy ...string) {
	c.states[name] = behaviorCheckNotEvaluated
	c.report.Stages = append(c.report.Stages, BehaviorAdapterCheckStage{Stage: name, State: behaviorCheckNotEvaluated, BlockedBy: blockedBy})
	c.report.Refusals = append(c.report.Refusals, BehaviorAdapterRefusal{Stage: name, State: behaviorCheckNotEvaluated, Code: "", Message: "stage was not evaluated because an earlier stage refused", BlockedBy: blockedBy})
}

func (c *behaviorAdapterChecker) skip(name string) {
	c.states[name] = behaviorCheckNotApplicable
	c.report.Stages = append(c.report.Stages, BehaviorAdapterCheckStage{Stage: name, State: behaviorCheckNotApplicable, BlockedBy: []string{}})
}

// notEvaluated records one item inside an evaluated stage that depends on an
// item an earlier stage refused.
func (c *behaviorAdapterChecker) notEvaluated(stage, detail string, blockedBy ...string) {
	c.items = append(c.items, BehaviorAdapterRefusal{Stage: stage, State: behaviorCheckNotEvaluated, Code: "", Message: detail, BlockedBy: blockedBy})
}

func behaviorAdapterCheckLimitations() []string {
	return []string{
		"check mode neither emits nor writes an adapter result or artifact; it reconciles internally only to determine final-stage refusals and computes no coverage",
		"a stage blocked by an earlier refusal is reported as not-evaluated; repairing that refusal can reveal further refusals",
		"items (inputs, mappings, observations, mapped records and discovery executions) are evaluated independently; within one item, evaluation stops at its first refusal and one not-evaluated entry names the item's remaining checks",
	}
}
