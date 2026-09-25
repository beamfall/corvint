package observations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAppendRotatesAndNeverPersistsTaskText(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	degradations := make([]string, 40)
	for index := range degradations {
		degradations[index] = "compaction-critical-evidence-overflow"
	}
	for index := 0; index < 100; index++ {
		err := Append(root, Event{Kind: "event", Event: "user-prompt", TaskSHA256: TaskHash("secret"), Degradations: degradations})
		if err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > maxFileBytes || bytes.Contains(data, []byte("secret")) {
		t.Fatalf("ledger bound/secret failure bytes=%d", len(data))
	}
}

type countingReaderAt struct {
	io.ReaderAt
	bytesRead int
}

func (reader *countingReaderAt) ReadAt(buffer []byte, offset int64) (int, error) {
	count, err := reader.ReaderAt.ReadAt(buffer, offset)
	reader.bytesRead += count
	return count, err
}

// SOL-V0-003: repairing an externally oversized ledger preserves oldest-first
// truncation without reading more than the retained half.
func TestAppendRepairReadIsBounded(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	oldRow := []byte("{\"kind\":\"event\",\"event\":\"user-prompt\"}\n")
	oversized := bytes.Repeat(oldRow, maxFileBytes*4/len(oldRow)+1)
	directory := filepath.Join(root, ".corvint")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "self-observations.jsonl")
	if err := os.WriteFile(path, oversized, 0o600); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(Event{Kind: "event", Event: "session-start"})
	if err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := &countingReaderAt{ReaderAt: file}
	previous, err := readPreviousRows(reader, int64(len(oversized)), len(encoded)+1)
	if err != nil {
		t.Fatal(err)
	}
	if reader.bytesRead > maxFileBytes/2 {
		t.Fatalf("repair read %d bytes, want at most %d", reader.bytesRead, maxFileBytes/2)
	}
	wantPrevious := keepNewest(oversized, maxFileBytes/2)
	if !bytes.Equal(previous, wantPrevious) {
		t.Fatal("bounded repair changed oldest-first truncation semantics")
	}

	if err := Append(root, Event{Kind: "event", Event: "session-start"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := append(append(wantPrevious, encoded...), '\n')
	if !bytes.Equal(got, want) {
		t.Fatal("repaired ledger differs from prior append semantics")
	}
	if len(got) > maxFileBytes {
		t.Fatalf("repaired ledger grew past the cap: %d bytes", len(got))
	}
}

func TestAppendRepairDiscardsUnterminatedRows(t *testing.T) {
	oldRow := []byte("{\"kind\":\"event\",\"event\":\"user-prompt\"}\n")
	for _, tc := range []struct {
		name string
		data []byte
		want int
	}{
		{"oversized row without newline", bytes.Repeat([]byte("x"), maxFileBytes*2), 1},
		{"partial trailing row", append(bytes.Clone(oldRow), []byte(`{"kind":`)...), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeIgnore(t, root)
			if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".corvint", "self-observations.jsonl"), tc.data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Append(root, Event{Kind: "event", Event: "session-start"}); err != nil {
				t.Fatal(err)
			}
			got, err := Read(root)
			if err != nil || got.Events != tc.want || got.SkippedRows != 0 {
				t.Fatalf("new event lost during repair: events=%d skipped=%d err=%v", got.Events, got.SkippedRows, err)
			}
		})
	}
}

