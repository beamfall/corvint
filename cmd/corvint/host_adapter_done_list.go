package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// The done-list (AHI-190 to AHI-197, V1-0403) names what a turn's change still owes: the selected
// tests, the unwitnessed committed ranges, and whether each new test kills a mutant. The Stop hook
// form is opt-in, user-facing only, and never runs witness or mutation (AHI-194); the explicit
// `corvint adapter done-list` command runs them on a committed range.
const (
	doneListProfile    = "corvint-done-list/0"
	doneListOptIn      = "CORVINT_STOP_DONE_LIST"
	doneListPluginOpt  = "CLAUDE_PLUGIN_OPTION_STOP_DONE_LIST"
	doneListUnitLimit  = 8
	doneListPathLimit  = 8
	doneListGitLimit   = 1 << 20
	doneListMessageCap = 2000
)

// doneListHeadReasons are the Stop hook's plain words for a receipt refused by its HEAD binding.
var doneListHeadReasons = map[string]string{"head-moved": "HEAD moved while it ran", "head-unavailable": "HEAD names no commit"}

var doneListFullSHA = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// adapterStdoutVerbs are the `corvint adapter <verb>` argv commands that read no hook payload. It
// is filled in init because done-list reaches runContext, which reaches runHostAdapter.
var adapterStdoutVerbs map[string]func(context.Context, []string, io.Writer) int

func init() {
	adapterStdoutVerbs = map[string]func(context.Context, []string, io.Writer) int{
		"source-view": runSourceViewAdapter,
		"done-list":   runDoneListCommand,
	}
	// AHI-194: the done-list Stop hook's 10-second kill, beside the AHI-017 table.
	adapterDeclaredHostKill["claude-code stop-done-list"] = 10 * time.Second
}

// runClaudeUnlistedEvent serves the Claude Code events outside the shared lifecycle set:
// subagent-start and pre-tool, which reuse the user-prompt path (AHI-804, AHI-805), and stop-done-list, which owes no
// lifecycle record and so skips normalization and degradation recording.
func runClaudeUnlistedEvent(ctx context.Context, event string, payload map[string]any) map[string]any {
	if event == "subagent-start" {
		return runClaudeSubagentStart(ctx, payload)
	}
	if event == "pre-tool" {
		return runClaudePreToolRead(ctx, payload)
	}
	if event != "stop-done-list" {
		return unsupportedHookEvent("claude-code")
	}
	root, err := claudeAdapterRoot(ctx)
	if err != nil {
		return degradedAdapterOutput("project-root-unavailable")
	}
	if !insideGitRepository(root) {
		return map[string]any{}
	}
	return runClaudeStopDoneList(ctx, root, payload)
}

// doneListOptedIn reports the Stop done-list opt-in (AHI-807, V1-2514): a set
// CORVINT_STOP_DONE_LIST decides alone (only "1" enables), otherwise the plugin's boolean
// stop_done_list userConfig option, which the host exports as CLAUDE_PLUGIN_OPTION_STOP_DONE_LIST.
func doneListOptedIn(ctx context.Context) bool {
	if value := adapterGetenv(ctx, doneListOptIn); value != "" {
		return value == "1"
	}
	option := adapterGetenv(ctx, doneListPluginOpt)
	return option == "true" || option == "1"
}

// runClaudeStopDoneList is the `claude-code stop-done-list` hook event. It stays silent unless the
// user opted in, on a continued Stop, in a subagent, and on a clean tree (AHI-191, AHI-192).
func runClaudeStopDoneList(ctx context.Context, root string, payload map[string]any) map[string]any {
	if !doneListOptedIn(ctx) {
		return map[string]any{}
	}
	if active, _ := payload["stop_hook_active"].(bool); active || claudeSubagent(payload) {
		return map[string]any{}
	}
	receipt := doneList(ctx, root, "", false)
	if receipt["clean"] == true {
		return map[string]any{}
	}
	if receipt["ok"] != true {
		code, _ := receipt["error"].(map[string]any)["code"].(string)
		return map[string]any{"systemMessage": "Corvint done-list: not reported: " + doneListHeadReasons[code] + " (" + code + ")"}
	}
	// AHI-193: systemMessage only. On Stop, additionalContext or a block decision would continue
	// the turn, and the completion policy, not this list, owns that decision.
	return map[string]any{"systemMessage": renderDoneList(root, receipt)}
}

