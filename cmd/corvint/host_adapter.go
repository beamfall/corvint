package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/observations"
	"github.com/Beamfall/corvint/internal/projectpath"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/unplannedread"
)

const (
	adapterVersion       = "0.1.0"
	adapterInputLimit    = 8 * 1024 * 1024
	adapterOutputLimit   = 8000
	untrustedDataPrefix  = repoenvelope.Prefix
	untrustedDataSuffix  = repoenvelope.Suffix
	completionBlockText  = "Corvint local completion policy is incomplete. Complete enrolled checks and evidence, then inspect and acknowledge the exact report set. Frontier authority remains unavailable."
	continuationLimitMsg = "Corvint local policy remains unresolved; continuation limit reached. Frontier authority remains unavailable."
)

func runHostAdapter(ctx context.Context, arguments []string, stdin io.Reader, stdout io.Writer) int {
	if len(arguments) == 0 {
		return emitAdapterOutput(stdout, degradedAdapterOutput("unsupported-hook-event"))
	}
	if arguments[0] == "pi-tool" {
		return runPiTool(ctx, arguments[1:], stdin, stdout)
	}
	if arguments[0] == "pi" {
		return runPiAdapter(ctx, arguments[1:], stdin, stdout)
	}
	if arguments[0] == "source-view" {
		return runSourceViewAdapter(ctx, arguments[1:], stdout)
	}
	deadline, bounded := ctx.Deadline()
	if !bounded {
		return emitAdapterOutput(stdout, hostAdapterOutput(ctx, arguments, stdin))
	}
	return emitAdapterOutput(stdout, watchedHostAdapterOutput(ctx, deadline, arguments, stdin))
}

// AHI-017: the kill each shipped hooks.json declares for an adapter invocation, keyed by the
// arguments after "adapter". An invocation absent here declares no kill and gets no watchdog.
var adapterDeclaredHostKill = map[string]time.Duration{
	"codex":                     2 * time.Second,
	"claude-code session-start": 2 * time.Second,
	"claude-code user-prompt":   2 * time.Second,
	"claude-code post-tool":     2 * time.Second,
	"claude-code stop":          2 * time.Second,
	"claude-code session-end":   time.Second,
	"claude-code pre-compact":   2 * time.Second,
	"claude-code post-compact":  2 * time.Second,
}

const (
	// Wall time the process clock cannot see: exec and Go runtime start before package
	// initialisation, then the stdout write and exit after emit. Measured 2026-09-12 on a
	// 12-core host at load average 68-143 as the whole wall of a trivial adapter call: p95
	// 351 ms, max 358 ms. 400 ms is that p95 plus a ~50 ms margin.
	adapterProcessReserve = 400 * time.Millisecond
	// The work context expires this much before the watchdog so a promptly cancelled event
	// still reports its own degradation reason; the watchdog is the backstop.
	adapterWatchdogGrace = 100 * time.Millisecond
	// A degradation row appended after the work deadline (the kill deadline, or a dogfood event
	// deadline) spends at most this much of the grace or reserve and is abandoned past it (SOL-V0-010).
	adapterDeadlineRecordBound = 50 * time.Millisecond
)

// adapterProcessStart is the earliest instant this process observes.
var adapterProcessStart = time.Now()

// adapterHostKillContext bounds an adapter invocation of a host with a declared kill to that
// kill, measured from process start, less the process reserve (AHI-017).
func adapterHostKillContext(parent context.Context, arguments []string, start time.Time) (context.Context, context.CancelFunc) {
	if len(arguments) < 2 || arguments[0] != "adapter" {
		return parent, func() {}
	}
	kill, ok := adapterDeclaredHostKill[strings.Join(arguments[1:], " ")]
	if !ok {
		return parent, func() {}
	}
	return context.WithDeadline(parent, start.Add(kill-adapterProcessReserve))
}

