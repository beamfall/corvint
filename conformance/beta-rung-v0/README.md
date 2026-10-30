# Beta rung V0 — per-command admission record

Authority: `docs/specs/go-production-kernel-migration-v0.md` `GPK-V0-042`, ratified by
`docs/decisions/0007-omnibus-ratification-2026-08-29.md` D1.

The machine-readable record is `internal/betarung/admissions.json`. It lives beside the code because
`corvint` must resolve its own compatibility label at runtime and `go:embed` cannot reach outside a
package directory; this file is the narrative, not a second copy. The record is the authority — if
the two disagree, the record is right and this file is stale.

## What the rung is

`GPK-V0-042` inserts a beta rung into `GPK-V0-023`'s fixed rollout order, between opt-in candidate
use and packaging Go as `corvint`. Three properties make it narrow, and this record keeps all three:

- **Install identity is unchanged.** A beta command is still `corvint`. `GPK-V0-015`'s prohibition
  on installing the candidate as `corvint` is unweakened, and nothing here creates `cmd/corvint`.
- **Admission is per command and per integration surface, never global.** The record keys every
  admission by both, and `internal/betarung` refuses a record whose admission names no surface.
- **The bar is exactly three named evidence sources**, with no additional class invented and no
  weaker one accepted: the `GPK-V0-017` criteria, the W11 reproducible-build proof, and the W12
  dogfood and rollback runs.

## The label

`internal/betarung` defines `FALLBACK`, `BETA`, and `FULL`, and resolves one label per
`(command, surface)` pair. An unadmitted pair keeps its prior label, which `GPK-V0-023` pins at
`FALLBACK` for a partial slice.

**Where the label is stated: this record, not the harness receipt.** `GPK-V0-042` says *Reports* MUST
state the label, and ties admission to "the shape `GPK-V0-026` already requires" — and `GPK-V0-026`'s
"Reports" are promotion reports, not receipts. `admissions.json` is that report: it states the label
per command and per surface, and `Record.Label(command, surface)` reads it.

The harness receipt's `support` field is deliberately **not** wired to it, and the reason is worth
recording because it is a wall rather than a preference. That field is byte-compared against the
oracle; the oracle emits `"support": "FALLBACK"` unconditionally (`src/context_corvint_harness.py`
line 551 at `c0d57181`, the last revision before `54735d98` removed the oracle); under `GPK-V0-033`
the oracle is the only admissible source of expected bytes; and `src/**` is frozen until W15. A
receipt asserting `BETA` could therefore have no oracle-authored expectation for those bytes at all.
Plumbing the resolver into
`internal/gokernel/harness.go` was tried and reverted: it went beyond what the clause asks, and
reverting it leaves the oracle-compared receipt byte-identical and needs no owner ruling.
`internal/gokernel/harness.go` is unchanged from its pre-beta-rung state.

One consequence is recorded rather than hidden: `betarung.Support`, the process-wide convenience
wrapper over `Record.Label`, currently has **no production caller**. It is the entry point for
whichever surface eventually renders the label, and the shipped-record test uses it to pin that every
surface still resolves `FALLBACK`. Nothing in the binary renders a compatibility label today.

Flipping any command to `BETA` in an oracle-compared receipt remains a divergence-register question
before it is an implementation one — see *Open question* below.

## Admission table

Verdicts are `PASS`, `FAIL`, or `NOT_RUN`. `GPK-V0-017` fixes the last one's meaning for all three
columns: *any threshold not measured is `NOT_RUN`, not waived*.

