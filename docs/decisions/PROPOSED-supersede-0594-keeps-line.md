# Proposed: supersede the Keeps line of decision 0594, the external runner protocol of decision 0592, and the CEM, OCM and dogfood ceremony

- Date: 2026-11-09
- Status: proposed. Not accepted. This file has no decision number and no row in
  `docs/decisions/README.md`. Only the owner can accept it, in whole or in part.
- Authority it would record (owner answers of 2026-11-09, recorded in the owner-answers file of
  that date):
  - G8, CEM / OCM / dogfood: "Replace fully (Recommended)".
  - G22, VS Code extension: "Move out, rebuild after 1.0 (Recommended)".
  - G23, runner plugins: "One built-in contract, drop protocol (Recommended)".
  - G24, test-quality features: "Partial cut (Recommended)".
  - G25, context and guidance verbs: "Revive as proposed".
  - G26, languages beyond the seven runners: "Remove both (Recommended)".
  - G27, process tooling, spec archive and removal order: "Panel plan (Recommended)".
  - Owner decision R14 (2026-11-09): provider binaries retire after G23, folded into `corvint`.
  - Owner decision R28/R29 (2026-11-09): `corvint context` ships redesigned and non-blocking, with
    no claim of measured benefit; the measurement runs in 1.0.x.
  - Owner decision R32 (2026-11-09): a change no runner owns, or a removed or unavailable runner,
    runs all tests with a named reason; `UNOWNED_DIRTY_PATH` comes off the allowlist.
  - Owner decision R33 (2026-11-09): the VS Code extension moves to the public repository
    `beamfall/corvint-vscode`; orphaned code is deleted and tracked in a table.
  - Owner decision R34 (2026-11-09): one build-log entry per removal batch; this decision stays
    short.
  - Owner decision R36 (2026-11-09): `make windows-vet` runs once per removal batch, before that
    batch's push, for removal lanes only.
- Would amend: decision 0594 (the Keeps line, item 4, and item 8's layer list and the analyzer
  part of its binary catalog); decision 0592 items 1 and 2; decision 0591 and
  `docs/specs/core-bundled-apps-v0.md` where they name `corvint-plugin/1`; decision 0595 and the
  CMS-V0-915 trace row where they name `internal/testplan` as a flow reader; `AGENTS.md` line 3,
  product invariants 3, 4, 5, 7 and 8, the Spec-driven development section, the Verify section and
  the Agent routing section (proposed text, or an open owner question, below, item 8).
- Keeps in force until removal batch B11: decisions 0224 (full-gate receipt) and 0352 (gate-ledger
  per-package bound) (item 8).
- Read with: the change `docs/specs/openspec/changes/source-removal/` (`proposal.md`, `design.md`,
  `tasks.md`, `specs/source-removal/spec.md` with requirements RMV-V1-001 to RMV-V1-031, and
  `path-verdicts.md`), which carries out this decision; `docs/specs/PRODUCT-PLAN-1.0.md`;
  acceptance through `docs/decisions/PROPOSED-accept-product-plan-1.0.md`.

Code and text facts were checked read-only at `9ec3a29d0` (`claude/layers-integ`) and in this
branch (`claude/product-specs`, `fc486d7d9`). Anything not checked says NOT_VERIFIED. Citations
in `path:line@hash` form name files on this branch; facts that hold only in the integration
checkout are written as prose naming the file, the line and `9ec3a29d0`.

## Context

Decision 0594 (accepted 2026-11-09) refocused Corvint on test selection and listed what it keeps
(`docs/decisions/0594-refocus-on-test-selection-2026-11-09.md:25-29@ce2918fe`): the top-level verbs adopt,
affected, cem, context, feature, features, help, impact, index, init, mcp, overview, review, runs,
test, test-evidence, test-plan, test-validity and tests, with docs, flows and how; every
`corvint-analyzer-*`, `corvint-*-test-provider` and `corvint-test-runner`; the release support
binaries; `corvint-mcp`, `corvint-test-validity-mcp`, `corvint-docs-mcp` and `corvint-web-flows`.
Item 4 (`docs/decisions/0594-refocus-on-test-selection-2026-11-09.md:58-65@1b9b96e2`) removed the dogfood ceremony but kept the `corvint cem` verbs begin, prepare,
cite, mark, cover, status, verify, report and verify-stable.

