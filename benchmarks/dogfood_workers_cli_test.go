package benchmarks_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// A caller-owned directory retains identical absolute input paths for causal replay.
// Ordinary gates use a private, automatically cleaned temporary directory.
var replayDirectory = flag.String("dogfood-fixture-dir", "", "caller-owned temporary directory for exact-byte causal replay")

var usageFields = []string{"input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens", "output_tokens"}
var receiptFields = []string{"inputTokens", "cacheCreationTokens", "cacheReadTokens", "outputTokens"}

func TestDogfoodWorkersCLI(t *testing.T) {
	t.Run("BRAIN-DOG-013 repeated Claude snapshots retain uncertainty through receipt CLI", func(t *testing.T) {
		_, source, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("source root unavailable")
		}
		root := filepath.Dir(filepath.Dir(source))
		dir := workerFixtureDirectory(t)
		binary := buildWorkerCLI(t, root)
		env := []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C", "TMPDIR=" + dir}
		t.Logf("native-cli-sha256=%s test-sha256=%s", hashFile(t, binary), hashFile(t, source))

		workers := []map[string]any{}
		transcriptHashes := map[string]string{}
		for index, id := range []string{"healthy", "decrease", "missing", "boolean", "negative", "mixed", "all-sidechain"} {
			events := []map[string]any{}
			side := index >= 5
			if index == 5 {
				events = append(events, usageEvent("shared", false, usage(5)))
			}
			events = append(events, usageEvent("shared", side, usage(40)))
			middle := usage(40)
			switch index {
			case 1:
				middle[usageFields[0]] = 7
			case 2:
				delete(middle, usageFields[1])
			case 3:
				middle[usageFields[2]] = true
			case 4:
				middle[usageFields[3]] = -1
			case 5, 6:
				middle[usageFields[0]] = 7
			}
			events = append(events, usageEvent("shared", side, middle), usageEvent("shared", side, usage(60)), usageEvent("other", side, usage(3)))
			raw := []byte{}
			for _, event := range events {
				raw = append(raw, jsonBytes(t, event)...)
				raw = append(raw, '\n')
			}
			path := filepath.Join(dir, id+".jsonl")
			writeFixture(t, path, raw)
			transcriptHashes[id] = digest(raw)
			workers = append(workers, map[string]any{"id": id, "role": "builder", "host": "claude-code", "status": "completed", "model": "synthetic-fixture", "transcript": path, "solved": "NOT_OBSERVED"})
		}
		manifest := jsonBytes(t, map[string]any{"profile": "corvint-dogfood-workers-manifest/0", "task": "synthetic repeated usage CLI boundary", "pricingVersion": "NOT_OBSERVED", "workers": workers})
		manifestPath, outputPath := filepath.Join(dir, "manifest.json"), filepath.Join(dir, "receipt.json")
		writeFixture(t, manifestPath, manifest)
		t.Logf("manifest-sha256=%s transcript-sha256=%s", digest(manifest), jsonBytes(t, transcriptHashes))
		if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		stdout := runWorkerCLI(t, root, env, binary, "--manifest", manifestPath, "--output", outputPath)
		if len(stdout) != 0 {
			t.Fatalf("unexpected CLI stdout: %q", stdout)
		}
		raw, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		var receipt struct {
			Profile        string           `json:"profile"`
			Task           string           `json:"task"`
			PricingVersion string           `json:"pricingVersion"`
			ManifestSHA256 string           `json:"manifestSha256"`
			Workers        []map[string]any `json:"workers"`
			Totals         map[string]any   `json:"totals"`
			Cost           any              `json:"cost"`
		}
		if err := json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Profile != "corvint-dogfood-workers/0" || receipt.Task != "synthetic repeated usage CLI boundary" || receipt.PricingVersion != "NOT_OBSERVED" || receipt.ManifestSHA256 != digest(manifest) || receipt.Cost != "NOT_OBSERVED" {
			t.Fatalf("receipt identity or cost mismatch: %s", raw)
		}
		if len(receipt.Workers) != len(workers) {
			t.Fatalf("worker count=%d", len(receipt.Workers))
		}
		seen := map[string]bool{}
		for index, worker := range receipt.Workers {
			id, ok := worker["id"].(string)
			if !ok || seen[id] || workers[index]["id"] != id {
				t.Fatalf("unexpected worker identity: %v", worker["id"])
			}
			seen[id] = true
			if worker["transcriptSha256"] != transcriptHashes[id] {
				t.Errorf("%s transcript digest mismatch", id)
			}
			unknownField := index - 1
			if index >= 5 {
				unknownField = 0
			}
			expectedTurns, expectedValue := float64(2), float64(63)
			if index == 5 {
				expectedTurns, expectedValue = 1, 5
				unknownField = -1
			}
			checkMetrics(t, id, worker, expectedValue, expectedTurns, unknownField)
			if index == 5 {
				child, ok := worker["sidechain"].(map[string]any)
				if !ok {
					t.Fatal("mixed worker omitted sidechain")
				}
				checkMetrics(t, id+" sidechain", child, 63, 2, 0)
			}
			if index == 6 && worker["sidechain"] != nil {
				t.Error("all-sidechain worker was not promoted")
			}
		}
		for _, field := range receiptFields {
			if receipt.Totals[field] != "NOT_OBSERVED" {
				t.Errorf("totals %s=%v, want NOT_OBSERVED", field, receipt.Totals[field])
			}
		}
		for field, want := range map[string]any{"observedWorkers": float64(7), "failedWorkers": float64(0), "cancelledWorkers": float64(0), "solvedWorkers": "NOT_OBSERVED"} {
			if receipt.Totals[field] != want {
				t.Errorf("totals %s=%v, want %v", field, receipt.Totals[field], want)
			}
		}
	})
}

