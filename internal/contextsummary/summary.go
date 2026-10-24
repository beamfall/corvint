// Package contextsummary is the experimental evidence summary of one context
// packet (ESV-V0-008, decision 0364), shared by `corvint context --summary`
// and the MCP corvint.context summaryBytes argument (MCPV0-111, V1-1003), so
// both read one projection of the unchanged context wire.
package contextsummary

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	// View names the summary projection.
	View = "experimental-evidence-summary/0"
	// DefaultBytes, MinBytes and MaxBytes bound a summary, LF included.
	DefaultBytes = 8192
	MinBytes     = 1024
	MaxBytes     = 65536
	// HandlePrefix opens a `cv1:TREE:BLOB:RANGE:PATH` evidence handle.
	HandlePrefix = "cv1:"
	// CLIContinuation is the continuation member `corvint context --summary` emits.
	CLIContinuation = "rerun this invocation without --summary and --summary-bytes: its stdout has packet_sha256, and results[results_shown:] are the omitted rows"
	// MCPContinuation is the continuation member corvint.context emits under summaryBytes.
	MCPContinuation = "call corvint.context again without summaryBytes: the canonical receipt plus LF has packet_sha256, and results[results_shown:] are the omitted rows"
)

// Summarize projects the exact canonical bytes (with their LF) of one
// context packet into a summary of at most budget bytes including its LF
// (ESV-V0-008). Every top-level member except results is kept verbatim, so
// coverage, subject, uncertainty and critical omissions survive; result rows
// keep identity, authority, freshness and a handle, in packet order, until the
// budget. A budget that cannot hold the fixed members and every critical row
// refuses rather than dropping one.
func Summarize(raw []byte, budget int, continuation string) ([]byte, error) {
	if budget < MinBytes || budget > MaxBytes {
		return nil, &gokernel.Error{Code: "summary-budget", Message: fmt.Sprintf("--summary-bytes must be an integer from %d to %d", MinBytes, MaxBytes)}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var packet map[string]any
	if err := decoder.Decode(&packet); err != nil {
		return nil, &gokernel.Error{Code: "summary-shape", Message: "the context packet does not decode as one JSON object"}
	}
	rows, critical, err := summaryRows(packet)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	summary := map[string]any{
		"view":               View,
		"packet_sha256":      hex.EncodeToString(sum[:]),
		"packet_bytes":       len(raw),
		"results_total":      len(rows),
		"budget_bytes":       budget,
		"evidence_complete":  "UNKNOWN",
		"row_fields_omitted": []string{"action", "evidence", "score", "summary"},
		"continuation":       continuation,
		"expand":             "corvint [--root PATH] context --expand HANDLE [--max-bytes N]",
	}
	packet["summary"] = summary
	fitted, shown := []byte(nil), -1
	for count := 0; count <= len(rows); count++ {
		encoded, err := encode(packet, summary, rows, count)
		if err != nil {
			return nil, err
		}
		if len(encoded)+1 > budget {
			break
		}
		fitted, shown = encoded, count
	}
	if shown >= critical {
		return fitted, nil
	}
	needed, err := encode(packet, summary, rows, critical)
	if err != nil {
		return nil, err
	}
	return nil, &gokernel.Error{Code: "summary-budget", Message: fmt.Sprintf("%d bytes cannot hold the fixed members and the %d leading rows through the last critical row, which need %d; raise --summary-bytes or read the full packet", budget, critical, len(needed)+1)}
}

// encode encodes the summary showing the first count rows.
func encode(packet, summary map[string]any, rows []any, count int) ([]byte, error) {
	summary["results_shown"], summary["results_omitted"] = count, len(rows)-count
	packet["results"] = rows[:count]
	return gokernel.CanonicalJSON(packet)
}

// summaryRows returns the compact rows and the number of leading rows
// that must be shown to include every coverage.critical row (0 when none).
func summaryRows(packet map[string]any) ([]any, int, error) {
	if _, taken := packet["summary"]; taken || packet["tool"] != "context" {
		return nil, 0, &gokernel.Error{Code: "summary-shape", Message: "the input is not a context packet this summary can extend"}
	}
	results, _ := packet["results"].([]any)
	revision, _ := packet["revision"].(string)
	critical := criticalSet(packet)
	rows := make([]any, 0, len(results))
	required := 0
	for index, value := range results {
		item, _ := value.(map[string]any)
		evidence, _ := item["evidence"].([]any)
		if len(evidence) != 1 {
			return nil, 0, &gokernel.Error{Code: "summary-shape", Message: fmt.Sprintf("results[%d] does not carry exactly one evidence row", index)}
		}
		row := row(index, item, evidence[0], revision)
		if critical[fmt.Sprint(item["kind"])+"\x00"+fmt.Sprint(item["id"])] {
			row["critical"], required = true, index+1
		}
		rows = append(rows, row)
	}
	return rows, required, nil
}

func criticalSet(packet map[string]any) map[string]bool {
	coverage, _ := packet["coverage"].(map[string]any)
	entries, _ := coverage["critical"].([]any)
	critical := map[string]bool{}
	for _, value := range entries {
		entry, _ := value.(map[string]any)
		critical[fmt.Sprint(entry["relation"])+"\x00"+fmt.Sprint(entry["path"])] = true
	}
	return critical
}

func row(index int, item map[string]any, value any, revision string) map[string]any {
	evidence, _ := value.(map[string]any)
	row := map[string]any{"index": index, "kind": item["kind"], "id": item["id"], "handle": nil}
	for _, field := range []string{"authority", "trust", "confidence", "reason", "line", "blob_hash", "evidence_gap"} {
		if member, ok := evidence[field]; ok {
			row[field] = member
		}
	}
	path, _ := evidence["path"].(string)
	blob, _ := evidence["blob_hash"].(string)
	if revision != "" && blob != "" && SafePath(path) {
		row["handle"] = HandlePrefix + revision + ":" + blob + ":all:" + path
	}
	return row
}

// SafePath reports whether value is a relative POSIX path without empty, dot
// or dot-dot components, backslashes, controls or a leading dash, of at most
// 4096 bytes: the source view's safe path, which a handle may name.
func SafePath(value any) bool {
	path, ok := value.(string)
	if !ok || path == "" || len([]byte(path)) > 4096 || strings.HasPrefix(path, "-") || strings.Contains(path, "\\") || filepath.IsAbs(path) {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	for _, char := range path {
		if char < 32 || char == 127 {
			return false
		}
	}
	return true
}
