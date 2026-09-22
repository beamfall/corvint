package main

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const (
	dogfoodProfile   = "corvint-dogfood-event/0"
	qualifiedProfile = "corvint-qualified-lifecycle/0"
)

var stopReceiptPattern = regexp.MustCompile(`^Corvint qualified lifecycle (FULL|FALLBACK)/(QUALIFIED|UNQUALIFIED) \((qualified-native-runtime|qualified-shared-runtime|candidate-native-runtime|candidate-shared-runtime)\); Frontier (OPEN|EMPTY) \(VERIFIED\); local completion (block|release) \(([a-z][a-z-]{0,127})\); request provenance caller-asserted; result (qualified-lifecycle:sha256:[0-9a-f]{64})\.( One remediation is requested by the enrolled local policy or protected Frontier permission\.| This Stop releases without a closure claim\.)$`)

type comparison struct {
	gold         map[string]any
	goldDigest   string
	cases        []any
	inventory    []map[string]any
	completed    int
	sequence     int
	started      map[string]string
	seen         map[string]bool
	continuation *continuationWitness
}

func loadComparison(path, cwd string) (*comparison, error) {
	decoded, err := loadRegularJSON(path, maxFrameBytes)
	if err != nil {
		return nil, err
	}
	gold, err := asObject(decoded, "comparison-fields")
	if err != nil {
		return nil, err
	}
	withContinuation := gold["continuation"] != nil
	if withContinuation {
		if exactFields(gold, "profile", "cwdSHA256", "cases", "continuation") != nil {
			return nil, errors.New("comparison-fields")
		}
	} else if exactFields(gold, "profile", "cwdSHA256", "cases") != nil {
		return nil, errors.New("comparison-fields")
	}
	profile := "corvint-native-comparison-gold/0-experimental"
	if withContinuation {
		profile = "corvint-native-comparison-gold/1-experimental"
	}
	cwdDigest, _ := textDigest(cwd, 4096)
	if gold["profile"] != profile || gold["cwdSHA256"] != cwdDigest {
		return nil, errors.New("comparison-root")
	}
	cases, err := asArray(gold["cases"], "comparison-case-limit")
	if err != nil || len(cases) < 1 || len(cases) > 32 {
		return nil, errors.New("comparison-case-limit")
	}
	for _, rawCase := range cases {
		caseValue, err := asObject(rawCase, "comparison-case")
		if err != nil {
			return nil, err
		}
		if err = validateComparisonCase(caseValue); err != nil {
			return nil, err
		}
	}
	digest, err := digestValue(gold)
	if err != nil {
		return nil, err
	}
	result := &comparison{gold: gold, goldDigest: digest, cases: cases, started: map[string]string{}, seen: map[string]bool{}}
	if withContinuation {
		config, err := asObject(gold["continuation"], "comparison-continuation")
		if err != nil || exactFields(config, "blockedCase", "releasedCase", "resultTextSHA256") != nil {
			return nil, errors.New("comparison-continuation")
		}
		witness, err := newContinuation(config)
		if err != nil {
			return nil, err
		}
		if witness.releasedCase >= len(cases) {
			return nil, errors.New("comparison-continuation-case")
		}
		blocked := cases[witness.blockedCase].(map[string]any)
		released := cases[witness.releasedCase].(map[string]any)
		if blocked["kind"] != "stop" || blocked["decision"] != "block" || blocked["stopHookActive"] != false || released["kind"] != "stop" || released["decision"] != "release" || released["stopHookActive"] != true {
			return nil, errors.New("comparison-continuation-binding")
		}
		if config["resultTextSHA256"] != nil && !validSHA(config["resultTextSHA256"]) {
			return nil, errors.New("comparison-digest")
		}
		result.continuation = witness
	}
	return result, nil
}

