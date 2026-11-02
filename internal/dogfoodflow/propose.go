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

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
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
	// FromCEM names a sealed CEM whose hunks seed the refresh rows in place
	// of a --refresh plan (DCW-V0-843).
	FromCEM string
}

// proposal drafts a per-hunk citation plan from the prepared map (DCW-V0-150).
type proposal struct {
	flow
	options ProposeOptions
	stdout  io.Writer
	// siblingDirs caches each parent's BASE directories (DCW-V0-841).
	siblingDirs map[string][]string
	// priorBase is the BASE the --refresh plan was drafted at when it is not
	// BASE, the map's baseRevision; carriedFiles holds the first cite row
	// carried to each file (DCW-V0-842).
	priorBase    string
	carriedFiles map[string]string
	// cemBase is the --from-cem CEM's baseRevision (DCW-V0-843).
	cemBase string
	blobs   map[string][]byte
	lines   map[string][][]byte
	// batchBlobs reads uncached blobs through one cat-file --batch reader,
	// batch, which the flow boundary closes (CKC-V0-015).
	batchBlobs bool
	batch      *blobBatch
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
	// staleIDs counts the --refresh rows keyed by a hunk ID the current patch
	// lacks, and redrafted the owed hunks no row was carried to, which a
	// finish-lane auto-refresh reads (DCW-V0-947).
	staleIDs  int
	redrafted []proposedHunk
	// oversized holds each PATH:LINE:LINE candidate whose BASE line is longer
	// than cem cite's evidence span bound, with its byte length; the draft
	// never cites one and names each it skipped (DCW-V0-920).
	oversized map[string]int
	// refreshPlan is the --refresh plan as read; refreshDeferred holds its
	// validated deferred rows, carried as written (DCW-V0-370).
	refreshPlan     []byte
	refreshDeferred []string
	// moves maps each side of a file the change moves, the deleted BASE path
	// and the added path, to that move (DCW-V0-803).
	moves map[string]movedFile
	// decisions maps each BASE decision number to its path, read once
	// (DCW-V0-805).
	decisions map[string]string
	// proposals holds, by row key, each drafted row that cites the nearest
	// stable BASE span of the modified file its hunk edits; stderr marks each
	// one for the author to confirm or replace (DCW-V0-962).
	proposals map[string]bool
	// refines maps each requirement ID this change defines to the BASE IDs
	// its entry says it refines or amends (DCW-V0-967); inventoryRead is set
	// once git's changed-path list was read whole (DCW-V0-966).
	refines       map[string][]string
	inventoryRead bool
	// reasons holds, by row key, why a drafted proposal was chosen.
	reasons map[string]string
	// governing holds the change's governing specs, which path anchors
	// prefer; weak holds the low-confidence note of each row whose anchor is
	// a bare mention outside them (DCW-V0-886).
	governing map[string]bool
	weak      []string
	// modifiedOwners holds, by pattern, the specs this change modifies whose
	// BASE lines name it, counted as governing specs; their stable lines are
	// cited like an untouched spec's, never a line the change edits (DCW-V0-886).
	modifiedOwners map[string][]string
	// claims is the --refresh file when it is a content-keyed claims file;
	// patchHunks are the canonical patch's hunks in order, tombstones the
	// removed cite rows of each hunk's claims, and claimStats count the
	// refresh (CKC-V0-004, CKC-V0-007, CKC-V0-010).
	claims     *claimsFile
	patchHunks []*patch.Hunk
	tombstones map[string]map[string]bool
	claimStats claimsStats
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
		options: options, stdout: stdout, blobs: map[string][]byte{}, lines: map[string][][]byte{}, batchBlobs: batchBlobReads,
	}
	return boundary(ctx, run.run, run.closeBlobBatch)
}

