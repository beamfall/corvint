package doccorpus

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// LegacyInventorySchema names the generated static legacy inventory (DCP-V1-060).
const LegacyInventorySchema = "corvint-legacy-inventory/1"

// ErrLegacyMappingRefused marks an inventory whose crosswalk refused a reviewed same/stronger
// mapping (DCP-V1-066). The encoded inventory is still returned with it.
var ErrLegacyMappingRefused = errors.New("legacy mapping refused")

// LegacyInventoryInput is one static legacy discovery: the runner vocabulary, the full commit
// whose Git blobs are read, the repository path (a test file or a directory) and an optional
// behavior registry for the generated crosswalk.
type LegacyInventoryInput struct {
	Root, Runner, Revision, Path string
	Registry                     []byte
}

// LegacyInventory is generated, never reviewed: its legacy cases use the existing
// BehaviorLegacyCase shape so a reviewer can adopt them, and everything the static reader cannot
// classify is listed in Unknown instead of being dropped.
type LegacyInventory struct {
	Schema        string               `json:"schema"`
	Status        string               `json:"status"`
	Runner        string               `json:"runner"`
	Repository    string               `json:"repository"`
	Revision      string               `json:"revision"`
	Path          string               `json:"path"`
	Legacy        []BehaviorLegacyCase `json:"legacy"`
	Preconditions []LegacyPrecondition `json:"preconditions"`
	Unknown       []LegacyUnknown      `json:"unknown"`
	Crosswalk     *LegacyCrosswalk     `json:"crosswalk,omitempty"`
	Limitations   []string             `json:"limitations"`
}

// LegacyPrecondition is a content-bound precondition anchor. Fixture is the identity a legacy case
// lists in its fixtures; it embeds the digest of the layout-free call text, so changed seed
// arguments change the identity.
type LegacyPrecondition struct {
	Fixture  string `json:"fixture"`
	Kind     string `json:"kind"`
	Callee   string `json:"callee"`
	Text     string `json:"text"`
	SHA256   string `json:"sha256"`
	Evidence Anchor `json:"evidence"`
}

// LegacyUnknown is something the static reader saw but could not turn into evidence.
type LegacyUnknown struct {
	Case     string  `json:"case,omitempty"`
	Path     string  `json:"path"`
	Reason   string  `json:"reason"`
	Detail   string  `json:"detail"`
	Evidence *Anchor `json:"evidence,omitempty"`
}

var legacyLimitations = []string{
	"static extraction only: no legacy test is executed, so runtime parity still needs a qualified corvint-legacy-behavior-run/1 baseline",
	"helper and custom-command bodies, support files and fixture files are not followed; a precondition binds its call text by digest",
	"roles are not inferred and are always empty",
	"global support hooks and root-level hooks in other files are not seen",
	"the inventory and crosswalk are generated; neither creates a reviewed legacy_criteria relation",
}

type legacyRunner struct {
	suites, cases, hooks map[string]bool
	members              bool
}

var (
	legacyRunners = map[string]legacyRunner{
		"jasmine": {suites: words("describe fdescribe xdescribe"), cases: words("it fit xit"), hooks: words("beforeEach beforeAll")},
		"mocha":   {suites: words("describe context suite xdescribe xcontext"), cases: words("it specify test xit xspecify"), hooks: words("before beforeEach suiteSetup setup"), members: true},
	}
	legacyDisabledCallee = words("xdescribe xcontext xit xspecify")
	legacyFocusedCallee  = words("fdescribe fit")
	legacyKeyword        = words("if for while switch catch return function typeof new await async do else try finally throw yield void delete in of instanceof const let var class import export default case break continue super")
	legacyPreconditionOf = map[string]string{
		"cy.visit": "visit", "browser.get": "visit", "browser.url": "visit", "page.goto": "visit",
		"cy.intercept": "intercept", "cy.route": "intercept",
		"cy.task": "setup", "cy.exec": "setup", "cy.fixture": "setup",
	}
	legacyAssertionWord = words("expect expectAsync assert should")
	// legacyTemplateAssertionWord adds the Cypress chain words that are assertions only as members.
	legacyTemplateAssertionWord = words("expect expectAsync assert should contains and")
	legacyEachHook              = words("beforeEach setup")
	legacyConditional           = words("if for while switch try do catch else")
)

const legacyNoValue = "(no expected value)"

type legacyFile struct {
	path, blob string
	data       []byte
	tokens     []legacyToken
	match      []int
}

// legacyScope is what a suite passes to its children: its title path, the precondition
// identities of its run-once (all) and per-case (each) hooks in execution order, and the hook
// uncertainty every case under it inherits.
type legacyScope struct {
	titles                                   []string
	all, each, uncertain                     []string
	disabled, focused, dynamic, dynamicTitle bool
}

type legacyCall struct {
	name, member              string
	start, open, close, depth int
}

type legacyRange struct{ lo, hi int }

type legacyProducer struct {
	runner      legacyRunner
	name        string
	repository  string
	revision    string
	inv         *LegacyInventory
	pre         map[string]LegacyPrecondition
	visits      map[string][]string
	caseUnknown map[string]bool
	ids         map[string]bool
	duplicates  map[string]bool // case IDs whose full title repeats later in the file
}

