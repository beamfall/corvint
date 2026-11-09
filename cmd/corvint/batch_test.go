package main

// The batch and query-widening tests left with the retired command families (decision 0594);
// impact_test.go, affected_summary_test.go and impact_range_per_path_test.go still use these
// helpers. The context-injection lane removes this file and keeps the helpers in
// fixture_helpers_test.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// standaloneContextMember returns the "context" member a standalone verb emits,
// as the exact bytes it wrote.
func standaloneContextMember(t *testing.T, root string, arguments ...string) json.RawMessage {
	t.Helper()
	var stdout, stderr bytes.Buffer
	full := append([]string{"--root", root}, arguments...)
	if code := runContext(context.Background(), full, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("standalone %v exit %d: %s", arguments, code, stderr.String())
	}
	var payload struct {
		Context json.RawMessage `json:"context"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode standalone payload: %v: %s", err, stdout.String())
	}
	return payload.Context
}

func commitAll(t *testing.T, root, message string) {
	t.Helper()
	for _, arguments := range [][]string{{"add", "-A"}, {"commit", "-qm", message}} {
		command := fixtureGitCommand(arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}
