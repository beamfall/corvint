package doccorpus

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
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
	report    BehaviorAdapterCheck
	bytes     int
	states    map[string]string
	items     []BehaviorAdapterRefusal
	room      behaviorCheckRoom
	start     int
	keptStart int
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
		run(bound+"-bound", []string{"request"}, func() error {
			err := behaviorAdapterBoundError(request, bound)
			if err != nil {
				c.notEvaluated(bound+"-bound", []string{bound + "-bound"}, "individual "+bound+" not evaluated because the "+bound+" count bound refused")
			}
			return err
		})
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
				c.notEvaluated("required-inputs", []string{"inputs"}, "required input ", id, " not evaluated because the input was refused")
			}
		}
		return nil
	})
	run("mappings", []string{"inputs-bound", "mappings-bound"}, func() error {
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
				c.notEvaluated("mappings", []string{"inputs"}, "record checks for ", item, " not evaluated because input ", mapping.Input, " was refused")
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
		return behaviorAdapterDeltaError(previous, result)
	})
	c.report.Accepted = len(c.report.Refusals) == 0
	c.report.Refusals = boundBehaviorAdapterRefusals(c.report.Refusals)
	return c.report
}

// The check report lists at most behaviorCheckMaxEntries refusal entries and
// about behaviorCheckMaxEntryBytes of encoded entries, so a readable request
// always yields an encodable report (DCP-V1-044).
const (
	behaviorCheckMaxEntries      = 1024
	behaviorCheckMaxEntryBytes   = 1 << 20
	behaviorCheckMaxMessageBytes = 4 << 10
)

// behaviorCheckOmittedState marks, inside the checker only, the position of
// the first entry a stage dropped because the report room was exhausted.
// boundBehaviorAdapterRefusals cuts the report there, so the report stays a
// prefix of every determinable entry; the state is never emitted.
const behaviorCheckOmittedState = "omitted"

var errBehaviorCheckOmitted = &Error{Code: "", Message: behaviorCheckOmittedState}

// behaviorCheckAfterStage, when set by a test, runs after each evaluated
// stage while its buffers are still held, so retention can be measured.
var behaviorCheckAfterStage func()

type behaviorCheckAdmission int

const (
	behaviorCheckAdmitted behaviorCheckAdmission = iota
	behaviorCheckFirstOmission
	behaviorCheckDropped
)

// behaviorCheckRoom is the report room left when a stage starts. Each stage
// buffer admits entries against its own copy, so it retains no more than the
// report can still list and memory stays bounded however many items refuse.
// Message bytes never exceed encoded bytes, so every entry the final bound
// would keep is admitted.
type behaviorCheckRoom struct {
	entries, bytes int
	full           bool
}

func (r *behaviorCheckRoom) admit(message string) behaviorCheckAdmission {
	switch {
	case r.full:
		return behaviorCheckDropped
	case r.entries <= 0 || len(message) > r.bytes:
		r.full = true
		return behaviorCheckFirstOmission
	}
	r.entries--
	r.bytes -= len(message)
	return behaviorCheckAdmitted
}

func (r *behaviorCheckRoom) append(entries []BehaviorAdapterRefusal, entry BehaviorAdapterRefusal) []BehaviorAdapterRefusal {
	switch r.admit(entry.Message) {
	case behaviorCheckAdmitted:
		return append(entries, entry)
	case behaviorCheckFirstOmission:
		return append(entries, BehaviorAdapterRefusal{State: behaviorCheckOmittedState})
	}
	return entries
}

// behaviorCheckError returns the code and message the report lists for err.
func behaviorCheckError(err error) (string, string) {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code, typed.Message
	}
	return "corpus-refused", err.Error()
}

// behaviorCheckMessage caps one report message at behaviorCheckMaxMessageBytes,
// cut on a UTF-8 boundary and marked with the number of bytes removed. Entry 0
// equals Build's refusal message under this rule (DCP-V1-044).
func behaviorCheckMessage(message string) string {
	return behaviorCheckJoin(message)
}

// behaviorCheckJoin returns behaviorCheckMessage of the joined parts without
// building more than the capped prefix, so a request string echoed into many
// entries is never copied in full.
func behaviorCheckJoin(parts ...string) string {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	if total <= behaviorCheckMaxMessageBytes {
		return strings.Join(parts, "")
	}
	prefix := make([]byte, 0, behaviorCheckMaxMessageBytes+1)
	for _, part := range parts {
		prefix = append(prefix, part[:min(len(part), cap(prefix)-len(prefix))]...)
	}
	cut := behaviorCheckMaxMessageBytes
	for cut > 0 && !utf8.RuneStart(prefix[cut]) {
		cut--
	}
	return string(prefix[:cut]) + " … [truncated " + strconv.Itoa(total-cut) + " bytes]"
}

// boundBehaviorAdapterRefusals keeps the first entry, which is Build's
// refusal, and then entries in order while both bounds hold and no stage
// dropped an entry. A terminal not-evaluated entry says how many entries were
// kept when any were omitted. Messages were capped when appended.
func boundBehaviorAdapterRefusals(refusals []BehaviorAdapterRefusal) []BehaviorAdapterRefusal {
	size := 0
	for index, refusal := range refusals {
		encoded, err := Encode(refusal)
		if err == nil {
			size += len(encoded)
		}
		if refusal.State == behaviorCheckOmittedState || index > 0 && (err != nil || index >= behaviorCheckMaxEntries || size > behaviorCheckMaxEntryBytes) {
			kept := append([]BehaviorAdapterRefusal(nil), refusals[:index]...)
			return append(kept, BehaviorAdapterRefusal{Stage: "report", State: behaviorCheckNotEvaluated, Code: "", Message: "further entries omitted after " + strconv.Itoa(index), BlockedBy: []string{}})
		}
	}
	return refusals
}

