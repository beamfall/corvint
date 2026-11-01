package dogfoodflow

import (
	"fmt"
	"sort"
	"strings"
)

// governingSpecs names the change's governing specs for path anchors
// (DCW-V0-886): of the specifications whose requirement-bearing BASE lines
// name an owed hunk's path, the ones naming the most distinct owed paths. A
// spec the change modifies, though never cited, is the change's own contract,
// so when any modified spec names an owed path only modified specs compete. A
// change of several paths where no specification names two of them has none.
func (p *proposal) governingSpecs(owed []proposedHunk) map[string]bool {
	named := map[string]map[string]bool{}
	modified := false
	for _, hunk := range owed {
		name := anchorPath(p.derivedHunk(hunk))
		if name == "" {
			continue
		}
		specs := append([]string(nil), p.modifiedOwners[name]...)
		modified = modified || len(specs) > 0
		for _, candidate := range p.matches[name] {
			if documentRank(candidate.path) == 0 && requirementToken.Match(p.lineText(candidate)) {
				specs = append(specs, candidate.path)
			}
		}
		for _, spec := range specs {
			if named[spec] == nil {
				named[spec] = map[string]bool{}
			}
			named[spec][name] = true
		}
	}
	if modified {
		for spec := range named {
			if !p.changed[spec] {
				delete(named, spec)
			}
		}
	}
	best := 1
	for _, paths := range named {
		best = max(best, len(paths))
	}
	governing := map[string]bool{}
	for spec, paths := range named {
		// One spec naming a single path is no evidence it governs the change.
		if len(paths) == best && (best > 1 || len(owed) == 1) {
			governing[spec] = true
		}
	}
	return governing
}

// anchorRank orders a path-naming candidate (DCW-V0-886): a requirement line
// of a governing spec, any line of one, a requirement line of another
// document, then a bare mention.
func (p *proposal) anchorRank(candidate spanLine) int {
	requirement := requirementToken.Match(p.lineText(candidate))
	switch {
	case p.governing[candidate.path] && requirement:
		return 0
	case p.governing[candidate.path]:
		return 1
	case requirement:
		return 2
	}
	return 3
}

// rankAnchors returns the path-naming candidates in anchorRank order, keeping
// the prior order (specifications first, then git grep's) within a rank.
func (p *proposal) rankAnchors(candidates []spanLine) []spanLine {
	ranked := append([]spanLine(nil), candidates...)
	sort.SliceStable(ranked, func(i, j int) bool { return p.anchorRank(ranked[i]) < p.anchorRank(ranked[j]) })
	return ranked
}

// lineText is the BASE text of candidate's line, nil when it has none.
func (p *proposal) lineText(candidate spanLine) []byte {
	if lines := p.baseLines(candidate.path); candidate.line >= 1 && candidate.line <= len(lines) {
		return lines[candidate.line-1]
	}
	return nil
}

// weakAnchorNote is the low-confidence note for a row whose anchor lies
// outside every governing spec (DCW-V0-886).
func (p *proposal) weakAnchorNote(row string, candidate spanLine, hunkPath string) string {
	specs := make([]string, 0, len(p.governing))
	for spec := range p.governing {
		specs = append(specs, spec)
	}
	sort.Strings(specs)
	key, _, _ := strings.Cut(row, "\t")
	why := "no stable line of which names it"
	for index, spec := range specs {
		if p.changed[spec] {
			specs[index] = spec + " (modified by this change)"
			why = "which a draft never cites"
		}
	}
	return fmt.Sprintf("row %s cited low-confidence (DCW-V0-886): %s:%d names %s outside the change's governing spec %s, %s; review the cite, or keep the row unknown", key, candidate.path, candidate.line, hunkPath, strings.Join(specs, ", "), why)
}
