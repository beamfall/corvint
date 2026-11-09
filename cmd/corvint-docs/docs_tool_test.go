// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/docsenvelope"
)

func runDocsToolVerb(t *testing.T, root, tool, arguments string) (int, docsenvelope.ToolResult, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{toolVerb, "--root", root, tool}, strings.NewReader(arguments), &stdout, &stderr)
	var result docsenvelope.ToolResult
	if code == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("docs-tool stdout is not one envelope: %v %q", err, &stdout)
		}
	}
	return code, result, stderr.String()
}

// The docs-tool verb answers a draft with the bytes `docs draft` prints, inside one
// corvint-docs-tool/1 envelope, and carries every refusal inside the envelope with exit 0.
func TestDocsToolVerbAnswersInOneEnvelope(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	code, draft, stderr := runDocsFixture(t, root, "draft", "")
	if code != 0 {
		t.Fatalf("draft exit %d: %s", code, stderr)
	}
	code, result, stderr := runDocsToolVerb(t, root, docsenvelope.ToolDraft, `{"source":"owner.md","package":"cache"}`)
	if code != 0 || stderr != "" || result.Schema != docsenvelope.ToolSchema || result.Tool != docsenvelope.ToolDraft || result.Failure != nil || result.Error != nil {
		t.Fatalf("exit %d stderr %q result %+v", code, stderr, result)
	}
	if result.Text != draft || result.Structured["state"] != "READY" || result.Structured["markdown"] != draft {
		t.Fatalf("envelope text differs from the CLI draft: %q vs %q", result.Text, draft)
	}
	code, result, _ = runDocsToolVerb(t, root, docsenvelope.ToolDraft, `{"source":"owner.md","package":"cache","bogus":1}`)
	if code != 0 || result.Failure == nil || result.Failure.Code != "invalid-arguments" || result.Structured != nil {
		t.Fatalf("malformed arguments: exit %d %+v", code, result)
	}
	code, result, _ = runDocsToolVerb(t, root, docsenvelope.ToolConsume, `{"source":"owner.md","package":"cache","task":"Cache","draft":"tampered"}`)
	if code != 0 || result.Failure == nil || result.Failure.Code != "stale-documentation-draft" {
		t.Fatalf("tampered consume: exit %d %+v", code, result)
	}
	code, result, _ = runDocsToolVerb(t, t.TempDir(), docsenvelope.ToolDraft, `{"source":"owner.md","package":"cache"}`)
	if code != 0 || result.Error == nil || result.Error.Code != "invalid-root" {
		t.Fatalf("unbound root: exit %d %+v", code, result)
	}
	code, result, _ = runDocsToolVerb(t, root, docsenvelope.ToolDraft, strings.Repeat("{", docsenvelope.MaxArgumentBytes+1))
	if code != 0 || result.Error == nil || result.Error.Code != "invalid-arguments" {
		t.Fatalf("oversized arguments: exit %d %+v", code, result)
	}
}

// A malformed docs-tool invocation is the conventional invalid-arguments refusal, exit 2.
func TestDocsToolVerbUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{toolVerb}, {toolVerb, "--root", "/r"}, {toolVerb, "--root", "/r", "corvint.other"}, {toolVerb, "/r", "corvint.docs_draft"}} {
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), args, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code": "invalid-arguments"`) {
			t.Errorf("%q: exit %d stdout %q stderr %q", args, code, &stdout, &stderr)
		}
	}
}
