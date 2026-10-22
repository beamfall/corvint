package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// ProposeOptions names the inputs of `corvint dogfood propose-citations BASE`.
type ProposeOptions struct {
	Root string
	Base string
}

// proposal drafts a per-hunk citation plan from the prepared map (DCW-V0-150).
type proposal struct {
	flow
	options ProposeOptions
	stdout  io.Writer
	blobs   map[string][]byte
	lines   map[string][][]byte
	// derived holds the canonical patch's hunks by ID and by path and old
	// range; matches holds the BASE lines naming each anchor pattern.
	derived map[string]*patch.Hunk
	matches map[string][]spanLine
}

// spanLine is one line of a BASE file.
type spanLine struct {
	path string
	line int
}

type proposedHunk struct {
	ordinal     int
	id          string
	path        string
	disposition string
	reason      string
	oldStart    int
	oldCount    int
}

// ProposeCitations prints, on stdout, a draft citation plan with one row per
// hunk of .corvint/change.cem.json that still owes a row: a stable BASE span
// next to the hunk, or an unknown row where none is found. It is read-only:
// it writes no plan, map or evidence, and the draft is accepted only when the
// lane reviews it and writes it to its DOGFOOD_CITATIONS file (DCW-V0-150).
func ProposeCitations(ctx context.Context, options ProposeOptions, stdout, stderr io.Writer) (int, error) {
	run := &proposal{
		flow:    flow{ctx: ctx, prefix: "dogfood-propose-citations", root: options.Root, stderr: stderr},
		options: options, stdout: stdout, blobs: map[string][]byte{}, lines: map[string][][]byte{},
	}
	return boundary(ctx, run.run, func() {})
}

func (p *proposal) run() int {
	p.requireRoot()
	base, status := p.git(true, "rev-parse", "--verify", "-q", p.options.Base+"^{commit}")
	if status != 0 {
		p.refuse("base-unavailable")
	}
	p.base = chomp(base)
	p.target = p.gitValue("rev-parse", "HEAD^{commit}")
	data, err := readPrefix(filepath.Join(p.root, ".corvint", "change.cem.json"), maxPlanBytes)
	if err != nil {
		p.say("%s: REFUSE map-unavailable\n  fix: run corvint dogfood change %s first to prepare .corvint/change.cem.json\n", p.prefix, p.options.Base)
		p.exit(2)
	}
	var document struct {
		BaseRevision string `json:"baseRevision"`
		Hunks        []struct {
			ID          string `json:"id"`
			Path        string `json:"path"`
			Disposition string `json:"disposition"`
			Reason      string `json:"reason"`
			OldRange    struct {
				Start int `json:"start"`
				Count int `json:"count"`
			} `json:"oldRange"`
		} `json:"hunks"`
	}
	if json.Unmarshal(data, &document) != nil {
		p.say("%s: REFUSE map-unavailable\n  fix: run corvint dogfood change %s first to prepare .corvint/change.cem.json\n", p.prefix, p.options.Base)
		p.exit(2)
	}
	if document.BaseRevision != p.base {
		p.say("%s: REFUSE map-base-mismatch\n  map baseRevision=%s BASE=%s\n  fix: pass the BASE the map was prepared against, or rerun corvint dogfood change BASE\n", p.prefix, document.BaseRevision, p.base)
		p.exit(2)
	}
	hunks := make([]proposedHunk, len(document.Hunks))
	for index, hunk := range document.Hunks {
		hunks[index] = proposedHunk{index + 1, hunk.ID, hunk.Path, hunk.Disposition, hunk.Reason, hunk.OldRange.Start, hunk.OldRange.Count}
	}
	rows, notes := p.draft(hunks)
	p.say("%s: PROPOSED rows=%d hunks=%d base=%s\n", p.prefix, len(rows), len(hunks), p.base)
	p.say("  this draft is not a plan: review every row, replace each with the evidence the hunk really depends on or implements, then write it to the DOGFOOD_CITATIONS file; nothing is cited until dogfood change runs with that file (DCW-V0-150)\n")
	for _, note := range notes {
		p.say("  %s\n", note)
	}
	for _, row := range rows {
		_, _ = fmt.Fprintln(p.stdout, row)
	}
	return 0
}

