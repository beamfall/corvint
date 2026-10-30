package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFrontierNextCannotAdmitCallerRoot(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--enrollment", strings.Repeat("a", 64)}, {"--root", "caller.json"}, {"--fixture"}, {"--enrollment", strings.Repeat("a", 64), "--accepted"}} {
		var out bytes.Buffer
		if runFrontierNext(args, &out) != 2 || !strings.Contains(out.String(), `"authority":"NONE"`) || strings.Contains(out.String(), `"state":"EMPTY"`) {
			t.Fatal(out.String())
		}
	}
}
