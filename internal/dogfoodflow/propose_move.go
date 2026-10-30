package dogfoodflow

import (
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
)

// movedFile is one file the change moves. The canonical patch is taken with
// --no-renames, so a move is a whole-file deletion of old plus a whole-file
// addition of added; oldRoot is the BASE directory the move renamed, such as
// cmd/corvint-mcp for cmd/corvint-mcp/x.go moved to internal/mcp/cli/x.go.
type movedFile struct {
	old, added, oldRoot string
}

// movedCite is the one span both sides of a move cite.
type movedCite struct {
	path     string
	line     int
	relation string
}

// terms are the texts a BASE line can name the moved file by, most specific
// first: its BASE path, then each BASE directory above it up to the directory
// the move renamed. A top-level directory such as cmd or internal names too
// much to anchor a move, so it is never a term.
func (m movedFile) terms() []string {
	terms := []string{m.old}
	for dir := path.Dir(m.old); strings.Contains(dir, "/"); dir = path.Dir(dir) {
		terms = append(terms, dir)
		if dir == m.oldRoot {
			break
		}
	}
	return terms
}

// movedFiles pairs each whole-file addition with the whole-file deletion
// whose path shares the longest suffix of path components with it, at least
// the basename (DCW-V0-803). An addition with two equally good deletions, or
// a deletion two additions pick, is left unpaired, since the draft cannot
// tell which file moved where. Each pair is keyed by both of its paths.
func movedFiles(derived map[string]*patch.Hunk) map[string]movedFile {
	deleted, added := map[string]bool{}, map[string]bool{}
	for _, hunk := range derived {
		switch {
		case hunk.OldPath != nil && hunk.NewPath == nil:
			deleted[*hunk.OldPath] = true
		case hunk.OldPath == nil && hunk.NewPath != nil:
			added[*hunk.NewPath] = true
		}
	}
	chosen := map[string][]string{}
	picks := map[string]string{}
	for target := range added {
		best, bestLength, tied := "", 0, false
		for source := range deleted {
			length := commonSuffix(source, target)
			switch {
			case length > bestLength:
				best, bestLength, tied = source, length, false
			case length == bestLength && length > 0:
				tied = true
			}
		}
		if best != "" && !tied {
			picks[target] = best
			chosen[best] = append(chosen[best], target)
		}
	}
	moves := map[string]movedFile{}
	for target, source := range picks {
		if len(chosen[source]) != 1 {
			continue
		}
		components := strings.Split(source, "/")
		root := strings.Join(components[:len(components)-commonSuffix(source, target)], "/")
		move := movedFile{old: source, added: target, oldRoot: root}
		moves[source], moves[target] = move, move
	}
	return moves
}

// commonSuffix counts the trailing path components a and b share.
func commonSuffix(a, b string) int {
	left, right := strings.Split(a, "/"), strings.Split(b, "/")
	count := 0
	for count < len(left) && count < len(right) && left[len(left)-1-count] == right[len(right)-1-count] {
		count++
	}
	return count
}

// movedProposal drafts the row of either side of a move (DCW-V0-803): both
// sides cite one stable BASE line that names the moved file's BASE path, its
// directory or the directory the move renamed, in a file the change neither
// deletes nor edits at that line. A span no other row cites is preferred; when
// every naming line is taken, moves share one, since a moved package is often
// named by a single line. It returns "" for a hunk that is not a move, or when
// no BASE line names the move.
func (p *proposal) movedProposal(hunk proposedHunk, ordinal string, edited map[string][][2]int, used map[string]bool) (string, string, string) {
	move, moved := p.moves[hunk.path]
	if !moved {
		return "", "", ""
	}
	chosen, found := p.moveCites[move.old]
	if !found {
		var shared *movedCite
		for _, term := range move.terms() {
			for _, candidate := range p.matches[term] {
				if !p.stable(candidate.path, candidate.line, edited) || !namesPathOrDirectory(p.baseLines(candidate.path)[candidate.line-1], term) {
					continue
				}
				relation, _ := proposalRelation(candidate.path)
				cite := movedCite{candidate.path, candidate.line, relation}
				if !used[fmt.Sprintf("%s:%d:%d", cite.path, cite.line, cite.line)] {
					chosen, found = cite, true
					break
				}
				if shared == nil {
					shared = &cite
				}
			}
			if found {
				break
			}
		}
		if !found && shared == nil {
			return "", "", ""
		}
		if !found {
			chosen = *shared
		}
		if p.moveCites == nil {
			p.moveCites = map[string]movedCite{}
		}
		p.moveCites[move.old] = chosen
	}
	cite := fmt.Sprintf("%s:%d:%d", chosen.path, chosen.line, chosen.line)
	note := ""
	if kinds, _ := verify.SpanAnchors(p.derivedHunk(hunk), p.baseLines(chosen.path)[chosen.line-1]); len(kinds) == 0 {
		note = fmt.Sprintf("%s is one side of the move %s -> %s; both sides cite a BASE line naming the moved path (DCW-V0-803)", hunk.path, move.old, move.added)
	}
	return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, chosen.path, chosen.line, chosen.line, chosen.relation), cite, note
}

// namesPathOrDirectory reports whether text names term as a path or a
// directory: some occurrence is not preceded by a name byte (a slash may
// precede it, as in a Go import path) and is followed by the end, a slash, a
// byte that cannot continue a path, or a sentence-ending dot.
func namesPathOrDirectory(text []byte, term string) bool {
	name := func(b byte) bool {
		return b == '.' || b == '-' || b == '_' || b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
	}
	for offset := 0; term != ""; {
		found := strings.Index(string(text[offset:]), term)
		if found < 0 {
			return false
		}
		start, end := offset+found, offset+found+len(term)
		if (start == 0 || !name(text[start-1])) &&
			(end == len(text) || text[end] == '/' || !name(text[end]) || text[end] == '.' && (end+1 == len(text) || !name(text[end+1]))) {
			return true
		}
		offset = start + 1
	}
	return false
}
