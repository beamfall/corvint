package dogfoodflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// changeCoverage is what the check SUMMARY's requirements, linked and
// unlinked members count (OCM-V0-019): requirements is the declared-intent
// requirements the bound change cites or edits, and linked those of them
// that something traces. The whole intent scope stays in the OCM aggregate.
// unlinked is the referenced IDs nothing traces, sorted and comma-joined, so
// the check can name them (OCM-V0-025).
type changeCoverage struct {
	requirements, linked int
	unlinked             string
}

// maxNamedUnlinked bounds how many unlinked requirement IDs the check names.
const maxNamedUnlinked = 32

// unlinkedNote is the stderr NOTE naming each unlinked requirement, at most
// maxNamedUnlinked of them, or "" when there is none or the coverage could
// not be read (OCM-V0-025), under its OCM-V0-920 name.
func (coverage *changeCoverage) unlinkedNote() string {
	if coverage == nil || coverage.unlinked == "" {
		return ""
	}
	ids := strings.Split(coverage.unlinked, ",")
	named := strings.Join(ids[:min(len(ids), maxNamedUnlinked)], ",")
	if len(ids) > maxNamedUnlinked {
		named += fmt.Sprintf(" (+%d more)", len(ids)-maxNamedUnlinked)
	}
	return fmt.Sprintf("dogfood-check: NOTE untraced-requirements count=%d ids=%s%s", len(ids), named, tracedForms)
}

// tracedForms is the note's tail naming every link form the traced test
// accepts (OCM-V0-950), so an author whose link is missing knows which to add.
const tracedForms = "; traced by: a linked obligation, an intent table row naming the ID or a range " +
	"over it (X-V0-001..003), or a test-file line naming the ID or its test-name suffix (_XV0001, _X_V0_001)"

// summaryFields prints the three members, or NOT_OBSERVED for each when the
// coverage could not be read, never a guessed count (invariant 2). Each name
// carries the change- prefix so it is never read as the aggregate's linked
// count (OCM-V0-920).
func (coverage *changeCoverage) summaryFields() []string {
	if coverage == nil {
		return []string{"change-requirements=NOT_OBSERVED", "change-traced=NOT_OBSERVED", "change-untraced=NOT_OBSERVED"}
	}
	return []string{"change-requirements=" + strconv.Itoa(coverage.requirements), "change-traced=" + strconv.Itoa(coverage.linked),
		"change-untraced=" + strconv.Itoa(coverage.requirements-coverage.linked)}
}

// requirementLine is the frozen OCM requirement-line syntax (OCM-V0-001).
var requirementLine = regexp.MustCompile("^- (?:`([A-Z][A-Z0-9-]{2,31}-[0-9]{3})`:[ ]|\\*\\*([A-Z][A-Z0-9-]{2,31}-[0-9]{3})[.:]\\*\\*[ ])")

// requirementItem is one requirement of a blob's `## Requirements` section:
// its lines [first, end) (zero-based) and bytes [start, stop).
type requirementItem struct {
	id                      string
	first, end, start, stop int
}

// requirementSection is a blob's lines and its requirements; first and end
// bound the section's lines, and first is -1 unless exactly one unfenced
// `## Requirements` heading exists.
type requirementSection struct {
	lines      []string
	items      []requirementItem
	first, end int
}