func validateComparisonCase(value map[string]any) error {
	kind, _ := value["kind"].(string)
	common := []string{"kind", "eventName", "sourcePathSHA256", "hookSHA256", "scope"}
	switch kind {
	case "context":
		if exactFields(value, append(common, "receiptProfile", "input", "repository", "selectorsSHA256", "criticalMissingSHA256", "unavailableSelectorsSHA256")...) != nil {
			return errors.New("comparison-fields")
		}
		if value["eventName"] != "sessionStart" && value["eventName"] != "userPromptSubmit" || value["receiptProfile"] != dogfoodProfile && value["receiptProfile"] != qualifiedProfile {
			return errors.New("comparison-context-profile")
		}
		input, err := asObject(value["input"], "comparison-gold-input")
		if err != nil {
			return err
		}
		allowed := stringSet("sessionIdSha256", "startSource")
		if value["eventName"] == "userPromptSubmit" {
			allowed = stringSet("sessionIdSha256", "task")
		}
		for key := range input {
			if !allowed[key] {
				return errors.New("comparison-request-fields")
			}
		}
		if input["sessionIdSha256"] != nil && !validSHA(input["sessionIdSha256"]) {
			return errors.New("comparison-digest")
		}
		if value["eventName"] == "sessionStart" && !stringSet("startup", "resume", "clear", "compact")[fmt.Sprint(input["startSource"])] {
			return errors.New("comparison-start-source")
		}
		if value["eventName"] == "userPromptSubmit" {
			task, ok := input["task"].(string)
			if !ok || task == "" || strings.TrimSpace(task) != task || len([]rune(task)) > 2000 || len([]byte(task)) > 16384 {
				return errors.New("comparison-task")
			}
		}
		if _, err := asObject(value["repository"], "comparison-gold-input"); err != nil {
			return err
		}
		for _, field := range []string{"selectorsSHA256", "criticalMissingSHA256", "unavailableSelectorsSHA256"} {
			if !validSHA(value[field]) {
				return errors.New("comparison-digest")
			}
		}
	case "stop":
		if exactFields(value, append(common, "entryKind", "textSHA256", "resultDigest", "decision", "stopHookActive")...) != nil {
			return errors.New("comparison-fields")
		}
		if value["eventName"] != "stop" || !entryKinds[fmt.Sprint(value["entryKind"])] || value["decision"] != "block" && value["decision"] != "release" || !validSHA(value["textSHA256"]) {
			return errors.New("comparison-stop")
		}
		if matched, _ := regexp.MatchString(`^qualified-lifecycle:sha256:[0-9a-f]{64}$`, fmt.Sprint(value["resultDigest"])); !matched {
			return errors.New("comparison-stop-digest")
		}
		if _, ok := value["stopHookActive"].(bool); !ok {
			return errors.New("comparison-recursion-state")
		}
	case "no-receipt":
		if exactFields(value, common...) != nil || value["eventName"] != "sessionEnd" {
			return errors.New("comparison-no-receipt")
		}
	default:
		return errors.New("comparison-kind")
	}
	if !validSHA(value["sourcePathSHA256"]) || !validSHA(value["hookSHA256"]) {
		return errors.New("comparison-digest")
	}
	if value["scope"] != "thread" && value["scope"] != "turn" {
		return errors.New("comparison-scope")
	}
	return nil
}

func (c *comparison) bindInventory(inventory map[string]any) error {
	if c.inventory != nil || c.completed != 0 || len(c.started) > 0 {
		return errors.New("comparison-inventory-rebind")
	}
	hooksRaw, err := asArray(inventory["hooks"], "comparison-inventory")
	if err != nil {
		return err
	}
	for _, rawCase := range c.cases {
		caseValue := rawCase.(map[string]any)
		matches := []map[string]any{}
		for _, rawHook := range hooksRaw {
			hook := rawHook.(map[string]any)
			digest, _ := digestValue(hook)
			if digest == caseValue["hookSHA256"] {
				matches = append(matches, hook)
			}
		}
		if len(matches) != 1 {
			return errors.New("comparison-inventory-mapping")
		}
		hook := matches[0]
		if hook["eventName"] != caseValue["eventName"] || hook["sourcePathSHA256"] != caseValue["sourcePathSHA256"] || hook["enabled"] != true || hook["async"] != false {
			return errors.New("comparison-inventory-incompatible")
		}
		compatible := 0
		for _, rawHook := range hooksRaw {
			candidate := rawHook.(map[string]any)
			if candidate["enabled"] == true && candidate["eventName"] == hook["eventName"] && candidate["sourcePathSHA256"] == hook["sourcePathSHA256"] && candidate["source"] == hook["source"] && candidate["displayOrder"] == hook["displayOrder"] && candidate["async"] == hook["async"] {
				compatible++
			}
		}
		if compatible != 1 {
			return errors.New("comparison-inventory-ambiguous")
		}
		c.inventory = append(c.inventory, hook)
	}
	return nil
}

