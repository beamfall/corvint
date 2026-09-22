# Decision 0016 — the change-anchored falsifiable packet

Date: 2026-09-01. Status: accepted. Authority: repository owner, verbatim instruction "build the
change-anchored packet path next" (2026-09-01), given after the Agent Retrieval Bench runs showed
`query` retrieval losing to grep on every task type while the change-aware surfaces were the only
ones reaching gold tests.

## Scope

The instruction accepts `FPK-V0-017` in `docs/specs/falsifiable-packet-v0.md` and the matching
amendment of `FPK-V0-010`: `prove --base FULL_COMMIT_ID [--limit N] [--mutate]` compiles the
packet `impact --base` would emit for the committed range and adds one `affected-test` row per
test the affected-plan selector reaches from the range's changed paths, each a claim that the
test covers the changed path that reached it, judged by `test-kills-mutant` for Go tests under
`--mutate`. This joins the three existing change-aware pieces (range impact, the affected-plan
selector, the sandboxed mutation runner) into one document with one falsification tally.

## Explicit exclusions

No mutation runner for Python, Rust, or any language but Go; no invocation of `prove` from any
harness or hook; no attestation change; no bench-arm change (the harness's `affected` arm is a
separate slice); no promotion, cutover, merging, tagging, signing, publication, or release gate.

## Consequences

`compileProof` gains a change branch; `judgeMutation` reads either claim form; `summarizeProof`
counts affected-test results beside packet results; `prove-observe` recounts them with the rest.
