// SPDX-License-Identifier: Apache-2.0

package docsbridge

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/docsenvelope"
	"github.com/Beamfall/corvint/internal/execfile"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "init", "-q")
	command.Dir = resolved
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	return resolved
}

// fakeApp is a corvint-docs stand-in: it records its argument vector and stdin, then answers
// with the envelope the stdin text names, so every bridge branch is reachable without the index.
func fakeApp(t *testing.T) (executable, capture string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	dir := t.TempDir()
	capture = filepath.Join(dir, "capture")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$CAPTURE"
input=$(cat)
printf '%s' "$input" >> "$CAPTURE"
case "$input" in
  *failure*) printf '{"schema":"corvint-docs-tool/1","tool":"%s","failure":{"code":"unsupported-documentation-source","message":"named"}}\n' "$4" ;;
  *error*) printf '{"schema":"corvint-docs-tool/1","tool":"%s","error":{"code":"repository-index-timeout"}}\n' "$4" ;;
  *garbage*) printf 'not json\n' ;;
  *exit*) exit 1 ;;
  *stderrline*) printf 'loading\n' >&2; printf 'no corpus artifact at .corvint/corpus\n' >&2; exit 2 ;;
  *wrongtool*) printf '{"schema":"corvint-docs-tool/1","tool":"corvint.other","structured":{}}\n' ;;
  *slow*) sleep 5; printf '{"schema":"corvint-docs-tool/1","tool":"%s","structured":{}}\n' "$4" ;;
  *) printf '{"schema":"corvint-docs-tool/1","tool":"%s","structured":{"state":"READY","markdown":"# hi"},"text":"# hi"}\n' "$4" ;;
