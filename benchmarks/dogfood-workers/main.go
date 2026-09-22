// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const unknown = "NOT_OBSERVED"
const maxTranscript = 64 << 20

var metricFields = []string{"turns", "inputTokens", "cacheCreationTokens", "cacheReadTokens", "outputTokens", "sourceOpens", "broadSearches", "compactions", "retries", "unparsedLines"}
var claudeFields = []string{"input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens", "output_tokens"}
var codexFields = []string{"input_tokens", "cached_input_tokens", "output_tokens"}

type object = map[string]any

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dogfood-workers:", err)
		os.Exit(2)
	}
}
func run(args []string) error {
	flags := flag.NewFlagSet("dogfood-workers", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "", "worker manifest")
	output := flags.String("output", "", "receipt destination")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *manifestPath == "" || *output == "" || flags.NArg() != 0 {
		return errors.New("--manifest and --output are required")
	}
	manifest, raw, err := loadManifest(*manifestPath)
	if err != nil {
		return err
	}
	receipt, err := buildReceipt(manifest, raw)
	if err != nil {
		return err
	}
	return writeReceipt(*output, receipt)
}
func decode(raw []byte) (object, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value object
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, errors.New("expected object")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	return value, nil
}
func asObject(value any) object { result, _ := value.(map[string]any); return result }
func text(value any) string     { result, _ := value.(string); return result }
func boundedText(value any) bool {
	s, ok := value.(string)
	return ok && s != "" && utf8.ValidString(s) && len(s) <= 4096
}
func member(value any, choices ...string) bool {
	for _, s := range choices {
		if value == s {
			return true
		}
	}
	return false
}
func keys(value object, required []string, optional ...string) bool {
	if value == nil {
		return false
	}
	allowed := map[string]bool{}
	for _, key := range required {
		if _, ok := value[key]; !ok {
			return false
		}
		allowed[key] = true
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range value {
		if !allowed[key] {
			return false
		}
	}
	return true
}
func boundedFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("must be a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, errors.New("file changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("exceeds %d bytes", limit)
	}
	return raw, nil
}
func resolvedPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return abs, nil
	}
	resolved, err = resolvedPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(abs)), nil
}
func loadManifest(path string) (object, []byte, error) {
	path, err := resolvedPath(path)
	if err != nil {
		return nil, nil, err
	}
	raw, err := boundedFile(path, 1<<20)
	if err != nil {
		return nil, nil, fmt.Errorf("workers manifest: %w", err)
	}
	if _, err := wire.Parse(raw); err != nil {
		return nil, nil, fmt.Errorf("workers manifest is invalid JSON: %w", err)
	}
	doc, err := decode(raw)
	if err != nil {
		return nil, nil, err
	}
	if !keys(doc, []string{"profile", "task", "pricingVersion", "workers"}) {
		return nil, nil, errors.New("workers manifest has invalid fields")
	}
	if doc["profile"] != "corvint-dogfood-workers-manifest/0" || !boundedText(doc["task"]) || !boundedText(doc["pricingVersion"]) {
		return nil, nil, errors.New("workers manifest has invalid identity")
	}
	workers, ok := doc["workers"].([]any)
	if !ok || len(workers) < 1 || len(workers) > 256 {
		return nil, nil, errors.New("workers must contain 1 to 256 entries")
	}
	ids, paths := map[string]bool{}, map[string]bool{}
	for i, rawWorker := range workers {
		w := asObject(rawWorker)
		if !keys(w, []string{"id", "role", "host", "status", "model", "transcript", "solved"}, "ticket") || !boundedText(w["id"]) || !boundedText(w["model"]) {
			return nil, nil, fmt.Errorf("worker[%d] has invalid fields", i)
		}
		if ids[text(w["id"])] {
			return nil, nil, errors.New("worker ids must be distinct")
		}
		ids[text(w["id"])] = true
		if !member(w["role"], "coordinator", "builder", "reviewer", "repair") || !member(w["host"], "claude-code", "codex", "atm") || !member(w["status"], "completed", "failed", "cancelled") {
			return nil, nil, fmt.Errorf("worker[%d] role, host or status unsupported", i)
		}
		if _, ok := w["solved"].(bool); !ok && w["solved"] != unknown {
			return nil, nil, errors.New("solved must be boolean or NOT_OBSERVED")
		}
		if w["ticket"] != nil && !boundedText(w["ticket"]) {
			return nil, nil, errors.New("ticket must be bounded non-empty text")
		}
		if w["transcript"] != nil {
			if !boundedText(w["transcript"]) {
				return nil, nil, errors.New("invalid transcript path")
			}
			p, err := resolvedPath(text(w["transcript"]))
			if err != nil {
				return nil, nil, err
			}
			if paths[p] {
				return nil, nil, errors.New("transcript path is listed for more than one worker")
			}
			paths[p] = true
		}
	}
	return doc, raw, nil
}

