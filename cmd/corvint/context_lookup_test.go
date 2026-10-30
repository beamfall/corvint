package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseContextLookupInvocation(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	options, isLookup, err := parseContextLookupInvocation([]string{"--root", root, "context", "grep", "split", "key", "--limit", "7"})
	if err != nil || !isLookup {
		t.Fatalf("parse: isLookup=%v err=%v", isLookup, err)
	}
	if options.mode != "grep" || strings.Join(options.terms, ",") != "split,key" || options.limit != 7 {
		t.Fatalf("options = %+v", options)
	}
	if options, _, err := parseContextLookupInvocation([]string{"--root", root, "context", "defs", "Split"}); err != nil || options.limit != 20 {
		t.Fatalf("default limit: %+v %v", options, err)
	}
	if _, isLookup, err := parseContextLookupInvocation([]string{"--root", root, "context", "refs", "a", "b"}); !isLookup || err == nil {
		t.Fatalf("two identifiers: isLookup=%v err=%v", isLookup, err)
	}
	if _, isLookup, err := parseContextLookupInvocation([]string{"--root", root, "context", "defs", "Split", "--mutate"}); !isLookup || err == nil {
		t.Fatalf("unknown flag: isLookup=%v err=%v", isLookup, err)
	}
	if _, isLookup, _ := parseContextLookupInvocation([]string{"--root", root, "context", "--task", "t"}); isLookup {
		t.Fatal("context --task parsed as a lookup")
	}
}

func TestRunContextLookupIsReadOnlyAndDeterministic(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	before := repositoryListing(t, root)
	outputs := make([]string, 0, 2)
	for range 2 {
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), []string{"--root", root, "context", "refs", "Split"}, strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		outputs = append(outputs, stdout.String())
	}
	if outputs[0] != outputs[1] {
		t.Fatalf("output differs across runs:\n%s\n%s", outputs[0], outputs[1])
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(outputs[0]), &result); err != nil {
		t.Fatal(err)
	}
	rows := result["results"].([]any)
	if result["mode"] != "refs" || result["state"] != "READY" || len(rows) != 1 || rows[0].(map[string]any)["id"] != "cache/demux_test.go" {
		t.Fatalf("refs = %s", outputs[0])
	}
	if after := repositoryListing(t, root); after != before {
		t.Fatalf("lookup wrote into the repository:\n%s\n%s", before, after)
	}
}

func TestRunContextLookupRefusesAnEmptyIdentifier(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "context", "defs", ""}, strings.NewReader(""), &stdout, &stderr)
	var envelope struct {
		Code string `json:"code"`
		OK   bool   `json:"ok"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr %q: %v", stderr.String(), err)
	}
	if code != 2 || stdout.Len() != 0 || envelope.OK || envelope.Code != "unsupported-context-lookup-identifier" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}

// ATI-V0-009, ATI-V0-011: the authority mode is the one lookup that takes several
// subjects, and like every other `context` mode it writes nothing.
func TestContextAuthorityModeAcceptsSeveralPathsAndStaysReadOnly(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	options, isLookup, err := parseContextLookupInvocation([]string{"--root", root, "context", "authority", "a/one.go", "a/two.go", "--limit", "3"})
	if err != nil || !isLookup || options.mode != "authority" || strings.Join(options.terms, ",") != "a/one.go,a/two.go" || options.limit != 3 {
		t.Fatalf("authority parse: %+v isLookup=%v err=%v", options, isLookup, err)
	}
	before := repositoryListing(t, root)
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "context", "authority", "a/one.go"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["mode"] != "authority" || result["mutates"] != false || result["untracked_paths"] == nil {
		t.Fatalf("authority envelope = %s", stdout.String())
	}
	if after := repositoryListing(t, root); after != before {
		t.Fatal("authority lookup mutated the repository")
	}
}