func observationKeys(value map[string]any) (string, string, error) {
	params, err := asObject(value["params"], "comparison-notification")
	if err != nil {
		return "", "", err
	}
	run, err := asObject(params["run"], "comparison-notification")
	if err != nil {
		return "", "", err
	}
	thread, err := textDigest(params["threadId"], 4096)
	if err != nil {
		return "", "", err
	}
	turn := ""
	if params["turnId"] != nil {
		turn, err = textDigest(params["turnId"], 4096)
		if err != nil {
			return "", "", err
		}
	}
	runID, err := textDigest(run["id"], 4096)
	if err != nil {
		return "", "", err
	}
	started, _, err := integer(run["startedAt"], false)
	if err != nil {
		return "", "", err
	}
	sourcePath, err := textDigest(run["sourcePath"], 4096)
	if err != nil {
		return "", "", err
	}
	key := strings.Join([]string{thread, turn, runID, fmt.Sprint(started)}, "\x00")
	binding := strings.Join([]string{thread, turn, fmt.Sprint(run["eventName"]), sourcePath, fmt.Sprint(run["handlerType"]), fmt.Sprint(started), fmt.Sprint(run["source"]), fmt.Sprint(run["executionMode"]), fmt.Sprint(run["scope"]), fmt.Sprint(run["displayOrder"])}, "\x00")
	return key, binding, nil
}

func (c *comparison) observe(value map[string]any) (map[string]any, error) {
	if c.inventory == nil {
		return nil, errors.New("comparison-inventory-unbound")
	}
	c.sequence++
	key, binding, err := observationKeys(value)
	if err != nil {
		return nil, err
	}
	if c.seen[key] {
		return nil, errors.New("comparison-duplicate-run")
	}
	method, _ := value["method"].(string)
	if method == "hook/started" {
		if _, ok := c.started[key]; ok || len(c.started) >= 32 {
			return nil, errors.New("comparison-duplicate-start")
		}
		c.started[key] = binding
		return nil, nil
	}
	if previous, present := c.started[key]; present {
		delete(c.started, key)
		if previous != binding {
			return nil, errors.New("comparison-run-mismatch")
		}
	} else {
		for pending := range c.started {
			parts := strings.Split(pending, "\x00")
			candidate := strings.Split(key, "\x00")
			if len(parts) == 4 && len(candidate) == 4 && parts[2] == candidate[2] && parts[3] == candidate[3] {
				return nil, errors.New("comparison-run-mismatch")
			}
		}
	}
	if c.completed >= len(c.cases) {
		return nil, errors.New("comparison-unexpected-completion")
	}
	caseValue := c.cases[c.completed].(map[string]any)
	params := value["params"].(map[string]any)
	run := params["run"].(map[string]any)
	if run["completedAt"] == nil || run["durationMs"] == nil {
		return nil, errors.New("comparison-incomplete-timing")
	}
	sourcePath, _ := textDigest(run["sourcePath"], 4096)
	if run["eventName"] != caseValue["eventName"] || sourcePath != caseValue["sourcePathSHA256"] || run["handlerType"] != "command" {
		return nil, errors.New("comparison-event-gold")
	}
	hook := c.inventory[c.completed]
	if run["source"] != hook["source"] || !equalInteger(run["displayOrder"], hook["displayOrder"]) || run["executionMode"] != "sync" || run["scope"] != caseValue["scope"] {
		return nil, errors.New("comparison-inventory-observation")
	}
	entries, err := asArray(run["entries"], "comparison-entries")
	if err != nil {
		return nil, err
	}
	var result map[string]any
	switch caseValue["kind"] {
	case "context":
		if run["status"] != "completed" || len(entries) != 1 {
			return nil, errors.New("comparison-context-entry")
		}
		entry, _ := entries[0].(map[string]any)
		if entry["kind"] != "context" {
			return nil, errors.New("comparison-context-entry")
		}
		result, err = contextProjection(entry["text"], caseValue)
	case "no-receipt":
		if run["status"] != "completed" || len(entries) != 0 {
			return nil, errors.New("comparison-no-receipt-entry")
		}
		result = map[string]any{"profile": "corvint-native-receipt-comparison/0-experimental", "qualification": "UNQUALIFIED", "outcome": "NO_RECEIPT", "event": "session-end", "continuationConsumption": "NOT_OBSERVED"}
	case "stop":
		if len(entries) != 1 {
			return nil, errors.New("comparison-stop-entry")
		}
		entry, _ := entries[0].(map[string]any)
		text, _ := entry["text"].(string)
		match := stopReceiptPattern.FindStringSubmatch(text)
		if entry["kind"] != caseValue["entryKind"] || digestBytes([]byte(text)) != caseValue["textSHA256"] || match == nil || match[7] != caseValue["resultDigest"] {
			return nil, errors.New("comparison-stop-text")
		}
		blocked := caseValue["decision"] == "block"
		if run["status"] != map[bool]string{true: "blocked", false: "completed"}[blocked] || strings.Contains(text, "One remediation") != blocked {
			return nil, errors.New("comparison-stop-decision")
		}
		result = map[string]any{"profile": "corvint-native-receipt-comparison/0-experimental", "qualification": "UNQUALIFIED", "outcome": "MATCH", "event": "stop", "textSHA256": caseValue["textSHA256"], "resultDigest": caseValue["resultDigest"], "decision": caseValue["decision"], "requestProvenance": "caller-asserted", "eventSurface": "unattributed", "continuationConsumption": "NOT_OBSERVED"}
	}
	if err != nil {
		return nil, err
	}
	if c.continuation != nil {
		if err = c.continuation.hook(c.completed, params, result, c.sequence); err != nil {
			return nil, err
		}
	}
	c.completed++
	c.seen[key] = true
	return result, nil
}