// watchedHostAdapterOutput returns the adapter's own output, or a degradation once the
// deadline passes while the work is still running (a blocked stdin read, a slow cancellation).
// The abandoned work ends with the process, as it would under the host's kill.
func watchedHostAdapterOutput(ctx context.Context, deadline time.Time, arguments []string, stdin io.Reader) map[string]any {
	work, cancel := context.WithDeadline(ctx, deadline.Add(-adapterWatchdogGrace))
	defer cancel()
	result := make(chan map[string]any, 1)
	go func() { result <- hostAdapterOutput(work, arguments, stdin) }()
	watchdog := time.NewTimer(time.Until(deadline))
	defer watchdog.Stop()
	select {
	case output := <-result:
		return output
	case <-watchdog.C:
		if len(arguments) == 2 && arguments[0] == "claude-code" {
			output := claudeDegradedOutput(arguments[1], "adapter-host-kill-deadline")
			recordClaudeKillDeadline(ctx, arguments[1], output)
			return output
		}
		return degradedAdapterOutput("adapter-host-kill-deadline")
	}
}

func hostAdapterOutput(ctx context.Context, arguments []string, stdin io.Reader) map[string]any {
	raw, err := io.ReadAll(io.LimitReader(stdin, adapterInputLimit+1))
	if err != nil || len(raw) > adapterInputLimit {
		return degradedAdapterOutput("hook-input-too-large")
	}
	payload, err := decodeAdapterJSON(raw)
	if err != nil {
		return degradedAdapterOutput("malformed-hook-json")
	}
	switch arguments[0] {
	case "codex":
		return runCodexAdapter(ctx, payload)
	case "claude-code":
		if len(arguments) != 2 {
			return degradedAdapterOutput("unsupported-hook-event")
		}
		return runClaudeAdapter(ctx, arguments[1], payload)
	case "claude-source-handoff":
		return runClaudeSourceHandoff(ctx, arguments[1:], payload)
	default:
		return degradedAdapterOutput("unsupported-hook-event")
	}
}

func decodeAdapterJSON(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) || !validJSONSurrogateEscapes(raw) {
		return nil, fmt.Errorf("invalid-json-unicode")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeAdapterValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing-json")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("hook-input-not-object")
	}
	return object, nil
}

func validJSONSurrogateEscapes(raw []byte) bool {
	inString := false
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || index+1 >= len(raw) {
				continue
			}
			if raw[index+1] != 'u' {
				index++
				continue
			}
			if index+6 > len(raw) {
				return false
			}
			value, err := strconv.ParseUint(string(raw[index+2:index+6]), 16, 16)
			if err != nil {
				return false
			}
			if value >= 0xdc00 && value <= 0xdfff {
				return false
			}
			if value >= 0xd800 && value <= 0xdbff {
				if index+12 > len(raw) || raw[index+6] != '\\' || raw[index+7] != 'u' {
					return false
				}
				low, err := strconv.ParseUint(string(raw[index+8:index+12]), 16, 16)
				if err != nil || low < 0xdc00 || low > 0xdfff {
					return false
				}
				index += 11
				continue
			}
			index += 5
		}
	}
	return true
}

func decodeAdapterValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 256 {
		return nil, fmt.Errorf("json-nesting-too-deep")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, compound := token.(json.Delim)
	if !compound {
		return token, nil
	}
	switch delim {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok {
				return nil, fmt.Errorf("invalid-object-key")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("duplicate-key")
			}
			value, err := decodeAdapterValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, fmt.Errorf("unterminated-object")
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeAdapterValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, fmt.Errorf("unterminated-array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected-delimiter")
	}
}

func runCodexAdapter(ctx context.Context, payload map[string]any) (hookOutput map[string]any) {
	eventName, _ := payload["hook_event_name"].(string)
	events := map[string]string{"SessionStart": "session-start", "UserPromptSubmit": "user-prompt", "Stop": "stop", "SessionEnd": "session-end"}
	event, ok := events[eventName]
	if !ok {
		return map[string]any{}
	}
	root, ok := payload["cwd"].(string)
	if !ok || root == "" {
		return codexDegraded(eventName, "missing-cwd")
	}
	if filepath.IsAbs(root) && !insideGitRepository(root) {
		return map[string]any{}
	}
	defer func() { recordAdapterDegradation(ctx, root, "codex", event, hookOutput) }()
	normalized, reason := normalizeAdapterInput("codex", event, payload, root)
	if reason != "" {
		return codexDegraded(eventName, reason)
	}
	disclosure := promptBoundDisclosure(event, payload)
	kernel := experimentalKernelContext(ctx, event, root)
	result, reason := invokeDogfoodEvent(ctx, root, "codex", event, normalized, adapterOutputLimit-promptBoundReserve(disclosure)-promptBoundReserve(kernel))
	if reason != "" {
		return codexDegraded(eventName, reason)
	}
	return withAdapterContextSuffix(withPromptBoundDisclosure(renderAdapterResult("codex", eventName, event, root, normalized, result), disclosure), kernel)
}

