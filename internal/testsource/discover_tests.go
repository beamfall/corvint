package testsource

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The static test-evidence producer behind `corvint docs corpus discover-tests`
// (DCX-V1-370..376, GitHub #1582). It reads committed Playwright or RSpec test files at one full
// commit through the TSA-V0 readers and emits, per example, its title path, tags, annotations,
// expectations, navigations and requests, statically reached modules, fixtures and shared
// groups, and the hooks that apply to it. Every fact carries a line anchor pinned to the blob;
// what the readers cannot classify is an unknown with a reason, never a guess. The record is
// generated trust: the producer sets no review verdict.

const (
	// TestDiscoverySchema names the record.
	TestDiscoverySchema = "corvint-test-discovery/1"
	// MaxTestDiscoveryBytes bounds the encoded record.
	MaxTestDiscoveryBytes = 32 << 20
	// maxDiscoveryResolvePaths bounds the tracked source paths kept for import resolution.
	maxDiscoveryResolvePaths = 200000
	// maxDiscoveryText bounds one rendered expression, receiver or detail.
	maxDiscoveryText = 1024
	// maxDiscoveryFacts bounds the facts of one kind on one example.
	maxDiscoveryFacts = 512
)

// Unknown reasons the producer emits (DCX-V1-376). computed-lookup, import-unresolved and
// registry-unresolved keep the meaning setup-select gives them (TSA-V0-003).
const (
	discoverReasonComputedLookup   = "computed-lookup"
	discoverReasonImportUnresolved = "import-unresolved"
	discoverReasonRegistry         = "registry-unresolved"
	discoverReasonTargetNotLiteral = "target-not-literal"
	discoverReasonTitleNotLiteral  = "title-not-literal"
	discoverReasonMatcherUnread    = "matcher-not-read"
	discoverReasonAssertionShape   = "assertion-shape-not-read"
	discoverReasonMetadata         = "metadata-not-literal"
	discoverReasonProjectNotStatic = "project-name-not-static"
	discoverReasonLoadPath         = "load-path-not-read"
	discoverReasonFixtureBody      = "fixture-body-not-read"
	discoverReasonFixtureUnknown   = "fixture-not-builtin"
	discoverReasonFactBound        = "fact-bound-exceeded"
	discoverReasonFileTooLarge     = "source-too-large"
	discoverReasonResolveBound     = "resolve-bound-exceeded"
	discoverReasonScopedLoop       = "scoped-id-loop-generated"
	discoverReasonScopedUnplaced   = "scoped-id-unplaced"
	discoverReasonScopedInline     = "scoped-id-inline-include"
	discoverReasonScopedShared     = "scoped-id-shared-body"
	discoverReasonSharedDefinition = "shared-definition-example"
	discoverReasonFactUnanchored   = "fact-span-unanchored"
)

var testDiscoveryLimitations = []string{
	"static reading only: no test is run, so a fact says what the committed source declares, not what a run executed",
	"the record is generated trust; it sets no review verdict and creates no reviewed relation",
	"helper, page-object and fixture bodies are not followed; a reached module is named by its import, and an assertion inside a helper is not counted on the example or hook that calls it",
	"a hook's asserts flag says only whether the hook's own body holds an assertion call",
	"JavaScript imports resolve relative specifiers by TypeScript's file, extension and index rules over the tracked tree; tsconfig paths and package exports are not read, so an alias is import-unresolved",
	"a Playwright example's js-title-path identity needs the project name the run receipt carries, which the source does not fix; its full name joins the title path with \" > \", and a receipt whose full title is only the leaf title matches the last title-path element",
	"RSpec shared groups resolve only within the same file, require resolves only require_relative, and global config hooks in support files are not seen",
	"an RSpec fact's span covers its logical line, through the line before the next logical line",
}

// TestDiscoveryInput is one production: the repository root, the runner (playwright or rspec), a
// full commit and a repository-relative path ("." for the whole tree).
type TestDiscoveryInput struct {
	Root     string
	Runner   string
	Revision string
	Path     string
}

