package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Claude Code compaction hooks (decision 0340, AHI-026 to AHI-030). PreCompact emits one pin
// line that the host joins verbatim into the compactor's custom instructions (verified against
// Claude Code 2.1.267); PostCompact reads the pin the summary preserved, verifies it against the
// immutable object store with one hermetic cat-file, and reports the verdict to the user. The
// host shows PostCompact stdout to the user only, so the model-facing packet re-emission stays
// the compact SessionStart receipt (AHI-003). Neither event writes anything: the only write on
// this path is the SOL-V0-010 degradation row the shared adapter path already records.
const (
	compactionPinProfile     = "corvint-compaction-pin/0"
	compactionReportProfile  = "corvint-compaction-report/0"
	compactionPinInstruction = "Corvint compaction pin: preserve the following line verbatim in the summary so Corvint can re-pin its evidence after compaction.\n"
	// A pin names at most this many paths and this many path bytes; the rest count as elided.
	compactionPinPathLimit = 24
	compactionPinByteLimit = 1500
	compactionGitDeadline  = 500 * time.Millisecond
	// adapterPlainStdoutKey marks an adapter output the host consumes as raw stdout text
	// (the compactor's custom instructions, the PostCompact user display) rather than hook JSON.
	adapterPlainStdoutKey = "corvintPlainStdout"
)

// compactionTriggers is the closed PreCompact/PostCompact trigger set the host documents.
var compactionTriggers = map[string]bool{"manual": true, "auto": true}

