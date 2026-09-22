package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func guidanceWrite(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func guidanceFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "test@example.test"}, {"config", "user.name", "Test"}} {
		guidanceGit(t, root, args...)
	}
	for name, body := range map[string]string{
		"go.mod":                   "module example.test/demo\n\ngo 1.27.0\n",
		"cmd/demo/main.go":         "package main\n// Feature: greeting\nfunc main() {}\n",
		"cmd/demo/main_test.go":    "package main\nimport \"testing\"\nfunc TestGreeting(t *testing.T) {}\n",
		"web/e2e/greeting.spec.ts": "// Scenario: greeting\n",
		"package.json":             "{\"name\":\"demo\",\"bin\":{\"demo\":\"cmd/demo/main.go\"},\"scripts\":{\"test\":\"touch MUST_NOT_EXECUTE\"}}\n",
		".gitignore":               ".corvint/\n",
	} {
		guidanceWrite(t, root, name, body)
	}
	guidanceGit(t, root, "add", ".")
	guidanceGit(t, root, "commit", "-qm", "base")
	base := guidanceGit(t, root, "rev-parse", "HEAD")
	guidanceWrite(t, root, ".corvint/self-observations.jsonl", "sentinel\n")
	guidanceWrite(t, root, ".corvint/index/sentinel", "unchanged\n")
	return root, base
}

