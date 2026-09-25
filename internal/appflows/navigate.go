package appflows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// Navigation wire identities, the effect ladder and the packet bound (AFU-V1-025..028).
const (
	NavigationMapSchema     = "application-navigation-map/0"
	NavigationPacketSchema  = "application-navigation-packet/0"
	TrafficSchema           = "application-flow-traffic/0"
	EffectRead              = "read"
	EffectWriteReversible   = "write-reversible"
	EffectWriteIrreversible = "write-irreversible"
	EffectExternal          = "external-side-effect"
	GrantGranted            = "granted"
	GrantRequired           = "requires-grant"
	maxPacketSteps          = 256
)

// effectRank orders the effect classes; an unknown class ranks 0 and is never granted.
var effectRank = map[string]int{EffectRead: 1, EffectWriteReversible: 2, EffectWriteIrreversible: 3, EffectExternal: 4}

var (
	apiMethods      = []string{"GET", "HEAD", "OPTIONS", "POST", "PUT", "PATCH", "DELETE"}
	safeMethods     = []string{"GET", "HEAD", "OPTIONS"}
	methodPattern   = regexp.MustCompile(`^[A-Z]{1,16}$`)
	rolePattern     = regexp.MustCompile(`^[a-z]{1,32}$`)
	templatePattern = regexp.MustCompile(`^/[A-Za-z0-9._~{}/:@!$&'()*+,;=%-]{0,511}$`)
)

// FlowNavigation is an intent's optional navigation block: the flows that must run first, such as
// sign-in, and per-step locators taken from the intent or test source (AFU-V1-025, AFU-V1-026).
type FlowNavigation struct {
	PreconditionFlows []string  `json:"precondition_flows"`
	Steps             []NavStep `json:"steps"`
}

// NavStep places one intent step: its route template (ui), locator, readiness condition, input
// fixture ID, expected outcomes, declared effect class and recovery step.
type NavStep struct {
	StepID       string      `json:"step_id"`
	State        string      `json:"state,omitempty"`
	Locator      NavLocator  `json:"locator"`
	Ready        *NavLocator `json:"ready,omitempty"`
	InputFixture string      `json:"input_fixture,omitempty"`
	Expect       []string    `json:"expect"`
	Effect       string      `json:"effect,omitempty"`
	Recovery     string      `json:"recovery,omitempty"`
}

// NavLocator is an accessible role and name, a test ID, or an API method and path template.
type NavLocator struct {
	Role   string `json:"role,omitempty"`
	Name   string `json:"name,omitempty"`
	TestID string `json:"test_id,omitempty"`
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
}

// TrafficRecord is one observed request of a step, one application-flow-traffic/0 JSONL line.
type TrafficRecord struct {
	Schema     string `json:"schema"`
	FlowID     string `json:"flow_id"`
	StepID     string `json:"step_id"`
	Method     string `json:"method"`
	FormSubmit bool   `json:"form_submit"`
}

// NavigationMap is `flows navigate` without --goal.
type NavigationMap struct {
	Schema   string     `json:"schema"`
	Revision string     `json:"revision"`
	States   []NavState `json:"states"`
	Flows    []NavFlow  `json:"flows"`
}

// NavState is a route template or an API operation with its stable ID.
type NavState struct {
	StateID  string `json:"state_id"`
	Kind     string `json:"kind"`
	Template string `json:"template"`
}

// NavFlow is one flow's transitions, one per intent step in intent order.
type NavFlow struct {
	FlowID            string       `json:"flow_id"`
	Kind              string       `json:"kind"`
	Status            string       `json:"status"`
	PreconditionFlows []string     `json:"precondition_flows"`
	Transitions       []Transition `json:"transitions"`
}

