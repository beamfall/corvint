package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/localcompletion"
)

const dogfoodEventProfile = "corvint-dogfood-event/0"
const dogfoodEventDigestPrefix = "dogfood-event:sha256:"
const dogfoodEventMaxBytes = 8000
const dogfoodEventDeadline = 1600 * time.Millisecond

// dogfoodSessionEndDeadline is Claude Code's session-end query deadline only.
// integrations/claude-code/plugins/corvint/hooks/hooks.json declares a 1-second
// SessionEnd kill (every other declared event/host pair gets 2 seconds), so
// dogfoodEventDeadline's usual 1600ms plus cleanup would run past the kill before
// Corvint can emit its degradation JSON. The other events keep 1600ms deadline +
// ~300ms cleanup = 1900ms, 100ms under their 2s kill; 600ms + ~300ms cleanup =
// 900ms keeps the same 100ms margin under session-end's 1s kill.
// dogfoodEventWithin returns at the deadline without waiting for a read stage that
// ignores cancellation; the AHI-017 watchdog in host_adapter.go remains the backstop
// that keeps the adapter's output ahead of the kill.
const dogfoodSessionEndDeadline = 600 * time.Millisecond

var dogfoodSessionPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func dogfoodEventError(code string) error {
	return &gokernel.Error{Code: code, Message: code}
}

// dogfoodEventDeadlineFor returns the query deadline for one host/event pair. It
// must stay strictly below that pair's declared host kill; see
// dogfoodSessionEndDeadline for the one exception the admitted hosts require.
func dogfoodEventDeadlineFor(host, event string) time.Duration {
	if host == "claude-code" && event == "session-end" {
		return dogfoodSessionEndDeadline
	}
	return dogfoodEventDeadline
}

// dogfoodEventDeadlineKey carries a per-invocation replacement for dogfoodEventDeadlineFor.
// Tests that verify lifecycle semantics rather than latency set it (decision 0082).
type dogfoodEventDeadlineKey struct{}

// dogfoodEventReadKey carries a per-invocation replacement for dogfoodEvent, the repository read
// runLocalCompletionEvent bounds. Tests set it to a stage that ignores cancellation; a context value
// rather than a package variable keeps that stage out of concurrently running tests.
type dogfoodEventReadKey struct{}

type dogfoodEventReader = func(context.Context, options, map[string]any) (map[string]any, error)

// dogfoodEventMissKey carries the flag localEventContext raises when no index snapshot matches
// the tree and the read falls back to its in-memory build, so an event whose deadline then
// expires names the stale snapshot as its cause (AHI-031).
type dogfoodEventMissKey struct{}

// dogfoodEventBuildKey carries a per-invocation replacement for the in-memory index build a
// snapshot miss runs. Tests set it to a build that ignores cancellation, as the real one does.
type dogfoodEventBuildKey struct{}

// dogfoodExpiryCode is the code an expired event reports: the stale snapshot when the read had
// fallen back to the in-memory build, else the bare time bound (LCP-V0-008, AHI-031).
func dogfoodExpiryCode(missed *atomic.Bool) string {
	if missed.Load() {
		return "dogfood-event-index-snapshot-stale"
	}
	return "dogfood-event-deadline"
}

