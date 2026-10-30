package changee2e

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"sort"
	"strconv"
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
	Hosted bool
	// Screen is the map screen the compiled outcome assertion shows; recorded coverage is
	// attributed to it (CCE-V0-053). A hosted test has none.
	Screen         string
	AssertionLines []int
	EventLines     []int
	DeclLine       int
	// SetupLines and CleanupLines are a compiled scenario's fixture setup and teardown calls
	// (CCE-V0-108): each setup of a fixture with no outputs is an omitted-event control (removing
	// one whose outputs a later block reads fails by construction, so it would be vacuous), and a
	// failure at a cleanup line reads cleanup-failed.
	SetupLines, CleanupLines []int
	// LocatorForm is the outcome locator's form, empty for a test id; for a typed locator Locator
	// is its accessible name (CCE-V0-109).
	LocatorForm string
	// SessionEnv names the storage-state variable of the scenario's session, if any (CCE-V0-107).
	SessionEnv string
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
	// ProvenPath and ProvenBy explain a path synthesised over ANL ledger edges: the screens from
	// the admitted entry and the test key proving each hop (CCE-V0-104).
	ProvenPath []string `json:"proven_path,omitempty"`
	ProvenBy   []string `json:"proven_by,omitempty"`
	// Steps map each `test.step` line of the scenario to the block it performs (CCE-V0-112).
	Steps []StepLine `json:"steps,omitempty"`
}

// StepLine is one `test.step` of a compiled scenario and the admitted block it performs.
type StepLine struct {
	Line  int    `json:"line"`
	Block string `json:"block"`
}

// CompileReport is the e2e-compile/0 document.
type CompileReport struct {
	Schema            string `json:"schema"`
	Compiler          string `json:"compiler"`
	Base              string `json:"base"`
	Head              string `json:"head"`
	CatalogDigest     string `json:"catalog_digest"`
	ObligationsDigest string `json:"obligations_digest"`
	// Roots are the head map's aliased-root pins; scenarios then run against the test tree
	// read from the tests root at its pin (CCE-V0-135).
	Roots             []appmap.RootPin `json:"roots,omitempty"`
	State             string           `json:"state"` // COMPILED | INCOMPLETE | NONE
	Unknown           []UnknownSpan    `json:"unknown"`
	Scenarios         []CompiledEntry  `json:"scenarios"`
	AcceptancesInDiff []string         `json:"acceptances_in_diff,omitempty"` // CCE-V0-044
	Truncated         int              `json:"truncated,omitempty"`           // entries dropped by the byte cap (CCE-V0-014)
	Digest            string           `json:"digest"`

	compiled []Scenario
	tests    testsRoot
	encoded  []byte
}

// TestsRoot is the aliased root a two-tree prove reads its test tree from (CCE-V0-136).
type TestsRoot struct {
	Repo     string // the manifest's tests.repo alias
	Dir      string // the alias's checkout
	Revision string // the pinned revision the catalog read
}

// TestsRoot returns the aliased tests root the report's catalog read, nil for one tree.
func (r *CompileReport) TestsRoot() *TestsRoot {
	if r.tests.repo == "" {
		return nil
	}
	return &TestsRoot{Repo: r.tests.repo, Dir: r.tests.dir, Revision: r.tests.rev}
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
		ObligationsDigest: obs.Digest, Roots: obs.Roots, Unknown: obs.Unknown, Scenarios: []CompiledEntry{}, AcceptancesInDiff: obs.AcceptancesInDiff,
		tests: cat.tests}
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
	session  *Block
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
			pe, hops, path, by, reason := cat.provenEntry(obs, flow.Steps[0].Screen, flow)
			switch {
			case pe != nil:
				p.entry, p.steps, entry.ProvenPath, entry.ProvenBy = pe, hops, path, by
			case reason != "":
				miss(KindPath, pathID, reason)
			default:
				miss(KindPath, pathID, cat.unadmittedReason(KindPath, pathID, "no-admitted-path"))
			}
		}
		for _, pre := range flow.Preconditions {
			// A fixture satisfies a precondition first, then a session (CCE-V0-107); a scenario
			// runs under at most one session.
			if fx := cat.fixtureFor(pre); fx != nil {
				if !slices.Contains(p.fixtures, fx) {
					p.fixtures = append(p.fixtures, fx)
				}
				continue
			}
			se := cat.satisfying(KindSession, pre)
			switch {
			case se == nil:
				miss(KindFixture, pre, "no-admitted-fixture")
			case p.session != nil && p.session != se:
				miss(KindSession, pre, "session-conflict")
			default:
				p.session = se
			}
		}
		if p.entry != nil {
			for _, rp := range p.entry.Params {
				// A route parameter bound to a fixture output needs that fixture in this scenario
				// (CCE-V0-106).
				if rp.Fixture != "" && !slices.ContainsFunc(p.fixtures, func(b *Block) bool { return b.ID == rp.Fixture }) {
					miss(KindPath, p.entry.ID, "route-param-unbound:"+rp.Name)
				}
			}
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
	sc, steps := render(cat, obs, ob, p)
	sc.Screen = flow.Steps[expecting].Screen
	entry.State, entry.Placement, entry.AssertionID, entry.AssertionLine = Compiled, sc.Placement, sc.AssertionID, sc.AssertionLine
	entry.Steps = steps
	entry.FileDigest = headerDigest(sc.Text)
	entry.Blocks = p.blockIDs()
	sc.OracleOrigin = obs.oracleOrigin(p.outcome)
	entry.OracleOrigin = sc.OracleOrigin
	return sc, entry
}

