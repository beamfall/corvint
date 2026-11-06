// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/contextindex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDTLV0009TestFormatJSON(t *testing.T) {
	t.Run("DTL-V0-009", func(t *testing.T) {
		for _, args := range [][]string{{"--base", "HEAD", "--dry-run", "--format", "json"}, {"--base", "HEAD", "--format=json"}} {
			options, err := parseTestOptions(args)
			if err != nil {
				t.Fatal(err)
			}
			if !options.full || testSummarySelected(options) {
				t.Fatalf("explicit JSON selected a summary: %v", args)
			}
		}
		for _, args := range [][]string{{"--base", "HEAD", "--format", "xml"}, {"--base", "HEAD", "--format=json", "--dry-run", "--summary"}, {"--base", "HEAD", "--format=json", "--format=json"}} {
			if _, err := parseTestOptions(args); err == nil {
				t.Fatalf("admitted %v", args)
			}
		}
	})
}

func TestDTLV0004FinishStopsAfterCheck(t *testing.T) {
	t.Run("DTL-V0-004", func(t *testing.T) {
		run := portableDogfoodRunner(t)
		root, base := portableDogfoodRepo(t)
		// inputs-check takes --seal no, so a stop-before-seal pre-flight's retry keeps SEAL=0; an
		// unsorted intent manifest is refused before any step.
		_, unsorted := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "b.md\na.md\n")
		preflight := append([]string{"dogfood", "inputs-check", "--base", base, "--seal", "no", "--entry", "dogfood-finish"}, unsorted...)
		if code, _, stderr := run.exec(t, root, nil, preflight...); code != 2 || !strings.Contains(stderr, "rerun make dogfood-finish BASE="+base+" SEAL=0 CITATIONS=") {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
		if code, _, stderr := run.exec(t, root, nil, "dogfood", "inputs-check", "--base", base, "--seal", "invalid"); code != 2 || !strings.Contains(stderr, "invalid-dogfood-seal-value") {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
		env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "intent.md\n")
		argv := append([]string{"dogfood", "finish-lane", "--base", base, "--seal", "no"}, args...)
		code, stdout, stderr := run.exec(t, root, append(env[:3:3], finishLaneIdentity...), argv...)
		if code != 0 || !strings.Contains(stdout, "dogfood-check: PASS") || strings.Contains(stdout, "dogfood-seal: PASS") {
			t.Fatalf("code=%d out=%s err=%s", code, stdout, stderr)
		}
		if subjects := finishLaneSubjects(t, root, base); subjects != "chore: bind change evidence\nchange" {
			t.Fatalf("subjects=%q", subjects)
		}
		if _, err := os.Stat(filepath.Join(root, ".corvint/change.cem.json")); err != nil {
			t.Fatal(err)
		}
		if code, _, stderr := run.exec(t, root, nil, "dogfood", "finish-lane", "--base", base, "--seal", "invalid"); code != 2 || !strings.Contains(stderr, "invalid-dogfood-seal-value") {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
	})
}

func TestDTLV0001DirtyEditsReuseSnapshot(t *testing.T) {
	t.Run("DTL-V0-001", func(t *testing.T) {
		root := queryCLIRepository(t)
		var stderr bytes.Buffer
		if code := runContext(context.Background(), []string{"--root", root, "index", "--if-stale"}, strings.NewReader(""), io.Discard, &stderr); code != 0 {
			t.Fatal(stderr.String())
		}
		// A slow historical build must never reject a matching snapshot hit.
		if err := contextindex.RecordBuildCost(root, time.Hour); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package example\n"), 0644); err != nil {
			t.Fatal(err)
		}
		ctx := context.WithValue(lifecycleDeadlineContext(), dogfoodEventBuildKey{}, func(context.Context, string, string) (*contextindex.Index, error) {
			return nil, errors.New("injected: dirty edit attempted rebuild")
		})
		var stdout bytes.Buffer
		if code := runLocalCompletionEvent(ctx, root, dogfoodEventArguments("session-start"), strings.NewReader(`{}`), &stdout, &stderr); code != 0 {
			t.Fatalf("code=%d %s", code, &stderr)
		}
		if !strings.Contains(stdout.String(), `"dirtyPathCount":1`) {
			t.Fatalf("dirty observation absent: %s", &stdout)
		}
	})
}

func TestDTLV0002FallbackDiagnostics(t *testing.T) {
	t.Run("DTL-V0-002", func(t *testing.T) {
		var out bytes.Buffer
		emitAdapterOutput(&out, degradedAdapterOutput("corvint-event-rejected:dogfood-event-unavailable"))
		if !strings.Contains(out.String(), "cause:") || !strings.Contains(out.String(), "fix:") {
			t.Fatal(out.String())
		}
		rows := hookDegradationDiagnostics([]any{"frontier-authority-unavailable"})
		row := rows[0].(map[string]any)
		if row["cause"] != "no-accepted-closing-authority" || !strings.Contains(row["fix"].(string), "none exists") {
			t.Fatal(row)
		}
		// A receipt without Frontier degradation must not invent a Frontier cause.
		receipt := claudeReceiptOutput("post-tool", map[string]any{
			"receiptId": "harness-receipt:sha256:00", "degradations": []any{"host-version-unknown"},
		})
		out.Reset()
		emitAdapterOutput(&out, receipt)
		if !strings.Contains(out.String(), "cause: host-version-unknown") || strings.Contains(out.String(), "no-accepted-closing-authority") {
			t.Fatal(out.String())
		}
		// Even a broken stdout has one remaining diagnostic channel.
		closed, err := os.CreateTemp(t.TempDir(), "closed-output")
		if err != nil {
			t.Fatal(err)
		}
		if err := closed.Close(); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		if _, err := (hookStdout{stdout: closed, stderr: &out}).Write([]byte("receipt")); err == nil || !strings.Contains(out.String(), "cause: hook-stdout-unwritable\nfix:") {
			t.Fatalf("err=%v output=%s", err, &out)
		}
	})
}

// DTL-V0-002: the PreCompact fallback the host receives is the plain stdout text, which starts
// with the pin-unavailable line, so the cause and fix lines must reach that text and not only
// the systemMessage field.
func TestDTLV0002PreCompactPlainFallbackDiagnostics(t *testing.T) {
	t.Run("DTL-V0-002", func(t *testing.T) {
		for _, event := range []string{"pre-compact", "post-compact"} {
			var stdout bytes.Buffer
			if status := runHostAdapter(context.Background(), []string{"claude-code", event}, strings.NewReader("{not json"), &stdout); status != 0 {
				t.Fatalf("%s status=%d output=%s", event, status, &stdout)
			}
			text := stdout.String()
			if strings.HasPrefix(strings.TrimSpace(text), "{") || !(strings.Contains(text, "Corvint FALLBACK ") || strings.Contains(text, "Corvint fallback: ")) {
				t.Fatalf("%s did not emit a plain fallback: %q", event, text)
			}
			if event == "pre-compact" && !strings.HasPrefix(text, compactionPinUnavailable) {
				t.Fatalf("pre-compact fallback lost the pin-unavailable line: %q", text)
			}
			if !strings.Contains(text, "\ncause: ") || !strings.Contains(text, "\nfix: ") {
				t.Fatalf("%s plain fallback has no cause/fix diagnostics: %q", event, text)
			}
		}
	})
}