// draft returns the proposed rows in worklist order and one note per cite row
// that is not anchored, saying why. Only hunks still owed a row get one: an
// unknown hunk without a recorded unknown reason.
func (p *proposal) draft(hunks []proposedHunk) ([]string, []string) {
	edited := map[string][][2]int{}
	owed := []proposedHunk{}
	for _, hunk := range hunks {
		edited[hunk.path] = append(edited[hunk.path], [2]int{hunk.oldStart, hunk.oldStart + hunk.oldCount})
		if hunk.disposition == "unknown" && !recordedUnknownReasons[hunk.reason] {
			owed = append(owed, hunk)
		}
	}
	p.prepareAnchors(owed)
	used := map[string]bool{}
	rows, notes := []string{}, []string{}
	cited := 0
	lastCite := ""
	uniform := true
	for _, hunk := range owed {
		row, cite, fellBack := p.propose(hunk, edited, used)
		if fellBack != "" {
			notes = append(notes, fmt.Sprintf("row %d cited, not anchored (CEM-V1-081): %s", hunk.ordinal, fellBack))
		}
		if cite != "" {
			used[cite] = true
			uniform = uniform && (lastCite == "" || lastCite == cite)
			lastCite = cite
			cited++
		} else {
			uniform = false
		}
		rows = append(rows, row)
	}
	// One span for every hunk is refused (DCW-V0-048); keep the last hunk
	// unknown rather than propose a plan the bind loop refuses.
	if uniform && cited > 1 {
		last := len(rows) - 1
		ordinal, _, _ := strings.Cut(rows[last], "\t")
		rows[last] = unknownProposal(ordinal, "no second stable span was found near this hunk")
	}
	return rows, notes
}

// propose returns one row for hunk, the PATH:START:END it cites, or "" for an
// unknown row, and, for a cite row that is not anchored, why it fell back.
func (p *proposal) propose(hunk proposedHunk, edited map[string][][2]int, used map[string]bool) (string, string, string) {
	ordinal := fmt.Sprint(hunk.ordinal)
	if workflow.GoverningInstructionPath(hunk.path) {
		return unknownProposal(ordinal, hunk.path+" is a governing file this change modifies, so it cannot govern its own hunk (CEM-V1-114)"), "", ""
	}
	if row, cite := p.anchoredProposal(hunk, ordinal, edited, used); row != "" {
		return row, cite, ""
	}
	fellBack := "no stable BASE line names " + hunk.path + " or defines a requirement ID the hunk adds"
	if _, ok := p.blob(p.base, hunk.path); !ok {
		return unknownProposal(ordinal, hunk.path+" is absent at BASE and "+fellBack+"; name the BASE contract this new file implements"), "", ""
	}
	lines := p.baseLines(hunk.path)
	if len(lines) == 0 {
		return unknownProposal(ordinal, hunk.path+" is empty at BASE"), "", ""
	}
	anchor := min(max(hunk.oldStart, 1), len(lines))
	relation, preferred := proposalRelation(hunk.path)
	candidates := []int{}
	for line := anchor; line >= 1; line-- {
		if preferred(string(lines[line-1])) {
			candidates = append(candidates, line)
		}
	}
	for distance := 1; distance <= len(lines); distance++ {
		candidates = append(candidates, anchor-distance, anchor+hunk.oldCount+distance-1)
	}
	fallback := ""
	for _, line := range candidates {
		if !p.stable(hunk.path, line, edited) {
			continue
		}
		cite := fmt.Sprintf("%s:%d:%d", hunk.path, line, line)
		row := fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, hunk.path, line, line, relation)
		if !used[cite] {
			return row, cite, fellBack
		}
		if fallback == "" {
			fallback = row + "\x00" + cite
		}
	}
	if fallback != "" {
		row, cite, _ := strings.Cut(fallback, "\x00")
		return row, cite, fellBack
	}
	return unknownProposal(ordinal, "no BASE line of "+hunk.path+" both outside every hunk and unique after the change"), "", ""
}

// anchoredProposal returns the first row, an unused span first, whose stable
// BASE line earns hunk a CEM-V1-081 anchor (DCW-V0-161): the line defining a
// requirement ID the hunk adds, then a line naming the hunk path, a
// specification first. It returns "" when there is none.
func (p *proposal) anchoredProposal(hunk proposedHunk, ordinal string, edited map[string][][2]int, used map[string]bool) (string, string) {
	derived := p.derivedHunk(hunk)
	_, ids := verify.SpanAnchors(derived, nil)
	candidates := []spanLine{}
	for _, id := range ids {
		candidates = append(candidates, p.matches[definitionPattern(id)]...)
	}
	candidates = append(candidates, p.matches[anchorPath(derived)]...)
	fallback := [2]string{}
	for _, candidate := range candidates {
		if !p.stable(candidate.path, candidate.line, edited) {
			continue
		}
		if kinds, _ := verify.SpanAnchors(derived, p.baseLines(candidate.path)[candidate.line-1]); len(kinds) == 0 {
			continue
		}
		relation, _ := proposalRelation(candidate.path)
		cite := fmt.Sprintf("%s:%d:%d", candidate.path, candidate.line, candidate.line)
		row := fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, candidate.path, candidate.line, candidate.line, relation)
		if !used[cite] {
			return row, cite
		}
		if fallback[0] == "" {
			fallback = [2]string{row, cite}
		}
	}
	return fallback[0], fallback[1]
}

