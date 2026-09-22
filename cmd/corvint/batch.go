package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/trace"
)

const (
	// batchProfile names the wire this verb emits (SBQ-V0-004); the standalone
	// verbs emit no profile member, so a batch receipt is never mistaken for one.
	batchProfile = "snapshot-batch/0"
	// maxBatchOperations and maxBatchOperationID are the request bounds
	// (SBQ-V0-001); the 128 KiB stdin bound is the kernel's own.
	maxBatchOperations  = 16
	maxBatchOperationID = 64
	// maxPossessedEntries is SBQ-V0-007(a)'s entry cap. It is secondary to the
	// byte bound: `path` and `blob_hash` are unbounded, so 32 entries at
	// ~4,200-byte paths exceed 128 KiB and the byte bound refuses them first.
	maxPossessedEntries = 32
)

// possessedEntryMembers is the closed member set of one possession entry
// (SBQ-V0-007(a)); `line` is validated and then dropped, so a stored line and
// one dropped after validation are indistinguishable on the wire.
var possessedEntryMembers = map[string]struct{}{"path": {}, "blob_hash": {}, "line": {}}

// batchVerbMembers names the argument members each verb admits. The map is the
// verb table: an unlisted verb and an unlisted member are both structural
// refusals, decided before any repository read (SBQ-V0-001).
var batchVerbMembers = map[string]map[string]struct{}{
	"query":   {"task": {}, "limit": {}, "budget_bytes": {}, "possessed": {}},
	"context": {"task": {}, "subject": {}, "limit": {}, "possessed": {}},
	"impact":  {"paths": {}, "limit": {}, "possessed": {}},
}

// batchVerbDefaultLimit is each standalone verb's own default, so an operation
// that omits limit answers exactly as the standalone invocation does.
var batchVerbDefaultLimit = map[string]int{"query": 10, "context": taskContextDefaultLimit, "impact": 10}

// batchOperation is one parsed request row. Its arguments are bounded but not
// yet validated by the verb itself: an oversize limit, a missing task or an
// untracked path stays a per-operation refusal (SBQ-V0-004). refusal carries an
// argument error the standalone verb would raise at its own parse; it is that
// operation's ok=false and never aborts the batch.
type batchOperation struct {
	id      string
	verb    string
	task    string
	subject string
	paths   []string
	limit   int
	budget  *int
	refusal error
	// possessed is the caller's possession list (SBQ-V0-007); supplied says
	// whether the member was present at all, since an operation that supplied
	// none takes no fallback entry and contributes nothing to delta.
	possessed []contextindex.PossessedEntry
	supplied  bool
}

// preloadedIndex is the batch seam. An operation hands the standalone path the
// one snapshot the batch already loaded, so the standalone verb skips its own
// acquisition and its receipt stays byte-identical (SBQ-V0-003).
func preloadedIndex(loaded []*contextindex.Index) *contextindex.Index {
	if len(loaded) == 0 {
		return nil
	}
	return loaded[0]
}

// parseBatchInvocation recognises `[--root PATH] batch`; any other shape is not
// a batch invocation. The root is returned unresolved: SBQ-V0-001 refuses a
// malformed request before any repository read, so the root is stat'ed only
// after the request validates (runBatch).
func parseBatchInvocation(arguments []string) (string, bool, error) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return "", false, nil
	}
	root := ""
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		value := ""
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", false, nil
			}
			value, index = arguments[index+1], index+2
		} else {
			value, index = strings.TrimPrefix(arguments[index], "--root="), index+1
		}
		root = value
	}
	if index >= len(arguments) || arguments[index] != "batch" {
		return "", false, nil
	}
	if index+1 < len(arguments) {
		return "", true, argumentError("unrecognized arguments: " + arguments[index+1])
	}
	if root == "" {
		root = "."
	}
	return root, true, nil
}

