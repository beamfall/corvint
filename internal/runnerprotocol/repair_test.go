// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func TestRSPV0029SymlinkedRepositoryMutation(t *testing.T) {
	resolved := fixtureResolved(t)
	root := fixtureRepo(t, "mutation")
	// Both snapshots have the same dirty status; only content hashing detects it.
	write(t, filepath.Join(root, "tracked.txt"), "already dirty")
	link := filepath.Join(t.TempDir(), "repository")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	s, err := Open(context.Background(), resolved, Options{Repository: link, RunDir: t.TempDir(), CoreVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	if s.options.Repository != root {
		t.Fatalf("repository not canonical: %s", s.options.Repository)
	}
	requireFailure(t, s.Close(context.Background()), "repository-mutation")
}

func TestRSPV0029SymlinkedRepositoryRunDirectory(t *testing.T) {
	resolved := fixtureResolved(t)
	root := fixtureRepo(t, "reference")
	link := filepath.Join(t.TempDir(), "repository")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	run := filepath.Join(root, "run")
	if err := os.Mkdir(run, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(context.Background(), resolved, Options{Repository: link, RunDir: run, CoreVersion: "test"})
	if s != nil {
		_ = s.Close(context.Background())
	}
	requireFailure(t, err, "run-directory-inside-repository")
}

func TestRSPV0028MissingAndMalformedProjectPins(t *testing.T) {
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	pin := Pin{Name: "reference", Protocol: Protocol, Executable: "reference", SHA256: strings.Repeat("a", 64), Runners: []string{ReferenceRunner}}
	valid, err := json.Marshal(Discovery{Schema: DiscoverySchema, Plugins: []Pin{pin}})
	if err != nil {
		t.Fatal(err)
	}
	missing := fmt.Errorf("blob absent: %w", fs.ErrNotExist)
	for _, tc := range []struct {
		name             string
		base, head       []byte
		baseErr, headErr error
		ci               bool
		fullAll, changed bool
		pins             int
	}{
		{name: "local-absent", baseErr: missing},
		{name: "ci-absent", baseErr: missing, headErr: missing, ci: true},
		{name: "local-malformed", base: []byte(`{}`), fullAll: true},
		{name: "local-empty-file", base: []byte{}, fullAll: true},
		{name: "local-unreadable", baseErr: errors.New("read failed"), fullAll: true},
		{name: "ci-added", baseErr: missing, head: valid, ci: true, changed: true},
		{name: "ci-removed", base: valid, headErr: missing, ci: true, changed: true, pins: 1},
		{name: "ci-malformed", baseErr: missing, head: []byte(`{}`), ci: true, fullAll: true},
		{name: "ci-unreadable", baseErr: missing, headErr: errors.New("read failed"), ci: true, fullAll: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := func(rev, path string) ([]byte, error) {
				if path != ".corvint/runner-plugins.json" {
					t.Fatal(path)
				}
				if rev == base {
					return tc.base, tc.baseErr
				}
				if !tc.ci || rev != head {
					t.Fatalf("unexpected revision %s", rev)
				}
				return tc.head, tc.headErr
			}
			plan := ReadProjectPins(read, base, head, tc.ci)
			if (plan.FullAll != "") != tc.fullAll || len(plan.Pins) != tc.pins {
				t.Fatalf("unexpected plan: %+v", plan)
			}
			if tc.changed {
				if len(plan.Full) != 1 || plan.Full[ReferenceRunner] != "candidate-pins-changed" {
					t.Fatalf("lost claims: %+v", plan)
				}
			} else if len(plan.Full) != 0 {
				t.Fatalf("unexpected runner claims: %+v", plan)
			}
		})
	}
}

func TestRSPV0025CompiledInClaimsRefused(t *testing.T) {
	runners := append(registry.Runners(), "script", "make")
	for _, runner := range runners {
		t.Run(runner, func(t *testing.T) {
			pin := Pin{Name: "reference", Protocol: Protocol, Executable: "reference", SHA256: strings.Repeat("a", 64), Runners: []string{runner}}
			data, err := json.Marshal(Discovery{Schema: DiscoverySchema, Plugins: []Pin{pin}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = DecodeDiscovery(data)
			requireFailure(t, err, "compiled-in-runner-claim:"+runner)
			if plan := PlanDiscovery(data, data, false); plan.FullAll == "" {
				t.Fatalf("claim narrowed: %+v", plan)
			}
			_, err = Resolve(pin, Discovery{}, Discovery{})
			requireFailure(t, err, "compiled-in-runner-claim:"+runner)
		})
	}
	if _, compiled := registry.Lookup(ReferenceRunner); compiled {
		t.Fatal("reference claims compiled-in kind")
	}
}

func TestRSPV0027NulEnvironmentRefused(t *testing.T) {
	s, c := openFixture(t, "nul-environment", Limits{})
	plugin, err := s.Plugin(ReferenceRunner)
	if err != nil {
		t.Fatal(err)
	}
	_, err = plugin.Command(context.Background(), registry.CommandRequest{Key: c.Key, Purpose: "run", RunDir: c.RunDir})
	requireFailure(t, err, "schema-violation:string")
	requireFailure(t, s.Close(context.Background()), "schema-violation:string")
	if s.staged != nil {
		t.Fatal("failed session retained evidence")
	}
}

func TestRSPV0029HookMutationFailsSession(t *testing.T) {
	s, _ := openFixture(t, "hook-mutation", Limits{})
	requireFailure(t, s.Close(context.Background()), "repository-mutation")
	if s.staged != nil {
		t.Fatal("failed session retained evidence")
	}
}

func TestRSPV0029GitMetadataMutation(t *testing.T) {
	for _, linked := range []bool{false, true} {
		for _, kind := range []string{"config", "worktree-config", "hook", "fsmonitor"} {
			t.Run(fmt.Sprintf("linked=%t/%s", linked, kind), func(t *testing.T) {
				resolved := fixtureResolved(t)
				root := fixtureRepo(t, "reference")
				if linked {
					worktree := filepath.Join(t.TempDir(), "worktree")
					gitFixture(t, root, "worktree", "add", "--detach", worktree)
					root = worktree
				}
				gitdir, common, err := gitstatus.Directories(root)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "worktree-config" {
					gitFixture(t, root, "config", "extensions.worktreeConfig", "true")
					write(t, filepath.Join(gitdir, "config.worktree"), "[repair]\nchanged = false\n")
				}
				s, err := Open(context.Background(), resolved, Options{Repository: root, RunDir: t.TempDir(), CoreVersion: "test"})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = s.Close(context.Background()) })
				s.staged["repair"] = []registry.NativeEvidence{{}}
				marker := filepath.Join(t.TempDir(), "executed")
				switch kind {
				case "config":
					gitFixture(t, root, "config", "repair.changed", "true")
				case "worktree-config":
					write(t, filepath.Join(gitdir, "config.worktree"), "[repair]\nchanged = true\n")
				case "hook", "fsmonitor":
					hook := filepath.Join(common, "hooks", "planted")
					write(t, hook, "#!/bin/sh\n: > '"+marker+"'\n")
					if err := os.Chmod(hook, 0700); err != nil {
						t.Fatal(err)
					}
					if kind == "fsmonitor" {
						gitFixture(t, root, "config", "core.fsmonitor", hook)
					}
				}
				requireFailure(t, s.Close(context.Background()), "repository-mutation")
				if s.staged != nil {
					t.Fatal("failed session retained evidence")
				}
				if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("hook executed or marker unreadable: %v", err)
				}
			})
		}
	}
}

