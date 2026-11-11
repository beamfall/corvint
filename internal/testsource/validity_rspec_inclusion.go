package testsource

import (
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/testdiscovery"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Shared examples read through one inclusion site (TSA-V0-089..092). A qualified reference names
// the including spec's include line and the shared example's line; the reader splices the shared
// group's body into the including group, exactly where RSpec evaluates it, and reads the example
// there with the ordinary rules. Anything that would make the splice a guess refuses with a named
// reason instead.

// RSpecQualifiedRef is a qualified reference: `SPEC_rb:INCLUDE_LINE>SHARED_rb:EXAMPLE_LINE`, with
// `same` for a shared group defined in the including spec.
var RSpecQualifiedRef = regexp.MustCompile(`^(.+_spec\.rb):([1-9][0-9]{0,8})>(same|.+\.rb):([1-9][0-9]{0,8})$`)

// rspecItem is testdiscovery.RSpecItem, kept under its old name.
type rspecItem = testdiscovery.RSpecItem

// inclusionReading is the context of one qualified read.
type inclusionReading struct {
	q       testvalidity.RSpecInclusion
	spec    *rspecReader
	shared  *rspecReader
	support *rspecSupport
	// cause is the support file behind a rspec-shared-group-unbound refusal (TSA-V0-397).
	cause string
}

// RSpecInclusionSource reads the shared example q names through its one inclusion site and returns
// its source facts, with every finding's Test the qualified reference; Read is the refusal reason
// when the reader cannot bind the inclusion (TSA-V0-089..092).
func RSpecInclusionSource(q testvalidity.RSpecInclusion) (testvalidity.ExampleSource, string) {
	x := &inclusionReading{q: q, spec: newRSpecReader(q.SpecPath, q.SpecText), support: newRSpecSupport(q.Support)}
	refuse := func(reason string) (testvalidity.ExampleSource, string) {
		return testvalidity.ExampleSource{Read: reason, Findings: []testvalidity.SourceFinding{}, Cause: x.cause}, q.Ref(q.IncludeLine)
	}
	if reason := x.spec.Read(); reason != "" {
		return refuse(reason)
	}
	x.shared = x.spec
	if q.SharedPath != q.SpecPath {
		x.shared = newRSpecReader(q.SharedPath, q.SharedText)
		if reason := x.shared.Read(); reason != "" {
			return refuse(reason)
		}
	}
	inc := -1
	if q.ScopedID != nil {
		k, reason := x.placeScoped()
		if reason != "" {
			return refuse(reason)
		}
		inc = k
	} else {
		for k, candidate := range x.spec.Includes {
			if candidate.Line == q.IncludeLine {
				inc = k
			}
		}
		if inc < 0 {
			return refuse(x.missingInclude())
		}
	}
	include := x.spec.Includes[inc]
	q.IncludeLine = include.Line
	ref := q.Ref(include.Line)
	def, reason := x.definition(include)
	if reason != "" {
		return testvalidity.ExampleSource{Read: reason, Findings: []testvalidity.SourceFinding{}, Cause: x.cause}, ref
	}
	if reason := x.exampleInGroup(def); reason != "" {
		return testvalidity.ExampleSource{Read: reason, Findings: []testvalidity.SourceFinding{}}, ref
	}
	// A receipt-based read needs the receipt's description to cross-check the placement.
	if q.ScopedID != nil && q.Description == "" || q.Description != "" && !describesInclusion(q.Description, include) {
		return testvalidity.ExampleSource{Read: testvalidity.InclusionDescriptionMismatch, Findings: []testvalidity.SourceFinding{}}, ref
	}
	merged, at := x.splice(include, def)
	if reason := merged.Read(); reason != "" {
		return testvalidity.ExampleSource{Read: reason, Findings: []testvalidity.SourceFinding{}}, ref
	}
	merged.Support = x.support
	merged.Excluded = rspecExclusions(q.Support.Files)
	// The it_behaves_like head was replaced by its body; it no longer names a group to resolve.
	merged.Includes = slices.DeleteFunc(merged.Includes, func(m rspecInclude) bool { return m.Index == include.Index })
	for _, e := range merged.Examples {
		if e.Index < at[0] || e.Index >= at[1] || e.Path != q.SharedPath || e.Line != q.ExampleLine {
			continue
		}
		findings := append(memoizedFindings(merged, e), blockNotCalledFor(merged, e)...)
		findings = append(findings, exampleFindings(merged, ref, e)...)
		for k := range findings {
			findings[k].Test = ref
		}
		count, anchors := expectations(e.Path, e.Body)
		return testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: count, Findings: findings, ExpectationAnchors: anchors}, ref
	}
	return testvalidity.ExampleSource{Read: testvalidity.InclusionExampleNotInGroup, Findings: []testvalidity.SourceFinding{}}, ref
}

