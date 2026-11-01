package dogfoodflow

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// refreshRow is one row of the --refresh plan: its selector, resolved to a
// hunk ID where it can be, and the three fields after it (DCW-V0-442).
type refreshRow struct {
	line     int
	selector string
	id       string
	rest     string
}

// hunkFacts are what --refresh matches an earlier hunk ID by once the current
// patch no longer has it: the file, BASE range and content it covered.
type hunkFacts struct {
	path     string
	oldStart int64
	oldCount int64
	content  string
}

// refreshHistoryLimit caps the earlier commits --refresh derives patches for
// when it resolves hunk IDs the current patch no longer has.
const refreshHistoryLimit = 32

// staleHunkIDMarker marks a citation-plan-map-mismatch detail that names a
// re-hunked row, so the fix line names --refresh (DCW-V0-441).
const staleHunkIDMarker = "re-hunked it (DCW-V0-441)"

// readRefreshPlan reads the --refresh plan before any git read, refusing an
// unreadable one with its recovery (DCW-V0-442).
func (p *proposal) readRefreshPlan() {
	if p.options.Refresh == "" {
		return
	}
	name := p.options.Refresh
	if !filepath.IsAbs(name) {
		if wd, err := os.Getwd(); err == nil {
			name = filepath.Join(wd, name)
		}
	}
	// A non-regular file such as a FIFO is refused before it is opened, which
	// could otherwise block with no writer.
	var data []byte
	err := errors.New("not a regular file")
	if isRegular(name) {
		data, err = readPrefix(name, maxPlanBytes+1)
	}
	if err != nil || len(data) > maxPlanBytes {
		p.say("%s: REFUSE refresh-plan-unavailable\n  --refresh %s is not a readable regular citation plan of at most %d bytes\n  fix: corvint dogfood propose-citations %s --refresh \"$DOGFOOD_CITATIONS\" > plan.new, naming the plan the bind loop read\n",
			p.prefix, strconv.Quote(p.options.Refresh), maxPlanBytes, p.options.Base)
		p.exit(2)
	}
	p.refreshPlan = data
	deferredLines := []string{}
	for index, line := range textLines(data) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		selector, rest, _ := strings.Cut(line, "\t")
		// A deferred row records an intent-scope deferral, not a hunk, so it
		// is kept apart from hunk matching and carried as written
		// (DCW-V0-370).
		if selector == deferredRowKind {
			deferredLines = append(deferredLines, line)
			continue
		}
		if len(strings.Split(line, "\t")) != 4 || selector == "" {
			p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh row %d dropped: not a four-field SELECTOR PATH START:END RELATION row", index+1))
			continue
		}
		p.refreshRows = append(p.refreshRows, refreshRow{line: index + 1, selector: selector, rest: rest})
	}
	if len(deferredLines) == 0 {
		return
	}
	if _, _, valid := splitDeferredRows([]byte(strings.Join(deferredLines, "\n") + "\n")); !valid {
		p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh dropped %d deferred row(s): together they are not valid deferred<TAB>PATH<TAB>REASON rows under the DCW-V0-370 record rules; rewrite them by hand", len(deferredLines)))
		return
	}
	p.refreshDeferred = deferredLines
}

// refreshLabel is the PROPOSED header's account of a refresh.
func (p *proposal) refreshLabel() string {
	if p.options.Refresh == "" {
		return ""
	}
	carried, rekeyed, dropped := 0, 0, 0
	for _, note := range p.carryNotes {
		switch {
		case strings.Contains(note, " re-keyed "):
			rekeyed++
		case strings.Contains(note, " dropped"):
			dropped++
		}
	}
	for _, rows := range p.carried {
		carried += len(rows)
	}
	return fmt.Sprintf(" refreshed=%s carried=%d rekeyed=%d dropped=%d deferred=%d", strconv.Quote(p.options.Refresh), carried, rekeyed, dropped, len(p.refreshDeferred))
}

