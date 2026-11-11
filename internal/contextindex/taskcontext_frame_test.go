package contextindex

import (
	"context"
	"testing"
)

func frameRankFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                   "module example.test/frame\n\ngo 1.27.0\n",
		"AGENTS.md":                "# Instructions\nPreserve project authority.\n",
		"implementation/codec.go":  "package codec\nfunc EncodeFrame() {}\nfunc DecodeFrame() {}\n",
		"implementation/extra.go":  "package codec\nfunc ExpandFrame() {}\n",
		"implementation/final.go":  "package codec\nfunc FinishFrame() {}\n",
		"implementation/unused.go": "package codec\nfunc OtherFrame() {}\n",
		"spec/wire_test.go":        "package codec\nfunc TestWire() { EncodeFrame(); DecodeFrame(); ExpandFrame(); FinishFrame(); OtherFrame() }\n",
		"noise/alpha.go":           "package noise\nfunc NoiseAlpha() {}\n",
		"noise/beta.go":            "package noise\nfunc NoiseBeta() {}\n",
		"noise/gamma.go":           "package noise\nfunc NoiseGamma() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}
