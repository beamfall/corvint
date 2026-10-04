## 2026-10-04 V1-0720: where each agent-memory workaround belongs

Human-owned intent: on 2026-10-04 the owner asked for V1-0640's follow-ups to be done, with
V1-0720 after V1-0721. V1-0720 lists seven Claude Code memory files on this machine that exist only
to work around Corvint, Corvint Tasks or shell friction. Each new session rereads them and still
trips on the first attempt. Each item should land at the earliest layer that removes it: product
behavior or refusal text, a repository helper or check, or a documented host limit.

### Classification

Each item was checked against current behavior before it was classified, so a memory that
outlived its fix is not filed again. Tasks behavior was probed on `corvint-tasks`
0.0.0-tcp01-unverified+build.202, using a scratch store seeded from this repository's
`queue.json` and `policy.json`.

| # | Memory | Layer | Disposition | Why not a lower layer |
|---|---|---|---|---|
| 1 | corvint-tasks-payload-canonical | Tasks product behavior | V1-0750 (bug) | Refusals already name whitespace, key order and the unsorted array path. But input is refused, not canonicalized. Pretty-printed JSON reports `byte 1: expected object key`, which names neither cause. A repository helper would only wrap a product's input rule for one repository. |
| 2 | corvint-tasks-ticket-create-payload | Tasks product behavior | V1-0751 (idea) | Already resolved: `ticket create --help` lists the 20 keys and the canonical rules, and refusals name enum values and missing keys. Still missing: a fill-in template with the nested `effects` and `source` shapes. Help text cannot carry a queue-specific `sourceQueueId`. |
| 3 | corvint-tasks-intent-branch, corvint-taskman-store-routing | Tasks product rule | existing V1-0325, related V1-0309 | V1-0325 already asks for an intent-worktree rule that lets a feature branch file a ticket without switching the primary checkout. A carry helper would automate the checkout dance that ticket removes. |
| 4 | line-citations-check-reads-head | repository check | fixed here, DCG-V0-021 | The memory is wrong on one point: the gate reads the Git index, not HEAD (DCG-V0-013), so `git add` is enough. The real friction is a silent pass. An author who edits a cited file and runs the gate before staging gets a pass that never examined the edit. The gate now names each file it read that has an unstaged edit. Neither the result nor the index-only resolution changes. |
| 5 | dogfood-bind-amend-breaks-trace | dogfood refusal text | already fixed, no ticket | V1-0144 added `corvint migrate-traces` for a trace stranded by an amend, and the dogfood refusal names it. The rebind-on-a-bad-plan case now says to delete `.corvint/change.cem.json` and rerun. Both are in `internal/dogfoodflow/change.go` on `origin/main`. The memory predates both. |
| 6 | interactive-rm-cp-aliases | host limit | none in this repository | The prompting `rm` and `cp` are zsh aliases in the owner's interactive profile. Repository scripts run under `sh` and are unaffected. Only an agent's own ad hoc shell commands hit them, so the fix is in the owner's profile. |
| 7 | codex-cli-headless-lanes | host limit, partly resolved | none new; related V1-0669 | The model flag works around an unsupported model in the owner's Codex configuration. The draft-PR half was resolved in AGENTS.md by beamfall/corvint#520. |

### Item 4: what DCG-V0-021 does

After resolving every citation, `script/check-line-citations.sh` runs `git diff --name-only -z`
with `GIT_OPTIONAL_LOCKS=0`, so the index file on disk is not refreshed. It intersects the result
with the files it read from the index: scanned documents, cited files, and its own allowlist,
waiver and requirements inputs. It prints one stderr note naming each match. The note runs after the index reader has closed. It never
resolves a citation against the worktree and never names a file the gate did not read. It leaves
the exit status alone, including when `git diff` itself fails: that failure becomes a note too. Rejected alternatives:

- **Resolving citations against the worktree.** This reopens the defect DCG-V0-013 closed: a
  green local run that CI, reading committed content, fails.
- **Failing on unstaged edits.** This would block running the gate mid-edit, which is the normal
  loop, and one unstaged file anywhere would fail an otherwise clean check.

### Evidence

- `script/check-line-citations_test.sh` case 21:
  - An unstaged edit to a cited file passes and is named.
  - An unstaged edit to a scanned document is named beside a real failure, which still exits 1.
  - An unstaged file the gate never read is not named.
  - Staging the edit clears the note.
- Run against the previous script, case 21 fails: it reports that the edit was not named.
- One independent review approved. Its findings were fixed in the same change:
  - A failed `git diff` could change the exit status; it is now a note.
  - A path listed twice for an unmerged file was counted twice; paths are now deduplicated.
  - The scanned-document and failing-run cases were missing from case 21; both are added.
  - The memory-retirement advice was overstated; it is corrected below.
  - Paths hidden by `assume-unchanged` or `skip-worktree` are now named as out of scope.
- `line-citations-check`, `line-citations-test`, `spec-requirements-check`,
  `requirement-definitions-check` and `traceability-tests-check` pass.
- Tickets V1-0750 and V1-0751 were filed from the primary checkout. Their read-back and
  `receipt audit` (CONSISTENT, AGREES) were taken at filing.

### Memory retirement

Once this lands, the owner can make these memory changes:

| Memory | Change |
|---|---|
| line-citations-check-reads-head | Drop the HEAD claim and the commit-first advice: the gate reads the index and now names unstaged edits. Keep its advice not to hide the gate's exit code behind `tail`, and to append new declarations after the last cited line to avoid repins. The note covers neither. |
| dogfood-bind-amend-breaks-trace | Delete or shorten. Both refusals name their recovery. |
| interactive-rm-cp-aliases | Keep until the aliases leave the profile. |
| codex-cli-headless-lanes | Keep until the Codex configuration changes. |
| corvint-tasks-payload-canonical | Name V1-0750 as the ticket that retires it. |
| corvint-tasks-ticket-create-payload | Name V1-0751 as the ticket that retires it, and drop the enum notes, which refusals now carry. |
| corvint-tasks-intent-branch, corvint-taskman-store-routing | Name V1-0325 as the ticket that retires them. |

### Rollback

To roll back the gate change, revert the note block in `script/check-line-citations.sh`, test
case 21, and DCG-V0-021 in the spec. Then regenerate `docs/specs/REQUIREMENTS.tsv`. No other
check reads the note.
