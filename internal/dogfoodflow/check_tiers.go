package dogfoodflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// maxTierWarnings bounds the per-hunk tier warnings one check prints; the rest
// are counted on one closing line.
const maxTierWarnings = 50

// tierBelowAnchoredFix names the CEM-V1-081 anchor rules a hunk below
// `anchored` misses and how an author meets one of them.
const tierBelowAnchoredFix = "rule=CEM-V1-081 fix=\"cite a span that defines a requirement ID the hunk adds (- `ID`:) or that contains the hunk path or its basename of at least 6 bytes\""

// tierBelowAnchoredUnavailable names why the hunks a NOTE line counts carry no
// tier warning.
const tierBelowAnchoredUnavailable = "rule=CEM-V1-081 reason=\"no stable BASE line names the hunk path, or its basename of at least 6 bytes, or defines a requirement ID the hunk adds, so no citation can anchor it; a touched-anchored or all-anchored cem-verdict gate refuses it unless a passed runner receipt in the map binds it (CEM-V1-606, CEM-V1-607)\""

// tierBelowAnchoredSamePath names why the hunks a same-path NOTE line counts
// carry no tier warning: each cites its own file, the ordinary citation of the
// function it edits, which is evidence but earns no anchor (DCW-V0-388).
const tierBelowAnchoredSamePath = "rule=CEM-V1-081 reason=\"each cites a BASE span of its own file, such as the function it edits: cited evidence, but CEM-V1-081 anchors only a span that names the hunk path or its basename of at least 6 bytes or defines a requirement ID the hunk adds, so a touched-anchored or all-anchored cem-verdict gate still refuses it; to anchor one, also cite such a span (corvint dogfood propose-citations BASE drafts one)\""

// anchorSearchBudget bounds the citation-anchor search of unanchorableHunks,
// and anchorSearchProgressHunks is the hunk count from which the check
// announces it (DCW-V0-818). Tests lower the budget.
var anchorSearchBudget = 60 * time.Second

const anchorSearchProgressHunks = 16

// Quiet classes of a below-anchored hunk: it draws a NOTE count, not a WARN.
const (
	quietUnanchorable = "unanchorable"
	quietSamePath     = "same-path"
)

// tierRows reads the stable `cem status` line and returns, in citation order,
// the hunks whose recomputed tier is below `anchored` (`cited` or `unknown`)
// with their paths and tiers. A line without a citation block yields none.
func tierRows(cemLine []byte) (below []string, paths, tiers map[string]string) {
	var status struct {
		StableVerification *struct {
			Hunks []struct {
				ID   string `json:"id"`
				Path string `json:"path"`
			} `json:"hunks"`
			Citation *struct {
				Hunks []struct {
					HunkID string `json:"hunkId"`
					Tier   string `json:"tier"`
				} `json:"hunks"`
			} `json:"citation"`
		} `json:"stableVerification"`
	}
	if json.Unmarshal(cemLine, &status) != nil || status.StableVerification == nil || status.StableVerification.Citation == nil {
		return nil, nil, nil
	}
	paths, tiers = map[string]string{}, map[string]string{}
	for _, hunk := range status.StableVerification.Hunks {
		paths[hunk.ID] = hunk.Path
	}
	for _, row := range status.StableVerification.Citation.Hunks {
		if row.Tier == "cited" || row.Tier == "unknown" {
			below = append(below, row.HunkID)
			tiers[row.HunkID] = row.Tier
		}
	}
	return below, paths, tiers
}

// belowAnchoredWarnings returns one warning per hunk whose recomputed tier is
// below `anchored`, naming its path and the anchor rule it misses. A policy
// cem-verdict gate with the touched-anchored or all-anchored predicate refuses
// such a hunk at closeout, after the map is sealed, so the check names it
// first (V1-1760, DCW-V0-112). A quiet hunk gets no warning: one NOTE line
// counts the hunks for which no citation could earn an anchor (DCW-V0-165),
// and another those that cite their own file (DCW-V0-388). The lines never
// change the check's outcome.
func belowAnchoredWarnings(cemLine []byte, quiet map[string]string) []string {
	tally := tallyBelowAnchored(cemLine, quiet)
	lines := tally.lines[:min(len(tally.lines), maxTierWarnings)]
	if len(tally.lines) > maxTierWarnings {
		lines = append(lines, fmt.Sprintf("dogfood-check: WARN tier-below-anchored more=%d", len(tally.lines)-maxTierWarnings))
	}
	return append(lines, tally.note()...)
}

// belowAnchored is the tally of one check's below-anchored hunks: every
// per-hunk warning line, uncapped, the counts by tier and distinct path, and
// the quiet hunks that draw no warning.
type belowAnchored struct {
	lines           []string
	cited, unknown  int
	paths           map[string]bool
	first           string
	unanchorableCnt int
	samePathCnt     int
}