func guidanceRead(t *testing.T, root, command, base string) (guidanceReceipt, []byte) {
	t.Helper()
	raw, err := compileGuidance(context.Background(), guidanceInvocation{Root: root, Command: command, Base: base, MaxRefs: 32}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var result guidanceReceipt
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result, raw
}

func TestRepositoryGuidanceImmutableDiscovery(t *testing.T) {
	t.Run("RGV-V0-003 immutable evidence", func(t *testing.T) {
		t.Run("RGV-V0-005 overview composition", func(t *testing.T) {
			t.Run("RGV-V0-012 deterministic nonmutation", func(t *testing.T) {
				root, _ := guidanceFixture(t)
				before := treeDigest(t, root)
				for _, command := range []string{"features", "overview"} {
					out, raw := guidanceRead(t, root, command, "")
					_, again := guidanceRead(t, root, command, "")
					if !bytes.Equal(raw, again) {
						t.Fatal("nondeterministic output")
					}
					if out.Revision != guidanceGit(t, root, "rev-parse", "HEAD") || out.Tree != guidanceGit(t, root, "rev-parse", "HEAD^{tree}") {
						t.Fatal("identity mismatch")
					}
					if out.Authority != "inferred" || out.Mutates || !out.Experimental {
						t.Fatal("authority mismatch")
					}
					found := false
					for _, f := range out.Features {
						if f.Label == "greeting" {
							found = true
							if len(f.Evidence) != 2 {
								t.Fatalf("collision lost evidence: %+v", f)
							}
						}
						for _, e := range f.Evidence {
							if e.Blob != guidanceGit(t, root, "rev-parse", "HEAD:"+e.Path) || e.Start < 1 || e.End < e.Start {
								t.Fatal("invalid immutable evidence")
							}
						}
					}
					if !found || out.Index["state"] != "UNKNOWN" || (command == "overview" && (len(out.Entrypoints) < 2 || len(out.Tests) < 3)) {
						t.Fatalf("missing composition: %+v", out)
					}
					if command == "features" && (bytes.Contains(raw, []byte(`"languages"`)) || bytes.Contains(raw, []byte(`"entrypoints"`)) || bytes.Contains(raw, []byte(`"tests"`))) {
						t.Fatal("features emitted overview-only fields")
					}
					for _, call := range out.NextCalls {
						if !call.Inert || strings.Contains(strings.Join(call.Argv, " "), "touch") {
							t.Fatal("unsafe nextCall")
						}
					}
				}
				if treeDigest(t, root) != before {
					t.Fatal("repository state mutated")
				}
			})
		})
	})
}

func TestRepositoryGuidanceReviewBranches(t *testing.T) {
	t.Run("RGV-V0-007 affected composition", func(t *testing.T) {
		t.Run("RGV-V0-009 ancestry exclusions", func(t *testing.T) {
			root, base := guidanceFixture(t)
			guidanceGit(t, root, "checkout", "-qb", "overlap", base)
			guidanceWrite(t, root, "cmd/demo/main.go", "package main\n// Feature: greeting\nfunc main() {println(1)}\n")
			guidanceGit(t, root, "commit", "-qam", "peer")
			guidanceGit(t, root, "branch", "stack-parent")
			guidanceWrite(t, root, "peer.txt", "peer\n")
			guidanceGit(t, root, "add", "peer.txt")
			guidanceGit(t, root, "commit", "-qm", "stack-tip")
			guidanceGit(t, root, "checkout", "-qb", "disjoint", base)
			guidanceWrite(t, root, "separate.txt", "separate\n")
			guidanceGit(t, root, "add", "separate.txt")
			guidanceGit(t, root, "commit", "-qm", "disjoint")
			guidanceGit(t, root, "checkout", "-q", "main")
			guidanceWrite(t, root, "cmd/demo/main.go", "package main\n// Feature: greeting\nfunc main() {println(2)}\n")
			guidanceGit(t, root, "commit", "-qam", "target")
			target := guidanceGit(t, root, "rev-parse", "HEAD")
			guidanceGit(t, root, "branch", "ancestor", base)
			guidanceGit(t, root, "checkout", "-qb", "descendant")
			guidanceWrite(t, root, "descendant.txt", "d\n")
			guidanceGit(t, root, "add", ".")
			guidanceGit(t, root, "commit", "-qm", "descendant")
			guidanceGit(t, root, "checkout", "-q", "main")
			before := treeDigest(t, root)
			out, raw := guidanceRead(t, root, "review", base)
			_, again := guidanceRead(t, root, "review", base)
			if !bytes.Equal(raw, again) {
				t.Fatal("review nondeterministic")
			}
			if out.Review.Affected.Revision != target || out.Review.Affected.Range.Base != base || len(out.Review.ChangedFeatures) == 0 {
				t.Fatal("affected identity or changed scope missing")
			}
			states := map[string]string{}
			for _, r := range out.Review.Overlaps {
				states[r.Ref] = r.State
			}
			if states["refs/heads/overlap"] != "OVERLAP" || states["refs/heads/disjoint"] != "DISJOINT" {
				t.Fatalf("bad overlaps %v", states)
			}
			skips := map[string]string{}
			for _, r := range out.Review.Skipped {
				skips[r.Ref] = r.Reason
			}
			for ref, reason := range map[string]string{"main": "current-ref", "ancestor": "ancestor-of-target", "descendant": "descendant-of-target", "stack-parent": "ancestry-stacked"} {
				if skips["refs/heads/"+ref] != reason {
					t.Fatalf("missing skip %s: %v", ref, skips)
				}
			}
			actual, err := compileAffected(context.Background(), affectedInvocation{Root: root, Base: base})
			if err != nil {
				t.Fatal(err)
			}
			actualRaw, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			composedRaw, err := json.Marshal(out.Review.Affected)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actualRaw, composedRaw) {
				t.Fatalf("immutable composition differs from actual affected receipt:\n%s\n%s", actualRaw, composedRaw)
			}
			if treeDigest(t, root) != before {
				t.Fatal("review mutated repository")
			}
		})
	})
}