| Command | `GPK-V0-017` | W11 build | W12 dogfood + rollback | Admitted |
|---|---|---|---|---|
| `init` | NOT_RUN | PASS | NOT_RUN | no |
| `adopt` | NOT_RUN | PASS | NOT_RUN | no |
| `query` | NOT_RUN | PASS | **PASS** | no |
| `impact` | NOT_RUN | PASS | NOT_RUN | no |
| `harness` | NOT_RUN | PASS | **FAIL** | no |
| `cem` | NOT_RUN | PASS | NOT_RUN | no |
| `ocm` | NOT_RUN | PASS | NOT_RUN | no |
| `lrf` | NOT_RUN | PASS | NOT_RUN | no |
| `frontier` | NOT_RUN | PASS | NOT_RUN | no |
| `record` | NOT_RUN | PASS | NOT_RUN | no |
| `migrate-traces` | NOT_RUN | PASS | NOT_RUN | no |
| `feature` | NOT_RUN | PASS | NOT_RUN | no |
| `eval` | NOT_RUN | PASS | NOT_RUN | no |

Per-command detail, with the evidence file behind every verdict, is in the record's `evidence` and
`blockers` fields. The summary of why the columns fall the way they do:

**`GPK-V0-017` is `NOT_RUN` for every command, for one shared reason.** Criterion (d) requires that
no *other* supported command's p95 or peak resident memory regress by more than 10%. Seven supported
commands — `cem`, `ocm`, `lrf`, `frontier`, `record`, `migrate-traces`, and the undocumented `feature`
and `eval` — have never been measured at any scope, so (d) is unmeasured slice-wide and cannot be
waived. Every report under `conformance/perf-v0/results/` states this itself: all of them carry
`"outcome": "insufficient_evidence"`. Commands differ in how close they are, and the record says so:
`init`, `adopt`, and `query` clear every threshold that names their own scope; `impact`'s own scope is
invalid under DR-0004 `unequal-stdout`; `harness` is measured on four of its six events and only on
one of two corpora.

**W11 is `PASS` for every command**, read narrowly as the clause words it — *the reproducible-build
proof*. Five targets are byte-identical across two same-profile builds with distinct cold caches, the
three pinned legal files match their digests, and there are zero module requirements and zero binary
dependencies. Two limits ride along and are recorded rather than absorbed: native smoke is `PASS` on
darwin/arm64 only and `NOT_RUN` on the other four targets, so under `GPK-V0-018` there is execution
evidence on exactly one platform; and the report still carries
`pendingEvidence: ["W10-performance-GPK-V0-016-017"]`.

**W12 is `PASS` for `query` alone.** It is the only command byte-equal against the oracle on all three
legs — Corvint self-dogfood, Beamfall dogfood, and the rollback proof. `harness` retains its historical
`FAIL`: the Beamfall `user-prompt` case was 5527 oracle bytes against 4784 candidate bytes. DR-0005
and the independent DR-0013 transport defect are now repaired, so that run must be repeated rather
than reinterpreted as current evidence. Every other command is `NOT_RUN` because at least one leg
never exercised it — the two dogfoods were
restricted to read-only cases, so `cem`, `ocm`, `record`, and `migrate-traces` appear only in the
rollback run, and `lrf`, `ocm`, and `frontier` appear in none.

## Disclosure obligation, and how it is checked

`GPK-V0-042` term 5 is the contestable one and decision 0007 ratified it deliberately: an open
divergence-register entry does **not** block a command's beta admission. It continues to block that
command's retirement contribution under `GPK-V0-034` and the `GPK-V0-025` window, and a command
admitted to beta while holding an open entry **must disclose that entry**, with admission invalid
without the disclosure.

One entry is open at this revision, and the command it names discloses it without being admitted:
DR-0032 (`ocm`). DR-0034 is LANDED, closed by `IDX-SNAP-V0-018` (decision 0095), so `query` and
`eval` no longer disclose it. DR-0005 and DR-0013 are CLOSED;
the earlier entries are adjudicated and landed or known-divergent as recorded in the register.

The obligation is enforced in code, not merely documented. `internal/betarung` refuses a record in
which:

