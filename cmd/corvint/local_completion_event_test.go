package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/localcompletion"
)

func dogfoodEventArguments(event string) []string {
	return []string{"--host", "codex", "--host-version", "unknown", "--surface", "plugin", "--adapter-version", "0.1.0", "--event", event, "--input", "-", "--budget-bytes", "8000"}
}

func TestDogfoodEventStopLifecycle(t *testing.T) {
	t.Parallel()
	t.Run("LCP-V0-008 stop", func(t *testing.T) {
		for _, test := range []struct {
			lifecycle string
			satisfied bool
			active    bool
			decision  string
			reason    string
		}{
			{"inactive", false, false, "release", "local-policy-inactive"},
			{"cancelled", false, false, "release", "local-policy-cancelled"},
			{"satisfied", true, false, "release", "local-policy-satisfied"},
			{"active", false, false, "block", "local-policy-incomplete"},
			{"active", false, true, "release", "local-policy-continuation-limit"},
			{"satisfied", false, false, "block", "local-policy-incomplete"},
			{"satisfied", false, true, "release", "local-policy-continuation-limit"},
		} {
			got := dogfoodCompletion("stop", map[string]any{"stopHookActive": test.active}, localcompletion.Evaluation{Lifecycle: test.lifecycle, Satisfied: test.satisfied})
			if got["decision"] != test.decision || got["reason"] != test.reason {
				t.Fatalf("%+v: %#v", test, got)
			}
		}
	})
}

func TestDogfoodEventStrictInputAndDeadline(t *testing.T) {
	t.Parallel()
	t.Run("LCP-V0-012 bound", func(t *testing.T) {
		for _, raw := range []string{`{"stopHookActive":true,"stopHookActive":false}`, `{"stopHookActive":1}`, `{"changedPaths":["../escape"]}`, `{"sessionIdSha256":"raw-private-id"}`, `{"transcript":"private"}`, `[]`, `{}`, `{"task":""}`} {
			event := "stop"
			if raw == `{}` || raw == `{"task":""}` {
				event = "user-prompt"
			}
			if _, err := dogfoodEventInput(event, []byte(raw)); err == nil {
				t.Fatalf("accepted %s", raw)
			}
		}
		if _, err := dogfoodEventInput("stop", []byte(strings.Repeat(" ", gokernel.MaxInputBytes+1))); err == nil {
			t.Fatal("accepted oversized input")
		}
	})
	t.Run("LCP-V0-009 envelope", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var stdout, stderr bytes.Buffer
		start := time.Now()
		status := runLocalCompletionEvent(ctx, t.TempDir(), dogfoodEventArguments("stop"), strings.NewReader(`{}`), &stdout, &stderr)
		if status != 2 || stdout.Len() != 0 || time.Since(start) > time.Second {
			t.Fatalf("cancelled event: %d %s %s", status, &stdout, &stderr)
		}
		for _, args := range [][]string{
			append(dogfoodEventArguments("stop"), "--host", "claude-code"),
			append(dogfoodEventArguments("stop"), "--budget-bytes", "8001"),
		} {
			stdout.Reset()
			if runLocalCompletionEvent(context.Background(), t.TempDir(), args, strings.NewReader(`{}`), &stdout, &stderr) != 2 || stdout.Len() != 0 {
				t.Fatal("unsupported tuple/budget returned a successful envelope")
			}
		}
	})
	t.Run("LCP-V0-008 deadline", func(t *testing.T) {
		root := queryCLIRepository(t)
		expired := false
		for deadline := time.Millisecond; deadline < time.Minute; deadline *= 2 {
			ctx := context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return deadline })
			var stdout, stderr bytes.Buffer
			if runLocalCompletionEvent(ctx, root, dogfoodEventArguments("session-start"), strings.NewReader(`{}`), &stdout, &stderr) == 0 {
				break
			}
			// The repository has no snapshot, so expiry in its in-memory build names that (AHI-031).
			stale := strings.Contains(stderr.String(), `"dogfood-event-index-snapshot-stale"`)
			if !stale && !strings.Contains(stderr.String(), `"dogfood-event-deadline"`) && !strings.Contains(stderr.String(), `"dogfood-event-input-unavailable"`) {
				t.Fatalf("%s expiry was not reported as a time bound: %s", deadline, &stderr)
			}
			expired = expired || stale || strings.Contains(stderr.String(), `"dogfood-event-deadline"`)
		}
		if !expired {
			t.Fatal("no deadline expired during the repository read")
		}
	})
	t.Run("LCP-V0-008 expiry does not wait for an uncancellable read", func(t *testing.T) {
		release := make(chan struct{})
		defer close(release)
		ctx := context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return 10 * time.Millisecond })
		// Like the in-memory index compile, this read never observes cancellation.
		ctx = context.WithValue(ctx, dogfoodEventReadKey{}, func(context.Context, options, map[string]any) (map[string]any, error) {
			<-release
			return nil, errors.New("released")
		})
		root := queryCLIRepository(t)
		returned := make(chan string, 1)
		go func() {
			var stdout, stderr bytes.Buffer
			runLocalCompletionEvent(ctx, root, dogfoodEventArguments("session-start"), strings.NewReader(`{}`), &stdout, &stderr)
			returned <- stderr.String()
		}()
		select {
		case stderr := <-returned:
			if !strings.Contains(stderr, `"dogfood-event-deadline"`) {
				t.Fatalf("expired event was not reported as a time bound: %s", stderr)
			}
		case <-time.After(time.Minute): // hang detector, not a latency budget (decision 0082)
			t.Fatal("expired event waited for a read that ignores cancellation")
		}
	})
}

