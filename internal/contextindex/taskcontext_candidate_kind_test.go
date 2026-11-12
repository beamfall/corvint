package contextindex

import (
	"context"
	"testing"
)

func candidateKindFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":              "module example.test/kinds\n\ngo 1.27.0\n",
		"AGENTS.md":           "Standing instructions.\n",
		"cache/demux.go":      "package cache\n\nfunc Split(key string) string { return key }\n",
		"cache/demux_test.go": "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
		"server/server.go":    "package server\n\nimport \"example.test/kinds/cache\"\n\nfunc Run() string { return cache.Split(\"k\") }\n",
		// An authority-shaped document the task does not concern: a spec that
		// shares only the common word "keys" with the task.
		"docs/specs/billing-v0.md": "# Billing\n\n- `BIL-V0-001`: invoices carry their own keys.\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}