Decision 0592 (2026-11-07) item 1 (`docs/decisions/0592-runner-selection-plugins-2026-11-07.md:31-36@a66e94c2`)
set two runner plugin forms, compiled-in and an out-of-process `corvint-runner/1` executable pinned
in `.corvint/runner-plugins.json` (RSP-V0-025 to RSP-V0-032). Item 2 (`docs/decisions/0592-runner-selection-plugins-2026-11-07.md:37-41@290d9793`) set the language
scope: Go, TypeScript and JavaScript with Playwright, Swift, Kotlin and Java, C#, Rust, Python,
Ruby, shaders, and shell and make.

The owner then fixed 1.0 to three products and seven release-blocking runners (G1: Go, Vitest,
Jest, Playwright, pytest, RSpec, Minitest) and answered the source keep-or-remove audit
(G22 to G27). Those answers contradict the 0594 Keeps line and 0592 items 1 and 2, so a decision
is needed before the removals.

## Decision (proposed)

1. **Keeps line superseded (G24, G25, G26).** The 0594 Keeps line is replaced by this list.
   - Kept verbs: adopt, affected, help, init, mcp, test, test-evidence (preview), tests (only
     `setup-select`, `teardown`, `unselected`, `role-locators`, `env`, `citations`,
     `example-join`; `cmd/corvint/tests.go:345-363@ce5185b3`), docs, flows, how.
   - Kept for now, pending their revival proposals (G25): impact (to become the cross-product change
     report, preview), review (to become the pre-merge evidence check, preview), context (ships
     redesigned and non-blocking, with no claim of measured benefit; the measurement runs in 1.0.x;
     owner decision R28/R29 (2026-11-09)), feature, features and overview (to fold into
     flows), index (kept only as the warm cache for `watch` if it helps the 1.0 s gate). None of
     them blocks the release.
   - Removed verbs: cem, runs, test-plan, test-validity, `tests lint`, `tests duplicates`,
     `tests validity`.
   - Kept binaries: `corvint`, `corvint-docs`, `corvint-cli`, `corvint-mcp`, `corvint-docs-mcp`,
     `corvint-web-flows`, `corvint-test-runner`, `corvint-update`, and the release support binaries
     other than `corvint-go-toolchain-receipt`.
   - Retired after G23: `corvint-go-test-provider`, `corvint-js-test-provider` and
     `corvint-ruby-test-provider`, each folded into `corvint` once its compiled-in runner contract
     lands; not shipped as separate 1.0 artifacts (owner decision R14 (2026-11-09)).
   - Removed binaries: every `corvint-analyzer-*` (16), `corvint-test-validity-mcp`,
     `corvint-go-toolchain-receipt`.
   - "Every package those reach through kept code" stays as the rule for packages.

2. **0594 item 4 amended (G8).** The `corvint cem` verbs leave 1.0. The packages under
   `internal/cem` that kept code imports (`wire`, `gitauth`, `gitrun`, `cemcode`, `coverprofile`,
   and `patch`, which `gitauth/diff.go` imports) move to the shared layer first (RMV-V1-020).
   `internal/cem/verify` and `internal/cem/sim` do not move: the CEM stable-result release gate
   that reads them (`internal/corerelease/core_stable_result.go`, the `releasecandidate` CEM
   readiness gate and `tools/cem-release-suites`) leaves with the verbs.

3. **0594 item 3 amended.** Item 3 keeps `docviews`; it has no importer at `9ec3a29d0` (VERIFIED),
   so batch B1 deletes it as orphaned code and lists it in the batch's orphan table (owner decision
   R33 (2026-11-09); RMV-V1-031).

4. **0592 item 1 narrowed (G23).** Only the compiled-in form remains for 1.0. RSP-V0-025 is
   narrowed to "runner pieces are compiled in"; RSP-V0-026 to RSP-V0-032, RSP-V0-047 and every
   reference to `corvint-runner/1` and `.corvint/runner-plugins.json` are deferred. The external protocol may
   return after 1.0 under its own proposal. `internal/runnerprotocol`, `runnerpolicy`,
   `runnerqualification` and `tools/runner-*` leave. One compiled-in contract for the seven
   runners is chosen by its own proposal.

5. **0592 item 2 superseded (G1, G26).** The 1.0 language scope is the seven runners plus Go,
   shell and make targets and declared test scripts. Swift, Kotlin and Java, C#, Rust, shaders and
   the other analyzer languages leave. A change no runner owns, or a change for a removed or
   unavailable runner, runs all tests with a named reason, and `UNOWNED_DIRTY_PATH` comes off the
   allowlist (owner decision R32 (2026-11-09); RMV-V1-014). `corvint test` does not do that today
   (`design.md` open item), so removal batch B5 waits for it.

