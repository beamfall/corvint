package appflows

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Import formats (AFU-V1-004).
const (
	FormatAdapterRequest = "behavior-adapter-request"
	FormatOpenAPI        = "openapi"
	FormatPlaywrightList = "playwright-list"
	maxImportedID        = 100
)

var importers = map[string]func(raw []byte, source string) ([]FlowIntent, error){
	FormatAdapterRequest: importAdapterRequest,
	FormatOpenAPI:        importOpenAPI,
	FormatPlaywrightList: importPlaywrightList,
}

// Import compiles raw into new proposed intents with inferred links and writes each as a new file under
// dir. It is all-or-nothing: the combined set is bounded and validated before any write, every file is
// created exclusively, and a failed write removes exactly the files this call created (AFU-V1-004,
// AFU-V1-037). Flow IDs are lowercase and every case variant of a .json name is loaded as an intent,
// so a name that differs only in case from an existing file is refused before writing.
func Import(root, dir string, raw []byte, format, source string) ([]string, error) {
	set, intents, err := planImport(root, dir, raw, format, source)
	if err != nil {
		return nil, fmt.Errorf("%v; %s", err, nothingWritten)
	}
	return writeAll(root, set, intents)
}

const nothingWritten = "no intent from this import remains written"

// writeIntentFile is a test seam for a failed write.
var writeIntentFile = WriteConfined

func planImport(root, dir string, raw []byte, format, source string) (IntentSet, []FlowIntent, error) {
	importer, ok := importers[format]
	if !ok {
		return IntentSet{}, nil, errors.New("--format must be behavior-adapter-request, openapi or playwright-list")
	}
	set, err := LoadIntents(root, dir)
	if err != nil {
		return set, nil, err
	}
	intents, err := importer(raw, sourcePath(root, source))
	if err != nil {
		return set, nil, err
	}
	if len(set.Flows)+len(intents) > MaxFlows {
		return set, nil, fmt.Errorf("import would exceed %d intents", MaxFlows)
	}
	combined := IntentSet{Dir: set.Dir, Flows: append(slices.Clone(set.Flows), intents...), Retired: set.Retired}
	slices.SortFunc(combined.Flows, func(a, b FlowIntent) int { return strings.Compare(a.FlowID, b.FlowID) })
	if err = uniqueFlowIDs(combined.Flows); err != nil {
		return set, nil, err
	}
	return set, intents, combined.validate()
}

func writeAll(root string, set IntentSet, intents []FlowIntent) ([]string, error) {
	written := []string{}
	for _, intent := range intents {
		name := set.IntentPath(intent.FlowID)
		data, err := encodeIntent(intent)
		if err == nil {
			err = writeIntentFile(root, filepath.Join(root, filepath.FromSlash(name)), data)
		}
		if err != nil {
			return nil, rollback(root, written, fmt.Errorf("%s: %v", name, err))
		}
		written = append(written, name)
	}
	return written, nil
}

// rollback removes exactly the files this import created and names them in the returned error.
func rollback(root string, written []string, cause error) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("%v; rollback failed, these files remain: %v", cause, written)
	}
	defer r.Close()
	remaining := []string{}
	for _, name := range written {
		if r.Remove(filepath.FromSlash(name)) != nil {
			remaining = append(remaining, name)
		}
	}
	if len(remaining) > 0 {
		return fmt.Errorf("%v; rollback failed, these files remain: %v", cause, remaining)
	}
	return fmt.Errorf("%v; rolled back %v; %s", cause, written, nothingWritten)
}

func uniqueFlowIDs(flows []FlowIntent) error {
	for i := 1; i < len(flows); i++ {
		if flows[i].FlowID == flows[i-1].FlowID {
			return fmt.Errorf("flow %s already exists; import never overwrites an intent", flows[i].FlowID)
		}
	}
	return nil
}

func encodeIntent(intent FlowIntent) ([]byte, error) {
	data, err := json.MarshalIndent(intent, "", "  ")
	return append(data, '\n'), err
}

// sourcePath is the repository-relative path of the import source, or "" when it lies outside root.
func sourcePath(root, source string) string {
	abs, err := filepath.Abs(source)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || !safePath(filepath.ToSlash(rel)) {
		return ""
	}
	return filepath.ToSlash(rel)
}

func proposedIntent(id, kind string) FlowIntent {
	return FlowIntent{
		Schema: FlowIntentSchema, FlowID: id, Revision: 1, Proposed: true, Kind: kind, Actor: "unknown",
		Preconditions: []string{}, Steps: []FlowStep{}, Outcomes: []FlowOutcome{}, Variations: []FlowVariation{}, Links: []FlowLink{},
	}
}

