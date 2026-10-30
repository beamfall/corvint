package changee2e

import (
	"bytes"
	"context"
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Block kinds (CCE-V0-001).
const (
	KindPageObject = "page-object"
	KindPath       = "path"
	KindOutcome    = "outcome"
	KindInput      = "input"
	KindFixture    = "fixture"
	KindSession    = "session"
)

// Span binds a block to committed bytes at the catalog revision.
type Span struct {
	// Repo names the aliased root the span was read from (CCE-V0-130); empty is --root.
	Repo   string `json:"repo,omitempty"`
	Path   string `json:"path"`
	Start  int    `json:"start_line"`
	End    int    `json:"end_line"`
	SHA256 string `json:"span_sha256,omitempty"`
}

// Block is one admitted, reusable building block. Exactly the kind-specific fields are set.
type Block struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Blob     string `json:"blob"`
	Span     Span   `json:"span"`
	Relation string `json:"relation"`
	// Provenance is `harvested:<blob>:<start>-<end>` on a harvested proposal (CCE-V0-041).
	Provenance string `json:"provenance,omitempty"`

	// page-object
	Class  string `json:"class,omitempty"`
	File   string `json:"file,omitempty"`
	Method string `json:"method,omitempty"`
	// path
	Screen string `json:"screen,omitempty"`
	URL    string `json:"url,omitempty"`
	// outcome
	Flow    string `json:"flow,omitempty"`
	Matcher string `json:"matcher,omitempty"`
	Locator string `json:"locator,omitempty"`
	Value   string `json:"value,omitempty"`
	// input
	Values []InputValue `json:"values,omitempty"`
	// fixture
	Satisfies []string    `json:"satisfies,omitempty"`
	Setup     *MethodCall `json:"setup,omitempty"`
	Cleanup   *MethodCall `json:"cleanup,omitempty"`
	// Outputs are the fields the fixture's setup returns, which later blocks may read (CCE-V0-105).
	Outputs []string `json:"outputs,omitempty"`
	// session (CCE-V0-107): the environment variable naming a Playwright storage-state file
	StorageStateEnv string `json:"storage_state_env,omitempty"`
	// Typed is a declared role or label locator for an outcome or input (CCE-V0-109).
	Typed *TypedLocator `json:"typed_locator,omitempty"`
	// Params bind a parameterised entry's URL parameters (CCE-V0-111).
	Params []RouteParam `json:"params,omitempty"`
}

// TypedLocator is a declared role or label locator (CCE-V0-109): `getByRole(role, { name })` or
// `getByLabel(name)`, both exact.
type TypedLocator struct {
	By   string `json:"by"` // role | label
	Role string `json:"role,omitempty"`
	Name string `json:"name"`
}

// RouteParam binds one URL parameter of a parameterised entry (CCE-V0-111) to a constant value or
// to a declared output of a fixture.
type RouteParam struct {
	Name    string `json:"name"`
	Value   string `json:"value,omitempty"`
	Fixture string `json:"fixture,omitempty"`
	Output  string `json:"output,omitempty"`
}

// APIContract is one declared API contract join (CCE-V0-114): a change to a path matching a glob
// reaches each screen whose placed file names the endpoint literal.
type APIContract struct {
	ContractID string   `json:"contract_id"`
	Paths      []string `json:"paths"`
	Endpoint   string   `json:"endpoint"`
}

// Unadmitted is a candidate block withheld from compilation, with its reason (CCE-V0-002).
type Unadmitted struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// CatalogUnknown is a named unknown that is not a block, such as `blocks-not-declared`.
type CatalogUnknown struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

