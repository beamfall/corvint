package dogfoodflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// droppedTrailer is the explicit disposition of one dropped hunk, a line of
// the merge commit's message naming the hunk id and a reason (DCW-V0-073).
const droppedTrailer = "Corvint-Dropped-Hunk:"

var droppedHunkID = regexp.MustCompile(`^[0-9a-f]{64}$`)

// droppedTally counts what PASS reports of dropped conflict sides.
type droppedTally struct {
	merges, hunks, restored, dispositioned int
}

// droppedHunk is one conflict region a one-sided merge resolved by keeping one
// side: kept lines start at line start of the merge commit's blob.
type droppedHunk struct {
	path          string
	ordinal       string
	start         int
	kept, dropped []string
	id            string
}

// droppedSide reports, per file and hunk, the conflict regions a merge
// derived with -Xours or -Xtheirs (or as its first parent's tree) resolved by
// dropping one side (DCW-V0-073). Each is recomputed from the plain ort
// conflict tree of the two parents and checked against the merge's own blob;
// a hunk passes only when a sealed change based on the merge restores the
// dropped side at HEAD or the merge's message dispositions its id. What
// cannot be recomputed is an UNKNOWN hunk, which fails.
func (b *batch) droppedSide(merge, first, second string, keepFirst bool, conflicted string) {
	droppedParent := second
	if !keepFirst {
		droppedParent = first
	}
	fields := strings.Split(conflicted, "\x00")
	if len(fields) < 2 || !fullRevision.MatchString(fields[0]) {
		b.dropUnknown(merge, droppedParent, "", "conflicts-not-observed")
		return
	}
	stages := map[string]map[string]string{}
	paths := []string{}
	for _, entry := range fields[1:] {
		if entry == "" {
			break
		}
		info, path, found := strings.Cut(entry, "\t")
		parts := strings.Fields(info)
		if !found || len(parts) != 3 {
			b.dropUnknown(merge, droppedParent, "", "conflicts-not-observed")
			return
		}
		if stages[path] == nil {
			stages[path] = map[string]string{}
			paths = append(paths, path)
		}
		stages[path][parts[2]] = parts[1]
	}
	if len(paths) == 0 {
		b.dropUnknown(merge, droppedParent, "", "conflicts-not-observed")
		return
	}
	b.dropped.merges++
	disposed := b.dispositions(merge)
	sort.Strings(paths)
	for _, path := range paths {
		hunks, reason := b.conflictHunks(merge, first, second, fields[0], path, keepFirst, stages[path])
		if reason != "" {
			b.dropUnknown(merge, droppedParent, path, reason)
			continue
		}
		for _, hunk := range hunks {
			b.dropped.hunks++
			detail := fmt.Sprintf("path=%s hunk=%s kept=L%d+%d dropped=%d side=%s id=%s",
				path, hunk.ordinal, hunk.start, len(hunk.kept), len(hunk.dropped), droppedParent, hunk.id)
			coverage := "none"
			if bind := b.restoredBy(merge, hunk); bind != "" {
				coverage = "restored:" + bind
				b.dropped.restored++
			} else if disposed[hunk.id] {
				coverage = "disposition"
				b.dropped.dispositioned++
			} else {
				b.failure("merge-side-dropped", merge+" "+detail)
			}
			b.print("  dropped-hunk %s coverage=%s\n", detail, coverage)
		}
	}
}

// dropUnknown records a dropped side whose hunks could not be recomputed.
func (b *batch) dropUnknown(merge, side, path, reason string) {
	detail := "side=" + side + " hunk=UNKNOWN reason=" + reason
	if path != "" {
		detail = "path=" + path + " " + detail
	}
	b.print("  dropped-hunk %s coverage=none\n", detail)
	b.failure("merge-side-dropped", merge+" "+detail)
}

// conflictHunks parses one conflicted path of the plain ort conflict tree and
// returns its conflict regions, or the reason they are UNKNOWN. Rebuilding the
// kept side from the regions must give exactly the merge's blob.
func (b *batch) conflictHunks(merge, first, second, conflictTree, path string, keepFirst bool, stages map[string]string) ([]droppedHunk, string) {
	recorded, status, bounded := b.boundedGit("cat-file", "blob", merge+":"+path)
	if status != 0 || !bounded {
		return nil, "merge-blob-unavailable"
	}
	marked, status, bounded := b.readScratch(true, "cat-file", "blob", conflictTree+":"+path)
	if status != 0 || !bounded {
		return nil, "conflict-blob-unavailable"
	}
	hunks, rebuilt, ok := parseConflict(marked, first, second, keepFirst)
	if ok && len(hunks) == 0 {
		// No markers: a binary or other whole-file conflict kept one stage.
		kept, dropped := stages["2"], stages["3"]
		if !keepFirst {
			kept, dropped = dropped, kept
		}
		if blob := b.gitRevision(merge + ":" + path); kept == "" || dropped == "" || blob != kept {
			return nil, "conflict-not-parsed"
		}
		return []droppedHunk{{path: path, ordinal: "whole", start: 1, kept: []string{kept}, dropped: []string{dropped},
			id: hunkID(path, "blob "+kept, "blob "+dropped)}}, ""
	}
	if !ok || rebuilt != recorded {
		return nil, "conflict-not-parsed"
	}
	for index := range hunks {
		hunks[index].path = path
		hunks[index].ordinal = fmt.Sprint(index + 1)
		hunks[index].id = hunkID(path, strings.Join(hunks[index].kept, ""), strings.Join(hunks[index].dropped, ""))
	}
	return hunks, ""
}

