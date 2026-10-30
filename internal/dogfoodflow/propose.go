package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	// Refresh names an earlier plan whose rows are carried forward to the
	// current hunks by hunk ID, else by path and content or BASE range
	// (DCW-V0-442).
	Refresh string
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
	// basenames also searches each hunk path's basename of at least 6
	// bytes, which CEM-V1-081 counts as a path anchor. The check sets it to
	// decide that no anchor is available (DCW-V0-165); drafts do not, since
	// DCW-V0-161 excludes basename matches.
	basenames bool
	// keyByID keys each row by its full hunk ID instead of its ordinal: a
	// draft taken from the canonical patch before any map exists has no map
	// order to number against (DCW-V0-350).
	keyByID bool
	// changed holds every path BASE..HEAD adds, modifies or deletes; a draft
	// never cites one (DCW-V0-393).
	changed map[string]bool
	// changeIDs maps each requirement ID an added "- `ID`:" line of the
	// canonical patch defines to the path defining it (DCW-V0-440).
	changeIDs map[string]string
	// refreshRows are the --refresh plan's rows; carried holds those carried
	// to each current hunk ID, and carryNotes says what happened to each
	// (DCW-V0-442).
	refreshRows []refreshRow
	carried     map[string][]string
	carryNotes  []string
	// refreshPlan is the --refresh plan as read; refreshDeferred holds its
	// validated deferred rows, carried as written (DCW-V0-370).
	refreshPlan     []byte
	refreshDeferred []string
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
// hunk of .corvint/change.cem.json that still owes a row, or of the canonical
// BASE..HEAD patch when no usable map is prepared (DCW-V0-350, DCW-V0-394): a
// stable BASE span of a file the change leaves untouched that anchors the hunk,
// or an unknown row where none is found (DCW-V0-393). It is read-only:
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
	p.readRefreshPlan()
	base, status := p.git(true, "rev-parse", "--verify", "-q", p.options.Base+"^{commit}")
	if status != 0 {
		p.refuse("base-unavailable")
	}
	p.base = chomp(base)
	p.target = p.gitValue("rev-parse", "HEAD^{commit}")
	mapPath := filepath.Join(p.root, ".corvint", "change.cem.json")
	if _, err := os.Lstat(mapPath); errors.Is(err, fs.ErrNotExist) {
		return p.template("NOT_PREPARED")
	}
	// A map this draft cannot use is not refused: the canonical patch drafts
	// the plan without it, so a lane that commits before its first prepare
	// needs no extra dogfood change (DCW-V0-394).
	data, err := readPrefix(mapPath, maxPlanBytes)
	baseRevision, hunks, ok := preparedHunks(data)
	if err != nil || !ok {
		p.say("%s: NOTE map-unreadable\n  .corvint/change.cem.json is not a readable map, so the draft is keyed by the current patch's hunk IDs; the bind does not replace an unreadable map, so remove it (or git restore a committed one) before dogfood change BASE (DCW-V0-394)\n", p.prefix)
		return p.template("UNREADABLE")
	}
	if baseRevision != p.base {
		p.say("%s: NOTE map-base-mismatch\n  map baseRevision=%s BASE=%s, so the draft is keyed by the current patch's hunk IDs against BASE; dogfood change BASE prepares a fresh map when it binds (DCW-V0-394)\n", p.prefix, baseRevision, p.base)
		return p.template("OTHER_BASE")
	}
	if p.options.Refresh != "" {
		// A refreshed plan is keyed by hunk ID, so it survives later reordering.
		p.keyByID = true
	}
	if p.mapStale(hunks) {
		p.say("%s: NOTE map-stale\n  .corvint/change.cem.json no longer matches the canonical BASE..HEAD patch (a commit followed its prepare), so its ordinals would name other hunks and its ranges miss new edits; drafting hunk-ID rows from the current patch instead (DCW-V0-382)\n", p.prefix)
		return p.template("STALE")
	}
	rows, notes := p.draft(hunks)
	p.say("%s: PROPOSED rows=%d hunks=%d base=%s%s\n", p.prefix, len(rows), len(hunks), p.base, p.refreshLabel())
	p.say("  this draft is not a plan: review every row, replace each with the evidence the hunk really depends on or implements, then write it to the DOGFOOD_CITATIONS file; nothing is cited until dogfood change runs with that file (DCW-V0-150)\n")
	p.printDraft(hunks, rows, notes)
	return 0
}