// BuildLegacyInventory reads Jasmine or Mocha/Cypress test files from Git at a full commit and
// emits the generated corvint-legacy-inventory/1 record (DCP-V1-060..068). It never runs a test,
// never reads the working tree and writes nothing.
func BuildLegacyInventory(ctx context.Context, input LegacyInventoryInput) ([]byte, error) {
	runner, ok := legacyRunners[input.Runner]
	if !ok {
		return nil, fail("legacy runner must be jasmine or mocha")
	}
	if !wire.IsGitOid(input.Revision) {
		return nil, fail("legacy discovery requires a full commit revision")
	}
	if input.Path != "." && !affected.ValidRelativePath(input.Path) {
		return nil, fail("legacy path is not a canonical repository-relative path")
	}
	repository, err := contextindex.CorpusRepositoryID(ctx, input.Root, input.Revision)
	if err != nil {
		return nil, err
	}
	auth, err := gitauth.Open(input.Root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	release := auth.BeginObjectSession()
	defer release()
	paths, err := legacyPaths(ctx, auth, input.Revision, input.Path)
	if err != nil {
		return nil, err
	}
	inventory := &LegacyInventory{Schema: LegacyInventorySchema, Status: "generated", Runner: input.Runner, Repository: repository, Revision: input.Revision, Path: input.Path, Legacy: []BehaviorLegacyCase{}, Preconditions: []LegacyPrecondition{}, Unknown: []LegacyUnknown{}, Limitations: legacyLimitations}
	p := &legacyProducer{runner: runner, name: input.Runner, repository: repository, revision: input.Revision, inv: inventory, pre: map[string]LegacyPrecondition{}, visits: map[string][]string{}, caseUnknown: map[string]bool{}, ids: map[string]bool{}, duplicates: map[string]bool{}}
	for _, current := range paths {
		entry, ok, err := auth.LookupTreeEntry(ctx, input.Revision, current)
		if err != nil {
			return nil, err
		}
		if !ok || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return nil, fail("legacy test file is not a regular Git blob at the revision: " + current)
		}
		data, over, err := auth.BlobBytesWithin(ctx, entry.OID, MaxBytes)
		if err != nil {
			return nil, err
		}
		file := &legacyFile{path: current, blob: entry.OID, data: data}
		if over {
			p.unknown(file, "", "file-over-bound", "the test file exceeds the input bound; its cases are not inventoried", 0, 0)
			continue
		}
		p.file(file)
		if len(inventory.Legacy) > MaxRecords || len(inventory.Unknown) > MaxRecords {
			return nil, fail("legacy inventory exceeds the record bound")
		}
	}
	for _, precondition := range p.pre {
		inventory.Preconditions = append(inventory.Preconditions, precondition)
	}
	sort.Slice(inventory.Preconditions, func(i, j int) bool {
		return inventory.Preconditions[i].Fixture < inventory.Preconditions[j].Fixture
	})
	refused := false
	if input.Registry != nil {
		if inventory.Crosswalk, refused, err = p.crosswalk(input.Registry); err != nil {
			return nil, err
		}
	}
	data, err := Encode(inventory)
	if err == nil && refused {
		err = ErrLegacyMappingRefused
	}
	return data, err
}

// legacyPaths resolves --path at the revision: a regular blob is read as is, a directory yields
// its test-named files in path order, skipping node_modules.
func legacyPaths(ctx context.Context, auth *gitauth.Repository, revision, root string) ([]string, error) {
	if root != "." {
		entry, ok, err := auth.LookupTreeEntry(ctx, revision, root)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fail("legacy path does not exist at the revision: " + root)
		}
		if entry.Type == "blob" {
			return []string{root}, nil
		}
	}
	tree, err := auth.RevisionFS(ctx, revision, MaxBytes)
	if err != nil {
		return nil, err
	}
	paths, entries := []string{}, 0
	err = fs.WalkDir(tree, root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entries++; entries > affected.MaxWalkEntries {
			return affected.ErrWalkLimit
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() && legacyTestPath(current) {
			paths = append(paths, current)
		}
		return nil
	})
	if err != nil {
		return nil, fail("legacy discovery cannot enumerate the revision: " + err.Error())
	}
	if len(paths) == 0 || len(paths) > MaxRecords {
		return nil, fail("legacy path has no test files or exceeds the record bound: " + root)
	}
	return paths, nil
}

