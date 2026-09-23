package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/workqueue"
)

func workFixtureScriptPrefix(t *testing.T, root, prefix string) {
	t.Helper()
	path := filepath.Join(root, "script/corvint-work-queue")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line, rest, _ := strings.Cut(string(raw), "\n")
	if err := os.WriteFile(path, []byte(line+"\n"+workSharedCompilerPrelude(t)+"\n"+prefix+"\n"+rest), 0755); err != nil {
		t.Fatal(err)
	}
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "commit", "-qm", "production boundary fixture")
}

// WQO-V0-004/005/042: production snapshot/details reach the qualified immutable
// collision compiler under a poisoned ambient executable/config/temp boundary.
func TestObserveWorkUsesOnlyTargetMaterialization(t *testing.T) {
	if root := os.Getenv("CORVINT_TEST_OBSERVE_WORK_ROOT"); root != "" {
		testObserveWorkUsesOnlyTargetMaterialization(t, root, os.Getenv("CORVINT_TEST_OBSERVE_WORK_MARKER"))
		return
	}
	t.Parallel()
	t.Run("WQO-V0-004 isolated target materialization", func(t *testing.T) {
		root := workProductionFixture(t)
		cemWrite(t, root, ".gitignore", "caller-only\n")
		workFixtureScriptPrefix(t, root, "test ! -e caller-only || exit 91\ntest \"$PWD\" != '"+root+"' || exit 92")
		cemWrite(t, root, "caller-only", "private caller bytes")
		before := materializationManifest(t, root)
		poison := t.TempDir()
		marker := filepath.Join(poison, "launched")
		if err := os.WriteFile(filepath.Join(poison, "git"), []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 99\n"), 0755); err != nil {
			t.Fatal(err)
		}
		directory, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		result := procgroup.Run(t.Context(), procgroup.Spec{
			Argv: []string{os.Args[0], "-test.run=^TestObserveWorkUsesOnlyTargetMaterialization$"}, Dir: directory,
			Env: testEnvironment(
				"CORVINT_TEST_OBSERVE_WORK_ROOT="+root,
				"CORVINT_TEST_OBSERVE_WORK_MARKER="+marker,
				"PATH="+poison,
				"TMPDIR="+filepath.Join(poison, "missing"),
				"GIT_CONFIG_COUNT=1",
				"GIT_CONFIG_KEY_0=core.worktree",
				"GIT_CONFIG_VALUE_0="+poison,
			),
			Timeout: 30 * time.Minute, OutputLimit: 1 << 20,
		})
		if result.Err != nil || result.ExitStatus != 0 {
			t.Fatalf("isolated observation: %v exit=%d\n%s\n%s", result.Err, result.ExitStatus, result.Stdout, result.Stderr)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("ambient Git ran")
		}
		after := materializationManifest(t, root)
		if len(before) != len(after) {
			t.Fatal("caller additions/removals")
		}
		for path, digest := range before {
			if after[path] != digest {
				t.Fatalf("caller changed: %s", path)
			}
		}
	})
}

func testObserveWorkUsesOnlyTargetMaterialization(t *testing.T, root, marker string) {
	capture, code := observeWork(context.Background(), root)
	if code != "" {
		t.Fatalf("observe: %s", code)
	}
	defer capture.Close()
	if capture.observation.State != workqueue.StateValidated || capture.observation.MutationState != "UNCHANGED_OBSERVED" {
		t.Fatalf("observation %#v", capture.observation)
	}
	t.Run("WQO-V0-013 receipt count", func(t *testing.T) {
		if len(capture.observation.AdapterReceipts) != 3 || !capture.closure.Complete {
			t.Fatalf("production path incomplete: %#v", capture)
		}
	})
	t.Run("WQO-V0-033 unobserved network qualifier", func(t *testing.T) {
		if capture.observation.NetworkState != "HOST_UNOBSERVED" {
			t.Fatalf("network qualification = %q", capture.observation.NetworkState)
		}
		if !slices.Contains(capture.observation.Unknowns, workqueue.UnknownNetworkUnobserved) {
			t.Fatalf("network qualification missing from %v", capture.observation.Unknowns)
		}
	})
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("ambient Git ran")
	}
	runRoot := capture.materialization.root
	capture.Close()
	if _, err := os.Stat(runRoot); !os.IsNotExist(err) {
		t.Fatal("private run root leaked")
	}
}

// WQO-V0-017: closing source rejection cannot erase an observed caller mutation.
func TestObserveWorkMutationDiagnostics(t *testing.T) {
	t.Parallel()
	workCaptureSlot(t)
	t.Run("WQO-V0-017", func(t *testing.T) {
		root := workProductionFixture(t)
		cemWrite(t, root, "victim", "before")
		workFixtureScriptPrefix(t, root, "printf changed > '"+filepath.Join(root, "victim")+"'")
		capture, code := observeWork(context.Background(), root)
		if code != "" {
			t.Fatalf("diagnostic observation lost: %s", code)
		}
		defer capture.Close()
		if capture.observation.MutationState != "CHANGED" || capture.observation.State != workqueue.StateUnknown {
			t.Fatalf("mutation %#v", capture.observation)
		}
		found := false
		for _, unknown := range capture.observation.Unknowns {
			if unknown == workqueue.UnknownMutationDetected {
				found = true
			}
		}
		if !found {
			t.Fatal("positive mutation diagnostic lost")
		}
	})
}

