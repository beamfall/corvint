package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestTestRunCommandRefusesBeforeRunning(t *testing.T) {
	for _, arguments := range [][]string{{"--timeout", "0s"}, {"--unknown"}, {"--dir", t.TempDir()}} {
		var stdout, stderr bytes.Buffer
		if code := runTestRunCommand(context.Background(), arguments, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("%v: code %d stdout %q", arguments, code, stdout.String())
		}
		if !strings.Contains(stderr.String(), "test-run") {
			t.Fatalf("%v: stderr %q", arguments, stderr.String())
		}
	}
}
