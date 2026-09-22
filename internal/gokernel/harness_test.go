package gokernel

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func runGit(t testing.TB, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}

func testRepository(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "corvint@example.test")
	runGit(t, root, "config", "user.name", "Corvint Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "initial")
	return root
}

func request(root, event, input string) EventRequest {
	return EventRequest{
		Root: root, Host: "codex", HostVersion: "1.2.3", Surface: "plugin",
		AdapterVersion: "0.1.0", Event: event, Input: []byte(input), BudgetBytes: 8_000,
	}
}

func TestAHI011EmbeddedHostSchemaDrivesAdmission(t *testing.T) {
	want := []string{"claude-code", "codex", "gemini-cli", "opencode", "pi"}
	if embeddedHostSchema.Version != "0" {
		t.Fatalf("host schema version = %q", embeddedHostSchema.Version)
	}
	if embeddedHostSchema.DegradationPolicy.Match != "subset-of-recognised" || embeddedHostSchema.DegradationPolicy.OnUnrecognised != "refuse" {
		t.Fatalf("host schema degradation policy = %+v", embeddedHostSchema.DegradationPolicy)
	}
	if got := HarnessHosts(); !slices.Equal(got, want) {
		t.Fatalf("embedded hosts = %v, want %v", got, want)
	}
	for _, host := range want {
		if !KnownHarnessHost(host) {
			t.Fatalf("embedded host %q is not admitted", host)
		}
	}
	if KnownHarnessHost("unregistered-host") {
		t.Fatal("host absent from the embedded schema was admitted")
	}
}

func kernelCode(err error) string {
	var structured *Error
	if errors.As(err, &structured) {
		return structured.Code
	}
	return ""
}