func legacyTestPath(name string) bool {
	base := path.Base(name)
	ext := path.Ext(base)
	if !words(".js .ts .jsx .tsx .mjs .cjs")[ext] {
		return false
	}
	stem := strings.TrimSuffix(base, ext)
	for _, suffix := range []string{".spec", ".test", ".cy", "_spec", "-spec", "_test"} {
		if strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	return false
}

func (p *legacyProducer) anchor(f *legacyFile, start, end int, reason string) (Anchor, error) {
	excerpt, err := span(f.data, start, end)
	if err != nil {
		return Anchor{}, err
	}
	return Anchor{Repository: p.repository, Revision: p.revision, Path: f.path, Blob: f.blob, SHA256: Digest(f.data), Start: start, End: end, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "source", Reason: reason}, nil
}

// unknown records what could not become evidence. A zero span records it without an anchor.
func (p *legacyProducer) unknown(f *legacyFile, caseID, reason, detail string, start, end int) {
	u := LegacyUnknown{Case: caseID, Path: f.path, Reason: reason, Detail: legacyBounded(detail)}
	if start > 0 {
		if a, err := p.anchor(f, start, end, "static legacy "+p.name+" "+reason); err == nil {
			u.Evidence = &a
		}
	}
	p.inv.Unknown = append(p.inv.Unknown, u)
	if caseID != "" {
		p.caseUnknown[caseID] = true
	}
}

func legacyBounded(text string) string {
	if len(text) <= 240 {
		return text
	}
	cut := 240
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "..."
}

func (p *legacyProducer) file(f *legacyFile) {
	tokens, match, ok := lexLegacy(string(f.data))
	if !ok {
		lines := bytes.Count(f.data, []byte{'\n'})
		if len(f.data) > 0 && f.data[len(f.data)-1] != '\n' {
			lines++
		}
		p.unknown(f, "", "lex-failed", "the static tokenizer could not classify this file; its cases are not inventoried", 1, lines)
		return
	}
	f.tokens, f.match = tokens, match
	before := len(p.inv.Legacy)
	p.scope(f, 0, len(tokens), legacyScope{}, true)
	// The first of repeated titles keeps the unnumbered ID but is as ambiguous as the rest.
	for _, legacy := range p.inv.Legacy[before:] {
		if p.duplicates[legacy.ID] {
			p.unknown(f, legacy.ID, "duplicate-title", "a later case in this file has the same full title; this first one keeps the unnumbered ID", legacy.Evidence.Start, legacy.Evidence.End)
		}
	}
	if len(p.inv.Legacy) == before {
		p.unknown(f, "", "no-static-cases", "no "+p.name+" case call was found in this test file", 0, 0)
	}
}

func legacyChained(t []legacyToken, i int) bool {
	return i > 0 && t[i-1].kind == 'p' && (t[i-1].text == "." || t[i-1].text == "?.")
}

func legacyPunct(t []legacyToken, i int, text string) bool {
	return i >= 0 && i < len(t) && t[i].kind == 'p' && t[i].text == text
}

// apiCall recognizes a runner suite, case or hook call rooted at token i.
func (p *legacyProducer) apiCall(f *legacyFile, i int) (legacyCall, bool) {
	t := f.tokens
	if t[i].kind != 'i' || legacyChained(t, i) {
		return legacyCall{}, false
	}
	name := t[i].text
	if !p.runner.suites[name] && !p.runner.cases[name] && !p.runner.hooks[name] {
		return legacyCall{}, false
	}
	j, member := i+1, ""
	if p.runner.members && !p.runner.hooks[name] && legacyPunct(t, j, ".") && j+1 < len(t) && t[j+1].kind == 'i' && (t[j+1].text == "skip" || t[j+1].text == "only") {
		member, j = t[j+1].text, j+2
	}
	if !legacyPunct(t, j, "(") {
		return legacyCall{}, false
	}
	return legacyCall{name: name, member: member, start: i, open: j, close: f.match[j]}, true
}

// legacyArgs splits the top-level arguments of the call whose '(' is at open.
func legacyArgs(f *legacyFile, open int) []legacyRange {
	t, close := f.tokens, f.match[open]
	args := []legacyRange{}
	lo := open + 1
	for k := open + 1; k < close; k++ {
		if t[k].kind == 'p' && (t[k].text == "(" || t[k].text == "[" || t[k].text == "{") {
			k = f.match[k]
			continue
		}
		if legacyPunct(t, k, ",") {
			args = append(args, legacyRange{lo, k})
			lo = k + 1
		}
	}
	if lo < close {
		args = append(args, legacyRange{lo, close})
	}
	return args
}

// legacyCallback returns the body of a function or arrow argument that spans exactly r.
func legacyCallback(f *legacyFile, r legacyRange) (legacyRange, bool) {
	t, k := f.tokens, r.lo
	if k < r.hi && t[k].kind == 'i' && t[k].text == "async" {
		k++
	}
	if k < r.hi && t[k].kind == 'i' && t[k].text == "function" {
		if k++; k < r.hi && t[k].kind == 'i' {
			k++
		}
		if !legacyPunct(t, k, "(") || f.match[k] >= r.hi {
			return legacyRange{}, false
		}
		k = f.match[k] + 1
		if !legacyPunct(t, k, "{") || f.match[k] != r.hi-1 {
			return legacyRange{}, false
		}
		return legacyRange{k + 1, r.hi - 1}, true
	}
	switch {
	case k < r.hi && t[k].kind == 'i':
		k++
	case legacyPunct(t, k, "(") && f.match[k] < r.hi:
		k = f.match[k] + 1
	default:
		return legacyRange{}, false
	}
	if !legacyPunct(t, k, "=>") || k+1 >= r.hi {
		return legacyRange{}, false
	}
	k++
	if legacyPunct(t, k, "{") && f.match[k] == r.hi-1 {
		return legacyRange{k + 1, r.hi - 1}, true
	}
	return legacyRange{k, r.hi}, true
}

// legacyTitle returns the static title of the first argument, or its canonical text when it is
// not one string literal.
func legacyTitle(f *legacyFile, args []legacyRange) (string, bool) {
	if len(args) == 0 {
		return "(missing title)", true
	}
	r := args[0]
	if r.hi-r.lo == 1 && f.tokens[r.lo].kind == 's' && textOK(f.tokens[r.lo].text) {
		return f.tokens[r.lo].text, false
	}
	if text := legacyCanonical(f.tokens, r.lo, r.hi); textOK(text) {
		return text, true
	}
	return "(unreadable title)", true
}

// scope inventories the calls directly in tokens[lo:hi]. Hooks apply to every case of the scope
// wherever they appear, so they are collected first; a call nested in non-runner brackets (a loop,
// a conditional or a helper callback) is still inventoried but marked as dynamic.
func (p *legacyProducer) scope(f *legacyFile, lo, hi int, s legacyScope, root bool) {
	calls := []legacyCall{}
	depth := 0
	for i := lo; i < hi; i++ {
		if c, ok := p.apiCall(f, i); ok {
			c.depth = depth
			if !legacyStatementStart(f, i, lo) {
				// An unbraced if/else/loop body or an operand (flag && it(...)) may not run.
				c.depth++
			}
			calls = append(calls, c)
			i = c.close
			continue
		}
		if f.tokens[i].kind == 'p' {
			switch f.tokens[i].text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			}
		}
	}
	inner := s
	inner.all, inner.each, inner.uncertain = slices.Clone(s.all), slices.Clone(s.each), slices.Clone(s.uncertain)
	for i := lo; i < hi; i++ {
		if f.tokens[i].kind != 't' {
			continue
		}
		runner, hook := false, false
		for _, callee := range legacySubCalls(f.tokens[i].sub) {
			name, _, _ := strings.Cut(callee, ".")
			runner = runner || p.runner.suites[name] || p.runner.cases[name] || p.runner.hooks[name]
			hook = hook || p.runner.hooks[name]
		}
		if runner {
			p.unknown(f, "", "runner-call-in-template", "a runner call inside a template substitution is not inventoried", f.tokens[i].line, f.tokens[i].line)
		}
		if hook {
			// A hook registered from a substitution still runs before the cases in this scope.
			inner.uncertain = append(inner.uncertain, "a hook call inside a template substitution is not inventoried")
		}
	}
	for _, c := range calls {
		if !p.runner.hooks[c.name] {
			continue
		}
		line, end := f.tokens[c.start].line, f.tokens[c.close].line
		// A root or dynamic hook is an unknown, never a precondition, and every case under it
		// inherits the doubt.
		if root {
			p.unknown(f, "", "root-level-hook", c.name+" outside any suite applies to every suite the runner loads", line, end)
			inner.uncertain = append(inner.uncertain, c.name+" outside any suite applies to every suite the runner loads")
			continue
		}
		if c.depth > 0 || s.dynamic {
			p.unknown(f, "", "hook-in-dynamic-scope", c.name+" is nested in code the static reader does not evaluate", line, end)
			inner.uncertain = append(inner.uncertain, c.name+" is nested in code the static reader does not evaluate")
			continue
		}
		args := legacyArgs(f, c.open)
		if len(args) == 0 {
			inner.uncertain = append(inner.uncertain, c.name+" has no callback")
			continue
		}
		body, ok := legacyCallback(f, args[len(args)-1])
		if !ok {
			inner.uncertain = append(inner.uncertain, c.name+" callback is not an inline function: "+legacyBounded(legacyCanonical(f.tokens, args[len(args)-1].lo, args[len(args)-1].hi)))
			continue
		}
		fixtures, doubts := p.hookPreconditions(f, body)
		inner.uncertain = append(inner.uncertain, doubts...)
		if legacyEachHook[c.name] {
			inner.each = append(inner.each, fixtures...)
		} else {
			inner.all = append(inner.all, fixtures...)
		}
	}
	for _, c := range calls {
		switch {
		case p.runner.suites[c.name]:
			p.suite(f, c, inner)
		case p.runner.cases[c.name]:
			p.testCase(f, c, inner)
		}
	}
}

