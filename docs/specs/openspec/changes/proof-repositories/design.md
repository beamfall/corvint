# Design: proof-repositories

Status: proposed (2026-11-09). Requirement text lives in `specs/proof-repositories/spec.md`.

## Decisions

1. A selection rule file, measured against each named repository and then committed and hashed
   before any run (review release-tasks 15).
2. Green rate = successful divided by completed runs of the named test workflow on default-branch
   pushes over the 90 days before the seal, from the GitHub Actions API (review release-tasks 15).
3. A manifest per repository (named or held-out) with every pin. Source is fetched, never vendored.
4. Held-out repositories are fastapi/fastapi, solidusio/solidus and payloadcms/payload, each
   checked before freezing; a failure goes back to the owner with a proposed replacement (owner
   decision R21 (2026-11-09)). This replaces the earlier text, which drew one held-out repository
   each for Go, JS/TS, Python and Ruby from a sealed pool at RC freeze, seeded by the RC commit SHA,
   with a "deterministic draw" scenario (same pool and SHA select the same four). The held-out Go
   repository is helm/helm, treated as those three are (100 faults, the 50-PR replay), verified and
   frozen before the M5a tag; its alternate is go-task/task (decided 2026-11-09 under R42, A14).
5. Those three are withdrawn as alternates for the named set (owner decision R21 (2026-11-09)).
6. Replacement pools of at least 5 candidates each, runner stated per entry, untouched search
   recorded (review release-tasks 16).
7. Held-out sets carry N = 100 faults, block above 3 misses, and cap excluded tests at 5 percent
   (owner decision R19 (2026-11-09)).
8. Cypress is dropped from 1.0 and listed as post-1.0 (owner decision R6 (2026-11-09)).
9. Flows on the pinned Playwright repository (decided 2026-11-09 under R42, A16). Agents write the
   flow declarations on the freeCodeCamp pin; the owner accepts links per flow. Target: 10 flows
   with at least 40 accepted links, accepted in one sitting by 2026-12-04, then frozen and hashed
   into the manifest before the Playwright fault set is sealed. The acceptances are the owner's
   act. The 10 and the 40 are panel numbers; freeCodeCamp's spec count is NOT_VERIFIED.
10. Qualification mirrors (decided 2026-11-09 under R42, N4; low confidence). Seven public mirrors
    `beamfall/corvint-qual-<runner>` (`go`, `vitest`, `jest`, `playwright`, `pytest`, `rspec`,
    `minitest`) are needed by 2026-11-27. Each is a pushed copy of the pinned commit, not a GitHub
    fork, with upstream workflows disabled, issues off, the licence kept and a README line saying
    it is an unaffiliated test mirror. Creating them is the owner's act. None exists at the time of
    writing; a manifest records its mirror URL only after the mirror is created (PRR-V1-003).

## Verification of the named repositories (checked 2026-11-09)

Sources: `https://api.github.com/repos/OWNER/REPO` (licence, language, last push, archived flag),
the GitHub search API query `repo:OWNER/REPO is:pr is:merged merged:>=2025-11-09` (merged PRs in
12 months), and repository files read through `https://api.github.com/repos/OWNER/REPO/contents/PATH`.
All checks were made on 2026-11-09. Test count, full-suite time and the 90-day green rate were not
measured for any repository; each is NOT_VERIFIED and is measured by task 4 before the rule is
hashed.

| Runner | Repository | Licence | Runner evidence | Merged PRs, 12 months | Result |
| --- | --- | --- | --- | --- | --- |
| Go | prometheus/prometheus | Apache-2.0 | `go.mod`, language Go | 1475 | passes checked criteria |
| Vitest | vuejs/core | MIT | `vitest.config.ts`; `package.json` `"test": "vitest"` | 1168 | passes checked criteria |
| Jest | prettier/prettier | MIT | `jest.config.js`; `jest` 30.5.2 in `package.json` | 1344 | passes checked criteria |
| Playwright | freeCodeCamp/freeCodeCamp | BSD-3-Clause | `@playwright/test` 1.64.1; `e2e/` workspace with `*.spec.ts` files | 3528 | passes checked criteria; running-app and 30-minute criteria NOT_VERIFIED |
| pytest | pydantic/pydantic | MIT | `pytest` in `pyproject.toml`; `tests/` | 686 | passes checked criteria |
| RSpec | rubocop/rubocop | MIT | `.rspec` (`--require spec_helper`); `spec/` | 1226 | passes checked criteria |
| Minitest | rails/rails, `activesupport/` | MIT | `activesupport.gemspec:41` depends on `minitest >= 5.1`; `activesupport/test/` | 2099 (whole repository) | passes checked criteria; PR count is for the whole repository |

Alternates. Entries marked "withdrawn (R21)" name a repository that is now held out and cannot be
used as a swap.