// TestAppendRedactsSecretShapedPathAfterContractAdmission covers SOL-V0-002
// defense in depth: a credential-shaped path component still gets screened
// after the writer admits the structurally valid path.
func TestAppendRedactsSecretShapedPathAfterContractAdmission(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	githubToken := "ghp_abcdefghijklmnopqrstuvwxyz"
	err := Append(root, Event{Kind: "event", Event: "file-change", TouchedPaths: []string{"tokens/" + githubToken}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(githubToken)) {
		t.Fatalf("ledger row leaked secret-shaped content: %s", data)
	}
	if !bytes.Contains(data, []byte(`[REDACTED]`)) {
		t.Fatalf("ledger row missing redaction placeholder: %s", data)
	}
}

func TestAppendRedactsQuotedCredentialPath(t *testing.T) {
	for _, value := range []string{`synthetic-example-value`, `top secret value`, `top \"secret\" value`, `top \\ secret value`} {
		t.Run(value, func(t *testing.T) {
			root := t.TempDir()
			writeIgnore(t, root)
			err := Append(root, Event{Kind: "event", Event: "file-change", TouchedPaths: []string{`config/{"password":"` + value + `","task":"repair config"}.go`}})
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			var row struct {
				Paths []string `json:"touchedPaths"`
			}
			if err := json.Unmarshal(data, &row); err != nil {
				t.Fatal(err)
			}
			want := `config/{[REDACTED],"task":"repair config"}.go`
			if len(row.Paths) != 1 || row.Paths[0] != want {
				t.Fatalf("quoted credential was not completely redacted: %s", data)
			}
		})
	}
}

func TestAppendRedactsUnterminatedQuotedCredentialPath(t *testing.T) {
	t.Run("EAF-V0-001", func(t *testing.T) {
		for _, tail := range []string{`top secret value`, `top secret value\`, `top \"secret\" value`, `top secret,benign:suffix.go`} {
			t.Run(tail, func(t *testing.T) {
				root := t.TempDir()
				writeIgnore(t, root)
				path := `config/{"password":"` + tail
				if err := Append(root, Event{Kind: "event", Event: "file-change", TouchedPaths: []string{path}}); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
				if err != nil {
					t.Fatal(err)
				}
				var row struct {
					Paths []string `json:"touchedPaths"`
				}
				if err := json.Unmarshal(data, &row); err != nil {
					t.Fatal(err)
				}
				if len(row.Paths) != 1 || row.Paths[0] != `config/{[REDACTED]` {
					t.Fatalf("unterminated credential suffix persisted: %s", data)
				}
			})
		}
	})
}

// TestAppendRefusesProseWithTypedError is the executable SOL-V0-002 writer
// boundary: task text, source text, and command prose are rejected before the
// ledger path is inspected or created, rather than silently dropped or merely
// redacted.
func TestAppendRefusesProseWithTypedError(t *testing.T) {
	tests := []struct {
		name      string
		event     Event
		wantField string
	}{
		{"task text", Event{Kind: "unsupported", Code: "unsupported-query-intent", QueryIntent: "fix the parser"}, "queryIntent"},
		{"source text", Event{Kind: "event", Event: "file-change", TouchedPaths: []string{"package main\nfunc main() {}"}}, "paths"},
		{"command prose", Event{Kind: "event", Event: "session-start", Degradations: []string{"go test ./..."}}, "degradations"},
		{"token-shaped task text", Event{Kind: "event", Event: "session-start", Degradations: []string{"fix-parser"}}, "degradations"},
		{"token-shaped source text", Event{Kind: "proof", Counts: map[string]map[string]int{"package-main": {"PASS": 1}}}, "counts-falsifier"},
		{"token-shaped command", Event{Kind: "dogfood-step", Step: "make", Status: "PRODUCED", Reason: "go-test"}, "step"},
		{"token-shaped command reason", Event{Kind: "dogfood-step", Step: "cem-status", Status: "NOT_PRODUCED", Reason: "command-rm"}, "reason"},
		{"token-shaped task reason", Event{Kind: "dogfood-step", Step: "cem-status", Status: "NOT_PRODUCED", Reason: "invalid-package-main"}, "reason"},
		{"oversized exit reason", Event{Kind: "dogfood-step", Step: "cem-status", Status: "NOT_PRODUCED", Reason: "exit-" + strings.Repeat("1", 92)}, "reason"},
		{"wrong row schema", Event{Kind: "event", Event: "session-start", Reason: "task"}, "event-schema"},
		{"wrong event schema", Event{Kind: "event", Event: "user-prompt", TouchedPaths: []string{"task"}}, "user-prompt-schema"},
		{"proof map prose", Event{Kind: "proof", Counts: map[string]map[string]int{"source body": {"PASS": 1}}}, "counts-falsifier"},
		{"empty kind with prose", Event{QueryIntent: "fix the parser"}, "kind"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			err := Append(root, test.event)
			var contractErr *Error
			if !errors.As(err, &contractErr) || contractErr.Code != CodeProhibitedContent || contractErr.Field != test.wantField {
				t.Fatalf("Append() error = %#v, want typed %s/%s", err, CodeProhibitedContent, test.wantField)
			}
			if strings.Contains(err.Error(), "parser") || strings.Contains(err.Error(), "package") || strings.Contains(err.Error(), "go test") {
				t.Fatalf("typed refusal echoed rejected content: %q", err)
			}
			if _, statErr := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(statErr) {
				t.Fatalf("rejected row touched ledger directory: %v", statErr)
			}
		})
	}
}