func TestSupportedEventsProduceCanonicalNativeEnvelopes(t *testing.T) {
	root := testRepository(t)
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	cases := []struct {
		name, event, input string
	}{
		{"start-default", "session-start", `{}`},
		{"start-null-optionals", "session-start", `{"sessionIdSha256":null,"startSource":null}`},
		{"start-resume", "session-start", `{"sessionIdSha256":"` + digestA + `","startSource":"resume"}`},
		{"post-tool", "post-tool", `{"changedPaths":["src/main.go"],"observedEvidenceHandles":["evidence:\u2028:café"],"verification":[{"commandSha256":"` + digestA + `","status":"passed"}]}`},
		{"stop", "stop", `{"changedPaths":["src/main.go"],"stopHookActive":true}`},
		{"end-default", "session-end", `{}`},
		{"end-null-optionals", "session-end", `{"outcome":null,"sessionIdSha256":null,"taskSha256":null}`},
		{"end-outcome", "session-end", `{"changedPaths":["src/main.go"],"openedPaths":["src/main.go"],"outcome":"passed","taskSha256":"` + digestA + `","verification":[{"commandSha256":"` + digestB + `","status":"passed"}]}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result, err := HandleEvent(request(root, test.event, test.input))
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := CanonicalJSON(result)
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) == 0 || result["event"] != test.event || result["support"] != "FALLBACK" || result["mutates"] != false || result["profile"] != "corvint-harness-event/0" {
				t.Fatalf("invalid %s envelope: %s", test.event, encoded)
			}
		})
	}
}

func TestMixedRepositoryEnvelopeMatchesPythonOracle(t *testing.T) {
	root := testRepository(t)
	runGit(t, root, "mv", "src/main.go", "src/renamed.go")
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := HandleEvent(request(root, "session-start", `{}`))
	if err != nil {
		t.Fatal(err)
	}
	repository := result["repository"].(map[string]any)
	if repository["worktreeState"] != "mixed" || repository["dirtyPathCount"] != 3 {
		t.Fatalf("unexpected mixed envelope: %#v", repository)
	}
}

func TestSessionStartProfileUsesStableTreeNotUntrackedFiles(t *testing.T) {
	root := testRepository(t)
	for _, relative := range []string{"testing/features.yaml", "testing/scenarios.yaml"} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("items: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := HandleEvent(request(root, "session-start", `{}`))
	if err != nil {
		t.Fatal(err)
	}
	context := result["context"].(map[string]any)
	if context["profile"] != "generic" {
		t.Fatalf("untracked profile = %v, want generic", context["profile"])
	}
	runGit(t, root, "add", "testing")
	runGit(t, root, "commit", "-qm", "add profile")
	result, err = HandleEvent(request(root, "session-start", `{}`))
	if err != nil {
		t.Fatal(err)
	}
	context = result["context"].(map[string]any)
	if context["profile"] != "beamfall" {
		t.Fatalf("committed profile = %v, want beamfall", context["profile"])
	}
}

func TestLinkedWorktreeMatchesPythonOracle(t *testing.T) {
	primary := testRepository(t)
	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, primary, "worktree", "add", "-q", "--detach", linked, "HEAD")
	metadata, err := os.Stat(filepath.Join(linked, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if !metadata.Mode().IsRegular() {
		t.Fatalf("linked-worktree .git mode = %s, want regular file", metadata.Mode())
	}
	result, err := HandleEvent(request(linked, "session-start", `{}`))
	if err != nil {
		t.Fatal(err)
	}
	repository := result["repository"].(map[string]any)
	if repository["worktreeState"] != "clean" || repository["commitRevision"] == "" || repository["treeRevision"] == "" {
		t.Fatalf("linked-worktree repository=%#v", repository)
	}
}

func TestSHA256RepositoryMatchesPythonOracle(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "init", "-q", "--object-format=sha256", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("Git SHA-256 repositories are unavailable: %v (%s)", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "config", "user.email", "corvint@example.test")
	runGit(t, root, "config", "user.name", "Corvint Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "initial")

	result, err := HandleEvent(request(root, "session-start", `{}`))
	if err != nil {
		t.Fatal(err)
	}
	repository := result["repository"].(map[string]any)
	if repository["objectFormat"] != "sha256" {
		t.Fatalf("object format = %v, want sha256", repository["objectFormat"])
	}
	if len(repository["commitRevision"].(string)) != 64 || len(repository["treeRevision"].(string)) != 64 {
		t.Fatalf("invalid SHA-256 identity: %#v", repository)
	}
}

func TestProbeDisablesRepositoryConfiguredFSMonitor(t *testing.T) {
	root := testRepository(t)
	marker := filepath.Join(root, "fsmonitor-ran")
	hook := filepath.Join(root, "hostile-fsmonitor.sh")
	script := "#!/bin/sh\nprintf invoked > " + marker + "\nprintf '2\\n\\n'\n"
	if err := os.WriteFile(hook, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "config", "core.fsmonitor", hook)
	if _, err := HandleEvent(request(root, "session-start", `{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("repository-configured fsmonitor executed: %v", err)
	}
}

func TestRepositoryConfiguredExcludesMatchHardenedPythonOracle(t *testing.T) {
	root := testRepository(t)
	excludes := filepath.Join(t.TempDir(), "global-ignore")
	if err := os.WriteFile(excludes, []byte("ignored.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "config", "core.excludesFile", excludes)
	if err := os.WriteFile(filepath.Join(root, "ignored.txt"), []byte("must remain visible\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := HandleEvent(request(root, "session-start", `{}`))
	if err != nil {
		t.Fatal(err)
	}
	repository := result["repository"].(map[string]any)
	if repository["worktreeState"] != "mixed" || repository["dirtyPathCount"] != 1 {
		t.Fatalf("configured-excludes repository=%#v", repository)
	}
}

func TestReceiptRecomputesFromNormalizedInputAndExactRepository(t *testing.T) {
	root := testRepository(t)
	result, err := HandleEvent(request(root, "stop", `{}`))
	if err != nil {
		t.Fatal(err)
	}
	basis := map[string]any{
		"adapter":    result["adapter"],
		"event":      "stop",
		"input":      map[string]any{"changedPaths": []any{}, "stopHookActive": false},
		"repository": result["repository"],
	}
	encoded, err := CanonicalJSON(basis)
	if err != nil {
		t.Fatal(err)
	}
	if want := ReceiptPrefix + sha256Hex(encoded); result["receiptId"] != want {
		t.Fatalf("receipt = %v, want %s", result["receiptId"], want)
	}
	if result["support"] != "FALLBACK" || result["mutates"] != false {
		t.Fatalf("unsafe support envelope: %#v", result)
	}
}

func TestUnsupportedIndexEventsFailBeforeRepositoryWork(t *testing.T) {
	missingRoot := filepath.Join(t.TempDir(), "missing")
	for _, test := range []struct{ event, input string }{
		{"user-prompt", `{"task":"secret"}`},
		{"file-change", `{"paths":["src/main.go"]}`},
		{"session-start", `{"startSource":"compact"}`},
	} {
		_, err := HandleEvent(request(missingRoot, test.event, test.input))
		if code := kernelCode(err); code != "unsupported-harness-event" {
			t.Fatalf("%s code = %q, want unsupported-harness-event (%v)", test.event, code, err)
		}
	}
}

func TestStrictBoundedInputAndClosedMetadata(t *testing.T) {
	root := testRepository(t)
	tests := []struct {
		name, event, input, code string
	}{
		{"duplicate-top", "stop", `{"stopHookActive":true,"stopHookActive":false}`, "invalid-harness-input"},
		{"duplicate-nested", "post-tool", `{"verification":[{"commandSha256":"` + strings.Repeat("a", 64) + `","status":"passed","status":"failed"}]}`, "invalid-harness-input"},
		{"raw-tool", "post-tool", `{"toolOutput":"secret"}`, "invalid-harness-input"},
		{"unnormalized-path", "stop", `{"changedPaths":["src/../secret"]}`, "invalid-harness-input"},
		{"invalid-number", "stop", `{"stopHookActive":1}`, "invalid-harness-input"},
		{"non-object", "stop", `[]`, "invalid-harness-input"},
		{"unpaired-high-surrogate", "post-tool", `{"observedEvidenceHandles":["\ud800"]}`, "invalid-harness-input"},
		{"unpaired-low-surrogate", "post-tool", `{"observedEvidenceHandles":["\udfff"]}`, "invalid-harness-input"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := HandleEvent(request(root, test.event, test.input))
			if code := kernelCode(err); code != test.code {
				t.Fatalf("code = %q, want %q (%v)", code, test.code, err)
			}
		})
	}
	tooLarge := request(root, "stop", `{}`)
	tooLarge.Input = bytes.Repeat([]byte{' '}, MaxInputBytes+1)
	if _, err := HandleEvent(tooLarge); kernelCode(err) != "harness-input-too-large" {
		t.Fatalf("oversize code = %q (%v)", kernelCode(err), err)
	}
}

func TestMalformedStatusErrorsMatchPythonOracle(t *testing.T) {
	tests := []struct {
		name, raw, message string
	}{
		{"missing-terminal-nul", "?? path.py", "Git status output is malformed"},
		{"extra-terminal-nul", "?? path.py\x00\x00", "Git status output is malformed"},
		{"empty-rename-source", "R  renamed.py\x00\x00", "Git status output contains an empty path"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseDirtyPaths([]byte(test.raw))
			if err == nil || err.Error() != test.message {
				t.Fatalf("error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestCanonicalJSONMatchesProtocolSpelling(t *testing.T) {
	value := map[string]any{
		"z": "café\u2028", "a": map[string]any{"β": true, "a": false},
		"literal": `\u2028`, "slashThenSeparator": "\\\u2029",
	}
	encoded, err := CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"a":{"a":false,"β":true},"literal":"\\u2028","slashThenSeparator":"\\` + "\u2029" + `","z":"café` + "\u2028" + `"}`)
	if !bytes.Equal(encoded, want) {
		t.Fatalf("canonical = %q, want %q", encoded, want)
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
}

func TestNonIndexLifecyclePerformanceHook(t *testing.T) {
	if testing.Short() {
		t.Skip("performance hook")
	}
	root := testRepository(t)
	started := time.Now()
	for index := 0; index < 3; index++ {
		if _, err := HandleEvent(request(root, "session-start", `{}`)); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("three cold-process Git probes: %s", time.Since(started))
}

func BenchmarkHandleSessionStart(b *testing.B) {
	root := testRepository(b)
	req := request(root, "session-start", `{}`)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := HandleEvent(req); err != nil {
			b.Fatal(err)
		}
	}
}

// TestHandleEventRefusesIndexBackedEventsWithoutAProvider pins the fail-closed
// half of the injection: this package cannot build an index itself, so an event
// that needs one must refuse rather than emit a receipt without it.
func TestHandleEventRefusesIndexBackedEventsWithoutAProvider(t *testing.T) {
	root := testRepository(t)
	cases := []struct{ name, event, input string }{
		{"user-prompt", "user-prompt", `{"task":"anything"}`},
		{"file-change", "file-change", `{"paths":["README.md"]}`},
		{"compact-session-start", "session-start", `{"startSource":"compact"}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result, err := HandleEvent(request(root, test.event, test.input))
			if err == nil {
				t.Fatalf("expected a refusal, got %#v", result)
			}
			if code := kernelCode(err); code != "unsupported-harness-event" {
				t.Fatalf("unexpected code %q", code)
			}
		})
	}
}

// TestNonCompactSessionStartNeedsNoProvider keeps the refusal above narrow: the
// three other start sources carry no index-backed context and must still answer.
func TestNonCompactSessionStartNeedsNoProvider(t *testing.T) {
	root := testRepository(t)
	for _, source := range []string{"clear", "resume", "startup"} {
		t.Run(source, func(t *testing.T) {
			if _, err := HandleEvent(request(root, "session-start", `{"startSource":"`+source+`"}`)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
