// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/lspsnapshot"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/goplsclient"
)

func TestContextClosedParams(t *testing.T) {
	good := `{"textDocument":{"uri":"file:///repo/a.go"},"task":"investigate","limit":5}`
	for _, raw := range []string{good, strings.Replace(good, "investigate", strings.Repeat(`\u0001`, 32000), 1)} {
		input, _, err := parseContextParams([]byte(raw), "/repo")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(input)
		if len(b) > contextInputLimit {
			t.Fatal("valid escaped maximum does not fit worker input")
		}
	}
	for _, raw := range []string{`null`, `{}`, strings.Replace(good, `"task":"investigate"`, `"task":null`, 1), strings.Replace(good, `"limit":5`, `"limit":null`, 1), strings.Replace(good, `"limit":5`, `"limit":0`, 1), strings.Replace(good, `"limit":5`, `"limit":51`, 1), strings.Replace(good, `"task":"investigate"`, `"task":"a","task":"b"`, 1), strings.Replace(good, `"uri":"file:///repo/a.go"`, `"uri":"file:///repo/a.go","uri":"file:///repo/b.go"`, 1), strings.Replace(good, `"uri":"file:///repo/a.go"`, `"uri":"file:///repo/a.go","extra":1`, 1), strings.Replace(good, `"limit":5`, `"limit":5,"extra":1`, 1), strings.Replace(good, `file:///repo/a.go`, `file:///other/a.go`, 1), strings.Replace(good, `file:///repo/a.go`, `file:///repo/%61.go`, 1), strings.Replace(good, `file:///repo/a.go`, `file:///repo/x/../a.go`, 1), strings.Replace(good, `investigate`, strings.Repeat("x", 32001), 1)} {
		if _, _, err := parseContextParams([]byte(raw), "/repo"); err == nil {
			t.Errorf("accepted invalid params %.150s", raw)
		}
	}
}

func TestContextFreshnessAfterCore(t *testing.T) {
	root := contextFixture(t)
	before, e := observeRepository(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	stable := func(context.Context, string) (repositoryObservation, error) { return before, nil }
	raw := json.RawMessage(`{"repository":null,"state":"ABSTAINED","abstention":{"active":true,"reason":"NOT_TRACKED_AT_REVISION"}}`)
	cases := []struct {
		name    string
		change  func()
		observe func(context.Context, string) (repositoryObservation, error)
		want    error
	}{
		{"stable", func() {}, stable, nil},
		{"probe-unavailable", func() {}, func(context.Context, string) (repositoryObservation, error) {
			return repositoryObservation{}, errCoreUnavailable
		}, errCoreUnavailable},
		{"same-commit-branch", func() { fixtureGit(t, root, "checkout", "-qb", "same-commit") }, observeRepository, errRepositoryChanged},
		{"dirty-after-core", func() {
			os.WriteFile(filepath.Join(root, "internal/widget/widget.go"), []byte("package widget\n"), 0600)
		}, observeRepository, errRepositoryChanged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			observe := func(ctx context.Context, r string) (repositoryObservation, error) {
				calls++
				if calls == 1 {
					return before, nil
				}
				return tc.observe(ctx, r)
			}
			result, err := contextEvidence(context.Background(), root, contextWorkerInput{}, observe, func(context.Context, string, contextWorkerInput) (json.RawMessage, error) {
				tc.change()
				return raw, nil
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if tc.want == nil && string(result) != string(raw) {
				t.Fatal("abstention was modified")
			}
			if tc.name == "same-commit-branch" {
				fixtureGit(t, root, "checkout", "-q", "-")
			}

		})
	}
}

func TestContextResponseErrorsAndBounds(t *testing.T) {
	source := goplsclient.Snapshot{URI: "file:///repo/a.go", Identity: "1", Version: 3, Text: "unsaved"}
	for _, tc := range []struct {
		name            string
		err             error
		current, cancel bool
		code            int
		reason          string
	}{
		{"cancel", context.Canceled, true, false, -32800, "CANCELLED"},
		{"deadline", context.DeadlineExceeded, true, false, -32800, "DEADLINE"},
		{"content", nil, false, false, -32801, "CONTENT_CHANGED"},
		{"repository", errRepositoryChanged, true, false, -32801, "REPOSITORY_CHANGED"},
		{"private-error", errors.New("SECRET /private/task text"), true, false, -32001, "CORE_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := contextResponse(&out, json.RawMessage(`1`), nil, source, "0123456789abcdef0123456789abcdef", tc.err, tc.current, tc.cancel); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "SECRET") || strings.Contains(out.String(), "/private") {
				t.Fatal("raw error leaked")
			}
			b, e := readFrame(bufio.NewReader(&out))
			if e != nil {
				t.Fatal(e)
			}
			var v struct {
				Error struct {
					Code int
					Data struct{ Reason string }
				}
			}
			json.Unmarshal(b, &v)
			if v.Error.Code != tc.code || v.Error.Data.Reason != tc.reason {
				t.Fatalf("wrong fixed error %s", b)
			}
		})
	}
	var out bytes.Buffer
	core := json.RawMessage(`{"state":"ABSTAINED","repository":null,"receipt":null}`)
	if e := contextResponse(&out, json.RawMessage(`1`), core, source, "0123456789abcdef0123456789abcdef", nil, true, false); e != nil {
		t.Fatal(e)
	}
	b, _ := readFrame(bufio.NewReader(&out))
	var v struct {
		Result struct {
			Core               json.RawMessage
			OverlayObservation map[string]any
		}
	}
	json.Unmarshal(b, &v)
	if string(v.Result.Core) != string(core) || v.Result.OverlayObservation["captureID"] != "1" {
		t.Fatalf("changed core or capture %s", b)
	}
	out.Reset()
	large, _ := json.Marshal(strings.Repeat("x", contextOutputLimit))
	contextResponse(&out, json.RawMessage(`1`), large, source, "0123456789abcdef0123456789abcdef", nil, true, false)
	b, _ = readFrame(bufio.NewReader(&out))
	if len(b) > contextOutputLimit || !bytes.Contains(b, []byte("CORE_UNAVAILABLE")) {
		t.Fatal("overflow did not refuse intact")
	}
}