// AHI-031: an event whose deadline expires in the in-memory build of a snapshot miss names the
// stale snapshot, not the bare deadline; once the snapshot is refreshed the event needs no build.
func TestDogfoodEventSnapshotMissExpiryNamesStaleSnapshot(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	ctx := context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return 3 * time.Second })
	ctx = context.WithValue(ctx, dogfoodEventBuildKey{}, func(ctx context.Context, _, _ string) (*contextindex.Index, error) {
		<-ctx.Done() // outlasts the event deadline, as the real build does on a large repository
		return nil, ctx.Err()
	})
	var stdout, stderr bytes.Buffer
	if runLocalCompletionEvent(ctx, root, dogfoodEventArguments("session-start"), strings.NewReader(`{}`), &stdout, &stderr) != 2 || !strings.Contains(stderr.String(), `"dogfood-event-index-snapshot-stale"`) {
		t.Fatalf("snapshot-miss expiry did not name the stale snapshot: %s", &stderr)
	}
	stderr.Reset()
	if runContext(context.Background(), []string{"--root", root, "index", "--if-stale"}, strings.NewReader(""), io.Discard, &stderr) != 0 {
		t.Fatalf("index --if-stale: %s", &stderr)
	}
	// A minute and a refused build, not the 3-second bound: this verifies the snapshot hit, not
	// latency, so a loaded host cannot expire it (decision 0082).
	refreshed := context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return time.Minute })
	refreshed = context.WithValue(refreshed, dogfoodEventBuildKey{}, func(context.Context, string, string) (*contextindex.Index, error) {
		return nil, errors.New("refreshed snapshot started a build")
	})
	if runLocalCompletionEvent(refreshed, root, dogfoodEventArguments("session-start"), strings.NewReader(`{}`), &stdout, &stderr) != 0 {
		t.Fatalf("refreshed snapshot still degraded: %s", &stderr)
	}
}

