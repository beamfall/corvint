package dogfoodflow

import (
	"bytes"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// finishRefreshedPlan is the file, under the Git directory's corvint evidence
// directory, that keeps a citation plan finish-lane refreshed, and
// finishRefreshedPlanAlt the one it writes instead when the plan it refreshes
// is that file, so a refresh never writes over its own input (DCW-V0-947).
const (
	finishRefreshedPlan    = "dogfood-finish.refreshed-plan.tsv"
	finishRefreshedPlanAlt = "dogfood-finish.refreshed-plan.alt.tsv"
)

// maxRefreshListed bounds the uncarried hunks a refusal names.
const maxRefreshListed = 5

// refreshedPlanPath is the refreshed plan's file in evidence: the default one,
// or the alternate one when plan already is the default file, so the two
// alternate and the input plan is never rewritten.
func refreshedPlanPath(evidence, plan string) string {
	name := evidence + "/" + finishRefreshedPlan
	planInfo, planErr := os.Stat(plan)
	nameInfo, nameErr := os.Stat(name)
	if planErr == nil && nameErr == nil && os.SameFile(planInfo, nameInfo) {
		return evidence + "/" + finishRefreshedPlanAlt
	}
	return name
}

// refreshMovedPlan refreshes the citation plan, as propose-citations BASE
// --refresh CITATIONS would, when a row names a hunk ID the current BASE..HEAD
// patch lacks: HEAD moved since the plan was drafted (DCW-V0-947). A refresh
// that carried or re-keyed every row and drafted no hunk afresh continues on
// the refreshed plan; any other stops before a step runs, naming the plan to
// review. A plan whose hunk IDs are all current, or that names no hunk ID, is
// used unchanged, and a bind or seal HEAD is never refreshed. The caller's
// file is never rewritten.
func (l *finishLane) refreshMovedPlan() {
	plan := l.options.Change.Citations
	if !isRegular(plan) || isSymlink(plan) || l.isSealCommit(l.target) || l.isBindCommit(l.target) {
		return
	}
	data, err := readPrefix(plan, maxPlanBytes)
	if err != nil || !bytes.Contains(data, []byte(wire.HunkPrefix)) {
		return
	}
	var stdout, notes bytes.Buffer
	run := &proposal{
		flow:    flow{ctx: l.ctx, prefix: "dogfood-propose-citations", root: l.root, stderr: &notes},
		options: ProposeOptions{Root: l.root, Base: l.base, Refresh: plan}, stdout: &stdout,
		blobs: map[string][]byte{}, lines: map[string][][]byte{},
	}
	code, err := boundary(l.ctx, run.run, func() {})
	if err != nil {
		l.exit(-1)
	}
	if code != 0 {
		l.say("%s: REFUSE citations-refresh-failed exit=%d\n", l.prefix, code)
		_, _ = l.stderr.Write(notes.Bytes())
		l.say("  fix: the plan names a hunk ID BASE..HEAD no longer has, and propose-citations %s --refresh %s refused as above; repair that, then rerun (DCW-V0-947)\n", l.base, plan)
		l.exit(2)
	}
	if run.staleIDs == 0 {
		return
	}
	carried, rekeyed, dropped := 0, 0, 0
	for _, rows := range run.carried {
		carried += len(rows)
	}
	for _, note := range run.carryNotes {
		switch {
		case strings.Contains(note, " re-keyed "):
			rekeyed++
		case strings.Contains(note, " dropped"):
			dropped++
		}
	}
	name := refreshedPlanPath(l.gitDir+"/corvint", plan)
	if os.MkdirAll(l.gitDir+"/corvint", 0o777) != nil || removeFile(name) != nil || writePrivate(name, stdout.Bytes()) != nil {
		l.say("%s: REFUSE citations-refresh-unwritable plan=%s\n", l.prefix, name)
		l.say("  fix: the plan names a hunk ID BASE..HEAD no longer has; run corvint dogfood propose-citations %s --refresh %s > PLAN.new yourself, review it, then rerun with that plan (DCW-V0-947)\n", l.base, plan)
		l.exit(2)
	}
	if dropped > 0 || len(run.redrafted) > 0 {
		l.say("%s: REFUSE citations-refresh-incomplete plan=%s carried=%d rekeyed=%d dropped=%d drafted=%d\n", l.prefix, name, carried, rekeyed, dropped, len(run.redrafted))
		l.say("  HEAD moved since %s was drafted: %d of its rows name hunk IDs BASE..HEAD no longer has, and the refresh could not carry every hunk; no step ran (DCW-V0-947)\n", plan, run.staleIDs)
		for index, hunk := range run.redrafted {
			if index == maxRefreshListed {
				l.say("  (+%d more)\n", len(run.redrafted)-maxRefreshListed)
				break
			}
			l.say("  drafted afresh: %s path=%s BASE %d,%d\n", hunk.id, hunk.path, hunk.oldStart, hunk.oldCount)
		}
		for _, note := range run.carryNotes {
			if strings.Contains(note, " dropped") {
				l.say("  %s\n", note)
			}
		}
		l.say("  fix: review the drafted rows in %s (a drafted citation is a proposal, not evidence), then rerun with CITATIONS=%s\n", name, name)
		l.say("  install: after that review, cp -f -- %s %s installs the candidate as the plan CITATIONS names, so the same rerun binds it (DCW-V0-982)\n", shellQuote(name), shellQuote(plan))
		l.exit(2)
	}
	l.say("%s: NOTE citations-refreshed plan=%s carried=%d rekeyed=%d\n", l.prefix, name, carried, rekeyed)
	l.say("  HEAD moved since %s was drafted, so its %d stale hunk-ID rows were re-keyed to the current hunks; this run binds the refreshed plan; pass CITATIONS=%s next time (DCW-V0-947)\n", plan, run.staleIDs, name)
	l.options.Change.Citations = name
}