func (p scenarioPlan) blockIDs() []string {
	ids := []string{p.entry.ID, p.outcome.ID}
	if p.session != nil {
		ids = append(ids, p.session.ID)
	}
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
func (c *Catalog) fixtureFor(pre string) *Block { return c.satisfying(KindFixture, pre) }

// satisfying is the lexically first admitted block of kind satisfying precondition pre.
func (c *Catalog) satisfying(kind, pre string) *Block {
	for i := range c.Blocks {
		b := &c.Blocks[i]
		if b.Kind != kind {
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

// render writes the scenario text (compiler /1, CCE-V0-105..113). Every selector, URL, value and
// matcher is the exact text of an admitted block; string literals are single-quoted JSON-escaped
// text. Each fixture is a test-scoped Playwright fixture that depends on the one before it, so
// Playwright tears them down in reverse even when a later setup fails (CCE-V0-108); each action
// is one `test.step` named by its block, and the outcome assertion stands on its own line.
func render(cat *Catalog, obs *Obligations, ob Obligation, p scenarioPlan) (*Scenario, []StepLine) {
	placement := placementFor(ob.ID)
	assertionID := "assert:" + strings.TrimPrefix(ob.ID, "obligation:")
	marker := markerPrefix + assertionID
	files := map[string]string{} // class -> page-object file
	for _, f := range p.fixtures {
		files[cat.classOf(fileOf(f.Setup))] = fileOf(f.Setup)
		files[cat.classOf(fileOf(f.Cleanup))] = fileOf(f.Cleanup)
	}
	var bodyClasses []string
	for _, s := range p.steps {
		if s.method != nil {
			files[s.method.Class] = s.method.File
			if !slices.Contains(bodyClasses, s.method.Class) {
				bodyClasses = append(bodyClasses, s.method.Class)
			}
		}
	}
	delete(files, "")
	classes := make([]string, 0, len(files))
	for k := range files {
		classes = append(classes, k)
	}
	sort.Strings(classes)
	sort.Strings(bodyClasses)

	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	next := func() int { return strings.Count(b.String(), "\n") + 1 }
	sc := &Scenario{Obligation: ob.ID, Placement: placement, TestTitle: ob.ID, AssertionID: assertionID, Marker: marker,
		Matcher: p.outcome.Matcher, Locator: p.outcome.Locator, Value: p.outcome.Value}
	if t := p.outcome.Typed; t != nil {
		sc.LocatorForm, sc.Locator = t.By, t.Name
	}
	line("// corvint-test e2e compiled scenario: generated, do not edit.")
	values := []string{cat.Digest, obs.Base, obs.Head, ob.ID, CompilerVersion}
	for i, k := range headerFields {
		line("// %s: %s", k, values[i])
	}
	line("%s", digestLinePrefix)
	line("import { test as base, expect } from '@playwright/test';")
	for _, k := range classes {
		rel, _ := relImport(placement, files[k])
		line("import { %s } from %s;", k, jsString(rel))
	}
	line("")
	fxVar := map[string]string{}
	var fxNames []string
	if len(p.fixtures) == 0 {
		line("const test = base;")
	} else {
		// One test-scoped `po` fixture builds each page object once, so setup, actions and
		// cleanup share an instance per class as they did before fixtures were extended.
		types := []string{"po: Record<string, any>"}
		objs := make([]string, len(classes))
		for i, k := range classes {
			objs[i] = varName(k) + ": new " + k + "(page)"
		}
		for i, f := range p.fixtures {
			fxVar[f.ID] = "fx" + strconv.Itoa(i+1)
			fxNames = append(fxNames, fxVar[f.ID])
			types = append(types, fxVar[f.ID]+": Record<string, any>")
		}
		line("const test = base.extend<{ %s }>({", strings.Join(types, "; "))
		line("  po: async ({ page }, use) => {")
		line("    await use({ %s });", strings.Join(objs, ", "))
		line("  },")
		for i, f := range p.fixtures {
			deps := "po"
			if i > 0 {
				deps += ", " + fxNames[i-1]
			}
			line("  %s: async ({ %s }, use) => {", fxNames[i], deps)
			setupClass, cleanupClass := cat.classOf(fileOf(f.Setup)), cat.classOf(fileOf(f.Cleanup))
			var used []string
			for _, k := range uniqueStrings(sortedCopy([]string{setupClass, cleanupClass})) {
				if k != "" {
					used = append(used, varName(k))
				}
			}
			line("    const { %s } = po;", strings.Join(used, ", "))
			if len(f.Outputs) == 0 {
				sc.SetupLines = append(sc.SetupLines, next())
				line("    await %s.%s();", varName(setupClass), f.Setup.Method)
				line("    await use({});")
				sc.CleanupLines = append(sc.CleanupLines, next())
				line("    await %s.%s();", varName(cleanupClass), f.Cleanup.Method)
			} else {
				outs := make([]string, len(f.Outputs))
				for j, o := range f.Outputs {
					outs[j] = jsString(o)
				}
				// Once setup succeeds, cleanup runs even when a declared output is missing.
				line("    const out = await %s.%s();", varName(setupClass), f.Setup.Method)
				line("    try {")
				line("      for (const k of [%s]) if (out?.[k] === undefined) throw new Error(%s + k);", strings.Join(outs, ", "), jsString("corvint-e2e fixture-output-missing "))
				line("      await use(out);")
				line("    } finally {")
				sc.CleanupLines = append(sc.CleanupLines, next())
				// A declared method is callable only with no parameters, so cleanup reads any
				// setup state from its own page-object instance, not from the outputs.
				line("      await %s.%s();", varName(cleanupClass), f.Cleanup.Method)
				line("    }")
			}
			line("  },")
		}
		line("});")
	}
	line("test.use({ trace: 'retain-on-failure' });")
	if p.session != nil {
		env := "process.env." + p.session.StorageStateEnv
		sc.SessionEnv = p.session.StorageStateEnv
		line("test.use({ storageState: %s });", env)
		line("test.skip(!%s, %s);", env, jsString("corvint-e2e session-unset "+p.session.StorageStateEnv))
	}
	line("")
	if len(p.fixtures) == 0 {
		line("test(%s, async ({ page }) => {", jsString(ob.ID))
		for _, k := range bodyClasses {
			line("  const %s = new %s(page);", varName(k), k)
		}
	} else {
		line("test(%s, async ({ %s }) => {", jsString(ob.ID), strings.Join(append([]string{"page", "po"}, fxNames...), ", "))
		if len(bodyClasses) != 0 {
			vars := make([]string, len(bodyClasses))
			for i, k := range bodyClasses {
				vars[i] = varName(k)
			}
			line("  const { %s } = po;", strings.Join(vars, ", "))
		}
	}
	var steps []StepLine
	step := func(block, action string) {
		steps = append(steps, StepLine{Line: next(), Block: block})
		line("  await test.step(%s, () => %s);", jsString(block), action)
	}
	step(p.entry.ID, "page.goto("+urlExpr(p.entry, fxVar)+")")
	for _, s := range p.steps {
		if s.method != nil {
			step(s.method.ID, varName(s.method.Class)+"."+s.method.Method+"()")
		} else {
			step(s.input.ID, locatorExpr(s.input)+".fill("+jsString(s.value)+")")
		}
	}
	sc.AssertionLine = next()
	line("  await expect(%s, %s).%s(%s);", locatorExpr(p.outcome), jsString(marker), p.outcome.Matcher, expectedLiteral(p.outcome.Matcher, p.outcome.Value))
	line("});")
	text := []byte(b.String())
	sc.Text = []byte(strings.Replace(string(text), "\n"+digestLinePrefix+"\n", "\n"+digestLinePrefix+headerDigest(text)+"\n", 1))
	return sc, steps
}

// locatorExpr is the Playwright locator of an outcome or input block: its declared role or label
// locator, else its test id (CCE-V0-109).
func locatorExpr(b *Block) string {
	switch t := b.Typed; {
	case t == nil:
		return "page.getByTestId(" + jsString(b.Locator) + ")"
	case t.By == "role":
		return "page.getByRole(" + jsString(t.Role) + ", { name: " + jsString(t.Name) + ", exact: true })"
	default:
		return "page.getByLabel(" + jsString(t.Name) + ", { exact: true })"
	}
}

// expectedLiteral renders an outcome value: a count unquoted, any other value as a string
// (CCE-V0-110).
func expectedLiteral(matcher, value string) string {
	if matcher == MatcherCount {
		return value
	}
	return jsString(value)
}

// urlExpr is the entry URL: a literal, or for a declared route each `{name}` replaced by its
// URI-encoded constant or fixture output (CCE-V0-106, CCE-V0-111).
func urlExpr(entry *Block, fxVar map[string]string) string {
	if len(entry.Params) == 0 {
		return jsString(entry.URL)
	}
	byName := map[string]RouteParam{}
	for _, rp := range entry.Params {
		byName[rp.Name] = rp
	}
	var parts []string
	rest := entry.URL
	for _, loc := range urlParam.FindAllStringSubmatchIndex(entry.URL, -1) {
		offset := len(entry.URL) - len(rest)
		if lit := rest[:loc[0]-offset]; lit != "" {
			parts = append(parts, jsString(lit))
		}
		rp := byName[entry.URL[loc[2]:loc[3]]]
		val := jsString(rp.Value)
		if rp.Fixture != "" {
			val = fxVar[rp.Fixture] + "." + rp.Output
		}
		parts = append(parts, "encodeURIComponent(String("+val+"))")
		rest = entry.URL[loc[1]:]
	}
	if rest != "" {
		parts = append(parts, jsString(rest))
	}
	return strings.Join(parts, " + ")
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