// printDraft prints the notes and, on stderr, which file and BASE range each
// drafted row's hunk edits, so a reviewer need not rebuild the mapping from
// git diff; the rows themselves keep the four-field plan grammar (DCW-V0-382).
func (p *proposal) printDraft(hunks []proposedHunk, rows, notes []string) {
	for _, note := range notes {
		p.say("  %s\n", note)
	}
	edits := map[string]proposedHunk{}
	for _, hunk := range hunks {
		edits[fmt.Sprint(hunk.ordinal)], edits[hunk.id] = hunk, hunk
	}
	for _, row := range rows {
		key, _, _ := strings.Cut(row, "\t")
		if hunk, ok := edits[key]; ok && key != "" {
			p.say("  hunk %s edits %s at BASE %d,%d\n", key, hunk.path, hunk.oldStart, hunk.oldCount)
		}
		_, _ = fmt.Fprintln(p.stdout, row)
	}
}

// canonicalPatch reads and parses the canonical BASE..HEAD patch the map is
// derived from, or reports it unreadable within its bound.
func (p *proposal) canonicalPatch() (*patch.Patch, bool) {
	raw, status, bounded := p.boundedGit(p.canonicalDiffArguments()...)
	if status != 0 || !bounded {
		return nil, false
	}
	if raw == "" {
		return new(patch.Patch), true
	}
	parsed, err := patch.Parse([]byte(raw))
	return parsed, err == nil
}

// mapStale reports whether the prepared map's hunks, by path, BASE range and
// any recorded hunk ID, differ from the current canonical patch's in either direction: a
// commit after the prepare added a hunk, reverted one the map still holds, or
// amended a hunk's content in place, which keeps its range but re-hunks its ID
// (DCW-V0-441). An unreadable patch is not judged stale; the draft then keeps
// the map (DCW-V0-382).
func (p *proposal) mapStale(hunks []proposedHunk) bool {
	parsed, ok := p.canonicalPatch()
	if !ok {
		return false
	}
	current := map[string]string{}
	for _, hunk := range parsed.Hunks {
		current[rangeKey(hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count)] = hunk.ID
	}
	if len(current) != len(hunks) {
		return true
	}
	for _, hunk := range hunks {
		id, found := current[rangeKey(hunk.path, int64(hunk.oldStart), int64(hunk.oldCount))]
		if !found || hunk.id != "" && hunk.id != id {
			return true
		}
	}
	return false
}

// template drafts the plan before any map exists (DCW-V0-350): the hunks come
// from the canonical BASE..HEAD patch cem prepare derives the map from, and
// each row is keyed by its full hunk ID, which the bind accepts in place of an
// ordinal, so writing the plan needs no probe bind pass. Every hunk is drafted
// as owed; the bind and its plan precheck (DCW-V0-019, DCW-V0-335) still judge
// every row against the map they prepare.
func (p *proposal) template(mapState string) int {
	parsed, ok := p.canonicalPatch()
	if !ok {
		p.say("%s: REFUSE patch-unavailable\n  the canonical BASE..HEAD patch could not be read within its bound, so no draft is printed\n  fix: run corvint dogfood change %s first to prepare .corvint/change.cem.json, then rerun this command\n", p.prefix, p.options.Base)
		p.exit(2)
	}
	hunks := make([]proposedHunk, len(parsed.Hunks))
	for index, hunk := range parsed.Hunks {
		hunks[index] = proposedHunk{index + 1, hunk.ID, hunk.DisplayPath, "unknown", "", int(hunk.OldRange.Start), int(hunk.OldRange.Count)}
	}
	p.keyByID = true
	rows, notes := p.draft(hunks)
	p.say("%s: PROPOSED rows=%d hunks=%d base=%s map=%s keyed=hunk-id%s\n", p.prefix, len(rows), len(hunks), p.base, mapState, p.refreshLabel())
	p.say("  no usable map is prepared, so the hunks come from the canonical BASE..HEAD patch and each row is keyed by its full hunk ID, valid in any map order; no probe bind pass is needed, and the bind's plan precheck still judges every row (DCW-V0-350)\n")
	p.say("  this draft is not a plan: review every row, replace each with the evidence the hunk really depends on or implements, then write it to the DOGFOOD_CITATIONS file; nothing is cited until dogfood change runs with that file (DCW-V0-150)\n")
	p.printDraft(hunks, rows, notes)
	return 0
}

