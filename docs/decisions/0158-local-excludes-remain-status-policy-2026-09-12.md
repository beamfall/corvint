# Decision 0158 — Local excludes remain Git-status policy

Date: 2026-09-12. Status: accepted, delegated call. Authority: repository owner delegated
the A/B/C policy choice to the coordinator and this leaf. Base:
`004ce3eacda4b2aea098f9cee2674f51591a4937`.

## Outcome: C — accept the existing behavior

Keep the existing ignore behavior, including each caller's existing suppression of external
excludes. Do not add a receipt member, a second digest, or a new refusal. Decision 0142's
ignored-path allowance remains an operational Git-status policy, not an assertion that ignore
rules are committed authority. `GPK-V0-060`, `GPK-V0-061`, and `ARTIFACT-GO-V0-009` already describe
listed versus ignored paths; their behavior and wire contracts do not change. No requirement ID
or analyzer schema changes in this decision.

This accepts an actual limitation: an untracked path hidden only by a local exclude can bypass
a status-based refusal that committed-only rules would trigger. `CLEAN` and `statusSha256` do
not attest that every filesystem path was considered, that ignore configuration is immutable,
or that a later live-checkout build uses only committed inputs. An ignored path is outside the
observed status set, not proven disjoint by `corvint-untracked-allowance/0`. Invariant 2 forbids
promoting that omission into a verified-absence, test-completeness, or source-isolation claim.

## Consumer audit

The inventory comes from searches for `--exclude-standard`, status/porcelain, `info/exclude`,
`core.excludesFile`, and the callers of `gitstatus.Status`/`StatusIn`. Paths below identify the
observed implementation at the base; consequences for unexecuted gates are code-derived, not
claims that those complete gates were run.