func TestContextCaptureCounterBound(t *testing.T) {
	session, e := lspsnapshot.NewSession("0123456789abcdef0123456789abcdef", "file:///repo", lspsnapshot.Bounds{Documents: 2, DocumentBytes: 100, TotalBytes: 200})
	if e != nil {
		t.Fatal(e)
	}
	store := &overlayStore{session: session, docs: map[string]captured{}, next: math.MaxUint64 - 1}
	if e := store.put("file:///repo/a.go", 1, "x", true); e != nil {
		t.Fatal(e)
	}
	capture, _ := store.snapshot("file:///repo/a.go")
	if capture.Identity != "18446744073709551615" {
		t.Fatal(capture.Identity)
	}
	if e := store.put("file:///repo/a.go", 2, "y", false); e == nil {
		t.Fatal("capture counter wrapped")
	}
	if !store.current(capture) {
		t.Fatal("overflow mutated existing capture")
	}
}

func TestSemanticContextAdmission(t *testing.T) {
	h := semanticHarnessNew(t, "normal")
	h.init(t)
	for _, tc := range []struct {
		id     any
		params any
	}{{8, map[string]any{"textDocument": map[string]string{"uri": h.uri}, "task": "investigate", "limit": 1}}, {strings.Repeat("x", 4097), map[string]any{}}, {9, map[string]any{"textDocument": nil, "task": "investigate", "limit": 1}}} {
		h.send(t, map[string]any{"jsonrpc": "2.0", "id": tc.id, "method": "corvint/context", "params": tc.params})
		m := h.recv(t)
		var failure struct{ Code int }
		json.Unmarshal(m["error"], &failure)
		if failure.Code != -32602 {
			t.Fatalf("expected bounded invalid params: %s", m)
		}
	}
}

func TestContextLateCoreCancellationAndRootIdentity(t *testing.T) {
	root := contextFixture(t)
	before, e := observeRepository(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observe := func(context.Context, string) (repositoryObservation, error) { return before, nil }
	_, e = contextEvidence(ctx, root, contextWorkerInput{}, observe, func(context.Context, string, contextWorkerInput) (json.RawMessage, error) {
		cancel()
		return json.RawMessage(`{"repository":null}`), nil
	})
	if !errors.Is(e, context.Canceled) {
		t.Fatal("late successful Core escaped cancellation", e)
	}
	other := before
	other.root, e = os.Stat(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if sameRepository(before, other) {
		t.Fatal("different root identity accepted")
	}
	wrong := json.RawMessage(`{"repository":{"commitRevision":"wrong"}}`)
	if coreMatches(wrong, before) {
		t.Fatal("wrong Core binding accepted")
	}
}
