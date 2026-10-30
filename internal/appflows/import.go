package appflows

import (
	"crypto/sha256"
	"encoding/hex"
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
	FormatExtractor      = "extractor-inventory"
	maxImportedID        = 100
)

// An importer compiles raw into intents. root is the absolute --root; source is the import source's
// repository-relative path, or "". skipped lists the items it could not import (playwright-list only).
type importer func(root string, raw []byte, source string) (intents []FlowIntent, skipped []ImportSkip, err error)

var importers = map[string]importer{
	FormatAdapterRequest: importAdapterRequest,
	FormatOpenAPI:        importOpenAPI,
	FormatPlaywrightList: importPlaywrightList,
	FormatExtractor:      importExtractorInventory,
}

// ImportFormats is the --format list in refusal and help text.
const ImportFormats = "behavior-adapter-request, extractor-inventory, openapi or playwright-list"

// Import compiles raw into new proposed intents with inferred links and writes each as a new file under
// dir. It is all-or-nothing: the combined set is bounded and validated before any write, every file is
// created exclusively, and a failed write removes exactly the files this call created (AFU-V1-004,
// AFU-V1-037). Flow IDs are lowercase and every case variant of a .json name is loaded as an intent,
// so a name that differs only in case from an existing file is refused before writing.
//
// A playwright-list import is all-or-nothing only over the tests it can import: a listed test whose
// flow or variation ID collides, or whose file lies outside root, is skipped, the rest are written,
// and the returned error is an *ImportSkipped next to the written paths (AFU-V1-128..130).
func Import(root, dir string, raw []byte, format, source string) ([]string, error) {
	set, intents, skipped, err := planImport(root, dir, raw, format, source)
	if err != nil {
		return nil, fmt.Errorf("%v; %s", err, nothingWritten)
	}
	written, err := writeAll(root, set, intents)
	if err != nil || len(skipped) == 0 {
		return written, err
	}
	return written, &ImportSkipped{Written: len(written), Items: skipped}
}

// ImportSkip is one listed test a playwright-list import did not write, and why.
type ImportSkip struct {
	TestKey, FlowID, Reason string
}

// ImportSkipped reports the listed tests an otherwise successful import skipped.
type ImportSkipped struct {
	Written int
	Items   []ImportSkip
}

func (e *ImportSkipped) Error() string {
	lines := []string{fmt.Sprintf("imported %d listed tests; skipped %d:", e.Written, len(e.Items))}
	for _, s := range e.Items {
		lines = append(lines, fmt.Sprintf("skipped %q: %s", s.TestKey, s.Reason))
	}
	return strings.Join(lines, "\n")
}

const nothingWritten = "no intent from this import remains written"

// writeIntentFile is a test seam for a failed write.
var writeIntentFile = WriteConfined

func planImport(root, dir string, raw []byte, format, source string) (IntentSet, []FlowIntent, []ImportSkip, error) {
	compile, ok := importers[format]
	if !ok {
		return IntentSet{}, nil, nil, errors.New("--format must be " + ImportFormats)
	}
	set, err := LoadIntents(root, dir)
	if err != nil {
		return set, nil, nil, err
	}
	intents, skipped, err := compile(root, raw, sourcePath(root, source))
	if err != nil {
		return set, nil, nil, err
	}
	if format == FormatPlaywrightList {
		var collided []ImportSkip
		intents, collided = dropCollisions(set, intents)
		skipped = append(skipped, collided...)
	}
	if len(set.Flows)+len(intents) > MaxFlows {
		return set, nil, nil, fmt.Errorf("import would exceed %d intents", MaxFlows)
	}
	combined := IntentSet{Dir: set.Dir, Flows: append(slices.Clone(set.Flows), intents...), Retired: set.Retired}
	slices.SortFunc(combined.Flows, func(a, b FlowIntent) int { return strings.Compare(a.FlowID, b.FlowID) })
	if err = uniqueFlowIDs(combined.Flows); err != nil {
		return set, nil, nil, err
	}
	return set, intents, skipped, combined.validate()
}

