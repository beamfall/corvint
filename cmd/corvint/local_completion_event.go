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
	"slices"
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
	if !dogfoodEventHostAdmitted(options) {
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
		emitError(stderr, dogfoodEventError(dogfoodInputFailureCode(ctx, missed)))
		return 2
	}
	input, err := dogfoodEventInput(options.event, raw)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	result, err := dogfoodEventWithin(ctx, options, input)
	if err != nil && (errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, errDogfoodSnapshotStale) || probeExpired(err)) {
		// An expired deadline surfaces in later reads as unrelated drift or unavailability, and a
		// skipped miss build (AHI-031) forecasts one; report the time bound, not a diagnosed fault.
		emitError(stderr, dogfoodEventError(dogfoodExpiryCode(missed)))
		return 2
	}
	if err != nil {
		// Underlying repository/artifact diagnostics can contain caller data. The
		// native envelope uses a fixed error; explicit status retains its worklist.
		emitError(stderr, dogfoodEventError(dogfoodReadFailureCode(err)))
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

// Pi's ordinary extension uses a separate exact tuple. This admits only the
// caller-owned local policy, never the protected harness or Frontier authority.
func dogfoodEventHostAdmitted(options options) bool {
	if options.host == "pi" {
		return options.hostVersion == piHostVersion && options.surface == "extension" && options.adapterVersion == piAdapterVersion
	}
	hostVersion, admitted := dogfoodHostVersions[options.host]
	return admitted && options.surface == "plugin" && options.adapterVersion == adapterWireVersion && options.hostVersion == hostVersion
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
	if options.event == "user-prompt" || options.event == "session-start" {
		ctx = context.WithValue(ctx, dogfoodSnapshotPrefetchKey{}, &snapshotPrefetch{root: options.root, done: make(chan struct{})})
	}
	return localEventRead(ctx, options, input, dogfoodEventEnvelope, dogfoodEventContext, "")
}

// dogfoodSnapshotPrefetchKey carries the snapshot read a context-reading dogfood event starts
// from its opening observation's identity, overlapping that observation's status read
// (proposed LCP-V0-033; V1-1524, V1-2144). Measured on a 708-tracked/759-untracked dirty
// worktree under host load 40-150, the opening status read and the snapshot decode each took
// about a third of the event; sequentially they were the event's two largest costs.
type dogfoodSnapshotPrefetchKey struct{}

// snapshotPrefetch is one snapshot read issued against the opening identity alone. It names
// the same file the loader would (object format and tree) and applies no dirty set; the
// event's context read takes it only for that tree and then applies the complete observation.
type snapshotPrefetch struct {
	root               string
	done               chan struct{}
	objectFormat, tree string
	index              *contextindex.Index
	hit                bool
	err                error
}

// start is the bracket's prefetch callback: it runs on the observing goroutine, so it only
// starts the read. A decode the event abandons writes nothing and ends with its process.
func (prefetch *snapshotPrefetch) start(objectFormat, commitRevision, treeRevision string) {
	prefetch.objectFormat, prefetch.tree = objectFormat, treeRevision
	go func() {
		defer close(prefetch.done)
		prefetch.index, prefetch.hit, prefetch.err = loadSnapshotObserved(prefetch.root, contextindex.Observation{ObjectFormat: objectFormat, CommitRevision: commitRevision, Revision: treeRevision})
	}()
}

// started reports whether the bracket handed this prefetch an identity.
func (prefetch *snapshotPrefetch) started() bool {
	return prefetch.tree != ""
}

// take returns the prefetched read for the opening observation, with that observation applied
// exactly as loadSnapshotObserved applies it, or ok false when no prefetch read that tree.
func (prefetch *snapshotPrefetch) take(root string, observation contextindex.Observation) (*contextindex.Index, bool, error, bool) {
	if !prefetch.started() || prefetch.objectFormat != observation.ObjectFormat || prefetch.tree != observation.Revision {
		return nil, false, nil, false
	}
	<-prefetch.done
	if prefetch.err != nil || !prefetch.hit {
		return nil, false, prefetch.err, true
	}
	adoptObservation(prefetch.index, root, observation)
	return prefetch.index, true, nil, true
}

// adoptObservation applies the opening observation to a snapshot hit of the tree it names,
// exactly as contextindex.LoadSnapshotObserved applies it to its own hits: the root, the sorted
// unique dirty set, the status digest and the observed HEAD (the file is keyed by tree). It lives
// here rather than in contextindex so the analyzer-schema inputs (IDX-SNAP-V0-017) stay unchanged;
// TestDogfoodEventPrefetchesTheSnapshotFromTheOpeningIdentity pins byte equality with that loader.
func adoptObservation(index *contextindex.Index, root string, observation contextindex.Observation) {
	dirty := append(make([]string, 0, len(observation.DirtyPaths)), observation.DirtyPaths...)
	slices.Sort(dirty)
	index.Root, index.DirtyPaths, index.StatusSHA256 = root, slices.Compact(dirty), observation.StatusSHA256
	index.CommitRevision = observation.CommitRevision
}

func localEventRead(ctx context.Context, options options, input map[string]any,
	envelope func(options, map[string]any, gokernel.Repository, localcompletion.Evaluation) map[string]any,
	contextPacket func(context.Context, options, map[string]any, localcompletion.Evaluation, map[string]any, gokernel.Repository, gokernel.Observation) (map[string]any, error),
	expectedTarget string,
) (map[string]any, error) {
	// One GPK-V0-007 bracket spans the whole read (proposed LCP-V0-016, V1-0881): the
	// opening observation is the repository every block reports and the index loader
	// hits against, and the closing observation is the drift check. Before this the
	// event probed twice, each probe its own two observations and a profile read nothing
	// emitted, and the loader read the pair a third time.
	var before gokernel.Repository
	var evaluation localcompletion.Evaluation
	var observed localcompletion.Observed
	var result map[string]any
	var prefetch func(objectFormat, commitRevision, treeRevision string)
	if pending, ok := ctx.Value(dogfoodSnapshotPrefetchKey{}).(*snapshotPrefetch); ok {
		prefetch = pending.start
	}
	after, err := gokernel.ProbeRepositoryAroundPrefetching(ctx, options.root, prefetch, func(ctx context.Context, observation gokernel.Observation) error {
		var err error
		if before, err = observation.Repository(); err != nil {
			return err
		}
		// Only the held protected campaign scope supplies this guard. Reuse these
		// common probes: no candidate-only Git work can prewarm the context path.
		if expectedTarget != "" && before.CommitRevision != expectedTarget {
			return dogfoodEventError("qualified-lifecycle-target-drift")
		}
		evaluation = localcompletion.Evaluation{Lifecycle: "inactive", Unmet: []string{}}
		if session, ok := input["sessionIdSha256"].(string); ok {
			if evaluation, observed, err = localcompletion.EvaluateObserved(ctx, options.root, session); err != nil {
				return err
			}
		}
		result = envelope(options, input, before, evaluation)
		if options.event == "user-prompt" || options.event == "session-start" {
			packet, err := contextPacket(ctx, options, input, evaluation, result, before, observation)
			if err != nil {
				return err
			}
			result["context"] = packet
		}
		return nil
	})
	if errors.Is(err, gokernel.ErrRepositoryDrift) {
		return nil, dogfoodEventError("dogfood-event-repository-drift")
	}
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
		current, err := localEventReevaluate(ctx, options.root, session, observed, after)
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

// Reuse facts only after the event's closing observation proves their identity.
func localEventReevaluate(ctx context.Context, root, session string, observed localcompletion.Observed, after gokernel.Repository) (localcompletion.Evaluation, error) {
	if observed.Matches(after.CommitRevision, after.TreeRevision) {
		return localcompletion.EvaluateAgainst(ctx, root, session, observed)
	}
	return localcompletion.Evaluate(ctx, root, session)
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
	return withOtherSessionOwner(map[string]any{"decision": decision, "reason": reason}, event, evaluation)
}

func dogfoodEventContext(ctx context.Context, options options, input map[string]any, evaluation localcompletion.Evaluation, envelope map[string]any, repo gokernel.Repository, observation gokernel.Observation) (map[string]any, error) {
	return localEventContext(ctx, options, input, evaluation, envelope, repo, observation, true, dogfoodEventBytes, func(encoded []byte) (int, error) {
		quoted, err := gokernel.CanonicalJSON(string(bytes.TrimSuffix(encoded, []byte{'\n'})))
		return len(quoted) + 512, err
	})
}

// localEventContext compiles the prompt packet. With rehydrate, a compact
// session start over a dirty worktree also carries the compaction block
// (AHI-003) from the same index, under the packet's "compaction" key.
func localEventContext(ctx context.Context, options options, input map[string]any, evaluation localcompletion.Evaluation, envelope map[string]any, repo gokernel.Repository, observation gokernel.Observation,
	rehydrate bool, encode func(map[string]any, int) ([]byte, error), nativeSize func([]byte) (int, error),
) (map[string]any, error) {
	compact := rehydrate && options.event == "session-start" && input["startSource"] == "compact" && repo.DirtyPathCount > 0
	// The hit is read against the event's own opening observation, so it spawns
	// nothing; a miss still builds under the build's own bracket.
	observed := contextindex.Observation{
		ObjectFormat: observation.ObjectFormat, CommitRevision: observation.CommitRevision,
		Revision: observation.TreeRevision, DirtyPaths: observation.DirtyPaths, StatusSHA256: observation.StatusSHA256,
	}
	var index *contextindex.Index
	var hit, prefetched bool
	var err error
	if pending, ok := ctx.Value(dogfoodSnapshotPrefetchKey{}).(*snapshotPrefetch); ok {
		index, hit, err, prefetched = pending.take(options.root, observed)
	}
	if !prefetched {
		index, hit, err = loadSnapshotObserved(options.root, observed)
	}
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
		index, err = dogfoodMissBuild(ctx, options.root, func(ctx context.Context) (*contextindex.Index, error) {
			return contextindex.Build(ctx, options.root)
		})
	} else if err != nil || !hit {
		index, err = dogfoodMissBuild(ctx, options.root, func(ctx context.Context) (*contextindex.Index, error) {
			return buildContext(ctx, options.root, "")
		})
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

// dogfoodMissBuild runs a hook event's snapshot-miss build. With no usable
// build-cost record the build gets half the time left before the deadline and,
// when it has not finished by then, the miss reports the stale snapshot with
// the other half unspent, so a loaded host's watchdog does not kill the first
// hook of a never-indexed repository (AHI-920). The in-memory compile does not
// observe cancellation, so the abandoned build ends with its process, as in
// dogfoodEventWithin. A recorded cost or an unbounded context builds as before.
func dogfoodMissBuild(ctx context.Context, root string, build func(context.Context) (*contextindex.Index, error)) (*contextindex.Index, error) {
	deadline, bounded := ctx.Deadline()
	if _, recorded := contextindex.RecordedBuildCost(root); !bounded || recorded {
		return build(ctx)
	}
	half, cancel := context.WithDeadline(ctx, time.Now().Add(time.Until(deadline)/2))
	defer cancel()
	type outcome struct {
		index *contextindex.Index
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		index, err := build(half)
		done <- outcome{index, err}
	}()
	select {
	case finished := <-done:
		if finished.err != nil && half.Err() != nil && ctx.Err() == nil {
			return nil, errDogfoodSnapshotStale
		}
		return finished.index, finished.err
	case <-half.Done():
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errDogfoodSnapshotStale
	}
}

// probeExpired reports a Git repository probe that expired on its own fixed
// bound before the event's deadline: a time bound, not an unavailable
// repository (LCP-V0-008, V1-0396).
func probeExpired(err error) bool {
	var failure *gokernel.Error
	return errors.As(err, &failure) && failure.Code == "repository-probe-timeout"
}

// withOtherSessionOwner names another session's active enrollment on a Stop
// that releases because this key has none (LCP-V0-008).
func withOtherSessionOwner(completion map[string]any, event string, evaluation localcompletion.Evaluation) map[string]any {
	if event != "stop" || evaluation.Owner == "" {
		return completion
	}
	return map[string]any{"decision": "release", "reason": "local-policy-other-session-active", "owner": evaluation.Owner}
}

// dogfoodInputFailureCode is the code a failed stdin read reports. The read returns the context's
// error once the deadline passes, so a read that ends with the deadline passed reports the event's
// time bound, as an expiry in any later read does (LCP-V0-008, V1-1391); otherwise the input is
// unavailable.
func dogfoodInputFailureCode(ctx context.Context, missed *atomic.Bool) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return dogfoodExpiryCode(missed)
	}
	return "dogfood-event-input-unavailable"
}

// dogfoodReadFailureCode is the fixed code a failed event read reports (LCP-V0-008, LCP-V0-016,
// decision 0533): a worktree that moved under the read is dogfood-event-worktree-churn, an
// observed policy change keeps dogfood-event-policy-drift, a prompt the bounded prompt-context
// contract refuses is dogfood-event-prompt-bounds (LCP-V0-034), and every other failure stays
// the fixed dogfood-event-unavailable. Only the typed code is read, never message text.
func dogfoodReadFailureCode(err error) string {
	var bounds *contextindex.Error
	if errors.As(err, &bounds) {
		switch bounds.Code {
		case "unsupported-dogfood-context-bounds", "unsupported-dogfood-context-budget", "invalid-dogfood-context":
			return "dogfood-event-prompt-bounds"
		}
	}
	var failure *gokernel.Error
	if !errors.As(err, &failure) {
		return "dogfood-event-unavailable"
	}
	switch failure.Code {
	case "repository-state-unstable", "dogfood-event-repository-drift", "dogfood-event-context-drift":
		return "dogfood-event-worktree-churn"
	case "dogfood-event-policy-drift":
		return failure.Code
	}
	return "dogfood-event-unavailable"
}
