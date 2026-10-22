package changee2e

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// Compile states (CCE-V0-004..006).
const (
	Compiled   = "COMPILED"
	Incomplete = "INCOMPLETE"
)

// PlacementDir is where compiled scenarios are placed, relative to the tree a proof runs in.
const PlacementDir = "e2e/corvint-compiled"

// Scenario is one compiled scenario (CCE-V0-005/007).
type Scenario struct {
	Obligation    string
	Placement     string // repository-relative placement, under PlacementDir
	Text          []byte
	TestTitle     string
	AssertionID   string
	AssertionLine int // 1-based line of the outcome assertion in Text
	// Marker is the outcome assertion's own expect message; Playwright prefixes the failure
	// message with it, which places a failure on this assertion (CCE-V0-009 step 6).
	Marker                  string
	Matcher, Locator, Value string
	// OracleOrigin is the CCE-V0-016 join of the outcome block with the base map.
	OracleOrigin string
	// Hosted marks an existing hosted test run as given (CCE-V0-034): AssertionLines are its
	// outcome assertions, EventLines the awaited actions its behaviour-removal controls omit, and
	// DeclLine its test declaration.
	Hosted         bool
	AssertionLines []int
	EventLines     []int
	DeclLine       int
}

// oracle_origin values (CCE-V0-016).
const (
	OriginAdmittedAtBase = "ADMITTED_AT_BASE"
	OriginChangedInDiff  = "CHANGED_IN_DIFF"
	OriginNewInDiff      = "NEW_IN_DIFF"
)

// Missing names one block a scenario needs that is missing or unadmitted (CCE-V0-006).
type Missing struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// CompiledEntry reports one obligation's compilation.
type CompiledEntry struct {
	Obligation    string    `json:"obligation"`
	State         string    `json:"state"`
	Placement     string    `json:"placement,omitempty"`
	FileDigest    string    `json:"file_digest,omitempty"`
	AssertionID   string    `json:"assertion_id,omitempty"`
	AssertionLine int       `json:"assertion_line,omitempty"`
	OracleOrigin  string    `json:"oracle_origin,omitempty"`
	Blocks        []string  `json:"blocks"`
	Missing       []Missing `json:"missing"`
}

// CompileReport is the e2e-compile/0 document.
type CompileReport struct {
	Schema            string          `json:"schema"`
	Compiler          string          `json:"compiler"`
	Base              string          `json:"base"`
	Head              string          `json:"head"`
	CatalogDigest     string          `json:"catalog_digest"`
	ObligationsDigest string          `json:"obligations_digest"`
	State             string          `json:"state"` // COMPILED | INCOMPLETE | NONE
	Unknown           []UnknownSpan   `json:"unknown"`
	Scenarios         []CompiledEntry `json:"scenarios"`
	AcceptancesInDiff []string        `json:"acceptances_in_diff,omitempty"` // CCE-V0-044
	Truncated         int             `json:"truncated,omitempty"`           // entries dropped by the byte cap (CCE-V0-014)
	Digest            string          `json:"digest"`

	compiled []Scenario
	encoded  []byte
}

// Encoded returns the sealed bytes of the report.
func (r *CompileReport) Encoded() []byte { return r.encoded }

// Compiled returns the emitted scenarios in obligation order.
func (r *CompileReport) Compiled() []Scenario { return r.compiled }

// Compile emits at most one scenario per REACHED obligation from admitted block text only
// (CCE-V0-005..007). An UNKNOWN obligations report compiles to INCOMPLETE overall with its spans.
func Compile(ctx context.Context, o DiffOptions) (*CompileReport, error) {
	obs, err := BuildObligations(ctx, o)
	if err != nil {
		return nil, err
	}
	return CompileObligations(obs)
}

