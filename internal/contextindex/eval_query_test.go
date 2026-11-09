package contextindex

import (
	"context"
	"testing"
)

func evalQueryRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		".gitignore": ".context-corvint/\n",
		"go.mod":     "module example.test/eval\n\ngo 1.27.0\n",
		"testing/features.yaml": "features:\n" +
			"  - id: session-revocation\n    area: auth\n    summary: Revoke an expired device session and enforce future denial.\n    adr: []\n    applies: [server]\n    status: shipped\n",
		"testing/scenarios.yaml": "scenarios:\n" +
			"  - id: revoked-device\n    area: auth\n    summary: Expired device session is revoked.\n    features: [session-revocation]\n    applies: [server]\n    status: shipped\n",
		"internal/auth/session.go":      "package auth\n\nfunc EnforceSessionRevocation() bool { return true }\n",
		"internal/auth/session_test.go": "package auth\n\n// feature:session-revocation\nfunc TestEnforceSessionRevocation() {}\n",
		"web/session.ts":                "export function ignoredSessionRevocation() { return true }\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "enforce expired device session revocation")
	return root
}

func TestEvalImpactAppliesBudgetWithoutChangingImpact(t *testing.T) {
	root := evalQueryRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	budget := 1800
	receipt, err := EvalImpact(index, []string{"internal/auth/session.go"}, 10, &budget)
	if err != nil {
		t.Fatal(err)
	}
	request := receipt["request"].(map[string]any)
	coverage := receipt["coverage"].(map[string]any)
	if request["budget_bytes"] != budget || coverage["budget_bytes"] != budget || coverage["within_budget"] != true {
		t.Fatalf("request=%v coverage=%v", request, coverage)
	}
}