// TestDiscovery is the corvint-test-discovery/1 record.
type TestDiscovery struct {
	Schema      string              `json:"schema"`
	Status      string              `json:"status"`
	Trust       string              `json:"trust"`
	Runner      string              `json:"runner"`
	Repository  string              `json:"repository"`
	Revision    string              `json:"revision"`
	Path        string              `json:"path"`
	Files       []DiscoveredFile    `json:"files"`
	Examples    []DiscoveredExample `json:"examples"`
	Unknown     []DiscoveryUnknown  `json:"unknown"`
	Limitations []string            `json:"limitations"`
}

// DiscoveredFile is one test file read: its pinned blob and whether the reader placed it.
type DiscoveredFile struct {
	Path   string `json:"path"`
	Blob   string `json:"blob"`
	SHA256 string `json:"sha256"`
	Read   string `json:"read"` // "located" or the reader's uncertainty reason
}

// DiscoveredExample is one example and its anchored facts.
type DiscoveredExample struct {
	ID           string                  `json:"id"`
	Path         string                  `json:"path"`
	Line         int                     `json:"line"`
	Identity     ExampleIdentity         `json:"identity"`
	TitlePath    []DiscoveredTitle       `json:"title_path"`
	TitleUnknown string                  `json:"title_unknown"` // why the title path is empty, or ""
	Tags         []DiscoveredTag         `json:"tags"`
	Annotations  []DiscoveredAnnotation  `json:"annotations"`
	Expectations []DiscoveredExpectation `json:"expectations"`
	Navigations  []DiscoveredNavigation  `json:"navigations"`
	Reaches      []DiscoveredReach       `json:"reaches"`
	Hooks        []DiscoveredHook        `json:"hooks"`
	Read         string                  `json:"read"` // "located" or why the example's body was not read
	Review       string                  `json:"review"`
	Evidence     doccorpus.Anchor        `json:"evidence"`
}

// ExampleIdentity is the line-free key `tests example-join` and run receipts use (TSA-V0-410):
// rspec-scoped-id `path[1:2:1]`, or js-title-path `path > PROJECT > full name` whose project the
// receipt supplies. Example is "" and Unknown names why when the source cannot fix the key.
type ExampleIdentity struct {
	Kind     string `json:"kind"`
	Example  string `json:"example"`
	File     string `json:"file"`
	FullName string `json:"full_name"`
	Unknown  string `json:"unknown"`
}

// DiscoveredTitle is one title on the example's path, outermost group first.
type DiscoveredTitle struct {
	Title    string           `json:"title"`
	Evidence doccorpus.Anchor `json:"evidence"`
}

// DiscoveredTag is one tag or metadata entry, declared on the example or an enclosing group.
type DiscoveredTag struct {
	Name     string           `json:"name"`
	Value    string           `json:"value"`
	Evidence doccorpus.Anchor `json:"evidence"`
}

// DiscoveredAnnotation is one annotation or run modifier (skip, fixme, fail, slow, only, todo).
type DiscoveredAnnotation struct {
	Type        string           `json:"type"`
	Description string           `json:"description"`
	Conditional bool             `json:"conditional"`
	Evidence    doccorpus.Anchor `json:"evidence"`
}

// DiscoveredExpectation is one assertion in the example's own body.
type DiscoveredExpectation struct {
	Matcher  string           `json:"matcher"`
	Negated  bool             `json:"negated"`
	Receiver string           `json:"receiver"`
	Unknown  string           `json:"unknown"`
	Evidence doccorpus.Anchor `json:"evidence"`
}

// DiscoveredNavigation is one navigation, request or awaited route in the example or a hook
// that applies to it (Source "example" or "hook").
type DiscoveredNavigation struct {
	Kind       string           `json:"kind"`
	Method     string           `json:"method"`
	Target     string           `json:"target"`
	Expression string           `json:"expression"`
	Source     string           `json:"source"`
	Unknown    string           `json:"unknown"`
	Evidence   doccorpus.Anchor `json:"evidence"`
}