func (p *legacyProducer) suite(f *legacyFile, c legacyCall, s legacyScope) {
	args := legacyArgs(f, c.open)
	title, dynamic := legacyTitle(f, args)
	child := legacyScope{
		titles:       append(slices.Clone(s.titles), title),
		all:          s.all,
		each:         s.each,
		uncertain:    s.uncertain,
		disabled:     s.disabled || legacyDisabledCallee[c.name] || c.member == "skip",
		focused:      s.focused || legacyFocusedCallee[c.name] || c.member == "only",
		dynamic:      s.dynamic || c.depth > 0,
		dynamicTitle: s.dynamicTitle || dynamic,
	}
	if len(args) < 2 {
		return
	}
	body, ok := legacyCallback(f, args[len(args)-1])
	if !ok {
		// Never drop cases: scan the argument as a dynamic scope.
		child.dynamic = true
		body = args[len(args)-1]
	}
	p.scope(f, body.lo, body.hi, child, false)
}

func (p *legacyProducer) testCase(f *legacyFile, c legacyCall, s legacyScope) {
	t := f.tokens
	args := legacyArgs(f, c.open)
	title, dynamicTitle := legacyTitle(f, args)
	full := strings.Join(append(slices.Clone(s.titles), title), " > ")
	suite := strings.Join(s.titles, " > ")
	if suite == "" {
		suite = f.path
	}
	base := "legacy:" + f.path + "::" + full
	id, name, duplicate := base, title, 1
	for p.ids[id] {
		p.duplicates[base] = true
		duplicate++
		id, name = base+" #"+strconv.Itoa(duplicate), title+" #"+strconv.Itoa(duplicate)
	}
	p.ids[id] = true
	line, end := t[c.start].line, t[c.close].line
	evidence, err := p.anchor(f, line, end, "static legacy "+p.name+" case")
	if err != nil {
		p.unknown(f, "", "case-span-unanchorable", full, 0, 0)
		return
	}
	legacy := BehaviorLegacyCase{ID: id, Suite: suite, Case: name, Evidence: evidence, State: "executable", Fixtures: []string{}, Roles: []string{}, Criteria: []BehaviorLegacyCriterion{}}
	if s.disabled || legacyDisabledCallee[c.name] || c.member == "skip" {
		legacy.State = "disabled"
	}
	note := func(reason, detail string) { p.unknown(f, id, reason, detail, line, end) }
	if duplicate > 1 {
		note("duplicate-title", "another case in this file has the same full title; this one is numbered #"+strconv.Itoa(duplicate))
	}
	if s.focused || legacyFocusedCallee[c.name] || c.member == "only" {
		note("focused", "a focused case or suite changes which cases the runner executes")
	}
	if s.dynamic || c.depth > 0 {
		note("case-in-dynamic-scope", "the case is declared inside code the static reader does not evaluate")
	}
	if dynamicTitle || s.dynamicTitle {
		note("dynamic-title", "the title is computed at runtime: "+full)
	}
	for _, doubt := range slices.Compact(slices.Sorted(slices.Values(s.uncertain))) {
		note("hook-not-static", doubt)
	}
	fixtures := append(slices.Clone(s.all), s.each...)
	var body legacyRange
	hasBody := false
	if len(args) < 2 {
		legacy.State = "disabled"
	} else if body, hasBody = legacyCallback(f, args[1]); !hasBody {
		note("case-callback-not-static", "the case body is not an inline function: "+legacyCanonical(t, args[1].lo, args[1].hi))
	}
	if hasBody {
		fixtures = append(fixtures, p.casePreconditions(f, id, body, false)...)
		for i := body.lo; i < body.hi; i++ {
			if p.name == "jasmine" && t[i].kind == 'i' && t[i].text == "pending" && !legacyChained(t, i) && legacyPunct(t, i+1, "(") {
				legacy.State = "disabled"
			}
			if t[i].kind == 'i' && t[i].text == "this" && legacyPunct(t, i+1, ".") && i+2 < len(t) && t[i+2].text == "skip" && legacyPunct(t, i+3, "(") {
				note("runtime-skip", "the case may skip itself at runtime")
			}
		}
		legacy.Criteria = p.assertions(f, id, body)
	}
	// Execution order is part of the precondition identity; a repeated call keeps its first place.
	for _, fixture := range fixtures {
		if !slices.Contains(legacy.Fixtures, fixture) {
			legacy.Fixtures = append(legacy.Fixtures, fixture)
		}
	}
	if len(legacy.Criteria) == 0 {
		note("no-static-assertion", "no assertion tuple was extracted from the case")
	}
	p.inv.Legacy = append(p.inv.Legacy, legacy)
	for _, fixture := range legacy.Fixtures {
		if visit := p.pre[fixture]; visit.Kind == "visit" {
			if url, ok := legacyVisitURL(visit.Text); ok {
				p.visits[id] = append(p.visits[id], url)
			}
		}
	}
}