// runDoneListCommand is `corvint adapter done-list [--root PATH] [--base FULL_SHA] [--mutate]`
// (AHI-195). It reads no stdin and prints one enveloped corvint-done-list/0 receipt.
func runDoneListCommand(ctx context.Context, arguments []string, stdout io.Writer) int {
	root, base, mutate := "", "", false
	for index := 0; index < len(arguments); index++ {
		switch argument := arguments[index]; argument {
		case "--mutate":
			mutate = true
		case "--root", "--base":
			if index+1 >= len(arguments) {
				return emitDoneList(stdout, doneListRefusal("invalid-arguments"), 2)
			}
			index++
			if argument == "--root" {
				root = arguments[index]
			} else {
				base = arguments[index]
			}
		default:
			return emitDoneList(stdout, doneListRefusal("invalid-arguments"), 2)
		}
	}
	if (base != "" && !doneListFullSHA.MatchString(base)) || (mutate && base == "") {
		return emitDoneList(stdout, doneListRefusal("invalid-arguments"), 2)
	}
	if root == "" {
		var err error
		if root, err = claudeAdapterRoot(ctx); err != nil {
			return emitDoneList(stdout, doneListRefusal("project-root-unavailable"), 2)
		}
	}
	if !insideGitRepository(root) {
		return emitDoneList(stdout, doneListRefusal("not-a-git-repository"), 2)
	}
	receipt := doneList(ctx, root, base, mutate)
	if receipt["ok"] != true {
		return emitDoneList(stdout, receipt, 1)
	}
	return emitDoneList(stdout, receipt, 0)
}

func doneListRefusal(code string) map[string]any {
	return map[string]any{"profile": doneListProfile, "ok": false, "mutates": false, "error": map[string]any{"code": code}}
}

func emitDoneList(stdout io.Writer, receipt map[string]any, code int) int {
	raw, err := json.Marshal(receipt)
	if err != nil {
		raw, code = []byte(`{"profile":"`+doneListProfile+`","ok":false,"mutates":false,"error":{"code":"encoding-failed"}}`), 1
	}
	framed, frameErr := repoenvelope.Frame(string(raw))
	if frameErr != nil {
		framed, _ = repoenvelope.Frame(`{"profile":"` + doneListProfile + `","ok":false,"mutates":false,"error":{"code":"` + repoenvelope.CollisionCode + `"}}`)
		code = 1
	}
	_, _ = stdout.Write(append([]byte(framed), '\n'))
	return code
}

// doneList builds the receipt. With base empty it describes the working tree against HEAD; with a
// base it describes the committed range base..HEAD plus any dirty paths affected sees (AHI-196).
// The receipt names the commit it describes as head: the one HEAD named before any verb ran and
// still names after the last.
func doneList(ctx context.Context, root, base string, mutate bool) map[string]any {
	// The next base is the commit the working tree sits on: after committing, base..HEAD is
	// exactly this change.
	start, err := doneListHead(ctx, root)
	if err != nil {
		return doneListHeadRefusal("head-unavailable", root, base)
	}
	receipt := map[string]any{"profile": doneListProfile, "ok": true, "mutates": false, "root": root, "base": base}
	affected, dirty := doneListAffected(ctx, root, base)
	receipt["affected"] = affected
	newTests, newErr := doneListNewTests(ctx, root, base)
	receipt["clean"] = base == "" && dirty == 0 && newErr == nil && len(newTests) == 0
	receipt["witness"] = doneListWitness(ctx, root, base, start)
	receipt["mutation"] = doneListMutation(ctx, root, base, start, mutate, newTests, newErr)
	return doneListBindHead(ctx, root, start, receipt)
}

var errDoneListHead = errors.New("HEAD names no full commit identifier")

// doneListHead is the full commit identifier HEAD names; an unborn, unreadable or malformed HEAD
// is an error, never an empty identifier two probes could agree on.
func doneListHead(ctx context.Context, root string) (string, error) {
	head, err := sourceGitText(ctx, root, 128, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", err
	}
	if !doneListFullSHA.MatchString(head) {
		return "", errDoneListHead
	}
	return head, nil
}

