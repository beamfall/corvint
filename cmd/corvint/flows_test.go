package main

import (
	"bytes"
	"context"
	"testing"
)

// Handler-level flows tests live in internal/command/flows. These remain here because they exercise
// root dispatch, the `how` alias, root help or the compiled binary.

func TestFlowsHelpDoesNotRequireRepository(t *testing.T) {
	for _, args := range [][]string{{"help", "flows"}, {"flows", "--help"}, {"flows", "record", "--help"}} {
		var out, diagnostic bytes.Buffer
		if code := runContext(context.Background(), args, bytes.NewReader(nil), &out, &diagnostic); code != 0 {
			t.Fatal(args, code, diagnostic.String())
		}
		if !bytes.Contains(out.Bytes(), []byte("application-flow-report/0")) && !bytes.Contains(out.Bytes(), []byte("Full topic: corvint help flows")) {
			t.Fatal("missing help")
		}
	}
}