func contextProjection(rawText any, caseValue map[string]any) (map[string]any, error) {
	text, ok := rawText.(string)
	prefix, suffix := repoenvelope.Prefix, repoenvelope.Suffix
	if !ok || len([]byte(text)) > 8192 || !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, suffix) {
		return nil, errors.New("comparison-envelope")
	}
	raw := []byte(text[len(prefix) : len(text)-len(suffix)])
	decoded, err := decodeClosedJSON(raw)
	if err != nil {
		return nil, err
	}
	receipt, err := asObject(decoded, "comparison-receipt")
	if err != nil {
		return nil, err
	}
	canonicalRaw, err := canonical(receipt)
	// The emitter frames the canonical receipt with hidden characters escaped
	// and refuses a terminator collision (AHI-004); expected is "" on refusal.
	expected, _ := repoenvelope.Frame(string(canonicalRaw))
	frame, _ := canonical(map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": map[string]string{"sessionStart": "SessionStart", "userPromptSubmit": "UserPromptSubmit"}[caseValue["eventName"].(string)], "additionalContext": text}})
	if err != nil || expected != text || len(raw)+1 > 8000 || len(frame)+1 > 8000 {
		return nil, errors.New("comparison-canonical")
	}
	if receipt["profile"] != caseValue["receiptProfile"] {
		return nil, errors.New("comparison-receipt-profile")
	}
	if err = validateReceipt(receipt, caseValue); err != nil {
		return nil, err
	}
	expectedRepository, _ := canonical(caseValue["repository"])
	actualRepository, _ := canonical(receipt["repository"])
	if !bytes.Equal(expectedRepository, actualRepository) {
		return nil, errors.New("comparison-repository-gold")
	}
	contextValue, err := asObject(receipt["context"], "comparison-context")
	if err != nil {
		return nil, err
	}
	selectors := map[string]any{"governance": contextValue["governance"], "declared_scope": contextValue["declared_scope"], "task_evidence": contextValue["task_evidence"]}
	coverage, err := asObject(contextValue["coverage"], "comparison-context")
	if err != nil {
		return nil, err
	}
	checks := []struct {
		value any
		field string
	}{{selectors, "selectorsSHA256"}, {coverage["critical_missing"], "criticalMissingSHA256"}, {coverage["unavailable_selectors"], "unavailableSelectorsSHA256"}}
	for _, check := range checks {
		digest, _ := digestValue(check.value)
		if digest != caseValue[check.field] {
			return nil, errors.New("comparison-selector-gold")
		}
	}
	receiptBytes := len(raw) + 1
	return map[string]any{"profile": "corvint-native-receipt-comparison/0-experimental", "qualification": "UNQUALIFIED", "outcome": "MATCH", "receiptProfile": receipt["profile"], "event": receipt["event"], "startSource": caseValue["input"].(map[string]any)["startSource"], "requestProvenance": "caller-asserted", "eventSurface": "unattributed", "requestSHA256": receipt["requestSha256"], "receiptSHA256": digestBytes(raw), "envelopeSHA256": digestBytes([]byte(text)), "contextSHA256": mustDigest(contextValue), "receiptBytes": receiptBytes, "repository": receipt["repository"], "selectors": "MATCH", "criticalMissingCount": arrayLength(coverage["critical_missing"]), "unavailableSelectorCount": arrayLength(coverage["unavailable_selectors"]), "continuationConsumption": "NOT_OBSERVED"}, nil
}

