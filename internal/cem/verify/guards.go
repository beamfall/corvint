package verify

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// GuardingTestsProfile names the producer's guarding-test search (TCQ-V0-062).
// It is producer guidance reported beside the stable result: it never sets a
// support tier, and a hunk it finds guarded is held only when a consumer later
// re-executes the cited test itself (CEM-V1-088).
const GuardingTestsProfile = "cem-guarding-tests/0"

// Guarding-test results. Guarded names at least one base test the producer
// observed pass at the target and fail with only this hunk reverted.
const (
	GuardGuarded   = "guarded"
	GuardUnguarded = "unguarded"
	GuardNotRun    = "not-run"
)

// GuardNoCandidate is the not-run reason of a hunk whose package has no base
// test that references a declaration the hunk changes, directly or through the
// package's other declarations. Every other reason is a CEM-V1-089 reason.
const GuardNoCandidate = "no-candidate-test"

// GuardingTests is the guarding-test report. Plan holds one ready citation-plan
// row per guarded hunk whose guard can be cited stably:
// HUNK_ID<TAB>PATH<TAB>START:END<TAB>test-claim (TCQ-V0-060).
type GuardingTests struct {
	Profile        string     `json:"profile"`
	Runner         string     `json:"runner"`
	PerRunSeconds  int64      `json:"perRunSeconds"`
	TotalSeconds   int64      `json:"totalSeconds"`
	Hunks          []GuardRow `json:"hunks"`
	Guarded        int        `json:"guarded"`
	Unguarded      int        `json:"unguarded"`
	NotRun         int        `json:"notRun"`
	Plan           []string   `json:"plan"`
	ElapsedSeconds float64    `json:"elapsedSeconds"`
}

// GuardRow is one non-mechanical hunk of non-test Go source that cites no
// _test.go span as test-claim yet. Candidates are the base tests of the hunk's
// package that reference what it changes; Guards are the candidates observed
// failing with the hunk reverted; Citation is the plan row's span, or null.
type GuardRow struct {
	HunkID     string         `json:"hunkId"`
	Path       string         `json:"path"`
	Candidates []string       `json:"candidates"`
	Result     string         `json:"result"`
	Reason     string         `json:"reason"`
	Guards     []string       `json:"guards"`
	Citation   *GuardCitation `json:"citation"`
}

// GuardCitation is a base test file and the one-based base lines holding the
// guard's declaration header, from func through its parameter list.
type GuardCitation struct {
	Path  string `json:"path"`
	Lines string `json:"lines"`
	Test  string `json:"test"`
}

// StableWithGuardingTests runs Stable and, only when it accepts, the
// guarding-test search against the accepted target. The result and exit are
// exactly what Stable returns; the report is nil unless the result accepted.
func StableWithGuardingTests(ctx context.Context, raw []byte, o StableOptions, limits HunkRevertLimits) (StableResult, int, *GuardingTests) {
	capture := &stableCapture{}
	o.capture = capture
	result, exit := Stable(ctx, raw, o)
	if result.Outcome != "ACCEPT" || capture.change == nil {
		return result, exit, nil
	}
	return result, exit, runGuardingTests(ctx, o.Repository, capture, limits, goTestRunner(o.Repository))
}

// guardHunk is one hunk the search judges.
type guardHunk struct {
	mapped wire.Hunk
	hunk   *patch.Hunk
	kind   patch.GroupKind
}

