package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestBBFV0001CLIRequiresExperimentalAdmission(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, "usage:"},
		{[]string{"plan"}, "explicit --experimental required"},
		{[]string{"execute", "--experimental"}, "explicit --approve-plan required"},
		{[]string{"unknown", "--experimental"}, "unknown operation"},
	} {
		err := run(context.Background(), test.args, strings.NewReader(`{}`), new(bytes.Buffer))
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("run(%v) error = %v, want %q", test.args, err, test.want)
		}
	}
}

func TestBBFV0001CLIRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, input := range []string{`{"unknown":true}`, `{} {}`} {
		err := run(context.Background(), []string{"plan", "--experimental"}, strings.NewReader(input), new(bytes.Buffer))
		if err == nil {
			t.Fatalf("input %q unexpectedly accepted", input)
		}
	}
}
