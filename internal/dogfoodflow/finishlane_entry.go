package dogfoodflow

import (
	"fmt"
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
// in the full inputs-check (DCW-V0-731).
func (l *finishLane) staticPass() int {
	l.requireOutcomeInputs()
	l.line("inputs-check STATIC PASS plan, verify, intents and outcome inputs; the full inputs-check still runs the prepare pass")
	return 0
}