// compactionPinPathPattern admits the project-relative paths a pin may name: no leading dash,
// no whitespace or comma (the pin's own separators), and no `..` segment (checked separately).
var compactionPinPathPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._/+@~-]*$`)

// compactionPinPattern recovers a pin line from the untrusted compaction summary.
var compactionPinPattern = regexp.MustCompile(`corvint-compaction-pin/0\s+revision=([0-9a-f]{40,64})\s+tracked=([0-9]+)\s+untracked=([0-9]+)\s+paths=(\S*)\s+elided=([0-9]+)`)

// compactionBlock is the receipt's context.compaction block the compact SessionStart carries.
type compactionBlock struct {
	Revision string `json:"revision"`
	Request  struct {
		Paths []string `json:"paths"`
	} `json:"request"`
	Rehydration struct {
		Tracked   int `json:"trackedDirtyPathCount"`
		Untracked int `json:"untrackedDirtyPathCount"`
	} `json:"rehydration"`
}

type compactionPin struct {
	Revision  string
	Tracked   int
	Untracked int
	Elided    int
	Paths     []string
}

func runClaudeCompactionEvent(ctx context.Context, root, event string, normalized, payload map[string]any) map[string]any {
	trigger, _ := payload["trigger"].(string)
	if !compactionTriggers[trigger] {
		return degradedAdapterOutput("invalid-compaction-trigger")
	}
	if event == "post-compact" {
		return runClaudePostCompact(ctx, root, normalized, payload)
	}
	block, reason := compactionBlockFor(ctx, root, normalized)
	if reason != "" {
		return degradedAdapterOutput(reason)
	}
	return map[string]any{adapterPlainStdoutKey: compactionPinInstruction + compactionPinLine(block)}
}

func runClaudePostCompact(ctx context.Context, root string, normalized, payload map[string]any) map[string]any {
	summary, _ := payload["compact_summary"].(string)
	pin, ok := parseCompactionPin(summary)
	if !ok {
		return degradedAdapterOutput("compaction-pin-not-preserved")
	}
	revisionMissing, missing, reason := compactionPinMissing(ctx, root, pin)
	if reason != "" {
		return degradedAdapterOutput(reason)
	}
	if revisionMissing {
		return degradedAdapterOutput("compaction-pin-revision-unavailable")
	}
	block, reason := compactionBlockFor(ctx, root, normalized)
	if reason != "" {
		return degradedAdapterOutput(reason)
	}
	return map[string]any{adapterPlainStdoutKey: compactionReportLine(pin, missing, block)}
}

// compactionBlockFor rebuilds the compact SessionStart receipt's compaction block at the current
// revision through the same read-only dogfood event AHI-003 uses.
func compactionBlockFor(ctx context.Context, root string, normalized map[string]any) (compactionBlock, string) {
	input := map[string]any{"sessionIdSha256": normalized["sessionIdSha256"], "startSource": "compact"}
	result, reason := invokeDogfoodEvent(ctx, root, "claude-code", "session-start", input, adapterOutputLimit)
	if reason != "" {
		return compactionBlock{}, reason
	}
	if !claudeDegradationsRecognised(result) {
		return compactionBlock{}, "corvint-degradations-unrecognised"
	}
	contextBlock, _ := result["context"].(map[string]any)
	raw, err := json.Marshal(contextBlock["compaction"])
	var block compactionBlock
	if err != nil || contextBlock["compaction"] == nil || json.Unmarshal(raw, &block) != nil || !validGitObjectID(block.Revision) {
		return compactionBlock{}, "compaction-block-unavailable"
	}
	return block, ""
}

func compactionPinLine(block compactionBlock) string {
	named := []string{}
	elided, size := 0, 0
	for _, path := range block.Request.Paths {
		size += len(path) + 1
		if !compactionPinPathAdmitted(path) || len(named) == compactionPinPathLimit || size > compactionPinByteLimit {
			elided++
			continue
		}
		named = append(named, path)
	}
	return fmt.Sprintf("%s revision=%s tracked=%d untracked=%d paths=%s elided=%d", compactionPinProfile, block.Revision, block.Rehydration.Tracked, block.Rehydration.Untracked, strings.Join(named, ","), elided)
}

func compactionPinPathAdmitted(path string) bool {
	if !compactionPinPathPattern.MatchString(path) {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

// parseCompactionPin takes the last pin line the summary preserved. The summary is untrusted
// repository-adjacent text: every field is re-validated and a malformed pin counts as absent.
func parseCompactionPin(summary string) (compactionPin, bool) {
	matches := compactionPinPattern.FindAllStringSubmatch(summary, -1)
	if len(matches) == 0 {
		return compactionPin{}, false
	}
	match := matches[len(matches)-1]
	pin := compactionPin{Revision: match[1], Paths: []string{}}
	counts := []*int{&pin.Tracked, &pin.Untracked, &pin.Elided}
	for index, field := range []string{match[2], match[3], match[5]} {
		value, err := strconv.Atoi(field)
		if err != nil {
			return compactionPin{}, false
		}
		*counts[index] = value
	}
	for _, path := range strings.Split(match[4], ",") {
		if path == "" {
			continue
		}
		if !compactionPinPathAdmitted(path) {
			return compactionPin{}, false
		}
		pin.Paths = append(pin.Paths, path)
	}
	if !validGitObjectID(pin.Revision) || len(pin.Paths) > compactionPinPathLimit {
		return compactionPin{}, false
	}
	return pin, true
}

// compactionPinMissing verifies the pinned tree and each pinned path against the object store
// with one bounded, hermetic cat-file --batch-check. It reads objects only.
func compactionPinMissing(ctx context.Context, root string, pin compactionPin) (bool, []string, string) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return false, nil, "git-unavailable"
	}
	queries := []string{pin.Revision + "^{tree}"}
	for _, path := range pin.Paths {
		queries = append(queries, pin.Revision+":"+path)
	}
	deadline, cancel := context.WithTimeout(ctx, compactionGitDeadline)
	defer cancel()
	command := exec.CommandContext(deadline, gitExecutable, "--no-optional-locks", "-C", root, "cat-file", "--batch-check")
	command.Env = []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "LANG=C", "LC_ALL=C",
	}
	command.Stdin = strings.NewReader(strings.Join(queries, "\n") + "\n")
	output, err := command.Output()
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if err != nil || len(lines) != len(queries) {
		return false, nil, "compaction-pin-verification-unavailable"
	}
	if strings.HasSuffix(lines[0], " missing") {
		return true, nil, ""
	}
	missing := []string{}
	for index, line := range lines[1:] {
		if strings.HasSuffix(line, " missing") || strings.HasSuffix(line, " ambiguous") {
			missing = append(missing, pin.Paths[index])
		}
	}
	return false, missing, ""
}

// compactionReportLine is the user-visible PostCompact verdict: which pinned paths rehydrate
// from the pinned tree, which do not (by name where pinned, by count where the pin elided them
// or they were untracked), and whether the pin still matches the current revision.
func compactionReportLine(pin compactionPin, missing []string, block compactionBlock) string {
	rehydrated := len(pin.Paths) - len(missing)
	drift := "matches"
	if block.Revision != pin.Revision {
		drift = "moved to " + block.Revision
	}
	named := "none"
	if len(missing) > 0 {
		named = strings.Join(missing, ",")
	}
	return fmt.Sprintf("%s pinned=%s current=%s rehydrated=%d non-rehydratable=%s elided=%d untracked=%d current-dirty=%d", compactionReportProfile, pin.Revision, drift, rehydrated, named, pin.Elided, pin.Untracked, len(block.Request.Paths))
}

// compactionDisclosure is the trusted line a compact SessionStart carries on every host (AHI-030):
// it names where pin verification runs and that this packet is the model-facing rehydration, so a
// host that silently ignores the PreCompact/PostCompact registration still shows the gap.
const compactionDisclosure = "Corvint compaction (trusted adapter guidance): PreCompact/PostCompact pin verification is registered for Claude Code 2.1.267 and reports to the user only; this SessionStart(source=compact) packet is the model-facing rehydration, and on a host without those hook events it is the only one.\n"

func compactSessionDisclosure(event string, normalized map[string]any) string {
	if event != "session-start" || normalized["startSource"] != "compact" {
		return ""
	}
	return compactionDisclosure
}