// dropCollisions removes each imported intent whose flow ID another imported intent shares, or whose
// flow or variation ID an existing intent declares or retired.json reserves, and reports each one
// (AFU-V1-129).
func dropCollisions(set IntentSet, intents []FlowIntent) ([]FlowIntent, []ImportSkip) {
	owners := map[string]string{}
	for _, f := range set.Flows {
		owners[f.FlowID] = "flow " + f.FlowID + " already exists; import never overwrites an intent"
		for _, v := range f.Variations {
			owners[v.VariationID] = "variation " + v.VariationID + " is declared by flow " + f.FlowID
		}
	}
	for _, id := range set.Retired.FlowIDs {
		owners[id] = "flow " + id + " is retired and cannot be reused"
	}
	for _, id := range set.Retired.VariationIDs {
		owners[id] = "variation " + id + " is retired and cannot be reused"
	}
	count := map[string]int{}
	for _, f := range intents {
		count[f.FlowID]++
	}
	kept, skipped := []FlowIntent{}, []ImportSkip{}
	for _, f := range intents {
		reason := owners[f.FlowID]
		for _, v := range f.Variations {
			if reason == "" {
				reason = owners[v.VariationID]
			}
		}
		if count[f.FlowID] > 1 {
			reason = fmt.Sprintf("%d listed tests share this file, describe path and title within one project, so flow %s is ambiguous", count[f.FlowID], f.FlowID)
		}
		if reason == "" {
			kept = append(kept, f)
			continue
		}
		key := ""
		if len(f.Links) != 0 {
			key = f.Links[0].Target.TestKey
		}
		skipped = append(skipped, ImportSkip{TestKey: key, FlowID: f.FlowID, Reason: reason})
	}
	return kept, skipped
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
func importAdapterRequest(_ string, raw []byte, _ string) ([]FlowIntent, []ImportSkip, error) {
	result, err := doccorpus.BuildBehaviorAdapter(raw, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("behavior adapter refused the request: %v", err)
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
			return nil, nil, fmt.Errorf("flow %s: %v", flow.ID, err)
		}
		intents = append(intents, intent)
		delete(byFlow, flow.ID)
	}
	for flow := range byFlow {
		return nil, nil, fmt.Errorf("variation names unmapped flow %s", flow)
	}
	return intents, nil, nil
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
func importOpenAPI(_ string, raw []byte, source string) ([]FlowIntent, []ImportSkip, error) {
	var doc struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := lenientDecode(raw, &doc); err != nil {
		return nil, nil, err
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.0.") && !strings.HasPrefix(doc.OpenAPI, "3.1.") {
		return nil, nil, errors.New("openapi import accepts OpenAPI 3.0 or 3.1 JSON")
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
				return nil, nil, err
			}
			intents = append(intents, intent)
		}
	}
	return intents, nil, nil
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
	Title  string            `json:"title"`
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
		ExpectedStatus string             `json:"expectedStatus"`
		Status         string             `json:"status"`
		Results        []playwrightResult `json:"results"`
	} `json:"tests"`
}

// playwrightListConfig is the part of a list's config that places its spec files: Playwright
// reports each file relative to config.rootDir.
type playwrightListConfig struct {
	RootDir  string `json:"rootDir"`
	Projects []struct {
		TestDir string `json:"testDir"`
	} `json:"projects"`
}

// listedSpec is one spec of a list: its rootDir-relative file, its enclosing describe titles
// (outermost first), and once placed, its repository path, test key and flow ID.
type listedSpec struct {
	spec          playwrightSpec
	file          string
	describe      []string
	path, key, id string
}

// importPlaywrightList makes one proposed ui flow per spec of a `playwright test --list
// --reporter=json` list. Each spec file is resolved against the list's config.rootDir, or the one
// testDir its projects share, to a repository path under root; a list whose root lies outside root
// is refused. The test key is the repository path, the describe titles and the title joined by
// " > ", the form affected and the JavaScript test provider use, and the flow ID carries the
// describe path. A spec whose file leaves root is skipped and reported (AFU-V1-128, AFU-V1-129).
func importPlaywrightList(root string, raw []byte, _ string) ([]FlowIntent, []ImportSkip, error) {
	var doc struct {
		Config *playwrightListConfig `json:"config"`
		Suites []playwrightSuite     `json:"suites"`
	}
	if err := lenientDecode(raw, &doc); err != nil {
		return nil, nil, err
	}
	dir, err := playwrightListDir(root, doc.Config)
	if err != nil {
		return nil, nil, err
	}
	specs := listSpecs(doc.Suites, "", nil, true, nil)
	if len(specs) > MaxFlows {
		return nil, nil, fmt.Errorf("playwright list exceeds %d specs", MaxFlows)
	}
	placed, skipped := []listedSpec{}, []ImportSkip{}
	for _, s := range specs {
		titles := strings.Join(append(slices.Clone(s.describe), s.spec.Title), " > ")
		file := filepath.ToSlash(s.file)
		s.path = path.Join(dir, file)
		if file == "" || path.IsAbs(file) || !safePath(s.path) {
			skipped = append(skipped, ImportSkip{TestKey: file + " > " + titles, Reason: fmt.Sprintf("test file %q does not resolve to a repository path under --root", file)})
			continue
		}
		s.key = s.path + " > " + titles
		for _, t := range s.spec.Tests {
			for _, a := range t.Annotations {
				if a.Type == "corvint-test-key" && a.Description != "" {
					s.key = a.Description
				}
			}
		}
		stem := strings.TrimSuffix(path.Base(s.path), path.Ext(s.path))
		s.id = importedID(strings.Join(append([]string{stem}, append(slices.Clone(s.describe), s.spec.Title)...), "-"))
		if s.id == "" {
			skipped = append(skipped, ImportSkip{TestKey: s.key, Reason: "file, describe path and title yield no flow ID"})
			continue
		}
		placed = append(placed, s)
	}
	placed = mergeProjects(placed)
	disambiguate(placed)
	intents := []FlowIntent{}
	for _, s := range placed {
		intents = append(intents, playwrightIntent(s))
	}
	return intents, skipped, nil
}