// Exact integers preserve unknown propagation without overflow or float rounding.
func number(value any) any {
	var s string
	switch v := value.(type) {
	case json.Number:
		s = string(v)
	case int:
		if v >= 0 {
			return json.Number(fmt.Sprint(v))
		}
	case string:
		return unknown
	}
	if s == "" || strings.ContainsAny(s, ".eE+-") {
		return unknown
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.Sign() < 0 {
		return unknown
	}
	return json.Number(n.String())
}
func add(a, b any) any {
	a, b = number(a), number(b)
	if a == unknown || b == unknown {
		return unknown
	}
	left, _ := new(big.Int).SetString(string(a.(json.Number)), 10)
	right, _ := new(big.Int).SetString(string(b.(json.Number)), 10)
	return json.Number(left.Add(left, right).String())
}
func cumulative(previous, usage object, fields []string) object {
	result := object{}
	for _, field := range fields {
		value := number(usage[field])
		prior := any(0)
		if previous != nil {
			prior = previous[field]
		}
		prior = number(prior)
		if value == unknown || prior == unknown {
			result[field] = unknown
			continue
		}
		left, _ := new(big.Int).SetString(string(value.(json.Number)), 10)
		right, _ := new(big.Int).SetString(string(prior.(json.Number)), 10)
		if left.Cmp(right) < 0 {
			result[field] = unknown
		} else {
			result[field] = value
		}
	}
	return result
}
func emptyMetrics(value any) object {
	result := object{}
	for _, field := range metricFields {
		result[field] = value
	}
	return result
}
func increment(metrics object, key string) { metrics[key] = add(metrics[key], 1) }
func transcriptRecords(raw []byte, visit func(object)) int {
	unparsed := 0
	for _, line := range bytes.Split(bytes.ToValidUTF8(raw, []byte("�")), []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		record, err := decode(line)
		if err != nil {
			unparsed++
			continue
		}
		visit(record)
	}
	return unparsed
}
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case json.Number:
		return v != "0"
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return true
}
func classifyTool(block, metrics object) {
	switch block["name"] {
	case "Read":
		increment(metrics, "sourceOpens")
	case "Grep", "Glob":
		increment(metrics, "broadSearches")
	case "Bash":
		command := text(asObject(block["input"])["command"])
		for _, prefix := range []string{"cat ", "sed -n", "head ", "tail "} {
			if strings.HasPrefix(command, prefix) {
				increment(metrics, "sourceOpens")
				break
			}
		}
		for _, marker := range []string{"rg ", "grep -r", "find "} {
			if strings.Contains(command, marker) {
				increment(metrics, "broadSearches")
				break
			}
		}
	}
}
func parseClaude(raw []byte) object {
	metrics := [2]object{emptyMetrics(0), emptyMetrics(0)}
	delete(metrics[1], "unparsedLines")
	usages := [2]map[string]object{{}, {}}
	requests := [2]map[string]bool{{}, {}}
	anonymous, present := [2]bool{}, [2]bool{}
	seenTools := map[string]bool{}
	unparsed := transcriptRecords(raw, func(record object) {
		side := 0
		if truthy(record["isSidechain"]) {
			side = 1
		}
		if record["isCompactSummary"] == true || record["subtype"] == "compact_boundary" {
			increment(metrics[side], "compactions")
		}
		if record["type"] != "assistant" {
			return
		}
		present[side] = true
		message := asObject(record["message"])
		usage := asObject(message["usage"])
		request := record["requestId"]
		if truthy(usage["requestId"]) {
			request = usage["requestId"]
		}
		if id, ok := request.(string); ok && id != "" {
			requests[side][id] = true
			if usage != nil {
				usages[side][id] = cumulative(usages[side][id], usage, claudeFields)
			}
		} else {
			anonymous[side] = true
		}
		blocks, _ := message["content"].([]any)
		for _, value := range blocks {
			block := asObject(value)
			if block["type"] != "tool_use" {
				continue
			}
			if id, ok := block["id"].(string); ok {
				if seenTools[id] {
					continue
				}
				seenTools[id] = true
			}
			classifyTool(block, metrics[side])
		}
	})
	for side := range 2 {
		metrics[side]["turns"] = len(requests[side])
		if anonymous[side] {
			metrics[side]["turns"] = unknown
		}
		incomplete := anonymous[side]
		for id := range requests[side] {
			if usages[side][id] == nil {
				incomplete = true
			}
		}
		for i, field := range claudeFields {
			var sum any = 0
			for _, usage := range usages[side] {
				sum = add(sum, usage[field])
			}
			if incomplete {
				sum = unknown
			}
			metrics[side][[]string{"inputTokens", "cacheCreationTokens", "cacheReadTokens", "outputTokens"}[i]] = sum
		}
		metrics[side]["retries"] = unknown
	}
	metrics[0]["unparsedLines"] = unparsed
	if !present[0] && present[1] {
		metrics[1]["unparsedLines"] = unparsed
		metrics[1]["sidechain"] = nil
		return metrics[1]
	}
	if !present[0] && !present[1] {
		result := emptyMetrics(unknown)
		result["unparsedLines"] = unparsed
		result["sidechain"] = nil
		return result
	}
	metrics[0]["sidechain"] = metrics[1]
	return metrics[0]
}
func parseCodex(raw []byte) object {
	metrics := emptyMetrics(unknown)
	turns := 0
	sawTurn, sawDelta, tail := false, false, false
	var snapshot object
	deltas := object{"input_tokens": 0, "cached_input_tokens": 0, "output_tokens": 0}
	unparsed := transcriptRecords(raw, func(record object) {
		container := asObject(record["msg"])
		if container == nil {
			container = record
		}
		kind := record["type"]
		if !truthy(kind) {
			kind = container["type"]
		}
		if member(kind, "turn.completed", "task_complete", "agent_message") {
			turns++
			sawTurn = true
		}
		if usage := asObject(container["total_token_usage"]); usage != nil {
			snapshot = cumulative(snapshot, usage, codexFields)
			tail = false
			return
		}
		usage := asObject(container["last_token_usage"])
		if usage == nil {
			usage = asObject(container["usage"])
		}
		if usage != nil {
			sawDelta = true
			tail = snapshot != nil
			for _, field := range codexFields {
				deltas[field] = add(deltas[field], number(usage[field]))
			}
		}
	})
	metrics["unparsedLines"] = unparsed
	if sawTurn {
		metrics["turns"] = turns
	}
	totals := object{}
	if snapshot != nil && !tail {
		totals = snapshot
	} else if snapshot == nil && sawDelta {
		totals = deltas
	}
	for i, field := range codexFields {
		value := totals[field]
		if value == nil {
			value = unknown
		}
		metrics[[]string{"inputTokens", "cacheReadTokens", "outputTokens"}[i]] = value
	}
	return metrics
}
func workerMetrics(worker object) (object, error) {
	missing := func() object {
		m := emptyMetrics(unknown)
		m["transcriptSha256"] = unknown
		if worker["host"] == "claude-code" {
			m["sidechain"] = unknown
		}
		return m
	}
	if worker["transcript"] == nil {
		return missing(), nil
	}
	path := text(worker["transcript"])
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return missing(), nil
	}
	raw, err := boundedFile(path, maxTranscript)
	if err != nil {
		return nil, fmt.Errorf("transcript for worker %s: %w", worker["id"], err)
	}
	var result object
	if worker["host"] == "claude-code" {
		result = parseClaude(raw)
	} else {
		result = parseCodex(raw)
	}
	result["transcriptSha256"] = fmt.Sprintf("%x", sha256.Sum256(raw))
	return result, nil
}
func buildReceipt(manifest object, raw []byte) (object, error) {
	workers := []any{}
	totals := emptyMetrics(0)
	for _, key := range []string{"observedWorkers", "cancelledWorkers", "failedWorkers", "solvedWorkers"} {
		totals[key] = 0
	}
	for _, value := range manifest["workers"].([]any) {
		worker := asObject(value)
		entry, err := workerMetrics(worker)
		if err != nil {
			return nil, err
		}
		for _, key := range []string{"id", "role", "host", "status", "model", "solved"} {
			entry[key] = worker[key]
		}
		if worker["ticket"] != nil {
			entry["ticket"] = worker["ticket"]
		}
		for _, field := range metricFields {
			totals[field] = add(totals[field], entry[field])
		}
		if entry["transcriptSha256"] != unknown {
			increment(totals, "observedWorkers")
		}
		if entry["status"] == "cancelled" {
			increment(totals, "cancelledWorkers")
		}
		if entry["status"] == "failed" {
			increment(totals, "failedWorkers")
		}
		if entry["solved"] == unknown {
			totals["solvedWorkers"] = unknown
		} else if entry["solved"] == true {
			increment(totals, "solvedWorkers")
		}
		workers = append(workers, entry)
	}
	return object{"profile": "corvint-dogfood-workers/0", "task": manifest["task"], "pricingVersion": manifest["pricingVersion"], "manifestSha256": fmt.Sprintf("%x", sha256.Sum256(raw)), "workers": workers, "totals": totals, "cost": unknown}, nil
}
func writeReceipt(path string, receipt object) error {
	path, err := resolvedPath(path)
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(receipt); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".dogfood-workers-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(buffer.Bytes()); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
