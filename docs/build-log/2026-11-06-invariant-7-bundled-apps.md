# 2026-11-06: invariant 7 amended for a core plus bundled apps (decision 0591)

The owner reopened D6 (Tasks as its own module and app) and D7 (the app split is a goal). The owner
then accepted the drafted invariant text as written, choosing "Accept as drafted". That acceptance
covers three things:

- the new invariant text;
- docs, flows and Tasks installing by default;
- one shared release version.

The four split criteria (easy for an LLM to work in, isolated tests, plugins where they make sense,
fast and light on memory) became measured requirements `CMS-V0-008..011` in
`docs/specs/core-bundled-apps-v0.md`.

An independent review of the draft returned CONCERNS with eight findings. Each finding was
spot-checked against the source before the draft was revised:

1. "Companion" already names optional, unfrozen surfaces (CAP-V0-700, PUB-V0, CCF-V1), so this
   decision says "bundled app" instead.
2. Additions to the manifest need a decision, and the release check refuses an entry that cites
   none.
3. The console is never in the manifest. LAC-V0-001 is amended to say so.
4. Import edges that already exist must be cut before extraction. For example, `cmd/corvint` imports
   `doccorpus` 34 times, `appflows` 9 times and `taskman` 3 times, and `command/breakage` imports
   `appflows`.
5. A frozen Core surface that never forwards, plus exec-only `NOT_INSTALLED` shims for the
   commands that do.
6. Module mechanics and the licence boundary (`CMS-V0-007`).
7. Every contract that repeated "one binary" is amended in the same change. Decision 0081 and
   decision 0540 stay as history.
8. A write ratchet for invariant 4, an interrupted-update test, and a recall-loss threshold for
   test selection.

Not yet measured: every baseline (`CMS-V0-005`, `-006`, `-008`, `-010`, `-011`) is `NOT_OBSERVED`
until the first measurement ticket runs. No code has moved.

Merge of `origin/main` at `b75053a5`: one conflicted path,
`docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md`. The resolution keeps main's `help.go:1827` pin and
this change's one-line README shift (`README.md:155-159`).
