package contextindex

import (
	"context"
	"testing"
)

// specCitedFixture is a spec-routed implementation task's repository in
// miniature (V1-1269): the spec cites a command verb, a make target, a tool
// directory, a library file and a sibling document, none of which shares
// the task's words, while lexical decoys do.
func specCitedFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":    "module example.test/cited\n\ngo 1.27.0\n",
		"AGENTS.md": "Standing instructions.\n",
		"Makefile":  "gate:\n\tgo test ./...\n",
		"docs/specs/runner-v0.md": "# Runner\n\nExists: `tool plan --base`, `tools/select`, `make gate`.\n\n" +
			"- `RUN-V0-001`: the runner reuses `tool plan` and `make gate` through `tools/select/`.\n" +
			"- `RUN-V0-002`: `lib/kind.go:12` types results; see `docs/other.md` and `missing/path.go`.\n" +
			"- `RUN-V0-003`: `tool plan` again, and `lib/extra.go`.\n",
		"docs/other.md":         "# Other\n",
		"cmd/tool/plan.go":      "package main\n\nfunc plan() {}\n",
		"tools/select/main.go":  "package main\n\nfunc main() {}\n",
		"lib/kind.go":           "package lib\n\nfunc Kind() {}\n",
		"lib/extra.go":          "package lib\n\nfunc Extra() {}\n",
		"decoy/runner_tests.go": "package decoy\n\n// implement the runner tests once and type each result\nfunc Run() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

const specCitedTask = "Implement the runner per RUN-V0-001: run the selected tests once and type each result."
