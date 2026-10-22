package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

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
}

type proposedHunk struct {
	ordinal     int
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
		options: options, stdout: stdout, blobs: map[string][]byte{},
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
		hunks[index] = proposedHunk{index + 1, hunk.Path, hunk.Disposition, hunk.Reason, hunk.OldRange.Start, hunk.OldRange.Count}
	}
	rows := p.draft(hunks)
	p.say("%s: PROPOSED rows=%d hunks=%d base=%s\n", p.prefix, len(rows), len(hunks), p.base)
	p.say("  this draft is not a plan: review every row, replace each with the evidence the hunk really depends on or implements, then write it to the DOGFOOD_CITATIONS file; nothing is cited until dogfood change runs with that file (DCW-V0-150)\n")
	for _, row := range rows {
		_, _ = fmt.Fprintln(p.stdout, row)
	}
	return 0
}

// draft returns the proposed rows in worklist order. Only hunks still owed a
// row get one: an unknown hunk without a recorded unknown reason.
func (p *proposal) draft(hunks []proposedHunk) []string {
	edited := map[string][][2]int{}
	for _, hunk := range hunks {
		edited[hunk.path] = append(edited[hunk.path], [2]int{hunk.oldStart, hunk.oldStart + hunk.oldCount})
	}
	used := map[string]bool{}
	rows := []string{}
	cited := 0
	lastCite := ""
	uniform := true
	for _, hunk := range hunks {
		if hunk.disposition != "unknown" || recordedUnknownReasons[hunk.reason] {
			continue
		}
		row, cite := p.propose(hunk, edited[hunk.path], used)
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
	return rows
}

// propose returns one row for hunk and the PATH:START:END it cites, or "" for
// an unknown row.
func (p *proposal) propose(hunk proposedHunk, edited [][2]int, used map[string]bool) (string, string) {
	ordinal := fmt.Sprint(hunk.ordinal)
	if workflow.GoverningInstructionPath(hunk.path) {
		return unknownProposal(ordinal, hunk.path+" is a governing file this change modifies, so it cannot govern its own hunk (CEM-V1-114)"), ""
	}
	baseBlob, ok := p.blob(p.base, hunk.path)
	if !ok {
		return unknownProposal(ordinal, hunk.path+" is absent at BASE; name the BASE contract this new file implements"), ""
	}
	targetBlob, _ := p.blob(p.target, hunk.path)
	lines := splitRecords(baseBlob)
	if len(lines) == 0 {
		return unknownProposal(ordinal, hunk.path+" is empty at BASE"), ""
	}
	free := func(line int) bool {
		for _, span := range edited {
			// A hunk's old range, plus the line after a pure insertion,
			// may be edited; its three context lines make this conservative.
			if line >= span[0] && line <= span[1] {
				return false
			}
		}
		return line >= 1 && line <= len(lines) && strings.TrimSpace(string(lines[line-1])) != "" &&
			spanOccurrences(targetBlob, lines[line-1]) == 1 && spanOccurrences(baseBlob, lines[line-1]) == 1
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
		if !free(line) {
			continue
		}
		cite := fmt.Sprintf("%s:%d:%d", hunk.path, line, line)
		row := fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, hunk.path, line, line, relation)
		if !used[cite] {
			return row, cite
		}
		if fallback == "" {
			fallback = row + "\x00" + cite
		}
	}
	if fallback != "" {
		row, cite, _ := strings.Cut(fallback, "\x00")
		return row, cite
	}
	return unknownProposal(ordinal, "no BASE line of "+hunk.path+" both outside every hunk and unique after the change"), ""
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