// carry maps each --refresh row onto a current owed hunk (DCW-V0-442): a row
// whose hunk ID is current is carried verbatim; one whose ID a later edit,
// amend or renumber re-hunked is re-keyed to the hunk with the same path and
// content, else the same path and BASE range, when exactly one unclaimed hunk
// matches; any other row is dropped with a note and its hunk drafted afresh.
// Carried cite spans are not proposed again.
func (p *proposal) carry(owed []proposedHunk, used map[string]bool) {
	if len(p.refreshRows) == 0 {
		return
	}
	p.carried = map[string][]string{}
	current := map[string]proposedHunk{}
	for _, hunk := range owed {
		current[hunk.id] = hunk
	}
	mapIDs, bound := p.refreshOrdinalIDs()
	unresolved := map[string]bool{}
	for index := range p.refreshRows {
		row := &p.refreshRows[index]
		switch {
		case strings.HasPrefix(row.selector, wire.HunkPrefix):
			row.id = row.selector
		case ordinal.MatchString(row.selector):
			if value, _ := strconv.Atoi(row.selector); value <= len(mapIDs) {
				row.id = mapIDs[value-1]
			}
		}
		if row.id != "" {
			if _, ok := current[row.id]; !ok {
				unresolved[row.id] = true
				if strings.HasPrefix(row.selector, wire.HunkPrefix) {
					p.staleIDs++
				}
			}
		}
	}
	facts := p.resolveHunkFacts(unresolved)
	rekey := map[string]string{}
	claimed := map[string]bool{}
	for _, row := range p.refreshRows {
		if _, ok := current[row.id]; ok {
			claimed[row.id] = true
		}
	}
	for _, match := range []func(proposedHunk, hunkFacts) bool{
		func(hunk proposedHunk, old hunkFacts) bool {
			derived := p.derived[hunk.id]
			return hunk.path == old.path && derived != nil && derived.ContentSha256 == old.content
		},
		func(hunk proposedHunk, old hunkFacts) bool {
			return hunk.path == old.path && int64(hunk.oldStart) == old.oldStart && int64(hunk.oldCount) == old.oldCount
		},
	} {
		for _, id := range sortedKeys(unresolved) {
			old, known := facts[id]
			if _, done := rekey[id]; done || !known {
				continue
			}
			found := []string{}
			for _, hunk := range owed {
				if !claimed[hunk.id] && match(hunk, old) {
					found = append(found, hunk.id)
				}
			}
			if len(found) == 1 {
				rekey[id] = found[0]
				claimed[found[0]] = true
			}
		}
	}
	for _, row := range p.refreshRows {
		target, how := "", ""
		switch {
		case row.id == "" && ordinal.MatchString(row.selector) && !bound:
			p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh row %d dropped: ordinal %s cannot be resolved, because no citation-plan binding records the map this plan was cited against and the prepared map may have been renumbered since; name the row by its full hunk ID to carry it; its hunk is drafted afresh", row.line, row.selector))
			continue
		case row.id == "":
			p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh row %d dropped: selector %s names no hunk of the map this plan was cited against", row.line, strconv.Quote(row.selector)))
			continue
		case current[row.id].id != "":
			target = row.id
		case rekey[row.id] != "":
			target = rekey[row.id]
			how = "content changed, so review the row"
			if derived := p.derived[target]; derived != nil && derived.ContentSha256 == facts[row.id].content {
				how = "content unchanged"
			}
		default:
			why := "no current hunk has its path and content or its path and BASE range"
			if _, known := facts[row.id]; !known {
				why = "it is in neither the current patch nor the last " + strconv.Itoa(refreshHistoryLimit) + " commits' patches since BASE"
			}
			p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh row %d dropped: hunk %s is gone, %s; its hunk is drafted afresh", row.line, row.id, why))
			continue
		}
		if how != "" {
			hunk := current[target]
			p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh row %d re-keyed %s -> %s (%s BASE %d,%d; %s)", row.line, row.id, target, hunk.path, hunk.oldStart, hunk.oldCount, how))
		}
		p.carried[target] = append(p.carried[target], target+"\t"+row.rest)
		fields := strings.Split(row.rest, "\t")
		if fields[0] != "unknown" {
			used[fields[0]+":"+fields[1]] = true
		}
	}
}

