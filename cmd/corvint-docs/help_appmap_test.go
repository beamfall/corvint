package main

import (
	"bytes"
	"strings"
	"testing"
)

// AMAP-V0-975: `flows appmap SUB` prints SUB's own help for `--help` or `-h` anywhere before
// `--`, after flags or the --root preamble, and for `help flows appmap SUB`.
func TestAMAPV0975SubcommandHelp(t *testing.T) {
	for _, args := range [][]string{
		{"--root", "R", "flows", "appmap", "coverage", "--map", "M", "--full", "--help"},
		{"flows", "appmap", "coverage", "-h", "--map", "M"},
		{"--root=R", "flows", "appmap", "coverage", "--budget", "9", "--help"},
		{"help", "flows", "appmap", "coverage"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := run(args, strings.NewReader(""), &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
			t.Fatalf("%v: exit %d, stderr %s", args, exit, stderr.String())
		}
		out := stdout.String()
		for _, want := range []string{"corvint-docs flows appmap coverage", "Usage:", "--map FILE", "Flags: ", "Required: --map FILE", "Example:"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: help lacks %q:\n%s", args, want, out)
			}
		}
		if strings.Contains(out, "flows appmap build") {
			t.Errorf("%v: printed the generic flows help:\n%s", args, out)
		}
	}
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"help", "flows", "appmap", "build"}, strings.NewReader(""), &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), "--manifest FILE") {
		t.Errorf("help flows appmap build: exit %d, %s%s", exit, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exit := run([]string{"help", "flows", "appmap", "nope"}, strings.NewReader(""), &stdout, &stderr); exit != 2 || !strings.Contains(stderr.String(), "unknown help topic") {
		t.Errorf("an unknown SUB must keep the unknown-topic refusal: exit %d, %s", exit, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	// After `--`, a --help is an argument, not a help request.
	run([]string{"flows", "appmap", "coverage", "--map", "M", "--", "--help"}, strings.NewReader(""), &stdout, &stderr)
	if strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("--help after -- printed help:\n%s", stdout.String())
	}
}