func tallyBelowAnchored(cemLine []byte, quiet map[string]string) belowAnchored {
	below, paths, tiers := tierRows(cemLine)
	tally := belowAnchored{paths: map[string]bool{}}
	for _, id := range below {
		switch quiet[id] {
		case quietUnanchorable:
			tally.unanchorableCnt++
			continue
		case quietSamePath:
			tally.samePathCnt++
			continue
		}
		tier := tiers[id]
		if tier == "cited" {
			tally.cited++
		} else {
			tally.unknown++
		}
		if len(tally.paths) == 0 {
			tally.first = paths[id]
		}
		tally.paths[paths[id]] = true
		tally.lines = append(tally.lines, fmt.Sprintf("dogfood-check: WARN tier-below-anchored path=%s hunk=%s tier=%s %s",
			strconv.Quote(paths[id]), strconv.Quote(id), summaryString(&tier), tierBelowAnchoredFix))
	}
	return tally
}

func (tally belowAnchored) note() []string {
	var notes []string
	if tally.unanchorableCnt > 0 {
		notes = append(notes, fmt.Sprintf("dogfood-check: NOTE tier-below-anchored-unanchorable hunks=%d %s", tally.unanchorableCnt, tierBelowAnchoredUnavailable))
	}
	if tally.samePathCnt > 0 {
		notes = append(notes, fmt.Sprintf("dogfood-check: NOTE tier-below-anchored-same-path hunks=%d %s", tally.samePathCnt, tierBelowAnchoredSamePath))
	}
	return notes
}

// tierBelowAnchoredCause names why the hunks one aggregated warning counts sit
// below `anchored`: each has an anchoring BASE line (DCW-V0-165 found one), so
// the plan cited a span that earns no anchor, such as one shared file cited
// for every hunk, rather than an anchor being impossible.
const tierBelowAnchoredCause = "cause=\"the citation plan cites spans that neither define a requirement ID the hunk adds nor name its path or basename, although a BASE line that does exists; corvint dogfood propose-citations BASE drafts an anchoring row per hunk\""

// aggregated is the default form of the check's CEM-V1-081 tier
// lines (DCW-V0-344): one WARN line with the hunk, tier and path counts, the
// first path, the cause and the fix, naming list, the private file that holds
// every per-hunk line; then the unanchorable NOTE. It returns nil when no hunk
// warns, so a caller that cannot write list keeps the per-hunk form.
func (tally belowAnchored) aggregated(list string) []string {
	if len(tally.lines) == 0 {
		return tally.note()
	}
	line := fmt.Sprintf("dogfood-check: WARN tier-below-anchored hunks=%d cited=%d unknown=%d paths=%d first=%s list=%s %s %s",
		len(tally.lines), tally.cited, tally.unknown, len(tally.paths), strconv.Quote(tally.first), list, tierBelowAnchoredCause, tierBelowAnchoredFix)
	return append([]string{line}, tally.note()...)
}

// tierWarnings returns the check's CEM-V1-081 tier lines for cemLine: the
// aggregated line when the per-hunk detail is written to the private evidence
// file, and the bounded per-hunk form when it cannot be (DCW-V0-344), so no
// warning is ever dropped.
func (c *check) tierWarnings(cemLine []byte) []string {
	quiet := c.quietHunks(cemLine)
	if c.evidence == "" {
		return belowAnchoredWarnings(cemLine, quiet)
	}
	list := c.evidence + "/dogfood-check.tier-below-anchored"
	if removeFile(list) != nil {
		return belowAnchoredWarnings(cemLine, quiet)
	}
	tally := tallyBelowAnchored(cemLine, quiet)
	if len(tally.lines) == 0 {
		return tally.note()
	}
	if os.MkdirAll(c.evidence, 0o700) != nil || writePrivate(list, []byte(strings.Join(tally.lines, "\n")+"\n")) != nil {
		return belowAnchoredWarnings(cemLine, quiet)
	}
	return tally.aggregated(list)
}

// quietHunks classes the below-anchored hunks that draw a NOTE count instead
// of a warning: those no citation could anchor (DCW-V0-165), then, of the
// rest, those whose basis cites a span of the hunk's own path (DCW-V0-388).
// A map that is unreadable or bound to another base classes none, and so
// does an anchor search past its budget, so every hunk keeps its warning
// (DCW-V0-818).
func (c *check) quietHunks(cemLine []byte) map[string]string {
	quiet := map[string]string{}
	unanchorable, classified := c.unanchorableHunks(cemLine)
	if !classified {
		return quiet
	}
	for id := range unanchorable {
		quiet[id] = quietUnanchorable
	}
	data, err := readPrefix(filepath.Join(c.root, ".corvint", "change.cem.json"), maxPlanBytes)
	if err != nil {
		return quiet
	}
	if baseRevision, _, ok := preparedHunks(data); !ok || baseRevision != c.base {
		return quiet
	}
	for id := range samePathHunks(data) {
		if quiet[id] == "" {
			quiet[id] = quietSamePath
		}
	}
	return quiet
}