// preparedHunks reads a prepared map's base revision and its hunks in map order,
// or reports it unreadable.
func preparedHunks(data []byte) (string, []proposedHunk, bool) {
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
		return "", nil, false
	}
	hunks := make([]proposedHunk, len(document.Hunks))
	for index, hunk := range document.Hunks {
		hunks[index] = proposedHunk{index + 1, hunk.ID, hunk.Path, hunk.Disposition, hunk.Reason, hunk.OldRange.Start, hunk.OldRange.Count}
	}
	return document.BaseRevision, hunks, true
}

// editedRanges returns each path's hunk old ranges, which no stable span may touch.
func editedRanges(hunks []proposedHunk) map[string][][2]int {
	edited := map[string][][2]int{}
	for _, hunk := range hunks {
		edited[hunk.path] = append(edited[hunk.path], [2]int{hunk.oldStart, hunk.oldStart + hunk.oldCount})
	}
	return edited
}

// draft returns the proposed rows in worklist order and one note per cite row
// that is not anchored, saying why. Only hunks still owed a row get one: an
// unknown hunk without a recorded unknown reason.
func (p *proposal) draft(hunks []proposedHunk) ([]string, []string) {
	edited := editedRanges(hunks)
	owed := []proposedHunk{}
	p.changed = map[string]bool{}
	for _, hunk := range hunks {
		p.changed[hunk.path] = true
		if hunk.disposition == "unknown" && !recordedUnknownReasons[hunk.reason] {
			owed = append(owed, hunk)
		}
	}
	// The inventory is git's name list, not the parsed patch, so a mode-only
	// change still counts; without it no file is provably untouched, so the
	// draft cites nothing (DCW-V0-393).
	names, status, bounded := p.boundedGit("diff", "--name-only", "-z", "--no-renames", "--end-of-options", p.base, p.target, "--")
	for _, name := range strings.Split(names, "\x00") {
		if name != "" {
			p.changed[name] = true
		}
	}
	p.prepareAnchors(owed)
	if status != 0 || !bounded {
		p.matches = map[string][]spanLine{}
		p.changed[sealContract] = true
	}
	used := map[string]bool{}
	p.carry(owed, used)
	rows, notes := []string{}, append([]string{}, p.carryNotes...)
	cited := 0
	lastCite := ""
	uniform := true
	for _, hunk := range owed {
		if carried, ok := p.carried[hunk.id]; ok {
			rows = append(rows, carried...)
			uniform = false
			continue
		}
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
	return append(rows, p.refreshDeferred...), notes
}

// propose returns one row for hunk, the PATH:START:END it cites, or "" for an
// unknown row, and, for a cite row that is not anchored, why it fell back.
func (p *proposal) propose(hunk proposedHunk, edited map[string][][2]int, used map[string]bool) (string, string, string) {
	ordinal := fmt.Sprint(hunk.ordinal)
	if p.keyByID {
		ordinal = hunk.id
	}
	if workflow.GoverningInstructionPath(hunk.path) {
		return unknownProposal(ordinal, hunk.path+" is a governing file this change modifies, so it cannot govern its own hunk (CEM-V1-114)"), "", ""
	}
	if bookkeepingPath(hunk.path) {
		return unknownProposal(ordinal, hunk.path+" is a bookkeeping record (DCW-V0-333); name the BASE contract the record describes"), "", ""
	}
	// Checked before any anchor: a BASE line that happens to name the path
	// must not displace the mandatory unknown row (DCW-V0-440).
	if id, where := p.sameChangeRequirement(p.derivedHunk(hunk)); id != "" {
		return unknownProposal(ordinal, "the hunk names requirement "+id+", which this change itself adds in "+where+
			", and same-change intent is not BASE evidence (cem cite cites BASE spans only), so it cannot be cited (DCW-V0-440): keep this row unknown, or cite the BASE contract "+id+" refines"), "", ""
	}
	if row, cite, note := p.sealedProposal(hunk, ordinal, edited, used); row != "" {
		return row, cite, note
	}
	if row, cite := p.anchoredProposal(hunk, ordinal, edited, used); row != "" {
		return row, cite, ""
	}
	// A draft cites only files the change leaves untouched (LANE-ORCHESTRATION
	// §3): the hunk's own file is modified, so no nearby line of it is drafted,
	// and a hunk with no anchor elsewhere is unknown (DCW-V0-393).
	missing := "no stable BASE line of a file this change leaves untouched names " + hunk.path + " or defines a requirement ID the hunk adds"
	if _, ok := p.blob(p.base, hunk.path); !ok {
		// An added file has no BASE line of its own; it reaches the verdict
		// gate through a cite row plus a passed runner receipt (DCW-V0-395).
		return unknownProposal(ordinal, hunk.path+" is absent at BASE and "+missing+"; name the BASE contract this new file implements, and bind a passed runner receipt for it (CEM-V1-606)"), "", ""
	}
	// The nearest stable heading or declaration of the modified file is named
	// as a review hint only, never drafted (DCW-V0-439).
	hint := ""
	if lines := p.baseLines(hunk.path); len(lines) > 0 {
		anchor := min(max(hunk.oldStart, 1), len(lines))
		relation, preferred := proposalRelation(hunk.path), preferredAnchor(hunk.path)
		candidates := []int{}
		for line := anchor; line >= 1; line-- {
			if preferred(string(lines[line-1])) {
				candidates = append(candidates, line)
			}
		}
		for distance := 1; distance <= len(lines); distance++ {
			candidates = append(candidates, anchor-distance, anchor+hunk.oldCount+distance-1)
		}
		for _, line := range candidates {
			if p.stable(hunk.path, line, edited) {
				hint = fmt.Sprintf("; its nearest stable line %s %d:%d %s is a review hint only (DCW-V0-439)", hunk.path, line, line, relation)
				break
			}
		}
	}
	return unknownProposal(ordinal, missing+"; the change modifies "+hunk.path+", so its own lines are not proposed"+hint+"; name the BASE contract the hunk depends on or implements"), "", ""
}

// sealContract is the BASE document whose lines define the sealed CEM path
// `.corvint/changes/<bind-commit>.cem.json` that dogfood-seal writes.
const sealContract = "docs/DOGFOOD.md"

// sealedProposal drafts the stacked-repair row (DCW-V0-385): when a commit
// follows a seal, BASE..HEAD adds that seal's CEM as a new file, which no BASE
// line can anchor. Its row cites the first stable BASE line of sealContract
// that defines the sealed path and no other row cites, so a second stacked
// seal gets a distinct span. It returns "" for any other hunk, a sealed file
// already present at BASE, or when no such line is stable.
func (p *proposal) sealedProposal(hunk proposedHunk, ordinal string, edited map[string][][2]int, used map[string]bool) (string, string, string) {
	if !sealedCEMPath.MatchString(hunk.path) || p.changed[sealContract] {
		return "", "", ""
	}
	if _, atBase := p.blob(p.base, hunk.path); atBase {
		return "", "", ""
	}
	relation := proposalRelation(sealContract)
	for index, text := range p.baseLines(sealContract) {
		line := index + 1
		cite := fmt.Sprintf("%s:%d:%d", sealContract, line, line)
		if !bytes.Contains(text, []byte(".corvint/changes/<bind")) || used[cite] || !p.stable(sealContract, line, edited) {
			continue
		}
		return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, sealContract, line, line, relation), cite,
			hunk.path + " is a prior seal's CEM a later commit carries into BASE..HEAD; cited to the seal contract that writes it (DCW-V0-385)"
	}
	return "", "", ""
}

