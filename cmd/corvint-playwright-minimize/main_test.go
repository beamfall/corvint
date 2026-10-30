package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestMinimizerCLIRefusesImplicitExecution(t *testing.T) {
	for _, args := range [][]string{{"plan"}, {"execute", "--experimental"}, {"plan", "--experimental", "--approve-plan=sha256:bad"}, {"execute", "--experimental", "--approve-plan=sha256:bad"}, {"unknown", "--experimental"}} {
		var output bytes.Buffer
		if run(context.Background(), args, strings.NewReader("{}"), &output) == nil {
			t.Fatalf("admitted %v", args)
		}
		if output.Len() != 0 {
			t.Fatal("refusal published a report")
		}
	}
}
