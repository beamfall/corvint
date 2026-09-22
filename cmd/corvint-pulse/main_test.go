package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/pulse"
)

type fakeActor struct {
	generation uint64
	shutdown   bool
}

func (a *fakeActor) Invalidate(_ context.Context, expected uint64) (uint64, bool, error) {
	if a.shutdown {
		return a.generation, false, errors.New("actor stopped")
	}
	if expected != a.generation {
		return a.generation, false, nil
	}
	a.generation++
	return a.generation, true, nil
}

func (a *fakeActor) Shutdown(context.Context) error {
	a.shutdown = true
	return nil
}

func fakeFactory(string, string) (workspaceActor, error) { return &fakeActor{}, nil }

func fixedSession() (string, error) { return "0123456789abcdef0123456789abcdef", nil }

type corpus struct {
	Cases []struct {
		ExpectedExitCode int      `json:"expectedExitCode"`
		ExpectedLines    []string `json:"expectedLines"`
		ID               string   `json:"id"`
		RequestLines     []string `json:"requestLines"`
	} `json:"cases"`
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func loadCorpus(t *testing.T) corpus {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), "conformance", "pulse-snapshot-v0", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result corpus
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFrozenP0ATranscripts(t *testing.T) {
	document := loadCorpus(t)
	if len(document.Cases) != 12 {
		t.Fatalf("cases = %d", len(document.Cases))
	}
	for _, test := range document.Cases {
		t.Run(test.ID, func(t *testing.T) {
			input := strings.Join(test.RequestLines, "\n") + "\n"
			var stdout bytes.Buffer
			if err := serve(context.Background(), "", strings.NewReader(input), &stdout, fakeFactory, fixedSession); err != nil {
				t.Fatalf("serve: %v", err)
			}
			expected := strings.Join(test.ExpectedLines, "\n") + "\n"
			expected = strings.ReplaceAll(expected, "$SESSION", "0123456789abcdef0123456789abcdef")
			if stdout.String() != expected {
				t.Fatalf("stdout mismatch\nactual:   %s\nexpected: %s", &stdout, expected)
			}
		})
	}
}

func TestSessionIDsAreFreshBoundedTokens(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9a-f]{32}$`)
	seen := make(map[string]struct{})
	for range 100 {
		value, err := freshSessionID()
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(value) {
			t.Fatalf("invalid session token %q", value)
		}
		if _, duplicate := seen[value]; duplicate {
			t.Fatalf("duplicate session token %q", value)
		}
		seen[value] = struct{}{}
	}
}

func TestDecodeRequestRejectsNoncanonicalAndDeepJSON(t *testing.T) {
	valid := []byte(`{"body":{},"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"r1","type":"initialize"}`)
	if _, err := decodeRequest(valid); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	cases := [][]byte{
		[]byte(`{"body":{}, "clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"r1","type":"initialize"}`),
		[]byte(`{"body":{},"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"r1","type":"initialize","type":"shutdown"}`),
		[]byte(`{"body":{},"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"r1","type":"initialize"}x`),
		[]byte(`{"body":{},"clientSeq":1.0,"protocol":"corvint-pulse-snapshot/0","requestId":"r1","type":"initialize"}`),
		[]byte(`{"body":{"value":1e0},"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"r1","type":"initialize"}`),
		[]byte(`{"body":{"value":-0},"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"r1","type":"initialize"}`),
		append([]byte(`{"body":`), append(bytes.Repeat([]byte{'['}, maxJSONDepth+2), append(bytes.Repeat([]byte{']'}, maxJSONDepth+2), []byte(`,"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"r1","type":"initialize"}`)...)...)...),
	}
	for index, raw := range cases {
		if _, err := decodeRequest(raw); err == nil {
			t.Fatalf("case %d accepted", index)
		}
	}
}

func TestDecodeRequestReportsTypedResourceLimits(t *testing.T) {
	deep := append([]byte(`{"body":`), append(bytes.Repeat([]byte{'['}, maxJSONDepth+2), append(bytes.Repeat([]byte{']'}, maxJSONDepth+2), []byte(`,"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"r1","type":"initialize"}`)...)...)...)
	if _, protocolErr := decodeRequest(deep); protocolErr == nil || protocolErr.code != "resource-exhausted" {
		t.Fatalf("deep request error = %#v", protocolErr)
	}
	if _, protocolErr := decodeRequest(bytes.Repeat([]byte{'x'}, maxFrameBytes)); protocolErr == nil || protocolErr.code != "resource-exhausted" {
		t.Fatalf("oversized request error = %#v", protocolErr)
	}
	if _, protocolErr := decodeRequest([]byte{'{', 0xff, '}'}); protocolErr == nil || protocolErr.code != "invalid-request" {
		t.Fatalf("invalid UTF-8 error = %#v", protocolErr)
	}
}

func TestProtocolSafeIdentifiersAndPaths(t *testing.T) {
	unsafeID := []byte(`{"body":{},"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"bad\u0000id","type":"initialize"}`)
	if _, protocolErr := decodeRequest(unsafeID); protocolErr == nil || protocolErr.requestID != nil {
		t.Fatalf("unsafe request ID error = %#v", protocolErr)
	}
	for _, path := range []string{"C:/secret", "a/\x00b", "a/\nb", "/absolute", `a\b`} {
		if _, ok, exhausted := normalizedPaths([]any{path}); ok || exhausted {
			t.Fatalf("unsafe path accepted: %q", path)
		}
	}
	items := make([]any, maxPaths+1)
	if _, ok, exhausted := normalizedPaths(items); ok || !exhausted {
		t.Fatal("path-count exhaustion was not typed")
	}
	var stdout bytes.Buffer
	s := &server{
		actor:       &fakeActor{},
		initialized: true,
		output:      &stdout,
		sessionID:   "0123456789abcdef0123456789abcdef",
	}
	err := s.invalidate(context.Background(), request{
		body: map[string]any{
			"expectedGeneration": json.Number("0"),
			"paths":              items,
			"reason":             "tool-write",
		},
		requestID: "paths",
	})
	if err != nil || !strings.Contains(stdout.String(), `"code":"resource-exhausted"`) {
		t.Fatalf("path-count response = (%v, %q)", err, &stdout)
	}
}

func TestOversizedFrameReturnsTypedErrorAndStops(t *testing.T) {
	input := append(bytes.Repeat([]byte{'x'}, maxFrameBytes), '\n')
	var stdout bytes.Buffer
	if err := serve(context.Background(), "", bytes.NewReader(input), &stdout, fakeFactory, fixedSession); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"code":"resource-exhausted"`) {
		t.Fatalf("stdout=%q", &stdout)
	}
}

func TestIncompleteFrameFailsClosed(t *testing.T) {
	if err := serve(context.Background(), "", strings.NewReader(`{"body":{}`), io.Discard, fakeFactory, fixedSession); err == nil {
		t.Fatal("incomplete frame accepted")
	}
}

func TestServeRejectsReusedRequestIDWithoutConsumingSequence(t *testing.T) {
	input := strings.Join([]string{
		`{"body":{"clientBootId":"boot","clientId":"client"},"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"i1","type":"initialize"}`,
		`{"body":{},"clientSeq":2,"protocol":"corvint-pulse-snapshot/0","requestId":"dup","type":"snapshot.acquire"}`,
		`{"body":{},"clientSeq":3,"protocol":"corvint-pulse-snapshot/0","requestId":"dup","type":"shutdown"}`,
		`{"body":{},"clientSeq":3,"protocol":"corvint-pulse-snapshot/0","requestId":"z1","type":"shutdown"}`,
	}, "\n") + "\n"
	var stdout bytes.Buffer
	if err := serve(context.Background(), "", strings.NewReader(input), &stdout, fakeFactory, fixedSession); err != nil {
		t.Fatal(err)
	}
	if strings.Count(stdout.String(), `"code":"duplicate-request-id"`) != 1 ||
		!strings.Contains(stdout.String(), `"requestId":"z1"`) ||
		!strings.Contains(stdout.String(), `"type":"shutdown.complete"`) {
		t.Fatalf("stdout=%q", &stdout)
	}
}

func TestRequestIDTrackingIsBounded(t *testing.T) {
	seen := make(map[string]struct{}, maxRequestIDs)
	for index := range maxRequestIDs {
		seen[strconv.Itoa(index)] = struct{}{}
	}
	var stdout bytes.Buffer
	s := &server{
		actor:          &fakeActor{},
		initialized:    true,
		output:         &stdout,
		seenRequestIDs: seen,
		sessionID:      "0123456789abcdef0123456789abcdef",
	}
	stop, err := s.handle(context.Background(), request{requestID: "fresh"})
	if err != nil || stop || !strings.Contains(stdout.String(), `"code":"resource-exhausted"`) {
		t.Fatalf("capacity result = (%v, %v, %q)", stop, err, &stdout)
	}
	stop, err = s.handle(context.Background(), request{
		body: map[string]any{}, requestID: "shutdown", typeName: "shutdown",
	})
	if err != nil || !stop || !strings.Contains(stdout.String(), `"type":"shutdown.complete"`) {
		t.Fatalf("bounded shutdown = (%v, %v, %q)", stop, err, &stdout)
	}
}

func TestServeSequenceGapLocksSessionExceptShutdown(t *testing.T) {
	input := strings.Join([]string{
		`{"body":{"clientBootId":"boot","clientId":"client"},"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"i1","type":"initialize"}`,
		`{"body":{},"clientSeq":3,"protocol":"corvint-pulse-snapshot/0","requestId":"gap","type":"snapshot.acquire"}`,
		`{"body":{},"clientSeq":2,"protocol":"corvint-pulse-snapshot/0","requestId":"blocked","type":"snapshot.acquire"}`,
		`{"body":{},"clientSeq":99,"protocol":"corvint-pulse-snapshot/0","requestId":"z1","type":"shutdown"}`,
	}, "\n") + "\n"
	var stdout bytes.Buffer
	if err := serve(context.Background(), "", strings.NewReader(input), &stdout, fakeFactory, fixedSession); err != nil {
		t.Fatal(err)
	}
	if strings.Count(stdout.String(), `"type":"workspace.unknown"`) != 1 ||
		strings.Count(stdout.String(), `"code":"session-unknown"`) != 1 ||
		strings.Contains(stdout.String(), `"code":"identity-incomplete"`) ||
		!strings.Contains(stdout.String(), `"type":"shutdown.complete"`) {
		t.Fatalf("stdout=%q", &stdout)
	}
}

func TestServeRejectsCRLFAndShortWrites(t *testing.T) {
	request := "{\"body\":{},\"clientSeq\":1,\"protocol\":\"corvint-pulse-snapshot/0\",\"requestId\":\"r1\",\"type\":\"snapshot.acquire\"}\r\n"
	var stdout bytes.Buffer
	if err := serve(context.Background(), "", strings.NewReader(request), &stdout, fakeFactory, fixedSession); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"message":"request framing must use one LF"`) {
		t.Fatalf("stdout=%q", &stdout)
	}

	writer := shortWriter{}
	valid := "{\"body\":{" +
		"\"clientBootId\":\"boot\",\"clientId\":\"client\"},\"clientSeq\":1," +
		"\"protocol\":\"corvint-pulse-snapshot/0\",\"requestId\":\"i1\",\"type\":\"initialize\"}\n"
	if err := serve(context.Background(), "", strings.NewReader(valid), writer, fakeFactory, fixedSession); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write error = %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) { return len(value) - 1, nil }

func TestRunRejectsInvalidRootWithoutStartingActor(t *testing.T) {
	started := false
	factory := func(string, string) (workspaceActor, error) {
		started = true
		return &fakeActor{}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runContext(
		context.Background(),
		[]string{"--root", filepath.Join(t.TempDir(), "missing"), "serve", "--stdio"},
		strings.NewReader(""), &stdout, &stderr, factory,
	)
	if code != 2 || started || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"invalid-arguments"`) {
		t.Fatalf("code=%d started=%v stdout=%q stderr=%q", code, started, &stdout, &stderr)
	}
}