// adapterEnvKey carries a per-invocation replacement for os.LookupEnv on the Claude adapter's
// process-boundary reads (CLAUDE_PROJECT_DIR and the experimental kernel opt-in). Production never
// sets it, so those reads stay os.LookupEnv; tests set it instead of t.Setenv so they can run in parallel.
type adapterEnvKey struct{}

func adapterGetenv(ctx context.Context, name string) string {
	value, _ := adapterLookup(ctx)(name)
	return value
}

func runClaudeAdapter(ctx context.Context, event string, payload map[string]any) (hookOutput map[string]any) {
	known := map[string]bool{"session-start": true, "user-prompt": true, "file-change": true, "post-tool": true, "stop": true, "session-end": true, "pre-compact": true, "post-compact": true}
	if !known[event] {
		return degradedAdapterOutput("unsupported-hook-event")
	}
	root, err := claudeAdapterRoot(ctx)
	if err != nil {
		return degradedAdapterOutput("project-root-unavailable")
	}
	if !insideGitRepository(root) {
		return map[string]any{}
	}
	defer func() { recordAdapterDegradation(ctx, root, "claude-code", event, hookOutput) }()
	normalized, reason := normalizeAdapterInput("claude-code", event, payload, root)
	if reason != "" {
		refuseUndeliveredPacket(root, event, payload)
		return claudeDegradedOutput(event, reason)
	}
	if event == "pre-compact" || event == "post-compact" {
		return runClaudeCompactionEvent(ctx, root, event, normalized, payload)
	}
	if event == "post-tool" {
		unplannedread.HookPostToolSession(root, normalized["sessionIdSha256"].(string), payload)
	}
	if event == "file-change" || event == "post-tool" {
		silent := event == "post-tool" && postToolChangeOutOfRoot(root, payload)
		return invokeLegacyClaudeEvent(ctx, root, event, normalized, silent)
	}
	guidance := claudeGuidance(root, normalized["sessionIdSha256"].(string))
	reserve, _ := json.Marshal(renderClaudeContext(event, "", guidance))
	disclosure := promptBoundDisclosure(event, payload) + compactSessionDisclosure(event, normalized)
	kernel := experimentalKernelContext(ctx, event, root)
	budget := adapterOutputLimit - len(reserve) - 1 - promptBoundReserve(disclosure) - promptBoundReserve(kernel)
	result, reason := invokeDogfoodEvent(ctx, root, "claude-code", event, normalized, budget)
	if reason != "" {
		refuseUndeliveredPacket(root, event, payload)
		return withSnapshotRemediation(root, event, reason, claudeDegradedOutput(event, reason))
	}
	output := renderAdapterResult("claude-code", claudeEventName(event), event, root, normalized, result)
	recordDeliveredPacket(root, event, normalized, result, output)
	return withAdapterContextSuffix(withPromptBoundDisclosure(output, disclosure), kernel)
}

// claudeSessionHash is the Claude adapter's session identity hash.
func claudeSessionHash(session string) string {
	sum := sha256.Sum256([]byte("corvint-local-completion-session/claude-code/0\x00" + session))
	return hex.EncodeToString(sum[:])
}

// insideGitRepository reports whether dir or an ancestor holds a .git entry. Only a confirmed
// absence at every level is outside a repository: an expected absence that emits and records
// nothing (decision 0178); an unreadable level counts as inside, so its failure keeps its notice.
func insideGitRepository(dir string) bool {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	for current := filepath.Clean(dir); ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); !errors.Is(err, fs.ErrNotExist) {
			return true
		}
		if filepath.Dir(current) == current {
			return false
		}
	}
}