// importAdapterRequest runs the request through the existing DCP-V1 adapter and keeps its
// normalized flows and variations, so export reproduces the request (AFU-V1-005).
func importAdapterRequest(raw []byte, _ string) ([]FlowIntent, error) {
	result, err := doccorpus.BuildBehaviorAdapter(raw, nil)
	if err != nil {
		return nil, fmt.Errorf("behavior adapter refused the request: %v", err)
	}
	registry := result.Provider.BehaviorContracts
	testPaths := map[string]string{}
	for _, t := range registry.Tests {
		testPaths[t.ID] = t.Evidence.Path
	}
	byFlow := map[string][]doccorpus.BehaviorAdapterVariation{}
	for _, v := range result.Variations {
		byFlow[v.Flow] = append(byFlow[v.Flow], v)
	}
	intents := []FlowIntent{}
	for _, flow := range registry.Flows {
		intent, err := adapterIntent(flow, byFlow[flow.ID], testPaths)
		if err != nil {
			return nil, fmt.Errorf("flow %s: %v", flow.ID, err)
		}
		intents = append(intents, intent)
		delete(byFlow, flow.ID)
	}
	for flow := range byFlow {
		return nil, fmt.Errorf("variation names unmapped flow %s", flow)
	}
	return intents, nil
}

func adapterIntent(flow doccorpus.BehaviorFlow, variations []doccorpus.BehaviorAdapterVariation, testPaths map[string]string) (FlowIntent, error) {
	intent := proposedIntent(flow.ID, "ui")
	intent.Adapter = &FlowAdapter{
		Derivation: flow.Derivation, Evidence: flow.Evidence, RequiredPages: nonNil(flow.RequiredPages),
		NegativeControls: nonNil(flow.NegativeControls), OrderedEvents: nonNil(flow.OrderedEvents), MissingReview: flow.MissingReview,
	}
	steps := map[string]string{}
	outcomes := map[string]doccorpus.BehaviorAdapterOutcome{}
	for _, v := range variations {
		iv := FlowVariation{VariationID: v.ID, Preconditions: nonNil(v.Preconditions), Steps: []string{}, ObservableFacts: nonNil(v.ObservableFacts), Outcomes: []string{}, Projects: nonNil(v.Projects)}
		for _, action := range v.Actions {
			if steps[action] == "" {
				steps[action] = fmt.Sprintf("step-%d", len(steps)+1)
				intent.Steps = append(intent.Steps, FlowStep{StepID: steps[action], Action: action})
			}
			iv.Steps = append(iv.Steps, steps[action])
		}
		for _, o := range v.ExpectedOutcomes {
			if prior, seen := outcomes[o.ID]; seen && prior != o {
				return intent, fmt.Errorf("outcome %s differs between variations", o.ID)
			}
			outcomes[o.ID] = o
			iv.Outcomes = append(iv.Outcomes, o.ID)
		}
		for _, test := range v.Tests {
			intent.Links = append(intent.Links, inferredTestLink(v.ID, test, testPaths[test]))
		}
		intent.Variations = append(intent.Variations, iv)
	}
	for _, id := range slices.Sorted(maps.Keys(outcomes)) {
		o := outcomes[id]
		intent.Outcomes = append(intent.Outcomes, FlowOutcome{OutcomeID: o.ID, Behavior: o.Behavior, Matcher: o.Matcher, Locator: o.Locator, Value: o.Value})
	}
	return intent, nil
}

func inferredTestLink(from, key, file string) FlowLink {
	target := LinkTarget{Type: "test", TestKey: key}
	if safePath(file) {
		target.Path = file
	}
	return FlowLink{From: from, Basis: "inferred", Target: target}
}

// lenientDecode bounds and secret-screens a third-party document, then ignores fields Corvint does not read.
func lenientDecode(raw []byte, out any) error {
	if len(raw) > MaxBytes {
		return errors.New("import source exceeds byte limit")
	}
	if secretscreen.MatchString(string(raw)) {
		return errors.New("import source contains secret-shaped data")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.New("import source is not valid JSON")
	}
	return nil
}

type openAPIOperation struct {
	OperationID string                     `json:"operationId"`
	Responses   map[string]json.RawMessage `json:"responses"`
}

var openAPIMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// importOpenAPI makes one proposed api flow per OpenAPI 3.0/3.1 JSON operation.
func importOpenAPI(raw []byte, source string) ([]FlowIntent, error) {
	var doc struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := lenientDecode(raw, &doc); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.0.") && !strings.HasPrefix(doc.OpenAPI, "3.1.") {
		return nil, errors.New("openapi import accepts OpenAPI 3.0 or 3.1 JSON")
	}
	intents := []FlowIntent{}
	for _, route := range slices.Sorted(maps.Keys(doc.Paths)) {
		for _, method := range openAPIMethods {
			op, ok := doc.Paths[route][method]
			if !ok {
				continue
			}
			intent, err := openAPIIntent(route, method, op, source)
			if err != nil {
				return nil, err
			}
			intents = append(intents, intent)
		}
	}
	return intents, nil
}