// Catalog is the e2e-block-catalog/0 document.
type Catalog struct {
	Schema    string `json:"schema"`
	Revision  string `json:"revision"`
	MapDigest string `json:"map_digest"`
	// Roots are the map's aliased-root pins: each root, the revision read and what was read from
	// it (CCE-V0-130, AMAP-V0-017); absent for a single-root map.
	Roots        []appmap.RootPin `json:"roots,omitempty"`
	Declarations string           `json:"declarations"` // DECLARED | NOT_DECLARED
	Environment  *Environment     `json:"environment,omitempty"`
	Blocks       []Block          `json:"blocks"`
	Unadmitted   []Unadmitted     `json:"unadmitted"`
	Unknowns     []CatalogUnknown `json:"unknowns"`
	Truncated    int              `json:"truncated,omitempty"` // blocks dropped by the byte cap (CCE-V0-014)
	Digest       string           `json:"digest"`

	m       *appmap.Map
	tests   testsRoot           // where the map's test files were read (CCE-V0-131)
	expects map[string][]string // step ID -> outcome IDs the flow intent's step expects
	encoded []byte
	// paramEntries are entries blocked only by their required URL parameters, which a declared
	// route may bind (CCE-V0-111), by screen ID.
	paramEntries map[string]paramEntry
}

type paramEntry struct {
	screen *appmap.Screen
	url    string
	params []string
}

// Encoded returns the sealed bytes of the catalog.
func (c *Catalog) Encoded() []byte { return c.encoded }

// Block returns the admitted block with id, or nil.
func (c *Catalog) Block(id string) *Block {
	for i := range c.Blocks {
		if c.Blocks[i].ID == id {
			return &c.Blocks[i]
		}
	}
	return nil
}

// InputValue is one declared input value (CCE-V0-003).
type InputValue struct {
	ValueID string `json:"value_id"`
	Value   string `json:"value"`
	Class   string `json:"class"`
}

// MethodCall names one page-object method.
type MethodCall struct {
	PageObject string `json:"page_object"`
	Method     string `json:"method"`
}

type declarations struct {
	Schema string `json:"schema"`
	Inputs []struct {
		InputID  string       `json:"input_id"`
		ScreenID string       `json:"screen_id"`
		Locator  string       `json:"locator"`
		Values   []InputValue `json:"values"`
	} `json:"inputs"`
	Fixtures []struct {
		FixtureID string     `json:"fixture_id"`
		Satisfies []string   `json:"satisfies"`
		Setup     MethodCall `json:"setup"`
		Cleanup   MethodCall `json:"cleanup"`
		Outputs   []string   `json:"outputs"`
	} `json:"fixtures"`
	Sessions []struct {
		SessionID       string   `json:"session_id"`
		Satisfies       []string `json:"satisfies"`
		StorageStateEnv string   `json:"storage_state_env"`
	} `json:"sessions"`
	Locators []struct {
		Block string `json:"block"`
		TypedLocator
	} `json:"locators"`
	Routes []struct {
		ScreenID string       `json:"screen_id"`
		Params   []RouteParam `json:"params"`
	} `json:"routes"`
	APIContracts []APIContract    `json:"api_contracts"`
	Environment  *Environment     `json:"environment"`
	ReachExempt  []ReachExemption `json:"reach_exempt"`
}

// ReachExemption is one reviewed `reach_exempt` entry (CCE-V0-020): a path.Match glob over the
// repository-relative path (`*` does not cross `/`) and the reason a human recorded for it.
type ReachExemption struct {
	Glob   string `json:"glob"`
	Reason string `json:"reason"`
}

// CatalogOptions are the catalog inputs.
type CatalogOptions struct {
	Root, Rev string
	Manifest  string // repository-relative application-map manifest; default appmap.json
	// Roots binds the manifest's aliased roots, as `flows appmap build --repo` does (CCE-V0-130).
	Roots appmap.Roots
}

