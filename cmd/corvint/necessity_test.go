package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/necessity"
)

// NEC-V0-001: the verb takes the context verb's arguments, defaults the limit,
// and refuses an unknown flag or a missing task.
func TestParseNecessityInvocation(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	options, isNecessity, err := parseNecessityInvocation([]string{"--root", root, "necessity", "--task", "find the demux", "--subject", "cache/demux.go", "--limit", "7"})
	if err != nil || !isNecessity {
		t.Fatalf("parse: isNecessity=%v err=%v", isNecessity, err)
	}
	if options.Task != "find the demux" || options.Subject != "cache/demux.go" || options.Limit != 7 {
		t.Fatalf("options = %+v", options)
	}
	if options, _, err := parseNecessityInvocation([]string{"--root", root, "necessity", "--task=t"}); err != nil || options.Limit != necessity.DefaultLimit {
		t.Fatalf("default limit: %+v %v", options, err)
	}
	if _, isNecessity, err := parseNecessityInvocation([]string{"--root", root, "necessity", "--subject", "cache/demux.go"}); !isNecessity || err == nil {
		t.Fatalf("missing --task: isNecessity=%v err=%v", isNecessity, err)
	}
	if _, isNecessity, err := parseNecessityInvocation([]string{"--root", root, "necessity", "--task", "t", "--mutate"}); !isNecessity || err == nil {
		t.Fatalf("unknown flag: isNecessity=%v err=%v", isNecessity, err)
	}
	if _, isNecessity, _ := parseNecessityInvocation([]string{"--root", root, "context", "--task", "t"}); isNecessity {
		t.Fatal("context parsed as necessity")
	}
}

// NEC-V0-002, NEC-V0-008: the verb emits the ordinary packet plus the
// necessity block, writes nothing into the repository, and is deterministic.
func TestRunNecessityIsReadOnlyAndLabelsThePacket(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	before := repositoryListing(t, root)
	outputs := make([]string, 0, 2)
	for range 2 {
		var stdout, stderr bytes.Buffer
		if code := runNecessity(t.Context(), []string{"--root", root, "necessity", "--task", "does `Split` keep empty keys", "--subject", "cache/demux.go"}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		outputs = append(outputs, stdout.String())
	}
	if outputs[0] != outputs[1] {
		t.Fatalf("output differs across runs:\n%s\n%s", outputs[0], outputs[1])
	}
	var packet map[string]any
	if err := json.Unmarshal([]byte(outputs[0]), &packet); err != nil {
		t.Fatal(err)
	}
	if packet["tool"] != "context" || packet["mutates"] != false || packet["state"] != "READY" {
		t.Fatalf("packet = %s", outputs[0])
	}
	block := packet["necessity"].(map[string]any)
	if len(block["labels"].([]any)) != len(packet["results"].([]any)) {
		t.Fatalf("one label per result expected: %s", outputs[0])
	}
	if after := repositoryListing(t, root); after != before {
		t.Fatalf("necessity wrote into the repository:\n%s\n%s", before, after)
	}
}

// NEC-V0-008: an argument failure is one error envelope on stderr and exit 2.
func TestRunNecessityRefusesAnUnknownFlag(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	var stdout, stderr bytes.Buffer
	code := runNecessity(t.Context(), []string{"--root", root, "necessity", "--task", "t", "--mutate"}, &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}

// NEC-V0-008: a cancelled invocation (SIGINT or SIGTERM through main's signal
// context) is a repository failure, never a compiled answer.
func TestRunNecessityHonorsCancellation(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	code := runContext(ctx, []string{"--root", root, "necessity", "--task", "does `Split` keep empty keys", "--subject", "cache/demux.go"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "cancel") {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}
