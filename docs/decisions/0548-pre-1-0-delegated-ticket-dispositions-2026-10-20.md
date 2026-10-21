# Decision 0548 — dispositions for 21 pre-1.0 decision tickets

Date: 2026-10-20. Status: accepted by the orchestrator under the owner's delegation. During the
pre-1.0 sweep of every open bugs, fixes, optimizations and tests ticket, the owner wrote "you can
make decisions for me on this" in chat. The delegation covers only the 21 tickets below. It is not
a standing grant. Releases, `.github` control-plane changes and visibility changes are not
delegated.

Each disposition follows the same order of preference:

1. a minimal, honest fix;
2. otherwise, a recorded narrowing (a limit, `NOT_PRODUCED`, or an abstention);
3. otherwise, a post-1.0 deferral, used only for work that needs spend, external hosts or
   owner-only runs.

No disposition weakens a gate, a fence or a product invariant.

## Build before 1.0

| Ticket | Disposition |
|---|---|
| V1-1020 | No corvint-tasks code path reads `~/Music`, so the cause is recorded as `NOT_OBSERVED`. Add a regression guard that lists the only home-relative paths the product writes. Tests that build the binary run with an isolated `HOME`. |
| V1-1321 | Amend CAL-V0-021. The claim scope is DECLARED ∪ REQUESTED, admitted through the same collision check as a CAL-V0-025 widen. A collision refuses and names the paths it rejected. The claim output reports the effective scope. |
| V1-0849 | Give each of the 12 child flakes without a disposition one of: a condition wait, a recorded load limit, or its own ticket. Do not raise any timeout. |
| V1-1256 | Add an opt-in `CORVINT_TASKS_READ_ONLY=1`. Every mutating verb then refuses with a typed code. It guards against accidental writes and is not a security boundary. Check V1-1441 for a duplicate first. |
| V1-0571 | LOD-V0 is still proposed and conflicts with itself (line 80 against line 846). The 1.0 path therefore narrows: the disputed qualification is reported as `NOT_PRODUCED`, and the producer stays unchanged until LOD-V0 is accepted. |
| V1-1472 | The legacy `verify-stable` envelope (4,096 entries, 1,024 Git operations) is intentional. Only `verify-stable-canonical` may be cited at repository scale. Add a boundary test. |
| V1-1756 | Add an explicit `--repository-only` mode. It reports `host: NOT_OBSERVED` and exits 0 when the repository is clean. With no host, the default result stays `UNKNOWN` (exit 3). |
| V1-0151 | Accept the LAC-V0-035 sub-bullet. It is already implemented and tested, so only the spec text changes. |
| V1-0301 | A binary-content refusal names the path, with no change to the refusal code. Record the limit in CEM-V1. Mode-only changes are already covered by CEM-V1-300. CEM-PILOT-001 moves to a post-1.0 profile. |
| V1-0828 | Keep the live-lease requirement. A submit after expiry refuses with a typed code and a fix line: re-claim, resubmit, then gate (reusable gates replay). |
| V1-0579 | Amend DLT-V0-009 so that native docs-only delta is `NOT_PRODUCED`, matching the interim decision of 2026-10-01. Acceptance stays PARTIAL. |
| V1-0887 | Amend the CAL-V0-224 non-goal to allow a next-name probe plus checkpoint-decode reuse. Target: 50 ms or less, flat from 3k to 13k receipts. If it misses, escalate to a sharded layout. |
| V1-0945 | Accept CCF-V1-009 (4,096 B, delivered at 3,967 B) and record the ~1.5 KB floor as a limit. Apply the CCF-V1-008 text that decision 0539 item 7 already accepted. |
| V1-1322 | A lane that has merged main forward binds against `merge-base origin/main HEAD`. `dogfood-change` refuses early and names that base, and the verifier names the bound that fired. The verifier logic itself is unchanged. |
| V1-1895 | Option B. `complete` keeps tree equality on the submitted candidate. On a merge-queue tree mismatch, the refusal names the sequence to follow after the merge. CAL-V0-017 is not loosened. |

## Close

| Ticket | Disposition |
|---|---|
| V1-0725 | Not a defect. INDEX.json contains no `\u` escapes, and SRG-V0-012/013/015/016/018 already cover regeneration and staleness. |
| V1-0145 | DCW-V0-030 is declined. The accepted DCW-V0-014 fix lines already separate stale from incomplete, and this is tested. |
| V1-0777 | Split. The untouched-repository reader becomes an acceptance line of the V1-0019 preregistration. The daily-loop-v0 half is archived. |

## Defer past 1.0

| Ticket | Reason |
|---|---|
| V1-0415 | Evaluation spend. Decision 0539 item 1 deferred it, and the grant was not renewed. |
| V1-0835 | The trial run is reserved for the owner (decision 0544). Building the cases may continue under the 0539 grant. |
| V1-0836 | Decision 0539 item 2. |

## Forks raised by sweep lanes

The same delegation covers forks that bug lanes in this sweep raise. These follow the same order of
preference.

| Ticket | Disposition |
|---|---|
| V1-2069 | Project authority bounds the reader, not a syntax heuristic. `internal/tasks/transaction` declares its read scope in `.corvint/test-read-scopes.json`, so the generic `/tickets/` literal stops counting as a reader of repository ticket files. A ticket-only diff then projects as nonSource. Literal fragment matching itself is unchanged, because narrowing it can drop real readers and their uncertainty. |
| V1-2090 | Build. When a Vitest or unprofiled receipt anchor is a failure frame, discovery may place the declaration from committed source at the receipt's bound revision. More than one candidate gives a typed absence for ambiguity. A missing match keeps `anchor-not-declaration`. |
| V1-2078 | Narrow. The lane delivered `key: if`, `<< if` and line-start `else if`. Ternary-branch openers, keywords inside regex literals and `x = if a.end?` are recorded as limits of the TSA-V0-396 reader. Files with those forms stay unbalanced and, under TSA-V0-397, block only the shared groups they could define, so the result abstains rather than guesses. |
| V1-0912 | Build. Raise the decode and capture bound from 1,000 to 16,384 entries. The existing input-byte cap still applies. Above the bound, decode refuses with the current typed code. The bound is stated in native-taskman-planning-v0 and in `--help`, and is tested on both sides. |
| V1-1091 | Keep `btime`. Treat a 1-second difference as the same boot only when the pid and the process start ticks also match, so persisted ledgers, markers and run records keep their format and need no migration. A test covers a live process whose `btime` reads 1 second apart. The runner measurement is retained as evidence. Migrating to `boot_id` is post-1.0. |
| V1-2009 | Decided under the same day's 1.0 feature-cut delegation ("you make the decision for me"). Closed as already delivered on main (cfe88b2af, 64ec6076b, 11cb37f76, 39f0427b9). Criterion 3 is narrowed: under PWP-V0-040, node and browser-source are observed only after a run, so a pre-run refusal document names only the runner field. #1270 stays a reference. |

## Rollback

To roll back, revert this file and its index row. The 21 tickets then return to open owner
questions. Any build already landed under a row above is reverted with that row's own change.
