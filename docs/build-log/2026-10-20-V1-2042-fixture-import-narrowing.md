# 2026-10-20 V1-2042: advisory import-graph narrowing for test-support changes

Ticket: V1-2042 (GitHub #1294). Spec:
[`typescript-javascript-affected-adapter-v0.md`](../specs/typescript-javascript-affected-adapter-v0.md)
(`TJAA-V0-030..033`). The requirements were accepted under the owner's standing auto-accept of lane
requirements (2026-10-11) and remain experimental.

## Decision

GitHub #1294 reported a change to a shared test-support module that selected zero units and fell
back to `FULL_RELEVANT_SUITE`, because one project's config made discovery `STATIC_UNRESOLVED`.

- The fallback is kept unchanged, as AGENTS.md invariant 2 requires. The `playwright-affected/0`
  plan instead gains an optional `narrowed` member, present only when the fallback is
  `FULL_RELEVANT_SUITE`. It carries three things:
  - the statically importing Playwright units as advisory `candidates`;
  - each project's `membership`, with the `argv` of every `UNRESOLVED` project, which must still
    run in full;
  - the changed `symbols`.
- Scope, selection, exclusions and unknowns are not touched, and bounded plans are byte-identical.
- Narrowing is limited to test-side modules: every root of the module's reverse static import
  closure must be a test. End-to-end tests drive the application at runtime, so a static import
  graph cannot bound the effect of a change to application code. Such changes, the config, its
  hooks and unowned paths report `NOT_NARROWED` with a reason.
- Per-project unknowns now stay attached to their project, so one dynamic project no longer erases
  the narrowing for the others (acceptance criterion 3). A bound discovery receipt cross-checks
  each static project.
- A test-side dynamic import adds every test that statically reaches it as a candidate. A dynamic
  import in application code refuses the narrowing.
- Symbols are a syntactic statement diff against the `--base` commit, or `HEAD` when no base is
  given. The base content comes from one bounded `git cat-file --batch`. Symbols never narrow,
  because an added export can still run code at load time.

## Gaps named in the spec

- Only the opt-in Playwright profile is covered. `affected-plan/0`, other runners and other
  languages are not.
- Non-module runtime loading is not modelled: bundler globs, HTML script tags, file reads and
  worker-shared state.
- Frontiers other than a dynamic import refuse the whole narrowing.
- A multi-binding declaration is keyed by its first name.
- The recall of the narrowed set has not been measured live. That measurement is `NOT_RUN`.