func claudeAdapterRoot(ctx context.Context) (string, error) {
	root := adapterGetenv(ctx, "CLAUDE_PROJECT_DIR")
	if root == "" {
		root, _ = os.Getwd()
	}
	return filepath.Abs(root)
}

// adapterDegradationFrames are the fixed texts degradedAdapterOutput, claudeDegradedOutput and
// codexDegraded compose around a reason.
var adapterDegradationFrames = [][2]string{
	{"Corvint FALLBACK degraded: ", "; coding continues"},
	{"Corvint fallback: ", "; unrelated coding continues."},
}

// adapterDegradationReason is the reason a degraded hook output carries, or "" when the output
// is not a degradation.
func adapterDegradationReason(output map[string]any) string {
	hook, _ := output["hookSpecificOutput"].(map[string]any)
	for _, value := range []any{output["systemMessage"], hook["additionalContext"]} {
		text, _ := value.(string)
		text, _, _ = strings.Cut(text, "\n") // a remediation line follows the frame (AHI-031)
		for _, frame := range adapterDegradationFrames {
			if strings.HasPrefix(text, frame[0]) && strings.HasSuffix(text, frame[1]) && len(text) > len(frame[0])+len(frame[1]) {
				return text[len(frame[0]) : len(text)-len(frame[1])]
			}
		}
	}
	return ""
}

// recordAdapterDegradation appends a degraded output's SOL-V0-010 row to the root the adapter
// resolved for this invocation. The append is best-effort and waits no longer than the later of
// ctx's deadline and adapterDeadlineRecordBound, since a dogfood-event-deadline degradation is
// returned only after the work deadline expired; an unadmitted reason, relative root, or
// unignored ledger records nothing.
func recordAdapterDegradation(ctx context.Context, root, host, event string, output map[string]any) {
	reason := adapterDegradationReason(output)
	if reason == "" {
		return
	}
	if !filepath.IsAbs(root) {
		return
	}
	row := observations.AdapterDegradationEvent(host, event, reason, version, time.Now())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = observations.Append(root, row)
	}()
	waitAppend(ctx, done, adapterDeadlineRecordBound)
}

// waitAppend waits for done until the later of ctx's deadline and bound.
func waitAppend(ctx context.Context, done <-chan struct{}, bound time.Duration) {
	deadline, bounded := ctx.Deadline()
	if !bounded {
		<-done
		return
	}
	timer := time.NewTimer(max(time.Until(deadline), bound))
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}

// recordClaudeKillDeadline records the watchdog's degradation under adapterDeadlineRecordBound.
// Codex takes its root from the hook input the watchdog abandoned, so its deadline is not recorded.
func recordClaudeKillDeadline(ctx context.Context, event string, output map[string]any) {
	root, err := claudeAdapterRoot(ctx)
	if err != nil {
		return
	}
	bounded, cancel := context.WithTimeout(context.Background(), adapterDeadlineRecordBound)
	defer cancel()
	recordAdapterDegradation(bounded, root, "claude-code", event, output)
}

// adapterStartSources maps a host session-start source to the closed Corvint
// startSource enum. Claude Code's "fork" resumes an existing transcript under
// a new session id, so it is a resume.
var adapterStartSources = map[string]string{"startup": "startup", "resume": "resume", "clear": "clear", "compact": "compact", "fork": "resume"}