// staleSnapshotRepository is a repository whose explicit `index` run recorded its build cost and
// whose tree then moved past that snapshot, so the next event misses.
func staleSnapshotRepository(t *testing.T) string {
	t.Helper()
	root := queryCLIRepository(t)
	runIndexForTest(t, root, false)
	if _, recorded := contextindex.RecordedBuildCost(root); !recorded {
		t.Fatal("index recorded no build cost")
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "parser", "lexer.go"), []byte("package parser\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, root, "add", ".")
	gitFixture(t, root, "commit", "-qm", "move the tree past the snapshot")
	return root
}

// IDX-SNAP-V0-012, AHI-031: a snapshot miss whose recorded `index` build cost does not fit the
// time left is decided from that record without starting the in-memory build; where the
// recorded cost fits, as on a small repository, the miss still builds within the deadline.
func TestDogfoodEventSnapshotMissUsesRecordedBuildCost(t *testing.T) {
	t.Parallel()
	// A minute, not the production deadline: these cases verify the decision, not latency (decision 0082).
	deadline := context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return time.Minute })
	t.Run("recorded cost outlasts the deadline", func(t *testing.T) {
		t.Parallel()
		root := staleSnapshotRepository(t)
		if err := contextindex.RecordBuildCost(root, time.Hour); err != nil {
			t.Fatal(err)
		}
		built := new(atomic.Bool)
		ctx := context.WithValue(deadline, dogfoodEventBuildKey{}, func(ctx context.Context, _, _ string) (*contextindex.Index, error) {
			built.Store(true)
			<-ctx.Done()
			return nil, ctx.Err()
		})
		var stdout, stderr bytes.Buffer
		started := time.Now()
		code := runLocalCompletionEvent(ctx, root, dogfoodEventArguments("session-start"), strings.NewReader(`{}`), &stdout, &stderr)
		t.Logf("miss decided in %s", time.Since(started))
		if code != 2 || !strings.Contains(stderr.String(), `"dogfood-event-index-snapshot-stale"`) || built.Load() {
			t.Fatalf("miss was not decided from the recorded cost: code=%d built=%t %s", code, built.Load(), &stderr)
		}
		if cost, _ := contextindex.RecordedBuildCost(root); cost != time.Hour {
			t.Fatalf("the hook rewrote the build-cost record: %s", cost)
		}
	})
	t.Run("recorded cost fits the deadline", func(t *testing.T) {
		t.Parallel()
		root := staleSnapshotRepository(t)
		// A fixed cost, not the fixture's measured one, so a loaded host cannot turn this into a skip.
		if err := contextindex.RecordBuildCost(root, time.Millisecond); err != nil {
			t.Fatal(err)
		}
		built := new(atomic.Bool)
		ctx := context.WithValue(deadline, dogfoodEventBuildKey{}, func(ctx context.Context, root, subject string) (*contextindex.Index, error) {
			built.Store(true)
			return contextindex.BuildContext(ctx, root, subject)
		})
		var stdout, stderr bytes.Buffer
		if code := runLocalCompletionEvent(ctx, root, dogfoodEventArguments("session-start"), strings.NewReader(`{}`), &stdout, &stderr); code != 0 || !built.Load() {
			t.Fatalf("small-repository miss did not build: code=%d built=%t %s", code, built.Load(), &stderr)
		}
	})
}

func TestDogfoodEventDeadlineBelowDeclaredHostKill(t *testing.T) {
	t.Parallel()
	t.Run("LCP-V0-009 kill ordering", func(t *testing.T) {
		root, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		hookName := map[string]string{"session-start": "SessionStart", "user-prompt": "UserPromptSubmit", "stop": "Stop", "session-end": "SessionEnd"}
		hooksPath := map[string]string{
			"claude-code": filepath.Join(root, "integrations", "claude-code", "plugins", "corvint", "hooks", "hooks.json"),
			"codex":       filepath.Join(root, "integrations", "codex", "plugins", "corvint", "hooks", "hooks.json"),
		}
		for host, path := range hooksPath {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var parsed struct {
				Hooks map[string][]struct {
					Hooks []struct {
						Timeout float64 `json:"timeout"`
					} `json:"hooks"`
				} `json:"hooks"`
			}
			if err := json.Unmarshal(raw, &parsed); err != nil {
				t.Fatal(err)
			}
			for event, name := range hookName {
				entries, ok := parsed.Hooks[name]
				if !ok || len(entries) == 0 || len(entries[0].Hooks) == 0 {
					t.Fatalf("%s: no declared %s hook", host, name)
				}
				kill := time.Duration(entries[0].Hooks[0].Timeout * float64(time.Second))
				if deadline := dogfoodEventDeadlineFor(host, event); deadline >= kill {
					t.Fatalf("%s %s: deadline %s not strictly below declared host kill %s", host, event, deadline, kill)
				}
			}
		}
	})
}

func TestDogfoodEventFailedPolicyReReadIsNotDrift(t *testing.T) {
	t.Parallel()
	t.Run("LCP-V0-008 failed policy re-read", func(t *testing.T) {
		root := queryCLIRepository(t)
		key := localcompletion.HashSession("reread-session")
		corrupt := func(_ context.Context, _ options, _ map[string]any, _ localcompletion.Evaluation, _ map[string]any, _ gokernel.Repository) (map[string]any, error) {
			directory := filepath.Join(root, ".git", "corvint", "local-completion", key)
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "state.json"), []byte("{"), 0600); err != nil {
				t.Fatal(err)
			}
			return map[string]any{}, nil
		}
		input := map[string]any{"sessionIdSha256": key}
		_, err := localEventRead(context.Background(), options{root: root, event: "session-start"}, input, dogfoodEventEnvelope, corrupt, "")
		if err == nil {
			t.Fatal("unreadable policy re-read accepted")
		}
		var kernelErr *gokernel.Error
		if errors.As(err, &kernelErr) && kernelErr.Code == "dogfood-event-policy-drift" {
			t.Fatalf("failed policy re-read labelled drift: %v", err)
		}
	})
}