func TestAppendAcceptsNormalizedStructuralPathWithSpaces(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	if err := Append(root, Event{Kind: "event", Event: "file-change", TouchedPaths: []string{"docs/design notes.md"}}); err != nil {
		t.Fatal(err)
	}
}

func TestAppendAcceptsOnlyIgnoreRulesThatCoverTheLedger(t *testing.T) {
	for _, test := range []struct {
		name       string
		ignorePath string
		ignore     string
		wantWrite  bool
	}{
		{
			name:       "Corvint parent ignore",
			ignorePath: ".corvint/.gitignore",
			ignore:     "/.gitignore\n/index/\n/self-observations.jsonl\n/.self-observations.*\n",
			wantWrite:  true,
		},
		{
			name:       "root rules cover ledger and temporary",
			ignorePath: ".gitignore",
			ignore:     "/.corvint/self-observations.jsonl\n/.corvint/.self-observations.*\n",
			wantWrite:  true,
		},
		{
			name:       "root ledger-only rule leaves temporary visible",
			ignorePath: ".gitignore",
			ignore:     "/.corvint/self-observations.jsonl\n",
		},
		{
			name:       "snapshot child ignore does not cover sibling ledger",
			ignorePath: ".corvint/index/.gitignore",
			ignore:     "*\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			full := filepath.Join(root, filepath.FromSlash(test.ignorePath))
			if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(test.ignore), 0o600); err != nil {
				t.Fatal(err)
			}
			err := Append(root, Event{Kind: "event", Event: "session-start"})
			ledger := filepath.Join(root, ".corvint", "self-observations.jsonl")
			_, statErr := os.Stat(ledger)
			if test.wantWrite && (err != nil || statErr != nil) {
				t.Fatalf("Append() error=%v ledger error=%v", err, statErr)
			}
			if !test.wantWrite && (err == nil || !os.IsNotExist(statErr)) {
				t.Fatalf("unsafe ignore accepted: error=%v ledger error=%v", err, statErr)
			}
		})
	}
}

