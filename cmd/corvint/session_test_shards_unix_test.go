//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

func writeShardsInput(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func shardsHistory(revision, milliseconds string) string {
	return `{"milliseconds": {` + milliseconds + `}, "profile": "corvint-ci-package-costs/0", "source": {"goVersion": "go1.27.2", "revision": "` +
		strings.Repeat(revision, 40) + `", "runURL": "https://example.test/runs/1"}}`
}

// STR-V0-050..053: `corvint test shards` prints one deterministic plan to
// stdout and writes nothing else; missing history is the reported
// round-robin; usage errors exit 2.
func TestSTRV0050CLIShardsPlan(t *testing.T) {
	dir := t.TempDir()
	files := writeShardsInput(t, dir, "files.txt", "a\nb\nc\n")
	old := writeShardsInput(t, dir, "old.json", shardsHistory("1", `"a": 900, "b": 10`))
	recent := writeShardsInput(t, dir, "new.json", shardsHistory("2", `"a": 100, "b": 30`))
	before, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"test", "shards", "--count", "2", "--files", files, "--durations", old, "--durations", recent, "--runs", "2"}
	code, stdout, stderr := runCLITwice(t, args...)
	if code != 0 || stderr != "corvint-test: shards 3 files over 2 shards from 2 runs, 1 unknown\n" {
		t.Fatalf("shards %d %q", code, stderr)
	}
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, []byte(stdout), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := sessiontest.ReadShardPlan(planPath)
	if err != nil {
		t.Fatalf("the printed plan does not read back: %v\n%s", err, stdout)
	}
	// a = median(900,100) = 500, b = 20, c unknown = median(20,500) = 260.
	if plan.WeightMilliseconds["a"] != 500 || plan.WeightMilliseconds["c"] != 260 || strings.Join(plan.Unknown, ",") != "c" ||
		strings.Join(plan.Shards[0].Files, ",") != "a" || strings.Join(plan.Shards[1].Files, ",") != "b,c" {
		t.Fatalf("plan %+v", plan)
	}
	after, err := os.ReadDir(dir)
	if err != nil || len(after) != len(before) {
		t.Fatalf("the read command changed its input directory: %d entries, then %d", len(before), len(after))
	}
	code, stdout, stderr = runCLI(t, "test", "shards", "--count", "2", "--files", files)
	if code != 0 || stderr != "corvint-test: shards FALLBACK no retained run durations were supplied\n" || !strings.Contains(stdout, `"files":["a","c"]`) {
		t.Fatalf("fallback %d %q %s", code, stderr, stdout)
	}
	code, _, stderr = runCLI(t, "test", "shards", "--count", "2", "--files", files, "--durations", filepath.Join(dir, "absent.json"))
	if code != 0 || !strings.HasPrefix(stderr, "corvint-test: shards FALLBACK invalid run durations: ") {
		t.Fatalf("absent history %d %q", code, stderr)
	}
	duplicate := writeShardsInput(t, t.TempDir(), "dup.txt", "a\na\n")
	for _, refused := range [][]string{
		{"test", "shards", "--files", files},
		{"test", "shards", "--count", "65", "--files", files},
		{"test", "shards", "--count", "2"},
		{"test", "shards", "--count", "2", "--files", files, "--runs", "0"},
		{"test", "shards", "--count", "2", "--files", files, "extra"},
		{"test", "shards", "--count", "2", "--files", filepath.Join(dir, "absent.txt")},
		{"test", "shards", "--count", "2", "--files", duplicate},
	} {
		if code, stdout, stderr := runCLI(t, refused...); code != 2 || stdout != "" || stderr == "" {
			t.Errorf("%v: code %d stdout %q", refused, code, stdout)
		}
	}
}