func TestRunPreservesResolvedRootWithoutMutation(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "marker")
	if err := os.WriteFile(marker, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	var receivedRoot string
	factory := func(actorRoot, _ string) (workspaceActor, error) {
		receivedRoot = actorRoot
		return &fakeActor{}, nil
	}
	input := strings.Join([]string{
		`{"body":{"clientBootId":"boot","clientId":"client"},"clientSeq":1,"protocol":"corvint-pulse-snapshot/0","requestId":"i1","type":"initialize"}`,
		`{"body":{},"clientSeq":2,"protocol":"corvint-pulse-snapshot/0","requestId":"z1","type":"shutdown"}`,
	}, "\n") + "\n"
	if code := runContext(context.Background(), []string{"--root", root, "serve", "--stdio"}, strings.NewReader(input), io.Discard, io.Discard, factory); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "unchanged" || receivedRoot != resolved {
		t.Fatalf("root=%q content=%q err=%v", receivedRoot, content, err)
	}
}

func TestPulseActorAdapterTranslatesGeneration(t *testing.T) {
	actor, err := newWorkspaceActor(t.TempDir(), "adapter-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = actor.Shutdown(context.Background()) })

	generation, accepted, err := actor.Invalidate(context.Background(), 0)
	if err != nil || !accepted || generation != 1 {
		t.Fatalf("accepted invalidation = (%d, %v, %v)", generation, accepted, err)
	}
	generation, accepted, err = actor.Invalidate(context.Background(), 0)
	if err != nil || accepted || generation != 1 {
		t.Fatalf("conflicting invalidation = (%d, %v, %v)", generation, accepted, err)
	}
	if _, _, err := actor.Invalidate(context.Background(), math.MaxUint64); err == nil {
		t.Fatal("generation translation overflow accepted")
	}
}

func TestTranslateActorResultPreservesCoordinates(t *testing.T) {
	cause := &pulse.ActorError{
		Cause:               pulse.ErrGenerationConflict,
		WorkspaceGeneration: 4,
		ServerSequence:      9,
	}
	generation, err := translateActorResult(pulse.Update{
		WorkspaceGeneration: 4,
		ServerSequence:      9,
	}, cause)
	if generation != 3 || !errors.Is(err, pulse.ErrGenerationConflict) {
		t.Fatalf("translated result = (%d, %v)", generation, err)
	}
	var actorErr *pulse.ActorError
	if !errors.As(err, &actorErr) || actorErr.WorkspaceGeneration != 3 || actorErr.ServerSequence != 9 {
		t.Fatalf("translated actor error = %#v", err)
	}
	if _, err := translateActorResult(pulse.Update{
		WorkspaceGeneration: 4,
		ServerSequence:      10,
	}, cause); err == nil || errors.Is(err, pulse.ErrGenerationConflict) {
		t.Fatalf("inconsistent coordinates accepted: %v", err)
	}
	generation, err = translateActorResult(pulse.Update{}, cause)
	if generation != 3 || !errors.As(err, &actorErr) ||
		actorErr.WorkspaceGeneration != 3 || actorErr.ServerSequence != 9 {
		t.Fatalf("coordinate-only actor error = (%d, %#v)", generation, err)
	}
	plain := errors.New("plain actor failure")
	if _, err := translateActorResult(pulse.Update{}, plain); !errors.Is(err, plain) {
		t.Fatalf("plain actor error changed: %v", err)
	}
}