func TestDogfoodEventGoPythonWireAndSession(t *testing.T) {
	t.Run("LCP-V0-009 envelope", func(t *testing.T) {
		repo := gokernel.Repository{CommitRevision: strings.Repeat("a", 40), TreeRevision: strings.Repeat("b", 40), ObjectFormat: "sha1", DirtyPathsSHA: dogfoodSHA([]byte("[]")), WorktreeState: "clean"}
		options := options{host: "codex", hostVersion: "unknown", surface: "plugin", adapterVersion: "0.1.0", event: "stop"}
		session := localcompletion.HashSession("native-session-é")
		input := map[string]any{"sessionIdSha256": session, "stopHookActive": false, "changedPaths": []any{}}
		result := dogfoodEventEnvelope(options, input, repo, localcompletion.Evaluation{Lifecycle: "active", Base: strings.Repeat("c", 40), Target: repo.CommitRevision, PlanDigest: strings.Repeat("d", 64), Unmet: []string{"selected-check-unverified:private-check"}})
		encoded, err := dogfoodEventBytes(result, 8000)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(encoded, []byte("private-check")) {
			t.Fatal("caller check ID leaked through automatic diagnostics")
		}
		if got := hashAdapterSession("corvint-local-completion-session/0", "native-session-é"); got != session {
			t.Fatalf("adapter session digest=%s want=%s", got, session)
		}
		host := renderAdapterResult("codex", "Stop", "stop", "", input, result)
		if host["decision"] != "block" || !strings.Contains(host["reason"].(string), "Frontier authority remains unavailable") {
			t.Fatalf("native Stop output=%v", host)
		}
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		originalDigest := decoded["resultDigest"]
		decoded["policy"].(map[string]any)["satisfied"] = true
		changed, err := dogfoodEventBytes(decoded, 8000)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(changed, []byte(originalDigest.(string))) {
			t.Fatal("result digest did not cover policy answer")
		}
		if _, err := dogfoodEventBytes(result, len(encoded)-1); err == nil {
			t.Fatal("complete response budget excluded terminating LF")
		}
	})
	t.Run("LCP-V0-002 enrollment", func(t *testing.T) {
		t.Setenv("CODEX_THREAD_ID", "native-session-é")
		key, err := localcompletion.SessionKey("")
		if err != nil || key != localcompletion.HashSession("native-session-é") {
			t.Fatalf("CLI session domain: %s %v", key, err)
		}
	})
}