func runGuardingTests(ctx context.Context, repository string, c *stableCapture, limits HunkRevertLimits, run testRunner) *GuardingTests {
	started := time.Now()
	report := &GuardingTests{
		Profile: GuardingTestsProfile, Runner: HunkRevertRunner,
		PerRunSeconds: int64(limits.PerRun / time.Second), TotalSeconds: int64(limits.Total / time.Second),
		Hunks: []GuardRow{}, Plan: []string{},
	}
	defer func() { report.ElapsedSeconds = time.Since(started).Round(time.Millisecond).Seconds() }()
	hunks := guardHunks(c)
	if len(hunks) == 0 {
		return report
	}
	total, cancel := context.WithTimeout(ctx, limits.Total)
	defer cancel()
	rows := make([]GuardRow, len(hunks))
	for i, h := range hunks {
		rows[i] = GuardRow{HunkID: h.mapped.ID, Path: h.mapped.Path, Candidates: []string{}, Result: GuardNotRun, Guards: []string{}}
	}
	workspace, err := disposableWorkspace(total, repository, "corvint-guarding-tests-")
	if err == nil {
		// The disposable copy is removed on every exit path; the caller's
		// repository is only read (product invariant 4).
		defer os.RemoveAll(workspace)
		tree := filepath.Join(workspace, "tree")
		if err = exportTree(total, repository, c.target, tree, filepath.Join(workspace, "tmp")); err == nil {
			loadBase := func(name string) []byte {
				data, _ := gitOutput(total, repository, nil, "cat-file", "blob", c.change.BaseRevision+":"+name)
				return data
			}
			judgeGuards(total, tree, hunks, rows, limits, run, loadBase)
		}
	}
	for i := range rows {
		if err != nil {
			rows[i].Reason = RevertInfrastructure
			if total.Err() != nil {
				rows[i].Reason = RevertTotalLimit
			}
		}
		switch rows[i].Result {
		case GuardGuarded:
			report.Guarded++
			if cite := rows[i].Citation; cite != nil {
				report.Plan = append(report.Plan, rows[i].HunkID+"\t"+cite.Path+"\t"+cite.Lines+"\ttest-claim")
			}
		case GuardUnguarded:
			report.Unguarded++
		default:
			report.NotRun++
		}
	}
	report.Hunks = rows
	return report
}

// guardHunks selects, in map order, every non-mechanical hunk of non-test Go
// source that does not already cite a _test.go span as test-claim; a hunk that
// does is a hunk-revert candidate already.
func guardHunks(c *stableCapture) []guardHunk {
	derived := map[string]*patch.Hunk{}
	kinds := map[string]patch.GroupKind{}
	for _, group := range c.parsed.Groups {
		for _, hunk := range group.Hunks {
			derived[hunk.ID] = hunk
			kinds[hunk.ID] = group.Kind
		}
	}
	paths := map[string]string{}
	for _, record := range c.change.Evidence {
		paths[record.ID] = record.Path
	}
	var hunks []guardHunk
	for _, mapped := range c.change.Hunks {
		hunk := derived[mapped.ID]
		if mapped.Disposition == "mechanical" || hunk == nil || !goSource(mapped.Path) {
			continue
		}
		cites := false
		for _, basis := range mapped.Basis {
			cites = cites || basis.Relation == "test-claim" && strings.HasSuffix(paths[basis.EvidenceID], "_test.go")
		}
		if !cites {
			hunks = append(hunks, guardHunk{mapped: mapped, hunk: hunk, kind: kinds[mapped.ID]})
		}
	}
	return hunks
}

// judgeGuards names each hunk's candidate tests from the copy and judges them
// with the falsifier's own revert-and-run classification, so a guard is a test
// that passed at the target and failed with only that hunk reverted; a broken
// build, a test failing without the revert, a skip or a timeout guards nothing.
func judgeGuards(total context.Context, tree string, hunks []guardHunk, rows []GuardRow, limits HunkRevertLimits, run testRunner, loadBase func(string) []byte) {
	packages := map[string]*goPackage{}
	load := func(dir string) *goPackage {
		if packages[dir] == nil {
			packages[dir] = loadGoPackage(tree, dir, loadBase)
		}
		return packages[dir]
	}
	var importers *importIndex
	candidates := make([]revertCandidate, len(hunks))
	for i, h := range hunks {
		candidate := revertCandidate{hunk: h.hunk, kind: h.kind, tests: []string{}}
		if h.kind == patch.KindModify && h.hunk.NewPath != nil {
			dir := path.Dir(*h.hunk.NewPath)
			reached := load(dir).closure(*h.hunk.NewPath, changedLines(h.hunk))
			if names := load(dir).testsMentioning(reached); len(names) > 0 {
				candidate.units = []testUnit{{dir: dir, names: names}}
			} else if exported := exportedNames(reached); len(exported) > 0 {
				// No test of the hunk's own directory reaches it, so the
				// tests of its direct importers that reach an exported name
				// it changes are the candidates (TCQ-V0-761).
				if importers == nil {
					importers = indexImporters(tree)
				}
				for _, importer := range importers.of(dir) {
					if names := load(importer).testsMentioning(load(importer).grow(exported)); len(names) > 0 {
						candidate.units = append(candidate.units, testUnit{dir: importer, names: names})
					}
				}
			}
			for _, unit := range candidate.units {
				for _, name := range unit.names {
					candidate.tests = append(candidate.tests, unit.dir+"::"+name)
				}
			}
		}
		candidates[i] = candidate
		rows[i].Candidates = candidate.tests
	}
	// A base read cut short by the total bound reads as an absent test, so
	// no candidate is trusted only when naming finished within the bound.
	named := total.Err() == nil
	verdicts := make([]HunkRevertRow, len(candidates))
	for i := range verdicts {
		verdicts[i] = HunkRevertRow{Result: RevertNotRun, Failing: []string{}}
	}
	judgeCandidates(total, tree, candidates, verdicts, limits, run)
	for i, verdict := range verdicts {
		row := &rows[i]
		switch verdict.Result {
		case RevertHeld:
			row.Result, row.Guards = GuardGuarded, verdict.Failing
			if units := candidates[i].units; len(units) == 1 {
				row.Citation = packages[units[0].dir].citation(tree, verdict.Failing)
			}
		case RevertSurvived:
			row.Result = GuardUnguarded
		default:
			// A modification with no candidate would run nothing, so a
			// budget spent after naming is not why it guards nothing.
			row.Reason = verdict.Reason
			if row.Reason == RevertNoNamedTest || named && candidates[i].kind == patch.KindModify && len(candidates[i].tests) == 0 {
				row.Reason = GuardNoCandidate
			}
		}
	}
}