// legacyChain extends the call chain rooted at token i and returns its end, its callee (the
// member path up to the first call) and the first call's '(' index (-1 when nothing is called).
func legacyChain(f *legacyFile, i, hi int) (int, string, int) {
	t := f.tokens
	k, callee, open := i+1, t[i].text, -1
	for k < hi {
		switch {
		case (legacyPunct(t, k, ".") || legacyPunct(t, k, "?.")) && k+1 < hi && t[k+1].kind == 'i':
			if open < 0 {
				callee += "." + t[k+1].text
			}
			k += 2
		case legacyPunct(t, k, "("):
			if open < 0 {
				open = k
			}
			k = f.match[k] + 1
		case legacyPunct(t, k, "["):
			k = f.match[k] + 1
		default:
			return k, callee, open
		}
	}
	return k, callee, open
}

// record binds one precondition call chain tokens[i:end] by the digest of its canonical text.
func (p *legacyProducer) record(f *legacyFile, i, end int, kind, callee string) (string, bool) {
	t := f.tokens
	text := legacyCanonical(t, i, end)
	digest := Digest([]byte(text))
	fixture := kind + ":" + callee + "@sha256:" + digest
	if _, seen := p.pre[fixture]; !seen {
		evidence, err := p.anchor(f, t[i].line, t[end-1].line, "static legacy "+p.name+" "+kind+" precondition")
		if err != nil {
			return "", false
		}
		p.pre[fixture] = LegacyPrecondition{Fixture: fixture, Kind: kind, Callee: callee, Text: text, SHA256: digest, Evidence: evidence}
	}
	return fixture, true
}

// legacyStatementStart reports whether token i begins a statement of the body starting at lo, so
// a call there runs unconditionally whenever the body runs. After a line break it applies
// JavaScript's automatic semicolon insertion, except where the previous token still governs the
// next statement: an if/for/while/with header, else, do, another keyword, or an operator.
func legacyStatementStart(f *legacyFile, i, lo int) bool {
	t := f.tokens
	if i > lo && t[i-1].kind == 'i' && (t[i-1].text == "await" || t[i-1].text == "void") {
		i--
	}
	if i == lo || legacyPunct(t, i-1, ";") || legacyPunct(t, i-1, "}") || legacyPunct(t, i-1, "{") {
		return true
	}
	previous := t[i-1]
	if previous.line == t[i].line {
		return false
	}
	switch {
	case previous.kind == 'i':
		return !legacyKeyword[previous.text]
	case previous.kind != 'p':
		return true
	case previous.text == ")":
		open := f.match[i-1]
		return open < 1 || t[open-1].kind != 'i' || !legacyRegexAfterParen[t[open-1].text]
	}
	return previous.text == "]" || previous.text == "++" || previous.text == "--"
}

// hookPreconditions records each top-level call statement of a hook body as a precondition.
// Anything the reader cannot bind exactly (a conditional, a call nested outside a recorded
// statement, code inside a template substitution, an unrecordable span) is returned as doubt,
// which every case under the hook inherits as an unknown.
func (p *legacyProducer) hookPreconditions(f *legacyFile, r legacyRange) ([]string, []string) {
	t := f.tokens
	fixtures, doubts := []string{}, []string{}
	doubt := func(detail string) {
		if !slices.Contains(doubts, detail) {
			doubts = append(doubts, detail)
		}
	}
	depth := 0
	for i := r.lo; i < r.hi; i++ {
		switch t[i].kind {
		case 'p':
			switch t[i].text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			case "&&", "||", "??", "?", "&&=", "||=", "??=":
				if depth == 0 {
					doubt("a hook statement is conditional: " + legacyBounded(legacyCanonical(t, max(r.lo, i-6), min(r.hi, i+6))))
				}
			}
			continue
		case 't':
			if len(legacySubCalls(t[i].sub)) > 0 {
				doubt("a hook calls code inside a template substitution")
			}
			continue
		case 'i':
		default:
			continue
		}
		if legacyConditional[t[i].text] {
			doubt("a hook uses " + t[i].text + "; its preconditions may not run unconditionally")
			continue
		}
		if legacyChained(t, i) || legacyKeyword[t[i].text] {
			continue
		}
		end, callee, open := legacyChain(f, i, r.hi)
		if open < 0 {
			continue
		}
		if depth != 0 {
			doubt("a hook call is nested in code the static reader does not evaluate: " + callee)
			i = end - 1
			continue
		}
		if !legacyStatementStart(f, i, r.lo) {
			doubt("a hook call is not a statement of its own: " + callee)
		}
		if legacyHiddenCode(t, i, end) {
			doubt("a hook call passes code the static reader does not evaluate: " + callee)
		}
		kind := legacyPreconditionOf[callee]
		if kind == "" {
			kind = "setup"
		}
		if fixture, ok := p.record(f, i, end, kind, callee); ok {
			fixtures = append(fixtures, fixture)
		} else {
			doubt("a hook precondition span cannot be anchored: " + callee)
		}
		i = end - 1
	}
	return fixtures, doubts
}