// dogfoodEventWithin returns the read's outcome, or the context's error as soon as the
// context ends first (LCP-V0-008). The in-memory index compile a snapshot miss runs does not
// observe cancellation; on a loaded host it was measured holding an expired event for seconds
// (2026-09-12). The abandoned read writes nothing and ends with its process.
func dogfoodEventWithin(ctx context.Context, options options, input map[string]any) (map[string]any, error) {
	type outcome struct {
		result map[string]any
		err    error
	}
	read, ok := ctx.Value(dogfoodEventReadKey{}).(dogfoodEventReader)
	if !ok {
		read = dogfoodEvent
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := read(ctx, options, input)
		done <- outcome{result, err}
	}()
	select {
	case finished := <-done:
		return finished.result, finished.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// The automatic surface owns neither enrollment nor observations. In particular it
// does not call HandleEventContext, whose legacy self-observation ledger is mutable.
func runLocalCompletionEvent(parent context.Context, root string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if !dogfoodUniqueOptions(args) {
		emitError(stderr, dogfoodEventError("invalid-dogfood-event-arguments"))
		return 2
	}
	options, err := parse(append([]string{"--root", root, "harness", "event"}, args...))
	if err != nil {
		emitError(stderr, dogfoodEventError("invalid-dogfood-event-arguments"))
		return 2
	}
	if hostVersion, admitted := dogfoodHostVersions[options.host]; !admitted || options.surface != "plugin" || options.adapterVersion != "0.1.0" || options.hostVersion != hostVersion {
		emitError(stderr, dogfoodEventError("unsupported-dogfood-event-host"))
		return 2
	}
	if options.budgetBytes > dogfoodEventMaxBytes {
		emitError(stderr, dogfoodEventError("invalid-dogfood-event-budget"))
		return 2
	}
	ctx, cancel := context.WithTimeout(parent, dogfoodEventDeadlineOf(parent, options.host, options.event))
	defer cancel()
	missed := new(atomic.Bool)
	ctx = context.WithValue(ctx, dogfoodEventMissKey{}, missed)
	raw, err := readInput(ctx, stdin)
	if err != nil {
		emitError(stderr, dogfoodEventError("dogfood-event-input-unavailable"))
		return 2
	}
	input, err := dogfoodEventInput(options.event, raw)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	result, err := dogfoodEventWithin(ctx, options, input)
	if err != nil && (errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, errDogfoodSnapshotStale)) {
		// An expired deadline surfaces in later reads as unrelated drift or unavailability, and a
		// skipped miss build (AHI-031) forecasts one; report the time bound, not a diagnosed fault.
		emitError(stderr, dogfoodEventError(dogfoodExpiryCode(missed)))
		return 2
	}
	if err != nil {
		// Underlying repository/artifact diagnostics can contain caller data. The
		// native envelope uses a fixed error; explicit status retains its worklist.
		emitError(stderr, dogfoodEventError("dogfood-event-unavailable"))
		return 2
	}
	encoded, err := dogfoodEventBytes(result, options.budgetBytes)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if ctx.Err() != nil {
		emitError(stderr, dogfoodEventError(dogfoodExpiryCode(missed)))
		return 2
	}
	if _, err := stdout.Write(encoded); err != nil {
		emitError(stderr, dogfoodEventError("dogfood-event-output-unavailable"))
		return 2
	}
	return 0
}

func dogfoodUniqueOptions(args []string) bool {
	seen := map[string]bool{}
	for index := 0; index < len(args); index++ {
		name, _, inline := strings.Cut(args[index], "=")
		if seen[name] {
			return false
		}
		seen[name] = true
		if !inline {
			index++
		}
	}
	return true
}

func dogfoodEventInput(event string, raw []byte) (map[string]any, error) {
	if len(raw) > gokernel.MaxInputBytes || !utf8.Valid(raw) {
		return nil, dogfoodEventError("invalid-dogfood-event-input")
	}
	fields := map[string]string{"sessionIdSha256": "session"}
	switch event {
	case "session-start":
		fields["startSource"] = "source"
	case "user-prompt":
		fields["task"] = "task"
	case "stop":
		fields["stopHookActive"], fields["changedPaths"] = "bool", "empty"
	case "session-end":
		fields["openedPaths"], fields["changedPaths"], fields["verification"] = "empty", "empty", "empty"
	default:
		return nil, dogfoodEventError("unsupported-dogfood-event")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, dogfoodEventError("invalid-dogfood-event-input")
	}
	result := map[string]any{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		key, ok := keyToken.(string)
		if err != nil || !ok || fields[key] == "" {
			return nil, dogfoodEventError("invalid-dogfood-event-input")
		}
		if _, duplicate := result[key]; duplicate {
			return nil, dogfoodEventError("invalid-dogfood-event-input")
		}
		var value any
		if err := decoder.Decode(&value); err != nil || !dogfoodEventField(fields[key], value) {
			return nil, dogfoodEventError("invalid-dogfood-event-input")
		}
		result[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, dogfoodEventError("invalid-dogfood-event-input")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, dogfoodEventError("invalid-dogfood-event-input")
	}
	if event == "user-prompt" && result["task"] == nil {
		return nil, dogfoodEventError("invalid-dogfood-event-input")
	}
	return result, nil
}

func dogfoodEventField(kind string, value any) bool {
	text, isString := value.(string)
	switch kind {
	case "session":
		return isString && dogfoodSessionPattern.MatchString(text)
	case "source":
		return isString && (text == "startup" || text == "resume" || text == "clear" || text == "compact")
	case "task":
		return isString && strings.TrimSpace(text) != "" && len(text) <= gokernel.MaxTaskBytes && utf8.RuneCountInString(text) <= gokernel.MaxQueryCharacters
	case "bool":
		_, ok := value.(bool)
		return ok
	case "empty":
		array, ok := value.([]any)
		return ok && len(array) == 0
	}
	return false
}

func dogfoodEvent(ctx context.Context, options options, input map[string]any) (map[string]any, error) {
	return localEventRead(ctx, options, input, dogfoodEventEnvelope, dogfoodEventContext, "")
}

func localEventRead(ctx context.Context, options options, input map[string]any,
	envelope func(options, map[string]any, gokernel.Repository, localcompletion.Evaluation) map[string]any,
	contextPacket func(context.Context, options, map[string]any, localcompletion.Evaluation, map[string]any, gokernel.Repository) (map[string]any, error),
	expectedTarget string,
) (map[string]any, error) {
	before, err := gokernel.ProbeRepositoryContext(ctx, options.root)
	if err != nil {
		return nil, err
	}
	// Only the held protected campaign scope supplies this guard. Reuse these
	// common probes: no candidate-only Git work can prewarm the context path.
	if expectedTarget != "" && before.CommitRevision != expectedTarget {
		return nil, dogfoodEventError("qualified-lifecycle-target-drift")
	}
	evaluation := localcompletion.Evaluation{Lifecycle: "inactive", Unmet: []string{}}
	if session, ok := input["sessionIdSha256"].(string); ok {
		evaluation, err = localcompletion.Evaluate(ctx, options.root, session)
		if err != nil {
			return nil, err
		}
	}
	result := envelope(options, input, before, evaluation)
	if options.event == "user-prompt" || options.event == "session-start" {
		packet, err := contextPacket(ctx, options, input, evaluation, result, before)
		if err != nil {
			return nil, err
		}
		result["context"] = packet
	}
	after, err := gokernel.ProbeRepositoryContext(ctx, options.root)
	if err != nil {
		return nil, err
	}
	if before != after || (expectedTarget != "" && after.CommitRevision != expectedTarget) {
		return nil, dogfoodEventError("dogfood-event-repository-drift")
	}
	if evaluation.Target != "" && evaluation.Target != before.CommitRevision {
		return nil, dogfoodEventError("dogfood-event-policy-drift")
	}
	// Re-read private policy inputs too: Git status cannot detect their mutation.
	if session, ok := input["sessionIdSha256"].(string); ok {
		current, err := localcompletion.Evaluate(ctx, options.root, session)
		if err != nil {
			// A failed re-read observed no drift; report the read failure.
			return nil, err
		}
		if !reflect.DeepEqual(evaluation, current) {
			return nil, dogfoodEventError("dogfood-event-policy-drift")
		}
	}
	return result, nil
}

func dogfoodEventEnvelope(options options, input map[string]any, repo gokernel.Repository, evaluation localcompletion.Evaluation) map[string]any {
	requestBytes, _ := gokernel.CanonicalJSON(input)
	unmet := dogfoodUnmet(evaluation.Unmet)
	return map[string]any{
		"profile": dogfoodEventProfile, "ok": true, "mutates": false, "support": "FALLBACK",
		"event":         options.event,
		"adapter":       map[string]any{"host": options.host, "hostVersion": options.hostVersion, "surface": options.surface, "adapterVersion": options.adapterVersion},
		"repository":    map[string]any{"commitRevision": repo.CommitRevision, "treeRevision": repo.TreeRevision, "objectFormat": repo.ObjectFormat, "worktreeState": repo.WorktreeState, "dirtyPathCount": repo.DirtyPathCount, "dirtyPathsSha256": repo.DirtyPathsSHA},
		"requestSha256": dogfoodSHA(requestBytes),
		"degradations":  dogfoodDegradations(options.hostVersion),
		"frontier":      map[string]any{"state": "UNAVAILABLE", "shouldContinue": false, "reason": "frontier-authority-unavailable"},
		"policy":        map[string]any{"lifecycle": evaluation.Lifecycle, "satisfied": evaluation.Satisfied, "unmet": unmet, "base": evaluation.Base, "target": evaluation.Target, "planDigest": evaluation.PlanDigest, "reportSetDigest": evaluation.ReportSetDigest},
		"completion":    dogfoodCompletion(options.event, input, evaluation),
	}
}

// Check IDs are caller declarations; the automatic surface reports only fixed
// diagnostic categories. Explicit status retains the per-check worklist.
func dogfoodUnmet(unmet []string) []string {
	unique := map[string]bool{}
	for _, reason := range unmet {
		code, _, _ := strings.Cut(reason, ":")
		switch code {
		case "worktree-owner-mismatch", "uncommitted-work", "selected-check-unverified",
			"reports-not-produced", "report-set-stale", "review-required", "final-check-required":
			unique[code] = true
		default:
			unique["policy-condition-unavailable"] = true
		}
	}
	result := make([]string, 0, len(unique))
	for code := range unique {
		result = append(result, code)
	}
	sort.Strings(result)
	return result
}

func dogfoodCompletion(event string, input map[string]any, evaluation localcompletion.Evaluation) map[string]any {
	decision, reason := "release", "local-policy-"+evaluation.Lifecycle
	if event != "stop" {
		reason = "not-stop-event"
	} else if evaluation.Lifecycle == "active" || evaluation.Lifecycle == "satisfied" && !evaluation.Satisfied {
		decision, reason = "block", "local-policy-incomplete"
		if input["stopHookActive"] == true {
			decision, reason = "release", "local-policy-continuation-limit"
		}
	}
	return map[string]any{"decision": decision, "reason": reason}
}

func dogfoodEventContext(ctx context.Context, options options, input map[string]any, evaluation localcompletion.Evaluation, envelope map[string]any, repo gokernel.Repository) (map[string]any, error) {
	return localEventContext(ctx, options, input, evaluation, envelope, repo, true, dogfoodEventBytes, func(encoded []byte) (int, error) {
		quoted, err := gokernel.CanonicalJSON(string(bytes.TrimSuffix(encoded, []byte{'\n'})))
		return len(quoted) + 512, err
	})
}

// localEventContext compiles the prompt packet. With rehydrate, a compact
// session start over a dirty worktree also carries the compaction block
// (AHI-003) from the same index, under the packet's "compaction" key.
func localEventContext(ctx context.Context, options options, input map[string]any, evaluation localcompletion.Evaluation, envelope map[string]any, repo gokernel.Repository,
	rehydrate bool, encode func(map[string]any, int) ([]byte, error), nativeSize func([]byte) (int, error),
) (map[string]any, error) {
	compact := rehydrate && options.event == "session-start" && input["startSource"] == "compact" && repo.DirtyPathCount > 0
	index, hit, err := loadSnapshot(ctx, options.root)
	if missed, ok := ctx.Value(dogfoodEventMissKey{}).(*atomic.Bool); ok && err == nil && !hit {
		missed.Store(true)
	}
	buildContext := contextindex.BuildContext
	if replacement, ok := ctx.Value(dogfoodEventBuildKey{}).(func(context.Context, string, string) (*contextindex.Index, error)); ok {
		buildContext = replacement
	}
	if dogfoodMissOutlastsDeadline(ctx, options.root) {
		return nil, errDogfoodSnapshotStale
	}
	if (err != nil || !hit) && compact {
		// Impact needs the test-relation imports BuildContext omits.
		index, err = contextindex.Build(ctx, options.root)
	} else if err != nil || !hit {
		index, err = buildContext(ctx, options.root, "")
	}
	if err != nil {
		return nil, err
	}
	dirtyBytes, err := gokernel.CanonicalJSON(append([]string{}, index.DirtyPaths...))
	if err != nil {
		return nil, err
	}
	if index.CommitRevision != repo.CommitRevision || index.Revision != repo.TreeRevision || dogfoodSHA(dirtyBytes) != repo.DirtyPathsSHA {
		return nil, dogfoodEventError("dogfood-event-context-drift")
	}
	pointers := make([]contextindex.PinnedIntentPointer, 0, len(evaluation.IntentPointers))
	for _, pointer := range evaluation.IntentPointers {
		pointers = append(pointers, contextindex.PinnedIntentPointer{Path: pointer.Path, Revision: pointer.Revision, BlobHash: pointer.BlobHash})
	}
	var compaction map[string]any
	compactionBytes := 0
	if compact {
		// Rehydration may spend at most half the response; the prompt packet keeps the rest.
		if compaction, err = compactionContext(index, options.budgetBytes/2); err != nil {
			return nil, err
		}
		for _, code := range gokernel.CompactionDegradations(options.event, compaction) {
			envelope["degradations"] = append(envelope["degradations"].([]string), code.(string))
		}
		encodedCompaction, err := gokernel.CanonicalJSON(compaction)
		if err != nil {
			return nil, err
		}
		compactionBytes = len(encodedCompaction) + len(`,"compaction":`)
	}
	// Reserve the exact envelope plus digest, context key and one final LF.
	encoded, err := encode(envelope, options.budgetBytes)
	if err != nil {
		return nil, err
	}
	budget := options.budgetBytes - len(encoded) - len(`,"context":`) - compactionBytes
	task, _ := input["task"].(string)
	for range 8 {
		packet, err := contextindex.DogfoodPromptContext(ctx, index, task, pointers, 10, budget)
		if err != nil {
			return nil, err
		}
		if compaction != nil {
			packet["compaction"] = compaction
		}
		envelope["context"] = packet
		encoded, err := encode(envelope, options.budgetBytes)
		delete(envelope, "context")
		if err != nil {
			return nil, err
		}
		// Native additionalContext escapes the serialized receipt a second time.
		// The profile callback counts its own complete framing and any explicit
		// candidate qualification reserve; legacy bytes retain their old bound.
		nativeBytes, err := nativeSize(encoded)
		if err != nil {
			return nil, err
		}
		excess := nativeBytes - options.budgetBytes
		if excess <= 0 {
			return packet, nil
		}
		budget -= excess + 32
	}
	return nil, dogfoodEventError("dogfood-event-native-budget")
}

func dogfoodSHA(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func dogfoodEventBytes(result map[string]any, budget int) ([]byte, error) {
	copy := make(map[string]any, len(result)+1)
	for key, value := range result {
		if key != "resultDigest" {
			copy[key] = value
		}
	}
	basis, err := gokernel.CanonicalJSON(copy)
	if err != nil {
		return nil, err
	}
	copy["resultDigest"] = dogfoodEventDigestPrefix + dogfoodSHA(append([]byte(dogfoodEventProfile+"\x00"), basis...))
	encoded, err := gokernel.CanonicalJSON(copy)
	if err != nil {
		return nil, err
	}
	if len(encoded)+1 > budget {
		return nil, dogfoodEventError("dogfood-event-output-too-large")
	}
	return append(encoded, '\n'), nil
}

// dogfoodEventDeadlineOf is the deadline runLocalCompletionEvent applies: the
// context's dogfoodEventDeadlineKey replacement if set, else dogfoodEventDeadlineFor.
func dogfoodEventDeadlineOf(ctx context.Context, host, event string) time.Duration {
	if deadline, ok := ctx.Value(dogfoodEventDeadlineKey{}).(func(string, string) time.Duration); ok {
		return deadline(host, event)
	}
	return dogfoodEventDeadlineFor(host, event)
}

// dogfoodDegradations mirrors the harness rule: only an `unknown` host version is a degradation. A
// host that documents no version reports that once in its adapter tuple instead (AHI-023).
func dogfoodDegradations(hostVersion string) []string {
	if hostVersion != "unknown" {
		return []string{"frontier-authority-unavailable"}
	}
	return []string{"frontier-authority-unavailable", "host-version-unknown"}
}

// errDogfoodSnapshotStale is the stale-snapshot outcome a miss reports without building when
// the recorded `index` build cost does not fit the time left (IDX-SNAP-V0-012, AHI-031).
var errDogfoodSnapshotStale = dogfoodEventError("dogfood-event-index-snapshot-stale")

// dogfoodMissOutlastsDeadline says whether a hook event's snapshot miss should skip the
// in-memory build: the last explicit `index` build took at least the time left. With no
// record, or outside a hook event, the miss builds as before.
func dogfoodMissOutlastsDeadline(ctx context.Context, root string) bool {
	missed, _ := ctx.Value(dogfoodEventMissKey{}).(*atomic.Bool)
	if missed == nil || !missed.Load() {
		return false
	}
	deadline, bounded := ctx.Deadline()
	cost, recorded := contextindex.RecordedBuildCost(root)
	return bounded && recorded && cost >= time.Until(deadline)
}
