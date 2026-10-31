package dogfoodflow

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// maxStageRows is the bound of one citation plan, so of one stage file.
const maxStageRows = 256

// stagedRoute names the manual staged route a refused plan falls back to.
const stagedRoute = "docs/DOGFOOD.md Staged binding of one oversized commit"

// planStages splits a well-formed LF-terminated plan of more than 256 rows
// into the staged route's plans, deterministically from the plan bytes alone,
// so a resumed run rebuilds the same final stage (DCW-V0-446). Every row of one
// selector stays in one stage; the final stage takes the selectors with the
// fewest rows (the later ones among equals), then the deferral rows, so it names as many hunks as fit, and the
// earlier stages take the rest in plan order, the one before the final stage
// being the longest run that fits. It returns nil stages for a plan
// the ordinary path decides (at most 256 rows, unreadable, over the byte bound
// or not LF-terminated), and a refusal detail for a plan no staging can bind:
// a selector with more than 256 rows, or a stage that would run while at most
// 256 hunks remain, which the staged route admits only as the last plan
// (DCW-V0-447).
func planStages(plan []byte) (stages [][]string, detail string) {
	if len(plan) == 0 || len(plan) > maxPlanBytes || plan[len(plan)-1] != '\n' || bytes.Count(plan, []byte("\n")) <= maxStageRows {
		return nil, ""
	}
	lines := textLines(plan)
	group := make([]int, len(lines)) // -1 marks a deferral row
	selectors, sizes := []string{}, []int{}
	index := map[string]int{}
	deferrals := 0
	for i, line := range lines {
		selector, _, _ := strings.Cut(line, "\t")
		if selector == deferredRowKind {
			group[i], deferrals = -1, deferrals+1
			continue
		}
		g, seen := index[selector]
		if !seen {
			g = len(selectors)
			index[selector] = g
			selectors, sizes = append(selectors, selector), append(sizes, 0)
		}
		group[i] = g
		sizes[g]++
	}
	for g, size := range sizes {
		if size > maxStageRows {
			return nil, fmt.Sprintf("hunk selector %s has %d rows, more than the %d rows of one plan", strconv.Quote(selectors[g]), size, maxStageRows)
		}
	}
	budget := maxStageRows - deferrals
	if budget < 1 {
		return nil, fmt.Sprintf("the plan's %d deferral rows leave no room for a citation row in its last %d-row stage", deferrals, maxStageRows)
	}
	order := make([]int, len(sizes))
	for g := range order {
		order[g] = g
	}
	// Fewest rows first; among equals the later selector, so the earlier
	// stages keep the plan's leading rows.
	slices.SortFunc(order, func(a, b int) int {
		if sizes[a] != sizes[b] {
			return sizes[a] - sizes[b]
		}
		return b - a
	})
	stageOf := make([]int, len(sizes))
	used := 0
	for _, g := range order {
		if used+sizes[g] > budget {
			break
		}
		used += sizes[g]
		stageOf[g] = -1 // the final stage
	}
	// The remaining selectors fill the earlier stages in plan order. The stage
	// just before the final one is the longest suffix that fits, since it is
	// the one that must still leave more than 256 hunks to cite; the rest
	// fill stages greedily from the start, so a greedy packing cannot leave a
	// short stage last when a valid split keeps the plan order.
	rest := []int{}
	for g := range sizes {
		if stageOf[g] != -1 {
			rest = append(rest, g)
		}
	}
	suffix, rows := len(rest), 0
	for suffix > 0 && rows+sizes[rest[suffix-1]] <= maxStageRows {
		suffix--
		rows += sizes[rest[suffix]]
	}
	count := 0
	rows = maxStageRows
	for _, g := range rest[:suffix] {
		if rows+sizes[g] > maxStageRows {
			count, rows = count+1, 0
		}
		stageOf[g], rows = count-1, rows+sizes[g]
	}
	if suffix < len(rest) {
		count++
		for _, g := range rest[suffix:] {
			stageOf[g] = count - 1
		}
	}
	final := count
	stages = make([][]string, count+1)
	for i, line := range lines {
		stage := final
		if group[i] >= 0 && stageOf[group[i]] >= 0 {
			stage = stageOf[group[i]]
		}
		stages[stage] = append(stages[stage], line)
	}
	// The staged route admits a subset plan only while more than 256 hunks
	// remain unknown, so each earlier stage must leave the later ones naming
	// more than 256 hunks (DCW-V0-037).
	remaining := len(sizes)
	for k := 0; k < final; k++ {
		if remaining <= maxStageRows {
			return nil, fmt.Sprintf("stage %d of %d would cite a subset while only %d hunks remain, so the last plan must name all of them, but they need %d rows; this split keeps the plan order, puts the fewest-row hunks last and the longest fitting run just before them, and no stage boundary in plan order avoids it", k+1, final+1, remaining, stageRows(stages[k:], true))
		}
		remaining -= len(stageSelectors(stages[k]))
	}
	return stages, ""
}

// stageRows counts the citation rows of stages, with or without deferrals.
func stageRows(stages [][]string, deferrals bool) int {
	total := 0
	for _, stage := range stages {
		for _, line := range stage {
			if deferrals || !strings.HasPrefix(line, deferredRowKind+"\t") {
				total++
			}
		}
	}
	return total
}