func normalizeAdapterInput(host, event string, payload map[string]any, root string) (map[string]any, string) {
	result := map[string]any{}
	session, present := payload["session_id"]
	if host == "codex" {
		if value := os.Getenv("CODEX_THREAD_ID"); value != "" {
			session, present = value, true
		} else if value := os.Getenv("CODEX_SESSION_ID"); value != "" {
			session, present = value, true
		}
	}
	text, ok := session.(string)
	if host == "claude-code" && (!present || !ok || text == "") {
		return nil, "missing-session-identity"
	}
	if present && (!ok || len([]byte(text)) > 4096) {
		return nil, "invalid-session-identity"
	}
	if text != "" {
		if host == "claude-code" {
			result["sessionIdSha256"] = claudeSessionHash(text)
		} else {
			result["sessionIdSha256"] = hashAdapterSession("corvint-local-completion-session/0", text)
		}
	}
	switch event {
	case "session-start":
		if source, exists := payload["source"]; exists {
			value, _ := source.(string)
			startSource, ok := adapterStartSources[value]
			if !ok {
				return nil, "invalid-start-source"
			}
			result["startSource"] = startSource
		}
	case "user-prompt":
		prompt, ok := payload["prompt"].(string)
		prompt = strings.TrimSpace(prompt)
		if !ok || prompt == "" {
			return nil, "missing-prompt"
		}
		if promptOverQueryBound(prompt) {
			prompt, _ = deriveOverBoundQuery(prompt)
		}
		if prompt == "" {
			return nil, "prompt-over-query-bound"
		}
		result["task"] = prompt
	case "stop":
		active := false
		if value, exists := payload["stop_hook_active"]; exists {
			var ok bool
			active, ok = value.(bool)
			if !ok {
				return nil, "invalid-stop-hook-active"
			}
		}
		result["stopHookActive"] = active
		result["changedPaths"] = []any{}
	case "session-end":
		result["openedPaths"], result["changedPaths"], result["verification"] = []any{}, []any{}, []any{}
	case "file-change":
		path, ok := projectRelativePath(root, payload["file_path"])
		if !ok {
			return nil, "file-change-path-not-project-relative"
		}
		result["paths"] = []string{path}
	case "post-tool":
		var path any
		if input, ok := payload["tool_input"].(map[string]any); ok {
			fields := map[string]string{"Edit": "file_path", "Write": "file_path", "NotebookEdit": "notebook_path"}
			if name, ok := payload["tool_name"].(string); ok {
				path = input[fields[name]]
			}
		}
		relative, ok := projectRelativePath(root, path)
		changed := []string{}
		if ok {
			changed = []string{relative}
		}
		result["observedEvidenceHandles"], result["changedPaths"], result["verification"] = []any{}, changed, []any{}
	}
	return result, ""
}

func invokeDogfoodEvent(ctx context.Context, root, host, event string, input map[string]any, budget int) (map[string]any, string) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, "invalid-input"
	}
	args := []string{"--host", host, "--host-version", dogfoodHostVersions[host], "--surface", "plugin", "--adapter-version", adapterVersion, "--event", event, "--input", "-", "--budget-bytes", fmt.Sprint(budget)}
	var stdout bytes.Buffer
	var stderr adapterErrorTail
	if status := runLocalCompletionEvent(ctx, root, args, bytes.NewReader(raw), &stdout, &stderr); status != 0 {
		return nil, adapterRejectedReason(stderr.Bytes())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, "malformed-corvint-output"
	}
	return result, ""
}

func renderAdapterResult(host, eventName, event, root string, input, result map[string]any) map[string]any {
	if event == "stop" {
		completion, _ := result["completion"].(map[string]any)
		if completion["decision"] == "block" {
			reason := completionBlockText
			if host == "claude-code" {
				reason += "\n" + claudeGuidance(root, input["sessionIdSha256"].(string))
			}
			return map[string]any{"decision": "block", "reason": reason}
		}
		if completion["reason"] == "local-policy-continuation-limit" {
			return map[string]any{"systemMessage": continuationLimitMsg}
		}
		return map[string]any{}
	}
	if event == "session-end" {
		return map[string]any{}
	}
	if !claudeDegradationsRecognised(result) {
		// The offending code is unvalidated text, so the fault names no code (decision 0232).
		if host == "claude-code" {
			return degradedAdapterOutput("corvint-degradations-unrecognised")
		}
		return codexDegraded(eventName, "corvint-degradations-unrecognised")
	}
	raw, _ := json.Marshal(result)
	if host == "claude-code" {
		return renderClaudeContext(event, string(raw), claudeGuidance(root, input["sessionIdSha256"].(string)))
	}
	context, err := repoenvelope.Frame(string(raw))
	if err != nil {
		return codexDegraded(eventName, repoenvelope.CollisionCode)
	}
	return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": eventName, "additionalContext": context}}
}

func renderClaudeContext(event, receipt, guidance string) map[string]any {
	name := claudeEventName(event)
	framed, err := repoenvelope.Frame(receipt)
	if err != nil {
		return degradedAdapterOutput(repoenvelope.CollisionCode)
	}
	return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": name, "additionalContext": guidance + framed}}
}

