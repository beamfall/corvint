package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// Handler-level flows tests live in internal/command/flows. These remain here because they exercise
// root dispatch, the `how` alias, root help or the compiled binary.

// SKH-V0-810: `corvint how` is a top-level alias of `flows appmap how`: the same flags give
// byte-identical stdout and the same refusal codes, and root help lists it.
func TestSKHV0810HowAlias(t *testing.T) {
	root, filename := inferCLIRepo(t)
	run := func(args ...string) (int, string, string) {
		var out, diagnostic bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root}, args...), bytes.NewReader(nil), &out, &diagnostic)
		return code, out.String(), diagnostic.String()
	}
	flags := []string{"--map", filename, "--source", "e2e", "--condition", "an order exists", "--full", "ship an order"}
	wantCode, want, _ := runFlowsCLI(root, append([]string{"appmap", "how"}, flags...)...)
	code, got, diag := run(append([]string{"how"}, flags...)...)
	if wantCode != 0 || code != 0 || got != want {
		t.Fatalf("alias = %d %q (%s), nested = %d %q", code, got, diag, wantCode, want)
	}
	for _, args := range [][]string{{"--map", filename, "--source", "e2e"}, {"--source", "e2e", "ship"}} {
		nested, _, _ := runFlowsCLI(root, append([]string{"appmap", "how"}, args...)...)
		if alias, _, _ := run(append([]string{"how"}, args...)...); alias != 2 || nested != 2 {
			t.Fatalf("%v exit alias=%d nested=%d, want 2", args, alias, nested)
		}
	}
	if code, out, _ := run("help"); code != 0 || !strings.Contains(out, "\n    how ") {
		t.Fatalf("root help does not list how: %d %s", code, out)
	}
}
