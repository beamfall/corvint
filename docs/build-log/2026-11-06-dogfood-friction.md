<!-- SPDX-License-Identifier: Apache-2.0 -->
# Dogfood and seal friction before 1.0

Lane `claude/dogfood-friction`, based on `origin/main` `b75053a5`, removes four repeated
round trips from the dogfood bind and seal path. Requirements: `DWC-V0-7600` to `DWC-V0-7603` in
`docs/specs/daily-change-evidence-workflow-companion-v0.md` and `AZU-V0-9950` in
`docs/specs/affected-zero-unknown-v0.md`, all experimental under the standing lane auto-accept.

## Decisions

- V1-4788 (`DWC-V0-7600`): `dogfood finish-lane` and `dogfood inputs-check` rewrite an untracked
  `--intents` manifest whose only fault is order or duplicates to its `LC_ALL=C sort -u` form,
  atomically with the mode kept, and go on. The rewrite runs before the input snapshot, so the
  sorted bytes are what the run binds. A tracked manifest is never rewritten, because that would
  dirty the change; it and any manifest with a non-canonical line keep the `intent-manifest-unsorted`
  refusal (`ASD-V0-840`). Static mode (`--static yes`) canonicalizes the same way, since it is the
  shell pre-check of the same file.
- V1-4721 and V1-0527 (`DWC-V0-7601`, `DWC-V0-7602`): the conflicted merge-forward rebind is a
  supported path. `script/dogfood-merge-base.sh` derives `BASE` as the second parent of the newest
  merge on HEAD's first-parent chain and checks that it is on `origin/main`; `--paths` lists the
  conflict-resolved paths from `git show --remerge-diff`. `make <target> BASE=merge-forward` uses it.
  `docs/DOGFOOD.md` "Rebinding after a conflicted merge-forward" documents the recipe.
- V1-4919 (`AZU-V0-9950`, filed by this lane): an out-of-order, duplicate or undeclarable entry in
  `.corvint/non-source-paths.json` failed the ownership audit with an opaque `path "X"` error. Each
  fault now names the entry index, the rule it breaks and the fix, including a `jq` reorder command.
  The grammar is unchanged.
- `DWC-V0-7603`: the `line-citations-invalid` refusal prints the exact rerun command with every
  input the run took, after `script/check-line-citations.sh --repin` and a commit. The finish does
  not repin on its own: a repin rewrites tracked documents, whose hunks the bound change must
  contain and cite.

## Review fixes

One independent review (Codex) returned four findings; each is fixed in a new commit with a
regression that fails on the reviewed head.

- `DWC-V0-7600`: tracked-ness came from any `git ls-files` failure, so a corrupt index read as
  untracked and a tracked manifest could be rewritten. `intentsUntracked` now treats a manifest
  outside the worktree as untracked, asks Git for one inside it with a `:(literal)` pathspec, and
  accepts only exit 0 (tracked) or 1 (untracked). Any other outcome keeps the file and the refusal
  and prints an `intents not canonicalized` line naming the error. The worktree test resolves
  symlinks and compares directories by identity, so a `/var` to `/private/var` alias or a
  case-folded path still matches.
- `DWC-V0-7601`: POSIX sh has no pipefail, so a failed `git show --remerge-diff` piped into `sort`
  exited 0 with no paths. The helper captures Git's status first and refuses `remerge-diff-failed`.
  `script/dogfood-merge-base_test.sh` runs make, so it is declared in `.corvint/shell-read-scopes.json`
  with its one read, `Makefile`.
- `DWC-V0-7603`: Make expands `$` in a command-line variable, so the printed rerun now doubles each
  `$` before the `%q` quoting. The test reruns the printed command in a child `bash -c`, not `eval`:
  bash 3.2 runs the ERR trap inside an `eval` even in an `||` list, while the output redirection is
  still open, and the trap's `cat` of the output then appended to itself until it was killed at
  166 GB.

