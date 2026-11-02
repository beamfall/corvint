package dogfoodflow

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// finishLaneEntries are the make targets a wrapper names with --entry
// (DCW-V0-730).
var finishLaneEntries = map[string]bool{"dogfood-inputs-check": true, "dogfood-finish": true, "dogfood-closeout": true}

// ValidFinishLaneEntry reports whether entry is a make target --entry admits.
func ValidFinishLaneEntry(entry string) bool {
	return finishLaneEntries[entry]
}

// finishLaneRerun matches every rerun hint finish-lane prints, with the
// recovery's make alternative and "with the same inputs" tail.
var finishLaneRerun = regexp.MustCompile(`rerun corvint dogfood (?:finish-lane|inputs-check) --base ([^\s,;()]+)(?: \(or make dogfood-finish BASE=[^\s()]+\))?(?: with the same inputs)?`)

// say prints like flow.say, but under a pre-flight or a wrapper's --entry
// each rerun hint names the command the operator ran, with every input it
// took, so the fix line reruns the same check (DCW-V0-730, V1-3883, V1-3930).
func (l *finishLane) say(format string, values ...any) {
	text := fmt.Sprintf(format, values...)
	if l.options.Entry != "" || l.options.Preflight {
		text = finishLaneRerun.ReplaceAllStringFunc(text, func(match string) string {
			return "rerun " + l.rerunCommand(finishLaneRerun.FindStringSubmatch(match)[1])
		})
	}
	l.flow.say("%s", text)
}

// rerunCommand is the command that reruns this run on base: the --entry make
// target with its variables, or else corvint dogfood inputs-check with its
// flags. An inline DOGFOOD_VERIFY has no file to name, so the hint says so.
func (l *finishLane) rerunCommand(base string) string {
	change := l.options.Change
	words := []string{"corvint", "dogfood", "inputs-check", "--base", shellWord(base)}
	names := []string{"--citations", "--intents", "--verify", "--outcome"}
	if l.options.Entry != "" {
		words = []string{"make", l.options.Entry, "BASE=" + shellWord(base)}
		names = []string{"CITATIONS=", "INTENTS=", "VERIFY=", "OUTCOME="}
	}
	for index, value := range []string{change.Citations, change.IntentsFile, change.VerifyFile, change.Outcome} {
		if value == "" {
			continue
		}
		if l.options.Entry != "" {
			words = append(words, names[index]+shellWord(value))
		} else {
			words = append(words, names[index], shellWord(value))
		}
	}
	if change.VerifyFile == "" && change.Verify != "" {
		words = append(words, "with the same DOGFOOD_VERIFY")
	}
	return strings.Join(words, " ")
}

// staticPass ends a static pre-check whose input checks all passed: it names
// what it did not check, since the prepare pass and its OCM scopes still run
// in the full inputs-check (DCW-V0-731). A seal HEAD needs no outcome inputs
// or plan rows, since the full run reports the seal before it reads them, so
// the pre-check passes it on to that run.
func (l *finishLane) staticPass() int {
	if l.isSealCommit(l.target) {
		l.line("inputs-check STATIC SKIP already-sealed; the full inputs-check reports the seal")
		return 0
	}
	l.requireOutcomeInputs()
	l.requireStaticPlanRows()
	l.line("inputs-check STATIC PASS plan, verify, intents and outcome inputs; the full inputs-check still runs the prepare pass")
	return 0
}

// requireStaticPlanRows refuses, in the static pre-check, a plan the prepare
// pass would refuse for its row grammar or a RELATION outside the registered
// basis relations: the same checks planShape runs first, which read no map.
// A plan over the row or byte bound is left to the staged route (DCW-V0-446).
func (l *finishLane) requireStaticPlanRows() {
	path := l.options.Change.Citations
	if !isRegular(path) || isSymlink(path) {
		return
	}
	c := &change{flow: flow{ctx: l.ctx, prefix: "dogfood-change", root: l.root, stderr: io.Discard, gitDir: l.gitDir, base: l.base, target: l.target}, options: l.options.Change}
	plan, valid := c.validateCitationPlan()
	reason, detail := "", ""
	switch {
	case !valid && (c.citationCount > 256 || oversizedPlan(path)):
		return
	case !valid:
		reason = "invalid-citation-plan"
	case c.unknownRelationRow(plan):
		reason, detail = "citation-plan-relation-unknown", " "+c.relationDetail
	default:
		return
	}
	l.say("%s: REFUSE %s --citations %s%s\n", l.prefix, reason, path, detail)
	l.say("  fix: %s; then rerun corvint dogfood finish-lane --base %s\n", c.fixHint(step{"cem-cite", "NOT_PRODUCED", reason}), l.options.Change.Base)
	l.say("  nothing was prepared, bound or committed\n")
	l.exit(2)
}

// oversizedPlan reports a plan file over the byte bound one plan may hold.
func oversizedPlan(path string) bool {
	data, err := readPrefix(path, maxPlanBytes+1)
	return err != nil || len(data) > maxPlanBytes
}
