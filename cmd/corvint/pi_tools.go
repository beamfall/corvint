package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

const piToolOutputLimit = 65536

func piToolEnvelope(operation, version, fault any) map[string]any {
	return map[string]any{"profile": "corvint-pi-tool/0", "operation": operation, "hostVersion": version, "adapterVersion": piAdapterVersion, "support": "FALLBACK", "ok": false, "mutation": "not-attempted", "context": "", "packet": nil, "fault": fault}
}

func runPiTool(parent context.Context, args []string, stdin io.Reader, stdout io.Writer) int {
	var operation any
	if len(args) == 1 && (args[0] == "context" || args[0] == "expand" || args[0] == "record") {
		operation = args[0]
	}
	result := piToolEnvelope(operation, nil, "unsupported-operation")
	if operation != nil {
		ctx, cancel := context.WithTimeout(parent, 1100*time.Millisecond)
		defer cancel()
		result = piToolResult(ctx, operation.(string), stdin)
	}
	raw, err := json.Marshal(result)
	if err != nil || len(raw)+1 > piToolOutputLimit {
		failed := piToolEnvelope(operation, result["hostVersion"], "output-too-large")
		failed["mutation"] = result["mutation"]
		raw, _ = json.Marshal(failed)
	}
	_, err = stdout.Write(append(raw, '\n'))
	if err != nil {
		return 2
	}
	return 0
}

