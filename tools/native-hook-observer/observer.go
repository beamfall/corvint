package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"time"
)

const (
	maxFrameBytes      = 65536
	maxEvents          = 256
	maxConnection      = 30 * time.Second
	maxProjectionBytes = 8192
)

var (
	hookMethods = stringSet("hook/started", "hook/completed")
	statuses    = stringSet("running", "completed", "failed", "blocked", "stopped")
	entryKinds  = stringSet("warning", "stop", "feedback", "context", "error")
	eventNames  = stringSet("preToolUse", "permissionRequest", "postToolUse", "preCompact", "postCompact", "sessionStart", "sessionEnd", "userPromptSubmit", "subagentStart", "subagentStop", "stop", "interrupt")
	sources     = stringSet("system", "user", "project", "mdm", "sessionFlags", "plugin", "cloudRequirements", "cloudManagedConfig", "legacyManagedConfigFile", "legacyManagedConfigMdm", "unknown")
)

func stringSet(values ...string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func boundedText(value any, limit int) (string, error) {
	text, ok := value.(string)
	if !ok || text == "" || len([]byte(text)) > limit {
		return "", errors.New("invalid-text")
	}
	return text, nil
}

func textDigest(value any, limit int) (string, error) {
	text, err := boundedText(value, limit)
	if err != nil {
		return "", err
	}
	return digestBytes([]byte(text)), nil
}

func enum(value any, allowed map[string]bool) (string, error) {
	text, ok := value.(string)
	if !ok || !allowed[text] {
		return "", errors.New("unsupported-enum")
	}
	return text, nil
}

func normalizeHook(raw []byte) (map[string]any, error) {
	if len(raw) > maxFrameBytes {
		return nil, errors.New("frame-too-large")
	}
	decoded, err := decodeClosedJSON(raw)
	if err != nil {
		return nil, err
	}
	value, err := asObject(decoded, "invalid-notification")
	if err != nil {
		return nil, err
	}
	method, _ := value["method"].(string)
	if !hookMethods[method] {
		return nil, nil
	}
	if len(value) < 2 || len(value) > 3 || value["jsonrpc"] != nil && value["jsonrpc"] != "2.0" {
		return nil, errors.New("unsupported-notification-fields")
	}
	for key := range value {
		if key != "jsonrpc" && key != "method" && key != "params" {
			return nil, errors.New("unsupported-notification-fields")
		}
	}
	params, err := asObject(value["params"], "invalid-params")
	if err != nil || exactFields(params, "threadId", "turnId", "run") != nil {
		return nil, errors.New("invalid-params")
	}
	run, err := asObject(params["run"], "unsupported-run-fields")
	if err != nil || exactFields(run, "id", "eventName", "handlerType", "executionMode", "scope", "sourcePath", "source", "displayOrder", "status", "statusMessage", "startedAt", "completedAt", "durationMs", "entries") != nil {
		return nil, errors.New("unsupported-run-fields")
	}
	entries, err := asArray(run["entries"], "invalid-entries")
	if err != nil || len(entries) > 32 {
		return nil, errors.New("invalid-entries")
	}
	kinds := make([]any, 0, len(entries))
	entryDigests := make([]any, 0, len(entries))
	for _, rawEntry := range entries {
		entry, err := asObject(rawEntry, "unsupported-entry-fields")
		if err != nil || exactFields(entry, "kind", "text") != nil {
			return nil, errors.New("unsupported-entry-fields")
		}
		kind, err := enum(entry["kind"], entryKinds)
		if err != nil {
			return nil, err
		}
		text, ok := entry["text"].(string)
		if !ok || len([]byte(text)) > 8192 {
			return nil, errors.New("entry-too-large")
		}
		kinds, entryDigests = append(kinds, kind), append(entryDigests, digestBytes([]byte(text)))
	}
	status, err := enum(run["status"], statuses)
	if err != nil || (method == "hook/started") != (status == "running") {
		return nil, errors.New("inconsistent-lifecycle")
	}
	started, _, err := integer(run["startedAt"], false)
	if err != nil || started < 0 || started > 1<<53-1 {
		return nil, errors.New("invalid-timing")
	}
	completed, hasCompleted, err := integer(run["completedAt"], true)
	if err != nil || completed < 0 || completed > 1<<53-1 {
		return nil, errors.New("invalid-timing")
	}
	duration, hasDuration, err := integer(run["durationMs"], true)
	if err != nil || duration < 0 || duration > 1<<53-1 {
		return nil, errors.New("invalid-timing")
	}
	if status == "running" && (hasCompleted || hasDuration) || hasCompleted && completed < started {
		return nil, errors.New("inconsistent-running-timing")
	}
	display, _, err := integer(run["displayOrder"], false)
	if err != nil || display < 0 || display > 1<<53-1 {
		return nil, errors.New("invalid-timing")
	}
	if message := run["statusMessage"]; message != nil {
		text, ok := message.(string)
		if !ok || len([]byte(text)) > 4096 {
			return nil, errors.New("invalid-status-message")
		}
	}
	thread, err := textDigest(params["threadId"], 4096)
	if err != nil {
		return nil, err
	}
	var turn any
	if params["turnId"] != nil {
		turn, err = textDigest(params["turnId"], 4096)
		if err != nil {
			return nil, err
		}
	}
	runDigest, err := textDigest(run["id"], 4096)
	if err != nil {
		return nil, err
	}
	sourcePath, err := textDigest(run["sourcePath"], 4096)
	if err != nil {
		return nil, err
	}
	event, err := enum(run["eventName"], eventNames)
	if err != nil {
		return nil, err
	}
	handler, err := enum(run["handlerType"], stringSet("command", "mcpTool", "prompt", "agent"))
	if err != nil {
		return nil, err
	}
	execution, err := enum(run["executionMode"], stringSet("sync", "async"))
	if err != nil {
		return nil, err
	}
	scope, err := enum(run["scope"], stringSet("thread", "turn"))
	if err != nil {
		return nil, err
	}
	source, err := enum(run["source"], sources)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"profile": "corvint-native-hook-observation/0-experimental", "qualification": "UNQUALIFIED", "method": method,
		"threadSHA256": thread, "turnSHA256": turn, "runSHA256": runDigest, "sourcePathSHA256": sourcePath,
		"eventName": event, "handlerType": handler, "executionMode": execution, "scope": scope, "source": source,
		"status": status, "startedAt": started, "completedAt": nullableInt(completed, hasCompleted), "durationMs": nullableInt(duration, hasDuration),
		"entryKinds": kinds, "entryTextSHA256": entryDigests,
	}, nil
}