// Transition is one step. A step without a navigation entry has no state or locator and is
// write-irreversible; grant fields appear only in a packet.
type Transition struct {
	FlowID        string        `json:"flow_id"`
	StepID        string        `json:"step_id"`
	Action        string        `json:"action"`
	State         string        `json:"state,omitempty"`
	Locator       *NavLocator   `json:"locator,omitempty"`
	Preconditions []string      `json:"preconditions"`
	Ready         *NavLocator   `json:"ready,omitempty"`
	InputFixture  string        `json:"input_fixture,omitempty"`
	Expect        []FlowOutcome `json:"expect"`
	EffectClass   string        `json:"effect_class"`
	EffectBasis   string        `json:"effect_basis"`
	Recovery      string        `json:"recovery,omitempty"`
	Verification  string        `json:"verification"`
	Grant         string        `json:"grant,omitempty"`
	GrantNeeded   string        `json:"grant_needed,omitempty"`
}

// NavigationPacket is `flows navigate --goal`: the precondition flows then the goal, in run order.
type NavigationPacket struct {
	Schema    string       `json:"schema"`
	Revision  string       `json:"revision"`
	Goal      string       `json:"goal"`
	MaxEffect string       `json:"max_effect"`
	States    []NavState   `json:"states"`
	Flows     []PacketFlow `json:"flows"`
	Steps     []Transition `json:"steps"`
	Recovery  []Transition `json:"recovery"`
}

// PacketFlow is one flow of the packet with its completeness status.
type PacketFlow struct {
	FlowID string `json:"flow_id"`
	Status string `json:"status"`
}

// ReadTraffic reads --traffic files of application-flow-traffic/0 records with the flow-input
// discipline; all files together share the run-evidence record bound.
func ReadTraffic(filenames []string) ([]TrafficRecord, error) {
	records := []TrafficRecord{}
	for _, name := range filenames {
		raw, err := ReadFile(name)
		if err != nil {
			return nil, err
		}
		for line := range bytes.Lines(raw) {
			if len(records) >= maxRunRecords {
				return nil, navigationBound("traffic records exceed the record bound")
			}
			r, err := decodeTraffic(line)
			if err != nil {
				return nil, err
			}
			records = append(records, r)
		}
	}
	return records, nil
}

func decodeTraffic(line []byte) (TrafficRecord, error) {
	var r TrafficRecord
	if err := Decode(line, &r); err != nil {
		return r, err
	}
	if r.Schema != TrafficSchema || !flowIDPattern.MatchString(r.FlowID) || !memberPattern.MatchString(r.StepID) || !methodPattern.MatchString(r.Method) {
		return r, errors.New("invalid " + TrafficSchema + " record")
	}
	return r, nil
}

// FlowNavigationMap derives the navigation map at the set's revision (AFU-V1-025..027).
func FlowNavigationMap(ctx context.Context, root string, set IntentSet, evidence []TestRunEvidence, traffic []TrafficRecord) ([]byte, error) {
	m, err := navigationMap(ctx, root, set, evidence, traffic)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(m)
}

// FlowNavigationPacket returns the bounded packet for one goal flow, each step above maxEffect
// marked requires-grant with the class it needs (AFU-V1-028).
func FlowNavigationPacket(ctx context.Context, root string, set IntentSet, evidence []TestRunEvidence, traffic []TrafficRecord, goal, maxEffect string) ([]byte, error) {
	if effectRank[maxEffect] == 0 {
		return nil, errors.New("--max-effect must be read, write-reversible, write-irreversible or external-side-effect")
	}
	m, err := navigationMap(ctx, root, set, evidence, traffic)
	if err != nil {
		return nil, err
	}
	flows := map[string]NavFlow{}
	for _, f := range m.Flows {
		flows[f.FlowID] = f
	}
	if _, ok := flows[goal]; !ok {
		return nil, fmt.Errorf("--goal %q names no flow", goal)
	}
	order, err := preconditionOrder(flows, goal)
	if err != nil {
		return nil, err
	}
	packet := NavigationPacket{Schema: NavigationPacketSchema, Revision: m.Revision, Goal: goal, MaxEffect: maxEffect, Flows: []PacketFlow{}, Steps: []Transition{}, Recovery: []Transition{}}
	for _, id := range order {
		packet.Flows = append(packet.Flows, PacketFlow{FlowID: id, Status: flows[id].Status})
		steps, recovery := splitRecovery(flows[id].Transitions)
		packet.Steps = append(packet.Steps, grantAll(steps, maxEffect)...)
		packet.Recovery = append(packet.Recovery, grantAll(recovery, maxEffect)...)
	}
	if len(packet.Steps)+len(packet.Recovery) > maxPacketSteps {
		return nil, navigationBound(fmt.Sprintf("the packet exceeds %d steps", maxPacketSteps))
	}
	packet.States = referencedStates(m.States, append(slices.Clone(packet.Steps), packet.Recovery...))
	return doccorpus.Encode(packet)
}