func piToolResult(ctx context.Context, operation string, stdin io.Reader) map[string]any {
	result := piToolEnvelope(operation, nil, "invalid-input")
	raw, err := io.ReadAll(io.LimitReader(stdin, piInputLimit+1))
	if err != nil || len(raw) > piInputLimit || !utf8.Valid(raw) {
		return result
	}
	request, err := decodeAdapterJSON(raw)
	if err != nil || len(request) != 2 {
		return result
	}
	version, ok := request["hostVersion"].(string)
	if !ok {
		return result
	}
	if version != piHostVersion {
		result["fault"] = "unsupported-host-version"
		return result
	}
	result["hostVersion"] = version
	input, ok := request["input"].(map[string]any)
	if !ok || !piNoNull(input) {
		return result
	}
	root, err := os.Getwd()
	if err != nil {
		result["fault"] = "core-unavailable"
		return result
	}
	var payload any
	switch operation {
	case "context":
		task, ok := input["task"].(string)
		if !ok || len(input) != 1 || strings.TrimSpace(task) == "" || utf8.RuneCountInString(task) > 2000 || len(task) > 16384 {
			return result
		}
		commit, readErr := sourceGitText(ctx, root, 8192, "rev-parse", "HEAD")
		if readErr != nil {
			result["fault"] = "core-unavailable"
			return result
		}
		var out, stderr bytes.Buffer
		status := runContext(ctx, []string{"--root", root, "context", "--task", task, "--limit", "5"}, bytes.NewReader(nil), &out, &stderr)
		packet, readErr := decodeAdapterJSON(out.Bytes())
		if status != 0 || readErr != nil || out.Len() > sourcePacketLimit {
			result["fault"] = "context-unavailable"
			return result
		}
		current, readErr := sourceGitText(ctx, root, 8192, "rev-parse", "HEAD")
		tree, treeErr := sourceGitText(ctx, root, 8192, "rev-parse", commit+"^{tree}")
		if readErr != nil || treeErr != nil || current != commit || packet["revision"] != tree {
			result["fault"] = "stale-context"
			return result
		}
		sum := sha256.Sum256(out.Bytes())
		result["packet"] = map[string]any{"json": out.String(), "sha256": hex.EncodeToString(sum[:]), "commit": commit, "evidenceHandle": "context-packet:sha256:" + hex.EncodeToString(sum[:])}
		payload = packet
	case "expand":
		allowed := map[string]bool{"packet": true, "packetSha256": true, "commit": true, "result": true, "evidence": true, "lines": true, "requirement": true, "maxBytes": true}
		for key := range input {
			if !allowed[key] {
				return result
			}
		}
		packet, packetOK := input["packet"].(string)
		digest, digestOK := input["packetSha256"].(string)
		commit, commitOK := input["commit"].(string)
		row, rowOK := piToolIndex(input["result"])
		evidence, evidenceOK := piToolIndex(input["evidence"])
		lines, linesOK := input["lines"].(string)
		requirement, requirementOK := input["requirement"].(string)
		if _, exists := input["lines"]; exists && !linesOK {
			return result
		}
		if _, exists := input["requirement"]; exists && !requirementOK {
			return result
		}
		maxBytes := 2048
		if value, exists := input["maxBytes"]; exists {
			var valid bool
			maxBytes, valid = piToolIndex(value)
			if !valid || maxBytes < 1 || maxBytes > 8192 {
				return result
			}
		}
		if !packetOK || !digestOK || !commitOK || !rowOK || !evidenceOK || !sourceHex(digest, 64) || (linesOK == requirementOK) || (linesOK && lines == "") || (requirementOK && requirement == "") {
			return result
		}
		view := map[string]any{"profile": "experimental-source-view/0", "ok": false, "mutates": false, "selector_complete": false, "task_evidence_complete": "UNKNOWN", "packet_metadata": nil}
		view, err = executeSourceViewBytes(ctx, sourceViewOptions{root: root, digest: digest, commit: commit, result: row, evidence: evidence, lines: lines, requirement: requirement, maxBytes: maxBytes}, view, []byte(packet))
		if err != nil {
			result["fault"] = "source-unavailable"
			return result
		}
		payload = view
	case "record":
		allowed := map[string]bool{"task": true, "openedPaths": true, "changedPaths": true, "verification": true, "outcome": true}
		for key := range input {
			if !allowed[key] {
				return result
			}
		}
		task, taskOK := input["task"].(string)
		outcome, outcomeOK := input["outcome"].(string)
		opened, openedOK := piToolStrings(input["openedPaths"], false)
		changed, changedOK := piToolStrings(input["changedPaths"], true)
		verification, verificationOK := piToolStrings(input["verification"], true)
		if !taskOK || !outcomeOK || !openedOK || !changedOK || !verificationOK || strings.TrimSpace(task) == "" || len(task) > 16384 || utf8.RuneCountInString(task) > 2000 || (outcome != "passed" && outcome != "failed" && outcome != "blocked") {
			return result
		}
		// A cancellation or write error can occur after atomic publication. Never
		// claim the store was unchanged without a successful core receipt.
		result["mutation"] = "unknown"
		recorded, recordErr := tracerecordrepo.Record(ctx, root, tracerecordrepo.Input{Task: task, OpenedPaths: opened, ChangedPaths: changed, Verification: verification, Outcome: outcome})
		if recordErr != nil {
			result["fault"] = "record-unavailable"
			return result
		}
		result["mutation"] = "recorded"
		payload = map[string]any{"profile": "corvint-explicit-record/0", "traceId": recorded.Record.TraceID, "revision": recorded.Record.Revision, "outcome": outcome, "provenance": "caller-reported", "truncatedAncestry": recorded.TruncatedAncestry}
	default:
		return result
	}
	raw, err = json.Marshal(payload)
	if err != nil {
		result["fault"] = "invalid-core-response"
		return result
	}
	framed, err := repoenvelope.Frame(string(raw))
	if err != nil {
		result["packet"] = nil
		result["fault"] = "invalid-core-response"
		return result
	}
	result["ok"], result["fault"], result["context"] = true, nil, framed
	return result
}

func piToolIndex(v any) (int, bool) {
	number, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	n, err := number.Int64()
	return int(n), err == nil && n >= 0 && n <= 65536
}
func piToolStrings(v any, required bool) ([]string, bool) {
	if v == nil {
		return nil, !required
	}
	values, ok := v.([]any)
	if !ok || len(values) > 256 || (required && len(values) == 0) {
		return nil, false
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		s, ok := value.(string)
		if !ok || s == "" || len(s) > 4096 {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}
