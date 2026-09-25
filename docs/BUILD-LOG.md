# Build log

Append-only record of material design decisions, independent findings, failed evaluations, and
promotion evidence. Add new entries at the end so no cited line moves; each entry carries a date
heading and its requirement or decision IDs, so `rg -n '^## ' docs/BUILD-LOG.md` is the index.

## 2026-09-25 V1-0264 DCW-V0-025 (accepted, decision 0388): no-module and module-root impact refusals are typed abstentions

`corvint dogfood change` kept only `unsupported-impact-range` as a non-blocking `prechange-impact`
abstention, so a repository with no Go module (`unsupported-impact-repository`) or a change to a Go
file at the module root (`unsupported-impact-path`) could never reach `"complete": true`. Observed
envelopes from a current-tree build (0.8.1): both exit 2 with empty stdout and one stderr line,
`{"code": "unsupported-impact-repository", "error": "native Go impact requires a slash-qualified Go
module", "ok": false}` and `{"code": "unsupported-impact-path", "error": "native Go range impact
requires changed Go paths in a non-root package", "ok": false}`. Chosen: typed abstention through the
existing machinery, each code kept as the row reason and in the abstention artifact, because impact
is a Go-native profile and its absence is a visible scope limit, not a failed step. Any other code
or shape still blocks. Evidence: `TestDogfoodDailyPathCompletesWhenImpactRefusesTheRepositoryOrModuleRoot`
(real refusals: change, check and seal pass) and new `script/dogfood-change_test.sh` cases. The
requirement is accepted with both amendments (decision 0388). Review repairs: accepted `GOC-V0-009` and
`ERI-V0-006` admit only `unsupported-impact-range`, so each now carries a proposed amendment, not
accepted, widening the set to the three codes; the code change must not merge until the owner
accepts `DCW-V0-025` and both amendments. `dogfood change` and `dogfood check` now print one
`NOTE prechange-impact NOT_PRODUCED CODE` stderr line for each of the three codes, and the shell
failure loop asserts each invalid shape's exact reason. Follow-up, left unchanged: the one-stderr-line
test (`internal/dogfoodflow/change.go:223`, `check.go:230`) counts newlines, so an envelope line
followed by unterminated trailing bytes still qualifies (`textLines` splits them off, `flow.go:255`).

## 2026-09-25 V1-0261 DCW-V0-020: adopter fix lines name the `corvint dogfood` subverb

`corvint dogfood change|check` printed `fix:` and `required order:` lines telling the operator to
rerun `make dogfood-change BASE=...`, a target only Corvint's own tree has; an adopter such as
Beamfall runs `corvint dogfood change $BASE`. Nothing the subverbs receive says whether a Makefile
wrapper started them, so every such line now names the subverb form (`corvint dogfood change
<base>`), which also works in Corvint's tree. Evidence:
`TestDogfoodDailyPathRunsFromBinaryInForeignRepository` and
`TestDogfoodChangeNamesDeleteWhenACorrectedPlanJoinsEarlierCitations` assert the adopter wording;
`script/dogfood-change_test.sh` assertions were updated. Not changed: the console's empty-report hint
(`internal/console/views.go`) still names the make target.

## 2026-09-25 V1-0259 OIF-V0-001..011 / decision 0386 (proposed): declared OCM intent forms

`ocm prepare --intent-form adr-decisions|roadmap-acceptance` (experimental) reads ADR
`## Decisions` items and roadmap Acceptance tickets as OCM obligations; the form is recorded in
`intentScope.form` and the default form stays byte-identical. A live sweep of Beamfall `2a8e06b28`
found the grammar narrow: 89 of 215 ADRs and 11 of 44 roadmap shards prepare, and 935 of 1200
tickets in passing shards lack Acceptance and are excluded. Shard 42 showed a block-style
`  - **Acceptance:**` list with nested bullets, which the reader now accepts. LRF and the change
universe refuse declared forms. The dogfood wrapper cannot pass a form yet (open decision 5).

## 2026-09-25 V1-0259 DCW-V0-024: daily path for a repository with no requirements spec

`dogfood change` required 1 to 16 intent specs with one `## Requirements` heading, so a repository
whose intent lives in ADRs and roadmap tickets (Beamfall) could never reach `"complete": true`,
against AGENTS.md invariant 6. Route (c), owner-approved: an explicit declaration. The opt-out is a
`DOGFOOD_INTENTS_FILE` whose whole content is `#no-intent-declared`. Rejected: an unset variable or
an empty file, because both arise by accident (a forgotten export, a generator that matched nothing)
and must keep refusing `missing-intent-scope`; a sentinel variable value, because the Corvint wrapper
absolutizes relative values and any bare word is also a legal path. A `#` line was already refused
as an intent path by both the manifest check and `dogfood-ocm status`, so no valid manifest changes
meaning. Under the declaration the three OCM rows report `no-intent-declared` without blocking, the
report's `ocmStatus` is `NOT_ASSESSED`, and the check runs no OCM verifier, refuses a snapshot that
disagrees with the report, and prints a `NOTE intent-linkage NOT_ASSESSED` line. Reading ADR or
roadmap intent forms in OCM is separate work. Pre-change `corvint query` (0.8.1) missed the owning
spec: its top results were the Go kernel migration spec and decision 0012.

Live run, binary built from this change, in a scratch Go module with no spec and one changed hunk
(`greet/greet.go`) cited to `README.md`: after the sidecar commit, `dogfood change` exited 0 with
`complete: true`, rows `ocm-prepare`, `ocm-status` and `ocm-aggregate` `NOT_PRODUCED
no-intent-declared`, every other row `PRODUCED`, `ocmStatus` `NOT_ASSESSED`, `bootstrapUnknown` 0;
`dogfood check` printed `NOTE intent-linkage NOT_ASSESSED no-intent-declared` then `PASS`, and
`dogfood seal` printed `PASS`. A change at the module root first refused `prechange-impact:
unsupported-impact-path`, an impact-index limit independent of this change.

## 2026-09-24 0.8.1 version tuple and DCW code vocabulary (decision 0381 item 11)

The version tuple moves to 0.8.1 for the pre-release decision 0381 item 11 approved. Release prep
found `make error-code-ownership-check` failing on main: #160 added 26 `dogfood change/check/seal`
refusal and step codes that no spec named. The new "Code vocabulary" section of
`daily-change-evidence-workflow-v0` owns them (`DCW-V0-020`). CI runs neither that check nor the
other doc checks, so the gap reached main unobserved; a follow-up ticket covers it.

## 2026-09-24 Decision 0381 items 1 and 11: HLQ-V1 intent accepted, 0.8.1 approved

The owner approved the two open items of decision 0381 on 2026-09-24. The intent of
`host-lifecycle-qualification-v1` is accepted; delivery stays experimental. `HLQ-V1-008` requires
each tuple's raw runner report and its sha256 under `conformance/host-lifecycle-v1/results/`, so the
0.8.0 Results rows (transcribed, no retained report) go stale at the next release. 0.8.1 publication
is approved as a pre-release under v0-8.
## 2026-09-24 V1-0175 / V1-0174 / V1-0180 DCW-V0-015, DCW-V0-019: truthful daily pass, cite correction, sealed review

V1-0175: `dogfood-change` classified the local outcome before `cem-prepare`, so the recorder saw
the tree clean and a first pass with every input reported `"complete": true` while the prepared
`.corvint/change.cem.json` was untracked. `script/dogfood-change.sh` had the same order, and its test
recorder ignored untracked files, so the check never existed. The recorder now runs last; an
untracked or modified sidecar reports `local-outcome: record-index-failed` (DCW-V0-015). The
built-binary test `TestDogfoodDailyPathRunsFromBinaryInForeignRepository` fails on the old order.

V1-0174: reachable through `dogfood change`, because `cem prepare` resumes a matching map with its
citations and `cem cite` only adds evidence. Replace semantics would need a new CEM CLI removal
primitive, so the documented route was taken: DOGFOOD.md step 4 says to delete the map before
re-citing, and a pass that cites onto an already cited map prints that line (DCW-V0-019). The
built binary showed the union (two evidence records after a corrected plan) and the fix (one after
the delete).

V1-0180: DOGFOOD.md section 6 now gives the sealed form. In a clone at seal 3ad6287 with bind
28a8dfa375f1f47c3ce7086998db7fcc12b30d0f, that form's `cem report` exited 0 (91/91 supported). The
old `.corvint/change.cem.json` form exited 2, and the sealed map with `--target HEAD` exited 1.

## 2026-09-24 Decision 0381: path choices from 0.9 to 1.0 (V1-0235)

Two independent read-only reviews, an evidence-ledger audit and a product-strategy review, decided
the open 1.0 questions. Where they disagreed, the one with direct evidence won: the store refuses to
close V1-0016 (unsatisfied dependency, missing approval, `full-gate` NOT_RUN), and V1-0189 was
already delivered by #117 and #146. The owner applied the store changes on 2026-09-24 (25 mutations,
`receipt audit` CONSISTENT, head seq 469):

- Six features deferred past 1.0 at P2 and dropped from v0-8: V1-0023, V1-0154, V1-0191, V1-0195,
  V1-0201, V1-0205.
- V1-0174, V1-0175, V1-0180 and V1-0229 raised to P1.
- V1-0173, V1-0188 (trimmed to criterion 1) and V1-0189 completed manually.
- V1-0005 and V1-0016 moved to v0-9.
- Release chain: v0-7 now follows v0-5, and v0-9 follows v0-6 and v0-8.

Finding: readiness stops at the missing candidate; `release candidate` binds ticket digests without
checking status, and promotion needs each bound ticket COMPLETED and unchanged: close scope first.
HLQ-V1 acceptance (item 1) and 0.8.1 publication (item 11)
remain proposed pending the owner.
## 2026-09-24 V1-0229 PRS-V1-005: Core-only candidate assembler

Finding: the reader and installer have admitted `corvint-core-release-candidate/0` since V1-0125,
but nothing could produce it. `corvint-release-candidate` refused an empty `-companion-dir`, and
`releasecandidate.Assemble` loaded companion evidence unconditionally. The ticket cited
`corvint-companion-release`'s `-tasks-root` check. That check is correct for the companion bundle
and is unchanged.

Decision: omitting `-companion-dir` assembles `corvint-v<version>-core`. The core gate is verified
exactly as before. The source archive comes from the clean `-source-root` HEAD through the
companion bundle's own export and deterministic tar.gz (`companionrelease.CoreSourceArchive`).
Assembly refuses unless that commit and tree equal the core gate report's. `QUALIFICATION.json`
marks core-archive PASS and every other row `NOT_RUN` with `companion not present`. The staged
candidate must pass `VerifyContext`, including the host `--version` probe, before a no-replace
promotion. The companion path, its inputs and its checks are unchanged. No new root verb.

Evidence: `TestPRSV1005CoreOnlyAssemblyNeedsNoCompanion` covers a mismatched core revision refused
with nothing retained, then a valid Core-only assembly that verifies with no companion or Tasks
role. Real run on darwin/arm64 against a scratch commit `e921b0c` that set the alpha version
0.8.1a1: the archive gate passed (5 targets), `corvint-release-candidate` with no companion or
Tasks input exited 0 with profile `corvint-core-release-candidate/0`, build 76, and
`corvint-release-install` installed it and printed `Corvint 0.8.1a1 (build 76)`.

Limits: the version token is still alpha-only, so `1.0.0` is refused. The reader binds the Core-only
source archive by digest only; the commit/tree binding is an assembler check. Linux assembly and
the full gate are `NOT_RUN`.
## 2026-09-24 V1-0236 DCW-V0-020..023 / LCP-V0-014..015: daily change/check/seal run from the installed binary

Owner decision (2026-09-24): the daily path runs as subverbs of the existing Core verb,
`corvint dogfood change|check|seal BASE`, with no new root verb. The logic that lived in
`script/dogfood-change.sh` and `script/dogfood-check.sh` is now in `internal/dogfoodflow`. The Go
subverbs do not build, resolve or version-check a binary: the running executable is the default for
every role, and `--corvint-bin` or the verifier flags override it. The two scripts are now thin
wrappers. They keep the Corvint-only steps (reading VERSION, matching the version, building the
change binary and the verifiers), so the make targets and the `DOGFOOD_*` inputs are unchanged.
`script/dogfood-seal.sh` remains a script. `dogfood finish` runs the change and the final check
in-process through the same package. It no longer runs repository scripts, reads VERSION, builds,
resolves `corvint` on PATH (Git is still found on PATH), or reads `CORVINT_BIN` or `DOGFOOD_*`
(`LCP-V0-014`). The check-guard now also recognizes the
`dogfood check` and `dogfood seal` argv (`LCP-V0-015`).

Behaviour changes, all recorded in the two specs:
- The wrappers build before any Go refusal. A fresh-clone check therefore writes the verifier
  builds before it reports `dogfood-report-missing`, and a check refusal now costs two builds.
- The `rg` prerequisite is gone.
- New refusals: `not-repository-root`, which comes before any write (reproduced for both change and
  check from a subdirectory), `verifier-unavailable`, `dogfood-base-required` and
  `dogfood-executable-unavailable`.
- finish loses the script's process-group cleanup of descendants. It now has a 10-minute context
  deadline instead, which kills a running Git or step child and is checked between steps; an
  in-process step is not pre-empted.
- finish runs both verifier roles on the running binary, so its base and tree digests are equal
  and show self-consistency only. The independent base build stays with `make dogfood-check`.

Independent review (Opus) repairs before binding:
- The change wrapper runs the whole flow in a set `CORVINT_BIN`. With VERSION 0.8.0, installed
  `Corvint 0.8.0 (build 65)` matches the version but has no `dogfood change` and refuses
  `invalid-local-completion-action`. Kept, since running the path from the selected binary is the
  point of V1-0236 and the script tests pin it; the skew window closes when VERSION moves to 0.9.0.
  Now stated in `DCW-V0-022`.
- The wrapper's move to the repository root re-based relative `DOGFOOD_*` file paths; it now makes
  them absolute first.
- Git subprocesses now run under the flow context, so the deadline and a signal kill them.
- The check report is staged in the Git directory and renamed, not truncated in place.
- The recorded impact argv's root is compared after resolving symlinks, as the scripts'
  `cd && pwd` did (`/tmp` against `/private/tmp`).
- `LCP-V0-015` gained a test; the `DCW-V0-020` trace row now states which parity is tested.
- Not repaired, inferred only: a SIGINT that kills a step child before the notifier cancels the
  context can record that step as `exit-130` and continue to the next checkpoint.

Finding: a foreign repository must ignore `.context-corvint/` as well as the `.corvint` private
outputs. Otherwise the recorder refuses `repository-identity-changed`. This was pre-existing:
installed `Corvint 0.8.0 (build 65)` behaves the same. `docs/DOGFOOD.md` now lists the entries.

Evidence:
- In-tree only: `script/dogfood-change_test.sh` and `script/dogfood-bind-range_test.sh`.
- Foreign-repository portability: `TestDogfoodDailyPathRunsFromBinaryInForeignRepository` and
  `TestDogfoodFinishRunsFromBinaryInForeignRepository`. Each builds the binary, runs it in a
  fixture repository with no `script/`, VERSION or Corvint source, puts a failing `corvint` impostor
  first on PATH, and points `CORVINT_BIN` at a missing file.
- NOT_OBSERVED: no real non-Corvint repository.
- `internal/localcompletion/runtime_environment_test.go` tested the environment finish gave the
  scripts; finish no longer runs them, so it is deleted and the `CRB-V0-018` trace row cites
  `TestDogfoodFinishRunsFromBinaryInForeignRepository`, which points `CORVINT_BIN` at a missing file.
- Full gate NOT_RUN (owner policy).

## 2026-09-24 V1-0016 HLQ-V1-001..008 / PRS-V1-006: host lifecycle qualification on 0.8.0

New contract `host-lifecycle-qualification-v1.md` and runner `conformance/host-lifecycle-v1`. The
runner runs nine cases for each Core host tuple, in a private workspace whose `HOME`,
`CLAUDE_CONFIG_DIR` and `CODEX_HOME` are fresh. Plugin install, disable and uninstall go through
each host's own `plugin` commands. The hook cases run the command registered in the installed
`hooks/hooks.json` with host-shaped payloads.

All three decision 0373 Core host tuples passed 9/9 on darwin/arm64. The tuples were plain CLI,
Codex CLI 0.153.2 with adapter 0.2.2, and Claude Code 2.1.267 with adapter 0.2.3. The current
binary is from the published v0.8.0 archive (sha256 `95ae7446…a710e`) and the N-1 binary from the
published v0.7.0 archive (sha256 `5fdbab20…b0bad`). Support stays FALLBACK.

Findings while building the runner:

- `exec.Command` resolves a bare name against the parent `PATH`, not `Cmd.Env`. The first draft
  therefore ran the operator's `corvint` in every case: its upgrade case reported 0.8.0 on both
  sides, and its uninstall case still found a binary. The runner now resolves names against the
  private `PATH` (`HLQ-V1-003`).
- `corvint help` and a Core refusal envelope are written to stderr.
- Claude Code 2.1.267 does not delete an uninstalled plugin version. It marks the cache directory
  with `.orphaned_at` and removes it later. The uninstall predicate accepts that marker and checks
  every other file under the private `HOME` for Corvint residue.
- An isolated Codex home has no hook trust and Codex has no plugin disable verb. Both are recorded
  as known gaps.

Supplementary live runs: Codex injected the envelope at SessionStart and UserPromptSubmit in a real
session. Claude Code reported both hook responses, but its model call failed on an expired OAuth
session. linux tuples `NOT_RUN`. Full gate NOT_RUN (owner policy).

An independent review returned CONCERNS, and each finding was fixed in the runner or narrowed in the
contract before the rerun:

- The upgrade case claimed the current binary reads the N-1 index. A snapshot is keyed by the
  binary that wrote it, so the current binary never reads it. The case now requires
  `index --if-stale` to rebuild rather than report `state=fresh`.
- The tuple identity is now enforced: an unreadable host, adapter or corvint version, a failed
  fixture setup, a dirty `--source` (for a plugin host, also an ignored file in the package
  directory), or an operator `corvint` beside the host or Git exits 2 before any case runs. The
  report names the source revision.
- The worktree is checked after the change case and the plugin frontier case as well as at
  uninstall. The plain CLI frontier case works in a clone. The residue
  scan covers the whole private `HOME` and reads every file.
- Each hook event must register exactly one command. The enabled, disabled and version checks read
  the plugin's own listing row.
- The Codex Stop hook now receives `CODEX_THREAD_ID`, which differs from the payload session id.
- A setup error, SIGINT or SIGTERM removes the workspace. A signal does not wait for a running
  host child.
- `HLQ-V1-006` now names what is checked: enveloped context receipts, `git status --porcelain`, and
  the Git directory not compared.

The rerun used plugin sources from a clean `e667812` checkout. The packages are unchanged since
`df66aba4`. All three tuples passed 9/9 again. A second review's findings were fixed or narrowed
the same way before the final run.

## 2026-09-24 V1-0223 ARTIFACT-RDY-V0-003 / decision 0380 (accepted): release tag names the notes commit

`script/release-checklist` passed its tag row only when the release tag pointed at the gated HEAD,
while `RELEASE-RUNBOOK.md` step 10 tags the release-notes commit, so the 0.8.0 pre-promotion run
failed the row. Decision 0380 makes the notes commit the one tag target. Its only parent
must be HEAD, and it must change `docs/RELEASE-NOTES.md` and only Markdown under `docs/`, which no
`//go:embed` directive reaches. `script/release-checklist_test.sh` covers a tag on HEAD (FAIL), a
valid notes commit (PASS), a notes commit without the notes file, one with a non-docs path and a
tag one commit further on (each FAIL with its own reason). With the new script, the checklist run
from `41f2b68` reports `PASS tag` for `v0.7.0`, and run from `31d68b4` it reports `PASS tag` for
`v0.8.0`; neither tag moved. The owner accepted decision 0380 on 2026-09-24; it was drafted as
0379 until the OpenCode decision took that number. Full gate NOT_RUN (owner policy).

## 2026-09-24 TCP-V0-048, decisions 0375/0377/0378: requirement-definitions and line-citations checks repaired on main

`origin/main` at `e667812` failed `requirement-definitions-check` and `line-citations-check` with no
change in flight.

`TCP-V0-048` was not defined twice. Its one clause is the numbered requirement at
`task-context-packet-v0.md` line 792, added with `TCP-V0-049` and `TCP-V0-050` by `b705bf3`
(V1-0219, decision 0377), and `REQUIREMENTS.tsv` already pointed there. The same commit began an
acceptance-evidence paragraph with `TCP-V0-048..050 (proposed`, and the checker's definition pattern
(an ID at line start followed by `:` or `.`) reads `TCP-V0-048.` as a second clause. The paragraph
now opens `Proposed TCP-V0-048..050`; no ID changed.

Five citations in decisions 0375, 0377 and 0378 did not resolve. Each was reread before repinning:

- 0375's two `docs/BUILD-LOG.md` citations: both cited lines are unchanged in the "V1-0017 decision
  0360" entry and moved down with later prepends, so the anchors stay and the line numbers move.
- 0375's `stable-operations-v0.md:77-85`: `SOP-V0-003` is byte-identical at 84-92 after V1-0190
  inserted the current N-1 evidence paragraph above it, so the anchor `@feb4322f` stays.
- 0377's `taskcontext.go:237-283` is still the whole `compile` function; it gains `@fc3cbfcb`.
- 0378 cited `packages/opencode/src/project/project.ts:217` in `sst/opencode`, a file this
  repository does not track, so no content anchor can resolve. Line 217 at OpenCode tag `v1.18.31`
  (commit `014614d35b39`, the installed version this log records for the OpenCode MCP check) sets
  `worktree` to `/` for a global project with no VCS. The decision now names that tag and commit in
  prose instead of a `path:line` token.

Gates: `make requirement-definitions-check line-citations-check spec-requirements-check` and
`GOTOOLCHAIN=local go test ./internal/specindex/` passed. `make gate` NOT_RUN by owner instruction.

## 2026-09-24 V1-0190 SOP-V0-003 / PRS-V1-002: 0.7.0 to 0.8.0 N-1 upgrade qualification

`script/check-install-lifecycle.sh` at `3cd62ca9`, with `CORVINT_LIFECYCLE_ARCHIVE` set to the
published 0.7.0 archive for the tuple and `CORVINT_LIFECYCLE_UPGRADE_BINARY` set to the `corvint`
from the published 0.8.0 archive for the same tuple. Both releases were downloaded with
`gh release download` and `shasum -a 256 -c SHA256SUMS` passed for all eight archives.

| Tuple | 0.7.0 archive sha256 | 0.8.0 archive sha256 | Run |
|---|---|---|---|
| darwin arm64 | `230b68d8c03806732d84afc041ee4622b6d9c1ba37acd06f112b3ba07bb18282` | `2174898d52f906e61de643b0c10e5d528eaea6aca34ede445b57ccc44b38a8c2` | native host |
| darwin amd64 | `549fed9b7c201f0601c683b777a7b3ce29e06b7afe88bf118b5e89760eec1dcf` | `4d6cab6981525c4d4e90a76ca08712159dbed475cfb5a80db96bac3833d15d6f` | Rosetta 2 on the arm64 host |
| linux arm64 | `a03f04442ec7ffc3af408a73457ab68a7ffb9b33f0d5b8d13475534d3ace3981` | `69d6fe01139519e4ce4ba5ab67cc47ab6cc70f394a4bad5be478908f235d9f22` | `golang:1.27.1` container, git 2.47.3 |
| linux amd64 | `03d4712ef30f6487293f93b4cda52428721ceafd30dcf3e8090a37e4073328f7` | `9325237ea860903b86bc293fbda5269cec38b1913b283caac85397a3f40006af` | `golang:1.27.1` linux/amd64 image under QEMU emulation |

Every run reported `install-a` `Corvint 0.7.0 (build 46)`, `upgrade-b` `Corvint 0.8.0 (build 65)
packet=changed`, `rollback-a ok` (the 0.7.0 packet after the upgrade is byte-identical to the first
packet), `uninstall`, `backup-restore`, `corrupt-truncate` and `corrupt-overwrite` `ok`, and
`SUMMARY status=PASS`. The four step reports are retained outside the repository.

The 0.6.0 to 0.7.0 `upgrade-b` failure under the decision 0341 rule is now recorded in
`stable-operations-v0.md` as a closed historical finding, and this run as the current N-1 evidence;
the 1.0 spec traces `PRS-V1-002` to it. V1-0016 has not yet fixed the supported tuple list, so all
four tuples were run. linux amd64 ran emulated, not on native hardware, so it is not native Core
platform evidence for `PRS-V1-004`. Full gate NOT_RUN (owner policy).

## 2026-09-24 Decision 0376: owner acceptance of DCW-V0-018/019, AFP-V0-021 and RCB-V0 intent

The owner approved a listed set of pending acceptances ("approved to all", 2026-09-24). Decision
0376 records the specification part: `DCW-V0-018` and `DCW-V0-019` (tickets V1-0142, V1-0173),
`AFP-V0-021` (V1-0126), and the receipt bundle V0 intent with both V1-0197 ticket deviations
confirmed (full-gate receipt instead of the gate ledger; directory only, no archive). `DCW-V0-016`
was not in the approved list and stays proposed. Delivery status stays experimental in every
spec; acceptance of intent is not qualification or promotion.

## 2026-09-24 decision 0375 PUB-V0-021: build number is provenance, not order (V1-0149)

Ticket V1-0149 asked the owner to decide whether build numbers should restart from the public
lineage or carry an offset, after decision 0331 restarted `origin/main`'s first-parent commit
count and `origin/main` HEAD stamped build 12 while the already-published `0.7.0` prerelease is
`Corvint 0.7.0 (build 46)` (`docs/BUILD-LOG.md:2040@231c2812`, the "V1-0017 decision 0360 /
SOP-V0-003 / SOP-V0-009" entry above, 2026-09-23). A release-engineering expert review the owner
requested (2026-09-24) verified every citing consumer checks the build number for shape or exact
equality, never order (`conformance/release-artifact-v0/smoke.go:26@1bcee6df`,
`internal/releasecandidate/install.go:109@0a2f615a`,
`internal/releasecandidate/candidate.go:187@ffe1e5e9`,
`extensions/vscode/src/executable.ts:13@29803a42`,
`cmd/corvint/work_executable_binding.go:73@d5b9289a`,
`internal/companionrelease/core_smoke.go:52@d62aa33e`, `script/dogfood-check.sh:107@c47ed46e`),
that `docs/specs/vscode-extension-v0.md:164@2ced530d` already states the build number is not part
of the pin, that `SOP-V0-003` (`docs/specs/stable-operations-v0.md:77-85@feb4322f`) compares
upgrade/cold-index packet bytes rather than build numbers, and that a real N-1 lifecycle upgrade
from `0.6.0 (build 90)` into the installed `0.7.0 (build 46)` passed
(`docs/BUILD-LOG.md:2023@b85d8a61`) despite the published build number going down
(`docs/RELEASE-NOTES.md:23,46@1528a86b`). The review also found a second, independent defect: the
0.7.0 release commit `41f2b68934ce0d7b2ee6f0b22e31dab41ddffa25` was never on `origin/main`'s
first-parent chain (`docs/RELEASE-NOTES.md:32-33@6a77dbd2` records the release source was a local
clone rather than a pushed-and-merged commit), so the commit that actually sits at first-parent
position 46 on `origin/main`, `e9a6e5456cfb0e7dbc72482fc3c43688c4f1372a`, is unrelated and would
stamp the same `Corvint 0.7.0 (build 46)` banner if built today.

Decision (`docs/decisions/0375-build-number-is-provenance-not-order-2026-09-24.md`, amends decision
0314 and `PUB-V0-021`): no build-number offset. `PUB-V0-021` in `docs/specs/public-release-v0.md`
now states the build number is monotonic only along `origin/main`'s first-parent chain since
decision 0331, is neither an ordering nor an identity key (`VERSION` orders releases; commit plus
executable digest identify a build), and a release artifact MUST be built from a commit on that
first-parent chain. `docs/RELEASE-RUNBOOK.md` step 10 gains a pre-tag check
(`git merge-base --is-ancestor` and `git rev-list --first-parent origin/main`) so a future release
cannot repeat the `41f2b68`/`e9a6e54` collision; `script/release-checklist` is unchanged.
`docs/specs/REQUIREMENTS.tsv` is regenerated (only `PUB-V0-022..026` line numbers shift, no ID
added or removed) and `docs/decisions/README.md` gains the 0375 index row.

Gates: `make -s spec-requirements-check requirement-definitions-check traceability-tests-check
line-citations-check error-code-ownership-check` all passed (0 traceability tests planned, as
before this change). This is a docs-only change; no Go source, wire format, or test was touched, so
`go test`/`go vet` were not run for this change beyond confirming the toolchain
(`GOTOOLCHAIN=local go env GOVERSION` = `go1.27.1`). `make gate` was NOT_RUN, per the owner's
standing preference for scoped issue work.

## 2026-09-24 V1-0191 MCPV0-024..026, decision 0374: task-review tools move to an opt-in descendant profile

Finding: the first V1-0191 candidate added `corvint.context` and `corvint.cem.report` to the default
MCP 2026-07-28 V0 tool list and to conformance profile `/0`. Decision 0103 froze that list at three
tools and requires new tools to use a descendant profile. `extensions/vscode/src/mcp.ts` spawns
`corvint-mcp --root ROOT` and rejects any tool list other than the three V0 descriptors, so the
five-tool default would also have broken a shipped client.

Decision 0374 (a wire-contract expert decision made at the owner's request; it does not supersede
0103):
- The default stays at exactly three tools (`MCPV0-008`), and profile `/0` keeps its meaning.
- `--tool-profile task-review` is a closed argv selector (`MCPV0-026`) that follows the
  `MCPV0-021` rules. It additionally advertises the two tools, and it composes with
  `--protocol-version 2025-11-25`.
- An unadvertised tool fails as `unsupported-tool` (`-32602`).
- Profile `/1` (`cases-task-review.json`) carries the selector and the two tools' cases.

Evidence (Go 1.27.1):
- Test runs:
  - `cmd/corvint-mcp`, `internal/mcp/...` and `internal/cem/...` pass.
  - `conformance/mcp-2026-07-28` passes with `CORVINT_MCP_OFFICIAL_SCHEMA` set, including
    `TestServerTrafficMatchesOfficialSchema` (profile `/0`) and
    `TestTaskReviewTrafficMatchesOfficialSchema` (profile `/1`).
- Negative control, reverted: with `Registry.advertises` returning true for every tool, the
  following fail.
  - `TestToolCatalogueAndResourceOmission`
  - `TestTaskReviewDefaultProfileUnchanged`
  - `TestTaskReviewLegacyProtocol`
- Not run:
  - Official Streamable HTTP conformance, fuzzing, full race and cross-build evidence.
  - `make gate` (owner preference).

## 2026-09-23 V1-0198 DIRTY-CACHE-003: linked worktrees each build and store their own index

Finding: linked worktrees of one repository at one commit do not share the immutable index. Each
worktree builds the clean base and stores its own copy. The snapshot directory is
`<root>/.corvint/index` (`internal/contextindex/snapshot.go:30@659a5c23`,
`internal/contextindex/snapshot.go:142-144@fe5dad2c`). `root` is the `--root` worktree, which
needs only a `.git` entry, and a linked worktree's `.git` file passes
(`cmd/corvint/main.go:647@9f517478`). The file name holds object format, tree OID and engine
digest, but no root (`internal/contextindex/snapshot.go:166-167@74ceb0e3`). The writer and reader
both use that path (`internal/contextindex/snapshot.go:216@e047a2f4`,
`internal/contextindex/snapshot.go:612@c5cc88b1`). The Git common dir is never consulted. With no
snapshot, a query builds the index in memory and persists nothing
(`cmd/corvint/harness_context.go:67-71@81daa288`).

Measurement: `script/measure-worktree-index-share.sh` created three detached linked worktrees at
`01b6804` (tree `458fe9c5`) under the scratchpad. It ran the same repository query in each, then
`index --if-stale`, then the query again. The binary was built from that commit (engine
`3d50e7ba37f7e749`). Load average was 80.7 at the start and 87.5 at the end, so wall times are
inflated and only diagnostic. Bytes and build counts are the primary evidence.

| Step (per worktree 1 / 2 / 3) | Result | Seconds |
|---|---|---|
| query, no snapshot (first) | full in-memory build, `READY`/`fresh` | 2.33 / 3.49 / 2.26 |
| query, no snapshot (repeat) | full in-memory build again | 2.33 / 4.66 / 2.91 |
| `index --if-stale` | `BUILT` in every worktree, 72,803,003 bytes each | 7.67 / 6.05 / 8.47 |
| query, snapshot hit | `READY`/`fresh` | 1.05 / 1.28 / 1.67 |

In total: three builds and 218,409,009 bytes on disk, all under the worktrees' `.corvint/index/`.
There were zero `.gob` files in the common Git dir. The three files have the same size but different
Git blob OIDs. Writing the same tree twice in one worktree also gave different OIDs
(`fafb2a52`, then `90c2855f`), so the gob snapshot is not byte-deterministic across writes. The
cause is inferred to be map iteration order and was not confirmed.

Dirty view: in worktree 1 an appended line in `README.md` left `index --if-stale` `fresh`
(0.17 s, no rebuild). The query returned `READY`/`mixed-worktree` with `mixed_paths` `README.md`
(1.77 s). Worktree 2's query stayed `fresh` during the edit. Worktree 1 was `fresh` again after
the restore, and its snapshot OID (`03162da6`) was unchanged. `DIRTY-CACHE-003` holds within
each worktree: the dirty overlay is per worktree and leaves the clean base untouched.

Repository mutation: `git worktree list` showed only the primary checkout before and after, and
`git status --short` was the same before and after (only the then-untracked script). The trap
removed each worktree with `git worktree remove`.

Decision: record the result; change no code. Sharing the clean base across linked worktrees is
filed as BUG V1-0212 (P2, v0-9), with the numbers above and the atomic-rename requirement.

## 2026-09-23 V1-0205, V1-0206 TCP-V0-047: routing idf floor 2.0, unheld terms and fenced blocks

Cause (V1-0186 follow-up): `instructionRoutedRows` routed any governing passage that shared two task
terms however common, so ordinary words (`test`, `spec`, `index`, `command`) routed paths for
unrelated tasks and cost code2test recall@5 and edit2ripple recall@10/@5. A second cause came up
in review. A task term the body term table does not hold, such as a camelCase compound the table
splits, got the maximum idf and counted as rare. Paths inside fenced code blocks were also routed
as if they were prose, and the fence opened and closed on any fence line.

Fix: a shared term counts only when the table holds it and its idf is at least
`contextRoutedMinIDF` = 2.0, which excludes terms held by more than about 13.5% of sources. A
passage still needs two such terms. Fenced blocks are skipped, and `nextFence` closes a fence only
with the same character, at least the opening length and an empty info string, as CommonMark
requires. A fence indented four or more spaces inside a list item is still not recognized; that is
a known limit. Analyzer schema moves to `corvint-analyzer/79` (after #123 took 78). New tests:
`TestTaskContextRoutingClosesFencesAsCommonMark` (other character, shorter fence, info string) and
`TestTaskContextRoutingSkipsCompoundsTheTableSplits`. The promotion control now asserts that the
`documentation docs/ROUTES.md` pair is absent. Negative controls: with the floor removed, the held
check removed, or the fence rules reverted, the matching test fails.

Frozen `tools/retrieval-bench` v2, `--arms context`, all samples, `CORVINT_CONTEXT_*` unset
(recall@20 / @10 / @5), base d138a58 then floor 1.5 then floor 2.0: code2test 0.5116/0.3994/0.2830,
then 0.5116/0.3994/0.2877 in both floor arms; comment2context 0.5042/0.3438/0.2562 and trace2code
0.7937/0.5083/0.4010 unchanged in every arm; edit2ripple 0.6293/0.4928/0.3448, then
0.6293/0.5101/0.3448, then 0.6293/0.5101/0.3621; the abstention rate is 0.1707 in every arm. Floor
2.0 recovers the pre-V1-0186 numbers on every subset, and it dominates 1.5, so 2.0 is chosen. The
bench binaries predate the held-term guard and the fence-close rules. Those two changes only remove
routed rows, and they were not re-benched. Reports:
`/private/tmp/claude-501/-Users-russelllewis-projects-corvint/dd54e7f8-328f-4e5e-ac2a-20e9a455cd73/scratchpad/w0186-bench-v205{base,f15,f20}-{code2test,comment2context,trace2code,abstention,edit2ripple}.json`.

Probes against this repository, using seven unrelated tasks (a set reconstructed after the session
context was compacted, so it is not a frozen fixture). The base binary routes four of the seven,
and the final binary routes none. The V1-0186 orientation task still routes `docs/AGENT-ROUTES.md`
and `docs/specs/INDEX.json` through `backlog`, `memory`, `store`. The independent review found a
MEDIUM issue (unheld compounds took the maximum idf), a LOW issue (the fence toggle), a LOW issue
(the loose promotion control) and nits; all are fixed here. A stale `taskLexicalTerms` comment
found in that review is filed as V1-0214.

## 2026-09-23 V1-0197 RCB-V0-001..007: `cem export` writes a content-addressed receipt bundle

Decision: the export is `corvint cem export`, an action of an existing verb, because no new root
verb may be added and the CEM defines the change a bundle is keyed by. `witness` is a
single-report command and `dogfood` is the mutating lease lifecycle. The bundle holds exact copies
of the CEM, a saved witness report, `.corvint/dogfood-report.json` and the GOC-V0-010 full-gate
receipt. The CEM must pass the same canonical verification `cem verify` runs against an explicit
`--expected-base` and `--target`, including its evidence-drift check (`evidence-drift`), and the
target must commit it byte for byte (`bundle-map-uncommitted` otherwise); the map's own base is
never the authority. Every other
receipt binds only by exact full commit IDs, never resolved, and the gate receipt only as the exact
canonical line naming the target and its tree. Each receipt is listed with its sha256 and every
`NOT_RUN`, `NOT_PRODUCED` or `not-run` value as an RFC 6901 pointer. A receipt that is missing or
bound elsewhere is listed as absent with a reason, never synthesized. Gate-ledger records stay
out, because GL-V0-006 forbids any product-path reader. The output's opened parent and its ancestors must
match, by file identity, none of the worktree, both Git directories, the primary worktree, a
common-config `core.worktree` and every linked worktree (`bundle-output-refused`), and all writes
go through that opened parent. Known limit: a `--separate-git-dir` primary worktree without
`core.worktree` is named nowhere in the common directory (Git reports the Git directory as the
main worktree), so an export from one of its linked worktrees cannot protect it; the opener
already refuses an export run from that primary worktree.
`script/verify-receipt-bundle.sh` needs only POSIX tools and a SHA-256 command, and exits 2 on any
manifest that is not the header, the four receipt lines in order with the CEM present, and `]}`.

Deviations from the ticket, pending owner confirmation, so the spec's intent is `proposed`: the
GOC-V0-010 full-gate receipt replaces the ticket's "gate ledger" (GL-V0-006 forbids a product-path
reader), and the ticket's "or archive" option is dropped.

Evidence: PR #122's sealed CEM, with the witness report compiled in a plain clone checked out at
ace0a96 (`corvint witness --base a6a6b8b6 --head ace0a96 --cem .corvint/change.cem.json --json`,
byte-identical CEM), exported from a clone of this branch, after the review fixes, with
`corvint cem export --map .corvint/changes/ace0a96bd5ffcfa2af8013e23a1cf3220b46c24f.cem.json
--expected-base a6a6b8b66c44c486fc86daddfab3fd2d931a31dc
--target ace0a96bd5ffcfa2af8013e23a1cf3220b46c24f --output $OUT --witness $WITNESS`. Target
ace0a96 commits the byte-identical map at `.corvint/change.cem.json`, so the canonical check and
the committed-map check pass. The manifest bytes equal the pre-review export's. The manifest
sha256 is `65d16c73974d8b09f1882fe38617592cc201ffbfebb920ac134f4a9c110564da`. The CEM is present
(sha256 `b9c6f94b…c052`, no axes). The witness is present (sha256 `85f5f431…39dc`) with nine
`NOT_RUN` axes at `/obligations/0..8/verdict`. The dogfood report and gate receipt are absent
`not-found`: that clone has neither `.corvint/dogfood-report.json` nor `$GIT_DIR/corvint`, and the
primary checkout was out of bounds for the worker. The verifier, run as
`env -i PATH=/usr/bin:/bin sh script/verify-receipt-bundle.sh $OUT` from `/`, printed:

```
manifest sha256 65d16c73974d8b09f1882fe38617592cc201ffbfebb920ac134f4a9c110564da
MATCH receipts/cem.json b9c6f94bd4c103cdedc6ffe2b9727e8b760aedcd4654ea501da6311a3e53c052
MATCH receipts/witness.json 85f5f4318a9b8ac97bbd5c627e58eb649bdc801a5e256dd41ed8c22031a639dc
PASS
```

It exited 0. An independent review found text-based output checks, an unverified CEM, unexamined
sibling worktrees, a lax verifier and missing negative controls; each fix above carries a unit or
script test that was checked to fail with its guard removed, except the handle-identity check that
closes the parent swap race, which no deterministic test reaches. DR-0040's candidate cem choice
list now has twelve actions. A re-review then found four more, each fixed with a control checked to
fail without its guard: the export admitted a CEM that `cem verify` reports not ok for evidence
drift (`TestExportRefusesADriftedCEM`); `core.worktree` was not protected; the two refusal codes
lived outside `internal/cem/cemcode`; and the verifier accepted a 41-63-hex header revision (now
exactly 40 or 64). `make gate` was not run (owner preference).

## 2026-09-23 V1-0213 GOC-V0-008: the Pi tests invoke no Python and the no-Python check gates

Finding: `script/no-python-runtime-dependency_test.sh`, the GOC-V0-008 falsifying assertion, failed
on main with six hits in `integrations/pi` and `integrations/pi-protected`. The TUI tests drove a
PTY with `tui-fixture.py`, two cleanup tests forked a TERM-ignoring descendant with `python3`, and
`build.mjs` extracted the pinned Node binary with Python's `tarfile`. No gate ran the check.

Decision: `tools/pi-tui-fixture` is a stdlib-only Go PTY driver for darwin and linux. It ports both
Python drivers: the default one-prompt script and `-protected` for the reload and session
replacement sequence. It keeps the same witness, the SIGTERM-then-SIGKILL group cleanup, the
`128+signal` exit code, and the output and time bounds. On darwin the window size is set on the
replica, because `TIOCSWINSZ` on the master returns `ENOTTY` before the replica is open. The
descendant fixtures are now `/bin/sh` with `trap '' TERM`, and `build.mjs` extracts the Node binary
with `/usr/bin/tar --strip-components 2`, then refuses a member that is not a regular file. The
check is gate step `no-python-runtime-dependency-test`. It now masks its own name, because the
Makefile must spell that name. Independent review found the old line-level allow-list let any line or
path containing an allowed name hide a real invocation (a checkout directory named after the check,
or `python3` appended to a `check-analyzer-python-*` script); the check now drops only
`CORVINT_TEST_EXTERNAL_PYTEST` opt-in lines, masks the two allowed names, matches again on
repository-relative paths, and a self-test case proves an allowed name cannot hide `python3`. The GOC-V0-008
traceability row names the wiring. The gate-ledger `scopes` entry for the step is left as a
follow-up (GL-V0-003: the step runs unrecorded).

Evidence: the check exits 0 (exit 1 on the base). `node --test integrations/pi/host.test.mjs`
passes 4 of 4 against Pi 0.85.1 on macOS arm64. With the SIGKILL escalation removed, AHI-025 fails,
so the new fixture still detects a leaked process. PPI-V0-004 (startup) and PPI-V0-003 pass. A
scripted `/bin/sh` TUI stand-in drove `-protected` through every phase. The extraction command was
checked on synthetic `.tar.gz` and `.tar.xz` archives, including a symlink member.
NOT_RUN: `pi-protected-build` and the built-binary protected tests (PPI-V0-001/002 runtime and
startup injection), because the pinned SDK and Node archive are not installed. Linux PTY execution
was not run; linux and windows `go vet` pass. `make gate` was not run (owner preference).

## 2026-09-23 V1-0212 DIRTY-CACHE-013: linked worktrees share one clean index snapshot

Finding: the snapshot store was joined to the worktree root, so each linked worktree at one commit
built and stored its own full clean snapshot (V1-0198 measurement: 3 builds, 3 x 72.8 MB).

Decision: the store is `corvint/index/` under the Git common directory, keyed as before by
(object format, tree OID, engine). `gitstatus.CommonDirectory` resolves it with the bounded no-follow
`.git`/`commondir` reads status already makes and spawns no Git process, so `IDX-SNAP-V0-009`
holds. Dirty paths stay per worktree and in memory (`DIRTY-CACHE-003`/`004` unchanged). One-writer
rule: no lock; each `index` publishes a synced temporary file by rename, the last rename wins, and a
reader keeps the complete file it opened. Symlink refusal covers the worktree `.corvint` and both
store components; the eight-entry eviction bound applies to the shared store. An unresolvable common
directory falls back to the worktree's `.corvint/index/`. An existing worktree `.corvint/index/` is
neither read nor deleted: its engine digest cannot match a binary with this change. `internal/gitstatus`
and `internal/contextindex` are audited analyzer inputs, so the analyzer schema moves to
`corvint-analyzer/79`; the pack facts are unchanged. The edited task-orientation hostile test is
repinned in `UC-TASK-ORIENTATION/hostile-tests.json` by a follow-up commit, as in V1-0192.

Evidence: PR #125's `measure-worktree-index-share.sh`, run unmodified apart from its repository
path against this branch's binary, reported 1 BUILT and 2 fresh reuses across three linked
worktrees, 0 worktree snapshot bytes, one 72,890,863-byte `.gob` under the common directory, and a
dirty view (`mixed=README.md`) private to the edited worktree. Focused tests:
`TestLinkedWorktreesShareOneCleanSnapshot`, `TestConcurrentWorktreeWritersPublishCompleteSnapshotsByRename`,
`TestSharedSnapshotStoreKeepsTheEntryBoundAcrossWorktrees`,
`TestSnapshotStoreFallsBackToTheWorktreeWhenTheCommonDirectoryIsUnresolved`.

Review fixes: the install-lifecycle script now takes the store from the index receipt's `path`, so
the 0.7.0 N-1 run passes; the shared store's bound is 8 x (1 + linked worktrees), capped at 64,
with the fallback kept at 8; each operation resolves the store once; only the fallback store writes
a `.gitignore`; the analyzer schema moves to `corvint-analyzer/80`; and the spec and docs now state
the fallback, symlink-following, and `core.sharedRepository` behaviour exactly.

Owner review: `DIRTY-CACHE-013` is new; `IDX-SNAP-V0-001`/`005` (accepted, decision 0049) and
`SOP-V0-002`/`004`/`005` (accepted, decision 0341) are amended for the store location.
Rollback: revert this change; the shared directory is disposable derived state.

## 2026-09-23 V1-0126 AFP-V0-021: a dirty path selects the packages that name it

Finding: `corvint affected` selected nothing for a docs-only change. On 34e798b a one-line append to
`docs/RELEASE-NOTES.md` gave zero selections and `EMPTY_SELECTION`, although
`conformance/release-artifact-v0` reads that file by literal and the fast tier's AFP-V0-012 rule (c)
already names such readers.

Decision: the Go plugin records each package's path tokens with the rule (c) lexicon, and
`affected.Select` adds every unit whose tokens name an unowned dirty path, witnessed
`PATH_LITERAL_READER`, without traversing its dependents. The path keeps `UNOWNED_DIRTY_PATH`, so
the scope stays `UNKNOWN`. The gate tool reports 129 packages whose reads no literal bounds (rule
(d)), and this plan does not model them. The naming relation is mirrored, not shared:
`tools/gate-affected-select` is a stdlib-only `package main` built by the trusted PR driver.

Evidence: the same probe now selects 26 packages, `RUNNABLE`, still `UNKNOWN`. That set equals the
gate's rule (c) readers of the path plus those among its unresolved packages. Graph build CPU rose
from about 2.3 s to 4.3 s user time on this repository. The CEM sidecar narrowing is not mirrored.
The doc checks and the selected packages pass; `make gate` was not run (owner preference).

Review follow-up: the gate applies rule (c) to every dirty path, so the plan now does too. An
appended `extensions/vscode/src/executable.ts`, owned by the TypeScript plugin, had selected only
TypeScript tests; it now also selects `conformance/release-artifact-v0`, which names it. The token
bound became a per-package mark reported as `go:path-token-bound:<unit>` only in a plan that
attempts a match, instead of a graph frontier that would widen every plan; no package in this
repository reaches it. A lex error under a `testdata` or `_`-prefixed directory no longer raises
`go:unparsed-source`.

## 2026-09-23 V1-0199 SESSION-V0-017..019: a dogfood handoff receipt re-resolves or reports drift

Finding: a handed-off enrollment kept its session key and root (LCP-V0-003), but nothing named the
context the sender compiled. A receiving session re-derived its dogfood prompt packet and could see
different evidence after a commit without any signal.

Decision: add the read-only `corvint dogfood handoff` subverb under the existing `dogfood` verb, not
a new root verb and not a change to the frozen `dogfood status` output, so status stays cheap. The
emitted receipt names the key, root, bound revision, enrollment, sorted anchor tokens each with a
per-anchor evidence digest, the SHA-256 and bytes of the unchanged `corvint-dogfood-prompt/0`
packet, and the degradation list. It carries `authority: none` and is repeated inside the untrusted
data envelope. With `--receipt`, the receiver recompiles and either returns the byte-identical packet
(exit 0) or reports ordered root, revision, enrollment, anchor and packet drift and withholds the
recompiled packet (exit 1). The slice is recorded as SESSION-V0-017..019 in the otherwise deferred
session-context-dividend spec. SESSION-V0-001..016 stay deferred. CCF-V1-002 lists `handoff` as an
unpinned dogfood mode.

Evidence: `TestDogfoodHandoffReceiptReresolvesSamePacket` (same revision re-resolves the same
digest and bytes, and neither step changes private state) and
`TestDogfoodHandoffReportsRevisionAndAnchorDrift` (a commit that moves the anchored requirement line
reports revision drift and drift for that anchor only; a foreign key, a malformed receipt and
exclusive options fail closed). `go test ./cmd/corvint`, `go vet ./cmd/corvint` and the spec
index, requirement, traceability, decision-number and line-citation checks pass. The post-commit
CEM bind, check and seal loop was not run for this change.

Review repair (independent review, same day): receipt fields are untrusted, so the receiver now
accepts only a document whose bytes equal what `emit` produces for its decoded value (this rejects
duplicate, case-folded, unknown and reordered members, reformatting and trailing data; it is
stricter than the unexported `strictJSON`/`wire.Parse` path, which a mutation check showed added
nothing) and whose every field has its emitted shape: clean absolute root of at most 4096 bytes,
lowercase-hex or empty revisions and plan digest, closed worktree and lifecycle enums, the constant
packet profile and bytes in 1..budget. Drift rows echo only validated values. Stdout escapes
non-ASCII, so exit 0 now returns `packetBase64` with exactly the digested bytes rather than claiming
byte identity for escaped JSON; the test hashes the decoded bytes of a non-ASCII fixture. Consume
reports current degradations, roots compare as symlink-resolved Git toplevels, and the spec now
says the receipt re-resolves the handoff packet (budget 8000, anchor-only task text), not an earlier
prompt-event packet. The session-context-dividend MUST NOT sentence now carries the V1-0199
exception itself (owner review pending). Added evidence:
`TestDogfoodHandoffReportsEnrollmentDriftAndDegradations`,
`TestDogfoodHandoffRefusesMalformedReceiptsAndAnchors`, `TestDogfoodHandoffRefusesUnstableRepository`
(through a probe seam) and the unix-only `TestDogfoodHandoffReceiptUnavailable` (symlink, FIFO,
oversized, missing), plus a no-write assertion on the drift path.

## 2026-09-23 V1-0200 AGW-V0-003, DCW-V0-016: receipts name each compiled packet's cost

Finding: the dogfood report and the witness report compiled context packets but did not record what
they cost. Each packet carries a coverage block, but only the raw step output under
`<git-dir>/corvint/` kept it, and the witness report kept none, so context cost could not be traced
to a change.

Decision: both reports gain one additive member, `packetCoverage`. Each entry copies the packet's
`packet_bytes`, `budget_bytes`, `within_budget`, `included_results` and `omitted_results` under
those names. Neither profile changes: `corvint-dogfood-change/0` and `corvint-witness/0`.
- `dogfood-change` writes one line after `dogfoodPolicy`, with an entry for `prechange-query` and one
  for `prechange-impact`. A step that compiled no packet is `NOT_PRODUCED` with
  `packet-not-compiled`. Output without exactly one well-formed occurrence of each field is
  `NOT_PRODUCED` with `packet-coverage-unreadable`. The line never changes `complete`.
- `corvint dogfood begin` compiles no packet, so it records no coverage.
- `dogfood finish` reads the report with a strict parser followed by a struct decode, which ignores
  unknown members, so finish needed no change.
- Witness lists the `admission` packet, then one `closure` packet for each admitted path, and adds a
  `PACKETS` text section. A refused stage adds no entry.
- The console dogfood pane shows the numbers. For a report written before the member existed, it
  shows "not reported". Historical receipts are not rewritten.
- Both requirements are proposed additions to specs whose intent is accepted, and they await owner
  review.

Evidence:
- `TestPacketCoverageEqualsEveryCompiledReceipt` compares the witness JSON with the receipts that
  `RangeImpact` and `Impact` return. It fails when `Compile` leaves the member empty.
- `script/dogfood-change_test.sh`, run by `TestGoOnlyContextAbstentionRemainsClosed`, checks three
  things: the exact line, `packet-not-compiled` on the impact abstention, and
  `packet-coverage-unreadable` on a duplicated key. It exits 1 when the line is removed.
- `TestConsoleDogfoodPacketCoverage` reads the sealed historical fixture without error.
- A scratch test, not kept, parsed the real report below with finish's strict `wire.Parse` and
  passed.

Real run, on 2026-09-23, of the committed branch head 8729b92 against base d138a58:
- Setup: a binary built from the branch into scratch ran `dogfood-change` in a throwaway clone,
  because the run writes the tracked CEM and the local trace.
- The report recorded `prechange-query` with `packet_bytes` 6314, `budget_bytes` null,
  `within_budget` true, 1 included and 4 omitted.
- It recorded `prechange-impact` with `packet_bytes` 8831, null budget, within budget, 6 included
  and 0 omitted.
- Both sets of numbers match the step outputs field for field. The run was deliberately not
  complete: no citation, intent or outcome inputs were given.
- `corvint witness --json` on the same range listed 7 packets totalling 60977 bytes. The admission
  packet was 8781 bytes. The closure packets ranged from 3335 bytes (`internal/witness/witness_test.go`)
  to 14469 bytes (`internal/console/views.go`). All were unbudgeted and within budget.

## 2026-09-23 V1-0182 DCW-V0-017: dogfood-check verifier agreement is author-only evidence

Finding: in a fresh clone of the V1-0213 bind commit `5397b08` (base `34e798b`), `make dogfood-check`
with `CORVINT_BIN` set fails `dogfood-report-missing` and names only the author's `dogfood-change`.
Copying the author's report into the clone moves the failure to `local-outcome-evidence-drift`: the
check also needs the author's private `<git-dir>/corvint/local-outcome.json`, OCM maps, intent
snapshot and abstention evidence, and no committed artifact binds the report's digest.

Decision: option (b) of the ticket. A `DOGFOOD_REPORT` input would have to trust an unauthenticated
report plus further private files, or rerun the author's coordinator, so reviewer-side
`outputsAgree` is not offered. `DCW-V0-017` (new requirement in an accepted spec, owner review) and
`docs/DOGFOOD.md` step 11 state that the verifier set and `outputsAgree` are author-only and name
what the reviewer verifies instead: `cem verify` and strict `cem status` on the committed CEM with
their own binary, the seal as one exact rename, and the report's semantics. When `HEAD` tracks
`.corvint/change.cem.json`, `dogfood-report-missing` also prints a `review:` line with that
`cem verify` command; reason code and exit status are unchanged. `docs/AUTOMATION.md` cites the
new line range.

Evidence: the same reviewer clone now prints the `review:` line, and the named command exits 0
(`valid`, `canonical`); strict `cem status` reports `ready-for-ci`, 19 of 19 hunks supported.
`script/dogfood-change_test.sh` adds a bind-commit reviewer case and a no-sidecar absence case;
forcing the line unconditionally fails the test. `make gate` was not run (owner preference).

## 2026-09-23 V1-0173 DCW-V0-019: a citation plan must match the map prepared in the same run

Finding: `dogfood-change` applied a stale 9-row `DOGFOOD_CITATIONS` plan by ordinal to a map a later
commit had re-prepared with 10 hunks; nothing refused it, and the tenth hunk stayed unknown.

Decision: before any `cem cite`, the coordinator binds a nonempty plan to the prepared map. No
ordinal may exceed the hunk count, and every hunk the map still records as `unknown` must be named
by ordinal or full hunk ID. The one exception is the hunk of an intent spec absent at BASE, which an
author leaves uncited on purpose (decision 0055). A mismatch refuses `cem-cite` with
`citation-plan-map-mismatch`, cites nothing, and prints a `fix:` line. A plain row-count equality
was set aside because it would refuse that bootstrap omission, repeated rows (several bases for one
hunk), and split plans on a resumed map. The plan format is unchanged, because the first field
already accepts content-derived full hunk IDs and a stale ID already refuses `unknown-hunk-id`. The
remaining gap: a stale ordinal plan whose ordinals still cover every hunk, such as one after a
reorder, cannot be detected. DOGFOOD.md recommends full IDs when a later commit may reorder hunks.

Evidence: `script/dogfood-change_test.sh` covers four cases: stale-nine-of-ten, stale-ten-of-nine,
bootstrap-omitted and other-omitted. `TestDogfoodReasonAdmitsCitationPlanMapMismatch` covers the new
reason. In a scratch clone against the real 22-hunk map at base 34e798b, a 1-row plan refused
`citation-plan-map-mismatch` with no cite, and a 22-row plan cited all 22 hunks. `make gate` was
NOT_RUN.

Review fixes (independent review): with more than 256 unknown hunks, one plan cannot name them all,
so the unnamed-hunk rule is not applied and split plans on a fresh map stay usable (case
split-over-row-limit). A numeric selector that is not a canonical ordinal, such as `01`, is now
refused before any cite (case noncanonical-ordinal); previously `cem cite` refused it only after
earlier rows had staged. The fix line now names both causes of a mismatch, not only a re-prepared map.

## 2026-09-23 V1-0142 DCW-V0-018: the dogfood loop links OCM obligations from an explicit author plan

Finding: `dogfood-change` regenerates every OCM map with `--replace` on each pass and never runs
`ocm link`, so each sealed change reported every requirement `unassessed`. Replaying sealed V1-0196
(base 01b6804, bind 3500aba) gave 0 of 12 linked. Links added by hand were dropped by the next commit.

Decision: an optional `DOGFOOD_OCM_LINKS` TSV plan names, per intent, the requirement, the cited CEM
hunks, the test path and the test claims. After each map is prepared on every pass, each row runs
through the verified `corvint ocm link` and reports `ocm-link-NNN`, numbered by plan row; a refused
row does not stop later rows. Nothing is inferred: without the plan no link runs and no row is
reported. A missing, malformed or empty plan (`ocm-links`) or a refused row is NOT_PRODUCED with a
`fix:` line and blocks completion; the OCM aggregate is still produced. The report records the plan
as `ocmLinkPlan` (sha256 and row count) because the plan and the maps stay local, so a reviewer
reproduces coverage only by rerunning with the same plan. No new verb; OCM-V0-013 already limits
map changes to verified link and mark.

Evidence: `script/dogfood-change_test.sh` covers no plan, exact argv and prepare-link-status order,
the plan digest, refusals of rows 2 and 4 across two intents with rows 3 and 4 still run, and
unlisted-intent, CRLF, field-count, empty-item (`1,,2`, refused by validation), over-256-row, empty
and absent plans. Restoring stop-at-first-refusal or dropping the empty-item check fails the test.
Replaying sealed 75039ff (base af6fd52, LAC-V0-032) through the patched script with a one-row plan
produced `ocm-link-001`, 1 of 32 linked and `ocmLinkPlan` matching the plan's sha256. A link on a change delivered through this
loop is NOT_OBSERVED; `make gate` was not run.

## 2026-09-23 V1-0125 PRS-V1-005: Core-only candidate reader and installer

Finding: `releasecandidate.VerifyContext`, and through it `InstallCore`, refused a candidate that
lists only Core artifacts with `candidate manifest identity is invalid`. The reader required two
sources, the companion and Tasks roles, a verified companion bundle and PASS darwin/arm64 companion
rows, so `PRS-V1-005` (no companion input) had no reader or installer path.

Decision: a second closed manifest profile, `corvint-core-release-candidate/0`, selects a Core-only
inventory (sources `[corvint]`, no companion or Tasks roles, each single-file role bound to its
assembler path). All Core checks are unchanged. The
receipt keeps its 28 rows: `core-archive` PASS and every other row `NOT_RUN`, with
`companion-bundle` evidence `companion not present`. Any other status is refused. The combined
profile is unchanged. Contract: `public-release-v0.md`, "Core-only candidate reader and installer".

Evidence: `TestPRSV1005CoreOnlyCandidateVerifiesAndInstalls` verifies and installs a Core-only
fixture without consulting companion evidence. It refuses a companion PASS claim and a drifted
core archive. Limits: the Core-only source archive is bound by digest only, and the assembler still
requires companions, so no Core-only candidate can be produced yet.

## 2026-09-23 V1-0191 MCPV0-024, MCPV0-025: task-context and CEM report as read-only MCP tools

Finding: `corvint-mcp` exposed only `corvint.query`, `corvint.impact` and `corvint.status`. An agent
connected over MCP therefore could not get the task-context packet or the CEM reviewer report
without shelling out to the CLI. `corvint cem report` also always publishes
`.git/corvint/cem-review.md`, which a read-only tool must not do. The CEM Git runner looked `git`
up on `PATH` at every spawn, outside the start-time pin that `MCPV0-016` requires.

Decision: add `corvint.context` and `corvint.cem.report` (`MCPV0-024`, `MCPV0-025`). Decision 0374
(entry above) later moved them behind an opt-in selector. Both tools reuse the `MCPV0-007` descriptor rules, the
`MCPV0-008` envelope, terminator-collision refusal and 393,216-byte budget, and a bridge `tool` enum
widened to five names. Both run in process, with no shell.
- The context tool builds the CLI packet from an existing snapshot or an in-memory observed index. It
  never writes a snapshot and skips the gopls attachment.
- The report tool uses a new `report-preview` workflow action. It renders the CLI report bytes and
  publishes nothing.
- The map argument is a closed repository-relative path, with no `.git` segment in any case. A
  symlinked map or ancestor fails as `cem-map-unavailable`. The map is repository-authored, and
  following a link would let a tracked file make the server read outside, or inside Git metadata of,
  the root bound at start.
- Schemas advertise byte bounds as code points divided by `utf8.UTFMax`. The task pattern excludes
  U+0085 as well as ECMA `\s`, so every schema-valid argument is runtime-valid.
- `gitrun.PinBinary` fixes the CEM runner to the Git path `gitstatus.Pin` resolved.

Accepted decision 0103 froze MCP V0 at three tools and names in-place tool additions a silent
profile broadening. This first candidate conflicted with it; decision 0374 resolves that with a
descendant profile.

`mcp-server-unavailable` stays in `integrations/compatibility.json:73` and both adapter manifests.
`integrations/README.md:43-45` defines `globalDegradations` as the V7 items `ROADMAP.md:611`
records as not delivered. This spec is still proposed/experimental, and AGENTS.md forbids
advertising an experimental prototype as delivered. The official Streamable HTTP runner is still
`NOT_RUN`, and the receipt below covers no host tuple. Removal belongs with owner acceptance, in the
same change as `integrations/README.md:15` and `ROADMAP.md:611`.

Evidence: all runs used Go 1.27.1 and the official schema with sha256
`ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`, verified before each run.
- `CORVINT_MCP_OFFICIAL_SCHEMA=… go test -count=1 -timeout 30m -v ./conformance/mcp-2026-07-28`
  passed: 48 PASS, 0 SKIP, 0 FAIL, and `TestServerTrafficMatchesOfficialSchema` passed. The same run
  with `-race` also passed with no race reported. `-race` instruments the test process only;
  `TestMain` builds the server binary without it.
- The official-schema exchanges now include context success and CEM report success, tool error and
  `-32602`.
- New compiled-process cases cover the rest. The whole root, `.git` included, is unchanged, and no
  `cem-review.md` is written. The receipt is enveloped. Fourteen argument shapes return `-32602`.
  Symlinked maps are refused without leaking the outside path. A terminator in a hunk path returns
  the collision error, and a 1,500-hunk report abstains with `OUTPUT_BUDGET_EXCEEDED`. The planted-Git
  case now also calls `corvint.cem.report`.
- Negative controls were each reverted. Removing `PinBinary` fails
  `TestGitPlantedOnPathAfterStartNeverRuns`. Previewing through the publishing `report` action
  fails `TestContextAndCEMReportAreBoundReadOnlyAndFramed`.
- In-package tests prove preview Markdown byte-identical to the CLI-published file, and probe drift
  returns `REPOSITORY_STATE_UNSTABLE`.
- Follow-ups:
  - (corrected 2026-09-24) the VS Code extension's `expectedTools()` lists exactly the three V0
    tools, so this candidate's five-tool default would have failed it with `toolset-mismatch`
  - official conformance, fuzzing, complete race and cross-build evidence remain open

## 2026-09-23 V1-0196 triggered-automation contract (docs/AUTOMATION.md)

Finding: nothing stated which Corvint commands are safe as a triggered CI, hook or team-automation
step, what each may write, or how each exits.

Decision: `docs/AUTOMATION.md` lists `affected` (writes nothing) and `cem verify`, `cem status`,
`witness` and `impact --base` (at most one bounded self-observation row, and only when that ledger
and its temporaries are gitignored) as the triggered read-only steps. Each has source-cited exit
codes, output profile and inputs. `cem report` and `make dogfood-check` are excluded because they
write local state. The page states that Corvint runs no always-on component and that a triggered
run grants no authority (decisions 0081 and 0373). `stable-operations-v0.md` and `AGENT-ROUTES.md`
link it.

Evidence: the worked example ran from a fresh `git clone --no-local` at 01b6804 with base a6a6b8b.
It verified the sealed V1-0204 CEM (canonical, 39 of 39 hunks supported), left the clone unchanged,
and reproduced V1-0182 (`dogfood-check` fails `dogfood-report-missing`). Its runtimes were measured
under heavy benchmark load and are inflated. Signal exit codes of the Go commands, Linux and
shallow clones were not exercised. The doc checks pass; `make gate` was not run (owner preference).

## 2026-09-23 V1-0191 MCPV0-016: corvint-mcp pins Git at start; official schema executes

Finding: the MCP 2026-07-28 spec kept two promotion blockers. The shared Git resolver memoised
`git` per `PATH` and `DEVELOPER_DIR` but resolved it at first use, not at start, and
`plansnapshot` looked `git` up on `PATH` on every call that carried a planning snapshot. A `git`
planted earlier on the server's `PATH` after start could therefore run.
Official-schema execution was `NOT_RUN`, because the no-network suite had no local copy of the
pinned schema to validate against.

Decision: `gitstatus.Pin` resolves Git once and fixes that absolute path for the process, and
`Executable` returns it from then on. `plansnapshot` now spawns the resolver's path instead of its
own lookup. `corvint-mcp` calls `Pin` before building the bridge and exits with status 2 when Git
does not resolve. The official schema stays unvendored. `PROVENANCE.md`
states that all source here is owner-authored, and the upstream schema repository is moving from
MIT to Apache-2.0, so copying it in would need its own rights record. Instead
`TestServerTrafficMatchesOfficialSchema` runs when `CORVINT_MCP_OFFICIAL_SCHEMA` names a local copy.
It refuses any digest other than the pinned one. It validates the suite's discovery, tool-list,
tool-call and error requests and the live server's responses against the schema's `$defs`, and it
fails on any JSON Schema keyword its checker does not implement. `cases.json` now lists no
promotion blockers. `internal/gitstatus` is an audited analyzer input, so the analyzer schema moves
to `corvint-analyzer/78`. The status stays proposed and experimental: official MCP conformance, the
task-context and `cem report` tools, and owner acceptance remain. This amends proposed MCP spec
text; the owner's PR review is its human review.

Evidence: `TestGitPlantedOnPathAfterStartNeverRuns` plants a `git` that writes a marker after the
server starts. It then calls `corvint.status` and `corvint.impact` with a planning snapshot; both
succeed with no marker written. The test fails with the `Pin` call removed from `corvint-mcp`, and
again with the old `plansnapshot` lookup restored, so it observes both. A second subtest starts the
server with no `git` on `PATH` and gets exit status 2 with `corvint-mcp: git unavailable`. An
independent review found the `plansnapshot` bypass, which this change then closed. The opt-in schema run
passed on 2026-09-23 against a copy whose SHA-256 matched the pin, with and without `-race`. The
affected Go packages pass; `TestHostAdapterJavaScriptHarnessInterruption` failed once at host load
above 100 and passed three times alone. `make gate` was not run (owner preference).

## 2026-09-23 V1-0204 AFP-V0-020: every plugin names a changed unit no test reaches

Finding: AFP-V0-020 named a changed Go package with no tests as `NO_SELECTABLE_TEST`, but the
other seven plugins silently omitted a changed source unit that no test reached. The plan stayed
`BOUNDED` with nothing selected for it.

Decision: each plugin defines "no selectable test" through one lookup table in `affected.Select`.
Go keeps its own-tests rule, because the go tool runs a package's tests only against that package.
Every other plugin may keep tests in the unit itself, as Rust does, or in units that depend on it.
A changed unit there has no selectable test when no unit it reaches through the graph, itself
included, declares a test. The graph computes that set once when it is built, with one walk over
forward imports from every unit that declares a test, so each changed unit costs one lookup. A
frontier plan now also names such a unit when the frontier hides the edge from its test. The
TypeScript, Ruby and .NET frontier tests expect that second unknown. The Playwright plan still
widens to the full relevant suite only on the shared graph's other unknowns, so it does not yet
name such a helper and stays `BOUNDED` for it. How it should report one is V1-0211, a follow-up.
This amends proposed AFP-V0-020 text; the owner's PR review is its human review.

Evidence: `TestSeamWidensWhenNoTestReachesAChangedUnit_AFPV0020` removes the conformance fixture's
`solo` tests in every language. An edit to `solo` must give `NO_SELECTABLE_TEST` at `UNKNOWN`
scope, and an edit to `core` must not. It fails for all seven non-Go plugins on the base rule and
passes with the change, and it checks that the unknown names the `solo` unit. `make gate` was
not run (owner preference).

## 2026-09-23 V1-0192 LCP-V0-010, LCP-V0-013: task mentions anchor prompt context

The native user-prompt event resolved only explicit paths, requirement IDs and source-backed
identifiers, so a prompt naming `pkg/packet.go:42`, `pkg/packet.go#ParsePacket` or a commit gave
`explicit-task-anchor-required` or incidental rows. `LCP-V0-013` adds three mention forms to the
same pure compiler, and `LCP-V0-010` now counts them as explicit anchors. That amends accepted
intent, so it needs the owner's review on the PR.

A mention is one whitespace field whose path part is path-shaped: it contains `/`, has an
extension, or is tracked. That keeps `localhost:8080`, `issue#12` and hex-looking English words
out. A line or range is checked against the bound source's line count and names its start line
with the existing row schema. `path#name` matches exact declarations in that path only, and a
dotted name falls back to its terminal part as `qualification-unverified`, because Go method
symbols carry bare names. A commit resolves only as a prefix of the bound commit. Any other commit
is `anchor-evidence-unavailable`, because the profile reads no history (`LCP-V0-011`). Resolving
other commits needs caller-acquired Git facts and is a follow-up.

Evidence: 20 frozen cases in `internal/contextindex/testdata/local-completion/mention-cases.json`,
identity, dirty, deleted-source and budget tests, and two new UC-TASK-ORIENTATION hostile cases
through the native prompt entrypoint, whose `hostile-tests` receipt is repinned. The analyzer
schema moves to `corvint-analyzer/77`.

Deviation from the ticket: acceptance criterion 4 names the frozen daily-loop evaluation. Its
orientation job scores `corvint context --task`, which this change does not touch, and it builds
the preregistered candidate commit, so it cannot regress here. Adding mention tasks to its corpus
would amend the preregistration; that is a follow-up ticket, not part of this change.

Review repair: an independent review found that trailing `).` and closing brackets were not trimmed
and fell back to a wrong line, that a line or declaration added to a dirty file was reported as
`anchor-not-found` rather than `anchor-worktree-changed`, that mention removal cut substrings out of
unrelated words, that `./path:4` and `path:4` counted as two anchors, and that the commit-prefix test
could skip its later checks. The parser now trims any trailing run of brackets and punctuation,
accepts `path:line:column`, removes matched fields whole, and deduplicates by parsed form; a dirty
candidate without a match in its bound blob counts as worktree-changed. `LCP-V0-013` now states
these rules and narrows the `host:port` claim: a dotted host is path-shaped. The frozen corpus grows
to 33 cases and the analyzer digest is repinned. A second review found that trailing `-`, `…` or
`**` still fell back to line 1, that a zero or inverted range on a dirty path claimed a worktree
change, and that a dirty base-name candidate beside a clean match made the anchor ambiguous without
the spec saying so. The trailing trim now covers any punctuation or symbol except `_`, impossible
ranges are `anchor-not-found` first, and `LCP-V0-013` states the ambiguity: the worktree copy may
hold the line. Markdown links such as `[a.go:4](a.go#L4)` remain unparsed and report
`anchor-not-found`.

## 2026-09-23 V1-0189, V1-0208: the policy projection is restored; the checklist gate is not rewired

Correction to the V1-0189 entry below. PR #117 edited `.taskman/policy.json` so the
`release-checklist` gate would run `script/release-checklist --pre-promotion`. That file is a
projection of the task-store journal, and corvint-tasks writes `intent/policy.json` only in its
`init` transaction, so the edit could not be adopted. After the merge every store read and
mutation refused with `INTENT_DIVERGED` on `intent/policy.json`.

This change restores the projection to the journal's policy bytes, so `receipt audit` reports OK
again. The script mode from V1-0189 stays on main and its tests still pass. The gate itself still
runs plain `script/release-checklist`, which cannot exit 0 at any commit, so
`gate:release-checklist` cannot be attested and every candidate stays `BLOCKED` on it.

V1-0208 records that blocker and needs the owner to choose a route: a policy mutation in
corvint-tasks, which its own specification already describes as an owner or operator mutation, or a
recorded decision that changes how this gate is attested. V1-0189 stays open and now depends on
V1-0208. V1-0209 and V1-0210 are follow-ups filed from V1-0192.

## 2026-09-23 V1-0203 AFP-V0-008: the Go plugin ignores `testdata`

Finding (V1-0187 review, NIT 3): the Go plugin built units from `testdata/` directories, which the
go tool never treats as packages. Under AFP-V0-020 a fixture edit could then read as a changed Go
package with no tests (`NO_SELECTABLE_TEST`), and a fixture `go.mod` raised
`go:nested-module-frontier`.

Decision: a directory named `testdata` below an observed module's root, with its descendants,
contributes no unit and no nested-module frontier, and `Owns` rejects a `.go` path with a
`testdata` component. The rule is applied relative to the owning module, as the go tool applies
it, so a `go.work` module whose own root lies below `testdata` is still observed. A fixture edit
now reads as `UNOWNED_DIRTY_PATH`, the reason the selector already documents for fixtures, so the
plan stays `UNKNOWN` for it. Attributing the fixture to the enclosing package was rejected here:
other packages can read the same files, so that narrowing would be unsound in the full plan. The
fast tier (AFP-V0-012) keeps its own enclosing-package attribution. This amends accepted AFP-V0-008
text (decision 0057); the owner's PR review is the human review that change requires.

Evidence: `TestTestdataIsFixtureDataNotAPackage_AFPV0008` and
`TestWorkspaceModuleBelowTestdataIsObserved_AFPV0008` fail on the base plugin and pass with the
change. Replay of `corvint affected --base 062b0151b603a5d637cdd0ad0f560edefee921a4` on a clean
tree at b195af3 gives the same 65 selected units with identical witnesses and identical unknowns
before and after; the only graph difference is one fixture unit
(`internal/liveverify/gotest/testdata/livefixture`) gone from `plan.excluded`. The Go packages the
affected plan selected for this change pass. `make gate` was not run (owner preference).

Review repair: an independent review found that `Owns`, which sees only the repository-relative
path, also disowns an unindexed file of such a workspace module. A deleted or added file there is
labelled `UNOWNED_DIRTY_PATH` instead of `UNINDEXED_SOURCE_PATH`, and the module's unit is then
excluded as `NO_DEPENDENCY_PATH_TO_DIRTY_UNIT`. The plan stays `UNKNOWN`, so no selection is
narrowed. The spec now states the limit and the workspace test asserts it; module-aware ownership
was set aside as a larger change than this rare layout warrants.

## 2026-09-23 V1-0189 ARTIFACT-RDY-V0-001: `release-checklist --pre-promotion` exit for candidates

Ticket V1-0189 (filed with decision 0373 item 17): the `release-checklist` policy gate expects exit
0, but `script/release-checklist` exits 0 only when all seven rows are `PASS`, and that is
unreachable at any commit, not only before promotion. native-performance is `NOT_RUN`
unconditionally under `GOC-V0-005`; the tag row is `PASS` only when the tag points at HEAD, while
the publication row reads a receipt committed after the tag (decision 0141), so the two cannot be
`PASS` at one HEAD. Every candidate therefore recorded `gate:release-checklist` as missing (the
v0-5 attestation of 2026-09-23 records exit 1 with the tag row `FAIL`).

Chosen fix: the first option in the ticket, a mode, over dropping the gate. The checklist gains the
single argument `--pre-promotion`. The seven rows, their statuses and reasons are identical in both
modes (the closed `PASS`/`FAIL`/`NOT_RUN` vocabulary is unchanged; `NOT_RUN` is the existing
"pending" state, so no `PENDING` status was added). Only the exit differs: zero when
native-runtime, go-archive and full-gate are `PASS` and no row is `FAIL`, one otherwise. `NOT_RUN`
on native-performance, tag, publication and promotion does not lower that exit; a `NOT_RUN` is
never reported as `PASS`. The mode keeps the gate's real pre-promotion value, the receipt bindings
of the archive witness and the full gate to the candidate commit and tree, and it keeps the tag
`FAIL` for a candidate whose `VERSION` names a tag already placed on another commit, which is a
defect of the candidate. `.taskman/policy.json` now runs `script/release-checklist
--pre-promotion` for the `release-checklist` gate; existing candidates already need
re-candidating because their `candidate-source-or-policy` binding is stale.

Evidence: `script/release-checklist_test.sh` passes with the new block (identical rows in both
modes, exit 0 with the three candidate rows `PASS` under a fake `go` archive-status reader and the
other four `NOT_RUN`, exit 1 with the witness absent or the tag row `FAIL`, exit 2 for any other
argument). On this tree at b195af3 both modes exit 1 with identical rows: native-runtime `PASS`,
tag `FAIL` (`VERSION` 0.7.0 while `v0.7.0` points at another revision), the rest `NOT_RUN`; the
pre-promotion exit becomes 0 only for a candidate whose `VERSION` names an unplaced tag after
`make gate` records both witnesses at its head. `ARTIFACT-RDY-V0-001` and the `GOC-V0` current-state
paragraph record the mode; `make gate` is `NOT_RUN` under the focused-verification policy.

The argument parser moved the archive-witness guard down, so the anchored citation in
`docs/specs/go-archive-gate-v0.md` was repinned from lines 47-48 to 74-75 after reading them: the
anchor `74e4657d` is unchanged and the cited sentence still holds.

Review repair (independent review of PR #117): the test suite now isolates each candidate row
(a stale archive witness with a PASS full gate, and a PASS witness with no gate receipt, each exit
1) and the tag `FAIL` alone (publication `NOT_RUN`, every candidate row `PASS`, exit 1). Dropping
`go-archive` or `full-gate` from the candidate set, or the `FAIL` latch, now fails the suite; before
the repair the first two passed it. The `go-only-cutover-v0.md` sentence now says the exit is 0
only when no row is `FAIL`. Open for the owner: `VERSION` is 0.7.0, already tagged at 678c1b1, so a
candidate on main keeps a tag `FAIL` until `VERSION` names an untagged release.

## 2026-09-23 V1-0207 UCV0-006, UCV0-010: corvint-dogfood receipts for the three Core use cases

V1-0011 criterion 3 asks that Corvint and Beamfall dogfood receipts bind real changes and stay
distinct from hostile tests and sealed benchmarks. This entry records the Corvint half; Beamfall
dogfood stays owner-run (V1-0184).

Change chosen: the V1-0188 review-repair increment of PR #111 (commits 1c6451f and 789245c),
completed in a fresh clone through the `docs/DOGFOOD.md` daily path with binaries `dogfood-change`
builds from the tree, not the installed release. Its base a062f730426424c4767eb79ea88d89363c412963
is the seal of the first V1-0188 cycle, so the bound range is seven files: two Go test edits, two
repinned `hostile-tests` receipts, the ledger, this log and the CEM. Its bind commit is
8668f77b73eaf7b203abbb922aa1fe9cff8fd6c8; its CEM is sealed on `main` at
`.corvint/changes/8668f77b73eaf7b203abbb922aa1fe9cff8fd6c8.cem.json` by commit 955d2ad, which
`script/dogfood-seal.sh` writes only after `dogfood-check.sh` exits 0, so `dogfood-check` PASS for
that binding is inferred from the seal, not held by any subject. The increment is itself
hostile-tests work on the same three rows. It still serves as dogfood evidence because the class
attests that the daily path was used on a real Corvint change, not what the change contains, and the
`hostile-tests` receipts stay pinned to their own entrypoints; whether that distinctness is enough
for V1-0011 criterion 3 is the owner's call. It was preferred over the V1-0202 change, whose
`prechange-impact` is `OUT_OF_SCOPE` with no result because that change touched no Go file. The
daily path never retains its orientation, consequence and completion outputs in the tree
(`.git/corvint/` and the ignored `.corvint/dogfood-report.json`), so each receipt pins a
byte-identical copy under `receipts/<useCaseId>/corvint-dogfood/` beside the sealed CEM.
`local-outcome.json` is not copied because it names an absolute local trace-store path; the report
binds it through `localOutcomeEvidenceSha256`.

What each retained artifact shows, read before writing the receipt:

- UC-TASK-ORIENTATION, `prechange-query.json` (revision tree 15fd5f4e, state `READY`): the
  script's default request `Dogfood change from a062f7304264 to HEAD` (`DOGFOOD_TASK` unset),
  limit 1, returned `docs/decisions/0048-four-owner-calls-2026-09-04.md` with authoritative
  evidence and no abstention. It is the post-commit rerun at the bind commit: its `history_tip` is
  8668f77b and 15fd5f4e is that commit's tree. It shows the orientation step ran on the change,
  not that orientation preceded the work (`docs/DOGFOOD.md` §1) or that its answer was relevant;
  relevance is what the sealed benchmark and hostile tests measure. A later receipt should set
  `DOGFOOD_TASK` and retain the start-of-change query.
- UC-CHANGE-CONSEQUENCE, `prechange-impact.json` (profile
  `corvint-range-impact-expanded/experimental`, range `CLEAN`, state `READY`): two authoritative
  results, the two changed Go test files, with the uncertainty row that five non-Go changed paths
  are outside the native Go range profile.
- UC-EVIDENCE-CARRYING-COMPLETION, `dogfood-report.json` (profile `corvint-dogfood-change/0`,
  `complete: true`): all nine steps `PRODUCED`; OCM aggregate `ready-for-review` with 13 of 13
  UCV0 requirements `unassessed`; `testExecution` `NOT_RUN`; `dogfoodCheck.outputsAgree` true;
  anchor `NOT_OBSERVED`.

Each receipt attests `{"outcome":"PASS","repository":"corvint"}`, pins the bind commit, and lists
two subjects: the retained artifact and the sealed CEM. The ledger rows gain a `corvint-dogfood`
evidence entry and stay `experimental` with claim `UNPROVEN`; the validator reports `valid: true`
with 15 evidence references. The receipts prove that one real change was completed through the
daily path with bound, internally consistent evidence (`DCW-V0-015`), not that the change is
correct or that its tests are adequate. Remaining for `verified`: `beamfall-dogfood` (V1-0184).
No product code changed.

## 2026-09-23 V1-0202, PCCO-V0-015..016, AFP-V0-020, UCV0-003: daily-loop preregistration amendment 1 and sealed run-002 (three PASS)

Defect (V1-0202): `harness.py` `consequence_case` scored only `plan.selected`, so the AFP-V0-020 fix
(V1-0187), which names a changed Go package without tests as a `NO_SELECTABLE_TEST` unknown, could
not change the UC-CHANGE-CONSEQUENCE score. The sealed preregistration fixed that rule ("selected =
go: unit IDs"), so the rule change is preregistration amendment 1, recorded in
`benchmarks/daily-loop-v0/preregistration.json` (`amendments[0]`) before any run used it: the
treatment covers a critical package when `plan.selected` names a `go:` unit of it or when
`plan.unknown` carries a `NO_SELECTABLE_TEST` entry naming it; named packages are reported
separately (`namedNoSelectableTest`) and are not counted as selected. The corpus, jobs, baseline,
thresholds and exclusions are unchanged. The amended preregistration seals harness sha256
`4a27e8ef8b753b9c87b4d5d86da77aefe95343bd1dc980f59fdc00aa7d4c6960` (was `17b41f50…2d242`) and
candidate `d3c8d0f1fceecbc66cf29c375cc16b122e8aa2fe` (was 1894b9e); its own sha256 is
`ee94b7ca16c3f3a88ffe253483bdeb0d0f21c1a7de710126244dcec34f20dc96` (was `73f178f8…04b9d`).
Under the invalidation rule a changed harness is a new numbered run; run-001 and its receipts stay
as recorded.

Sealed run-002 (`benchmarks/daily-loop-v0/runs/run-002.json`, sha256
`e1ccb1e6bf960020e2b8c662c18f02612f7e3df8f0ad0a48916cb37b20ca9af4`) used candidate binary
`15ec2fb02a52ddeb779d2079c953098a3c25553a6db6e6bbc48f9842c1fc476e` built from d3c8d0f, three runs
per measurement, rehearsal target a64624c. Its three sealed-benchmark receipts under
`benchmarks/daily-loop-v0/receipts/run-002/`:

| Receipt | sha256 | Result |
| --- | --- | --- |
| `UC-TASK-ORIENTATION.json` | `44eebef9960a2ec362e104cf43210af4a02ba19ce24194546f8ebe90817c1f77` | PASS |
| `UC-CHANGE-CONSEQUENCE.json` | `208de893cb83a3d15cb39fb38b53901b33bba5158f08b4e10230d5ece8fbe7ab` | PASS |
| `UC-EVIDENCE-CARRYING-COMPLETION.json` | `ae6ece856092a08b737d938ab6f87e36939281316a590809859388b4c1138471` | PASS |

- **Consequence, PASS (C2).** Three scored cases, zero abstentions, zero treatment-only critical
  misses. The five run-001 misses are now named: `internal/cem/coverprofile` and
  `internal/cemdiscriminate` at 50a9647, `cmd/corvint-test-validity-mcp` and
  `integrations/testfixture` at 362721c, and `cmd/corvint-web-flows` at af247a1, each as a
  `NO_SELECTABLE_TEST` unknown and nothing else; every scope is `UNKNOWN` as AFP-V0-004 requires.
- **Orientation, PASS (C1).** Six scored cases, zero abstentions, zero treatment-only critical
  misses. The run-001 miss (`docs/AGENT-ROUTES.md` at 1d4cbaa) is resolved by the V1-0186 fix
  (PR #110), which is in the candidate.
- **Completion, PASS (C3).** 36 designated cases, 0 false complete verdicts, 0 positive-control
  refusals, unchanged from run-001.

Control (unsealed, amendment 1 `control`): the same harness with `candidateCommit` 1894b9e (the
run-001 candidate) and preregistration sha256
`e7d4c3ff15dd423c2e8945bf1120aa11a375c5a9af285dd352d3568e1820eb4e`, run-id `control-prefix-1894b9e`,
result sha256 `95e4eca99a7cc23f0b51f943e5564100c13fa8ceb13535078218c2fc8ea74733`, kept under
`~/projects/corvint-release-evidence/daily-loop-control-1894b9e/`. It records consequence FAIL with
the same five treatment-only misses as run-001 (none named as unknown), orientation FAIL with the
same one miss, and completion PASS. So the amendment does not score the pre-fix build as covered:
the PASS comes from AFP-V0-020, not from the rule change.

A first execution of run-002 was discarded: it passed a sibling clone as `--repo`, so its receipts
carried subject paths relative to that clone (result sha256
`68e7edc18bff8ccfa9bc11309bb736930f35efa240f6f00d0b604f032150613b`, same three PASS outcomes). The
retained run-002 is the second execution with the harness's own checkout as `--repo`, as run-001
was.

Ledger: `conformance/use-cases-v0/ledger.json` binds the three run-002 receipts as
`sealed-benchmark` evidence on the three Core rows (`UCV0-003`, `UCV0-007`); the validator reports
`valid: true`. The rows stay `experimental` and `UNPROVEN`: no `corvint-dogfood` or
`beamfall-dogfood` receipt exists (V1-0184, V1-0011). Complete task cost and human failure rate stay
`NOT_OBSERVED`; the result reads "measured, no savings claim" (PCCO-V0-017). Full gate: `NOT_RUN` by
owner policy; the V1-0202 ticket's `full-gate` requirement is left to the candidate head.
## 2026-09-23 V1-0188 follow-up: the dirty-package-without-tests hostile case runs

The V1-0188 hostile receipt for `UC-CHANGE-CONSEQUENCE` recorded one skipped case,
`hostile/dirty-worktree/dirty package without tests is named`, because a dirty Go package with no
test files was silently omitted from the plan (the V1-0187 defect). V1-0187 merged in PR #108
(343368d, `AFP-V0-020`, `NO_SELECTABLE_TEST`), so the skip is removed and the case asserts what the
receipt already described: scope `UNKNOWN` with `util` named in the unknown list.

- `cmd/corvint/usecase_hostile_change_consequence_test.go`: the `t.Skip` line is deleted; nothing
  else changes. The case now fails if the omission returns.
- `conformance/use-cases-v0/receipts/UC-CHANGE-CONSEQUENCE/hostile-tests.json` is repinned to the
  test commit (`repositoryRevision` 6b601a9, subject sha256 043337bb…) and the ledger row's receipt
  digest follows (ce42c53f…). The attestation cases, result and the row's `experimental` /
  `UNPROVEN` status are unchanged: a running hostile case is not a dogfood or benchmark receipt.

Verification (this clone, HEAD after the test commit):

- `go test -count=1 -run TestUseCaseHostile ./cmd/corvint/`: exit 0; the change-consequence
  table reports 18 `PASS` cases and no `SKIP` (the fixed case among them).
- `go test -count=1 ./conformance/use-cases-v0/`, `go run ./conformance/use-cases-v0 -root .
  -ledger conformance/use-cases-v0/ledger.json`, `go vet ./cmd/corvint/
  ./conformance/use-cases-v0/`, `go test ./internal/specindex/` and
  `make spec-requirements-check requirement-definitions-check traceability-tests-check
  decision-numbers-check line-citations-check`: exit 0.

NOT_RUN: `make gate` (owner focused-verification policy). Criterion 2 of V1-0188 stays NOT_RUN.

## 2026-09-23 v0-5 candidate recaptured at 5957c5f with the one authorized candidate-head gate run; V1-0202 to V1-0206 filed

The owner authorized one `make gate` run at a candidate head (decision 0373, answer C). It ran in a
clean clone at c25f8a0ae009c97ad500adafb3cb2f33e160f2c5 (the PR #107 merge). The task store at that
commit lacks the PR #109 mutations, so `release candidate` could not bind it; the candidate is bound
at 5957c5f4f3509947c5aa383360bd720f617190d9 instead, whose tree is byte-identical to c25f8a0 outside
`.taskman/` (`git diff --stat c25f8a0 5957c5f -- . ':!.taskman'` is empty; `.taskman/policy.json` is
identical, `.taskman/queue.json` differs by one line). The only gate step known to read the store is
the companion-release smoke fixture, which takes the queue and policy projections. The previous v0-5
candidate at e0d80d2 was reported stale (`candidate-source-or-policy`) and is superseded; its six
attestations stay in the journal. Store revision 9 to 16.

| Gate | Result | Exit | Evidence (sha256 of the log) |
|---|---|---|---|
| `full-gate` | PASS | `make gate` 0, 228 packages ok | 34dd93e50196dfbc94a238464e11bcf9fcdb7540fc4d9ec779b77f1a477bbceb |
| `interop-gate` | PASS | `make interop-gate` 0 | 070759c2e77a7fb2cba49f169c0db23c71dfe5186c67ccee819bc112e95b349a |
| `focused-docs` | PASS | five docs checks 0 | bc400ee07cb6c032d5a989fcfc195b9b77c043c8d77af0d588db09c9946a8cb2 |
| `companion-release` | PASS | `make companion-release-gate` 0; browser/UI qualification not claimed | 61e8ed83cfe415b3b12784d4ccd68fe016097b70cb180dbaa016f93390831247 |
| `public-release` | FAIL | `make public-release-check` 2: `CORVINT_RELEASE_BUNDLE_DIR` unset; the core bundle inputs are unrecoverable | f17ac2ac07ae195a6cd830e935d276a191d622bca7147cd8b8e0bc0d8e3936e1 |
| `release-checklist` | FAIL | `script/release-checklist` 1: `tag` FAIL (`ARTIFACT-RDY-V0-003`, the V1-0189 structural pre-promotion exit); `publication`, `promotion` and `native-performance` NOT_RUN | b909e64c1786af2c436bac2f5a3a392353d530107152268ae647adffc3a0636a |

The logs are retained outside the checkout at `~/projects/corvint-release-evidence/v0-5-5957c5f/`.
`release readiness v0-5` is `BLOCKED` on `gate:public-release` and `gate:release-checklist`;
`nativeGateExecution` is `NOT_RUN`. Promotion is an owner action and is not claimed. A v0-6 candidate
was attempted and refused (`predecessor not promoted`), which is the expected chain order.

Follow-up tickets filed from the V1-0186 and V1-0187 reviews: V1-0202 (v0-6, P1: the daily-loop
harness must score `plan.unknown` `NO_SELECTABLE_TEST` entries before the V1-0187 criterion 2 re-run),
V1-0203 (`testdata/` directories indexed as Go units), V1-0204 (changed untested non-Go units still
omitted silently), V1-0205 (idf floor for `instruction-routed` matching, re-measure the recall@5 and
recall@10 dips), V1-0206 (untested TCP-V0-047 cases and the fenced-code-block path question).

## 2026-09-23 V1-0188 UCV0-006: hostile-tests receipts for the three Core use cases

Criterion 1 of V1-0188. One table-driven, entrypoint-level hostile test file per Core use case,
with subtests named `<category>/<class>/<case>` so the UCV0-006 categories `negative`, `hostile`
and `abstention` are visible per use case. The tests were committed first (`5eddd496`). Each
`hostile-tests` receipt pins a `repositoryRevision` and the `shasum -a 256` of its test file at that
revision, and each attests `cases` `["abstention","hostile","negative"]`. After independent review,
UC-TASK-ORIENTATION and UC-CHANGE-CONSEQUENCE pin 1c6451f4aab403bbc59d8ba2bbb2416f9d57638e;
UC-EVIDENCE-CARRYING-COMPLETION keeps 5eddd496edf240997ed6af58501a31bf9cdfa320, because its test file
did not change.
The ledger rows gain a `hostile-tests` evidence entry and stay `experimental` with claim `UNPROVEN`.
The `go run ./conformance/use-cases-v0` validator reports `valid: true` with 9 evidence references.
No product code changed.

UC-TASK-ORIENTATION (`corvint context`, `corvint query`), in
`cmd/corvint/usecase_hostile_task_orientation_test.go`, receipt
`conformance/use-cases-v0/receipts/UC-TASK-ORIENTATION/hostile-tests.json`:

- Stale index is hostile, a miss:
  - A snapshot behind HEAD, garbage `.gob` bytes, or a symlinked `.corvint/index` each give context
    output byte-identical to a cold build, with `revision` equal to the HEAD tree.
  - For the stale and corrupt snapshots, query output is byte-identical to a cold build as well.
  - For the symlinked snapshot, query labels `.corvint/index` as a `mixed-worktree` path. Git sees
    the symlink as untracked.
- Dirty worktree:
  - Hostile: context cites only committed blobs and never a worktree-only definition.
  - Hostile: query reports `freshness.state` `mixed-worktree` and names the dirty path.
  - Abstention: `unindexed-worktree-changes`.
- Missing anchors:
  - Negative: an untracked or absent `--subject` gets exit 2 with
    `subject path is not tracked at revision`.
  - Abstention: query reports `no-relevant-candidates`, and context reports answerability
    `no-specific-terms`.
- Malformed provider records are hostile and degrade: a `gopls` that emits a truncated LSP frame gives
  one `external.providers` row with state `unavailable`, reason `gopls session failed`, and no
  results.
- Symlinked or relocated subjects:
  - Negative: `../outside.go` is refused with `impact path must be normalized and
    repository-relative`, exit 2.
  - Negative: an uncommitted `git mv` target is refused as not tracked.
  - Hostile: a tracked symlink subject is pinned to the link's own blob, and its `reference`
    relation is `subject-symbols-incomplete`.
- Skipped: none.

UC-CHANGE-CONSEQUENCE (`corvint affected`), in
`cmd/corvint/usecase_hostile_change_consequence_test.go`, receipt
`conformance/use-cases-v0/receipts/UC-CHANGE-CONSEQUENCE/hostile-tests.json`:

- Stale index is hostile:
  - A stale `.corvint/index` snapshot leaves the plan byte-identical. `corvint affected` never reads
    the snapshot, so this case documents that independence rather than a refusal.
  - A provider record pinned to a superseded revision gives `full-relevant-suite-required` with
    `stale-provider-revision`.
- Dirty worktree:
  - Hostile: a dirty edit selects its dependency closure while scope stays `BOUNDED`.
  - Abstention: a deleted source gives scope `UNKNOWN`, with `UNINDEXED_SOURCE_PATH` and the
    exclusion `UNINDEXED_DIRTY_GO_PATH_MAY_BE_DELETED_OR_RENAMED`.
- Missing anchors:
  - Negative, AFP-V0-006, exit 2: a base that is not a commit and a repository without HEAD both
    give `unsupported-affected-revision`.
  - Negative, AFP-V0-006, exit 2: an abbreviated base gives `invalid-arguments`.
  - Abstention: a clean tree reports `NO_REPOSITORY_GATE_DECLARED` and `NO_ADVISORY_GO_COMMAND` with
    no checks.
- Malformed provider records are hostile:
  - A truncated record or a foreign schema gives `blocked` / `provider-unavailable` with
    `provider-invalid`.
  - An unparseable `go.mod` gives `UNKNOWN` with `LANGUAGE_FRONTIER go:module-path-unresolved` and
    `MODULE_PATH_UNRESOLVED`.
- Symlinked or relocated subjects:
  - Hostile: a record naming a relocated test path gives `full-relevant-suite-required` with
    `missing-path-reference`.
  - Hostile: a record naming `../outside_test.go` gives `unresolved-endpoint`.
  - Hostile: an untracked file symlink gives `UNKNOWN` with `UNINDEXED_SOURCE_PATH`.
  - Hostile: an untracked directory symlink gives `UNKNOWN` with `UNOWNED_DIRTY_PATH`.
  - Hostile: a committed symlink to an outside directory is not walked. The outside directory holds
    `extdir_test.go`, so a walked directory would enter the plan: replacing the symlink with a real
    directory holding the same files makes the case fail with `go:example.com/g/extdir` excluded.
  - Abstention: an uncommitted `git mv` gives `UNKNOWN`. The move destination appears in no plan
    list; that is part of the V1-0187 defect and is not asserted here.
- Skipped (defect, not fixed here): `hostile/dirty-worktree/dirty package without tests is named`.
  A dirty Go package with no test files leaves scope `BOUNDED` and is absent from the selected,
  excluded and unknown lists. That is a silent omission. The unmerged V1-0187 branch fixes it
  (AFP-V0-020, `NO_SELECTABLE_TEST`); remove the skip when that branch lands.

UC-EVIDENCE-CARRYING-COMPLETION (`corvint cem`, `corvint dogfood`), in
`cmd/corvint/usecase_hostile_evidence_completion_test.go`, receipt
`conformance/use-cases-v0/receipts/UC-EVIDENCE-CARRYING-COMPLETION/hostile-tests.json`:

- Stale index is hostile, a refusal: a candidate map followed by a later commit gives
  `cem status` exit 1, state `invalid`, `patch-digest-mismatch`.
- Dirty worktree:
  - Hostile: a worktree edit leaves `cem status` byte-identical, because status is derived from
    commits.
  - Hostile: `cem anchor` on a dirty committed map gives exit 2 `anchor-map-dirty`.
  - Negative: on an uncommitted map it gives `anchor-map-uncommitted`.
  - The `dirty-worktree` refusal of the dogfood loop lives in `script/dogfood-check.sh`, not in the
    Go CLI. The test comment records this.
- Missing anchors:
  - Negative: a citation of an absent evidence path gives `missing-evidence`, exit 2.
  - Negative: a wrong `--expected-base` gives exit 1, `invalid`, `base-revision-mismatch`.
  - Abstention: an uncited hunk counts `unknown` 1 and `supported` 0, and a zero-unknown policy
    fails with `max-unknown-exceeded`, exit 1.
  - Abstention: an unenrolled `dogfood status` reports lifecycle `inactive` and `satisfied` false.
- Malformed provider records (the CEM map):
  - Negative: `cem/9.9` gives `unsupported-spec`.
  - Hostile: a truncated map or a duplicate key gives `invalid-json`, exit 2.
- Symlinked or relocated subjects are hostile, and each gives exit 2:
  - A committed symlink evidence path gives `missing-evidence`.
  - A symlinked `--map` gives `cannot read CEM map`.
  - A symlinked `.git` gives `repository-object-unavailable`.
  - A map relocated under a subdirectory `--root` gives `repository-object-unavailable`.
- Skipped: none.

Spec: the `use-case-conformance-v0.md` status paragraph now records the `hostile-tests` receipts. No
requirement was added or renumbered; `REQUIREMENTS.tsv` was regenerated for the line shift.

Verification was focused, per the owner's policy for scoped work; `make gate` is NOT_RUN. Results
after the review fixes:

- `go test -count=1 -run 'Hostile|UseCase' ./cmd/corvint/`: exit 0, 46 use-case subtests pass and 1
  is skipped.
- `go test -count=1 -timeout 30m ./cmd/corvint/ ./conformance/use-cases-v0/` (before the review):
  exit 0.
- `go vet ./cmd/corvint/ ./conformance/use-cases-v0/`: exit 0. `gofmt -l cmd conformance`: no files.
- `go run ./conformance/use-cases-v0`: exit 0, `valid: true`, `useCaseCount` 22, `evidenceCount` 9,
  `experimental` 3, `specified` 19, `verified` 0, `UNPROVEN` 22.
- `go test -count=1 ./conformance/use-cases-v0/ ./internal/specindex/`: exit 0.
- `make spec-requirements-check requirement-definitions-check traceability-tests-check
  decision-numbers-check line-citations-check`: exit 0.

Criterion 2 of V1-0188 is NOT_RUN. The three rows stay `experimental` and `UNPROVEN`, with no
`corvint-dogfood` or `beamfall-dogfood` receipt. The branch is stacked on
`claude/v1-0001-scope-ratified` (8d6c5fa), because the rows and receipts it extends exist only
there and not yet on `origin/main`.

## 2026-09-23 V1-0186 TCP-V0-047: governing instructions route task orientation

Cause (daily-loop run-001 UC-TASK-ORIENTATION critical miss): for "move the agent-memory backlog into
the Corvint task store" at 933b4306, `docs/AGENT-ROUTES.md` is a lexical `documentation` hit (seven
distinct terms, fifth documentation row) but `lexicalRows` (TCP-V0-013) places at most two
documentation rows after the five-row code head and then every remaining code row (about 1,630)
before the rest, so it sits far outside `--limit` 20 or 50. `AGENTS.md`, the governing row, names it
in the "Agent routing" passage that shares `agent`, `backlog`, `memory`; no reservation read it.

Fix: `instructionRoutedRows` reserves up to two `instruction-routed` rows after the `spec-mentioned`
rows: tracked, backtick-quoted paths the governing file names in a passage (paragraph or list item)
sharing at least two distinct task terms, strongest passage by summed idf first. Authority
`instruction-reference`; reason "named by the governing instructions for this task: FILE:LINE shares
...". Analyzer schema moves to `corvint-analyzer/76`; both context goldens gain only the
`instruction-routed` `unexamined` entry. Tests: `TestTaskContextRoutesPathsTheGoverningInstructionsNameForTheTask`
(positive, one-term negative, byte identity) and `TestTaskContextCapsInstructionRoutedRows`.
Reproduction: `docs/AGENT-ROUTES.md` is result 2, reason cites `AGENTS.md:90` sharing `agent`,
`backlog`, `memory`; `docs/specs/INDEX.json` is result 3 (`AGENTS.md:91`).

Frozen `tools/retrieval-bench` v2, `--arms context`, all samples, `CORVINT_CONTEXT_*` unset, base
062b0151 then fix (recall@20 / @10 / @5): code2test 0.5116/0.3994/0.2877 then 0.5116/0.3994/0.2830;
comment2context 0.5042/0.3438/0.2562 unchanged; trace2code 0.7937/0.5083/0.4010 unchanged;
edit2ripple 0.6293/0.5101/0.3621 then 0.6293/0.4928/0.3448; abstention rate 0.1707 unchanged. Ten of
427 packets changed (transformers, eslint, caddy); no sample lost recall@20, one edit2ripple sample
lost recall@10. Accepted cost: code2test recall@5 0.2877 to 0.2830, edit2ripple recall@10 0.5101 to
0.4928 and recall@5 0.3621 to 0.3448; recall@20 is flat on every subset, which is the ticket's
closing rule. The likely cause is that the passage match has no idf floor (any two shared task
terms route, however common), filed as a follow-up. Retained reports (with their
`.registration.json` sidecars):
`/private/tmp/claude-501/-Users-russelllewis-projects-corvint/dd54e7f8-328f-4e5e-ac2a-20e9a455cd73/scratchpad/w0186-bench-reports/w0186-bench-{base,fix}-{code2test,comment2context,trace2code,abstention,edit2ripple}.json`.
Review follow-up: `coreSpans` (TCP-V0-025) now skips `instruction-routed` rows like the other
reserved rows (`TestContextSpansSkipInstructionRoutedRows`). Criterion 2, the sealed daily-loop
re-run: NOT_RUN (not this ticket's step).

## 2026-09-23 V1-0187 AFP-V0-020: changed Go packages without tests are unknown scope

Defect: `affected.Select` skipped every unit with no tests before checking whether traversal reached
it, so a changed Go package without a `_test.go` file appeared in none of `selected`, `excluded`,
`unknown`, and the plan stayed `BOUNDED`. All five `treatmentOnlyCriticalMisses` in
`benchmarks/daily-loop-v0/runs/run-001.json` are such packages. Fix: a changed `go:` unit (one that
owns a changed path) with no tests now adds `plan.unknown` `{reason: NO_SELECTABLE_TEST, detail:
<unit id>}`, which makes the scope `UNKNOWN` under AFP-V0-004; an untested dependent of the change or
an unreached unit adds nothing. Review correction: the first commit fired for every reached untested
unit, which added unrelated `cmd/` and testdata packages to a change's unknowns (4 for `--base
062b0151`, 13 at run-001 commit `50a96470`). The rule is Go-only: applied to
every plugin it made about 30 existing tests fail across six plugins (TypeScript, Ruby, Kotlin, Rust,
.NET, Playwright), because those plugins model sources and tests as separate units. Every
TypeScript source edit, for example, would have gone `UNKNOWN`. Tests:
`TestSelectNamesChangedUntestedGoPackageAsUnknownScope`,
`TestSelectTraversesUntestedUnitsWithoutSelectingThem` (non-Go stays bounded),
`TestAffectedUntestedGoPackageIsUnknownScope`. A build at the corrected fix names exactly the five
misses as `NO_SELECTABLE_TEST` at their three run-001 commits (2, 2 and 1), and nothing else.
Remaining gap: an untested non-Go source unit that no test unit reaches is still omitted. Criterion 2, the sealed daily-loop re-run and
re-score, is NOT_RUN here. `harness.py` `consequence_case` scores only `plan.selected`, so the
re-score has to read `plan.unknown` as well.
## 2026-09-23 V1-0001, decision 0373, PRS-V1-001..012, V1-0011 UCV0-003, V1-0088 decision 0368: 1.0 scope ratified, invariant-4 amendment applied, Core ledger rows experimental

The owner answered the eleven 1.0-scope questions of `docs/specs/corvint-1.0-product-and-release-v1.md`
yes on 2026-09-23 and delegated the five open 0.7 dispositions. Decision 0373 records the answers, the
delegated dispositions (items 12 to 17) and their effects; the spec status moves to accepted and every
"owner question 5" placeholder now reads "Core (decision 0373, question 5: yes)". Ticket V1-0001 is
completed against the decision file (sha256
`010092b1b13dcdc85e13794da51cd19d6f2b6198b66e794524a022351d0f8bd0` after the review fixes below;
ticket revision 6). Scope consequences applied in
this change: `docs/specs/public-release-v0.md` gains a "1.0 Core scope amendment" section (0.6 scope
extends to the 1.0 Core candidate; the FULL clause and the PUB-V0-004/005/006/009/010/020/022 inputs
bind the companion host profile; installed Codex and Claude Core rows stay FALLBACK; no
interoperability claim, V1-0014 is post-1.0); V1-0019 becomes MANUAL with one owner-selected untouched
public repository; V1-0021 and the v1-0 criterion replace "independent interoperability passes" with
the frozen wire's canonical vectors plus the in-repo second consumer, with no interoperability claim;
V1-0015 keeps its recorded V1-0014 dependency (the ticket is COMPLETED, so `set-dependencies` is
refused with TICKET_STATE and the dependency is moot); v0-7 drops V1-0014, whose milestone becomes
the label `post-1-0` (the store refuses an empty milestone), and its proof-wire criterion now reads
"versioned and usable from a digest-pinned CI verifier through its canonical vectors and the
in-repo second consumer; no independent interoperability claim" (release revision 6).

Delegated dispositions. Item 12: decision 0368's invariant-4 amendment is ratified and applied to
`AGENTS.md`, `SOL-V0-003`, the `unplanned-read-events-v0` non-goals, the `reads` help text and the
`unplannedread` package comment: the ledgers reach ranking only through the operator-invoked
`corvint eval --learn-slot-weights` step and the frozen held-out gate (`LTA-V0-009` to `LTA-V0-012`);
V1-0088 was completed as experimental delivery before this ratification and the 0368 status line says
so. Item 13: V1-0083 (decision 0367, identifier PageRank) did not meet its closing rule, so 0367 is
accepted as experimental and off by default, the ticket is archived as not delivered and removed from
v0-7, and the v0-7 state-of-the-art criterion now says items failing their frozen evaluation are
archived as not delivered; the follow-ups (score-competing graph, smaller cap, test-file edges, the NOT_RUN full-sample
rerun) are filed as ticket V1-0185. Item 14: V1-0023 compact summaries move to v0-8 with spike V1-0154;
the v0-7 summaries criterion now reads that summaries ship experimental and opt-in with the trial
deferred to 0.8. Item 15: V1-0100 appflows is completed as delivered-experimental (revision 5)
against its last ticket criterion and the v0-7 application-flow criterion; its first criterion is
amended to record the browser gate on base a98d770, the sealed binding and the full gate as NOT_RUN
until the candidate head, and its evidence digests are the decision file and
`docs/specs/application-flow-understanding-v0.md`; its acceptance packet is NOT_PRODUCED. Item 16:
the three Core rows of `conformance/use-cases-v0/ledger.json` (UC-TASK-ORIENTATION,
UC-CHANGE-CONSEQUENCE, UC-EVIDENCE-CARRYING-COMPLETION) move from `specified` to `experimental` on
`contract` and `implementation` receipts pinned to `062b0151b603a5d637cdd0ad0f560edefee921a4`
(`corvint-use-case-evidence/0`, digests: orientation contract `4bbed299…c8fe`, implementation
`76adf551…eb5d`; consequence `fe946089…d968`, `bd61d670…cb75`; completion `01a4daf8…0cbb`,
`2505a414…27dc`). Validator: valid, 22 use cases, 3 experimental, 19 specified, 6 evidence receipts,
ledger sha256 `2f80c0bf5721ef827c43180e3bb113615c713070c35edb05d18b8c065e0ba559`; every claim stays
UNPROVEN. `verified` was not reachable: daily-loop run-001 records FAIL on two Core jobs, no
Beamfall dogfood receipt exists and no hostile-tests receipts exist, so V1-0011 stays open and the
four gaps are filed as tickets V1-0186 (orientation miss of `docs/AGENT-ROUTES.md`), V1-0187 (`affected`
naming test-less changed packages as unknown scope), V1-0184 (a Beamfall daily-path receipt) and V1-0188
(hostile-tests receipts for the three jobs). `TestUCV0ProfileMigration` now validates the in-tree ledger and strips evidence before its
temp-root checks. Also filed: V1-0190, the 0.7.0 to 0.8.0 N-1 `upgrade-b` qualification for 0.8 (owner answer 19), and
V1-0189, the structural non-zero exit of the release-checklist gate before promotion (decision item
17).

Independent read-only review of the first commit (3ab6c56) returned FIX-FIRST with twelve findings,
all applied in the review-fix commit: `REQUIREMENTS.tsv` had not been regenerated, so
`spec-requirements-check` failed at that commit although this entry claimed it passed; the v0-7
proof-wire criterion still required independent interoperability and V1-0014 still carried the
v0-7 milestone; criteria were cited by ambiguous ordinal instead of text; owner answer 19 and
tickets V1-0189/V1-0190 were missing from the decision; V1-0100 was COMPLETED against an unmet
first criterion and an unnamed evidence digest; the 1.0 spec still read as a draft in five places;
the decisions index lacked a 0373 row and kept 0367/0368 as proposed; the `URE-V0` amendment
history contradicted `AGENTS.md`; `LTA-V0-009` to `LTA-V0-012` stayed marked proposed; two
`public-release-v0.md` nits; one unwrapped `AGENTS.md` line. Verification after the fixes:
`go run ./conformance/use-cases-v0`, `go test ./conformance/use-cases-v0/ ./internal/unplannedread/`
and `go test ./cmd/corvint/` in a clean clone (the same test fails in the primary checkout only
because of an uncommitted local `plugin.json` version edit), `go vet` on the touched packages and
the focused docs targets (`spec-requirements-check requirement-definitions-check
traceability-tests-check decision-numbers-check line-citations-check`) pass. `make gate` NOT_RUN
for this change; the authorized single gate run is scheduled at the v0-6/v0-7 candidate head. Rollback: revert the change,
reopen V1-0001, V1-0083 and V1-0100, restore the v0-7 ticket list and criteria, and return the three
ledger rows to `specified`.

## 2026-09-23 V1-0004 PUB-V0-016, PUB-V0-020: core wrapper mode and first PASS retained core run

`script/public-release-check` selects `editor` (unset) or `core` via `CORVINT_PUBLIC_RELEASE_QUALIFICATION`;
core forwards `CORVINT_NODE_SHA256`, `CORVINT_NPM_SHA256`, `CORVINT_GO_AUTHORITY_BUNDLE`, `CORVINT_GO_AUTHORITY_SHA256`
as the four core flags, editor refuses them, and empty or unknown selection exits 2 before effects.

Attempt 1 at commit `4b9957ed` (bundle/2 `07d5245b...f04a6`) completed offline-dependencies, providers,
provider-interruption and docs, then failed planning: `script/seed-planning-store-data.json` pinned
`22298b8d...`, a digest no committed version of `docs/plans/integrated-product-roadmap-2026-09-12.md`
ever had. The pin was set in 405d63d (the 0.6.0 integration) while that commit rewrote the roadmap's
Outcome paragraph and next-action heading (file digest `cd658a3e...`), so the seed step had been broken
since 405d63d; b228c33 (V1-0002) later added only the two-line superseded header (now `d3b7a8d0...`).
`seed-planning-store.sh` refused, console was NOT_RUN and no result file was produced. The eleven IPR
sections are unchanged across both commits, so commit `72a16ef1` repins the digest (its message names
b228c33 alone; the review of PR #104 corrected the provenance); `seed-planning-store_test.sh` passes
against the bundle `corvint-tasks`.

Attempt 2 at commit `72a16ef1016a9ed4b8f9481e05fe09ea9857a202`, tree `d6e93e58d5a9f648656af0d4259992bde56f4d87`,
bundle/2 `d84773d58848ffaaecd7c7d4a7b2ec75616aaea117a959657464baa470085bc8` (Tasks `e6b9d76`): `status: PASS`,
profile `corvint-public-release-core-installed/0`, result sha256
`aa86e5552c6ea2eed5fc6aeadeaf4f1839b40368f8222b073c88b01c26265b2d`, identity `ac9e3917...320f7`. All six
stages completed (offline-dependencies, providers with 28 receipts, provider-interruption, docs, planning with
the eleven-ticket roadmap read back through `corvint-tasks`, console). Inputs: a 16 MB npm cache seeded from the
fixture lock (`04aea4de...49c2c`), a browser cache of only `chromium_headless_shell-1243` (`9edf5670...3f98`,
Chrome for Testing 153.0.8010.12), node 22.23.2 `0143ba3f...cc1a1e`, npm 10.9.8 `8e5f6f34...fcbe7`,
`/usr/bin/python3` `b8763cf2...f610e9`, go1.27.1 `548608a9...8509b` and a caller-measured trusted-local 0600 Go
attachment `d2fe8448...5dc98` (verifier `1b897c55...61704`). The result and its `SHA256SUMS` are retained
locally (the repository names no committed location); the checker removed its scratch on success.
NOT_RUN: `make gate`; the other four platforms of the companion matrix (no attempt made). This run qualifies
the commit above, not a release candidate: a candidate head needs its own run.

## 2026-09-23 V1-0088, decision 0368, LTA-V0-009..012: ledger negative labels for slot weights

V1-0088 adds an explicit, operator-invoked learning step on an existing verb:
`corvint eval --learn-slot-weights [--goldens FILE] [--admit]`, with the rollback
`corvint eval --reset-slot-weights`. The step reads the unplanned-read and self-observation ledgers
through their existing bounded readers as negative labels. Distinct paths are capped at 256, and
planned re-reads are counted but not labelled. It proposes at most four slot-weight traces within
-2..2 and scores each against the default slot order on the frozen golden's held-out `query` rows
through the `context` packet. It admits a proposal to `.context-corvint/slot-weights.json` only with
`--admit` and an `improved` delta. `context` and `query` never open the ledgers, and the default
packet bytes are unchanged when no admitted file exists.

Decision 0368 is `proposed`. The ticket conflicts with AGENTS.md invariant 4, `SOL-V0-003` and the
`URE-V0` non-goals, which say the ledgers are never a learning input. The decision proposes the
amendment text for owner ratification and edits none of those documents. Delivery stays
experimental until the owner rules.

First run on this repository, at implementation commit `fa6367b` (tree `d6d5f3b7`) with goldens
`testing/context-retrieval-goldens.json` (`sha256:d4ae96df2a3bf01d05a44a848894390c915b8ea34b93cabba9ef2a31a4ad9392`):

- Real ledger: the main checkout's `.corvint/self-observations.jsonl` (58 rows,
  sha256 `ea276e6b…d0e6d`), copied into the clone's ignored `.corvint/`. The main checkout has no
  unplanned-read ledger (the marker is off). The run found 0 label paths (0 `OBSERVED` misses and
  0 unplanned paths) and refused with `no negative labels`; nothing was scored or written.
- Synthetic labels, labelled as synthetic and not observed data: four hand-written unplanned rows
  (three test paths, one Go source path). Two held-out cases scored. The baseline had 0 critical
  misses, 0 must-include hits and 0 top-five hits. All four proposals (`test` +1/+2,
  `definition` +1/+2) were `not distinguished`, with 0 improved and 0 regressed cases. The run refused
  with `no held-out improvement`, even with `--admit`, and wrote no file.
- Why every arm is zero: the frozen golden describes the Atlas fixture, not this repository. Of its
  two held-out `query` rows, one carries only `feature:`/`scenario:` selectors, which have no
  path-bearing form in a `context` packet. The other's four `symbol:` selectors all name
  `internal/auth/auth.go`, which does not exist here. With this golden the gate cannot admit any
  proposal. A useful admission needs a golden whose held-out `symbol:`/`file:` rows name this
  repository's paths.

Known mismatch: the adapter prompt packet that feeds the ledgers is not the `context` compile() packet
being weighted. That is why the held-out gate, not the labels, decides admission.

Verification: `corvint affected --base a98d770` selected 63 packages (scope UNKNOWN, LANGUAGE_FRONTIER
unknowns). `go vet` on them exited 0. `go test -count=1 -timeout 30m` on them passed 58 packages and
failed 5 under a loaded host. Reruns sorted the failures:
- `internal/contextindex` `TestAnalyzerSchemaInputs` was caused by this change: the new
  contextindex sources are audited inputs. Fixed by bumping `analyzerSchemaID` to
  `corvint-analyzer/74` and repinning both audit pins (moved to `corvint-analyzer/75` when merged with main, which took `/74` for decision 0367).
- Passed when rerun alone: `cmd/corvint` `TestExperimentalKernelAdapterContext` and
  `TestHostAdapterJavaScriptHosts`, `internal/playwrightminimize`
  `TestPSMLiveResetFailureStillCleansUp`, and `benchmarks/selfuse-batch`
  `TestRealNativeBatchFallbackAndReadOnlyParity`.
- Pre-existing: four `cmd/corvint-go-test-provider` tests
  (`TestProviderCommandUsesPinnedLiveParentAuthorityE2E`,
  `TestProductionParentBindsNonASCIIRepositoryPath`,
  `TestExecute{Normal,Interrupted}...HasNoRecordedSurvivors`). They fail identically at BASE
  `a98d770`, and the package does not import any changed package.

After the bump:
- The `LTAV0` and `TestAnalyzer*` tests in `internal/slotlearn`, `internal/evalrepo`,
  `internal/contextindex` and `cmd/corvint` pass, and `go vet` passes.
- The focused-docs gate (`make spec-requirements-check requirement-definitions-check
  traceability-tests-check decision-numbers-check line-citations-check`) exits 0.

NOT_RUN: `make gate` (owner policy); the exhaustive `go test ./...`; independent review; a frozen
external retrieval benchmark (not requested). NOT_OBSERVED: any admitted trace on real labels.

Review fixes (independent review of PR #92, three confirmed defects). (1) `batch`'s `context`
operation now loads the admitted file and calls `TaskContextWeighted`, so it stays byte-equal to
standalone `context` under weights (`SBQ-V0-003`, `TestBatchContextAppliesAdmittedSlotWeights`, which
fails without the fix); `necessity`, `disagree` and `touchsurprise` stay unweighted, now stated in
`LTA-V0-011`. (2) Prose labels (`.md`, `.mdx`, `.rst`, `.txt`) map to `documentation`, the kind the
packet emits for them, and `documentation` joins the learnable slots; other `docs/` paths stay
`lexical` (`LTA-V0-009`). Default packets are unchanged because an empty weight map is the identity
order; the whole `internal/contextindex` suite passes. (3) The loader refuses an absent, non-object
or incomplete `evaluation` block (`goldens_sha256`, a 40- or 64-hex `revision`, `baseline` and `arm`
results); the gate now writes both arm results. The file is operator-owned and the loader checks
shape, not provenance (`LTA-V0-011`). `context --help` names `learned_slot_weights`, one loader test
is renamed to `TestLTAV0011...`, and the analyzer audit digest (`/75` after the merge with main) is repinned for the changed
`slot_weights.go` bytes. No extraction or encoding changed. Verified: the focused-docs gate exits 0;
`go test` of `internal/slotlearn`, `internal/evalrepo` and `internal/contextindex` passes;
`cmd/corvint -run 'TestLTAV0|TestAnalyzer|TestEval|TestBatch|TestSBQ|TestTaskContext|Help'` passes;
`go vet` and `gofmt -l` are clean. Still NOT_RUN: `make gate` and the exhaustive `go test ./...`.

## 2026-09-23 V1-0146, V1-0010 AC3: reviewer leg of the daily path from the v0.7.0 archive, recorded outcome

Independent reviewer, fresh clone of PR #102 at seal head `165e2d7` (merge base `d18db3d`), binary
extracted from the public v0.7.0 `corvint_darwin_arm64.tar.gz` (release and inner SHA256SUMS OK),
`Corvint 0.7.0 (build 46)`, sha256 `5fdbab207f6d15bd8ef341365642769cb58a11a76d935c37df77776ad0d09bad`,
first on PATH and exported as `CORVINT_BIN`. The reviewer wrote nothing to the author's checkouts or
to GitHub. Verdict on the change: MERGE; all five documentation checks exit 0 at the bind commit
`ec6cfa48`, the old citation fails `line-citations-check` (exit 2) once the file is scanned, and all
ten repinned citations were read and hold.

What the extracted binary established (DCW-V0-015: structural closure, never correctness).
`cem report` on the sealed map with `--expected-base d18db3d --target ec6cfa48`: exit 0, 10 of 10
hunks supported, 0 unknown, 0 mechanical; `cem verify` agrees. `frontier`: exit 1, OPEN, 5 hunks
with weak lexical support and 20 DCG obligations unassessed, matching the author's step 8 output.
`ocm report` on a map the reviewer had to rebuild with `ocm prepare` (intent guessed as
`documentation-citation-gate-v0.md`): 0 of 20 linked. `make dogfood-check` at the seal head:
`REFUSE sealed-head`, as documented.

What the reviewer leg could not observe. `make dogfood-check BASE=d18db3d` at the bind commit with
`CORVINT_BIN` set printed `NOTE unbound-commits count=1` and then `FAIL dogfood-report-missing`
(`script/dogfood-check.sh:305`), whose fix line names the author's `dogfood-change`; the check stops
before building or running any verifier. Whether the override binary is accepted as a verifier and
whether `outputsAgree` holds is therefore NOT_OBSERVED from the reviewer side; the author's
`outputsAgree: true` rests on the PR #102 body and the author's local receipts, not on the tree.

Reviewer-instruction mismatches (each filed as a ticket): DOGFOOD section 6 assumes the author's
worktree, so at the seal head the CEM path is gone and `--target HEAD` reports
`patch-digest-mismatch`; OCM maps are gitignored (`.gitignore:9-12`), so the step 11 hand-off carries
no OCM and the reviewer must guess the intent and run a writing command; section 6 names
`change.ocm.json` while the loop writes `change.ocm.001.json`; `dogfood-check` in a fresh clone
always fails on the missing local report, so the independent verifier comparison is not reproducible
by a reviewer; step 9's expected state omits the `NOTE unbound-commits` form; observations made after
the bind commit have no place in BUILD-LOG within the same change. Also found:
`docs/SPEC-TOOLCHAIN-INTEGRATION.md:74` cites only the ID regex at `internal/lrfrepo/ocm.go:37` for
the full requirement-line grammar (the prefix check is at `ocm.go:581`), and the bare-basename
citation `change-frontier-v0.md:217` at line 121 is stale.

Outcome. V1-0146 acceptance (one reviewer-leg run recorded with its outcome) is met by this entry.
V1-0010 AC3 (a fresh agent and a reviewer complete the same real change from extracted public
artifacts) is met for the change itself and for the reviewer's evidence reads; the reviewer-side
verifier comparison stays NOT_OBSERVED until `dogfood-check` can run against a handed-off report.
NOT_RUN by the reviewer: `dogfood-change`, `dogfood-seal`, `make gate`, `ocm link`/`ocm mark`.

## 2026-09-23 V1-0148 DCG-V0-001, V1-0010 AC3, V1-0146: stale DOGFOOD citation fixed via the daily path from the v0.7.0 archive

Fix (V1-0148). `docs/SPEC-TOOLCHAIN-INTEGRATION.md` constraint 1 cited DOGFOOD lines 32-36 (now the
daily-path steps); the same-commit CEM constraint is DOGFOOD section 2, lines 152-155. The file was
outside the gate, so the stale citation passed at the base. DCG-V0-001 and `scanned_doc()` now name
it; outside the legacy allowlist (DCG-V0-018) all ten of its citations need anchors, so each was read
and pinned: seven had moved and were repointed, one kept its lines with reworded text that still
holds, one named the deleted `src/context_corvint_ocm.py` and now names the requirement-ID pattern in
`internal/lrfrepo/ocm.go`, one was unchanged. With the old citation restored the gate fails (anchor
required); with the fix it passes, as do `line-citations-test` and `spec-requirements-check`.
Follow-ups, not changed: the file's two bare-basename citations stay unchecked by DCG-V0-001 design,
and its 2026-08-29 claim that `corvint frontier` is unimplemented is stale prose.

Fresh-agent leg (V1-0010 AC3). Binary from the public v0.7.0 `corvint_darwin_arm64.tar.gz` (release
and inner SHA256SUMS OK), `Corvint 0.7.0 (build 46)`, sha256
`5fdbab207f6d15bd8ef341365642769cb58a11a76d935c37df77776ad0d09bad`, equal to the arm64 build A, build B
and retained digest in the release `verification-report.json` (PASS, revision `41f2b689`). Orientation
and every pre-bind `dogfood-change` pass ran with it as `CORVINT_BIN`; it was accepted (no private
build under the Git evidence directory). The bind pass, check and seal follow this commit and are
recorded in the pull request.

Orientation. `corvint query` (limit 1) returned decision 0136, not the owning spec: a partial miss.
Path impact on the two files did not surface `documentation-citation-gate-v0.md`: a miss. Range
impact before any edit (base equal to head) was CLEAN with no results.

Deviations from the DOGFOOD.md expected state. Step 3 matched. Step 4: (a) the first plan got
`cem-cite: cite-span-not-stable` with no `fix:` line and no row ordinal (DCW-V0-014 promises a `fix:`);
a row cited DCG-V0-001 lines hunk 7 edits, and citing unchanged lines 76-77 cleared it. (b) After an
added BUILD-LOG commit the map was re-prepared with 10 hunks, and the 9-row plan was applied by
ordinal without any row-count refusal: every citation shifted one hunk and one hunk stayed unknown.
(c) Rerunning with a corrected plan added to, not replaced, those citations, so the untracked map had
to be deleted. (d) The pass after that deletion started clean and ended exit 0, no output,
`"complete": true`, `local-outcome` PRODUCED, with `?? .corvint/change.cem.json` still uncommitted,
instead of the documented `local-outcome: record-index-failed`.

NOT_RUN: `make gate` and the full Go suite (owner policy); step 7 (`ocm link`/`ocm mark`).
NOT_OBSERVED: the independent reviewer leg from extracted archives (V1-0146), pending on the PR.

## 2026-09-23 V1-0097, decision 0372, CEP-V0-001..CEP-V0-006: external retrieval evaluation and the already-fixed control

V1-0097 (spike) adds the external evaluation slice to `docs/specs/context-evolution-program-v0.md`
as the separate accepting slice decision 0179 requires (decision 0372); the five hypotheses stay
prose. Results below are evidence a later promotion may cite, never a promotion (`CEP-V0-006`).

**ContextBench adapter (`CEP-V0-001`..`003`).** `tools/retrieval-bench --samples` now reads
ContextBench (arXiv 2602.05892; evaluator `EuniAI/ContextBench` at
`1436c28a8eb95496da4ea69ad458b9f8a8eb7d61`, Apache-2.0) rows exported one JSON object per line.
Gold paths replicate `_normalize_rel_path` exactly, including Python's `lstrip("./")`, which also
turns `.github/x.yml` into `github/x.yml`; the adapter keeps that quirk so gold matches the
upstream scorer. Metrics replicate `metrics/compute.py` `coverage_precision` at file and line
granularity (coverage = shared/gold, empty gold 1; precision = shared/predicted, empty prediction 1;
line intervals merge when they overlap or touch). The prediction is the ranking cut at `--limit`,
and a ranked file predicts all of its lines, so `cb_line_precision` is a whole-file lower bound.
Symbol and byte-span granularities: NOT_MEASURED (they need tree-sitter definitions and byte
offsets the packet does not carry). Fixture `tools/retrieval-bench/testdata/contextbench/rows.jsonl`
(sha256 `9678b04832679f000e2eb5cac3230c5698f34b0f78c1c195cedfb0250af61fbc`, two synthetic rows,
one chunk-file snapshot). A smoke run of the base `corvint context` arm on that fixture, offline,
answered READY on both rows (file coverage 1.0 on both; file precision 0.667 and 0.5; line
precision 0.444 and 0.375); fixture numbers are scoring checks, not evidence.
Full ContextBench run: NOT_RUN. The rows are downloadable (Hugging Face `Contextbench/ContextBench`,
`default` 1,136 rows in a 26,102,607-byte Parquet per the dataset API, not downloaded), but the
dataset ships no repository snapshots: its evaluator clones 66 upstream repositories at their base
commits, a networked fetch well above the 2 GB bound and outside the offline rule (`CEP-V0-003`).

**Agent Retrieval Bench baseline (`context` arm, base a98d770).** `corvint` and
`tools/retrieval-bench` built from a98d770 (`GOTOOLCHAIN=local` go1.27.1), `--arms context`,
`--limit 20` (default), all samples, snapshots from the local `--corpus` chunk files; reports stay
under the session scratchpad, uncommitted. Metric definitions are the tool's (`tools/retrieval-bench/README.md`
"Metrics"): per positive sample, `recall@k` = gold files in the top k over gold files, with the
sample's given files removed from the ranking first; means over positive samples.

| Subset (samples file) | sha256 | n (positive) | errors | recall@5 | recall@10 | recall@20 | hit@20 | abstained |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| `v2_trace2code/trace2code.jsonl` | `9d0ff50155fa4f65c1bbb632abc4f1f11393c50fa7b0a9249e06128b368b6266` | 101 (101) | 0 | 0.4010 | 0.5083 | 0.7937 | 0.8614 | 0.0099 |
| `v2_edit2ripple/edit2ripple.jsonl` | `31d97ffe815dda8730149880c0159239a986eceb306e5a5df816dfc420717cef` | 58 (58) | 0 | 0.3621 | 0.5101 | 0.6293 | 0.7586 | 0 |
| `v2_comment2context/comment2context.jsonl` | `543267024f7f06127c50664a3a1b8825d3c213e4b12dbbfabf9a3346902ad796` | 80 (80) | 0 | 0.2562 | 0.3438 | 0.5042 | 0.6500 | 0 |
| `v2_code2test/code2test.jsonl` | `712b2699d3f963c2f940688ae5248304263c4ecda1b3831b2801c95712a50b85` | 106 (106) | 0 | 0.2877 | 0.3994 | 0.5116 | 0.6038 | 0 |

No sample was skipped as unlabeled in any subset. The code2test run was started separately after
comment2context (same binaries and flags) because host contention slowed the sequential loop.

**Already-fixed control (`CEP-V0-004`, `CEP-V0-005`; arXiv 2603.25764).** `tools/cw-trial` tasks may
carry `control: "already-fixed"` with no gold; every valid claim is FALSE, and a `certain` claim or
a produced corvint packet that does not abstain sets `control_failed`. Fixture
`tools/cw-trial/testdata/already-fixed` (one synthetic Go task: the reported empty-string panic is
already handled and tested at the pinned revision). Pilot arm run offline with the base `corvint`
and two script agents (no model): an abstaining agent scored `control_failed` 0 on `none` and
`grep` but 1 on `corvint`, because the packet answered READY with `port.go`, `port_test.go`,
`README.md` (answerability verdict `no-specific-terms`), so `packet_abstained` 0; a certain agent
scored `control_failed` 1 on all three arms. Current Corvint therefore fails the control: it has no
"already fixed" signal. Codex/model arm: NOT_RUN (needs a model and the network).

Gaps: ContextBench full run NOT_RUN; symbol/span granularities NOT_MEASURED; model-agent control
arm NOT_RUN; no held-out control set (the fixture is synthetic, NOT_OBSERVED as held-out evidence).

Review fixes (PR #96): control tasks now stay out of every other arm aggregate and are counted only
under `already_fixed`; `control_failed` is the agent's `certain` claim alone, shared by every arm,
with packet abstention recorded apart as `packet_abstained`; ContextBench spans ending past line
10,000,000 are refused and line numbers stay integers; rescoring both pilot reports with
`cw-trial score` gives `control_failed` 0 on all arms for the abstaining agent and 1 on all arms for
the certain one, with the corvint packet `packet_abstained` 0 in both.
## 2026-09-23 V1-0099 TCP-V0-043..046 EEP-V0-023..026: opt-in gopls definition/reference expansion (decision 0371)

V1-0099 adds an optional, Core-owned local language-server provider (`internal/lspprovider`,
decision 0371). `CORVINT_CONTEXT_LSP=gopls` is the only enabling value; unset or any other value
leaves `corvint context` byte-identical to `cmd/corvint/testdata/context-default-wire.golden`
(`TestContextLSPOffKeepsTheGoldenAndOnDegrades`). When on, one `gopls serve` process per invocation
runs under `procgroup` with a 20 s wall bound, owned process-group cleanup and a private cache
directory. It expands at most 3 committed, unmodified Go seeds (the subject plus Go result rows) by
1-2 hops of `textDocument/definition` and `textDocument/references`, bounded by 64 queries, 32 rows
and 64 KiB. The result is an `external-evidence-provider/2` record decoded by the shared extevidence
path into `packet.external` only. Every row names its hop origin and the query digest; its authority
stays `external-provider`, never project authority. `results` ranking is untouched by construction.
A missing, failing, timed-out or not-applicable provider yields an `unavailable` provider row with
a reason, and the rest of the packet is unchanged. Conformance fixture:
`internal/extevidence/testdata/conformance-path/lsp-gopls.json`.

Frozen retrieval bench (`tools/retrieval-bench`, `--arms context --max-samples 20`, first 20
samples per subset, candidate binary sha256 `2ef0ffa7c20182a36061d8e6fc847d55f353501e29eb365731efa02a2c270bb3`
built from this branch, host load 30-390 from parallel workers; reports are scratch, uncommitted):

| Subset | recall@5 | recall@10 | recall@20 | ranked lists identical off/on | provider with ≥1 relation / loaded with 0 relations / not applicable | p50 wall ms off / on |
| --- | --- | --- | --- | --- | --- | --- |
| v2_code2test | 0.333 | 0.358 | 0.428 | 20/20 | 7 / 8 / 5 | 3840 / 7636 |
| v2_comment2context | 0.383 | 0.492 | 0.633 | 20/20 | 3 / 9 / 8 | 2901 / 3751 |
| v2_trace2code | 0.375 | 0.450 | 0.817 | 20/20 | 8 / 11 / 1 | 874 / 1988 |
| v2_edit2ripple | 0.358 | 0.488 | 0.592 | 20/20 | 4 / 9 / 7 | 385 / 746 |
| v2_abstention | no positives (abstained 0.05 both) | | | 20/20 | 0 / 0 / 20 | 769 / 544 |

The applicability column is a recount derived from the existing on-run capture JSONL, not a rerun.
The first report counted every `loaded` provider row (15/12/19/13). Independent review found that
`ask` swallowed JSON-RPC query errors, so a run where every query failed still said `loaded`. Real
gopls v0.22.0 on caddyserver__caddy@aed1af59 answered 39 of 39 queries with `no package metadata
for file`. 37 of the 59 `loaded` runs had 0 relations: all 13 caddy runs, 23 etcd runs and 1 gin
run. The captures predate `failed_queries`, so the recount splits on at least one relation rather
than on at least one successful query. Review fixes (decision 0371 unchanged): `failed_queries` now
sits beside `queries_issued` (EEP-V0-025, TCP-V0-043). A run whose every issued query failed is an
`unavailable` row naming the first error (EEP-V0-026, TCP-V0-045). A server whose `serverInfo.name`
is not `gopls` is refused, and a `PATH` lookup error, including `exec.ErrDot`, is `gopls executable
not found` (EEP-V0-024). `TestExpandEveryQueryFailedIsUnavailable` covers these with a fake server.
The bench was not rerun after the fixes; results are unaffected by construction.

Recall is identical off and on in every subset, as designed: the bench scores `results` and the
provider writes only `external`. The not-applicable count is the observed provider state; gopls
applies only to Go modules, and non-Go samples report `not applicable: no committed, unmodified Go
file among the seeds`. The offline capture diagnostic found gold files among external path
endpoints in 7 (code2test), 2 (comment2context) and 6 (trace2code) samples, but in 0 samples was
such a gold file absent from `results`. On this slice the expansion added no new gold evidence.
The on-mode cost is up to about 2x p50 wall and 1.6-3.0 KB of p50 packet bytes. On the
Corvint repository itself one run stopped at the 15 s soft deadline after 8 queries (hop 2 not
reached, 25.6 s total context wall). No `gopls serve` process or `corvint-gopls-*` temp directory
survived the tests or the bench.

NOT_RUN: full-subset bench (bounded to 20 samples per subset); a paired agent trial on the
external section; other language servers. NOT_OBSERVED: any recall change, or gold newly surfaced
by the external member. NOT_PRODUCED: owner acceptance and promotion; TCP-V0-043..046 and
EEP-V0-023..026 stay proposed and experimental.
## 2026-09-23 V1-0098 TCP-V0-025..029: opt-in line-budgeted span rows and sufficiency check (decision 0366)

V1-0098 adds `CORVINT_CONTEXT_SPANS=on` to `context`: `packet.spans` (core declaration rows
plus call-site windows under a declared 240-line budget, each with explicit lines and a reason)
and `coverage.sufficiency` (per task anchor `satisfied|insufficient|unknown`, set `satisfied`
only when every anchor is, missing anchors named). Code lives in
`internal/contextindex/span_rank.go` and `sufficiency.go`; `TaskContext` gains one call. With the
flag unset or any other value the packet equals the recipe golden (`TestContextSpansDefaultBytes`),
and the flag changes no `results` byte (`TestContextSpansBudgetAndBounds`). No index encoding
changed: `analyzerSchemaID` stays `corvint-analyzer/73` and only the `TestAnalyzerSchemaInputs`
digest was repinned for the two new production files, as for earlier consumer-only changes.

Frozen evaluation (TCP-V0-029): `tools/retrieval-bench --arms context --context-packets`, all
samples of the five `v2_*` releases (no `--max-samples`), flag set empty (off) and `on`, `corvint`
built from this branch. recall@5/10/20, flag off = flag on in every subset: `v2_trace2code`
0.401/0.508/0.794 (n=101); `v2_code2test` 0.288/0.399/0.512 (n=106; the flag-off run lost 3 samples
to the 30-second Git index deadline under host load and read 0.278/0.390/0.502, and the 3 retried
flag-off rank identically to flag-on); `v2_comment2context` 0.256/0.344/0.504 (n=80);
`v2_edit2ripple` 0.356/0.504/0.624 (n=58, 3 sample errors in both modes); `v2_abstention` has no
gold (abstained 0.171 in both modes). Core-span recall against the ±15-line control: trace2code
0.133 vs 0.077 (7 wins, 4 losses), code2test 0.034 vs 0.007 (7 wins, 2 losses), comment2context
0.091 vs 0.016 (11 wins, 1 loss); edit2ripple has file-level gold only and is not scorable.
Sufficiency: trace2code 0 `satisfied` (precision undefined, base rate 0.109); code2test 1 of 41
`satisfied` fully covered (0.024 vs base rate 0.009); comment2context 1 of 22 `satisfied` fully
covered (0.045 vs base rate 0.038); abstention 2 `satisfied` on no-gold samples (0.0); edit2ripple 1
`satisfied`, not scorable against spans (its rows do include the gold file). Losing cases: span
recall below control on trace2code `05041faae6e19b6882e3074a`, `3bd1eecf0ebcd9c6b334fa92`,
`7db765ce2d8ea9b3f68029fd`, `9f0d0d1bb836481d62b838aa`, comment2context `3fd987cc42a8a4550add3562`,
code2test `29168597c41ad9e94b95412c`, `c9059c66a5c31bceb46c9edf`. No sufficiency precision falls
below its base rate, but `satisfied` also appears on no-gold
`abstention_candidate__organic_issue__e783b22ef915b1c2fb513b60` and `...__5013784e701897f60233c4dc`
(generic anchors `TargetClosedError`; `mock`, `patch`). Verdict: the feature stays opt-in (decision
0366); `satisfied` is not evidence of gold coverage.

The ten runs (plus the 3-sample code2test retry) ran in parallel on a host shared with other workers
(load average 65 to 371), so latency figures in the reports are not comparable and are not recorded.
Reports and captures stay in the session scratchpad and are not committed. Verification: `go test`
of `./internal/contextindex/... ./internal/specindex/` and `-run Context ./cmd/corvint/` pass, `go
vet` on those packages is clean, and the focused-docs gate passes. NOT_RUN: the other units selected
by `corvint affected --base a98d770` (61 Go units plus 16 unknowns; the change is flag-gated and the
default bytes are test-proven identical), `make gate` (owner policy), and any paired or promotion
evaluation. NOT_PRODUCED: a committed span scorer (the TCP-V0-029 scorer is a scratch script).
NOT_OBSERVED: any agent consuming span rows in a real task.

Review fixes (PR #99): `coverage.sufficiency` gains `scope: task-anchors` and its reason now reads
"N of M task anchors carried by the selected lines; not evidence the task is answered", the
call-site reason reads "names `S` at line L; `S` is declared by the core span P:S-E" (TCP-V0-026/028
reworded), two stale comments are corrected, and the analyzer digest is repinned with the schema
still `corvint-analyzer/73`; the frozen-evaluation figures above predate the fix, which changes only
wording and adds one member.
## 2026-09-23 V1-0083, decision 0367, TCP-V0-030..034: identifier graph and opt-in personalized PageRank slot

V1-0083 adds a deterministic identifier definition/reference graph to the index and an opt-in
`graph` slot in `corvint context` that ranks files near the task anchors by personalized PageRank.
The graph is derived from the indexed Symbols and the Words postings at the indexed revision and
stored as `TermTable.IdentGraph` (pack section `vocab.identgraph`); `analyzerSchemaID` moves to
`corvint-analyzer/74` and `TestAnalyzerSchemaInputs` is repinned. Under `CORVINT_CONTEXT_GRAPH=on`
the slot seeds from the subject and anchored rows, admits at most five low-confidence tail rows
after every relation row, names the seed and hop path in each reason, and abstains as `no-seed` or
`graph-bounded`. Unset, the packet is byte-identical to the recipe golden.

Frozen evaluation, `tools/retrieval-bench --arms context --max-samples 30` per subset, flag unset
versus `on`, same branch binary (reports `V1-0083-{off,on}-<subset>.json` in the session
scratchpad, not committed). The sample bound is recorded: unbounded runs managed about 0.5 samples
a minute at host load ~300.

| Subset | recall@5 off/on | recall@10 off/on | recall@20 off/on |
| --- | --- | --- | --- |
| code2test | 0.4056 / 0.4056 | 0.5222 / 0.5222 | 0.5856 / 0.5289 |
| comment2context | 0.3278 / 0.3278 | 0.4222 / 0.4222 | 0.5278 / 0.4389 |
| edit2ripple | 0.3833 / 0.3833 | 0.4694 / 0.4694 | 0.5667 / 0.5639 |
| trace2code | 0.4833 / 0.4833 | 0.5833 / 0.5833 | 0.8611 / 0.8278 |

recall@20 regresses on all four subsets (13 losing samples, 2 wins, listed in decision 0367), so
the ticket's closing rule (improve recall@20 on code2test, comment2context and edit2ripple) is not
met and the slot stays opt-in. The losses are gold files held by lexical tail rows that graph
rows displaced.

Verification: `corvint affected --base a98d770` (62 packages selected; the exhaustive gate is
NOT_RUN per owner policy); `go test` of `./internal/contextindex/...` and `./internal/specindex/`
passed; `go vet` of those and `./cmd/corvint` passed; `go test -run 'Context|Index|Snapshot|Pack'
./cmd/corvint` failed only `TestExperimentalKernelAdapterContext`, a 250 ms deadline test that also
failed once in six runs at BASE under the same load (six of six passed on the branch when
alternated); focused-docs gate passed. NOT_RUN: full-sample evaluation, the other 57 affected
packages, the unfiltered `./cmd/corvint` suite. NOT_OBSERVED: any recall@20 gain.

Review fix: a graph past a bound was stored with no name offset, so `check` rejected it and every
saved bounded index was a silent cache miss; `boundedIdentGraph` now stores one offset, a bounded
flag other than 0 or 1 fails decode, and `TestIdentGraphBoundedSnapshotReloads` covers write, load
and `graph-bounded` abstention under both snapshot formats. Default-path cost, measured on this
repository with the flag unset (3,489 paths, 47,384 arcs, 10,706 names): 836,194 bytes of snapshot
section, 30-31 ms to build, about 0.2 ms to decode and check (host load 13-35). The Git index
deadline error no longer names a fixed 30-second limit.
## 2026-09-23 V1-0084 TCP-V0-022 anchor evaluation: flag off/on over four v2 subsets (decision 0333 kept)

Ticket `V1-0084` acceptance criterion 3, requirement `TCP-V0-022`, decision 0333 (unchanged).
`tools/retrieval-bench` now marks a sample `anchor_bearing: true` when its full query text carries
at least one anchor by `contextindex.TaskHasAnchors` (the five classes the context compiler
extracts under `CORVINT_CONTEXT_ANCHORS=on`, over the decoded JSON string values) and averages
those samples under `stratum:anchor-bearing`; the field is omitted when false, so reports without
anchors keep their bytes (`TestAnchorBearingSamplesFormTheirOwnStratum`). The export touches an
analyzer input file, so the `TestAnalyzerSchemaInputs` source digest is repinned; no extraction or
encoding change, `analyzerSchemaID` stays `corvint-analyzer/73`.

Run: `--arms context`, k 20, no `--max-samples`, branch binary sha256 `0b53bcd15fdb…`, flag unset
versus `on` (registration `environment` shows `[]` and `[CORVINT_CONTEXT_ANCHORS=on]`; the child
inherits the bench's environment). Host load averaged 30 to 350 during the run (parallel workers):
wall times are not evidence. Recall off -> on, overall and anchor-bearing stratum:

| subset | n (anchor) | r@5 | r@10 | r@20 | anchor r@20 |
| --- | --- | --- | --- | --- | --- |
| code2test | 106 (74) | 0.2877 -> 0.2877 | 0.3994 -> 0.3994 | 0.5116 -> 0.5116 | 0.5293 -> 0.5293 |
| comment2context | 80 (56) | 0.2563 -> 0.2563 | 0.3438 -> 0.3500 | 0.5042 -> 0.5083 | 0.4583 -> 0.4643 |
| edit2ripple | 58 (57) | 0.3621 -> 0.3621 | 0.5101 -> 0.4871 | 0.6293 -> 0.6394 | 0.6228 -> 0.6330 |
| trace2code | 101 (100) | 0.4010 -> 0.4109 | 0.5083 -> 0.4934 | 0.7937 -> 0.7591 | 0.7917 -> 0.7567 |

Per-sample wins/losses (r@5, r@10, r@20): code2test 0/0 everywhere (24 rankings reordered, no
gold moved across a cut); comment2context 0/0, 1/1, 1/0; edit2ripple 0/0, 0/2, 2/0; trace2code
1/0, 2/3, 2/5. All five trace2code recall@20 losses are `pallets/click` (fold B: r@20 0.6532 ->
0.5450; fold A 0.8750 -> 0.8828): 08d24e63, 49589b87, 8882930f, 9372208a, e50d9d50. The recall@5
falsifier (at most 0.01 loss on every fold of code2test and trace2code) passes. Decision: the
field stays opt-in under decision 0333, because decision 0070's paired ladder is NOT_RUN and
recall@10 (edit2ripple, trace2code) and recall@20 (trace2code) losses were observed; the stricter
"no recall@20 loss on any subset" bar was proposed after this run, not governing. Reserved decision
0373 is unused. `TaskHasAnchors` applies the compiler's 32,000-byte task bound; no sample exceeded
it (no context-arm length error in any run).

Errors: 2 code2test samples (spring-projects/spring-boot 9c3412df, e8ef6b1c) failed identically in
both arms with `Git repository index exceeded its 30-second deadline` under host load; they score
zero in both, so the paired comparison is unaffected, but their flag-on behaviour is NOT_OBSERVED.
NOT_RUN: bootstrap intervals for the off/on difference (the bench pairs retrieval arms against
lexical baselines only, not two runs of one arm), decision 0070's paired ladder, and `v2_abstention`
(outside the four task subsets named by the criterion). Reports stay outside the repository.
## 2026-09-23 V1-0096 TCP-V0-039..042: opt-in role-line field from doc comments (decision 0370)

Contract: a file's role line is the first sentence of its package, module or top-level doc comment
(Go, Python, Rust, and `/**`/`///` doc-marker languages), read deterministically from the pinned
blob, at most 160 bytes, with licence and generator headers refused and 64 KiB scanned
(`internal/contextindex/rolesummary.go`, golden `testdata/role-summary-golden.tsv`). With
`CORVINT_CONTEXT_ROLES=on`, the role lines of the 512 highest-scoring lexical sources are a fifth
lexical field (path-field form, body idf, gain 1.0 frozen before the run). A row that uses one
carries the reason prefix `role: "LINE" (PATH:START-END) matches ...; `, keeps score 300 and
authority `vocabulary`, and never precedes a reserved authority row. The line is derived per call,
never stored: no index, snapshot or pack change, `analyzerSchemaID` stays `corvint-analyzer/73`,
and only the `TestAnalyzerSchemaInputs` source digest is repinned. With the flag unset, `off` or
unknown, the recipe golden bytes are unchanged (`TestContextRolesDefaultBytes`).

Frozen evaluation: `tools/retrieval-bench --arms context`, the same branch-built binary, flag
unset versus `on`, over the full positive strata with no `--max-samples`. Paired means over
positives:

| Subset | n | recall@5 off/on | recall@10 off/on | recall@20 off/on | W/L @20 |
| --- | --- | --- | --- | --- | --- |
| v2_code2test | 106 | 0.2689 / 0.2689 | 0.3805 / 0.3711 | 0.4928 / 0.5022 | 1/0 |
| v2_comment2context | 80 | 0.2562 / 0.2604 | 0.3438 / 0.3250 | 0.5042 / 0.4938 | 1/4 |
| v2_edit2ripple | 58 | 0.3563 / 0.3563 | 0.5043 / 0.4813 | 0.6236 / 0.6336 | 2/0 |
| v2_trace2code | 101 | 0.4010 / 0.4257 | 0.5083 / 0.5033 | 0.7937 / 0.7987 | 1/0 |

Losing cases (sample IDs, on below off): comment2context @20 `41deac7db89b57cead1c85e0`,
`7d5c2788e4c30bb773cb6643`, `c2af1d6140c0b8b749bd1b77`, `e1280404f66f39671f1939e5`; @10
`0b514d819e10c606b274e8c0`, `38c2a13af5bdc49dd7d75a2f`, `8e6bf4a9d26a7468cab5a5da`. edit2ripple
@10 `18a155cebfee9969b166934c`, `262ecc80fa3618111feb4987`. trace2code @10
`a4218c7e484b796962f32982`, `fa19b2ec3770df1f1285f072`. code2test @10
`b2bcc7a9cd02595dccde4bb0`. Seven samples failed in both arms: edit2ripple
`78906550756a325d653e88b9`, `e271598f05638161b8b0fbdc`, `445f0e5cb04c0403b28b2294`, and code2test
`2177ce0889655fd1979b99c1`, `9c3412dfb452df23d783c5e6`, `d58f6487e6e2721dd5266c21`,
`e8ef6b1c59b7afde52d0cded`. The error was "Git repository index exceeded its 30-second deadline",
on a host at load 40 to 300 from parallel workers. They score zero in both arms, so the pairing
stays symmetric. The losing cases were not inspected (NOT_OBSERVED: the reason text was not read, to keep corpus content out).
Reports stay in the scratchpad, uncommitted.

Gating: comment2context lost recall@20, so the rule in TCP-V0-042 keeps the field opt-in
(decision 0370). No bootstrap interval was computed for the on/off difference (NOT_PRODUCED: the
bench pairs arms, not flag settings).

Gates: `corvint affected` selected 62 Go packages. `go test -count=1 -timeout 30m` passed 60 of
them. `internal/contextindex` first failed `TestAnalyzerSchemaInputs` on the source digest, which
was repinned, and then passed. `cmd/corvint-go-test-provider` failed process-lifecycle
qualification tests (1 failure, then 5 different ones, on a rerun under load). That package does
not depend on `internal/contextindex`; this is recorded as pre-existing and load-sensitive, not
fixed here. `go vet` is clean on all 62, and the focused-docs gate passes. `make gate`:
NOT_RUN (owner policy).
Review fix: a `*/` block now attaches only when its opening line starts with `/*` and no other
comment or trailing-code `*/` lies between, and blocks are read lazily, which removes a
misattributed role line and a quadratic 64 KiB scan (golden `go/misattributed.go`,
`go/quadratic.go`, `go/trailing.go`, now bounded under 16 MiB and 100 ms); the frozen evaluation
above predates the fix and was not rerun (NOT_RUN).
## 2026-09-23 V1-0089 TCP-V0-035..038, decision 0369: opt-in recency, blame and ownership in context

V1-0089 adds three history features to the `context` packet behind `CORVINT_CONTEXT_RECENCY=on`:
90-day half-life recency, blame last-touch freshness and CODEOWNERS/blame disagreement
(`internal/contextindex/recency.go`, `blame.go`). All three read only Git objects reachable from
the indexed commit, are bounded (200-commit window, 10 blamed files, 4 MiB), reorder the lexical
and `cochange` slots before their cap and the limit (so which candidates they admit can change),
and name each value or abstention reason in the row reason. A CODEOWNERS owner who matches no
in-window blame author is reported as `disagrees` or `unverifiable`; it never changes ranking.
Unset, the packet bytes are unchanged.

Frozen `agent_retrieval_bench` context arm, off vs on, candidate `a3521dd9…cde3f1`, bench binary
`229abcc7…670ff`, `--limit 20`, `--max-samples 20` per subset (folds A and B, same sample set both arms,
`samples_sha256` equal):

| Subset | recall@5 off/on | recall@10 off/on | recall@20 off/on |
| --- | --- | --- | --- |
| code2test | 0.3333 / 0.3333 | 0.3583 / 0.3583 | 0.4283 / 0.4283 |
| trace2code | 0.3750 / 0.3750 | 0.4500 / 0.4500 | 0.8167 / 0.8167 |
| comment2context | 0.3833 / 0.3833 | 0.4917 / 0.4917 | 0.6333 / 0.6333 |
| edit2ripple | 0.3583 / 0.3583 | 0.4875 / 0.4875 | 0.5917 / 0.5917 |
| abstention | n/a (0 positives) | n/a | n/a; abstained 0.05 / 0.05 |

Every ranked list is identical on and off (100/100 samples). The features did run: summed packet
bytes rose 31-40% per subset (row reasons and `coverage.recency`). The equality is structural, not
evidence of safety: the bench rebuilds each snapshot as one commit, so recency is 1 everywhere and
every blame line is a root boundary. `corvint eval` against beamfall/core at 6e82abd (7 cases) is
also identical off and on (recall 1.0, top-5 success 1.0, must-read 11/11, no critical misses,
abstention 1/1), because eval exercises query, feature and impact and never `context`. On this
repository at limit 10 the `context` call took 1.27 s off and 1.64 s on.

Decision 0369 keeps the features opt-in: no available corpus can show a gain or a loss, so a
no-regression reading does not justify default-on. Promotion needs a frozen corpus that keeps
commit history. The ticket said CODEOWNERS was already parsed; it was not, so `blame.go` adds a
bounded parser. `TestAnalyzerSchemaInputs` repins only `auditedSHA256` (query-side change;
schema stays `corvint-analyzer/73`).

NOT_RUN: `make gate` (owner policy); the full-sample bench (host contention, load average above
60; `--max-samples 20` recorded above); any corpus with real history. Under the same load one
`cmd/corvint-go-test-provider` test failed on the branch with a post-run authority revalidation
timeout. It passed on the branch and at base when run side by side, and the package does not import
`internal/contextindex`.

Review fixes: the spec, decision 0369 and this entry now say the reorder can change which
candidates a slot admits (`TestContextRecencyCanChangeLexicalMembership`); blame runs only on
candidates the lexical slot can still admit, so `coverage.recency.ownership` never lists an
earlier slot's row (`TestContextRecencyBlamesOnlyRowsTheLexicalSlotCanAdmit`); `parseBlame`
skips an all-whitespace header line; and the full window's oldest commit, a blame range
boundary, is documented as outside the window. The bench readings above predate these fixes.
## 2026-09-23 V1-0092 cem-v1-emission: `prove --attest-cem-v1` and the checked OpenFab/agentattest alignment

Ticket V1-0092, decision 0365 (amends 0354), FPK-V0-050 and FPK-V0-051 (new), FPK-V0-033,
FPK-V0-035, and FPK-V0-036 (amended). All are experimental and not advertised. The entry closes the
two gaps that the `cem-intoto-predicate-v1` entry recorded.

Changed:
- `prove --cem MAP ... --attest-cem-v1` is a new flag on the existing CEM mode. It prints the
  unchanged first line, then `CEMStatementV1(MAP, bytes)`, signed under `--attest-key`. The flag
  takes no value, cannot repeat, and cannot be given with `--attest-cem` (`invalid-arguments`). No
  root verb is added. `helpBooleanFlags` and the two `prove` usage lines name the flag.
- The known-deviations table now compares `cem/v1` field by field with fetched sources:
  ossf/tac issue 628, which links the OpenFab `openfab/generation` v0.1 draft (revision 0.1.3,
  Open-fab-ai/openfab `f558da05`); agentattest predicate v1 (AuroraAeon/agentattest `a19e7f96`);
  in-toto/attestation `spec/v1` (`fd2609c1`); and DSSE `envelope.md` (`1d3370f6`).
- Confirmed aligned: the in-toto Statement v1 layer and the `{name, digest.sha256}` subject shape
  in both drafts, and the DSSE envelope used by agentattest.
- Deviations: the OpenFab v0.1 envelope is DSSE-style with no PAE. Member names are snake_case in
  OpenFab and camelCase in `cem/v1`. agentattest's `repo.baseCommit` corresponds to `cem/v1`'s
  `base.digest.gitCommit`. OpenFab's `spec_ref` means something other than `cem/v1`'s `spec`. The
  agent, model, prompt, material, time, acceptance, and approval fields are absent from `cem/v1`.
- No `cem/v1` byte changed, and no draft field was adopted. The OpenFab draft's own `$id`
  (`openfab.ai`) disagrees with its stated `predicateType` (`open-fab.ai`).

Measured:
- A committed map (`.corvint/changes/d517913....cem.json`) was run with the base binary (a98d770)
  and the branch binary. Five invocations were byte-identical on stdout and stderr: no attest,
  `--attest`, `--attest --attest-cem`, `--attest-key`, and `--attest-key --attest-cem`. Their
  sizes were 3917, 4351, 4842, 6049, and 6954 bytes.
- `--attest-key --attest-cem-v1` gave the same first line. Its second line verified through
  `prove --verify-cem-attestation` as `cem/v1` `VERIFIED` with `baseRevision` and `patchSha256`.
  A scratch test (not kept) passed the same envelope to the `interop/cem01-go` reader
  `readCEMAttestation`, which returned `VERIFIED` with the map and `NOT_RUN` without it.
- `go list -deps ./cmd/corvint` has 0 packages matching sigstore, rekor, fulcio, cosign, gitsign,
  or securesystemslib. It has 0 packages outside the standard library and the Corvint module.
  `go.mod`, `go.sum`, and `interop/cem01-go/go.mod` are unchanged.

NOT_RUN / NOT_PRODUCED:
- The Sigstore gitsign/cosign/Rekor external step is NOT_RUN. It remains an optional operator step.
- `make gate` and the exhaustive `go test ./...` are NOT_RUN (owner policy). Verification was
  `corvint affected`, which selected `cmd/corvint`, plus `internal/attest`, the interop gate, and the
  focused-docs gate.
- No independent adopter has read `cem/v1` (V1-0014 is unchanged). Neither draft's own verifier
  was run against a Corvint envelope, so envelope incompatibility with OpenFab v0.1 is inferred from
  its documented shape and was NOT_OBSERVED.
- "agentattest" is ambiguous on GitHub: three repositories carry the name. The in-toto one
  (AuroraAeon) was compared, and this choice is recorded as an assumption in FPK-V0-051.
## 2026-09-23 V1-0100 AFU-V0-001..AFU-V0-012: application-flow gate evidence and traceability

The experimental slice already landed through PR #65. This change adds no behaviour; it reruns the
frozen browser evaluation on base `a98d770` and replaces the grouped traceability rows in
`docs/specs/application-flow-understanding-v0.md` with one row per requirement, each naming its
tests or an explicit gap. No new requirement ID or decision was needed.

`script/web-flows-gate` (after `npm ci` in `tools/web-flows`; Node v22.23.2, Playwright 1.63.0,
Chromium already installed) exited 0: 6/6 scanner and lifecycle tests, 9/9 browser cases
(`development-fixture`, `held-back-selectors`, `persist-broken`, `auth-broken`,
`wrong-served-identity`, `cross-origin-http-and-websocket` with 0 sentinel requests, `source-only`,
`SIGINT` with 5 and `SIGTERM` with 3 retired descendants), `seededDefectsDetected:2`,
`falseConfirmations:0`. The false-confirmation field is a literal in `tools/web-flows/test/e2e.mjs`;
the zero rests on the per-case assertions that pass, not on a computed count.

Explicit gaps now recorded instead of implied coverage: no test asserts the emitted
`test-syntax-unresolved:<path>` gap or a test file from another framework, the standing
`non-http-browser-transports-unqualified` and `escaped-daemon-descendants-unqualified` gaps, the
`unaddressable-or-visual-control` gap, budget truncation gaps, secret-shaped input refusal, the
per-flow `next` action, or the copy of evidence gaps into the report frontier. The absence of a
combined confidence score is structural (the `Flow` and `Report` types) with no negative test.

`make gate` is NOT_RUN in this change by owner policy and remains a release-attestation item.
Complete-command benefit, blinded discovery precision/recall and external-application accuracy
remain NOT_OBSERVED. Profile acceptance and promotion are not performed: the spec stays
intent proposed / delivery experimental and awaits an explicit owner decision recorded in the spec
and this log.

## 2026-09-23 V1-0012 PCCO-V0-015..017: sealed daily-loop correctness and cost measurement

V1-0012 measured the daily change-evidence loop as it exists at `origin/main` 1894b9e against a
fixed plain-Git baseline. `benchmarks/daily-loop-v0/preregistration.json` (sha256
`73f178f8b1bcaaa57115c2ee9389f3a57fa3d889ee6e8a50035edeec3ae04b9d`) seals the three Core jobs
(task orientation, change consequence, evidence-carrying completion), the eight-commit first-parent
corpus (`corpus.json`, sha256 `412058c2…4cde2`, four exclusions under rules E1/E2), the harness
(`harness.py`, sha256 `17b41f50…2d242`), thresholds C1-C3, exclusions E1-E6 and invalidation rules.
It was committed in 69b5f4d before any sealed observation. Harness smoke checks ran before the seal,
on synthetic non-corpus commits only; the preregistration discloses them. PCCO-V0-015..017 are
proposed, not accepted.

Sealed run-001 (`benchmarks/daily-loop-v0/runs/run-001.json`, sha256
`70e452d219ade1041dd4a05b8f762f858c44af5bd57627c2813d7389b1c2c29f`) used candidate binary
`d28312fc…7096e` built from 1894b9e, with three runs per measurement. Its three
`corvint-use-case-evidence/0` sealed-benchmark receipts are under
`benchmarks/daily-loop-v0/receipts/run-001/`:

| Receipt | sha256 | Result |
| --- | --- | --- |
| `UC-TASK-ORIENTATION.json` | `4bd2a48c3a9d421e0f9e40d9ba47d5de9fe198ee84e9c2e7de25551e12325f98` | FAIL |
| `UC-CHANGE-CONSEQUENCE.json` | `66513302c06063447419232d7bff8053bba87534b66c26301d4b2babd230a06f` | FAIL |
| `UC-EVIDENCE-CARRYING-COMPLETION.json` | `64c81ecc5f258420df3d2330d723b07eac0a836f13c27ea253231f8ac8fa136f` | PASS |

All three are kept.

- **Orientation, FAIL (C1).** Six scored cases produced one treatment-only critical miss:
  `docs/AGENT-ROUTES.md` at 1d4cbaa. The lexical baseline found it and the `context` packet did not.
  There were zero abstentions.
- **Consequence, FAIL (C2).** Three scored cases produced five treatment-only critical misses:
  `internal/cem/coverprofile` and `internal/cemdiscriminate` at 50a9647,
  `cmd/corvint-test-validity-mcp` and `integrations/testfixture` at 362721c, and
  `cmd/corvint-web-flows` at af247a1. All five are changed packages with no `_test.go` file.
  `affected` selects test-bearing units, so it neither selected these packages nor named them as
  unknown; its overall scope stayed `UNKNOWN`. A post-run re-observation with the byte-identical
  candidate confirmed the af247a1 plan. The scoring is unchanged, because the sealed critical
  definition counts every changed root-module package.
- **Completion, PASS (C3).** 36 designated missing-evidence cases produced 0 false complete
  verdicts, and every case was informative. The CEM cases were five committed CEMs times six
  mutants, and every mutant was refused with its intended code (`patch-digest-mismatch`,
  `uncited-hunk`, `unsupported-without-basis`, `fabricated-evidence-id`, `base-revision-mismatch`,
  `max-unknown-exceeded`). The six loop variants were all refused:
  - L1 stale-after-bind, L2 CEM removed, L3 CEM tampered, L5 without `DOGFOOD_VERIFY_FILE`, and
    L6 without `DOGFOOD_CITATIONS` each gave `dogfood-check: FAIL dogfood-report-drift`.
  - L4 local outcome removed gave `FAIL local-outcome-evidence-drift`.

  All positive and re-encoded controls were COMPLETE. The plain-Git baseline gives no evidence
  verdict, so it is `NOT_APPLICABLE` here.

Latency, retries and failures were measured over three runs:

- Every treatment output was deterministic across its three runs, and no command needed a retry.
- Each complete fresh loop took 4 invocations, 2 of them the documented expected first-pass
  failures. The rehearsed loop took a median of 54.3 s (34.6 s to 102.5 s).
- Median `context` time was 2.9 s against 0.5 s for the baseline. Median `affected` time was 2.6 s
  against 0.05 s. Median `cem status` time was 0.33 s to 1.0 s.
- Host load averaged 60-67 on 12 CPUs, so latency is descriptive only and no p95 claim is made.

Complete task tokens and human failure rate are `NOT_OBSERVED`, because no live model-driven agent
or human reviewer ran. The harness accepts them through `--agent-observations`. The result reads
"measured, no savings claim".

The completion receipt validates against a scratch ledger copy with `conformance/use-cases-v0`
(`valid: true`). The two FAIL receipts are refused there only as `result-not-pass` and
`outcome-not-pass`. `conformance/use-cases-v0/ledger.json` is unchanged, since that is V1-0011
scope. The full gate is `NOT_RUN` by owner policy.
## 2026-09-23 V1-0024, decision 0361, FPK-V0-041..FPK-V0-049: failure-reproduction bundles

Ticket V1-0024 adds `prove --export-bundle` and `prove --replay-bundle FILE`. These are
experimental and proposed, not accepted. There is no new root verb. Export is an explicit opt-in
for query and checkpoint results on a clean worktree. It writes one canonical JSON bundle (at most
8 MiB, string values secret-screened, self-digested) to stdout. Replay recomputes the frozen
request on another checkout and reports `reproduced`, or `diverged` with the differing member
paths, or refuses with a named code. The declared exclusions are `error.message` and
`proof.ledger`. Both documents carry `historical: true`, and `prove-observe` now refuses any
document with a `historical` member.

Two findings shaped the contract. First, screening the whole encoded receipt as one string
matched the `{"history-consistent":{"PASS":N}` counts as a credential assignment, so every cited
bundle was refused. Only string values are screened now. Second, the checkpoint parser does not
resolve symlinks, but the query parser does. A root-equality check on replay therefore refused
`/tmp` against `/private/tmp`. It was removed, because the wrapped parsers already refuse `--root`
after `prove`.

AC5 real reproduction on a second plain clone. The binary was
`go build -o $S/corvint-ac5 ./cmd/corvint` at `d88342b955b8246613fe22e62926380c3fc0eee5` (tree
`439ea68b2ce9357654117b6335a19cab3433d0c6`), and it reports `Corvint 0.7.0 (build 0)`. `$S` is a
scratch directory outside the repository.

- `corvint-ac5 prove --export-bundle --task "add OAuth2 login with Google to the web dashboard"`
  was run in the clean first clone. It exited 0 with a 5021-byte bundle, `bundle_sha256`
  `d87ecc12...5b8bc3ad`. The original receipt is the context failure `state: OUT_OF_SCOPE`,
  `receipt_sha256` `be7e6a5c...0ce37ee1`, with no cited blobs.
- `corvint-ac5 prove --export-bundle --checkpoint $S/ac5-checkpoint.json` was run against a
  document of `{"version":"corvint-checkpoint/0"}`. It exited 0 with a 794-byte bundle,
  `bundle_sha256` `0cb8f977...b11fedf6`. The original is the refusal
  `invalid-checkpoint-document` (exit 2).
- `git clone -q <first clone> $S/ac5-second` produced HEAD `d88342b9...`. It has no `.git/commondir`,
  so it is not a linked worktree. The checkpoint input file was then deleted.
- In `$S/ac5-second`, `corvint-ac5 prove --replay-bundle $S/ac5-query.bundle` exited 0 with
  `outcome: "reproduced"`, `differences: []`, `historical: true`, and profile
  `corvint-failure-replay/0`. The same command for `$S/ac5-checkpoint.bundle` also exited 0,
  `reproduced`, from the frozen bytes.
- A cited receipt also reproduced. `prove --export-bundle --task "replay a failure bundle on a
  second checkout"` was run in the clean `$S/ac5-second` and gave `state: READY`, 14 cited blobs,
  18140 bytes, and `bundle_sha256` `92ed8a1f...7cc0f2d8`. `prove --replay-bundle` in a third plain
  clone, `$S/ac5-third`, exited 0 `reproduced`.
- Hostile cases on the same clone:
  - a version edit without resealing exited 2 `tampered`;
  - a tracked-file edit exited 2 `mixed-worktree`;
  - `corvint-ac5 prove-observe` given the bundle and given the replay report each exited 2
    `invalid-proof-document`, and no `.corvint/self-observations.jsonl` was created.

Independent review of PR #86 returned FIX-FIRST with two defects, both fixed in a follow-up
commit:
- Replay checked only that `repository.blobs` ids were well-formed and present. A resealed
  bundle with a cited blob moved to another path, with `blobs: []`, or with an extra
  `../../etc/passwd` entry still reported `reproduced`. Replay now refuses `tampered` unless the
  blob list equals the set the original receipt cites.
- `prove-observe` accepted `"historical": null`. It now refuses any document with the member,
  whatever its value.

The new test cases fail without the fixes and pass with them.

Focused hostile-input tests are in `cmd/corvint/prove_bundle_test.go`. They cover 19 replay
refusal cases, including drift and mixed worktree, plus divergence, export refusals, the size
bound, and the `prove-observe` historical refusal.

Verification. `corvint affected --base 1894b9e5` selected only
`go:github.com/Beamfall/corvint/cmd/corvint`, with 18 unknowns: 13 language-frontier and 5
docs paths. The following passed:
- `GOTOOLCHAIN=local go vet ./cmd/corvint`;
- the focused-docs targets (`spec-requirements-check`, `requirement-definitions-check`,
  `traceability-tests-check`, `decision-numbers-check`, `line-citations-check`);
- `make error-code-ownership-check`;
- `make interop-gate`, which took 21 s.

`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./cmd/corvint` also passed, in 245 s, on the
tree committed as `d88342b9`.

The help.go insertion moved lines, so three existing line citations were repinned: two in
`FRONTIER-DECISION-BRIEF-2026-08-29.md`, and the `prove_checkpoint.go` spans in
`falsifiable-packet-v0.md`.

Not produced:
- `make gate`: NOT_RUN (owner policy).
- `full-gate`: NOT_RUN.
- Independent review of the bundle contract: NOT_PRODUCED.
- A replay across two different Corvint builds of the same version: NOT_OBSERVED. `build` is
  recorded but not compared.
## 2026-09-23 V1-0010 DCW-V0-013..015: daily change-evidence adopter path

`docs/DOGFOOD.md` now opens with one ordered daily adopter path from the pre-change receipts to the
seal, with each input's exact format, the expected state after every step and a fail-closed table.
`dogfood-change` follows each not-complete row caused by an input mistake or by the uncommitted
sidecar with a `fix:` line; `dogfood-check` adds one to `dogfood-report-missing`,
`dogfood-report-drift` (separate lines for a report bound to another base or head and for an
incomplete report) and `intent-scope-drift`. No reason code changed. `docs/INSTALL.md` points to the
path. The shell test asserts every new line and fails when one is altered (mutation observed).
Dogfooding this change showed that before the sidecar commit `cem-status` is `not-ready` with an
empty `policyIssues` and `excluded-artifact-mismatch` in `verification.issues`, so its hint names both.
Amending the implementation commit after the first pass stranded the trace that pass recorded: the
next pass refused `prechange-query: unsupported-query-trace-state` and `local-outcome: record-failed`
with `local trace store contains unreachable revision`. Restoring that commit as an ancestor
(`git reset --soft` onto it, then a new commit) cleared both without touching the trace store, so the
path now says to add commits and the coordinator names the cause. No supported command removes a
stranded trace; that recovery gap is reported, not fixed here.

Friction that motivated the change came from seven fresh-agent worker runs that each bound and
sealed a real change from plain clones of the public repository (PRs #74 to #80): the intents file
must name specs with exactly one `## Requirements` heading; `DOGFOOD_CITATIONS` is a path; the first
passes always fail `excluded-artifact-mismatch` until the sidecar is committed; the base carries an
unsealed 0.6.0-integration CEM whose `baseRevision` is on the private lineage (decision 0331), so
every check prints `unbound-commits NOT_OBSERVED previous-cem-base-unavailable` and every seal
removes that path; and OCM aggregates report every requirement `unassessed`.

Each fail-closed class was reproduced in a scratch clone at base 1894b9e: dirty (tracked and
untracked), stale (commit after the report), unknown (uncited hunk), interrupted (`SIGTERM`, exit
143, no report), unsupported (`PATH` without `rg`; intent without a Requirements heading), aggregate
drift after `ocm mark` without a rerun, and both sealed refusals. `ocm mark` after the sidecar commit
survives a rerun on the same `HEAD` and is dropped by any later commit. A linked worktree was not
refused. NOT_OBSERVED: `SIGINT` interruption, `ocm link` with a test claim, and the independent
reviewer leg of `DCW-V0-006` (no reviewer report exists for these runs); the worker runs used Git
clones, not extracted release archives. The pre-change query ranked
`docs/specs/analyzer-capability-contract-v0.md` first and omitted both `docs/DOGFOOD.md` and the
owning spec: a context miss. The required full gate is NOT_RUN by owner policy.

Review of PR #83 returned FIX-FIRST, reproduced by the reviewer and re-reproduced here in a scratch
clone at base 0ed41f2 (sealed, so no tracked shared sidecar): the first pass lists only
`cem-cite: citation-plan-not-provided` and `cem-status: not-ready` (`max-unknown-exceeded`) with
`?? .corvint/change.cem.json`, and the cited rerun lists only `local-outcome: record-index-failed`.
The OCM refusals and ` M` state documented first occur only while `BASE` tracks an unsealed sidecar,
so steps 3 and 4 now describe both cases. The worktree hint no longer claims the sidecar is the cause,
since `record-index-failed` and `unsupported-impact-worktree` also follow any other uncommitted file,
and `ocm-status-NNN` rows, which printed no `fix:` line, now name the matching `ocm-prepare` row first.
## 2026-09-23 V1-0023 ESV-V0-005, ESV-V0-008..010, TCP-V0-024 / decision 0364: opt-in evidence summaries with exact drill-down

Ticket V1-0023 adds two opt-in views to the existing `context` verb. It adds no root verb, changes
no `protocol/**` wire, and makes no `query` change.

- `--summary [--summary-bytes N]` projects the exact default stdout into at most N bytes.
  - Every non-`results` member is kept verbatim.
  - Rows are compact, each with a `cv1:TREE:BLOB:RANGE:PATH` handle.
  - The `summary` member records totals, the full packet's sha256, `evidence_complete: "UNKNOWN"`
    and a continuation route.
  - It refuses rather than drop a critical row.
- `--expand HANDLE` returns only the pinned Git-object bytes and recomputes the blob object ID.
  - It refuses invalid, stale, missing and ambiguous handles, never substituting current content.
  - It refuses hostile handles (traversal, absolute path, leading dash, control or non-UTF-8 bytes,
    oversize input, huge or reversed ranges) before any Git read.

ESV-V0-005 is resolved. The manifest's `currentState.nativeSourceDigests` freezes the sha256 of
`cmd/corvint/source_handoff.go` and `cmd/corvint/context_summary.go`, and
`TestSourceViewNativeSourceDigestsAreFrozen` recomputes both. `cmd/corvint/host_adapter.go` is
excluded (decision 0364).

Measured:

- `TestContextDefaultWireIsTheGolden`: the default stdout equals a golden captured from a binary
  built at base `1894b9e5` over the same fixture (tree `348e320a`).
- Budget sweeps: output stays at or under budget, and truncation and critical-row refusal are both
  observed.
- `TestContextSummaryAndExpandAreReadOnly`: no `.corvint/` directory and an unchanged
  `git status --porcelain --ignored` after summary, expansion and refusals.
- A self-repository sample at base with `--limit 50` went from 31054 bytes to 7865. The summary
  showed 13 of 50 rows, the 2 critical rows included, and its sha256 matched the full packet.
  This is a byte measurement only, not a task-cost result.

Labels:

- The matched complete-task trial (AC4) is NOT_OBSERVED. It is preregistered in
  `benchmarks/evidence-summary-trial-v0.json`: 19 jobs, 5 impact cases excluded, metrics and a
  decision rule.
- The views stay experimental and do not satisfy the V1-0023 release delivery claim until that
  trial runs and the owner accepts it.
- Full gate NOT_RUN (owner policy).
- Owner acceptance NOT_PRODUCED.
## 2026-09-23 V1-0017 decision 0360 / SOP-V0-003 / SOP-V0-009: cross-version lifecycle and hostile matrix closure

Reproduced the known N-1 failure: `script/check-install-lifecycle.sh` with 0.6.0 (build 90, built
from tag `v0.6.0`) as `CORVINT_LIFECYCLE_BINARY` and 0.7.0 as the upgrade failed `upgrade-b` with
"packet bytes changed across the upgrade". The only diff was the additive 0.7.0 packet fields
`coverage.governance_refused` and evidence `trust` (decision 0346), so the 0341 byte rule could never
pass a wire-changing release. Decision 0360 compares a distinct upgrade with the packet the upgrade
binary builds from a cold index of a clone at the same commit, and reports `packet=identical|changed`.
Same-bytes upgrades and rollback, the downgrade path, keep byte identity. Wrapper case 5 stubs a wire
change (must pass, `packet=changed`) and a nondeterministic packet (must fail `upgrade-b`). PR #84
review found that an upgrade whose read verb exits 0 with no output passed, because two empty packet
files compare equal; the step now requires a non-empty packet and an `ok` cold index, and case 5
adds that stub, which must fail `upgrade-b` with "read verb produced no packet".

Lifecycle on release archives produced from `1894b9e` by `conformance/release-artifact-v0 archive`
(`Corvint 0.7.0 (build 12)`; `shasum -a 256 -c SHA256SUMS` OK for all five archives). Every cell is
the step line of the retained report; each run ended `SUMMARY status=PASS`.

| Step | darwin arm64 same | darwin arm64 N-1 | linux arm64 same | linux arm64 N-1 | darwin amd64 (Rosetta) same | darwin amd64 (Rosetta) N-1 | linux amd64 |
|---|---|---|---|---|---|---|---|
| install-a | ok | ok (0.6.0 b90) | ok | ok (0.6.0 b90) | ok | ok (0.6.0 b90) | NOT_RUN |
| first-index | ok | ok | ok | ok | ok | ok | NOT_RUN |
| upgrade-b | ok same-bytes | ok packet=changed | ok same-bytes | ok packet=changed | ok same-bytes | ok packet=changed | NOT_RUN |
| rollback-a | ok | ok | ok | ok | ok | ok | NOT_RUN |
| uninstall | ok | ok | ok | ok | ok | ok | NOT_RUN |
| backup-restore | ok | ok | ok | ok | ok | ok | NOT_RUN |
| corrupt-truncate | ok | ok | ok | ok | ok | ok | NOT_RUN |
| corrupt-overwrite | ok | ok | ok | ok | ok | ok | NOT_RUN |

linux arm64 ran in the local `golang:1.27.1` container (git 2.47.3), not on native hardware; darwin
amd64 ran under Rosetta 2 on the arm64 host. linux amd64 is NOT_RUN: no amd64 image or host was
available. The N-1 run from 0.6.0 into the installed 0.7.0 (build 46) also passed on darwin arm64.

Hostile matrix (SOP-V0-009): the `memory` row is a Go-heap bound on one index build over a tracked
source 64 times `maxSourceBytes`; about 2.5 MB is allocated, and with the size exclusion mutated
away the test fails at 514 MB. The `case-folds-context-index` row builds two tracked paths that
differ only by case on a case-insensitive worktree; it fails only when both the dirty-set guard and
the worktree blob-oid check are removed, so either alone keeps each path on its own blob. `make
hostile-regressions-check` on darwin arm64: 29 rows PASS, `memory-resident` NOT_COVERED (no
regression bounds whole-process or git child memory). In the linux arm64 container it passed with
both case-fold rows NOT_RUN (case-sensitive filesystem).

AC3: `SECURITY.md` already held the 0.x support window and private reporting channel; `docs/SECURITY.md`
now points to it instead of calling the policy a draft, and on 2026-09-23 the GitHub API reported
private vulnerability reporting enabled (no live report sent). `docs/RELEASE-NOTES.md` is the
changelog; the runbook gains the 0700 output parent, the unqualified Windows zip and the upgrade
report form. The 1.0 support duration stays an owner decision for V1-0021.

Found, not fixed: the build stamp at `origin/main` HEAD is 12 while the published 0.7.0 is build 46,
because the history reset restarted the first-parent count; build numbers are no longer monotonic
(PUB-V0-021). NOT_RUN: full gate (owner policy), linux amd64 lifecycle and hostile matrix, native
linux hardware. No artifact was promoted or published.
## 2026-09-23 CEM-PILOT-024..027: understand, review and CI-verification recipes (V1-0026)

`examples/cem/recipes/` adds three Bash recipes over one committed `BASE..HEAD` change, composed
only from existing commands: `understand-change.sh` (`impact --base`, `affected --base`,
`context`), `review-change.sh` (`cem prepare`, strict `cem status`, a committed-map check,
`cem report`, `review --base`) and `ci-verify.sh` (the V1-0015 `verify-portable.sh`). A shared
`bounded.sh` runs each step in its own process group under `RECIPE_TIMEOUT` and kills it on
recipe exit, `HUP`, `INT` or `TERM`: `TERM`, then `KILL` after a 2-second grace. Understand and review exit 3 on any refusal or incomplete
evidence and keep every step's output; the CI recipe keeps the verifier's 0..5. No decision was
needed: no verb, wire format or gate changed. `make cem-recipes-test` is not a gate member.

Measured locally on darwin/arm64 by `script/cem-recipes_test.sh`, 17 fixture cases, all passing:

- against the installed Corvint 0.7.0 build 46 (`CORVINT_BIN=$(command -v corvint)`): 12.7 s
  and 22.7 s wall in two runs;
- against a build of this checkout (reports `0.7.0 (build 0)`): 17.1 s and 20.4 s wall
  in two runs, including the build.
- The portable verifier is built from the checkout's `interop/cem01-go` in both runs; there is no
  released `ci`-mode verifier to exercise.

Review fix (PR #81): the first cut sent only `TERM`, so a step ignoring it (`trap "" TERM;
sleep 8`, `RECIPE_TIMEOUT=1`) ran 9 s. `KILL` now follows a 2-second grace; the same stub ends in
3 s. The review also led to refusing a non-empty `RECIPE_OUT`, rejecting non-integer
`CEM_MAX_*` with exit 2 before `cem prepare`, and mapping a verdict-less verifier exit to 2. With
three new cases (`b-stubborn` asserts `step=impact exit=124` within 6 s, `b-out-reused`,
`r-bad-cap`; `b-interrupt` now uses the `TERM`-ignoring stub) all 20 cases pass: 14.6 s wall
against the installed 0.7.0 build 46, 15.0 s against the checkout build.

Author single-sample observation, not a reader measurement: in the fixture the review recipe
went from its first run to `complete` in two recovery steps (cite the hunk, commit the map); a
stale map needs one owner decision (`cem prepare --replace`) and new citations.

`NOT_OBSERVED`: time and recovery steps to a first valid receipt, review or CI verification by a
reader unfamiliar with the recipes (V1-0026 acceptance criterion 4).
`NOT_RUN`: `make gate` (owner policy); the recipes on a GitHub runner; a pinned fetch from
`proxy.golang.org`.

## 2026-09-23 V1-0025 LAC-V0-033..036 / decision 0362: console chain pane from hunk to recorded verification

The optional console gains `/chain`. For a sealed change it renders the chain hunk → cited evidence →
governing requirement → recorded verification at pinned revisions. It reads only the sealed CEM at its
object id, the untracked OCM maps under `.corvint/`, the local trace
`.context-corvint/traces/<change>.jsonl`, and Git objects those maps pin. Every edge names the artifact
and field that justify it (for example `hunks[0].basis[0].evidenceId = evidence[0].id`, or
`obligations[0].hunkIds[0]` of an OCM map bound by `targetRevision` and `cem.mapSha256`). Cited spans are
re-read at their `blobOid` and rehashed. An edge the artifacts do not establish renders as a
`missing`, `stale`, `ambiguous`, `unverified` or `unsupported` gap row with its reason.

Measured: `TestConsoleChainComplete`, `TestConsoleChainGaps` (one subtest per class),
`TestConsoleChainTextMatchDecoy`, `TestConsoleChainHostileContent` and
`TestConsoleChainKeyboardNavigation` pass. A deliberate mutation that binds every OCM map regardless of
digest and revision made the decoy test fail on `href="#req-FIX-V0-003"`, so that test discriminates.
Against this repository at base `1894b9e` the built console listed all 35 sealed changes. Change
`f6e68755` rendered its binding and three evidence edges linked, its three requirement edges
`missing`, and its verification `unverified`, because OCM maps and traces are local to the worktree
that sealed a change.

NOT_OBSERVED: the U4 operator-time comparison against the CLI with a human (ticket AC4); no timing is
recorded. NOT_RUN: `make gate` (owner policy) and the companion-release gate. Frontier artifacts are not
consumed; none exist per change. Launch behaviour is untouched, so foreground-process cleanup is
unchanged (`TestConsoleHTTPProcessCleanup`, `internal/console/lifecycle_test.go:168`).
## 2026-09-23 CCF-V1-001..CCF-V1-008, decision 0358: Core compatibility freeze (V1-0007)

Ticket V1-0007 asked for a frozen compatibility boundary for the Core commands without freezing
companion or research profiles by accident. The new contract `docs/specs/core-compatibility-freeze-v1.md`
(intent proposed, delivery experimental) takes the Core set from the accepted decision 0332:
`init`, `adopt`, `index`, `query`, `context`, `impact`, `affected`, `prove`, `cem`, `ocm`, `frontier`
and `dogfood`'s retained local outcome. A first draft used the ticket's seven-verb list; review
corrected it to 0332 because project authority outranks the ticket (invariant 3). The contract lists
the per-mode identifiers, the refusal envelope, exit classes and code families, the admission,
freshness, omission and abstention members, a breaking-change rule, and the N-1 policy. The N-1
baseline is 0.7.0; `git diff v0.7.0 1894b9e -- cmd/corvint internal` is empty, so no Core state or
profile changed before this change. Decision 0358 records the policy. Root help gains a
`Command maturity:` section (the `commandMaturityHelp` const, concatenated into `rootHelp` after every
anchored help.go line citation; help.go grows from 1119 to 1140 lines). It lists the twelve Core
verbs and labels the other 30 `topLevelCommands` verbs Experimental with an indexed owning spec prefix.
A first draft claimed no verb is hidden; review found that `runContext` dispatches `native-hook`,
`authority-event` and `qualified-event` before the `topLevelCommands` check. They are now recorded as
undocumented adapter plumbing outside the freeze and pinned by a source-scan test.

Measured: `cmd/corvint/core_freeze_test.go` passes four tests with 27 subtests (22 Core modes,
5 refusals). The five added verbs pin `index` (`corvint-index-snapshot/1`, and the `--if-stale` fresh
receipt without `ok` or `profile`), `cem status`/`verify` (`cem/0.2`), `ocm status`/`verify`
(`ocm/0.1-experimental`), `frontier --json` (`frontier/0`, exit 1 when open, no `ok` or `mutates`)
and `dogfood status` (`corvint-local-completion/0`). Negative edits failed the tests as intended: one
dropped a help label and named an unindexed owner, and one added a fifth literal pre-dispatch verb to
`runContext`. The first draft's cli-parity-v0 replay reported parity=104 retired=29 with exit 0, and 15
invocations of the then-seven Core verbs on a scratch fixture were byte-identical under the installed
0.7.0 build 46 and the candidate. Reviewer-observed, not rerun here: `affected --base`,
`prove --base` and `context --task` were byte-identical against installed 0.7.0. The authority-start
query reports `context.mode=query` with `context.intent.id=project-operations`. Owner assignments for
`eval` (REC-V0), `adapter` (AHI), `record` (LTPM-V0) and `test-validity` (MTV-V0) are judgement calls
from spec mentions and remain open to owner correction.

NOT_RUN: `make gate` and the exhaustive `go test ./...` (owner policy; focused tests only).
NOT_RUN: V1-0001 ratification of this contract. NOT_PRODUCED: pinned modes for `cem begin`,
`prepare`, `cite`, `mark`, `report`, `cover`, `discriminate`, `anchor` and `provenance`; `ocm prepare`,
`link`, `mark` and `report`; the frontier human rendering and dynamic test mode; `dogfood begin`,
`verify`, `finish`, `review` and `cancel`; and CCF-V1-005 member lists for the five added verbs.
NOT_PRODUCED: a byte-level 0.7.0 comparison for the five added verbs.
## 2026-09-22 V1-0008 IDX-SNAP-V0-022, IDX-SNAP-V0-023, GENESIS-025: init, adopt and index lifecycle qualification

Ticket V1-0008 asked for three things: the two activation doors, the cold-versus-incremental index
lifecycle, and hostile snapshot states, each qualified with evidence. The three requirements are
proposed and record outcomes the code at base `1894b9e5c992d69a7cbefa4b305485494e5622c4` already
produces. No hostile case panicked or ran unbounded, so no behaviour changed. The corpus is this
repository at that base: 3,819 tracked files, tree `7aa62ddc4c6b1afa9c2cc3d9940d7d6c2632d45f`,
object format sha1.

Hosts. The Darwin host reports `uname -m` = `arm64` and `uname -sr` = `Darwin 25.6.0`. `sw_vers`
gives ProductName macOS, ProductVersion 26.6.2, BuildVersion 25G83. It is an Apple M2 Max with 12
CPUs and 64 GiB, running Apple Git 2.54.0. The Linux run used a `golang:1.27.1` container on that
host's Docker VM with `--network none`: `uname -m` = `aarch64`, `uname -sr` = `Linux
6.8.0-117-generic`, Debian GNU/Linux 13, 6 CPUs, git 2.47.3, go1.27.1 linux/arm64. Both
binaries were built from the base tree.

Activation timing (`GENESIS-025`, AC1). Each door ran 20 times on the corpus from a minimal
environment: `PATH=/usr/bin:/bin` and a scratch `HOME`. Network was denied on Darwin by
`sandbox-exec` with `(deny network*)` and on Linux by `--network none`. No model, account or build
step ran, and every receipt records `model.callCount` 0. p95 is the nearest rank, sample
`ceil(0.95n)` of the sorted samples, so the 19th of 20.

| Host | Door | min | median | p95 | max (s) |
|---|---|---|---|---|---|
| Darwin arm64 | `init` | 0.252 | 0.255 | 0.259 | 0.262 |
| Darwin arm64 | `adopt` | 0.252 | 0.254 | 0.257 | 0.259 |
| Linux arm64 | `init` | 0.579 | 0.685 | 0.843 | 0.893 |
| Linux arm64 | `adopt` | 0.459 | 0.633 | 0.774 | 0.783 |

All 80 runs exited 0 with `ok:true` and a `PARTIAL` receipt. `PARTIAL` comes from six `binary-asset`
gaps (`GENESIS-024`) out of 3,813 `INCLUDED` and 6 `UNSUPPORTED` entries. The receipts cite the
revision and tree. The receipt ID was identical on both hosts:
- `init`: `genesis-inventory:sha256:540e45d3ba2f39742fef72632a36b2606e2dfa994444c802486fde896b87f2ef`
- `adopt`: `genesis-inventory:sha256:b7d91a68c0bb1919cfc2eedac3ff13126184dd37bcf65a6e0d385a88a8fe2f8c`

Fallback. The default activation budget is 120 s, well under ten minutes.
`TestActivationFallsBackToABoundedReceiptWhenGitHangs` asserts this. It also uses a Git wrapper that
hangs after repository open, with a 0.5 s caller deadline. Under that wrapper each door returns
within about 0.6 s. The receipt is `PARTIAL`, still pins revision and tree, and carries the single
gap `git-timeout` with zero model calls.

Review repair: the first version of this test used a 0.5 s wall-clock deadline that also had to
cover the real `rev-parse` calls that open the repository. Under load, independent review saw 57 of
60 runs fail, with `INVALID` + `invalid-repository`, or `INVALID` + `git-timeout` and no revision.
The repaired test cancels only after the wrapper records that it has entered its hang branch, and
the cancelled context reports an expired deadline. Built with `go test -c` and run on the Darwin
host with 8 `yes > /dev/null` burners at load average 50 to 80:
- `-test.count=20`, sequential: 20 of 20 pass, twice.
- 12 parallel copies of `-test.count=5`: 60 of 60 pass.
The burners were killed afterwards. The lifecycle tests now clear `CORVINT_INDEX_SHARDS` and
`CORVINT_SNAPSHOT_FORMAT`, so exported opt-in settings cannot move them off the default gob path.

Finding, recorded and not changed: a Git that hangs during repository open yields the `INVALID`
gap `invalid-repository`, not `git-timeout`. `openRepository` maps a failed layout probe to
`invalid-repository`. The outcome is bounded but names the wrong cause.

Cold versus incremental (`IDX-SNAP-V0-022`, AC2). `TestColdAndIncrementalSnapshotsAreByteIdentical`
indexes the target in a fresh clone. It then indexes it again in a clone that first indexed the
prior commit and moved ahead; that clone must probe not fresh before re-indexing. The test requires
the two served indexes to be byte-identical under a canonical JSON encoding with the worktree fields
cleared.

With `CORVINT_LIFECYCLE_CORPUS` set, the corpus subtest used target `1894b9e5`, prior `7e9b1856`.
It passed three of three runs on Darwin and three of three on Linux arm64. The canonical index was
87,658,954 bytes, sha256 `76b96397439184b02f7173942d76dfb87d5d7d35183a4d8b5c6bd0044ffdbfa9`, on every
run on both hosts. The fixture subtest, covering modify, add, delete and rename, runs unconditionally.

Negative result: the gob file itself is not byte-identical. Two `corvint index` writes of tree
`7aa62ddc` with engine `8084efe883cb0fe3` produced 68,770,546-byte files with sha256 `f3d1a145...`
and `9ec03e48...`, because gob encodes maps in iteration order. The spec Non-goals already state
this. A byte-deterministic file encoding therefore stays NOT_PRODUCED behind the
`deployment-neutral-index-platform-v0.md` format gate.

The default path has no incremental build: a moved repository rebuilds in full. The only
incremental path is blob shards (`IDX-SNAP-V0-016`, proposed/off), which is NOT_RUN here.

Hostile states (`IDX-SNAP-V0-023`, AC3). `TestSnapshotLifecycleHostileStatesHaveBoundedOutcomes`
asserts one exact outcome for each state and saw no panic or error return:
- Unsupported input: the exclusion `source exceeds size bound` for a source over 1,000,000 bytes, an
  unsupported-suffix count of 1 for a PNG, and a binary body under an admitted suffix that is loaded
  but not valid text. The snapshot round-trips equal.
- Corruption: an empty file, a torn header, a torn body, a file one byte short and a garbled header
  each produce a miss on load, probe and compact event load. Restoring the file serves the original
  index again.
- Staleness: produces a miss.
- Dirty state: the hit remains, `DirtyPaths` holds only the modified path, the committed body is
  served, and the snapshot directory is unchanged.
- Rollback (`reset --hard` to a retained prior commit): the hit is identical to the original and the
  probe reports fresh.
Focused tests passed on both hosts.

NOT_RUN:
- Linux amd64, and Linux outside a container VM.
- `make gate` and the full-gate required by the ticket (owner policy).
- The blob-shard incremental path.
- Timing under load, and timing on repositories other than this one.
- A hang during repository open in the timed runs.

NOT_OBSERVED: a qualified external corpus.
## 2026-09-22 V1-0013 / CEM-CB-025 / OCM-V0-015 / CF-V0-034: minimum portable proof wire frozen

Ticket V1-0013, decision 0357. The minimum portable proof wire is frozen as of 2026-09-22:
`cem/0.2` with the N-1 `cem/0.1` reader, `ocm/0.1-experimental`, and `frontier/0` with
`frontier-error/0`. `cem/0.3` stays experimental and outside the frozen minimum.
`protocol/cem-0.2/manifest.json` `status` is now `frozen`, and its new manifest SHA-256
`2ad18195...93cd` is pinned in the packet README and in the native and interop tests.
`internal/cem/workflow/portable_test.go` now reads every packet `legacyMap` through the current
reader (`CEM-CB-025`). It asserts acceptance, `cem/0.1`, non-canonical assurance, and the same
drift and unknowns. The OCM and frontier manifests gain `artifactSha256`, which pins 6 and 32
files, and `states`, which pin the six hostile states. Their runners refuse byte drift, unlisted
files and missing files before any case runs. Tests mutate one pinned file and expect the named
refusal.

A new OCM fixture, `intent-scope-drift`, measured the verifier's real codes. A shifted intent span
or a stale span digest gives `intent-scope-mismatch`. An absent intent blob or a deleted intent
path gives `repository-object-unavailable` (per `OCM-V0-012`), not `intent-scope-mismatch`.

Gaps, stated and not vectored: frontier relocated evidence and inherited CEM `evidence-drift` cannot
be reached through the real producers. `cite` refuses an unstable span, and a moved cited span makes
the evidence file an unbindable hunk. The frontier deleted state and part of its stable, stale,
ambiguous and unknown coverage use hand-authored fixtures that the runner skips by capability. OCM and
frontier have no own-profile N-1, because each is the first frozen version of its wire. Their
upstream N-1 is `cem/0.1`: OCM reads it, and the frontier refuses it with
`unsupported-frontier-context`. The V1-0010 daily-loop dependency remains open. The freeze
promotes no profile.

Moving `CF-V0-034` into `change-frontier-v0.md` shifted two cited line ranges. They were
re-pointed to the same text in `harness-authority-relation-v0.md` and
`change-frontier-profile-1.md`.

Verification: 11 affected packages passed `go test`, and `go vet` passed. The `interop/cem01-go`
tests and vet passed. The focused-docs gate passed. `make gate` (full-gate) and interop-gate were
NOT_RUN, per owner policy for scoped ticket work.
## 2026-09-22 V1-0001 PRS-V1-001..PRS-V1-012: draft Corvint 1.0 scope for owner ratification

Ticket V1-0001 drafts `docs/specs/corvint-1.0-product-and-release-v1.md` (`PRS-V1`), labelled
DRAFT pending owner acceptance. It defines 1.0 Core as the local change-evidence loop (`init`,
`adopt`, `index`, `query`, `context`, `impact`, `affected`, `prove`), the CEM/OCM/frontier proof
wire and the dogfood loop; lists companions; proposes darwin/arm64 and linux/amd64 as Core
platforms, darwin/amd64 and linux/arm64 as FALLBACK and Windows as UNSUPPORTED; classifies every
top-level verb, `cmd/` binary and integration tree at the base commit; and proposes dispositions
for V1-0014 (post-1.0, no interoperability claim), V1-0019 (Core blocker, owner-closable), host
FULL/authority tuples (post-1.0) and `PUB-V0-020`/V1-0004 (companion), each with the
`public-release-v0.md` amendment it needs. Eleven yes/no owner questions close the draft.
`public-release-v0.md` gains only a "Proposed v1 amendment" pointer and `docs/PRODUCT.md` only a
pointer sentence; no `PUB-V0` requirement changes. Owner acceptance: `NOT_PRODUCED` (pending; no
decision file is created for a draft). The linux/amd64 native lifecycle stays `NOT_RUN`, and the
0.7.0 N-1 upgrade failure at SOP-V0-003 `upgrade-b` remains an open 1.0 compatibility blocker.
Verification is the focused-docs gate and the `internal/specindex` tests; no behaviour changed.
## 2026-09-22 V1-0027 EEP-V0-020/021/022: kit 0.2.0 profile contract and two-transport proof

Re-audit of kit 0.1.0 found three open acceptance gaps. The checker's schema comparison used Go's
last-value-wins decoding, so a record repeating its top-level `schema` (first `/3`, then `/1`)
passed a `/1` pin, and unsupported and mismatched profiles shared one reason. The kit's
two-transport comparison ran only in-process, not through a `corvint impact` binary, and no script
proved authoring from the kit directory alone. Kit 0.2.0 (`EEP-V0-020`) versions the kit separately
from provider revisions. It keeps the exact `/0`, `/1`, `/2` window and gives ambiguous (repeated
member, or pinned origin claimed twice), unsupported and mismatched profiles distinct refusals.
`TestProviderKitProfileReasons` pins the checker outcome for each of the valid, stale, malformed,
ambiguous, repository-mismatched and unsupported classes. The standard-library runner (`EEP-V0-021`)
builds a copied `main.go` offline and runs 9 cases through `impact --provider` and
`--provider-command`. Each case produced an equal external section apart from `source`, a core
receipt equal to the no-provider receipt, `mutates` false and its pinned outcome.

Clean-checkout proof (`EEP-V0-022`): `sh examples/evidence-provider/v0/authoring-proof.sh
LOCAL_CLONE 8bd9cc3d979e39129c728cd1d75b0e433ee964a3 CORVINT` (a local clone) sparse-checked-out
only the kit directory, built the runner and provider offline and passed all 9 cases. It passed
against a Corvint built from that commit and against the installed 0.7.0 build 46. A copy authored
as `authored-docs` revision `1.4.2` passed the same runner. Core `impact` still decodes a repeated
member as its last value. Changing that is a Core behaviour change outside this slice and is
reported as a follow-up. Default product unchanged: no file under `cmd/`, no flag, wire or receipt
changed, and `ReadPinned` has no caller outside the kit checker.

Focused verification: `go test` of the kit packages, `internal/extevidence`, `internal/specindex`
and `cmd/corvint`, `go vet` of the same, and the focused-docs gate. Full gate and interop gate
NOT_RUN (owner policy for scoped work). Pre-change dogfood collection NOT_RUN. V1-0013 freeze,
owner acceptance, independent review and external validation NOT_OBSERVED. The fixtures are
synthetic.
## 2026-09-22 CEM-PILOT-020..023 / decision 0356: portable digest-pinned CEM CI verifier (V1-0015)

`interop/cem01-go` gains a `ci` mode. It derives the exact base-to-head patch with the
`docs/CEM-CI.md` profile and reads `.corvint/change.cem.json` from the head tree. `verify` makes
every structural and drift decision. The mode writes one fixed-schema `cem-ci-report/0` line. The
`verify` ABI (CEM-GO-002) is unchanged. `examples/cem/github-actions-portable.yml` and
`verify-portable.sh` pin the module pseudo-version and the built executable's SHA-256, check the
digest before execution, and verify under `unshare --net`. `examples/cem/README.md` is the runbook.

Measured locally on darwin/arm64 with Go 1.27.1:

- `TestCIVerdictsAndExits` gives the eight fixture cases their distinct exits: accepted 0, unsafe
  drift and invalid map 1, map absent and unknown hunk 3, `cem/0.2` 4, base mismatch and absent
  head 5.
- Each report is byte-identical across two runs, is one line under 6 MiB, and contains none of the
  fixture's base or head source lines.
- `TestCIReportWorstCaseBound` keeps a 4096-item, 512-byte-path report under the bound.
- `TestCIPortableWorkflowOffline`:
  - Two fresh-cache `go install -trimpath` runs from a local file proxy produce the same digest.
  - A stub whose digest mismatches exits 2 and is never executed.
  - Under `sandbox-exec (deny network*)`, which refused a probe connection to a loopback listener,
    the script returns the in-process report byte for byte.
- A linux/amd64 cross-compile from the same file proxy produced the same SHA-256 in two fresh
  caches.
- Reviewer-observed, not reproduced by the author: the independent reviewer of PR #75 resolved the
  pseudo-version from `proxy.golang.org`. Two fresh-cache linux/amd64 cross-compiles gave the same
  SHA-256, `42a0a316…294b0e` (abbreviated as reported).

Failed evaluation, retained: a reinstall with `GOPROXY=off` fails because `go install
MODULE@VERSION` looks up deprecation, so it cannot show offline reproducibility.

`NOT_RUN`:
- a fetch from `proxy.golang.org`;
- the GitHub-hosted `sudo -E unshare --net -- setpriv` step;
- the linux `unshare --user --net` branch of the test;
- equality of a darwin cross-compile with a native linux/amd64 build (inferred only).

`NOT_PRODUCED`: a published pseudo-version and executable digest for the workflow placeholders.

V1-0014 (independent producers and consumers) is an external dependency, not a blocker.
## 2026-09-22 V1-0002: roadmap reconciled to the task store as the one execution authority

Status audit and repair, not capability promotion. `ROADMAP.md` now opens by naming the Corvint task
store `.taskman/` as the only live execution status, with its read commands, and declares every
checkbox, `Status` line and selection in the body history as of `1894b9e`. The body is kept in place
because specifications cite it by line (`compat-trial-v0.md`, `compat-replay-runner-v0.md`); the
preamble keeps its seven-line length so those citations still land on the same text. A generated
disposition table at the end maps all 131 `docs/specs/INDEX.json` entries and all 46 shipped verbs
(42 `topLevelCommands` plus four dispatcher-only) to implemented / experimental / proposed / deferred /
superseded / dropped / reading aid, derived from each index `delivery`, `intent` and `supersededBy`,
plus the non-archived tickets whose touch paths name the spec or the verb's source file. Three broken
roadmap paths are repaired (`docs/reviews/nextgen-wave1-2026-09-05.md` to decisions 0073/0075/0078,
`benchmarks/dogfood_measure.py` to `benchmarks/dogfood-measure`, the `workflow-screening-v0` pair).
Both `docs/plans` files gain a one-line header that supersedes them as execution status only; their
owner intent is unchanged. `INDEX.json` `implementation` lists drop 20 retired Python/experiment paths
that no longer exist in the tree; four entries left empty are repointed to the Go packages that cite
their requirement IDs or dispatch them (`internal/genesis` with `cmd/corvint/init_adopt.go`,
`internal/contextindex` with `internal/worktreeimpact`, `benchmarks/dogfood-measure`, `internal/tcq`).
No spec body, intent or delivery value changes. Verified: focused-docs gate, `internal/specindex`,
`internal/console`, `internal/companionrelease` tests and vet. `make gate` `NOT_RUN` (owner policy).
Dogfood: bound with intent `docs/specs/corvint-self-development-v0.md`; every dogfood-change step `PRODUCED`
(20/20 hunks supported), dogfood-check passed and `make dogfood-seal` sealed the CEM. An earlier attempt with
`docs/specs/README.md` and `docs/SPEC-DRIVEN-DEVELOPMENT.md` was refused `invalid-requirements-section`.
V1-0001 (scope ratification) remains open, so the dispositions reflect the index, not ratified 1.0 scope.

## 2026-09-22 AFU-V0-001..AFU-V0-012: experimental web flow understanding

The owner requested application-flow understanding, test-gap mapping and runtime confirmation, then
selected a safe web application and authorized implementation. The proposed AFU-V0 contract remains
experimental: pure `flows` reports compose immutable declared input, literal Playwright assertion
candidates and optional guided Chromium observations. `flows record` exclusively writes screened
caller-owned evidence. The separate companion installs no default runtime dependency and performs
no automatic intent acceptance, ranking change, production crawling or complete-flow claim.

The early end-to-end evaluation found the unchecked-save and missing-viewer-test gaps, discovered
eight structural controls and two test journeys in each healthy fixture, and detected both seeded
backend defects. A client cache deliberately hid failed persistence from reload; the independent
backend probe still contradicted it. A renamed-control/route variant, wrong served identity,
source drift, source-only scanning, private recording and HTTP/WebSocket sentinels passed. General
application accuracy, blinded discovery precision/recall, billed tokens and complete-command savings
remain NOT_OBSERVED. Raw task receipts are retained in the private task evidence directory
`/private/tmp/corvint-application-flows-20260922`; final exact-source checks belong to keyed dogfood
observations, rather than being asserted by this preliminary entry.

Independent plan review required a fresh owned-server nonce and frontend/backend/fixture identity.
The first browser attempt exposed incorrect Git executable/working-directory handling; the next
exposed that procgroup deliberately refuses its broader descendant-qualification profile. Both
failed evaluations were retained. The reviewed boundary instead requires non-PARTIAL owned-group
cleanup, completed browser close and observed server exit, with escaped daemon descendants and
non-HTTP transports explicitly unqualified. Final review found concurrent shutdown and ambiguous
route/role evidence; shared cleanup joins pending launches/repeated signals, conflicting route roles
are refused, and role identity stays caller-declared-unverified. The reviewer accepted both repairs;
real Chromium SIGINT/SIGTERM regressions observed all captured descendants retired.
The frozen repository gate then exposed missing root-help inventory and option-like `--root`
handling in the new dispatcher. The repair adds the command listing and reuses the shared root-value
classifier, checked by the existing all-command regressions. An earlier gate attempt hit Git-reader
timeouts; both the isolated cases and their full package subsequently passed unchanged. Failed
attempts remain in the private evidence; source and CEM are rebound before the next frozen gate.

Corvint self-use: pre-change query and initial dogfood attempt used; the initial no-diff/missing-intent
refusals remain visible. `affected` selected the new core and CLI with the full gate still mandatory.
Nonmutating `prove` retained UNPROVEN with twelve citation checks passing and mutation checks NOT_RUN.
The optional flow route was exercised through real compiled binaries in disposable repositories.
Mutation, retrieval experiments, documentation generation, native-host qualification and service
routes are not applicable to this slice; none is counted as adoption or qualification. Bootstrap
intent and partial requirement coverage retain their actual CEM/OCM dispositions. No promotion.

The public tree starts this log at the 0.4.0a4 alpha. Entries written before publication are internal
working records and are referenced from decisions and specifications as historical context only.

## 2026-09-22 decisions 0331 / 0332: 0.6.0 source reapplied onto the public history

The published `v0.6.0` prerelease (build 90, `a03321028e0254bb8d554a2ca1b70e4349568e5e`) was cut
on the history that predates decision 0331. Following that decision, its changes were reapplied,
not merged: the `v0.5.0a3`..`a033210` diff was applied three-way onto public main as one ordinary
commit, so no pre-snapshot commit enters main's ancestry. Four files conflicted only because both
lines added entries at the same place (this log, two agent-memory files, the spec index table);
each was resolved as a union. The contributor agreement, contribution, licensing and provenance
documents and decisions 0330/0331 keep the public bytes exactly. The 0.6 local-workflow decision is
renumbered 0330 → 0332 with every reference; the `v0.6.0` tag and release body still cite it as
0330. Sealed CEM records from the earlier history keep their original commit identities as
historical context. The build count on this history differs from the published build 90, so a
binary built from this commit is not the qualified artifact and requalifies nothing.

## 2026-09-22 decision 0331: clean public history with private provenance

The owner authorized a clean public snapshot after the existing public visibility and surviving
license grants were explained. The publication base is current public main, which contains 258
commits beyond the earlier licensing branch's base; taking that older branch directly would omit
public fixes. Decision 0330 renumbers the reviewed licensing decision because current main already
owns 0321. The agreement and contribution, licensing, and provenance documents retain the reviewed
bytes; only the two affected legal digests change in the current release manifest.

Independent review requires a verified private mirror and local-history bundle, retention of
working files and released assets, a parentless candidate with an exact allowed-path comparison,
and a guarded remote replacement. The final source gate is bound to the snapshot itself; the
older branch's passing gate is not reused as proof for the newer source tree. Historical receipts
retain their original identities and remain historical context when their objects are absent from
the public root. The Git-derived build count restarts at 1 without creating a new binary release.

The existing prereleases, tags, and additional branches await a separately recorded scope choice
before any withdrawal. GitHub-hosted references and third-party copies may survive a ref rewrite.
The prior manual-policy OCM refusal remains visible; no code or test is invented to assert legal
consent. Exact backup, gate, review, and post-publication outcomes are retained privately and must
be reported with their actual status. No source gate or publication success is claimed by this entry.

## 2026-09-22 decision 0330: preserve commercial licensing options

The owner authorized preserving future commercial licensing without changing the AGPL product or
Apache interoperability boundary, then expressly removed the proposed hired-counsel prerequisite.
The prior contribution policy granted only destination-path terms. Agreement version 1.0 now
provides retained ownership, explicit commercial sublicensing, scoped copyright/patent grants,
moral-rights consent within legal limits, successor duties, and authenticated acceptance of exact
text and contribution commits. Apache-only contributions keep the existing path terms.

The owner-directed agent review used Apache individual/corporate CLAs, Harmony's contributor
template, the GNU FAQ, and the actual public licenses (linked in decision 0330). It identified the
need for reciprocal public-license/record-handling commitments, moral-rights treatment, and a
usable acceptance channel. The result requires a contributor PR statement and authorized project
acknowledgment, with private retention of exact text, contribution bytes, and authority evidence.
This is an agent-performed review, not a professional opinion or guarantee of enforceability.

Independent review found that submitted commit IDs alone leave a squash/rebase gap. The repaired
process retains an accepted-to-merged mapping, checks preserved content, and requires rights for
conflict-resolution edits and other additions. Missing grants keep affected product contributions
on hold; no outside lawyer or signing service is required. A further independent review caught a
privacy instruction that conflicted with intentional public electronic acceptance; it now separates
the public statement from private supporting records. Publication is not a signed acceptance,
retroactive permission, third-party clearance, or authorization for a commercial release.

Corvint query and initial coordination were used in an isolated worktree at base
`cd9fec9ae5ed8199ee3c844884ca4212d3e4031c`. Original private receipts retain initial empty-change
and missing-input refusals. CEM citations identify existing license and ownership constraints,
not proof of the new agreement's legal effect. OCM refused this prose decision with
`invalid-requirements-section`; inventing executable test witnesses for consent would be unsound.
Separate non-Go path impact, affected tests, mutation, and ranking evaluations are inapplicable.
The first full gate was interrupted for the owner's material scope correction before source
changed; captured owned descendants were verified gone. The subsequent gate exposed stale
LICENSING.md and PROVENANCE.md hashes in the release-artifact manifest (`legal digest mismatch`).
That failed run was retained and stopped before refreshing only those two hashes to the exact
reviewed file bytes; public LICENSE texts and archive membership did not change. Final check outcomes remain separately
bound to the eventual clean commit. Professional legal approval and contributor acceptances are
NOT_PRODUCED; professional approval is not a required gate.

## 2026-09-22 MTV-V0-001 / SDD-V0-006: literal test anchors for the 0.6 completion

Native finish at 9e57c41 refused with `ocm-bindings-required` after all eight plan checks passed:
MTV-V0-001, SDD-V0-006, PUB-V0-010 and PUB-V0-020 carried explicit `no-test-claim` marks, which
local completion treats as an assessed gap. The owner selected anchoring: the unchanged
`TestToolCatalogueIsExactlyOneReadOnlyTool` body now runs as case `MTV-V0-001 exactly one
read-only tool`, and the existing docs round-trip profiles are named `SDD-V0-006 docs draft and
consume round trip <version>`, following the MCPV0-011 precedent (8d8eed2). No assertion changes.
PUB-V0-010 and PUB-V0-020 have no extractable test and stay unassessed; their no-test-claim
assessment remains in the review record and release packet. The refused finish receipt is retained.

## 2026-09-22 IPR-03: seed data re-pins the reconciled roadmap digest

The exact-target seed-fixture check refused because 9148240 edited the roadmap header, outcome and
historical-next-action lines without refining `script/seed-planning-store-data.json`. The old pin
is the digest of 9148240^, and all eleven IPR-01..IPR-11 sections the ticket bodies copy are
byte-identical at the new digest, so only `expectedDigestSha256` changes; no ticket text moves.
The failed seed-fixture receipt at a9854aa is retained.

## 2026-09-22 PPI-V0 / decision 0332: protected Pi source joins the 0.6 candidate

The owner directly approved one replacement of the integration enrollment to include the reviewed
protected Pi source as optional experimental FALLBACK. The 19fb7ea generation, its maps and reports
are preserved; it was cancelled once as NON-SUCCESS. The contract commit landed first because Begin
pins every intent at HEAD, then one Begin under the same key adopted the independently reviewed
thirteen-scope, eight-check plan. The runtime and lifecycle commits and the MCPV0-011 literal anchor
follow as source only; every runtime blob matches its reviewed commit, and only build-log, spec
index and citation metadata differ. The added Makefile target shifted three citations, which now
point at the same cited content; the unanchored compat-replay citation had already named the wrong
line and now names `spec-requirements-check`. This authorizes no protected installation, principal
admission, activation, FULL claim or compiled runtime distribution, whose notices remain incomplete.
The earlier entry that excludes the protected runtime records the previous generation's scope.

## 2026-09-22: selected completed source enters the 0.6 candidate

The owner selected the completed installation/recovery, candidate portable-proof, experimental
provider-kit and native Pi tool slices for 0.6, retaining their original qualification limits.
Their source commits, the exact-content gate repair, browser disclosure proof repair and MCP
empty-PID-file fixture repair are integrated without importing another task's CEM or local outcome.
Protected Pi runtime and formal FULL authority remain excluded. The previous integration enrollment
is retained as cancelled non-success; its single replacement keeps the existing base/checks and
adds the four owning scopes plus gate-ledger and releasecandidate race coverage.

Independent combined-source review found no blockers. The early metadata preflight found citation
line drift from the additive host/Makefile changes and an unanchored failure-backlog citation;
relocations preserve the exact cited content and historical references retain their named commit.
Corvint query, path impact and affected planning supplied change context; the unavailable first
impact path remains a retained refusal. Learning and provider ingestion are excluded from this
integration evidence. Final source/CEM binding, exact-target checks, artifacts, native/installed
hosts and the separately governed workflow campaign remain required; no slice result promotes 0.6.

## 2026-09-22 MCPV0-011: lifecycle fixture waits for PID publication

The c0f1eee full gate failed `TestClosedStdoutCancelsInFlightDescendantGroup` with `<nil>` at its
PID-file wait, before the stdout-close and descendant-cleanup assertions. The fixture's shell can
create its PID file before writing the bytes; the reader treated an existing empty file's nil
error as a fatal error. A focused empty-file-to-complete-publication regression reproduced that
failure immediately. Only non-nil unexpected read errors now fail the wait; empty or absent files
keep the existing bounded retry. Malformed PID data, deadlines, process cleanup and MCP runtime
behavior remain unchanged. The regression joins or stops its delayed writer during cleanup.
The failed full-gate receipt is retained; focused lifecycle validation and a fresh exact-candidate
full gate are required, with no scope or enrollment replacement and no relabeling of old evidence.

## 2026-09-22 PPI-V0-005..009: isolated Pi authority source and packaging

The experimental Pi lane adds closed root/3, campaign/2, qualified-pi-host/0 and QLF/2
without changing earlier profile meanings. Both native surfaces require separate evidence
bindings to one admitted image. Runtime checks bind protected full-image bytes, live hardened
code flags/CDHash, immediate parent and process birth, boot, OS/architecture and actual cwd.
Non-Stop reads retain their protected-publication privacy boundary. Candidate campaigns remain
900-second FALLBACK/UNQUALIFIED exercises; they cannot supply completed qualification.

The fixed SDK embeds its consumer hash, checks protected ancestry before spawning, verifies
closed receipts and waits for native idle completion before one permitted follow-up. Independent
review found stale Stop reuse after trust withdrawal, lost ordinary context on missing admission,
and silent recursive unresolved state. Fresh epoch-bound receipts/current trust, separate FALLBACK
requests with explicit degradation, and visible bounded unresolved status repair those findings.
Repair review found no additional issue; cancellation regression confirms no surviving descendant.
Focused Pi/Direct/Qualified Go tests and the rebuilt native TUI/RPC/reload/replacement/image/startup
suite pass. The final handler repair's rebuilt native run is still required at source freeze.

Optional Pi release preparation binds the host, consumer and build manifest in a distinct immutable
release profile. The installer refuses another host's admission; exact-file removal and revoked
reader withdrawal remain explicit. Focused packaging and the optional authority module's ordinary
suite pass; independent packaging review found no actionable issue. No protected state was changed.
Full gate, license/input closure audit, privileged installation, real protected OPEN/EMPTY/recursive
campaign, latency/recall and independent FULL admission remain NOT_PRODUCED/NOT_RUN.

## 2026-09-22 PPI-V0-001..004: closed Pi SDK runtime proof

The owner accepted the reviewed optional protected Pi direction, separately from privileged
installation or completed qualification. Bun standalone executables still accepted executable
`BUN_OPTIONS`/`BUN_BE_BUN` injection despite configuration-autoload switches. The implemented
alternative uses official Node22.23.2 SEA, fixed exec arguments, hardened runtime with only JIT
permission, exact Pi0.85.1 dependencies and embedded assets/worker/WASM. Ordinary Pi remains separate.

Native RPC and TUI prompts, reload, session replacement and a real image read/resize pass while
macOS denies reads of both global and build-time SDK modules. Hostile project/global extension
files remain unexecuted. NODE_OPTIONS, forged argv0, CLI eval/preload, DYLD and OpenSSL injection
negatives pass; executable Node/DYLD/OpenSSL controls prove the canaries work. SIGUSR1 does not
activate the inspector. Harness interruption reaps a TERM-ignoring descendant. These are local
source/runtime checks, not an admitted campaign or completed native qualification.

Independent review identified unresolved lazy OAuth/Bedrock imports and inherited argv keys;
static provider bundling and own-key argument parsing repair both. The repair review found no
additional issue in that boundary. A subsequent async credential-write audit found validation ran
before a Promise resolved. The guarded backend now validates after awaiting, rejects command keys
and credential environment overrides before storage, and preserves literal keys without SDK
interpolation. Explicit nonpersistence and literal-dollar regressions cover the repair.

Failed test attempts remain evidence: the TUI driver initially submitted reload before completion;
the image fixture initially exceeded the harness output cap and incorrectly selected repeated
tool calls. Corrected fixtures wait for visible readiness/bounded completion and use a tiny image
that still requires resizing. No production output limit was weakened. The full gate remains
coordinator-held. Pi authority profiles, independent admission, latency/recall, actual protected
OPEN/EMPTY/recursive behavior, installation and revocation remain NOT_PRODUCED/NOT_RUN.

## 2026-09-22 AHI-025: explicit Pi operations and full-support direction

The owner explicitly requested complete Pi support, including protected authority and formal FULL.
The additive functional slice reuses native context, immutable source-view validation and the
explicit trace writer. It adds bounded in-memory packet handles and typed supplied observations,
without changing legacy lifecycle authority or making automatic outcome writes. Existing enrollment
and its failed canonical query-fixture restoration check remain visible; the next source invalidates
prior checks. The functional slice is independently reviewable and remains separate from release
0.6 integration until selected. Native and focused regression results are retained in the Pi task
checkpoint; unrun qualification stays unclaimed.

Independent review found two functional defects: the explicit slash command discarded uncertain-
write guidance, and the context tool failed to forward its supplied evidence handle. Both are
repaired and the repair-only review found no remaining required issue. Focused native Pi/source
checks, 21 JavaScript regressions, and four actual Pi host/cleanup checks pass. Actual host evidence
includes print-mode query/expansion/edit/verification/explicit recording, RPC new-session recovery,
and native TUI prompt/shutdown. Canonical verification is queued against the frozen next target;
these focused results are not a full gate or protected qualification.

The protected-runtime feasibility review found the installed Node/JavaScript Pi cannot inherit the
Codex-only direct admission. The official Pi 0.85.1 standalone darwin-arm64 archive is a concrete
candidate, but immutable mapped code, external extension/resource closure and actual native launch
must be proved before a Pi-specific technical profile or execution-root admission is accepted.

## 2026-09-22 EEP-V0-016/017/018: frozen kit gate failed, no retry

The one enrolled full gate on `b835a7464836ee8d25ea828de0f2921ad672a254` failed with exit 2
(no timeout or cancellation). `internal/specindex` rejected the kit's overlong INDEX claim and
nonidentical INDEX/digest/README metadata. The repair restores the original bounded claim and
synchronizes the delivery/status copies; requirement semantics and executable source are unchanged.
The existing `TestIndexCoversSpecsAndHeaders` is the focused regression for that repair.

The same run separately failed the existing Core
`TestRepositoryQueryTraceStateFailuresAreTypedAndNonmutating/oversized` at the pre-query
`repositoryBytesDigest`: a temporary Git pack index disappeared during `lstat`, followed by a
TempDir `.git` directory-not-empty cleanup error. Cause remains UNKNOWN; no isolated retries or
Core repair were performed in this kit slice. Full `internal/extevidence` and `tools/gate-ledger`
packages passed in this run, which is distinct from the portable-proof gate-ledger failure.
The frozen logs and leftover fixture are retained in the private task checkpoint. Recorded gate
process handles exited. No further full gate is authorized; final completion and seal remain
blocked. A metadata repair does not turn the failed frozen gate into PASS.

## 2026-09-22 EEP-V0-016/017/018, EEP-TR-011: experimental local provider authoring kit

V1-0027 adds kit 0.1.0: a single-file standard-library Go provider and a separately built checker
that reuses existing strict record decoders and contained command execution. Consumer pins bind
exact schema, provider identity, repository revision/root and command executable SHA-256. No Core
flag/wire/version, authority, installation or transport promotion changes. The experimental record
window is exactly `/0`, `/1`, `/2` with today's consumer, not historical engine compatibility.
Provider/consumer examples and implementation remain AGPL; no Apache boundary expansion.

The smallest complete proof copied and authored the provider in scratch, built it offline with
local Go 1.27.1, and compared its `/0`, `/1`, `/2` file and command composition. The documented
focused command passed across kit, extevidence, procgroup and Core packages, including the existing
valid/stale/malformed/ambiguous/repository-mismatch/unsupported cases, exact-pin refusals, Core
separation, timeout/output/environment bounds and descendant interruption cleanup. An initial
fixture expectation used `app` where the retained fixture declares `application`; correcting the
test restored agreement. Independent Sol/low review found no HIGH/MED; stale digest wording was
corrected. The inherited test fixtures remain synthetic, not external validation.

V1-0013's portable proof freeze (itself awaiting V1-0010), incomplete native ticket coverage and
owner acceptance remain open. Kit MCP integration stays proposed/out of scope without downgrading
the already accepted separate MCP profile. Full/interop gate and CEM/OCM completion evidence are
produced after the source freeze; this entry does not claim those pending gates passed. Pre-change
dogfood at the empty base-to-HEAD range retained `cem-prepare git-diff-failed`, missing CEM/citations,
missing intent scope and missing outcome inputs. No billed-token or before-first-query measurement
was available; no efficiency or promotion claim follows. OCM correctly rejected hyphen-adjacent
requirement IDs in the new test labels as non-exact anchors; labels now use whitespace delimiters.
The source target was refrozen before any full gate.

## 2026-09-22 V1-0013 / CEM-CB-003: candidate portable packet finds cross-hunk interop defect

The candidate `protocol/cem-0.2` packet pins a synthetic SHA-1 base, six exact target commits
including their raw CEM sidecars, and 21 artifacts. Formula-derived expectations cover every drift
state, ordered mixed evidence and explicit unknowns. Native status verifies canonical authority and
sidecar bytes and refuses zero-unknown completion for the unknown case. The separate historical
reader rejects 0.2 and consumes independently decoded, pinned 0.1 exact-patch equivalents; the
original frozen 32-case matrix and wire profiles are unchanged.

The first historical-reader run rejected legal evidence/relation reuse across different hunks as
`duplicate-basis`. Its pair set lived outside the hunk loop. Independent contract review confirmed
that supported-hunk uniqueness is local to the hunk; moving the set preserves same-hunk rejection.
The supplemental process-boundary regression exercises both cases. This is reference portability
evidence, not independently authored 0.2 interoperability or a semantic-support claim.

Pre-change enrollment uses base `ab5310cb4f00d15c33fe112c0e0335fe28f9db20`. The original Corvint
query returned an unrelated accepted-spec lead and explicit omissions; scoped original contracts
supplied context. Initial `dogfood-change` retained `git-diff-failed` because base and target were
identical before implementation, missing intent-file input and absent outcome input; the corrected
enrollment retains the actual CEM intent. Measurement-before-first-query and billed task costs are
NOT_OBSERVED. `affected` retains nested-module, unowned-vector and language-frontier unknowns.
Applicable routes are query, affected, CEM/OCM, frontier and enrolled completion. Provider, mutation,
learning/retrieval evaluations and service routes are not applicable to this packet/reader repair.

Focused native and independent-reader vectors passed after the repair; the integrated independent
diff review found no blockers. Full/interop gates and
final CEM/OCM/report review are required on the final committed target; retain their exact receipts
in the task's private evidence rather than treating these focused passes as those gates. The native
store remains fixture-only with V1-0010 open and coverage incomplete. Formal minimum-wire freeze,
independently authored 0.2 consumer/producer, OCM/frontier portability qualification and owner
acceptance remain blockers. No version, release candidate, publication or runtime authority changes.

## 2026-09-22 PUB-V0-023/025/026: V1-0017 operational prerequisites

The owner's parallel V1-0017 instruction authorizes a prerequisite slice, not ticket completion or
future-release qualification. Native ticket audit retained V1-0008/V1-0015 OPEN, coverage INCOMPLETE,
and actor authentication, historical acceptance, runtime qualification, liveness and publication
NOT_OBSERVED. The initial native query abstained below its relevance floor; targeted Go impact and
original installer/spec sources supplied context. No current release candidate or user installation
was modified.

Audit found that candidate and installed version probes used unbounded `exec.Output` without owned
descendant cleanup, and installation followed static symlinked store components. Existing procgroup
supervision now bounds both probes and suppresses child output in errors; store admission refuses
symlink components, aliases, overlap and invalid existing paths before effects. Candidate input
materialization is bounded to its existing closed set. Independent plan review rejected WalkDir's
unbounded pre-callback enumeration; bounded ReadDir fixes that before implementation. Initial focused
fixtures caught sibling checks extending into a large unrelated temporary ancestor; checks now cover
the store name and managed descendants. No new release format, service or persisted-state migration
was introduced. Removing this change restores the preceding installer; retained candidates and
older installs need no migration.

Temporary shell/archive fixtures exercise coexistence, explicit rollback, corrupt-destination
refusal, backup reinstall, scoped removal, hostile paths, bounded probes and joined interruption.
These are mechanism evidence, not genuine future release artifact/native-platform qualification.
Focused package tests passed (2.015s), race tests passed (3.543s), and focused vet plus specification
checks passed. Independent final review found one test-only PID-reuse cleanup hazard and one
missing exact-version/nonzero-exit fixture. Identity-bound cleanup after cancel/join and the new
fixture passed targeted race tests (3.414s); the same reviewer accepted the repair with no remaining
findings. The release orchestrator explicitly holds the terminal full gate behind current release
qualification. It remains NOT_RUN here; native finish/seal and exact future-artifact qualification
remain pending. Exact source/CEM/OCM handles are retained privately for continuation. See
[the runbook](RELEASE-RUNBOOK.md) for remaining artifact/platform, support policy and predecessor
requirements. Existing alpha security policy is preserved; stable promises remain drafts. Store
ownership is exclusive; concurrent hostile renames, escaped groups and power-loss durability are
explicitly unqualified.

## 2026-09-22 PUB-V0-020 / GL-V0-001: approved patch qualification repairs

The owner approved the combined repair packet and exact replacement enrollment, retaining the
original enrollment as cancelled NON-SUCCESS and preserving all failed receipts. The patch adopts
only the reviewed statless-index source and requirement-anchor delimiters. Candidate c85881a's
installed browser proof omitted opening the existing roadmap disclosure before checking its text;
the corrected proof clicks that disclosure, retains all six assertions and reports missing text.
Primary failures are logged before unchanged cleanup. The temporary corrected browser proof passed
but does not qualify the original candidate. The prior installed attempt with a mode-0644 local
authority attachment was also retained as a failed runner setup; its corrected attachment is 0600.
A combined-source review, fresh immutable CEM, all selected checks and new source-bound installed
and OpenCode qualification remain required before exact-packet publication approval.

## 2026-09-22 GL-V0-001/005: exact bytes without cached index stats

A deterministic fixture reproduced a false ledger HIT and stale `one\n` blob after a same-size
`two\n` edit with restored mtime under coarse Git stat settings. The newer-index variant also
failed, so preserving only the copied index timestamp is insufficient. The original full-gate
failure's exact timing/configuration remains unknown; its failed source/enrollment is preserved.
Independent plan review selected a fresh private index imported from Git's NUL-delimited
mode/object/stage/path entries, retaining tracked membership while discarding cached stat data.
Private commands disable fsmonitor and ignorestat; the original index bytes and mtime stay intact.
Tests cover restored timestamps, ignored tracked and intent-to-add content, staged changes,
deletions, unusual paths, executable/symlink modes, unborn and linked worktrees, refusals and
failed-import cleanup. An initial cleanup assertion included Apple's unrelated `xcrun_db` cache;
it was corrected to assert only the ledger-owned private index/lock names. Corvint pre-change
query/impact, dirty affected/path impact and enrolled CEM/OCM are the applicable self-use routes;
learning/evaluation/provider routes are not applicable. Focused checks qualify this repair only;
the mandatory full gate remains pending on the coordinator's frozen integrated Core target.

## 2026-09-22 PUB-V0-001: prepare the 0.6.0 candidate version tuple

The owner-selected candidate moves the native version, archive smoke, VS Code exact admission
and live fixture together. Existing tuple and historical-identity tests carry explicit PUB-V0-001
claim anchors for OCM review. The draft notes retain UNPROVEN jobs and pending candidate gates;
no historical evidence, first-parent build calculation or optional alpha parser changes.
Independent review caught stale executable-test fixtures and the VSC-V0-007 admission clause;
the repair updates both, the extension README and the selected editor checks.

The retained prechange query located accepted decision 0072, with four ranked results omitted
and test symbols withheld; original sources and accepted decision 0332 supplied scope. The initial
empty-diff coordinator refused CEM preparation, as expected. This slice uses query, affected,
CEM/OCM/frontier and keyed completion; native/archive/host and sealed measurement are deferred
until the integrated candidate freezes. No paired savings or milestone qualification is claimed.

## 2026-09-22 decision 0332: Core candidate and evidence packet clarification

The accepted Core-only 0.6 scope now distinguishes immutable tested candidate `T` from
later evidence-only publication snapshot `E`. The public-release contract maps the
retained `PUB-V0-022..026` Core safeguards to existing native archive/report/checksum
inputs and keeps optional combined alpha machinery separate. The daily-workflow
acceptance and rollback clauses retain all six evidence classes, exact candidate
invalidation and literal archived `UNPROVEN` claims. This is a development contract
clarification, not candidate qualification, ledger promotion or publication.

## 2026-09-22 decision 0332: 0.6 portfolio and native status reconciliation

The 127-entry index and installed 0.5.0a3 build 45 command help were inventoried into
[the 0.6 portfolio](PORTFOLIO-0.6.md); the private coverage comparison retains both inventories.
Core is a restricted qualification target, not delivered intent or promotion. An initialized native
`.taskman` store supplies current execution status in the workspace where it exists; this checkout
has none, so the historical AT/E/U roadmap cannot serve as a live queue. The original directory's
fixture queue is `queue:corvint:main`; no V1 ticket completion is inferred. The broad integrated
outcome remains historical owner intent outside the accepted 0.6 Core prerequisite set.

Prechange query and full-base range impact returned receipts. Keyed enrollment returned
`operation-in-progress` while keyed status was inactive; initial `make dogfood-change` could not
write the linked Git directory under the sandbox. The development worker continued after that refusal, so its full workflow trial is incomplete.
The coordinator preserved the exact patch and restored its owned files before retrying enrollment.
The sealed prior worktree then correctly refused stale prior completion; a fresh isolated leaf was
enrolled from the sealed base before replaying the patch. No lifecycle record was removed or relabelled.
Command-owner mappings and supporting-gate classifications were corrected before review.
The worker's five focused documentation checks passed. Fresh Astra/medium review found no HIGH/MED
findings after the repair; final binding and enrolled checks follow. Raw worker/reviewer usage and
all refusals remain private development evidence, not sealed cost or savings evidence. The final
enrolled check exposed four shifted roadmap line citations; each was moved one line after an exact
old/new passage comparison, preserving the original AT-09/10 meaning and the failed check.

## 2026-09-22 DCW-V0 / UCV0-013: owner-selected 0.6 local-workflow scope

Decision 0332 records explicit owner acceptance of three narrowly scoped daily-workflow jobs,
retaining six evidence classes and separating installed Codex/Claude use from formal FULL authority.
The canonical ledger moves to `/1`; exact historical `/0` admission and bytes remain supported.
All twenty-two rows remain specified/UNPROVEN. No milestone, host or platform is promoted.

Independent Gate A review found the closed-ID compatibility risk, alpha-only companion-required
candidate admission, and freeze/enrollment sequencing constraints. The first slice resolves the
ledger contract and enrolls existing intent before editing; later candidate and evaluation work
remains required. Original query on the dirty primary abstained with unindexed-worktree-changes.
Initial no-diff dogfood preparation reported git-diff-failed/missing-intent-scope; enrollment then
exposed the required lexical intent order and succeeded after canonical ordering. These failed
attempts are retained in the private task evidence, not reclassified as successes.

Corvint feature routes used: query/index, tracked-Go path impact for the validator/tests, dirty-change
affected advice and keyed dogfood. The affected advice retains the terminal repository gate and
unknown scope; it does not replace that gate. No ranking change, sealed corpus access, mutation
trial or optional service is needed for this slice. Final CEM/OCM/frontier inspection remains required.
Focused conformance tests and the spec/requirement/traceability/decision/citation checks passed.
The actual historical/current reader matrix accepts historical `/0` in both readers, accepts `/1`
only in the new reader, and observes explicit `wrong-spec` from the old reader. Independent
implementation review found no blocker; its documentation corrections are included. No full gate,
workflow qualification or savings claim is reported.

## 2026-09-22 PUB-V0-001: v0.5.0a3 published

Tag `v0.5.0a3` points at `822888a`, the merge of pull request #59 (`codex/release-v050a3`), and the
GitHub prerelease carries the four non-Windows archives plus the gate-produced `SHA256SUMS`, all
built from the gated candidate `f441e96` with digests equal to the private archive witness
(decision 0329, publication record). Evidence chain: `make gate` exit 0 on clean `f441e96`
(Go-archive `verdict=PASS`); CEM bind `f441e96` (116 supported hunks, OCM linkage 0/26 unknown,
recorded as `NOT_OBSERVED` for the base window); `dogfood-check` PASS; seal `cc8792a`; independent
review with no blocker. Two limitations are recorded rather than hidden: the local trace record
`.context-corvint/traces/6b50c2e7....jsonl` from a discarded earlier bind commit was moved out of
the worktree so the rebind could read the trace store (`unsupported-query-trace-state`), and the
`cmd/corvint/query_test.go:139` flake seen once on the #56 gate did not recur and remains unproven.
Issues #53, #54, #55, #56 and #57 closed on merge. `v0.5.0a2` is unchanged.

## 2026-09-21 audit fix batch: thirteen defects closed before the v0.5.0a3 candidate gate

A pre-release audit of `cmd/corvint`, `internal/contextindex`, `internal/jstestprovider`,
`internal/behaviorfalsify` and `internal/doccorpus` recorded fifteen defects in
`docs/agent-memory/bugs.md`; thirteen are fixed on the candidate, each with a focused regression
test. Contract-visible changes: `dogfood-ocm`, `taskman-fixture` and `corpus` emit an
`output-failed` envelope when their own output cannot be written; the `corpus` `--root` preamble
refuses an option-like value (other native values stay uninterrupted because DCP-V1-012/018 require
`--task --corpus=x` to pass through); BBF-V0-010 now states that the declared wall-clock budget must
exceed the executor's cleanup reserve, and the per-receipt output limit has a floor equal to the
validator's accepted receipt bound; JLTP `report-not-written` also covers a Vitest report over the
4 MiB bounded-report limit and external config input over 256 entries is refused with
`report-output-overflow`; sensitive-input redaction orders values longest-first, and the
`sensitive-input-finding-bound-exceeded` slot-63 overwrite is retained as the spec-listed cap
behavior rather than treated as a defect; DCP-V1 reverse-link keys use a NUL separator, so
`lost_reverse_links` strings now carry `\u0000` between their parts, `missing-reverse-link` detail
ends with the test ID, and `artifacts()` refuses an observation whose input is unretained, whose
digest differs from its run, or whose input already serves another artifact role. Context-index
production edits move the analyzer schema to `corvint-analyzer/73` with a re-pinned input digest.
Not fixed: the DCP-V1-032 `--previous` refusal of a bundle carrying a real reconciliation finding
is an owner decision (`docs/agent-memory/questions.md`), and the `CPUPROFILE` read-command write
knob stays in `docs/agent-memory/fixes.md`. The 32-bit symbol-window overflow fix is confirmed by
inspection only; no 32-bit build was run.

## 2026-09-21 PUB-V0-001: v0.5.0a3 candidate integrates issues #53–#57

Decision 0329 moves the version tuple to `0.5.0a3` for the rerelease that integrates the sealed
issue branches #53, #54, #55, #56 and #57 plus `main`'s gate ledger. The token is the next alpha
increment; it awaits the owner's confirmation before any tag is pushed. Each issue branch carries
its own enrolled local-completion or recorded gate evidence and a sealed CEM under
`.corvint/changes/`; the integrated candidate must additionally pass the full repository gate on
its exact clean commit before publication. Unsigned prerelease, `NOT_VERIFIED` publisher identity,
four non-Windows archives plus gate-produced `SHA256SUMS`, no companion, and `NOT_RUN` live
Playwright `/2` and external MCP host qualification are retained unchanged.

## 2026-09-21 GLTP-V0-048/049: lifecycle test deadline and joined shutdown

`TestRunningFailedPassed` used the production-like fresh `GOCACHE` with both its runner and outer
terminal-event waits fixed at 20 seconds. The observed full-suite timeout is consistent with cold
compilation exhausting that budget. A fatal wait also cancelled the session without joining `Run`,
allowing temporary-file cleanup to race the runner. The test harness now sets the specified
`GOENV=off`, uses a five-minute
per-run hang-detector budget, fails immediately with the complete event sequence on an unexpected
terminal state, and unconditionally cancels and joins `Run` before `TempDir` cleanup. Production
defaults and session behavior are unchanged. The complete session package passed in 35.808 seconds,
and ten serial repetitions of the exact lifecycle test passed in 39.069 seconds.

## 2026-09-21 LTA-V0-004: verbose Go PASS marker exception

The pre-change Corvint query selected an unrelated decision and omitted the governing writer-screen
intent. Direct inspection found that the generic bare-`pass` assignment branch classified exact Go
verbose-test marker lines as secrets. The writer screen now masks only the structural `PASS:` prefix on complete
`[whitespace]--- PASS: TestName (seconds)` lines while still detecting a real `pass: value`, token, or other secret
inside the test name or elsewhere in the same output; `StoredV1Pattern` is unchanged. `TestGoVerbosePassMarkerBoundary` and the
local-completion `go-verbose-pass-log` regressions cover detector and executed-check behavior.
Because the writer-screen source is an analyzer input, the reviewed change advances
`analyzerSchemaID` from `corvint-analyzer/69` to `corvint-analyzer/71` and refreshes its audit pin.
## 2026-09-21 BBF-V0-001..012: criterion-level browser behavior falsification (issue 54)

The experimental `corvint-behavior-falsify` companion separates deterministic planning from exact
digest approval, stages caller-owned argument-free hooks, and runs them under bounded process-group
containment in a caller-marked disposable workspace. It records contract/criterion/assertion,
application/test/documentation revision, runner/browser/config/environment, perturbation,
attempt/retry, cleanup and artifact identities. Only the expected assertion failure with unrelated
criteria and setup still passing can classify `killed`; selector errors, unrelated failures, retry
masking, stale bindings and cleanup drift are invalid, while process/timeout loss remains
`infrastructure_failed`. The report retains all six raw statuses and always preserves
`full-relevant-suite` fallback.

Focused `go test -count=1 ./internal/behaviorfalsify ./cmd/corvint-behavior-falsify` and matching
`go vet` passed; the same packages also pass `go test -race`. The synthetic matrix covers the expected kill, tautology, hidden duplicate,
wrong-value survival, wrong assertion, unrelated failure, selector error, timeout, cleanup failure,
retry masking and stale revision/perturbation/artifact identities. A staged live test helper produced
and then cleaned a retained artifact with equal pre/post workspace digests; a separate one-second
timeout proved owned process-group cleanup and workspace restoration; missing descendant-observer
evidence remains infrastructure rather than success. These are authored synthetic fixtures, not a
real adopter or browser run. Hook semantics and absence of persistent external effects remain
caller-owned and unauthenticated; live utility is `NOT_OBSERVED`.

Independent review found cancellation could schedule untouched cleanup hooks, wall-clock accounting
did not reserve both process shutdown windows, process stdin retained a lower hidden default, plan
controls were duplicated, report/receipt output was not aggregate-bounded, infrastructure receipts
accepted contradictory caller strings, and the initial acceptance matrix was incomplete. Repairs
stop after the interrupted attempt, count only started controls, reserve hook/cleanup shutdown and
execution time, isolate and bound Git reads, divide the report budget across approved attempts, use
one non-HTML-escaping deterministic JSON encoding, close infrastructure reason/shape validation and
add live crash, overflow, cancellation, slow-termination, cleanup, stale-artifact and JSON-expansion
regressions. The final independent re-review returned `PASS`.

The first frozen canonical gate exposed two local evidence defects. Under concurrent host load the
timeout regression obtained owned process-group cleanup and restored the workspace but a transient
`ps` snapshot was unavailable; the already-fail-closed infrastructure result is now asserted without
turning observer availability into a test prerequisite. The new specification also used a prose
`Boundary` digest bullet instead of the required `Exists` and `Blocked on` fields, and its README
clause differed from the indexed claim. The digest and README now share the exact indexed claim.
The replacement gate passed the full suite, vet/cross-vet, archive and interop before detecting the
resulting stale requirement line numbers; `REQUIREMENTS.tsv` was regenerated from the repaired spec.
The next frozen gate passed those checks plus spec, traceability, EOL, CI, release and receipt policy
before the error-code ownership tail found the new `approved-plan-drift` code unnamed; the owning
spec now records that code and the shared authorization code explicitly.

Dogfood orientation exposed two limitations retained for review: the initial limit-one query ranked
the Go-kernel migration spec rather than the behavior-contract seam, and the later focused context
packet reported captured index revision `70ffae556ba8cecc499501a492b9051485310759` rather than the
worktree HEAD. Exact repository inspection found `documentation-corpus-v1.md` and
`internal/doccorpus/behavior.go`; no completeness claim is made for the stale context packet.

The first committed CEM and full gate passed, but enrolled `finish` exposed a distinct
traceability miss: Go test function names normalized the requirement numbers and yielded no exact
`BBF-V0-###` OCM claim anchors. Requirement-labelled test cases now bind the existing behavior
assertions, with added closed-vocabulary and report-limitation checks. The first enrolled full test
failed in unrelated process/timing tests under concurrent repository-wide runs; a clean serialized
retry passed. Both observations remain in the private completion evidence rather than being
reclassified as product behavior.

## 2026-09-21 GL-V0-001..GL-V0-008: gate ledger, one pass per distinct content

The owner asked that Corvint manage its own gate so parallel agents stop each running a full
`make gate` on content another worktree already proved. `docs/specs/gate-ledger-v0.md` is accepted
and implemented: `tools/gate-ledger` keys every step on a digest of its declared inputs over the
exact worktree (tracked and untracked, ignored excluded, written through a private Git index) plus
the gate's tooling, records only passes in a per-user `0700` directory, and skips a step only on
key equality. `go-archive-gate` always runs because GOC-V0-010 binds its witness to HEAD, and every
doubt (undeclared step, skip-worktree or assume-unchanged entry, ignored compiled `.go`, git
failure, shared ledger directory) runs the step and records nothing.

Independent finding that changed the design: Go's test cache never hits across worktrees, even
with `-trimpath`, because its test log hashes the absolute paths of files a test opens under the
module root (measured on this host with a second `git worktree` at the same commit). The proposal
had assumed the cache would deduplicate resolved packages across worktrees; it deduplicates only
same-worktree reruns. So `ledger/go-test` (GL-V0-004) runs the 93 packages the affected-plan index
resolves without `-count=1`, under Go's cache, and the 104 unresolved packages (`cmd/corvint`
among them, for one `os.Getwd`) with `-count=1` under one record keyed on the whole tree. A tree
change therefore still reruns the unresolved set; narrowing it is the affected tier's job
(`affected-plan-v0.md`), and a per-package cross-worktree key is recorded as a follow-up in
`agent-memory/ideas.md`. `make go-test`, `make gate-affected` and CI keep `-count=1` unchanged.

Measured on this host (Mac Studio, `-p 1`), from the baseline `go test -json ./...` at the base
commit: 197 packages, 2,167s of package time, of which the 104 unresolved packages take 1,634s and
the 93 resolved ones 533s; `cmd/corvint` alone is 174s. The partition on this tree lists 93
resolved and 105 unresolved packages (the module root counts once more than the baseline's
package list). Three cheap steps run through `ledger/` twice: the first pass ran and recorded all
three in 3.7s, the second hit all three in 2.0s, so the per-step ledger cost (worktree digest plus
`go run` start-up) is about 0.65s. `plan` prints `go-archive-gate: always runs` and `RUN` with
`no declared input scope` for an unknown step. A `go-test` run whose unresolved set failed (the
host-adapter test reading a pre-existing dirty `plugin.json`) recorded nothing, as GL-V0-002
requires. Full gate, measured twice in a clean `git worktree` at `d6626ae` with an empty ledger
directory: the first `make gate` ran and recorded all 28 keyed steps in 1492s and recorded the
receipt; the second hit all 28 (the full `ledger/go-test` among them), ran only `go-archive-gate`,
and recorded the receipt in 173s. The first attempt at `22208f6` found two defects the unit tests
had not: a linked worktree's index path is absolute, so the private-index copy was empty and no
step recorded (fixed, `TestRunStepRecordsFromLinkedWorktree`), and the Windows cross-vet rejected
`syscall.Stat_t` and `syscall.Flock` (fixed by build-tagged `platform_unix.go`/`platform_other.go`;
a non-Unix host refuses the ledger directory and records nothing).

## 2026-09-21 SEG-018..SEG-021: typed semantic choice decisions

The owner directed Corvint to adopt the useful typed-decision ideas from TypeSafe AI's System One
model announcement without adding Jev or another hosted dependency. The unwired
`internal/semescalate` experiment now has a separate provider-neutral choice path over mechanically
supplied anchored options. Providers return only an option ID and exact integer probability mass;
Core derives an explicitly uncalibrated winner margin, supports a reserved abstain option, rebuilds
the immutable proposal, and still requires the registered verifier before emitting an `INFERRED`
candidate. The legacy proposal request and schema are unchanged. No provider, network path, serving
integration, calibration corpus, authority, or product claim is added.

Focused provider-spy tests cover the successful end-to-end path, pre-call question refusal,
case-folded/duplicate/missing/fabricated distributions, non-unique maxima, low-confidence and
explicit abstention, schema separation, complete cache identity, and the unchanged authority ceiling.
Independent review identified shared-state races, mutable evidence aliases, and incomplete screening
of transmitted handles. The repaired gate serializes run reservations and ledger reuse, snapshots
selected evidence, and screens every transmitted caller-authored string; focused race tests cover
concurrent budget/cache behavior and mutation during provider latency.
The canonical gate then exposed one malformed wrapped Agent-digest bullet, which was repaired and
independently re-reviewed. Two subsequent exact-target gate runs failed only because the large
`contextindex` fixture used the benchmark Git helper, allowing detached auto-maintenance to recreate
`.git/info` during `t.TempDir` cleanup. A 20-run loop reproduced 16 failures; applying the existing
`testGit` synchronous-maintenance policy to benchmark fixtures made all 20 pass without changing
runtime behavior.
The frozen calibration, held-out replay, kill-gate, and first accepted extractor profile remain open;
this deterministic slice is not evidence that any model's probabilities are calibrated.

## 2026-09-21 AHI-022: OpenCode file-change burst fallback

Issue #55 reproduced two adapter-local failures: concurrent `file.edited` callbacks overlapped
Corvint subprocesses, and the structured `unsupported-impact-path-suffix` refusal reached the
terminal as a fault. The OpenCode adapter now shares one bounded file-change drain, coalesces
duplicate per-session paths, and records that expected refusal through `client.app.log` while
leaving the Core non-zero refusal unchanged. The adapter fixture asserts both no overlap across a
twenty-event burst and preservation of the structured refusal code. Node 16 was outside the
package's declared `>=20` runtime; supported-runtime verification used Node 22.23.2.

The first full gate passed the issue #55 adapter coverage, but
`TestBuildQueryAgreesAcrossWorkerCounts` failed cleanup once there and twice in isolated retries
while `t.TempDir` removed `.git`. The exact failing test passed with Git auto-maintenance disabled.
The helper now applies the same
`maintenance.autoDetach=false`, `gc.autoDetach=false`, and `gc.auto=0` fixture boundary as
`testGit`, so every fixture writer finishes before cleanup. After the repair, the test passed ten
consecutive isolated runs under concurrent gate load.

## 2026-09-20 LAC-V0-032: safe roadmap auto-recheck

The roadmap repeats its existing read-only request every 30 seconds. Eligibility remains derived by
Corvint Tasks: no mutation, approval, external/manual completion, unknown-evidence waiver,
admission, release candidacy, attestation, or promotion is added. Pages containing mutation forms
do not refresh automatically. A page-preserving pause/resume link prevents timed reloads from
interrupting deliberate inspection. `TestRoadmapSafeAutoRecheck` binds the refresh and hard-stop
notice on successful and refused reads and checks that paused roadmaps and the board remain stable.
The first full gate reached every package but failed three `internal/contextindex` tests during
`t.TempDir` cleanup: detached Git maintenance recreated `.git/objects/info/packs` and
`.git/info/refs` after removal began. The shared fixture Git helper now disables auto-gc and keeps
any maintenance synchronous; the exact combined reproducer and the full gate must pass after this
repair before the console change is qualified.

## 2026-09-21 PWP-V2: sensitive browser-input evidence boundary

GitHub issue #56 adds explicit `corvint-playwright-external/2` selection. The reporter redacts
default and bounded provider-added input actions before its private JSON write; the Go boundary
validates the untrusted report and retained canonical document again. Findings retain only a typed
code and structural path. `/0` and `/1` reject the new fields and keep their prior behavior.

Focused `internal/jstestprovider`, `internal/testvaliditydoc`, and
`cmd/corvint-js-test-provider` tests passed, including a deliberately leaking conformance payload
and an accepted redacted payload. The checked-in Node regression executes the actual reporter over
nested, retried, escaped, metadata-declared and deliberately leaking actions; its output retains no
fixture values. Go regressions cover whole-receipt validation, normalization, Unicode case-folding,
the depth, total-step, string and finding bounds, sibling risk fields, short-value structural
noninterference, and fixed value-free decoder failures. Raw-step bounds run before fixture recursion;
scrubbing is restricted to action titles, declared sensitive metadata, and error/attachment/failure
detail fields so status, identity and criterion text remain unchanged. `node --check` and
`node --test` passed. The live Playwright reporter matrix is `NOT_RUN`, so `/2` is explicitly non-promotable and
cannot project passing execution; existing `/0` and `/1` qualifications are unchanged.

The fresh repair reproduced three independent-review P1s before changing code: an already-redacted
action admitted arbitrary raw risk fields; receiver-prefixed unquoted values escaped sibling errors;
and retained-document unknown-property diagnostics echoed attacker text. Sensitive tests now require
canonical redaction of every nonempty diagnostic/attachment field across all retries, including when
no original value exists. Raw-title matching and extraction share one receiver-aware matcher without
case-transformed offsets. Malformed `/2` retained documents and inputs whose kind cannot be decoded
return a fixed typed `sensitive-input-document-invalid` finding. Successfully probed legacy `/0` and
`/1` closed-decode diagnostics retain their prior behavior. All three focused Go packages, six actual
Node reporter regressions, reporter syntax checking and focused vet passed after this repair; the
coordinator owns independent review, the frozen full gate and final dogfood binding.

The fresh task's next independent review reproduced punctuation/Unicode leading-action gaps and
missing custom/final-argument extraction, including cross-test echoes. Repair cycle 1 replaced
substring/ASCII-regexp classification with shared Go/JavaScript Unicode token semantics at a bounded
leading action or receiver position. The original rune sequence supplies the parsed tail; quoted
commas, escapes and nested parentheses cannot split an argument. Any sensitive action now protects
every risk field report-wide, including already-redacted manifests with no original candidate.
Regression controls preserve assertion/navigation prose containing embedded action names. All nine
actual Node reporter tests, the three focused Go packages and focused vet passed; the independent
overlay replay reported `leak=false` and successful sanitized document decoding for every reviewed
title. No live reporter qualification or promotion is inferred from these bounded tests.

Repair cycle 2 reproduced an admitted `custom+entry` pattern that the matcher ignored and a
zero-word `***` declaration that silently disabled its own detection. Normalization and matching
now share exactly the non-Unicode-letter/number/mark separator class, and both implementations
reject zero-word or oversized declarations before accepting evidence. A bounded lexical refusal
also closes call-bearing receiver expressions containing sensitive action tokens or quoted property
names. These expressions remain unsupported; rejection is typed and value-free and prevents the
reporter from writing a partial report after earlier tests. All eleven actual Node reporter tests,
the three focused Go packages, focused vet, and the independent boundary-check overlay passed.
The reviewer's unchanged latest replay now stops at `sensitive-input-policy-invalid` because it
adds `***` to every policy while still expecting successful serialization; the checked-in regression
asserts that required rejection explicitly. The live `/2` qualification hold remains in force.

## 2026-09-21 AFP-V0-019 / MCPV0-020: explicit immutable planning snapshot (issue #57)

The owner requested authoritative evidence for an explicit immutable snapshot while unrelated
checkout paths remain dirty. The experimental route binds current commit, complete tree, base,
exact changed paths and canonical path digest. It does not authenticate caller intent or accept
runtime coverage. CLI selectors consume bounded committed blobs in disposable private scratch;
MCP uses the existing immutable revision reader and preserves the outer mixed-worktree binding.
Query snapshot authority excludes mutable traces/history rather than weakening their drift gate.
Missing/stale/mismatched receipts fail closed; unsupported overlays and provider/discovery
composition remain explicit exclusions. Existing live-worktree routes are unchanged.

Pre-change native query succeeded at base `3191c0b95fb154a94f1e758d935b72fe237dfd65`;
raw evidence, usage baseline and keyed local-completion plan are in `/tmp/corvint-issue57` and the
worktree-private Git evidence directory. Initial dogfood retained `NOT_PRODUCED` reasons
`git-diff-failed` (empty range), `missing-intent-scope`, `cem-map-not-produced`, and
`outcome-input-not-provided`; these are not passing verification. `affected --base` was used
before tests. Query, affected, CEM/OCM and MCP are applicable routes; learning, providers, mutation,
console and external host qualification are not part of this source-selection change.

Focused snapshot regressions exposed query history's live-state binding; the repair retains that
binding for legacy queries and explicitly omits history in the immutable profile. Focused receipt,
CLI and MCP regressions passed. Final canonical checks and independent review are recorded by
keyed dogfood observations and the coordinating task, not inferred from these fixture results.
Independent review cycle 1 reproduced an MCP admission gap: symlink/gitlink rejection had
only guarded CLI materialization. Shared bounded tree validation now guards both routes, with
MCP query/impact regressions for both shapes. The same review found final coverage compilation
dropped the history/trace exclusion disclosure; the compiler now receives that disclosure and
the MCP wire test asserts it alongside authoritative evidence. The in-flight canonical root suite was cancelled
before repair (`verification-cancelled`), never counted as passing evidence.
The builder did not delegate under the sole-builder instruction; independent review belongs to
the coordinator. Gate-plan/gate-intent scripts are Beamfall workflow tooling, absent here; the
practical plan review and requirement-linked tests provide local review inputs, not an independent
gate claim. Rollback removes only explicit snapshot admission and its optional scope fields.

## 2026-09-20 PWP-V0-003/007/008: standard Playwright device-spread regression

GitHub issue #49 reported that the ordinary Playwright project form
`use: {...devices['Desktop Chrome']}` produced `report-identity-unknown`. An isolated issue branch
reproduced that exact failure before resolving Playwright's default bundled headless executable; the
same implementation area was then superseded on `main` by issue #50's stricter registry revision,
manifest version, executable suffix and SHA-256 qualification. The final integration retains that
stricter implementation and adds a separate real-browser regression for the original device spread.

The retained fixture requires project/browser identity, config digest, stable test ID, nonempty user
agent, 1280×720 viewport, bundled browser version/path and a passing projection in one receipt. It
passed with the pinned Playwright 1.63.0 modules and browser. The exact Golf checkout and hosted CI
remain `NOT_OBSERVED`; the minimal checked-in fixture proves the reported configuration shape, not
the unavailable consumer repository.

## 2026-09-20 PWP-V0-008: Playwright 1.63 bundled headless-shell qualification

GitHub issue #50 exposed that the only qualified Darwin arm64 / Node v22.23.2 Playwright 1.63 path
used auto-updating system Chrome. The reporter now distinguishes configured and Playwright-registry
executables. Its bundled path binds registry name `chromium-headless-shell`, revision `1243`, manifest
and observed browser version `153.0.8010.12`, a cache-root-independent executable suffix, and exact
SHA-256 `a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282`. Projection rejects any
changed Node version, revision, manifest/browser version, executable kind/path/digest, headed mode or
explicit override. Remote-browser connections also abstain because the local executable identity does
not describe the connected browser. The previous system-Chrome tuple remains a separate exact branch.

The real external-server matrix now runs against the bundled headless shell and retains pass,
assertion failure, timeout, retry, cancellation, browser infrastructure, two-project identity,
external-server survival and MCP discovery controls, then smoke-tests the previous system path.
Behavior and stability corpus fixtures consume the bundled tuple through the ordinary retained-
receipt qualification path. Additional Node/browser tuples require a spec amendment, exact lock and
registry identities, the complete live matrix and negative drift controls; no semver widening is
accepted. The live matrix passed in 25.91s with headed and remote-connection negative controls using
the preinstalled locked 1.63.0 modules and browser;
no package or browser download ran. The pre-change Corvint query selected an unrelated documentation
compiler and omitted four ranked results; targeted path impact identified the PWP spec, reporter,
tests and consumers instead.

Independent review found that `launchOptions.headless=false` and explicit or environment-selected
remote browser connections could initially retain the local bundled identity. The reporter now
matches Playwright's headless precedence and abstains for remote connections and custom launch
fixtures; retained projection rejects serialized connection options. The added live negatives pass,
and the repair re-review found no remaining actionable issue.

The first scoped local-completion plan could not retain the successful verbose live check because the
writer secret screen classified Go's `--- PASS: TestQualifiedPlaywrightLive` marker as a credential
assignment. That plan was cancelled without satisfaction, the confirmed false positive was added to
`docs/agent-memory/bugs.md`, and the same frozen live test was selected without `-v` for retained
evidence; this changes log verbosity, not execution or assertions.

## 2026-09-20 Integrated canonical-gate repair

The first combined `make gate` rejected the candidate before publication. The work-queue OCM
enumeration still stopped at WQO-V0-048 after WQO-V0-049..050 were added, and the Playwright
minimizer claim exceeded the 160-character index limit while its README and generated index had
diverged. Those conformance records now agree. Two context-index tests also exposed a repeatable
macOS cleanup race: Apple Git auto-maintenance could recreate `.git/objects/info/packs` while Go
removed a temporary repository. A second full-gate run exposed the same race in a different query
fixture, proving the first fixture-local repair too narrow. The shared context-index Git fixture
launchers now disable automatic GC and maintenance for every mutating test command. The failed
full-gate receipts remain retained and invalidated; a new commit-bound canonical gate is required.

## 2026-09-20 PSM-V0-004/008/009: original failure identity repair

Independent final review found that consistent failures of a different class could be called
reproductions of the original failure. Reproduction and candidate trials now require exact sorted
distinct failure-class sets; missing or additional classes retain observations but invalidate the
trial with `original-failure-signature-mismatch` and prevent confidence/minimality claims. Native
planning rejects caller classes inconsistent with the qualified original target and binds rederived
observations into the plan. The original receipt bytes/digest remain immutable; new-run evidence
digests and summaries are retained, not compared for impossible byte equality. Class-set equality
does not prove identical root cause. Isolation failures still stop minimization independently.
Synthetic assertion-to-fixture/synchronization controls cover reproduction and both candidate
kinds; multi-class controls cover missing/additional classes, ordering, duplicates and fresh evidence.
Focused minimizer and companion tests pass; no additional live-world claim or release promotion.

## 2026-09-20 Issues 42, 43, 47 and PUB-V0: integrated review repair

Stability accepts fully qualified `/1` application attestations, including a distinct application
repository, while preserving `/0` declaration semantics. Invalid attestation and contradictory
application revision refuse. The Docker fixture now selects the accepted explicit system-Chrome
tuple for Playwright 1.63. Its explicit qualification passed (23.829s) with installed modules at
`/private/tmp/corvint-pw163.589jVO/node_modules`; no runtime package was downloaded.

The separately built experimental minimizer now offers read-only planning and digest-approved
execution, rederives actual corpus stability evidence, qualifies actual `/1` receipts, constructs
provider selectors, compares observed schedule/topology, and retains complete native trial evidence.
Reset/cleanup run pinned operator commands with bounded output and cancellation cleanup. A separate
20ms PID/start observer terminates and verifies absence of observed escaped descendants; failures
invalidate the trial. It explicitly does not prove universal containment or unobserved fast-detach
absence. CRR-V0-003(c) and `RequireDescendantCleanup` continue to refuse before launch unchanged.
Detached-child, PID-reuse, authorization, evidence-tampering, reset-failure and cancellation controls
cover the new boundary. Real Docker/Playwright predecessor-failure then isolated-pass qualification
passed (11.807s). The other classification fixtures remain synthetic, not six claimed live worlds.

The owner selected `v0.5.0a1` for publication. Decision 0327 supersedes the pending version target
without rewriting historical decisions or measurement evidence; active release tuple, notes,
installation, editor admission and publication fixtures move together. Unsigned prerelease,
publisher `NOT_VERIFIED`, four non-Windows core archives, separately qualified optional companion,
and no-promotion semantics remain. Full integrated gate and publication remain coordinator-owned.

## 2026-09-20 Issues 39–47: consolidated integration

The completed issue branches are merged in dependency order: 39, 43, 41, 42 (including 40),
46, 47, 45 and 44. The integration preserves each branch history and sealed CEM; inherited shared
CEMs are removed so the coordinator can bind one combined change. Conflicts retain both independent
build-log entries and provider tests, the newest stability requirements, and both Playwright
consuming-path qualification and application-attestation contracts. The generated requirement index
is rebuilt from the merged specifications. Compilation caught two synthetic receipt fixtures that
still referenced the removed single-version constant; both explicitly retain their original 1.60.0
version. Spec-index validation caught a README claim-prefix mismatch, repaired without dropping the
issue-41 discovery status. The installed release smoke also supplies issue-45's explicit executable
binding when initializing its work queue. Focused conflict checks and compilation precede the coordinator-owned
combined review, frozen gate and separate release qualification; those remain required.

## 2026-09-20 PWP-V1: externally managed application attestation

Issue 43 adds `corvint-playwright-external/1` without changing `/0`. A generic bounded command
provider receives one canonical expectation document on stdin and emits the same closed canonical
application-attestation shape before and after Playwright. The receipt binds clean test-repository
root/revision/tree; application root/revision/tree and dirty policy; image, Compose configuration,
container/start generation and health; provider executable/config/output digests; runner, browser,
argv and declared environment. The provider executable runs from a private content copy. Corvint
owns only provider and Playwright process groups and has no application lifecycle verb.

The local qualification used `@playwright/test@1.63.0`, its installed Chromium, and a disposable
scratch-image Docker server built from the checked-in closed Compose JSON manifest. A healthy bound
run projected passed; healthy wrong-revision and wrong-image inputs stopped before Playwright, and a
fixture-harness restart changed container start generation and forced infrastructure. Generic command
tests also cover unavailable, unhealthy, missing and contradictory attestations. Docker 29.5.2 was
available; no Compose frontend was installed, so the qualification harness executed the manifest's
closed build/run/health/port subset through project-scoped Docker commands and retained the exact
manifest digest. Signal-aware cleanup removed the fixture container and image; no external app was
started, stopped or changed.

Independent review found that the first implementation sampled the test repository before the test,
accepted attestation on the managed-server path, inherited ambient Git repository redirects, allowed
mixed `/0` and `/1` fields, under-validated retained provider identities, and could consume Docker
cleanup before provisioning completed. The repaired qualification changes an otherwise unbound
tracked test-repository file during Playwright and cancels cleanup before provisioning; both remain
non-passing and the latter leaves no container or image. Provider/config hash syntax and canonical
configuration binding, profile shapes, external-only admission, post-run repository identity, and a
Git environment without `GIT_*` redirects have focused regressions.

The first frozen canonical gate exposed two fixture/metadata failures: the spec index repeated a
longer, non-identical digest, and the real local-completion fixture omitted the existing
`qualified-reporter.cjs` embed required by the issue-39 baseline. The digest is now one exact
sub-160-character value across the spec, index and README; the import-closure fixture copies that
embedded asset. Focused `internal/specindex` and real-evidence local-completion regressions cover
both repairs before the gate is rerun on the replacement frozen commit.
That replacement gate passed the full test suite, native/cross vet, archive and interop, then caught
the stale generated `REQUIREMENTS.tsv` summaries for the revised PWP-V1-001 and PWP-V1-006 clauses;
the registry was regenerated before the final frozen gate.

## 2026-09-20 PWP-V0: Playwright 1.63.0 external-server qualification

Issue 39 extends the accepted external-server profile's exact runner allowlist from Playwright
1.60.0 to 1.60.0 and 1.63.0. The checked-in real-browser matrix passed locally on Darwin with
macOS arm64, Node v22.23.2 and `@playwright/test@1.63.0` with system Google Chrome
153.0.8010.48 at `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`; no channel override
was used and the Playwright `chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell`
executable was present. It preserved pass, assertion failure,
test timeout, interruption, browser infrastructure failure, retry/attempt state, two-project
identity, inherited `webServer` suppression, external-server survival, cancellation cleanup, and
retained MCP discovery, setup dependencies, global use, project inheritance, two-worker execution
and repeat-each identities. Executable option metadata and a custom `page` fixture both produced
unknown identity/infrastructure and never a passing projection. Other Playwright versions remain
unqualified; Playwright 1.63 on another Node/platform/browser path also remains diagnostic-only.
The Linux amd64 installed/bundled-browser arm is `NOT_RUN`. A local `golf-e2e` checkout does not
exist, so its deterministic consumer fixture and CI observation are `NOT_OBSERVED`. The externally
managed application command for `http://127.0.0.1:3002` is recorded in
the accepted profile; its config owns the bound system-Chrome executable path, and the provider
neither starts nor stops that application.
## 2026-09-20 AFP-V0-018 / TJAA-V0-012..017: issue 41 discovery amendment

The owner authorized cancellation of the superseded TJAA-only enrollment and re-enrollment from
the same original base `536e560e1fa35573e49df644dde4257a8bb7e050` with AFP and TJAA. Original
enrollment, source commits and failed gate evidence remain archived; that gate failed in
`TestLocalCompletionRealEvidenceWorkflow/LCP-V0-007_completion` with
`dogfood-change REFUSE current-tree-corvint-build-failed` and is not passing evidence for this scope.

The amended profile gates all file argv on canonical caller-owned discovery reconciliation and
uses a single complete-config command when discovery is unproven. Project membership alone defines
candidate tests; helpers remain dependency sources. A matched universe with unknown reachability
widens to exactly its file/project pairs, eliminating the former helper-by-project Cartesian fallback.
The input binds revision, config and current source bytes; missing evidence remains explicit.

The assumed existing Playwright installation was unavailable. A temporary installation from the
repository's pinned interactive-alpha lockfile supplied Playwright 1.63.0 without browsers.
Its real unfiltered `--list --reporter=json` yielded exactly eight pairs: two spec files under
Chromium/Angular/React plus setup and cleanup. Reconciliation matched all eight; a page-object edit
selected five pairs and excluded the three unrelated spec variants, with no helper argv. Raw listing,
input receipt and CLI outputs are retained under `/private/tmp/issue-41-list*` and
`/private/tmp/issue-41-real-discovery.json`; no consumer-checkout or runtime-execution claim follows.
Frozen synthetic 117-file qualification also requires an independent 353-pair receipt. Canonical
repeatability, mismatch differences, stale bindings, malformed inputs, strict bounds and fallback
regressions pass focused checks. The exact golf-e2e checkout remains `NOT_OBSERVED`.

Corvint pre-change context and required dogfood preparation were used. Initial preparation retained
missing citation/scope/outcome reasons; final reports and the parent-owned serialized gate remain
required. Optional mutation/provider execution and runtime promotion are outside this static slice.

Independent review found custom config names lacked test-to-config edges, allowing a transitive
global-setup helper change to select nothing despite matched discovery. Repair explicitly binds
every admitted physical test to the selected config; a custom `e2e.config.ts` regression requires
the entire matched suite for its setup-helper change without making helpers executable units.

## 2026-09-20 TJAA-V0-012..017: golf-shaped Playwright selection (issue 41)

The owner-requested follow-up to issue 18 adds static global-use inheritance, nearest-tsconfig
baseUrl/paths resolution and global setup/teardown dependency edges to the opt-in profile. The
shared default adapter retains its previous alias frontier. Unsupported inheritance, loader-shaped
resolution, ambiguous or missing targets, computed imports and config still widen; application state
remains an execution unknown. No JavaScript/config is executed.

The synthetic golf-shaped qualification proves 41 selected units out of 353 for one changed cohort,
all 353 for global-setup helpers/config, distinct Chromium/Angular/React units, setup/cleanup closure,
and identical canonical bytes for identical inputs. The actual golf-e2e checkout was unavailable:
consumer configuration and consumer recall remain `NOT_OBSERVED`, with no runtime promotion claim.
The original qualification fixture and all shared TypeScript tests remain required gate inputs.

Corvint query and initial dogfood-change were used at base
`536e560e1fa35573e49df644dde4257a8bb7e050`; the query retained four omitted results and the initial
empty-change coordinator retained `NOT_PRODUCED` CEM/OCM/outcome reasons. A pre-first-query measurement
receipt was `NOT_OBSERVED`; no token/cost savings are claimed. The enrolled gate and final CEM/OCM
reports remain the authoritative completion evidence. Mutation, external providers and runtime
qualification are not applicable to this bounded static observer change. Independent review is owned
by the parent task, with no nested delegation.

Independent review found equal-prefix alias ordering, multiple existing alias targets, and explicit
browser inheritance across device spreads could differ from Playwright 1.63. Repair widens both
alias ambiguities and preserves explicit browserName over a device defaultBrowserType. Conflict
regressions cover both pattern orders, competing targets and inherited/same-layer browser defaults.
Final review also confirmed explicit `use.defaultBrowserType` cannot be ignored: the closed subset
now rejects that key with browser-identity uncertainty, covered at both global and project scope.

## 2026-09-20 MER-V0-001..012: revision-bound migration evidence ratchet (issue 46)

The experimental `migration-ratchet` profile compares digest-verified baseline and candidate
snapshots across stable migration identities and emits raw denominators plus separate addition,
removal, content, state, stale-evidence, reverse-link and unknown deltas. Repository policy prevents
new legacy debt, uncontracted test additions or changes, terminal regression, stale review/runtime
reuse and unresolved-denominator growth. Exact reviewed rules are required for otherwise
incomparable domains; exact owner-reviewed expiring exceptions never erase their underlying deltas.

The earliest synthetic baseline-to-candidate receipt advances one grandfathered unresolved identity
without growing its denominator and is byte-identical across repeated compilation. Six issue-46
negative controls and whole-input refusal controls pass in the focused package tests. The command
exit contract passes its focused CLI tests. These synthetic fixtures do not establish provider
honesty, contract adequacy, migration completeness or real consumer compatibility. The full shared
gate is intentionally NOT_RUN pending coordinator release.

The task-start query retained four omitted results and six withheld test-path candidates. The first
pre-change coordinator attempt at base `6098291c9ed84c0de5c1a76afa3599d6a6faa352` correctly remained
`not-complete` before any change existed. Dirty-diff `affected` selected only `cmd/corvint` and
`internal/migrationratchet`, while retaining its language-frontier and unowned-document unknowns.
The documentation-corpus, stability, mutation, provider and service execution routes were not
applicable: this profile compares caller-supplied immutable artifacts and executes none of them.

Independent review found five fail-closed defects in the first implementation: trailing scalar or
malformed JSON was not required to reach EOF; an explicit identity mapping could reuse an implicitly
paired candidate and omit another; one exception could waive every defect of the same class on an
identity; contradictory selections were order-dependent; and record order/duplicate links were not
canonical. The repair requires exact EOF, globally one-to-one candidate pairing, one canonical delta
digest per exception, unique validated selections, sorted records and unique sorted links. Focused
regressions exercise every repair. Re-review found two remaining holes: mapped evidence identities
could skip renewal comparison, and stale/reverse-link delta fingerprints omitted the exact content
bindings that distinguish two defects. The final repair derives baseline records through the
one-to-one pair set and retains expected/actual content plus relation and reverse-relation bindings;
focused regressions cover both findings.

## 2026-09-20 PSM-V0-001..013: bounded Playwright suite-interaction planning (issue 47)

The experimental `internal/playwrightminimize` package emits a deterministic digest-bound schedule
that reproduces the original suite failure and rechecks isolation before separately enumerating
ordered predecessor sequences and unordered load sets. Trial count, repetition and wall-clock limits
stay explicit; partial searches cannot claim complete minimality. Execution exists only as an
abstract synthetic qualification seam and refuses without separate operator approval bound to the
exact plan. No Playwright, browser, server or application process ran for this slice.

Every synthetic trial binds revision/config/runner/browser/project/order/topology/fixture/seed and
application-instance identity, declares its reset policy, and carries setup, assertion, retry,
cleanup, server-health, resource and failure evidence. Failed reset/cleanup, attestation change,
identity drift, malformed/duplicate receipts, and missing evidence invalidate without erasing the
observation. Reports retain all seven failure classes, separate ordered and load findings, label
members necessary only in the observed universe, and never claim global minimality.

Six synthetic qualifications cover a predecessor leak, load-only resource failure, restart,
cleanup failure, nondeterminism and isolated product regression; controls cover authorization,
`not_reproduced`, exact planning and bounded incompleteness. #39 runner qualification and #43
application attestation are divergent development refs rather than integrated frozen dependencies at
base `6098291`; missing #39/#42/#43 receipts therefore remain distinct confidence blockers. Live
integration and the shared full gate are `NOT_RUN` pending their owning coordination.

Independent review reproduced seven boundary defects: a late or partial reproduction could become
conclusive; runners lacked the remaining wall-clock deadline; errored runners dropped returned
receipts; singleton necessity crossed worker topologies; isolation infrastructure failures were
called product regressions; opposite baselines could share one digest; and findings omitted passing
receipts that supported minimality. The repair deadline-bounds and post-validates every call,
requires complete repetition groups, preserves invalid errored receipts, gates necessity on exact
topology, derives only an explicitly product-only isolation label, rejects duplicate baselines, and
retains all comparison receipts. Focused regressions `TestPSMV0012` through `TestPSMV0016` cover the
review cases; re-review is recorded separately by the task coordinator. Corvint `affected` selected
only the new Go package while preserving the mandatory repository gate and documentation unknowns;
the pre-commit `prove` expansion returned `unsupported-impact-path-suffix` because the new package
was absent from its pinned repository revision, so no proof-of-impact claim is made.
Re-review then found that cancellation during the final runner call could publish confidence and that
mixed classifications returned before inspecting a later invalid repetition. The final repair reads
the trial context before releasing its deadline, invalidates cancelled results, and validates every
repetition before comparing classifications. `TestPSMV0017` and `TestPSMV0018` retain both cases.

## 2026-09-20 DCP-V1-021..026: revision-bound Playwright stability evidence (issue 42)

The experimental behavior-stability provider keeps issue-40 behavior coverage and repeated-run
stability as separate artifact axes. A repository-owned digest-bound policy selects one-spec,
feature-batch or suite thresholds; reports preserve planned/started/completed and every outcome,
retry, cleanup and manual-rerun count plus all contributing receipt/attempt evidence. A failed first
attempt remains both failed and flaky after a later pass. Missing iterations, duplicate receipts,
silently consumed retries, cross-application revisions and contradictory identities refuse; failed
cleanup is retained as a non-clean verdict. Corpus and MCP expose the exact aggregate without an
adequacy, parity, freshness or narrowing claim.

The earliest end-to-end aggregate and five requested negative controls pass on synthetic qualified
receipts. The original task query preceded private measurement and remains `NOT_PRODUCED`; the later
required DOGFOOD enrollment pins base `06eb443565979b313ecb63f7316f06677a908f65` and the owning
documentation-corpus spec. Live repeated browser execution and consumer policy qualification are
NOT_RUN, so the feature remains proposed/experimental.

The focused `cmd/corvint` regression exposed that its minimal local-completion repository copied Go
sources but omitted the qualified Playwright reporter embedded by the now-reachable provider import.
The fixture now carries that production embed; the product binary and provider profile are unchanged.

Independent review found that the first aggregate accepted carried identities without rebinding
test/config bytes, could hide failed native runner cleanup behind a carried pass, established its
identity baseline after an earlier manual run, ignored manual cleanup, and counted only final timeout,
interruption and infrastructure states. The repair requires explicit source mappings and the exact
behavior-test anchor, recomputes and verifies native projections, fixes the baseline to planned
repetition one, applies cleanup to every contributor, and retains each earlier attempt category.
Adversarial regressions cover each finding plus noncontiguous retry ordinals.
Re-review found that an invalid receipt and a genuine infrastructure outcome shared the same native
projection. The final repair verifies qualified lifecycle/identity binding independently of outcome
classification; paired regressions accept a bound infrastructure outcome and refuse the same outcome
when runner cleanup failed.
The authorized final repair rejects state/failure-kind contradictions at both the qualified receipt
binding and stability classification boundaries; a passed attempt carrying assertion-failure
metadata and an artifact now refuses instead of contributing to a clean verdict.
Late coordination integrated the amended, evidence-bound issue-40 contract at `e54ffcb`, including
patch-equivalent copies of both shared local-completion fixture repairs. The earlier gate on `4a60483`
is superseded and failed at the error-code ownership ratchet because the accepted Playwright provider
had never enumerated its existing refusal vocabulary. The owning PWP-V0 spec now records those codes;
no error behavior or wire value changed.
Integration review found that receipt identity qualification did not derive the aggregate outcome
from the ordered attempts. The stability consumer now applies the reporter's exact terminal-state
rule, including requiring a prior non-passing attempt before `flaky`; contradictory terminal states
refuse before policy counting.
The later owner acceptance comment makes declared-versus-observed execution topology first-class.
The source, focused-check and review evidence at `6098291` remains retained but is superseded for
completion by this amendment. Repository policy and each observed run now bind separate canonical
full-file topology inputs covering CI nodes/shards, Playwright workers per node, database mode,
sorted project set, split algorithm/version and resource class; each observation also binds its
receipt digest. Exact mismatch refuses before counting, with an explicit six-declared/four-observed
negative witness plus deterministic per-dimension and source-binding controls. The comment reports
that contradiction in the consuming repository, but no exact policy, CircleCI or run artifacts were
provided, so the consumer-specific 6-vs-4 result remains `NOT_OBSERVED`. The amendment-start Corvint
query selected an unrelated public-release spec and omitted four ranked results; repository-owned
spec routing supplied the owning DCP contract instead.

## 2026-09-20 DCP-V1-004/007..013/019: experimental behavior contract ingestion (issue 40)

The optional behavior-provider profile joins bidirectional flow/criterion/test/project identities
and a separate ordered runtime witness to pinned native Playwright observations. It preserves
provider-reported review versus unreviewed joins, scoped denominators and full-suite fallback.
Existing title-only joins were insufficient for identical titles in different projects; optional
exact test/project selectors preserve the legacy profile while admitting an unambiguous join.

Synthetic fixture paths model the proposed registry and schema-2 migration manifest. The consumer's
actual fixture bytes were not supplied: compatibility and live runtime utility are NOT_OBSERVED.
Pre-change query succeeded with three omitted results. Measurement before that first call was
NOT_PRODUCED; the required coordinator retained its own receipts and reported no-change CEM/OCM
and outcome NOT_PRODUCED. The local completion plan is enrolled against the immutable issue base.
No browser, server or provider process was launched; live qualification is not claimed.
Independent review found that demanding a passed run projection and current E2E freshness made
recorded verification unreachable for native Playwright receipts. The repair uses the native per-test
execution projection, rebinds test/configuration inputs, and preserves the exact unknown app-freshness
axis. A full Build/Open regression imports a synthetic qualified receipt through native decoding and
projection; no manually assigned CURRENT projection is used by that end-to-end test.
The owner's subsequent issue-40 acceptance amendment strengthens assertion target/value identity,
ordered browser-context/page/frame navigation, live discovery denominator reconciliation and the
app/e2e/docs revision set. Synthetic negatives cover same matcher/wrong element or value, same route
without assertion, wrong project, reordered/scoped visits and stale revision members. The consumer's
463/117 inventory and local consumer fixtures remain NOT_OBSERVED; generated prose retains its label.
Amendment review required rejecting digest-valid but noncurrent behavior anchors and checking every
assertion, not only finding one qualifying assertion per criterion. End-to-end regressions retain
validly pinned alternate-revision source and matching extra runtime events while rejecting their
stale, unreviewed or undeclared joins.
Final amendment review also required the reverse runtime-assertion check: a runtime event and
matching expected order cannot invent an assertion absent from the validated test declaration.
The retained-run regression covers that previously one-way join explicitly.
The later legacy acceptance amendment adds an independently digested suite/file/case inventory,
executable/disabled state, extracted observable criteria and fixture/role preconditions. Exact
reviewed target-to-legacy relations and complete reverse criterion coverage distinguish target
journey verification from retained legacy runtime parity. Same/stronger preserve the original
observable tuple; new/obsolete/blocked remain non-parity. Synthetic Build/Open fixtures cover a
consolidated test dropping one legacy branch and a same-named target changing success criteria.
Unavailable or disabled legacy runtime remains unknown even with source/docs/product review.
The supported runtime qualifier reuses retained native Playwright receipts, not a fabricated
legacy runner adapter. Exact consumer legacy inputs and actual live legacy execution remain
NOT_OBSERVED; unsupported legacy runners cannot establish runtime parity.
Legacy amendment review found that a classified but ineligible mapping could hide a dropped branch
from another target's parity result. Reverse parity coverage now counts only fully eligible targets;
a multi-target regression preserves the first target's journey while rejecting migration parity
when another target drops the original observable.
## 2026-09-20 WQO-V0-049..050: explicit work-queue executable binding (issue 45)

Decision 0326 binds repository adoption to one operator-selected canonical external Corvint file.
The reviewed adapter records its absolute path, SHA-256, version/build output and Go module/VCS
source identity. The observer rejects relative, missing, linked, unsafe-parent, writable,
repository-controlled or drifted bindings before adapter execution. It privately materializes only
the already-opened verified bytes and passes that path as trusted adapter argv, preserving the fixed
VPO environment and avoiding ambient `PATH`. Darwin cannot make the VPO-V0-024 exact-object claim,
so the receipt remains honestly `UNQUALIFIED`; the binding is drift protection, not attestation.
`work rebind` changes only the generated adapter for explicit review and commit.

The smallest `~/.local/bin` init/observe/propose path passed before the wider matrix. Focused
`TestWork*` passed, including `~/.local/bin`, `/opt/homebrew/bin`, `/usr/local/bin`, changed binary,
reviewed rebind, symlink swap, repository-local, unsafe-parent, relative and missing executable
fixtures. The focused help/parser checks and `internal/companionrelease` package passed; its first
sandboxed run could not bind an `httptest` listener and the authorized unsandboxed rerun passed.
The canonical gate is deliberately enrolled but NOT_RUN pending the parent queue's explicit slot
release; these focused results do not substitute for it.

Actual self-development routes: original query selected the accepted WQO spec with four ranked
results and test-symbol candidates omitted; the clean pre-change impact was OUT_OF_SCOPE with zero
changed paths; start-time dogfood-change retained empty-range CEM/missing-scope outcomes as
NOT_PRODUCED; immutable local completion enrollment froze the WQO intent and focused/spec/full-gate
checks; dirty `affected` selected four Go packages while retaining unowned documentation paths and
language-frontier unknowns. Mutation, corpus, external-provider, service and learning routes are not
applicable. No paired baseline exists and no savings claim is made.

Independent security review found and the first repair cycle closed three issues: companion
`-buildvcs=false` binaries now retain an explicit no-VCS module/toolchain identity; rebind pins and
rechecks the opened `.corvint` directory plus exact adapter bytes before replacement; and bound
executable drift during an operation now maps to `SOURCE_UNQUALIFIED` rather than `ADAPTER_FAILED`.
## 2026-09-20 PUB-V0-022..026: closed qualified release candidate (issue 44, Corvint half)

The Corvint release path now closes the existing seven-file core archive-gate output and the
three-file companion retained output into one versioned candidate. The candidate verifier binds
the exact Corvint commit/tree/toolchain across both inputs, retains both source archives and gate
receipts, requires the installed `Corvint <version> (build N)` identity, and records explicit
platform/workflow `PASS` or `NOT_RUN` rows. The companion `/2` installed smoke now exercises
affected selection, external Playwright receipt discovery, documentation-corpus discovery and
repository work-queue observation through the extracted `corvint` binary. Legacy companion
profiles keep their historical smoke inventory.

The versioned installer reverifies the closed candidate, retains the host core archive at a unique
version/platform path, refuses replacement and never writes a current/latest selector. Focused
native and Linux cross-build checks pass. Independent review found that the first implementation
bound only compressed core archive bytes, reread companion inputs without verifying the completed
staging tree, used a fixed removable version-probe path, and omitted three release-note disclosures.
The repair decodes the exact six-member core archives, binds binary/checksum/build identities and
executes the host core version, verifies completed staging before no-replace promotion, uses only a
unique owned probe, refuses scratch/output overlap, and names local-Git trust, unmeasured performance
and unavailable hosted CI. Re-review then found that the host probe polluted the exact three-file
companion verification directory and lacked caller cancellation. The final repair isolates both
directories, propagates caller cancellation with a bounded probe, and adds a closed-candidate
regression that executes the host identity check and proves the companion verifier receives exactly
three files. The pre-change core archive gate passed. The pre-change
companion gate reached the separately owned Corvint Tasks checkout and failed before retention at
`corvint-tasks init`; therefore no combined candidate was produced and Linux installed workflows
remain `NOT_RUN`. A retained-scratch reproduction identified the refusal as
`INTENT_BRANCH_MISMATCH`: the closed Git environment initialized the smoke repository on `master`
while the companion-owned intent fixture requires `main`. The repair pins the fixture branch and
retains structured command stdout in failed smoke evidence so a typed refusal cannot be hidden by
an empty stderr stream. The repaired companion gate passed at Corvint `68c9bbc` against exact
Corvint Tasks commit `f6ec200337160545b5e120a7242a8e862ecbcff0` and tree
`7573360392d58000f30dc0f82e6716cba09d81d1`; its retained archive SHA-256 is
`9dac067f16da86b8dc8bcb97414c96472d2fe613947d8acdca95cfb2024ad2cd`. All native installed
smoke rows passed and the four non-native targets remain explicitly `NOT_RUN`. The canonical full
suite was run twice on the unchanged repair commit; both runs passed the changed companion and
release-candidate packages but `internal/liveverify/session` exceeded its fixed 20-second event
wait under full-suite load. The same package passed immediately in isolation (35.8 seconds total),
as did the initially load-affected `internal/procgroup`; full vet and the CEM interop test/vet gate
passed. No unrelated timing-test source was changed. Publication, tagging, pushing, signing, upload
and promotion were not attempted.

## 2026-09-20 EEP-MCP / EEP-REMOTE: complete issue 11 transport profiles

Owner approval accepts decisions 0324/0325: bounded local MCP stdio and a separately built opt-in
HTTPS adapter. MCP needs interactive stdin; extending the existing process-group lifecycle avoids
a nested detached adapter group that Core could fail to kill. The protocol pins 2025-11-25, one
tool and one text envelope. HTTPS has normal TLS plus SPKI validation, explicit optional CA roots,
private bearer files and no redirects/proxies/retries. Core never imports the HTTPS package.

The original EEP-V0/V1/V2 and ETS conformance fixture matrices passed unchanged through both
transports, including strict decode, stale references, learned exclusions and authority separation.
Focused hostile checks cover bounds, timeouts, unavailable transports, MCP requests/extra frames,
normal and interrupted descendant cleanup, credential permissions, TLS and HTTP failures.
Independent security review found JSON-escaped credential reflection and case-insensitive MCP
field coercion. Repairs screen decoded credential strings and require exact MCP key spelling and
boolean values; the regression places `isError:true` before `IsError:false` to exercise the bypass.
The owner explicitly selected the existing affected-package fast tier instead of the full gate.
Its non-executing preflight selected packages without FALLBACK on merged base `bf2685dd`;
the canceled earlier-base enrollment remains non-success. The selected gate and final CEM/OCM
outcomes are retained in enrolled private observations; focused results do not substitute for them.

Actual self-development routes: original query (three omitted results retained), start-time
dogfood-change (empty-range CEM and missing scope/outcome NOT_PRODUCED), immutable profile enrollment,
dirty affected advice, and final CEM/OCM reports. Original query preceded the measurement scratch
receipt; its latency, billed tokens and complete source-open counts are NOT_OBSERVED. New profile
intents needed a planning-only commit before enrollment could resolve them; implementation began
only after enrollment. No savings claim. Live remote Internet services, external MCP server adoption,
mutation campaigns, documentation corpus and learning qualification are not applicable to this
transport-conformance change; frozen existing provider fixtures remain the behavioral witnesses.

## 2026-09-20 PWP-V0: external-server Playwright receipt qualification

The owner accepted the bounded `corvint-playwright-external/0` profile for issue 19. A local Darwin
qualification ran the checked-in fixture with Playwright 1.60.0 and its installed Chromium browser.
The matrix observed passing, assertion-failing, timed-out and browser-infrastructure outcomes across
distinct Chromium and React project identities; inherited `webServer` was suppressed, relative
global hooks executed, cancellation left the externally managed server alive, and retained evidence
was rediscovered through the actual test-validity MCP registry with lifecycle and freshness unknowns
preserved. Literal `test.use` overrides were attributed to their effective browser and viewport;
executable or unsupported overrides abstained. Focused provider, projection, CLI and MCP tests passed.

Independent review reproduced four defects before promotion: project defaults could misattribute an
effective `test.use` override, removing the profile discriminator bypassed lifecycle validation,
malformed qualified identity/attempt evidence could still project passing, and a temporary config
wrapper broke relative global setup resolution. The repair binds the qualified 1.60.0 reporter ABI,
rejects downgrade and incomplete or contradictory evidence, resolves original-config-relative
modules, and adds adversarial and live regressions. The same reviewer reran those overlays and the
expanded real browser matrix and returned PASS. Playwright versions other than 1.60.0 remain
unqualified; declared application identity is not proof of served content, Vitest and LPCV authority
are unchanged. The owner subsequently selected changed-feature-only verification instead of the
full native gate. On merged base `4c5f0fa4dc1a24698062287d0ffa2e7f4129a639`, the affected preflight
selected 148 packages without FALLBACK; its conservative reader closure was retained as overbroad,
not executed. The owner explicitly replaced it with tests and vet for the five provider/MCP/spec
packages, six formatting/spec/traceability checks, and the separate real Playwright 1.60.0 matrix.
Both superseded enrollments are preserved as canceled non-success. Final selected-check and CEM/OCM
observations are retained by the replacement enrolled workflow rather than claimed passed here.

## 2026-09-19 DCP-V1: experimental revision-pinned documentation corpus

Issue 31 was explicitly scoped to all six phases. The change supplies a native immutable corpus,
separately attributed native read integrations and CEM citation sidecar, capability-gated stdio MCP,
Markdown/JSON rendering and explicit block maintenance, and an independent flow adapter outside Core.
Gate A required real native receipt decoding rather than trusting normalized verification labels,
separate source/provider commits, complete inventory before analyzer admission, and same-call
maintenance rederivation. Those boundaries govern the implementation. Generated intent remains
proposed; opaque Go run identity, E2E served-build identity, assertion adequacy and external utility
remain unknown. Passing synthetic journey fixtures do not claim real browser execution.

Independent review found cross-binary engine identity, scope additions, native range impact,
excluded-source handling, maintenance publication races and a CEM base-mode gap. The consolidated
repair uses shared compiler identity, native admission/receipt metadata and descriptor-confined
no-clobber publication with retained recovery inodes. Actual two-binary and concurrency regressions
cover the previously hidden boundaries.

Focused corpus, provider, maintenance, adapter, native/CEM and transport tests passed during development.
A new receipt-join regression exposed the host's `/var` versus `/private/var` root alias; reads now use
the caller's lexical receipt root and the same confined file reader. An E2E freshness review exposed
that source identity alone was insufficient; unknown served builds now remain unknown. These failed
iterations were repaired before final verification. Frozen synthetic labels report precision, recall,
abstention, false relationships, latency and bytes, without a savings or universal-quality claim.

The first full gate found help usage lines being interpreted as executable draft examples, an
unupdated analyzer-input fingerprint, and a README claim that differed from the spec digest. Usage
now includes the optional root prefix, the analyzer schema advances to 69 for the added immutable
reader inputs, and the README repeats the contract claim. These native gate failures are retained;
the source must be rebound and the full gate rerun before local completion.

Self-development routes used: actual pre-change query/context, dirty affected advice (UNKNOWN with
explicit frontiers), and the committed self-corpus fixture. Draft maintenance was exercised by the
new corpus path; existing draft host adoption and external provider/host interoperability remain
NOT_OBSERVED. CEM/OCM and clean selected-check receipts are retained by the enrolled local completion
session. Independent review and final gate results must be inspected before completion; the build
log records the contract, not an assertion that a pending gate already passed.

Integration with current main preserves the native work-init and fixture-planning additions.
Independent delta review identified three new native option values that the corpus wrapper must
leave untouched; an option-isolation regression covers that compatibility boundary. The merged
source requires fresh canonical verification and newly bound evidence against current main.

The integration gate exposed an existing Go provider admission hang: a missing capability descriptor
could be reused as a runtime pipe, and the byte-bounded read waited indefinitely before cancellation
was installed. The failed gate was stopped and its owned processes were confirmed gone. Under
GLTP-V0-026/028/040, admission now requires exactly 32 preloaded bytes plus EOF from a pipe and uses
raw nonblocking reads. Deterministic empty/open and complete/open pipe cases failed before repair;
closed short/complete/oversized cases preserve refusal and valid capability behavior. This restores
the existing bounded-refusal contract without changing authority or qualification claims. Fresh
evidence and full verification are required after this repair.

Issue 53 adds only an opt-in native producer/reconciler for the existing issue-40 profile. Gate A
rejected a conditional fallback, a CLI-only proof that stopped before the compiler, and silent
genericization of the legacy `golf_e2e` wire member. The accepted shape therefore retains
unconditional full-suite fallback and that compatibility field, and proves mapped input through
separately retained migration/discovery/runtime artifacts into the existing Build/Open boundary.
Nested evidence objects stay closed; only outer record and scalar/list field names are mapped.

The owner clarification added while implementation was in progress makes the caller-reviewed
documentation inventory normative. Gate A was rerun before continuing. The revised result therefore
retains a normalized projection keyed by globally unique variation IDs with explicit preconditions,
actions, observable facts, expected outcomes and allowed projects. Tests carry closed semantic claims;
reconciliation compares those claims, assertions, pages/events/controls, project executions and
runtime witnesses in both directions. Source/test proposals cannot mutate that projection. Semantic
mismatch, undocumented tested behavior, documented untested behavior and missing/extra project
witnesses remain explicit fail-closed frontier rows.

Pre-change `query` selected unrelated genesis evidence and retained four omissions; tracked-path
impact selected the corpus implementation/tests with 119 omissions. Dirty `affected` selected the
corpus and CLI packages, kept language/frontier unknowns, and independently required `make gate`.
The enrolled local completion session freezes the documentation-corpus spec plus focused, full Go,
vet, interop and requirement-definition checks. Exact consumer data and live browser execution remain
NOT_OBSERVED. Independent Sol/high review found incomplete lost-link deltas, observation-subject
repair, permissive previous-result validation and incomplete orphan/runtime diagnostics. Two bounded
repair passes closed those findings, including independently testable variation-to-flow,
variation-to-test and test-to-variation losses; focused adapter and CLI tests passed after repair.
Gate B then exposed that same-revision lineage rejected ordinary historical comparison and that some
mapped-input errors named only a logical field rather than its exact JSON pointer. The owner selected
the backward-compatible interpretation of the vocabulary criterion: `golf_e2e` remains attributed
legacy caller input, while all new mapping/result/diagnostic vocabulary stays domain-neutral. The
repair admits only self-consistent earlier revisions of the same repository identities, validates
retained artifact digests, and reports exact mapped record pointers. The second Gate B pass found
three remaining diagnostic defects: compound validation could name the wrong field, trailing empty
RFC-6901 tokens were collapsed, and map iteration made multi-field refusals nondeterministic. Ordered
field decoding, field-specific validation and literal pointer composition close those cases with
regressions. Final gate and CEM/OCM reports remain pending; no acceptance, runtime authenticity,
utility, narrowing or promotion claim is recorded here.

## 2026-09-19 NTP-V0 integration with repository work-queue adoption

The owner authorized merging the verified fixture extension. Current main `d9144000` also
implements WQO repository adoption; the combined command keeps native fixture dispatch separate
from adoption and shadow observation. Both build-log entries are preserved. The fixture decision
is renumbered from 0321 to 0322 because the adoption decision already owns 0321 on main; accepted
requirements and the original prerequisite approval are unchanged. Historical review receipts keep
their original identifiers. The completed `1f639857` gate and `66c33c02` seal remain prior evidence;
integration requires a new current-main CEM/OCM binding and checks on the combined clean target.
GP, live reservations, CONFIG_PIN, admission and production promotion remain held.

## 2026-09-19 NTP-V0 native taskman fixture planning

Decision 0322 records the owner's accepted prerequisite amendment and pre-edit baseline freeze.
The opt-in `work plan-fixture` reads a native fixture journal through fixed audit/status/export
commands, then emits a source-bound priority-first plan. WQO shadow selection is unchanged.
A built task-store fixture selected P0 touching A+B over the larger P1(A)/P2(B) wave; two complete
reads were byte-identical and all 57 source/store files stayed unchanged. Exact binary, snapshot,
source hashes and output are in `.agent-evidence/native-taskman/fixture-smoke-final.json` and its
adjacent receipt/source manifest. Fixture reservations/history remain caller-owned observations.

Independent review reproduced whole-repository exclusion escaping empty resource sets, native
Code enum drift and malformed observation/history acceptance. The repairs check whole scope before
pairwise resources, preserve the closed native Code enum (DEVELOPMENT_MODE for fixture SELECTED),
and reject contradictory identity/revision/resource/history input without releasing reservations.
`internal/taskman/review_test.go` retains the failure regressions; focused planner, adapter, WQO
boundary and cancellation tests passed. The exact committed native gate and CEM/OCM/local completion
reports are post-commit evidence; these focused results alone do not close that gate.
The first full gate at `865e554b4e5a601282423dbdbbf6264dfdda1b6e` failed only
`internal/specindex`: header/digest/README wording did not exactly match the registry. The
metadata was aligned and the focused registry check rerun before rebinding; the failed gate is
retained in the enrolled check history, never represented as a passing full gate.

Self-development routes used: original prechange query (including omissions), native fixture
planning, `affected` (two Go units advised; full gate mandatory), tracked-path `prove` (97 omissions),
and the enrolled keyed dogfood workflow. New untracked-path `prove` refused; no substitute success
is claimed for it. Snapshot indexing was used by fixture planning. Batch/mutation/learning/service,
foreign adapters and live-provider qualification were not applicable to this bounded fixture slice.
Exact ATCP history remains unrecovered. GP is NOT_RUN: retired harness replacement is diagnostic
only, complete workloads/allocation/I/O/environment witnesses and real runtime conditions are absent.
No executor admission, CONFIG_PIN, production completion, real-queue cutover or performance promotion
is claimed. The explicit executor dependency handoff remains open.
## 2026-09-19 TJAA-V0-010..017 / AFP-V0-018 Playwright project-aware affected selection (Beamfall/corvint#18)

Follow-up qualification (2026-09-20): the repository-owned manifest
`internal/liveverify/affected/typescript/testdata/playwright-qualification.tsv` independently fixes
117 file identities, nine feature cohorts and 468 cases. `TestPlaywrightQualification` checks the
generated inventory before selecting, then compares seven change scenarios against cohort-based
expected project/file sets. Page object, scenario builder and helper changes each select 41 units;
one spec selects five; shared fixture, setup and config each select all 353. The 1,187 required-unit
observations have zero misses and zero extra units (file-unit recall and precision both 100% on this
synthetic corpus). Untouched cohorts are negative controls. Four tag categories per file exercise
shared, Angular, React and Chromium case identity without inferring file exclusions from grep.
Dynamic imports and unknown membership must include every one of the 353 baseline units with no
exclusions; unknown projects emit no runnable approximation and require full-config fallback.
Every scenario repeats identical canonical bytes.

The existing reporter parser and pinned test-validity projector compose retained source/config
digests with a green report: matching E2E inputs remain freshness UNKNOWN, source mismatch and stale
build remain STALE, absent input remains UNKNOWN, ambiguous envelopes are refused, and strength
remains NOT_MEASURED. This is synthetic retained-evidence composition, not a provider run or an
authenticated cross-project join. The latter depends on #19. Actual consumer-repository recall,
runtime/framework/OS qualification and full-CI execution remain NOT_RUN; this work supplies the
issue's stated fixture acceptance only and does not relabel the general adapter as promoted.

Self-development routes: pre-change `query` and `make dogfood-change` were used against
833bbd3278dc485696d5b639bf4488f5a0cfe5bc. The initial empty change correctly left CEM preparation,
intent scope and outcome incomplete; the original evidence is retained in the worktree Git evidence
directory. Provider execution is unavailable for this source-only corpus; mutation, documentation
drafting, learning changes, console and service routes are not applicable. No savings claim is made.
The first canonical gate passed the qualification tests but failed `TestIndexCoversSpecsAndHeaders`:
the README summary must preserve the exact Agent digest claim as its prefix. The follow-up restores
that prefix; the failed gate remains retained and cannot qualify the corrected revision.

The existing TypeScript adapter assigns one physical path to one generic graph unit and deliberately
keeps executable Playwright config unresolved. Issue #18 requires the same file to remain attributable
under several projects, so changing `affected-plan/0` would either violate unique path ownership or
silently change its closed bytes. The experimental `playwright-affected/0` profile instead reuses the
generic graph for physical reachability, then expands reached tests into project-distinct units.

The static config subset binds the config digest, project fragment, grep, metadata, file membership,
dependency/teardown edges, browser/device, exact project argv, and a revalidated digest of every
source input observed by the TypeScript adapter. Unsupported dynamic config or
source reachability selects every statically known test/project pair and reports
`FULL_RELEVANT_SUITE`; an unknown project set emits no runnable approximation. Runtime feature flags
and external application state stay visible on the execution axis. The mixed Chromium/Angular/React
fixture covers page-object reachability, setup/dependent/teardown expansion, config widening,
dynamic-import widening, absolute-path matchers, globstar zero-directory matching, partial project
abstention, custom fixture-based test discovery, unsupported-glob widening, transitive setup expansion,
and repeat-byte identity. Real-repository recall and provider composition
remain `NOT_RUN`, so the profile is experimental and issue #18 is not yet promotion-complete.
## 2026-09-19 WQO-V0-046..048 repository work-queue adoption (decision 0321, Beamfall/corvint#20)

Before this change, `corvint work observe` could never return `VALIDATED_AT`. `workManifest.complete`
was never set, so WQO-V0-017 always added `SOURCE_UNQUALIFIED` and `propose-wave` always abstained.
`corvint work init` and `corvint work adapter` now give any repository a committed policy,
worklist, and adapter. The observer qualifies store scope only when it reproduces the
`repository-worklist-v0` documents byte for byte.

A first cut qualified both closed mappings. It turned Corvint's own `decision-0046-v0` observations
`VALIDATED_AT` and broke the WQO-V0-021/025/032 final-check witnesses in `cmd/corvint`, which rely
on an incomplete initial capture ("initial capture did not retain incomplete scope"; three
`WQO-V0-032` codes became `MALFORMED_INPUT`). Restricting qualification to the adoption mapping kept
those witnesses and the self-dogfood contract unchanged.

Pre-landing review reproduced a symlink escape and an interrupted-write residue in `work init`.
The repair roots every write with `os.Root`, rejects a symlinked `.corvint`, delays success output,
and rolls back files created by a failed or racing initialization.

A second independent review found that init still accepted a plain directory or repository
subdirectory, a partial committed adoption without the worklist surfaced `ADAPTER_FAILED` instead
of `SOURCE_UNQUALIFIED`, and the generated adapter unnecessarily required Bash. The repair refuses
non-root targets before writing, preflights the committed adoption worklist with the other source
inputs, and emits the POSIX-only adapter with `/bin/sh`.

The first canonical-gate attempt after that repair intentionally did not qualify: the release
artifact conformance test refused the staged repair as a dirty worktree. The repair was committed
unchanged before rerunning the gate from clean, frozen source.

After the final evidence seal, current `main` advanced with the accepted corvid artwork. Merging it
correctly invalidated the pinned README workflow citation because the themed logo moved that span by
one line. The canonical gate refused the stale pin; the repair relocates it to
`README.md:188-198@3297e31e` without changing the cited workflow.

`TestWorkAdoptedRepositoryWorklist` starts from a clean fixture and runs init, a refused second
init, commit, observe, and propose-wave over four verification tickets. One is a suite batch, one a
failure-classification repair, one a test-validity receipt, and one a cleanup/retry that shares
`internal/parser`. The result is `VALIDATED_AT/UNCHANGED_OBSERVED`, `ELIGIBLE_AT` with three
tickets selected, the cleanup/retry ticket `EXCLUDED` with its collision group, and a byte-identical
repository manifest. A scratch-repository probe of the built binary recorded the unknowns:
`ERROR/SOURCE_UNQUALIFIED` with no policy, with an uncommitted adoption, and with a tracked
modification, and `ERROR/ADAPTER_FAILED` when `corvint` is absent from the fixed `PATH`.

## 2026-09-19 CRB-V0-014: owner-selected corvid artwork

The owner selected the first, corvid direction from three generated concepts and then approved
adoption ("look good. use it"). The chosen silhouette
was redrawn as editable SVG with a custom lowercase wordmark; the light, dark and universal mark
and lockup variants share geometry. The README selects a theme-appropriate lockup; the 24 px editor
icon inherits its host color. `assets/brand/README.md` records usage, raster dimensions and rollback.

Manual asset checks passed: SVG parsing, accessible titles/descriptions, no font or external-image
dependencies, PNG dimensions/alpha, shared variant geometry and README references. Independent
Codex review passed with no material findings, including comparison to the selected concept,
24 px legibility and exact SVG-to-PNG rendering parity. Requirement-index regeneration and focused
spec-requirement, requirement-definition and decision-number checks passed using an isolated index
containing the updated spec; `git diff --check` passed. Go/runtime code is unchanged; the full Go
and release gates were not run for this manual artwork slice, and installed editor qualification
remains unclaimed.

Pre-change Corvint query ran against `6a423ac091d848b8ac5b49e8002c61c252993ac3`, with three ranked
results omitted and two test-path candidates withheld. Dirty-Go advice, mutation and retrieval
evaluations are not applicable. The initial `make dogfood-change` returned `not-complete`
(`cem-prepare: git-diff-failed`, missing intent scope and outcome input); it is not passing evidence.
Post-commit CEM/OCM qualification is NOT_PRODUCED for this manual asset slice; native
CEM's PNG binary-patch limitation remains explicit. Original query, generation prompts and manual
asset hashes are retained in `/tmp/corvint-logo-20260919/` for this task.

## 2026-09-19 AFP-V0-012 fast tier: the CEM sidecar's readers, the cutover-test frontier, and the unresolved floor

Decision 0323 narrows rule (c) for `.corvint/change.cem.json` to readers whose literal resolves
to it or can form it under rule (c)'s outer partial-component semantics. The independent review
found that exact resolved-string comparison omitted a package constructing the path as
`filepath.Join("..", ".corvint/change.cem") + ".json"`; the repaired selector conservatively pairs
a naming fragment with a root-climbing or compatible root-anchored token in the same package.
These are selector-only package counts, with no tests run, on `Russells-Mac-Studio.local`,
`go1.27.1`. Selection counts come from `tools/gate-affected-select` over one `corvint affected`
receipt. The script end to end with `GO_TEST_COMMAND=:` agrees. A branch from `3f30a02` changing
`internal/touchsurprise/compute.go` plus the sidecar: 121 → 116 packages, where 116 is the count
for the Go change alone. With only the sidecar dirty: 112 → 104. `feat/build-number` against
`05e17d0` (29 changed paths): 150 → 148.

The report that "the sidecar selects nearly every package" overstates its share. The audit prints
only the first cause per package, so 105 `reader` lines do not mean 105 packages added by
literals. Of the 30 packages that named the sidecar by fixture tokens, most were also reached by
another rule. The dominant width is rule (d): 103 packages are `unresolved` at `3f30a02` and are
selected whenever any path is dirty. 37 call `runtime.Caller` or `os.Getwd` themselves. The rest
carry a root-reaching literal or depend on a package whose non-test code locates the root, 10 of
them through `cmd/corvint`. With 2 or 3 packages from the plan, any change therefore selects at
least about 105 packages.

`cmd/corvint/go_only_cutover_test.go` puts 32 packages on the frontier: `cmd/corvint` and 31
dependents. `cmd/corvint` is `package main` and has no importers. Every dependent comes from a
token edge, a literal naming `cmd/corvint` such as `go build ./cmd/corvint`, or from importers of
those packages. This is justified under the current rules. `go build` skips `_test.go`, but
`go test` and `go vet` of that package compile it, and the index cannot tell which command a
literal feeds. Those direct namers are also rule (c) readers of every path under `cmd/corvint`. A
throwaway variant stopped propagation through tokens that occur only in `_test.go` files, since
test files are never imported. It moved the single-path selection from 119 to 114. The 5 packages
it drops are `cmd/corvint-analyzer-python`, `cmd/corvint-docs-mcp`, `conformance/frontier-v0`,
`internal/dogfoodocm`, and `internal/frontiernextrepo`. It is not adopted here.

## 2026-09-18 EEP-V0 provider-to-impact workflow: synthetic fixture evaluation, and unsupported cases

`TestImpactProviderEvaluation` (`internal/extevidence/extevidence_test.go`) runs the
provider-to-`impact` composition path (`Section`) over the 5-relation mock fixture
`internal/extevidence/testdata/mock-provider.json`: 3 relations that must be admitted (one
`declared` `implements`, one `inferred` `mockdocs:enables`, one `observed` `verifies`) and 2 that
must be excluded (one `learned` relation, `EEP-V0-007`; one relation from a foreign provider
endpoint, `EEP-V0-006`). Results on commit `4a2c00f` (`Russells-Mac-Studio.local`, `go1.27.1`):
precision 1.000 (3 of 3 admitted rows expected), recall 1.000 (3 of 3 expected relations admitted),
zero false-positive relationships, zero `learned`-evidence admission, abstention accuracy 2 of 2
(the learned relation reports `excluded` and the foreign-provider relation reports `unresolved`,
both under `unknowns`, neither admitted), latency about 0.68 ms, and a 3022-byte `context.external`
section. `TestSelectionEvaluation` was rerun the same day over its existing corpora (63 cases:
see the two entries below) with the same results already on record. The fixture used here is one
record with five relations, not an independent corpus; the entries below already exercise a larger,
independent labelled set for the fail-closed selection profile. This shows the exclusion and
authority-assignment rules hold on the documented worked example; it is not an adopter outcome.

Unsupported in this slice, per `external-evidence-provider-v0.md` and `external-test-selection-v0.md`:
command, MCP, and remote provider transports (file transport only); multi-hop obligations beyond one
relation hop; and checkout worktree inspection for a V1 checkout binding (a checkout's canonical path
is echoed, never opened). None of these are measured above; none are estimated. The Change Frontier
sidecar for external obligations landed separately (EFO-V0, entry below) and is not measured here.

## 2026-09-18 EFO-V0 external obligations sidecar: reference-only join to the frontier

Decision 0313. `corvint obligations --cem FILE --impact FILE` writes `external-frontier-obligations/0`
(`docs/specs/external-frontier-obligations-v0.md`). The CEM and frontier wires are unchanged; the
sidecar cites the frontier through `binding.cem_sha256` (the `CF-V0-019` raw-copy digest) and CEM hunk
ids, and nothing reads it. Tested by `TestObligations*` in `internal/extevidence` and `cmd/corvint`;
no adopter receipt or labelled review sample exists yet, so promotion stays open.

## 2026-09-18 EEP-V2 path-to-path relations: synthetic conformance evaluation

Decision 0312. `TestSelectionEvaluation` now runs over both labelled corpora: the 23 ETS-V0 cases,
plus 40 cases in the independent two-repository fixture
`internal/extevidence/testdata/conformance-path/cases.json`, 63 in all. Ten of the path cases
cover directory scopes (`EEP-V2-012`, `EEP-V2-013`): a held path, a changed test inside the scope,
a scope that widens, and missing, dirty, other-repository, and slash-less cases that must not
narrow. Results: precision 1.000 (36 of 36 selected tests expected), unsafe narrowing 0 of 49 cases
that must not narrow, abstention accuracy 6 of 6, latency p50 about 98 ms and max about 181 ms per
case on one loaded development host, and a largest `test_selection` member of 5393 bytes. The V1 and no-provider CLI
tests keep their bytes. A V1 record carrying the same path relation stays an `unsupported`
unknown, which is why `TestAffectedSelectionPathRelation` gives `full` under V1 and `narrow` under
V2. The corpus is synthetic and was authored with the feature. It shows that the per-side and
worst-side rules hold. It is not an adopter outcome.

## 2026-09-18 ETS-V0 external test selection: synthetic conformance evaluation

Decision 0311. `TestSelectionEvaluation` over the 23 labelled cases in
`internal/extevidence/testdata/conformance-selection/cases.json`: precision 1.000 (16 of 16 selected
tests expected), unsafe-narrowing 0 of 20 cases that must not narrow, abstention accuracy 3 of 3,
latency p50 about 35 ms and max about 125 ms per case on one development host, and a largest
`test_selection` member of 4470 bytes. The corpus is synthetic and authored with the feature, so
these numbers show the fail-closed rules hold. They are not an adopter outcome, and promotion needs
a corpus drawn from a real change history.


## 2026-09-22 — OpenCode explicit MCP compatibility (V1-0022)

The installed OpenCode client sent the legacy initialize handshake and received method-not-found
from the modern-only server. A positive modern discover replay on the same binary/root isolated
protocol admission as the defect. The owner approved correcting the frozen check plan before
implementation; independent plan review resolved ordered initialization and legacy response framing.

The opt-in 2025-11-25 profile reuses the bounded shared stdio transport and all four existing native
tool registries. The first real client probe then exposed omitted tools/list params; normalizing
those at the legacy boundary produced successful real OpenCode discovery. Both failed exchanges
and the successful probe are retained in the local release evidence packet. ClientInfo versions
1.17.18 and 1.18.31 were observed separately with CLI 1.18.31. Discovery alone does not prove actual
host tool execution, sealed workflow usefulness, official conformance or formal FULL authority.
Existing docs/corpus workflows, test-validity vectors and INT/TERM descendant checks now exercise
both profiles. The original modern contract, companion distribution set and release gates remain.

The subsequent real OpenCode run executed `corvint.status` and received the READY read-only
repository receipt. The provider was an explicit deterministic localhost fixture with native
outbound-network restriction, not an inference or cost benchmark. The independent implementation
review found one completion-evidence defect: the new clauses initially followed a level-two
heading, outside OCM's Requirements parser. Moving that heading to level three preserves actual
clause enumeration; final OCM linkage and frozen checks remain required before local completion.


The dedicated OpenCode task retained that source and enrollment, then corrected the developer
preview installation: the example now uses an available local file URL instead of an unpublished
npm name. A separate native configuration enables core and test-validity MCP servers with the
explicit legacy selector. The packaged OpenCode skill routes existing tools on demand without
expanding the closed MCP registries. Independent review found no remaining protocol defect and
no blocking setup or skill issue; its forward cases preserved Unicode, non-Go and missing-test
boundaries. Executable requirement anchors now make MCPV0-021..023 directly linkable by OCM.

The 1.18.31 native probe loaded the plugin and skill and executed status, query, tracked-Go impact,
test-validity discovery, native context and caller-reported blocked-outcome tools. All repository
receipts matched the temporary fixture commit; absent retained tests stayed UNSUPPORTED. A first
probe inherited the parent PWD and therefore selected the wrong project despite subprocess cwd;
that failed fixture was retained and corrected by binding the child PWD to its actual directory.
The loopback-only deterministic provider supplies transport evidence, not model-quality evidence.
The expanded probe's INT/TERM cleanup is checked separately. Original and expanded probe receipts
remain under the private `corvint-v060-evidence/opencode-compat` and `corvint-opencode-evidence`
temporary directories. Full gates and exact final CEM/OCM closure remain separate observations.

Focused protocol/server regressions passed. Direct JavaScript invocation was refused because the
suite requires its Go-owned native fixture; the canonical host-adapter target remains the runner.
The documentation check exposed an unquoted requirement range parsed as a duplicate definition;
quoting that prose range preserves its meaning and the executable requirements. The original
failed checks remain in the private follow-up evidence.


### 2026-09-22 — patch companion qualification found an uninitialized planning branch

The exact `0.5.0a4` candidate `04e9d473` passed reproducible companion assembly and native
OpenCode's seven tool routes. Installed core qualification passed provider transitions and docs/MCP
stages, then failed planning seed with `INTENT_BRANCH_MISMATCH` against the unchanged Tasks
`e6b9d766` dependency. The planning helper created an empty `.git` directory while declaring
`intentBranch: main`; Tasks initialization correctly refused it. The same helper is present at the
public `v0.5.0a3` tag, whose published assets contain no companion/Tasks dependency. Initialize the
fixture's real Git repository on `main` before Tasks initialization, and exercise the seed with a
different caller default branch. Preserve the failed candidate and scratch as failure evidence;
rebuild and qualify the corrected exact candidate before publication. No MCP or Tasks authority
contract is weakened, and no partial installed-stage success is an overall qualification.

## 2026-09-22 AHI-002, AHI-003, AHI-010, AHI-024: Pi lifecycle and outcome repair

The owner requested a complete Pi integration audit. Independent review reproduced discarded
startup/compaction packets, hidden outcome-persistence degradation, malformed outcome JSON
silently normalized to empty input, and conflicting package/native adapter identities. Four
handler regressions failed against the original logic before repair. The extension now supplies
bounded startup/compaction recovery once through Pi's ephemeral context hook, including retries
without a new agent-start event, clears stale session/root state, rejects duplicate and mixed
identity command input, and exposes fallback persistence limits without storing automatic faults
in model history. Native invalid outcome input is distinguished from core unavailability. Package,
native translator, shim and compatibility metadata agree on 0.1.2; the host pin remains 0.85.1.

Actual Pi 0.85.1 on macOS arm64 passed an offline local-provider fixture for package install,
disable/update/re-enable/remove, two-turn ephemeral context and session non-persistence, explicit
outcome errors and startup SIGINT/SIGTERM descendant cleanup. The canonical host-adapter target now
runs the Pi JavaScript regressions. Full gate and immutable CEM/OCM completion evidence are retained
in the task's private dogfood reports, not inferred from these focused passes. No protected FULL,
interactive TUI/RPC, Linux/Windows, latency/recall or outcome-persistence claim is added.

Self-development routes used: initial query, pre-change dogfood coordination, dirty-diff affected
advice and native adapter/host tests; final CEM/OCM/frontier/finish use the enrolled plan. The first
query's measurement was not started in advance (NOT_OBSERVED); no savings claim is made. The
initial same-base coordination reported cem-prepare git-diff-failed, missing intent scope and
outcome input, retained under /tmp/corvint-pi-audit/start-dogfood.log. Non-Go path impact, provider
qualification, trace migration and mutation testing are inapplicable to this adapter repair.

### 2026-09-22 — OpenCode automatic file-change deadline

The user reported file-change `corvint-command-failed`/`timeout` on an unavailable work repository.
Against the exact `1ed0e673` binary, this repository reproduced a 500 ms default timeout while
the existing 2,000 ms override completed a receipt in 1,077 ms. These are correctness observations
under concurrent gates, not p95 results or a diagnosis of the remote repository. OpenCode now uses
the existing automatic ceiling by default and reports the actual deadline as a bound rather than
a diagnosed fault. A 750 ms valid-receipt regression fails under the old default; default hang,
explicit override, descendant cleanup, and successful FALLBACK preservation remain covered.
Independent review found no blocker. A subsequent dirty-worktree invocation still exceeded the
ceiling, so this repair makes no universal latency or absence-of-timeout claim. Query deadlines,
fault visibility and legitimate evidence degradations are unchanged.

### 2026-09-22 — installed roadmap proof followed obsolete table markup

Installed candidate `1ed0e673` passed the corrected planning seed, then failed its roadmap browser
inventory assertion. The retained console at the original fixture location renders eleven unique
ticket links and no forms, inside roadmap cards; the proof still searched for table links. Update
only the three selectors to the existing roadmap-ticket container, preserving count, detail,
Origin/Host/session refusal, state-preservation and cleanup assertions. A diagnostic copy of the
Tasks store correctly refused relocation and was discarded as qualification evidence. Browser
diagnostic attempts were interrupted and remain failed; HTTP inspection establishes this selector
defect but does not replace the required complete installed browser qualification.

## 2026-09-22 decision 0332: integrate reviewed source for the 0.6 candidate

The version slice passed its tuple, historical-identity, documentation and editor checks, then
strict completion at `055257f` and seal `7230485`. Its later sealed head is not relabelled as the
completion target. New enrollment there refused `worktree-prior-completion-stale`; a fresh isolated
checkout of the same sealed base was enrolled before source edits, preserving both generations.

The integration retains main's publication-record content (`63635b4`), reviewed MCP/OpenCode
source (`5f5b754`, `dbdda0d`), the declared-branch fixture repair (`39b61a0`), Pi fallback repair
(`b8b2ffc`), OpenCode automatic-event deadline/diagnostics (`4d1b9b7`) and the reviewed roadmap
selectors (`98d74ea`). Source-only commits preserve original references and receipts; importing
sealed sidecars would violate the new slice's CEM boundary. Build-log conflicts retain each added
record without copying unrelated alpha version changes. The requirement locator is regenerated
from the integrated specs. Later Pi protected/FULL work remains a separate lane.

The Pi input's full gate failed a query fixture restoration assertion; its focused repetitions do
not establish a fix or gate pass. The pending test backlog retains that failure and unknown cause.
Prechange query and tracked-Go impact were captured at the actual isolated root; query omissions
and the initial empty-diff coordinator failure remain in private evidence. Independent plan review
accepted the source/check scope and exact-target sequence. No new runtime behavior was designed
in this integration. Its CEM/OCMs, final gate and independent source review must bind the integrated
target; earlier input checks cannot qualify it. Optional browser/host evidence remains separately
qualified, and sealed correctness/cost and genuine dual-repository workflow evidence remain
required before owner acceptance of the exact 0.6 packet.

## 2026-09-22 MCPV0-011: literal lifecycle claim anchor

Candidate 794f93d passed the full gate, focused checks, four-archive qualification and direct
OpenCode tool/edit proofs. Native completion correctly refused an explicitly assessed MCPV0-011
claim gap: the existing stdout-close test names the requirement only in an excluded comment.
A literal named subtest now wraps the unchanged stdout-close lifecycle assertions. The wrapper
preserves all behavior, deadlines and cleanup; extraction and completion policy are unchanged.
The known gap is not relabeled unassessed. The new source invalidates target-bound qualification;
prior passes remain historical and mandatory checks must bind the replacement candidate. The
optional companion is omitted under decision 0331 because its separate installed browser stage
reached its three-minute timeout; that failure's root cause remains UNKNOWN.

## 2026-09-22 context-repository-anchors: TCP-V0-022 opt-in verbatim anchor field (V1-0084, decision 0333)

Contract: with `CORVINT_CONTEXT_ANCHORS=on`, five fixed anchor classes (quoted error string,
URL, `Scope::Value` enum, dotted config key, `file.ext:line` frame) are extracted from the task
and verified verbatim under a whole-anchor edge rule against the bounded bodies of the sources
whose `Words` postings share every word run of the anchor. Credit is the body-term BM25 form
inside the lexical slot; the reason carries the distinct prefix `anchor: `literal` xN verbatim; `;
the row keeps score 300 and authority `vocabulary`, so reserved authority rows always precede it.
Bounds: 4 to 256 bytes per literal, 16 anchors per task, 512 candidates per anchor (abstain
beyond). No index, snapshot or pack change; unset or other flag values keep the packet bytes
(`TestContextAnchorsDefaultBytes` against the recipe golden).

Decision: opt-in rather than default, the TCP-V0-019 shape, because the promotion evidence cannot
be produced on this host. Decision number: 0321 was assigned but already names the work-queue
adoption record on `main`; 0333 is used.

Evaluation: NOT_RUN. The frozen retrieval evaluation is `tools/retrieval-bench` over the external
`agent_retrieval_bench` releases (`benchmark/v2_*/…jsonl`, `corpus/v2_*`); neither directory is
present on the build host, so no flag-off/flag-on numbers exist. The bench also has no
anchor-bearing sample subset, so "anchor-bearing queries measured separately" needs a bench change
outside this ticket's ownership. The acceptance criterion is open, not met.

Gates: `go test ./internal/contextindex/ ./internal/specindex/`, `go vet`, gofmt, and the spec,
requirement, traceability, decision-number and line-citation checks; results in the ticket
report.

Audit review (IDX-SNAP-V0-017): the change adds `context_anchors.go` and edits `taskcontext.go`
and `context_terms.go` on the query side only; no extraction, fact or pack encoding changes, so
`analyzerSchemaID` stays `corvint-analyzer/73` and only the `TestAnalyzerSchemaInputs` source
digest is refreshed.
## 2026-09-22 tcq-environment-variants-and-flake-qualifier: shared flake rule and declared observation variants

Ticket `V1-0091`, decision 0339, requirements `TCQ-V0-048..050` in
`docs/specs/test-claim-qualification-v0.md`. A test observation may now declare a ResultDB-style
environment variant (`environment` object, key grammar `^[a-z][a-z0-9_]{0,63}$`, at most 32 pairs,
values at most 256 bytes); an undeclared variant is reported unknown and adds no wire member, so the
frozen `conformance/tcq-0` vectors are byte-identical. `tcq.Flaky` is the one divergence rule: more
than one distinct terminal status among `PASSED`/`FAILED`/`ERROR` across runs. `Request.PriorObservations`
(at most 16, dynamic tuple only, target-bound) lets TCQ pool row statuses per execution key across
byte-identical variants; a divergent key adds reason 18 `test-flaky` to claims that matched a row and
removes the relation while the current report state stands. The JS provider derives its Playwright
state and `flaky-retry` reason from the same rule over recorded attempts; a reporter label can only
add the qualification. Evidence: `TestObservationEnvironmentIsAdditive`,
`TestFlakyRuleNeedsDivergentTerminalOutcomes`, `TestSameRevisionDivergentOutcomesAreFlaky`,
`TestPriorObservationVariantMismatchIsNotFlaky`, `TestPriorObservationsRequireDynamicTupleAndTarget`
(`internal/tcq/flake_test.go`), `TestFlakyOutcomeIsSharedRule`
(`internal/jstestprovider/projection_test.go`). Not done here: the frontier CF-V0-016 closed
vocabulary and `docs/tcq-0.schema.json` do not yet list `test-flaky` or `observation.environment`;
the frontier shim supplies no priors, so neither can reach them today.
## 2026-09-22 V1-0009 read-only compiler qualification: case-fold collisions and a read-verb tripwire

Two hostile-path tests and one filesystem tripwire qualify the read-only Core evidence compiler
(ticket V1-0009, requirements GPK-V0-006 and GPK-V0-007; no requirement text changed). The
case-fold fixture commits `internal/token/token.go` and `internal/token/Token.go` through Git
plumbing (`hash-object`, `update-index --cacheinfo`, `write-tree`, `commit-tree`) so it exists on
case-insensitive macOS, where the worktree can hold only one file. Measured on APFS: `Build`
pins each path to its own committed blob (token.go f3c6b480, Token.go bc6664aa), never the other
case's bytes; `DirtyPaths` equals Git's ` M internal/token/Token.go`; `query` and path `impact`
report freshness `mixed-worktree` naming that path and `learning.local_trace_state`
`blocked-mixed-worktree`; `context` returns both rows under distinct blob hashes; range `impact`
refuses `unsupported-impact-worktree` (exit 2); two consecutive runs of every verb are
byte-identical. No verb silently picks one file, so no kernel change and no `t.Skip` was needed.
The case-sensitive branch of both tests (no dirty path, `fresh`, range impact accepted) is
written but not measured locally. `TestReadOnlyVerbsWriteNothing` snapshots the whole fixture
tree, `.git` and `.corvint` included, as path to kind, mode, sha256 and mtime before and after
`init`, `adopt`, `query` (with and without the `unplanned-reads.enabled` marker), path `impact`,
`docs draft`, `harness event` session-start and stop, and `lrf`; the only permitted delta is the
one self-observation row on session-start with `.corvint/` gitignored. A negative run over
`index` tripped the comparison on the index files, so the tripwire is live. No decision was
recorded: tests alone need none, so the reserved number 0342 stays unused. Gates run: gofmt, `go build ./...`, `go vet ./...`,
`go test ./cmd/corvint/ -run 'ReadOnlyVerbs|CaseFold'` (also `-count=5`),
`./internal/contextindex/...` and `./internal/specindex/` fully, and the spec-requirements,
requirement-definitions, traceability-tests, decision-numbers and line-citations checks. Open
observation for follow-up: the `context` receipt carries no freshness block, so the worktree
divergence reaches a `context` caller only through the blob hashes, not a named state.
## 2026-09-22 stable-operations: V1-0017 lifecycle check, hostile matrix, support window, command runbook

Decision 0341 and `docs/specs/stable-operations-v0.md` (`SOP-V0-001`..`012`). Two additive shell
checks, no Go change. `script/check-install-lifecycle.sh` ran the eight steps against a built
binary in 2.5 s and its wrapper (two stamped builds, archive path, tampered `SHA256SUMS`, usage)
in 9.3 s. `script/check-hostile-regressions.sh` ran 27 rows over 12 packages, all PASS, in 28.9 s
with `memory` and `case-folds-context-index` printed NOT_COVERED; its stub-driven wrapper passed in
4.5 s. Independent finding against the ticket wording: a truncated or byte-damaged snapshot does
not fail closed; the read verb exits 0 with byte-identical packet output and leaves the file
untouched, and `index --if-stale` rebuilds a same-size snapshot that differs in 460 bytes, so the
spec fixes packet identity as the recovery invariant and leaves snapshot byte identity to
`index-snapshot-v0.md`. The brief presumed case-fold and interruption cleanup uncovered; both have
tracked regressions and sit in the matrix. Evidence is darwin arm64 only; other hosts NOT_RUN. No
frozen evaluation applies to this operations slice. Proposed `make` targets
`install-lifecycle-test`, `hostile-regressions-check`, `hostile-regressions-test` are not added
here. `docs/SECURITY.md` still says the support window is a draft owner decision and is stale
against `SECURITY.md`.
## 2026-09-22 cem-0-3-structural-mechanical: Go structural mechanical reasons the verifier re-proves

Ticket V1-0087, decision 0338, spec `docs/specs/cem-0.3-structural-mechanical.md` (`CEM-SM-001..010`).
`cem/0.3` is `cem/0.2` plus `rename`, `move`, `import-reorder`, and `formatter-only`; 0.1 and 0.2
maps still reject that vocabulary, `prepare` still emits 0.2, and `mark` upgrades only when a
structural reason is used. Each reason is recomputed from the base blob and the patch with the Go
standard library and refused as `unproven-mechanical` on any parse or format failure. Fixture
pairs cover a true positive and a near-miss per class (a hidden `"hello"`/`"hi"` edit, a
`ToUpper`/`ToLower` swap, a shadowing rename, a `helper(21)`/`helper(22)` edit), plus selector,
exported, directive, `var`-order and `init`-order refusals. One admitted overlap: gofmt sorts
imports, so an import reorder is also formatter-only. No frozen evaluation exists for mechanical
classification precision; the reviewer-audit kill gate in `docs/CHANGE-EVIDENCE-MAP.md` (20%
false classification) is the only measured bar and was not exercised here. Not shipped: a
`cem-0.3.schema.json` and `protocol/cem-0.3` vectors (Apache-2.0 boundary), frontier and dashboard
profile acceptance, exported or type-aware renames, non-Go languages.
## 2026-09-22 issue 64 close-out: `deleted` verification, `affected --provider-command`, vocabulary mapping

Issue Beamfall/corvint#64 (generic revision-aware provider contract) was audited in 54fe6c3 as
mostly delivered by EEP-V0/V1/V2, ETS-V0/V1 and EFO-V0; this change closes the actionable
remainder. `EEP-V0-010` gains `deleted`: an untracked path endpoint whose path the record's
declared revision tracked, decided by one extra `git cat-file --batch-check` per view per
repository over the untracked paths only, and only when freshness is `repository-ahead`,
`provider-ahead`, or `unrelated-history` (so a `revision-unavailable` record can never claim
deletion). `deleted` is stale for `EEP-V1-008` and keeps the ETS code `missing-path-reference`,
so no selection wire changes (`TestReferenceVerificationDeleted`,
`TestTwoRepositoryDeletedTestPath`). `corvint affected` now takes `--provider-command ARGV_JSON`
under the same parser and bound as `--provider` (`EEP-TR-001`, `ETS-V0-001`,
`TestAffectedProviderCommandMatchesFile`). The issue's freshness vocabulary is mapped onto the
accepted wire names rather than renaming them (recorded in `docs/EXTERNAL-EVIDENCE-PROVIDERS.md`):
`provider-is-ancestor` is `repository-ahead`, `reference-missing` is `missing`/`deleted`,
`not-observed` is `not-verified`, and `reference-ambiguous` needs symbol identity (V1-0101).
Ticket V1-0104's premise was wrong: `--provider-mcp` never existed and the guide's MCP bullet was
accurate; the guide's stale bullets were the two ETS-V1 items (one-hop widening, no checkout
inspection), now corrected. V1-0101, V1-0102, V1-0107 and V1-0108 stay open. Port note: this entry was
reapplied from the old lineage onto the public history, where `impact --provider-mcp` does exist
(decision 0324); the guide's MCP bullet on this lineage is a pre-existing follow-up, not part of
this change.
## 2026-09-22 ocm-v0-conformance-vectors: freeze the OCM V0 proof wire in `conformance/ocm-v0/`

Ticket V1-0013, decision 0343, requirement `OCM-V0-014`. The suite freezes 25 structural vectors
(5 valid, 20 hostile) and 4 fixtures with 19 verifier cases for `ocm/0.1-experimental`. Valid
vectors are the byte output of the real `prepare`, `link`, and `mark` producers over a
deterministic seed repository with pinned Git identity and dates; `TestFrozenVectorsMatchTheRealProducer`
rebuilds all 5 universes on every run and requires byte equality. Every vector runs through the
real structural parser and every fixture case through the real `status` verifier; no double is
used. All 25 declared refusal codes matched the parser on the first run and no vector exposed a
defect, so `internal/lrfrepo` is unchanged. Two outcomes are frozen as observed: an extra top-level
member refuses with `unknown-field` (closed object), and a bound OID absent from the repository
refuses with `repository-object-unavailable`, ahead of `target-mismatch`. The package test runs
in about 24 s on a quiet host, dominated by Git subprocesses for the 9 universes it builds.
## 2026-09-22 batch-A cmd/corvint chores: V1-0030 V1-0047 V1-0048 V1-0051 V1-0052

Five queued `cmd/corvint` chores closed together, all read/write behavior only, no wire or
requirement-ID change. V1-0047: `taskman fixture`'s stdout-write failure now emits the same
`output-failed` error envelope as every other command instead of a bare stderr line
(`taskman_fixture.go`); the stale plain-text assertion in `taskman_fixture_test.go` and a new
case in `output_write_failure_test.go` cover it. V1-0048: the corpus relay's stdout-copy-failure
path (`corpus_integration.go`) no longer appends a second `output-failed` envelope after a failed
native command has already written its own; `docs_corpus_test.go` asserts the resulting stderr is
byte-identical to the native-only baseline. V1-0052: `docs_corpus_test.go` gained a pinned test for
`--root --corpus=FILE`, confirming `--corpus` is not consumed as `--root`'s value and the native
command instead refuses it with `--corpus is supported only on native evidence reads`. V1-0051:
documented the operator-set `CPUPROFILE` env knob (`context` and the harness-event path) in
`help.go`'s `context`/`harness event` help text and in `task-context-packet-v0.md`'s Non-goals and
authority section; no flag added, no requirement ID touched. V1-0030: the advertised
`docs draft`/`docs consume` example (`docs.go`, `source-documentation-draft-v0.md`) referenced
`internal/doccompiler`, which exceeds the SDD-V0-005 64-declaration bound and fails
`documentation-limit-exceeded` when actually run; replaced with `internal/docmaintain --task
Preview`, verified live against the built binary. `TestDocsHelpPrerequisitesAndWorkingExample` no
longer runs the parsed example against a synthetic fixture package that happened to export only
one declaration; it now runs `--root $(cd ../.. )` against this repository's own committed HEAD
source, so it would have caught the original bound violation.

Gates run: `gofmt -l` on the changed files, `go build ./...`, `go vet ./...`, the targeted tests
above plus `TestImpactAndHarnessHelpExposeActualLimitsAndUnsupportedProfiles` and
`TestSupportBoundaryDisclosesTheSelfObservationLedgerWrite`, `internal/specindex`'s
`TestIndexCoversSpecsAndHeaders`, and `make spec-requirements-check requirement-definitions-check
traceability-tests-check decision-numbers-check line-citations-check` — all pass. `REQUIREMENTS.tsv`
unchanged (prose-only spec edits, no requirement IDs touched). No decision record: no published
wire contract, spec bound, or refusal vocabulary changed; the envelope fixes bring two call sites
into line with the pattern already used elsewhere in the same files. Full `go test ./...` was not
run for this scoped batch (per `AGENTS.md`'s Verify section, exhaustive gate reserved for the
terminal boundary); only the targeted tests above and the listed `make` checks were executed.
## 2026-09-22 batch C: bounded lists, published bounds and redundant-parse cleanup

Six scoped fixes closed from the agent-memory backlog. V1-0043 (BBF-V0-010): `validateControl`
now caps `UnrelatedCriteria` and `RequiredSetup` at 1000 entries each (`maxControlListLength`,
decision 0336), so `acceptedReceiptBound` can no longer exceed the 32 MiB document bound; math and
rollback are in `docs/decisions/0336-behaviorfalsify-control-list-cap-2026-09-22.md`. V1-0046
published both previously-unstated bounds next to their spec rows with no new requirement IDs:
`externalMaxConfigInputs` (256) beside PWP-V0's `config-inputs-unobserved`, and the 10s
`cleanupReserve` beside BBF-V0-010; PWP-V0's row also notes the current code path actually refuses
an over-count as `report-output-overflow`, not `config-inputs-unobserved`, ahead of the ticket's
framing. V1-0045: `RunUnit`'s report read now uses its own `unitReportOutputLimit` (16 MiB, equal
to `defaultOutputLimit`) instead of the external provider's 4 MiB `externalOutputLimit`, cited in
`js-live-test-provider-v0.md`'s `report-not-written` row. V1-0056: `qualified-reporter.cjs` now
memoizes `--version` per executable path (`observedVersions`, mirroring `bundledBrowsers`); Go's
`sensitive_input_boundary.go` builds one `strings.Replacer` per receipt instead of one
`ReplaceAll` per sensitive value per field, preserving the existing longest-first prefix ordering
(`TestSensitiveInputPrefixOverlappingValuesRedactLongestFirst` still passes unmodified in intent,
call-site signature only). `application_attestation.go`'s "read the executable three times" item
does not apply to the current file: it has exactly two content reads, at prepare-time (stage and
digest) and inside `unchanged()` (later drift re-check), which are semantically required to happen
at different times and cannot be merged without breaking drift detection; left untouched. The "JS
emits paths only" companion item was skipped per the batch brief, since it is not a pure removal.
V1-0058: `internal/extevidence/mcp.go` dropped the two `mcpObject(...)`-then-`strictMCP(...)`
redundant pairs whose `mcpObject` result was already discarded (`mcpResponse`'s envelope decode,
and the initial handshake's `info` decode), since `strictMCP` alone already re-derives the same
duplicate-key and unknown-field checks; content/isError decoding, which uses its `mcpObject`
return value, is unchanged. V1-0057: `candidateAdjacency` (`internal/workqueue/proposal.go`) now
calls a new allocation-free `overlaps` helper instead of `len(intersection(...)) == 0`, dropping
the per-pair slice allocation and sort; `TestCandidateAdjacencyGroupOverlap` pins the adjacency
edges. All six changes keep existing test suites green; behaviorfalsify, jstestprovider,
extevidence and workqueue package tests all pass. UNKNOWN: whether the PWP-V0 `config-inputs-
unobserved` naming mismatch found while doing V1-0046 needs its own ticket, versus being purely a
documentation clarification — left as a note rather than filed separately.
## 2026-09-22 claude-code-compaction-pin-hooks: PreCompact/PostCompact pin verdict for V1-0094

Decision 0340 registers `PreCompact` and `PostCompact` on the Claude Code plugin (AHI-026 to
AHI-030). The hook names, payload fields and stdout routing were read from the installed Claude
Code 2.1.267 hook runner; no further compaction event exists there to register. `pre-compact`
prints a `corvint-compaction-pin/0` line (HEAD tree, dirty-path counts, at most 24 tracked dirty
paths) that the host joins into the compactor's instructions; `post-compact` re-validates the pin
the summary preserved, checks the tree and each path with one hermetic read-only `cat-file`, and
prints a `corvint-compaction-report/0` line naming every non-rehydratable path. The host shows that
report to the user only, so the model-facing rehydration remains `SessionStart(source=compact)`,
which now opens with a disclosure saying so; a host without the events ignores the registration
and that disclosure plus `compatibility.json` `compactionHooks` keep the gap visible. Evidence:
`TestAHI026`..`TestAHI029` in `cmd/corvint/host_adapter_compaction_test.go`, the AHI-030
assertion in `TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths`, and the extended
`TestAHI017AdapterHostKillMatchesDeclaredHooks`. Live compaction cycle NOT_RUN; black-box status
STATIC_ONLY; no frozen evaluation fits (the CEP §3 gate is an unrun 30-task three-cycle trial).
The plugin version stays 0.2.2 because `integrations/host-adapters.test.mjs` binds it to the
shared compatibility matrix this change does not own.
## 2026-09-22 opencode-mcp-verify: V1-0022 verified at HEAD with real OpenCode sessions

The ticket's repair, the explicit `--protocol-version 2025-11-25` profile (`MCPV0-021..023`), was
already on the public lineage before this batch; this entry records the verification of the
current tree against freshly built `corvint-mcp`, `corvint-docs-mcp` and
`corvint-test-validity-mcp`. Replaying the captured OpenCode `initialize` frame
(`protocolVersion` `2025-11-25`, `capabilities.roots` `{}`, `clientInfo` opencode) without the
selector still returns `{"code":-32601,"message":"Method not found"}`, the frame the owner report
reduced to; with the selector it returns `protocolVersion` `2025-11-25`, `serverInfo` and the
tools capability, then `tools/list` succeeds. Two installed clients, `/opt/homebrew/bin/opencode`
reporting 1.18.31 and `~/.opencode/bin/opencode` reporting 1.17.18, each ran an isolated
`opencode mcp list` (three servers `connected`) and a non-interactive `opencode run` session
against a deterministic loopback provider under a native outbound-network sandbox. Each session
completed `corvint.status`, `corvint.query`, `corvint.impact`, `corvint.test_validity`
(`discover: true`, evidence absent, `UNSUPPORTED`) and `corvint.docs_draft`; every repository
receipt pinned the fixture commit. Both clients send `notifications/cancelled` for every
`tools/call` after its response; the server ignores them, now pinned by
`TestMCPV0022LegacyCancelledAfterCompletionIsIgnored`. A first docs call refused a fixture whose
owner Markdown lacked an Agent digest (`unsupported-documentation-source`), a visible tool
refusal, not a transport failure. The provider was a transport fixture, not model evidence; the
owner's original failing machine and configuration remain UNKNOWN, and no FULL host authority is
claimed. Raw frames and receipts are retained under the session scratchpad `opencode-v1-0022/`.
## 2026-09-22 gate-affected floor and dogfood rg dependency: V1-0059, V1-0081, V1-0038, V1-0031

V1-0059: `link()` in `tools/gate-affected-select/readers.go` wired a string literal naming a
package directory as an importer edge whether or not the literal sat in a `_test.go` file. A test
file is never imported, so its holder's importers can never legitimately be reached through it.
`link()` now sources that componentRuns edge from a new `nonTestHolders` index (test-only literals
excluded) while the special root-module literal `"/"` case keeps using every holder, since
`componentRuns("/")` has no fallback and narrowing it silently drops the holder rather than just
its closure. New fixture `TestSelectPackagesStopsClosureAtTestOnlyTokenHolder` in `main_test.go`
pins the behavior. Measured at commit `3f30a02`: global `-unresolved` floor 130 → 125 packages (net
5 fewer: `cmd/corvint-analyzer-python`, `conformance/frontier-v0`, `internal/dogfoodocm`,
`internal/frontiernextrepo`, `benchmarks/selfuse-batch`, all previously unresolved only via a
test-file literal falsely propagating `cmd/corvint`'s unresolved status to its dependents).

V1-0081: with the V1-0059 fix applied, a one-package dirty-path selection
(`cmd/corvint/go_only_cutover_test.go`) narrowed from 143 to 139 of 218 total packages (65.6% →
63.8%), still far short of "well under half." Root cause: of the 125 packages left in the
`-unresolved` floor, 96 (44% of the whole module) self-locate directly — they call `os.Getwd` /
`runtime.Caller` or carry an escaping literal in their own non-test source — and are correctly
fail-closed under rule (d); only 29 are propagated through the importer/dependents graph, the only
part lever (3) can touch from inside `tools/gate-affected-select`. Lever (1) (a shared bounded
root-location helper) is out of ownership. Lever (2) (drop `-p 1` in `script/gate-affected.sh` when
the union is wide) was considered and rejected: `AGENTS.md` documents `cmd/corvint` panicking under
concurrent load at its current ~591s serial runtime, and `cmd/corvint` is selected in nearly every
plan, so removing serial package execution risks reintroducing that instability for a speed gain
that does not move the selection-count AC. Disposition: PARTIAL — lever (3) applied and measured
(130→125 unresolved, 143→139 one-package selection), AC unreachable within ownership because 96/218
packages self-locate directly in source outside `tools/gate-affected-select`.

V1-0038: `cmd/corvint/dogfood_record.go`'s `os.Getwd()` (line 40) is not the only reason `cmd/corvint`
is unresolved. `grep -rln 'os\.Getwd\|runtime\.Caller' cmd/corvint/*.go | grep -v _test.go` lists 16
files with real, independent calls (`dogfood_record.go`, `frontier.go`, `host_adapter.go`,
`local_completion.go`, `pi_adapter.go`, `eval.go`, `init_adopt.go`, `pi_tools.go`, `lrf.go`,
`migrate_traces.go`, `main.go`, `ocm.go`, `work.go`, `record.go`, `source_handoff.go`, `witness.go`).
Bounding only `dogfood_record.go`'s read cannot make `cmd/corvint` leave `-unresolved`; the other 15
files keep the package fail-closed regardless. Swapping `os.Getwd()` for the lexically-unflagged
`filepath.Abs("")` (the pattern `resolveExplicitRoot`/`normalizeRoot` already use in `main.go`) was
rejected as gaming the detector rather than genuinely bounding the read. Disposition: NOT DONE; the
stated AC needs a coordinated pass over all 16 files, out of this ticket's single-file scope.

V1-0031: `script/dogfood-check.sh`, `script/dogfood-change.sh`, `script/dogfood-bind-range_test.sh`,
and `script/dogfood-change_test.sh` (89 call sites total, not only the two files the ticket named)
call `rg` with no preflight; a host without it got a bare "command not found" partway through a run.
Each of the four now fails closed immediately after its `set` line with `REFUSE
unsupported-environment-missing-rg` when `rg` is absent from `PATH`, verified by running all four
with `rg` stripped from `PATH` (each refuses at exit 1 before any Git or build work starts) and
unchanged (`dogfood-bind-range_test.sh`, `dogfood-change_test.sh` both still exit 0) with `rg`
present. `docs/DOGFOOD.md` §4 now names `rg` as a prerequisite for `dogfood-change`/`dogfood-check`.
Disposition: DONE.

Gates run: `gofmt -l tools/gate-affected-select cmd/corvint/dogfood_record.go` (clean); `go vet
./tools/gate-affected-select/... ./cmd/corvint/...` (clean); `go test -count=1
./tools/gate-affected-select/...` (pass, includes the new fixture); `bash -n` on all four edited
scripts (clean); `shellcheck -S warning` on all four (clean); `script/dogfood-bind-range_test.sh`
and `script/dogfood-change_test.sh` full runs (both exit 0); `make spec-requirements-check
requirement-definitions-check traceability-tests-check decision-numbers-check line-citations-check`
(pass, no published-contract change). No `cmd/corvint/*.go` file was edited, so its own suite was
not rerun.
## 2026-09-22 flaky-batch-b: detached Git auto maintenance and ctime-tick witnesses

Five load-dependent `cmd/corvint` failures (V1-0032, V1-0034, V1-0035, V1-0061, V1-0071) share
one confirmed mechanism: on this host (git 2.54.0, no global config) every `git commit` spawns
`git maintenance run --auto --quiet --detach`, a grandchild that outlives the commit, holds
`.git/objects/maintenance.lock`, and can write `.tmp-<pid>-pack-*` files. Its writes race the
fixture byte digests, the materialization manifests, `t.TempDir` removal (`.git: directory not
empty`) and the work adapter runner's process-residue and 250ms pipe-drain checks. Setting
`maintenance.auto=false` and `gc.auto=0` removes the spawn entirely (GIT_TRACE=1: zero
maintenance processes across repeated commits). The specific pack write that V1-0061 recorded
was not reproduced here; that it comes from the same detached process is inferred, not observed.
Fixed in-scope: the affected fixture, the materialization fixture and the two committing
final-check scripts now disable both settings, and the runner keeps its last failure cause and
receipt in memory so the fatal message can name them (the command result and wire are unchanged).
Integration follow-up: `queryCLIRepository` in `cmd/corvint/query_test.go` now also disables
both settings (the same two `git config` lines as the affected fixture), which removes the
detached-maintenance cause behind V1-0061 and V1-0071; the drift restoration fatal now prints the
differing paths. `workDrainTimeout` (250ms) stays as the WQO-V0-034 pin. Repetitions: the five
named tests `-count=3` pass, the wider work/affected/query set `-count=1` passes.

Three ctime witnesses (V1-0033: `internal/trace`, `internal/authoritystore`,
`internal/cem/gitauth`) assumed the change time advances between adjacent syscalls; on a coarse
ctime clock the same-bytes restore lands in the tick of the original write. Each test now
re-applies its mutation until ctime differs, bounded at 2s, and skips otherwise. Product
limitation recorded: a restore that completes within one ctime tick is invisible to the witness.
`TestRunningFailedPassed` (V1-0036) waited 5s for each running event under whole-suite load; the
waits are now the 5-minute hang detector already used for completion (decision 0082). All four
tests pass `-count=5` with no skips on this host.
### 2026-09-22 batch D: contextindex/doccorpus fixes and decision 0337

Six V1 tickets landed in one change. `internal/contextindex`: V1-0049 rewords `citedNumbers`'s doc
comment to name its two malformed-range degradations explicitly and adds a table test pinning
`path:5-0`/`path:9-3`; V1-0053 hoists the two `regexp.MustCompile` calls in `eval_query.go` to
package vars; V1-0054 replaces `impact.go`'s per-changed-path linear scans over `index.Sources` with
a directory-to-paths index and a sorted path slice built once per call, output verified byte-
identical against the prior implementation via the package's existing tests; V1-0055 caches
`authority_trigger.go`'s per-target line splits, builds `range_impact.go`'s map before its linear
scan instead of after, and merges the two-spawn `cat-file -t`/`rev-parse base^{tree}` open of
`compileRangeImpact` into one `git rev-parse base^{commit} base^{tree}` call (not `--verify`, which
this Git version refuses with more than one revision argument) while leaving `verifyRangeBase`'s own
closing repeat of the same two-spawn pattern untouched, per the ticket. `TestAnalyzerSchemaInputs`'s
structural digest pin moved `e9058d1...` to `bca73e3...` (schema ID `corvint-analyzer/73` unchanged)
to reflect these edits.

`internal/doccorpus` (decision 0337, DCP-V1-032 amended): V1-0044 — `reconcileTest` (forward
emission) already retains a criterion or claim naming a variation absent from the normative set and
reports it as `undocumented-tested-behavior` (DCP-V1-031) rather than pruning it, but
`validatePreviousBehaviorAdapterResult` (the `--previous` strict check) fatally refused that same
retained shape, so a `run1.json` carrying exactly the finding DCP-V1-031 exists to report could not
be reused for the DCP-V1-032 delta. The validator now tolerates it on the same terms forward emission
does — the dangling reference must still be a criterion the test itself declares — and a new
`TestBehaviorAdapterPreviousToleratesDanglingCriterion` end-to-end test exercises run 1 (dangling
criterion, exits 0 with the `undocumented-tested-behavior` finding) then run 2 (`--previous` run1,
exits 0). V1-0050 — `lost_reverse_links` entries were joined with a literal NUL, which `textOK`
forbids in every other corpus text field; the six join sites now use a new `reverseLinkJoin` helper
(printable `|` separator, backslash-escaped where a field contains `|` or `\`, preserving the same
collision-freedom the NUL separator gave), and `validatePreviousBehaviorAdapterResult` now runs
`textOK` over every `Delta.LostReverseLinks` entry on read.

Owner question V1-0060 (should a prior bundle with dangling criteria disqualify a delta outright
instead of being tolerated) remains open; this batch implements the tolerant reading pending that
answer and does not close it.

Gates: `gofmt -l`, `GOTOOLCHAIN=local go build ./... && go vet ./...`, targeted
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/contextindex/... ./internal/doccorpus/...
./internal/specindex/`, and `make spec-requirements-check requirement-definitions-check
traceability-tests-check decision-numbers-check line-citations-check` all passed. Full `make gate`
was not run, per batch scope.

## 2026-09-22 patch-coverage-witness: `cem cover` records a per-hunk coverage witness from one local coverprofile and `cem report` downgrades unwitnessed test claims

Ticket V1-0085, decision 0347
(`docs/decisions/0347-patch-coverage-witness-from-a-local-coverprofile-2026-09-22.md`),
`TCQ-V0-051..054` in `docs/specs/test-claim-qualification-v0.md`. The TCQ YAGNI paragraph's
"coverage ingestion" exclusion is replaced by the four requirements, the acceptance matrix gains a
`coverage witness` row, and the rollback paragraph names how the slice comes out.

What changed: `internal/cem/wire/map.go` admits one optional closed-key hunk member `coverage`
(`{profileSha256, testRun, mode, state, covered}`) on `cem/0.3` only, with ranges required to be
ascending, non-adjacent, inside `newRange`, and consistent with `state`; `cem/0.1` and `cem/0.2`
keep rejecting it as `unknown-field`, so no fixture, conformance vector, `protocol/cem-0.2`
schema, or `interop/cem01-go` consumer changed. `internal/cem/workflow/cover.go` adds
`Session.Cover` and `internal/cem/cli/cli.go` the `cem cover --map --coverprofile --test-run
[--output]` action: one operator-named local profile, bounded at the gorunner coverage bound,
parsed by the gorunner parser now exported as `ParseCoverProfile`/`ParseCoverageBlockLine` (no
behaviour change to live verify), intersected with each hunk's added lines (diff-cover semantics),
and written on every hunk as `covered` or `uncovered`; the map is upgraded to `cem/0.3` as a
structural `mark` does. `internal/cem/workflow/read.go` adds a `## Test claims` section to
`cem report` listing each `test-claim` hunk as `tested` or downgraded with reason
`no-coverage-witness` / `coverage-witness-uncovered`; dispositions, counts, worklist and the
`status`/`verify` envelopes are unchanged.

Measured: four new tests (`TestSpec03CoverageWitness`, `TestParseCoverProfileExportsBlocks`,
`TestCoverRecordsCoverageWitnessAndReportDowngrades`, `TestCoverRefusesAmbiguousAndInvalidInputs`)
pass; package runs `internal/cem/wire` 1.7s, `internal/cem/workflow` 87.3s,
`internal/liveverify/gorunner` 24.0s, `internal/specindex` 0.9s, all `ok` at `-count=1`.

NOT MET / UNKNOWN: `corvint cem cover --help` and the cem help text do not list `cover` because
`cmd/corvint/help.go` (`cemHelpActions`) was outside this change's ownership; the command
dispatches. `docs/specs/cem-0.3-structural-mechanical.md` still describes the profile as adding
only the structural reasons and was not amended (not owned). No live `go test -coverprofile`
end-to-end run against a real repository was performed; the workflow test uses a synthetic
profile whose path suffix matches the hunk path. Reporter and labelled-corpus promotion gates of
the TCQ spec remain NOT_RUN.

Gates: `gofmt -l` (nothing), `GOTOOLCHAIN=local go build ./... && go vet ./...`, targeted
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/cem/... ./internal/liveverify/gorunner/
./internal/specindex/`, and `make spec-requirements-check requirement-definitions-check
traceability-tests-check decision-numbers-check` passed. `make line-citations-check` fails on 18
pre-existing citations in `docs/specs/falsifiable-packet-v0.md`, `docs/decisions/0082-*.md` and
`docs/specs/go-production-kernel-migration-v0.md` that cite `internal/contextindex` and
`cmd/corvint` lines this change does not touch; they were not repinned because those files are
outside this change's ownership. Full `make gate` was not run, per ticket scope.
## 2026-09-22 packet-trust-class: one `trust` class per cited row; tainted rows satisfy no basis

Ticket V1-0090, decision 0346 (proposed, experimental delivery), TCP-V0-023 and FPK-V0-032.

What changed: `internal/contextindex/trust.go` holds the closed five-class enum
(`project-authority`, `repository-content`, `repository-history`, `external-provider`,
`tool-output`), the one derivation table `trustByAuthority` keyed on the existing `authority`
label (an unlisted label is `tool-output`), and `TrustTainted`. `context` stamps `trust` on every
`results[].evidence[]` row, computes `governance` and `critical` over the non-tainted reserved rows
only, and adds the always-present `coverage.governance_refused` array naming each refused row.
`prove` (`cmd/corvint/prove_trust.go`) stamps `trust` on every `proof.rows[]` entry through the
same table; a tainted row keeps falsifier `none` (never `PASS`, never `proven_results`) and carries
a `refusal` naming the row. No new input is read; the `query`/`impact` wires, the `external`
section and the CEM ledger readers are untouched.

Measured: the recipe golden `internal/contextindex/testdata/context-recipe-default-golden.json`
re-captured with exactly 12 added `"trust"` members (11 `repository-content`, 1
`project-authority`) and one added `"governance_refused": []`; no other byte changed. The
`TestAnalyzerSchemaInputs` audit digest was repinned (consumer-only change, schema stays
`corvint-analyzer/73`, as the two prior repins did). Every row of the `prove` fixture proof is
untainted and unrefused (`TestProveRowsCarryOneTrustClassAndOldConsumersDecode`). Old-consumer
decoding covered for both wires by decoding the previous struct shapes and comparing canonical
JSON with the new members deleted.

NOT MET / follow-ups (text only, no tickets filed): the `query`/`impact` evidence rows carry no
`trust` (byte-exact under GPK-V0-002 and `conformance/cli-parity-v0`; needs its own amendment);
`internal/extevidence` external-section rows are not stamped (outside this change's ownership);
`prove checkpoint` claimed-authority rows are not classified; no external consumer has exercised
the new members.

Gates: `gofmt -l` (nothing), `GOTOOLCHAIN=local go build ./... && go vet ./...`,
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/contextindex/... ./internal/specindex/`
(ok, 134.5s and 0.2s), `cmd/corvint -run` over the two new prove tests plus the twenty context-,
answerability- and prove-row tests the wire change touches (ok), and `make spec-requirements-check
requirement-definitions-check traceability-tests-check decision-numbers-check` all passed.
`make line-citations-check` FAILS on 18 citations (decision 0082, `falsifiable-packet-v0.md`
rows citing `internal/contextindex/impact.go`, and `go-production-kernel-migration-v0.md` citing
`range_impact.go`); the same 18 fail with the index read from base 4519cad and none names a file
this change touched, so they are pre-existing from the batch D commit and were not repinned here.
Full `make gate` was not run, per ticket scope.

## 2026-09-22 gate-repair: DR-0039/DR-0040 `cem` choice-list extensions declared and the CEM trust citation repinned

The full `make gate` on the V1-0085 tree (base 1603d4a) reported two failures; repairing the first
exposed a third of the same shape. None touched a frozen expectation.

- `conformance/cli-parity-v0` `TestGPKV0002ManifestReplay`: `cem-mark-invalid-reason` failed
  `stderrSha256` (candidate `531926b2…`, manifest `b03c3c67…`) because decision 0338 added the four
  `cem/0.3` structural reasons to the `--reason` choice set (`CEM-SM-001`). Once declared, the next
  case `cem-invalid-subcommand` failed the same way (candidate `bab3d5ea…`, manifest `2a7db57b…`)
  because decision 0347 added `cem cover` to the action list (`TCQ-V0-051`). Both are recorded as
  intentional Go extensions in `conformance/divergence-register.md` (`DR-0039`, `DR-0040`) with the
  exact candidate and oracle bytes, declared as one-rewrite `stderrRewrites` `knownDivergence`
  entries in `manifest.json`, and pinned by `validMarkReasonDivergence` and
  `validCEMActionDivergence` in `manifest.go`. Measured: applying each single substitution to the
  candidate stderr reproduces the frozen digest byte-for-byte; the `SUMMARY` moves from
  `known-divergences=24` to `known-divergences=26`; the `cem` inventory row reports 19 byte-exact
  cases and names both declarations. The `flag provided but not defined: -candidate` text in the
  package output is emitted by the passing `TestCaptureCLIRejectsCandidateAuthority`, which expects
  that refusal; it is not a failure.
- `conformance/release-artifact-v0` `TestReleaseNotesCEMTrustCitationLandsOnTrustRoots`:
  `docs/RELEASE-NOTES-alpha.md` cited `docs/CHANGE-EVIDENCE-MAP.md:226-227@012d2dcc`, which
  commits 5ab91e3 and 1603d4a moved to lines 241-242. Repinned to `241-242`; the `@012d2dcc` anchor
  is unchanged because `script/check-line-citations.sh --hash docs/CHANGE-EVIDENCE-MAP.md:241-242`
  reproduces it. No other document cites `docs/CHANGE-EVIDENCE-MAP.md` by line. The two declarations
  shifted lines in `manifest.json` and `runner_test.go`, so `docs/decisions/0051-*.md:24` and
  `docs/specs/compat-replay-runner-v0.md:17` were repinned to `manifest.json:3552@f1a2ef9a` and
  `runner_test.go:1216@6ead4d11` (anchors unchanged). `make line-citations-check` still reports the
  18 pre-existing failures in `docs/specs/falsifiable-packet-v0.md`, `docs/decisions/0082-*.md` and
  `docs/specs/go-production-kernel-migration-v0.md` that this change does not touch.

Not done in this change: `conformance/cli-parity-v0/README.md`'s known-divergence list (outside the
repair's ownership) does not yet carry `DR-0039`/`DR-0040` bullets. UNKNOWN: whether the rest of the
full `make gate` is green after this repair; only the two named packages were rerun.
## 2026-09-22 learned-rule-skill-export: `corvint skill-export --out DIR` projects admitted traces to Agent Skills documents

Ticket V1-0095, decision 0349, requirements `LTA-V0-006` to `LTA-V0-008` in
`docs/specs/learned-trace-admission-v0.md`. New package `internal/skillexport` renders one
`corvint-learned-<16 trace-id hex>/` directory per admitted rule (a stored row with outcome
`passed` that `tracerecordrepo.Read` re-validated): `SKILL.md` with YAML frontmatter (`name`,
one-line `description` folded from the task and bounded at 200 runes) and a short body, and
`references/trace.md` with the opened and changed paths and verification commands (progressive
disclosure). Each document names the admission evidence digest, `sha256:` over the exact
`trace.Encode` row bytes, and the evaluation result verbatim as `NOT_RECORDED`, because
`LTA-V0-001` admits the learned-path mechanism and never an individual row, so no per-row
evaluation result exists to cite. `failed` and `blocked` rows and rows whose digest does not
re-validate are refused. New verb `cmd/corvint/skill_export.go` reads through the ordinary
snapshot and trace reader, accepts only `--out DIR`, refuses a `DIR` inside `.corvint` (after
symlink resolution of the nearest existing ancestor), writes only under `DIR`, and prints a JSON
manifest; `main.go` gained one dispatch and `help.go` one topic, which shifted twelve unchanged
citations in four specs by three or five lines (repinned; every content hash unchanged).

Measured: `internal/skillexport` 3 tests and `cmd/corvint` 2 new tests pass; the four existing
verb-registration tests pass; in the fixture (`calibrateRepository(t, 3)`, one `passed` row) the
export writes 1 directory, the repository tree digest is unchanged, and a second run into another
`DIR` yields identical file bytes and a manifest identical apart from `DIR`.

UNKNOWN / NOT MET: no real Claude Code or Codex host loaded an exported skill; the round-trip
fixture `TestHostRoundTripLoadsExportedSkill_LTA008` is a Go parser shaped like a loader's
frontmatter read and the published Agent Skills bounds (name 1..64 `[a-z0-9-]`, description
1..1024). The evaluation result is `NOT_RECORDED` for every row until a per-row evaluation
linkage exists. The worktree was fast-forwarded from 362721c to the wave base 4519cad before
work started.

Gates: `gofmt -l`, `GOTOOLCHAIN=local go build ./... && go vet ./...`, targeted
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/skillexport/... ./internal/specindex/`
plus `cmd/corvint -run` over the six touched tests, and `make spec-requirements-check
requirement-definitions-check traceability-tests-check decision-numbers-check` all passed;
`make line-citations-check` reports the same 18 pre-existing failures as the base, none added.
Full `make gate` was not run, per wave scope.
## 2026-09-22 generated-evidence-kind: `generated` joins the external evidence kinds (V1-0107, decision 0350)

`generated` is now the fourth admitted evidence kind in `internal/extevidence` (`EEP-V0-007`,
`EEP-V0-019`): `impact` composes it like the other kinds with the kind visible in
`relation.evidence` and the item `reason`, and test selection lists it as a candidate coded
`generated-only-evidence` that never qualifies or blocks (`ETS-V0-014`, `weakEvidence` lookup in
`selection.go`). `learned` stays excluded. The decode path is unchanged: `evidence` was already an
identifier and unknown kinds were excluded at composition, so no existing record decodes
differently.

Measured: `internal/extevidence` passes in 62s; `TestSelectionEvaluation` over the labelled corpus
(now 31 selection cases, 71 evaluated variants) reports precision 1.000 (54/54), unsafe narrowing
0/53, abstention accuracy 6/6, receipt max 7032 bytes. New evidence: `TestEvidenceKindGeneratedAdmitted`
and the `positive-observed-with-generated-candidate` case over
`testdata/conformance-selection/generated.json`.

NOT MET / UNKNOWN: no external consumer has exercised the marker; an independent adopter record is
still the EEP-V0 promotion criterion. `conformance/` holds no external-evidence suite, so the pinned
fixture lives only under `internal/extevidence/testdata`.

## 2026-09-22 provider-capability-declaration: optional `capabilities` record member checked by Core (V1-0102, decision 0351)

Changed: `internal/extevidence` records (`Record`, `Record1`) accept one optional top-level member
`capabilities` with lists `schemas` and `evidence_kinds`, validated like the rest of the record
(`EEP-TR-012`). `decodeRecord` checks a declared list before composition: the record schema must be
declared; under `impact` every used evidence kind must be declared; under `affected` `declared` or
`observed` must be declared. A shortfall is the new closed state `unsupported` with a Core-authored
reason naming the provider id and the first missing capability; `affected` then blocks with a
`provider-unsupported` blocking reason (`EEP-TR-013`). A declaration never widens acceptance
(`EEP-TR-014`). The shipped handshake is the record member because the only shipped transports are
file and command; MCP stays an unshipped profile (`EEP-TR-009`), so no transport-level negotiation
was added. Absent means undeclared: every existing fixture and conformance corpus passes unchanged.

Measured: `internal/extevidence` passes in 91.6s; `TestSelectionEvaluation` over the labelled corpus
(now 33 selection cases, 73 evaluated variants) reports precision 1.000 (55/55), unsafe narrowing
0/54, abstention accuracy 7/7, receipt max 7032 bytes. New evidence: `TestCapabilitiesNegotiation`
(absent, sufficient, kinds-undeclared, missing-kind, missing-schema, empty-schemas),
`TestCapabilitiesDecodeStrict`, and the `positive-capabilities-sufficient` and
`negative-capabilities-unsupported` cases over `testdata/conformance-selection/capabilities.json`.

NOT MET / UNKNOWN: no transport-level handshake exists because no shipped transport can carry one;
no external provider has declared the member; `conformance/` holds no external-evidence suite, so the
fixture lives only under `internal/extevidence/testdata`.
## 2026-09-22 cem-intoto-predicate-v1: a versioned `cem/v1` in-toto predicate binds the CEM to its base and patch

Ticket V1-0092, decision 0354, FPK-V0-033 to FPK-V0-036 (experimental prototype, not advertised).
`internal/attest.CEMStatementV1` builds an in-toto Statement v1 with `predicateType`
`https://corvint-context.dev/attestation/cem/v1`, whose predicate adds `base.digest.gitCommit` and
`patch.digest.sha256` beside the `cem` ResourceDescriptor, `size`, and `spec`.
`VerifyCEMPredicate` dispatches `cem/0` and `cem/v1` through a closed table, and
`prove --verify-cem-attestation` now calls it. `cem/0` receipts carry no new member. A
standard-library reader in `interop/cem01-go` verifies the fixture envelope Corvint emits.

Measured: the fixture envelope for `interop/cem-0.1/maps/valid/supported-sha256.json` (863 bytes)
under the seed-derived test key has sha256
`283792cd974edb5112edfe9e23df7f4b155148310850c1001ae6c9cd9c976b38`, pinned in both modules.
`TestCEMV1RefusesAClaimItCouldNotHaveProduced` refuses 13 signed edits, none as a byte mismatch.
`go.mod`, `go.sum`, and `interop/cem01-go/go.mod` are unchanged. The new files import no `net/*`,
`os/exec`, or `crypto/tls` package.

UNKNOWN / NOT MET:
- Every OpenSSF openfab/generation draft (ossf/tac issue 628) and agentattest field name is
  UNCONFIRMED, because the work ran without network access and the repository holds no copy of
  either. The spec records them as deviations. Only the in-toto Statement v1, ResourceDescriptor,
  DigestSet, and DSSE names are claimed as aligned, and those were recalled, not re-fetched.
- No CLI flag emits `cem/v1`. The emission flags live in `cmd/corvint/prove.go`, outside this
  change's ownership.
- The interop reader was written by the same author after reading `internal/attest`, so it is not
  independent-adopter evidence (V1-0014 unchanged).
- The Sigstore gitsign/cosign/Rekor path is documented as an operator step and was NOT_RUN.
- Commands ran without the brief's `nice -n 10` prefix, and the interop gate ran as
  `go -C interop/cem01-go`, because the worktree-isolation hook refuses compound commands.
## 2026-09-22 git-native-cem-anchoring: `cem anchor` notes-ref pointer and read-only `cem provenance` (V1-0093)

Changed (decision 0355, FPK-V0-037 to FPK-V0-040, all experimental):
- `cem anchor --map MAP [--commit REV]` is an explicit mutation (`mutates: true`). It writes a
  `corvint-cem-anchor/0` JSON pointer (commit, path, blob, SHA-256, spec) for the map committed
  at HEAD to `refs/notes/corvint` on REV, under a fixed committer identity.
- It refuses an untracked, absent, staged, or modified map, a blob missing from the object
  database, and a different existing note. Every refusal leaves the notes ref unmoved.
- `cem provenance --commit REV` is read-only. It verifies the anchor by digest, and reads a
  foreign Git AI `authorship/3.0.0` note on `refs/notes/ai` and the `Assisted-by` and
  `Agent-Logs-Url` trailers.
- Every row it emits carries `trust: repository-history` and `authority: git-history`, with a
  distinct `kind` per source. Foreign text sits only in a bounded `untrusted` member.
- The trust enum, `trust.go`, and `prove_trust.go` are untouched.
- `internal/gitnotes` is reached from `internal/cem/cli` only through the `cemcli.GitNotes` hook
  set in `cmd/corvint`.
- Two FRONTIER brief citations into `cmd/corvint/help.go` were repinned (785-792, 880). Their
  content is unchanged.

Measured:
- `internal/gitnotes`: 4 tests pass (7.0s), including 6 refusal subtests.
- `internal/cem/cli` and `internal/specindex` pass.
- `cmd/corvint -run` over `TestCEMAnchorAndProvenanceInteropThroughTheCLI`, `TestCEMHelpSurfaces`,
  and `TestCEMErrorPrecedence` passes.
- The `go list -deps` closure of `internal/gitnotes` holds no `net` package.
- Bounds: 256 bytes per foreign string, 64 entries per list, 1 MiB per note, 4 MiB per map.

NOT MET / UNKNOWN:
- `TestCEMSeamsDependOnlyOnStdlibAndGit` fails on `internal/liveverify/gorunner`. The import is
  in `internal/cem/workflow/cover.go` (a43c652, V1-0085), which this change leaves untouched, and
  `cli.go` gains no import. So the failure is inferred to be pre-existing at d2aa0c8; it was not
  rerun at base.
- The fixtures are Go tests in `internal/gitnotes` and `cmd/corvint`, not `interop/cem01-go`.
  That module is an independent Apache-2.0 CEM 0.1 verifier and the pointer is not CEM wire.
- The Git AI format was checked against its published v3.0.0 spec only. No note produced by the
  real Git AI tool was read.
- No provenance row feeds `query`, `prove`, ranking, or authority.
- Notes are not pushed or fetched.
- The root `--help` mutation-boundary paragraph does not yet name `cem anchor`; it was outside
  this change's ownership.
- `nice -n 10` could not be used: the worktree guard refused it, so tests ran un-niced.
- Full `make gate` was not run, per ticket scope.

## 2026-09-22 hunk-mutation-discriminates-witness: `cem discriminate` records a bounded mutation witness per hunk (V1-0086, decision 0353)

What changed: a new explicit action `corvint cem discriminate --map MAP --target REV
[--max-hunks N] [--max-mutants N] [--wall-time DURATION] [--output MAP]`
(`internal/cem/workflow/discriminate.go`, wired in `internal/cem/cli/cli.go` and `cem` help)
reuses the FPK-V0-028 runner (`internal/liveverify/mutate`: `Open`, `Export.Judge` with
`Complete`) on the map's changed Go hunks against the `_test.go` files their `test-claim` basis
cites, and writes the optional closed-key `cem/0.3` hunk member `discriminates`
(`internal/cem/wire/discriminate.go`) with `state` `discriminates`, `survived`, or `not-run`,
killed/survived counts, every survivor described, the bounds enforced, the resolved target
object ID, and the selection digest of the selected test paths. `cem report` appends the witness
to each `## Test claims` line and downgrades a `survived` hunk with reason `mutants-survived`,
listing each survivor; status, verify, counts, worklist, and exit status are unchanged. The only
runner change is additive: `Report.Survivors` (`[]Survivor{Operator, Line, Start, End}`) filled by
both judge paths; `prove --mutate` reads none of it. `cem` help also gained the `cover` action
that decision 0347 left out. Specs: TCQ-V0-055..058 with an acceptance row, rollback, and
traceability; one paragraph appended to the FPK requirements records the shared runner and
keeps FPK-V0-028's experimental label and 19-of-20 replay gate `NOT_RUN`.

Measured on this host (macOS `sandbox-exec`, workflow test fixture of one Go module, one
selected hunk, 4 mutants, `--max-mutants 6`, `--wall-time 5m`): one bounded run 5.7 s on a quiet
host and 30.5–32.8 s while seven other agents were building and testing concurrently; refusals run
no mutant and complete in under a second. The runner's cost is one sandboxed `go test` per mutant.

UNKNOWN / NOT MET: no end-to-end run against a real repository change was performed, only the
synthetic fixture; the cost numbers are from one host under two load conditions and are not a
budget. FPK-V0-028's replay cohort and the TCQ promotion gates remain `NOT_RUN`. The prototype
label was narrowed, not removed: the TCQ requirements, cost, and rollback now govern the shared
runner, while FPK-V0-028's own experimental label stays because its acceptance is unmet.
## 2026-09-22 self-dogfood-mapping-qualifies-store-scope: decision 0348, V1-0082

`workMappingReproduced` (`cmd/corvint/work.go`) now accepts Corvint's own `decision-0046-v0`
self-dogfood mapping (`docs/worklist.json`) alongside `repository-worklist-v0`, using the same
byte-reproduction argument decision 0321 established and explicitly deferred ("qualify both closed
mappings... can be proposed separately"). WQO-V0-017 and the WQO-V0-046 requirement body in
`docs/specs/work-queue-observation-v0.md` are amended to name `decision-0046-v0` as the same
exception, citing decision 0348; the section 5.7 header, its witness table row for WQO-V0-033, the
following paragraph, and the WQO-V0-046 traceability row are updated to match.

The WQO-V0-021/025/032 final-check fixtures (`cmd/corvint/work_final_check_test.go`) previously got
incomplete initial store scope only as a side effect of the mapping staying unqualified. The
`.git`-nested-FIFO marker (`workFinalIncompleteScope`) still produces that incompleteness, unchanged;
what changed is how `closing-inability` and `prior-mutation-and-closing-inability` now signal the
required mutation: each script prepends `git commit --allow-empty` (the same technique
`source-drift-and-mutation` already used) so the Git-directory monitored root's own independent
manifest scan records the change via its existing-entry content-diff branch, instead of relying on
the caller-tree root walk — which turned out to be permanently blind to new top-level files whenever
anything inside `.git` (sorted first, fully depth-first) aborts the walk. The untracked-file write
that dirties `git status` for the closing failure is unchanged. `TestWorkMappingReproducedSelfDogfood`
(`cmd/corvint/work_adopt_test.go`) directly proves `decision-0046-v0` now reproduces and that tampered
adapter output still disqualifies it. `TestObserveWorkUsesOnlyTargetMaterialization`
(`cmd/corvint/work_observe_test.go`) is updated from `StateUnknown`/`"UNKNOWN"` to
`StateValidated`/`"UNCHANGED_OBSERVED"` — its clean, undrifted production fixture now reaches complete
store scope, which was never its stated purpose (materialization isolation) but was an incidental
dependency on the mapping staying unqualified.

`script/gen-spec-requirements.sh` regenerated `docs/specs/REQUIREMENTS.tsv` with no field diff besides
line numbers (requirement IDs stable). The spec's Agent-digest `Claim:` line is unchanged, so no
README/INDEX sync was needed.

Gates: `gofmt -l cmd/corvint internal/workqueue internal/specindex docs` clean;
`GOTOOLCHAIN=local go build ./...` and full `go vet ./...` clean;
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/specindex/...` passed (0.42s);
targeted `-run '^(TestWorkFinalCheckCaptureBinding|TestWorkMappingReproduced|
TestWorkMappingReproducedSelfDogfood|TestObserveWorkUsesOnlyTargetMaterialization)$'
./cmd/corvint/...` passed at `-count=1` (57.9s) and was separately confirmed stable at `-count=3`
(479.8s, exit 0) earlier in the same session; `internal/workqueue/...` passed (2.3s). All 107
`Test...` functions found across `cmd/corvint/work*.go`, `internal/workqueue/*.go`, and
`internal/worklistadapter/*.go` were run and pass (one pre-existing, unrelated skip:
`TestWorkAdapterProcess`). `make spec-requirements-check requirement-definitions-check
traceability-tests-check decision-numbers-check` passed. `make line-citations-check` fails on 18
pre-existing citations in `docs/decisions/0082-*.md` and `docs/specs/falsifiable-packet-v0.md`/
`go-production-kernel-migration-v0.md`, all pointing at `internal/contextindex/impact.go`,
`internal/contextindex/range_impact.go`, and `cmd/corvint/work_materialization_test.go` — files last
changed by already-committed, already-merged commits (`4519cad`, `b8b1225`) outside this ticket's
ownership (`cmd/corvint/work*.go` and `internal/workqueue/**`, not `work_materialization_test.go`,
and not `internal/contextindex`); none of the 18 broken citations reference any file this change
touched. Full `make gate` was not run, per ticket scope.

## 2026-09-22 cem-seam-closure: `cem cover` and `cem discriminate` no longer pull runners into the stdlib-only CEM seams

Two integration regressions broke the native-cem-adapter claim ("CEM seams depend only on the Go
standard library and local Git", `TestCEMSeamsDependOnlyOnStdlibAndGit`): `go list -deps
./internal/cem/...` reached `internal/liveverify/gorunner` through `internal/cem/workflow/cover.go`
(TCQ-V0-051..054) and `internal/liveverify/mutate` through `internal/cem/workflow/discriminate.go`
(TCQ-V0-055..058). Fix shape: the Go coverprofile grammar (`Mode`, `Block`, `Parse`,
`ParseBlockLine`, `MaxBytes`) moved into the stdlib-only `internal/cem/coverprofile`, and gorunner
keeps its exported API by aliasing and thin wrappers. The mutation judge (`Open`, per-hunk judging,
survivor folding) moved to `internal/cemdiscriminate`, outside `internal/cem`, and reaches workflow
through the injected hook `workflow.OpenHunkJudge`, which `cmd/corvint/cem_discriminate.go`
installs in the style of `cemcli.GitNotes`. When the hook is not installed, discriminate treats
the runner as unavailable and marks every selected hunk `not-run`; it does not panic. Outputs,
refusals, and wire are unchanged. A third stale expectation from the same integration,
`internal/cem/cli/anchor_test.go`'s invalid-choice list without `discriminate`, was corrected.
Specs: TCQ-V0-051 and TCQ-V0-055..058 traceability rows and the TCQ-V0-051 parser prose name the
new surfaces; native-cem-adapter lists `coverprofile` and both binary-installed hooks.

Gates: gofmt clean; `go build ./...` and `go vet ./...`; the seam closure contains no non-stdlib
package outside `internal/cem` except `crypto/internal/entropy/v1.0.0`; `go test` over
`./internal/cem/...`, `./internal/cemdiscriminate/...`, gorunner, mutate, and specindex;
`TestCEMSeamsDependOnlyOnStdlibAndGit`, `TestCEMHelpSurfaces`, and `TestCEMErrorPrecedence`;
`interop/cem01-go` build; spec-requirements, requirement-definitions, traceability-tests, and
decision-numbers checks. The rest of `cmd/corvint` was not run. NOT MET: `line-citations-check`
fails on two citations in `docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md` (:245 and :257,
pointing at `cmd/corvint/help.go`). Those citations were already stale at the base commit, and this
change touches neither file. gorunner's `TestRunCollectsRealUnitCoverage` timed out at its 30 s
fixture bound once, at a 15-minute load average of 128, and passed on rerun.
## 2026-09-22 gate-ledger-per-package-bound: resolved test packages key on a proven per-package bound

Ticket V1-0037 (decision 0352, GL-V0-009). `ledger/go-test` used to hand the resolved packages to
Go's own test cache, which is per-`GOCACHE` and keyed on absolute paths, so a second worktree of the
same commit reran every one of them. Each resolved package now runs under its own ledger step
`go-test-package` whose key digests the worktree entries in a proven bound plus the gate tooling
(`Makefile`, `go.mod`, `go.sum`, `script/`, `tools/`), `GO_TEST_TIMEOUT`, and `GOFLAGS`. The bound is
the union of two readers the gate already trusts: the in-module files of the package's test closure
from one `go list -deps -test -json ./...` (`tools/gate-ledger/main.go:369@4dd19278`), and every
path `gate-affected-select -bounds` attributes to the package under the selector's rules (a)-(c)
(`tools/gate-affected-select/main.go:262@ba85708b`; the new `pathMatcher`,
`tools/gate-affected-select/readers.go:651@50d71fd4`, evaluates rule (c) once per literal token
over all paths, 16.7 s to 3.1 s on this tree, with output identical to the per-path `readers()`,
pinned by `TestPathMatcherAgreesWithNamesPath`). No second dependency walker was written. A package
whose bound cannot be proven (the selector attributes nothing to it, `go list` does not list it, or
`go list` names a file the worktree digest does not hold) runs in the same batch unrecorded
(`tools/gate-ledger/main.go:399@cbbc1553`); if the bounds cannot be computed at all the step prints
`BOUNDS unavailable: ...` and falls back to the pre-change Go-test-cache path. Each record carries
an additive `bound` field stating how the bound was proven; `gate-ledger/1` entries without it keep
matching. The resolved packages still run as one `go test` batch
(`tools/gate-ledger/main.go:538@fc23e453`), so a batch failure records none of them. Spec:
`docs/specs/gate-ledger-v0.md:99@20c09b98`; README/INDEX claim mirrored; REQUIREMENTS.tsv
regenerated; `docs/specs/go-archive-gate-v0.md` citation `Makefile:109` repinned to `:111` (same
anchor, moved by the ledger comment). The `Makefile` change is comment-only on the `ledger/` block.

Measured on this tree (219 test packages): 94 resolved packages received a proven per-package key
(none unprovable), 125 unresolved stay on the whole-tree key. Run A, throwaway `git worktree add`
of the WIP commit under the scratchpad, empty ledger dir, `GO_TEST_TIMEOUT=30m`, host shared with
other agents' test runs: 94 `RUN go-test-package ... no recorded pass` lines, one batch `go test`
over the 94 packages passed and recorded 94 records with `duration_ms` 230319 (230.3 s for the
batch; every record of a batch carries the batch time). Example record: `internal/projectprofile`,
key `552bcef9a88e...`, bound `go list -deps -test 1 files; selector frontier 1, reader 1697 paths;
1929 entries digested with the gate tooling` (rule (c) attributes 1697 literal-named paths to that
package, which is the price of never narrowing a step). The unresolved batch then ran and failed in
8 of 125 packages (`cmd/corvint`, `cmd/corvint-go-test-provider`, `conformance/cli-parity-v0`,
`conformance/release-artifact-v0`, `internal/analyzernativebridge`, `internal/behaviorfalsify`,
`internal/liveverify/session`, `internal/playwrightminimize`), so no `go-test-unresolved` record
was written; whole run 18:56 wall, 335% CPU. Seven of those failures are timeouts or event waits
under the shared load (V1-0032, V1-0034, V1-0036 describe the same shapes); the eighth,
`TestReleaseNotesCEMTrustCitationLandsOnTrustRoots` in `conformance/release-artifact-v0`, is a
`docs/CHANGE-EVIDENCE-MAP.md:226-227` wording check that fails in 0.2 s on this branch and touches
no file this change edits, so it predates the change. Run B, a second `git worktree add` of the
same commit, same ledger dir: process start to `PARTITION` 3.0 s (includes the `go run` build and
both bound readers), 94 `HIT go-test-package` lines by 4.96 s from start, zero
`RUN go-test-package`, e.g. `HIT go-test-package github.com/Beamfall/corvint/internal/projectprofile
552bcef9a88e (recorded 2026-09-23T00:22:26Z on Russells-Mac-Studio.local)`: the 230 s batch
became a 5 s check from another worktree. The unresolved batch then reran under its tree key
because run A's batch never recorded, and failed again in 5 of the same 8 packages (12:58 wall for
the whole run B). Both throwaway worktrees were removed afterwards. Timings are
from a loaded host and are upper bounds, not benchmarks.

NOT MET / UNKNOWN: `make dogfood-change` and the post-commit dogfood bind were not run (the batch
brief limited gates to the listed commands). The brief's `nice -n 10 env ... go test` form was
refused by the sandbox; the same test command ran without `nice`. `make line-citations-check`
reports 18 pre-existing failures in `docs/specs/falsifiable-packet-v0.md` and
`docs/specs/go-production-kernel-migration-v0.md` (stale `internal/contextindex/*` citations, all
present on the untouched base tree and outside this change's file ownership); the one citation this
change moved was repinned.

Gates: `gofmt -l` over every Go package directory printed nothing; `GOTOOLCHAIN=local go build
./... && go vet ./...`; `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/gate-ledger/...
./tools/gate-affected-select/... ./internal/specindex/` (55.7 s / 0.8 s / 0.7 s); and `make
spec-requirements-check requirement-definitions-check traceability-tests-check
decision-numbers-check` all passed. Full `make gate` was not run, per batch scope.

## 2026-09-24 opencode-terminal-notices: decision 0379, AHI-022

- The `invalid-arguments` FALLBACK notice reproduced live on OpenCode 1.18.31. OpenCode opens a
  directory outside Git with worktree `/`, and the real binary refuses that root. Decision 0378
  (upstream) fixed the plugin. With this adapter, a live `opencode serve` in a non-repository
  directory and in a repository both printed no `[corvint/opencode]` line.
- No test caught it because every OpenCode test ran the permissive `integrations/testfixture`.
  `TestHostAdapterJavaScriptHosts` now also builds `./cmd/corvint`. A new lifecycle test covers
  every stable hook and both tools against that build.
- The audit found a second terminal notice: `unsupported-impact-repository` on `file-change`,
  triggered by a `.go` edit in a repository without `go.mod`. It is now expected and goes to
  OpenCode's log.
- Mutation checks:
  - Removing either fix fails the new tests.
  - Removing the 0378 guard fails both the 0378 test and the new lifecycle test.
- Follow-ups, not changed here:
  - A repository with no commit fails every hook with `corvint-command-failed`.
  - `runtime.js` still sends `adapterVersion` 0.1.0.
  - The 2 s automatic ceiling can still produce a disclosed `timeout` notice under load.

## 2026-09-24 work-source-refusal-reason: WQO-V0-051, issue #157

- Issue #157: `work observe` returned `ERROR/SOURCE_UNQUALIFIED` with no reason. The reported
  `work init` failure did not reproduce on 0.8.0: init succeeds, and observe refuses until the
  three adoption files are committed. That refusal is intended (WQO-V0-047), but nothing said so.
- Every `SOURCE_UNQUALIFIED` from `work observe` or `work propose-wave` now writes one stderr line
  with a fixed reason and next step: uncommitted policy, worklist, or adapter; invalid committed
  policy; dirty or partially committed worktree; unqualified adapter binding or changed bound
  executable; policy change during the invocation; or other Git source facts. Reasons are fixed
  text and never echo file contents or Git output. The `work-command-result/0` stdout is
  byte-identical.
- A successful `work init` writes one stderr line saying to review and commit the three files.
  `corvint help work` says observe and propose-wave need them committed.
- `internal/worksource` exports `ErrWorktreeNotClean` and `ErrIndexDiffers` so the caller can name
  those refusals. Their messages are unchanged.
- Test: `TestWorkSourceUnqualifiedNamesReasonWQOV0051`. The demo on a fresh repository showed the
  missing-policy, dirty-worktree, and changed-executable reasons, and empty stderr once committed.

## 2026-09-24 Issue #156 EAF-V0-011: isolated Git status refusals name their cause

A 0.8.0 user reported `repository-probe-failed` / "Git status cannot safely observe repository
metadata" on a clean checkout that plain `git status` reads, and `corvint.status` returning
`repository-unavailable`. `internal/gitstatus` returned one undifferentiated `errUnsafe` from about
thirty sites, and both ordinary kernels replaced it with the fixed sentence, so the refused feature
could not be identified without access to the machine.

Each site now returns a reason that still satisfies `errors.Is(err, errUnsafe)`, and
`gitstatus.RefusalMessage` appends it: for example `index records a submodule (gitlink)`,
`repository config sets filter.lfs.process`, `repository config uses an include directive (include.*)`,
`metadata file packed-refs exceeds 32 MiB`, `metadata file info/exclude is a FIFO`. A reason names
a feature, config key, Git-relative metadata name or byte limit, never a config value, content or
outside path. What is refused is unchanged. A split index still fails first inside Git's own
`ls-files` probe (the private copy omits the shared index), so it keeps its existing
`MetadataProbeError` shape. The MCP tool-error object is closed under `MCPV0` ("never underlying
Git, repository, or process text"), so `corvint.status` still reports only `repository-unavailable`;
the CLI message is the diagnostic path. Touching `internal/gitstatus` moves the analyzer identity to
`corvint-analyzer/81`.

- Follow-up in the same change: `corvint work` also ran its own stricter refusals in
  `internal/worksource` before the probe, and those fell to the catch-all reason. They now carry a
  fixed reason too (`worksource.RefusalReason`), for example `the repository is unsupported:
  repository config uses an include directive (include.* or includeIf.*)`. Filter-driver refusals say
  `filter.*` rather than naming the driver, because WQO-V0-051 forbids repository-controlled bytes.
  Probe refusals reach `work` as `Git status refused the repository: REASON`.
- Independent review repair: a filter key in `.git/config.worktree` reached `work` stderr through the
  probe with its raw subsection, including C1 control bytes. A filter driver name now appears only
  when it is 1 to 32 characters of `[a-z0-9_-]`, otherwise `*` (EAF-V0-011, WQO-V0-051). The work
  test now covers the probe route and the exact dirty-worktree reason.

## 2026-09-24 work-executable-symlink: WQO-V0-049, issue #157 follow-up

- The reporter installs Corvint as a symlink in `~/.local/bin`; `work init --corvint-executable`
  refused it ("path and parent components must not be symlinks"). Owner decision in this change:
  `work init` and `work rebind` resolve the given absolute path through symlinks once, bind the real
  target, and say so on stderr. Every existing check (canonical absolute, no symlink component,
  safe parents, not group/world-writable, outside the repository) applies to the resolved target, so
  a link into the repository or to an unsafe file is still refused. Observation is unchanged: it
  reopens the recorded target without following links and rederives the digest.
- An upgrade that retargets the link leaves the old target bound; observation then refuses until
  `work rebind`, as it already did for any byte change.
- Tests: `TestWorkInitBindsResolvedSymlinkTargetWQOV0049`; `TestWorkInitRejectsUnqualifiedExecutableWQOV0049`
  now covers links to a missing, an unsafe-parent, and a repository-local executable.
- Review repair: the given path must also lie outside the repository before it is resolved, so a
  link committed in the repository cannot choose the bound target even when that target is a safe
  external file. The refusal table now asserts each specific reason. The positive test also
  retargets the link after commit (observation still passes against the bound target) and runs
  `work rebind` through the link, which reports and binds the new target.

## 2026-09-24 core-promotion-gates: decision 0382, companion smoke tool list

- Finding: task-store policy version 2 required the `public-release` gate for every release, but
  `make public-release-check` qualifies an installed companion bundle only and has no Core-only
  input, so no Core release could be promoted (v0-5 recorded it `FAIL`). Decision 0382 makes that
  gate optional in policy version 3 (`PRS-V1-005`); the other five gates stay required.
- Finding: `make companion-release-gate` failed at the 0.8.1 release commit `1281e26`. Its installed
  smoke required five `corvint-mcp` tools, but decision 0374 (`caf742b`) had returned the default
  profile to `corvint.query`, `corvint.impact` and `corvint.status`. `make gate` does not run the
  companion gate and the 0.8.0 and 0.8.1 releases did not run it, so the drift went unseen. Fix: the
  smoke requires the three default tools. The gate passed in a clean clone at `222b51d`.
- The v0-5, v0-7 and v0-8 promotions move from `1281e26` to this change's merge commit; the gates run
  there, with `release-checklist` before the `v0.8.1` tag exists.

## 2026-09-24 Issue #170 EAF-V0-012: isolated status reads through a search-only parent

Issue #170 reported the same refusal as #156 on a checkout that plain `git status` reads. One of
the causes we reproduced is a false refusal: `internal/gitstatus` pins every ancestor directory by
opening it `O_RDONLY`, so a parent the caller can search but not read (mode `0711` owned by another
user, common for shared and home directories) failed with `EACCES`. Git only needs search
permission there. The remaining reproduced causes (submodule gitlink, `filter.lfs.process`,
reftable, `include.*`, `core.attributesFile`, `TMPDIR` inside the repository or missing) are
refusals by design and now name their reason under `EAF-V0-011`.

- The Go standard library cannot hold a search-only directory as an `os.Root`: `os.OpenRoot` opens
  its directory for reading, and darwin's `syscall` exports no `openat`. Linux alone could use
  `O_PATH` with `syscall.Openat`, which would split the two platforms' semantics, and adding
  `golang.org/x/sys` was set aside as a new dependency for one call.
- Chosen: on `EACCES` the reader pins the directory by `Lstat` identity (a real directory, never a
  symlink) and opens its child by absolute path, final component `O_NOFOLLOW`. The existing status
  brackets re-check every pinned identity, so a replaced ancestor, or a child that no longer
  resolves to the handle that was read, is refused. The trade: a swap-and-restore of the ancestor
  entirely between the brackets goes unseen, which Git's own path-based reads share. A metadata
  file's own directory still needs read access.
- The MCP half of #170 (`corvint.status` returning only `repository-unavailable`) is unchanged:
  the tool-error object is closed under `MCPV0`. Decision 0383 settles it as a separate change: an
  opt-in `--error-profile reason-class` selector adds a closed `reasonClass`, with no free text.
- Test: `TestStatusReadsThroughSearchOnlyParent` reads through a `0311` parent, then replaces that
  parent with one holding the same repository directory, so only the parent's own pinned identity
  can refuse it. The fallback applies only when a directory's own open is denied, not an
  ancestor's. Touching `internal/gitstatus` moves the analyzer identity to `corvint-analyzer/82`.

## 2026-09-24 V1-0016 HLQ-V1-008: host lifecycle rerun on 0.8.1 with retained reports

The three decision 0373 Core tuples passed 9/9 on darwin/arm64 against the 0.8.1 release build
(`Corvint 0.8.1 (build 82)`, source `1281e26`), with the published 0.8.0 binary as N-1. The tuples
are unchanged: plain CLI, Codex CLI 0.153.2 with adapter 0.2.2, and Claude Code 2.1.267 with adapter
0.2.3. The runner reports are retained under
`conformance/host-lifecycle-v1/results/0.8.1-darwin-arm64/` and their sha256s are in the spec's
Results section, as `HLQ-V1-008` requires. This supersedes the 0.8.0 results, which had no retained
report.

Linux tuples, other host versions, the companion adapters, and live model-session cases stay
`NOT_RUN`; support stays FALLBACK. The result is stale at the next release.

## 2026-09-24 V1-0005 AHI-010: host matrix names exact tuples, tiers and conformance (decision 0381 item 3)

`integrations/compatibility.json` now names what decision 0381 item 3 asks for. The Codex CLI and
Claude Code rows are `tier: core` on darwin-arm64 at Codex 0.153.2 with adapter 0.2.2 and Claude
Code 2.1.267 with adapter 0.2.3, each with the 0.8.1 `host-lifecycle-qualification-v1` PASS report.
The Codex row previously named 0.149.0 static manifest evidence and the Claude Code row adapter
0.1.4 static evidence. Gemini CLI, OpenCode and Pi are `tier: companion` with `lifecycleConformance`
`NOT_RUN`. Every row stays FALLBACK and carries `fullSupport: external-dependent`, because FULL needs
authority or identity the host API does not supply (decision 0373 item 6).

The global degradation `black-box-install-session-uninstall-conformance-not-run` is removed: it no
longer holds for every host, and each row's `lifecycleConformance` now says which tuples ran.

The shipped per-package declarations are unchanged. They still describe static validation, because
recording a test of a package build inside that package needs a version bump, which changes the
tuple the test ran against. `AHI-010` and `integrations/README.md` name the new row fields.

## 2026-09-24 task-store projection: promotions, closures and the 1.0.0-rc.1 rename (decisions 0381, 0382, 0384)

This change carries the `.taskman/` projection of store mutations the owner ran on the primary
checkout on 2026-09-24.

- Policy version 3 (decision 0382). v0-5, v0-7 and v0-8 are promoted in order at `142d679` (merge
  of #171). Each carries five PASS attestations whose evidence is the sha256 of one clean-clone log
  per gate. The first run refused one attestation `LIMIT_EXCEEDED`: its `sourceIdentity` was 173
  bytes against the 128-byte identifier limit. The store wrote nothing for it; the rerun used the
  log's file name as the identity.
- V1-0174, V1-0175 and V1-0180 (PR #163) and V1-0229 (PR #161) are completed manually, closing
  decision 0381 step 2.
- V1-0240 to V1-0247 are filed from the 0.8.1 session's findings, including V1-0247 for issue #167.
- Decision 0384: release `v0-9` keeps its id with version `1-0-0-rc-1` and title `Corvint
  1.0.0-rc.1 release candidate`, and V1-0018 is retitled to match. No 0.9.0 is published.

## 2026-09-24 decision 0385: application flow proof in 1.0 (issue #175, AFU-V1)

The owner asked for "full and researched support" for issue #175 in 1.0, covering E2E test
selection for a change, website navigation by an agent, and generated documentation proven by the
same evidence. Six expert reviews informed the design: flow model and interop, test-evidence
provenance, E2E test-impact selection, agent website navigation, evidence-proven documentation, and
an adversarial trust reviewer. Their claims about current code were checked against `978b37b`.

The owner chose a split classification. The `affected --selection-profile e2e-safe` value is Core
and narrows only when every omitted test has an exclusion proof; otherwise it falls back to the full
relevant suite with a named code. `corvint flows` map, gaps, impact, navigation, proven docs and the
MCP `flows` profile ship as a qualified companion and cannot block the Core candidate.
`docs/specs/application-flow-understanding-v1.md` holds `AFU-V1-001..040`, rollout slices S1-S8
and the kill criterion. The flows row in the 1.0 product spec moves from experimental to companion.
Nothing is built yet.

The reviews also found two defects in the shipped experimental `flows` command: output paths are not
confined to the root, and the input file is opened before it is checked, so a FIFO blocks the read.
Both are fixed in slice S1 (`AFU-V1-036`). A third defect: the Playwright provider keeps only the
final attempt (`AFU-V1-012`).

The owner delegated the three open questions to experts. The navigation packet gets no external
agent format in 1.0, because no candidate format carries effect classes, verified state and
untrusted marking. Review is self-attested, because Git identity fields cannot be verified offline.
Claim anchors in hand-written docs are opt-in, and unanchored documents are reported as `UNPROVEN`.
An independent review of the first draft led to four changes: the E2E inventory is reconciled with
runner discovery, global paths are built in, exclusion proofs are split into `coverage` and
`reviewed-links` bases, and evidence carries forward only when nothing it depends on changed. The
parts of the model that `e2e-safe` reads are Core-owned, and the candidate ships without the value
if it is not qualified in time.

## 2026-09-24 Decision 0383: MCP reasonClass error profile

The MCP half of issue #170. `corvint-mcp --root ROOT` output is unchanged. Adding the
`--error-profile reason-class` argv selector (`MCPV0-027`) makes every tool error
`corvint-mcp-tool-error/1`, which is the `/0` object plus a required `reasonClass` from a closed set
of 16 values (`MCPV0-028`). `code` is unchanged. The black-box vectors are conformance profile `/2`.

- The class is chosen where the refusal is built. `internal/gitstatus` refusals carry a
  `reasonClass` value next to their reason, and `RefusalClass(err)` reads it with `errors.As`.
  `errDrift` is `metadata-drift`. Every other error is `unclassified`, including Git's own probe
  failures (`MetadataProbeError`), cancellation, and text that only looks like a refusal. The kernel
  errors (`gokernel.Error`, `contextindex.Error`) keep the class next to their code, and the bridge
  forwards it. No reason text or filter driver name reaches the MCP object.
- `TestEveryRefusalSiteCarriesAClosedClass` parses the package source. It requires every
  `unsupported(...)` call, and every refusing return of `unsafeConfig` and `captureReason`, to name
  a class constant other than `unclassified`. A new refusal site without a class therefore fails the
  test. That includes the #172 search-only-parent refusal, which is classed `metadata-unreadable`
  like the other irregular-mode refusals.
- Judgement calls: `core.bare` is `worktree-config`, because it redirects which tree status
  observes. A missing or uninspectable `.git` is `gitdir-pointer`. Scratch-directory ancestry and
  private-copy failures are `scratch-dir`.
- Observed gap: a split index never reaches the `split-index` refusal. Git's own `ls-files` probe
  refuses it first, so over MCP it is `unclassified`. `TestReasonClassUnclassifiedToolError` pins
  that behaviour.
- Negative controls were each run once and reverted. Giving the gitlink site `unclassified` failed
  the source test. Emitting `/1` without the selector failed
  `TestReasonClassToolErrorOverRefusedRepositories`.
- Touching `internal/gitstatus` and `internal/contextindex` moves the analyzer identity to
  `corvint-analyzer/83`.

## 2026-09-24 V1-0018: release version grammar admits 1.0.0-rc.N and 1.0.0 (decision 0384)

`corvint-release-candidate` and `corvint-release-install` share one version pattern
(`internal/releasecandidate/candidate.go` `versionPattern`, used by `Assemble` and by candidate
verification before install). It admitted only `MAJOR.MINOR.PATCHaN`, so neither `1.0.0-rc.1` nor
`1.0.0` assembled. It now admits exactly `MAJOR.MINOR.PATCHaN`, `MAJOR.MINOR.PATCH-rc.N` and
`MAJOR.MINOR.PATCH`, with no leading zeros and the `-rc.N` `N` at least 1. The alpha form still
admits `a0`, since `0.4.0a0` was a published tag, and every retained alpha fixture (`0.5.0a1..a3`)
still passes. One narrowing is deliberate: a leading zero in any numeric part (such as `01.2.3a1`) is
now refused, and no published version has one. `PUB-V0-023` now states the grammar; the Core-only amendment and
`daily-change-evidence-workflow-v0.md` no longer say a non-alpha version is refused.
`TestPUBV0023CandidateVersionGrammar` pins the admitted and refused forms, and
`TestPUBV0023ReleaseCandidateRC1AssemblesVerifiesAndInstalls` assembles, verifies and installs a
`1.0.0-rc.1` Core-only fixture.

Left unchanged: `script/release-checklist` already admits any `[0-9A-Za-z.+-]` VERSION, the VS Code
executable pin already admits `-rc.N`, and the `--version` banner checks match shape, not grammar.
No real `1.0.0-rc.1` candidate was assembled; `VERSION` is still `0.8.1`.

## 2026-09-25 V1-0251: Core `e2e-safe` selection profile and frozen corpus (AFU-V1 S4)

`corvint affected --provider FILE --selection-profile e2e-safe` (`internal/appflows/selection.go`
`SelectE2E`) selects every test whose file changed, that the impact graph reaches, that links to the
obligation closure, or whose observed coverage names a changed path. It omits a test only with a
`coverage` or `reviewed-links` exclusion proof, and otherwise returns the full relevant suite
(inventory plus discovered-but-uninventoried tests) with the seven closed `e2e-*` codes. `strict` and
`coverage` output is unchanged: goldens captured before the change pin it byte for byte.

Decisions, recorded in the spec's new "E2E-safe wire contract":

- New wire shapes: the provider is `application-flow-selection-provider/1` and the output is
  `e2e-safe-selection/0`. The ETS `omitted` member counts list cuts, so it is not reused.
- Coverage records live in a separate `application-flow-coverage/0` file that the provider names.
  With the records inside the provider, a global path, every new record would change a global path
  after its own evidence commit, so coverage would always be stale. Folding coverage into
  `test-run-evidence/0` is a follow-up.
- Named fixtures and seeds are global only through `global_paths`. Any other changed path that
  neither the graph owns, nor a reviewed link targets, nor a coverage record names blocks the
  coverage basis with `e2e-unmapped-change`.
- The TypeScript `e2e-runtime-dependency` and `executable-config-unresolved` frontiers do not unbound
  the closure here, the same precedent as the Playwright profile. Without that rule, every TS change
  was unmapped and nothing could narrow.

Corpus (AFU-V1-040): `cmd/corvint/testdata/e2e-safe-corpus.json` has 20 labelled, fault-injected
cases over one five-test shop app. `coverage` omits 15 tests with 0 unsafe (reduction 0.15), and
`reviewed-links` omits 3 with 0 unsafe (reduction 0.03). Fallback counts: exclusion-unproven 3,
global-path-changed 4, inferred-link-only 2, inventory-incomplete 3, map-stale 2, unmapped-change 3,
and bound-exceeded 0 (covered only by its unit case). No basis is withdrawn.

Negative control, run once and reverted: dropping the `linked-to-closure` and `observed-coverage`
reasons produced 4 unsafe omissions. Both live changes (the Corvint Playwright fixture suite and
Beamfall) stay `NOT_RUN`.

## 2026-09-25 V1-0260: unsupported records yield to floor-clearing symbols (`GPK-V0-066`, decision 0387)

A precise Beamfall task (base `0d7796be`, "Validate plugin trust roots at model loader
construction …") abstained with `below-relevance-floor` at `--limit 1`, while `corvint impact`
found `internal/plugin/trust.go`. A stage dump showed six plugin feature records scoring 705, each
resting only on the word `plugin`, plus an ADR. `PluginTrustRoots` (8 query words) and
`NewModelLoader` (11 words) were confident symbols, but `evalQuery` admits confident symbols only
when no competitive record exists. The emitted one-record packet failed `GPK-V0-039` and was
withdrawn, even though a supported answer was in the index. Three rewordings behaved the same at
limits 1 and 3, so this is ranking precedence, not phrasing.

`GPK-V0-066` (accepted 2026-09-25, decision 0387): when the record/document packet fails
the floor, `evalQuery` compiles the confident-symbol packet and applies the same floor to it before
withdrawing. The global floor is unchanged. Packets that already clear the floor are unchanged, so
at `--limit 10` single-word records still crowd the packet (`NEEDS_WIDENING`, no `trust.go`); that
residual is a follow-up. The fixed limit-1 packet is `READY` with `authoritative_results` 0 and the
`GPK-V0-046` syntax-only uncertainty line. `TestEvalQueryUnsupportedRecordsYieldToSupportedSymbols`
fails without the change. The analyzer schema moves to `corvint-analyzer/84`.

Frozen evaluations, base → fix:
- Beamfall goldens (`corvint eval`, 7 cases): recall 0.9, must_read 9/10, critical misses 0/5,
  top-5 6/7, abstention 1/1 and budget compliance 1.0 are all unchanged. Byte-weighted precision
  moved 0.707676 → 0.702464 (bytes 47574 → 47927). One case, `completed-atlas-impact-repair`,
  moved from a floor withdrawal to `symbol:script/context_atlas.py:impact`. That symbol is in the
  gold file, but the golden labels only `learned-path:` selectors, so it scores as not relevant.
- `tools/retrieval-bench --arms corvint`:
  - `v2_abstention`: all 82 samples are identical (abstained 0.073171 in both).
  - `v2_comment2context`: the first 40 samples are identical (hit@k 0.075, mrr@k 0.041667).

## 2026-09-25 V1-0184: beamfall-dogfood receipts for UC-CHANGE-CONSEQUENCE and UC-EVIDENCE-CARRYING-COMPLETION

Bound a `beamfall-dogfood` receipt for two of the three daily-workflow rows in
`conformance/use-cases-v0/ledger.json`. Both cite Beamfall's agent-run LCRES-15 change, bind
commit `86fde0eb21d2e0fc41bfc06a4c06c9c3aef63e59`, published on Beamfall/core branch
`claude/corvint-dogfood-LCRES-15` (PR beamfall/core#31, open), which went through the
`docs/DOGFOOD.md` daily path against base `0d7796be23efe4e2579408a5516b8aeff1a23008`. The sealed
CEM bytes at that commit were verified byte-identical to Corvint's local dogfood run (`cmp` against
`git show 9a4dcefe...:.corvint/changes/86fde0eb....cem.json`). `UC-CHANGE-CONSEQUENCE`'s subject is
the path-impact packet, named `prechange-impact.json` by the corvint-dogfood receipt convention,
because Beamfall's pre-edit range-impact packet is empty by construction (no committed diff exists
yet at query time); the actual path-impact packet found the affected model-loader and trust tests
and their `go test` commands (ticket V1-0262). `UC-EVIDENCE-CARRYING-COMPLETION`'s subject is the
`dogfood-report.json` with `complete: true`. Both receipts also carry a byte-identical copy of the
sealed CEM under their `beamfall-dogfood/` subject directory, since `.corvint/changes/...` does not
exist in Corvint's own tree for a Beamfall commit. `UC-TASK-ORIENTATION` gets no `beamfall-dogfood`
receipt: its pre-change query on Beamfall abstained with zero results (ticket V1-0260), so no PASS
outcome can be attested for it.

Updated the "Verified current state" paragraph of `docs/specs/use-case-conformance-v0.md` to record
the two bound receipts, the orientation absence and reason, and that the Beamfall intent spec these
receipts attest against (`docs/plugins/trust-roots.md`) was accepted by the owner on 2026-09-25 in
the same Beamfall PR (after the bind), while no row is `verified` yet because V1-0011 promotes the three
Core rows together and orientation has no Beamfall receipt; both rows stay `experimental`/`UNPROVEN`. The
edit shifted `UCV0-001..013`'s line numbers (no requirement IDs renumbered), so
`docs/specs/REQUIREMENTS.tsv` was regenerated (`script/gen-spec-requirements.sh`, run against the
staged spec file) and `make spec-requirements-check` passes. No existing `contract.json` receipt
pins `use-case-conformance-v0.md` itself (they pin `daily-change-evidence-workflow-v0.md` and
decision 0332), so no receipt repin was needed (V1-0216 does not apply here).

`go run ./conformance/use-cases-v0` reports `valid: true`, `evidenceCount: 17` (was 15),
`statusCounts.experimental: 3`, all claims `UNPROVEN`. `go test ./conformance/use-cases-v0/...` and
`go test ./internal/specindex/...` pass.

## 2026-09-25 V1-0011: the three Core use-case rows are VERIFIED

`UC-TASK-ORIENTATION`, `UC-CHANGE-CONSEQUENCE` and `UC-EVIDENCE-CARRYING-COMPLETION` move from
`experimental`/`UNPROVEN` to `verified`/`VERIFIED`. The UCV0-003 promotion input is one receipt from
each of the six evidence classes on every row:
- contract and implementation (decision 0373);
- hostile-tests (V1-0188);
- corvint-dogfood (V1-0207);
- the sealed-benchmark from daily-loop run-002, which passed all three jobs (V1-0012);
- beamfall-dogfood (V1-0184, completed with the orientation receipt above).

Promotion needs both Corvint and Beamfall dogfood (UCV0-010), and each row has both. The
nineteen historical rows stay `UNPROVEN`. The ledger now reports `claimCounts` `UNPROVEN` 19 and
`VERIFIED` 3, `statusCounts.verified` 3, and `evidenceCount` 18.

- Scope of the claim: the orientation Beamfall receipt came from a build that includes
  `GPK-V0-066` (decision 0387). The published 0.8.1 archive abstains on that query (V1-0260), so under
  `UCV0-012` the claim applies to releases that include decision 0387 (the 1.0.0-rc.1 candidate),
  not to 0.8.1.
- Benchmark quality: UCV0-007 notes that mechanical completeness does not replace independent review
  of benchmark quality. No independent quality review of run-002 is recorded, so that review is
  `NOT_PRODUCED`. The owner accepted run-002 as it stands and this claim scope (decision 0389).
- Test changes: `TestUCV0ProfileMigration` pinned the canonical ledger at 22 `UNPROVEN`. It now pins 19/3, and its
  fixture resets the promoted rows' claim along with their status.

`conformance/use-cases-v0/README.md` is updated to match. `go run ./conformance/use-cases-v0` reports
`valid: true`, and `go test ./conformance/use-cases-v0/...` passes.

Rollback: set the three rows back to `experimental`/`UNPROVEN` and revert the test pin. No
receipt bytes change.

## 2026-09-25 V1-0184: beamfall-dogfood receipt for UC-TASK-ORIENTATION

This completes V1-0184's third row. The LCRES-15 pre-change query abstained with zero results
(V1-0260). With `GPK-V0-066` accepted (decision 0387), a new agent-run Beamfall change went through
the daily path using Corvint 0.8.1 built from PR #189. The change adds tests anchored to the accepted
`PTR-V0-002` and `PTR-V0-003` for `internal/plugin` trust assessment: base
`6a95ae32116718d55687da97efd9108b5174fe5e`, bind commit `1c9fa18da1a3cd715b56f73b267c4ee5e8d3c76e`,
Beamfall/core branch `claude/corvint-dogfood-PTR-tests`, PR beamfall/core#32 (stacked on #31).

- Pre-change query result: `READY`, abstention `none`. It returned `internal/plugin/trust.go:trustAssessment`, the function the change tests. Its
  uncertainty keeps "all results are syntax matches; no project-owned authority corroborates the
  task" visible.
- `dogfood check` and `dogfood seal` result: `PASS`. The OCM links 2 of 4 requirements. `PTR-V0-001` and `PTR-V0-004` stay unknown because the change does not assess them.
  One `unbound-commits` note names the acceptance commit `6a95ae32` from #31.
- Receipt contents: the new `receipts/UC-TASK-ORIENTATION/beamfall-dogfood.json` carries byte-identical copies of `prechange-query.json`
  and the sealed CEM.

Adopter friction found along the way:
- An OCM claim anchors only to a slice-table `name:` field or a `t.Run` literal; map-key
  table cases are not extracted.
- Selector fragments are normalized: numeric suffixes are dropped and `checks`/`versions` are singularized.
  Each fragment had to be probed before the links resolved.

`go run ./conformance/use-cases-v0` reports `valid: true` and `evidenceCount: 18` (was 17). All claims are still
`UNPROVEN`.

## 2026-09-25 V1 bug batch: V1-0123, V1-0159, V1-0172, V1-0238, V1-0222, V1-0131

- V1-0123 (`EEP-V0-001`): a provider record whose object repeats a member name is now `invalid`,
  worded as in `EEP-V0-020`; Go's decoder previously kept the last value silently.
  `TestRepeatedMemberIsInvalid`.
- V1-0159 (`ESV-V0-009`): a repeated `context --expand` is an argument error, and an empty HANDLE
  now reports `invalid-handle` instead of reading as absent. `TestParseContextViewArguments`.
- V1-0172 (`DCW-V0-014`): the `cem-cite` `cite-span-not-stable` fix hint names the failing
  citation-plan row. `script/dogfood-change_test.sh`.
- V1-0238 (`WQO-V0-051`): the `work rebind` unqualified-adoption refusal prints the fixed
  `workSourceReason` text, never Git stderr. `TestWorkRebindUnqualifiedAdoptionOmitsGitStderr`.
- V1-0222: `docs/RELEASE-RUNBOOK.md` step 8 now shows the N-1 upgrade lifecycle invocation.
- V1-0131: `docs/decisions/README.md` loses its stale 0105 and 0232 duplicates and is sorted again.
  The index still lacks rows for about 51 decision files and has no duplicate-row check; neither is
  in this change.

## 2026-09-25 AFU S2 remainder: every Playwright attempt on the receipt (V1-0249)

- `AFU-V1-012`: the unprofiled `corvint-js-test-provider` receipt gains the additive
  `attemptDetails` member, so a retry no longer erases the earlier attempts' duration, failure,
  anchor or attachments on the wire. The `corvint-playwright-external` profiles refuse the member on
  encode and on qualified-reporter decode rather than widening `/0`..`/2`: a profiled wire field
  needs another profile revision (`docs/specs/playwright-external-provider-v0.md`). The profiled
  reporter still emits only the last attempt's detail; closing that needs a `/3` profile and live
  reporter qualification. `TestAFUV1PlaywrightProviderKeepsEveryAttempt`.
- `AFU-V1-038`: the run-evidence hygiene moves to `internal/runhygiene` (an import cycle kept
  `internal/jstestprovider` from importing `internal/appflows`). The provider now applies it and the
  product secret screen to every attempt and to the last-attempt fields, after state classification
  has read the raw message. `TestAFUV1PlaywrightProviderScrubsEveryAttempt`.
- Still partial: `AFU-V1-013` (repeated-run aggregation needs the `internal/doccorpus` stability
  counting exposed for `test-run-evidence/0`) and `AFU-V1-014` (the AFU-V0-010 observer observes
  flows, not tests, so it has no test key to emit a `LOCALLY_OBSERVED` record for).

## 2026-09-25 OIF-V0-005: closed Decisions heading variants (open decision 3 answered)

The owner delegated open decision 3 of `docs/specs/ocm-intent-forms-v0.md`, and the answer is yes.
`adr-decisions` now reads exactly one unfenced heading from the closed set `## Decisions`,
`## Decision`, `## 2. Decisions` and `## 2. Decision`. The fourth heading occurs once at Beamfall
`2a8e06b28` and has the same `### 2.1` items as `## 2. Decisions`, so it is included. Two headings
from the set, or any other shape such as `## Decisions:`, still fail `invalid-decisions-section`.
The item grammar is unchanged. A scratch sweep of the 215 ADRs through the reader moved from
89 derived, 113, 6 and 7 refused (the live run's numbers) to 99 derived (620 requirements), 49
`invalid-decisions-section`, 42 `invalid-decision-item` (all 22 numbered-heading ADRs) and 25
`missing-requirements` (18 `## Decision` ADRs have prose and no level-3 items). The failures stay
visible and no item shape is guessed. The spec stays proposed. Corvint orientation found decision
0386 by query and the spec, the test file and the OCM callers by path impact.

## 2026-09-25 V1-0252: application navigation map (AFU-V1 S5)

`corvint flows navigate --flows DIR` (`internal/appflows/navigate.go`, `cmd/corvint/flows_navigate.go`)
writes `application-navigation-map/0` from the intents committed at HEAD, `--evidence` run records
and `--traffic` observations. `--goal FLOW_ID [--max-effect CLASS]` writes the bounded
`application-navigation-packet/0`. `internal/appflows/origins.go` holds the AFU-V1-029 gate. The spec's
new "Navigation wire contract" records the shapes. The conservative readings chosen:

- Navigation data is an optional closed `navigation` member on `application-flow-intent/1`, so
  locators come only from the intent (AFU-V1-026). A step with no entry gets no state and no locator,
  and its outcome's locator string is never promoted to one. Inputs, credentials included, are named
  only by `input_fixture` ID; no field carries a value.
- An `api` entry's state is its method and path. A declared `read` on a non-safe method is refused at
  validation, because the intent contradicts itself.
- Effect raising (AFU-V1-027): "non-GET" is literal, so `HEAD` and `OPTIONS` traffic also raises.
  Every observed form submit counts as undeclared, because the intent has no form-submit
  declaration. Either observation contradicts only a declared `read`, which rises to
  `write-irreversible` (the undeclared class), since traffic cannot show that a write is reversible.
  Declared write classes stand. Traffic records apply whatever revision they came from, because they
  can only restrict.
- Verified state per step: a step is `verified` only when a variation listing it has at least one
  required evidence pair and every pair is verified. Otherwise it is `unverified`.
- The packet orders precondition flows depth first, each once, then the goal. Recovery targets move
  out of the ordered steps. An unknown or cyclic precondition flow refuses both the map and the
  packet. An unknown class is never granted. Bounds: 256 packet steps, and 8,192 traffic records,
  the run-evidence bound. Either one exceeded is `navigation-bound-exceeded`.
- AFU-V1-029 is partial. `AdmitTransition` admits `read` and `write-reversible` anywhere, and
  anything else only against a canonical origin listed in the committed `origins.json`. A
  working-tree edit is never honored. No Corvint observer executes navigation transitions yet. The
  AFU-V0 manifest observer carries no effect class, so wiring the gate into it would refuse every
  V0 observation; it is left unchanged.

Evidence: the `navigate` and `navigate-checkout` goldens, and a deterministic scripted agent. From the
packet alone it completes each of the five mapped fixture goals, including one recovery. It performs
no write under the default `read`, and it cannot start the unmapped goal. A negative control was run
once and reverted: with raising and grant marking disabled, both AFU-V1-027 tests and the two
AFU-V1-028 packet and agent tests failed. No live observer qualification exists (`NOT_RUN`).

Review repair (same slice): the packet moved every step another step names as `recovery` out of
the path. So `pay` with recovery `open` dropped the path step `open`, and mutual recovery emptied the
path. An intent now refuses a recovery that is not a later step in intent order or that some
variation lists. Recovery targets therefore sit off every declared path, and no cycle can form. The
unit sample dropped its backward recovery. `earlier recovery` and `path recovery` joined the
AFU-V1-026 refusal cases. Bounds on the `--traffic` file count and on the quadratic per-step filter
stay follow-ups: the total record bound caps both.

## 2026-09-25 V1-0253: proven flow documentation (AFU-V1 S6, AFU-V1-030..033)

`corvint flows docs` (`internal/appflows/docs.go`, `cmd/corvint/flows_docs.go`) renders one
Markdown page from the intents committed at `HEAD` with a fixed template and writes the
`flow-doc-claims/0` sidecar. `--check` writes `flow-doc-check/0` and writes nothing. The shapes are
in the spec's new "Proven-documentation wire contract".

Decisions. Each resolves an ambiguity with the most conservative reading:

- A claim is one outcome of one variation, so a variation with no outcome makes no claim.
- `PROVEN` needs more than a verified variation (AFU-V1-031 states only a necessary condition). The
  outcome also needs a declared assertion link, and every declared link from the variation, its
  steps and the outcome must be `reviewed`.
- The state order is `CONTRADICTED`, then `STALE`, then `PROVEN`, then `UNPROVEN`. `flaky` evidence
  counts as `CONTRADICTED` because it contains a failed attempt, so contradicting evidence is never
  hidden behind staleness.
- "Evidence IDs" are the required test key and project pairs, not run IDs. Run IDs change on every
  run, so a sidecar holding them could never match regeneration. The page and sidecar embed no
  commit for the same reason.
- Everything `--check` compares is read at `HEAD` through Git, never from the working tree: the page,
  the sidecar, the waivers file and the hand-written Markdown. Only committed data carries authority.
- The waivers file is named by `--waivers` rather than kept in the flows directory, where every
  `.json` file is an intent candidate.
- A waiver has expired on its `expires` date (UTC). A waiver also covers a committed claim whose state
  changed without losing `PROVEN`, and a claim that is no longer rendered. The committed claim is
  then used for the byte comparison. Without that, every waived claim would still fail as
  `rendered-bytes-differ`, and AFU-V1-032 would have no exception.
- An anchored claim is stored in the sidecar under `PATH:FLOW/VARIATION/OUTCOME`. It is therefore
  checked for a lost `PROVEN` like a page claim. An anchor naming an outcome that its variation does
  not list is `unknown-anchor`. A malformed anchor is `invalid-anchor`, and its values are not
  echoed. Either failure also refuses the render.
- A document counts as anchored only when it has at least one valid anchor. The coverage row counts
  unanchored documents over the Markdown documents under the docs root, leaving out the generated
  page. `value` is a count, so a zero denominator gives 0 of 0, not a ratio.
- Intent text on the page has all ASCII punctuation backslash-escaped. Text therefore renders
  literally and cannot forge an anchor comment or markup.
- Rendering replaces each file through an `O_EXCL` temporary file in the same directory and
  `os.Root.Rename`. It refuses a non-regular target or a symlinked parent (AFU-V1-036). The page and
  the sidecar are two separate renames, not one atomic pair.

Limitation: evidence holds only at the exact commit it ran. `--check` therefore needs run evidence
from the checked commit, such as a CI step after the E2E run. The page is committed one commit after
the evidence it was rendered from, and the check passes only when that evidence is regenerated at
the new commit.

Evidence:
- `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/appflows/... ./cmd/corvint/ -run 'Flows|AFU|Docs'`
  passed. The new tests are `TestAFUV1030DocsRenderGoldenAndByteStable`, `TestAFUV1031ClaimStateOrder`,
  `TestAFUV1032DocsCheckDrift`, `TestAFUV1032WaiverExpiry`, `TestAFUV1032WaiverExpiryBoundary`,
  `TestAFUV1033AnchoredMarkdown`, `TestAFUV1033AnchorParsing` and `TestAFUV1036DocsReplaceConfined`.
- The goldens are `cmd/corvint/testdata/flows/docs.golden.md` and `docs.claims.golden.json`. They
  reach every claim state.

`NOT_RUN`: live qualification on a real application (S8), and the full `./...` suite.

Review repair: a review found two defects that contradicted the spec. First, an unexpired waiver
for a claim that is no longer rendered, such as an anchor deleted from `docs/guide.md`, listed the
claim under `waived` but still failed with `rendered-bytes-differ` on the sidecar, because
`keepCommitted` only replaced a claim that was still present. It now appends the committed claim when
it is absent, so the committed form is used for the byte comparison (AFU-V1-032). Second, any `.md`
under `--docs-root` that is not a regular blob, such as a symlink or a submodule, refused both render
and check, although an unanchored document must never fail the check (AFU-V1-033). Such an entry is
now not read and counts as unanchored in the existing coverage row; the spec wire text says so and no
wire field was added. The tests are `TestAFUV1032WaiverKeepsUnrenderedClaim` and
`TestAFUV1033NonRegularMarkdownSkipped`; both fail without the fix. No golden changed. Follow-ups,
not fixed here: anchors inside fenced code blocks are still parsed as anchors, and `--page` and
`--claims` still accept paths under `.git/`.

## 2026-09-25 V1-0255: acceptance fixture (AFU-V1 S8, AFU-V1-039)

`TestAFUV1039AcceptanceFixture` (`cmd/corvint/flows_acceptance_test.go`) runs `flows map`, `gaps`,
`navigate` and `docs` through the CLI over a committed fixture in
`cmd/corvint/testdata/flows/acceptance/`. The fixture has four intents: `checkout` is a UI flow and
`orders-api` is an API flow, and both reach verified. `returns` is declared with no links, which gives
`unmapped-flow`. `profile` has a source link to a line span that changed after review, which gives
`stale-link`.

Decisions:

- The fixture sits beside the other flows goldens under `cmd/corvint/testdata/flows/`, not under
  `conformance/`, because the spec does not name a conformance location for AFU-V1-039.
- The test builds three commits in a temporary repository. Commit A holds the application, the
  Playwright specs and the intents. Commit B sets every `reviewed_at` to A. Commit C changes line 4
  of `app/web/profile.js`, which is inside the reviewed span 3-5. The committed intents hold the
  placeholder `REVIEW_ANCHOR` because the anchor SHA exists only at test time.
- Evidence is `playwright-report.json`, ingested with `flows ingest --format playwright-json` at C.
  The committed synthetic report records all three as passed, including profile's. `profile` is
  therefore incomplete only because of the stale link, and verified evidence cannot mask it.
- `docs` renders the page and sidecar at the repository root without `--docs-root`, because that
  flag needs a directory that exists at `HEAD`.

The test asserts:

- In map, both flows are `complete` with every variation verified, and the other two are
  `incomplete`.
- In gaps, `checkout` and `orders-api` report no codes, `profile` reports exactly
  `stale-link save-name` and `returns` reports exactly `unmapped-flow`.
- In navigate, the four statuses match, the checkout `pay` and orders-api `create-order` transitions
  are `verified`, and `--goal profile` and `--goal returns` each return one `incomplete` flow.
- In docs, the claim states are `PROVEN`, `PROVEN`, `STALE` and `UNPROVEN`. Only the two `PROVEN`
  claims appear on the page without a `**STATE:**` marker.

No command lacked a way to express one of these assertions.

Observations, not changed:

- The navigate transition for profile's `save-name` step reads `verified`, and map reports its
  variation `verified: true`, although the flow is `incomplete`. Verification is per evidence pair,
  and the stale link shows only in flow status, gaps and the docs claim.
- The `docs/specs/README.md` row for the spec still says slices S1-S8 are unbuilt. That was stale
  before this change.

Evidence:

- `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/appflows/ ./cmd/corvint/ -run 'AFUV1|Flows'`
  passed.
- A mutation check moved the post-review edit from `profile.js` to `checkout.js`. The test then failed
  in map, gaps, navigate, the goal packet and docs. The fixture was restored.

`NOT_RUN`:

- Companion surfaces on Beamfall: this needs a Beamfall checkout and a networked browser E2E run.
- A real change on `conformance/interactive-alpha/fixture`: this needs a Playwright browser run.
- A real change on Beamfall: this needs a Beamfall checkout and a networked browser E2E run.
- The full `./...` suite and `make gate`: both are out of scope for this slice.

Follow-up: `navigate` could carry link review state into its steps or packets, so an agent reading
only a step does not see `verified` on a step whose reviewed link is stale.

Review repairs: an independent review found no blockers. The repairs are:

- Fixture: profile gains a `navigation` entry, so its `save-name` step is actionable.
- Tests:
  - The test pins the step's current `verified` value with a comment, and asserts that returns'
    `request-return` step is `unverified`.
  - Map now requires at least one variation for a complete flow and asserts that profile's
    variation is verified.
  - Each surface compares the sorted flow or claim IDs with the `want` keys, not just counts.
  - Ingest uses a dedicated Playwright header (`--runner-version 1.50.0`, no `--control`).
  - The `filepath.Rel` error is checked, and the goal packets go through `decodePacket`.
- Spec:
  - The navigation wire contract states that step verification is evidence-only.
  - The AFU-V1-039 row says the report is synthetic and that observed runs are the `NOT_RUN` items.
  - "Both stay" becomes "All three stay".
- `docs/specs/README.md`: the row now says S1-S6 and S8 are implemented, unqualified, with live
  qualification `NOT_RUN`, and S7 pending. S7, the flows MCP profile, is not on this branch:
  `cmd/corvint-mcp` has no `flows` tool profile here, so the row does not claim S1-S8. Delivery
  stays `planned` in the spec header, `INDEX.json` and the README, which the specindex test keeps
  in agreement.

## 2026-09-25 V1-0254: corvint-mcp flows tool profile (AFU-V1 S7)

`corvint-mcp --tool-profile flows` advertises the three V0 tools plus the read-only
`corvint.flows.map`, `corvint.flows.gaps`, `corvint.flows.impact` and `corvint.flows.navigate`
(AFU-V1-034). This amends MCPV0-026, and the MCP spec records the amendment.

Decisions:

- The selector stays closed. Its values are exactly `task-review` and `flows`, and each value maps
  to one registry constructor. A second `--tool-profile`, even with another value, is refused, as
  are an empty or unknown value, the `=` spelling, and the selector beside `--version`. Without the
  selector, the `tools/list` bytes are unchanged. Golden files captured from the pre-change binary
  pin both the default and the task-review list.
- Each tool calls the same `internal/appflows` functions as its CLI verb and adds no logic of its
  own. It reads the intents at `HEAD` between two repository probes, and it abstains with
  `REPOSITORY_STATE_UNSTABLE` if `HEAD`, the tree or the dirty-path set differs between them. The verb's JSON document becomes the bridge receipt. The receipt
  schema must be one of the verb's schemas, and its `revision` must equal the bound commit.
- To share the impact path, the CLI's `flows impact` body moved into `appflows.FlowImpactAt`. The
  eight affected-language adapters moved into `internal/liveverify/affected/languages`, which both
  `corvint affected` and `FlowImpactAt` use. The CLI output is unchanged.
- `internal/appflows` now spawns the process-wide Git from `gitstatus.Executable()` instead of its
  own PATH lookup. Under the MCP server that is the start-time pinned Git (MCPV0-016). In the CLI it
  is still the Git on PATH.
- Arguments are closed JSON Schemas. Input files are repository-relative, at most 100 per array,
  and never under `.git`. Each component of an input file's parent directory must be a real
  directory, not a symlink, when it is checked before the open. `impact.base` is a full
  object ID, not a revision expression. `navigate.goal` is at most 64 characters, and `maxEffect` is
  allowed only beside a goal. Invalid arguments are `-32602`. Any verb refusal is the single code
  `flows-refused`, because the verb's message can name repository content.
- Flow intents carry repository-authored text (AFU-V1-035). A flows result therefore omits
  `structuredContent` and returns the bridge object only as the enveloped text. The
  envelope-terminator collision refusal applies as it does for every tool.
- The line citations of `internal/mcp/bridge/bridge.go` in `docs/GLOSSARY.md`,
  `docs/SPEC-TOOLCHAIN-INTEGRATION.md` and the MCP failure-code table were repinned. The table rows
  were already stale before this change, and they now carry anchors.

Evidence: `TestAFUV1034FlowsToolProfile`, `TestAFUV1034FlowsToolsMatchCLIVerbs` (receipts equal the
CLI verbs' output on the shop and navigation fixtures) and `TestAFUV1035FlowsTextStaysInsideEnvelope`
(a hostile navigation step string appears only inside the envelope).

NOT_RUN: compiled-process MCP conformance vectors and a manifest for the flows profile; the
official-schema exchange under the flows selector; live qualification; the exhaustive `make gate`.

Review repairs (same slice):

- The appflows Git runner now spawns Git with `gokernel.SanitizedGitEnvironment()`, which was
  exported for this, and with `-c credential.helper=`. The environment sets `GIT_NO_LAZY_FETCH=1`,
  `GIT_NO_REPLACE_OBJECTS=1` and no system or global config. Before this, a `cat-file blob` in a
  partial clone could lazily fetch and write `.git/objects`, which breaks invariant 4 and MCPV0-017.
  `TestAFUV1034FlowGitRunsSanitized` checks the environment and arguments with a fake Git; it
  failed with the environment removed.
- `affected.Build` shares one file walk per root among Builds that overlap in time. An impact call
  could therefore reuse a walk taken before its own first probe. The bridge now runs one
  `corvint.flows.impact` at a time, holding a mutex across both probes and the verb. This was
  chosen over keying the walk per Build call because the walk is reached from language plugins
  that carry no Build identity, so keying it would change the `affected` plugin interface. In the
  MCP process no other tool calls `Build`, so serializing impact is enough.
- The map and navigate schemas now match the runtime checks. Path and test-key characters exclude
  U+0080 to U+009F, which `unicode.IsControl` refuses. A path may not end in `/`. `map` forbids
  `path` with `testKey`, and forbids either one with a non-empty `evidence`. `navigate` has
  `dependentRequired: {maxEffect: [goal]}`. The default and task-review goldens are unchanged.
- New tests: `TestAFUV1034FlowsAbstainWhenTheCheckoutMoves` (a probe stub changes the dirty-path set),
  `TestAFUV1034FlowsRefuseSymlinkedInputParent` and `TestAFUV1034FlowsLeaveRepositoryBytesUnchanged`
  (all four tools, with `.git` included in the digest).
- The symlink wording now states what is checked: the parent components are checked with Lstat
  before the open. A swap after that check is not detected, and this is not worth code here. The
  probe wording now names what is compared: `HEAD`, the tree and the dirty-path set.
- Follow-up: `affected.Build` takes no context, so cancelling a `corvint.flows.impact` call does not
  stop a walk already in progress.

## 2026-09-25 Panel M2: a limit that omits a competing record needs widening (`GPK-V0-068`, decision 0396)

The pre-1.0 panel (finding M2) ran `corvint query --limit 1` on Beamfall with the COREAPI-AUDIT-0822-3
headline ("Carry HLS foreground priority in the signed stream grant instead of the
X-Beamfall-Foreground request header"). It returned `READY` with one result,
`feature:access-request-grant` (score 450). The relevant `feature:hls-transcode` (420) was second
at limit 10. `evalQuery` set `NEEDS_WIDENING` only on an exact tie of the top two record scores.

The panel suggested requiring term support for the top record before `READY`. That test does not
fire here: `access-request-grant` rests on two query words, `request` and `grant`, so it clears the
`GPK-V0-039` floor. What the limit-1 packet hid was a competing reading. `hls-transcode` rests on
`hls`, a word no emitted result rests on, and only score chose between the two.

`GPK-V0-068` (accepted 2026-09-25, decision 0396): when the result limit omits a competitive
record that rests on a query word no emitted result rests on, the packet is `NEEDS_WIDENING` with
an active abstention, reason `omitted-competing-record`, and keeps its results. Support is
counted per result in the query's own words, as `GPK-V0-039` counts it.
`TestEvalQueryLimitOmittingCompetingRecordNeedsWidening` fails without the check. The analyzer
schema moves to `corvint-analyzer/85`.

Beamfall fixture `2a8e06b2`, same headline: base limit 1 `READY`; fix limit 1 `NEEDS_WIDENING` /
`omitted-competing-record`; limits 5 and 10 stay `READY`.

Frozen evaluations, base → fix:
- Beamfall goldens (`corvint eval`, 7 cases): all identical. Recall 0.9, must_read 9/10, critical
  misses 0/5, top-5 6/7, abstention 1/1, budget compliance 1.0, byte-weighted precision 0.702464,
  and every case state unchanged.
- With the M2 headline added as an eighth, unfrozen case (limit 1, expected `NEEDS_WIDENING`):
  epistemic state accuracy 0.5 → 1.0. This case is not in Beamfall's frozen file, which Beamfall
  owns.
- Golden query texts rerun at limit 1: only `ambiguous-reveal-navigation`, whose gold set holds both
  competing features, changes (`READY` → `NEEDS_WIDENING`). At limit 3 none changes.
- `tools/retrieval-bench --arms corvint`, default limit: `v2_abstention` (82 samples) and
  `v2_comment2context` (first 40 samples) have identical ranked lists, abstentions and states
  (abstained 0.073171; hit@k 0.075, mrr@k 0.041667). At `--limit 1` (first 40 samples of each)
  the rule fires on no sample. These corpora have no canonical feature records, so they cannot
  exercise it. The Beamfall case is the only evidence that the rule fires.
- `conformance/cli-parity-v0` replay with the fix: exit 0 (parity 104, known divergences 26). No
  base replay was run for comparison.