// DiscoveredReach is one module, fixture or shared group the example reaches statically.
type DiscoveredReach struct {
	Kind        string            `json:"kind"` // import, fixture, shared-example or shared-context
	Name        string            `json:"name"`
	Specifier   string            `json:"specifier"`
	Target      string            `json:"target"`
	State       string            `json:"state"` // repository, package, builtin or unresolved
	Reason      string            `json:"reason"`
	Evidence    doccorpus.Anchor  `json:"evidence"`
	Declaration *doccorpus.Anchor `json:"declaration,omitempty"`
}

// DiscoveredHook is one hook that applies to the example.
type DiscoveredHook struct {
	Kind        string           `json:"kind"`
	Scope       string           `json:"scope"`
	Asserts     bool             `json:"asserts"`
	Conditional bool             `json:"conditional"`
	Evidence    doccorpus.Anchor `json:"evidence"`
}

// DiscoveryUnknown is something the reader saw and could not classify.
type DiscoveryUnknown struct {
	Example  string            `json:"example"`
	Path     string            `json:"path"`
	Reason   string            `json:"reason"`
	Detail   string            `json:"detail"`
	Evidence *doccorpus.Anchor `json:"evidence,omitempty"`
}

func discoveryFail(format string, args ...any) error {
	return &doccorpus.Error{Code: "corpus-refused", Message: fmt.Sprintf(format, args...)}
}

// discoveryTyped keeps a corpus code and maps a TSA-V0 refusal onto corpus-refused with its code
// in the message, so the command's error stays typed.
func discoveryTyped(err error) error {
	var t *Error
	if errors.As(err, &t) {
		code := "corpus-refused"
		if t.Code == "tests-invalid-revision" {
			code = "corpus-invalid-revision"
		}
		return &doccorpus.Error{Code: code, Message: t.Code + ": " + t.Message}
	}
	return err
}