func TestRepositoryGuidanceRefBoundsAndDrift(t *testing.T) {
	t.Run("RGV-V0-008 ref capture and drift", func(t *testing.T) {
		t.Run("RGV-V0-010 bounded branch paths", func(t *testing.T) {
			root, base := guidanceFixture(t)
			guidanceGit(t, root, "checkout", "-qb", "large")
			for i := 0; i < 257; i++ {
				guidanceWrite(t, root, fmt.Sprintf("large/%03d.txt", i), "data\n")
			}
			guidanceGit(t, root, "add", ".")
			guidanceGit(t, root, "commit", "-qm", "large")
			tip := guidanceGit(t, root, "rev-parse", "HEAD")
			guidanceGit(t, root, "checkout", "-q", "main")
			guidanceWrite(t, root, "cmd/demo/main.go", "package main\nfunc main() {println(2)}\n")
			guidanceGit(t, root, "commit", "-qam", "target")
			out, _ := guidanceRead(t, root, "review", base)
			if len(out.Review.Overlaps) != 1 || out.Review.Overlaps[0].State != "UNKNOWN" || len(out.Review.Overlaps[0].Paths) != 0 {
				t.Fatalf("truncated negative: %+v", out.Review.Overlaps)
			}
			_, err := compileGuidance(context.Background(), guidanceInvocation{Root: root, Command: "review", Base: base, MaxRefs: 32}, func() { guidanceGit(t, root, "update-ref", "refs/heads/new", tip) })
			if err == nil || !strings.Contains(err.Error(), "ref drift") {
				t.Fatalf("ref drift accepted: %v", err)
			}
			_, err = compileGuidance(context.Background(), guidanceInvocation{Root: root, Command: "overview", MaxRefs: 32}, func() { guidanceWrite(t, root, "dirty.txt", "changed") })
			if err == nil {
				t.Fatal("worktree drift accepted")
			}
		})
	})
}

func TestRepositoryGuidanceRefusalsDoNotObserve(t *testing.T) {
	t.Run("RGV-V0-002 clean read-only refusal", func(t *testing.T) {
		root, base := guidanceFixture(t)
		guidanceWrite(t, root, "dirty.txt", "dirty")
		before := treeDigest(t, root)
		for _, args := range [][]string{{"features"}, {"overview", "--unexpected"}, {"review", "--base", base}, {"review", "--base", "HEAD"}, {"review", "--base", base, "--max-refs", "33"}} {
			var stdout, stderr bytes.Buffer
			code := run(append([]string{"--root", root}, args...), strings.NewReader(""), &stdout, &stderr)
			if code != 2 {
				t.Fatalf("accepted %v: %d %s", args, code, stdout.String())
			}
			if treeDigest(t, root) != before {
				t.Fatalf("refusal mutated state: %v", args)
			}
		}
	})
}

func TestRepositoryGuidanceAncestryTrustAndDeadline(t *testing.T) {
	root, base := guidanceFixture(t)
	guidanceWrite(t, root, ".git/info/grafts", base+"\n")
	if _, err := compileGuidance(context.Background(), guidanceInvocation{Root: root, Command: "review", Base: base, MaxRefs: 32}, nil); err == nil {
		t.Fatal("grafts accepted")
	}
	if err := os.Remove(filepath.Join(root, ".git/info/grafts")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compileGuidance(ctx, guidanceInvocation{Root: root, Command: "overview", MaxRefs: 32}, nil); err == nil {
		t.Fatal("cancelled request accepted")
	}
}

func guidanceGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(affectedGit(t, root, args...))
}