func claudeGuidance(root, key string) string {
	status, _ := json.Marshal([]string{"corvint", "--root", root, "dogfood", "status", "--session-key", key})
	warm, _ := json.Marshal([]string{"corvint", "--root", root, "index", "--if-stale"})
	return "Corvint local workflow argv (trusted adapter guidance):\n" + string(status) + "\nUse this explicit key and root for begin, verify, review and finish. A handed-off enrollment keeps its original key/root; this hook checks only its native key.\nAfter runtime or committed-tree changes, prepare the index explicitly under supervision:\n" + string(warm) + "\n"
}

func invokeLegacyClaudeEvent(ctx context.Context, root, event string, input map[string]any, silent bool) map[string]any {
	raw, _ := json.Marshal(input)
	args := []string{"--root", root, "harness", "event", "--host", "claude-code", "--host-version", claudeHostVersion, "--surface", "plugin", "--adapter-version", adapterVersion, "--event", event, "--input", "-", "--budget-bytes", "8000"}
	var stdout bytes.Buffer
	var stderr adapterErrorTail
	if runContext(ctx, args, bytes.NewReader(raw), &stdout, &stderr) != 0 {
		return degradedAdapterOutput(adapterRejectedReason(stderr.Bytes()))
	}
	var result map[string]any
	if json.Unmarshal(stdout.Bytes(), &result) != nil {
		return degradedAdapterOutput("malformed-corvint-output")
	}
	// An out-of-root post-tool change is not a project change (AGENTS.md invariant 1
	// only pins evidence to the repository), so its receipt is intentionally surfaced
	// nowhere: post-tool is not a SOL-V0-001 ledger event.
	if silent {
		return map[string]any{}
	}
	// A routine receipt is not a notice for the user (decision 0161, AHI-019).
	return claudeReceiptOutput(event, result)
}

// claudeExpectedDegradation is the closed set of Claude adapter degradation reasons that are
// expected, non-fault outcomes (decision 0161, AHI-021). Every other reason is a fault the user
// must act on and keeps its systemMessage.
var claudeExpectedDegradation = map[string]bool{
	"prompt-over-query-bound":               true,
	"missing-prompt":                        true,
	"file-change-path-not-project-relative": true,
	"adapter-host-kill-deadline":            true,
	// The dogfood event's own time bound (LCP-V0-008) usually expires before the watchdog.
	"corvint-event-rejected:dogfood-event-deadline": true,
}

// claudeContextEvents maps the adapter events whose Claude hook accepts
// hookSpecificOutput.additionalContext to their native hook event name.
var claudeContextEvents = map[string]string{"session-start": "SessionStart", "user-prompt": "UserPromptSubmit", "post-tool": "PostToolUse"}

func claudeContextOutput(name, text string) map[string]any {
	return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": name, "additionalContext": text}}
}

// claudeReceiptOutput gives a post-tool receipt ID and its degradation codes to the model. A
// file-change receipt is dropped from host output: `harness event` has already appended it, with
// its degradations, to the SOL-V0-001 self-observation ledger.
func claudeReceiptOutput(event string, result map[string]any) map[string]any {
	if !claudeDegradationsRecognised(result) {
		// The offending code is unvalidated text, so the fault names no code (decision 0232).
		return degradedAdapterOutput("corvint-degradations-unrecognised")
	}
	name, ok := claudeContextEvents[event]
	if !ok {
		return map[string]any{}
	}
	return claudeContextOutput(name, "Corvint FALLBACK "+fmt.Sprint(result["receiptId"])+claudeNamedCodes(result["degradations"]))
}

// claudeRecognisedDegradation is receiptDegradationPolicy.recognised in
// integrations/compatibility.json; its onUnrecognised rule is refuse (decision 0232).
var claudeRecognisedDegradation = map[string]bool{
	"compaction-critical-evidence-overflow":       true,
	"compaction-dirty-set-over-budget":            true,
	"compaction-untracked-paths-not-rehydratable": true,
	"frontier-authority-unavailable":              true,
	"host-version-unknown":                        true,
	"outcome-persistence-unavailable":             true,
}