func navigationBound(message string) error {
	return &gokernel.Error{Code: "navigation-bound-exceeded", Message: message}
}

func navigationMap(ctx context.Context, root string, set IntentSet, evidence []TestRunEvidence, traffic []TrafficRecord) (NavigationMap, error) {
	links, at, err := evaluateSet(ctx, root, set)
	if err != nil {
		return NavigationMap{}, err
	}
	m := NavigationMap{Schema: NavigationMapSchema, Revision: at.commit, States: navStates(set.Flows), Flows: []NavFlow{}}
	for _, intent := range set.Flows {
		own := flowLinks(links, intent.FlowID)
		m.Flows = append(m.Flows, NavFlow{FlowID: intent.FlowID, Kind: intent.Kind, Status: status(flowGaps(intent, own, evidence, at)),
			PreconditionFlows: nonNil(navigationOf(intent).PreconditionFlows), Transitions: flowTransitions(intent, own, evidence, traffic, at)})
	}
	return m, checkPreconditions(m.Flows)
}

// checkPreconditions refuses a precondition flow that is not declared or that closes a cycle.
func checkPreconditions(flows []NavFlow) error {
	byID := map[string]NavFlow{}
	for _, f := range flows {
		byID[f.FlowID] = f
	}
	for _, f := range flows {
		if _, err := preconditionOrder(byID, f.FlowID); err != nil {
			return err
		}
	}
	return nil
}

// preconditionOrder lists goal's precondition flows depth first, each once, before goal itself.
func preconditionOrder(flows map[string]NavFlow, goal string) ([]string, error) {
	order := []string{}
	visiting := map[string]bool{}
	var visit func(id string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("flow %s: precondition flows form a cycle", goal)
		}
		f, ok := flows[id]
		if !ok {
			return fmt.Errorf("flow %s: precondition flow %q is not declared", goal, id)
		}
		if slices.Contains(order, id) {
			return nil
		}
		visiting[id] = true
		for _, p := range f.PreconditionFlows {
			if err := visit(p); err != nil {
				return err
			}
		}
		visiting[id] = false
		order = append(order, id)
		return nil
	}
	return order, visit(goal)
}

func navigationOf(f FlowIntent) FlowNavigation {
	if f.Navigation == nil {
		return FlowNavigation{}
	}
	return *f.Navigation
}

func flowTransitions(intent FlowIntent, links []EvaluatedLink, evidence []TestRunEvidence, traffic []TrafficRecord, at head) []Transition {
	entries := map[string]NavStep{}
	for _, e := range navigationOf(intent).Steps {
		entries[e.StepID] = e
	}
	verified := verifiedSteps(intent, links, evidence, at)
	out := []Transition{}
	for _, s := range intent.Steps {
		observed := slices.DeleteFunc(slices.Clone(traffic), func(r TrafficRecord) bool { return r.FlowID != intent.FlowID || r.StepID != s.StepID })
		out = append(out, transition(intent, s, entries[s.StepID], observed, verified[s.StepID]))
	}
	return out
}