// BuildCatalog compiles the revision-pinned catalog (CCE-V0-001): page-object methods, deep-link
// entry paths and flow outcomes from the application map, inputs and fixtures from the declared
// blocks file. Candidates without proof are listed under unadmitted with a reason, never emitted
// as blocks (CCE-V0-002).
func BuildCatalog(ctx context.Context, o CatalogOptions) (*Catalog, error) {
	if o.Root == "" {
		return nil, errNoRoot
	}
	// The map reads run Git under the root; a relative root (the flag default ".") resolves here
	// so they never misread it as a revision that is not a commit (V1-1711).
	root, err := resolveExisting(o.Root)
	if err != nil {
		return nil, err
	}
	o.Root = root
	rev, err := resolveRev(ctx, o.Root, o.Rev)
	if err != nil {
		return nil, err
	}
	manifest := o.Manifest
	if manifest == "" {
		manifest = "appmap.json"
	}
	m, err := appmap.BuildRoots(ctx, o.Root, manifest, rev, o.Roots)
	if err != nil {
		return nil, refuseMapUnavailable("", err)
	}
	c := &Catalog{Schema: CatalogSchema, Revision: rev, MapDigest: m.Digest, Roots: m.Roots, Blocks: []Block{}, Unadmitted: []Unadmitted{},
		Unknowns: []CatalogUnknown{}, m: m, tests: testsRootOf(o.Root, rev, m, o.Roots), expects: map[string][]string{}, paramEntries: map[string]paramEntry{}}
	pageObjects := map[string]*appmap.TestFile{}
	for i := range m.Files {
		f := &m.Files[i]
		if f.Role != "page-object" {
			continue
		}
		pageObjects[f.ID] = f
		reporter := isReporterClass(f)
		for _, mt := range f.Methods {
			if reporter {
				// A Playwright Reporter under a declared page_objects directory is not a page object (CCE-V0-139).
				c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindPageObject, ID: mt.ID, Reason: "page-object-is-reporter"})
				continue
			}
			if !mt.Callable {
				c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindPageObject, ID: mt.ID, Reason: "method-not-callable"})
				continue
			}
			c.Blocks = append(c.Blocks, Block{Kind: KindPageObject, ID: mt.ID, Blob: mt.Anchor.Blob, Span: span(mt.Anchor),
				Relation: "page-object-method", Class: f.Class, File: f.Path, Method: mt.Name})
		}
	}
	if err := c.addEntries(ctx, o.Root, rev); err != nil {
		return nil, err
	}
	for _, e := range m.Edges {
		c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindPath, ID: "path:edge:" + e.From + "->" + e.To, Reason: "edge-unproven"})
	}
	for _, f := range m.Flows {
		for _, oc := range f.Outcomes {
			id := "outcome:" + f.FlowID + "/" + oc.ID
			if reason := outcomeAdmissionReason(oc.Matcher, oc.Locator, oc.Value); reason != "" {
				c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindOutcome, ID: id, Reason: reason})
				continue
			}
			c.Blocks = append(c.Blocks, Block{Kind: KindOutcome, ID: id, Blob: f.Anchor.Blob, Span: span(f.Anchor),
				Relation: "flow-outcome", Flow: f.FlowID, Matcher: oc.Matcher, Locator: oc.Locator, Value: oc.Value})
		}
		if err := c.readExpects(ctx, o.Root, rev, f); err != nil {
			return nil, err
		}
	}
	// Declarations and acceptances live beside the suite: in a two-repository layout they are read
	// from the pinned test root, never from --root (CCE-V0-130).
	if err := c.addDeclarations(ctx, c.tests.dir, c.tests.rev, pageObjects); err != nil {
		return nil, err
	}
	if err := c.addAcceptances(ctx, c.tests.dir, c.tests.rev); err != nil {
		return nil, err
	}
	for _, u := range m.Unknowns {
		c.Unknowns = append(c.Unknowns, CatalogUnknown{Kind: u.Kind, Ref: u.Ref, Reason: u.Reason})
	}
	sort.Slice(c.Blocks, func(i, j int) bool {
		a, b := c.Blocks[i], c.Blocks[j]
		return a.Kind < b.Kind || (a.Kind == b.Kind && a.ID < b.ID)
	})
	sort.Slice(c.Unadmitted, func(i, j int) bool {
		a, b := c.Unadmitted[i], c.Unadmitted[j]
		return a.Kind < b.Kind || (a.Kind == b.Kind && (a.ID < b.ID || (a.ID == b.ID && a.Reason < b.Reason)))
	})
	// One row per (kind, id, reason): the same edge, listed once per map edge record, reads once (CCE-V0-138).
	c.Unadmitted = slices.CompactFunc(c.Unadmitted, func(a, b Unadmitted) bool { return a == b })
	sort.SliceStable(c.Unknowns, func(i, j int) bool {
		a, b := c.Unknowns[i], c.Unknowns[j]
		return a.Kind+"\x00"+a.Ref+"\x00"+a.Reason < b.Kind+"\x00"+b.Ref+"\x00"+b.Reason
	})
	c.encoded, err = capList(&c.Blocks, &c.Truncated, func() ([]byte, error) { return seal(c, &c.Digest) })
	return c, err
}