// refreshOrdinalIDs are the hunk IDs, in ordinal order, that the --refresh
// plan's ordinals name, and whether they are known. They come from the
// citation-plan binding the bind loop recorded when it cited this plan (its
// digest must match the plan, with or without its deferred rows). The map on
// disk is not used: a later prepare may have renumbered it, so without the
// binding an ordinal is ambiguous and its row is dropped rather than carried
// onto whichever hunk now holds that ordinal (DCW-V0-442).
func (p *proposal) refreshOrdinalIDs() ([]string, bool) {
	gitDir, status := p.git(true, "rev-parse", "--absolute-git-dir")
	if status == 0 {
		recorded := readLines(readFile(strings.TrimSpace(gitDir) + "/corvint" + citationBinding))
		digests := map[string]bool{sha256Hex(p.refreshPlan): true}
		if stripped, _, valid := splitDeferredRows(p.refreshPlan); valid {
			digests[sha256Hex(stripped)] = true
		}
		if len(recorded) > 0 && digests[recorded[0]] {
			return recorded[1:], true
		}
	}
	return nil, false
}

// resolveHunkFacts finds each wanted hunk ID in the canonical patch of BASE to
// an earlier commit of this change: the first-parent commits since BASE, then
// the HEAD reflog's descendants of BASE, which hold an amended-away tip.
func (p *proposal) resolveHunkFacts(wanted map[string]bool) map[string]hunkFacts {
	facts := map[string]hunkFacts{}
	if len(wanted) == 0 {
		return facts
	}
	// Each source has its own budget of refreshHistoryLimit patches, so a long
	// first-parent history cannot crowd out an amended-away reflog tip.
	sources := [][]string{}
	if listed, status, bounded := p.boundedGit("rev-list", "--first-parent", "-n", strconv.Itoa(refreshHistoryLimit), p.base+".."+p.target); status == 0 && bounded {
		sources = append(sources, textLines([]byte(chomp(listed))))
	}
	if listed, status, bounded := p.boundedGit("log", "-g", "--format=%H", "-n", strconv.Itoa(refreshHistoryLimit), "HEAD"); status == 0 && bounded {
		sources = append(sources, textLines([]byte(chomp(listed))))
	}
	seen := map[string]bool{p.target: true}
	for _, candidates := range sources {
		p.collectHunkFacts(candidates, wanted, seen, facts)
	}
	return facts
}

// collectHunkFacts adds to facts each wanted hunk found in the canonical patch
// of BASE to one of candidates, trying at most refreshHistoryLimit patches.
func (p *proposal) collectHunkFacts(candidates []string, wanted, seen map[string]bool, facts map[string]hunkFacts) {
	tried := 0
	for _, commit := range candidates {
		if len(facts) == len(wanted) || tried == refreshHistoryLimit {
			break
		}
		if commit == "" || seen[commit] {
			continue
		}
		seen[commit] = true
		if _, status := p.git(true, "merge-base", "--is-ancestor", p.base, commit); status != 0 {
			continue
		}
		tried++
		raw, status, bounded := p.boundedGit(p.canonicalDiffArgumentsFor(p.base, commit)...)
		if status != 0 || !bounded || raw == "" {
			continue
		}
		parsed, err := patch.Parse([]byte(raw))
		if err != nil {
			continue
		}
		for _, hunk := range parsed.Hunks {
			if wanted[hunk.ID] {
				facts[hunk.ID] = hunkFacts{hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count, hunk.ContentSha256}
			}
		}
	}
}

func sortedKeys[V any](set map[string]V) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var changeRequirementID = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,31}-[0-9]{3}$`)

// addedDefinitions are the requirement IDs a hunk's added "- `ID`:" lines
// define (DCW-V0-440).
func addedDefinitions(hunk *patch.Hunk) []string {
	ids := []string{}
	for _, line := range hunk.Body {
		if line.Prefix != '+' {
			continue
		}
		text := bytes.TrimLeft(line.Payload, " \t")
		if !bytes.HasPrefix(text, []byte("- `")) {
			continue
		}
		text = text[3:]
		if end := bytes.IndexByte(text, '`'); end > 0 && end+1 < len(text) && text[end+1] == ':' && changeRequirementID.Match(text[:end]) {
			ids = append(ids, string(text[:end]))
		}
	}
	return ids
}

