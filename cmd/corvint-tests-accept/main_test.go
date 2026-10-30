package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNEAV0007CLIClosedSurface(t *testing.T) {
	for _, args := range [][]string{nil, {"accept", "--unsupported"}, {"accept", "--plan", "missing"}, {"worker", "extra"}} {
		var out bytes.Buffer
		if run(context.Background(), args, strings.NewReader("{}"), &out) == nil {
			t.Fatalf("invalid invocation admitted: %v", args)
		}
		if out.Len() != 0 {
			t.Fatal("partial report emitted")
		}
	}
	t.Setenv("GITHUB_TOKEN", "credential-sentinel")
	t.Setenv("NODE_OPTIONS", "credential-sentinel")
	var out bytes.Buffer
	err := run(context.Background(), []string{"worker"}, strings.NewReader(`{"kind":"repeat","request":{"schema":"x"}}`), &out)
	if err == nil || err.Error() != "worker-environment-unsafe" || out.Len() != 0 {
		t.Fatal("direct worker did not reject inherited credential/runtime keys before execution")
	}
}