// reporterLifecycle names the Playwright Reporter interface methods that a page object does not
// declare (CCE-V0-139).
var reporterLifecycle = map[string]bool{"onBegin": true, "onTestBegin": true, "onTestEnd": true, "onStepBegin": true,
	"onStepEnd": true, "onEnd": true, "onExit": true, "onStdOut": true, "onStdErr": true, "printsToStdio": true}

// reporterModule is the Playwright Reporter API module; only a reporter imports it.
const reporterModule = "@playwright/test/reporter"

// isReporterClass reports whether a file under a declared page_objects directory is a Playwright
// Reporter rather than a page object (CCE-V0-139). The structural signal comes first: the file
// imports the Reporter API module. Without it, a lifecycle method name counts only when the file
// also shows no page-object structure (no Page or Locator import, no selector and no literal
// navigation), so a page object that happens to name a method onEnd stays admitted.
func isReporterClass(f *appmap.TestFile) bool {
	pageTyped := false
	for _, imp := range f.Imports {
		if imp.Specifier == reporterModule {
			return true
		}
		pageTyped = pageTyped || slices.Contains(imp.Names, "Page") || slices.Contains(imp.Names, "Locator")
	}
	if pageTyped || len(f.Selectors) > 0 || len(f.Gotos) > 0 || len(f.HelperGotos) > 0 {
		return false
	}
	lifecycle := false
	for _, mt := range f.Methods {
		if len(mt.Selectors) > 0 {
			return false
		}
		lifecycle = lifecycle || reporterLifecycle[mt.Name]
	}
	return lifecycle
}

// admittedMatchers are the web-first matchers a compiled assertion may use: text and value, and
// (CCE-V0-110) the accessible name and a positive element count.
var admittedMatchers = map[string]bool{"toHaveText": true, "toContainText": true, "toHaveValue": true,
	"toHaveAccessibleName": true, MatcherCount: true}

// MatcherCount compares a positive integer count; its value renders unquoted (CCE-V0-110).
const MatcherCount = "toHaveCount"

var countValue = regexp.MustCompile(`^[1-9][0-9]{0,5}$`)

func span(a appmap.Anchor) Span {
	return Span{Repo: a.Repo, Path: a.Path, Start: a.Start, End: a.End, SHA256: a.SpanSHA256}
}