func transition(intent FlowIntent, s FlowStep, e NavStep, observed []TrafficRecord, verified bool) Transition {
	t := Transition{FlowID: intent.FlowID, StepID: s.StepID, Action: s.Action, Preconditions: nonNil(intent.Preconditions), Expect: []FlowOutcome{},
		Ready: e.Ready, InputFixture: e.InputFixture, Recovery: e.Recovery, Verification: "unverified"}
	if e.StepID != "" {
		t.State, t.Locator = stateOf(intent.Kind, e).StateID, &e.Locator
	}
	for _, id := range e.Expect {
		t.Expect = append(t.Expect, intent.Outcomes[slices.IndexFunc(intent.Outcomes, func(o FlowOutcome) bool { return o.OutcomeID == id })])
	}
	if verified {
		t.Verification = "verified"
	}
	t.EffectClass, t.EffectBasis = effectOf(e.Effect, observed)
	return t
}

// effectOf applies AFU-V1-027: an undeclared class is write-irreversible; observed non-GET traffic or
// a form submit contradicts a declared read and raises it to write-irreversible; nothing lowers it.
func effectOf(declared string, observed []TrafficRecord) (string, string) {
	if declared == "" {
		return EffectWriteIrreversible, "undeclared"
	}
	if declared == EffectRead && slices.ContainsFunc(observed, wrote) {
		return EffectWriteIrreversible, "observed-traffic"
	}
	return declared, "declared"
}

func wrote(r TrafficRecord) bool { return r.Method != "GET" || r.FormSubmit }

// verifiedSteps holds the steps of every variation whose required evidence pairs are all verified.
func verifiedSteps(intent FlowIntent, links []EvaluatedLink, evidence []TestRunEvidence, at head) map[string]bool {
	verified := map[string]bool{}
	for _, v := range intent.Variations {
		pairs := variationEvidence(intent, v, links, evidence, at)
		if len(pairs) == 0 || slices.ContainsFunc(pairs, func(e TestEvidence) bool { return e.State != "verified" }) {
			continue
		}
		for _, s := range v.Steps {
			verified[s] = true
		}
	}
	return verified
}

func stateOf(kind string, e NavStep) NavState {
	if kind == "api" {
		op := e.Locator.Method + " " + e.Locator.Path
		return NavState{StateID: "api:" + op, Kind: "api-operation", Template: op}
	}
	return NavState{StateID: "ui:" + e.State, Kind: "route", Template: e.State}
}

func navStates(flows []FlowIntent) []NavState {
	states := []NavState{}
	for _, f := range flows {
		for _, e := range navigationOf(f).Steps {
			states = append(states, stateOf(f.Kind, e))
		}
	}
	slices.SortFunc(states, func(a, b NavState) int { return strings.Compare(a.StateID, b.StateID) })
	return slices.Compact(states)
}

func referencedStates(states []NavState, steps []Transition) []NavState {
	return slices.DeleteFunc(slices.Clone(states), func(s NavState) bool {
		return !slices.ContainsFunc(steps, func(t Transition) bool { return t.State == s.StateID })
	})
}

// splitRecovery separates the steps some other step names as its recovery from the ordered path.
func splitRecovery(transitions []Transition) ([]Transition, []Transition) {
	recovery := func(t Transition) bool {
		return slices.ContainsFunc(transitions, func(o Transition) bool { return o.Recovery == t.StepID })
	}
	path := slices.DeleteFunc(slices.Clone(transitions), recovery)
	targets := slices.DeleteFunc(slices.Clone(transitions), func(t Transition) bool { return !recovery(t) })
	return path, targets
}

func grantAll(steps []Transition, maxEffect string) []Transition {
	for i := range steps {
		steps[i].Grant = GrantGranted
		if effectRank[steps[i].EffectClass] == 0 || effectRank[steps[i].EffectClass] > effectRank[maxEffect] {
			steps[i].Grant, steps[i].GrantNeeded = GrantRequired, steps[i].EffectClass
		}
	}
	return steps
}

