package appmap

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/jsonend"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Site knowledge (docs/specs/site-knowledge-howto-v0.md, SKH-V0). Stage 1 reads page objects
// (SKH-V0-010..014) and stage 2 normalises spec bodies and page-object methods into one closed
// step IR (SKH-V0-020..023). Everything is learned, lexical, static and pinned to the blobs read.
const (
	PageObjectsSchema = "site-knowledge-page-objects/0"
	StepsSchema       = "site-knowledge-steps/0"

	skAuthority = "learned"

	// Form identities (SKH-V0-010).
	FormPlaywrightClass  = "playwright-ts-class"
	FormPlaywrightModule = "playwright-ts-module"
	FormCypressClass     = "cypress-ts-class"
	FormCypressModule    = "cypress-ts-module"
	FormCapybaraClass    = "capybara-ruby-class"
	FormSitePrismPage    = "siteprism-ruby-page"

	// Closed step kinds (SKH-V0-020). skCall exists only in a page object's action list before
	// inlining; the step IR never carries it.
	StepVisit   = "visit"
	StepClick   = "click"
	StepFill    = "fill"
	StepSelect  = "select"
	StepCheck   = "check"
	StepUpload  = "upload"
	StepWaitFor = "wait-for"
	StepAssert  = "assert"
	StepPress   = "press"
	StepFocus   = "focus"
	StepHover   = "hover"
	StepOpaque  = "opaque"
	skCall      = "call"

	// Closed opaque reasons (SKH-V0-020).
	OpaqueParseUnsupported = "parse-unsupported"
	OpaqueDynamicValue     = "dynamic-value"
	OpaqueUnresolvedCall   = "unresolved-call"

	// Bounds set before the extractors were built (build log 2026-10-22-sk-1022l).
	maxSKFiles     = 4096
	maxSKHops      = 8
	maxSKBodySteps = 512
	maxSKInline    = 2048
)

var skStepKinds = map[string]bool{StepVisit: true, StepClick: true, StepFill: true, StepSelect: true, StepCheck: true,
	StepUpload: true, StepWaitFor: true, StepAssert: true, StepPress: true, StepFocus: true, StepHover: true, StepOpaque: true}

// skKeyboard is the typed target of a press or fill sent to the focused element through the
// page keyboard rather than to a located element (SKH-V0-024).
const skKeyboard = "keyboard"

var skOpaqueReasons = map[string]bool{OpaqueParseUnsupported: true, OpaqueDynamicValue: true, OpaqueUnresolvedCall: true}

// SKArg is one call argument: an exact literal, a caller parameter slot, or dynamic.
type SKArg struct {
	Literal string `json:"literal,omitempty"`
	Slot    string `json:"slot,omitempty"`
	Dynamic bool   `json:"dynamic,omitempty"`
	// Template lists the parameter slots a template literal interpolates; Literal then holds the
	// text with each written as ${slot}.
	Template []string `json:"template,omitempty"`
}

// resolveArg binds an argument's slots through the caller's bindings.
func resolveArg(a SKArg, bind map[string]SKArg) SKArg {
	if a.Slot != "" {
		if o, ok := bind[a.Slot]; ok {
			return o
		}
		return a
	}
	if len(a.Template) == 0 {
		return a
	}
	out := SKArg{Literal: a.Literal}
	for _, sl := range a.Template {
		o, ok := bind[sl]
		mark := "${" + sl + "}"
		switch {
		case !ok:
			out.Template = append(out.Template, sl)
		case o.Dynamic:
			return SKArg{Dynamic: true}
		case o.Slot != "":
			out.Literal = strings.ReplaceAll(out.Literal, mark, "${"+o.Slot+"}")
			out.Template = append(out.Template, o.Slot)
		default:
			out.Literal = strings.ReplaceAll(out.Literal, mark, o.Literal)
			out.Template = append(out.Template, o.Template...)
		}
	}
	return out
}