// samePathHunks returns the hunks of a CEM map whose basis cites evidence
// whose path is the hunk's own path, the samePathCitations notion of
// CEM-V1-082. An evidence ID the map does not list matches nothing.
func samePathHunks(data []byte) map[string]bool {
	var document struct {
		Evidence []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"evidence"`
		Hunks []struct {
			ID    string `json:"id"`
			Path  string `json:"path"`
			Basis []struct {
				EvidenceID string `json:"evidenceId"`
			} `json:"basis"`
		} `json:"hunks"`
	}
	if json.Unmarshal(data, &document) != nil {
		return nil
	}
	paths := map[string]string{}
	for _, evidence := range document.Evidence {
		paths[evidence.ID] = evidence.Path
	}
	same := map[string]bool{}
	for _, hunk := range document.Hunks {
		for _, basis := range hunk.Basis {
			if path, ok := paths[basis.EvidenceID]; ok && path == hunk.Path {
				same[hunk.ID] = true
			}
		}
	}
	return same
}

// unanchorableHunks returns the below-anchored hunks of cemLine for which the
// DCW-V0-161 search, widened to path basenames of 6 bytes or more, finds no
// stable BASE line that earns a CEM-V1-081 anchor: none defines a requirement
// ID the hunk adds and none names its path or that basename. It
// reads the bound map in the clean worktree, so its hunks and old ranges are
// the ones the status verified. Any read it cannot complete (an unreadable
// map, an oversized or failed patch or grep, a Git refusal) returns none, so
// every such hunk keeps its warning (DCW-V0-165). classified is false only
// when the search ran past its budget: then no hunk is classed at all, not
// even by its same-path basis (DCW-V0-818). An interrupt of the check itself
// unwinds as its stop.
func (c *check) unanchorableHunks(cemLine []byte) (unanchorable map[string]bool, classified bool) {
	below, _, _ := tierRows(cemLine)
	if len(below) == 0 {
		return nil, true
	}
	data, err := readPrefix(filepath.Join(c.root, ".corvint", "change.cem.json"), maxPlanBytes)
	if err != nil {
		return nil, true
	}
	baseRevision, hunks, ok := preparedHunks(data)
	if !ok || baseRevision != c.base {
		return nil, true
	}
	// The search runs after the SUMMARY line, so it is bounded and, for a
	// large set, announced; past its budget no hunk is classed unanchorable
	// and every below-anchored hunk keeps its warning (DCW-V0-818).
	if len(below) >= anchorSearchProgressHunks {
		c.say("dogfood-check: NOTE tier-anchor-search hunks=%d budget=%s (DCW-V0-818)\n", len(below), anchorSearchBudget)
	}
	ctx, cancel := context.WithTimeout(c.ctx, anchorSearchBudget)
	defer cancel()
	overBudget := func() bool { return c.ctx.Err() == nil && ctx.Err() != nil }
	defer func() {
		if recovered := recover(); recovered != nil {
			unwound, isStop := recovered.(stop)
			if !isStop || (unwound.code == -1 && !overBudget()) {
				panic(recovered)
			}
			unanchorable, classified = nil, true
		}
		if overBudget() {
			unanchorable, classified = nil, false
			c.say("dogfood-check: NOTE tier-anchor-search NOT_OBSERVED budget-exceeded hunks=%d; every below-anchored hunk keeps its warning (DCW-V0-818)\n", len(below))
		}
	}()
	searched := c.flow
	searched.ctx = ctx
	searched.stderr = io.Discard
	search := &proposal{flow: searched, blobs: map[string][]byte{}, lines: map[string][][]byte{}, basenames: true}
	wanted := map[string]bool{}
	for _, id := range below {
		wanted[id] = true
	}
	owed := []proposedHunk{}
	for _, hunk := range hunks {
		if wanted[hunk.id] {
			owed = append(owed, hunk)
		}
	}
	if !search.prepareAnchors(owed) {
		return nil, true
	}
	edited := editedRanges(hunks)
	unanchorable = map[string]bool{}
	for _, hunk := range owed {
		anchorSearchBeforeHunk()
		// A deadline or an interrupt between hunks unwinds as stop{-1}: the
		// deferred recover keeps a budget stop and re-raises an interrupt.
		search.checkpoint()
		if row, _ := search.anchoredProposal(hunk, "", edited, map[string]bool{}); row == "" {
			unanchorable[hunk.id] = true
		}
	}
	return unanchorable, true
}

// anchorSearchBeforeHunk runs before each hunk of the check's anchor search;
// tests replace it to interrupt the search between hunks.
var anchorSearchBeforeHunk = func() {}

// writeLines writes each line followed by a newline.
func writeLines(out io.Writer, lines []string) {
	if out == nil {
		return
	}
	for _, line := range lines {
		_, _ = io.WriteString(out, line+"\n")
	}
}