| Surface and evidence | Exclude behavior and consequence versus committed-only rules |
|---|---|
| Shared private status, `internal/gitstatus/status.go:123` | Copies the common directory's `info/exclude`, including for linked worktrees, and checks captured metadata again after status. It does not itself override `core.excludesFile`. Copying and checking mutable metadata is not binding it to the commit or to a receipt. |
| Contextindex status/snapshot, `internal/contextindex/git.go:315` and `internal/contextindex/index.go:449` | `gitRaw` disables external excludes at `internal/contextindex/git.go:194`; local excludes remain. They can remove an untracked path from `DirtyPaths` and its status digest. Tree enumeration and pinned tracked sources still determine evidence: ignore rules cannot hide a tracked modification or introduce ignored source bytes. |
| Range impact, `internal/contextindex/git.go:659` and `internal/contextindex/range_impact.go:74` | Same policy. A locally hidden `.go` file bypasses overlap classification and can leave `range.status` CLEAN. The receipt analyzes the committed range and names checks without executing them; it does not attest the live build. A committed-only status would refuse that `.go` file, or bind a disjoint path as UNTRACKED-ALLOWED. |
| Gokernel/harness and trace reads, `internal/gokernel/repository.go:198`, `internal/tracerecordrepo/read.go:81`, `internal/contextindex/history.go` | Gokernel and the trace adapters explicitly disable external excludes. Local-only ignored paths do not trigger the mixed-worktree block. Historical trace/source binding is separate; this omission does not admit untracked bytes as historical evidence. A committed-only status would activate the block. |
| Archive builder, `conformance/release-artifact-v0/archive_run.go:422` and `conformance/release-artifact-v0/archive_run.go:456` | `closedGit` disables global/system configuration and supplies a private HOME/XDG directory, but does **not** override repository-configured `core.excludesFile`; both that external file and `info/exclude` can bypass the initial/final clean-status checks. A configured external file is not necessarily a global-config setting. The two raw commit exports at `conformance/release-artifact-v0/archive_run.go:103` exclude all untracked content independently of ignore policy. Archive verification additionally binds bytes to the loose binary (ARTIFACT-GO-V0-003). |
| Standalone loose gate, `conformance/release-artifact-v0/gate.go:165` and `conformance/release-artifact-v0/gate.go:275` | Ordinary Git inherits local/global excludes. Both builds use `options.Root`. A hidden source can escape the clean check and reach Go; a committed-only status would catch a local-only hidden source, but still miss the same source hidden by a committed ignore. Repeatability and `vcs.modified=false` alone do not prove source isolation. This distinct live-build provenance test gap is filed in `docs/agent-memory/tests.md`; no full loose-gate success or archive bypass was measured here. |
| Work source capture and work materialization, `internal/worksource/source.go:234`, `internal/worksource/git.go:191`, `cmd/corvint/work_materialization.go:235` | Source admission uses shared status with external excludes disabled; a local hidden path can bypass its clean precondition. Materialization rechecks the separately prepared private tree and metadata. Its source set comes from committed entries, not a recursive copy of every ignored live file. |
| Affected selection and parent verification, `internal/liveverify/affected/dirty.go:52` and `internal/liveverify/parentverify/repository.go:91` | Shared status with external excludes disabled. Local excludes can suppress a dirty-path selection or status guard that committed-only rules would trigger. AFP-V0-002 explicitly excludes ignored paths; the affected plan is non-authoritative and not proof of complete test coverage. Live execution must not infer filesystem isolation from an empty dirty set. |
| Dashboard repository authority, `internal/dashboard/repository/process.go:111`; console, `internal/console/code.go:51`; roadmap status, `internal/dashboard/roadmap/atm.go:131` | Repository authority disables external excludes; console uses shared status without that override, and roadmap uses ordinary status. These can omit local-only ignored paths from dirty labels; the latter two can also honor repository-configured external excludes. The console's committed blob reads do not become reads of ignored files. |
| Companion release, `internal/companionrelease/clean.go:26`; genesis, `internal/genesis/repository.go:525@a9e7d6a1` | Shared status without an explicit external-exclude override. Local and repository-configured external excludes can bypass the clean preconditions. Scrubbing global configuration alone does not disable a locally configured external ignore file. No whole companion/genesis gate was executed in this audit. |
| CEM target currentness, `internal/cem/gitauth/authority_target.go:32`, `internal/cem/gitauth/object.go:24` | `ls-files --others --exclude-standard` honors local and repository-configured external excludes; global/system config is disabled. A hidden path escapes the untracked-currentness veto. Raw tracked-file checks and immutable object-view binding remain separate. `TestObjectViewKeepsIndexUntrackedAndIgnoreLive` explicitly preserves live ignore behavior. |
| Local completion, `internal/localcompletion/storage.go:430` and `internal/localcompletion/storage.go:470` | Direct status against the resolved Git directory; global/system config disabled, no external-exclude override. The clean bit can omit local/configured-external hidden paths; the subsequent committed-tree listing is independent. |
| Touch surprise, `internal/touchsurprise/git.go:98` | Requests `--untracked-files=no`; excludes do not change its already tracked-only status universe. |
| Release scan, dependency source, migration adapters | `core.excludesFile` matches in their Git prefixes are not themselves additional status consumers. Committed tree/object reads do not omit a tracked path because an ignore rule matches it. |
| Developer checks and measurement tools | `internal/dogfoodflow/check.go:143`, `tools/native-bridge-pre-review.sh:49`, retrieval-bench status/snapshot checks, release smoke, and the path-scoped CEM interop consumer use ordinary Git status. Local/global ignores can hide untracked drift in those status observations; they do not prove a complete filesystem snapshot. No policy change is applied to those tools. |

Disabling `GIT_CONFIG_GLOBAL` alone also does not disable Git's default XDG `git/ignore` file.
A scratch probe observed that file still hiding a path with global configuration disabled;
`-c core.excludesFile=/dev/null` exposed it. Callers with the explicit empty/null override avoid
both this default and repository-configured external files. This decision does not claim that
all other callers have equivalent HOME/XDG handling or an atomic external-file snapshot.