// playwrightListDir is the repository-relative directory a list's spec files are relative to: its
// config.rootDir, else the one testDir every project declares. It refuses a list that declares
// neither, or whose directory does not exist inside root.
func playwrightListDir(root string, config *playwrightListConfig) (string, error) {
	dir := ""
	if config != nil {
		dir = config.RootDir
		for i := 0; dir == "" && i < len(config.Projects); i++ {
			if config.Projects[i].TestDir != config.Projects[0].TestDir {
				break
			}
			if i == len(config.Projects)-1 {
				dir = config.Projects[0].TestDir
			}
		}
	}
	if dir == "" {
		return "", errors.New("playwright list declares neither config.rootDir nor one testDir shared by its projects, so its test files cannot be placed; produce it with `playwright test --list --reporter=json`")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("repository root is unreadable: %v", err)
	}
	real := ""
	if filepath.IsAbs(dir) {
		real, err = filepath.EvalSymlinks(dir)
	}
	if real == "" || err != nil {
		return "", fmt.Errorf("playwright list rootDir %q is not an existing absolute directory under --root", dir)
	}
	rel, err := filepath.Rel(realRoot, real)
	rel = filepath.ToSlash(rel)
	if err == nil && rel == "." {
		return "", nil
	}
	if err != nil || !safePath(rel) {
		return "", fmt.Errorf("playwright list rootDir %q is outside --root", dir)
	}
	return rel, nil
}

// listSpecs walks a list as affected's listing walker does: a spec's file is its own, else its
// top-level suite's, and every other suite title is a describe title.
func listSpecs(suites []playwrightSuite, file string, describe []string, top bool, into []listedSpec) []listedSpec {
	for _, s := range suites {
		suiteFile, titles := file, describe
		if s.File != "" && top {
			suiteFile = s.File
		} else if s.Title != "" {
			titles = append(slices.Clone(describe), s.Title)
		}
		for _, spec := range s.Specs {
			f := suiteFile
			if spec.File != "" {
				f = spec.File
			}
			into = append(into, listedSpec{spec: spec, file: f, describe: titles})
		}
		into = listSpecs(s.Suites, suiteFile, titles, false, into)
	}
	return into
}

// mergeProjects joins the specs a list repeats once per project: specs with one file, describe
// path, title and test key whose projects do not overlap are one test, listed under each of their
// projects (AFU-V1-251). Two that share a project, or one without a project name, stay apart, so
// a duplicate within one project is still reported as a collision.
func mergeProjects(specs []listedSpec) []listedSpec {
	out := []listedSpec{}
	at := map[string]int{}
	for _, s := range specs {
		id := strings.Join(append([]string{s.path, s.key}, append(slices.Clone(s.describe), s.spec.Title)...), "\x00")
		if k, seen := at[id]; seen && disjointProjects(out[k].spec, s.spec) {
			out[k].spec.Tests = append(out[k].spec.Tests, s.spec.Tests...)
			continue
		}
		at[id] = len(out)
		out = append(out, s)
	}
	return out
}