// prepareAnchors reads the canonical BASE..HEAD patch the map was derived
// from and, in one `git grep` of BASE, the lines naming each owed hunk's path
// or defining a requirement ID it adds. An unreadable or oversized read
// leaves no candidates, and the drafts fall back to nearby lines.
func (p *proposal) prepareAnchors(owed []proposedHunk) {
	p.derived, p.matches = map[string]*patch.Hunk{}, map[string][]spanLine{}
	if raw, status, bounded := p.boundedGit(p.canonicalDiffArguments()...); status == 0 && bounded {
		if parsed, err := patch.Parse([]byte(raw)); err == nil {
			for _, hunk := range parsed.Hunks {
				p.derived[hunk.ID] = hunk
				p.derived[rangeKey(hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count)] = hunk
			}
		}
	}
	patterns := map[string]bool{}
	for _, hunk := range owed {
		derived := p.derivedHunk(hunk)
		if name := anchorPath(derived); name != "" {
			patterns[name] = true
		}
		_, ids := verify.SpanAnchors(derived, nil)
		for _, id := range ids {
			patterns[definitionPattern(id)] = true
		}
	}
	if len(patterns) == 0 {
		return
	}
	// -o prints each match, not its line, so a long one-line document that
	// names a path cannot overflow the read bound.
	arguments := []string{"grep", "-z", "-n", "-o", "-F", "-I"}
	sorted := make([]string, 0, len(patterns))
	for pattern := range patterns {
		sorted = append(sorted, pattern)
	}
	sort.Strings(sorted)
	for _, pattern := range sorted {
		arguments = append(arguments, "-e", pattern)
	}
	out, status, bounded := p.boundedGit(append(arguments, p.base, "--", ".", ":(exclude).corvint/")...)
	if status != 0 || !bounded {
		return
	}
	for _, record := range strings.Split(out, "\n") {
		name, rest, named := strings.Cut(strings.TrimPrefix(record, p.base+":"), "\x00")
		number, text, numbered := strings.Cut(rest, "\x00")
		line, err := strconv.Atoi(number)
		if !named || !numbered || err != nil || line < 1 || workflow.GoverningInstructionPath(name) || strings.HasPrefix(name, ".corvint/") {
			continue
		}
		for _, pattern := range sorted {
			if strings.Contains(text, pattern) {
				p.matches[pattern] = append(p.matches[pattern], spanLine{name, line})
			}
		}
	}
	// Specifications first, then other documents; git grep's path order
	// within each.
	for _, found := range p.matches {
		sort.SliceStable(found, func(i, j int) bool { return documentRank(found[i].path) < documentRank(found[j].path) })
	}
}

// canonicalDiffArguments are the canonical patch arguments of `cem` map
// derivation (internal/cem/gitauth), so the parsed hunk IDs are the map's.
func (p *proposal) canonicalDiffArguments() []string {
	format, _ := p.git(true, "rev-parse", "--show-object-format")
	empty := map[string]string{
		"sha1":   "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
		"sha256": "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321",
	}[chomp(format)]
	return []string{"--attr-source=" + empty, "diff", "--full-index", "--no-color", "--no-ext-diff", "--no-textconv",
		"--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers", "--unified=3", "--inter-hunk-context=0",
		"--src-prefix=a/", "--dst-prefix=b/", "--ignore-submodules=none", "--end-of-options", p.base, p.target,
		"--", ".", ":(exclude).corvint/change.cem.json"}
}

// derivedHunk returns the canonical patch hunk of a map hunk, matched by ID or
// else by path and old range, or a hunk carrying only its path.
func (p *proposal) derivedHunk(hunk proposedHunk) *patch.Hunk {
	if found := p.derived[hunk.id]; hunk.id != "" && found != nil {
		return found
	}
	if found := p.derived[rangeKey(hunk.path, int64(hunk.oldStart), int64(hunk.oldCount))]; found != nil {
		return found
	}
	name := hunk.path
	return &patch.Hunk{NewPath: &name}
}