// BuildTestDiscovery reads the runner's committed test files under input.Path at input.Revision
// and returns the encoded corvint-test-discovery/1 record (DCX-V1-370). It reads Git objects only,
// writes nothing and runs no test.
func BuildTestDiscovery(ctx context.Context, input TestDiscoveryInput) ([]byte, error) {
	if input.Runner != "playwright" && input.Runner != "rspec" {
		return nil, discoveryFail("test discovery runner must be playwright or rspec")
	}
	if !wire.IsGitOid(input.Revision) {
		return nil, discoveryFail("test discovery requires a full commit revision")
	}
	if input.Path != "." && (!fs.ValidPath(input.Path) || input.Path == ".git" || strings.HasPrefix(input.Path, ".git/") || strings.ContainsAny(input.Path, "\x00\n\r\\")) {
		return nil, discoveryFail("test discovery path is not a canonical repository-relative path")
	}
	r := OpenRepo(ctx, input.Root)
	commit, err := r.Resolve(input.Revision)
	if err != nil || commit != input.Revision {
		return nil, &doccorpus.Error{Code: "corpus-invalid-revision", Message: "test discovery revision is not a commit"}
	}
	repository, err := contextindex.CorpusRepositoryID(ctx, input.Root, input.Revision)
	if err != nil {
		return nil, err
	}
	test := func(name string) bool {
		if input.Path != "." && name != input.Path && !strings.HasPrefix(name, input.Path+"/") {
			return false
		}
		if input.Runner == "rspec" {
			return strings.HasSuffix(name, "_spec.rb")
		}
		return IsTestFile(name)
	}
	tracked, overflow := map[string]bool{}, false
	entries, err := r.tree(commit, func(name string) bool {
		if webExtensions[path.Ext(name)] || path.Ext(name) == ".rb" {
			if len(tracked) < maxDiscoveryResolvePaths {
				tracked[name] = true
			} else {
				overflow = true
			}
		}
		return test(name)
	}, MaxFiles)
	if err != nil {
		return nil, discoveryTyped(err)
	}
	if len(entries) == 0 {
		return nil, discoveryFail("%s holds no committed %s test file at the revision", input.Path, input.Runner)
	}
	out := &TestDiscovery{Schema: TestDiscoverySchema, Status: "generated", Trust: "generated", Runner: input.Runner, Repository: repository,
		Revision: commit, Path: input.Path, Files: []DiscoveredFile{}, Examples: []DiscoveredExample{}, Unknown: []DiscoveryUnknown{}, Limitations: testDiscoveryLimitations}
	readable := []entry{}
	for _, e := range entries {
		if e.size > MaxFileBytes {
			out.Files = append(out.Files, DiscoveredFile{Path: e.path, Blob: e.oid, Read: discoverReasonFileTooLarge})
			out.Unknown = append(out.Unknown, DiscoveryUnknown{Path: e.path, Reason: discoverReasonFileTooLarge, Detail: fmt.Sprintf("the blob exceeds %d bytes", MaxFileBytes)})
			continue
		}
		readable = append(readable, e)
	}
	data, _, err := r.blobs(readable, maxTotalBytes)
	if err != nil {
		return nil, discoveryTyped(err)
	}
	resolver := &discoveryResolver{tracked: tracked, overflow: overflow}
	for _, e := range readable {
		f := &discoveryFile{path: e.path, index: doccorpus.NewLineIndex(repository, commit, e.path, e.oid, data[e.oid]), text: string(data[e.oid]), out: out, resolver: resolver}
		read := ""
		if input.Runner == "rspec" {
			read = f.discoverRSpec()
		} else {
			read = f.discoverJS()
		}
		if read == "" {
			read = discoveryLocated
		}
		out.Files = append(out.Files, DiscoveredFile{Path: e.path, Blob: e.oid, SHA256: f.index.SHA256(), Read: read})
		if f.err != nil {
			return nil, f.err
		}
	}
	slices.SortFunc(out.Files, func(a, b DiscoveredFile) int { return strings.Compare(a.Path, b.Path) })
	slices.SortStableFunc(out.Examples, func(a, b DiscoveredExample) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return a.Line - b.Line
	})
	slices.SortStableFunc(out.Unknown, func(a, b DiscoveryUnknown) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(a.Example, b.Example)
	})
	encoded, err := json.Marshal(out, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	if len(encoded)+1 > MaxTestDiscoveryBytes {
		return nil, discoveryFail("test discovery output exceeds %d bytes", MaxTestDiscoveryBytes)
	}
	return append(encoded, '\n'), nil
}

// discoveryLocated mirrors testvalidity.ExampleSourceLocated for a placed file or example.
const discoveryLocated = testvalidity.ExampleSourceLocated

// discoveryFile is the per-file producer state.
type discoveryFile struct {
	path     string
	text     string
	index    *doccorpus.LineIndex
	out      *TestDiscovery
	resolver *discoveryResolver
	err      error
}

// anchor is the source anchor of lines start..end. A span the compiler would refuse (over the
// excerpt bound) is narrowed to its first line, and says so in its reason; a vacuous span yields
// false, and callers then record an unknown instead of evidence.
func (f *discoveryFile) anchor(start, end int, reason string) (doccorpus.Anchor, bool) {
	if end < start {
		end = start
	}
	if end > f.index.Lines() {
		end = f.index.Lines()
	}
	if a, err := f.index.Anchor(start, end, "source", reason); err == nil {
		return a, true
	}
	if a, err := f.index.Anchor(start, start, "source", reason+" (first line only: the whole span exceeds the excerpt bound)"); err == nil && end > start {
		return a, true
	}
	return doccorpus.Anchor{}, false
}

// unknown records one unclassified reading, anchored when lines are given.
func (f *discoveryFile) unknown(example, reason, detail string, start, end int) {
	u := DiscoveryUnknown{Example: example, Path: f.path, Reason: reason, Detail: boundText(detail)}
	if start > 0 {
		if a, ok := f.anchor(start, end, "static test discovery "+reason); ok {
			u.Evidence = &a
		}
	}
	f.out.Unknown = append(f.out.Unknown, u)
}