6. **CEM, OCM and dogfood replaced (G8).** Release gates for 1.0 are: the `corvint affected` plan
   for the change, exhaustive `go test` and `go vet`, and one scripted end-to-end qualification per
   product (testing, documentation, task management). Self-use is recorded as use of those three
   products on Corvint itself. No change evidence map is produced or required. V1-0014 and V1-1344
   to V1-1348 close as superseded after acceptance.

7. **VS Code extension moves out (G22).** It goes to the new public repository
   `beamfall/corvint-vscode`, frozen and unpublished (owner decision R33 (2026-11-09)). `docs/EXTENSION-ECOSYSTEM.md` and
   `docs/specs/deployment-neutral-index-platform-v0.md` are archived in place.

8. **Governance text the removals make false (G8, G23, G24, G26, G27).** Each entry says what
   changes, or that it stays until a named batch, or that it is an open owner question.
   - `AGENTS.md` line 3, "local-first, proof-carrying context compiler for software agents"
     (`AGENTS.md:3@5902b87b`). It stops being true once the CEM (G8) and context ranking (G25) leave.
     The replacement wording is an open owner question (NEW-RMV-2); this decision proposes no text.
   - Invariants 3, 4 and 5 (`AGENTS.md:23-29@0afba87a`). Invariant 3 names "learned task traces",
     invariant 4 names "trace state" and learning, and invariant 5 governs learning. B1 removes
     `internal/trace`, `internal/tracerecordrepo` and `internal/tracerepopaths`, and B7 removes
     `slot_weights.go`. Invariant 4: proposed text below, following owner decision R28/R29
     (2026-11-09). Whether invariants 3 and 5 keep their learning text is an open owner question
     (NEW-RMV-2); until it is answered they stay unchanged, and they forbid nothing the removals
     need.
   - Invariant 7: proposed text below. Whether it keeps a general "anything outside the boundary
     needs its own profile" rule is an open owner question (NEW-RMV-1).
   - Decision 0594 item 8 layer list
     (`docs/decisions/0594-refocus-on-test-selection-2026-11-09.md:79-93@e2aeb515`). The
     analyzers layer is removed with the analyzers (G26). Four layers remain: shared (0); tests and
     docs (1); release (2). The rule string in `tools/import-ratchet/areas.json`, which cites 0594
     item 8 and names analyzers, is rewritten in batch B5 when the `analyzers` area and its 31
     prefixes are deleted (31 prefixes VERIFIED at `9ec3a29d0`).
   - Decision 0595 and requirement CMS-V0-915 (both only in the integration checkout at
     `9ec3a29d0`, not on this branch). 0595's Context and Layer rule sections, the CMS-V0-915
     trace row in `docs/specs/core-bundled-apps-v0.md` (line 170 there) and
     `tools/import-ratchet/flows_layer_test.go` (lines 20 and 64-68 there) name `internal/testplan`
     as one of three tests-layer flow readers. Batch B2 removes `internal/testplan` (G24), so B2
     drops it from that reader set; the readers become `cmd/corvint` and `internal/testsource`,
     which both import `internal/appmap` at `9ec3a29d0` (VERIFIED; `cmd/corvint` also imports
     `internal/appflows`). The CMS-V0-915 requirement text names only the shared packages and does
     not change. G5 ("Reports in corvint") amends 0595 further under its own proposal, not here.
   - Decisions 0224 (full-gate receipt) and 0352 (gate-ledger per-package bound). They stay in
     force until B11. `make gate` still runs `gate-receipt-clear` and `ledger/*`
     (`Makefile:159@e0f6fc30`), and RMV-V1-024 replaces `gate-ledger` and `gate-receipt` only at
     the action cut-over (B11, RMV-V1-028). The change that lands B11 supersedes them; if B11 slips
     past 1.0, both stay for 1.0.
   - `corvint-plugin/1`. It has no Go implementation (VERIFIED at `9ec3a29d0`: no `.go` file names
     it). Invariant 7 drops it (text below). Decision 0591
     (`docs/decisions/0591-core-plus-bundled-apps-invariant-7-2026-11-06.md:52@e4584ab4`) and
     `docs/specs/core-bundled-apps-v0.md` (the Plugins row and the CMS-V0-010 row,
     `docs/specs/core-bundled-apps-v0.md:49,144@3dcdb599`) get one status note each: "Deferred
     after 1.0 with the external runner protocol (G23); no 1.0 implementation." The
     runner-selection mention (`docs/specs/runner-selection-plugins-v0.md:332@4b72313c`) is covered
     by the RSP-V0-026 to RSP-V0-032 and RSP-V0-047 deferral (item 4).
   - `make changed-checks` in the Verify section (`AGENTS.md:133-136@dfc17712`). No change. It
     runs `line-citations-check`, `unbounded-readers-check` and `test-read-scopes-check`
     (`script/changed-checks.sh` header, VERIFIED at `9ec3a29d0`), and none of the three is on
     RMV-V1-024's removal list.