func TestAppendRemovesTemporaryLeftByDeadWriter(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	if err := Append(root, Event{Kind: "event"}); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, ".corvint", ".self-observations.dead")
	if err := os.WriteFile(stale, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Append(root, Event{Kind: "event"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temporary still present: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".corvint"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "self-observations.jsonl" {
		t.Fatalf("unexpected .corvint contents: %v", entries)
	}
}

func TestAppendFailureIsReportableForCallerFailOpen(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	if err := os.WriteFile(filepath.Join(root, ".corvint"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Append(root, Event{Kind: "event"}); err == nil {
		t.Fatal("Append unexpectedly succeeded")
	}
}

func TestAppendRefusesSymlinkedLedgerDirectoryOrFile(t *testing.T) {
	outside := t.TempDir()
	foreign := filepath.Join(outside, ".self-observations.keep")
	if err := os.WriteFile(foreign, []byte("foreign\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	directoryLink := t.TempDir()
	writeIgnore(t, directoryLink)
	if err := os.Symlink(outside, filepath.Join(directoryLink, ".corvint")); err != nil {
		t.Fatal(err)
	}
	fileLink := t.TempDir()
	writeIgnore(t, fileLink)
	if err := os.Mkdir(filepath.Join(fileLink, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, filepath.Join(fileLink, ".corvint", "self-observations.jsonl")); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{directoryLink, fileLink} {
		if err := Append(root, Event{Kind: "event"}); err == nil {
			t.Fatalf("Append through a symlink in %s succeeded", root)
		}
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 1 {
		t.Fatalf("Append wrote or removed outside the worktree: %v, %v", entries, err)
	}
	if data, err := os.ReadFile(filepath.Join(fileLink, ".corvint", "self-observations.jsonl")); err != nil || string(data) != "foreign\n" {
		t.Fatalf("ledger symlink was replaced: %q, %v", data, err)
	}
}

// SOL-V0-001: triage opens the ledger only when `.corvint` is a real directory
// and the ledger a regular file, so a link cannot digest a file outside the worktree.
func TestReadRefusesSymlinkedLedgerDirectoryOrFile(t *testing.T) {
	outside := t.TempDir()
	rows := []byte(`{"kind":"event","degradations":["OUTSIDE"]}` + "\n")
	if err := os.WriteFile(filepath.Join(outside, "self-observations.jsonl"), rows, 0o600); err != nil {
		t.Fatal(err)
	}
	directoryLink := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(directoryLink, ".corvint")); err != nil {
		t.Fatal(err)
	}
	fileLink := t.TempDir()
	if err := os.Mkdir(filepath.Join(fileLink, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "self-observations.jsonl"), filepath.Join(fileLink, ".corvint", "self-observations.jsonl")); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{directoryLink, fileLink} {
		if digest, err := Read(root); err == nil {
			t.Fatalf("Read through a symlink in %s succeeded: %+v", root, digest)
		}
	}
}

func TestAppendSerializesConcurrentWriters(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	const writers = 64
	start := make(chan struct{})
	errors := make(chan error, writers)
	var group sync.WaitGroup
	for index := 0; index < writers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			errors <- Append(root, Event{Kind: "unsupported", Code: "unsupported-query-intent", QueryIntent: "unknown", TaskSHA256: TaskHash(fmt.Sprintf("writer-%03d", index))})
		}()
	}
	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}

	verifyConcurrentRows(t, root, writers)
}

func TestAppendSerializesConcurrentProcesses(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	barrier := filepath.Join(root, "start")
	const writers = 24
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	commands := make([]*exec.Cmd, 0, writers)
	for index := 0; index < writers; index++ {
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAppendHelperProcess$")
		command.Env = append(os.Environ(),
			"CORVINT_OBSERVATIONS_HELPER_PROCESS=1",
			"CORVINT_OBSERVATIONS_ROOT="+root,
			"CORVINT_OBSERVATIONS_BARRIER="+barrier,
			fmt.Sprintf("CORVINT_OBSERVATIONS_CODE=writer-%03d", index),
		)
		if err := command.Start(); err != nil {
			cancel()
			for _, started := range commands {
				_ = started.Wait()
			}
			t.Fatal(err)
		}
		commands = append(commands, command)
	}
	if err := os.WriteFile(barrier, []byte("start\n"), 0o600); err != nil {
		cancel()
		for _, command := range commands {
			_ = command.Wait()
		}
		t.Fatal(err)
	}
	var waitErr error
	for _, command := range commands {
		if err := command.Wait(); err != nil && waitErr == nil {
			waitErr = err
		}
	}
	if waitErr != nil {
		t.Fatal(waitErr)
	}
	verifyConcurrentRows(t, root, writers)
}

func TestAppendHelperProcess(t *testing.T) {
	if os.Getenv("CORVINT_OBSERVATIONS_HELPER_PROCESS") != "1" {
		return
	}
	barrier := os.Getenv("CORVINT_OBSERVATIONS_BARRIER")
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := os.Stat(barrier)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("concurrent append barrier timed out")
		}
		time.Sleep(time.Millisecond)
	}
	root := os.Getenv("CORVINT_OBSERVATIONS_ROOT")
	code := os.Getenv("CORVINT_OBSERVATIONS_CODE")
	if err := Append(root, Event{Kind: "unsupported", Code: "unsupported-query-intent", QueryIntent: "unknown", TaskSHA256: TaskHash(code)}); err != nil {
		t.Fatal(err)
	}
}

func verifyConcurrentRows(t *testing.T, root string, writers int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'}) {
		var event Event
		if err := json.Unmarshal(row, &event); err != nil {
			t.Fatalf("invalid concurrent row %q: %v", row, err)
		}
		seen[event.TaskSHA256] = true
	}
	if len(seen) != writers {
		t.Fatalf("concurrent rows=%d, want %d", len(seen), writers)
	}
}

func TestRenderGoldenAndReadOnly(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	rows := []Event{
		{Kind: "event", Event: "user-prompt", Authoritative: 0, OmittedCount: 1, LatencyMS: 10, Degradations: []string{"frontier-authority-unavailable"}},
		{Kind: "event", Event: "file-change", LatencyMS: 30, Degradations: []string{"frontier-authority-unavailable"}, TouchedPaths: []string{"miss.go"}, RankedPaths: []string{}, MissState: "OBSERVED"},
		{Kind: "unsupported", Code: "unsupported-query-intent", QueryIntent: "repository"},
		{Kind: "unsupported", Code: "unsupported-query-intent", QueryIntent: "repository"},
		{Kind: "unsupported", Code: "unsupported-query-intent", QueryIntent: "repository"},
		{Kind: "unsupported", Code: "unsupported-query-intent", QueryIntent: "repository"},
		{Kind: "unsupported", Code: "unsupported-query-intent", QueryIntent: "repository"},
	}
	for _, row := range rows {
		if err := Append(root, row); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Render(root, 120, &output); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("triage wrote the ledger")
	}
	want := "SELF-OBSERVATIONS events=2 zero-authoritative-rate=1/2 budget-omission-rate=1/2 latency-p50=10ms latency-p95=30ms\nDEGRADATION count=2 key=frontier-authority-unavailable STANDING DRAFT fixes.md\nROUTING-MISS count=1 key=miss.go DRAFT ideas.md\nUNSUPPORTED count=5 key=unsupported-query-intent/repository CAPABILITY-GAP DRAFT ideas.md owning-spec=go-production-kernel-migration-v0.md\n"
	if output.String() != want {
		t.Fatalf("digest\n%s\nwant\n%s", output.String(), want)
	}
}

// TestReadSkipsOversizedRowAndKeepsReadingBoundedRows: self-observation-ledger-v0.md
// SOL-V0-005 (failure modes table, "malformed ledger row") requires triage to
// skip an over-limit row and keep reading, rather than abort the whole ledger
// as bufio.Scanner's fixed token buffer previously did (F17).
func TestReadSkipsOversizedRowAndKeepsReadingBoundedRows(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	before, err := json.Marshal(Event{Kind: "event", Event: "user-prompt", Authoritative: 1})
	if err != nil {
		t.Fatal(err)
	}
	oversized := append([]byte(`{"kind":"event","event":"user-prompt","authoritativeResultCount":1,"pad":"`), make([]byte, maxRowBytes)...)
	oversized = append(oversized, []byte(`"}`)...)
	after, err := json.Marshal(Event{Kind: "event", Event: "user-prompt", Authoritative: 1})
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Join([][]byte{before, oversized, after}, []byte("\n"))
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "self-observations.jsonl"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := Read(root)
	if err != nil {
		t.Fatalf("Read returned an error instead of skipping the oversized row: %v", err)
	}
	if digest.Events != 2 {
		t.Fatalf("events = %d, want 2 (both bounded rows read around the oversized one)", digest.Events)
	}
	if digest.SkippedRows != 1 {
		t.Fatalf("skippedRows = %d, want 1", digest.SkippedRows)
	}
	var output bytes.Buffer
	if err := Render(root, 120, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "SKIPPED-ROWS count=1 oversized") {
		t.Fatalf("digest did not surface the skipped row count:\n%s", output.String())
	}
}

// SOL-V0-005: triage counts only complete strict JSON Lines records; a
// hand-edited truncated, duplicate-key, invalid-UTF-8, or trailing-value row
// is malformed rather than a partial observation.
func TestRenderSkipsMalformedJSONLinesRows(t *testing.T) {
	valid := []byte("{\"kind\":\"event\",\"event\":\"user-prompt\"}\n")
	for name, malformed := range map[string][]byte{
		"unterminated":   []byte(`{"kind":"event","event":"user-prompt"}`),
		"duplicate-key":  []byte("{\"kind\":\"unsupported\",\"kind\":\"event\",\"event\":\"user-prompt\"}\n"),
		"nested-key":     []byte("{\"kind\":\"proof\",\"counts\":{\"history-consistent\":{\"PASS\":1,\"PASS\":2}}}\n"),
		"invalid-utf8":   append([]byte("{\"kind\":\"event\",\"event\":\"user-prompt\",\"x\":\""), []byte{0xff, '"', '}', '\n'}...),
		"trailing-value": []byte("{\"kind\":\"event\",\"event\":\"user-prompt\"}{}\n"),
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeIgnore(t, root)
			if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
				t.Fatal(err)
			}
			data := append(bytes.Clone(valid), malformed...)
			if err := os.WriteFile(filepath.Join(root, ".corvint", "self-observations.jsonl"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := Render(root, 120, &output); err != nil {
				t.Fatal(err)
			}
			want := "SELF-OBSERVATIONS events=1 zero-authoritative-rate=1/1 budget-omission-rate=0/1 latency-p50=0ms latency-p95=0ms\n"
			if output.String() != want {
				t.Fatalf("malformed row was counted:\n%s", output.String())
			}
		})
	}
}