// doneListBindHead keeps the receipt only when HEAD still names the commit it named before any
// verb ran: each verb resolves HEAD itself, so a commit in between would mix two ranges (AHI-196).
// Both probes must name a commit; a failed probe refuses instead of matching another failure.
func doneListBindHead(ctx context.Context, root, start string, receipt map[string]any) map[string]any {
	head, err := doneListHead(ctx, root)
	switch {
	case err != nil || !doneListFullSHA.MatchString(start):
		return doneListHeadRefusal("head-unavailable", root, receipt["base"])
	case head != start:
		return doneListHeadRefusal("head-moved", root, receipt["base"])
	}
	receipt["head"] = start
	return receipt
}

func doneListHeadRefusal(code, root string, base any) map[string]any {
	refusal := doneListRefusal(code)
	refusal["root"], refusal["base"] = root, base
	return refusal
}

// doneListRun runs one read-only Corvint command in process and decodes its JSON stdout.
func doneListRun(ctx context.Context, root string, arguments ...string) (map[string]any, string) {
	if ctx.Err() != nil {
		return nil, "deadline"
	}
	var stdout, stderr bytes.Buffer
	code := runContext(ctx, append([]string{"--root", root}, arguments...), strings.NewReader(""), &stdout, &stderr)
	if ctx.Err() != nil {
		return nil, "deadline"
	}
	var decoded map[string]any
	if code != 0 || json.Unmarshal(stdout.Bytes(), &decoded) != nil {
		var failure map[string]any
		if json.Unmarshal(stderr.Bytes(), &failure) == nil {
			if text, ok := failure["code"].(string); ok && text != "" {
				return nil, arguments[0] + "-failed:" + text
			}
		}
		return nil, arguments[0] + "-failed:exit-" + strconv.Itoa(code)
	}
	return decoded, ""
}

func doneListAffected(ctx context.Context, root, base string) (map[string]any, int) {
	arguments := []string{"affected"}
	if base != "" {
		arguments = append(arguments, "--base", base)
	}
	plan, reason := doneListRun(ctx, root, arguments...)
	if reason != "" {
		return map[string]any{"state": "NOT_RUN", "reason": reason}, -1
	}
	body, _ := plan["plan"].(map[string]any)
	tiers, _ := plan["tiers"].(map[string]any)
	selected, _ := body["selected"].([]any)
	unknown, _ := body["unknown"].([]any)
	dirty, _ := body["dirty"].([]any)
	units := []any{}
	for _, value := range selected {
		if len(units) == doneListUnitLimit {
			break
		}
		if unit, ok := value.(map[string]any); ok {
			units = append(units, map[string]any{"unitId": unit["unitId"], "testCount": unit["testCount"]})
		}
	}
	reasons := []any{}
	for _, value := range unknown {
		if len(reasons) == doneListUnitLimit {
			break
		}
		if row, ok := value.(map[string]any); ok {
			reasons = append(reasons, strings.TrimSpace(stringField(row, "reason")+" "+stringField(row, "detail")))
		}
	}
	return map[string]any{
		"state": "COMPLETED", "scope": body["scope"], "selected": len(selected), "units": units,
		"unknown": len(unknown), "unknownReasons": reasons, "dirty": len(dirty), "tiers": stringField(tiers, "summary"),
	}, len(dirty)
}

func stringField(value map[string]any, key string) string {
	text, _ := value[key].(string)
	return text
}

// doneListWitness reports the unwitnessed obligations of base..HEAD. Witness compiles committed
// ranges only, so the working-tree form names the command to run after committing (AHI-194).
func doneListWitness(ctx context.Context, root, base, next string) map[string]any {
	if base == "" {
		return map[string]any{"state": "NOT_RUN", "reason": "witness-requires-committed-range", "argv": []string{"corvint", "--root", root, "witness", "--base", next}}
	}
	report, reason := doneListRun(ctx, root, "witness", "--base", base, "--json")
	if reason != "" {
		return map[string]any{"state": "NOT_RUN", "reason": reason}
	}
	summary, _ := report["summary"].(map[string]any)
	scope, _ := report["range"].(map[string]any)
	obligations, _ := report["obligations"].([]any)
	paths := []any{}
	for _, value := range obligations {
		row, ok := value.(map[string]any)
		if !ok || row["verdict"] == "CLOSED" {
			continue
		}
		if text := stringField(row, "path"); text != "" && !slices.Contains(paths, any(text)) && len(paths) < doneListPathLimit {
			paths = append(paths, text)
		}
	}
	return map[string]any{
		"state": "COMPLETED", "opened": summary["opened"], "unproven": summary["unproven"], "notRun": summary["notRun"],
		"closed": summary["closed"], "closure": scope["closure"], "unwitnessedPaths": paths,
	}
}

