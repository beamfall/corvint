package main

import (
	"strings"
	"testing"
)

func TestImpactProviderMCP(t *testing.T) {
	t.Run("EEP-MCP-001 explicit flag", func(t *testing.T) {
		parsed, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, []string{"--provider-mcp", `["/bin/false"]`, "pkg/main.go"}, "darwin")
		if err != nil || len(parsed.impactProviders) != 1 || !strings.HasPrefix(parsed.impactProviders[0], "\x00mcp\x00") {
			t.Fatalf("MCP flag: %v %v", parsed.impactProviders, err)
		}
		for _, args := range [][]string{{"--provider-mcp"}, {"--provider-mcp", `["relative"]`, "pkg/main.go"}, {"--provider-mcp", `["/bin/false"]`, "--base", strings.Repeat("a", 40)}} {
			if _, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, args, "darwin"); err == nil {
				t.Fatalf("accepted %v", args)
			}
		}
	})
}