### Carry-over table for the replaced ceremony (G8)

| Old mechanism | What it gave | Replacement |
|---|---|---|
| CEM bound to every change (`corvint cem begin/prepare/cite/mark/cover/verify`) | A record of which evidence covered which changed lines | `corvint affected --base` plan retained with the PR, plus CI test results |
| `cem verify-stable`, `cem-release-gate` (`Makefile:344@c3ce6204`, `cem-release-gate.yml`), the `releasecandidate` CEM readiness row | Stable CEM 1.0 wire conformance across hosts | Removed. CEM is not a 1.0 product |
| `interop/cem01-go`, `protocol/cem-1.0`, `protocol/cem-completeness-1` | Independent producer and consumer interop (V1-0014) | Removed; V1-0014 closes as superseded |
| OCM `corvint ocm prepare` (removed in 389f59ba8) | Obligation coverage for specs | Spec-driven requirements traced to tests (invariant 8) and the spec gates |
| `make dogfood-change`, `dogfood-check`, `dogfood-seal` (gone from this branch) | Proof that Corvint was used on its own change | Self-use record: the three products used on Corvint, kept with the change (see proposed AGENTS.md text) |
| Reviewer report on the CEM | Independent review | One independent review, as before |
| Frozen evaluations run at change time | Measured outcome | Per-product scripted qualification at release; narrowing measurement under its own proposal |

## Proposed `AGENTS.md` amendments

Do not edit `AGENTS.md` until this decision is accepted; then make these edits in the same change
that removes the CEM verbs (G8). The two checkouts differ: `claude/layers-integ` has the binary list
in invariant 7 (AGENTS.md lines 33 to 39 at `9ec3a29d0`); this branch does not
(`AGENTS.md:37-43@b19414b1`). The text below applies to both.

### Already done; no change proposed

- **Invariant 4, ledger part.** Already rewritten in both checkouts to the user-cache text
  (`AGENTS.md:23-29@0afba87a`, citing AFP-V0-436 to AFP-V0-439 and AFP-V0-822). It no longer names
  the self-observation or unplanned-read ledgers. Its remaining "trace state" and "learning"
  wording is handled by the proposed text below.
- **"Dogfood Corvint" section.** Already absent from both checkouts. It was retired in commit
  13d292d0f, which is on the integration history but not on `origin/main`. The section survives only
  in old local checkouts of the pre-snapshot lineage. The replacement self-use text below is new.
- **Licence paragraph.** Already reads "Corvint is Apache-2.0 across the whole repository (decision
  0586); `LICENSING.md` is the authority" in both checkouts (`AGENTS.md:176@b8bc96f4` here, line 161 at
  `9ec3a29d0`) and on `origin/main`. It is not stale. The AGPL wording exists only in the pre-snapshot
  lineage. No change, except that `LICENSING.md` "Notes on retained files" (`LICENSING.md:21-25@f4770cda`) loses the
  `LICENSE-APACHE-2.0` copies in `interop/cem01-go/`, `protocol/cem-1.0/stable/`,
  `protocol/cem-1.0/stable/repository-envelope-packet/` and `protocol/cem-completeness-1/` when
  those directories are removed. The AGPL notes for the two preregistered benchmark harnesses
  (`LICENSING.md:26-31@697df8a8`) stay.

### Invariant 4: proposed replacement text

> 4. Read commands do not mutate repository state. Outside the repository, `affected` and the
>    planning step of `test` may write one bounded, private user-cache directory,
>    `os.UserCacheDir()/corvint/affected`, of derived graph entries admitted by `AFP-V0-436` to
>    `AFP-V0-439` and the scan memo of `AFP-V0-822`; it is never repository, Git or `.corvint/`
>    state and never an input to ranking, evidence or authority.