// claudeDegradationsRecognised accepts any subset of the recognised codes, including none. The
// Codex adapter and the Claude Code dogfood envelope apply it too (renderAdapterResult). A present
// value that is not a list, JSON null included, is refused rather than read as empty.
func claudeDegradationsRecognised(result map[string]any) bool {
	degradations, present := result["degradations"]
	list, isList := degradations.([]any)
	if present && !isList {
		return false
	}
	for _, item := range list {
		code, _ := item.(string)
		if !claudeRecognisedDegradation[code] {
			return false
		}
	}
	return true
}

// claudeMaxNamedCodes is the size of the recognised receipt degradation set in
// integrations/compatibility.json.
const claudeMaxNamedCodes = 6

var claudeSafeCode = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// claudeNamedCodes names every receipt degradation code, or counts the ones it leaves out
// ("+N more"), per the compatibility display rule. The count is taken against the raw list, so
// an entry that is not a safe code is signalled rather than interpolated or hidden.
func claudeNamedCodes(degradations any) string {
	list, _ := degradations.([]any)
	named := make([]string, 0, len(list))
	for _, item := range list {
		code, _ := item.(string)
		if claudeSafeCode.MatchString(code) && len(named) < claudeMaxNamedCodes {
			named = append(named, code)
		}
	}
	if elided := len(list) - len(named); elided > 0 {
		named = append(named, fmt.Sprintf("+%d more", elided))
	}
	if len(named) == 0 {
		return ""
	}
	return "; " + strings.Join(named, ",")
}

// claudeDegradedOutput moves an expected degradation to additionalContext where the event
// accepts it. A fault, or an expected reason on an event without that channel (where dropping
// it would leave it recorded nowhere), keeps the user-visible systemMessage.
func claudeDegradedOutput(event, reason string) map[string]any {
	name, quiet := claudeContextEvents[event]
	if !quiet || !claudeExpectedDegradation[reason] {
		return degradedAdapterOutput(reason)
	}
	return claudeContextOutput(name, "Corvint FALLBACK degraded: "+reason+"; coding continues")
}

// claudeSnapshotStaleReason is the dogfood event rejection that names a stale index snapshot as
// the cause of an expired event (AHI-031).
const claudeSnapshotStaleReason = "corvint-event-rejected:dogfood-event-index-snapshot-stale"

// withSnapshotRemediation appends the explicit refresh argv to a stale-snapshot fault notice and,
// where the event accepts additionalContext, gives the model the same text (AHI-031).
func withSnapshotRemediation(root, event, reason string, output map[string]any) map[string]any {
	if reason != claudeSnapshotStaleReason {
		return output
	}
	refresh, _ := json.Marshal([]string{"corvint", "--root", root, "index", "--if-stale"})
	text := output["systemMessage"].(string) + "\nNo index snapshot matches the current tree, so the in-memory build ran past the hook deadline. Refresh the snapshot once, outside the hook:\n" + string(refresh)
	output["systemMessage"] = text
	if name, ok := claudeContextEvents[event]; ok {
		output["hookSpecificOutput"] = map[string]any{"hookEventName": name, "additionalContext": text}
	}
	return output
}

// postToolChangeOutOfRoot reports whether a PostToolUse payload names a
// file-mutating tool (the hooks.json PostToolUse matcher: Edit|Write|
// NotebookEdit) whose target path resolves outside the project root. It reads
// only the raw host payload already trusted by the caller and never widens
// the corvint-harness-event/0 wire input.
func postToolChangeOutOfRoot(root string, payload map[string]any) bool {
	input, ok := payload["tool_input"].(map[string]any)
	if !ok {
		return false
	}
	name, ok := payload["tool_name"].(string)
	if !ok {
		return false
	}
	fields := map[string]string{"Edit": "file_path", "Write": "file_path", "NotebookEdit": "notebook_path"}
	field, known := fields[name]
	if !known {
		return false
	}
	path, ok := input[field].(string)
	if !ok || path == "" {
		return false
	}
	_, relative := projectRelativePath(root, path)
	return !relative
}

