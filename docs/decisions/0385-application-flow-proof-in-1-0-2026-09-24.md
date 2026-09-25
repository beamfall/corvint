# Decision 0385 — Application flow proof ships in 1.0: Core E2E-safe selection, companion flows

Date: 2026-09-24. Status: accepted. Authority: the repository owner asked for "full and researched
support" for issue #175 in 1.0 and delegated the design ("use experts to ensure we are not missing
any tricks"). On 2026-09-24 the owner added three jobs: mapping E2E tests to code changes, an agent
that fully understands how to navigate and use a website, and generated documentation proven
accurate by the same evidence. The owner chose the 1.0 classification below ("Split").

Six expert reviews informed the design: flow model and interop, test-evidence provenance, E2E
test-impact selection, agent website navigation, evidence-proven documentation, and an adversarial
trust and invariant reviewer. Their claims about current code were checked against `978b37b`
before use. The contract is `docs/specs/application-flow-understanding-v1.md` (`AFU-V1`). This
decision builds on decisions 0374 and 0315 and on the issue-53 adapter (`DCP-V1-027..032`), and
supersedes none of them.

## Classification (owner answer)

- **Core:** the `e2e-safe` value of `corvint affected --selection-profile` (`AFU-V1-019..024`). It
  serves the Core change-consequence job and takes the 1.0 stability promise. `strict` and
  `coverage` stay byte-identical. The parts of the model it reads (intents, links, review anchors
  and run-evidence ingest) are Core-owned. If it is not delivered and qualified when the candidate
  freezes, the candidate ships without the value rather than waiting for it.
- **Companion, qualified in 1.0:** the `corvint flows` subcommands `map`, `gaps`, `impact`,
  `navigate`, `import`, `export`, `record` and `docs`, and `corvint-mcp --tool-profile flows`. Each
  has its own Beamfall qualification and can't block the Core release candidate.
- The `corvint flows` row of `corvint-1.0-product-and-release-v1.md` moves from experimental
  (V1-0100) to companion, as `PRS-V1-010` requires.

## Design choices

1. **Model:** one `application-flow-intent/1` file per flow, in a tracked directory the caller names.
   Flow and variation IDs are stable and never regenerated. Intents compile to the issue-53 adapter
   request, so there is one reconciler, not two.
2. **Review:** "reviewed" means a review anchor, a Git commit that changed the intent file, with the
   linked target unchanged since then. Review is self-attested; Git identity fields are never read
   as authority. Scanner, import and observer output is `inferred`; per-test coverage in run
   evidence is `observed`. Neither counts as reviewed.
3. **Test identity:** stable test keys are caller-declared annotations or tags. A key derived from a
   title is `inferred`, so a rename makes the link stale instead of silently re-matching.
4. **Evidence:** `test-run-evidence/0` keeps every attempt, retry, flaky result, environment and
   fixture identity, cleanup and negative control. The authority ladder is `STATIC` < `INGESTED` <
   `LOCALLY_OBSERVED`. `EXTERNALLY_ATTESTED` is reserved and not accepted in 1.0. Static evidence is
   never shown as verified.
5. **Stability:** report N runs and their divergences under the existing DCP-V1-023/024 policy.
6. **Selection:** `e2e-safe` narrows only when the inventory matches runner discovery, no global
   path changed, and every omitted test has an exclusion proof on one of two bases: `coverage`
   (complete-tier per-test coverage disjoint from the change) or `reviewed-links` (reviewed links
   and static reach disjoint from the change, with every changed path claimed by some reviewed
   link, stated as an author attestation). Otherwise it fails closed to every discovered test with a
   named code. A learned or predictive ranking may only reorder tests. The issue-53 adapter keeps no
   narrowing authority. Ground truth comes from injected faults; the unsafe-narrowing rate must be 0
   per basis, and a basis that misses is withdrawn on its own.