// CompileObligations compiles an obligations report against its head catalog.
func CompileObligations(obs *Obligations) (*CompileReport, error) {
	cat := obs.catalog
	r := &CompileReport{Schema: CompileSchema, Compiler: CompilerVersion, Base: obs.Base, Head: obs.Head, CatalogDigest: cat.Digest,
		ObligationsDigest: obs.Digest, Unknown: obs.Unknown, Scenarios: []CompiledEntry{}, AcceptancesInDiff: obs.AcceptancesInDiff}
	for _, ob := range obs.Obligations {
		sc, entry := compileOne(cat, obs, ob)
		r.Scenarios = append(r.Scenarios, entry)
		if sc != nil {
			r.compiled = append(r.compiled, *sc)
		}
	}
	switch {
	case obs.State == Unknown:
		r.State = Incomplete
	case len(r.Scenarios) == 0:
		r.State = None
	default:
		r.State = Compiled
		for _, e := range r.Scenarios {
			if e.State != Compiled {
				r.State = Incomplete
			}
		}
	}
	var err error
	r.encoded, err = capList(&r.Scenarios, &r.Truncated, func() ([]byte, error) { return seal(r, &r.Digest) })
	return r, err
}

// scenarioPlan is the admitted text one scenario is built from.
type scenarioPlan struct {
	entry    *Block
	fixtures []*Block
	steps    []planStep
	outcome  *Block
}

type planStep struct {
	method *Block // a page-object method, or nil for a fill
	input  *Block
	value  string
}

func compileOne(cat *Catalog, obs *Obligations, ob Obligation) (*Scenario, CompiledEntry) {
	entry := CompiledEntry{Obligation: ob.ID, Blocks: []string{}, Missing: []Missing{}}
	miss := func(kind, id, reason string) {
		entry.Missing = append(entry.Missing, Missing{Kind: kind, ID: id, Reason: reason})
	}
	p := scenarioPlan{outcome: cat.Block(ob.Outcome)}
	if p.outcome == nil {
		miss(KindOutcome, ob.Outcome, cat.unadmittedReason(KindOutcome, ob.Outcome, "outcome-not-admitted"))
	}
	flow, expecting := cat.flowFor(ob.Outcome)
	if flow == nil || expecting < 0 {
		miss(KindPath, ob.Outcome, "no-flow-step-expects-outcome")
	} else {
		pathID := "path:entry:" + flow.Steps[0].Screen
		if p.entry = cat.Block(pathID); p.entry == nil {
			miss(KindPath, pathID, cat.unadmittedReason(KindPath, pathID, "no-admitted-path"))
		}
		for _, pre := range flow.Preconditions {
			fx := cat.fixtureFor(pre)
			if fx == nil {
				miss(KindFixture, pre, "no-admitted-fixture")
				continue
			}
			p.fixtures = append(p.fixtures, fx)
		}
		for _, st := range flow.Steps[:expecting+1] {
			ps, kind, reason := cat.stepBlock(st)
			if reason != "" {
				miss(kind, st.ID, reason)
				continue
			}
			p.steps = append(p.steps, ps)
		}
	}
	if len(entry.Missing) != 0 {
		entry.State = Incomplete
		return nil, entry
	}
	sc := render(cat, obs, ob, p)
	entry.State, entry.Placement, entry.AssertionID, entry.AssertionLine = Compiled, sc.Placement, sc.AssertionID, sc.AssertionLine
	entry.FileDigest = headerDigest(sc.Text)
	entry.Blocks = p.blockIDs()
	sc.OracleOrigin = obs.oracleOrigin(p.outcome)
	entry.OracleOrigin = sc.OracleOrigin
	return sc, entry
}

func (p scenarioPlan) blockIDs() []string {
	ids := []string{p.entry.ID, p.outcome.ID}
	for _, f := range p.fixtures {
		ids = append(ids, f.ID)
		ids = append(ids, methodID(f.Setup), methodID(f.Cleanup))
	}
	for _, s := range p.steps {
		if s.method != nil {
			ids = append(ids, s.method.ID)
		}
		if s.input != nil {
			ids = append(ids, s.input.ID)
		}
	}
	sort.Strings(ids)
	out := ids[:0]
	for i, id := range ids {
		if i == 0 || id != ids[i-1] {
			out = append(out, id)
		}
	}
	return out
}

// methodID is the AMAP method ID of a declared call (page-object file ID "file:<path>").
func methodID(c *MethodCall) string {
	return "method:" + strings.TrimPrefix(c.PageObject, "file:") + "#" + c.Method
}

func (c *Catalog) unadmittedReason(kind, id, fallback string) string {
	for _, u := range c.Unadmitted {
		if u.Kind == kind && u.ID == id {
			return u.Reason
		}
	}
	return fallback
}

