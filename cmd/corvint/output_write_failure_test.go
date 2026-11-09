package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// stdoutBrokenPipeWriter accepts a few bytes then fails every Write, the
// same shape as a closed pipe or a full disk hitting the success path.
type stdoutBrokenPipeWriter struct{ bytes.Buffer }

func (w *stdoutBrokenPipeWriter) Write(raw []byte) (int, error) {
	n, _ := w.Buffer.Write(raw[:min(4, len(raw))])
	return n, io.ErrClosedPipe
}

// TestStdoutWriteFailureReportsOutputFailedNotBareExitOne covers the
// docs/agent-memory/bugs.md 2026-09-13 entry: affected, context lookup,
// task-context, both runIndex branches, and docs used to `return 1` with no
// stderr envelope when the success-path stdout write failed, unlike every
// other verb's output-failed/exit-2 pattern (kernel.go:139-141,
// answerability.go:118-120).
func TestStdoutWriteFailureReportsOutputFailedNotBareExitOne(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		root func(t *testing.T) string
		args func(root string) []string
	}{
		{
			name: "affected",
			root: affectedFixtureRepository,
			args: func(root string) []string { return []string{"--root", root, "affected"} },
		},
		{
			name: "context lookup",
			root: taskContextRepository,
			args: func(root string) []string { return []string{"--root", root, "context", "refs", "Split"} },
		},
		{
			name: "task context",
			root: taskContextRepository,
			args: func(root string) []string {
				return []string{"--root", root, "context", "--task", "t", "--subject", "cache/demux.go"}
			},
		},
		{
			name: "index build-and-write",
			root: taskContextRepository,
			args: func(root string) []string { return []string{"--root", root, "index"} },
		},
		{
			name: "index if-stale fresh-hit",
			root: func(t *testing.T) string {
				root := taskContextRepository(t)
				runIndexForTest(t, root, false)
				return root
			},
			args: func(root string) []string { return []string{"--root", root, "index", "--if-stale"} },
		},
		{
			name: "docs",
			root: docsRepository,
			args: func(root string) []string {
				return []string{"--root", root, "docs", "draft", "--source", "owner.md", "--package", "cache"}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := testCase.root(t)
			var stdout stdoutBrokenPipeWriter
			var stderr bytes.Buffer
			code := runContext(context.Background(), testCase.args(root), strings.NewReader(""), &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit %d, want 2: stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), `"output-failed"`) {
				t.Fatalf("stderr missing output-failed envelope: %q", stderr.String())
			}
		})
	}
}

// TestDogfoodOCMStdoutWriteFailureReportsOutputFailed covers the same bare
// `return 2` in runDogfoodOCM: a failed success-path stdout write must emit
// the output-failed envelope that ocm.go's emitOCMEnvelope emits.
func TestDogfoodOCMStdoutWriteFailureReportsOutputFailed(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	cemGit(t, fixture.root, "add", fixture.mapPath)
	cemGit(t, fixture.root, "commit", "-qm", "CEM sidecar")
	fixture.target = cemGit(t, fixture.root, "rev-parse", "HEAD")
	writeOCM(t, fixture, ".corvint/change.ocm.001.json", false)
	cemWrite(t, fixture.root, ".corvint/change.ocm-intents", "docs/intent.md\n")
	arguments := []string{"--root", fixture.root, "dogfood-ocm", "status", "--expected-base", fixture.base, "--target", fixture.target}
	if code, _, stderr := runCLI(t, arguments...); code != 0 {
		t.Fatalf("baseline exit %d: %s", code, stderr)
	}
	var stdout stdoutBrokenPipeWriter
	var stderr bytes.Buffer
	code := runContext(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), `"output-failed"`) {
		t.Fatalf("exit %d, want 2 with output-failed: stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