func validateReceipt(receipt, caseValue map[string]any) error {
	profile, _ := receipt["profile"].(string)
	if profile == dogfoodProfile {
		if exactFields(receipt, "profile", "ok", "mutates", "support", "event", "adapter", "repository", "requestSha256", "degradations", "frontier", "policy", "completion", "context", "resultDigest") != nil {
			return errors.New("comparison-dogfood-fields")
		}
	} else if profile == qualifiedProfile {
		if exactFields(receipt, "profile", "ok", "mutates", "event", "requestProvenance", "requestSha256", "support", "qualification", "degradations", "qualifiedHost", "repository", "policy", "completion", "decision", "authority", "frontier", "context", "resultDigest") != nil {
			return errors.New("comparison-qualified-fields")
		}
	} else {
		return errors.New("comparison-receipt-profile")
	}
	input := caseValue["input"].(map[string]any)
	requestRaw, _ := canonical(input)
	if receipt["requestSha256"] != digestBytes(requestRaw) {
		if profile == qualifiedProfile {
			basis := map[string]any{"profile": qualifiedProfile, "event": receipt["event"], "input": input}
			digest, _ := digestValue(basis)
			if receipt["requestSha256"] != digest {
				return errors.New("comparison-qualified-request")
			}
		} else {
			return errors.New("comparison-request")
		}
	}
	basis := map[string]any{}
	for key, value := range receipt {
		if key != "resultDigest" {
			basis[key] = value
		}
	}
	basisRaw, _ := canonical(basis)
	prefix := "dogfood-event:sha256:"
	if profile == qualifiedProfile {
		prefix = "qualified-lifecycle:sha256:"
	}
	expected := prefix + digestBytes(append([]byte(profile+"\x00"), basisRaw...))
	if receipt["resultDigest"] != expected {
		return errors.New("comparison-result")
	}
	if receipt["ok"] != true || receipt["mutates"] != false {
		return errors.New("comparison-header")
	}
	wantEvent := map[string]string{"sessionStart": "session-start", "userPromptSubmit": "user-prompt"}[caseValue["eventName"].(string)]
	if receipt["event"] != wantEvent {
		return errors.New("comparison-event")
	}
	repository, err := asObject(receipt["repository"], "comparison-repository")
	if err != nil {
		return err
	}
	if err = validRepository(repository); err != nil {
		return err
	}
	if profile == dogfoodProfile {
		if receipt["support"] != "FALLBACK" {
			return errors.New("comparison-dogfood-support")
		}
		adapter, err := asObject(receipt["adapter"], "comparison-dogfood-adapter")
		if err != nil || exactFields(adapter, "host", "hostVersion", "surface", "adapterVersion") != nil || adapter["host"] != "codex" || adapter["surface"] != "plugin" {
			return errors.New("comparison-dogfood-adapter")
		}
		frontier, err := asObject(receipt["frontier"], "comparison-dogfood-frontier")
		if err != nil || !sameJSON(frontier, map[string]any{"state": "UNAVAILABLE", "shouldContinue": false, "reason": "frontier-authority-unavailable"}) {
			return errors.New("comparison-dogfood-frontier")
		}
	} else if profile == qualifiedProfile {
		if receipt["requestProvenance"] != "caller-asserted" || receipt["authority"] != "NONE" {
			return errors.New("comparison-qualified-authority")
		}
		host, err := asObject(receipt["qualifiedHost"], "comparison-qualified-host")
		if err != nil || exactFields(host, "host", "digest", "evidenceSHA256", "appSHA256", "engineSHA256", "adapterSHA256", "osBuild", "architecture", "supportScope", "eventSurface", "qualifiedSurfaces") != nil || host["host"] != "codex" || host["eventSurface"] != "unattributed" || host["architecture"] != "arm64" && host["architecture"] != "amd64" {
			return errors.New("comparison-qualified-host")
		}
		for _, field := range []string{"appSHA256", "engineSHA256", "adapterSHA256"} {
			if !validSHA(host[field]) {
				return errors.New("comparison-qualified-host")
			}
		}
		support := receipt["support"]
		if support == "FULL" {
			degradations, _ := receipt["degradations"].([]any)
			if receipt["qualification"] != "QUALIFIED" || len(degradations) != 0 || !validSHA(host["digest"]) || !validSHA(host["evidenceSHA256"]) {
				return errors.New("comparison-qualified-support")
			}
		} else if support == "FALLBACK" {
			degradations, _ := receipt["degradations"].([]any)
			if receipt["qualification"] != "UNQUALIFIED" || len(degradations) != 1 || degradations[0] != "native-tuple-unqualified" || host["digest"] != "" || host["evidenceSHA256"] != "" {
				return errors.New("comparison-qualified-support")
			}
		} else {
			return errors.New("comparison-qualified-support")
		}
		if !sameJSON(receipt["completion"], map[string]any{"decision": "release", "reason": "not-stop-event"}) || !sameJSON(receipt["decision"], receipt["completion"]) || !sameJSON(receipt["frontier"], map[string]any{"state": "NOT_EVALUATED", "universeSHA256": "", "decision": "release", "reason": "not-stop-event"}) {
			return errors.New("comparison-qualified-authority")
		}
	}
	policy, err := asObject(receipt["policy"], "comparison-policy")
	if err != nil || exactFields(policy, "lifecycle", "satisfied", "unmet", "base", "target", "planDigest", "reportSetDigest") != nil {
		return errors.New("comparison-policy")
	}
	if _, ok := policy["satisfied"].(bool); !ok {
		return errors.New("comparison-policy")
	}
	unmet, err := asArray(policy["unmet"], "comparison-policy")
	if err != nil || len(unmet) > 256 {
		return errors.New("comparison-policy")
	}
	allowedUnmet := stringSet("worktree-owner-mismatch", "uncommitted-work", "selected-check-unverified", "reports-not-produced", "report-set-stale", "review-required", "final-check-required", "policy-condition-unavailable")
	seen := map[string]bool{}
	for _, raw := range unmet {
		text, ok := raw.(string)
		if !ok || !allowedUnmet[text] || seen[text] {
			return errors.New("comparison-policy")
		}
		seen[text] = true
	}
	return nil
}