// addEntries admits each deep-linkable, parameter-free entry as a path block; every other entry
// row is unadmitted with its classification and reasons. An entry blocked only by its required
// URL parameters is also kept for a declared route to bind (CCE-V0-111).
func (c *Catalog) addEntries(ctx context.Context, root, rev string) error {
	raw, err := appmap.ProjectEntries(ctx, c.m, nil, appmap.Options{Root: root, Revision: rev, Full: true})
	if err != nil {
		return refuse("e2e-map-unavailable", "entries: %v", err)
	}
	var doc struct {
		Entries []struct {
			ID             string   `json:"id"`
			Classification string   `json:"classification"`
			Reasons        []string `json:"reasons"`
			URL            string   `json:"entry_url_template"`
			RequiredParams []string `json:"required_params"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return refuse("e2e-map-unavailable", "entries: %v", err)
	}
	screens := map[string]*appmap.Screen{}
	for i := range c.m.Screens {
		screens[c.m.Screens[i].ID] = &c.m.Screens[i]
	}
	for _, e := range doc.Entries {
		id := "path:entry:" + e.ID
		s := screens[e.ID]
		switch {
		case s == nil:
			c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindPath, ID: id, Reason: "screen-unresolved"})
		case e.Classification != "deep-linkable" || len(e.RequiredParams) != 0 || e.URL == "" || strings.ContainsAny(e.URL, ":{}?*"):
			reason := e.Classification
			if len(e.Reasons) != 0 {
				reason += ": " + strings.Join(e.Reasons, ",")
			}
			c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindPath, ID: id, Reason: reason})
			if params, ok := bindableURL(e.Classification, e.Reasons, e.URL, e.RequiredParams); ok {
				c.paramEntries[s.ID] = paramEntry{screen: s, url: e.URL, params: params}
			}
		default:
			c.Blocks = append(c.Blocks, Block{Kind: KindPath, ID: id, Blob: s.Anchor.Blob, Span: span(s.Anchor),
				Relation: "deep-link-entry", Screen: s.ID, URL: e.URL})
		}
	}
	return nil
}

// urlParam is one `{name}` placeholder of an entry URL template.
var urlParam = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// bindableURL reports whether an entry is blocked only by its required parameters: deep-linkable
// or requiring parent parameters, with no reason, and a URL whose only placeholders are exactly
// the required names, each once.
func bindableURL(class string, reasons []string, url string, required []string) ([]string, bool) {
	if (class != "deep-linkable" && class != "requires-parent-params") || len(reasons) != 0 || len(required) == 0 || url == "" {
		return nil, false
	}
	rest := urlParam.ReplaceAllString(url, "")
	if strings.ContainsAny(rest, ":{}?*") {
		return nil, false
	}
	var names []string
	for _, m := range urlParam.FindAllStringSubmatch(url, -1) {
		names = append(names, m[1])
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	want := append([]string(nil), required...)
	sort.Strings(want)
	if strings.Join(sorted, "\x00") != strings.Join(want, "\x00") {
		return nil, false
	}
	return want, true
}

// readExpects reads the flow intent's navigation step expectations at rev; the map's step type
// does not carry them.
func (c *Catalog) readExpects(ctx context.Context, root, rev string, f appmap.Flow) error {
	data, _, ok, err := blobAt(ctx, root, rev, f.Anchor.Path)
	if err != nil || !ok {
		return err
	}
	var intent struct {
		Navigation struct {
			Steps []struct {
				StepID string   `json:"step_id"`
				Expect []string `json:"expect"`
			} `json:"steps"`
		} `json:"navigation"`
	}
	if json.Unmarshal(data, &intent) != nil {
		return nil
	}
	for _, s := range intent.Navigation.Steps {
		if len(s.Expect) != 0 {
			c.expects["step:"+f.FlowID+"/"+s.StepID] = append([]string(nil), s.Expect...)
		}
	}
	return nil
}

var (
	declaredID     = regexp.MustCompile(`^[a-z][a-z0-9-]*:[A-Za-z0-9._/-]{1,128}$`)
	secretShaped   = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|bearer|private[_-]?key)|AKIA[0-9A-Z]{16}|-----BEGIN|gh[pousr]_[A-Za-z0-9]{20,}|sk-[A-Za-z0-9]{20,}`)
	nonEmptyText   = func(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 512 }
	allowedClasses = map[string]bool{"valid": true, "invalid": true}
)

// addDeclarations reads `.corvint/e2e-blocks.json` at rev (CCE-V0-003): a closed schema whose
// inputs and fixtures become blocks bound to the file's blob; a missing file is the named unknown
// `blocks-not-declared`, any schema error refuses `e2e-blocks-invalid`, and a secret-shaped value
// refuses `e2e-input-secret`. A fixture whose setup or cleanup is not an admitted page-object
// method is unadmitted.
func (c *Catalog) addDeclarations(ctx context.Context, root, rev string, pageObjects map[string]*appmap.TestFile) error {
	data, blob, ok, err := blobAt(ctx, root, rev, DeclarationsPath)
	if err != nil {
		return err
	}
	if !ok {
		c.Declarations = "NOT_DECLARED"
		c.Unknowns = append(c.Unknowns, CatalogUnknown{Kind: "declarations", Ref: DeclarationsPath, Reason: "blocks-not-declared"})
		return nil
	}
	c.Declarations = "DECLARED"
	d, err := decodeDeclarations(data)
	if err != nil {
		return err
	}
	c.Environment = d.Environment
	whole := Span{Path: DeclarationsPath, Repo: c.tests.repo, Start: 1, End: bytes.Count(data, []byte("\n")) + boolInt(!bytes.HasSuffix(data, []byte("\n"))),
		SHA256: sha256Hex(data)}
	seen := map[string]bool{}
	for _, in := range d.Inputs {
		if !declaredID.MatchString(in.InputID) || seen[in.InputID] || !nonEmptyText(in.ScreenID) || !nonEmptyText(in.Locator) || len(in.Values) == 0 {
			return refuse("e2e-blocks-invalid", "input %q", in.InputID)
		}
		seen[in.InputID] = true
		vals := map[string]bool{}
		for _, v := range in.Values {
			if !nonEmptyText(v.ValueID) || vals[v.ValueID] || !allowedClasses[v.Class] || len(v.Value) > 512 {
				return refuse("e2e-blocks-invalid", "input %q value %q", in.InputID, v.ValueID)
			}
			vals[v.ValueID] = true
			if secretShaped.MatchString(v.Value) || secretscreen.MatchString(v.Value) {
				return refuse("e2e-input-secret", "input %q value %q", in.InputID, v.ValueID)
			}
		}
		c.Blocks = append(c.Blocks, Block{Kind: KindInput, ID: in.InputID, Blob: blob, Span: whole, Relation: "declared-input",
			Screen: in.ScreenID, Locator: in.Locator, Values: in.Values})
	}
	for _, fx := range d.Fixtures {
		if !declaredID.MatchString(fx.FixtureID) || seen[fx.FixtureID] || len(fx.Satisfies) == 0 {
			return refuse("e2e-blocks-invalid", "fixture %q", fx.FixtureID)
		}
		seen[fx.FixtureID] = true
		for _, s := range fx.Satisfies {
			if !nonEmptyText(s) {
				return refuse("e2e-blocks-invalid", "fixture %q satisfies", fx.FixtureID)
			}
		}
		outs := map[string]bool{}
		for _, o := range fx.Outputs {
			if !outputName.MatchString(o) || outs[o] || len(fx.Outputs) > maxOutputs {
				return refuse("e2e-blocks-invalid", "fixture %q output %q", fx.FixtureID, o)
			}
			outs[o] = true
		}
		setup, cleanup := fx.Setup, fx.Cleanup
		if reason := c.methodReason(pageObjects, setup); reason != "" {
			c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindFixture, ID: fx.FixtureID, Reason: "setup-" + reason})
			continue
		}
		if reason := c.methodReason(pageObjects, cleanup); reason != "" {
			c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindFixture, ID: fx.FixtureID, Reason: "cleanup-" + reason})
			continue
		}
		c.Blocks = append(c.Blocks, Block{Kind: KindFixture, ID: fx.FixtureID, Blob: blob, Span: whole, Relation: "declared-fixture",
			Satisfies: append([]string(nil), fx.Satisfies...), Setup: &setup, Cleanup: &cleanup, Outputs: append([]string(nil), fx.Outputs...)})
	}
	for _, se := range d.Sessions {
		n := se.StorageStateEnv
		if !declaredID.MatchString(se.SessionID) || !strings.HasPrefix(se.SessionID, "session:") || seen[se.SessionID] || len(se.Satisfies) == 0 ||
			!envName.MatchString(n) || reservedEnvNames[n] || strings.HasPrefix(n, "CORVINT_") {
			return refuse("e2e-blocks-invalid", "session %q", se.SessionID)
		}
		seen[se.SessionID] = true
		for _, s := range se.Satisfies {
			if !nonEmptyText(s) {
				return refuse("e2e-blocks-invalid", "session %q satisfies", se.SessionID)
			}
		}
		c.Blocks = append(c.Blocks, Block{Kind: KindSession, ID: se.SessionID, Blob: blob, Span: whole, Relation: "declared-session",
			Satisfies: append([]string(nil), se.Satisfies...), StorageStateEnv: n})
	}
	typed := map[string]bool{}
	for _, l := range d.Locators {
		t := l.TypedLocator
		if typed[l.Block] || !nonEmptyText(t.Name) || (t.By == "role") != ariaRole.MatchString(t.Role) || (t.By != "role" && t.By != "label") {
			return refuse("e2e-blocks-invalid", "locator %q", l.Block)
		}
		typed[l.Block] = true
		if secretShaped.MatchString(t.Name) || secretscreen.MatchString(t.Name) {
			return refuse("e2e-input-secret", "locator %q", l.Block)
		}
		b := c.Block(l.Block)
		if b == nil || (b.Kind != KindOutcome && b.Kind != KindInput) {
			c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: "locator", ID: l.Block, Reason: "locator-block-unadmitted"})
			continue
		}
		b.Typed = &t
	}
	return c.addRoutes(d, blob, whole)
}

