package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestDCPV1097CorpusDepthFlag pins the --corpus-depth argument rules.
func TestDCPV1097CorpusDepthFlag(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"impact", "--corpus-depth=2"}, "--corpus-depth requires --corpus=FILE"},
		{[]string{"query", "--corpus=corpus.json", "--corpus-depth=2"}, "--corpus-depth applies only to impact and affected"},
		{[]string{"impact", "--corpus=corpus.json", "--corpus-depth=0"}, "invalid or duplicate --corpus-depth=N (1-16)"},
		{[]string{"impact", "--corpus=corpus.json", "--corpus-depth=17"}, "invalid or duplicate --corpus-depth=N (1-16)"},
		{[]string{"impact", "--corpus=corpus.json", "--corpus-depth", "2"}, "invalid or duplicate --corpus-depth=N (1-16)"},
		{[]string{"affected", "--corpus=corpus.json", "--corpus-depth=2", "--corpus-depth=3"}, "invalid or duplicate --corpus-depth=N (1-16)"},
	} {
		var out, stderr bytes.Buffer
		code, handled := runCorpusIntegration(context.Background(), c.args, strings.NewReader(""), &out, &stderr)
		if !handled || code != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), c.want) {
			t.Fatalf("%v: handled=%v code=%d stderr=%q, want %q", c.args, handled, code, stderr.String(), c.want)
		}
	}
}
