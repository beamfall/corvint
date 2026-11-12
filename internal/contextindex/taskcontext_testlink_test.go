package contextindex

import (
	"context"
	"testing"
)

// testLinkFixture holds one code/test pairing per TCP-V0-015 signal, each
// bound by that signal alone, plus three definers of the prose word `all`.
func testLinkFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                    "module example.test/link\n\ngo 1.27.0\n",
		"src/render/render.go":      "package render\n\nfunc Render() {}\n",
		"src/render/render_test.go": "package render\n\nfunc TestNothing() {}\n",
		"store/store.go":            "package store\n\nfunc Get() int { return 1 }\n",
		"probe/probe_test.go":       "package probe\n\nimport \"example.test/link/store\"\n\nfunc TestProbe() { _ = store.Get() }\n",
		"codec/frame.go":            "package codec\n\nfunc EncodeFrame() {}\n",
		"spec/wire_test.go":         "package spec\n\n// EncodeFrame is exercised here.\nfunc TestWire() {}\n",
		"queue/drain.go":            "package queue\n\nfunc DrainQueue() {}\n",
		"other/other_test.go":       "package other\n\nfunc TestDrainQueueTwice() {}\n",
		"alpha/alpha.go":            "package alpha\n\nfunc all() {}\n",
		"beta/beta.go":              "package beta\n\nfunc all() {}\n",
		"gamma/gamma.go":            "package gamma\n\nfunc all() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func contextReasonOf(t *testing.T, packet map[string]any, kind string) (string, string) {
	t.Helper()
	for _, item := range mapsFromAny(packet["results"]) {
		if item["kind"] != kind {
			continue
		}
		evidence := mapsFromAny(item["evidence"])
		return item["id"].(string), evidence[0]["reason"].(string)
	}
	return "", ""
}
