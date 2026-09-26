package genesis

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

func genesisGit(t *testing.T, git, root string, args ...string) {
	t.Helper()
	command := exec.Command(git, append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, raw)
	}
}

func genesisRepository(t *testing.T, git, format string) string {
	t.Helper()
	root := t.TempDir()
	genesisGit(t, git, root, "init", "-q", "--object-format="+format)
	genesisGit(t, git, root, "config", "user.name", "Fixture")
	genesisGit(t, git, root, "config", "user.email", "fixture@example.test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	genesisGit(t, git, root, "add", "README.md")
	genesisGit(t, git, root, "commit", "-qm", "fixture")
	return root
}

func TestInventoryPrivateStatusPreservesEightProcessBudget(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"sha1", "sha256"} {
		for _, clone := range []bool{false, true} {
			name := format
			if clone {
				name += "-clone"
			}
			t.Run(name, func(t *testing.T) {
				root := genesisRepository(t, git, format)
				if clone {
					destination := filepath.Join(t.TempDir(), "clone")
					genesisGit(t, git, root, "clone", "-q", "--no-local", root, destination)
					root = destination
				}
				bin := t.TempDir()
				counter := filepath.Join(bin, "calls")
				quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
				wrapper := "#!/bin/sh\nprintf x >> " + quote(counter) + "\nexec " + quote(git) + " \"$@\"\n"
				if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
				receipt := CompileRepositoryInventory(context.Background(), root, "init", nil, "HEAD", nil)
				if receipt["operationalState"] != "COMPLETE" {
					t.Fatalf("private status exhausted or degraded inventory: %#v", receipt)
				}
				calls, err := os.ReadFile(counter)
				if err != nil || len(calls) > 8 || len(calls) == 0 {
					t.Fatalf("git calls=%d err=%v; limit is 8", len(calls), err)
				}
			})
		}
	}
}

// TestInventoryBudgetCoversSparseIndexStatusProbes pins V1-0287: a sparse-index
// checkout with a worktree config and a non-trivial repository config makes the
// private status run its full probe plan, which must fit the inventory budget,
// and an exhausted budget names git-budget-exceeded, not git-read-failed.
func TestInventoryBudgetCoversSparseIndexStatusProbes(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := genesisRepository(t, git, "sha1")
	if err := os.Mkdir(filepath.Join(root, "kept"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "kept", "a.txt"), []byte("a\n"), 0600); err != nil {
		t.Fatal(err)
	}
	genesisGit(t, git, root, "add", "kept")
	genesisGit(t, git, root, "commit", "-qm", "kept")
	genesisGit(t, git, root, "sparse-checkout", "set", "--cone", "--sparse-index", "kept")
	genesisGit(t, git, root, "config", "core.multiPackIndex", "true")
	bin := t.TempDir()
	counter := filepath.Join(bin, "calls")
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	wrapper := "#!/bin/sh\nprintf x >> " + quote(counter) + "\nexec " + quote(git) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	receipt := CompileRepositoryInventory(context.Background(), root, "init", nil, "HEAD", nil)
	if receipt["operationalState"] != "COMPLETE" {
		t.Fatalf("sparse-index inventory degraded: %#v", receipt["gaps"])
	}
	calls, err := os.ReadFile(counter)
	if err != nil || len(calls) != inventoryReads+gitstatus.MaxProcesses {
		t.Fatalf("git calls=%d err=%v; want the full probe plan %d", len(calls), err, inventoryReads+gitstatus.MaxProcesses)
	}
	repo, err := openRepository(context.Background(), root, defaultLimits(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	repo.budget = gitrun.NewBudget(0, time.Minute)
	if _, _, _, err := repo.resolve(context.Background(), "HEAD"); err != genesisError("git-budget-exceeded") {
		t.Fatalf("exhausted budget err=%v", err)
	}
}

func TestDecodeLayoutCommitRejectsAmbiguousFraming(t *testing.T) {
	root := t.TempDir()
	gitDirectory := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(gitDirectory, "objects"), 0700); err != nil {
		t.Fatal(err)
	}
	layout := gitDirectory + "\n" + gitDirectory + "\n" + root + "\n"
	for _, width := range []int{40, 64} {
		oid := strings.Repeat("a", width)
		if got, err := decodeLayoutCommit(root, []byte(layout+oid+"\n")); err != nil || got != oid {
			t.Fatalf("valid width=%d got=%q err=%v", width, got, err)
		}
	}
	for name, raw := range map[string]string{
		"extra-line":      layout + strings.Repeat("a", 40) + "\nextra\n",
		"missing-newline": layout + strings.Repeat("a", 40),
		"missing-commit":  layout,
		"relative-layout": ".git\n.git\n" + root + "\n" + strings.Repeat("a", 40) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeLayoutCommit(root, []byte(raw)); err != genesisError("invalid-repository") {
				t.Fatalf("err=%v", err)
			}
		})
	}
	for _, oid := range []string{"", strings.Repeat("A", 40), strings.Repeat("a", 39), " " + strings.Repeat("a", 40), strings.Repeat("a", 40) + "\r"} {
		if _, err := decodeLayoutCommit(root, []byte(layout+oid+"\n")); err != genesisError("invalid-commit-object") {
			t.Fatalf("oid=%q err=%v", oid, err)
		}
	}
}

