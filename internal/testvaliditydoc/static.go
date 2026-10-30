package testvaliditydoc

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The static mode's vocabulary (LPCV-V0-150, LPCV-V0-151).
const (
	// StaticSource is a static document's source member.
	StaticSource = "corvint-static-test-source"
	// StaticKind is a static document's kind member.
	StaticKind = "static"
	// StaticState is a static test's state: no run observed it.
	StaticState = "not-observed"
	// StateNotObserved is the state of every axis a static source reading cannot observe.
	StateNotObserved = "NOT_OBSERVED"
	// ReasonStaticSourceOnly is the reason of every NOT_OBSERVED or UNSUPPORTED static axis.
	ReasonStaticSourceOnly = "static-source-only"
)

// StaticFile is one committed test file's example readings, as testsource.StaticExamples reads
// them: the examples keyed by declaring line, or the reader's uncertainty reason.
type StaticFile struct {
	Path     string
	Examples map[int]testvalidity.ExampleSource
	Reason   string
}

// Static identifies the committed content a static document read (LPCV-V0-150).
type Static struct {
	Revision string   `json:"revision"`
	Files    []string `json:"files"`
}

// staticBase is the projection a static reading starts from (LPCV-V0-151): association and run
// hygiene are UNSUPPORTED, and freshness, execution and strength are NOT_OBSERVED, because no run
// was observed. Only an example's hygiene is then projected from its source.
func staticBase() testvalidity.Projection {
	unsupported := testvalidity.Axis{State: testvalidity.StateUnsupported, Reason: ReasonStaticSourceOnly, Anchors: []string{}}
	unobserved := testvalidity.Axis{State: StateNotObserved, Reason: ReasonStaticSourceOnly, Anchors: []string{}}
	return testvalidity.Projection{Association: unsupported, Hygiene: unsupported, Freshness: unobserved, Execution: unobserved, Strength: unobserved}
}

// ProjectStatic projects committed test source without a retained run (LPCV-V0-150): one test per
// example the readers placed, keyed `path:line`, whose hygiene is the source rules' reading
// (ApplyExampleSource for RSpec, ApplyJSExampleSource for JS/TS) and whose execution, freshness
// and strength stay NOT_OBSERVED (LPCV-V0-151). A file the reader could not decide is one test
// keyed by its path, hygiene ABSTAINED with the reader's reason. Each test carries its location
// (LPCV-V0-145). The document is a non-promotable preview.
func ProjectStatic(revision string, files []StaticFile) Document {
	tests := []Test{}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
		rspec := strings.HasSuffix(file.Path, "_spec.rb")
		if file.Reason != "" {
			projection := staticBase()
			projection.Hygiene = testvalidity.Axis{State: testvalidity.HygieneAbstained, Reason: file.Reason, Anchors: []string{file.Path}}
			location := newLocation()
			location.setFile(file.Path)
			location.Absent[locationLine] = LocationSourceUnplaced
			location.Absent[locationTitlePath] = LocationSourceUnplaced
			tests = append(tests, Test{ID: file.Path, Name: file.Path, State: StaticState, Projection: projection, Location: location})
			continue
		}
		lines := make([]int, 0, len(file.Examples))
		for line := range file.Examples {
			lines = append(lines, line)
		}
		slices.Sort(lines)
		for _, line := range lines {
			source := file.Examples[line]
			anchor := fmt.Sprintf("%s:%d", file.Path, line)
			projection := staticBase()
			if rspec {
				projection = testvalidity.ApplyExampleSource(projection, source, anchor)
			} else {
				projection = testvalidity.ApplyJSExampleSource(projection, source, anchor)
			}
			location := newLocation()
			location.placeSource(file.Path, line, source)
			name := anchor
			if len(source.TitlePath) > 0 {
				name = source.TitlePath[len(source.TitlePath)-1]
			}
			rules := source
			tests = append(tests, Test{ID: anchor, Name: name, State: StaticState, Projection: projection, Location: location, SourceRules: &rules})
		}
	}
	promotable := false
	return Document{
		Schema:     Schema,
		Source:     StaticSource,
		Kind:       StaticKind,
		Tests:      tests,
		Run:        staticBase(),
		Tier:       "preview",
		Promotable: &promotable,
		Static:     &Static{Revision: revision, Files: paths},
	}
}