// doneListNewTests lists test files the change adds: added or untracked in the working tree, or
// added in base..HEAD when a base is given.
func doneListNewTests(ctx context.Context, root, base string) ([]string, error) {
	var tests []string
	if base != "" {
		raw, err := sourceGit(ctx, root, doneListGitLimit, "diff", "--no-renames", "--name-only", "-z", "--diff-filter=A", base, "HEAD")
		if err != nil {
			return nil, err
		}
		for _, name := range strings.Split(string(raw), "\x00") {
			if name != "" && doneListTestPath(name) {
				tests = append(tests, name)
			}
		}
		return tests, nil
	}
	// The worktree status runs through gitstatus.Status on private metadata, as affected's does,
	// so no repository-defined clean or process filter executes; a refusal there is an error.
	raw, err := gitstatus.Status(ctx, root, doneListGitLimit, sourceGit, "status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	for _, entry := range strings.Split(string(raw), "\x00") {
		if len(entry) < 4 {
			continue
		}
		status, name := entry[:2], entry[3:]
		if (status == "??" || status[0] == 'A') && doneListTestPath(name) {
			tests = append(tests, name)
		}
	}
	return tests, nil
}

func doneListTestPath(name string) bool {
	base := strings.ToLower(path.Base(name))
	return strings.HasSuffix(base, "_test.go") || strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, "_spec.rb") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
}

// doneListMutation judges each new Go test by the mutants it kills on base..HEAD (AHI-197). The
// hook form never mutates: the mutation budget is minutes, the hook kill is seconds (AHI-194).
func doneListMutation(ctx context.Context, root, base, next string, mutate bool, newTests []string, newErr error) map[string]any {
	if newErr != nil {
		return map[string]any{"state": "NOT_RUN", "reason": "new-tests-unavailable", "newTests": []any{}}
	}
	if len(newTests) == 0 {
		return map[string]any{"state": "NOT_APPLICABLE", "reason": "no-new-tests", "newTests": []any{}}
	}
	rows := map[string]map[string]any{}
	state, reason := "NOT_RUN", ""
	switch {
	case base == "":
		reason = "mutation-not-run-from-hook"
	case !mutate:
		reason = "mutation-not-requested"
	default:
		proof, failure := doneListRun(ctx, root, "prove", "--base", base, "--mutate", "--attack-tests")
		if failure != "" {
			reason = failure
			break
		}
		state = "COMPLETED"
		packet, _ := proof["proof"].(map[string]any)
		listed, _ := packet["rows"].([]any)
		for _, value := range listed {
			if row, ok := value.(map[string]any); ok && row["kind"] == kindAffectedTest {
				rows[stringField(row, "result")] = row
			}
		}
	}
	judged := []any{}
	for _, test := range newTests {
		entry := map[string]any{"path": test}
		switch row, found := rows[test]; {
		case !strings.HasSuffix(test, ".go"):
			entry["verdict"], entry["reason"] = "NOT_RUN", "mutation-unsupported-language"
		case state != "COMPLETED":
			entry["verdict"], entry["reason"] = "NOT_RUN", reason
		case !found:
			entry["verdict"], entry["reason"] = "NOT_RUN", "no-affected-test-row"
		default:
			entry["verdict"], entry["reason"] = doneListVerdict(row, entry)
		}
		judged = append(judged, entry)
	}
	result := map[string]any{"state": state, "newTests": judged}
	if reason != "" {
		result["reason"] = reason
	}
	if base == "" {
		result["argv"] = []string{"corvint", "adapter", "done-list", "--root", root, "--base", next, "--mutate"}
	} else if !mutate {
		result["argv"] = []string{"corvint", "adapter", "done-list", "--root", root, "--base", base, "--mutate"}
	}
	return result
}