// newExample starts one example record anchored at lines start..end.
func (f *discoveryFile) newExample(line, end int) (DiscoveredExample, bool) {
	id := fmt.Sprintf("%s:%d", f.path, line)
	a, ok := f.anchor(line, end, "static test discovery example")
	if !ok {
		f.unknown(id, "example-span-unanchored", "the example's lines are empty or exceed the excerpt bound", 0, 0)
		return DiscoveredExample{}, false
	}
	return DiscoveredExample{ID: id, Path: f.path, Line: line, TitlePath: []DiscoveredTitle{}, Tags: []DiscoveredTag{}, Annotations: []DiscoveredAnnotation{},
		Expectations: []DiscoveredExpectation{}, Navigations: []DiscoveredNavigation{}, Reaches: []DiscoveredReach{}, Hooks: []DiscoveredHook{},
		Read: discoveryLocated, Review: "unreviewed", Evidence: a}, true
}

// capFacts drops every fact whose source span could not be anchored, then keeps at most
// maxDiscoveryFacts facts of each kind on e, recording one unknown for each list it changes.
func (f *discoveryFile) capFacts(e *DiscoveredExample) {
	f.dropUnanchored(e)
	cut := func(n int, kind string) bool {
		if n <= maxDiscoveryFacts {
			return false
		}
		f.unknown(e.ID, discoverReasonFactBound, fmt.Sprintf("%d %s on the example; the first %d are kept", n, kind, maxDiscoveryFacts), 0, 0)
		return true
	}
	if cut(len(e.Tags), "tags") {
		e.Tags = e.Tags[:maxDiscoveryFacts]
	}
	if cut(len(e.Annotations), "annotations") {
		e.Annotations = e.Annotations[:maxDiscoveryFacts]
	}
	if cut(len(e.Expectations), "expectations") {
		e.Expectations = e.Expectations[:maxDiscoveryFacts]
	}
	if cut(len(e.Navigations), "navigations") {
		e.Navigations = e.Navigations[:maxDiscoveryFacts]
	}
	if cut(len(e.Reaches), "reaches") {
		e.Reaches = e.Reaches[:maxDiscoveryFacts]
	}
	if cut(len(e.Hooks), "hooks") {
		e.Hooks = e.Hooks[:maxDiscoveryFacts]
	}
}

// dropUnanchored removes facts whose anchor failed (a span that is empty or over the excerpt
// bound), so no fact is reported with zero-valued evidence: each kind it cuts is one unknown, and
// an unanchored title empties the title path (DCX-V1-370, DCX-V1-376).
func (f *discoveryFile) dropUnanchored(e *DiscoveredExample) {
	note := func(n int, kind string) {
		if n > 0 {
			f.unknown(e.ID, discoverReasonFactUnanchored, fmt.Sprintf("%d %s on the example could not be anchored to the source and are not reported", n, kind), 0, 0)
		}
	}
	unanchored := func(a doccorpus.Anchor) bool { return a == (doccorpus.Anchor{}) }
	if n := countFunc(e.TitlePath, func(t DiscoveredTitle) bool { return unanchored(t.Evidence) }); n > 0 {
		note(n, "titles")
		e.TitlePath = []DiscoveredTitle{}
		if e.TitleUnknown == "" {
			e.TitleUnknown = discoverReasonFactUnanchored
		}
	}
	var n int
	n, e.Tags = dropFunc(e.Tags, func(t DiscoveredTag) bool { return unanchored(t.Evidence) })
	note(n, "tags")
	n, e.Annotations = dropFunc(e.Annotations, func(a DiscoveredAnnotation) bool { return unanchored(a.Evidence) })
	note(n, "annotations")
	n, e.Expectations = dropFunc(e.Expectations, func(x DiscoveredExpectation) bool { return unanchored(x.Evidence) })
	note(n, "expectations")
	n, e.Navigations = dropFunc(e.Navigations, func(x DiscoveredNavigation) bool { return unanchored(x.Evidence) })
	note(n, "navigations")
	n, e.Reaches = dropFunc(e.Reaches, func(x DiscoveredReach) bool { return unanchored(x.Evidence) })
	note(n, "reaches")
	n, e.Hooks = dropFunc(e.Hooks, func(x DiscoveredHook) bool { return unanchored(x.Evidence) })
	note(n, "hooks")
	declarations := 0
	for i := range e.Reaches {
		if d := e.Reaches[i].Declaration; d != nil && unanchored(*d) {
			e.Reaches[i].Declaration = nil
			declarations++
		}
	}
	note(declarations, "reach declarations")
}

