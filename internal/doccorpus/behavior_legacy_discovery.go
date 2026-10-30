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
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
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
	"the closed-world allowlist over-refuses: a case using any helper call, Cypress action, declaration, await, async or expression-bodied callback, assertion chainer outside the exact admitted set, negated matcher, subject-changing continuation, contains option, ambiguous string value or other unlisted form is unknown, never parity",
	"other cases' bodies, hooks of non-enclosing suites, getters on operand paths and custom matchers or commands overriding a known name are not proven; cross-test state needs the runtime baseline",
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
	// The residue check (DCP-V1-062) requires every case-body occurrence of these words to have
	// been consumed by a recognised extraction; anything left over is an unknown, so a form the
	// reader does not list can never make an assertion or precondition vanish.
	legacyResidueRoot         = words("cy browser page")
	legacyPreconditionWord    = words("task request intercept fixture visit exec route goto")
	legacyComputedWord        = words("expect expectAsync assert should contains and task request intercept fixture visit exec route goto")
	legacyTemplateCommandWord = words("cy task request intercept fixture visit exec route goto")
	legacyEachHook            = words("beforeEach setup")
	legacyConditional         = words("if for while switch try do catch else")
)

const legacyNoValue = "(no expected value)"

type legacyFile struct {
	path, blob string
	data       []byte
	tokens     []legacyToken
	match      []int
	allowAt    int // the first token of the runner structure the allowlist refuses, or -1
}