// SKStep is one record of the closed step IR (SKH-V0-020, SKH-V0-023). Value and Expected hold
// exact literals only; a value read from a parameter names its slot instead, and a screened value
// is never kept (SKH-V0-006).
type SKStep struct {
	Kind     string    `json:"kind"`
	Locator  *Selector `json:"locator,omitempty"`
	Target   string    `json:"target,omitempty"`
	Matcher  string    `json:"matcher,omitempty"`
	Expected string    `json:"expected,omitempty"`
	Value    string    `json:"value,omitempty"`
	Slots    []string  `json:"slots,omitempty"`
	Screened bool      `json:"screened,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	Call     string    `json:"call,omitempty"`
	Args     []SKArg   `json:"args,omitempty"`
	// LocatorSlot names the parameter a locator's value is read from; binding a literal argument
	// to it rebuilds the locator (SKH-V0-021).
	LocatorSlot string   `json:"locator_slot,omitempty"`
	Via         []string `json:"via,omitempty"`
	Anchor      Anchor   `json:"anchor"`
}

// SKBinding is one locator bound to a page-object property or SitePrism element.
type SKBinding struct {
	Name    string   `json:"name"`
	Locator Selector `json:"locator"`
	Anchor  Anchor   `json:"anchor"`
}

// SKHop is one followed call out of a method: to a sibling, an inherited or included method, or a
// helper function, one bounded anchored hop at a time (SKH-V0-012).
type SKHop struct {
	Call     string `json:"call"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Resolved string `json:"resolved,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Line     int    `json:"line"`
}

// SKMethod is one page-object method with its parameters, ordered actions and effect.
type SKMethod struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Params []string `json:"params"`
	Effect string   `json:"effect"`
	// EffectReason names why Effect is unknown, from a closed set (SKH-V0-016).
	EffectReason string   `json:"effect_reason,omitempty"`
	Actions      []SKStep `json:"actions"`
	Hops         []SKHop  `json:"hops"`
	Anchor       Anchor   `json:"anchor"`
}

// SKPageObject is one page-object class or module (SKH-V0-010, SKH-V0-011).
type SKPageObject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Form        string `json:"form"`
	Path        string `json:"path"`
	Screen      string `json:"screen,omitempty"`
	ScreenBasis string `json:"screen_basis,omitempty"`
	Join        string `json:"join"`
	// JoinReason names why Join is unknown, from a closed set (SKH-V0-015).
	JoinReason string `json:"join_reason,omitempty"`
	// JoinDetail names the first unresolved visit's cause: a screen-index URL reason
	// (no-matching-screen, ambiguous-template, ...) or dynamic-target.
	JoinDetail string      `json:"join_detail,omitempty"`
	Extends    string      `json:"extends,omitempty"`
	Includes   []string    `json:"includes,omitempty"`
	URL        string      `json:"url,omitempty"`
	Bindings   []SKBinding `json:"bindings"`
	Methods    []SKMethod  `json:"methods"`
	Authority  string      `json:"authority"`
	Anchor     Anchor      `json:"anchor"`
}

// SKUnknown names one construct the extractors do not support (SKH-V0-002).
type SKUnknown struct {
	Path      string `json:"path"`
	Line      int    `json:"line,omitempty"`
	Form      string `json:"form,omitempty"`
	Construct string `json:"construct"`
	Knowledge string `json:"knowledge"`
	Reason    string `json:"reason"`
}

// SKTest is one test body normalised into the step IR (SKH-V0-020, SKH-V0-021).
type SKTest struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Path      string   `json:"path"`
	Framework string   `json:"framework"`
	Complete  bool     `json:"complete"`
	Record    string   `json:"record"`
	Steps     []SKStep `json:"steps"`
	// Setup names the resolved non-page helpers the body calls: setup knowledge for stage 3, not
	// UI steps (SKH-V0-030).
	Setup     []string `json:"setup,omitempty"`
	Authority string   `json:"authority"`
	Anchor    Anchor   `json:"anchor"`
}

// SKRecord is one deduplicated step sequence with its usage (SKH-V0-022).
type SKRecord struct {
	ID       string   `json:"id"`
	Source   string   `json:"source"`
	Steps    int      `json:"steps"`
	Complete bool     `json:"complete"`
	Usage    int      `json:"usage"`
	Tests    []string `json:"tests"`
}

// SKModel is the extracted knowledge of one source set at one revision.
type SKModel struct {
	PageObjects []SKPageObject
	Tests       []SKTest
	Records     []SKRecord
	Unknowns    []SKUnknown
	files       map[string]*skFile
	blobs       map[string]string      // committed path -> blob id under the declared sources
	roots       map[string]*sourceRoot // alias ("" is --root) -> the root the sources were read from
}

// stepScreen is the screen a step runs on through a page object (SKH-V0-185): the screen of the
// page object whose method is the innermost entry of its call chain, when that page object's join
// resolved through a manifest page_object_screens entry or the page-object-url join, or "" when
// it is not joined. An outer page object's screen is never borrowed.
func (k *SKModel) stepScreen(s SKStep) string {
	if len(s.Via) == 0 {
		return ""
	}
	via := s.Via[len(s.Via)-1]
	for j := range k.PageObjects {
		po := &k.PageObjects[j]
		if po.Join == StatusResolved && po.Screen != "" && strings.HasPrefix(via, po.ID+".") && !strings.Contains(via[len(po.ID)+1:], ".") {
			return po.Screen
		}
	}
	return ""
}

// skSource is one committed input file.
type skSource struct {
	entry blobEntry
	data  []byte
}

// skUnit is one callable body: a class method, a module function or a test body.
type skUnit struct {
	file     *skFile
	class    *skClass
	name     string
	params   []string
	pageArgs map[string]bool // parameters that carry the page or cy root
	root     bool            // a page function: takes the page, or drives the global cy
	setup    []string        // non-page helpers called (tests only)
	read     bool
	steps    []SKStep
	from, to int // body tokens [from, to); Ruby: body lines [from, to] (1-based)
	open     int // TS: the parameter list's open paren
	start    int // first line
	end      int // last line
	title    string
	method   *SKMethod
}

type skClass struct {
	name     string
	extends  string
	includes []string
	form     string
	url      string
	start    int
	end      int
	props    map[string]SKBinding
	roots    map[string]bool // properties holding the page
	file     *skFile
	methods  map[string]*skUnit
	order    []string
	po       *SKPageObject
}

type skFile struct {
	path     string
	entry    blobEntry
	data     []byte
	ruby     bool
	toks     []token
	raw      []string
	rb       *rubyFile
	classes  map[string]*skClass
	corder   []string
	funcs    map[string]*skUnit
	forder   []string
	imports  map[string]skImport // local name -> import
	nested   map[string]bool     // funcs declared inside a callback (spec-local helpers)
	shadowed map[string]bool     // nested helper names declared more than once
	consts   map[string]string   // module-level string constants
	tests    []*skUnit
	cypress  bool
}

// Action vocabularies (SKH-V0-020). Playwright and Cypress method names map onto the IR kinds.
var (
	skActionKinds = map[string]string{
		"click": StepClick, "dblclick": StepClick, "tap": StepClick,
		"fill": StepFill, "type": StepFill, "pressSequentially": StepFill,
		"selectOption": StepSelect, "select": StepSelect,
		"check": StepCheck, "uncheck": StepCheck, "setChecked": StepCheck,
		"setInputFiles": StepUpload, "selectFile": StepUpload, "attachFile": StepUpload,
		"goto": StepVisit, "visit": StepVisit,
		"waitForURL": StepWaitFor, "waitForSelector": StepWaitFor, "waitFor": StepWaitFor, "waitForLoadState": StepWaitFor,
		"should": StepAssert,
		"press":  StepPress, "focus": StepFocus, "hover": StepHover,
	}
	// Chain links that refine a locator without changing what it names for the IR.
	skRefiners = wordSet("first last nth filter and or getByTestId getByRole getByLabel getByPlaceholder getByAltText " +
		"getByTitle getByText locator get contains find within frameLocator")
	skSensitiveField = regexp.MustCompile(`(?i)pass(word|code|phrase)?|secret|token|otp|\bpin\b|api.?key|credential|ssn|card`)
	skUnsupported    = map[string]string{".java": "selenium-java", ".py": "selenium-python", ".cs": "selenium-csharp",
		".feature": "gherkin-feature", ".kt": "selenium-kotlin", ".php": "codeception-php"}
)

func skDigest(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

// screenValue screens one typed literal (SKH-V0-006): a secret-shaped value, or any value typed
// into a sensitive-looking field, is replaced by a named slot and never kept.
func screenValue(value string, field *Selector) (string, bool) {
	if secretscreen.MatchString(value) {
		return "", true
	}
	if field != nil && (skSensitiveField.MatchString(field.Value) || skSensitiveField.MatchString(field.Name)) {
		return "", true
	}
	return value, false
}

// ExtractSiteKnowledge reads page objects and test bodies from committed sources (SKH-V0-010,
// SKH-V0-020). It never runs code and never reads anything but the given bytes (SKH-V0-005).
func extractSiteKnowledge(sources []skSource, joins map[string]TestFile, screens *screenIndex) *SKModel {
	if screens == nil {
		screens = newScreenIndex("", "", nil)
	}
	m := &SKModel{PageObjects: []SKPageObject{}, Tests: []SKTest{}, Records: []SKRecord{}, Unknowns: []SKUnknown{}, files: map[string]*skFile{}}
	sort.Slice(sources, func(i, j int) bool { return sources[i].entry.path < sources[j].entry.path })
	paths := []string{}
	for _, s := range sources {
		ext := strings.ToLower(path.Ext(s.entry.path))
		if form := skUnsupported[ext]; form != "" {
			m.Unknowns = append(m.Unknowns, SKUnknown{Path: s.entry.path, Form: form, Construct: "file",
				Knowledge: "page-objects", Reason: "form-unsupported"})
			continue
		}
		var f *skFile
		switch {
		case webSuffix[ext]:
			f = readSKTS(s, m)
		case ext == ".rb":
			f = readSKRuby(s, m)
		}
		if f != nil {
			m.files[f.path] = f
			paths = append(paths, f.path)
		}
	}
	for _, p := range paths {
		f := m.files[p]
		for _, cn := range f.corder {
			c := f.classes[cn]
			po := SKPageObject{ID: "page-object:" + p + "#" + c.name, Name: c.name, Form: c.form, Path: p, Join: StatusUnknown,
				Extends: c.extends, Includes: c.includes, URL: c.url, Bindings: []SKBinding{}, Methods: []SKMethod{},
				Authority: skAuthority, Anchor: spanOf(f.entry, f.data, c.start, c.end)}
			for _, name := range sortedKeys(c.props) {
				po.Bindings = append(po.Bindings, c.props[name])
			}
			c.po = &po
			m.PageObjects = append(m.PageObjects, po)
		}
		if mod := skModuleForm(f); mod != "" {
			po := SKPageObject{ID: "page-object:" + p, Name: strings.TrimSuffix(path.Base(p), path.Ext(p)), Form: mod, Path: p,
				Join: StatusUnknown, Bindings: []SKBinding{}, Methods: []SKMethod{}, Authority: skAuthority, Anchor: wholeFile(f.entry, f.data)}
			m.PageObjects = append(m.PageObjects, po)
		}
	}
	// Methods are read after every class is known, so a hop can resolve across files.
	for i := range m.PageObjects {
		po := &m.PageObjects[i]
		f := m.files[po.Path]
		var units []*skUnit
		if c := f.classes[po.Name]; c != nil && po.ID == "page-object:"+po.Path+"#"+c.name {
			for _, n := range c.order {
				units = append(units, c.methods[n])
			}
		} else {
			for _, n := range f.forder {
				if u := f.funcs[n]; u.root {
					units = append(units, u)
				}
			}
		}
		for _, u := range units {
			meth := SKMethod{ID: po.ID + "." + u.name, Name: u.name, Params: u.params, Actions: []SKStep{}, Hops: []SKHop{},
				Anchor: spanOf(f.entry, f.data, u.start, u.end)}
			meth.Actions = append(meth.Actions, m.bodySteps(u)...)
			for _, a := range meth.Actions {
				if a.Kind == skCall {
					meth.Hops = append(meth.Hops, m.hop(u, a))
				}
			}
			meth.Effect, meth.EffectReason = skEffect(m.inline(meth.Actions, u, nil, 0), m.reachedSetup(u))
			po.Methods = append(po.Methods, meth)
		}
		for j := range po.Methods {
			if u := m.unitFor(po, po.Methods[j].Name); u != nil {
				u.method = &po.Methods[j]
			}
		}
		if tf, ok := joins[po.Path]; ok && tf.Screen != "" {
			po.Screen, po.ScreenBasis, po.Join = tf.Screen, tf.ScreenBasis, StatusResolved
		} else {
			m.urlJoin(po, units, screens)
		}
	}
	for _, p := range paths {
		f := m.files[p]
		framework := "playwright"
		switch {
		case f.ruby:
			framework = "capybara"
		case f.cypress:
			framework = "cypress"
		}
		for _, u := range f.tests {
			raw := m.bodySteps(u)
			steps := m.inline(raw, u, nil, 0)
			complete := true
			for _, s := range steps {
				if s.Kind == StepOpaque {
					complete = false
				}
			}
			t := SKTest{ID: "test:" + p + "#" + skDigest(u.title, strconv.Itoa(u.start)), Title: u.title, Path: p, Framework: framework,
				Complete: complete, Steps: steps, Setup: uniqueSorted(u.setup), Authority: skAuthority, Anchor: spanOf(f.entry, f.data, u.start, u.end)}
			if len(t.Setup) == 0 {
				t.Setup = nil
			}
			m.Tests = append(m.Tests, t)
		}
	}
	m.dedupe()
	return m
}

func (m *SKModel) unitFor(po *SKPageObject, name string) *skUnit {
	f := m.files[po.Path]
	if c := f.classes[po.Name]; c != nil && po.ID == "page-object:"+po.Path+"#"+c.name {
		return c.methods[name]
	}
	return f.funcs[name]
}

// skModuleForm reports the module page-object form of a file whose functions take the page or
// cy root, or "" when it has none.
func skModuleForm(f *skFile) string {
	if f.ruby {
		return ""
	}
	for _, n := range f.forder {
		if f.funcs[n].root {
			if f.cypress {
				return FormCypressModule
			}
			return FormPlaywrightModule
		}
	}
	return ""
}

// Closed reasons a method's effect stays unknown (SKH-V0-016). An opaque step names its own
// opaque reason (parse-unsupported, dynamic-value or unresolved-call).
const (
	EffectReasonClick     = "unclassified-click"
	EffectReasonPointer   = "focus-or-hover"
	EffectReasonArranges  = "arranges-run-only"
	EffectReasonNoActions = "no-ui-actions"
)

// skRequestSetup are the setup entries that issue a request: a state effect under ANL-V0-035.
var skRequestSetup = wordSet("request fetch page.request")

// skEffect applies the ANL-V0-035 classes, in their precedence, to a method's inlined steps and
// the setup its call graph performs (SKH-V0-016): state-effect when any step fills, selects,
// checks, uploads or presses, or any reached unit issues a request; otherwise unknown, with the
// first unclassifiable step's reason, when any step is a non-link click, a focus, a hover or
// opaque, or when the method performs no UI step; otherwise navigation-only. A dynamic-value
// step keeps its exact kind in Call, so a fill whose value is dynamic is still a state effect.
func skEffect(steps []SKStep, setup []string) (string, string) {
	reason := ""
	note := func(r string) {
		if reason == "" {
			reason = r
		}
	}
	for _, s := range steps {
		kind := s.Kind
		if kind == StepOpaque && s.Reason == OpaqueDynamicValue && skStepKinds[s.Call] {
			kind = s.Call
		}
		switch kind {
		case StepVisit, StepWaitFor, StepAssert:
		case StepFill, StepSelect, StepCheck, StepUpload, StepPress:
			return EffectState, ""
		case StepClick:
			if s.Locator == nil || s.Locator.Kind != "role" || s.Locator.Value != "link" {
				note(EffectReasonClick)
			}
		case StepFocus, StepHover:
			note(EffectReasonPointer)
		default:
			note(s.Reason)
		}
	}
	for _, n := range setup {
		if skRequestSetup[n] {
			return EffectState, ""
		}
	}
	switch {
	case reason != "":
		return EffectUnknown, reason
	case len(steps) == 0 && len(setup) > 0:
		return EffectUnknown, EffectReasonArranges
	case len(steps) == 0:
		return EffectUnknown, EffectReasonNoActions
	}
	return EffectNavigation, ""
}

// Closed reasons a page object stays unjoined (SKH-V0-015); they match the AMAP-V0 page-object
// join's own unknown reasons.
const (
	JoinReasonUnresolvedTarget = "page-object-unresolved-target"
	JoinReasonUnbound          = "page-object-unbound"
	JoinReasonAmbiguous        = "page-object-ambiguous"
	JoinDetailDynamic          = "dynamic-target"
)

// urlJoin joins a page object the import-graph join left unbound through the visit targets of its
// methods' inlined steps, as the AMAP-V0 page-object-url basis does for a page object's own gotos
// (SKH-V0-015): one screen joins with basis page-object-url; a target the map cannot place, no
// target, or two screens leave it unknown with a closed join reason.
func (m *SKModel) urlJoin(po *SKPageObject, units []*skUnit, screens *screenIndex) {
	targets, detail := map[string]bool{}, ""
	unresolve := func(why string) {
		if detail == "" {
			detail = why
		}
	}
	for _, u := range units {
		for _, s := range m.inline(m.bodySteps(u), u, nil, 0) {
			switch {
			case s.Kind == StepVisit && s.Target != "" && !s.Screened:
				url := s.Target
				for _, slot := range s.Slots {
					url = strings.ReplaceAll(url, "${"+slot+"}", substMark)
				}
				if id, why := screens.byURL(url); id != "" {
					targets[id] = true
				} else {
					unresolve(why)
				}
			case s.Kind == StepVisit, s.Kind == StepOpaque && s.Call == StepVisit:
				unresolve(JoinDetailDynamic)
			}
		}
	}
	switch {
	case detail != "":
		po.JoinReason, po.JoinDetail = JoinReasonUnresolvedTarget, detail
	case len(targets) == 1:
		for id := range targets {
			po.Screen, po.ScreenBasis, po.Join = id, "page-object-url", StatusResolved
		}
	case len(targets) == 0:
		po.JoinReason = JoinReasonUnbound
	default:
		po.JoinReason = JoinReasonAmbiguous
	}
}

// reachedSetup collects the setup entries of u and of every unit its resolved calls reach, one
// bounded hop at a time.
func (m *SKModel) reachedSetup(u *skUnit) []string {
	seen := map[*skUnit]bool{}
	out := []string{}
	var walk func(*skUnit, int)
	walk = func(v *skUnit, depth int) {
		if seen[v] || depth > maxSKHops {
			return
		}
		seen[v] = true
		steps := m.bodySteps(v)
		out = append(out, v.setup...)
		for _, s := range steps {
			if s.Kind != skCall {
				continue
			}
			if t, _, _ := m.resolveCall(v, s.Call); t != nil {
				walk(t, depth+1)
			}
		}
	}
	walk(u, 0)
	return uniqueSorted(out)
}

// hop resolves one call action of u (SKH-V0-012).
func (m *SKModel) hop(u *skUnit, a SKStep) SKHop {
	h := SKHop{Call: a.Call, Line: a.Anchor.Start, Status: StatusUnknown}
	target, kind, reason := m.resolveCall(u, a.Call)
	h.Kind = kind
	if target == nil {
		h.Reason = reason
		return h
	}
	h.Status, h.Resolved = StatusResolved, m.unitID(target)
	return h
}

func (m *SKModel) unitID(u *skUnit) string {
	if u.class != nil {
		return "page-object:" + u.file.path + "#" + u.class.name + "." + u.name
	}
	return "page-object:" + u.file.path + "." + u.name
}

// resolveCall resolves a call name written as this.method, Class.method, or a bare function.
func (m *SKModel) resolveCall(u *skUnit, call string) (*skUnit, string, string) {
	if name, ok := strings.CutPrefix(call, "this."); ok {
		if u.class == nil {
			return nil, "method", "not-defined"
		}
		return m.classMethod(u.file, u.class, name, 0)
	}
	cls, name, dotted := strings.Cut(call, ".")
	if !dotted {
		name = cls
		if fn := u.file.funcs[name]; fn != nil && fn != u {
			return fn, "helper", ""
		}
		if imp, ok := u.file.imports[name]; ok {
			g := m.importFile(imp.base)
			if g == nil {
				return nil, "helper", "import-not-read"
			}
			if fn := g.funcs[imp.name]; fn != nil {
				return fn, "helper", ""
			}
			return nil, "helper", "not-exported"
		}
		if u.class != nil && u.file.ruby {
			return m.classMethod(u.file, u.class, name, 0)
		}
		return nil, "helper", "not-defined"
	}
	c, f := m.findClass(u.file, cls)
	if c == nil {
		return nil, "method", "class-not-read"
	}
	return m.classMethod(f, c, name, 0)
}

// importFile finds the read file an import base names, trying the TypeScript resolution order.
func (m *SKModel) importFile(base string) *skFile {
	trimmed := strings.TrimSuffix(base, path.Ext(base))
	for _, c := range []string{base, trimmed + ".ts", trimmed + ".tsx", base + ".ts", base + ".tsx", base + ".js", base + ".jsx",
		base + ".mjs", base + "/index.ts", base + "/index.tsx", base + "/index.js"} {
		if f := m.files[c]; f != nil {
			return f
		}
	}
	return nil
}

// findClass finds a class visible from file f: declared in it, imported into it, or (Ruby) the one
// read file's constant of that name.
func (m *SKModel) findClass(f *skFile, name string) (*skClass, *skFile) {
	if c := f.classes[name]; c != nil {
		return c, f
	}
	if imp, ok := f.imports[name]; ok {
		if g := m.importFile(imp.base); g != nil && g.classes[imp.name] != nil {
			return g.classes[imp.name], g
		}
		return nil, nil
	}
	if f.ruby {
		var found *skClass
		var in *skFile
		for _, p := range sortedKeys(m.files) {
			g := m.files[p]
			if c := g.classes[name]; c != nil && g.ruby {
				if found != nil {
					return nil, nil // ambiguous constant
				}
				found, in = c, g
			}
		}
		return found, in
	}
	return nil, nil
}

// classMethod resolves name on class c, then its ancestors and included modules, one hop at a time.
func (m *SKModel) classMethod(f *skFile, c *skClass, name string, depth int) (*skUnit, string, string) {
	kind := "method"
	if depth > 0 {
		kind = "inherited"
	}
	if u := c.methods[name]; u != nil {
		return u, kind, ""
	}
	if depth >= maxSKHops {
		return nil, "inherited", "hop-limit"
	}
	for _, inc := range c.includes {
		if mc, _ := m.findClass(f, inc); mc != nil {
			if u := mc.methods[name]; u != nil {
				return u, "included", ""
			}
		}
	}
	if c.extends == "" {
		return nil, kind, "not-defined"
	}
	pc, pf := m.findClass(f, c.extends)
	if pc == nil {
		return nil, "inherited", "ancestor-not-read"
	}
	return m.classMethod(pf, pc, name, depth+1)
}

// bodySteps reads one unit's own ordered actions, before inlining, once.
func (m *SKModel) bodySteps(u *skUnit) []SKStep {
	if !u.read {
		u.read = true
		if u.file.ruby {
			u.steps = m.rubySteps(u)
		} else {
			u.steps = m.tsSteps(u)
		}
		if len(u.steps) > maxSKBodySteps {
			last := u.steps[maxSKBodySteps-1]
			u.steps = append(u.steps[:maxSKBodySteps-1:maxSKBodySteps-1], SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Anchor: last.Anchor})
		}
	}
	return u.steps
}

// inline replaces each call with the callee's steps, binding arguments to parameter slots
// (SKH-V0-021). An unresolved call becomes an opaque unresolved-call step, never dropped.
func (m *SKModel) inline(steps []SKStep, u *skUnit, bind map[string]SKArg, depth int) []SKStep {
	out := []SKStep{}
	for _, s := range steps {
		if len(out) >= maxSKInline {
			out = append(out, SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Anchor: s.Anchor})
			break
		}
		if s.Kind != skCall {
			out = append(out, bindStep(s, bind))
			continue
		}
		target, _, _ := m.resolveCall(u, s.Call)
		if target == nil || depth >= maxSKHops {
			out = append(out, SKStep{Kind: StepOpaque, Reason: OpaqueUnresolvedCall, Call: s.Call, Anchor: s.Anchor})
			continue
		}
		inner := map[string]SKArg{}
		for i, p := range target.params {
			if target.pageArgs[p] {
				continue
			}
			if i < len(s.Args) {
				inner[p] = resolveArg(s.Args[i], bind)
			} else {
				inner[p] = SKArg{Dynamic: true}
			}
		}
		via := m.unitID(target)
		for _, t := range m.inline(m.bodySteps(target), target, inner, depth+1) {
			t.Via = append([]string{via}, t.Via...)
			out = append(out, t)
		}
	}
	return out
}

// bindStep substitutes bound arguments into a step's slots. A slot bound to a dynamic argument
// makes the step opaque with dynamic-value; a screened literal is never kept (SKH-V0-006).
func bindStep(s SKStep, bind map[string]SKArg) SKStep {
	if bind == nil {
		return s
	}
	if s.LocatorSlot != "" && s.Locator != nil {
		if a, ok := bind[s.LocatorSlot]; ok {
			switch {
			case a.Dynamic || len(a.Template) > 0:
				return SKStep{Kind: StepOpaque, Reason: OpaqueDynamicValue, Call: s.Kind, Via: s.Via, Anchor: s.Anchor}
			case a.Slot != "":
				s.LocatorSlot = a.Slot
			default:
				loc := newSelector(s.Locator.Kind, a.Literal, s.Locator.Name, skKindStrength(s.Locator.Kind, a.Literal), s.Locator.Line)
				s.Locator, s.LocatorSlot = &loc, ""
			}
		}
	}
	if len(s.Slots) == 0 {
		return s
	}
	field := &s.Value
	switch s.Kind {
	case StepVisit:
		field = &s.Target
	case StepAssert:
		field = &s.Expected
	}
	slots, bound := []string{}, false
	for _, slot := range s.Slots {
		a, ok := bind[slot]
		mark := "${" + slot + "}"
		inTemplate := strings.Contains(*field, mark)
		switch {
		case !ok:
			slots = append(slots, slot)
		case a.Dynamic:
			return SKStep{Kind: StepOpaque, Reason: OpaqueDynamicValue, Call: s.Kind, Via: s.Via, Anchor: s.Anchor}
		case a.Slot != "":
			if inTemplate {
				*field = strings.ReplaceAll(*field, mark, "${"+a.Slot+"}")
			}
			slots = append(slots, a.Slot)
		default:
			if inTemplate {
				*field = strings.ReplaceAll(*field, mark, a.Literal)
			} else {
				*field = a.Literal
			}
			slots = append(slots, a.Template...)
			bound = true
		}
	}
	if bound {
		v, screened := screenValue(*field, s.Locator)
		*field, s.Screened = v, s.Screened || screened
	}
	s.Slots = nil
	if len(slots) > 0 {
		s.Slots = slots
	}
	return s
}

// skKindStrength is the strength a locator kind carries with an exact value (AMAP-V0-007).
func skKindStrength(kind, value string) string {
	if value == "" {
		return strengthUnknown
	}
	for _, k := range selectorKinds {
		if k.kind == kind {
			return k.strength
		}
	}
	return strengthWeak
}

// stepKey is the normalised identity of one step for deduplication: everything but its anchor.
func stepKey(s SKStep) string {
	loc := ""
	if s.Locator != nil {
		loc = s.Locator.ID
	}
	return strings.Join([]string{s.Kind, loc, s.LocatorSlot, s.Target, s.Matcher, s.Expected, s.Value, strings.Join(s.Slots, ","), s.Reason, s.Call}, "\x01")
}

// dedupe groups identical step sequences across tests into reusable records with usage counts
// citing every source test (SKH-V0-022): one record per distinct whole test body, and one per
// page-object method sequence used by at least one test.
func (m *SKModel) dedupe() {
	type rec struct {
		r     SKRecord
		tests map[string]bool
	}
	recs := map[string]*rec{}
	add := func(source string, steps []SKStep, complete bool, test string) string {
		id := skRecordID(source, steps)
		if recs[id] == nil {
			recs[id] = &rec{r: SKRecord{ID: id, Source: source, Steps: len(steps), Complete: complete}, tests: map[string]bool{}}
		}
		recs[id].tests[test] = true
		return id
	}
	for i := range m.Tests {
		t := &m.Tests[i]
		t.Record = add("test-body", t.Steps, t.Complete, t.ID)
		seen := map[string]bool{}
		for _, s := range t.Steps {
			if len(s.Via) == 0 || seen[s.Via[0]] {
				continue
			}
			seen[s.Via[0]] = true
			group := []SKStep{}
			for _, g := range t.Steps {
				if len(g.Via) > 0 && g.Via[0] == s.Via[0] {
					group = append(group, g)
				}
			}
			complete := true
			for _, g := range group {
				if g.Kind == StepOpaque {
					complete = false
				}
			}
			add(s.Via[0], group, complete, t.ID)
		}
	}
	for _, id := range sortedKeys(recs) {
		r := recs[id]
		r.r.Tests = sortedKeys(r.tests)
		r.r.Usage = len(r.r.Tests)
		m.Records = append(m.Records, r.r)
	}
}

// ValidateSKStep refuses a step whose kind or opaque reason is outside the closed IR
// (SKH-V0-023); it never coerces one.
func ValidateSKStep(s SKStep) bool {
	if !skStepKinds[s.Kind] {
		return false
	}
	if s.Kind == StepOpaque {
		return skOpaqueReasons[s.Reason]
	}
	return s.Reason == ""
}

// DecodeSKStep reads one step record strictly: an unknown field, a trailing value, an unknown
// kind or an unknown opaque reason is refused by name, never coerced or partly read (SKH-V0-023).
func DecodeSKStep(b []byte) (SKStep, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var s SKStep
	if err := dec.Decode(&s); err != nil {
		return SKStep{}, fmt.Errorf("site-knowledge step refused: %w", err)
	}
	if !jsonend.Done(dec) {
		return SKStep{}, fmt.Errorf("site-knowledge step refused: trailing data")
	}
	if !ValidateSKStep(s) {
		return SKStep{}, fmt.Errorf("site-knowledge step refused: kind %q reason %q is outside the closed IR", s.Kind, s.Reason)
	}
	return s, nil
}