// runBatch answers a bounded list of independent read operations from one
// loaded snapshot. Read-only on every path: no index, trace, cache or ledger
// write, and no self-observation append even on the unsupported- refusal
// (SBQ-V0-005, invariant 4).
func runBatch(ctx context.Context, root string, stdin io.Reader, stdout, stderr io.Writer) int {
	input, err := readInputBounded(ctx, stdin, gokernel.MaxInputBytes)
	if err != nil {
		code, message := "invalid-harness-input", "cannot read batch input"
		if ctx.Err() != nil {
			code, message = "harness-input-cancelled", "batch input read was cancelled"
		}
		emitError(stderr, &gokernel.Error{Code: code, Message: message})
		return 2
	}
	operations, err := parseBatchRequest(input)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	// SBQ-V0-001: only a valid request opens the repository, so a malformed
	// request against a directory that is not a repository is still
	// invalid-arguments.
	root, err = resolveExplicitRoot(root)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	// Under the opt-in the batch's status scans share their metadata probes,
	// but the loader keeps its closing identity read: context and impact
	// operations answer from the index without a bracket of their own.
	if sharedQueryObservation() {
		ctx = contextindex.WithSharedQueryObservation(ctx)
	}
	index, hit, opening, err := loadQuerySnapshot(ctx, root)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if !hit {
		// SBQ-V0-002: a deliberate divergence from IDX-SNAP-V0-003. A batch
		// answers at one identity; a caller wanting build fallback calls the
		// standalone verbs.
		emitError(stderr, batchSnapshotRefusal())
		return 2
	}
	// Only a hit carries the loader's observation into the query operations
	// (proposed GPK-V0-065); a miss has already returned above.
	ctx = context.WithValue(ctx, batchOpeningKey{}, opening)
	return emitBatchReceipt(stdout, stderr, batchReceipt(ctx, root, index, operations))
}

// batchReceipt is the stdout document (SBQ-V0-004). Operations run in request
// order and a failing one never stops the next. `delta` is the batch-level
// possession report; it is absent when no operation supplied `possessed`.
func batchReceipt(ctx context.Context, root string, index *contextindex.Index, operations []batchOperation) map[string]any {
	rows := make([]map[string]any, 0, len(operations))
	collector := possessionCollector{}
	for _, operation := range operations {
		rows = append(rows, batchOperationRow(ctx, root, index, operation, &collector))
	}
	receipt := map[string]any{
		"ok": true, "mutates": false, "tool": "batch", "profile": batchProfile,
		"snapshot":   map[string]any{"tree": index.Revision, "commit": index.CommitRevision},
		"operations": rows,
	}
	if collector.supplied {
		receipt["delta"] = collector.delta(index)
	}
	return receipt
}

func batchOperationRow(ctx context.Context, root string, index *contextindex.Index, operation batchOperation, collector *possessionCollector) map[string]any {
	collector.supplied = collector.supplied || operation.supplied
	ctx = context.WithValue(ctx, batchTraceObserverKey{}, func(records []trace.Record, state string) {
		collector.traces = append(collector.traces, map[string]any{
			"operation": operation.id, "state": state, "digest": batchTraceDigest(records, state),
		})
	})
	receipt, err := runBatchOperation(ctx, root, index, operation)
	if err == nil && operation.supplied {
		err = collector.observe(index, operation, receipt)
	}
	if err != nil {
		return map[string]any{"id": operation.id, "verb": operation.verb, "ok": false, "error": batchOperationError(err)}
	}
	return map[string]any{"id": operation.id, "verb": operation.verb, "ok": true, "context": receipt}
}

// possessionCollector accumulates the batch-level delta as operations run.
// Suppression runs here, on the receipt runBatchOperation returned, so no
// verb's code path changes (SBQ-V0-009).
type possessionCollector struct {
	supplied    bool
	traces      []any
	suppressed  []any
	invalidated []contextindex.InvalidatedResult
	ignored     []any
	fallback    []any
}

func (collector *possessionCollector) observe(index *contextindex.Index, operation batchOperation, receipt map[string]any) error {
	outcome, err := contextindex.ApplyPossession(index, receipt, operation.possessed)
	if err != nil {
		return err
	}
	if outcome.Fallback != "" {
		collector.fallback = append(collector.fallback, map[string]any{
			"operation": operation.id, "reason": outcome.Fallback,
		})
	}
	for _, item := range outcome.Suppressed {
		collector.suppressed = append(collector.suppressed, map[string]any{
			"kind": item.Kind, "id": item.ID, "rows": item.Rows,
			"reason": "possessed-retained", "profile": batchProfile,
			"operation": operation.id, "recover": "resend the operation without `possessed`",
		})
	}
	collector.invalidated = append(collector.invalidated, outcome.Invalidated...)
	for _, entry := range outcome.Ignored {
		collector.ignored = append(collector.ignored, map[string]any{
			"operation": operation.id, "path": entry.Path, "blob_hash": entry.BlobHash,
		})
	}
	return nil
}