func TestCodexHostAdapterNormalizesAndFramesNativeEvent(t *testing.T) {
	root := queryCLIRepository(t)
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")
	payload := map[string]any{
		"hook_event_name": "SessionStart", "cwd": root,
		"session_id": "native-session-é", "source": "startup",
		"transcript_path": "/private/transcript", "prompt": "private",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if status := runContext(lifecycleDeadlineContext(), []string{"adapter", "codex"}, bytes.NewReader(raw), &stdout, io.Discard); status != 0 {
		t.Fatalf("status=%d output=%s", status, &stdout)
	}
	if stdout.Len() > adapterOutputLimit || bytes.Contains(stdout.Bytes(), []byte("private/transcript")) || bytes.Contains(stdout.Bytes(), []byte(`"prompt":"private"`)) {
		t.Fatalf("unbounded or private output: %s", &stdout)
	}
	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	specific, ok := output["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("host frame=%v", output)
	}
	contextText, ok := specific["additionalContext"].(string)
	if !ok {
		t.Fatalf("host frame=%v", output)
	}
	if specific["hookEventName"] != "SessionStart" || !strings.HasPrefix(contextText, untrustedDataPrefix) || !strings.HasSuffix(contextText, untrustedDataSuffix) {
		t.Fatalf("host frame=%v", output)
	}
}

func TestDogfoodEventReadOnlyEnrolledStopAndPrompt(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "specs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "specs", "event.md"), []byte("# Event specification\n\n- `LCP-TEST-001`: Preserve an explicit requirement anchor.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", "docs/specs/event.md")
	gitOutput(t, root, "commit", "-qm", "add requirement fixture")
	key := localcompletion.HashSession("event-session")
	base := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	plan, err := json.Marshal(localcompletion.Plan{Base: base, Intents: []string{"AGENTS.md"}, Checks: []localcompletion.Check{{ID: "check", Argv: []string{"false"}, TimeoutSeconds: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localcompletion.Begin(context.Background(), root, key, plan); err != nil {
		t.Fatal(err)
	}
	t.Run("LCP-V0-003 readonly", func(t *testing.T) {
		before := dogfoodPrivateFiles(t, root)
		for _, test := range []struct{ event, input string }{
			{"stop", `{"sessionIdSha256":"` + key + `","stopHookActive":false}`},
			{"stop", `{"sessionIdSha256":"` + key + `","stopHookActive":true}`},
			{"user-prompt", `{"sessionIdSha256":"` + key + `","task":"what about it"}`},
			{"user-prompt", `{"sessionIdSha256":"` + key + `","task":"AGENTS.md"}`},
			{"user-prompt", `{"sessionIdSha256":"` + key + `","task":"ParseToken"}`},
			{"user-prompt", `{"sessionIdSha256":"` + key + `","task":"LCP-TEST-001"}`},
			{"session-start", `{"sessionIdSha256":"` + key + `","startSource":"compact"}`},
		} {
			var stdout, stderr bytes.Buffer
			if status := runLocalCompletionEvent(lifecycleDeadlineContext(), root, dogfoodEventArguments(test.event), strings.NewReader(test.input), &stdout, &stderr); status != 0 {
				t.Fatalf("%s: %d %s", test.event, status, &stderr)
			}
			if stdout.Len() > 8000 || bytes.Contains(stdout.Bytes(), []byte("what about it")) {
				t.Fatalf("unbounded or raw prompt output: %s", &stdout)
			}
			var result map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			dogfoodNativeValidate(t, test.event, test.input, stdout.Bytes())
			if test.event == "user-prompt" && !strings.Contains(test.input, "what about it") {
				packet := result["context"].(map[string]any)
				if len(packet["task_evidence"].([]any)) == 0 {
					t.Fatalf("explicit anchor lost: %s %s", test.input, &stdout)
				}
			}
			if result["policy"].(map[string]any)["satisfied"] != false {
				t.Fatal("unverified workflow satisfied")
			}
		}
		if !reflect.DeepEqual(before, dogfoodPrivateFiles(t, root)) {
			t.Fatal("automatic events mutated private or repository state")
		}
	})
	t.Run("LCP-V0-001 authority", func(t *testing.T) {
		request := gokernel.EventRequest{Root: root, Host: "codex", HostVersion: "unknown", Surface: "plugin", AdapterVersion: "0.1.0", Event: "stop", Input: []byte(`{}`), BudgetBytes: 8000}
		before, err := gokernel.HandleEvent(request)
		if err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if status := runLocalCompletionEvent(lifecycleDeadlineContext(), root, dogfoodEventArguments("stop"), strings.NewReader(`{"sessionIdSha256":"`+key+`"}`), &stdout, &stderr); status != 0 {
			t.Fatalf("event: %d %s", status, &stderr)
		}
		after, err := gokernel.HandleEvent(request)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("legacy harness changed: %v", err)
		}
		frontier := after["frontier"].(map[string]any)
		if frontier["state"] != "UNAVAILABLE" || frontier["shouldContinue"] != false {
			t.Fatal("legacy authority changed")
		}
	})
}

func dogfoodNativeValidate(t *testing.T, event, input string, output []byte) {
	t.Helper()
	var result, normalized map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(input), &normalized); err != nil {
		t.Fatal(err)
	}
	names := map[string]string{"user-prompt": "UserPromptSubmit", "session-start": "SessionStart", "stop": "Stop"}
	native := renderAdapterResult("codex", names[event], event, "", normalized, result)
	encoded, err := json.Marshal(native)
	if err != nil || len(encoded)+1 > adapterOutputLimit {
		t.Fatalf("native %s response: %v %d", event, err, len(encoded)+1)
	}
	if event != "stop" {
		context := native["hookSpecificOutput"].(map[string]any)["additionalContext"].(string)
		if !strings.HasPrefix(context, untrustedDataPrefix) || !strings.HasSuffix(context, untrustedDataSuffix) {
			t.Fatalf("untrusted-data framing missing: %s", context)
		}
	}
}

func dogfoodPrivateFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = dogfoodSHA(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
