package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// Handler-level flows tests live in internal/command/flows. These remain here because they exercise
// root dispatch, the `how` alias, root help or the compiled binary.

// AFU-V1-130
func TestAFUV1130FlowsHelpSaysTestLinksOnly(t *testing.T) {
	var out, diagnostic bytes.Buffer
	if code := runContext(context.Background(), []string{"help", "flows"}, bytes.NewReader(nil), &out, &diagnostic); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	help := strings.Join(strings.Fields(out.String()), " ")
	for _, want := range []string{"A playwright-list import (Playwright's JSON test list) supplies test links only",
		`keyed "path > describe > title"; steps, actors and outcomes are authored by hand`, "reported and skipped, the rest are written, and the command exits 1"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help lacks %q", want)
		}
	}
}