func openAPIIntent(route, method string, raw json.RawMessage, source string) (FlowIntent, error) {
	var op openAPIOperation
	if err := json.Unmarshal(raw, &op); err != nil {
		return FlowIntent{}, fmt.Errorf("operation %s %s is invalid", method, route)
	}
	id := importedID(op.OperationID)
	if id == "" {
		id = importedID(method + "-" + route)
	}
	call := strings.ToUpper(method) + " " + route
	intent := proposedIntent(id, "api")
	intent.Steps = []FlowStep{{StepID: "request", Action: call}}
	variation := FlowVariation{VariationID: id + ".call", Preconditions: []string{}, Steps: []string{"request"}, ObservableFacts: []string{}, Outcomes: []string{}, Projects: []string{}}
	codes := slices.Sorted(maps.Keys(op.Responses))
	for _, code := range codes {
		outcome := FlowOutcome{OutcomeID: "status-" + importedID(code), Behavior: "responds " + code, Matcher: "status", Locator: call, Value: code}
		intent.Outcomes = append(intent.Outcomes, outcome)
		variation.Outcomes = append(variation.Outcomes, outcome.OutcomeID)
	}
	intent.Variations = []FlowVariation{variation}
	if source != "" {
		intent.Links = []FlowLink{{From: variation.VariationID, Basis: "inferred", Target: LinkTarget{Type: "source", Path: source}}}
	}
	return intent, nil
}

type playwrightSuite struct {
	File   string            `json:"file"`
	Specs  []playwrightSpec  `json:"specs"`
	Suites []playwrightSuite `json:"suites"`
}

type playwrightSpec struct {
	Title string `json:"title"`
	File  string `json:"file"`
	Tests []struct {
		ProjectName string `json:"projectName"`
		Annotations []struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"annotations"`
	} `json:"tests"`
}

// importPlaywrightList makes one proposed ui flow per spec of a Playwright JSON test list.
func importPlaywrightList(raw []byte, _ string) ([]FlowIntent, error) {
	var doc struct {
		Suites []playwrightSuite `json:"suites"`
	}
	if err := lenientDecode(raw, &doc); err != nil {
		return nil, err
	}
	specs := collectSpecs(doc.Suites, nil)
	if len(specs) > MaxFlows {
		return nil, fmt.Errorf("playwright list exceeds %d specs", MaxFlows)
	}
	intents := []FlowIntent{}
	for _, spec := range specs {
		intents = append(intents, playwrightIntent(spec))
	}
	return intents, nil
}

func collectSpecs(suites []playwrightSuite, into []playwrightSpec) []playwrightSpec {
	for _, s := range suites {
		for _, spec := range s.Specs {
			if spec.File == "" {
				spec.File = s.File
			}
			into = append(into, spec)
		}
		into = collectSpecs(s.Suites, into)
	}
	return into
}

func playwrightIntent(spec playwrightSpec) FlowIntent {
	id := importedID(strings.TrimSuffix(path.Base(spec.File), path.Ext(spec.File)) + "-" + spec.Title)
	key := spec.File + " > " + spec.Title
	projects := []string{}
	for _, t := range spec.Tests {
		projects = append(projects, t.ProjectName)
		for _, a := range t.Annotations {
			if a.Type == "corvint-test-key" && a.Description != "" {
				key = a.Description
			}
		}
	}
	intent := proposedIntent(id, "ui")
	intent.Steps = []FlowStep{{StepID: "run", Action: spec.Title}}
	variation := FlowVariation{VariationID: id + ".spec", Preconditions: []string{}, Steps: []string{"run"}, ObservableFacts: []string{}, Outcomes: []string{}, Projects: sortedSet(slices.DeleteFunc(projects, func(p string) bool { return p == "" }))}
	intent.Variations = []FlowVariation{variation}
	intent.Links = []FlowLink{inferredTestLink(variation.VariationID, key, spec.File)}
	return intent
}

// importedID derives a flow-ID-shaped identity from third-party text, or "" when nothing remains.
func importedID(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	parts := strings.FieldsFunc(b.String(), func(r rune) bool { return r == '-' })
	id := strings.Join(parts, "-")
	if len(id) > maxImportedID {
		id = id[:maxImportedID]
	}
	return strings.Trim(id, "-._")
}