// stageSelectors is the set of hunk selectors one stage names.
func stageSelectors(stage []string) map[string]bool {
	named := map[string]bool{}
	for _, line := range stage {
		if selector, _, _ := strings.Cut(line, "\t"); selector != deferredRowKind {
			named[selector] = true
		}
	}
	return named
}

// stageHunkConflict names a hunk two stages name through different selectors
// (its ordinal and its full ID), which would split its rows across stages; it
// is read against the first prepared map.
func stageHunkConflict(stages [][]string, hunks []map[string]string) string {
	first := map[string]int{}
	for k, stage := range stages {
		for selector := range stageSelectors(stage) {
			id := selector
			if value, err := strconv.Atoi(selector); err == nil && ordinal.MatchString(selector) && value >= 1 && value <= len(hunks) {
				id = hunks[value-1]["id"]
			}
			if previous, seen := first[id]; seen && previous != k {
				return fmt.Sprintf("hunk %s is named in stages %d and %d through different selectors", id, min(previous, k)+1, max(previous, k)+1)
			}
			if _, seen := first[id]; !seen {
				first[id] = k
			}
		}
	}
	return ""
}

// stagePlan reads the supplied plan and, when it is over the row bound, writes
// its stage files into a private directory under the Git directory, which the
// returned cleanup removes. A plan the ordinary path decides returns no files;
// a plan no staging can bind is refused here, before any step runs
// (DCW-V0-446, DCW-V0-447).
func (l *finishLane) stagePlan() (files []string, stages [][]string, cleanup func()) {
	cleanup = func() {}
	// A non-regular plan, such as a FIFO that could block on open, is left to
	// the ordinary path's input refusal.
	if !isRegular(l.options.Change.Citations) {
		return nil, nil, cleanup
	}
	plan, err := readPrefix(l.options.Change.Citations, maxPlanBytes+1)
	if err != nil {
		return nil, nil, cleanup
	}
	stages, detail := planStages(plan)
	if detail != "" {
		l.say("%s: REFUSE citation-plan-not-stageable rows=%d\n", l.prefix, bytes.Count(plan, []byte("\n")))
		l.say("  %s\n", detail)
		l.say("  fix: keep at most %d rows per hunk and size the plan so its last stage names every remaining hunk in at most %d rows (fewer citations per hunk), or split the implementation into separately bound commits, then rerun corvint dogfood finish-lane --base %s (%s)\n", maxStageRows, maxStageRows, l.options.Change.Base, stagedRoute)
		l.exit(2)
	}
	if stages == nil {
		return nil, nil, cleanup
	}
	parent := filepath.Join(l.gitDir, "corvint")
	dir := ""
	if err = os.MkdirAll(parent, 0o700); err == nil {
		dir, err = os.MkdirTemp(parent, "finish-lane-stages.")
	}
	for k := 0; err == nil && k < len(stages); k++ {
		file := filepath.Join(dir, fmt.Sprintf("stage-%d.tsv", k+1))
		err = writePrivate(file, []byte(strings.Join(stages[k], "\n")+"\n"))
		files = append(files, file)
	}
	if err != nil {
		if dir != "" {
			_ = os.RemoveAll(dir)
		}
		l.say("%s: REFUSE citation-plan-stage-unwritable\n  %s\n", l.prefix, oneLine(err.Error()))
		l.say("  fix: make %s writable, then rerun corvint dogfood finish-lane --base %s\n", parent, l.options.Change.Base)
		l.exit(2)
	}
	return files, stages, func() { _ = os.RemoveAll(dir) }
}

// requireStageableMap refuses, after the first stage's prepare and before any
// bind, a staged plan the whole-plan checks refuse: one hunk named in two
// stages, or a plan citing one span for every hunk of a non-chore change, which
// no stage alone would show (DCW-V0-447, DCW-V0-048).
func (l *finishLane) requireStageableMap(stages [][]string, data []byte) {
	hunks := mapHunks(data)
	reason, detail, hint := "", stageHunkConflict(stages, hunks), ""
	if detail != "" {
		reason = "citation-plan-not-stageable"
		hint = "name every hunk of the plan by one selector, its full hunk ID, then rerun corvint dogfood finish-lane --base " + l.options.Change.Base + " (" + stagedRoute + ")"
	} else {
		cites := []string{}
		for _, stage := range stages {
			for _, line := range stage {
				if !strings.HasPrefix(line, deferredRowKind+"\t") {
					cites = append(cites, line)
				}
			}
		}
		count, span, uniform := uniformCitation([]byte(strings.Join(cites, "\n")+"\n"), hunks)
		if !uniform {
			return
		}
		c := &change{flow: flow{ctx: l.ctx, prefix: "dogfood-change", root: l.root, stderr: io.Discard, gitDir: l.gitDir, base: l.base, target: l.target}, options: l.options.Change}
		if c.choreRole() != "" || c.recordChange() {
			return
		}
		c.uniformHunks, c.uniformSpan = count, span
		reason, hint = "citation-plan-uniform", c.fixHint(step{"cem-cite", "NOT_PRODUCED", "citation-plan-uniform"})
	}
	l.say("%s: REFUSE %s\n", l.prefix, reason)
	if detail != "" {
		l.say("  %s\n", detail)
	}
	l.say("  fix: %s\n", hint)
	l.line("prepare FAIL exit=2 %s", reason)
	l.reportMissing()
	l.exit(2)
}
