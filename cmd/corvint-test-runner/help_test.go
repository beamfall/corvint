package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestHelpNeedsNoExecutablePin covers TRE-V0-078.
func TestHelpNeedsNoExecutablePin(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}, {"plan", "--help"}, {"run", "-h"}, {"run", "--plan", "x", "--help"}, {"runners", "--help"}, {"profiles", "-help"}, {"runners", "--help", "--help"}, {"plan", "-h", "-h"}} {
		var out, errout bytes.Buffer
		if rc := command(context.Background(), args, &out, &errout); rc != 0 {
			t.Fatalf("%v: rc %d stderr %q", args, rc, errout.String())
		}
		if !strings.Contains(out.String(), "usage") || strings.Contains(errout.String(), "refused") {
			t.Fatalf("%v: stdout %q stderr %q", args, out.String(), errout.String())
		}
	}
	var out, errout bytes.Buffer
	command(context.Background(), []string{"plan", "--help"}, &out, &errout)
	for _, flag := range []string{"-executable", "-executable-sha256", "-request"} {
		if !strings.Contains(out.String(), flag) {
			t.Fatalf("plan help lacks %s: %q", flag, out.String())
		}
	}
}

// TestListingVerbsNeedNoExecutablePin covers TRE-V0-079.
func TestListingVerbsNeedNoExecutablePin(t *testing.T) {
	var want string
	for _, verb := range []string{"runners", "profiles"} {
		var out, errout bytes.Buffer
		if rc := command(context.Background(), []string{verb}, &out, &errout); rc != 0 || !strings.HasPrefix(out.String(), "[") {
			t.Fatalf("%s: rc %d stdout %q stderr %q", verb, rc, out.String(), errout.String())
		}
		if want == "" {
			want = out.String()
		} else if out.String() != want {
			t.Fatalf("profiles differs from runners")
		}
	}
	var out, errout bytes.Buffer
	if rc := command(context.Background(), []string{"bogus"}, &out, &errout); rc != 2 || !strings.Contains(errout.String(), `unknown command "bogus"`) || strings.Contains(errout.String(), "refused") {
		t.Fatalf("unknown verb: rc %d stderr %q", rc, errout.String())
	}
}

// TestExecutingVerbsNameMissingPinFlags covers TRE-V0-080.
func TestExecutingVerbsNameMissingPinFlags(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"plan", "--request", "/nonexistent"}, "missing --executable and --executable-sha256"},
		{[]string{"run", "--executable", "/x"}, "missing --executable-sha256"},
		{[]string{"plan", "--executable-sha256", "a"}, "missing --executable\n"},
	}
	for _, c := range cases {
		var out, errout bytes.Buffer
		if rc := command(context.Background(), c.args, &out, &errout); rc != 1 || !strings.Contains(errout.String(), "runner refused: independent executable identity required: "+c.want) {
			t.Fatalf("%v: rc %d stderr %q", c.args, rc, errout.String())
		}
	}
}
