<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4729: terminal readers retain per-reason fixture proofs

Base: `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`.
Starting lane head: `6499869562f1f89c15e1d5e3a5ccb6c9d12fcfff`.
Intent: owner repair charter; AZU-V0-9900/9901/9902 refine AZU-V0-002.

## Finding and decision

The current historical replay still had 78 unknowns despite its named ratchet obligation
and declaration hints. Three terminal TypeScript readers of the modified Go source were
promoted to participation merely because the three added fixtures were unowned. This
bypassed their plugin's source-bound and fixture-closed classifications. The earlier
four-unknown measurement is superseded: its global fixture suppression was unsound and
had already been removed by review.

Retain terminal status when every unowned path changes tracked membership below a selected,
testable Go fixture host and the repository contains the ratchet. This does not remove the
paths from frontier analysis: each plugin/reason must still prove closure through the
existing FrontierReach classifier. Dynamic, bounded, unclassified, resolution-input,
foreign-fixture and independently reached reasons remain subject to their original rules.
A content-only plan, another unowned path or a host without selectable tests retains the
old participation behavior. No declaration is inferred from a fixture directory.

Two public dry-run regression callers also needed `--format json` after the mainline
DTL-V0-009 default changed to a local text summary. Only their invocation changes; every
JSON, argv, FULL/narrowed and named-obligation assertion remains intact. No frozen output,
golden, wire shape, error/exit code or stderr contract changes.

## Exact historical replay

The historical binary and final repair binary use the same clean isolated checkout of
`0c8ae2da918307490edc10dcb1b8320005ec39ee`, with parent
`28a3ff86a2c3d17662765da12cc9251c06466f5f`.

| Measurement | Historical | Final repair |
| --- | ---: | ---: |
| `test --base PARENT --dry-run --summary` execution units | 43 | 43 |
| affected selected units | 290 | 290 |
| affected unknown | 78 | 58 |
| LANGUAGE_FRONTIER | 75 | 55 |
| UNOWNED_DIRTY_PATH | 3 | 3 |

The summary has no unknown counter; the companion `affected --base PARENT` supplies it.
The repair removes 17 source-bound TypeScript import reasons and three fixture-closed
runner reasons. The 55 retained frontiers include dynamic imports, executable configs,
runtime flags, cross-language runtime edges and unclassified plugin reasons. All three
unowned fixture paths retain their declaration hint. The affected package was already
selected incidentally; its explicit must-run reason now names the ratchet and trigger.
The replay's cross-revision binary notice is retained, not presented as a fresh historical
binary. Raw artifacts remain outside the repository in `~/lanes/ratchetsel-evidence/`.

## Verification and limits

Selection used the tree binary's `affected --base 8fc0cf37642e7012f4c26671c9ed97a8706d6e09`
before tests: all three changed Go packages were selected, with three LANGUAGE_FRONTIER
and two UNSELECTED_BINARY_CONSUMER unknowns retained. All tests use heavy.sh, one package
per command, TMPDIR `~/lanes/ratchetsel.tmp` (mode 700), and no identity or lock overrides.

The graph regression fails before repair (terminal and foreign-fixture cases), and the
real TypeScript replay regression fails on an isolated copy of the starting head with only
that regression added. Both expose the unwanted source-bound frontier. Positive and
negative controls retain dynamic/reached reasons, resolution inputs, foreign content,
unbounded readers, other unowned paths, content-only behavior and a testless host.

Whole `internal/sessiontest` passes (1.639s); final whole `internal/liveverify/affected`
passes (82.416s), including the maintained ratchet, existing audits and performance tests.
Focused CLI ownership and frozen-profile checks pass (5.186s). The initial whole CLI run
failed in 282.573s: two lane tests assumed JSON plus the 45 previously retained unrelated
failure names. The two test invocations now explicitly select JSON.
The final complete CLI run fails in 290.506s with exactly those same 45 earlier unrelated
failure names, no additions or removals against the retained independent-repair run,
and no ownership or frozen-profile failure. Representative sandbox refusals (ancestor
Git discovery under lanes and `/home` UID 65534) were reproduced on the unchanged base
in earlier retained evidence; that does not prove the cause of every one of the 45.
Host qualification and independent review remain outstanding. The local outcome is failed,
not a passing CLI qualification or native ticket completion.

Content-only compatibility uses the same historical checkout with only a newline appended
to existing `tools/ci-reuse-plan/main.go`, comparing starting and final repair binaries with
`test --base 0c8ae2da918307490edc10dcb1b8320005ec39ee --dry-run --format json`.
Both stdout (11,810 bytes) and stderr (4,330 bytes) compare byte for byte with cmp. The
checkout is restored clean. This is repair parity, not a claim that mainline's intentional
local-summary default stayed JSON. The frozen-profile tests cover their original contracts.

On current mainline BASE, the shared audit returns 1,694 unowned paths from 12,406 tracked
paths: exactly the 1,694 baseline entries, zero added or stale. The CLI inventory is correct;
the ratchet additionally enforces no increase and baseline shrinkage, not zero existing
debt. Neither baseline nor CLI ownership/exit rules change.

The staged spec was regenerated with `make spec-requirements` (no generated changes).
Repinning moved one unchanged select.go citation by one line; a confirming pass has no
move. `make -s spec-requirements-check line-citations-check` passes. The first unstaged
repin excluded select.go and is superseded by these staged passes. No broad gate ran.

Rollback removes the scoped terminal-status exception and restores the redundant
source/runner cascade, without a migration. The earlier named selection, hint, shared audit
classifier, FULL and sharded-reason repairs remain. Pre-change capture is reused by owner
instruction; final binding happens once after all edits and stops after dogfood-check.
No seal, publication, Tasks write, model-backed review or repository-wide gate is run.

## Retained evidence digests

| External artifact | SHA-256 |
| --- | --- |
| `round-replay-final-affected.json` | `c3137f3458af43bdb2be5dceac0e89399ef3a2251e4b7e3e9ec5e088ac901d5a` |
| `round-replay-final-summary.txt` | `287fc53dd30ce86477d6f07bf961f457c01fc1c3b46091f6cbc52ac6f43a2242` |
| `round-main-audit.json` | `922e7b38eac5a07a837b703e607410ea0bab2d9828908a24dc8a517f9c1f142a` |
| `round-content-final.json` | `95d172ad1a59ff7f6895125783a0144cc330c442b0c25bf364f07aae781053ef` |
| `round-content-final.stderr` | `fe46f791ebbd967a2cb3b0ae2a506fcf69bf57372bdaa54b9d1bd8248442232a` |
| `round-terminal-before.log` | `0bccd89239ac01e5cfff3cc126b43e749a1f33893e695ca4883713e0b418fc3f` |
| `round-cli-terminal-before.log` | `497482da6a051eb100b16a686d6339a6829568d5179e65285d206452ad640379` |
| `round-affected-whole-final.log` | `105094645e45435ff7870ce6cf6639a723f63bf835ca2bd51bcd7e2ae779839c` |
| `round-cli-focused-final.log` | `4859d07ba61c9e60c6807625f5ce183223ceb49bae5d119b3994255f40620c56` |
| `round-sessiontest-whole.log` | `eae198057b4949685ed80e5e310255ad382eddafea040f2bd93aee19c094618f` |
| `round-spec-citations.log` | `6aeffec9b9f332d457c5b369066f7f30472fc991940ae66bd5d125db1c0464ec` |
| `round-cli-whole-final.log` | `b06afaf7889791431acba49982a8514693b4fbd7ddf8be780a4fd26a025767a1` |
