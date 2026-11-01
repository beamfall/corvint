package dogfoodflow

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// preflightReceipt is the private file a passing inputs-check leaves for the
// finish-lane run that follows it (DCW-V0-887).
const preflightReceipt = "inputs-check-pass"

// planHeadReceipt is the private file a passing inputs-check leaves naming the
// plan digest and the HEAD it passed at; unlike preflightReceipt no run
// consumes it, so a later citation-plan-map-mismatch can name both HEADs
// (DCW-V0-724).
const planHeadReceipt = "inputs-check-plan-head"

// recordPlanHead keeps the passing plan's digest and HEAD.
func (l *finishLane) recordPlanHead() {
	path := l.gitDir + "/corvint/" + planHeadReceipt
	digest, err := fileSHA256(l.options.Change.Citations)
	if l.options.Change.Citations == "" || err != nil {
		_ = removeFile(path)
		return
	}
	_ = writePrivate(path, []byte("digest="+digest+"\nhead="+l.target+"\n"))
}

// planPassedAt names the HEAD at which inputs-check passed these same plan
// bytes, or "" when no receipt matches the plan.
func planPassedAt(evidence, plan string) string {
	data, err := readPrefix(evidence+"/"+planHeadReceipt, 4<<10)
	digest, digestErr := fileSHA256(plan)
	if err != nil || digestErr != nil {
		return ""
	}
	lines := textLines(data)
	if len(lines) != 2 || lines[0] != "digest="+digest {
		return ""
	}
	head, _ := strings.CutPrefix(lines[1], "head=")
	return head
}

// preflightKey binds a prepare pass to everything it read: BASE, HEAD and its
// tree, each supplied input's path and content digest, the outcome, the plan's
// staging and the step executable's digest. An unreadable executable gives no
// key, so nothing is reused.
func (l *finishLane) preflightKey() string {
	executable, err := fileSHA256(l.options.Change.Steps.Path)
	if err != nil {
		return ""
	}
	fields := []string{preflightReceipt + "/1", l.base, l.target, l.gitValue("rev-parse", l.target+"^{tree}"),
		"outcome=" + l.options.Change.Outcome, fmt.Sprintf("stages=%d", len(l.stageFiles)), "executable=" + executable}
	for _, input := range l.inputs {
		fields = append(fields, fmt.Sprintf("%s=%s present=%t digest=%s", input.flag, input.path, input.present, hex.EncodeToString(input.digest[:])))
	}
	return sha256Hex([]byte(strings.Join(fields, "\n")))
}

// recordPreflight keeps, after a passing pre-flight, its key and the prepare
// lines it printed, so the finish-lane run with the same inputs skips the
// identical prepare pass.
func (l *finishLane) recordPreflight(lines []string) {
	path := l.gitDir + "/corvint/" + preflightReceipt
	key := l.preflightKey()
	if key == "" {
		_ = removeFile(path)
		return
	}
	_ = writePrivate(path, []byte(key+"\n"+strings.Join(lines, "\n")+"\n"))
}

// reusePreflight replays the prepare lines of a passing inputs-check whose key
// matches this run, consuming its receipt, and reports whether it did; any
// other receipt is removed and the prepare pass runs (DCW-V0-887).
func (l *finishLane) reusePreflight() bool {
	path := l.gitDir + "/corvint/" + preflightReceipt
	data, err := readPrefix(path, 64<<10)
	_ = removeFile(path)
	if err != nil {
		return false
	}
	lines := textLines(data)
	key := l.preflightKey()
	if len(lines) < 2 || key == "" || lines[0] != key {
		return false
	}
	for _, line := range lines[1:] {
		l.line("%s reused=inputs-check", line)
	}
	return true
}

// removePreflight drops a receipt this run cannot honour.
func (l *finishLane) removePreflight() {
	if err := removeFile(l.gitDir + "/corvint/" + preflightReceipt); err != nil && !os.IsNotExist(err) {
		l.say("%s: NOTE %s could not be removed: %v\n", l.prefix, preflightReceipt, err)
	}
}