// delta renders the batch-level member. Every list except `invalidated` is
// already in operation order, then result order within the operation;
// `invalidated` is keyed without an operation and sorts by (kind, id, path)
// alone. Each list is absent, not an empty array, when it holds nothing
// (SBQ-V0-010(f)).
func (collector *possessionCollector) delta(index *contextindex.Index) map[string]any {
	binding := map[string]any{
		"tree": index.Revision, "commit": index.CommitRevision,
		"engine": contextindex.LoadedEngineID(), "status_sha256": index.StatusSHA256,
		"trace_digests": collector.traces,
	}
	result := map[string]any{"binding": binding}
	if len(collector.suppressed) != 0 {
		result["suppressed"] = collector.suppressed
	}
	if len(collector.invalidated) != 0 {
		contextindex.SortInvalidated(collector.invalidated)
		rows := make([]any, 0, len(collector.invalidated))
		seen := make(map[contextindex.InvalidatedResult]struct{}, len(collector.invalidated))
		for _, item := range collector.invalidated {
			if _, duplicate := seen[item]; duplicate {
				continue
			}
			seen[item] = struct{}{}
			rows = append(rows, map[string]any{
				"kind": item.Kind, "id": item.ID, "path": item.Path, "verdict": item.Verdict,
			})
		}
		result["invalidated"] = rows
	}
	if len(collector.ignored) != 0 {
		result["ignored"] = collector.ignored
	}
	if len(collector.fallback) != 0 {
		result["fallback"] = collector.fallback
	}
	return result
}

// runBatchOperation runs the standalone verb's own code path against the loaded
// index, so the receipt is the standalone verb's own (SBQ-V0-003).
func runBatchOperation(ctx context.Context, root string, index *contextindex.Index, operation batchOperation) (map[string]any, error) {
	if operation.refusal != nil {
		return nil, operation.refusal
	}
	switch operation.verb {
	case "query":
		intent, err := contextindex.ValidateQueryCommand(operation.task, operation.limit)
		if err != nil {
			return nil, err
		}
		return standaloneQueryContext(ctx, options{
			root: root, queryTask: operation.task, queryLimit: operation.limit,
			queryBudget: operation.budget, queryIntent: intent,
		}, index)
	case "context":
		return contextindex.TaskContext(ctx, index, operation.task, operation.subject, operation.limit)
	}
	paths, err := batchImpactPaths(operation.paths)
	if err != nil {
		return nil, err
	}
	return standaloneImpactContext(index, paths, operation.limit)
}

// batchImpactPaths applies the standalone impact adapter's own path admission,
// so a path the standalone verb refuses is refused here with the same error.
func batchImpactPaths(paths []string) ([]string, error) {
	normalized := make([]string, 0, len(paths))
	for _, value := range paths {
		cleaned, err := normalizeImpactPath(value)
		if err != nil {
			return nil, err
		}
		if err := validateImpactPathAdmission(cleaned); err != nil {
			return nil, err
		}
		normalized = append(normalized, cleaned)
	}
	return normalized, nil
}

// batchOperationError renders one operation's refusal the way emitError renders
// the standalone verb's: a contextindex.Error without a code omits the member, and
// a DRC-V0 refusal carries the same additive diagnostic members.
func batchOperationError(err error) map[string]any {
	result := map[string]any{"message": err.Error()}
	var contextError *contextindex.Error
	if errors.As(err, &contextError) {
		if contextError.Code != "" {
			result["code"] = contextError.Code
		}
		return result
	}
	code := "internal-error"
	var kernelError *gokernel.Error
	if errors.As(err, &kernelError) {
		code = kernelError.Code
	}
	result["code"] = code
	addDiagnosticRowMembers(result, err)
	return result
}