Changes, and why: "trace state" and "trace" go because B1 removes `internal/trace`,
`internal/tracerecordrepo` and `internal/tracerepopaths`. "learning" goes because no learned
component ships in 1.0: B7 removes `slot_weights.go`, and `corvint context` ships redesigned and
non-blocking with no claim of measured benefit, its measurement running in 1.0.x (owner decision
R28/R29 (2026-11-09)). "ranking" stays because the redesigned context may still rank.

### Invariant 7: proposed replacement text

> 7. The default local product is one release of native-Go executables: the `corvint` core plus the
>    bundled apps its release manifest names; adding an executable to that manifest needs its own
>    accepted spec and decision. The product binaries are `corvint`, `corvint-docs`, `corvint-cli`,
>    the MCP servers `corvint-mcp` and `corvint-docs-mcp`, `corvint-web-flows`,
>    `corvint-test-runner` and `corvint-update`;
>    `tools/release-binary/binaries.json` is the catalog that names them, cites each one's decision
>    and says which must share a version, and each is released on its own (decision 0594 item 8,
>    `ARTIFACT-BIN-V0-001`). Every one of them is local only: no account, network dependency,
>    mutable/external database service, hosted service, embeddings, permanent daemon, UI, or broad
>    language rewrite. A bundled app may be its own Go module and executable but cannot widen that
>    boundary. It reaches the core or another app only through a published, versioned envelope,
>    never another app's internal packages; it writes no core evidence, index or trace state and
>    holds no authority the core lacks. Runner support is compiled in. The core's frozen command
>    surface works with no bundled app installed and never forwards to one. An immutable embedded
>    index encoding is derived state, not a database, and still needs the benchmark/format gate.
>    See `docs/specs/go-production-kernel-migration-v0.md` (hot-path language moves need exact
>    parity and staged rollback), decision 0081 / `docs/specs/local-admin-console-v0.md` (optional,
>    operator-started, loopback-only console with no authority; never in the default manifest), and
>    decision 0591 / `docs/specs/core-bundled-apps-v0.md` (core and bundled-app boundaries).

Changes from the current integ text, and why:
- Drops `corvint-test-validity-mcp` and "every `corvint-analyzer-*`" (G24, G26).
- Drops `corvint-plugin/1`: it has no Go implementation, and G23 drops the external protocol
  (item 8 lists where else it is named: `AGENTS.md:43@360c95d5` here, line 43 at `9ec3a29d0`).
  "Plugins" is dropped from "bundled apps and plugins" for the same reason.
- Drops the sentence "Anything outside that boundary needs its own accepted profile on the same
  transport-neutral immutable index contract" and the citation of
  `docs/specs/deployment-neutral-index-platform-v0.md`, which G22's panel recommendation archives.
  Whether to keep a general "anything outside needs its own profile" rule without the archived
  spec is an open owner question (NEW-RMV-1).
- Adds "Runner support is compiled in" (G23).
- Drops the three test-provider binaries, which retire after G23 and fold into `corvint` (owner
  decision R14 (2026-11-09)).
- `corvint-cli` and `corvint-docs` are kept as listed in integ; whether both are still separate
  binaries at the freeze is NOT_VERIFIED.

### Invariant 8: proposed replacement text

> 8. Substantive capabilities are spec-driven. Record the human-owned intent, numbered requirements,
>    non-goals, failure modes, acceptance evidence, and rollback before calling implementation done.
>    Acceptance evidence is tests, the product's scripted qualification and, where relevant, a
>    self-use record; no change evidence map is required. Generated or inferred documentation can
>    propose intent but cannot accept or govern it.

The current invariant 8 does not name CEM (VERIFIED, `AGENTS.md:54-56@7b1f3adf` here, lines 54
to 56 at `9ec3a29d0`); G8 asked for it to be
amended, so the proposed change only states what evidence replaces the CEM.

### New section after "Spec-driven development": proposed text (G8)

> ## Self-use
>
> Corvint is developed with its own three products. For a substantive change, use `corvint
> affected --base <sha>` to choose the tests to run, use the documentation product to find and
> update the docs the change makes stale, and use Corvint Tasks to claim, gate and complete the
> ticket. Record what was used and any friction with the change (the PR body or the ticket's
> progress note), and file friction as a ticket. Self-use is product evidence, never a substitute
> for the release gates: the affected plan, exhaustive `go test` and `go vet`, and one scripted
> end-to-end qualification per product.