// disjointProjects reports whether every test of a and b names a project and no project is in both.
func disjointProjects(a, b playwrightSpec) bool {
	seen := map[string]bool{}
	for _, t := range a.Tests {
		if t.ProjectName == "" {
			return false
		}
		seen[t.ProjectName] = true
	}
	for _, t := range b.Tests {
		if t.ProjectName == "" || seen[t.ProjectName] {
			return false
		}
	}
	return len(a.Tests) > 0 && len(b.Tests) > 0
}

// disambiguate suffixes each flow ID that listed tests with different keys share with a digest of
// the test's own key, so the suffix depends only on that test (AFU-V1-129). Tests with one key keep
// one ID, which dropCollisions reports.
func disambiguate(specs []listedSpec) {
	keys := map[string]map[string]bool{}
	for _, s := range specs {
		if keys[s.id] == nil {
			keys[s.id] = map[string]bool{}
		}
		keys[s.id][s.key] = true
	}
	for i := range specs {
		if len(keys[specs[i].id]) < 2 {
			continue
		}
		sum := sha256.Sum256([]byte(specs[i].key))
		suffix := "-" + hex.EncodeToString(sum[:4])
		base := specs[i].id[:min(len(specs[i].id), maxImportedID-len(suffix))]
		specs[i].id = strings.Trim(base, "-._") + suffix
	}
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

func playwrightIntent(s listedSpec) FlowIntent {
	projects := []string{}
	for _, t := range s.spec.Tests {
		projects = append(projects, t.ProjectName)
	}
	intent := proposedIntent(s.id, "ui")
	intent.Steps = []FlowStep{{StepID: "run", Action: s.spec.Title}}
	variation := FlowVariation{VariationID: s.id + ".spec", Preconditions: []string{}, Steps: []string{"run"}, ObservableFacts: []string{}, Outcomes: []string{}, Projects: sortedSet(slices.DeleteFunc(projects, func(p string) bool { return p == "" }))}
	intent.Variations = []FlowVariation{variation}
	intent.Links = []FlowLink{inferredTestLink(variation.VariationID, s.key, s.path)}
	return intent
}

// importExtractorInventory makes one proposed api flow per endpoint subject and one proposed ui flow
// per ui_surface subject of a corvint-extractor-inventory/1 document, with one inferred source link
// per exact subject anchor. Other subject kinds, relations and the inventory's gaps are not flows
// and are not imported (AFU-V1-122..AFU-V1-124).
func importExtractorInventory(_ string, raw []byte, _ string) ([]FlowIntent, []ImportSkip, error) {
	var inventory doccorpus.ExtractorInventory
	if err := lenientDecode(raw, &inventory); err != nil {
		return nil, nil, err
	}
	if inventory.Schema != doccorpus.ExtractorInventorySchema {
		return nil, nil, errors.New("extractor-inventory import accepts " + doccorpus.ExtractorInventorySchema)
	}
	intents := []FlowIntent{}
	for _, subject := range inventory.Provider.Subjects {
		kind, verb := "", ""
		switch subject.Kind {
		case "endpoint":
			kind, verb = "api", "call"
		case "ui_surface":
			kind, verb = "ui", "visit"
		default:
			continue
		}
		if len(intents) == MaxFlows {
			return nil, nil, fmt.Errorf("extractor inventory exceeds %d flow subjects", MaxFlows)
		}
		intent, err := extractorIntent(subject, kind, verb)
		if err != nil {
			return nil, nil, err
		}
		intents = append(intents, intent)
	}
	return intents, nil, nil
}

func extractorIntent(subject doccorpus.Subject, kind, verb string) (FlowIntent, error) {
	name := subject.Name
	if kind == "ui" {
		name = "ui " + name
	}
	id := importedID(name)
	if id == "" || len(subject.Evidence.Anchors) == 0 {
		return FlowIntent{}, fmt.Errorf("extractor subject %q has no flow identity or anchor", subject.ID)
	}
	intent := proposedIntent(id, kind)
	action := subject.Name
	if kind == "ui" {
		action = "visit " + subject.Name
	}
	intent.Steps = []FlowStep{{StepID: verb, Action: action}}
	variation := FlowVariation{VariationID: id + "." + verb, Preconditions: []string{}, Steps: []string{verb}, ObservableFacts: []string{}, Outcomes: []string{}, Projects: []string{}}
	intent.Variations = []FlowVariation{variation}
	for _, anchor := range subject.Evidence.Anchors {
		target := LinkTarget{Type: "source", Path: anchor.Path, StartLine: anchor.Start, EndLine: anchor.End}
		intent.Links = append(intent.Links, FlowLink{From: variation.VariationID, Basis: "inferred", Target: target})
	}
	return intent, nil
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