- an admitted command holds an open entry it does not disclose (the clause's own invalidity rule);
- a disclosure names an entry that is not open (a stale disclosure is how a record quietly stops
  describing reality);
- an admission carries anything but `PASS` on all three named sources;
- the record's declared open-entry set differs in either direction from the register's actual `OPEN`
  set at this revision, or attributes an open entry to a command its register `Command` line does not
  name, or omits a command that line does name (an omitted command would escape the disclosure rule
  and could state `BETA` without disclosing the entry;
  `TestCheckRegisterFailsWhenTheRegisterNamesAnUndeclaredCommand`).

`internal/betarung/repository_test.go` runs the last check against `conformance/divergence-register.md`
itself, so the claim is about the register rather than about a copy. Marking `harness` admitted with
its disclosure removed fails that test with
`harness is admitted to beta holding open divergence DR-0005 without disclosing it`.

## Findings this record makes explicit

1. **Decision 0007 D1 term 4 states that all three evidence sources are "already green". At this head
   that premise does not hold for any of the three.** `GPK-V0-017` reports 11 PASS / 0 FAIL / 3
   NOT_RUN with outcome `insufficient_evidence`; W11 is `PASS` on reproducibility while carrying
   `pendingEvidence` and smoke `NOT_RUN` on four of five targets; W12's `GPK-V0-021` self-dogfood
   closes **NOT_PROVEN** overall, `GPK-V0-022` carries `NOT_APPLICABLE` and `PARTIAL` rows, and
   `GPK-V0-024`'s fourth sentence is `NOT_PROVEN`. The rung is real and the machinery is in place;
   the evidence is not yet there to use it. The ratification brief was more careful than the decision
   record — it cited "11 PASS / 0 FAIL / 3 NOT_RUN" verbatim and then called it "already pass".

2. **`query` is the nearest miss**, and its remaining blocker is not about `query`. It clears W12
   outright and clears every `GPK-V0-017` threshold naming its own scope. What stops it is criterion
   (d)'s demand about *other* commands. Measuring p95 and peak resident for the seven unmeasured
   commands is the single cheapest action that changes this record.

3. **The W12 Corvint self-dogfood is stale against this head.** Its case 8 divergence became DR-0006,
   which has since been repaired and landed. Its `harness` `file-change` row therefore describes a
   candidate that no longer exists and must be re-run rather than re-read.

4. **DR-0010's Status line reads LANDED while its body leaves a `go-defect` open against `frontier`
   and TCQ.** `VerifyUniverse` (`internal/lrfrepo/universe.go` line 92 at `445fd751`) still passes
   `rejectPythonClaims: false`, so `internal/frontierrepo` continues to reach `pythonSyntaxValid`, and
   the entry says in its own words that this half "blocks their `PASS`". A machine reading the
   register's Status lines cannot see it. `frontier` is unadmitted here for independent reasons, so
   nothing is laundered by it today, but the register's shape lets a per-command open obligation hide
   inside a closed entry.
   **Superseded 2026-09-04 by `b5825edb`:** `VerifyUniverseWithRepository` now calls
   `verifyUniverseOCM` (`internal/lrfrepo/universe.go:119`), which takes no `rejectPythonClaims`
   flag and moves Python grammar uncertainty to the edge under `TCQ-V0-047`; the finding above
   describes `445fd751`, the head this record was written against.

## Open question, for the owner

Nothing in `GPK-V0-042` resolves how a `BETA` label reaches a receipt that is byte-compared against a
frozen oracle which can only ever emit `FALLBACK`. The three available resolutions are a
divergence-register adjudication (`python-defect` / known-divergent, with the `support` field
rewritten the way DR-0008 rewrites its seven members), a receipt-profile flag day, or confining the
label to reports that are not oracle-compared — `integrations/compatibility.json`, the per-adapter
`compatibility.json` files, and the release gate's own output. This change picks none of them. It
builds the mechanism, lands the record, and leaves the first command's flip to the ruling that
question needs.