// validateNavigation checks an intent's navigation block against its own steps and outcomes.
func validateNavigation(f FlowIntent, members map[string]string) error {
	if f.Navigation == nil {
		return nil
	}
	n := f.Navigation
	if len(n.PreconditionFlows) > maxFlowList || !uniqueMatching(n.PreconditionFlows, flowIDPattern) || slices.Contains(n.PreconditionFlows, f.FlowID) {
		return errors.New("navigation precondition_flows are invalid")
	}
	if len(n.Steps) > maxFlowList {
		return errors.New("navigation list bound exceeded")
	}
	seen := map[string]bool{}
	for _, s := range n.Steps {
		if seen[s.StepID] {
			return fmt.Errorf("navigation step %s is duplicated", s.StepID)
		}
		seen[s.StepID] = true
		if err := validateNavStep(f.Kind, s, members); err != nil {
			return fmt.Errorf("navigation step %s: %v", s.StepID, err)
		}
		if s.Recovery != "" && !recoveryOffPath(f, s) {
			return fmt.Errorf("navigation step %s: recovery must name a later step no variation lists", s.StepID)
		}
	}
	return nil
}

// recoveryOffPath: a recovery target comes after its step in intent order and no variation lists
// it, so moving recovery targets out of the path never drops a path step.
func recoveryOffPath(f FlowIntent, s NavStep) bool {
	order := func(id string) int {
		return slices.IndexFunc(f.Steps, func(fs FlowStep) bool { return fs.StepID == id })
	}
	listed := slices.ContainsFunc(f.Variations, func(v FlowVariation) bool { return slices.Contains(v.Steps, s.Recovery) })
	return order(s.Recovery) > order(s.StepID) && !listed
}

func validateNavStep(kind string, s NavStep, members map[string]string) error {
	if members[s.StepID] != "step" {
		return errors.New("names no step")
	}
	if !navPlacement(kind, s) {
		return errors.New("state and locator do not fit the flow kind")
	}
	if s.Ready != nil && (kind != "ui" || !uiLocator(*s.Ready)) {
		return errors.New("ready must be a ui role and name or test_id locator")
	}
	if s.InputFixture != "" && !memberPattern.MatchString(s.InputFixture) {
		return errors.New("input_fixture must be a fixture ID")
	}
	if len(s.Expect) > maxFlowList || !uniqueMatching(s.Expect, memberPattern) || slices.ContainsFunc(s.Expect, func(o string) bool { return members[o] != "outcome" }) {
		return errors.New("expect must name outcomes of the flow")
	}
	if s.Effect != "" && effectRank[s.Effect] == 0 {
		return errors.New("effect must be read, write-reversible, write-irreversible or external-side-effect")
	}
	if s.Effect == EffectRead && kind == "api" && !slices.Contains(safeMethods, s.Locator.Method) {
		return errors.New("a read effect needs a GET, HEAD or OPTIONS method")
	}
	if s.Recovery != "" && (members[s.Recovery] != "step" || s.Recovery == s.StepID) {
		return errors.New("recovery must name another step")
	}
	return nil
}

// navPlacement: a ui step has a route template and a ui locator; an api step has no state and an
// API method and path template, which are its state.
func navPlacement(kind string, s NavStep) bool {
	if kind == "api" {
		l := s.Locator
		return s.State == "" && slices.Contains(apiMethods, l.Method) && templatePattern.MatchString(l.Path) && l.Role == "" && l.Name == "" && l.TestID == ""
	}
	return templatePattern.MatchString(s.State) && uiLocator(s.Locator)
}

func uiLocator(l NavLocator) bool {
	if l.Method != "" || l.Path != "" {
		return false
	}
	if l.TestID != "" {
		return l.Role == "" && l.Name == "" && memberPattern.MatchString(l.TestID)
	}
	return rolePattern.MatchString(l.Role) && flowText(l.Name)
}