func TestOpenRepositoryPreservesFailurePrecedence(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := genesisRepository(t, git, "sha1")
	for _, revision := range []string{"", "--all", "HEAD\nHEAD"} {
		if _, err := openRepository(context.Background(), root, defaultLimits(), revision); err != genesisError("invalid-revision") {
			t.Fatalf("revision=%q err=%v", revision, err)
		}
	}
	if _, err := openRepository(context.Background(), root, defaultLimits(), "missing-revision"); err != genesisError("git-read-failed") {
		t.Fatalf("missing revision err=%v", err)
	}
	for _, revision := range []string{"--all", "missing-revision"} {
		if _, err := openRepository(context.Background(), t.TempDir(), defaultLimits(), revision); err != genesisError("invalid-repository") {
			t.Fatalf("invalid repository revision=%q err=%v", revision, err)
		}
	}
}

// unsafePathRepository commits two tracked paths that safePath refuses (a backslash),
// so their records carry no path and no source classes.
func unsafePathRepository(t *testing.T) string {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := genesisRepository(t, git, "sha1")
	for _, name := range []string{"a\\b.md", "c\\d.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	genesisGit(t, git, root, "add", "--", "a\\b.md", "c\\d.md")
	genesisGit(t, git, root, "commit", "-qm", "unsafe paths")
	return root
}

func TestInventoryDoesNotClaimAbsenceOverUnsafePathEntries(t *testing.T) {
	receipt := CompileRepositoryInventory(context.Background(), unsafePathRepository(t), "init", nil, "HEAD", nil)
	if receipt["operationalState"] != "PARTIAL" {
		t.Fatalf("receipt=%#v", receipt)
	}
	for _, value := range receipt["semanticFrontier"].([]any) {
		if value.(map[string]any)["reason"] == "declared-source-absent" {
			t.Fatalf("absence claimed over unsafe-path entries: %#v", value)
		}
	}
}

func TestInventoryDoesNotReadUnsafePathBlobs(t *testing.T) {
	receipt := CompileRepositoryInventory(context.Background(), unsafePathRepository(t), "init", nil, "HEAD", nil)
	denominator := receipt["denominator"].(map[string]any)
	if denominator["inspectedEntries"] != 1 || denominator["inspectedBlobBytes"] != len("# Fixture\n") {
		t.Fatalf("denominator=%#v", denominator)
	}
}

func TestBlobReadFailureCountsOnlyEntriesItWouldRead(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := genesisRepository(t, git, "sha1")
	files := map[string][]byte{"logo.png": []byte("png\n"), "large.txt": []byte(strings.Repeat("a", 2*1024*1024+1))}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	genesisGit(t, git, root, "add", "--", "logo.png", "large.txt")
	genesisGit(t, git, root, "commit", "-qm", "unread entries")
	bin := t.TempDir()
	wrapper := "#!/bin/sh\ncase \" $* \" in *\" cat-file --batch \"*) exit 1;; esac\nexec '" + git + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	receipt := CompileRepositoryInventory(context.Background(), root, "init", nil, "HEAD", nil)
	counts := map[string]int{}
	for _, value := range receipt["gaps"].([]any) {
		counts[value.(map[string]any)["code"].(string)] = value.(map[string]any)["count"].(int)
	}
	if counts["git-read-failed"] != 1 || counts["binary-asset"] != 1 || counts["blob-too-large"] != 1 {
		t.Fatalf("gaps=%#v", receipt["gaps"])
	}
}

func TestSummaryOrdersUnsafePathSamplesWithoutPanicking(t *testing.T) {
	receipt := CompileRepositoryInventory(context.Background(), unsafePathRepository(t), "init", nil, "HEAD", nil)
	summary, err := SummarizeInventoryReceipt(receipt, 5)
	if err != nil {
		t.Fatal(err)
	}
	samples := summary["sourceSamples"].(map[string]any)["UNCLASSIFIED"].([]any)
	if len(samples) != 2 || samples[0].(map[string]any)["path"].(*string) != nil {
		t.Fatalf("unclassified samples=%#v", samples)
	}
}