// casePreconditions records every visit, intercept and seed command of a case body at any depth.
// One that is not an unconditional statement of the body, or that sits in a template
// substitution (nested), is still bound by digest but also makes the case uncertain.
func (p *legacyProducer) casePreconditions(f *legacyFile, caseID string, r legacyRange, nested bool) []string {
	t := f.tokens
	fixtures := []string{}
	depth, conditional := 0, false
	for i := r.lo; i < r.hi; i++ {
		switch t[i].kind {
		case 'p':
			switch t[i].text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			case "&&", "||", "??", "?", "&&=", "||=", "??=":
				conditional = conditional || depth == 0
			}
			continue
		case 't':
			sub := &legacyFile{path: f.path, blob: f.blob, data: f.data, tokens: t[i].sub}
			if match, ok := legacyMatchBrackets(sub.tokens); ok {
				sub.match = match
				fixtures = append(fixtures, p.casePreconditions(sub, caseID, legacyRange{0, len(sub.tokens)}, true)...)
				continue
			}
			for _, callee := range legacySubCalls(t[i].sub) {
				if legacyPreconditionOf[callee] != "" {
					p.unknown(f, caseID, "nested-precondition", callee+" inside a template substitution is not bound", t[i].line, t[i].line)
				}
			}
			continue
		case 'i':
		default:
			continue
		}
		if legacyConditional[t[i].text] {
			conditional = true
			continue
		}
		if legacyChained(t, i) || legacyKeyword[t[i].text] {
			continue
		}
		end, callee, open := legacyChain(f, i, r.hi)
		kind := legacyPreconditionOf[callee]
		if open < 0 || kind == "" {
			continue
		}
		fixture, ok := p.record(f, i, end, kind, callee)
		if !ok {
			p.unknown(f, caseID, "nested-precondition", callee+" span cannot be anchored", t[i].line, t[end-1].line)
			continue
		}
		fixtures = append(fixtures, fixture)
		if nested || depth != 0 || conditional || !legacyStatementStart(f, i, r.lo) {
			p.unknown(f, caseID, "nested-precondition", callee+" may not run unconditionally before the assertions", t[i].line, t[end-1].line)
		}
	}
	return fixtures
}

// legacyHiddenCode reports whether tokens[lo:hi] contain a callback or a call inside a template
// substitution, whose effects a recorded call text binds but the static reader cannot order.
func legacyHiddenCode(t []legacyToken, lo, hi int) bool {
	for k := lo; k < hi; k++ {
		if legacyPunct(t, k, "=>") || t[k].kind == 'i' && t[k].text == "function" || t[k].kind == 't' && len(legacySubCalls(t[k].sub)) > 0 {
			return true
		}
	}
	return false
}

// legacySubCalls lists the callees (member path up to the call) inside template substitution
// tokens, including nested templates.
func legacySubCalls(tokens []legacyToken) []string {
	out := []string{}
	for i := range tokens {
		if tokens[i].kind == 't' {
			out = append(out, legacySubCalls(tokens[i].sub)...)
		}
		if tokens[i].kind != 'i' || legacyChained(tokens, i) {
			continue
		}
		callee, k := tokens[i].text, i+1
		for k+1 < len(tokens) && (legacyPunct(tokens, k, ".") || legacyPunct(tokens, k, "?.")) && tokens[k+1].kind == 'i' {
			callee, k = callee+"."+tokens[k+1].text, k+2
		}
		if legacyPunct(tokens, k, "(") {
			out = append(out, callee)
		}
	}
	return out
}

// legacySubWords reports whether template substitution tokens use one of names.
func legacySubWords(tokens []legacyToken, names map[string]bool) bool {
	for _, token := range tokens {
		if token.kind == 'i' && names[token.text] || token.kind == 't' && legacySubWords(token.sub, names) {
			return true
		}
	}
	return false
}

// legacyVisitURL returns the static first argument of a visit call text.
func legacyVisitURL(text string) (string, bool) {
	tokens, _, ok := lexLegacy(text)
	for i := 0; ok && i+2 < len(tokens); i++ {
		if legacyPunct(tokens, i, "(") {
			if tokens[i+1].kind == 's' && (legacyPunct(tokens, i+2, ")") || legacyPunct(tokens, i+2, ",")) && textOK(tokens[i+1].text) {
				return tokens[i+1].text, true
			}
			return "", false
		}
	}
	return "", false
}

type legacyTuple struct {
	at, from, to            int
	matcher, locator, value string
}

// legacyValue is the expected value of the call at open after skipping its first skip arguments:
// one static string is unquoted, anything else is its layout-free text.
func legacyValue(f *legacyFile, open, skip, only int) string {
	if open < 0 {
		return legacyNoValue
	}
	args := legacyArgs(f, open)
	if len(args) <= skip {
		return legacyNoValue
	}
	args = args[skip:]
	if only > 0 && len(args) > only {
		args = args[:only]
	}
	if len(args) == 1 && args[0].hi-args[0].lo == 1 && f.tokens[args[0].lo].kind == 's' && textOK(f.tokens[args[0].lo].text) {
		return f.tokens[args[0].lo].text
	}
	return legacyCanonical(f.tokens, args[0].lo, args[len(args)-1].hi)
}