// parseConflict splits a merge-style conflict blob labelled with the two
// parents into its regions and the content that keeping one side gives.
func parseConflict(marked, first, second string, keepFirst bool) ([]droppedHunk, string, bool) {
	var hunks []droppedHunk
	var rebuilt strings.Builder
	var ours, theirs []string
	line, state := 1, 0
	for _, text := range strings.SplitAfter(marked, "\n") {
		marker := strings.TrimSuffix(text, "\n")
		switch {
		case text == "":
		case state == 0 && (marker == "<<<<<<< "+first || strings.HasPrefix(marker, "<<<<<<< "+first+":")):
			state, ours, theirs = 1, nil, nil
		case state == 0:
			rebuilt.WriteString(text)
			line++
		case state == 1 && marker == "=======":
			state = 2
		case state == 1:
			ours = append(ours, text)
		case state == 2 && (marker == ">>>>>>> "+second || strings.HasPrefix(marker, ">>>>>>> "+second+":")):
			kept, dropped := ours, theirs
			if !keepFirst {
				kept, dropped = theirs, ours
			}
			hunks = append(hunks, droppedHunk{start: line, kept: kept, dropped: dropped})
			rebuilt.WriteString(strings.Join(kept, ""))
			line += len(kept)
			state = 0
		default:
			theirs = append(theirs, text)
		}
	}
	return hunks, rebuilt.String(), state == 0
}

func hunkID(path, kept, dropped string) string {
	sum := sha256.Sum256([]byte(path + "\x00" + kept + "\x00" + dropped))
	return hex.EncodeToString(sum[:])
}

// dispositions reads the hunk ids the merge commit's own message dispositions
// with a non-empty reason.
func (b *batch) dispositions(merge string) map[string]bool {
	disposed := map[string]bool{}
	commit, status, bounded := b.boundedGit("cat-file", "commit", merge)
	if status != 0 || !bounded {
		return disposed
	}
	_, message, _ := strings.Cut(commit, "\n\n")
	for _, line := range strings.Split(message, "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), droppedTrailer)
		fields := strings.Fields(rest)
		if found && len(fields) > 1 && droppedHunkID.MatchString(fields[0]) {
			disposed[fields[0]] = true
		}
	}
	return disposed
}

// restoredBy names a verified constituent, based on the merge, that changed
// the hunk's path, when HEAD holds the dropped side: its dropped lines (or
// dropped blob) are present, or, for a dropped deletion, the kept lines are
// gone.
func (b *batch) restoredBy(merge string, hunk droppedHunk) string {
	binds := make([]string, 0, len(b.bases))
	for bind := range b.bases {
		binds = append(binds, bind)
	}
	sort.Strings(binds)
	for _, bind := range binds {
		base := b.bases[bind]
		if !b.gitSucceeds("merge-base", "--is-ancestor", merge, base) {
			continue
		}
		if _, status := b.git(true, "diff", "--quiet", "--no-ext-diff", "--no-textconv", base, bind, "--", hunk.path); status != 1 {
			continue
		}
		if b.headRestores(hunk) {
			return bind
		}
	}
	return ""
}

func (b *batch) headRestores(hunk droppedHunk) bool {
	if hunk.ordinal == "whole" {
		return b.gitRevision(b.target+":"+hunk.path) == hunk.dropped[0]
	}
	head, status, bounded := b.boundedGit("cat-file", "blob", b.target+":"+hunk.path)
	if !bounded {
		return false
	}
	if status != 0 {
		head = ""
	}
	lines := strings.SplitAfter(head, "\n")
	if len(hunk.dropped) > 0 {
		return containsBlock(lines, hunk.dropped)
	}
	return !containsBlock(lines, hunk.kept)
}

func containsBlock(lines, block []string) bool {
	for start := 0; start+len(block) <= len(lines); start++ {
		matched := true
		for offset, line := range block {
			if lines[start+offset] != line {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
