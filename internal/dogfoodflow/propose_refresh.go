package dogfoodflow

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	// base is the BASE whose patch held the hunk; a range from another BASE
	// does not match a current range (DCW-V0-842).
	base string
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
	if p.options.FromCEM != "" && p.options.Refresh != "" {
		p.say("%s: REFUSE refresh-source-conflict\n  --from-cem and --refresh each name the rows to carry, so only one may be given\n  fix: rerun with --from-cem %s or --refresh %s, not both (DCW-V0-843)\n",
			p.prefix, strconv.Quote(p.options.FromCEM), strconv.Quote(p.options.Refresh))
		p.exit(2)
	}
	if !p.refreshing() {
		return
	}
	var data []byte
	if p.options.FromCEM != "" {
		data = p.sealedCEMPlan()
	} else {
		var err error
		data, err = readRegularInput(p.options.Refresh)
		if err != nil {
			p.say("%s: REFUSE refresh-plan-unavailable\n  --refresh %s is not a readable regular citation plan of at most %d bytes\n  fix: corvint dogfood propose-citations %s --refresh \"$DOGFOOD_CITATIONS\" > plan.new, naming the plan the bind loop read\n",
				p.prefix, strconv.Quote(p.options.Refresh), maxPlanBytes, p.options.Base)
			p.exit(2)
		}
	}
	// A claims file is JSON and no plan row starts with '{' (CKC-V0-009).
	if trimmed := bytes.TrimLeft(data, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '{' {
		claims, why := decodeClaims(data)
		if claims == nil {
			p.say("%s: REFUSE claims-unreadable\n  --refresh %s is not a valid %s claims file: %s; nothing was drafted\n  fix: rerun corvint dogfood record-claims %s --plan PLAN --claims FILE to rewrite it, or pass a citation plan (CKC-V0-009)\n",
				p.prefix, strconv.Quote(p.options.Refresh), claimsSchema, why, p.options.Base)
			p.exit(2)
		}
		p.claims = claims
		p.refreshDeferred = claims.Deferred
		return
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
	if !p.refreshing() {
		return ""
	}
	if p.claims != nil {
		stats := p.claimStats
		return fmt.Sprintf(" claims=%s carried=%d reassess=%d gone=%d suppressed=%d deferred=%d", strconv.Quote(p.options.Refresh), stats.carried, stats.reassess, stats.gone, stats.suppressed, len(p.refreshDeferred))
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
	source := " refreshed=" + strconv.Quote(p.options.Refresh)
	if p.options.FromCEM != "" {
		source = " from-cem=" + strconv.Quote(p.options.FromCEM)
	}
	return fmt.Sprintf("%s carried=%d rekeyed=%d dropped=%d deferred=%d", source, carried, rekeyed, dropped, len(p.refreshDeferred))
}

// carry maps each --refresh row onto a current owed hunk (DCW-V0-442): a row
// whose hunk ID is current is carried verbatim; one whose ID a later edit,
// amend or renumber re-hunked is re-keyed to the hunk with the same path and
// content, else the same path and BASE range, when exactly one unclaimed hunk
// matches; any other row is dropped with a note and its hunk drafted afresh.
// Carried cite spans are not proposed again.
func (p *proposal) carry(owed []proposedHunk, used map[string]bool) {
	if p.claims != nil {
		p.carryClaims(owed, used)
		return
	}
	if len(p.refreshRows) == 0 {
		return
	}
	p.carried = map[string][]string{}
	p.carriedFiles = map[string]string{}
	current := map[string]proposedHunk{}
	for _, hunk := range owed {
		current[hunk.id] = hunk
	}
	mapIDs, bound := p.refreshOrdinalIDs()
	unresolved := map[string]bool{}
	// A row is stale only when the canonical BASE..HEAD patch lacks its hunk
	// ID, not merely when the hunk is no longer owed (already cited or
	// reasoned in a leftover map); an unreadable patch judges nothing stale
	// (DCW-V0-947).
	patched := map[string]bool{}
	if parsed, ok := p.canonicalPatch(); ok {
		for _, hunk := range parsed.Hunks {
			patched[hunk.ID] = true
		}
	}
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
				if strings.HasPrefix(row.selector, wire.HunkPrefix) && len(patched) > 0 && !patched[row.id] {
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
			return old.base == p.base && hunk.path == old.path && int64(hunk.oldStart) == old.oldStart && int64(hunk.oldCount) == old.oldCount
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
		fields := strings.Split(row.rest, "\t")
		if how != "" {
			hunk := current[target]
			p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh row %d re-keyed %s -> %s (%s BASE %d,%d; %s)", row.line, row.id, target, hunk.path, hunk.oldStart, hunk.oldCount, how))
			// The NOTE of a re-keyed unknown row was written for the old
			// hunk, so it is carried marked stale for review (DCW-V0-884),
			// unless the hunk's content is unchanged (DCW-V0-842).
			if fields[0] == "unknown" && how != "content unchanged" {
				fields[2] = staleNotePrefix + row.id + ", review] " + staleNote.ReplaceAllString(fields[2], "")
				row.rest = strings.Join(fields, "\t")
				p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh row %d note marked stale: it was written for %s; review it for %s", row.line, row.id, target))
			}
		}
		if fields[0] != "unknown" {
			lines, note, gone := p.repinLines(fields[0], fields[1])
			if gone != "" {
				// The same coordinates would now cite other text, which the
				// vet cannot tell from the cited contract, so the row is
				// carried unknown for review instead (DCW-V0-842).
				row.rest = "unknown\tinsufficient-evidence\t" + staleCitePrefix + fields[0] + " " + fields[1] + ", review] " + gone + "; cite the current contract"
				p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh row %d made unknown: %s, so it is not re-pinned (DCW-V0-842)", row.line, gone))
				p.carried[target] = append(p.carried[target], target+"\t"+row.rest)
				continue
			}
			if note != "" {
				fields[1] = lines
				row.rest = strings.Join(fields, "\t")
				p.carryNotes = append(p.carryNotes, fmt.Sprintf("refresh row %d: %s", row.line, note))
			}
			used[fields[0]+":"+fields[1]] = true
			if hunk := current[target]; p.carriedFiles[hunk.path] == "" {
				p.carriedFiles[hunk.path] = row.rest
			}
		}
		p.carried[target] = append(p.carried[target], target+"\t"+row.rest)
	}
}

// staleNotePrefix opens the NOTE of an unknown row --refresh re-keyed; staleNote
// matches an earlier such mark, so a second refresh replaces it.
const staleNotePrefix = "[stale: re-keyed from "

// staleCitePrefix opens the NOTE of a cite row --refresh carried unknown
// because its cited bytes vanished from BASE (DCW-V0-842).
const staleCitePrefix = "[stale: cited "

var staleNote = regexp.MustCompile(`^\[stale: re-keyed from [^\]]*\] `)

// refreshOrdinalIDs are the hunk IDs, in ordinal order, that the --refresh
// plan's ordinals name, and whether they are known. They come from the
// citation-plan binding the bind loop recorded when it cited this plan (its
// digest must match the plan, with or without its deferred rows). The map on
// disk is not used: a later prepare may have renumbered it, so without the
// binding an ordinal is ambiguous and its row is dropped rather than carried
// onto whichever hunk now holds that ordinal (DCW-V0-442).
func (p *proposal) refreshOrdinalIDs() ([]string, bool) {
	if recorded, ok := p.boundIDs(); ok {
		return recorded, true
	}
	return p.sealedOrdinalIDs()
}

// boundIDs is the hunk IDs, in map order, that the citation-plan binding
// records the --refresh plan was cited against, or false when the binding
// records another plan or none.
func (p *proposal) boundIDs() ([]string, bool) {
	gitDir, status := p.git(true, "rev-parse", "--absolute-git-dir")
	if status != 0 {
		return nil, false
	}
	recorded := readLines(readFile(strings.TrimSpace(gitDir) + "/corvint" + citationBinding))
	digests := map[string]bool{sha256Hex(p.refreshPlan): true}
	if stripped, _, valid := splitDeferredRows(p.refreshPlan); valid {
		digests[sha256Hex(stripped)] = true
	}
	if len(recorded) == 0 || !digests[recorded[0]] {
		return nil, false
	}
	return recorded[1:], true
}

// boundToMap reports whether the binding records the --refresh plan against
// exactly the hunks of the prepared map, so the map's BASE is the plan's
// (DCW-V0-842).
func (p *proposal) boundToMap(hunks []proposedHunk) bool {
	recorded, ok := p.boundIDs()
	if !ok || len(recorded) != len(hunks) {
		return false
	}
	for index, hunk := range hunks {
		if recorded[index] != hunk.id {
			return false
		}
	}
	return true
}

// maxSealedCandidates bounds the sealed CEMs sealedOrdinalIDs reads.
const maxSealedCandidates = 16

// sealedOrdinalIDs resolves the --refresh plan's ordinals through the sealed
// CEM a seal of this change archived, when no binding records them
// (DCW-V0-885): the one sealed CEM BASE..HEAD adds whose hunk at each row's
// ordinal holds that row, a cite row's span among its basis or an unknown
// row's reason. A plan no single sealed CEM matches stays unresolved.
func (p *proposal) sealedOrdinalIDs() ([]string, bool) {
	if p.base == "" || p.target == "" {
		return nil, false
	}
	listed, status, bounded := p.boundedGit("diff", "--name-only", "--no-renames", "--diff-filter=A", p.base, p.target, "--", ".corvint/changes/")
	if status != 0 || !bounded {
		return nil, false
	}
	paths := textLines([]byte(chomp(listed)))
	if len(paths) > maxSealedCandidates {
		paths = paths[len(paths)-maxSealedCandidates:]
	}
	var found []string
	foundPath := ""
	for _, path := range paths {
		if sealedCEMPath.FindStringSubmatch(path) == nil {
			continue
		}
		data, status, bounded := p.boundedGit("cat-file", "blob", p.target+":"+path)
		var m citationMap
		if status != 0 || !bounded || json.Unmarshal([]byte(data), &m) != nil || !p.sealedHolds(&m) {
			continue
		}
		if found != nil {
			return nil, false
		}
		foundPath = path
		for _, hunk := range m.Hunks {
			found = append(found, hunk.ID)
		}
	}
	if found == nil {
		return nil, false
	}
	p.carryNotes = append(p.carryNotes, "refresh ordinals resolved through the sealed CEM "+foundPath+", whose hunks hold every ordinal row (DCW-V0-885)")
	return found, true
}

// sealedHolds reports whether m's hunk at each ordinal row's ordinal holds
// that row; a plan with no ordinal row is never matched. A cite row names
// BASE lines while the sealed evidence names bytes of its pinned blob, so the
// row's lines are resolved against that blob (DCW-V0-885).
func (p *proposal) sealedHolds(m *citationMap) bool {
	evidence := map[string]int{}
	for index, record := range m.Evidence {
		evidence[record.ID] = index
	}
	blobs := map[string][]byte{}
	matched := 0
	for _, row := range p.refreshRows {
		if !ordinal.MatchString(row.selector) {
			continue
		}
		index, _ := strconv.Atoi(row.selector)
		if index < 1 || index > len(m.Hunks) {
			return false
		}
		hunk := m.Hunks[index-1]
		fields := strings.Split(row.rest, "\t")
		if fields[0] == "unknown" {
			if hunk.Disposition != "unknown" || hunk.Reason != fields[1] {
				return false
			}
			matched++
			continue
		}
		holds := false
		for _, basis := range hunk.Basis {
			position, found := evidence[basis.EvidenceID]
			holds = holds || found && p.evidenceHoldsLines(m, blobs, position, fields[0], fields[1])
		}
		if !holds {
			return false
		}
		matched++
	}
	return matched > 0
}

// evidenceHoldsLines reports whether m's evidence at position is path's
// one-based inclusive line span lines, resolved to bytes in the evidence's
// pinned blob as cem cite resolves it.
func (p *proposal) evidenceHoldsLines(m *citationMap, blobs map[string][]byte, position int, path, lines string) bool {
	record := m.Evidence[position]
	if record.Path != path || !wire.IsGitOid(record.BlobOid) {
		return false
	}
	blob, cached := blobs[record.BlobOid]
	if !cached {
		output, status, bounded := p.boundedGit("cat-file", "blob", record.BlobOid)
		if status != 0 || !bounded {
			return false
		}
		blob = []byte(output)
		blobs[record.BlobOid] = blob
	}
	span, ok := lineByteSpan(blob, lines)
	return ok && span == strconv.FormatInt(record.Span.Start, 10)+":"+strconv.FormatInt(record.Span.End, 10)
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
	// A plan drafted at another BASE names hunk IDs of that BASE's patches,
	// including the current HEAD's (DCW-V0-842).
	bases := []string{p.base}
	if p.priorBase != "" && p.priorBase != p.base {
		bases = append(bases, p.priorBase)
		sources = append(sources, []string{p.target})
	}
	for _, from := range bases {
		seen := map[string]bool{}
		if from == p.base {
			seen[p.target] = true
		}
		for _, candidates := range sources {
			p.collectHunkFacts(from, candidates, wanted, seen, facts)
		}
	}
	return facts
}

// collectHunkFacts adds to facts each wanted hunk found in the canonical patch
// of from to one of candidates, trying at most refreshHistoryLimit patches.
func (p *proposal) collectHunkFacts(from string, candidates []string, wanted, seen map[string]bool, facts map[string]hunkFacts) {
	tried := 0
	for _, commit := range candidates {
		if len(facts) == len(wanted) || tried == refreshHistoryLimit {
			break
		}
		if commit == "" || seen[commit] {
			continue
		}
		seen[commit] = true
		if _, status := p.git(true, "merge-base", "--is-ancestor", from, commit); status != 0 {
			continue
		}
		tried++
		raw, status, bounded := p.boundedGit(p.canonicalDiffArgumentsFor(from, commit)...)
		if status != 0 || !bounded || raw == "" {
			continue
		}
		parsed, err := patch.Parse([]byte(raw))
		if err != nil {
			continue
		}
		for _, hunk := range parsed.Hunks {
			if wanted[hunk.ID] {
				facts[hunk.ID] = hunkFacts{hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count, hunk.ContentSha256, from}
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