func (p *proposal) run() int {
	p.requireRoot()
	p.requireTrustedAncestry()
	p.readRefreshPlan()
	// An abbreviated BASE resolves to its full ID; one that names no commit,
	// or more than one, is refused by name with its fix (DCW-V0-901).
	base, status := p.git(true, "rev-parse", "--verify", "-q", p.options.Base+"^{commit}")
	if status != 0 {
		p.say("%s: REFUSE base-unavailable BASE=%s\n", p.prefix, p.options.Base)
		p.say("  fix: pass BASE as a commit of this repository, preferably the full origin/main ID (git rev-parse --verify %s^{commit} names it, or says the prefix is unknown or ambiguous), then rerun corvint dogfood propose-citations BASE (DCW-V0-901)\n", p.options.Base)
		p.exit(2)
	}
	p.base = chomp(base)
	p.target = p.gitValue("rev-parse", "HEAD^{commit}")
	p.requireBaseAncestor(p.base, p.target)
	if p.cemBase != "" {
		// The sealed CEM's base is the BASE its rows were cited at (DCW-V0-843).
		if prior, status := p.git(true, "rev-parse", "--verify", "-q", p.cemBase+"^{commit}"); status == 0 {
			p.priorBase = chomp(prior)
		}
	}
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
		if p.options.Refresh != "" && p.options.FromCEM == "" {
			// The map's BASE is the plan's BASE only when the last bind
			// recorded this plan against this map; a plan refreshed since
			// already cites another BASE, and re-pinning it from the map's
			// would move valid spans onto other text (DCW-V0-842).
			if !p.boundToMap(hunks) {
				p.say("%s: NOTE refresh-base-unproven\n  no citation-plan binding records this --refresh plan against the map, so its BASE is not proven to be the map's baseRevision=%s; cite rows are carried as written, not re-pinned, and the vet judges them at BASE %s (DCW-V0-842)\n", p.prefix, baseRevision, p.base)
			} else if prior, status := p.git(true, "rev-parse", "--verify", "-q", baseRevision+"^{commit}"); status == 0 {
				p.priorBase = chomp(prior)
			}
		}
		p.say("%s: NOTE map-base-mismatch\n  map baseRevision=%s BASE=%s, so the draft is keyed by the current patch's hunk IDs against BASE; dogfood change BASE prepares a fresh map when it binds (DCW-V0-394)\n", p.prefix, baseRevision, p.base)
		return p.template("OTHER_BASE")
	}
	if p.refreshing() {
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
	p.say("  line basis: every START:END in a plan row is a BASE line number, counted in each file as it is at BASE %s, not at HEAD or in the working tree (DCW-V0-845)\n", p.base)
	p.printDraft(hunks, rows, notes)
	return 0
}

// printDraft prints the notes and, on stderr, which file and BASE range each
// drafted row's hunk edits, so a reviewer need not rebuild the mapping from
// git diff; the rows themselves keep the four-field plan grammar (DCW-V0-382).
func (p *proposal) printDraft(hunks []proposedHunk, rows, notes []string) {
	if len(p.proposals) > 0 {
		p.say("  proposals=%d rows are drafted proposals, each marked drafted proposal below with why it was chosen; review each before binding (DCW-V0-962, DCW-V0-967)\n", len(p.proposals))
	}
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
			p.say("  hunk %s edits %s%s\n", key, p.hunkSpan(hunk), p.refreshMark(hunk))
		}
		if reason := p.reasons[key]; p.proposals[key] && key != "" && reason != "" {
			p.say("  drafted proposal: %s\n    %s\n", row, reason)
		} else if p.proposals[key] && key != "" {
			p.say("  drafted proposal: %s\n    the nearest stable BASE span of the file this hunk edits, a same-file proposal rather than an anchor (DCW-V0-393): confirm it, or replace it with the BASE contract the hunk depends on or implements, before binding (DCW-V0-962)\n", row)
		}
		_, _ = fmt.Fprintln(p.stdout, row)
	}
}

// canonicalPatch reads and parses the canonical BASE..HEAD patch the map is
// derived from, or reports it unreadable within its bound.
func (p *proposal) canonicalPatch() (*patch.Patch, bool) {
	parsed, _, _ := p.readCanonicalPatch()
	return parsed, parsed != nil
}