// anchoredProposal returns the first row, on a span no other row cites, whose stable
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
	definitions := len(candidates)
	families := p.hunkFamilies(derived)
	candidates = append(candidates, p.matches[anchorPath(derived)]...)
	if name := anchorBasename(derived); p.basenames && name != "" {
		candidates = append(candidates, p.matches[name]...)
	}
	for index, candidate := range candidates {
		cite := fmt.Sprintf("%s:%d:%d", candidate.path, candidate.line, candidate.line)
		if used[cite] || !p.stable(candidate.path, candidate.line, edited) {
			continue
		}
		lines := p.baseLines(candidate.path)
		// The only line of a file, such as a one-line JSON document, cites the
		// whole file, which says nothing about this hunk (DCW-V0-440).
		if !p.basenames && len(lines) == 1 {
			continue
		}
		text := lines[candidate.line-1]
		if kinds, _ := verify.SpanAnchors(derived, text); len(kinds) == 0 {
			continue
		}
		// A path match inside a longer path, such as README.md within
		// docs/specs/README.md, names another file (DCW-V0-381).
		if index >= definitions && !p.basenames && !namesWholePath(text, anchorPath(derived)) {
			continue
		}
		// A hunk that names a requirement family is anchored only by a
		// line of its dominant family, never by another spec's row, a bare
		// list naming its path, or the definition of an ID it only mentions
		// in passing (DCW-V0-404).
		if !p.basenames && !relevantRequirementLine(families, text) {
			continue
		}
		relation := proposalRelation(candidate.path)
		return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, candidate.path, candidate.line, candidate.line, relation), cite
	}
	return "", ""
}