func countFunc[T any](facts []T, drop func(T) bool) int {
	n := 0
	for _, x := range facts {
		if drop(x) {
			n++
		}
	}
	return n
}

// dropFunc returns how many facts drop selects and the facts it keeps, in order.
func dropFunc[T any](facts []T, drop func(T) bool) (int, []T) {
	n := countFunc(facts, drop)
	if n == 0 {
		return 0, facts
	}
	return n, slices.DeleteFunc(slices.Clone(facts), drop)
}

func boundText(text string) string {
	if len(text) <= maxDiscoveryText {
		return text
	}
	cut := maxDiscoveryText
	for cut > 0 && !utf8Start(text[cut]) {
		cut--
	}
	return text[:cut] + "…"
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// discoveryResolver resolves import specifiers against the tracked source paths at the revision.
type discoveryResolver struct {
	tracked  map[string]bool
	overflow bool
}

var discoveryWebExtensions = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx", ".mts", ".cts", ".mjs", ".cjs"}

// web resolves a JavaScript or TypeScript specifier imported by file: the target path and the
// state (repository, package, builtin or unresolved) with the reason when unresolved.
func (d *discoveryResolver) web(file, spec string) (string, string, string) {
	switch {
	case strings.HasPrefix(spec, "node:") || nodeBuiltins[spec]:
		return "", "builtin", ""
	case spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../"):
	case strings.HasPrefix(spec, "/") || strings.HasPrefix(spec, "~") || strings.HasPrefix(spec, "#") || strings.HasPrefix(spec, "@/"):
		return "", "unresolved", discoverReasonImportUnresolved
	default:
		return "", "package", ""
	}
	joined := path.Join(path.Dir(file), spec)
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return "", "unresolved", discoverReasonImportUnresolved
	}
	candidates := []string{joined}
	ext := path.Ext(joined)
	stem := strings.TrimSuffix(joined, ext)
	switch ext {
	case ".js", ".jsx":
		candidates = append(candidates, stem+".ts", stem+".tsx")
	case ".mjs":
		candidates = append(candidates, stem+".mts")
	case ".cjs":
		candidates = append(candidates, stem+".cts")
	}
	for _, x := range discoveryWebExtensions {
		candidates = append(candidates, joined+x)
	}
	for _, x := range discoveryWebExtensions {
		candidates = append(candidates, joined+"/index"+x)
	}
	for _, c := range candidates {
		if d.tracked[c] {
			return c, "repository", ""
		}
	}
	if d.overflow {
		return "", "unresolved", discoverReasonResolveBound
	}
	return "", "unresolved", discoverReasonImportUnresolved
}

// ruby resolves a require_relative path from file.
func (d *discoveryResolver) ruby(file, spec string) (string, string, string) {
	joined := path.Join(path.Dir(file), spec)
	if !strings.HasSuffix(joined, ".rb") {
		joined += ".rb"
	}
	if d.tracked[joined] {
		return joined, "repository", ""
	}
	if d.overflow {
		return "", "unresolved", discoverReasonResolveBound
	}
	return "", "unresolved", discoverReasonImportUnresolved
}

var nodeBuiltins = map[string]bool{"assert": true, "buffer": true, "child_process": true, "crypto": true, "events": true, "fs": true,
	"fs/promises": true, "http": true, "https": true, "net": true, "os": true, "path": true, "process": true, "stream": true,
	"timers": true, "url": true, "util": true, "zlib": true}
