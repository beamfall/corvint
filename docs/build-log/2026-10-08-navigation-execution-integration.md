# Navigation execution integrated onto main

Date: 2026-10-08
Task: V1-0252 / AFU-V1-025..029 (S5 navigation map; NEX-V0-001..007 execution profile)
Base: 0c94c66c891c1a56b1f06b17953ca4e665151501

The 2026-09-28 navigation execution candidate (`c10b676a`, `9a94bbd6`, `84fa719a`, `d1aec4eb`)
was reviewed locally but never reached main. It is merged without rewriting its commits. Main had
not changed `internal/appflows/observer.go`, `cmd/corvint-web-flows/main.go`, `script/web-flows-gate`
or `tools/web-flows` since the candidate's base, so the code merged cleanly. The only conflicts were
spec catalogue rows and the generated requirement table: main's rows (including the application
map, scenario planner and run-verified navigation work of V1-0956/0957/0959) are kept, the one
navigation-execution row and INDEX entry are re-added, and `REQUIREMENTS.tsv` is regenerated. No
requirement ID changed; NEX-V0 did not collide.

The application map packages (`internal/appmap`, `corvint flows appmap`, corpus `map_plan`) do not
import or call the navigation execution companion, and its packet and intent schemas are unchanged,
so their surfaces are untouched; `internal/appmap` tests are rerun as a non-regression check.

Local qualification at this merge, with the pinned Playwright 1.63.0 and its bundled Chromium
153.0.8010.12 (revision 1243) on darwin/arm64 and Node 22.23.3: `script/web-flows-gate` passed the
Node unit tests (including NEX-V0-003/004 policy cases), the original observer evaluation (no false
confirmation, both seeded defects detected, zero external requests, SIGINT/SIGTERM descendants
retired) and the navigation end-to-end run (compiled goal, default-read and declared-read zero
writes, origin absent/uncommitted/unlisted, background/redirect/cross-origin/WebSocket/GET-form
refusals, hidden locator cases, bounded recovery, precondition order, unknown fixture; zero external
HTTP and upgrade attempts; SIGINT/SIGTERM/timeout cleanup). The browser discovery deadline remains
the owner-approved 5000 ms; the observer timeout (1800 ms) and the safety and cleanup assertions
are unchanged. Dependencies were installed from the committed lockfile out of the local npm cache
(`npm ci --offline --ignore-scripts`); no network fetch was made.

Corvint `affected` selected `internal/appflows`, `cmd/corvint-web-flows`, `internal/appmap` and the
two changed Node tests, but not `tools/web-flows/test/e2e.mjs`, although that end-to-end run drives
the `corvint-web-flows` binary whose observer runner this merge refactors. The gate script ran it.

Main's newer gates required two integration repairs: the error-code ownership ratchet now names
the nine `navigation-*` refusal codes in the profile's contract, and `cmd/corvint-web-flows`, which
gained its first test, declares an empty test read scope (its test only exercises flag refusal).

Not claimed: owner acceptance of the NEX-V0 profile, external or hosted-CI qualification, an API
action observer, or AFU-V1-014 per-test `LOCALLY_OBSERVED` authority.