// flowFor returns the flow declaring outcomeID and the index of the first step that expects it.
func (c *Catalog) flowFor(outcomeID string) (*appmap.Flow, int) {
	flowID, oc, _ := strings.Cut(strings.TrimPrefix(outcomeID, "outcome:"), "/")
	for i := range c.m.Flows {
		f := &c.m.Flows[i]
		if f.FlowID != flowID || len(f.Steps) == 0 {
			continue
		}
		for j, st := range f.Steps {
			for _, e := range c.expects[st.ID] {
				if e == oc {
					return f, j
				}
			}
		}
		return f, -1
	}
	return nil, -1
}

// fixtureFor is the lexically first admitted fixture block satisfying precondition pre.
func (c *Catalog) fixtureFor(pre string) *Block {
	for i := range c.Blocks {
		b := &c.Blocks[i]
		if b.Kind != KindFixture {
			continue
		}
		for _, s := range b.Satisfies {
			if s == pre {
				return b
			}
		}
	}
	return nil
}

// stepBlock picks the lexically first admitted page-object method the map names as reusing the
// step's selector; a fill step takes the first valid value of the admitted input on its locator.
func (c *Catalog) stepBlock(st appmap.Step) (planStep, string, string) {
	if st.Action == "fill" {
		if st.Selector == nil {
			return planStep{}, KindInput, "step-selector-unresolved"
		}
		for i := range c.Blocks {
			b := &c.Blocks[i]
			if b.Kind != KindInput || b.Screen != st.Screen || b.Locator != st.Selector.Value {
				continue
			}
			for _, v := range b.Values {
				if v.Class == "valid" {
					return planStep{input: b, value: v.Value}, "", ""
				}
			}
			return planStep{}, KindInput, "no-valid-input-value"
		}
		return planStep{}, KindInput, "no-admitted-input"
	}
	reuse := append([]string(nil), st.Reuse...)
	sort.Strings(reuse)
	for _, id := range reuse {
		if b := c.Block(id); b != nil && b.Kind == KindPageObject {
			return planStep{method: b}, "", ""
		}
	}
	if len(reuse) != 0 {
		return planStep{}, KindPageObject, c.unadmittedReason(KindPageObject, reuse[0], "reuse-method-not-admitted")
	}
	return planStep{}, KindPageObject, "no-reuse-method"
}

// Header field names, in order (CCE-V0-007).
var headerFields = []string{"catalog_digest", "base", "head", "obligation", "compiler"}

const digestLinePrefix = "// digest: "

// placementFor names an obligation's scenario file. The slug alone is not injective (a-b/c and
// a/b-c meet), so the obligation digest keeps every placement distinct (CCE-V0-007).
func placementFor(obligation string) string {
	slug := strings.NewReplacer("/", "-", ":", "-").Replace(strings.TrimPrefix(obligation, "obligation:"))
	return PlacementDir + "/" + slug + "-" + sha256Hex([]byte(obligation))[:12] + ".spec.ts"
}

// markerPrefix starts every outcome assertion's expect message.
const markerPrefix = "corvint-e2e outcome "