func emitBatchReceipt(stdout, stderr io.Writer, payload map[string]any) int {
	// contextindex's encoder is the one the query and impact receipts are
	// written with, so an embedded receipt keeps the bytes it has standalone.
	encoded, err := contextindex.CanonicalJSON(payload)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write batch output"})
		return 2
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write batch output"})
		return 2
	}
	return 0
}

// parseBatchRequest validates the whole request before the caller opens the
// repository (SBQ-V0-001).
func parseBatchRequest(input []byte) ([]batchOperation, error) {
	if len(input) > gokernel.MaxInputBytes {
		return nil, argumentError(fmt.Sprintf("batch request exceeds %d bytes", gokernel.MaxInputBytes))
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(input, &document); err != nil {
		return nil, argumentError("batch request must be a JSON object with one member: operations")
	}
	for member := range document {
		if member != "operations" {
			return nil, argumentError("unrecognized batch member: " + member)
		}
	}
	raw, present := document["operations"]
	if !present {
		return nil, argumentError("the following members are required: operations")
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil {
		return nil, argumentError("operations must be a list of objects")
	}
	if len(rows) == 0 {
		return nil, argumentError("operations must be a non-empty list")
	}
	if len(rows) > maxBatchOperations {
		return nil, argumentError(fmt.Sprintf("operations exceed the %d-operation bound", maxBatchOperations))
	}
	seen := make(map[string]struct{}, len(rows))
	operations := make([]batchOperation, 0, len(rows))
	for position, row := range rows {
		operation, err := parseBatchOperation(position, row)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[operation.id]; duplicate {
			return nil, argumentError("duplicate operation id: " + operation.id)
		}
		seen[operation.id] = struct{}{}
		operations = append(operations, operation)
	}
	return operations, nil
}

func parseBatchOperation(position int, row map[string]json.RawMessage) (batchOperation, error) {
	identifier, err := batchOperationIdentifier(position, row)
	if err != nil {
		return batchOperation{}, err
	}
	prefix := "operation " + identifier + ": "
	verb, present, err := batchStringMember(row, "verb", prefix)
	if err != nil {
		return batchOperation{}, err
	}
	if !present {
		return batchOperation{}, argumentError(prefix + "the following members are required: verb")
	}
	members, known := batchVerbMembers[verb]
	if !known {
		return batchOperation{}, argumentError(prefix + "unrecognized verb: " + verb)
	}
	if err := batchKnownMembers(row, members, prefix); err != nil {
		return batchOperation{}, err
	}
	operation := batchOperation{id: identifier, verb: verb, limit: batchVerbDefaultLimit[verb]}
	if err := batchOperationArguments(&operation, row, prefix); err != nil {
		return batchOperation{}, err
	}
	return operation, nil
}

func batchOperationArguments(operation *batchOperation, row map[string]json.RawMessage, prefix string) error {
	task, _, err := batchStringMember(row, "task", prefix)
	if err != nil {
		return err
	}
	subject, _, err := batchStringMember(row, "subject", prefix)
	if err != nil {
		return err
	}
	limit, present, err := batchIntegerMember(row, "limit", prefix)
	if err != nil {
		return err
	}
	if present {
		operation.limit = limit
	}
	budget, present, err := batchIntegerMember(row, "budget_bytes", prefix)
	if err != nil {
		return err
	}
	if present {
		// An out-of-range budget is the query verb's own parse refusal, so it
		// is this operation's ok=false and not the batch's (SBQ-V0-004).
		if budget < contextindex.MinPacketBytes || budget > contextindex.MaxPacketBytes {
			operation.refusal = argumentError(fmt.Sprintf(
				"%sbudget_bytes must be between %d and %d",
				prefix, contextindex.MinPacketBytes, contextindex.MaxPacketBytes,
			))
		} else {
			operation.budget = &budget
		}
	}
	paths, err := batchPathsMember(row, prefix)
	if err != nil {
		return err
	}
	possessed, supplied, err := batchPossessedMember(row, prefix)
	if err != nil {
		return err
	}
	operation.possessed, operation.supplied = possessed, supplied
	operation.task, operation.subject, operation.paths = task, subject, paths
	// A verb that admits task requires one, exactly as its standalone adapter
	// does (main.go, taskcontext.go): a per-operation refusal, not a batch one.
	if _, admitted := batchVerbMembers[operation.verb]["task"]; admitted && task == "" && operation.refusal == nil {
		operation.refusal = argumentError(prefix + "the following arguments are required: --task")
	}
	return nil
}

// batchPossessedMember parses and validates the possession list before any
// repository read (SBQ-V0-007(b)). The 32-entry cap is secondary to the byte
// bound parseBatchRequest already applied.
func batchPossessedMember(row map[string]json.RawMessage, prefix string) ([]contextindex.PossessedEntry, bool, error) {
	raw, present := row["possessed"]
	if !present {
		return nil, false, nil
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil {
		return nil, true, argumentError(prefix + "possessed must be a list of objects")
	}
	if len(rows) > maxPossessedEntries {
		return nil, true, argumentError(fmt.Sprintf("%spossessed exceeds the %d-entry bound", prefix, maxPossessedEntries))
	}
	entries := make([]contextindex.PossessedEntry, 0, len(rows))
	seen := make(map[contextindex.PossessedEntry]struct{}, len(rows))
	for position, item := range rows {
		entry, err := batchPossessedEntry(position, item, prefix)
		if err != nil {
			return nil, true, err
		}
		if _, duplicate := seen[entry]; duplicate {
			return nil, true, argumentError(fmt.Sprintf(
				"%spossessed repeats the pair (%s, %s)", prefix, entry.Path, entry.BlobHash))
		}
		seen[entry] = struct{}{}
		entries = append(entries, entry)
	}
	return entries, true, nil
}

func batchPossessedEntry(position int, item map[string]json.RawMessage, prefix string) (contextindex.PossessedEntry, error) {
	entryPrefix := fmt.Sprintf("%spossessed[%d]: ", prefix, position)
	for member := range item {
		if _, admitted := possessedEntryMembers[member]; !admitted {
			return contextindex.PossessedEntry{}, argumentError(entryPrefix + "unrecognized member: " + member)
		}
	}
	entry := contextindex.PossessedEntry{}
	for _, member := range []string{"path", "blob_hash"} {
		value, present, err := batchStringMember(item, member, entryPrefix)
		if err != nil {
			return contextindex.PossessedEntry{}, err
		}
		if !present || value == "" {
			return contextindex.PossessedEntry{}, argumentError(entryPrefix + member + " must be a non-empty string")
		}
		if member == "path" {
			entry.Path = value
		} else {
			entry.BlobHash = value
		}
	}
	// `line` is validated and then dropped: no V0 clause reads it.
	if raw, present := item["line"]; present {
		// The ignored integer has no machine-word bound in SBQ-V0-007.
		line, integer := new(big.Int).SetString(strings.TrimSpace(string(raw)), 10)
		if !integer || line.Sign() < 0 {
			return contextindex.PossessedEntry{}, argumentError(entryPrefix + "line must be a non-negative integer")
		}
	}
	return entry, nil
}

func batchKnownMembers(row map[string]json.RawMessage, members map[string]struct{}, prefix string) error {
	for member := range row {
		if member == "id" || member == "verb" {
			continue
		}
		if _, admitted := members[member]; !admitted {
			return argumentError(prefix + "unrecognized member: " + member)
		}
	}
	return nil
}

func batchOperationIdentifier(position int, row map[string]json.RawMessage) (string, error) {
	raw, present := row["id"]
	if !present {
		return "", argumentError(fmt.Sprintf("operation %d: the following members are required: id", position))
	}
	var identifier string
	if err := json.Unmarshal(raw, &identifier); err != nil {
		return "", argumentError(fmt.Sprintf("operation %d: id must be a string", position))
	}
	if identifier == "" || len(identifier) > maxBatchOperationID {
		return "", argumentError(fmt.Sprintf("operation %d: id must be 1 to %d bytes", position, maxBatchOperationID))
	}
	return identifier, nil
}

// batchStringMember and batchIntegerMember decode through a pointer so that an
// explicit JSON null is distinguishable from an absent member and from the zero
// value: null is a structural refusal, decided before any repository read
// (SBQ-V0-001).
func batchStringMember(row map[string]json.RawMessage, member, prefix string) (string, bool, error) {
	raw, present := row[member]
	if !present {
		return "", false, nil
	}
	var value *string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", true, argumentError(prefix + member + " must be a string")
	}
	if value == nil {
		return "", true, argumentError(prefix + member + " must not be null")
	}
	return *value, true, nil
}

func batchIntegerMember(row map[string]json.RawMessage, member, prefix string) (int, bool, error) {
	raw, present := row[member]
	if !present {
		return 0, false, nil
	}
	var value *int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, true, argumentError(prefix + member + " must be an integer")
	}
	if value == nil {
		return 0, true, argumentError(prefix + member + " must not be null")
	}
	return *value, true, nil
}

func batchPathsMember(row map[string]json.RawMessage, prefix string) ([]string, error) {
	raw, present := row["paths"]
	if !present {
		return nil, nil
	}
	var value *[]string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, argumentError(prefix + "paths must be a list of strings")
	}
	if value == nil {
		return nil, argumentError(prefix + "paths must not be null")
	}
	return *value, nil
}

