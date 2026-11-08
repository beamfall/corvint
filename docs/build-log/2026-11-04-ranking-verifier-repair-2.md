# Ranking verifier repair 2: activation fixture identity

Owner request: fix only the remaining host-verifier failure at
`021526670b9c16c59367e91cd22b3819495a6160`, against immutable base
`39c03198086681fb24f509fbd00b34696d3268c4`.

The host's whole `cmd/corvint` run failed only the three activation cases in
`TestCoreVerbsEmitTheFrozenProfiles`: `init summary`, `init full receipt` and
`adopt summary`. Their repository revision and derived receipt digests differed.
The raw host log is retained outside the repository at
`/home/russell/lanes/ranking.verify.log`, SHA-256
`977a3e2f053cc053c6dbc37f03a868743dbcb7e072986019435fc1164c7d27f8`.

Decision: repair `newActivationFixture` locally, following the earlier context
fixture isolation. Git author/committer environment variables override `user.*`
config, so pin both names and emails to the fixture's original `Corvint Test`
identity in its existing subprocess environment alongside the fixed dates.
Keep the shared Git helper and process identity unchanged. This preserves
CCF-V1-002's existing revision and receipt assertions; no production code,
assertion, JSON/wire contract, refusal, exit code or golden changes.
The owning spec's acceptance evidence documents execution with an exported
operator identity; no new requirement or profile is introduced.

The three goldens match BASE byte for byte. SHA-256:

| Golden under `cmd/corvint/testdata/core-freeze/` | SHA-256 |
|---|---|
| `init-summary.json` | `0f98548e71246090010c469cbb08390a49bf1801a194405d62910db9398d48b4` |
| `init-full-receipt.json` | `996703feb9793df248a51731e515f55768fb81a7a0ad9c4e637498c50df7c423` |
| `adopt-summary.json` | `1e1db1f0635536cf1f84beb29a2e3b8cf65859f795af2f044df2f506c9abdb7d` |

Rollback: revert this fixture and acceptance-evidence change together. The
operator-dependent test instability returns; product behavior stays unchanged.

Dogfood context, query and impact were used before editing and returned `ok`;
context located CCF-V1-002 in its owning spec. Their original outputs are retained
under `/home/russell/lanes/ranking-evidence/repair2-*`. The initial required
`make dogfood-change` observed existing bound prechange receipts, then refused
missing citation/intent/outcome inputs as expected. These receipts retain their
original chronology and are not recast as new BASE observations. The installed
`corvint affected --base` selected the CLI package among 149 units with scope
UNKNOWN and 82 unknowns (78 LANGUAGE_FRONTIER, four UNOWNED_DIRTY_PATH); it also
reported its existing stale-binary degradation for two earlier lane source
changes. Selection is advisory, not exhaustive coverage. Go is pinned to 1.27.3.

The applicable self-development routes are context, impact, affected, CEM/OCM
review and outcome binding. Fresh frozen ranking trials, optional provider/LSP
and live runtime qualification are not applicable to this test-only repair.
Independent review remains assigned to the orchestrator. Automatic guarding
test attribution is NOT_PRODUCED under the heavy-only Go execution rule.

Focused regression: `script/heavy.sh go test -count=1 -timeout 30m
-run TestCoreVerbsEmitTheFrozenProfiles ./cmd/corvint/` passes in 2.370 seconds
with Russell Lewis identity exported. Heavy admission waited 120 seconds under
shared-host load; total command runtime was 28.8 seconds. The exact three
previously failing cases are included, with every original assertion intact.
`make spec-requirements` passes with no generated changes and existing unrelated
spec-capacity warnings. `make line-citations-check` passes, retaining 180
pre-existing skipped non-root citations. Raw logs remain outside the repository.

The required complete existing CLI package was run without a `-run` filter:
`script/heavy.sh go test -count=1 -timeout 30m ./cmd/corvint/`.
It completed in 481.152 seconds and failed 46 top-level tests; the previous local
final run failed 47. Comparing their exact top-level failure names yields zero
added failures and only `TestCoreVerbsEmitTheFrozenProfiles` removed. The current
raw log is `repair2-cli-whole.log`, SHA-256
`0b9e99279fb6a28d53948eac901eb6af9979f9d16e987f2ca405382d566e21ae`.
Heavy admission waited 120 seconds; command runtime was 485.9 seconds.

A full local package PASS remains NOT_PRODUCED. The mandated scratch root is
inside the mode-775 `/home/russell/lanes` Git worktree. Private-executable and
work-materialization fixtures reject its writable ancestor; profile output and
outside-Git fixtures discover that ancestor repository. Other unchanged fixture
failures, including the migration-ratchet digest and JavaScript host suite,
remain in the raw log and the previous round. No frozen assertion is weakened,
no shared ancestor permissions or Git metadata are changed, and no additional
repair is claimed for those failures. Host-side whole-package qualification is
the remaining external step. CEM outcome is deliberately `failed`, preserving
the full package result alongside the passing targeted regression.