// missingInclude names why no inclusion was read at the requested line: one inside an example or
// a hook body, which the reader does not descend into, or none at all.
func (x *inclusionReading) missingInclude() string {
	for _, l := range x.spec.Lines {
		if l.Line == x.q.IncludeLine && rspecIncludeLine.MatchString(l.Text) {
			return testvalidity.InclusionInExampleOrHook
		}
	}
	return testvalidity.InclusionNotFound
}

// definition resolves the shared group an inclusion names to exactly one literal definition among
// the including spec, the named shared file and the support set (TSA-V0-091). A support file that
// did not read and could define the group is kept as x.cause (TSA-V0-397).
func (x *inclusionReading) definition(include rspecInclude) (sharedDefinition, string) {
	switch {
	case include.Loop:
		return sharedDefinition{}, testvalidity.InclusionLoopGenerated
	case !include.Literal:
		return sharedDefinition{}, testvalidity.InclusionNameNotLiteral
	}
	readers := []*rspecReader{x.spec}
	if x.shared != x.spec {
		readers = append(readers, x.shared)
	}
	for _, path := range sortedSupport(x.support) {
		if path != x.q.SpecPath && path != x.q.SharedPath {
			readers = append(readers, x.support.Readers[path])
		}
	}
	defs, unsure := sharedDefinitions(readers, include.Name)
	visible := []sharedDefinition{}
	for _, d := range defs {
		// A shared group defined inside an example group is visible only within that group; one
		// defined inside another shared group is kept so it refuses as nested below.
		if p := d.reader.Groups[d.group].Parent; p != 0 && !nestedInShared(d.reader, d.group) && !(d.reader == x.spec && rspecReaderWithin(x.spec, include.Group, p)) {
			continue
		}
		visible = append(visible, d)
	}
	gap := sharedGap(x.support, include.Name, x.q.SpecPath, x.q.SharedPath)
	switch {
	case len(visible) > 1 || unsure:
		return sharedDefinition{}, testvalidity.SharedGroupAmbiguous
	case x.support.Unbound != "":
		return sharedDefinition{}, testvalidity.SharedGroupUnbound
	case gap != "":
		// Another definition could sit in a support file the reader could not read.
		x.cause = gap
		return sharedDefinition{}, testvalidity.SharedGroupUnbound
	case len(visible) == 0:
		return sharedDefinition{}, testvalidity.SharedGroupNotFound
	}
	d := visible[0]
	if nestedInShared(d.reader, d.group) {
		return sharedDefinition{}, testvalidity.SharedGroupNested
	}
	for _, inc := range d.reader.Includes {
		if rspecReaderWithin(d.reader, inc.Group, d.group) {
			return sharedDefinition{}, testvalidity.SharedGroupNested
		}
	}
	return d, ""
}

// nestedInShared reports whether group g sits inside another shared group definition.
func nestedInShared(r *rspecReader, g int) bool {
	for g = r.Groups[g].Parent; g > 0; g = r.Groups[g].Parent {
		if sharedDefinitionWords[declarator(r.Groups[g].Head)] {
			return true
		}
	}
	return false
}

