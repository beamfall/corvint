package contextindex

import (
	"context"
	"encoding/json"
	"testing"
)

// TCP-V0-152 (V1-1010, V1-1015, V1-1285): anchors no evidence resolves are
// disclosed as uncertainty; a defined id or declared name is not listed.
func TestTaskContextDisclosesUnresolvedIDAnchors(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":             "module example.test/anchors\n\ngo 1.27.0\n",
		"main.go":            "package main\n\n// ExpectedTests is filled by a later change.\nfunc MintToken() {}\n",
		"docs/specs/demo.md": "# Demo\n\n- `DEMO-V0-001`: MintToken mints one token.\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := TaskContext(context.Background(), index, "Implement DEMO-V0-001 and DEMO-V0-002 for ticket V1-0523: MintToken fills ExpectedTests", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	spec := false
	for _, value := range packet["results"].([]any) {
		row := value.(map[string]any)
		spec = spec || row["kind"] == specMentionedRelation && row["id"] == "docs/specs/demo.md"
	}
	if !spec {
		t.Fatalf("the defined id did not route to its spec: %v", packet["results"])
	}
	got, _ := json.Marshal(packet["coverage"].(map[string]any)["unresolved_anchors"])
	want := `{"count":3,"samples":[{"anchor":"DEMO-V0-002","reason":"` + undefinedRequirementReason + `"},{"anchor":"ExpectedTests","reason":"` + undeclaredAnchorReason + `"},{"anchor":"V1-0523","reason":"` + ticketAnchorReason + `"}]}`
	if string(got) != want {
		t.Fatalf("unresolved_anchors = %s\nwant %s", got, want)
	}
	plain, err := TaskContext(context.Background(), index, "change MintToken", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := plain["coverage"].(map[string]any)["unresolved_anchors"]; present {
		t.Fatal("a task with no id anchor must not carry the member")
	}
}
