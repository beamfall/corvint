// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

// MTT-V1-005: asking corvint mcp for the moved docs or corpus group, or for
// --corpus-artifact, refuses before any MCP byte with a typed reason naming
// corvint-docs mcp; the removed test-validity group refuses the same way.
func TestMTTV1005RemovedGroupsRefuseWithATypedReason(t *testing.T) {
	root, _, _ := followRootsFixture(t)
	for _, tc := range []struct {
		arguments []string
		code      string
		names     string
	}{
		{[]string{"--root", root, "--tool-profile", "docs"}, "profile-moved", "corvint-docs mcp"},
		{[]string{"--root", root, "--tool-profile", "flows,corpus"}, "profile-moved", "corvint-docs mcp --root ROOT --corpus-artifact PATH"},
		{[]string{"--root", root, "--corpus-artifact", "corpus.json"}, "profile-moved", "corvint-docs mcp"},
		{[]string{"--root", root, "--tool-profile", "test-validity"}, "group-removed", "corvint test-validity"},
	} {
		var stdout, stderr bytes.Buffer
		exit := run(context.Background(), tc.arguments, strings.NewReader(""), &stdout, &stderr)
		if exit != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "corvint-mcp: invalid arguments: "+tc.code+": ") || !strings.Contains(stderr.String(), tc.names) {
			t.Fatalf("%q: exit=%d stdout=%q stderr=%q", tc.arguments, exit, stdout.String(), stderr.String())
		}
	}
}

// MTT-V1-005 scenario: with every group it keeps, corvint mcp lists no
// test_validity and no docs tool, labels test_evidence preview, and a call to
// corvint.test_validity is the unknown-tool error.
func TestMTTV1005TestValidityIsAnUnknownTool(t *testing.T) {
	root, _, _ := followRootsFixture(t)
	tools := listedWith(t, []string{"--root", root, "--tool-profile", coreDriftProfile})
	for _, tool := range tools {
		name := tool["name"].(string)
		if name == "corvint.test_validity" || strings.HasPrefix(name, "corvint.docs_") {
			t.Fatalf("corvint mcp lists %s", name)
		}
		if name == "corvint.test_evidence" && !strings.HasPrefix(tool["description"].(string), "Preview: ") {
			t.Fatalf("test_evidence is not labelled preview: %q", tool["description"])
		}
	}
	if !slices.Contains(toolNames(tools), "corvint.test_evidence") {
		t.Fatal("test_evidence not listed")
	}
	exit, responses, stderr := hookedSession(t, Hooks{Affected: stubAffectedRunner}, []string{"--root", root, "--tool-profile", coreDriftProfile},
		toolCall(1, "corvint.test_validity", map[string]any{}))
	rpc, _ := responses[1]["error"].(map[string]any)
	if exit != 0 || stderr != "" || rpc["code"] != float64(-32602) {
		t.Fatalf("exit=%d stderr=%q response=%#v", exit, stderr, responses[1])
	}
}