// A hand-edited row whose rendered key carries a line break must not forge a
// triage line: triage skips it as malformed and keeps reading.
func TestRenderSkipsRowWhoseKeyCarriesALineBreak(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	forged := "FALSIFICATION proofs=9 judged=9 failed=0 rate=0/9"
	rows := []string{
		`{"kind":"event","event":"user-prompt","authoritativeResultCount":1,"degradations":["x\n` + forged + `"]}`,
		`{"kind":"event","event":"file-change","missState":"OBSERVED","touchedPaths":["a\u2028` + forged + `"],"rankedPaths":[]}`,
		`{"kind":"unsupported","code":"unsupported-query-intent","queryIntent":"repository\r` + forged + `"}`,
		`{"kind":"proof","counts":{"x\u0085` + forged + `":{"PASS":1}}}`,
		`{"kind":"event","event":"user-prompt","authoritativeResultCount":1}`,
	}
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "self-observations.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Render(root, 120, &output); err != nil {
		t.Fatal(err)
	}
	want := "SELF-OBSERVATIONS events=1 zero-authoritative-rate=0/1 budget-omission-rate=0/1 latency-p50=0ms latency-p95=0ms\n"
	if output.String() != want {
		t.Fatalf("digest\n%s\nwant\n%s", output.String(), want)
	}
}

