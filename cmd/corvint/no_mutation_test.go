package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// selfObservationsRelativePath is the one write AGENTS.md invariant 4 /
// SOL-V0-007 permits a non-query command: the bounded local self-observation
// ledger. Every no-mutation assertion below excludes only this path from the
// repository digest.
const selfObservationsRelativePath = ".corvint/self-observations.jsonl"

// TestCLIReadVerbsLeaveTheRepositoryByteIdentical restates the 2026-09-12
// black-box read-verb audit (docs/agent-memory/tests.md) as a CLI-level
// assertion: witness, surprise, kernel, lrf, frontier, test-validity,
// depsource, dogfood-ocm status, and the adapter subcommands (codex,
// claude-code; source-view already carries its own check in
// TestClaudeSourceHandoffCLI) write nothing to the repository they read,
// including .git and .corvint, apart from the permitted self-observations
// ledger.
func TestCLIReadVerbsLeaveTheRepositoryByteIdentical(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		setup    func(t *testing.T) (root string, invoke func() int)
		wantExit int
	}{
		{
			name: "witness refuses an unknown base without reading further",
			setup: func(t *testing.T) (string, func() int) {
				root := cliRepository(t)
				return root, func() int {
					code, _, _ := runCLI(t, "--root", root, "witness", "--base", strings.Repeat("a", 40))
					return code
				}
			},
			wantExit: 2,
		},
		{
			name: "surprise refuses an unavailable base after reading the repository",
			setup: func(t *testing.T) (string, func() int) {
				root := cliRepository(t)
				target := cemGit(t, root, "rev-parse", "HEAD")
				return root, func() int {
					code, _, _ := runCLI(t, "--root", root, "surprise", "--task", "t",
						"--base", strings.Repeat("a", 40), "--target", target)
					return code
				}
			},
			wantExit: 2,
		},
		{
			name: "kernel renders the governance envelope",
			setup: func(t *testing.T) (string, func() int) {
				root := kernelRepository(t)
				return root, func() int {
					code, _, _ := runCLI(t, "--root", root, "kernel")
					return code
				}
			},
			wantExit: 0,
		},
		{
			name: "lrf accepts the canonical-derived CEM/02 fixture",
			setup: func(t *testing.T) (string, func() int) {
				fixture := newLRFFixture(t, wire.Spec02)
				return fixture.root, func() int {
					code, _, _ := runCLI(t, "--root", fixture.root, "lrf", "--cem", fixture.mapPath,
						"--expected-base", fixture.base, "--target", fixture.target)
					return code
				}
			},
			wantExit: 0,
		},
		{
			name: "frontier reports an EMPTY frontier through the registered adapters",
			setup: func(t *testing.T) (string, func() int) {
				root := frontierRepository(t)
				ctx := registerFrontierAdapters(t,
					stubFrontierVerifier{universe: frontierUniverse(0)},
					stubFrontierTCQ{id: "tcq:sha256:" + frontierHex("tcq")})
				return root, func() int {
					stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
					return runContext(ctx, frontierArguments(root, "--json"), strings.NewReader(""), stdout, stderr)
				}
			},
			wantExit: 0,
		},
		{
			name: "test-validity abstains without a receipt",
			setup: func(t *testing.T) (string, func() int) {
				root := cliRepository(t)
				return root, func() int {
					code, _, _ := runCLI(t, "--root", root, "test-validity")
					return code
				}
			},
			wantExit: 0,
		},
		{
			name: "migration-ratchet compares an external immutable profile",
			setup: func(t *testing.T) (string, func() int) {
				root := cliRepository(t)
				profile := writeMigrationRatchetProfile(t, migrationRatchetProfile())
				return root, func() int {
					code, _, _ := runCLI(t, "--root", root, "migration-ratchet", "--profile", profile)
					return code
				}
			},
			wantExit: 0,
		},
		{
			name: "depsource refuses a missing module argument",
			setup: func(t *testing.T) (string, func() int) {
				root := depsourceRoot(t)
				return root, func() int {
					code, _, _ := runCLI(t, "--root", root, "depsource")
					return code
				}
			},
			wantExit: 2,
		},
		{
			name: "dogfood-ocm status aggregates a canonical-derived verdict",
			setup: func(t *testing.T) (string, func() int) {
				fixture := newOCMReadFixture(t)
				cemGit(t, fixture.root, "add", fixture.mapPath)
				cemGit(t, fixture.root, "commit", "-qm", "CEM sidecar")
				fixture.target = cemGit(t, fixture.root, "rev-parse", "HEAD")
				writeOCM(t, fixture, ".corvint/change.ocm.001.json", false)
				cemWrite(t, fixture.root, ".corvint/change.ocm-intents", "docs/intent.md\n")
				return fixture.root, func() int {
					code, _, _ := runCLI(t, "--root", fixture.root, "dogfood-ocm", "status",
						"--expected-base", fixture.base, "--target", fixture.target)
					return code
				}
			},
			wantExit: 0,
		},
		{
			name: "adapter codex normalizes a session-start event",
			setup: func(t *testing.T) (string, func() int) {
				root := cliRepository(t)
				body := []byte(`{"hook_event_name":"SessionStart","session_id":"s","cwd":"` + root + `","source":"startup"}`)
				return root, func() int {
					stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
					return runContext(context.Background(), []string{"adapter", "codex"}, bytes.NewReader(body), stdout, stderr)
				}
			},
			wantExit: 0,
		},
		{
			name: "adapter claude-code normalizes a stop event",
			setup: func(t *testing.T) (string, func() int) {
				root := cliRepository(t)
				ctx := adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": root})
				body := []byte(`{"session_id":"s","stop_hook_active":false}`)
				return root, func() int {
					stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
					return runContext(ctx, []string{"adapter", "claude-code", "stop"}, bytes.NewReader(body), stdout, stderr)
				}
			},
			wantExit: 0,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root, invoke := testCase.setup(t)
			before := repositoryBytesDigest(t, root, selfObservationsRelativePath)
			if exit := invoke(); exit != testCase.wantExit {
				t.Fatalf("exit = %d, want %d", exit, testCase.wantExit)
			}
			if after := repositoryBytesDigest(t, root, selfObservationsRelativePath); after != before {
				t.Fatal("verb wrote to the repository")
			}
		})
	}
}