const batchHelp = `Answer several independent read requests from one loaded index snapshot.

Usage:
  corvint [--root PATH] batch < REQUEST

Read-only and Go-only (snapshot-batch-v0, experimental). Reads one canonical
JSON object from stdin, at most 131072 bytes:

  {"operations": [{"id": "q", "verb": "query", "task": "..."},
                  {"id": "c", "verb": "context", "task": "...",
                   "subject": "path.go"},
                  {"id": "i", "verb": "impact", "paths": ["path.go"]}]}

At most 16 operations, each with a distinct non-empty id of at most 64 bytes.
"query" takes task and optional limit, budget_bytes; "context" takes task and
optional subject, limit; "impact" takes paths and optional limit. Any other
member, verb, duplicate id, null scalar, or oversize request is refused with
invalid-arguments before the repository is read. An argument the standalone
verb refuses at its own parse (a missing task, an out-of-range budget_bytes)
is instead that one operation's invalid-arguments error.

The snapshot ".corvint/index/" holds is loaded once and every operation answers
from it, so each receipt is the standalone verb's own "context" member at that
one identity. Unlike "context", a missing or stale snapshot is refused with
unsupported-batch-snapshot rather than rebuilt: run "corvint index" first, or
call the standalone verbs for build fallback.

Stdout is one line carrying ok, mutates, tool, profile "snapshot-batch/0", the
snapshot tree and commit, and one operations row per request operation in
request order: ok with "context", or ok false with "error". A failing
operation does not stop the next, and the exit status is 0 whenever the batch
executed. Nothing is written on any path, including the self-observation
ledger the standalone impact verb appends to on failure.
`

