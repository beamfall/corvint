package dogfoodflow

import (
	"strings"

	"github.com/Beamfall/corvint/internal/trace"
)

// requireAggregateVerify refuses, before any bind, a verify line the
// recorder could only record as typed argv when the change admits more paths
// than a learning trace holds: such a change records the nonlearning aggregate
// receipt, which binds v1 commands only (DCW-V0-384), so the bind would refuse
// aggregate-outcome-failed only after its full run (DWC-V0-3726, V1-4169). The
// admitted set is the recorder's own, trace.AdmissibleCurrentPaths over the
// BASE..HEAD changed paths and HEAD's tracked paths; a git read over its bound
// leaves the check to the bind.
func (l *finishLane) requireAggregateVerify() {
	if !l.options.Change.AggregateOverLimit {
		return
	}
	verify, reason := verifyInput(l.options.Change)
	if reason != "" || !hasTypedVerify(verify) {
		return
	}
	changed, changedStatus, changedBounded := l.boundedGit("diff", "--name-only", "--no-renames", "--no-ext-diff", "--no-textconv", l.base, l.target, "--", ".")
	tracked, trackedStatus, trackedBounded := l.boundedGit("ls-tree", "-r", "--name-only", l.target)
	if changedStatus != 0 || trackedStatus != 0 || !changedBounded || !trackedBounded {
		return
	}
	_, err := trace.AdmissibleCurrentPaths(nonEmpty(strings.Split(changed, "\n")), nonEmpty(strings.Split(tracked, "\n")))
	if trace.AdmissionFailureReason(err) != "admitted-path-limit" {
		return
	}
	line, text := typedVerifyLine(l.options.Change)
	l.say("%s: REFUSE aggregate-verify-argv\n", l.prefix)
	if line > 0 && l.options.Change.VerifyFile != "" {
		l.say("  input: --verify %s line %d\n", l.options.Change.VerifyFile, line)
	}
	if text != "" {
		l.say("  %q is recorded as typed argv, a quoted -run alternation or quoted NAME='value' assignments\n", text)
	}
	l.say("  this change admits more than %d changed paths, so its local outcome is the nonlearning aggregate receipt, which records only plain verify commands (DCW-V0-374, DCW-V0-384); the bind would refuse aggregate-outcome-failed after its full run\n", trace.MaxTracePaths)
	l.say("  fix: list one command per test name (go test -run TestA ./pkg, then go test -run TestB ./pkg), or commit a script and list it as sh path/to/script.sh, then rerun corvint dogfood finish-lane --base %s (DWC-V0-3726)\n", l.options.Change.Base)
	l.say("  nothing was prepared, bound or committed\n")
	l.exit(2)
}

// hasTypedVerify reports a recorder argument list with a typed argv entry.
func hasTypedVerify(verify []string) bool {
	for index := 0; index+1 < len(verify); index += 2 {
		if verify[index] == "--verify-argv-json" {
			return true
		}
	}
	return false
}

// typedVerifyLine is the 1-based number and text of the first verify line
// recorded as typed argv, or 0 and "" when none is.
func typedVerifyLine(options ChangeOptions) (int, string) {
	data := []byte(options.Verify + "\n")
	if options.VerifyFile != "" {
		read, err := readPrefix(options.VerifyFile, maxVerifyFileBytes)
		if err != nil {
			return 0, ""
		}
		data = read
	}
	for index, line := range textLines(data) {
		if strings.TrimLeft(line, " \t\n\v\f\r") != "" && hasTypedVerify(verifyLineArguments(line)) {
			return index + 1, strings.TrimSpace(line)
		}
	}
	return 0, ""
}