esac
`
	executable = filepath.Join(dir, "corvint-docs")
	if err := execfile.WriteFile(executable, []byte(strings.ReplaceAll(script, "$CAPTURE", capture)), 0o755); err != nil {
		t.Fatal(err)
	}
	return executable, capture
}

func TestToolsAreTheTwoDocsTools(t *testing.T) {
	t.Parallel()
	if ToolDraft != docsenvelope.ToolDraft || ToolConsume != docsenvelope.ToolConsume {
		t.Fatal("the bridge and the envelope disagree on the tool names")
	}
	tools := (&Registry{}).Tools()
	if len(tools) != 2 || tools[0].Name != ToolConsume || tools[1].Name != ToolDraft || !tools[0].Annotations.ReadOnlyHint {
		t.Fatalf("tools = %+v", tools)
	}
	if (*Registry)(nil).Tools() != nil {
		t.Fatal("nil registry lists tools")
	}
}

// CMS-V0-003: the bridge runs the app's docs-tool verb with the bound root, the tool name and
// the raw arguments on stdin, and maps the envelope to the same four-way result as before.
func TestCallForwardsTheEnvelope(t *testing.T) {
	executable, capture := fakeApp(t)
	root := repositoryRoot(t)
	registry, failure := New(root)
	if failure != nil {
		t.Fatal(failure)
	}
	registry.locate = func() (string, error) { return executable, nil }
	ctx := context.Background()

	structured, text, toolFailure, transport := registry.Call(ctx, ToolDraft, []byte(`{"source":"owner.md","package":"cache"}`))
	if toolFailure != nil || transport != nil || text != "# hi" || structured["state"] != "READY" {
		t.Fatalf("draft = %v %q %v %v", structured, text, toolFailure, transport)
	}
	recorded, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if want := docsenvelope.ToolVerb + "\n--root\n" + root + "\n" + ToolDraft + "\n" + `{"source":"owner.md","package":"cache"}`; string(recorded) != want {
		t.Fatalf("app saw %q, want %q", recorded, want)
	}

	if _, _, toolFailure, transport := registry.Call(ctx, ToolConsume, []byte(`{"draft":"failure"}`)); transport != nil || toolFailure == nil || toolFailure.Code != "unsupported-documentation-source" || toolFailure.Message != "named" {
		t.Fatalf("failure envelope = %v %v", toolFailure, transport)
	}
	if _, _, toolFailure, transport := registry.Call(ctx, ToolDraft, []byte(`{"draft":"error"}`)); toolFailure != nil || transport == nil || transport.Code != "repository-index-timeout" {
		t.Fatalf("error envelope = %v %v", toolFailure, transport)
	}
	// DMF-V1-010: a process that exits non-zero is child-process-failed with its stderr; an
	// unreadable envelope stays internal-error.
	if _, _, toolFailure, transport := registry.Call(ctx, ToolDraft, []byte(`{"draft":"exit"}`)); toolFailure != nil || transport == nil || transport.Code != ChildFailedCode {
		t.Fatalf("exit = %v %v, want %s", toolFailure, transport, ChildFailedCode)
	}
	for _, input := range []string{`{"draft":"garbage"}`, `{"draft":"wrongtool"}`} {
		if _, _, toolFailure, transport := registry.Call(ctx, ToolDraft, []byte(input)); toolFailure != nil || transport == nil || transport.Code != "internal-error" {
			t.Fatalf("%s = %v %v, want internal-error", input, toolFailure, transport)
		}
	}
	bounded, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if _, _, toolFailure, transport := registry.Call(bounded, ToolDraft, []byte(`{"draft":"slow"}`)); toolFailure != nil || transport == nil || transport.Code != "cancelled" {
		t.Fatalf("cancelled = %v %v", toolFailure, transport)
	}
}

// The bridge's own refusals come before any process: zero registry, cancelled context, bounds,
// unknown tool and a moved root. A missing app is the named not-installed tool failure.
func TestCallRefusesBeforeRunningTheApp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if _, _, _, transport := (&Registry{}).Call(ctx, ToolDraft, []byte(`{}`)); transport == nil || transport.Code != "invalid-registry" {
		t.Fatalf("zero registry = %v", transport)
	}
	root := repositoryRoot(t)
	registry, failure := New(root)
	if failure != nil {
		t.Fatal(failure)
	}
	started := 0
	registry.locate = func() (string, error) { started++; return "", docsenvelope.ErrNotInstalled }
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	cases := []struct {
		ctx       context.Context
		name      string
		arguments []byte
		code      string
	}{
		{cancelled, ToolDraft, []byte(`{}`), "cancelled"},
		{ctx, ToolDraft, nil, "invalid-arguments"},
		{ctx, ToolDraft, []byte(strings.Repeat("{", docsenvelope.MaxArgumentBytes+1)), "invalid-arguments"},
		{ctx, ToolDraft, []byte("{\xff}"), "invalid-arguments"},
		{ctx, "corvint.docs_other", []byte(`{}`), "unsupported-tool"},
	}
	for _, test := range cases {
		if _, _, toolFailure, transport := registry.Call(test.ctx, test.name, test.arguments); toolFailure != nil || transport == nil || transport.Code != test.code {
			t.Errorf("%s %q: %v %v, want %s", test.name, test.arguments, toolFailure, transport, test.code)
		}
	}
	if started != 0 {
		t.Fatalf("the app was located %d times before the bridge's own refusals", started)
	}
	structured, text, toolFailure, transport := registry.Call(ctx, ToolDraft, []byte(`{"source":"a.md","package":"p"}`))
	if structured != nil || text != "" || transport != nil || toolFailure == nil || toolFailure.Code != docsenvelope.NotInstalledCode || !strings.Contains(toolFailure.Message, "NOT_INSTALLED") {
		t.Fatalf("missing app = %v %q %v %v", structured, text, toolFailure, transport)
	}
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, _, _, transport := registry.Call(ctx, ToolDraft, []byte(`{}`)); transport == nil || transport.Code != "repository-unavailable" {
		t.Fatalf("unbound root = %v", transport)
	}
}

func TestNewRefusesInvalidRoots(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"", "relative", "/definitely/not/here", t.TempDir(), "/tmp/../tmp"} {
		if _, failure := New(root); failure == nil || failure.Code != "invalid-root" {
			t.Errorf("New(%q) = %v", root, failure)
		}
	}
}

// The envelope types round-trip through JSON with the member names the app writes.
func TestEnvelopeMembers(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(docsenvelope.ToolResult{Schema: docsenvelope.ToolSchema, Tool: ToolDraft, Failure: &docsenvelope.Coded{Code: "c", Message: "m"}})
	if err != nil || string(raw) != `{"schema":"corvint-docs-tool/1","tool":"corvint.docs_draft","failure":{"code":"c","message":"m"}}` {
		t.Fatalf("%s %v", raw, err)
	}
}

// scriptApp installs body as the corvint-docs stand-in of a fresh registry.
func scriptApp(t *testing.T, body string) (*Registry, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	dir := t.TempDir()
	executable := filepath.Join(dir, "corvint-docs")
	if err := execfile.WriteFile(executable, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	registry, failure := New(repositoryRoot(t))
	if failure != nil {
		t.Fatal(failure)
	}
	registry.locate = func() (string, error) { return executable, nil }
	return registry, dir
}

// A cancelled call asks the app to stop and lets it stop its own children: the stand-in's
// TERM handler runs, which a kill would never allow, and the call still answers "cancelled"
// well inside the grace.
func TestCancelledCallLetsTheAppStopItsChildren(t *testing.T) {
	registry, dir := scriptApp(t, `#!/bin/sh