// doneListVerdict maps one affected-test attack report to a verdict. A completed or partial attack
// that executed mutants and killed none is KILLS_NO_MUTANT: the test passes whatever the changed
// lines do. Uncompilable and skipped mutants are not executions, so they never support it.
func doneListVerdict(row map[string]any, entry map[string]any) (string, string) {
	attack, ok := row["attack"].(map[string]any)
	if !ok {
		return "NOT_RUN", "attack-unavailable:" + stringField(row, "detail")
	}
	for _, key := range []string{"mutants", "killed", "survived"} {
		entry[key] = attack[key]
	}
	status := stringField(attack, "status")
	mutants, _ := attack["mutants"].(float64)
	killed, _ := attack["killed"].(float64)
	survived, _ := attack["survived"].(float64)
	switch {
	case status != "COMPLETED" && status != "PARTIAL":
		return "NOT_RUN", "attack-" + strings.ToLower(status) + ":" + stringField(attack, "detail")
	case mutants == 0:
		return "NOT_RUN", "no-mutants"
	case killed+survived == 0:
		return "NOT_RUN", "no-mutant-executed"
	case killed == 0:
		return "KILLS_NO_MUTANT", status
	}
	return "KILLS_MUTANTS", status
}

// renderDoneList is the Stop hook's user-facing text: plain lines, control characters dropped,
// capped at doneListMessageCap bytes (AHI-193).
func renderDoneList(root string, receipt map[string]any) string {
	var lines []string
	affected := receipt["affected"].(map[string]any)
	if affected["state"] == "COMPLETED" {
		line := "tests: " + strconv.Itoa(affected["selected"].(int)) + " units selected (" + affected["tiers"].(string) + ")"
		var units []string
		for _, value := range affected["units"].([]any) {
			unit := value.(map[string]any)
			count, noun := doneListText(unit["testCount"]), " tests)"
			if count == "1" {
				noun = " test)"
			}
			units = append(units, doneListText(unit["unitId"])+" ("+count+noun)
		}
		if len(units) > 0 {
			line += ": " + strings.Join(units, ", ")
		}
		if more := affected["selected"].(int) - len(units); more > 0 {
			line += " and " + strconv.Itoa(more) + " more"
		}
		if count := affected["unknown"].(int); count > 0 {
			line += "; " + strconv.Itoa(count) + " unknowns"
		}
		lines = append(lines, line)
	} else {
		lines = append(lines, "tests: NOT_RUN "+doneListText(affected["reason"]))
	}
	witness := receipt["witness"].(map[string]any)
	lines = append(lines, "unwitnessed ranges: NOT_RUN "+doneListText(witness["reason"])+"; after committing run "+doneListArgv(witness["argv"]))
	mutation := receipt["mutation"].(map[string]any)
	if mutation["state"] == "NOT_APPLICABLE" {
		lines = append(lines, "new-test mutation: NOT_APPLICABLE no new tests")
	} else {
		var tests []string
		for _, value := range mutation["newTests"].([]any) {
			entry := value.(map[string]any)
			test := doneListText(entry["path"])
			if entry["reason"] != mutation["reason"] {
				test += " " + doneListText(entry["reason"])
			}
			tests = append(tests, test)
		}
		line := "new-test mutation: NOT_RUN " + doneListText(mutation["reason"])
		if len(tests) > 0 {
			line += " for " + strings.Join(tests, ", ")
		}
		if argv, ok := mutation["argv"]; ok {
			line += "; after committing run " + doneListArgv(argv)
		}
		lines = append(lines, line)
	}
	text := "Corvint done-list:\n" + strings.Join(lines, "\n")
	if len(text) > doneListMessageCap {
		cut := doneListMessageCap - len(" [truncated]")
		for cut > 0 && !utf8RuneStart(text[cut]) {
			cut--
		}
		text = text[:cut] + " [truncated]"
	}
	return text
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

func doneListArgv(value any) string {
	raw, _ := json.Marshal(value)
	return doneListText(string(raw))
}

// doneListText renders a value as one line: control and hidden characters are dropped so a
// repository path cannot break or reorder the message.
func doneListText(value any) string {
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case nil:
		text = ""
	default:
		raw, _ := json.Marshal(typed)
		text = string(raw)
	}
	text = repoenvelope.EscapeHidden(text)
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
}
