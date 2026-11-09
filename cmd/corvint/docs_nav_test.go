package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestTrustedNavCLITrustBoundary(t *testing.T) {
	for _, args := range [][]string{{"docs", "nav"}, {"docs", "nav", "--request", "anything"}, {"docs", "nav", "--trusted-project=true", "--request", "anything"}, {"docs", "nav", "--trusted-project", "--request", "a", "--inspect-python", "b"}, {"docs", "nav", "--trusted-project", "--request", "a", "--apply"}} {
		if _, handled, err := parseDocsInvocation(args); !handled || err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	python := os.Getenv("CORVINT_TRUSTED_NAV_PYTHON")
	if python == "" {
		t.Skip("explicit pinned live environment required")
	}
	code, out, err := runCLI(t, "--root", docsRepository(t), "docs", "nav", "--trusted-project", "--inspect-python", python)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, err)
	}
	var pin map[string]any
	if json.Unmarshal([]byte(out), &pin) != nil || pin["profile"] != "corvint-trusted-project-navigation-environment/0" || pin["python"] != python {
		t.Fatalf("unexpected pins: %s", out)
	}
}
