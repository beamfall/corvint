package changee2e

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"sort"
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
)

// Span binds a block to committed bytes at the catalog revision.
type Span struct {
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
	Schema       string           `json:"schema"`
	Revision     string           `json:"revision"`
	MapDigest    string           `json:"map_digest"`
	Declarations string           `json:"declarations"` // DECLARED | NOT_DECLARED
	Environment  *Environment     `json:"environment,omitempty"`
	Blocks       []Block          `json:"blocks"`
	Unadmitted   []Unadmitted     `json:"unadmitted"`
	Unknowns     []CatalogUnknown `json:"unknowns"`
	Digest       string           `json:"digest"`

	m       *appmap.Map
	expects map[string][]string // step ID -> outcome IDs the flow intent's step expects
	encoded []byte
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
	} `json:"fixtures"`
	Environment *Environment `json:"environment"`
}

// CatalogOptions are the catalog inputs.
type CatalogOptions struct {
	Root, Rev string
	Manifest  string // repository-relative application-map manifest; default appmap.json
}

// BuildCatalog compiles the revision-pinned catalog (CCE-V0-001): page-object methods, deep-link
// entry paths and flow outcomes from the application map, inputs and fixtures from the declared
// blocks file. Candidates without proof are listed under unadmitted with a reason, never emitted
// as blocks (CCE-V0-002).
func BuildCatalog(ctx context.Context, o CatalogOptions) (*Catalog, error) {
	if o.Root == "" {
		return nil, errNoRoot
	}
	rev, err := resolveRev(ctx, o.Root, o.Rev)
	if err != nil {
		return nil, err
	}
	manifest := o.Manifest
	if manifest == "" {
		manifest = "appmap.json"
	}
	m, err := appmap.Build(ctx, o.Root, manifest, rev)
	if err != nil {
		return nil, refuse("e2e-map-unavailable", "%v", err)
	}
	c := &Catalog{Schema: CatalogSchema, Revision: rev, MapDigest: m.Digest, Blocks: []Block{}, Unadmitted: []Unadmitted{},
		Unknowns: []CatalogUnknown{}, m: m, expects: map[string][]string{}}
	pageObjects := map[string]*appmap.TestFile{}
	for i := range m.Files {
		f := &m.Files[i]
		if f.Role != "page-object" {
			continue
		}
		pageObjects[f.ID] = f
		for _, mt := range f.Methods {
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
			if !admittedMatchers[oc.Matcher] || oc.Locator == "" {
				c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: KindOutcome, ID: id, Reason: "outcome-not-compilable"})
				continue
			}
			c.Blocks = append(c.Blocks, Block{Kind: KindOutcome, ID: id, Blob: f.Anchor.Blob, Span: span(f.Anchor),
				Relation: "flow-outcome", Flow: f.FlowID, Matcher: oc.Matcher, Locator: oc.Locator, Value: oc.Value})
		}
		if err := c.readExpects(ctx, o.Root, rev, f); err != nil {
			return nil, err
		}
	}
	if err := c.addDeclarations(ctx, o.Root, rev, pageObjects); err != nil {
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
	sort.SliceStable(c.Unknowns, func(i, j int) bool {
		a, b := c.Unknowns[i], c.Unknowns[j]
		return a.Kind+"\x00"+a.Ref+"\x00"+a.Reason < b.Kind+"\x00"+b.Ref+"\x00"+b.Reason
	})
	c.encoded, err = seal(c, &c.Digest)
	return c, err
}

// admittedMatchers are the web-first text and value matchers a compiled assertion may use.
var admittedMatchers = map[string]bool{"toHaveText": true, "toContainText": true, "toHaveValue": true}

func span(a appmap.Anchor) Span {
	return Span{Path: a.Path, Start: a.Start, End: a.End, SHA256: a.SpanSHA256}
}

// addEntries admits each deep-linkable, parameter-free entry as a path block; every other entry
// row is unadmitted with its classification and reasons.
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
		default:
			c.Blocks = append(c.Blocks, Block{Kind: KindPath, ID: id, Blob: s.Anchor.Blob, Span: span(s.Anchor),
				Relation: "deep-link-entry", Screen: s.ID, URL: e.URL})
		}
	}
	return nil
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
	whole := Span{Path: DeclarationsPath, Start: 1, End: bytes.Count(data, []byte("\n")) + boolInt(!bytes.HasSuffix(data, []byte("\n"))),
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
			Satisfies: append([]string(nil), fx.Satisfies...), Setup: &setup, Cleanup: &cleanup})
	}
	return nil
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