7. **Navigation:** `application-navigation-map/0` serves only locators that are declared or come from
   test source, never text scraped from a page. Every action has an effect class, which observed
   non-GET traffic can only raise. An undeclared class is treated as irreversible. The caller grants
   a maximum class (default `read`) and steps above it are marked `requires-grant`. Steps carry
   readiness conditions and recovery transitions; sign-in is a precondition flow that names
   credentials by fixture ID. Disposable origins are listed in a repository-owned file.
8. **Documentation:** docs are rendered from templates with a claim sidecar. `docs --check` fails
   when a claim that was `PROVEN` stops being so, unless an unexpired waiver names it. There is no
   LLM prose in 1.0. Hand-written Markdown opts in with claim anchors.
9. **API flows:** API execution evidence is ingest-only in 1.0. There is no API traffic observer.
10. **Multiple repositories:** the behavior provider moves to a `/2` profile with a repository list in
    place of three fixed members.
11. **Hardening:** fix the `flows` output path confinement and the check-after-open FIFO read
    (`AFU-V1-036`) in the first slice.
12. **No new root verb:** everything sits under `corvint flows`, `corvint affected` or `corvint-mcp`.

## Options set aside

- **A second reconciler inside `flows`:** it would duplicate DCP-V1-031 and let the two disagree.
- **Narrowing from coverage or inferred links alone:** these miss configuration, data and
  server-side dependencies, so they may widen a selection but never prove an exclusion without
  complete declared tiers.
- **LLM-written documentation in 1.0:** its claims cannot be tied to evidence deterministically.
- **Serving live page text to agents:** it is a prompt-injection channel.
- **Everything Core:** that was the owner's alternative; it would make rc.1 wait for live navigation
  and docs qualification.

## Owner questions, decided by expert review

The owner delegated these three questions ("use experts to decide"). Each was answered by a separate
expert and the cited evidence was checked.

1. **External agent format for the navigation packet: deferred.** An agent navigation interop
   expert checked the candidates against their current published status: WebMCP, Arazzo 1.1, llms.txt
   and Playwright MCP. None is an offline map with effect classes, verified state and untrusted
   marking. WebMCP needs live page code, Arazzo covers only API calls, and llms.txt is a proposal
   without locators. `corvint.flows.navigate` already reaches any MCP agent. Revisit when a format
   reaches a stable release that carries those fields without loss, and a named consumer shows a
   task that fails through MCP and succeeds through it.
2. **Distinct reviewer: no, review is self-attested.** A review-authority expert found that Git
   author and committer fields are free text that a local, offline binary cannot verify. Checking
   them would make a basis look stronger without evidence behind it. For a solo owner whose agents
   commit under one identity, the rule would either block every review or push toward a fake
   second identity. The proposed, not yet accepted, ownership check `TCP-V0-037` takes the same
   line: blame identity is uncertainty, not authority. Revisit when a repository with two or more human committers asks for enforced
   review, or when `EXTERNALLY_ATTESTED` is accepted.
3. **Mandatory anchors in hand-written docs: no, opt-in and reported.** A docs-as-code expert found
   that a prose claim has no form a check can find mechanically, the way `path:line` citations do. A
   mandatory rule could only require one anchor per page, and that anchor would falsely cover the
   whole page. The repository's own citation gate kept legacy documents opt-in behind a ratchet
   (`script/check-line-citations.sh:191-192@4b76039b`). Unanchored documents are listed as
   `UNPROVEN` coverage (`DCP-V1-010`). Revisit when a repository asks to protect a docs path, or a
   qualification finds hand-written drift that an anchor would have caught.

## Independent review

An independent review of the first draft found that link completeness was undefined, the inventory
and global paths were unchecked, `O_EXCL` conflicted with re-rendering docs, and evidence lapsed at
every commit. The spec now reconciles the inventory with runner discovery and adds built-in global
paths. It splits exclusion into two named bases, restricts `O_EXCL` to `record` and `import`, and
carries evidence forward only when nothing it depends on changed.

## Rollback

Remove the `e2e-safe` value, the new `flows` subcommands and the MCP tool profile, and restore the
`corvint flows` classification row to experimental. Flow intents, waivers and claim sidecars are
repository data that need no migration.