| Runner | Alternate | Check result | Suggested swap if needed |
| --- | --- | --- | --- |
| Go | cli/cli | MIT, Go, 668 merged PRs: passes checked criteria | none needed |
| Vitest | vitejs/vite | MIT, `vitest.config.ts`, 1141: passes | none needed |
| Jest | babel/babel | MIT, 635; no `jest.config.js` at the root and `"test": "make test"`, so Jest use is NOT_VERIFIED | none until Jest use is confirmed |
| Playwright | RocketChat/Rocket.Chat | FAILS licence: GitHub reports `NOASSERTION`; `LICENSE` puts `apps/meteor/ee/` and `ee/` under a separate licence. Has `apps/meteor/playwright.config.ts`, 2806 merged | was payloadcms/payload (MIT, 2057 merged, `test:e2e` runs `playwright test`; running-app criterion NOT_VERIFIED): withdrawn (R21). No swap candidate now |
| pytest | pallets/flask | FAILS activity: BSD-3-Clause but 20 merged PRs | was fastapi/fastapi (MIT, Python, 920 merged; pytest use NOT_VERIFIED): withdrawn (R21). No swap candidate now |
| RSpec | thoughtbot/factory_bot | FAILS activity: MIT but 29 merged PRs | dependabot/dependabot-core: MIT, 1457 merged, `rspec-its` and `rubocop-rspec` in `Gemfile`, `common/spec/`. The other earlier option, solidusio/solidus (BSD-3-Clause, 258 merged, `rspec-rails` in `Gemfile`), is withdrawn (R21) |
| Minitest | jekyll/jekyll | FAILS activity: MIT but 20 merged PRs | No independent passing repository found: faker-ruby/faker (MIT, Minitest, 86 merged), heartcombo/devise (15), Shopify/liquid (26) fail activity; rack/rack reports `NOASSERTION`. Interim suggestion: another Rails component (`actionpack/` or `activemodel/`), which is not independent of the primary |

Held-out repositories against the checked criteria (same sources, 2026-11-09): fastapi/fastapi MIT,
920 merged; solidusio/solidus BSD-3-Clause, 258 merged; payloadcms/payload MIT, 2057 merged. All
three pass licence and activity on these readings. fastapi's pytest use and payload's running-app
criterion are NOT_VERIFIED; task 5 repeats the checks before freezing.

helm/helm, as recorded with decision A14 on 2026-11-09: Apache-2.0, language Go, 603 merged PRs in
12 months, 94 of the latest 100 completed `build-test` runs on `main` green, median run 365 s, and
no file on Corvint `origin/main` mentions it. Its test count is NOT_VERIFIED (227 Go files contain
`func Test`). restic/restic failed the green-rate rule (20 of 43). cli/cli stays the named Go
alternate and is not held out. Task 5 repeats the helm/helm checks before freezing.

Open points on the primary seven:

- freeCodeCamp: whether its Playwright suite drives a running application that can be started in a
  pinned image, and whether it finishes within 30 minutes, are NOT_VERIFIED. Its repository is large
  (GitHub reports 584,606 KB).
- rails/rails: the rule's PR and test counts should be read for `activesupport/` alone. The count of
  PRs touching `activesupport/` was not measured (NOT_VERIFIED).

Today's record (verified in `docs/specs/corvint-1.0-product-and-release-v1.md`): V1-0019 requires
the three Core workflows to pass on Corvint, Beamfall and at least one untouched external
repository, with cases frozen before execution (:224-226). go-chi/chi was held out for run-001 and
pinned in `benchmarks/untouched-repository-v1/` (:239). Because chi has now been run by Corvint, it
is no longer untouched under G12 and must not enter any held-out pool.

## Failure modes

- A named repository fails the rule when measured: it is swapped by rule for a repository that
  passes the hashed rule and has no Corvint run, before the freeze; the manifest reports the swap
  (R42 (panel pick) A13). Only the two cases of PRR-V1-010 go to the owner.
- A pin cannot be fetched (deleted SHA, image digest gone): the run refuses with
  `pin-unavailable`; no substitute is fetched.
- A held-out repository fails its licence, activity or untouched check before freezing: it is not
  frozen and goes back to the owner with a proposed replacement from its pool (owner decision R21
  (2026-11-09)).
- The held-out list or a pool hash does not match at run time: the run refuses.
- A held-out repository is found to have been touched after freezing: its result is void, the void
  result is recorded, and a replacement goes to the owner.
- Two identical reruns differ: the held-out result is `NON_REPRODUCIBLE` and blocks (G12).

## Acceptance evidence

- Measured rule values for each named repository, then the committed `rule.json` with hash; seven
  named manifests with measured values and sources.
- The frozen held-out list (fastapi, solidus, payload, helm) with check results and hash, and four
  pool files of at least 5 candidates each (PRR-V1-011 asks for one per held-out repository).
- The freeCodeCamp flow file hash in its manifest, with 10 flows and at least 40 accepted links
  (A16), and the seven mirror URLs once the owner has created the mirrors (N4).
- Go tests for rule-hash mismatch, pin mismatch, held-out swap refusal, freeze refusal on a failed
  check, touched-repository refusal, short pool, refusal ceiling, miss bound and the rerun
  comparison.

## Rollback

Revert the change's commits. Manifests and pools are data; deleting them returns the release to the
V1-0019 single untouched repository. Requirement IDs are retired, not reused.

## Open questions for the owner

- Decided, R42 (panel pick) A13: swaps (pytest, RSpec and others) are made by rule and reported
  (PRR-V1-010). Two cases still go to the owner when they arise: a Minitest alternate that is a
  second Rails component, and Playwright if freeCodeCamp fails with no passing candidate.
- Settled (decided 2026-11-09 under R42, A14): the held-out Go repository is helm/helm, alternate
  go-task/task. Earlier wording, superseded by A14: "whether a held-out Go repository is still
  required (G12 named four families) is not decided."
- Decided, R42 (panel pick) A12: the 50-PR replay of PROD-TEST-015 and NMS-V1-010 runs on all
  three held-out repositories (fastapi, solidus, payload), the 50 most recent merged PRs before
  each pin, with the list frozen with the manifest (review completeness F16). Settled (decided
  2026-11-09 under R42, N7): N7 is closed by A12, and A14 adds helm/helm on the same terms.
- Open for the owner to act on, not to decide: the link acceptances on the freeCodeCamp pin by
  2026-12-04 (A16) and the creation of the seven mirrors by 2026-11-27 (N4).
- Decided, R42 (panel pick) A9: held-out sets pass at up to 3 misses in 100, each published with
  its interval, filed P1 and fixed before the candidate; named sets allow zero known misses
  (PRR-V1-008).
