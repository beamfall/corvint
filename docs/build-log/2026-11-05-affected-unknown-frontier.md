# Affected unknown frontier locality — 2026-11-05

Ticket: V1-4679. Requirement: AFP-V0-2600 in
`docs/specs/affected-plan-selection-v0.md`, amending AFP-V0-511's fixture rule.
Base: `28a3ff86a2c3d17662765da12cc9251c06466f5f`.

## Finding and decision

The installed binary reports `self-binary-stale`, naming build commit `d436dff76de1`.
On PR #100's original six-path revision it selects 82 units, scope UNKNOWN, with
LANGUAGE_FRONTIER=79 and UNOWNED_DIRTY_PATH=1. This reproduces the reported cascade
(the charter's earlier sample selected 81). The fetched head had since gained a seventh
path, so the six-path revision is pinned below. The current base source already includes
the AZU-V0-001 driver claimant. Rebuilding that source removes the driver-triggered
unowned-path cascade before this change; credit for that removal belongs to the existing
claimant, not AFP-V0-2600. Current CI-driver plans still retain real uncertainty.

A remaining over-bearing case is in `frontierUnknowns`: `bearsOnUnowned` treats a
located runner frontier as path-open outside the all-fixtures exception, even when
no modelled read or dependency reaches its site. A sibling data path also undoes
an otherwise valid hosted-fixture proof for the whole dirty set.

Decision: require an explicit, validated `AncestorBoundFrontiers` declaration before
using site locality. The weaker existing fixture-closed classification alone does
not prove that arbitrary runner assets are local. TypeScript opts in only its
unknown-runner reason: `classify`/`possibleRunners` derive that classification from
source imports and ancestor manifests/configuration. Ambiguous runner entries carry
display tokens and remain conservative. The graph reconstructs declarations through
the same build admission when restoring its cache; no cache/wire shape changes.

Only an actually owned, located site is eligible. Every unowned path must separately
satisfy the hosted-fixture proof or lie outside the site's directory and ancestors.
Resolution inputs, CI/build drivers, arbitrary dynamic loads, unhosted fixtures,
missing/bare/summarized sites, reached readers and dependents remain unknown.
No selected unit or existing test assertion is removed. Plain unowned data remains
UNKNOWN; declared output becomes BOUNDED only when no other uncertainty remains.
Rollback removes the optional locality proof and restores the prior cascade.

## Six real diff replays

All runs use an external `git clone --quiet --shared` of this lane, with no cache,
using `corvint-before` built from BASE and `corvint-after` built from the delivered
Go sources. The clone is at `/home/russell/lanes/affunknown-evidence/probe` because
`/var/tmp/hostrun-affunknown/` is read-only in this sandbox. It is a planning probe,
not a dogfood clone. Every invocation uses `affected --no-cache --base FULL_SHA`.
The five history samples are from the last 20 first-parent merges or their constituent
commits. There is no purely non-spec documentation commit in that window; the doc-only
sample applies the original build-log path's binary diff to its parent in the external
clone. This is explicitly a path projection, not a whole-commit measurement.

| Diff | Scope before → after | Selected before → after | Uncertainty counts before → after |
|---|---|---:|---|
| pr100 | UNKNOWN → UNKNOWN | 299 → 299 | LANGUAGE_FRONTIER=3 → LANGUAGE_FRONTIER=3 |
| doc | UNKNOWN → UNKNOWN | 249 → 249 | LANGUAGE_FRONTIER=2 → LANGUAGE_FRONTIER=2 |
| spec | UNKNOWN → UNKNOWN | 248 → 248 | LANGUAGE_FRONTIER=1 → LANGUAGE_FRONTIER=1 |
| go | UNKNOWN → UNKNOWN | 250 → 250 | LANGUAGE_FRONTIER=2 → LANGUAGE_FRONTIER=2 |
| test | UNKNOWN → UNKNOWN | 273 → 273 | LANGUAGE_FRONTIER=1 → LANGUAGE_FRONTIER=1 |
| github | UNKNOWN → UNKNOWN | 243 → 243 | LANGUAGE_FRONTIER=2 → LANGUAGE_FRONTIER=2 |

The six plans are unchanged by this lane: they contain no newly closable runner
entry. This corpus does not demonstrate reduced fallback or CI runtime. Surviving
reasons are shell inbound invocations, Go build variants and a driver parse frontier;
none is closed by inventing an edge. The regressions below demonstrate the actual
new behavior separately.

| Diff | Head (original diff source) | Base | Projection |
|---|---|---|---|
| pr100 | `2fa23fa4031d68bef2bd1116915a6dd0b99e3803` | `28a3ff86a2c3d17662765da12cc9251c06466f5f` | whole diff |
| doc | `16b3050f5854705d58fff48e00149e2175cba3ba` | `890bea5d2f94f0e6710621e2f6c9bf5f5f7f7e5f` | docs/build-log/2026-11-04-affected-zero-unknown.md |
| spec | `b51987fd051c93cea6416619b0e5345c1c92f913` | `35455784fcf29706be5fa54820389339d3600734` | whole diff |
| go | `667d6b270478aac080895df707c2acc5c1642a60` | `d400b2d738fa5a4c06fb2fdc71b9ce8809c66733` | whole diff |
| test | `7d4733a02fb14ba7a6d8eecf99d800a9ce4461d2` | `b902c359d5b04613c35021f13b5eb95840a86a13` | whole diff |
| github | `24d8c04f9f5ffaaed86759951e520cb775dc2eff` | `f13f52b1607222033b089144a744fedae0d35239` | whole diff |

## Acceptance evidence

- The sibling-data and mixed hosted-fixture/data cases of
  `TestUnownedRunnerSiteLocality_AFPV02600` fail against the original selector.
  `TestDeclaredOutputOutsideRunnerSite_AFPV02600` likewise fails: scope is UNKNOWN
  before, BOUNDED after, with zero selected tests in both; its dynamic-load control
  stays UNKNOWN. The focused before run exited 1; the focused after run exited 0.
- `TestUnboundedRunnerInputsFailClosed_AFPV02600` retains UNKNOWN for drivers,
  ancestor/root configs, missing/bare/summarized sites, unhosted fixtures, resolution
  inputs and real literal-reader/dependency edges, including their selected tests.
- `TestRunnerLocalityNeedsExplicitValidatedDeclaration_AFPV02600` preserves the
  undeclared-plugin floor, rejects invalid subset declarations, preserves the graph
  digest and exercises cache restoration.
- `TestRunnerFrontierLocality_AFPV02600` exercises the actual TypeScript classifier,
  including the mixed-data regression and resolution/source/CI negative controls.
- Both touched packages' full existing suites passed, without `-run` filters:
  `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/liveverify/affected`
  (128.296s) and the same command for `./internal/liveverify/affected/typescript`
  (5.372s). The pre-test affected receipt selects both packages. Go is go1.27.3.
- `make windows-vet` passed for both modules. `make spec-requirements` passed after
  placing the new requirement in the existing companion (the parent has 256 clauses).
  Final spec/line-citation checks and CEM bind/check are recorded in the handback.

## Evidence digests

Raw outputs, binaries and the replay script remain outside the repository under
`/home/russell/lanes/affunknown-evidence/`. SHA-256:

| Artifact | Digest |
|---|---|
| `corvint-before` | `d716aa35d6721df33ac2769247579d6038d0def98fa0b3fe30f7b6b3d5e01efa` |
| `corvint-after` | `e172985bd234713bf910d3d497514bb24a9f8def9da3376dfd29da267abf6da3` |
| `cases.json` | `3b09104b6035a22bea908cf03978a32b017ea707762dba7c41826627aecd3453` |
| `installed-pr100.json` | `f4e4f3b5e2e66d5c3bbc1fba1f7148d3dfd2ec4be3c324a1c097de0a71f0dc62` |
| `regression-before.log` | `c95a678627a2d4d9e594d7ea21293f81c1e8b3d720d6ec5b1d42395dacfe76a3` |
| `regression-after.log` | `5218c7d03dc905bf533dd510fa6b9daf306284a8b177a61a8e08b635fa794dfa` |
| `test-affected.log` | `8211acb857b9bb2db075a1f499f92853270b221884702ddc43ecdb7e12414baf` |
| `test-typescript.log` | `d5eef06c185fb17f24844b180f6cf9fc008aff3169501ef92bbbc28bca1d18c5` |
| `windows-vet.log` | `cf42d6dbe4f437c087cfa8cfa5c7cbd77da7ab7e97343d25895959723299ddda` |
| `affected-final-tests.json` | `0b5f2061cddc4e4b65778343099adaef466e1e3e54cd4597099cb71028dd21c8` |
| `prechange.log` | `01647400d199a00138c2ffb2376bb64e83a389862a0b63c8c55df0c993324d24` |
| `replay.py` | `b8e39948c8e4a61f031c998efe271290c83bdca98353b4e13253c6b8413e3b94` |

`cases.json` retains each receipt's digest as well as both summaries, exact heads/bases
and uncertainty detail sets. It is the binding for the table, not an independent
qualification result. Native pre-change context/query/impact were retained; other
self-development routes (flows, replay of external runtimes, learning and promotion)
are excluded because this changes only graph selection. Frozen evaluations, broad
selection/doc gates and independent review are NOT_RUN here by charter; the host
verifier/orchestrator owns them. No runtime or hosted CI improvement is claimed.

## Friction and retained limits

1. `corvint-tasks ticket show V1-4679` refuses with MALFORMED / journal-absent:
   the ticket is absent from this checkout's snapshot and the common journal is
   unavailable. The supplied queue snapshot and primary-checkout read also omit it.
   The lane uses the explicit charter as scope; native ticket completion is
   NOT_PRODUCED, and all Tasks writes are forbidden.
2. The requested `/var/tmp/hostrun-affunknown/` clone location is outside writable
   roots. The equivalent shared probe clone and all raw evidence use the lane's
   external evidence directory; TMPDIR is the required private lane temp directory.
3. The installed binary is stale, so using it as the source baseline overstates this
   lane's improvement. Its 79-entry cascade is retained separately from the current
   source's three-entry PR plan.
4. Adding a 257th requirement to the parent makes `make spec-requirements` refuse
   SRG-V0-740. The existing selection-precision companion now owns AFP-V0-2600.
5. The heavy wrapper waited 20s for host load 44 to drop to its configured 40 limit
   during the focused regression run. Its configured lock directory was accepted;
   no lock/load overrides were used. Darwin-only files remain unexecuted on Linux.
6. The doc-only probe's added path was untracked at its parent, so `git restore --`
   could not remove it. The replay now removes exactly that scratch path after the
   measurement; both valid before/after receipts were retained and the remaining
   four cases completed. No repository source was affected.
