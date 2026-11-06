package main

import (
	"strings"
	"testing"
)

// TestFlowsImpactUnknownBaseUsesRootRefusal pins the root wiring of flowsEnv.BaseRefusal: the
// flows package tests use a stub Env, so only this end-to-end dispatch catches a miswired or nil
// refusal (AFU-V1 impact keeps the affected-revision refusal envelope after the V1-4756 move).
func TestFlowsImpactUnknownBaseUsesRootRefusal(t *testing.T) {
	fx := newShopFixture(t)
	base := strings.Repeat("0", 40)
	code, out, diagnostic := runFlowsCLI(fx.root, "impact", "--flows", "flows", "--base", base)
	if code != 2 || out != "" {
		t.Fatalf("code=%d out=%q diagnostic=%s", code, out, diagnostic)
	}
	for _, want := range []string{"unsupported-affected-revision", "base " + base + " is not a commit in this repository", "affected.use-commit-base"} {
		if !strings.Contains(diagnostic, want) {
			t.Fatalf("diagnostic lacks %q: %s", want, diagnostic)
		}
	}
}