// batchTraceObserverKey carries a synchronous observation of the exact read the
// standalone query consumes. It adds no reads and cannot outlive this operation.
type batchTraceObserverKey struct{}

func observeBatchTraces(ctx context.Context, records []trace.Record, state string) {
	if observe, ok := ctx.Value(batchTraceObserverKey{}).(func([]trace.Record, string)); ok {
		observe(records, state)
	}
}

func batchTraceDigest(records []trace.Record, state string) string {
	ordered := append([]trace.Record(nil), records...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].TraceID < ordered[j].TraceID })
	projected := make([]any, 0, len(ordered))
	for _, record := range ordered {
		row := map[string]any{
			"schema_version": record.SchemaVersion, "revision": record.Revision,
			"trace_id": record.TraceID, "task": record.Task, "outcome": record.Outcome,
		}
		if record.OpenedPaths != nil {
			row["opened_paths"] = record.OpenedPaths
		}
		if record.ChangedPaths != nil {
			row["changed_paths"] = record.ChangedPaths
		}
		if record.Verification != nil {
			row["verification"] = record.Verification
		}
		projected = append(projected, row)
	}
	// The closed projection contains only JSON strings, integers, maps and arrays.
	encoded, _ := contextindex.CanonicalJSON(map[string]any{"state": state, "records": projected})
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}
