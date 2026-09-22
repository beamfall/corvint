// Package gokernel contains the experimental dependency-free Corvint production
// kernel. The initial slice intentionally implements only non-index lifecycle
// events; unsupported events fail closed instead of delegating to Python.
package gokernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/observations"
)

const (
	Profile        = "corvint-harness-event/0"
	ReceiptPrefix  = "harness-receipt:sha256:"
	MaxInputBytes  = 131_072
	MinOutputBytes = 4_096
	MaxOutputBytes = 1_000_000
	// OutputOverheadBytes is reserved for the receipt envelope around the context
	// block, matching the oracle's OUTPUT_OVERHEAD_BYTES.
	OutputOverheadBytes = 2_048
	// MaxQueryCharacters and MaxTaskBytes bound the user-prompt task, matching the
	// oracle's MAX_QUERY_CHARS and MAX_TASK_BYTES.
	MaxQueryCharacters = 2_000
	MaxTaskBytes       = 16_384
	maxItems           = 256
	maxPathCharacters  = 1_024
	maxValueCharacters = 512
)

var (
	tokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+\-]{0,127}$`)
	hex64Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// IndexedContext supplies the context block for the two events that require a
// built repository index. It is injected rather than imported so this package
// stays dependency-free; when it is nil those events fail closed exactly as
// before.
type IndexedContext func(ctx context.Context, root, event string, normalized map[string]any, budgetBytes int) (map[string]any, error)

type EventRequest struct {
	Root                 string
	Host                 string
	HostVersion          string
	Surface              string
	AdapterVersion       string
	Event                string
	Input                []byte
	BudgetBytes          int
	IndexedContext       IndexedContext
	SharedIndexedContext SharedIndexedContext
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func token(value, label string) (string, error) {
	if !tokenPattern.MatchString(value) {
		return "", newError("invalid-harness-adapter", "invalid "+label)
	}
	return value, nil
}

func validPath(value any) (string, error) {
	path, ok := value.(string)
	if !ok || path == "" || utf8.RuneCountInString(path) > maxPathCharacters || path == "." || strings.HasPrefix(path, "/") {
		return "", newError("invalid-harness-input", "invalid repository-relative path")
	}
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", newError("invalid-harness-input", "path must be normalized and repository-relative")
		}
	}
	return path, nil
}

func stringList(value any, label string, paths bool) ([]string, error) {
	array, ok := value.([]any)
	if !ok || len(array) > maxItems {
		return nil, newError("invalid-harness-input", label+" must be a bounded list")
	}
	result := make([]string, 0, len(array))
	seen := make(map[string]struct{}, len(array))
	for _, item := range array {
		var rendered string
		var err error
		if paths {
			rendered, err = validPath(item)
		} else {
			rendered, ok = item.(string)
			if !ok || rendered == "" || utf8.RuneCountInString(rendered) > maxValueCharacters {
				err = newError("invalid-harness-input", label+" contains an invalid value")
			}
		}
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[rendered]; duplicate {
			return nil, newError("invalid-harness-input", label+" contains duplicates")
		}
		seen[rendered] = struct{}{}
		result = append(result, rendered)
	}
	return result, nil
}

func optionalArray(input map[string]any, key string) any {
	if value, exists := input[key]; exists {
		return value
	}
	return []any{}
}

func verification(value any) ([]map[string]any, error) {
	array, ok := value.([]any)
	if !ok || len(array) > maxItems {
		return nil, newError("invalid-harness-input", "verification must be a bounded list")
	}
	result := make([]map[string]any, 0, len(array))
	seen := make(map[string]struct{}, len(array))
	for _, raw := range array {
		item, ok := raw.(map[string]any)
		if !ok || len(item) != 2 {
			return nil, newError("invalid-harness-input", "invalid verification observation")
		}
		digest, digestOK := item["commandSha256"].(string)
		status, statusOK := item["status"].(string)
		if _, exists := item["commandSha256"]; !exists {
			return nil, newError("invalid-harness-input", "invalid verification observation")
		}
		if _, exists := item["status"]; !exists {
			return nil, newError("invalid-harness-input", "invalid verification observation")
		}
		if !digestOK || !hex64Pattern.MatchString(digest) {
			return nil, newError("invalid-harness-input", "invalid verification command digest")
		}
		if !statusOK || (status != "failed" && status != "not-run" && status != "passed" && status != "unknown") {
			return nil, newError("invalid-harness-input", "invalid verification status")
		}
		identity := digest + "\x00" + status
		if _, duplicate := seen[identity]; duplicate {
			return nil, newError("invalid-harness-input", "verification contains duplicates")
		}
		seen[identity] = struct{}{}
		result = append(result, map[string]any{"commandSha256": digest, "status": status})
	}
	return result, nil
}

func allowedFields(input map[string]any, fields ...string) bool {
	allowed := map[string]struct{}{"sessionIdSha256": {}}
	for _, field := range fields {
		allowed[field] = struct{}{}
	}
	for field := range input {
		if _, ok := allowed[field]; !ok {
			return false
		}
	}
	return true
}

func validateInput(event string, input map[string]any) (map[string]any, error) {
	var fields []string
	switch event {
	case "session-start":
		fields = []string{"startSource"}
	case "user-prompt":
		fields = []string{"task"}
	case "file-change":
		fields = []string{"paths"}
	case "post-tool":
		fields = []string{"observedEvidenceHandles", "changedPaths", "verification"}
	case "stop":
		fields = []string{"stopHookActive", "changedPaths"}
	case "session-end":
		fields = []string{"outcome", "taskSha256", "openedPaths", "changedPaths", "verification"}
	default:
		return nil, newError("unsupported-harness-event", "event is not implemented by the Go kernel")
	}
	if !allowedFields(input, fields...) {
		return nil, newError("invalid-harness-input", "harness input contains unsupported fields")
	}
	normalized := make(map[string]any)
	if session, exists := input["sessionIdSha256"]; exists {
		if session == nil {
			delete(input, "sessionIdSha256")
		} else {
			digest, ok := session.(string)
			if !ok || !hex64Pattern.MatchString(digest) {
				return nil, newError("invalid-harness-input", "invalid session identity digest")
			}
			normalized["sessionIdSha256"] = digest
		}
	}
	switch event {
	case "session-start":
		if sourceRaw, exists := input["startSource"]; exists {
			if sourceRaw == nil {
				break
			}
			source, ok := sourceRaw.(string)
			if !ok || (source != "clear" && source != "compact" && source != "resume" && source != "startup") {
				return nil, newError("invalid-harness-input", "invalid session start source")
			}
			normalized["startSource"] = source
		}
	case "user-prompt":
		task, ok := input["task"].(string)
		trimmed := strings.TrimSpace(task)
		if !ok || trimmed == "" || utf8.RuneCountInString(task) > MaxQueryCharacters || len(task) > MaxTaskBytes {
			return nil, newError("invalid-harness-input", "task exceeds the bounded query contract")
		}
		normalized["task"] = trimmed
	case "file-change":
		paths, pathsErr := stringList(input["paths"], "paths", true)
		if pathsErr != nil {
			return nil, pathsErr
		}
		if len(paths) == 0 {
			return nil, newError("invalid-harness-input", "paths must not be empty")
		}
		normalized["paths"] = stringsToAny(paths)
	case "post-tool":
		handles, err := stringList(optionalArray(input, "observedEvidenceHandles"), "observedEvidenceHandles", false)
		if err != nil {
			return nil, err
		}
		changed, err := stringList(optionalArray(input, "changedPaths"), "changedPaths", true)
		if err != nil {
			return nil, err
		}
		verified, err := verification(optionalArray(input, "verification"))
		if err != nil {
			return nil, err
		}
		normalized["observedEvidenceHandles"] = stringsToAny(handles)
		normalized["changedPaths"] = stringsToAny(changed)
		normalized["verification"] = mapsToAny(verified)
	case "stop":
		active := false
		if value, exists := input["stopHookActive"]; exists {
			var ok bool
			active, ok = value.(bool)
			if !ok {
				return nil, newError("invalid-harness-input", "stopHookActive must be boolean")
			}
		}
		changed, err := stringList(optionalArray(input, "changedPaths"), "changedPaths", true)
		if err != nil {
			return nil, err
		}
		normalized["stopHookActive"] = active
		normalized["changedPaths"] = stringsToAny(changed)
	case "session-end":
		outcome := ""
		if outcomeRaw, exists := input["outcome"]; exists && outcomeRaw != nil {
			var ok bool
			outcome, ok = outcomeRaw.(string)
			if !ok || (outcome != "blocked" && outcome != "failed" && outcome != "passed") {
				return nil, newError("invalid-harness-input", "invalid explicit outcome")
			}
		}
		task := ""
		if taskRaw, exists := input["taskSha256"]; exists && taskRaw != nil {
			var ok bool
			task, ok = taskRaw.(string)
			if !ok || !hex64Pattern.MatchString(task) {
				return nil, newError("invalid-harness-input", "invalid explicit task digest")
			}
		}
		opened, err := stringList(optionalArray(input, "openedPaths"), "openedPaths", true)
		if err != nil {
			return nil, err
		}
		changed, err := stringList(optionalArray(input, "changedPaths"), "changedPaths", true)
		if err != nil {
			return nil, err
		}
		verified, err := verification(optionalArray(input, "verification"))
		if err != nil {
			return nil, err
		}
		normalized["openedPaths"] = stringsToAny(opened)
		normalized["changedPaths"] = stringsToAny(changed)
		normalized["verification"] = mapsToAny(verified)
		if outcome != "" {
			if task == "" || len(changed) == 0 || len(verified) == 0 {
				return nil, newError("invalid-harness-input", "explicit outcome requires taskSha256, changedPaths, and verification")
			}
			normalized["outcome"] = outcome
			normalized["taskSha256"] = task
		}
	}
	return normalized, nil
}

func stringsToAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func mapsToAny(values []map[string]any) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func countSlice(value any) int {
	if values, ok := value.([]any); ok {
		return len(values)
	}
	return 0
}

func HandleEvent(request EventRequest) (map[string]any, error) {
	return HandleEventContext(context.Background(), request)
}

func HandleEventContext(ctx context.Context, request EventRequest) (map[string]any, error) {
	started := time.Now()
	if request.BudgetBytes < MinOutputBytes {
		return nil, newError("invalid-harness-budget", fmt.Sprintf("harness budget must be at least %d bytes", MinOutputBytes))
	}
	if request.BudgetBytes > MaxOutputBytes {
		return nil, newError("invalid-harness-budget", fmt.Sprintf("harness budget must not exceed %d bytes", MaxOutputBytes))
	}
	if !KnownHarnessHost(request.Host) {
		return nil, newError("unsupported-harness-host", "unsupported harness host")
	}
	if _, ok := supportedEventSet[request.Event]; !ok {
		return nil, newError("unsupported-harness-event", "unsupported harness event")
	}
	if _, err := token(request.HostVersion, "host version"); err != nil {
		return nil, err
	}
	if _, err := token(request.Surface, "surface"); err != nil {
		return nil, err
	}
	if _, err := token(request.AdapterVersion, "adapter version"); err != nil {
		return nil, err
	}
	if len(request.Input) > MaxInputBytes {
		return nil, newError("harness-input-too-large", "harness input exceeds its byte limit")
	}
	input, err := decodeStrictJSON(request.Input)
	if err != nil {
		return nil, err
	}
	normalized, err := validateInput(request.Event, input)
	if err != nil {
		return nil, err
	}
	if requiresIndex(request.Event, normalized) && request.IndexedContext == nil {
		return nil, newError("unsupported-harness-event", "event is not implemented by the Go kernel")
	}
	root, err := filepath.Abs(request.Root)
	if err != nil {
		return nil, wrapError("invalid-repository-root", "cannot resolve repository root", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	adapter := map[string]any{
		"adapterVersion": request.AdapterVersion,
		"host":           request.Host,
		"hostVersion":    request.HostVersion,
		"surface":        request.Surface,
	}
	// The GPK-V0-007 bracket and the context block: two concurrent observations
	// by default, or one shared bracket whose read stage is the event's own read
	// under SharedIndexedContext (proposed GPK-V0-058). Both paths report the
	// bracket's failure before the block's.
	repository, eventContext, err := observeAndBuildContext(ctx, request, normalized, root)
	if err != nil {
		return nil, err
	}
	repositoryWire := repository.wire()
	degradations := []any{"frontier-authority-unavailable"}
	degradations = append(degradations, CompactionDegradations(request.Event, eventContext)...)
	if request.Event == "session-end" {
		if _, hasOutcome := normalized["outcome"]; hasOutcome {
			degradations = append(degradations, "outcome-persistence-unavailable")
		}
	}
	if request.HostVersion == "unknown" {
		degradations = append(degradations, "host-version-unknown")
	}
	// file-change carries its paths under "paths"; the oracle counts that key when
	// "changedPaths" is absent.
	changedPaths := normalized["changedPaths"]
	if changedPaths == nil {
		changedPaths = normalized["paths"]
	}
	inputSummary := map[string]any{
		"changedPathCount":      countSlice(changedPaths),
		"observedEvidenceCount": countSlice(normalized["observedEvidenceHandles"]),
		"sessionBound":          normalized["sessionIdSha256"] != nil,
		"verificationCount":     countSlice(normalized["verification"]),
	}
	if source, exists := normalized["startSource"]; exists {
		inputSummary["startSource"] = source
	}
	response := map[string]any{
		"adapter":      adapter,
		"degradations": degradations,
		"event":        request.Event,
		"input":        inputSummary,
		"mutates":      false, // repository/trace state; the SOL-V0-001 ledger row is exempt (SOL-V0-003).
		"ok":           true,
		"profile":      Profile,
		"repository":   repositoryWire,
		"support":      "FALLBACK",
	}
	if eventContext != nil {
		response["context"] = eventContext
	}
	if request.Event == "stop" {
		response["frontier"] = map[string]any{
			"reason":         "frontier-authority-unavailable",
			"shouldContinue": false,
			"state":          "UNAVAILABLE",
		}
	}
	// The receipt basis covers the REQUEST, not the answer: adapter, event, normalized input, and
	// repository state. It deliberately excludes "context", "degradations", "support", and
	// "frontier", so two responses that agree on all four basis fields share a receiptId even when
	// their answers differ. A receiptId is therefore a request identity, NOT an integrity digest
	// over what this call returned, and must not be cited as one. Widening the basis to cover the
	// answer would change every emitted receiptId, so it is a profile flag day
	// (corvint-harness-event/1) across all four adapters; deferred by decision 0007 (D8, ruling B),
	// which ratified documenting the limit now and bumping the profile later.
	basis := map[string]any{
		"adapter":    adapter,
		"event":      request.Event,
		"input":      normalized,
		"repository": repositoryWire,
	}
	basisJSON, err := CanonicalJSON(basis)
	if err != nil {
		return nil, wrapError("canonical-json-failed", "cannot encode receipt basis", err)
	}
	response["receiptId"] = ReceiptPrefix + sha256Hex(basisJSON)
	encoded, err := CanonicalJSON(response)
	if err != nil {
		return nil, wrapError("canonical-json-failed", "cannot encode harness response", err)
	}
	if len(encoded) > request.BudgetBytes {
		return nil, newError("harness-output-too-large", "harness response exceeds its byte budget")
	}
	if request.Event == "session-start" || request.Event == "user-prompt" || request.Event == "file-change" {
		// Append enforces the SOL-V0-002 no-prose writer contract. Observations
		// remain advisory: that typed refusal or any storage failure must never
		// affect a routed session or alter its receipt.
		_ = observations.Append(root, observationEvent(request.Event, normalized, response, eventContext, time.Since(started)))
	}
	return response, nil
}

func observationEvent(event string, input, response, context map[string]any, elapsed time.Duration) observations.Event {
	result := observations.Event{Kind: "event", Event: event, ReceiptID: stringValue(response["receiptId"]), Support: stringValue(response["support"]), Degradations: stringsFromAny(response["degradations"]), LatencyMS: elapsed.Milliseconds()}
	result.SessionID = stringValue(input["sessionIdSha256"])
	if task := stringValue(input["task"]); task != "" {
		result.TaskSHA256 = observations.TaskHash(task)
	}
	if freshness, ok := context["freshness"].(map[string]any); ok {
		result.Freshness = stringValue(freshness["state"])
	}
	if coverage, ok := context["coverage"].(map[string]any); ok {
		result.UncertaintyCount = countSlice(coverage["uncertainty"])
		result.OmittedCount = integerValue(coverage["omitted_results"])
		result.CriticalMissing = countSlice(coverage["critical_missing"])
		result.Authoritative = integerValue(coverage["authoritative_results"])
	}
	if event == "file-change" {
		result.TouchedPaths = stringsFromAny(input["paths"])
		if result.Freshness == "mixed-worktree" {
			result.MissState = "MISS_DETECTION_NOT_OBSERVED"
		} else {
			result.MissState, result.RankedPaths = "OBSERVED", rankedPaths(context)
		}
	}
	return result
}

func stringsFromAny(value any) []string {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
func stringValue(value any) string { text, _ := value.(string); return text }
func integerValue(value any) int   { integer, _ := value.(int); return integer }
func rankedPaths(context map[string]any) []string {
	rows, ok := context["results"].([]any)
	if !ok {
		return nil
	}
	paths := make([]string, 0)
	for _, row := range rows {
		item, ok := row.(map[string]any)
		if !ok {
			continue
		}
		evidence, ok := item["evidence"].([]any)
		if !ok {
			continue
		}
		for _, raw := range evidence {
			entry, ok := raw.(map[string]any)
			if ok {
				if path := stringValue(entry["path"]); path != "" {
					paths = append(paths, path)
				}
			}
		}
	}
	return paths
}

func SupportedEvents() []string {
	events := make([]string, 0, len(supportedEventSet))
	for event := range supportedEventSet {
		events = append(events, event)
	}
	sort.Strings(events)
	return events
}

var supportedEventSet = map[string]struct{}{
	"file-change": {}, "post-tool": {}, "session-end": {},
	"session-start": {}, "stop": {}, "user-prompt": {},
}

// requiresIndex names the events whose context block can only be built from a
// repository index, so the caller must have injected a provider for them.
func requiresIndex(event string, normalized map[string]any) bool {
	if event == "user-prompt" || event == "file-change" {
		return true
	}
	return event == "session-start" && normalized["startSource"] == "compact"
}

// observeAndBuildContext runs the GPK-V0-007 bracket and the event's context
// block. By default the two are independent concurrent observations; with a
// SharedIndexedContext every event but user-prompt runs one bracket whose read
// stage is the event's own read: the index-building events' snapshot read, or
// nothing for an event that reads no index. The profile is read only for the
// startup session-start block, the one place it is emitted. The bracket's
// error keeps precedence over the context block's on both paths.
func observeAndBuildContext(ctx context.Context, request EventRequest, normalized map[string]any, root string) (Repository, map[string]any, error) {
	if request.SharedIndexedContext == nil || request.Event == "user-prompt" {
		probe := probeRepositoryConcurrently(ctx, root)
		eventContext, contextErr := eventContextBlock(ctx, request, normalized, root, probe)
		repository, err := probe.result()
		if err != nil {
			return Repository{}, nil, err
		}
		return repository, eventContext, contextErr
	}
	indexed := requiresIndex(request.Event, normalized)
	var eventContext map[string]any
	read := func(ctx context.Context, observation Observation) (func() error, error) {
		if !indexed {
			return nil, nil
		}
		block, err := request.SharedIndexedContext(ctx, root, request.Event, normalized, request.BudgetBytes-OutputOverheadBytes, observation)
		if err != nil {
			return nil, err
		}
		return func() error {
			computed, err := block()
			eventContext = computed
			return err
		}, nil
	}
	wantProfile := request.Event == "session-start" && !indexed
	repository, err := probeRepositorySharing(ctx, root, git, read, wantProfile)
	if err != nil {
		return Repository{}, nil, err
	}
	if !indexed {
		eventContext = plainEventContextBlock(request.Event, normalized, repository)
	}
	return repository, eventContext, nil
}

// eventContextBlock builds the per-event context block, or nil when the event
// carries none. Index-backed events are delegated to the injected provider; the
// rest are derived from the normalized input alone.
func eventContextBlock(ctx context.Context, request EventRequest, normalized map[string]any, root string, probe *repositoryProbe) (map[string]any, error) {
	if requiresIndex(request.Event, normalized) {
		return request.IndexedContext(ctx, root, request.Event, normalized, request.BudgetBytes-OutputOverheadBytes)
	}
	repository, err := probe.result()
	if err != nil {
		return nil, err
	}
	return plainEventContextBlock(request.Event, normalized, repository), nil
}

// plainEventContextBlock is the block of an event that reads no index,
// derived from the normalized input and the probe's repository alone.
func plainEventContextBlock(event string, normalized map[string]any, repository Repository) map[string]any {
	switch event {
	case "session-start":
		return map[string]any{
			"availableOperations": []any{"file-change", "user-prompt"},
			"profile":             repository.ProfileID,
			"state":               "READY",
		}
	case "post-tool":
		return map[string]any{
			"changedPathCount":      countSlice(normalized["changedPaths"]),
			"observedEvidenceCount": countSlice(normalized["observedEvidenceHandles"]),
			"verificationCount":     countSlice(normalized["verification"]),
		}
	case "session-end":
		if _, hasOutcome := normalized["outcome"]; hasOutcome {
			return map[string]any{
				"changedPathCount":  countSlice(normalized["changedPaths"]),
				"openedPathCount":   countSlice(normalized["openedPaths"]),
				"outcome":           normalized["outcome"],
				"outcomeObserved":   true,
				"taskSha256":        normalized["taskSha256"],
				"verificationCount": countSlice(normalized["verification"]),
			}
		}
	}
	return nil
}

// positiveCount reports whether a wire count field carries a value above zero,
// accepting either the int the provider writes or the float a decoded receipt
// would carry.
func positiveCount(value any) bool {
	switch typed := value.(type) {
	case int:
		return typed > 0
	case float64:
		return typed > 0
	}
	return false
}

// CompactionDegradations reads back the compaction rehydration block. The oracle
// raises each flag inside the branch that shaped that block, so the block itself
// is the evidence for which degradations belong on the receipt.
func CompactionDegradations(event string, block map[string]any) []any {
	if event != "session-start" || block == nil {
		return nil
	}
	rehydration, ok := block["rehydration"].(map[string]any)
	if !ok {
		return nil
	}
	var degradations []any
	if block["state"] == "REHYDRATION_BOUNDED" {
		degradations = append(degradations, "compaction-dirty-set-over-budget")
	}
	if positiveCount(rehydration["untrackedDirtyPathCount"]) {
		degradations = append(degradations, "compaction-untracked-paths-not-rehydratable")
	}
	if block["state"] == "CRITICAL_EVIDENCE_OVERFLOW" {
		degradations = append(degradations, "compaction-critical-evidence-overflow")
	}
	return degradations
}

// SharedIndexedContext is IndexedContext placed inside the GPK-V0-007 bracket
// (proposed GPK-V0-058): it consumes the bracket's opening observation instead
// of reading identity and status itself, performs the event's reads, and
// returns the IndexedBlock that computes the context block from what it read.
// When nil, the default concurrent path runs unchanged. It serves file-change
// and compact session-start; user-prompt keeps the default path.
type SharedIndexedContext func(ctx context.Context, root, event string, normalized map[string]any, budgetBytes int, observation Observation) (IndexedBlock, error)

// IndexedBlock finishes an index-backed event from an index already in memory.
// It reads no repository state, so the bracket runs it beside its closing
// observation rather than inside the read stage.
type IndexedBlock func() (map[string]any, error)