var (
	outputName = regexp.MustCompile(`^[a-z][A-Za-z0-9]{0,63}$`)
	ariaRole   = regexp.MustCompile(`^[a-z]{2,32}$`)
)

// maxOutputs bounds a fixture's declared outputs.
const maxOutputs = 8

// addRoutes admits each declared route that binds every required parameter of a parameterised
// entry (CCE-V0-111) as that entry's path block, replacing its unadmitted row. A fixture-bound
// parameter must name a declared fixture output; a constant is secret-screened. A route on a
// screen with no parameterised entry, or with other parameter names, is unadmitted.
func (c *Catalog) addRoutes(d declarations, blob string, whole Span) error {
	routed := map[string]bool{}
	for _, rt := range d.Routes {
		id := "path:entry:" + rt.ScreenID
		if !nonEmptyText(rt.ScreenID) || routed[rt.ScreenID] || len(rt.Params) == 0 {
			return refuse("e2e-blocks-invalid", "route %q", rt.ScreenID)
		}
		routed[rt.ScreenID] = true
		var names []string
		for _, p := range rt.Params {
			constant := p.Value != "" && p.Fixture == "" && p.Output == ""
			bound := p.Value == "" && declaredID.MatchString(p.Fixture) && outputName.MatchString(p.Output)
			if !urlParamName.MatchString(p.Name) || constant == bound || len(p.Value) > 512 {
				return refuse("e2e-blocks-invalid", "route %q param %q", rt.ScreenID, p.Name)
			}
			if secretShaped.MatchString(p.Value) || secretscreen.MatchString(p.Value) {
				return refuse("e2e-input-secret", "route %q param %q", rt.ScreenID, p.Name)
			}
			names = append(names, p.Name)
		}
		sort.Strings(names)
		pe, ok := c.paramEntries[rt.ScreenID]
		if !ok {
			// An entry that is guarded or otherwise unadmitted keeps its own, more specific reason.
			if c.unadmittedReason(KindPath, id, "") == "" {
				c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindPath, ID: id, Reason: "route-not-parameterised"})
			}
			continue
		}
		reason := c.routeFixtureReason(rt.Params)
		if strings.Join(names, ",") != strings.Join(pe.params, ",") {
			reason = "route-params-mismatch"
		}
		// The route's verdict replaces the entry's requires-parent-params reason.
		kept := c.Unadmitted[:0]
		for _, u := range c.Unadmitted {
			if u.Kind != KindPath || u.ID != id {
				kept = append(kept, u)
			}
		}
		c.Unadmitted = kept
		if reason != "" {
			c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindPath, ID: id, Reason: reason})
			continue
		}
		params := append([]RouteParam(nil), rt.Params...)
		sort.Slice(params, func(i, j int) bool { return params[i].Name < params[j].Name })
		c.Blocks = append(c.Blocks, Block{Kind: KindPath, ID: id, Blob: pe.screen.Anchor.Blob, Span: span(pe.screen.Anchor),
			Relation: "declared-route", Screen: pe.screen.ID, URL: pe.url, Params: params,
			Provenance: "route:" + blob + ":" + strconv.Itoa(whole.Start) + "-" + strconv.Itoa(whole.End)})
	}
	return nil
}

