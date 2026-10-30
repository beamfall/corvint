package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// DCP-V1-652 (V1-3071, GitHub #1973): started without --protocol-version, the
// server answers a 2025-11-25 initialize with an error naming the version it
// speaks and the flag that selects 2025-11-25, not method-not-found; with the
// flag the same initialize succeeds.
func TestDCPV1652LegacyInitializeWithoutFlagNamesIt(t *testing.T) {
	t.Parallel()
	root := corpusRoot(t)
	initialize := `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"test"}}}` + "\n"
	answer := func(arguments ...string) map[string]any {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), append([]string{"--root", root, "--artifact", "corpus.json"}, arguments...), strings.NewReader(initialize), &stdout, &stderr); exit != 0 {
			t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
		}
		var response map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &response); err != nil {
			t.Fatalf("%v: %q", err, stdout.String())
		}
		return response
	}
	refused, _ := answer()["error"].(map[string]any)
	message, _ := refused["message"].(string)
	if refused["code"] != float64(-32602) || !strings.Contains(message, "2026-07-28") || !strings.Contains(message, "--protocol-version 2025-11-25") {
		t.Fatalf("error = %v", refused)
	}
	if data, _ := refused["data"].(map[string]any); data["flag"] != "--protocol-version 2025-11-25" || data["requested"] != "2025-11-25" {
		t.Fatalf("data = %v", refused["data"])
	}
	if result, _ := answer("--protocol-version", "2025-11-25")["result"].(map[string]any); result["protocolVersion"] != "2025-11-25" {
		t.Fatalf("legacy initialize with the flag = %v", result)
	}
}