// WQO-V0-004/006: the actual script must also pass the checked runner path.
func TestWorkActualRunnerIntegration(t *testing.T) {
	t.Parallel()
	workCaptureSlot(t)
	root := workProductionFixture(t)
	source, err := acquireWorkSource(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer source.qualified.Close()
	materialization, err := newWorkMaterialization(context.Background(), source.qualified)
	if err != nil {
		t.Fatal(err)
	}
	defer materialization.Close()
	policy, err := workqueue.ParsePolicy(source.policyRaw)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := newWorkAdapterRunner(context.Background(), materialization.target, filepath.Join(materialization.target, policy.AdapterPath), source, policy)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	runner.env = materialization.environment
	runner.verifyTarget = materialization.Verify
	_, receipt, err := runner.run("snapshot", policy.Operations.Snapshot, 16<<20)
	if err != nil {
		t.Fatalf("actual checked runner: %v; receipt=%#v", err, receipt)
	}
}

// WQO-V0-017/025: final freshness must preserve mutation from the first bracket.
func TestObserveWorkFreshnessRetainsMutation(t *testing.T) {
	t.Parallel()
	workCaptureSlot(t)
	root := workProductionFixture(t)
	cemWrite(t, root, ".gitignore", "ignored-queue\n")
	workFixtureScriptPrefix(t, root, "printf changed > '"+filepath.Join(root, "ignored-queue")+"'")
	cemWrite(t, root, "ignored-queue", "before")
	capture, code := observeWork(context.Background(), root)
	if code != "" {
		t.Fatal(code)
	}
	defer capture.Close()
	if capture.observation.MutationState != "CHANGED" {
		t.Fatal("initial mutation not detected")
	}
	if drift, code := workFreshAtReturn(context.Background(), root, capture); drift || code != "" {
		t.Fatalf("equal pinned checkpoint became stale or failed: %v %s", drift, code)
	}
	if capture.observation.MutationState != "CHANGED" || capture.observation.State != workqueue.StateUnknown {
		t.Fatalf("mutation erased %#v", capture.observation)
	}
}

// WQO-V0-004/005: source refusals are proven at the real observer launch boundary.
func TestWorkSourceRefusalNeverLaunchesAdapter(t *testing.T) {
	t.Run("WQO-V0-005", func(t *testing.T) {
		cases := []struct {
			name   string
			change func(*testing.T, string)
		}{
			{"dirty", func(t *testing.T, p string) { cemWrite(t, p, "tracked", "changed") }},
			{"untracked", func(t *testing.T, p string) { cemWrite(t, p, "untracked", "new") }},
			{"skip-hidden-edit", func(t *testing.T, p string) {
				materializationGit(t, p, "update-index", "--skip-worktree", "tracked")
				cemWrite(t, p, "tracked", "changed")
			}},
			{"assume-hidden-edit", func(t *testing.T, p string) {
				materializationGit(t, p, "update-index", "--assume-unchanged", "tracked")
				cemWrite(t, p, "tracked", "changed")
			}},
			{"split-index", func(t *testing.T, p string) { materializationGit(t, p, "update-index", "--split-index") }},
			{"sparse", func(t *testing.T, p string) { materializationGit(t, p, "config", "core.sparseCheckout", "true") }},
			{"alternate-index", func(t *testing.T, p string) { t.Setenv("GIT_INDEX_FILE", filepath.Join(p, ".git/index")) }},
			{"grafts", func(t *testing.T, p string) { cemWrite(t, p, ".git/info/grafts", "") }},
			{"mode", func(t *testing.T, p string) {
				if err := os.Chmod(filepath.Join(p, "tracked"), 0755); err != nil {
					t.Fatal(err)
				}
			}},
			{"escaping-link", func(t *testing.T, p string) {
				if err := os.Symlink("../../outside", filepath.Join(p, "link")); err != nil {
					t.Fatal(err)
				}
				materializationGit(t, p, "add", "link")
				materializationGit(t, p, "commit", "-qm", "unsupported link")
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				// GIT_INDEX_FILE is process-global with no observer seam, so that case
				// runs synchronously before its parallel siblings resume.
				if tc.name != "alternate-index" {
					t.Parallel()
				}
				root := materializationFixture(t)
				policy, err := os.ReadFile(filepath.Join("..", "..", workPolicyPath))
				if err != nil {
					t.Fatal(err)
				}
				cemWrite(t, root, workPolicyPath, string(policy))
				marker := filepath.Join(t.TempDir(), "adapter-started")
				cemWrite(t, root, "script/corvint-work-queue", "#!/bin/sh\nprintf launched > '"+marker+"'\n")
				if err := os.Chmod(filepath.Join(root, "script/corvint-work-queue"), 0755); err != nil {
					t.Fatal(err)
				}
				materializationGit(t, root, "add", ".")
				materializationGit(t, root, "commit", "-qm", "sentinel adapter")
				tc.change(t, root)
				capture, code := observeWork(context.Background(), root)
				if capture != nil {
					capture.Close()
				}
				if code != "SOURCE_UNQUALIFIED" {
					t.Fatalf("refusal %q", code)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("adapter launched before source refusal")
				}
			})
		}
	})
}