// requirementToken is one requirement ID, such as DCW-V0-402, whose family
// is the prefix before its final number, DCW-V0.
var requirementToken = regexp.MustCompile(`\b[A-Z][A-Z0-9-]{2,31}-[0-9]{3}\b`)

// hunkText returns a hunk's added and removed lines and its context lines.
func hunkText(hunk *patch.Hunk) (edits, around []byte) {
	for _, line := range hunk.Body {
		if line.Prefix == ' ' {
			around = append(append(around, line.Payload...), '\n')
		} else {
			edits = append(append(edits, line.Payload...), '\n')
		}
	}
	return edits, around
}

// dominantFamilies returns the requirement families text names most often,
// so a status line that mentions a few other specs' IDs still anchors to its
// own family.
func dominantFamilies(text []byte) map[string]bool {
	counts, most := map[string]int{}, 0
	for _, id := range requirementToken.FindAll(text, -1) {
		family := string(id[:bytes.LastIndexByte(id, '-')])
		counts[family]++
		most = max(most, counts[family])
	}
	families := map[string]bool{}
	for family, count := range counts {
		if count == most {
			families[family] = true
		}
	}
	return families
}

// hunkFamilies returns the requirement families a hunk implements: those its
// added and removed lines name, else its context lines, else the edits to the
// same file, else the whole change's edits. It is empty when nothing names one.
func (p *proposal) hunkFamilies(hunk *patch.Hunk) map[string]bool {
	if hunk == nil {
		return nil
	}
	edits, around := hunkText(hunk)
	if changed := dominantFamilies(edits); len(changed) > 0 {
		return changed
	}
	if context := dominantFamilies(around); len(context) > 0 {
		return context
	}
	// The fallbacks count every ID of the other edits together, so the
	// family they name most often wins (DCW-V0-404).
	var file, all []byte
	for key, other := range p.derived {
		if key != other.ID {
			continue
		}
		edits, _ := hunkText(other)
		all = append(all, edits...)
		if other.DisplayPath == hunk.DisplayPath {
			file = append(file, edits...)
		}
	}
	if families := dominantFamilies(file); len(families) > 0 {
		return families
	}
	return dominantFamilies(all)
}

// relevantRequirementLine reports whether a BASE line naming a hunk's path
// may anchor its draft: any such line when the hunk names no requirement
// family, else only a line carrying an ID of one of them (DCW-V0-404).
func relevantRequirementLine(families map[string]bool, text []byte) bool {
	if len(families) == 0 {
		return true
	}
	for _, id := range requirementToken.FindAll(text, -1) {
		if families[string(id[:bytes.LastIndexByte(id, '-')])] {
			return true
		}
	}
	return false
}

