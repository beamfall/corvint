# Unbounded reader precision — 2026-11-05

Owner intent: unbounded lane charter, 2026-11-05; no ticket yet (the orchestrator owns filing).
Base: `9db7a94169821d9b28701eaa88866aee2fec1083`.
Requirements: AZU-V0-9930/9931, on the existing AFP-V0-012 rule (d) and AFP-V0-023 declaration contract.
Intent and delivery remain experimental; independent review belongs to the orchestrator.

## Decision and limits

A fresh local working-directory value used only in direct string-literal equality/inequality
comparisons consumes no repository contents. Both Go scanners now prove this with resolved AST
bindings, preserving offsets into their original scanner input. Every other use, parse failure,
shadowed qualifier or unsupported expression keeps the locator. The proof is deliberately small:
it does not infer purity across calls, treat a temporary-looking variable name as authority, or
exempt an arbitrary Git top-level option. A closure that captures the path for a read still widens.
The two copies stay separate because the gate selector has a standard-library-only boundary.

Declarations use the existing closed JSON grammar without new members. Dynamic CLI builds retain
module inputs and conservative source subtrees; smaller build consumers name their actual build
dependency directories, grouped at the first internal directory. The checker also exposed release
readiness files, manifests, scripts and sibling Tasks source reads, which remain in their scopes.
Some fixture-only path literals therefore have conservative extra scope entries; they were not
silently discarded. No existing assertion, wire/error/exit contract, golden, core-freeze output,
ownership code, unindexed-source handling or `.github` file changed.

## Audit of the charter files

The decisions apply to whole test processes, including other test files and subprocesses, not just
the single suspicious call. “Retain” records the conservative floor where the existing confinement
contract cannot admit a bounded declaration; it is not a claim that a fixture string itself reads
checkout contents.

| Charter file | Decision and reason |
|---|---|
| `internal/liveverify/mutate/export_test.go` | Retain (c, conservative admission limit): root-climbing symlink fixtures are data, but this package executes exported cited code in a nested bubblewrap sandbox. AFP-V0-615 and `.corvint/test-nested-sandboxes.json` prohibit declaring it under the current confinement wrapper. No per-file exemption or assertion weakening. |
| `tools/test-exec-writes-check/main_test.go` | Retain (c): `run("../..")` scans tracked test source throughout the checkout. |
| `internal/doccorpus/selfcorpus/self_test.go` | Retain (c): this deliberately isolated self-use test inventories/builds committed repository context and needs the checkout's Git authority; AFP-V0-023 cannot grant `.git`. |
| `internal/postmergeworkflow/historical_test.go` | Retain (c): discovers the real checkout and clones its immutable history for replay. |
| `internal/analyzerhtmlcss/analyzer_test.go` | Retain (c): searches the current Git checkout and ancestor checkpoint repositories for a historical commit. This is not a temporary fixture Git lookup. |
| `internal/dashboard/repository/authority.go` | (a), new empty scope: Git authority calls operate on fixture repositories; the process helper builds inside this package. No checkout content outside the package is needed. |
| `internal/jstestprovider/application_attestation.go` | (a), existing scope retained: `go.mod`, `go.sum`, `internal/`. Its root parameter identifies fixture/application repositories; its existing declaration already bounds the package's tests and nested helper builds. |
| `internal/postmergehost/envelope_linux.go` | (b): `Getwd` only guards the `/product` envelope identity, now proved generally. Also (a), declare the package's real nested build inputs (`cmd/corvint`, internal source, cishard embedded inputs, module files) and `protocol/postmerge-host` scripts. Other test files still locate the checkout for those builds. |
| `conformance/release-artifact-v0/publication_status.go` | Retain (c): publication/release tests use checkout history and release/source authority, not only temporary repositories; no `.git` grant is admitted. |
| `internal/analyzerpython/analyzer_test.go` | Retain (c): discovers pinned sibling fixture workspaces starting from the current checkout and its ancestors; needs actual checkout Git discovery. |
| `internal/opencodequalification/probe.go` | (a): the probe uses its child working directory, while tests build `tools/qualify-opencode` from bounded Go dependencies and read the declared OpenCode fixture/integration inputs. |
| `conformance/dashboard-snapshot-v0/git_authority.go` | (a): authority reads fixture repositories. Tests also build `cmd/corvint-dashboard-snapshot`; the declaration includes its bounded dependencies and the secretscreen fixture. |
| `internal/tasks/boundary_test.go` | Retain (c): `TestImportDirection` walks all repository Go source. |
| `internal/companionrelease/companion_cli_contract_test.go` | (a): builds a finite family of companion commands. Declaration retains command/internal source, embedded cishard data, module files and the other release tests' readiness/runtime/integration fixtures. |
| `internal/tasks/cli/command_help.go` | (a): help examples are not reads; the package's tests use fixture stores, build the Tasks command, inspect sibling Tasks source and read the explicitly declared help/criterion/integration/helper-script inputs. No special help-text exemption. |

The nested-sandbox limit for mutate remains visible rather than claiming a proven whole-tree
read or changing an existing fixture-marker assertion. Narrowing that remaining fixture-data
heuristic needs its own admitted proof or a confinement change; `.github` is outside this charter.