// adapterMaxErrorCodeTail bounds how much of a failing invocation's stderr is
// scanned for the engine's structured error code, so an unexpectedly large
// diagnostic cannot make this path do unbounded work.
const adapterMaxErrorCodeTail = 4096
const adapterMaxErrorCodeBytes = 96

type adapterErrorTail struct {
	bytes []byte
}

func (tail *adapterErrorTail) Write(value []byte) (int, error) {
	written := len(value)
	if len(value) >= adapterMaxErrorCodeTail {
		tail.bytes = append(tail.bytes[:0], value[len(value)-adapterMaxErrorCodeTail:]...)
		return written, nil
	}
	overflow := len(tail.bytes) + len(value) - adapterMaxErrorCodeTail
	if overflow > 0 {
		copy(tail.bytes, tail.bytes[overflow:])
		tail.bytes = tail.bytes[:len(tail.bytes)-overflow]
	}
	tail.bytes = append(tail.bytes, value...)
	return written, nil
}

func (tail *adapterErrorTail) Bytes() []byte {
	return tail.bytes
}

// adapterRejectedReason turns a non-zero in-process engine invocation into the
// hook's degradation reason. emitError (main.go) writes one line of JSON
// carrying a "code" field when the failure has a known code; previously that
// code was discarded and every rejection reported as the bare
// "corvint-event-rejected", which is indistinguishable from an unknown failure.
// When a code is recoverable it is appended so the degradation is
// diagnosable; otherwise the bare reason is unchanged.
func adapterRejectedReason(stderr []byte) string {
	const reason = "corvint-event-rejected"
	tail := stderr
	if len(tail) > adapterMaxErrorCodeTail {
		tail = tail[len(tail)-adapterMaxErrorCodeTail:]
	}
	var parsed struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(bytes.TrimSpace(tail), &parsed) != nil || !validAdapterErrorCode(parsed.Code) {
		return reason
	}
	return reason + ":" + parsed.Code
}

func validAdapterErrorCode(code string) bool {
	if len(code) < 1 || len(code) > adapterMaxErrorCodeBytes {
		return false
	}
	for _, char := range code {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char != '-' {
			return false
		}
	}
	return true
}

func projectRelativePath(root string, value any) (string, bool) {
	path, ok := value.(string)
	if !ok || path == "" {
		return "", false
	}
	return projectpath.Relative(root, path)
}

func hashAdapterSession(domain, value string) string {
	sum := sha256.Sum256([]byte(domain + "\x00" + value))
	return hex.EncodeToString(sum[:])
}
func claudeEventName(event string) string {
	if event == "session-start" {
		return "SessionStart"
	}
	return "UserPromptSubmit"
}
func degradedAdapterOutput(reason string) map[string]any {
	return map[string]any{"systemMessage": "Corvint FALLBACK degraded: " + reason + "; coding continues"}
}

func codexDegraded(event, reason string) map[string]any {
	message := "Corvint fallback: " + reason + "; unrelated coding continues."
	if event == "SessionStart" || event == "UserPromptSubmit" {
		return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": event, "additionalContext": message}}
	}
	return map[string]any{"systemMessage": message}
}
func emitAdapterOutput(stdout io.Writer, value map[string]any) int {
	raw, err := json.Marshal(value)
	if err != nil || len(raw)+1 > adapterOutputLimit {
		raw, _ = json.Marshal(degradedAdapterOutput("corvint-output-too-large"))
	}
	if text, ok := value[adapterPlainStdoutKey].(string); ok && err == nil && len(raw)+1 <= adapterOutputLimit {
		raw = []byte(text) // the host reads this event's stdout as text, not hook JSON (decision 0340)
	}
	raw = append(raw, '\n')
	_, _ = stdout.Write(raw)
	return 0
}

// claudeHostVersion is what the Claude Code adapter reports as its host version (AHI-023). Claude Code
// documents no version in its hook payload or hook environment, so the adapter names that fact once
// instead of carrying a per-receipt `host-version-unknown` degradation.
const claudeHostVersion = "unreported-by-hook-api"

// dogfoodHostVersions is the one admitted host-version spelling per plugin host (LCP-V0-009).
var dogfoodHostVersions = map[string]string{"codex": "unknown", "claude-code": claudeHostVersion}