// prepareAnchors reads the canonical BASE..HEAD patch the map was derived
// from and, in one `git grep` of BASE, the lines naming each owed hunk's path
// or defining a requirement ID it adds. An unreadable or oversized read
// leaves no candidates, and the drafts fall back to nearby lines; it reports
// whether both reads completed.
func (p *proposal) prepareAnchors(owed []proposedHunk) bool {
	p.derived, p.matches, p.changeIDs = map[string]*patch.Hunk{}, map[string][]spanLine{}, map[string]string{}
	complete := false
	if raw, status, bounded := p.boundedGit(p.canonicalDiffArguments()...); status == 0 && bounded {
		if parsed, err := patch.Parse([]byte(raw)); err == nil {
			complete = true
			for _, hunk := range parsed.Hunks {
				p.derived[hunk.ID] = hunk
				p.derived[rangeKey(hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count)] = hunk
				if p.changed != nil {
					p.changed[hunk.DisplayPath], p.changed[anchorPath(hunk)] = true, true
				}
				for _, id := range addedDefinitions(hunk) {
					if _, seen := p.changeIDs[id]; !seen {
						p.changeIDs[id] = hunk.DisplayPath
					}
				}
			}
		}
	}
	patterns := map[string]bool{}
	for _, hunk := range owed {
		derived := p.derivedHunk(hunk)
		if name := anchorPath(derived); name != "" {
			patterns[name] = true
		}
		if name := anchorBasename(derived); p.basenames && name != "" {
			patterns[name] = true
		}
		_, ids := verify.SpanAnchors(derived, nil)
		for _, id := range ids {
			patterns[definitionPattern(id)] = true
		}
	}
	if len(patterns) == 0 {
		return complete
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
	arguments = append(arguments, p.base, "--", ".", ":(exclude).corvint/",
		":(exclude).taskman/", ":(exclude)docs/build-log/", ":(exclude)docs/BUILD-LOG.md")
	if !p.basenames {
		// Only drafts skip data; the check's anchor search is unchanged.
		arguments = append(arguments, ":(exclude)benchmarks/", ":(exclude,glob)**/testdata/**",
			":(exclude)docs/specs/INDEX.json", ":(exclude)docs/specs/REQUIREMENTS.tsv",
			":(exclude,glob)**/raw/**", ":(exclude,glob)**/*.stdout", ":(exclude,glob)**/*.stderr")
	}
	out, status, bounded := p.boundedGit(arguments...)
	// git grep exits 1 when nothing matches: a complete read with no candidates.
	if status > 1 || !bounded {
		return false
	}
	for _, record := range strings.Split(out, "\n") {
		name, rest, named := strings.Cut(strings.TrimPrefix(record, p.base+":"), "\x00")
		number, text, numbered := strings.Cut(rest, "\x00")
		line, err := strconv.Atoi(number)
		if !named || !numbered || err != nil || line < 1 || workflow.GoverningInstructionPath(name) || strings.HasPrefix(name, ".corvint/") || bookkeepingPath(name) || !p.basenames && (dataPath(name) || p.changed[name]) {
			continue
		}
		for _, pattern := range sorted {
			if strings.Contains(text, pattern) {
				p.matches[pattern] = append(p.matches[pattern], spanLine{name, line})
			}
		}
	}
	// Specifications first, then other documents; git grep's path order
	// within each (DCW-V0-381). A draft has already dropped every path the
	// change touches (DCW-V0-393).
	for _, found := range p.matches {
		sort.SliceStable(found, func(i, j int) bool { return documentRank(found[i].path) < documentRank(found[j].path) })
	}
	return complete
}

// bookkeepingPath reports a path a draft never cites (DCW-V0-333): the task
// store under .taskman/, the closed docs/BUILD-LOG.md and the build-log
// entries under docs/build-log/. They record work rather than state a contract,
// and a lane's own diff usually touches them.
func bookkeepingPath(name string) bool {
	return strings.HasPrefix(name, ".taskman/") || name == "docs/BUILD-LOG.md" || strings.HasPrefix(name, "docs/build-log/")
}

// dataPath reports fixture and generated data a draft never cites as an
// anchor (DCW-V0-381): benchmark corpora under benchmarks/, test fixtures
// under any testdata/ directory, and the generated spec indexes. A path such
// data names, like a corpus row at benchmarks/untouched-repository-v1/
// corpus.json, is data, not a contract for the hunk. Recorded command output
// is data too (DCW-V0-389): any file under a raw/ directory and any .stdout
// or .stderr file, such as an inventory listing of every path in the tree.
// So are JSON documents and retained agent evidence (DCW-V0-440): a
// worklist, a migration table or an .agent-evidence/ capture names paths as
// records, not as a contract for the hunk.
func dataPath(name string) bool {
	return strings.HasPrefix(name, "benchmarks/") || strings.HasPrefix(name, "testdata/") || strings.Contains(name, "/testdata/") ||
		name == "docs/specs/INDEX.json" || name == "docs/specs/REQUIREMENTS.tsv" ||
		strings.HasPrefix(name, "raw/") || strings.Contains(name, "/raw/") || strings.HasSuffix(name, ".stdout") || strings.HasSuffix(name, ".stderr") ||
		strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonl") || strings.HasPrefix(name, ".agent-evidence/")
}

// namesWholePath reports whether text names name as a whole path: some
// occurrence is neither continued by a path byte nor preceded by one, so a
// longer path that ends in name does not count (DCW-V0-381).
func namesWholePath(text []byte, name string) bool {
	pathByte := func(b byte) bool {
		return b == '/' || b == '.' || b == '-' || b == '_' || b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
	}
	for offset := 0; name != ""; {
		found := bytes.Index(text[offset:], []byte(name))
		if found < 0 {
			return false
		}
		start, end := offset+found, offset+found+len(name)
		if (start == 0 || !pathByte(text[start-1])) && (end == len(text) || !pathByte(text[end]) || text[end] == '.' && (end+1 == len(text) || !pathByte(text[end+1]))) {
			return true
		}
		offset = start + 1
	}
	return false
}

// canonicalDiffArguments are the canonical patch arguments of `cem` map
// derivation (internal/cem/gitauth), so the parsed hunk IDs are the map's.
func (p *proposal) canonicalDiffArguments() []string {
	return p.canonicalDiffArgumentsFor(p.base, p.target)
}

// canonicalDiffArgumentsFor is the canonical patch of base..target, which
// --refresh also reads for earlier commits (DCW-V0-442).
func (p *proposal) canonicalDiffArgumentsFor(base, target string) []string {
	format, _ := p.git(true, "rev-parse", "--show-object-format")
	empty := map[string]string{
		"sha1":   "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
		"sha256": "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321",
	}[chomp(format)]
	return []string{"--attr-source=" + empty, "diff", "--full-index", "--no-color", "--no-ext-diff", "--no-textconv",
		"--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers", "--unified=3", "--inter-hunk-context=0",
		"--src-prefix=a/", "--dst-prefix=b/", "--ignore-submodules=none", "--end-of-options", base, target,
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

// anchorBasename is the basename of the hunk path when it is long enough to
// earn a CEM-V1-081 path anchor on its own (6 bytes or more), else "".
func anchorBasename(hunk *patch.Hunk) string {
	name := anchorPath(hunk)
	if name = name[strings.LastIndexByte(name, '/')+1:]; len(name) >= 6 {
		return name
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

// proposalRelation names the relation a draft row uses for path.
func proposalRelation(name string) string {
	switch {
	case strings.HasPrefix(name, "docs/decisions/") || strings.HasPrefix(name, "docs/build-log/"):
		return "decision"
	case strings.EqualFold(path.Ext(name), ".md"):
		return "specification"
	case strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_test.sh"):
		return "test-claim"
	default:
		return "dependency"
	}
}

// preferredAnchor reports the lines preferred as a review hint for name: the
// enclosing heading of a document, the enclosing declaration of source code.
func preferredAnchor(name string) func(string) bool {
	if strings.EqualFold(path.Ext(name), ".md") {
		return func(line string) bool { return strings.HasPrefix(line, "#") }
	}
	return declaration
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
