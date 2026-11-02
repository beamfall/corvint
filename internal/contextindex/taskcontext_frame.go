package contextindex

import (
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/projectprofile"
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
	frames := linker.frameIndex()
	found := map[string]*testCandidate{}
	entry := func(candidate string) *testCandidate {
		if found[candidate] == nil {
			found[candidate] = &testCandidate{path: candidate, anchor: anchor}
		}
		return found[candidate]
	}
	// Signal one: every relation but the module directory needs the anchor's
	// stem (pairRelation), so only the same-stem paths are read.
	stem := contextStem(anchor)
	for _, candidate := range frames.byStem[stem] {
		relation := pairRelation(anchor, candidate, stem, true)
		if relation != "" && relation != "module directory member" {
			current := entry(candidate)
			current.mirrored = relation
			current.weight += mirroredWeight(relation)
		}
	}
	// Signal two: the paths the anchor's own imports resolve to, the inverse
	// of reverseImporters over a view holding only the anchor's imports.
	for _, candidate := range frames.importedBy(linker.compiler.index, anchor) {
		current := entry(candidate)
		current.imports = true
		current.weight++
	}
	linker.creditFrameIdentifiers(anchor, frames.eligible, entry)
	result := make([]*testCandidate, 0, len(found))
	for _, candidate := range found {
		if candidate.signals() > 0 {
			result = append(result, candidate)
		}
	}
	return result
}

func (linker *testLinker) creditFrameIdentifiers(anchor string, eligible map[string]struct{}, entry func(string) *testCandidate) {
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
		for _, definer := range linker.declaredOf(word) {
			if _, ok := eligible[definer]; ok {
				entry(definer).mention(word, idf)
			}
		}
	}
}

// frameIndex indexes the frame candidates, the term table's paths that are
// neither tests nor documentation, once per linker (IDX-SNAP-V0-810): by
// stem for the mirrored relation, and by the key each reverse-import rule
// matches a path on -- its Go package import path, each Python import
// spelling, or its suffixless web target -- so an anchor reads only the paths
// its stem and imports name, where it used to test every path.
type frameIndex struct {
	eligible                                       map[string]struct{}
	byStem, goByPackage, pyBySpelling, byWebTarget map[string][]string
}

func (linker *testLinker) frameIndex() *frameIndex {
	if linker.frames != nil {
		return linker.frames
	}
	frames := &frameIndex{
		eligible: map[string]struct{}{}, byStem: map[string][]string{}, goByPackage: map[string][]string{},
		pyBySpelling: map[string][]string{}, byWebTarget: map[string][]string{},
	}
	module := linker.compiler.index.Module
	for _, candidate := range linker.table.Paths {
		if contextIsTest(candidate) || isDocumentationSuffix(candidate) {
			continue
		}
		frames.eligible[candidate] = struct{}{}
		frames.byStem[contextStem(candidate)] = append(frames.byStem[contextStem(candidate)], candidate)
		// The same dispatch as reverseImporters.
		switch {
		case strings.HasSuffix(candidate, goImpactSuffix):
			key := goPackageImportPath(module, candidate)
			frames.goByPackage[key] = append(frames.goByPackage[key], candidate)
		case strings.HasSuffix(candidate, pythonImpactSuffix):
			for spelling := range PythonImportCandidates(candidate) {
				frames.pyBySpelling[spelling] = append(frames.pyBySpelling[spelling], candidate)
			}
		case webSuffixes[strings.ToLower(pythonPathSuffix(candidate))]:
			key := pythonWithoutSuffix(candidate)
			frames.byWebTarget[key] = append(frames.byWebTarget[key], candidate)
		}
	}
	linker.frames = frames
	return frames
}

// importedBy is every frame candidate c for which reverseImporters, over a
// view whose only importer is anchor, finds anchor: Go and Python exclude the
// anchor itself, the web rule does not (its oracle has no such guard).
func (frames *frameIndex) importedBy(index *Index, anchor string) []string {
	imports := index.Imports[anchor]
	if len(imports) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(candidates []string, guard bool) {
		for _, candidate := range candidates {
			if guard && candidate == anchor {
				continue
			}
			if _, ok := seen[candidate]; !ok {
				seen[candidate] = struct{}{}
				out = append(out, candidate)
			}
		}
	}
	profile := projectprofile.ByID(index.ProfileID)
	for imported := range imports {
		add(frames.goByPackage[imported], true)
		add(frames.pyBySpelling[imported], true)
		if webSpecifierAdmitted(imported, profile) {
			resolved := webImportTarget(anchor, imported, profile)
			add(frames.byWebTarget[resolved], false)
			if index := strings.TrimRight(resolved, "/") + "/index"; index != resolved {
				add(frames.byWebTarget[index], false)
			}
		}
	}
	return out
}
