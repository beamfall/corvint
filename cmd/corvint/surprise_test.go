package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TSS-V0-001: the invocation is recognized only in its own shape, and every
// missing or malformed argument is refused before any repository read.
func TestSurpriseRefusesIncompleteArguments(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"missing task":      {"surprise", "--base", "HEAD~1", "--target", "HEAD"},
		"missing base":      {"surprise", "--task", "text", "--target", "HEAD"},
		"missing target":    {"surprise", "--task", "text", "--base", "HEAD~1"},
		"option revision":   {"surprise", "--task", "text", "--base", "--root", "--target", "HEAD"},
		"unknown flag":      {"surprise", "--task", "text", "--base", "a", "--target", "b", "--nope"},
		"limit out of band": {"surprise", "--task", "text", "--base", "a", "--target", "b", "--limit", "0"},
	}
	for name, arguments := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if status := runSurprise(t.Context(), arguments, &stdout, &stderr); status != 2 {
				t.Fatalf("status=%d, want 2", status)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout=%q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "\"ok\": false") {
				t.Fatalf("stderr=%q", stderr.String())
			}
		})
	}
}

// TSS-V0-001: a different verb is not a surprise invocation.
func TestSurpriseIgnoresOtherVerbs(t *testing.T) {
	t.Parallel()
	if _, isSurprise, err := parseSurpriseInvocation([]string{"observations"}); isSurprise || err != nil {
		t.Fatalf("isSurprise=%v err=%v", isSurprise, err)
	}
	options, isSurprise, err := parseSurpriseInvocation([]string{"surprise", "--task=t", "--base=a", "--target=b", "--limit=5"})
	if !isSurprise || err != nil {
		t.Fatalf("isSurprise=%v err=%v", isSurprise, err)
	}
	if options.Task != "t" || options.Base != "a" || options.Target != "b" || options.Limit != 5 {
		t.Fatalf("options=%+v", options)
	}
}

// TSS-V0-001: ctx is main's signal context, so a cancellation already in
// flight when the verb starts aborts the index load instead of completing it.
func TestRunSurpriseHonorsCancellation(t *testing.T) {
	t.Parallel()
	root, base, target := cemRepo(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	code := runContext(ctx, []string{"--root", root, "surprise", "--task", "t", "--base", base, "--target", target}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "cancel") {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}