// STR-V0-054: a measured plan feeds the sharded run: every shard executes
// against one dry-run manifest bound to the plan's digest, verify-shards
// accepts the union and is INCOMPLETE when a shard's receipt is missing;
// the usage refusals match balanced placement's.
func TestSTRV0054CLIMeasuredShards(t *testing.T) {
	root, _, _ := sessionFixture(t, "pass")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD~1"))
	if err := os.WriteFile(filepath.Join(root, "b/b.go"), []byte("package b\n\nfunc B() int { return 4 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change b")
	dir := t.TempDir()
	files := writeShardsInput(t, dir, "files.txt", "example.test/m/a\nexample.test/m/b\n")
	history := writeShardsInput(t, dir, "run.json", shardsHistory("3", `"example.test/m/a": 50, "example.test/m/b": 40`))
	code, stdout, stderr := runCLI(t, "test", "shards", "--count", "2", "--files", files, "--durations", history)
	if code != 0 {
		t.Fatalf("shards %d %s", code, stderr)
	}
	planPath := writeShardsInput(t, dir, "plan.json", stdout)
	plan, err := sessiontest.ReadShardPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, "manifest.json")
	code, stdout, stderr = runCLI(t, "--root", root, "test", "--base", base, "--placement", "measured", "--shard-plan", planPath, "--shard", "0/2", "--dry-run", "--manifest", manifest)
	if code != 0 || !strings.Contains(stdout, `"placement":"measured"`) {
		t.Fatalf("dry run %d %q %s", code, stderr, stdout)
	}
	planned, _, err := sessiontest.ReadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if planned.PartitionDigest != plan.PartitionDigest || len(planned.Units) != 2 || planned.Units[0].Shard == planned.Units[1].Shard {
		t.Fatalf("measured manifest %s %+v", planned.PartitionDigest, planned.Units)
	}
	receipts := []string{}
	for _, shard := range []string{"0/2", "1/2"} {
		name := filepath.Join(dir, "run."+shard[:1]+".json")
		if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--placement", "measured", "--shard-plan", planPath, "--shard", shard, "--manifest", manifest, "--receipt", name); code != 0 {
			t.Fatalf("shard %s: %d %s", shard, code, stderr)
		}
		receipts = append(receipts, name)
	}
	code, stdout, _ = runCLI(t, append([]string{"test", "verify-shards", "--manifest", manifest}, receipts...)...)
	if code != 0 || !strings.Contains(stdout, `"reasons":[],"result":"PASS"`) {
		t.Fatalf("verify-shards %d %s", code, stdout)
	}
	code, stdout, _ = runCLI(t, "test", "verify-shards", "--manifest", manifest, receipts[1])
	if code != 3 || !strings.Contains(stdout, `"result":"INCOMPLETE"`) || !strings.Contains(stdout, "has no accepted receipt") {
		t.Fatalf("missing shard %d %s", code, stdout)
	}
	tampered := writeShardsInput(t, dir, "tampered.json", strings.Replace(string(mustRead(t, planPath)), plan.PartitionDigest, "sha256:"+strings.Repeat("0", 64), 1))
	baseID := strings.Repeat("a", 40)
	for _, refused := range [][]string{
		{"test", "--base", baseID, "--placement", "measured"},
		{"test", "--base", baseID, "--shard-plan", planPath},
		{"test", "--base", baseID, "--placement", "measured", "--shard-plan", planPath, "--shard", "0/2"},
		{"test", "--base", baseID, "--placement", "measured", "--shard-plan", planPath, "--profile", "ci", "--confine", "/x", "--manifest", "/m"},
		{"--root", root, "test", "--base", base, "--placement", "measured", "--shard-plan", tampered, "--dry-run"},
	} {
		if code, stdout, stderr := runCLI(t, refused...); code != 2 || stdout != "" || stderr == "" {
			t.Errorf("%v: code %d stdout %q stderr %q", refused, code, stdout, stderr)
		}
	}
}

// STR-V0-053: more than 64 duration tables is the reported complete
// round-robin with exit 0, not a usage error; none of them is read.
func TestSTRV0053CLIShardsOverTableBound(t *testing.T) {
	dir := t.TempDir()
	files := writeShardsInput(t, dir, "files.txt", "a\nb\nc\n")
	args := []string{"test", "shards", "--count", "2", "--files", files}
	for i := range sessiontest.MaxHistoryRuns + 1 {
		args = append(args, "--durations", filepath.Join(dir, "absent-"+strconv.Itoa(i)+".json"))
	}
	code, stdout, stderr := runCLI(t, args...)
	if code != 0 || stderr != "corvint-test: shards FALLBACK 65 duration tables exceed the 64 bound\n" ||
		!strings.Contains(stdout, `"files":["a","c"]`) || !strings.Contains(stdout, `"fallback":"65 duration tables exceed the 64 bound"`) {
		t.Fatalf("over the table bound %d %q %s", code, stderr, stdout)
	}
}

// STR-V0-050: an optional input is read through its opened descriptor:
// a FIFO is refused without blocking, and a regular file that grows past
// the bound after it was checked is never returned over the bound.
func TestSTRV0050ShardsInputReadIsBounded(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan []byte, 1)
	go func() { done <- testReadOptional(fifo) }()
	select {
	case data := <-done:
		if data != nil {
			t.Fatalf("a FIFO was read: %q", data)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("reading a FIFO blocked")
	}
	if code, _, stderr := runCLI(t, "test", "shards", "--count", "1", "--files", fifo); code != 2 || !strings.Contains(stderr, "not a readable regular file") {
		t.Fatalf("FIFO list %d %q", code, stderr)
	}
	name := filepath.Join(dir, "growing.json")
	file, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	stop := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = file.Truncate(16)
			_ = file.Truncate(2 * sessiontest.MaxDocumentBytes)
		}
	}()
	defer func() { close(stop); wait.Wait() }()
	for range 2000 {
		if data := testReadOptional(name); len(data) > sessiontest.MaxDocumentBytes {
			t.Fatalf("read %d bytes, over the %d bound", len(data), sessiontest.MaxDocumentBytes)
		}
	}
}