// changedLines returns the one-based target lines a hunk adds, plus the line
// on each side of a pure deletion, so the declaration it edits is found. A
// replacement marks only its added lines, never an untouched neighbor.
func changedLines(hunk *patch.Hunk) map[int64]bool {
	changed := map[int64]bool{}
	line := hunk.NewRange.Start
	if hunk.NewRange.Count == 0 {
		line++
	}
	for i := 0; i < len(hunk.Body); {
		if hunk.Body[i].Prefix != '+' && hunk.Body[i].Prefix != '-' {
			line++
			i++
			continue
		}
		// One run of consecutive removed and added lines.
		added := false
		for ; i < len(hunk.Body) && (hunk.Body[i].Prefix == '+' || hunk.Body[i].Prefix == '-'); i++ {
			if hunk.Body[i].Prefix == '+' {
				changed[line], added = true, true
				line++
			}
		}
		if !added {
			changed[line-1], changed[line] = true, true
		}
	}
	return changed
}

// goDecl is one top-level declaration: the names it declares, the identifiers
// it mentions, and its target line range.
type goDecl struct {
	file       string
	names      []string
	refs       map[string]bool
	start, end int64
	test       bool // a func TestX(t *testing.T) declared in a _test.go file and at the base
}

// goPackage is one directory of the copy: its declarations and, per test
// file, the base blob that a citation must name.
type goPackage struct {
	decls []goDecl
	base  map[string][]byte // test file path -> base blob, nil when absent at base
}

func loadGoPackage(tree, dir string, loadBase func(string) []byte) *goPackage {
	p := &goPackage{base: map[string][]byte{}}
	entries, err := os.ReadDir(filepath.Join(tree, filepath.FromSlash(dir)))
	if err != nil {
		return p
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasSuffix(name, ".go") {
			continue
		}
		file := path.Join(dir, name)
		data, _, err := readCopyFile(tree, filepath.Join(tree, filepath.FromSlash(file)))
		if err != nil {
			continue
		}
		isTest := strings.HasSuffix(name, "_test.go")
		decls := parseDecls(file, data, isTest)
		if isTest {
			p.base[file] = loadBase(file)
			baseTests := map[string]bool{}
			for _, test := range declaredTests(p.base[file], wire.Span{Start: 0, End: int64(len(p.base[file]))}) {
				baseTests[test] = true
			}
			for i := range decls {
				// Only a test declared at the base can be cited (cem cite
				// requires base evidence), so only such a test is a candidate.
				decls[i].test = decls[i].test && baseTests[decls[i].names[0]]
			}
		}
		p.decls = append(p.decls, decls...)
	}
	return p
}

// parseDecls lists a file's top-level declarations other than imports. An
// unparsable file declares nothing.
func parseDecls(file string, data []byte, isTest bool) []goDecl {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "", data, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var decls []goDecl
	for _, decl := range parsed.Decls {
		d := goDecl{
			file: file, refs: map[string]bool{},
			// Physical lines: a //line directive must not move a declaration
			// away from the patch lines that edit it.
			start: int64(fileSet.PositionFor(decl.Pos(), false).Line), end: int64(fileSet.PositionFor(decl.End(), false).Line),
		}
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			d.names = []string{decl.Name.Name}
			d.test = isTest && goTestDecl(decl)
		case *ast.GenDecl:
			if decl.Tok == token.IMPORT {
				continue
			}
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					d.names = append(d.names, spec.Name.Name)
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						// The blank identifier declares nothing a test can mention.
						if name.Name != "_" {
							d.names = append(d.names, name.Name)
						}
					}
				}
			}
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok {
				d.refs[ident.Name] = true
			}
			return true
		})
		decls = append(decls, d)
	}
	return decls
}