// sameChangeRequirement returns a requirement ID this change itself defines
// that the hunk's added lines or its path name, as written or in a compact
// test-name form such as TestDCWV0440 or dcw_v0_440, and the path defining it.
// The defining hunk itself is not matched (DCW-V0-440).
func (p *proposal) sameChangeRequirement(hunk *patch.Hunk) (string, string) {
	if len(p.changeIDs) == 0 || hunk == nil {
		return "", ""
	}
	var added bytes.Buffer
	added.WriteString(anchorPath(hunk))
	added.WriteByte('\n')
	for _, line := range hunk.Body {
		if line.Prefix == '+' {
			added.Write(line.Payload)
			added.WriteByte('\n')
		}
	}
	text := added.Bytes()
	lower := bytes.ToLower(text)
	ids := make([]string, 0, len(p.changeIDs))
	for id := range p.changeIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		where := p.changeIDs[id]
		if where == hunk.DisplayPath {
			continue
		}
		if namesToken(text, id) || namesToken(text, strings.ReplaceAll(id, "-", "")) ||
			namesToken(lower, strings.ToLower(strings.ReplaceAll(id, "-", "_"))) {
			return id, where
		}
	}
	return "", ""
}

// namesToken reports whether token occurs in text not preceded by a letter of
// the token's case or a digit (or, for an uppercase token, '-' or '_') and
// not followed by a digit.
func namesToken(text []byte, token string) bool {
	for offset := 0; ; {
		found := bytes.Index(text[offset:], []byte(token))
		if found < 0 {
			return false
		}
		start := offset + found
		end := start + len(token)
		before := start == 0 || !isTokenByte(text[start-1], token[0] >= 'a' && token[0] <= 'z')
		after := end == len(text) || text[end] < '0' || text[end] > '9'
		if before && after {
			return true
		}
		offset = start + 1
	}
}

func isTokenByte(value byte, lower bool) bool {
	digit := value >= '0' && value <= '9'
	if lower {
		// test_dcw_v0_440 names the ID; xdcw_v0_440 does not.
		return value >= 'a' && value <= 'z' || digit
	}
	return value >= 'A' && value <= 'Z' || digit || value == '-' || value == '_'
}

// staleHunkRowDetail names a plan row whose hunk ID the prepared map lacks and
// the current hunks the plan leaves unnamed, one of which a later edit or
// amend most likely made of it (DCW-V0-441).
func staleHunkRowDetail(row int, selector string, hunks []map[string]string, named map[string]bool) string {
	unnamed := []string{}
	for index, hunk := range hunks {
		if hunk["disposition"] != "unknown" || recordedUnknownReasons[hunk["reason"]] || named[strconv.Itoa(index+1)] || named[hunk["id"]] {
			continue
		}
		unnamed = append(unnamed, fmt.Sprintf("hunk %d %s path=%s", index+1, hunk["id"], strconv.Quote(hunk["path"])))
	}
	detail := fmt.Sprintf("row %d hunk ID %s is not in the prepared map: a later edit or amend %s", row, selector, staleHunkIDMarker)
	switch {
	case len(unnamed) == 0:
		return detail + "; every current owed hunk is already named, so drop the row"
	case len(unnamed) == 1:
		return detail + "; current hunk ID: " + unnamed[0]
	case len(unnamed) <= 3:
		return detail + "; current unnamed hunks: " + strings.Join(unnamed, ", ")
	}
	return fmt.Sprintf("%s; current unnamed hunks: %s (+%d more)", detail, strings.Join(unnamed[:3], ", "), len(unnamed)-3)
}

// staleHunkIDHint is the fix line for a re-hunked plan row (DCW-V0-441).
func staleHunkIDHint(base, plan string) string {
	if plan == "" {
		plan = "PLAN"
	}
	return fmt.Sprintf("corvint dogfood propose-citations %s --refresh %s > %s.new, review the re-keyed rows it notes, then rerun with DOGFOOD_CITATIONS=%s.new (DCW-V0-442)", base, plan, plan, plan)
}
