package secretscreen

import (
	"strings"
	"testing"
)

func TestLTAV0017SKBranchNeedsAnUnhyphenatedRun(t *testing.T) {
	for name, key := range map[string]string{
		"legacy":          "sk-abcdefghijklmnopqrstuvwxyz",
		"legacy 48":       "sk-" + strings.Repeat("aB3dE5gH7j", 4) + "kL9nOp1r",
		"project":         "sk-proj-" + strings.Repeat("Ab3_Cd5-Ef7Gh9iJk1Lm3No5", 3),
		"anthropic-shape": "sk-ant-api03-" + strings.Repeat("aB3dE5gH7jkL9nOp1rSt", 4) + "-uV_wX",
		"exactly 20":      "sk-abcdefghij0123456789",
	} {
		for _, text := range []string{key, "OPENAI " + key + " end", "docs/" + key + ".md"} {
			redacted, hit := Screen(text)
			if !hit || strings.Contains(redacted, key[3:]) {
				t.Errorf("%s: Screen(%q) = (%q, %v), want the key redacted", name, text, redacted, hit)
			}
		}
	}
	for _, text := range []string{
		"docs/build-log/2026-10-22-sk-1022p-site-knowledge-setup.md",
		"sk-1022p-site-knowledge-setup",
		"sk-lane-runner-oc-batch-review-notes",
		"sk-abcdefghij012345678",
	} {
		if MatchString(text) {
			t.Errorf("writer screen matched hyphenated words %q", text)
		}
	}
	// The immutable schema-v1 reader keeps the original sk- shape.
	if !MatchStoredV1String("sk-1022p-site-knowledge-setup") {
		t.Error("StoredV1Pattern no longer matches the original sk- shape")
	}
}