func sortedSupport(s *rspecSupport) []string {
	paths := make([]string, 0, len(s.Readers))
	for path := range s.Readers {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

// within reports whether group g is group ancestor or inside it.
func rspecReaderWithin(r *rspecReader, g, ancestor int) bool {
	for ; g >= 0; g = r.Groups[g].Parent {
		if g == ancestor {
			return true
		}
	}
	return false
}

// exampleInGroup checks that the named example is declared in the definition, outside any loop.
func (x *inclusionReading) exampleInGroup(d sharedDefinition) string {
	if d.reader != x.shared {
		return testvalidity.InclusionExampleNotInGroup
	}
	for _, e := range x.shared.Examples {
		if e.Line != x.q.ExampleLine {
			continue
		}
		if !rspecReaderWithin(x.shared, e.Group, d.group) {
			return testvalidity.InclusionExampleNotInGroup
		}
		if e.Loop {
			return testvalidity.InclusionExampleLoop
		}
		for g := e.Group; g != d.group; g = x.shared.Groups[g].Parent {
			if x.shared.Groups[g].Loop {
				return testvalidity.InclusionExampleLoop
			}
		}
		return ""
	}
	return testvalidity.InclusionExampleNotInGroup
}

// splice returns a reader over the including spec with the shared group's body placed where RSpec
// evaluates it, and the [start, end) line indexes of the spliced body. it_behaves_like nests the
// body in a group, before its customization block; include_examples and include_context place it
// inline, before theirs.
func (x *inclusionReading) splice(include rspecInclude, d sharedDefinition) (*rspecReader, [2]int) {
	head := d.reader.Groups[d.group].Index
	body := d.reader.Lines[head+1 : rubyBlockEnd(d.reader.Lines, head)]
	lines := x.spec.Lines
	i := include.Index
	out := append([]rubyLine{}, lines[:i]...)
	var at [2]int
	nesting := include.Nests()
	switch {
	case nesting && include.Nested >= 0:
		out = append(out, lines[i])
		at[0] = len(out)
		out = append(out, body...)
		at[1] = len(out)
		out = append(out, lines[i+1:]...)
	case nesting:
		out = append(out, rubyLine{Line: lines[i].Line, Path: lines[i].Path, Text: "describe do"})
		at[0] = len(out)
		out = append(out, body...)
		at[1] = len(out)
		out = append(out, rubyLine{Line: lines[i].Line, Path: lines[i].Path, Text: "end"})
		out = append(out, lines[i+1:]...)
	default:
		at[0] = len(out)
		out = append(out, body...)
		at[1] = len(out)
		if rubyDo.MatchString(lines[i].Text) {
			end := rubyBlockEnd(lines, i)
			out = append(out, lines[i+1:end]...)
			i = end
		}
		out = append(out, lines[i+1:]...)
	}
	return &rspecReader{Path: x.spec.Path, Lines: out, Groups: []rspecGroup{{Parent: -1, Decls: map[string]string{}, Path: x.spec.Path}}}, at
}

// describesInclusion cross-checks a receipt's full description against an it_behaves_like or
// it_should_behave_like inclusion's generated group description (TSA-V0-092).
func describesInclusion(description string, include rspecInclude) bool {
	switch include.Kind {
	case "it_behaves_like":
		return strings.Contains(description, "behaves like "+include.Name)
	case "it_should_behave_like":
		return strings.Contains(description, "it should behave like "+include.Name)
	}
	return true
}

// placeScoped walks the receipt's scoped id path through the including spec, expanding inline
// inclusions and it_behaves_like groups as RSpec numbers them, to the inclusion the example ran
// through, and checks that the rest of the path lands on the named shared example (TSA-V0-092).
func (x *inclusionReading) placeScoped() (int, string) {
	ids := x.q.ScopedID
	if len(ids) < 2 {
		return -1, testvalidity.InclusionIDUnplaced
	}
	top := []rspecItem{}
	for _, item := range x.spec.Children(0) {
		if item.Group >= 0 || item.Include >= 0 && x.spec.Includes[item.Include].Nests() {
			top = append(top, item)
		}
	}
	if ids[0] > len(top) {
		return -1, testvalidity.InclusionIDUnplaced
	}
	item := top[ids[0]-1]
	for k := 1; ; k++ {
		if item.Include >= 0 { // an it_behaves_like group: the rest walks its shared body
			return x.landShared(item.Include, ids[k:])
		}
		if item.Group < 0 || k == len(ids) {
			return -1, testvalidity.InclusionIDUnplaced
		}
		next, offset, reason := x.slot(x.spec.Children(item.Group), ids[k])
		if reason != "" {
			return -1, reason
		}
		if next.Include >= 0 && !x.spec.Includes[next.Include].Nests() {
			// An inline inclusion: the slot's offset inside the shared body replaces this id.
			return x.landShared(next.Include, append([]int{offset}, ids[k+1:]...))
		}
		item = next
	}
}

// slot picks the n-th child of a sibling list, expanding each inline inclusion to the size of its
// shared body, and returns the offset inside that body when the slot falls in one.
func (x *inclusionReading) slot(kids []rspecItem, n int) (rspecItem, int, string) {
	for _, kid := range kids {
		if kid.Loop {
			return rspecItem{}, 0, testvalidity.InclusionLoopGenerated
		}
		size := 1
		if kid.Include >= 0 && !x.spec.Includes[kid.Include].Nests() {
			d, reason := x.definition(x.spec.Includes[kid.Include])
			if reason != "" {
				return rspecItem{}, 0, reason
			}
			size = len(d.reader.Children(d.group))
		}
		if n <= size {
			return kid, n, ""
		}
		n -= size
	}
	return rspecItem{}, 0, testvalidity.InclusionIDUnplaced
}

// landShared walks the rest of the id path through the shared definition the inclusion names and
// reports the inclusion when it lands on the named example.
func (x *inclusionReading) landShared(inc int, rest []int) (int, string) {
	d, reason := x.definition(x.spec.Includes[inc])
	if reason != "" {
		return -1, reason
	}
	g := d.group
	for k, n := range rest {
		kids := d.reader.Children(g)
		if n > len(kids) {
			return -1, testvalidity.InclusionIDUnplaced // the customization block's own children
		}
		kid := kids[n-1]
		if kid.Loop {
			return -1, testvalidity.InclusionExampleLoop
		}
		switch {
		case kid.Example >= 0 && k == len(rest)-1:
			e := d.reader.Examples[kid.Example]
			if d.reader != x.shared || e.Line != x.q.ExampleLine {
				return -1, testvalidity.InclusionIDUnplaced
			}
			return inc, ""
		case kid.Group >= 0:
			g = kid.Group
		default:
			return -1, testvalidity.InclusionIDUnplaced
		}
	}
	return -1, testvalidity.InclusionIDUnplaced
}