func writeIgnore(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestFalsificationRateCountsJudgedRowsOnly: NOT_RUN and `none` rows are not
// judgments, so they never move the rate; a proof row with an unknown verdict,
// a negative or oversized count, or no counts is skipped; the triage lines
// carry the rate over every retained proof and per falsifier, and nothing is
// drafted.
func TestFalsificationRateCountsJudgedRowsOnly(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	rows := []Event{
		{Kind: "proof", Counts: map[string]map[string]int{"history-consistent": {"PASS": 7}, "test-kills-mutant": {"PASS": 1, "FAIL": 3}, "none": {"NOT_RUN": 4}}},
		{Kind: "proof", Counts: map[string]map[string]int{"history-consistent": {"PASS": 2, "FAIL": 1}, "reference-resolves": {"NOT_RUN": 5}}},
		{Kind: "proof", Counts: map[string]map[string]int{"none": {"PASS": 9, "FAIL": 9}}},
		{Kind: "proof", Counts: map[string]map[string]int{"history-consistent": {"FAIL": -1}}},
		{Kind: "proof", Counts: map[string]map[string]int{"history-consistent": {"FAIL": maxProofCount + 1}}},
		{Kind: "proof"},
	}
	for _, row := range rows {
		if err := Append(root, row); err != nil {
			t.Fatal(err)
		}
	}
	unknownVerdict, err := json.Marshal(Event{Kind: "proof", Counts: map[string]map[string]int{"history-consistent": {"MAYBE": 1}}})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := os.OpenFile(filepath.Join(root, ".corvint", "self-observations.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Write(append(unknownVerdict, '\n')); err != nil {
		_ = ledger.Close()
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := data.Falsification(); got != (Falsification{Proofs: 3, Judged: 14, Failed: 4, Rate: "4/14"}) {
		t.Fatalf("falsification = %+v", got)
	}
	var output bytes.Buffer
	if err := Render(root, 120, &output); err != nil {
		t.Fatal(err)
	}
	want := "SELF-OBSERVATIONS events=0 zero-authoritative-rate=NOT_OBSERVED budget-omission-rate=NOT_OBSERVED latency-p50=0ms latency-p95=0ms\n" +
		"FALSIFICATION proofs=3 judged=14 failed=4 rate=4/14\n" +
		"FALSIFIER key=history-consistent judged=10 failed=1 not-run=0 rate=1/10\n" +
		"FALSIFIER key=none judged=0 failed=0 not-run=4 rate=NOT_OBSERVED\n" +
		"FALSIFIER key=reference-resolves judged=0 failed=0 not-run=5 rate=NOT_OBSERVED\n" +
		"FALSIFIER key=test-kills-mutant judged=4 failed=3 not-run=0 rate=3/4\n"
	if output.String() != want {
		t.Fatalf("digest\n%s\nwant\n%s", output.String(), want)
	}
	if empty, _ := Read(t.TempDir()); empty.Falsification().Proofs != 0 {
		t.Fatal("an absent ledger reports proofs")
	}
}

func TestAppendDocumentationCodeRemainsClosed(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	event := Event{Kind: "unsupported", Code: "unsupported-documentation-source", QueryIntent: "unknown"}
	if err := Append(root, event); err != nil {
		t.Fatalf("declared documentation code refused: %v", err)
	}
	for _, code := range []string{"unsupported-documentation-free-form-task", "unsupported-documentation-source secret"} {
		event.Code = code
		var refusal *Error
		if err := Append(root, event); !errors.As(err, &refusal) || refusal.Code != CodeProhibitedContent || refusal.Field != "code" {
			t.Fatalf("closed code boundary weakened for %q: %v", code, err)
		}
	}
}

// TestUnsupportedByDesignCodeIsReportedSeparately: SOL-V0-007 reports a code
// the by-design registry names on its own line with its decision record, and
// never as a CAPABILITY-GAP DRAFT however often it recurs.
func TestUnsupportedByDesignCodeIsReportedSeparately(t *testing.T) {
	const decision = "docs/decisions/0015-impact-without-go-module-2026-09-01.md"
	unsupportedByDesign["unsupported-impact-path-suffix"] = decision
	t.Cleanup(func() { delete(unsupportedByDesign, "unsupported-impact-path-suffix") })
	got := ranked("UNSUPPORTED", map[string]int{
		"unsupported-impact-path-suffix/unknown": 6,
		"unsupported-impact-range/unknown":       5,
	}, 0, false)
	want := []string{
		"UNSUPPORTED-BY-DESIGN count=6 key=unsupported-impact-path-suffix/unknown decision=" + decision,
		"UNSUPPORTED count=5 key=unsupported-impact-range/unknown CAPABILITY-GAP DRAFT ideas.md unsupported-with-no-owning-spec",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("ranked\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestDogfoodReasonAdmitsEveryRegisteredCEMCode: a dogfood step's reason is
// the code of its failing CEM command, so every code registered in cemcode
// (map-locked included) must be an admitted dogfood-step reason (SOL-V0-008).
func TestDogfoodReasonAdmitsEveryRegisteredCEMCode(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "cem", "cemcode", "cemcode.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	ast.Inspect(file, func(node ast.Node) bool {
		declaration, ok := node.(*ast.GenDecl)
		if ok && declaration.Tok == token.CONST {
			for _, spec := range declaration.Specs {
				for _, value := range spec.(*ast.ValueSpec).Values {
					code, _ := strconv.Unquote(value.(*ast.BasicLit).Value)
					codes = append(codes, code)
				}
			}
		}
		return true
	})
	if len(codes) < 50 {
		t.Fatalf("parsed %d cemcode codes, want the full registry", len(codes))
	}
	for _, code := range codes {
		row := Event{Kind: "dogfood-step", Step: "cem-cite", Status: "NOT_PRODUCED", Reason: code}
		if err := validateWriterContract(row); err != nil {
			t.Errorf("registered CEM code %q refused as a dogfood reason: %v", code, err)
		}
	}
}

// TestDogfoodReasonAdmitsCitationPlanMapMismatch: the coordinator's refusal of a
// plan written for another prepared map is observable (DCW-V0-019, V1-0173).
func TestDogfoodReasonAdmitsCitationPlanMapMismatch(t *testing.T) {
	row := Event{Kind: "dogfood-step", Step: "cem-cite", Status: "NOT_PRODUCED", Reason: "citation-plan-map-mismatch"}
	if err := validateWriterContract(row); err != nil {
		t.Fatalf("citation-plan-map-mismatch refused as a dogfood reason: %v", err)
	}
}

// TestDogfoodReasonAdmitsCitationStageRefusals: the two citation-stage refusals
// of `dogfood change` are observable like its other cem-cite reasons (V1-0228).
func TestDogfoodReasonAdmitsCitationStageRefusals(t *testing.T) {
	for _, reason := range []string{"citation-stage-exists", "citation-stage-cleanup-failed"} {
		row := Event{Kind: "dogfood-step", Step: "cem-cite", Status: "NOT_PRODUCED", Reason: reason}
		if err := validateWriterContract(row); err != nil {
			t.Fatalf("%s refused as a dogfood reason: %v", reason, err)
		}
	}
}

// SOL-V0-010: an adapter-degradation row carries only host, event, closed codes, an
// hour window and the Corvint version; content fields and unadmitted codes are refused.
func TestAdapterDegradationRowCarriesNoContentFields(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	now := time.Date(2026, 9, 12, 14, 37, 5, 0, time.UTC)
	if err := Append(root, AdapterDegradationEvent("claude-code", "user-prompt", "corvint-event-rejected:some-unregistered-code", "0.4.0a4", now)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"kind":"adapter-degradation","event":"user-prompt","host":"claude-code","adapterCodes":["corvint-event-rejected"],"window":"2026-09-12T14Z","corvintVersion":"0.4.0a4"}` + "\n"
	if string(data) != want {
		t.Fatalf("row\n%s\nwant\n%s", data, want)
	}
	row := AdapterDegradationEvent("claude-code", "user-prompt", "prompt-over-query-bound", "0.4.0a4", now)
	withTask, withPath, withProse := row, row, row
	withTask.TaskSHA256 = TaskHash("prompt")
	withPath.TouchedPaths = []string{"secret.go"}
	withProse.AdapterCodes = []string{"Fix the login bug"}
	for _, refused := range []Event{withTask, withPath, withProse, {Kind: "event", Event: "user-prompt", Host: "codex"}} {
		var contract *Error
		if err := Append(root, refused); !errors.As(err, &contract) || contract.Code != CodeProhibitedContent {
			t.Fatalf("row %+v was not refused: %v", refused, err)
		}
	}
}

// SOL-V0-010: one row per host, event and code set per hour window.
func TestAdapterDegradationDeduplicatesWithinWindow(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	hour := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	rows := []Event{
		AdapterDegradationEvent("claude-code", "user-prompt", "prompt-over-query-bound", "0.4.0a4", hour),
		AdapterDegradationEvent("claude-code", "user-prompt", "prompt-over-query-bound", "0.4.0a4", hour.Add(59*time.Minute)),
		AdapterDegradationEvent("claude-code", "user-prompt", "missing-prompt", "0.4.0a4", hour),
		AdapterDegradationEvent("claude-code", "user-prompt", "prompt-over-query-bound", "0.4.0a4", hour.Add(time.Hour)),
	}
	for _, row := range rows {
		if err := Append(root, row); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := bytes.Count(data, []byte("\n")); lines != 3 {
		t.Fatalf("rows=%d, want 3\n%s", lines, data)
	}
}

// SOL-V0-010, SOL-V0-003: adapter rows rotate under the unchanged ledger cap.
func TestAdapterDegradationRowsHonorLedgerCap(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	filler := `{"kind":"unsupported","code":"unsupported-spec"}` + "\n"
	ledger := bytes.Repeat([]byte(filler), maxFileBytes/len(filler))
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "self-observations.jsonl"), ledger, 0o600); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for index := 0; index < 3; index++ {
		if err := Append(root, AdapterDegradationEvent("codex", "session-start", "invalid-start-source", "0.4.0a4", start.Add(time.Duration(index)*time.Hour))); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > maxFileBytes || !bytes.HasSuffix(data, []byte(`"window":"2026-09-12T02Z","corvintVersion":"0.4.0a4"}`+"\n")) {
		t.Fatalf("ledger bytes=%d tail=%q", len(data), data[len(data)-120:])
	}
}

// SOL-V0-010: triage tallies retained adapter rows by host/event/code with the newest window.
func TestRenderTalliesAdapterDegradations(t *testing.T) {
	root := t.TempDir()
	writeIgnore(t, root)
	hour := time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)
	for _, row := range []Event{
		AdapterDegradationEvent("claude-code", "user-prompt", "prompt-over-query-bound", "0.4.0a4", hour),
		AdapterDegradationEvent("claude-code", "user-prompt", "prompt-over-query-bound", "0.4.0a4", hour.Add(3*time.Hour)),
		AdapterDegradationEvent("codex", "stop", "corvint-event-rejected:dogfood-event-deadline", "0.4.0a4", hour),
	} {
		if err := Append(root, row); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := Render(root, 120, &output); err != nil {
		t.Fatal(err)
	}
	want := "SELF-OBSERVATIONS events=0 zero-authoritative-rate=NOT_OBSERVED budget-omission-rate=NOT_OBSERVED latency-p50=0ms latency-p95=0ms\n" +
		"ADAPTER-DEGRADATION windows=2 key=claude-code/user-prompt/prompt-over-query-bound latest=2026-09-12T12Z\n" +
		"ADAPTER-DEGRADATION windows=1 key=codex/stop/corvint-event-rejected:dogfood-event-deadline latest=2026-09-12T09Z\n"
	if output.String() != want {
		t.Fatalf("digest\n%s\nwant\n%s", output.String(), want)
	}
}