// readCanonicalPatch reads and parses the canonical patch, or names why it
// cannot: the reason is git-diff-failed, or the cem/1.0 code cem prepare
// refuses the same patch with, and detail is that refusal's message
// (DCW-V0-802). A patch over cem prepare's Git output bound,
// patch.MaxPatchBytes, is git-output-exceeded as there, not the binary-patch
// code the parser gives an oversized input.
func (p *proposal) readCanonicalPatch() (parsed *patch.Patch, reason, detail string) {
	raw, status, bounded := p.boundedGit(p.canonicalDiffArguments()...)
	switch {
	case !bounded, len(raw) > patch.MaxPatchBytes:
		return nil, cemcode.GitOutputExceeded, fmt.Sprintf("the BASE..HEAD patch exceeds the %d-byte Git output bound cem prepare reads it under", patch.MaxPatchBytes)
	case status != 0:
		return nil, "git-diff-failed", fmt.Sprintf("git diff BASE HEAD exited %d", status)
	case raw == "":
		return new(patch.Patch), "", ""
	}
	parsed, err := patch.Parse([]byte(raw))
	if err != nil {
		reason = cemcode.CodeOf(err)
		if reason == "" {
			reason = "patch-unparsable"
		}
		return nil, reason, err.Error()
	}
	return parsed, "", ""
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
	parsed, reason, detail := p.readCanonicalPatch()
	if parsed == nil {
		p.refusePatchUnavailable(reason, detail)
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
	p.say("  line basis: every START:END in a plan row is a BASE line number, counted in each file as it is at BASE %s, not at HEAD or in the working tree (DCW-V0-845)\n", p.base)
	p.printDraft(hunks, rows, notes)
	return 0
}

// refusePatchUnavailable refuses a map-free draft whose canonical patch
// cannot be read or parsed (DCW-V0-802). cem prepare derives the map from the
// same patch with the same parser, so running dogfood change first cannot
// help: the refusal names why, what to change, and the first-run order, in
// which propose-citations is the first command and needs only BASE and a
// committed HEAD.
func (p *proposal) refusePatchUnavailable(reason, detail string) {
	base := p.options.Base
	p.say("%s: REFUSE patch-unavailable reason=%s\n", p.prefix, reason)
	p.say("  %s; no draft is printed and nothing was written\n", detail)
	switch reason {
	case "git-diff-failed":
		p.say("  fix: make BASE a commit of this repository that HEAD contains (git merge-base --is-ancestor %s HEAD), then rerun corvint dogfood propose-citations %s\n", base, base)
	case cemcode.GitOutputExceeded:
		p.say("  fix: cem prepare refuses a patch this large as well, so dogfood change cannot run first; split the change into smaller commits on separate branches, then rerun corvint dogfood propose-citations %s\n", base)
	default:
		// A binary file beside a textual hunk binds since CEM-V1-980; only the
		// shapes cem prepare still refuses are named (DCW-V0-902).
		p.say("  fix: cem prepare refuses this patch with the same code, so dogfood change and dogfood finish-lane cannot run first; give an empty file content, revert a mode-only change and drop a symbolic link; a binary file beside a textual hunk binds (CEM-V1-980), so change only a binary-only patch (add a textual hunk), a binary rename or copy (commit it as a delete and an add), a binary mode change or a GIT binary patch body or abbreviated object IDs (a git diff.binary or diff.noprefix setting); commit, then rerun corvint dogfood propose-citations %s\n", base)
	}
	p.say("  prerequisite: propose-citations is the first command of the closeout and needs only BASE and a committed HEAD; no prepared map, DOGFOOD_INTENTS_FILE, DOGFOOD_VERIFY_FILE or DOGFOOD_OUTCOME (DCW-V0-350)\n")
	p.say("  next: write the reviewed draft to plan.tsv, then make dogfood-inputs-check BASE=%s CITATIONS=plan.tsv INTENTS=<intents.txt> VERIFY=<verify.txt> OUTCOME=passed|failed|blocked, then make dogfood-finish with the same inputs (docs/DOGFOOD.md §4 first-run order)\n", base)
	p.exit(2)
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
	p.proposals, p.reasons = map[string]bool{}, map[string]string{}
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
	p.inventoryRead = status == 0 && bounded
	for _, name := range strings.Split(names, "\x00") {
		if name != "" {
			p.changed[name] = true
		}
	}
	complete, patched := p.parseAnchors()
	used := map[string]bool{}
	p.carry(owed, used)
	// The anchor search and the governing specs serve only drafted hunks, so
	// a refresh that carries every owed hunk skips both (CKC-V0-014).
	skip := allCarried(p, owed)
	if !skip {
		p.searchAnchors(owed, complete, patched)
	}
	if status != 0 || !bounded {
		p.matches = map[string][]spanLine{}
		p.changed[sealContract] = true
	}
	if !skip {
		p.governing = p.governingSpecs(owed)
	}
	rows, notes := []string{}, append([]string{}, p.carryNotes...)
	cited := 0
	lastCite := ""
	uniform := true
	for _, hunk := range owed {
		// A prior seal's CEM carried as unknown is redrafted (DCW-V0-966).
		if carried, ok := p.carriedRows(hunk); ok {
			rows = append(rows, carried...)
			uniform = false
			continue
		}
		// A seal no change is stacked on is replaced by the next bind, not
		// carried, so its CEM gets no drafted row (ASD-V0-843).
		if p.resealsSameContent(hunk.path) {
			notes = append(notes, fmt.Sprintf("hunk %s of %s drafted no row: no change is stacked on that seal, so a bind here reseals the same content and replaces its CEM rather than citing it (ASD-V0-843)", hunk.id, hunk.path))
			continue
		}
		if p.carried != nil {
			// Drafted afresh: no row was carried, or a carried all-unknown row
			// on a sealed CEM is redrafted (DCW-V0-966). A carried-forward
			// candidate still needs review, so it counts as redrafted too.
			p.redrafted = append(p.redrafted, hunk)
		}
		row, cite, fellBack := "", "", ""
		ordinal := fmt.Sprint(hunk.ordinal)
		if p.keyByID {
			ordinal = hunk.id
		}
		if row, cite = p.carriedForward(hunk, ordinal); row == "" {
			row, cite, fellBack = p.propose(hunk, edited, used)
			// A drafted cite the author removed for this edit is never drafted
			// again; the hunk keeps the unknown fallback (CKC-V0-007).
			if cite != "" && p.tombstoned(hunk.id, row) {
				key, _, _ := strings.Cut(row, "\t")
				delete(p.proposals, key)
				delete(p.reasons, key)
				fields := strings.Split(row, "\t")
				row = p.unknownProposal(hunk, key, "the drafted row cites "+fields[1]+" "+fields[2]+" ("+fields[3]+"), which the author removed for this edit (CKC-V0-007); cite the BASE contract the hunk depends on or implements, or keep it unknown")
				cite, fellBack = "", ""
				p.claimStats.suppressed++
			}
		}
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
		delete(p.proposals, ordinal)
		rows[last] = p.unknownProposal(owed[len(owed)-1], ordinal, "no second stable span was found near this hunk")
	}
	rows, refused := p.vetRows(owed, rows)
	notes = append(notes, refused...)
	notes = append(notes, p.weak...)
	for _, cite := range sortedKeys(p.oversized) {
		notes = append(notes, fmt.Sprintf("%s not cited: its BASE line is %d bytes, over the %d-byte evidence span cem cite admits (DCW-V0-920)", cite, p.oversized[cite], wire.MaxCitationSpanBytes))
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
		return p.unknownProposal(hunk, ordinal, hunk.path+" is a governing file this change modifies, so it cannot govern its own hunk (CEM-V1-114)"), "", ""
	}
	if bookkeepingPath(hunk.path) {
		return p.unknownProposal(hunk, ordinal, hunk.path+" is a bookkeeping record (DCW-V0-333); name the BASE contract the record describes"), "", ""
	}
	// Checked before any anchor: a BASE line that happens to name the path
	// must not displace the mandatory unknown row (DCW-V0-440).
	if id, where := p.sameChangeRequirement(p.derivedHunk(hunk)); id != "" {
		if row, cite := p.refinedProposal(hunk, ordinal, id, edited, used); row != "" {
			return row, cite, ""
		}
		// A new file following a BASE sibling's pattern is proposed that
		// sibling's contract (DCW-V0-841, amending DCW-V0-440).
		if row, cite := p.siblingProposal(hunk, ordinal, edited, used); row != "" {
			p.reasons[ordinal] = "the hunk names requirement " + id + ", which this change itself adds in " + where + " (DCW-V0-440); " + p.reasons[ordinal]
			return row, cite, ""
		}
		return p.unknownProposal(hunk, ordinal, "the hunk names requirement "+id+", which this change itself adds in "+where+
			", and same-change intent is not BASE evidence (cem cite cites BASE spans only), so it cannot be cited (DCW-V0-440): keep this row unknown, or cite the BASE contract "+id+" refines"), "", ""
	}
	// The defining spec's own hunk that adds a requirement, or only its
	// rows, has no BASE contract either; a path anchor would cite another
	// requirement's decision (DCW-V0-809).
	if id := p.addedRequirementRows(p.derivedHunk(hunk)); id != "" {
		// The BASE requirement the new entry refines, amends or names is a
		// drafted proposal; unknown only without one (DCW-V0-906).
		for _, defined := range append(p.definedRequirements(hunk), id) {
			if row, cite := p.refinedProposal(hunk, ordinal, defined, edited, used); row != "" {
				return row, cite, ""
			}
		}
		return p.unknownProposal(hunk, ordinal, "the hunk adds requirement "+id+" or rows naming it, which this change itself defines in "+hunk.path+
			", and same-change intent is not BASE evidence (cem cite cites BASE spans only), so it cannot be cited (DCW-V0-440, DCW-V0-809): keep this row unknown"), "", ""
	}
	if row, cite, note := p.sealedProposal(hunk, ordinal, edited, used); row != "" {
		return row, cite, note
	}
	if row, cite, note := p.movedProposal(hunk, ordinal, edited, used); row != "" {
		return row, cite, note
	}
	if row, cite, note := p.governingProposal(hunk, ordinal, edited, used); row != "" {
		return row, cite, note
	}
	row, cite, filtered := p.anchoredProposal(hunk, ordinal, edited, used)
	if row != "" {
		return row, cite, ""
	}
	if filtered != "" {
		// The set-aside candidates are named for a same-file proposal too.
		p.weak = append(p.weak, "row "+ordinal+": no anchor was drafted"+filtered)
	}
	// A draft anchors only on files the change leaves untouched (DCW-V0-393):
	// a hunk with no anchor elsewhere gets its own file's nearest stable line
	// as a marked proposal, or stays unknown (DCW-V0-962).
	missing := "no stable BASE line of another file names " + hunk.path + " or defines a requirement ID the hunk adds" + filtered
	if dataPath(hunk.path) {
		missing += "; a data record is never cited, so cite the BASE contract of the code that writes or reads it (DCW-V0-841)"
	}
	if row, cite := p.siblingProposal(hunk, ordinal, edited, used); row != "" {
		return row, cite, ""
	}
	if _, ok := p.blob(p.base, hunk.path); !ok {
		// An added file has no BASE line of its own; it reaches the verdict
		// gate through a cite row plus a passed runner receipt (DCW-V0-395).
		return p.unknownProposal(hunk, ordinal, hunk.path+" is absent at BASE and "+missing+"; name the BASE contract this new file implements, and bind a passed runner receipt for it (CEM-V1-606)"), "", ""
	}
	// The nearest stable heading or declaration of the modified file, on a
	// span no other row cites, is drafted as a marked proposal (DCW-V0-962,
	// amending DCW-V0-439 and DCW-V0-846). None is drafted for a data record
	// or a one-line file (DCW-V0-440); a hunk defining a same-change
	// requirement already stayed unknown above (DCW-V0-809), and one that
	// still defines a requirement here cites only what it refines.
	if defined := p.definedRequirements(hunk); len(defined) > 0 {
		for _, id := range defined {
			if row, cite := p.refinedProposal(hunk, ordinal, id, edited, used); row != "" {
				return row, cite, ""
			}
		}
		return p.unknownProposal(hunk, ordinal, "the hunk defines requirement "+strings.Join(defined, ", ")+", which is same-change intent, not BASE evidence (DCW-V0-440), and its entry names no BASE requirement it refines or amends with a stable definition line; cite the BASE contract it refines (DCW-V0-967)"), "", ""
	}
	if lines := p.baseLines(hunk.path); len(lines) > 1 && !dataPath(hunk.path) {
		anchor := min(max(hunk.oldStart, 1), len(lines))
		relation, preferred := proposalRelation(hunk.path), preferredAnchor(hunk.path)
		// A hunk editing a requirement's entry proposes that requirement's
		// own definition line first (DCW-V0-967).
		candidates := p.ownDefinitionLines(hunk)
		for line := anchor; line >= 1; line-- {
			if preferred(string(lines[line-1])) {
				candidates = append(candidates, line)
			}
		}
		for distance := 1; distance <= len(lines); distance++ {
			candidates = append(candidates, anchor-distance, anchor+hunk.oldCount+distance-1)
		}
		for _, line := range candidates {
			cite := fmt.Sprintf("%s:%d:%d", hunk.path, line, line)
			if !used[cite] && p.stable(hunk.path, line, edited) {
				p.proposals[ordinal] = true
				return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, hunk.path, line, line, relation), cite, ""
			}
		}
	}
	return p.unknownProposal(hunk, ordinal, missing+p.editedDefinitions(hunk, edited)+"; the change modifies "+hunk.path+" and no stable line of it is left to propose; name the BASE contract the hunk depends on or implements"), "", ""
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
	// DCW-V0-966: a modified docs/DOGFOOD.md still lends its stable lines;
	// only an unread changed-path inventory leaves none provably stable.
	if !sealedCEMPath.MatchString(hunk.path) || !p.inventoryRead {
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
	if row, cite := p.sealedFallback(ordinal, edited, used); row != "" {
		return row, cite, hunk.path + " is a prior seal's CEM a later commit carries into BASE..HEAD; no line of " + sealContract + " is stable, so it is cited to the DCW-V0-385 definition (DCW-V0-966)"
	}
	return "", "", ""
}

// resealsSameContent reports a sealed CEM path BASE..HEAD adds whose bind
// commit already holds HEAD's content outside .corvint/: nothing is stacked on
// that seal, so a bind of HEAD reseals the same content and replaces the CEM,
// which then is no part of the change to cite. A seal a later commit changes
// content on is a stacked rebind and keeps its DCW-V0-385 row (ASD-V0-843,
// V1-3961).
func (p *proposal) resealsSameContent(path string) bool {
	match := sealedCEMPath.FindStringSubmatch(path)
	if match == nil || p.target == "" {
		return false
	}
	if _, atBase := p.blob(p.base, path); atBase {
		return false
	}
	return p.gitSucceeds("diff", "--quiet", "--no-renames", "--end-of-options", match[1], p.target, "--", ".", ":(exclude).corvint/")
}

// anchoredProposal returns the first row, on a span no other row cites, whose stable
// BASE line earns hunk a CEM-V1-081 anchor (DCW-V0-161): the line defining a
// requirement ID the hunk adds, then a line naming the hunk path, a
// specification first. It returns "" when there is none, with a note naming
// the candidates DCW-V0-847 set aside.
func (p *proposal) anchoredProposal(hunk proposedHunk, ordinal string, edited map[string][][2]int, used map[string]bool) (string, string, string) {
	derived := p.derivedHunk(hunk)
	_, ids := verify.SpanAnchors(derived, nil)
	filtered := ""
	if !p.basenames {
		// An ID a test or data hunk names only as fixture text anchors
		// nothing (DCW-V0-847).
		var fixture []string
		if ids, fixture = p.ownRequirementIDs(hunk.path, ids); len(fixture) > 0 {
			filtered += "; " + hunk.path + " names " + strings.Join(fixture, ", ") + " only in test or fixture text, which is not an anchor (DCW-V0-847)"
		}
	}
	candidates := []spanLine{}
	for _, id := range ids {
		candidates = append(candidates, p.matches[definitionPattern(id)]...)
	}
	definitions := len(candidates)
	families, changeWide := p.hunkFamiliesScope(derived)
	if p.basenames {
		candidates = append(candidates, p.matches[anchorPath(derived)]...)
	} else {
		// A draft prefers the change's governing spec (DCW-V0-886); the
		// check's anchor search keeps its order.
		candidates = append(candidates, p.rankAnchors(p.matches[anchorPath(derived)])...)
	}
	if name := anchorBasename(derived); p.basenames && name != "" {
		candidates = append(candidates, p.matches[name]...)
	}
	for index, candidate := range candidates {
		// A large candidate set honours the deadline (DCW-V0-818).
		p.checkpoint()
		cite := fmt.Sprintf("%s:%d:%d", candidate.path, candidate.line, candidate.line)
		// The hunk's own file is a proposal, never an anchor (DCW-V0-962).
		if candidate.path == hunk.path || used[cite] || !p.stable(candidate.path, candidate.line, edited) {
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
		// A mention inside a test or data file is not an anchor (DCW-V0-847).
		if !p.basenames && testSourcePath(candidate.path) {
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
		// Families named only by the change's other files do not stop a line
		// of an untouched file naming the hunk's whole path from anchoring it
		// (DCW-V0-841).
		// DCW-V0-847: that admission never covers a line carrying another
		// requirement's IDs, such as a traceability row of another spec, and
		// a line naming requirement IDs anchors a hunk that names IDs only
		// when it carries one of them, never a neighbouring requirement's row.
		pathLine := index >= definitions && changeWide && !p.changed[candidate.path] && namesWholePath(text, anchorPath(derived)) &&
			!requirementToken.Match(text)
		if !p.basenames && !pathLine && !relevantRequirementLine(families, text) {
			continue
		}
		if index >= definitions && !p.basenames {
			if foreign := foreignRequirementLine(ids, text); foreign != "" {
				filtered += fmt.Sprintf("; %s:%d names %s but none of the hunk's %s, so it is another requirement's line (DCW-V0-847)", candidate.path, candidate.line, foreign, strings.Join(ids, ", "))
				continue
			}
		}
		relation := proposalRelation(candidate.path)
		row := fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, candidate.path, candidate.line, candidate.line, relation)
		if index >= definitions && !p.basenames && len(p.governing) > 0 && p.anchorRank(candidate) >= 2 {
			p.weak = append(p.weak, p.weakAnchorNote(row, candidate, anchorPath(derived)))
		}
		return row, cite, ""
	}
	return "", "", filtered
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
	families, _ := p.hunkFamiliesScope(hunk)
	return families
}

// hunkFamiliesScope is hunkFamilies, also reporting whether the families came
// only from the whole change's edits (DCW-V0-841).
func (p *proposal) hunkFamiliesScope(hunk *patch.Hunk) (map[string]bool, bool) {
	if hunk == nil {
		return nil, false
	}
	edits, around := hunkText(hunk)
	if changed := dominantFamilies(edits); len(changed) > 0 {
		return changed, false
	}
	if context := dominantFamilies(around); len(context) > 0 {
		return context, false
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
		return families, false
	}
	families := dominantFamilies(all)
	return families, len(families) > 0
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
	complete, patched := p.parseAnchors()
	return p.searchAnchors(owed, complete, patched)
}

// allCarried reports whether a refresh carries rows for every owed hunk, so
// draft proposes none and needs no anchor search (CKC-V0-014). A test
// replaces it to compare the output against the searching path.
var allCarried = func(p *proposal, owed []proposedHunk) bool {
	for _, hunk := range owed {
		if _, ok := p.carriedRows(hunk); !ok {
			return false
		}
	}
	return true
}

// parseAnchors reads the canonical BASE..HEAD patch, recording its hunks,
// the requirement IDs it adds and the paths it patches; it reports whether
// the read completed.
func (p *proposal) parseAnchors() (bool, map[string]bool) {
	p.derived, p.matches, p.changeIDs, p.refines = map[string]*patch.Hunk{}, map[string][]spanLine{}, map[string]string{}, map[string][]string{}
	p.modifiedOwners = map[string][]string{}
	complete := false
	patched := map[string]bool{}
	if raw, status, bounded := p.boundedGit(p.canonicalDiffArguments()...); status == 0 && bounded {
		// An empty diff is a patch with no hunks, as readCanonicalPatch has
		// it, not an unavailable one: every claimed edit was reverted, and
		// each claim is gone (CKC-V0-005).
		if raw == "" {
			complete, p.patchHunks = true, []*patch.Hunk{}
		} else if parsed, err := patch.Parse([]byte(raw)); err == nil {
			complete = true
			p.patchHunks = parsed.Hunks
			for _, hunk := range parsed.Hunks {
				p.checkpoint()
				p.derived[hunk.ID] = hunk
				patched[hunk.DisplayPath] = true
				p.derived[rangeKey(hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count)] = hunk
				if p.changed != nil {
					p.changed[hunk.DisplayPath], p.changed[anchorPath(hunk)] = true, true
				}
				for _, id := range addedDefinitions(hunk) {
					// A definition rewritten in place amends a requirement
					// BASE already defines; only a new one is same-change
					// intent (DCW-V0-440, DCW-V0-809).
					if _, seen := p.changeIDs[id]; !seen && !p.definedAtBase(hunk.DisplayPath, id) {
						p.changeIDs[id] = hunk.DisplayPath
					}
				}
				for id, refined := range refinedRequirements(hunk) {
					if _, added := p.changeIDs[id]; added {
						p.refines[id] = refined
					}
				}
			}
		}
	}
	return complete, patched
}

// draftDataExcludes are the rest of dataPath, which a draft drops from the
// search anyway, so git never reads them (CKC-V0-016). A test empties it to
// compare the unnarrowed search.
var draftDataExcludes = []string{":(exclude,glob)**/*.json", ":(exclude,glob)**/*.jsonl", ":(exclude).agent-evidence/"}

// searchAnchors runs the one `git grep` of BASE for the owed hunks' anchors;
// it reports whether the patch read (complete) and the search completed.
func (p *proposal) searchAnchors(owed []proposedHunk, complete bool, patched map[string]bool) bool {
	patterns := map[string]bool{}
	if !p.basenames {
		// Only drafts pair moves; the check's anchor search is unchanged.
		p.moves = movedFiles(p.derived)
		for _, move := range p.moves {
			for _, term := range move.terms() {
				patterns[term] = true
			}
		}
	}
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
	for _, refined := range p.refines {
		for _, id := range refined {
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
		arguments = append(arguments, draftDataExcludes...)
	}
	out, status, bounded := p.anchorGrep(arguments, sorted)
	// git grep exits 1 when nothing matches: a complete read with no candidates.
	if status > 1 || !bounded {
		return false
	}
	for _, record := range strings.Split(out, "\n") {
		p.checkpoint()
		name, rest, named := strings.Cut(strings.TrimPrefix(record, p.base+":"), "\x00")
		number, text, numbered := strings.Cut(rest, "\x00")
		line, err := strconv.Atoi(number)
		// DCW-V0-966 (amending DCW-V0-393): a modified file's BASE line is kept
		// as a candidate; stable drops every line a hunk edits or the change
		// repeats, so the draft never cites the change's own text.
		if !named || !numbered || err != nil || line < 1 || workflow.GoverningInstructionPath(name) || strings.HasPrefix(name, ".corvint/") || bookkeepingPath(name) || !p.basenames && dataPath(name) {
			continue
		}
		if !p.basenames && p.changed[name] {
			// A spec the change modifies may be the change's governing spec
			// (DCW-V0-886).
			if documentRank(name) == 0 && requirementToken.Match(p.lineText(spanLine{name, line})) {
				for _, pattern := range sorted {
					if strings.Contains(text, pattern) {
						p.modifiedOwners[pattern] = append(p.modifiedOwners[pattern], name)
					}
				}
			}
			// A path git lists as changed that the patch holds no hunk for (a
			// mode-only change) is still never cited (DCW-V0-966).
			if !patched[name] {
				continue
			}
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
	// A file the change leaves untouched ranks before a modified one
	// (DCW-V0-966).
	rank := func(name string) int {
		if p.changed[name] && !p.basenames {
			return documentRank(name) + 100
		}
		return documentRank(name)
	}
	for _, found := range p.matches {
		sort.SliceStable(found, func(i, j int) bool { return rank(found[i].path) < rank(found[j].path) })
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
	// cem cite refuses a span over wire.MaxCitationSpanBytes, such as the
	// only line of a large one-line JSON document (DCW-V0-920).
	if size := len(lines[line-1]); size > wire.MaxCitationSpanBytes {
		if p.oversized == nil {
			p.oversized = map[string]int{}
		}
		p.oversized[fmt.Sprintf("%s:%d:%d", name, line, line)] = size
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

// hunkSpan names the hunk's path, its BASE range and, when the canonical
// patch holds it, its new range at HEAD (DCW-V0-963).
// carriedRows returns the rows a refresh carries for hunk. A prior seal's CEM
// carried as unknown is redrafted instead (DCW-V0-966).
func (p *proposal) carriedRows(hunk proposedHunk) ([]string, bool) {
	carried, ok := p.carried[hunk.id]
	return carried, ok && !(sealedCEMPath.MatchString(hunk.path) && allUnknown(carried))
}

// refreshMark tells a refresh's reviewer whether a hunk's rows were carried
// from the earlier plan or claims or drafted afresh, so review can start with
// the drafted ones (CKC-V0-013). A plain draft has no mark.
func (p *proposal) refreshMark(hunk proposedHunk) string {
	if p.options.Refresh == "" {
		return ""
	}
	if _, ok := p.carriedRows(hunk); ok {
		return "; carried"
	}
	return "; drafted"
}

func (p *proposal) hunkSpan(hunk proposedHunk) string {
	span := fmt.Sprintf("%s at BASE %d,%d", hunk.path, hunk.oldStart, hunk.oldCount)
	if derived := p.derivedHunk(hunk); derived != nil && derived.Body != nil {
		span += fmt.Sprintf(" HEAD %d,%d", derived.NewRange.Start, derived.NewRange.Count)
	}
	return span
}

func (p *proposal) unknownProposal(hunk proposedHunk, ordinal, note string) string {
	// DCW-V0-963: the note names the file, BASE and HEAD range the hunk edits.
	note = "hunk edits " + p.hunkSpan(hunk) + "; " + note
	return ordinal + "\tunknown\tinsufficient-evidence\tproposed: " + strings.NewReplacer("\t", " ", "\n", " ").Replace(note)
}

// blob reads path at commit, or reports it absent or not a regular file.
func (p *proposal) blob(commit, name string) ([]byte, bool) {
	key := commit + ":" + name
	if data, ok := p.blobs[key]; ok {
		return data, data != nil
	}
	if data, present, ok := p.batchBlob(commit, name); ok {
		p.blobs[key] = data
		return data, present
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

// prefetchBlobs caches names at commit through one ls-tree and one cat-file
// --batch read, so a caller that knows its read set starts two Git processes
// rather than two a file (CKC-V0-012). A name the listing holds as no regular
// file is cached absent, as blob caches it. A name that is not a clean
// repository path, a blob that would take the batch past the flow's Git output
// bound, and every name of a failed read stay uncached, so blob reads them
// one by one as before. So does a name with a pathspec or revision
// metacharacter, which blob's unquoted ls-tree reads its own way.
func (p *proposal) prefetchBlobs(commit string, names []string) {
	const chunk = 1024
	for len(names) > chunk {
		p.prefetchBlobs(commit, names[:chunk])
		names = names[chunk:]
	}
	wanted := map[string]bool{}
	arguments := []string{"--literal-pathspecs", "ls-tree", "-z", "-l", commit, "--"}
	for _, name := range names {
		if _, cached := p.blobs[commit+":"+name]; cached || wanted[name] || name == "" || name == "." || path.Clean(name) != name || path.IsAbs(name) || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "*?[]\\:\x00\n") {
			continue
		}
		wanted[name] = true
		arguments = append(arguments, name)
	}
	if len(wanted) == 0 {
		return
	}
	listing, status, bounded := p.boundedGit(arguments...)
	if status != 0 || !bounded {
		return
	}
	limit := p.gitOutputLimit
	if limit <= 0 {
		limit = maxGitOutputBytes
	}
	byObject, request, total := map[string][]string{}, &bytes.Buffer{}, 0
	listed := map[string]bool{}
	for _, entry := range strings.Split(listing, "\x00") {
		meta, name, found := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if !found || !wanted[name] || len(fields) != 4 {
			continue
		}
		listed[name] = true
		size, err := strconv.Atoi(fields[3])
		if !strings.HasPrefix(fields[0], "100") || fields[1] != "blob" || err != nil {
			p.blobs[commit+":"+name] = nil
			continue
		}
		if byObject[fields[2]] == nil {
			// Each record adds its header and trailing LF to the object.
			if total+size+len(fields[2])+32 > limit {
				continue
			}
			total += size + len(fields[2]) + 32
			request.WriteString(fields[2] + "\n")
		}
		byObject[fields[2]] = append(byObject[fields[2]], name)
	}
	for name := range wanted {
		if !listed[name] {
			p.blobs[commit+":"+name] = nil
		}
	}
	if request.Len() == 0 {
		return
	}
	p.checkpoint()
	command := p.gitCommand(true, "cat-file", "--batch")
	command.Stdin = request
	raw, status, bounded := p.readGit(command)
	if status != 0 || !bounded {
		return
	}
	data := []byte(raw)
	for len(data) > 0 {
		header, rest, ok := bytes.Cut(data, []byte{'\n'})
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			return
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size+1 > len(rest) {
			return
		}
		for _, name := range byObject[fields[0]] {
			p.blobs[commit+":"+name] = rest[:size:size]
		}
		data = rest[size+1:]
	}
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