func validRepository(value map[string]any) error {
	if exactFields(value, "commitRevision", "treeRevision", "objectFormat", "worktreeState", "dirtyPathCount", "dirtyPathsSha256") != nil {
		return errors.New("comparison-repository")
	}
	if value["objectFormat"] != "sha1" && value["objectFormat"] != "sha256" || !validSHA(value["dirtyPathsSha256"]) {
		return errors.New("comparison-repository")
	}
	width := 40
	if value["objectFormat"] == "sha256" {
		width = 64
	}
	for _, field := range []string{"commitRevision", "treeRevision"} {
		text, ok := value[field].(string)
		if !ok || len(text) != width {
			return errors.New("comparison-repository")
		}
	}
	if _, _, err := integer(value["dirtyPathCount"], false); err != nil {
		return errors.New("comparison-repository")
	}
	return nil
}
func mustDigest(value any) string { digest, _ := digestValue(value); return digest }
func arrayLength(value any) int   { array, _ := value.([]any); return len(array) }
func sameJSON(left, right any) bool {
	a, _ := canonical(left)
	b, _ := canonical(right)
	return bytes.Equal(a, b)
}
func equalInteger(left, right any) bool {
	leftValue, _, leftErr := integer(left, false)
	rightValue, _, rightErr := integer(right, false)
	return leftErr == nil && rightErr == nil && leftValue == rightValue
}

func (c *comparison) observeActivity(value map[string]any) (map[string]any, error) {
	c.sequence++
	return c.continuation.observe(value, c.sequence)
}
func (c *comparison) finish() (map[string]any, error) {
	if len(c.started) > 0 || c.completed != len(c.cases) {
		return nil, errors.New("comparison-incomplete")
	}
	if c.continuation != nil {
		return c.continuation.finish()
	}
	return nil, nil
}
