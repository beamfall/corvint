package dogfoodflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// claimsSchema names the content-keyed claims file (CKC-V0-001).
const claimsSchema = "corvint-dogfood-claims/v0"

// Claims file bounds (CKC-V0-001).
const (
	maxClaimsBytes = 4 << 20
	maxClaims      = 256
	maxClaimRows   = 256
)

// claimsFile is a lane's reviewed citation-plan rows keyed by edit content
// rather than hunk ID, so they survive a fix commit or a rebase (CKC-V0-001).
type claimsFile struct {
	Schema   string   `json:"schema"`
	Base     string   `json:"base"`
	Claims   []claim  `json:"claims"`
	Deferred []string `json:"deferred"`
}

// claim is the reviewed rows of one edit: the hunk's path, content and BASE
// range, plus the anchor that relocates the range when BASE moves.
type claim struct {
	Path        string     `json:"path"`
	Content     string     `json:"content"`
	OldStart    int64      `json:"oldStart"`
	OldCount    int64      `json:"oldCount"`
	Anchor      string     `json:"anchor"`
	Occurrences int        `json:"occurrences"`
	Rows        []claimRow `json:"rows"`
	Tombstones  []claimRow `json:"tombstones"`
}

// claimRow is one plan row without its selector: a cite of BASE text, which
// keeps the SHA-256 of the cited bytes, or a reasoned unknown row.
type claimRow struct {
	Kind     string `json:"kind"`
	Path     string `json:"path,omitempty"`
	Lines    string `json:"lines,omitempty"`
	Relation string `json:"relation,omitempty"`
	Text     string `json:"text,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Note     string `json:"note,omitempty"`
}

// planFields are the three plan fields after the selector.
func (row claimRow) planFields() string {
	if row.Kind == "unknown" {
		return "unknown\t" + row.Reason + "\t" + row.Note
	}
	return row.Path + "\t" + row.Lines + "\t" + row.Relation
}

// tombKey identifies a cite row by what it cites, independent of where BASE
// numbers that text (CKC-V0-007).
func (row claimRow) tombKey() string {
	return row.Path + "\x00" + row.Relation + "\x00" + row.Text
}

var (
	fullCommitID = regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha256Value  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	lineSpan     = regexp.MustCompile(`^([1-9][0-9]*):([1-9][0-9]*)$`)
)

// parseLineSpan reads a one-based inclusive START:END line span.
func parseLineSpan(span string) (int, int, bool) {
	match := lineSpan.FindStringSubmatch(span)
	if match == nil {
		return 0, 0, false
	}
	start, err1 := strconv.Atoi(match[1])
	end, err2 := strconv.Atoi(match[2])
	return start, end, err1 == nil && err2 == nil && start <= end
}

// spanText returns the bytes a cite span names in records, or false.
func spanText(records [][]byte, span string) ([]byte, bool) {
	start, end, ok := parseLineSpan(span)
	if !ok || end > len(records) {
		return nil, false
	}
	return bytes.Join(records[start-1:end], nil), true
}

// lineOccurrences returns every zero-based record index where needle occurs
// line-aligned in records.
func lineOccurrences(records, needle [][]byte) []int {
	found := []int{}
	if len(needle) == 0 {
		return found
	}
	for index := 0; index+len(needle) <= len(records); index++ {
		match := true
		for offset, line := range needle {
			if !bytes.Equal(records[index+offset], line) {
				match = false
				break
			}
		}
		if match {
			found = append(found, index)
		}
	}
	return found
}

// rangeRecords returns the records an old range covers, or false.
func rangeRecords(records [][]byte, start, count int64) ([][]byte, bool) {
	if count == 0 {
		return nil, true
	}
	if start < 1 || start-1+count > int64(len(records)) {
		return nil, false
	}
	return records[start-1 : start-1+count], true
}

// fieldText admits a stored field: nonempty, one line, no tab, no forbidden
// control byte.
func fieldText(value string) bool {
	return value != "" && !strings.ContainsAny(value, "\t\n") && !hasForbiddenControl([]byte(value))
}

// valid reports whether row is a well-formed claim row (CKC-V0-009).
func (row claimRow) valid() bool {
	switch row.Kind {
	case "unknown":
		return row.Path == "" && row.Lines == "" && row.Relation == "" && row.Text == "" &&
			recordedUnknownReasons[row.Reason] && fieldText(row.Note) &&
			strings.IndexFunc(row.Note, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
	case "cite":
		if row.Reason != "" || row.Note != "" || !fieldText(row.Path) || !wire.Relations[row.Relation] {
			return false
		}
		if _, _, ok := parseLineSpan(row.Lines); ok {
			return sha256Value.MatchString(row.Text)
		}
		return strings.HasPrefix(row.Lines, bytesSpanPrefix) && fieldText(row.Lines) && row.Text == ""
	}
	return false
}

// decodeClaims reads and validates a claims file, or names why it cannot
// (CKC-V0-001, CKC-V0-009).
func decodeClaims(data []byte) (*claimsFile, string) {
	if len(data) > maxClaimsBytes {
		return nil, fmt.Sprintf("it exceeds the %d-byte claims bound", maxClaimsBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var file claimsFile
	if err := decoder.Decode(&file); err != nil {
		return nil, "it is not a claims JSON document"
	}
	if file.Schema != claimsSchema {
		return nil, "its schema is not " + claimsSchema
	}
	if !fullCommitID.MatchString(file.Base) {
		return nil, "its base is not a full commit ID"
	}
	if len(file.Claims) > maxClaims {
		return nil, fmt.Sprintf("it holds more than %d claims", maxClaims)
	}
	rows := 0
	for index, entry := range file.Claims {
		rows += len(entry.Rows) + len(entry.Tombstones)
		if !fieldText(entry.Path) || !sha256Value.MatchString(entry.Content) || !sha256Value.MatchString(entry.Anchor) ||
			entry.OldStart < 0 || entry.OldCount < 0 || entry.Occurrences < 0 || len(entry.Rows) == 0 {
			return nil, fmt.Sprintf("claim %d is malformed", index+1)
		}
		for _, row := range entry.Rows {
			if !row.valid() {
				return nil, fmt.Sprintf("claim %d holds a malformed row", index+1)
			}
		}
		for _, row := range entry.Tombstones {
			if row.Kind != "cite" || !row.valid() {
				return nil, fmt.Sprintf("claim %d holds a malformed tombstone", index+1)
			}
		}
	}
	if rows > maxClaimRows {
		return nil, fmt.Sprintf("it holds more than %d rows and tombstones", maxClaimRows)
	}
	if len(file.Deferred) > 0 {
		if _, _, valid := splitDeferredRows([]byte(strings.Join(file.Deferred, "\n") + "\n")); !valid {
			return nil, "its deferred rows are not valid deferred<TAB>PATH<TAB>REASON rows (DCW-V0-370)"
		}
	}
	return &file, ""
}

// encodeClaims renders file deterministically: claims sorted by path, BASE
// range and content; rows keep plan order (CKC-V0-001).
func encodeClaims(file *claimsFile) ([]byte, error) {
	sort.SliceStable(file.Claims, func(i, j int) bool {
		a, b := file.Claims[i], file.Claims[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.OldStart != b.OldStart {
			return a.OldStart < b.OldStart
		}
		if a.OldCount != b.OldCount {
			return a.OldCount < b.OldCount
		}
		return a.Content < b.Content
	})
	for index := range file.Claims {
		if file.Claims[index].Tombstones == nil {
			file.Claims[index].Tombstones = []claimRow{}
		}
	}
	if file.Deferred == nil {
		file.Deferred = []string{}
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// claimClass names how a claim compiled (CKC-V0-004, CKC-V0-005).
const (
	claimCarried       = "carried"
	claimDuplicate     = "duplicate"
	claimUnlocated     = "unlocated"
	claimSplit         = "split"
	claimMerged        = "merged"
	claimChanged       = "changed"
	claimCiteUnlocated = "cite-unlocated"
	claimGone          = "gone"
)

// claimResult is one claim's compilation: its class, the current hunks it
// matched or overlaps, and for a carried claim its rows in current BASE
// coordinates.
type claimResult struct {
	claim *claim
	class string
	hunks []*patch.Hunk
	rows  []claimRow
}

// claimsCompiler compiles claims recorded at one BASE onto the canonical
// patch of the current BASE; blob reads a file at a commit.
type claimsCompiler struct {
	recordedBase string
	currentBase  string
	blob         func(commit, name string) ([]byte, bool)
	records      map[string][][]byte
}

func (c *claimsCompiler) lines(commit, name string) ([][]byte, bool) {
	key := commit + ":" + name
	if records, ok := c.records[key]; ok {
		return records, records != nil
	}
	data, ok := c.blob(commit, name)
	if !ok {
		c.records[key] = nil
		return nil, false
	}
	c.records[key] = splitRecords(data)
	return c.records[key], true
}

// locate returns the claim's BASE range in current BASE coordinates: as
// recorded when BASE is unchanged, else moved to the one line-aligned
// occurrence of its anchor lines, which must also be unique where recorded
// (CKC-V0-004).
func (c *claimsCompiler) locate(entry *claim) (int64, bool) {
	if c.recordedBase == c.currentBase || entry.OldCount == 0 {
		return entry.OldStart, true
	}
	if entry.Occurrences != 1 {
		return 0, false
	}
	old, ok := c.lines(c.recordedBase, entry.Path)
	if !ok {
		return 0, false
	}
	needle, ok := rangeRecords(old, entry.OldStart, entry.OldCount)
	if !ok || sha256Hex(bytes.Join(needle, nil)) != entry.Anchor || len(lineOccurrences(old, needle)) != 1 {
		return 0, false
	}
	current, ok := c.lines(c.currentBase, entry.Path)
	if !ok {
		return 0, false
	}
	found := lineOccurrences(current, needle)
	if len(found) != 1 {
		return 0, false
	}
	return int64(found[0] + 1), true
}

// relocateCite moves a cite row to current BASE lines (CKC-V0-006): kept as
// recorded when BASE is unchanged, else moved to the one line-aligned
// occurrence of its cited text, whose hash must match the record.
func (c *claimsCompiler) relocateCite(row claimRow) (claimRow, bool) {
	if c.recordedBase == c.currentBase {
		return row, true
	}
	if row.Text == "" {
		return row, false
	}
	old, ok := c.lines(c.recordedBase, row.Path)
	if !ok {
		return row, false
	}
	start, end, _ := parseLineSpan(row.Lines)
	if end > len(old) {
		return row, false
	}
	needle := old[start-1 : end]
	if sha256Hex(bytes.Join(needle, nil)) != row.Text {
		return row, false
	}
	current, ok := c.lines(c.currentBase, row.Path)
	if !ok {
		return row, false
	}
	found := lineOccurrences(current, needle)
	if len(found) != 1 {
		return row, false
	}
	row.Lines = fmt.Sprintf("%d:%d", found[0]+1, found[0]+len(needle))
	return row, true
}

// overlaps reports whether two old ranges share a line; an empty range
// occupies its start line.
func overlaps(start, count, otherStart, otherCount int64) bool {
	end, otherEnd := start+max(count, 1), otherStart+max(otherCount, 1)
	return start < otherEnd && otherStart < end
}

// compile classifies every claim against hunks, the canonical patch in
// order (CKC-V0-004, CKC-V0-005, CKC-V0-006). A claim is carried only onto
// the one hunk with its path and content at its located BASE range, and only
// when no other hunk shares that content and no other claim reaches that
// hunk; everything else is refused or gone.
func (c *claimsCompiler) compile(claims []claim, hunks []*patch.Hunk) []claimResult {
	byContent := map[string][]*patch.Hunk{}
	byPath := map[string][]*patch.Hunk{}
	for _, hunk := range hunks {
		byContent[hunk.DisplayPath+"\x00"+hunk.ContentSha256] = append(byContent[hunk.DisplayPath+"\x00"+hunk.ContentSha256], hunk)
		byPath[hunk.DisplayPath] = append(byPath[hunk.DisplayPath], hunk)
	}
	results := make([]claimResult, len(claims))
	reach := map[*patch.Hunk][]int{}
	for index := range claims {
		entry := &claims[index]
		result := &results[index]
		result.claim = entry
		peers := byContent[entry.Path+"\x00"+entry.Content]
		start, located := c.locate(entry)
		switch {
		case len(peers) > 1:
			result.class, result.hunks = claimDuplicate, peers
			continue
		case !located:
			result.class, result.hunks = claimUnlocated, peers
			continue
		}
		if len(peers) == 1 && peers[0].OldRange.Start == start && peers[0].OldRange.Count == entry.OldCount {
			result.class, result.hunks = claimCarried, peers
		} else {
			for _, hunk := range byPath[entry.Path] {
				if overlaps(start, entry.OldCount, hunk.OldRange.Start, hunk.OldRange.Count) {
					result.hunks = append(result.hunks, hunk)
				}
			}
			switch len(result.hunks) {
			case 0:
				result.class = claimGone
			case 1:
				result.class = claimChanged
			default:
				result.class = claimSplit
			}
		}
		if len(result.hunks) == 1 {
			reach[result.hunks[0]] = append(reach[result.hunks[0]], index)
		}
	}
	for _, indexes := range reach {
		if len(indexes) < 2 {
			continue
		}
		class := claimDuplicate
		for _, index := range indexes {
			if results[index].class != claimCarried {
				class = claimMerged
			}
		}
		for _, index := range indexes {
			results[index].class = class
		}
	}
	for index := range results {
		result := &results[index]
		if result.class != claimCarried {
			continue
		}
		for _, row := range result.claim.Rows {
			if row.Kind == "cite" {
				moved, ok := c.relocateCite(row)
				if !ok {
					result.class, result.rows = claimCiteUnlocated, nil
					break
				}
				row = moved
			}
			result.rows = append(result.rows, row)
		}
	}
	return results
}

// describeRows renders a claim's rows for a REASSESS note, one line.
func describeRows(rows []claimRow) string {
	parts := make([]string, len(rows))
	for index, row := range rows {
		parts[index] = strings.ReplaceAll(row.planFields(), "\t", " ")
	}
	return strings.Join(parts, " | ")
}

// claimsReasons explains each refusal class in a REASSESS note.
var claimsReasons = map[string]string{
	claimDuplicate:     "another current hunk or claim shares this edit's content, so which one the rows belong to is ambiguous",
	claimUnlocated:     "BASE moved and the edit's BASE lines are absent from, or not unique in, the old or the current BASE",
	claimSplit:         "the edit is now split across several hunks",
	claimMerged:        "the edit now shares one hunk with another claimed edit",
	claimChanged:       "the hunk at this edit's BASE range now has other content",
	claimCiteUnlocated: "BASE moved and a cited BASE text is absent, changed or not unique in the current BASE",
}

// claimsStats count a claims refresh for its PROPOSED header (CKC-V0-010).
type claimsStats struct {
	carried, reassess, gone, suppressed int
}

// carryClaims carries each claim compiled onto an owed hunk, notes every
// refused or gone claim, and records each hunk's tombstones for the draft
// filter (CKC-V0-005, CKC-V0-007, CKC-V0-008).
func (p *proposal) carryClaims(owed []proposedHunk, used map[string]bool) {
	p.carried = map[string][]string{}
	p.tombstones = map[string]map[string]bool{}
	if p.patchHunks == nil {
		p.say("%s: REFUSE claims-patch-unavailable\n  the canonical BASE..HEAD patch could not be read or parsed, so no claim can be compiled; nothing was drafted (CKC-V0-009)\n", p.prefix)
		p.exit(2)
	}
	compiler := &claimsCompiler{recordedBase: p.claims.Base, currentBase: p.base, blob: p.blob, records: map[string][][]byte{}}
	results := compiler.compile(p.claims.Claims, p.patchHunks)
	current := map[string]bool{}
	for _, hunk := range owed {
		current[hunk.id] = true
	}
	for _, result := range results {
		for _, hunk := range result.hunks {
			for _, row := range result.claim.Tombstones {
				if p.tombstones[hunk.ID] == nil {
					p.tombstones[hunk.ID] = map[string]bool{}
				}
				p.tombstones[hunk.ID][row.tombKey()] = true
			}
		}
		where := fmt.Sprintf("%s BASE %d,%d", result.claim.Path, result.claim.OldStart, result.claim.OldCount)
		switch result.class {
		case claimCarried:
			target := result.hunks[0].ID
			if !current[target] {
				p.carryNotes = append(p.carryNotes, fmt.Sprintf("claim %s matches %s, which the prepared map already dispositions; nothing carried", where, target))
				continue
			}
			for _, row := range result.rows {
				p.carried[target] = append(p.carried[target], target+"\t"+row.planFields())
				if row.Kind == "cite" {
					used[row.Path+":"+row.Lines] = true
				}
			}
			p.claimStats.carried += len(result.rows)
		case claimGone:
			p.claimStats.gone++
			p.carryNotes = append(p.carryNotes, fmt.Sprintf("claim %s gone: no current hunk overlaps it, so its rows are not carried (CKC-V0-005)", where))
		default:
			p.claimStats.reassess++
			ids := make([]string, len(result.hunks))
			for index, hunk := range result.hunks {
				ids[index] = hunk.ID
			}
			named := "no current hunk"
			if len(ids) > 0 {
				named = strings.Join(ids, ", ")
			}
			p.carryNotes = append(p.carryNotes, fmt.Sprintf("REASSESS %s claim %s -> %s: %s; nothing carried, the hunk is drafted afresh; its reviewed rows were: %s (CKC-V0-005)",
				result.class, where, named, claimsReasons[result.class], describeRows(result.claim.Rows)))
		}
	}
}

// tombstoned reports whether a drafted cite row for hunkID repeats a row the
// author removed for that edit (CKC-V0-007).
func (p *proposal) tombstoned(hunkID, row string) bool {
	removed := p.tombstones[hunkID]
	if len(removed) == 0 {
		return false
	}
	fields := strings.Split(row, "\t")
	if len(fields) != 4 || unknownRow(fields) {
		return false
	}
	text, ok := spanText(p.baseLines(fields[1]), fields[2])
	if !ok {
		return false
	}
	return removed[claimRow{Kind: "cite", Path: fields[1], Relation: fields[3], Text: sha256Hex(text)}.tombKey()]
}