// render writes the scenario text. Every selector, URL, value and matcher is the exact text of an
// admitted block; string literals are single-quoted JSON-escaped text.
func render(cat *Catalog, obs *Obligations, ob Obligation, p scenarioPlan) *Scenario {
	placement := placementFor(ob.ID)
	assertionID := "assert:" + strings.TrimPrefix(ob.ID, "obligation:")
	marker := markerPrefix + assertionID
	type po struct{ class, file string }
	objects := map[string]po{}
	add := func(file, class string) {
		if class != "" {
			objects[class] = po{class, file}
		}
	}
	for _, f := range p.fixtures {
		add(fileOf(f.Setup), cat.classOf(fileOf(f.Setup)))
		add(fileOf(f.Cleanup), cat.classOf(fileOf(f.Cleanup)))
	}
	for _, s := range p.steps {
		if s.method != nil {
			add(s.method.File, s.method.Class)
		}
	}
	classes := make([]string, 0, len(objects))
	for k := range objects {
		classes = append(classes, k)
	}
	sort.Strings(classes)

	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("// corvint-test e2e compiled scenario: generated, do not edit.")
	values := []string{cat.Digest, obs.Base, obs.Head, ob.ID, CompilerVersion}
	for i, k := range headerFields {
		line("// %s: %s", k, values[i])
	}
	line("%s", digestLinePrefix)
	line("import { test, expect } from '@playwright/test';")
	for _, k := range classes {
		rel, _ := relImport(placement, objects[k].file)
		line("import { %s } from %s;", k, jsString(rel))
	}
	line("")
	line("test(%s, async ({ page }) => {", jsString(ob.ID))
	for _, k := range classes {
		line("  const %s = new %s(page);", varName(k), k)
	}
	for _, f := range p.fixtures {
		line("  await %s.%s();", varName(cat.classOf(fileOf(f.Setup))), f.Setup.Method)
	}
	line("  try {")
	line("    await page.goto(%s);", jsString(p.entry.URL))
	for _, s := range p.steps {
		if s.method != nil {
			line("    await %s.%s();", varName(s.method.Class), s.method.Method)
		} else {
			line("    await page.getByTestId(%s).fill(%s);", jsString(s.input.Locator), jsString(s.value))
		}
	}
	assertLine := strings.Count(b.String(), "\n") + 1
	line("    await expect(page.getByTestId(%s), %s).%s(%s);", jsString(p.outcome.Locator), jsString(marker), p.outcome.Matcher, jsString(p.outcome.Value))
	line("  } finally {")
	for i := len(p.fixtures) - 1; i >= 0; i-- {
		f := p.fixtures[i]
		line("    await %s.%s();", varName(cat.classOf(fileOf(f.Cleanup))), f.Cleanup.Method)
	}
	line("  }")
	line("});")
	text := []byte(b.String())
	text = []byte(strings.Replace(string(text), "\n"+digestLinePrefix+"\n", "\n"+digestLinePrefix+headerDigest(text)+"\n", 1))
	return &Scenario{Obligation: ob.ID, Placement: placement, Text: text, TestTitle: ob.ID,
		AssertionID: assertionID, AssertionLine: assertLine, Marker: marker,
		Matcher: p.outcome.Matcher, Locator: p.outcome.Locator, Value: p.outcome.Value}
}

// headerDigest is SHA-256 over text with the digest value left empty (CCE-V0-007).
func headerDigest(text []byte) string {
	s := string(text)
	i := strings.Index(s, "\n"+digestLinePrefix)
	if i >= 0 {
		j := strings.IndexByte(s[i+1:], '\n')
		s = s[:i+1] + digestLinePrefix + s[i+1+j:]
	}
	return "sha256:" + sha256Hex([]byte(s))
}

// VerifyHeader reports whether a compiled file's digest line matches its content.
func VerifyHeader(text []byte) bool {
	s := string(text)
	i := strings.Index(s, "\n"+digestLinePrefix)
	if i < 0 {
		return false
	}
	rest := s[i+1+len(digestLinePrefix):]
	got, _, _ := strings.Cut(rest, "\n")
	return got != "" && got == headerDigest(text)
}

func fileOf(c *MethodCall) string { return strings.TrimPrefix(c.PageObject, "file:") }

func (c *Catalog) classOf(file string) string {
	for _, f := range c.m.Files {
		if f.Path == file {
			return f.Class
		}
	}
	return ""
}

// relImport is the extensionless relative module specifier from placement to file.
func relImport(placement, file string) (string, error) {
	from := strings.Split(path.Dir(placement), "/")
	to := strings.Split(strings.TrimSuffix(file, path.Ext(file)), "/")
	i := 0
	for i < len(from) && i < len(to)-1 && from[i] == to[i] {
		i++
	}
	rel := strings.Repeat("../", len(from)-i) + strings.Join(to[i:], "/")
	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel, nil
}

func varName(class string) string {
	if class == "" {
		return "page"
	}
	return strings.ToLower(class[:1]) + class[1:]
}

// jsString is a single-quoted JavaScript literal of s.
func jsString(s string) string {
	b, _ := json.Marshal(s)
	inner := strings.ReplaceAll(string(b[1:len(b)-1]), `\"`, `"`)
	return "'" + strings.ReplaceAll(inner, "'", `\'`) + "'"
}