cat > /dev/null
sleep 30 &
child=$!
trap 'kill "$child"; echo stopped > "$MARK"; exit 0' TERM
echo started > "$MARK.started"
wait
`)
	mark := filepath.Join(dir, "mark")
	t.Setenv("MARK", mark)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if _, err := os.Stat(mark + ".started"); err == nil {
				break
			}
		}
		cancel()
	}()
	started := time.Now()
	_, _, toolFailure, transport := registry.Call(ctx, ToolDraft, []byte(`{}`))
	if toolFailure != nil || transport == nil || transport.Code != "cancelled" {
		t.Fatalf("cancelled = %v %v", toolFailure, transport)
	}
	if elapsed := time.Since(started); elapsed >= waitDelay {
		t.Fatalf("the call took %s, so the app was killed after the grace instead of stopping", elapsed)
	}
	if recorded, err := os.ReadFile(mark); err != nil || string(recorded) != "stopped\n" {
		t.Fatalf("the app's TERM handler did not run: %q %v", recorded, err)
	}
}

// PRB-V0-005: the server owns a profile file named by a fixed path, so the app does not
// inherit that knob; a path with the {pid} token names one file per process and stays.
func TestPRBV0005DocsToolChildKeepsOnlyPIDKnobs(t *testing.T) {
	registry, dir := scriptApp(t, `#!/bin/sh
cat > /dev/null
printf '%s|%s|%s\n' "${CORVINT_CPUPROFILE-unset}" "${CORVINT_MEMPROFILE-unset}" "$CORVINT_TEST_KEEP" > "$MARK"
printf '{"schema":"corvint-docs-tool/1","tool":"%s","structured":{}}\n' "$4"
`)
	mark := filepath.Join(dir, "mark")
	t.Setenv("MARK", mark)
	t.Setenv("CORVINT_TEST_KEEP", "kept")
	for _, test := range []struct{ cpu, mem, want string }{
		{"/profiles/cpu.prof", "/profiles/mem-{pid}.prof", "unset|/profiles/mem-{pid}.prof|kept\n"},
		{"/profiles/cpu-{pid}.prof", "/profiles/mem.prof", "/profiles/cpu-{pid}.prof|unset|kept\n"},
	} {
		t.Setenv("CORVINT_CPUPROFILE", test.cpu)
		t.Setenv("CORVINT_MEMPROFILE", test.mem)
		if _, _, toolFailure, transport := registry.Call(context.Background(), ToolDraft, []byte(`{}`)); toolFailure != nil || transport != nil {
			t.Fatalf("call = %v %v", toolFailure, transport)
		}
		if recorded, err := os.ReadFile(mark); err != nil || string(recorded) != test.want {
			t.Errorf("child environment %q (%v); want %q", recorded, err, test.want)
		}
	}
}

// DMF-V1-010: a corvint-docs process that exits 2 with stderr ending in a corpus line is
// child-process-failed with that line in the message and the corpus build command as next.
func TestDMFV1010ChildFailureKeepsStderr(t *testing.T) {
	executable, _ := fakeApp(t)
	registry, failure := New(repositoryRoot(t))
	if failure != nil {
		t.Fatal(failure)
	}
	registry.locate = func() (string, error) { return executable, nil }
	_, _, toolFailure, transport := registry.Call(context.Background(), ToolDraft, []byte(`{"draft":"stderrline"}`))
	if toolFailure != nil || transport == nil || transport.Code != ChildFailedCode ||
		!strings.Contains(transport.Message, "no corpus artifact at .corvint/corpus") || strings.Contains(transport.Message, "loading") ||
		!strings.Contains(transport.Next, "corvint-docs docs corpus build") {
		t.Fatalf("child failure = %v %#v", toolFailure, transport)
	}
}