// candidates names, sorted, the base tests of the package whose bodies mention
// a declaration the changed lines of file touch, directly or through the
// package's other declarations (test helpers and other tests included).
// Matching is by identifier name, so it over-approximates; the revert run
// decides.
func (p *goPackage) candidates(file string, changed map[int64]bool) []string {
	return p.testsMentioning(p.closure(file, changed))
}

// closure returns the names of the declarations the changed lines of file
// touch and of every package declaration that transitively mentions one.
func (p *goPackage) closure(file string, changed map[int64]bool) map[string]bool {
	reached := map[string]bool{}
	for _, d := range p.decls {
		if d.file != file || d.test {
			continue
		}
		for line := d.start; line <= d.end; line++ {
			if changed[line] {
				for _, name := range d.names {
					reached[name] = true
				}
				break
			}
		}
	}
	return p.grow(reached)
}

// grow adds to a copy of seed the names of every package declaration that
// transitively mentions one of its names.
func (p *goPackage) grow(seed map[string]bool) map[string]bool {
	reached := make(map[string]bool, len(seed))
	for name := range seed {
		reached[name] = true
	}
	for grew := len(reached) > 0; grew; {
		grew = false
		for _, d := range p.decls {
			if !mentions(d.refs, reached) {
				continue
			}
			for _, name := range d.names {
				if !reached[name] {
					reached[name], grew = true, true
				}
			}
		}
	}
	return reached
}

// testsMentioning names, sorted, the base tests of the package that mention
// one of reached.
func (p *goPackage) testsMentioning(reached map[string]bool) []string {
	seen := map[string]bool{}
	var names []string
	for _, d := range p.decls {
		if d.test && !seen[d.names[0]] && mentions(d.refs, reached) {
			seen[d.names[0]] = true
			names = append(names, d.names[0])
		}
	}
	sort.Strings(names)
	return names
}

func mentions(refs, names map[string]bool) bool {
	for name := range refs {
		if names[name] {
			return true
		}
	}
	return false
}

// citation returns the first guard, in name order, whose base declaration
// header lines a cite would admit: present at the base and stable or
// relocated at the target under the frozen drift classifier.
func (p *goPackage) citation(tree string, guards []string) *GuardCitation {
	for _, guard := range guards {
		for _, d := range p.decls {
			if !d.test || d.names[0] != guard {
				continue
			}
			base := p.base[d.file]
			start, end, ok := baseHeaderLines(base, guard)
			if !ok {
				continue
			}
			target, _, err := readCopyFile(tree, filepath.Join(tree, filepath.FromSlash(d.file)))
			if err != nil {
				continue
			}
			lines, err := patch.SplitLF(base)
			if err != nil || end > int64(len(lines)) {
				continue
			}
			var offset int64
			for _, line := range lines[:start-1] {
				offset += int64(len(line))
			}
			needle := bytes.Join(lines[start-1:end], nil)
			span := wire.Span{Start: offset, End: offset + int64(len(needle))}
			if span.End-span.Start > wire.MaxCitationSpanBytes {
				// cem cite would refuse this span, so it is never printed
				// (TCQ-V0-063); a later guard by name may still fit.
				continue
			}
			status, _ := ClassifySpan(true, bytes.Equal(base, target), target, needle, span)
			if status == DriftStable || status == DriftRelocated {
				return &GuardCitation{Path: d.file, Lines: strconv.FormatInt(start, 10) + ":" + strconv.FormatInt(end, 10), Test: guard}
			}
		}
	}
	return nil
}

// baseHeaderLines returns the one-based base lines holding a test's
// declaration header.
func baseHeaderLines(base []byte, name string) (int64, int64, bool) {
	if base == nil {
		return 0, 0, false
	}
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "", base, parser.SkipObjectResolution)
	if err != nil {
		return 0, 0, false
	}
	for _, decl := range parsed.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if ok && goTestDecl(function) && function.Name.Name == name {
			return int64(fileSet.PositionFor(function.Pos(), false).Line), int64(fileSet.PositionFor(function.Type.Params.End(), false).Line), true
		}
	}
	return 0, 0, false
}