func nullableInt(value int64, present bool) any {
	if !present {
		return nil
	}
	return value
}

func runObserver(ctx context.Context, input io.Reader, output io.Writer, lifetime time.Duration) int {
	deadline := time.Now().Add(lifetime)
	if file, ok := input.(*os.File); ok {
		_ = file.SetReadDeadline(deadline)
		defer file.SetReadDeadline(time.Time{})
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	reader := bufio.NewReaderSize(lifetimeReader{ctx: ctx, input: input}, maxFrameBytes+1)
	frames := 0
	for frames < maxEvents {
		if time.Now().After(deadline) || ctx.Err() != nil {
			return emitObserverFailure(output, deadline)
		}
		raw, err := reader.ReadSlice('\n')
		if errors.Is(err, io.EOF) && len(raw) == 0 {
			return 0
		}
		if err != nil || len(raw)-1 > maxFrameBytes {
			return emitObserverFailure(output, deadline)
		}
		frames++
		value, err := normalizeHook(raw[:len(raw)-1])
		if err != nil {
			return emitObserverFailure(output, deadline)
		}
		if value != nil {
			if err := emitProjection(output, value, deadline); err != nil {
				return 1
			}
		}
	}
	if _, err := reader.Peek(1); errors.Is(err, io.EOF) {
		return 0
	}
	return emitObserverFailure(output, deadline)
}

// lifetimeReader bounds each read by ctx even when the input is a blocking,
// non-pollable fd that ignores read deadlines (NPO-V0-009). A read abandoned at
// the deadline stays blocked on its goroutine until input arrives or the
// process exits; runObserver never reads again after a failed read.
type lifetimeReader struct {
	ctx   context.Context
	input io.Reader
}

type readResult struct {
	data []byte
	err  error
}

func (r lifetimeReader) Read(p []byte) (int, error) {
	result := make(chan readResult, 1)
	go func() {
		buffer := make([]byte, len(p))
		count, err := r.input.Read(buffer)
		result <- readResult{buffer[:count], err}
	}()
	select {
	case got := <-result:
		return copy(p, got.data), got.err
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	}
}

func emitObserverFailure(output io.Writer, deadline time.Time) int {
	_ = emitProjection(output, map[string]any{"profile": "corvint-native-hook-observation/0-experimental", "qualification": "UNQUALIFIED", "error": "observer-input-unavailable"}, minTime(deadline, time.Now().Add(100*time.Millisecond)))
	return 1
}

func emitProjection(output io.Writer, value any, deadline time.Time) error {
	raw, err := projectionBytes(value)
	if err != nil {
		return err
	}
	if file, ok := output.(*os.File); ok {
		_ = file.SetWriteDeadline(deadline)
		defer file.SetWriteDeadline(time.Time{})
	}
	for len(raw) > 0 {
		if time.Now().After(deadline) {
			return errors.New("output-deadline")
		}
		written, err := output.Write(raw)
		if err != nil {
			return err
		}
		raw = raw[written:]
	}
	return nil
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}