// parseRequirements enumerates a blob's requirements as OCM-V0-001 does
// (internal/lrfrepo requirementsFromBlob): one unfenced `## Requirements`
// heading, the section ending at the next unfenced line that is `##` or starts
// with `## `, and each requirement line in it, fenced or not, starting an item
// that runs to the next one or the section end.
func parseRequirements(blob []byte) requirementSection {
	lines := strings.SplitAfter(string(blob), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	offsets := make([]int, len(lines)+1)
	fenced := make([]bool, len(lines))
	var fence byte
	fenceLength := 0
	for i, line := range lines {
		offsets[i+1] = offsets[i] + len(line)
		text := strings.TrimRight(line, "\r\n")
		char, run, rest := fenceRun(text)
		switch {
		case fence == 0 && run >= 3 && (char != '`' || !strings.Contains(rest, "`")):
			fence, fenceLength, fenced[i] = char, run, true
		case fence != 0:
			fenced[i] = true
			if char == fence && run >= fenceLength && strings.Trim(rest, " \t") == "" {
				fence = 0
			}
		}
	}
	section := requirementSection{lines: lines, first: -1, end: len(lines)}
	for i, line := range lines {
		if !fenced[i] && strings.TrimRight(line, "\r\n") == "## Requirements" {
			if section.first >= 0 {
				return requirementSection{lines: lines, first: -1, end: len(lines)}
			}
			section.first = i
		}
	}
	if section.first < 0 {
		return section
	}
	for i := section.first + 1; i < len(lines); i++ {
		text := strings.TrimRight(lines[i], "\r\n")
		if !fenced[i] && (text == "##" || strings.HasPrefix(text, "## ")) {
			section.end = i
			break
		}
	}
	for i := section.first + 1; i < section.end; i++ {
		match := requirementLine.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}
		if count := len(section.items); count > 0 {
			section.items[count-1].end, section.items[count-1].stop = i, offsets[i]
		}
		section.items = append(section.items, requirementItem{id: match[1] + match[2], first: i, end: section.end, start: offsets[i], stop: offsets[section.end]})
	}
	return section
}

// fenceRun is a line's run of backticks or tildes after at most three spaces,
// and the rest of the line after it; a backtick run whose rest holds a backtick
// opens no fence (CommonMark, as internal/lrfrepo fencedLines).
func fenceRun(text string) (byte, int, string) {
	indent := len(text) - len(strings.TrimLeft(text, " "))
	if indent > 3 || indent == len(text) {
		return 0, 0, ""
	}
	char := text[indent]
	if char != '`' && char != '~' {
		return 0, 0, ""
	}
	run := indent
	for run < len(text) && text[run] == char {
		run++
	}
	if run-indent < 3 {
		return 0, 0, ""
	}
	return char, run - indent, text[run:]
}

// namesRequirement reports whether line holds id as an exact token: no
// adjoining byte in A-Z, a-z, 0-9, `_` or `-` continues it (OCM-V0-017).
func namesRequirement(line, id string) bool {
	for from := 0; ; {
		at := strings.Index(line[from:], id)
		if at < 0 {
			return false
		}
		at += from
		end := at + len(id)
		if (at == 0 || !tokenByte(line[at-1])) && (end == len(line) || !tokenByte(line[end])) {
			return true
		}
		from = at + 1
	}
}

// requirementRange is a `PREFIX-NNN..NNN` or `PREFIX-NNN..PREFIX-NNN` range
// token in a table row (OCM-V0-950).
var requirementRange = regexp.MustCompile(`([A-Z][A-Z0-9-]*-)([0-9]{3})\.\.(?:([A-Z][A-Z0-9-]*-))?([0-9]{3})`)

// rangeNamesRequirement reports whether line holds a range token over id whose
// first and last bytes adjoin no token byte, as namesRequirement requires.
func rangeNamesRequirement(line, id string) bool {
	prefix, number := id[:len(id)-3], id[len(id)-3:]
	for _, at := range requirementRange.FindAllStringSubmatchIndex(line, -1) {
		if at[0] > 0 && tokenByte(line[at[0]-1]) || at[1] < len(line) && tokenByte(line[at[1]]) {
			continue
		}
		if line[at[2]:at[3]] != prefix || at[6] >= 0 && line[at[6]:at[7]] != prefix {
			continue
		}
		if first, last := line[at[4]:at[5]], line[at[8]:at[9]]; first <= number && number <= last {
			return true
		}
	}
	return false
}

// suffixForms are the test-name spellings of id (OCM-V0-950): TCP-V0-991 as
// TCPV0991 and TCP_V0_991, as in TestX_TCPV0991 or TestX_TCP_V0_991.
func suffixForms(id string) []string {
	return []string{strings.ReplaceAll(id, "-", ""), strings.ReplaceAll(id, "-", "_")}
}