## Replay

Both binaries are built from this lane base, before/after the classifier change. The after binary
is a dirty-tree experimental build, not a release. Each command is
`corvint test --base <first-parent> --dry-run --summary`; no tests execute. Each after replay
uses synthetic parent/head trees carrying the six new declarations on both sides, preserving
exactly the original merge's changed paths. Existing declarations at each historical revision
are retained. Synthetic commits exist only in the private evidence clone; they are not published.

The qualified run uses a copied, independent Git object store. An initial shared-alternate clone
could not validate immutable historical read scopes and measured 252/246/273/269 units unchanged
on both sides. Those figures reproduce the charter's wide reference range, but cannot establish
reader precision: the fail-closed declaration floor dominates. A hardlink clone failed with
`Invalid cross-device link`; a `--local --no-hardlinks` clone admitted immutable reads. The lower
baseline below is a measurement-environment correction, **not** a lane reduction. Only each row's
paired delta is attributable to this change. No Git authority or declaration validation code was
changed to obtain these numbers. Stale-binary diagnostics are retained because historic snapshots
differ from the tool source; both tools intentionally plan those same snapshots.

| Merge | Changed files | Before units | After units | Fewer units |
|---|---:|---:|---:|---:|
| `317e91437` | 13 | 112 | 111 | 1 |
| `787223ab3` | 9 | 67 | 64 | 3 |
| `64321325b` | 38 | 107 | 106 | 1 |
| `716919894` | 26 | 156 | 156 | 0 |

All four remain NARROWED. Total: 442 to 437 selections (five fewer, about 1.1%). Broad build
inputs and dependency paths still reach bounded consumers; the fourth merge changes the actual
command/internal build inputs, so it has no count improvement. This is a bounded precision result,
not a claim that the original 229 historical unbounded selections have all disappeared.

Replay bindings (full parent/head and overlay identities are retained in the digest-bound result):

- `317e914371079f5a7458dea99ccf365e2006ee52`; original parent `f53d81d8f81ea2994afd313a4062151c899c7b45`; after base `fa8933959f723d941ec035565080f24acb3251a0`; after head `8b0e0e72e1437d55664b127f11481648839c5005`.
- `787223ab357285a5ab019b9cba937381815c3e40`; original parent `463f0dd5fb58132ce532e24e143484aa23ef3713`; after base `f88520016b7783b50d56b3c541b088e5f28c5066`; after head `76df153ad1dfd8e5f134760a894bf95ca819336f`.
- `64321325b76302fc11fb5b636b45927cb099c56f`; original parent `f89db15bd8a1f92887cde4d0c33e0aa26262c2a4`; after base `43684531d5501b31fa30b040081f860cc19562ba`; after head `a67685147e8390e81803451f00d16ddda40ea656`.
- `7169198944f962d90bab5ba69d2bcd3e4d181bed`; original parent `317e914371079f5a7458dea99ccf365e2006ee52`; after base `80c7e7454664aa7fa701c1c1b01d51e0894fa558`; after head `4c8f173e6ca0f9a44f98e48e1d755c8f7c481fc4`.

## Verification and retained uncertainty

- Whole packages through `script/heavy.sh`, with `GOTOOLCHAIN=local`, `-count=1 -timeout 30m`, no
  `-run`: `./internal/liveverify/affected/golang` PASS (13.884s), `./internal/gateselect` PASS
  (3.241s), `./tools/test-read-scopes-check` PASS (4.496s). These include the existing tests.
- `make spec-requirements` PASS (including its existing OCM-bound check); staged citation repin
  found no moved citation; `make -s spec-requirements-check line-citations-check` PASS (including
  existing internal/specindex and lrfrepo checks). The 180 existing untracked-basename citation
  skips remain explicit. No gate was weakened.
- Existing read-scope checker on the actual declaration: PASS. Module build dependency listing
  corroborates its nested build scopes. Go toolchain: `go1.27.3`.
- With the comparison proof disabled and the same test source, the two scanner integration
  regressions FAIL as intended: the unrelated guard is selected/the gate scanner reports its
  locator. Restoring the exact source produces the whole-package passes above. Unit cases cover
  positive guards and negative reads, variable comparisons, aliases, closure/return escape,
  reassignment, shadowing, malformed source and mixed safe/unsafe locators.
- `corvint affected --base 9db7a94169821d9b28701eaa88866aee2fec1083` selected both changed packages
  directly and the checker by dependency. Its retained frontier is UNKNOWN: LANGUAGE_FRONTIER=1,
  UNSELECTED_BINARY_CONSUMER=2. Focused tests do not close that frontier.
- Pre-change context/query/impact receipts were recorded by `make dogfood-prechange`; the start
  `make dogfood-change` collected coordination receipts. Available feature route: affected
  selection. Learning, ranking, LSP, external interoperability and live qualification are outside
  this static classifier/declaration slice; no substitute outcome is claimed.
- NOT_RUN: repository-wide tests/vet, Windows vet (no push), `make gate`, doc-gates, selection
  baseline, `corvint test` execution and gate-affected, as directed by the lane charter. Only the
  four specifically requested dry-run replays ran. The host owns subsequent repository gates.
