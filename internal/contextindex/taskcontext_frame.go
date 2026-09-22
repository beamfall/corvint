package contextindex

import (
	"math"
	"slices"
	"sort"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// frameRelationRows is the TCP-V0-020 experiment. The test named by a task is
// an anchor for implementation evidence, rather than evidence of the repair itself.
func (compiler *taskContextCompiler) frameRelationRows() ([]contextRow, bool) {
	if runtimeenv.Value("CONTEXT_FRAME_RELATION") != "1" {
		return nil, false
	}
	if compiler.subject != "" {
		return nil, false
	}
	anchors := compiler.namedTestFrames()
	if len(anchors) == 0 {
		return nil, false
	}
	compiler.markRan("test")
	if len(anchors) > contextMentionCap {
		compiler.markState("capped", "test")
		compiler.slotOmitted = true
		anchors = anchors[:contextMentionCap]
	}
	linker := compiler.newTestLinker()
	best := map[string]*testCandidate{}
	for position, anchor := range anchors {
		for _, candidate := range linker.frameCandidates(anchor) {
			candidate.position = position
			current := best[candidate.path]
			if current == nil || candidate.outranks(current) {
				best[candidate.path] = candidate
			}
		}
	}
	ordered := make([]*testCandidate, 0, len(best))
	for _, candidate := range best {
		ordered = append(ordered, candidate)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].outranks(ordered[j]) })
	rows := make([]contextRow, 0, len(ordered))
	for _, candidate := range ordered {
		confidence := "medium"
		if candidate.signals() >= 2 {
			confidence = "high"
		}
		reason := candidate.reason()
		rows = append(rows, contextRow{
			kind: "test", path: candidate.path, score: 650, line: 1,
			summary: reason, reason: reason, confidence: confidence, authority: "test-convention",
		})
	}
	return rows, true
}

func (compiler *taskContextCompiler) namedTestFrames() []string {
	paths := compiler.mentionedPaths()
	frames := make([]string, 0, len(paths))
	for _, candidate := range paths {
		if contextIsTest(candidate) {
			frames = append(frames, candidate)
		}
	}
	return slices.Compact(frames)
}

func (linker *testLinker) frameCandidates(anchor string) []*testCandidate {
	index := linker.compiler.index
	// The import resolver only reads this view. Restricting its importer map
	// to the frame avoids scanning every importer for every source candidate.
	view := *index
	view.Imports = map[string]map[string]struct{}{anchor: index.Imports[anchor]}
	found := map[string]*testCandidate{}
	for _, candidate := range linker.table.Paths {
		if contextIsTest(candidate) || isDocumentationSuffix(candidate) {
			continue
		}
		entry := &testCandidate{path: candidate, anchor: anchor}
		relation := pairRelation(anchor, candidate, contextStem(anchor), true)
		if relation != "" && relation != "module directory member" {
			entry.mirrored = relation
			entry.weight += mirroredWeight(relation)
		}
		if len(reverseImporters(&view, candidate)) != 0 {
			entry.imports = true
			entry.weight++
		}
		found[candidate] = entry
	}
	linker.creditFrameIdentifiers(anchor, found)
	result := make([]*testCandidate, 0, len(found))
	for _, candidate := range found {
		if candidate.signals() > 0 {
			result = append(result, candidate)
		}
	}
	return result
}

func (linker *testLinker) creditFrameIdentifiers(anchor string, found map[string]*testCandidate) {
	text, loaded := sourceTextBounded(linker.compiler.index.Sources[anchor])
	if !loaded {
		return
	}
	for _, word := range keys(scanWords(text)) {
		low, high, present := linker.table.Words.find(word)
		if !present || high-low > contextTestMentionPostings {
			continue
		}
		idf := math.Log(float64(len(linker.table.Paths)+1) / float64(high-low))
		for _, definer := range linker.declared[word] {
			if candidate := found[definer]; candidate != nil {
				candidate.mention(word, idf)
			}
		}
	}
}