func TestRSPV0029IgnoredTreeStillCovered(t *testing.T) {
	root := fixtureRepo(t, "reference")
	write(t, filepath.Join(root, ".gitignore"), "node_modules/\n")
	dir := filepath.Join(root, "node_modules")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "data"), "before")
	resolved := fixtureResolved(t)
	session, err := Open(context.Background(), resolved, Options{Repository: root, RunDir: t.TempDir(), CoreVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close(context.Background()) })
	write(t, filepath.Join(dir, "data"), "after")
	requireFailure(t, session.Close(context.Background()), "repository-mutation")
}

// Unknown pointer syntax cannot establish a complete metadata snapshot (RSP-V0-029).
func TestRSPV0029UnknownGitPointerFailsClosed(t *testing.T) {
	root := fixtureRepo(t, "reference")
	linked := filepath.Join(t.TempDir(), "worktree")
	gitFixture(t, root, "worktree", "add", "--detach", linked)
	s, err := Open(context.Background(), fixtureResolved(t), Options{Repository: linked, RunDir: t.TempDir(), CoreVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	s.staged["repair"] = []registry.NativeEvidence{{}}
	write(t, filepath.Join(linked, ".git"), "future-gitdir: "+filepath.Join(root, ".git")+"\n")
	if _, _, err := gitstatus.Directories(linked); err == nil {
		t.Fatal("unknown pointer form resolved")
	}
	if _, err := repositorySnapshot(context.Background(), linked); err == nil {
		t.Fatal("unknown pointer form certified")
	}
	if err := s.Close(context.Background()); err == nil || !strings.Contains(err.Error(), "repository-status-unavailable:") {
		t.Fatalf("expected unavailable snapshot, got %v", err)
	}
	if s.staged != nil {
		t.Fatal("failed session retained evidence")
	}
}

func TestRSPV0028DocumentedDiscovery(t *testing.T) {
	data, err := os.ReadFile("../../docs/specs/runner-selection-plugins-v0.md")
	if err != nil {
		t.Fatal(err)
	}
	_, clause, ok := strings.Cut(string(data), "- `RSP-V0-028`:")
	if !ok {
		t.Fatal("discovery requirement missing")
	}
	clause, _, _ = strings.Cut(clause, "- `RSP-V0-029`:")
	_, example, ok := strings.Cut(clause, "```json\n")
	if !ok {
		t.Fatal("discovery example missing")
	}
	example, _, ok = strings.Cut(example, "```")
	if !ok {
		t.Fatal("discovery example unterminated")
	}
	discovery, err := DecodeDiscovery([]byte(strings.TrimSpace(example)))
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Plugins) != 1 || len(discovery.Plugins[0].Runners) != 1 || discovery.Plugins[0].Runners[0] != ReferenceRunner {
		t.Fatalf("example must claim the reference twin: %+v", discovery)
	}
}