// legacyScope is what a suite passes to its children: its title path, the precondition
// identities of its run-once (all) and per-case (each) hooks in execution order, and the hook
// uncertainty every case under it inherits.
type legacyScope struct {
	titles                                   []string
	all, each, uncertain                     []string
	disabled, focused, dynamic, dynamicTitle bool
	allowAt                                  int // 1 + the first token an enclosing hook body has outside the allowlist; 0 when none
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
	auth, err := gitauth.Open(input.Root, newGitBudget(ctx))
	if err != nil {
		return nil, err
	}
	release := auth.BeginObjectSession()
	defer release()
	paths, err := legacyPaths(ctx, auth, input.Revision, input.Path)
	if err != nil {
		return nil, err
	}
	// Every test file is resolved and read in batches (DCP-V1-216); the loop
	// below keeps its per-file refusals and order.
	batched := newPathPrefetch(input.Revision)
	if err := batched.resolve(ctx, auth, paths); err != nil {
		return nil, err
	}
	if err := batched.read(ctx, auth, batched.regularOIDs(paths), MaxBytes, maxPinnedInputBytes); err != nil {
		return nil, err
	}
	inventory := &LegacyInventory{Schema: LegacyInventorySchema, Status: "generated", Runner: input.Runner, Repository: repository, Revision: input.Revision, Path: input.Path, Legacy: []BehaviorLegacyCase{}, Preconditions: []LegacyPrecondition{}, Unknown: []LegacyUnknown{}, Limitations: legacyLimitations}
	p := &legacyProducer{runner: runner, name: input.Runner, repository: repository, revision: input.Revision, inv: inventory, pre: map[string]LegacyPrecondition{}, visits: map[string][]string{}, caseUnknown: map[string]bool{}, ids: map[string]bool{}, duplicates: map[string]bool{}}
	for _, current := range paths {
		entry, ok, err := batched.lookup(ctx, auth, input.Revision, current)
		if err != nil {
			return nil, err
		}
		if !ok || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return nil, fail("legacy test file is not a regular Git blob at the revision: " + current)
		}
		data, over, err := batched.blobWithin(ctx, auth, entry.OID, MaxBytes)
		if err != nil {
			return nil, err
		}
		file := &legacyFile{path: current, blob: entry.OID, data: data}
		if over {
			p.unknown(file, "", "file-over-bound", "the test file exceeds the input bound; its cases are not inventoried", 0, 0)
			continue
		}
		p.file(file)
		if len(inventory.Legacy) > MaxRecords || len(inventory.Unknown) > MaxRecords || len(p.pre) > MaxRecords {
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

// legacyPaths resolves --path at the revision: a blob is read as is, a directory yields its
// test-named entries in path order, skipping node_modules; the read refuses a non-regular one.
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
		// A test-named symlink or other non-regular entry is kept, so the read refuses it exactly as
		// it refuses that path selected alone instead of the walk silently dropping it.
		if legacyTestPath(current) {
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
	f.allowAt = p.allowStructure(f, 0, len(tokens))
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
	refs := []string{}
	for i := lo; i < hi; i++ {
		if _, ok := p.apiCall(f, i); !ok && p.runnerReference(f.tokens, i) {
			// beforeEach.call(...), it['only'](...), const h = beforeEach: a runner function used
			// in a form the reader does not inventory. A hook so used may still run before every
			// case here.
			detail := f.tokens[i].text + " is used in a form the static reader does not inventory: " + legacyBounded(legacyCanonical(f.tokens, i, min(i+8, hi)))
			p.unknown(f, "", "runner-reference-not-static", detail, f.tokens[i].line, f.tokens[min(i+8, hi)-1].line)
			if p.runner.hooks[f.tokens[i].text] {
				refs = append(refs, detail)
			}
		}
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
	inner.all, inner.each, inner.uncertain = slices.Clone(s.all), slices.Clone(s.each), append(slices.Clone(s.uncertain), refs...)
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
		if inner.allowAt == 0 {
			inner.allowAt = p.allowHook(f, c)
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
		allowAt:      s.allowAt,
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
	keys, overBound := []int{}, false
	if bad, region := p.allowCase(f, s, args, &keys); bad >= 0 {
		bad = min(bad, len(t)-1)
		detail := region + ": " + legacyBounded(t[bad].raw) + " is the first token outside the closed-world allowlist: " + legacyCanonical(t, max(0, bad-4), min(len(t), bad+8))
		p.unknown(f, id, "not-allowlisted", detail, t[bad].line, t[bad].line)
	}
	if hasBody {
		consumed := map[int]bool{}
		fixtures = append(fixtures, p.casePreconditions(f, id, body, false, consumed)...)
		for i := body.lo; i < body.hi; i++ {
			if p.name == "jasmine" && t[i].kind == 'i' && t[i].text == "pending" && !legacyChained(t, i) && legacyPunct(t, i+1, "(") {
				legacy.State = "disabled"
			}
			if t[i].kind == 'i' && t[i].text == "this" && legacyPunct(t, i+1, ".") && i+2 < len(t) && t[i+2].text == "skip" && legacyPunct(t, i+3, "(") {
				note("runtime-skip", "the case may skip itself at runtime")
			}
		}
		legacy.Criteria = p.assertions(f, id, body, consumed)
		if len(legacy.Criteria) > MaxRecords {
			// A registry refuses a case over the criterion bound; a truncated list would claim a
			// partial case, so none is emitted and the case is unknown.
			note("criteria-over-bound", strconv.Itoa(len(legacy.Criteria))+" assertion tuples exceed the per-case bound of "+strconv.Itoa(MaxRecords))
			legacy.Criteria = []BehaviorLegacyCriterion{}
			overBound = true
		}
		p.residue(f, id, body, consumed)
		for _, key := range keys {
			if !consumed[key] {
				// Defense in depth: an admitted form the extraction did not bind is still refused.
				p.unknown(f, id, "not-allowlisted", "an admitted statement was not extracted: "+legacyBounded(legacyCanonical(t, key, min(body.hi, key+12))), t[key].line, t[key].line)
			}
		}
	}
	// Execution order is part of the precondition identity; a repeated call keeps its first place.
	for _, fixture := range fixtures {
		if !slices.Contains(legacy.Fixtures, fixture) {
			legacy.Fixtures = append(legacy.Fixtures, fixture)
		}
	}
	if len(legacy.Fixtures) > MaxRecords {
		// A registry refuses a case over the fixture bound; a truncated list would drop
		// preconditions, so none is emitted and the case is unknown.
		note("fixtures-over-bound", strconv.Itoa(len(legacy.Fixtures))+" preconditions exceed the per-case bound of "+strconv.Itoa(MaxRecords))
		legacy.Fixtures = []string{}
	}
	if len(legacy.Criteria) == 0 && !overBound {
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
			// Only a bracket holding exactly one static identifier-name string resolves
			// (cy['task'] is cy.task); any other non-literal computed member marks the callee
			// unresolved ("[]"), so it is never a precondition kind.
			switch {
			case open >= 0:
			case f.match[k] == k+2 && t[k+1].kind == 's' && legacyIdentifierName(t[k+1].text):
				callee += "." + t[k+1].text
			case f.match[k] != k+2 || t[k+1].kind != 's' && t[k+1].kind != 'n':
				callee += "[]"
			}
			k = f.match[k] + 1
		default:
			return k, callee, open
		}
	}
	return k, callee, open
}

// legacyIdentifierName reports whether text is a plain JavaScript identifier name.
func legacyIdentifierName(text string) bool {
	for i, r := range text {
		if r != '_' && r != '$' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return text != ""
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
	if !t[i].nl {
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
		if strings.Contains(callee, "[]") {
			doubt("a hook call reaches a computed member the static reader does not resolve: " + callee)
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
//
// consumed (nil inside a template substitution) receives the tokens of every recorded callee path,
// root through the member called, for the residue check.
func (p *legacyProducer) casePreconditions(f *legacyFile, caseID string, r legacyRange, nested bool, consumed map[int]bool) []string {
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
				fixtures = append(fixtures, p.casePreconditions(sub, caseID, legacyRange{0, len(sub.tokens)}, true, nil)...)
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
		if consumed != nil {
			for k := i; k < open; k++ {
				consumed[k] = true
			}
		}
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

// legacySubWords reports whether template substitution tokens use a name that match accepts, as
// an identifier or as a string (a computed member such as globalThis['expect']).
func legacySubWords(tokens []legacyToken, match func(string) bool) bool {
	for _, token := range tokens {
		if (token.kind == 'i' || token.kind == 's') && match(token.text) || token.kind == 't' && legacySubWords(token.sub, match) {
			return true
		}
	}
	return false
}

// legacyStandardTo lists the built-in to* methods that are not assertion matchers.
var legacyStandardTo = words("toString toFixed toPrecision toExponential toLowerCase toUpperCase toLocaleString toLocaleLowerCase toLocaleUpperCase toLocaleDateString toLocaleTimeString toISOString toUTCString toGMTString toDateString toTimeString toJSON toSorted toReversed toSpliced toWellFormed toArray")

// legacyMatcherName reports whether a member name is an assertion matcher or chain word (chai's
// to, Jasmine/Jest's toBe..., toEqual...), whatever expression it was reached from.
func legacyMatcherName(name string) bool {
	if name == "to" {
		return true
	}
	return len(name) > 2 && strings.HasPrefix(name, "to") && name[2] >= 'A' && name[2] <= 'Z' && !legacyStandardTo[name]
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

// assertions extracts the assertion tuples of a case body and marks every token a recognised
// form consumed; the residue check reports whatever is left.
func (p *legacyProducer) assertions(f *legacyFile, caseID string, r legacyRange, consumed map[int]bool) []BehaviorLegacyCriterion {
	t := f.tokens
	tuples := []legacyTuple{}
	continued := func(i, next int) {
		p.unknown(f, caseID, "unparsed-assertion", "a chained assertion continues after the first matcher: "+legacyBounded(legacyCanonical(t, i, min(next+12, r.hi))), t[i].line, t[min(next+12, r.hi)-1].line)
		legacyConsumeChain(f, consumed, next, r.hi)
	}
	for i := r.lo; i < r.hi; i++ {
		if t[i].kind != 'i' {
			continue
		}
		switch {
		case (t[i].text == "expect" || t[i].text == "expectAsync") && !legacyChained(t, i) && legacyPunct(t, i+1, "("):
			// A member expect (chai.expect, obj.expect) is not proven to be the runner's; the
			// residue check reports it.
			close := f.match[i+1]
			locator := legacyCanonical(t, i+2, close)
			k, members, call := close+1, []string{}, -1
			for k+1 < r.hi && legacyPunct(t, k, ".") && t[k+1].kind == 'i' {
				consumed[k+1] = true
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
			consumed[i] = true
			end := k - 1
			if call < 0 && (legacyPunct(t, k, "[") || legacyPunct(t, k, "?.")) {
				// A property chain that continues through a computed or optional member has no
				// static matcher.
				continued(i, k)
				continue
			}
			if call >= 0 {
				end = f.match[call]
				if next := end + 1; next+1 < r.hi && ((legacyPunct(t, next, ".") || legacyPunct(t, next, "?.")) && t[next+1].kind == 'i' || legacyPunct(t, next, "[") || legacyPunct(t, next, "(")) {
					continued(i, next)
				}
			}
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
				if legacyPunct(t, next, "[") || legacyPunct(t, next, "(") {
					continued(root, next)
					break
				}
				if !legacyPunct(t, next, ".") || next+2 >= r.hi || t[next+1].kind != 'i' || t[next+1].text != "and" || !legacyPunct(t, next+2, "(") {
					break
				}
				consumed[next+1] = true
				call = next + 2
			}
		case t[i].text == "contains" && legacyChained(t, i) && legacyPunct(t, i+1, "("):
			// Only a cy-rooted contains is a Cypress assertion; any other root is left for the
			// residue check to report.
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
			default:
				tuples = append(tuples, legacyTuple{at: i, from: root, to: f.match[i+1], matcher: "contains", locator: prefix + " " + legacyCanonical(t, args[0].lo, args[0].hi), value: legacyValue(f, i+1, 1, 1)})
			}
			consumed[i] = true
		}
	}
	sort.SliceStable(tuples, func(i, j int) bool { return tuples[i].at < tuples[j].at })
	helpers := legacyUnprovenFunctions(f, r)
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

// legacyConsumeChain marks the member names of the chain continuing at k as reported, so one
// unknown covers one unparsed chain.
func legacyConsumeChain(f *legacyFile, consumed map[int]bool, k, hi int) {
	t := f.tokens
	for k < hi {
		switch {
		case (legacyPunct(t, k, ".") || legacyPunct(t, k, "?.")) && k+1 < hi && t[k+1].kind == 'i':
			consumed[k+1] = true
			k += 2
		case legacyPunct(t, k, "["):
			if f.match[k] == k+2 {
				consumed[k+1] = true
			}
			k = f.match[k] + 1
		case legacyPunct(t, k, "("):
			k = f.match[k] + 1
		default:
			return
		}
	}
}

// residue is the fail-closed check of a case body (DCP-V1-062): every code-token occurrence of
// an assertion word, a Cypress chain word, a precondition root or member, or a runner call that no
// recognised extraction consumed becomes an unknown. It never adds a criterion or a fixture.
func (p *legacyProducer) residue(f *legacyFile, caseID string, r legacyRange, consumed map[int]bool) {
	t := f.tokens
	report := func(reason string, i int, detail string) {
		end := min(i+12, r.hi)
		p.unknown(f, caseID, reason, legacyBounded(detail+legacyCanonical(t, i, end)), t[i].line, t[end-1].line)
	}
	// Computed members first, so a chain walk does not report the same bracket twice.
	for i := r.lo; i < r.hi; i++ {
		if legacyPunct(t, i, "[") {
			// A watched name anywhere inside a bracket (['expect'], [('expect')], [c ? 'cy' : x])
			// may select an assertion or command the extraction never saw.
			kind, at := "", -1
			for j := i + 1; j < f.match[i]; j++ {
				if t[j].kind == 's' && (legacyComputedWord[t[j].text] || legacyResidueRoot[t[j].text]) && !consumed[j] {
					if at < 0 {
						at = j
					}
					if kind == "" || kind == "an assertion" {
						kind = "an assertion"
						if legacyPreconditionWord[t[j].text] || legacyResidueRoot[t[j].text] {
							kind = "a command"
						}
					}
					consumed[j] = true
				}
			}
			if kind != "" {
				p.unknown(f, caseID, "unparsed-assertion", kind+" reached through a computed member: "+legacyBounded(legacyCanonical(t, max(at-6, r.lo), min(at+8, r.hi))), t[i].line, t[f.match[i]].line)
			}
		}
		if t[i].kind != 't' {
			continue
		}
		if legacySubWords(t[i].sub, func(w string) bool { return legacyTemplateAssertionWord[w] || legacyMatcherName(w) }) {
			p.unknown(f, caseID, "unparsed-assertion", "an assertion inside a template substitution: "+legacyBounded(t[i].raw), t[i].line, t[i].line)
		} else if legacySubWords(t[i].sub, func(w string) bool { return legacyTemplateCommandWord[w] || legacyResidueRoot[w] }) && !slices.ContainsFunc(legacySubCalls(t[i].sub), func(callee string) bool { return legacyPreconditionOf[callee] != "" }) {
			// A recorded precondition in a substitution already has its nested-precondition unknown.
			p.unknown(f, caseID, "unparsed-command", "a command inside a template substitution: "+legacyBounded(t[i].raw), t[i].line, t[i].line)
		}
	}
	for i := r.lo; i < r.hi; i++ {
		if t[i].kind != 'i' || legacyObjectKey(t, i) {
			continue
		}
		word := t[i].text
		if legacyResidueRoot[word] && legacyChained(t, i) && (word == "cy" || legacyPunct(t, i+1, "[") ||
			(legacyPunct(t, i+1, ".") || legacyPunct(t, i+1, "?.")) && i+2 < r.hi && t[i+2].kind == 'i' && legacyPreconditionOf[word+"."+t[i+2].text] != "") {
			// cy, or a browser/page precondition, reached as a member (window.cy,
			// globalThis.page.goto) is not the case's own chain.
			report("unparsed-command", i, word+" reached as a member is not resolved: ")
			continue
		}
		if legacyResidueRoot[word] && !legacyChained(t, i) {
			if detail := legacyRootResidue(f, i, r.hi, consumed); detail != "" {
				report("unparsed-command", i, detail)
			}
			continue
		}
		if consumed[i] {
			continue
		}
		switch {
		case legacyAssertionWord[word] || word == "and" || word == "contains":
			report("unparsed-assertion", i, "")
		case legacyMatcherName(word) && legacyChained(t, i):
			// A matcher whose subject the extraction did not bind ((0, expect)(a).toBe(1)).
			report("unparsed-assertion", i, "a matcher the static reader did not bind: ")
		case legacyPreconditionWord[word]:
			if legacyChained(t, i) {
				// A member of a cy/browser/page chain is judged by that chain's walk; a member of
				// another plain object reached only through plain members and calls is not a
				// runner precondition.
				if root, ok := legacyChainRoot(f, i-1, r.lo); ok && t[root].kind == 'i' && !legacyKeyword[t[root].text] &&
					(legacyResidueRoot[t[root].text] && !legacyChained(t, root) || legacyPlainPath(f, root, i)) {
					continue
				}
				report("unparsed-command", i, "a precondition command on a chain the static reader does not resolve: ")
				continue
			}
			report("unparsed-command", i, "a precondition command the static reader does not bind: ")
		case p.runnerReference(t, i):
			report("runner-call-in-case", i, "a runner call inside a case body is not inventoried: ")
		}
	}
}

// legacyPlainPath reports whether the member chain from root to the member at i passes only plain
// non-runner-root members and calls: no computed member and no cy, browser or page segment.
func legacyPlainPath(f *legacyFile, root, i int) bool {
	t := f.tokens
	for k := root + 1; k < i-1; {
		switch {
		case (legacyPunct(t, k, ".") || legacyPunct(t, k, "?.")) && t[k+1].kind == 'i' && !legacyResidueRoot[t[k+1].text]:
			k += 2
		case legacyPunct(t, k, "("):
			k = f.match[k] + 1
		default:
			return false
		}
	}
	return true
}

// runnerReference reports whether token i names a suite, case or hook of the runner as a value
// (called, member-accessed, passed or aliased) rather than as a binding or property name.
func (p *legacyProducer) runnerReference(t []legacyToken, i int) bool {
	word := t[i].text
	if t[i].kind != 'i' || !(p.runner.suites[word] || p.runner.cases[word] || p.runner.hooks[word]) || legacyChained(t, i) || legacyObjectKey(t, i) {
		return false
	}
	if i > 0 && t[i-1].kind == 'i' && (t[i-1].text == "const" || t[i-1].text == "let" || t[i-1].text == "var" || t[i-1].text == "function" || t[i-1].text == "class") {
		return false
	}
	return !legacyPunct(t, i+1, "=") && !legacyPunct(t, i+1, ":")
}

// legacyRootResidue walks a cy, browser or page chain rooted at i and returns why it is not fully
// bound, or "". A cy chain may only continue through plain members and calls; any root may only
// reach a recorded precondition, and a computed member resolves only where its precondition was
// recorded or its bracket was already reported.
func legacyRootResidue(f *legacyFile, i, hi int, consumed map[int]bool) string {
	t := f.tokens
	root := t[i].text
	cy := root == "cy"
	k, open, path := i+1, -1, root
	if !legacyPunct(t, k, ".") && !legacyPunct(t, k, "[") && !legacyPunct(t, k, "?.") {
		if cy {
			return "cy is used in a form the static reader does not resolve: "
		}
		return ""
	}
walk:
	for k < hi {
		switch {
		case legacyPunct(t, k, ".") && k+1 < hi && t[k+1].kind == 'i':
			if open < 0 {
				path += "." + t[k+1].text
			}
			if legacyPreconditionWord[t[k+1].text] && !consumed[k+1] {
				return "a precondition command the static reader does not bind: "
			}
			k += 2
		case legacyPunct(t, k, "("):
			if open < 0 {
				open = k
			}
			if !cy {
				break walk
			}
			k = f.match[k] + 1
		case legacyPunct(t, k, "["):
			if f.match[k] == k+2 && consumed[k+1] {
				if open < 0 {
					path += "." + t[k+1].text
				}
				k += 3
				continue
			}
			return "a computed member on a " + root + " chain: "
		case legacyPunct(t, k, "?."):
			return "an optional " + root + " chain may skip its commands: "
		default:
			break walk
		}
	}
	if legacyPreconditionOf[path] != "" && !consumed[i] {
		return "a precondition command the static reader does not bind: "
	}
	return ""
}

// legacyObjectKey reports whether token i is a property name in an object literal ({key: ...}).
func legacyObjectKey(t []legacyToken, i int) bool {
	return (legacyPunct(t, i-1, "{") || legacyPunct(t, i-1, ",")) && legacyPunct(t, i+1, ":")
}

// legacyUnprovenFunctions returns the token ranges of function-like constructs in r whose call
// the reader cannot prove: anything other than a function or arrow expression passed whole as a
// direct call argument (a declared, assigned, grouped or immediately invoked function, an object
// or class method, getter or setter, or a class body). An assertion inside one is an unknown,
// never a criterion.
func legacyUnprovenFunctions(f *legacyFile, r legacyRange) []legacyRange {
	t := f.tokens
	spans := []legacyRange{}
	for k := r.lo; k < r.hi; k++ {
		head, end := -1, -1
		switch {
		case t[k].kind == 'i' && t[k].text == "class" && !legacyChained(t, k):
			for j := k + 1; j < r.hi; j++ {
				if legacyPunct(t, j, "{") {
					spans = append(spans, legacyRange{lo: k, hi: f.match[j] + 1})
					break
				}
				if legacyPunct(t, j, "(") || legacyPunct(t, j, "[") {
					j = f.match[j]
				}
			}
			continue
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
					j > k+1 && t[j].nl && legacyStatementStart(f, j, r.lo) {
					break
				}
			}
			end = j - 1
		case legacyPunct(t, k, "(") && legacyPunct(t, f.match[k]+1, "{") && legacyMethodName(t, k):
			// name(params) { ... } is a method, getter or setter definition.
			spans = append(spans, legacyRange{lo: k - 1, hi: f.match[f.match[k]+1] + 1})
			continue
		}
		if head < r.lo || end < k {
			continue
		}
		if head > r.lo && t[head-1].kind == 'i' && t[head-1].text == "async" {
			head--
		}
		if legacyDirectArgument(f, head, end, r.lo) {
			continue
		}
		spans = append(spans, legacyRange{lo: head, hi: end + 1})
	}
	return spans
}

// legacyMethodName reports whether the '(' at k follows a method name rather than a callee, a
// control keyword or a function expression's name.
func legacyMethodName(t []legacyToken, k int) bool {
	if k < 1 {
		return false
	}
	name := t[k-1]
	switch {
	case name.kind == 'i':
		if legacyChained(t, k-1) {
			return false
		}
		if name.text == "function" {
			// A function expression; the function branch judges it (and an object member named
			// function is not a direct argument there).
			return false
		}
		if legacyStatementHeader[name.text] {
			// if/for/while/switch/catch/with (...) { is a statement unless it is a member of an
			// object literal or follows a method modifier.
			prev := k - 2
			return prev >= 0 && (legacyPunct(t, prev, ",") || legacyPunct(t, prev, "*") || legacyPunct(t, prev, "{") && legacyObjectBrace(t, prev) ||
				t[prev].kind == 'i' && (t[prev].text == "get" || t[prev].text == "set" || t[prev].text == "async" || t[prev].text == "static"))
		}
		if legacyKeyword[name.text] {
			// No other keyword can head "kw (...) {", so it is a method name ({ return() {} }).
			return true
		}
		if k >= 2 && t[k-2].kind == 'i' && t[k-2].text == "function" || k >= 3 && legacyPunct(t, k-2, "*") && t[k-3].kind == 'i' && t[k-3].text == "function" {
			return false
		}
		return true
	case name.kind == 's' || name.kind == 'n':
		return true
	}
	return name.kind == 'p' && name.text == "]"
}

// legacyStatementHeader lists the keywords that head a statement shaped like a method definition.
var legacyStatementHeader = words("if for while switch catch with")

// legacyObjectBrace reports whether the '{' at k opens an object literal rather than a block. A
// brace after a header, an arrow, a block keyword or a statement boundary is a block; anything
// else (after '(', ',', '=', ':', an operator or return) is read as an object, which errs toward
// treating a member as an unproven method.
func legacyObjectBrace(t []legacyToken, k int) bool {
	if k < 1 {
		return false
	}
	prev := t[k-1]
	if prev.kind == 'p' {
		return !(prev.text == ")" || prev.text == "=>" || prev.text == "{" || prev.text == "}" || prev.text == ";")
	}
	return !(prev.kind == 'i' && (prev.text == "else" || prev.text == "try" || prev.text == "finally" || prev.text == "do"))
}

// legacyDirectArgument reports whether tokens[head:end+1] is one whole argument of a call: its
// enclosing bracket is a '(' that follows a callee, not a grouping, parameter list or header.
func legacyDirectArgument(f *legacyFile, head, end, lo int) bool {
	t := f.tokens
	open := -1
	for j := head - 1; j >= lo && open < 0; j-- {
		if t[j].kind != 'p' {
			continue
		}
		switch t[j].text {
		case ")", "]", "}":
			j = f.match[j]
		case "(", "[", "{":
			open = j
		}
	}
	if open < 1 || !legacyPunct(t, open, "(") || legacyPunct(t, f.match[open]+1, "{") || legacyPunct(t, f.match[open]+1, "=>") {
		return false
	}
	callee := t[open-1]
	switch {
	case callee.kind == 'i':
		if legacyKeyword[callee.text] || callee.text == "with" || open >= 2 && t[open-2].kind == 'i' && t[open-2].text == "function" {
			return false
		}
	case callee.kind == 'p' && callee.text == ")":
		// A call of a call result, unless the ')' closes a control header.
		if m := f.match[open-1]; m >= 1 && t[m-1].kind == 'i' && (legacyKeyword[t[m-1].text] || t[m-1].text == "with") {
			return false
		}
	case callee.kind == 'p' && callee.text == "]":
	default:
		return false
	}
	for _, arg := range legacyArgs(f, open) {
		if arg.lo == head && arg.hi == end+1 {
			return true
		}
	}
	return false
}