// legacyChainRoot walks back from the '.' at dot to the root of its call chain.
func legacyChainRoot(f *legacyFile, dot, lo int) (int, bool) {
	t := f.tokens
	k := dot - 1
	for k >= lo {
		switch {
		case legacyPunct(t, k, ")") || legacyPunct(t, k, "]"):
			m := f.match[k]
			if m-1 >= lo && t[m-1].kind == 'i' && !legacyKeyword[t[m-1].text] {
				k = m - 1
				continue
			}
			if m < lo {
				return 0, false
			}
			return m, true
		case t[k].kind == 'i' || t[k].kind == 's':
			if k-2 >= lo && legacyChained(t, k) {
				k -= 2
				continue
			}
			return k, true
		default:
			return 0, false
		}
	}
	return 0, false
}

// assertions extracts the assertion tuples of a case body. Every expect, assert or should the
// patterns do not consume is reported as an unparsed-assertion unknown.
func (p *legacyProducer) assertions(f *legacyFile, caseID string, r legacyRange) []BehaviorLegacyCriterion {
	t := f.tokens
	consumed := map[int]bool{}
	tuples := []legacyTuple{}
	for i := r.lo; i < r.hi; i++ {
		if t[i].kind != 'i' {
			continue
		}
		switch {
		case (t[i].text == "expect" || t[i].text == "expectAsync") && legacyPunct(t, i+1, "("):
			close := f.match[i+1]
			locator := legacyCanonical(t, i+2, close)
			k, members, call := close+1, []string{}, -1
			for k+1 < r.hi && legacyPunct(t, k, ".") && t[k+1].kind == 'i' {
				members, k = append(members, t[k+1].text), k+2
				if legacyPunct(t, k, "(") {
					if len(members) == 1 && members[0] == "withContext" {
						// Jasmine's failure message is not part of the observable.
						members, k = members[:0], f.match[k]+1
						continue
					}
					call = k
					break
				}
			}
			if len(members) == 0 {
				continue
			}
			end := k - 1
			if call >= 0 {
				end = f.match[call]
				if next := end + 1; next+1 < r.hi && ((legacyPunct(t, next, ".") || legacyPunct(t, next, "?.")) && t[next+1].kind == 'i' || legacyPunct(t, next, "[") || legacyPunct(t, next, "(")) {
					p.unknown(f, caseID, "unparsed-assertion", "a chained assertion continues after the first matcher: "+legacyBounded(legacyCanonical(t, i, min(next+12, r.hi))), t[i].line, t[min(next+12, r.hi)-1].line)
				}
			}
			consumed[i] = true
			tuples = append(tuples, legacyTuple{at: i, from: i, to: end, matcher: strings.Join(members, "."), locator: locator, value: legacyValue(f, call, 0, 0)})
		case t[i].text == "assert" && !legacyChained(t, i):
			k, matcher := i+1, "assert"
			if legacyPunct(t, k, ".") && k+1 < r.hi && t[k+1].kind == 'i' {
				matcher, k = matcher+"."+t[k+1].text, k+2
			}
			if !legacyPunct(t, k, "(") {
				continue
			}
			args := legacyArgs(f, k)
			if len(args) == 0 {
				continue
			}
			consumed[i] = true
			tuples = append(tuples, legacyTuple{at: i, from: i, to: f.match[k], matcher: matcher, locator: legacyCanonical(t, args[0].lo, args[0].hi), value: legacyValue(f, k, 1, 1)})
		case t[i].text == "should" && legacyChained(t, i) && legacyPunct(t, i+1, "("):
			root, ok := legacyChainRoot(f, i-1, r.lo)
			if !ok {
				continue
			}
			subject := i - 1
			for subject > root && legacyPunct(t, subject-1, ")") {
				m := f.match[subject-1]
				if m-2 < root || t[m-1].kind != 'i' || (t[m-1].text != "should" && t[m-1].text != "and") || !legacyChained(t, m-1) {
					break
				}
				subject = m - 2
			}
			locator := legacyCanonical(t, root, subject)
			consumed[i] = true
			for call := i + 1; ; {
				args := legacyArgs(f, call)
				if len(args) == 0 || args[0].hi-args[0].lo != 1 || t[args[0].lo].kind != 's' || !textOK(t[args[0].lo].text) {
					p.unknown(f, caseID, "should-callback", "a should/and assertion without a static chainer: "+legacyCanonical(t, root, f.match[call]+1), t[root].line, t[f.match[call]].line)
				} else {
					tuples = append(tuples, legacyTuple{at: call - 1, from: root, to: f.match[call], matcher: "should." + t[args[0].lo].text, locator: locator, value: legacyValue(f, call, 1, 0)})
				}
				next := f.match[call] + 1
				if !legacyPunct(t, next, ".") || next+2 >= r.hi || t[next+1].kind != 'i' || t[next+1].text != "and" || !legacyPunct(t, next+2, "(") {
					break
				}
				consumed[next+1] = true
				call = next + 2
			}
		case t[i].text == "contains" && legacyChained(t, i) && legacyPunct(t, i+1, "("):
			root, ok := legacyChainRoot(f, i-1, r.lo)
			if !ok || t[root].kind != 'i' || t[root].text != "cy" {
				continue
			}
			prefix := legacyCanonical(t, root, i-1)
			args := legacyArgs(f, i+1)
			switch {
			case len(args) == 0:
				continue
			case len(args) == 1 || len(args) == 2 && legacyPunct(t, args[1].lo, "{") && f.match[args[1].lo] == args[1].hi-1:
				tuples = append(tuples, legacyTuple{at: i, from: root, to: f.match[i+1], matcher: "contains", locator: prefix, value: legacyValue(f, i+1, 0, 1)})
			case args[1].hi-args[1].lo != 1 || !strings.ContainsRune("snr", rune(t[args[1].lo].kind)) && (t[args[1].lo].kind != 't' || len(t[args[1].lo].sub) > 0):
				// The second argument is neither a literal content nor a plain options object, so
				// which argument is the selector is not static.
				p.unknown(f, caseID, "unparsed-assertion", "a contains call whose arguments are not static: "+legacyBounded(legacyCanonical(t, root, f.match[i+1]+1)), t[root].line, t[f.match[i+1]].line)
				consumed[i] = true
			default:
				tuples = append(tuples, legacyTuple{at: i, from: root, to: f.match[i+1], matcher: "contains", locator: prefix + " " + legacyCanonical(t, args[0].lo, args[0].hi), value: legacyValue(f, i+1, 1, 1)})
			}
		}
	}
	for i := r.lo; i < r.hi; i++ {
		if t[i].kind == 's' && legacyPunct(t, i-1, "[") && legacyPunct(t, i+1, "]") && legacyTemplateAssertionWord[t[i].text] {
			p.unknown(f, caseID, "unparsed-assertion", "an assertion reached through a computed member: "+legacyBounded(legacyCanonical(t, max(i-6, r.lo), min(i+8, r.hi))), t[i].line, t[i].line)
		}
		if t[i].kind == 't' && legacySubWords(t[i].sub, legacyTemplateAssertionWord) {
			p.unknown(f, caseID, "unparsed-assertion", "an assertion inside a template substitution: "+legacyBounded(t[i].raw), t[i].line, t[i].line)
		}
		cypressAnd := t[i].kind == 'i' && t[i].text == "and" && legacyChained(t, i) && legacyPunct(t, i+1, "(")
		if t[i].kind == 'i' && (legacyAssertionWord[t[i].text] || cypressAnd) && !consumed[i] {
			p.unknown(f, caseID, "unparsed-assertion", legacyCanonical(t, i, min(i+12, r.hi)), t[i].line, t[min(i+12, r.hi)-1].line)
		}
	}
	sort.SliceStable(tuples, func(i, j int) bool { return tuples[i].at < tuples[j].at })
	helpers := legacyHelperSpans(f, r)
	criteria := []BehaviorLegacyCriterion{}
	for _, tuple := range tuples {
		if slices.ContainsFunc(helpers, func(h legacyRange) bool { return h.lo <= tuple.at && tuple.at < h.hi }) {
			p.unknown(f, caseID, "unparsed-assertion", "an assertion inside a function the case does not visibly call: "+legacyBounded(legacyCanonical(t, tuple.from, tuple.to+1)), t[tuple.from].line, t[tuple.to].line)
			continue
		}
		if !textOK(tuple.matcher) || !textOK(tuple.locator) || !textOK(tuple.value) {
			p.unknown(f, caseID, "unparsed-assertion", legacyCanonical(t, tuple.from, tuple.to+1), t[tuple.from].line, t[tuple.to].line)
			continue
		}
		evidence, err := p.anchor(f, t[tuple.from].line, t[tuple.to].line, "static legacy "+p.name+" assertion")
		if err != nil {
			p.unknown(f, caseID, "unparsed-assertion", legacyCanonical(t, tuple.from, tuple.to+1), 0, 0)
			continue
		}
		criteria = append(criteria, BehaviorLegacyCriterion{ID: "c" + strconv.Itoa(len(criteria)+1), Evidence: evidence, Matcher: tuple.matcher, Locator: tuple.locator, Value: tuple.value})
	}
	return criteria
}

