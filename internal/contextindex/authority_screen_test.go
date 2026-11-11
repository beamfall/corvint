package contextindex

import (
	"testing"
)

func screenFixture(t *testing.T, agents string, extra map[string]string) *Index {
	t.Helper()
	files := map[string]string{
		"go.mod":         "module example.test/screen\n\ngo 1.27.0\n",
		"AGENTS.md":      agents,
		"docs/STYLE.md":  "# Style\n\nWrap at 100 columns.\n",
		"cache/demux.go": "package cache\n\nfunc Split(key string) string { return key }\n",
	}
	for path, content := range extra {
		files[path] = content
	}
	return routedIndex(t, files)
}

func packetRow(t *testing.T, packet map[string]any, kind, path string) (map[string]any, map[string]any) {
	t.Helper()
	for _, row := range mapsFromAny(packet["results"]) {
		if row["kind"] == kind && row["id"] == path {
			return row, mapsFromAny(row["evidence"])[0]
		}
	}
	t.Fatalf("no %s row for %s in %v", kind, path, packet["results"])
	return nil, nil
}

func warningCodes(evidence map[string]any) []string {
	codes := make([]string, 0)
	for _, warning := range mapsFromAny(evidence["warnings"]) {
		codes = append(codes, warning["code"].(string))
	}
	return codes
}