var urlParamName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// routeFixtureReason is empty when every fixture-bound parameter names an admitted fixture that
// declares the output.
func (c *Catalog) routeFixtureReason(params []RouteParam) string {
	for _, p := range params {
		if p.Fixture == "" {
			continue
		}
		fx := c.Block(p.Fixture)
		if fx == nil || fx.Kind != KindFixture {
			return "route-fixture-unadmitted"
		}
		if !slices.Contains(fx.Outputs, p.Output) {
			return "route-output-undeclared"
		}
	}
	return ""
}

// decodeDeclarations decodes the closed blocks schema and validates its environment member.
func decodeDeclarations(data []byte) (declarations, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var d declarations
	if err := dec.Decode(&d); err != nil || dec.More() {
		return d, refuse("e2e-blocks-invalid", "%s: %v", DeclarationsPath, err)
	}
	if d.Schema != DeclarationsSchema {
		return d, refuse("e2e-blocks-invalid", "%s: schema %q", DeclarationsPath, d.Schema)
	}
	if d.Environment != nil {
		if err := d.Environment.validate(); err != nil {
			return d, err
		}
	}
	for _, a := range d.APIContracts {
		ok := declaredID.MatchString(a.ContractID) && len(a.Paths) != 0 && strings.HasPrefix(a.Endpoint, "/") && len(a.Endpoint) > 1 &&
			len(a.Endpoint) <= 256 && !strings.ContainsAny(a.Endpoint, " \t\n\r'\"`")
		for _, g := range a.Paths {
			if _, err := path.Match(g, ""); err != nil || !nonEmptyText(g) || strings.HasPrefix(g, "/") {
				ok = false
			}
		}
		if !ok {
			return d, refuse("e2e-blocks-invalid", "%s: api_contracts %q", DeclarationsPath, a.ContractID)
		}
	}
	for _, e := range d.ReachExempt {
		if _, err := path.Match(e.Glob, ""); err != nil || !nonEmptyText(e.Glob) || strings.HasPrefix(e.Glob, "/") || !nonEmptyText(e.Reason) {
			return d, refuse("e2e-blocks-invalid", "%s: reach_exempt %q", DeclarationsPath, e.Glob)
		}
	}
	return d, nil
}

// methodReason is empty when call names a callable method of an admitted page object.
func (c *Catalog) methodReason(pageObjects map[string]*appmap.TestFile, call MethodCall) string {
	f := pageObjects[call.PageObject]
	if f == nil {
		return "page-object-unknown"
	}
	for _, mt := range f.Methods {
		if mt.Name == call.Method {
			if !mt.Callable {
				return "method-not-callable"
			}
			return ""
		}
	}
	return "method-unknown"
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// outcomeAdmissionReason is empty for an admitted outcome: an admitted matcher on a non-empty
// locator compared with a non-empty value. An empty value is `outcome-vacuous` (CCE-V0-024),
// since the compiled assertion would hold on any element text.
func outcomeAdmissionReason(matcher, locator, value string) string {
	switch {
	case !admittedMatchers[matcher] || locator == "":
		return "outcome-not-compilable"
	case value == "" || matcher == MatcherCount && value == "0":
		// A zero count holds when the locator matches nothing, so it is vacuous (CCE-V0-110).
		return "outcome-vacuous"
	case matcher == MatcherCount && !countValue.MatchString(value):
		return "outcome-not-compilable"
	}
	return ""
}
