package main

import (
	"bytes"
	"strings"
	"testing"
)

// selfObservationsRelativePath is the one write AGENTS.md invariant 4 /
// SOL-V0-007 permits a non-query command: the bounded local self-observation
// ledger. Every no-mutation assertion below excludes only this path from the
// repository digest.
const selfObservationsRelativePath = ".corvint/self-observations.jsonl"

// TestCLIReadVerbsLeaveTheRepositoryByteIdentical restates the 2026-09-12
// black-box read-verb audit (docs/agent-memory/tests.md) as a CLI-level
// assertion: witness, lrf, frontier, test-validity and migration-ratchet
// write nothing to the repository they read,
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
			name: "lrf refuses the cem/1.0 fixture without writing (CEM-V1-210)",
			setup: func(t *testing.T) (string, func() int) {
				fixture := newLRFFixture(t)
				return fixture.root, func() int {
					code, _, _ := runCLI(t, "--root", fixture.root, "lrf", "--cem", fixture.mapPath,
						"--expected-base", fixture.base, "--target", fixture.target)
					return code
				}
			},
			wantExit: 2,
		},
		{
			name: "frontier reports the stable view through the registered adapter",
			setup: func(t *testing.T) (string, func() int) {
				root := frontierRepository(t)
				ctx := registerFrontierAdapters(t, stubStableViewer{})
				return root, func() int {
					stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
					return runContext(ctx, frontierArguments(root, "--artifacts", ".", "--json"), strings.NewReader(""), stdout, stderr)
				}
			},
			wantExit: 1,
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