- NOT_PRODUCED here: independent review (orchestrator-owned), seal, publication, native task
  completion (no ticket and Tasks writes forbidden), external outcome validation, billed-token
  and source-read telemetry (NOT_OBSERVED). Final CEM binding/check is recorded in the handback.

## Evidence digests

Raw outputs, binaries and replay checkouts stay under `/home/russell/lanes/unbounded-evidence/`.
Digest entries below are SHA-256; only their numbers and digests are committed.

- `corvint-before`: `7944575e4cacaefa47791f651b7839f3d780a1e621488dfb62adf55cbc2f48aa` (58223817 bytes).
- `corvint-after`: `d70302610cdca901c8ca796538bc2d5faff1187a625aaddcd8daca29a8f6e435` (58267251 bytes).
- `replay.py`: `7a8fdf8763c871a9ce0900bda99b4a53dfb1c8859948f5aab7fb784271c39818` (3032 bytes).
- `replay-results.json`: `2ec6d10061a71ba1a48137938ce1c2f08df36d72b27ed402a10b8520da285083` (36230 bytes).
- `golang-final.log`: `d12b77b2047e0a7eb0b0b1e7c7294ef33a7fff6b80b097e0bda6f2a3129591d2` (617 bytes).
- `gateselect-final.log`: `81b05427905225ef5e6e8a3835d783e3622b0121e145f8ed7acae8411aa91f20` (599 bytes).
- `read-scopes-tests.log`: `a04cd89b8db4c2f5365a4d8a9820b038f48dc3767b4a5386f9e33a4b57d9103e` (608 bytes).
- `read-scopes-final.log`: `70a3f3b6d1214ea6e3face9b11c48d36b81fa63ef67a9dd7dddf232f70cb07dd` (27 bytes).
- `golang-red.log`: `35b874baefb513f01c235f4bb8b03475268343c026a1fe3cdaaa2c4c80d589b6` (941 bytes).
- `gateselect-red.log`: `cc9609f3ffe25fce2fcdd4b90a94e8176b530149ee51ac041927c7461875316c` (762 bytes).
- `affected-final.json`: `36da2f38ff868b1823be87a1f0beb6225d87f3c00cd1ed4444e1a03579828cca` (136899 bytes).
- `prechange.log`: `0999fc17125425707cac77c2de5e08cfe8ff762c5bd68cd40802e925359bc1b5` (414 bytes).
- `317e91437-before`: stdout `efa3ff8a861cd0ca896b5f4b9d001c52d9d787dc0284a17677653cf13d1843a9`, stderr `424a3065ec113a07656c6d9ba9793fe951b1654e72f2f2f990af4456c7460454`.
- `317e91437-after`: stdout `ce5368d208b2f626ac45104677693f66520fc5d5dedd1c6d01ad25c346b82e44`, stderr `1e1d558a6a6c969ff65b5e59d2eadd080b66706d486e7a7a83f0586564008283`.
- `787223ab3-before`: stdout `82aff45a8e5ed3d49669f3fab188ef738cf70f58db7c4d90031b70e3ef7dff98`, stderr `a785794075020f17ce0be02bb0b423647449ba37434deb7d01c32a0dbaddc86c`.
- `787223ab3-after`: stdout `84802c5697cc001bc8e50e05699524b298b46abc472c06f11733377fcfba32f7`, stderr `d9fed1f94e18e750cf7badfc24658448e34695a894e15830e8686c5b1a901503`.
- `64321325b-before`: stdout `2a9f18d5d4e3536578a53025e475bce80b35c94c0ebaea0f05a16633ce4330c1`, stderr `0e881f2368954566755c0f03396769c403c9fa656b75bcf01323e4b94813e18b`.
- `64321325b-after`: stdout `ce83c1be205986e86ba12a955c1cb48cb334e24a7fd35d62a1b3980f2d944836`, stderr `44dbcd973aac92395ad1c7874e8667064584cd9744c5a4b31dc8fb3b0e335c07`.
- `716919894-before`: stdout `d852df9c3608fd69f95d317617ef6185c1c9e088a13b6d7f100277f799df352d`, stderr `f913581204ad5e2acd621739914970e507ad1b96128a865fb3aa9f06b6476a6a`.
- `716919894-after`: stdout `e511fa5d0b57d1d91d792e42243f6d6eb49149f7232a40cdec38ee681ee2afcd`, stderr `e03a532c5b814aaa28d39a5623be68ff59a6772d9a6fa26d098831d2a33b1c32`.

- `spec-requirements-final.log`: `8b67e7dd7db31edd5e405ed656e9810eed890febe08173ddd58594cfd908979c` (2113 bytes).

- `final-static-checks.log`: `12aca9baa50f8a0ab5031201e6a919b8cbce7c91354189a8e4357825f1c12a01` (3421 bytes).

- `citations-repin-final.log`: `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` (190 bytes).

## Rollback

Revert the comparison proof and scanner call sites and remove the six new declaration entries.
This restores conservative widening without wire migration or stored-state changes.