Where the self-use record lives (PR body or ticket note) is an open question for the owner; G8
says only that self-use is "recorded". Removal batches are recorded by one build-log entry each
(owner decision R34 (2026-11-09)).

### Spec-driven development section, history line

Current text (`AGENTS.md:67-69@5e9f9ea1` here, lines 67 to 69 at `9ec3a29d0`): "Entries, CEMs, closed tickets and dated results from
before 1.0 live in the `beamfall/corvint-history` archive". Proposed: no change. It describes
history; old CEMs remain in that archive.

### Verify section: proposed edit

Remove these two lines (`AGENTS.md:125-126@3aabdc7f` here, lines 110 and 111 at `9ec3a29d0`), because
`interop/cem01-go` is removed (RMV-V1-019):

```
GOTOOLCHAIN=local go -C interop/cem01-go test -count=1 -timeout 30m ./...
GOTOOLCHAIN=local go -C interop/cem01-go vet ./...
```

### Agent routing section: proposed edit

Current text (`AGENTS.md:153@f2b7c6c9` here, line 138 at `9ec3a29d0`): "Code search excludes tracked change
evidence: `git grep -n 'TEXT' -- . ':!.agent-evidence' ':!.corvint/changes'`." Proposed, after
`.agent-evidence` and `.corvint/changes` are removed: delete the sentence.

## Consequences

- Corvint 1.0 has no CEM producer or verifier. Any external user of `corvint cem` loses it; no such
  user is known (NOT_VERIFIED).
- beamfall-apple and beamfall-android run all tests, with a named reason, for a change in a
  removed language until a provider is rebuilt after 1.0 (G26, owner decision R32 (2026-11-09)).
  This needs RMV-V1-014 first.
- `docs/specs/runner-selection-plugins-v0.md` RSP-V0-026 to RSP-V0-032 and RSP-V0-047 are
  listed as retired (the REMOVED delta in the source-removal change); the CEM, analyzer, language and test-validity specs get retirement status lines (archived in place,
  G27).
- `docs/CAPABILITIES.md` is amended where it lists removed verbs, binaries and providers (at
  `9ec3a29d0` its line 311 says "Eleven language providers"; this branch's copy differs).
- `tools/import-ratchet/areas.json` and `baseline.json` change in every removal batch that deletes a
  mapped prefix (the proposal's RMV-V1-003 item 5).

## Contradictions found while writing this

- The brief for this document said the `AGENTS.md` licence paragraph was stale against
  `LICENSING.md` and decision 0586; it is not (see above).
- The brief named the "Dogfood Corvint" section and invariant 4 as needing amendment; both are
  already changed on this branch and integ, though 13d292d0f is not yet on `origin/main`.
- 0594 item 1 cut the `context` command family, yet the Keeps line lists `context`; G25 keeps the
  verb only pending revival.
- 0594 item 3 keeps `docviews`, which has no importer (now amended, item 3).
- The product plan's perfbudget section (in `docs/specs/PRODUCT-PLAN-1.0.md`) says perfbudget holds
  no watch budget; at `9ec3a29d0`, `internal/perfbudget/budgets.json` lines 223 and 250 have
  `watch-replan` and `watch-replan-self` (this branch's copy has neither). The plan may mean a
  different watch; NOT_VERIFIED.
- `docs/CAPABILITIES.md:128@41ae4f46` says paths no language claims widen the scope to `UNKNOWN`;
  true for `corvint affected`, not for the FULL decision in `corvint test`.

## Rollback

Before acceptance: delete this file. After acceptance: a new decision restores the 0594 Keeps line
and 0592 items 1 and 2, and the removal batches are reverted by their merge commits (RMV-V1-030). Code removed earlier is recoverable from 389f59ba8^ (OCM and process families),
408ea405b^ (hosted surfaces), 841a58cbd^ (host hook adapters) and b03184699 (CEM create and verify).
`AGENTS.md` edits revert with the change that made them.

## What the owner decides

1. Accept, amend or reject items 1 to 8.
2. The `AGENTS.md` line 3 wording and whether invariants 3 and 5 keep their learning text
   (NEW-RMV-2).
3. Whether invariant 7 keeps a general "anything outside the boundary needs its own profile" rule
   without the archived deployment-neutral spec (NEW-RMV-1).
4. Where the self-use record lives.

Decided on 2026-11-09 and applied above: `docviews` (owner decision R33), the RMV-V1-014 mechanism
(owner decision R32), the context wording behind invariant 4 (owner decision R28/R29), the
provider binaries (owner decision R14).