func usage(value int) map[string]any {
	result := map[string]any{}
	for _, field := range usageFields {
		result[field] = value
	}
	return result
}

func usageEvent(id string, side bool, values map[string]any) map[string]any {
	return map[string]any{"type": "assistant", "requestId": id, "isSidechain": side, "message": map[string]any{"usage": values, "content": []any{}}}
}

func checkMetrics(t *testing.T, id string, metrics map[string]any, value, turns float64, unknown int) {
	t.Helper()
	for index, field := range receiptFields {
		var want any = value
		if index == unknown {
			want = "NOT_OBSERVED"
		}
		if metrics[field] != want {
			t.Errorf("%s %s=%v, want %v", id, field, metrics[field], want)
		}
	}
	if metrics["turns"] != turns {
		t.Errorf("%s turns=%v, want %v", id, metrics["turns"], turns)
	}
}

func runWorkerCLI(t *testing.T, root string, env []string, argv ...string) []byte {
	t.Helper()
	result := procgroup.Run(context.Background(), procgroup.Spec{Argv: argv, Dir: root, Env: env, Timeout: 30 * time.Second, ShutdownTimeout: 2 * time.Second, OutputLimit: 1 << 20})
	// This standard-library CLI starts no subprocesses. Require the runner's
	// qualified owned-group cleanup, not unsupported escaped-setsid containment.
	if result.Err != nil || !result.Started || !result.ExitObserved || result.ExitStatus != 0 || !result.WaitCompleted || !result.PipesDrained || !result.OwnedProcessGroupCleanup || result.DescendantCleanupStatus != "owned-process-group" || result.TimedOut || result.Cancelled || result.OutputOverflow || len(result.Stderr) != 0 {
		t.Fatalf("CLI process boundary failed: %+v", result)
	}
	return result.Stdout
}

func jsonBytes(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func digest(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func hashFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return digest(raw)
}
func writeFixture(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

// Explicit replay is confined to a directory this test created and marked.
// Reject links and unknown entries before writing any retained fixture bytes.
func workerFixtureDirectory(t *testing.T) string {
	t.Helper()
	if *replayDirectory == "" {
		return t.TempDir()
	}
	dir, err := admitWorkerFixtureDirectory(*replayDirectory)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func admitWorkerFixtureDirectory(dir string) (result string, err error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return "", fmt.Errorf("replay directory must be a clean absolute path")
	}
	parent := filepath.Dir(dir)
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if resolvedParent != parent {
		return "", fmt.Errorf("replay parent must have no symlink components")
	}
	const marker = ".corvint-dogfood-cli-fixture"
	const owner = "corvint-dogfood-workers-cli-fixture/0\n"
	err = os.Mkdir(dir, 0700)
	created := err == nil
	if err != nil && !os.IsExist(err) {
		return "", err
	}
	admitted := false
	defer func() {
		if !created || admitted {
			return
		}
		// Remove only our newly created empty directory; never remove new contents
		// another actor may have placed there after admission failed.
		cleanupErr := os.Remove(dir)
		if cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			err = fmt.Errorf("%w; new fixture directory cleanup: %v", err, cleanupErr)
		}
	}()
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		return "", fmt.Errorf("replay directory must be a private directory, not a link")
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	if resolved != dir {
		return "", fmt.Errorf("replay directory must have no symlink components")
	}
	allowed := map[string]bool{marker: true, "manifest.json": true, "receipt.json": true, "missing-usage-manifest.json": true, "missing-usage-receipt.json": true, "missing-request.jsonl": true, "mixed-missing.jsonl": true}
	for _, id := range []string{"healthy", "decrease", "missing", "boolean", "negative", "mixed", "all-sidechain"} {
		allowed[id+".jsonl"] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		if !allowed[entry.Name()] || !info.Mode().IsRegular() {
			return "", fmt.Errorf("unowned replay entry %q", entry.Name())
		}
	}
	if len(entries) == 0 {
		if !created {
			return "", fmt.Errorf("existing replay directory is not owned by this test")
		}
		if err := os.WriteFile(filepath.Join(dir, marker), []byte(owner), 0600); err != nil {
			return "", err
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, marker))
	if err != nil {
		return "", err
	}
	if string(raw) != owner {
		return "", fmt.Errorf("replay directory lacks this test's ownership marker")
	}
	admitted = true
	return dir, nil
}

func TestDogfoodWorkersReplayDirectoryRejectsLinkedParentWithoutEffects(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked-parent")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(link, "private-new")
	if _, err := admitWorkerFixtureDirectory(candidate); err == nil {
		t.Fatal("linked parent was admitted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("refusal created outside entries: %v", entries)
	}
	if _, err := os.Lstat(filepath.Join(outside, "private-new")); !os.IsNotExist(err) {
		t.Fatalf("outside fixture path exists after refusal: %v", err)
	}
	target, err := os.Readlink(link)
	if err != nil || target != outside {
		t.Fatalf("refusal altered parent link: target=%q err=%v", target, err)
	}
}

func buildWorkerCLI(t *testing.T, root string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "dogfood-workers")
	command := exec.Command("go", "build", "-o", binary, "./benchmarks/dogfood-workers")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native workers CLI: %v: %s", err, output)
	}
	return binary
}