## Measured alternatives

On this leaf at the base, all five tracked `.gitignore` files byte-matched `HEAD`, and an
unfiltered `git ls-files --others -z` scan found no untracked `.gitignore`. A private metadata
directory held a copied index (timestamp preserved), detached HEAD, minimal config, empty refs,
an object-directory link, and no `info/exclude`. The live worktree was never modified for timing.
Thus the private ignored scan used committed rules **for this verified snapshot**. Removing
`info/exclude` alone would not establish committed-rule provenance in an arbitrary checkout:
modified or untracked per-directory `.gitignore` files would need separate handling.

Twenty interleaved samples after two warmups per command, milliseconds on the loaded host:

| Raw Git command, with optional locks/fsmonitor/untracked cache disabled and `core.excludesFile=/dev/null` | Median | Min–max | Output |
|---|---:|---:|---|
| `status --porcelain=v1 -z --untracked-files=all` | 36.986 | 32.157–44.101 | 0 bytes |
| `ls-files --others --ignored --exclude-standard -z`, live metadata | 33.128 | 28.045–36.968 | 2 paths, 57 bytes |
| Same ignored scan with `--git-dir=PRIVATE --work-tree=LEAF`, no local excludes | 31.398 | 27.247–38.715 | same 2 paths, same 57 bytes |

Both ignored outputs had SHA-256
`1e47869e05bceb4d7493d8bb707532924d3756becf012524bca8dec116a25698`.
These are measured subprocess costs, not full Corvint latency or a performance promotion.
Private metadata setup, committed-rule validation, set comparison, and receipt work are excluded.
A second scan therefore has a measurable cost even on this leaf with no local-only difference.

A scratch repository separately established all of the following (Git and `go list` exit 0):

- With committed `committed.go`, local `local.go`, and repository-configured external `global.go`
  ignore patterns, porcelain was empty. Disabling external excludes exposed `global.go`; also
  removing local excludes exposed both `global.go` and `local.go`.
- `GOTOOLCHAIN=local go list -f '{{join .GoFiles ","}}' .` still returned
  `committed.go,global.go,local.go,main.go`. Changing tracked `main.go` remained visible in status.
- A separate global-config fixture hid a path until global configuration or external excludes
  were disabled. The default XDG ignore fixture behaved as described above.

Raw measurements and probe results are retained under this worktree's private Git directory,
`cxgitexclude/{measurement,semantics,global-semantics}.json`. Scratch repositories and metadata
were removed. No scratch check was promoted into a permanent test.

## Why C instead of A or B

A boolean saying an exclude file exists does not establish that any path was omitted; a rule
digest says which mutable configuration was seen, not that omitted paths are disjoint. Extending
the many existing receipts with that observation would still not establish the stronger claim.

B can be specified correctly, but merely digesting the committed-rules-only **ignored** set is
insufficient: a file hidden only by `info/exclude` is absent from that set. Detection needs the
untracked universe minus the committed ignored set, or the standard-versus-committed ignored-set
difference combined with normal status, followed by refusal or explicit allowance binding.
It also needs pinned hierarchical `.gitignore` rules, bounds and drift handling. That changes
established status/trace/CEM compatibility and can refuse operator-local files deliberately kept
outside evidence. Even then, a committed ignore can hide the same live Go build input.

C preserves the existing scoped observations without treating local configuration as committed
authority. Archive source reconstruction and immutable evidence binding are the mechanisms that
answer provenance questions. Live-checkout provenance needs its own verification, not a broader
interpretation of the status digest. The separate test backlog item retains that unanswered gate
question; this decision does not certify it away or authorize any release promotion.

Revisit this choice if a consumer is required to prove a committed-only untracked universe or
complete live-filesystem isolation. That requires a numbered owning-spec change and explicit
failure behavior. Rollback of this decision is a new policy decision; there is no code to revert.