// legacyHelperSpans returns the token ranges of functions defined in r that are not passed
// directly as a call argument: a named or assigned helper may never run, so an assertion inside
// it is not a criterion of the case.
func legacyHelperSpans(f *legacyFile, r legacyRange) []legacyRange {
	t := f.tokens
	spans := []legacyRange{}
	for k := r.lo; k < r.hi; k++ {
		head, end := -1, -1
		switch {
		case t[k].kind == 'i' && t[k].text == "function" && !legacyChained(t, k):
			head = k
			open := k + 1
			for open < r.hi && !legacyPunct(t, open, "(") {
				open++
			}
			if open < r.hi && legacyPunct(t, f.match[open]+1, "{") {
				end = f.match[f.match[open]+1]
			}
		case legacyPunct(t, k, "=>"):
			head = k - 1
			if legacyPunct(t, head, ")") {
				head = f.match[head]
			}
			if legacyPunct(t, k+1, "{") {
				end = f.match[k+1]
				break
			}
			j := k + 1
			for ; j < r.hi; j++ {
				if t[j].kind == 'p' && (t[j].text == "(" || t[j].text == "[" || t[j].text == "{") {
					j = f.match[j]
					continue
				}
				if t[j].kind == 'p' && (t[j].text == ")" || t[j].text == "]" || t[j].text == "}" || t[j].text == ";" || t[j].text == ",") ||
					j > k+1 && t[j].line > t[j-1].line && legacyStatementStart(f, j, r.lo) {
					break
				}
			}
			end = j - 1
		}
		if head < r.lo || end < k {
			continue
		}
		if head > r.lo && t[head-1].kind == 'i' && t[head-1].text == "async" {
			head--
		}
		if head > r.lo && (legacyPunct(t, head-1, "(") || legacyPunct(t, head-1, ",")) {
			continue
		}
		spans = append(spans, legacyRange{lo: k, hi: end + 1})
	}
	return spans
}
