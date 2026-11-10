// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree"
)

// GPK-V0-064 as amended by decision 0196, for the docs parsers: an option-like next token is
// refused through the parser's own missing-value path, while the inline `=` form and a negative
// number are values. Moved from cmd/corvint with the parsers.
func TestDocsOptionValuesFollowArgparseOptionLikeClassification(t *testing.T) {
	t.Parallel()
	docs := func(args ...string) error {
		_, _, err := parseDocsInvocation(append([]string{"docs"}, args...))
		return err
	}
	maintain := func(args ...string) error {
		_, _, err := parseDocsMaintainInvocation(append([]string{"docs", "maintain"}, args...))
		return err
	}
	for _, test := range []struct {
		name, missing string
		parse         func(value string, inline bool) error
	}{
		{"docs", "missing docs argument value: --source", func(v string, inline bool) error {
			if inline {
				return docs("draft", "--source="+v)
			}
			return docs("draft", "--source", v)
		}},
		{"docs-maintain", "missing docs maintain argument value: --page", func(v string, inline bool) error {
			if inline {
				return maintain("--page=" + v)
			}
			return maintain("--page", v)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, value := range []string{"-x", "--help", "-h"} {
				if err := test.parse(value, false); err == nil || !strings.Contains(err.Error(), test.missing) {
					t.Errorf("bare value %q: err=%v, want %q", value, err, test.missing)
				}
				if err := test.parse(value, true); err != nil && strings.Contains(err.Error(), test.missing) {
					t.Errorf("inline value %q refused as missing: %v", value, err)
				}
			}
			if err := test.parse("-5", false); err != nil && strings.Contains(err.Error(), test.missing) {
				t.Errorf("negative number refused as missing: %v", err)
			}
		})
	}
}

// Help never reads stdin, prints the authored topic bytes and exits 0 (GPK-V0-062, GPK-V0-064),
// for every help form the binary accepts; an unknown topic is the conventional error.
func TestHelpFormsPrintAuthoredTopics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--help"}, rootHelp},
		{[]string{"-h"}, rootHelp},
		{[]string{"help"}, rootHelp},
		{[]string{"--root", "/definitely/not/a/repository", "docs", "--help"}, helpText("docs")},
		{[]string{"docs", "draft", "--source", "x", "--help"}, helpText("docs")},
		{[]string{"help", "docs"}, helpText("docs")},
		{[]string{"help", "docs", "corpus"}, docsCorpusHelp()},
		{[]string{"help", "docs corpus"}, docsCorpusHelp()},
		{[]string{"docs", "corpus", "--help"}, docsCorpusHelp()},
		{[]string{"docs", "corpus", "search", "--artifact", "a.json", "--help"}, docsCorpusOperationHelp("search")},
		{[]string{"help", "docs", "corpus", "search"}, docsCorpusOperationHelp("search")},
		{[]string{"docs", "maintain", "--page", "p", "--help"}, docsMaintainHelp},
		{[]string{"help", "docs", "flows"}, docsFlowsHelp},
		{[]string{"docs-tool", "--help"}, docsToolHelp},
		{[]string{"help", "docs-tool"}, docsToolHelp},
	} {
		reader := &forbiddenHelpReader{}
		var stdout, stderr bytes.Buffer
		if exit := runContext(context.Background(), test.args, reader, &stdout, &stderr); exit != 0 || stdout.String() != test.want || stderr.Len() != 0 || reader.reads != 0 {
			t.Errorf("args=%q exit=%d stderr=%q reads=%d stdout matches=%v", test.args, exit, &stderr, reader.reads, stdout.String() == test.want)
		}
	}
	reader := &forbiddenHelpReader{}
	var stdout, stderr bytes.Buffer
	if exit := runContext(context.Background(), []string{"help", "not-a-command"}, reader, &stdout, &stderr); exit != 2 || stdout.Len() != 0 ||
		!strings.HasPrefix(stderr.String(), "{\"code\": \"invalid-arguments\", \"error\": \"unknown help topic: 'not-a-command' (choose from 'docs', 'flows', 'how', 'docs-tool', 'docs corpus', ") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
	if exit := runContext(context.Background(), []string{"corpus", "info"}, reader, &stdout, &stderr); exit != 2 || !strings.Contains(stderr.String(), "\"code\": \"invalid-arguments\"") {
		t.Fatalf("a verb this binary lacks: exit=%d stderr=%q", exit, &stderr)
	}
}

// CAP-V0-040, CAP-V0-045: `help --json` is the corvint-command-tree/0 document every help topic
// derives, with a node for every docs subcommand, every corpus operation and the docs-tool verb.
func TestHelpJSONIsTheCommandTree(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if exit := runContext(context.Background(), []string{"help", "--json"}, strings.NewReader(""), &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, &stderr)
	}
	var tree clitree.Tree
	if err := json.Unmarshal(stdout.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	if tree.Binary != binaryName || tree.HelpJSON != binaryName+" help --json" {
		t.Fatalf("tree identity = %q %q", tree.Binary, tree.HelpJSON)
	}
	for _, path := range [][]string{{"docs"}, {"docs", "draft"}, {"docs", "consume"}, {"docs", "nav"}, {"docs", "citations"}, {"docs", "corpus"}, {"docs", "corpus", "search"}, {"docs", "maintain"}, {"docs", "flows"}, {toolVerb}} {
		if node, ok := tree.Find(path); !ok || len(node.Path) != len(path) {
			t.Errorf("tree lacks node %q", path)
		}
	}
	for _, operation := range corpusOperations {
		if node, ok := tree.Find([]string{"docs", "corpus", operation.name}); !ok || len(node.Path) != 3 {
			t.Errorf("tree lacks corpus operation %q", operation.name)
		}
	}
	if problems := clitree.Problems(tree); len(problems) != 0 {
		t.Fatalf("tree problems: %v", problems)
	}
}

// The draft verb's stdout failure is the one conventional envelope, exit 2 (moved from
// cmd/corvint's output-write-failure matrix).
func TestDraftStdoutWriteFailureIsOneEnvelope(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	var stdout stdoutBrokenPipeWriter
	var stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "docs", "draft", "--source", "owner.md", "--package", "cache"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), `"code": "output-failed"`) || strings.Count(stderr.String(), "\"ok\": false") != 1 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
}

// `docs draft` leaves the worktree byte-identical (moved from cmd/corvint's read-only matrix).
func TestDraftLeavesTheWorktreeUnchanged(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	before := treeSnapshot(t, root)
	if code, _, stderr := runDocsFixture(t, root, "draft", ""); code != 0 {
		t.Fatalf("draft exit %d: %s", code, stderr)
	}
	after := treeSnapshot(t, root)
	for path, want := range before {
		if got := after[path]; got != want {
			t.Errorf("%s changed: %q -> %q", path, want, got)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("%s was created", path)
		}
	}
}