// stageRefused reports whether the stage currently running has kept a
// refusal; the checker records the refusal count when the stage starts.
func (c *behaviorAdapterChecker) stageRefused(a *behaviorAdapter) bool {
	return a.kept > c.keptStart
}

// add appends report entries directly; stage buffers were already bounded.
func (c *behaviorAdapterChecker) add(entries ...BehaviorAdapterRefusal) {
	for _, entry := range entries {
		c.bytes += len(entry.Message)
	}
	c.report.Refusals = append(c.report.Refusals, entries...)
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
	c.room = behaviorCheckRoom{entries: behaviorCheckMaxEntries - len(c.report.Refusals), bytes: behaviorCheckMaxEntryBytes - c.bytes}
	refusedRoom := c.room
	kept := []error{}
	if a != nil {
		c.start, c.keptStart = len(a.refused), a.kept
		a.refusedRoom, a.unevaluatedRoom = c.room, c.room
	}
	err := check()
	if behaviorCheckAfterStage != nil {
		behaviorCheckAfterStage()
	}
	refused := err != nil
	if a != nil {
		kept = a.refused[c.start:]
		refused = refused || a.kept > c.keptStart
		refusedRoom = a.refusedRoom
		for _, item := range a.unevaluated {
			item.Stage, item.BlockedBy = name, []string{name}
			c.items = append(c.items, item)
		}
		a.unevaluated = nil
	}
	state := behaviorCheckPassed
	switch {
	case refused:
		state = behaviorCheckRefused
	case len(c.items) > 0:
		state = behaviorCheckNotEvaluated
	}
	c.states[name] = state
	c.report.Stages = append(c.report.Stages, BehaviorAdapterCheckStage{Stage: name, State: state, BlockedBy: []string{}})
	entries := make([]BehaviorAdapterRefusal, 0, len(kept)+1)
	for _, refusal := range kept {
		code, message := behaviorCheckError(refusal)
		if refusal == errBehaviorCheckOmitted {
			entries = append(entries, BehaviorAdapterRefusal{Stage: name, State: behaviorCheckOmittedState})
			continue
		}
		entries = append(entries, BehaviorAdapterRefusal{Stage: name, State: behaviorCheckRefused, Code: code, Message: message, BlockedBy: []string{}})
	}
	if err != nil {
		code, message := behaviorCheckError(err)
		entries = refusedRoom.append(entries, BehaviorAdapterRefusal{Stage: name, State: behaviorCheckRefused, Code: code, Message: behaviorCheckMessage(message), BlockedBy: []string{}})
	}
	c.add(entries...)
	c.add(c.items...)
	c.items = nil
}

// block records a stage that cannot be evaluated because the named earlier
// stages refused or were themselves not evaluated.
func (c *behaviorAdapterChecker) block(name string, blockedBy ...string) {
	c.states[name] = behaviorCheckNotEvaluated
	c.report.Stages = append(c.report.Stages, BehaviorAdapterCheckStage{Stage: name, State: behaviorCheckNotEvaluated, BlockedBy: blockedBy})
	c.add(BehaviorAdapterRefusal{Stage: name, State: behaviorCheckNotEvaluated, Code: "", Message: "stage was not evaluated because an earlier stage refused", BlockedBy: blockedBy})
}

func (c *behaviorAdapterChecker) skip(name string) {
	c.states[name] = behaviorCheckNotApplicable
	c.report.Stages = append(c.report.Stages, BehaviorAdapterCheckStage{Stage: name, State: behaviorCheckNotApplicable, BlockedBy: []string{}})
}

// notEvaluated records one item inside an evaluated stage that depends on an
// item an earlier stage refused. The parts are joined under the message cap.
func (c *behaviorAdapterChecker) notEvaluated(stage string, blockedBy []string, parts ...string) {
	c.items = c.room.append(c.items, BehaviorAdapterRefusal{Stage: stage, State: behaviorCheckNotEvaluated, Code: "", Message: behaviorCheckJoin(parts...), BlockedBy: blockedBy})
}

func behaviorAdapterCheckLimitations() []string {
	return []string{
		"check mode neither emits nor writes an adapter result or artifact; it computes no reconciliation frontier, coverage, provider subjects or delta",
		"a stage blocked by an earlier refusal is reported as not-evaluated; repairing that refusal can reveal further refusals",
		"items (inputs, mappings, observations, mapped records and discovery executions) are evaluated independently; within one item, evaluation stops at its first refusal and one not-evaluated entry names the item's remaining checks",
		"when a count bound refuses, the individual items it bounds are not evaluated",
		"the report lists at most 1024 refusal entries and about 1 MiB of them; a terminal entry says further entries were omitted",
		"each report message is capped at 4 KiB on a UTF-8 boundary with a truncation marker; the first entry equals the build refusal under the same cap",
	}
}