// anchorPath is the path a CEM-V1-081 path anchor names: the post-image path,
// or the old path of a pure deletion.
func anchorPath(hunk *patch.Hunk) string {
	switch {
	case hunk.NewPath != nil:
		return *hunk.NewPath
	case hunk.OldPath != nil:
		return *hunk.OldPath
	}
	return ""
}

func definitionPattern(id string) string { return "- `" + id + "`:" }

func rangeKey(name string, start, count int64) string {
	return fmt.Sprintf("%s\x00%d\x00%d", name, start, count)
}

func documentRank(name string) int {
	switch {
	case strings.HasPrefix(name, "docs/specs/") && strings.HasSuffix(name, ".md"):
		return 0
	case strings.EqualFold(path.Ext(name), ".md"):
		return 1
	}
	return 2
}

// stable reports whether line of name at BASE can be cited: it lies outside
// every hunk range of that path (a hunk's old range, plus the line after a
// pure insertion; its three context lines make this conservative), is not
// blank, and its text occurs exactly once at BASE and at the target.
func (p *proposal) stable(name string, line int, edited map[string][][2]int) bool {
	for _, span := range edited[name] {
		if line >= span[0] && line <= span[1] {
			return false
		}
	}
	lines := p.baseLines(name)
	if line < 1 || line > len(lines) || strings.TrimSpace(string(lines[line-1])) == "" {
		return false
	}
	baseBlob, _ := p.blob(p.base, name)
	targetBlob, _ := p.blob(p.target, name)
	return spanOccurrences(targetBlob, lines[line-1]) == 1 && spanOccurrences(baseBlob, lines[line-1]) == 1
}

// baseLines returns the LF records of name at BASE, none when it is absent.
func (p *proposal) baseLines(name string) [][]byte {
	if lines, ok := p.lines[name]; ok {
		return lines
	}
	data, _ := p.blob(p.base, name)
	p.lines[name] = splitRecords(data)
	return p.lines[name]
}

// proposalRelation names the relation a draft row uses for path and the lines
// preferred as its anchor: the enclosing heading of a document, the enclosing
// declaration of source code.
func proposalRelation(name string) (string, func(string) bool) {
	heading := func(line string) bool { return strings.HasPrefix(line, "#") }
	switch {
	case strings.HasPrefix(name, "docs/decisions/") || strings.HasPrefix(name, "docs/build-log/"):
		return "decision", heading
	case strings.EqualFold(path.Ext(name), ".md"):
		return "specification", heading
	case strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_test.sh"):
		return "test-claim", declaration
	default:
		return "dependency", declaration
	}
}

func declaration(line string) bool {
	for _, prefix := range []string{"func ", "type ", "var ", "const ", "def ", "class "} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return strings.HasSuffix(strings.TrimRight(line, "\n"), "() {")
}

func unknownProposal(ordinal, note string) string {
	return ordinal + "\tunknown\tinsufficient-evidence\tproposed: " + strings.NewReplacer("\t", " ", "\n", " ").Replace(note)
}

// blob reads path at commit, or reports it absent or not a regular file.
func (p *proposal) blob(commit, name string) ([]byte, bool) {
	key := commit + ":" + name
	if data, ok := p.blobs[key]; ok {
		return data, data != nil
	}
	entry, status := p.git(true, "ls-tree", commit, "--", name)
	if status != 0 || !strings.HasPrefix(entry, "100") || !strings.Contains(entry, " blob ") {
		p.blobs[key] = nil
		return nil, false
	}
	data, _ := p.git(true, "cat-file", "blob", key)
	p.blobs[key] = []byte(data)
	return p.blobs[key], true
}

// splitRecords splits data into LF-terminated records as cem cite counts
// lines; a final record without LF is kept.
func splitRecords(data []byte) [][]byte {
	records := [][]byte{}
	for len(data) > 0 {
		end := bytes.IndexByte(data, '\n')
		if end < 0 {
			records = append(records, data)
			break
		}
		records = append(records, data[:end+1])
		data = data[end+1:]
	}
	return records
}

// spanOccurrences counts, overlapping, where needle occurs in data, as the
// cite stability check matches span text.
func spanOccurrences(data, needle []byte) int {
	count := 0
	for offset := 0; len(needle) > 0; {
		index := bytes.Index(data[offset:], needle)
		if index < 0 {
			break
		}
		count++
		offset += index + 1
	}
	return count
}