func TestRepositoryGuidanceCandidateAndOverlapBounds(t *testing.T) {
	root, base := guidanceFixture(t)
	var markers strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&markers, "// Feature: candidate-%03d\n", i)
	}
	guidanceWrite(t, root, "markers.go", markers.String())
	guidanceWrite(t, root, "oversized.txt", strings.Repeat("x", (256<<10)+1))
	guidanceWrite(t, root, "broken/package.json", "{")
	guidanceWrite(t, root, "routes.ts", "router.get('/hello', handler); server.registerTool('mcp_demo', {});\n")
	guidanceGit(t, root, "add", ".")
	guidanceGit(t, root, "commit", "-qm", "bounded discovery")
	out, _ := guidanceRead(t, root, "features", "")
	if len(out.Features) != 256 || out.Omissions["features"] == 0 || out.Omissions["unread-sources"] == 0 || out.Omissions["manifest-parse"] == 0 {
		t.Fatalf("missing bound omissions: %v", out.Omissions)
	}
	guidanceGit(t, root, "checkout", "-qb", "peer", base)
	for i := 0; i < 70; i++ {
		guidanceWrite(t, root, fmt.Sprintf("shared/%03d.txt", i), "peer")
	}
	guidanceGit(t, root, "add", ".")
	guidanceGit(t, root, "commit", "-qm", "peer")
	guidanceGit(t, root, "checkout", "-q", "main")
	for i := 0; i < 70; i++ {
		guidanceWrite(t, root, fmt.Sprintf("shared/%03d.txt", i), "target")
	}
	guidanceGit(t, root, "add", ".")
	guidanceGit(t, root, "commit", "-qm", "target")
	out, _ = guidanceRead(t, root, "review", base)
	if len(out.Review.Overlaps) != 1 || len(out.Review.Overlaps[0].Paths) != 64 || out.Review.Overlaps[0].State != "OVERLAP_INCOMPLETE" || out.Omissions["overlap-paths"] != 6 {
		t.Fatalf("bad overlap bound: %+v %v", out.Review.Overlaps, out.Omissions)
	}
	guidanceGit(t, root, "branch", "peer-copy", "peer")
	raw, err := compileGuidance(context.Background(), guidanceInvocation{Root: root, Command: "review", Base: base, MaxRefs: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Omissions["refs"] != 1 {
		t.Fatalf("ref bound unreported: %v", out.Omissions)
	}
}

func TestRepositoryGuidanceCLIAndLiteralRegistration(t *testing.T) {
	t.Run("RGV-V0-001 public dispatch", func(t *testing.T) {
		t.Run("RGV-V0-004 literal registrations", func(t *testing.T) {
			t.Run("RGV-V0-006 inert calls", func(t *testing.T) {
				root, _ := guidanceFixture(t)
				guidanceWrite(t, root, "routes.ts", "router.get('/hello', handler); server.registerTool('mcp_demo', {});\n")
				guidanceGit(t, root, "add", ".")
				guidanceGit(t, root, "commit", "-qm", "routes")
				before := treeDigest(t, root)
				for _, args := range [][]string{{"features"}, {"overview"}, {"help", "features"}, {"overview", "--help"}, {"help", "review"}} {
					var stdout, stderr bytes.Buffer
					if code := run(append([]string{"--root", root}, args...), strings.NewReader(""), &stdout, &stderr); code != 0 {
						t.Fatalf("CLI %v: %d %s", args, code, stderr.String())
					}
					if len(args) == 1 && (!strings.Contains(stdout.String(), "/hello") || !strings.Contains(stdout.String(), "mcp_demo")) {
						t.Fatal("literal registrations absent")
					}
				}
				if treeDigest(t, root) != before {
					t.Fatal("CLI mutated state")
				}
			})
		})
	})
}

func TestRepositoryGuidanceCurrentRefDrift(t *testing.T) {
	root, base := guidanceFixture(t)
	guidanceGit(t, root, "branch", "same-tip")
	_, err := compileGuidance(context.Background(), guidanceInvocation{Root: root, Command: "review", Base: base, MaxRefs: 32}, func() { guidanceGit(t, root, "checkout", "-q", "same-tip") })
	if err == nil || !strings.Contains(err.Error(), "current ref drift") {
		t.Fatalf("current ref drift accepted: %v", err)
	}
}

func TestRepositoryGuidanceCurrentRefExcludedBeforeBudget(t *testing.T) {
	root, base := guidanceFixture(t)
	guidanceGit(t, root, "branch", "aaa")
	guidanceGit(t, root, "branch", "bbb")
	raw, err := compileGuidance(context.Background(), guidanceInvocation{Root: root, Command: "review", Base: base, MaxRefs: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out guidanceReceipt
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	for _, skip := range out.Review.Skipped {
		if skip.Ref == "refs/heads/main" && skip.Reason == "current-ref" {
			return
		}
	}
	t.Fatal("current ref lost to budget")
}

func TestRepositoryGuidanceCleanupFailureRefuses(t *testing.T) {
	t.Run("RGV-V0-011 cleanup failure refuses success", func(t *testing.T) {
		root, base := guidanceFixture(t)
		before := treeDigest(t, root)
		cleanupFailure := errors.New("injected cleanup failure")
		var scratch string
		remove := func(p string) error {
			scratch = p
			if err := os.RemoveAll(p); err != nil {
				t.Fatal(err)
			}
			return cleanupFailure
		}
		var stdout, stderr bytes.Buffer
		code := runGuidance(context.Background(), guidanceInvocation{Root: root, Command: "review", Base: base, MaxRefs: 32, removeScratch: remove}, &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "scratch cleanup failed") {
			t.Fatalf("cleanup failure reported success: %d %s %s", code, &stdout, &stderr)
		}
		if scratch == "" {
			t.Fatal("cleanup not attempted")
		}
		if _, err := os.Stat(scratch); !os.IsNotExist(err) {
			t.Fatalf("test scratch survived: %v", err)
		}
		original := errors.New("original operation failure")
		joined := cleanupGuidanceScratch("scratch", original, func(string) error { return cleanupFailure })
		if !errors.Is(joined, original) || !errors.Is(joined, cleanupFailure) {
			t.Fatal("lost original or cleanup failure")
		}
		if treeDigest(t, root) != before {
			t.Fatal("cleanup refusal mutated repository")
		}
	})
}

func TestRepositoryGuidanceDeletedFeatureIsExplicitUnknown(t *testing.T) {
	root, base := guidanceFixture(t)
	guidanceGit(t, root, "rm", "web/e2e/greeting.spec.ts")
	guidanceGit(t, root, "commit", "-qm", "delete scenario")
	out, _ := guidanceRead(t, root, "review", base)
	if !strings.Contains(strings.Join(out.Unknown, " "), "deleted/base-only candidates are not discovered") {
		t.Fatal("base-only inferred scope silently omitted")
	}
	if len(out.Review.Affected.Range.Paths) != 1 || out.Review.Affected.Range.Paths[0] != "web/e2e/greeting.spec.ts" {
		t.Fatal("deleted path absent from affected receipt")
	}
}

func TestRepositoryGuidanceEmptyOverviewFieldsRemainExplicit(t *testing.T) {
	root, _ := guidanceFixture(t)
	guidanceGit(t, root, "rm", "-r", "cmd", "web", "go.mod", "package.json")
	guidanceGit(t, root, "commit", "-qm", "empty overview")
	_, raw := guidanceRead(t, root, "overview", "")
	for _, field := range []string{"languages", "entrypoints", "manifests", "tests"} {
		if !bytes.Contains(raw, []byte(`"`+field+`":[]`)) {
			t.Fatalf("empty overview field omitted: %s", field)
		}
	}
}

func TestRepositoryGuidancePrivateStatusBudget(t *testing.T) {
	t.Run("RGV-V0-011 private Git status budget", func(t *testing.T) {
		t.Run("RGV-V0-010 bounded public output", func(t *testing.T) {
			root, base := guidanceFixture(t)
			for i := 0; i < 2000; i++ {
				guidanceWrite(t, root, fmt.Sprintf("inventory/%04d.txt", i), fmt.Sprintf("Feature: inventory-%04d\n%s\n", i, strings.Repeat("source evidence ", 32)))
			}
			guidanceGit(t, root, "add", ".")
			guidanceGit(t, root, "commit", "-qm", "nontrivial inventory")
			guidanceGit(t, root, "update-index", "--index-version=4")
			before := treeDigest(t, root)
			for _, args := range [][]string{{"features"}, {"overview"}, {"review", "--base", base, "--max-refs", "8"}} {
				var stdout, stderr bytes.Buffer
				code := run(append([]string{"--root", root}, args...), strings.NewReader(""), &stdout, &stderr)
				if code != 0 {
					t.Fatalf("private status %v: exit=%d stderr=%s", args, code, &stderr)
				}
				if stdout.Len() == 0 || stdout.Len() > (1<<20)+1 {
					t.Fatalf("public output bound: %d", stdout.Len())
				}
				var receipt guidanceReceipt
				if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
					t.Fatal(err)
				}
				if len(receipt.Features) != 256 || receipt.Omissions["features"] == 0 {
					t.Fatal("nontrivial discovery bound unexercised")
				}
			}
			if treeDigest(t, root) != before {
				t.Fatal("private-status invocation mutated repository")
			}
		})
	})
}