// namesRequirementSuffix reports whether line holds a test-name form of id that
// no upper-case letter or digit precedes and no digit follows, so a longer ID
// such as TCPV09910 or XTCPV0991 never matches.
func namesRequirementSuffix(line, id string) bool {
	for _, form := range suffixForms(id) {
		for from := 0; ; {
			at := strings.Index(line[from:], form)
			if at < 0 {
				break
			}
			at += from
			end := at + len(form)
			before := at == 0 || !(line[at-1] >= 'A' && line[at-1] <= 'Z' || line[at-1] >= '0' && line[at-1] <= '9')
			if before && (end == len(line) || line[end] < '0' || line[end] > '9') {
				return true
			}
			from = at + 1
		}
	}
	return false
}

func tokenByte(b byte) bool {
	return b == '_' || b == '-' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// changeCoverage counts the requirements of the declared intents that the
// bound change references, and how many of them are traced (OCM-V0-019).
// Referenced: a citation span in the map's evidence overlaps the requirement
// in the cited intent blob, or the BASE..target diff adds or changes a line
// of it. Traced: its obligation is linked in the change's OCM map, a table
// row outside the intent's requirements section names it, or a line of a
// `*_test.go` or `*_test.sh` file at the target names it, as an exact token.
// Any read that fails yields nil, so the SUMMARY prints NOT_OBSERVED.
func (f *flow) changeCoverage(mapPath string, intents []string) *changeCoverage {
	if !isRegular(mapPath) || isSymlink(mapPath) {
		return nil
	}
	data, err := readPrefix(mapPath, maxPlanBytes+1)
	if err != nil || len(data) > maxPlanBytes {
		return nil
	}
	var decoded struct {
		Evidence *[]struct {
			Path    string `json:"path"`
			BlobOID string `json:"blobOid"`
			Span    *struct {
				Start int `json:"start"`
				End   int `json:"end"`
			} `json:"span"`
		} `json:"evidence"`
	}
	if json.Unmarshal(data, &decoded) != nil || decoded.Evidence == nil {
		return nil
	}
	declared := map[string]bool{}
	for _, path := range intents {
		declared[path] = true
	}
	blobs := map[string]requirementSection{}
	referenced := map[string]bool{}
	for _, evidence := range *decoded.Evidence {
		if !declared[evidence.Path] || evidence.Span == nil {
			continue
		}
		section, ok := blobs[evidence.BlobOID]
		if !ok {
			blob, read := f.boundedBlob(evidence.BlobOID)
			if !read {
				return nil
			}
			section = parseRequirements(blob)
			blobs[evidence.BlobOID] = section
		}
		start, stop := evidence.Span.Start, max(evidence.Span.End, evidence.Span.Start+1)
		for _, item := range section.items {
			if start < item.stop && item.start < stop {
				referenced[item.id] = true
			}
		}
	}
	targets := make([]requirementSection, 0, len(intents))
	for _, path := range intents {
		blob, read := f.boundedBlob(f.target + ":" + path)
		if !read {
			return nil
		}
		section := parseRequirements(blob)
		targets = append(targets, section)
		changed, read := f.changedLines(path)
		if !read {
			return nil
		}
		for _, item := range section.items {
			for _, lines := range changed {
				if lines[0] < item.end && item.first < lines[1] {
					referenced[item.id] = true
				}
			}
		}
	}
	coverage := &changeCoverage{requirements: len(referenced)}
	if len(referenced) == 0 {
		return coverage
	}
	traced, read := f.linkedObligations(len(intents))
	if !read {
		return nil
	}
	for _, section := range targets {
		for i, line := range section.lines {
			if section.first >= 0 && i >= section.first && i < section.end || !strings.HasPrefix(line, "|") {
				continue
			}
			for id := range referenced {
				if namesRequirement(line, id) || rangeNamesRequirement(line, id) {
					traced[id] = true
				}
			}
		}
	}
	if !f.testsNaming(referenced, traced) {
		return nil
	}
	unlinked := []string{}
	for id := range referenced {
		if traced[id] {
			coverage.linked++
		} else {
			unlinked = append(unlinked, id)
		}
	}
	sort.Strings(unlinked)
	coverage.unlinked = strings.Join(unlinked, ",")
	return coverage
}

// boundedBlob reads one blob object of at most maxRequirementsBytes.
func (f *flow) boundedBlob(object string) ([]byte, bool) {
	size, status := f.git(true, "cat-file", "-s", object)
	if length, err := strconv.Atoi(chomp(size)); status != 0 || err != nil || length > maxRequirementsBytes {
		return nil, false
	}
	blob, status := f.git(true, "cat-file", "blob", object)
	return []byte(blob), status == 0
}

// hunkHeader is a zero-context hunk header's new-side start and length.
var hunkHeader = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,([0-9]+))? @@`)

// changedLines is the zero-based new-side line ranges [first, end) the
// BASE..target diff adds or changes in path; a pure deletion has none. A
// zero-context hunk with no inter-hunk context, whatever diff.interHunkContext
// says, spans only added or changed lines.
func (f *flow) changedLines(path string) ([][2]int, bool) {
	diff, status, bounded := f.boundedGit("diff", "-U0", "--inter-hunk-context=0", "--no-color", "--no-ext-diff", "--no-textconv",
		"--no-renames", f.base, f.target, "--", path)
	if status != 0 || !bounded {
		return nil, false
	}
	ranges := [][2]int{}
	for _, line := range textLines([]byte(diff)) {
		match := hunkHeader.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		start, _ := strconv.Atoi(match[1])
		length := 1
		if match[2] != "" {
			length, _ = strconv.Atoi(match[2])
		}
		if length > 0 {
			ranges = append(ranges, [2]int{start - 1, start - 1 + length})
		}
	}
	return ranges, true
}

// linkedObligations is the obligations the change's own OCM maps, one per
// declared intent and already verified by the aggregate, record as linked.
func (f *flow) linkedObligations(count int) (map[string]bool, bool) {
	linked := map[string]bool{}
	for index := 1; index <= count; index++ {
		path := fmt.Sprintf("%s/.corvint/change.ocm.%03d.json", f.root, index)
		if !isRegular(path) || isSymlink(path) {
			return nil, false
		}
		data, err := readPrefix(path, maxOCMMapBytes+1)
		if err != nil || len(data) > maxOCMMapBytes {
			return nil, false
		}
		var document struct {
			Obligations []struct {
				ID          string `json:"id"`
				Disposition string `json:"disposition"`
			} `json:"obligations"`
		}
		if json.Unmarshal(data, &document) != nil {
			return nil, false
		}
		for _, obligation := range document.Obligations {
			if obligation.Disposition == "linked" {
				linked[obligation.ID] = true
			}
		}
	}
	return linked, true
}

// maxOCMMapBytes is the OCM map read bound (OCM-V0-007).
const maxOCMMapBytes = 1 << 20

// testsNaming marks each referenced ID that a line of a Go or shell test file
// at the target names as an exact token or a test-name suffix form
// (OCM-V0-950). Repository grep settings cannot prefix the matched lines.
func (f *flow) testsNaming(referenced, traced map[string]bool) bool {
	ids := make([]string, 0, len(referenced))
	for id := range referenced {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	args := []string{"-c", "grep.lineNumber=false", "-c", "grep.column=false", "-c", "grep.fullName=false",
		"grep", "-I", "-h", "--no-color", "-F"}
	for _, id := range ids {
		args = append(args, "-e", id)
		for _, form := range suffixForms(id) {
			args = append(args, "-e", form)
		}
	}
	args = append(args, f.target, "--", "*_test.go", "*_test.sh")
	output, status, bounded := f.boundedGit(args...)
	if status > 1 || !bounded {
		return false
	}
	for _, line := range textLines([]byte(output)) {
		for _, id := range ids {
			if namesRequirement(line, id) || namesRequirementSuffix(line, id) {
				traced[id] = true
			}
		}
	}
	return true
}