## Second review fixes

The PR review (Greptile) returned two findings. Both were reproduced and are fixed in new commits,
each with a regression that fails on the reviewed head `676df6bf`.

- `DWC-V0-7600`: `git ls-files` matches a pathspec case-exactly, so on a case-folding filesystem a
  tracked `intents.txt` passed as `INTENTS.txt` read as untracked and was rewritten, leaving
  `M intents.txt`. Tracked now follows file identity. `trackedAlias` lists the tracked paths that
  equal the entry ignoring case at the same depth (`:(literal,icase)`, `-z`) and compares each with
  the manifest using `os.SameFile`. Git folds only ASCII case, so a non-ASCII path with no tracked
  match stays unknown and is kept. The regression has a case-folding branch and a case-sensitive
  branch, each skipped on the other filesystem kind. The case-folding branch was run with `TMPDIR`
  on the WSL host's Windows drive, which folds case. The case-sensitive branch hard-links
  `INTENTS.txt` to the tracked file and checks that a distinct alias is still rewritten.
- `DWC-V0-7601`: a refused `BASE=merge-forward` lookup left `DOGFOOD_BASE` empty, which reads as an
  omitted BASE: `dogfood-check` fell back to `--bound-base`, and `dogfood-closeout-precheck` ran
  batch-precheck with no base. The refusal is now a parse-time make error, so no target runs. The
  helper prints only a SHA, so the empty output is the signal, because make 3.81 has no
  `.SHELLSTATUS`. While testing it, `dogfood-closeout` turned out to pass the literal word
  `merge-forward` to spec-capacity-check, which refused it as no commit. It now passes the
  derived SHA. The shell test stubs the scripts the targets run, and it declares the two stubbed
  names as mentions in `.corvint/shell-foreign-callers.json`.

## Checks

- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run
  'TestDWCV07600|TestDCWV0882InputsCheckRefusesUnsortedIntentsFirst|TestDCWV0731StaticPreCheck|IntentManifest|Intents'
  ./internal/dogfoodflow/`: ok.
- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run
  'TestNonSourcePaths|TestOwnershipAuditListsUnclaimedPaths_AZU_V0_002' ./internal/liveverify/affected/`: ok.
- `script/dogfood-merge-base_test.sh`: PASS. `script/dogfood-cache_test.sh`: rc 0.
  `script/check-bash32.sh`: ok.
- Review fixes: the `TestDWCV07600` run above, `script/dogfood-merge-base_test.sh`,
  `script/dogfood-cache_test.sh` and `TestRepositoryShellReadScopes` pass, and
  `go run ./tools/unbounded-readers` is ok. Against the reviewed sources, the new
  `TestDWCV07600` case fails, `dogfood-merge-base_test.sh` fails at the injected `git show` case,
  and `dogfood-cache_test.sh` fails at the `$$` rerun check.
- Second review fixes: the `TestDWCV07600` run above (with `TestDCWV0882`, `TestDCWV0731`,
  `IntentManifest` and `Intents`), `script/dogfood-merge-base_test.sh`, `script/check-bash32.sh`,
  `TestRepositoryShellReadScopes` and `TestRepositoryShellForeignCallersComplete_TSP_V0_005` pass.
  Against `676df6bf`, `TestDWCV07600FinishLaneKeepsATrackedManifestUnderAnotherCase` fails on both
  filesystems, and `dogfood-merge-base_test.sh` fails at the refused-lookup case. With only the
  parse-time refusal restored, it fails at the closeout spec-check case.

## Limits

- The end-to-end `make dogfood-batch-check` pass after a conflicted merge-forward rebind was observed
  by hand by the orchestrator on 2026-11-06. This lane does not reproduce it; the helper test covers
  BASE derivation, conflicted-path listing, make wiring and refusals only.
- Not run: `make gate`, a full `go test ./...`, and a seal of this lane.
