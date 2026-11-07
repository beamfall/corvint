# Bugfix independent-review repairs

The owner requested four bounded repairs against
`8162ed235b723cbac4c5561cec6685bfce060b13`, with immutable base
`cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`. Existing spec-index entries for
Live Proof-Carrying Verification, Pi Workflow and Protected Pi Runtime govern this
repair; this creates no capability or production qualification.

For LPCV-V0-7601 and LPCV-V0-954, the collector consumes the observed build proof
once and checks the corresponding Wait return as well as the process exit state.
The successful completion's output digest remains the immutable content binding;
a repeated Wait cannot hash a partial replacement at the same path and confer the
original build's full-module proof. TestUninstrumentedChild/partial-replace-wait
builds the full executable, replaces it with main-only coverage, replays the
original Wait and requires the existing uninstrumented-child refusal. An overlay
negative control using the original audit implementation fails with an unsafe
in-order record; the repaired whole package passes. This preserves the existing
command results, refusal vocabulary and index contract.

For PWV-V0-8300 and PPI-V0-8100/8101, the Linux host workflow installs npm 12.2.1,
the exact Bun 1.3.11 Linux x64 binary package and the locked protected SDK before
portable tests. The binary package avoids relying on an npm lifecycle installer.
The SDK's tests resolve the installed SDK from import.meta.url. The whole npm
entry point now passes from the package directory, including bundled highlighter,
frontmatter and HTML-export assertions. The exact CI Bun install was exercised
locally and its executable reports 1.3.11. Hosted CI execution remains NOT_OBSERVED;
Darwin protected-image qualification is separate.

For PWV-V0-8300/8101, the MCP registration test checks hostAvailable before it
creates fixtures. A new child-runner regression uses a nonexistent Pi executable
and an unusable temporary root: optional qualification must give its named skip,
while required qualification must fail with the prerequisite reason before any
fixture creation. Installed-tool failures and the exact 0.99.1 assertion remain.
The regression removes inherited NODE_TEST_CONTEXT and explicitly requests TAP;
the initial test run's empty child output was retained, then corrected without
weakening its assertions.

For LPCV-V0-8301/979, the CDP transport records its terminal closed state before
rejecting pending requests. Every later send rejects immediately with the existing
connection-closed text. The real-hook fixture closes with the first script source
request pending and a second script still to retrieve. It requires prompt take
completion, two source-failure notes and exactly one wire request. The original
hook times out in the fresh negative-control fixture. No coverage wire shape or
existing assertion changes.

Used: the original pre-change context/query/impact captures, affected advice,
whole touched Go packages, both npm suites, the existing CEM rebind/check route
and native reviewer views. The installed affected binary reports itself stale;
its initial plan still selects both touched Go packages, with scope UNKNOWN and
88 retained unknowns. This advice is not complete coverage. Existing pre-change
binding names the earlier lane base and is not relabelled as fresh evidence for
the merged base. No fresh pre-change capture is authorized in this repair.
Ranking, learning, mutation campaigns and repository-wide gates are not applicable
to these bounded repairs. Independent review belongs to the orchestrator.

Raw logs and controls stay under /home/russell/lanes/bugfix-evidence/review-fix-*.
Observed final check logs (all exit 0):

| Check | Raw log SHA-256 |
| --- | --- |
| Whole internal/linecov; 20.352 s | `929518d86861bb76a4b70c56bd8fe1322ce087b9b78b7f7187c317f1d57df512` |
| Whole internal/browsercov including live Playwright and Cypress; 52.357 s | `956b92be574c548216a60728598d247110752b2d45710a96fa02a10581aff7ec` |
| Pi npm suite; 159 pass, 19 named skips, 0 fail | `c556b3d902cd1abe48e51b0c8de855fc20df4ab4d66e2093dcbf614dfd4457ee` |
| Protected Pi npm suite; 26 pass, 2 Darwin-only skips, 0 fail | `915f360a4959317ef9a310a02ff103b72cc65dc3e49df37af063a5c78adfc2b5` |
| Windows vet, Core and interop | `1baaa0a5799a22d26268254d2a9d3f034b2556e34fbd8795b047e1b1316e690f` |
| Spec checks including whole internal/specindex and focused internal/lrfrepo | `de717c9142c6f7c545d51fd07bfdc973f606920fd64e0c6d008a5ec1c8b883a2` |
| Line citations, host versions, fixture writes, read scopes and CI least privilege | `1eae64a82d79401ade930440f2e02dc0cc013aeb263ead21195b531a163c8a42` |

Negative-control log SHA-256 values:
- `linecov-control`: `eb13d22c66254c80581ccb2949f73eee9452707976dc81a433d4dc187cb3e276` (expected exit 1).
- `browser-control-fresh`: `1e76a3d5f4cee89b645fc689fc8f191ca98b1171d3ab775c4d7a0aa2a9431d39` (expected exit 1).

The first live-browser attempt failed because Cypress wrote the sandbox's read-only
/home/russell/.config/Cypress/cy/production/cache. A writable XDG_CONFIG_HOME fixed
that; one retry inadvertently changed heavy.sh's default config lookup as well.
That run is not the retained semaphore qualification. The final whole-package
run explicitly names the original HEAVY_CONFIG and uses the shared configured
lock directory without overriding HEAVY_LOCK_DIR. A reused negative-control output
directory also failed its earlier note-count assertion; the retained fresh control
instead fails on the intended source-retrieval timeout. These intermediate failed
observations remain in external evidence. No security check or assertion was
relaxed. Citation repinning moved three unchanged spans in two documents; the
second invocation found no further moves, and line-citations-check passes.

Rollback reverts these four repairs and their spec clarifications together,
then rebinds the CEM; the named failures return. No release, hosted qualification,
Tasks completion or publication is claimed. Prior whole-CLI sandbox failures
remain outside this repair and await host verification. Stop after the CEM is
rebound and dogfood-check passes, without seal or native task writes.
