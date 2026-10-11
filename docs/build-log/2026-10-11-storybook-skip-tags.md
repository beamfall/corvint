# V1-0612 Storybook test-runner fixed skip tags

Date: 2026-10-11. Base: `10efc02d16e6069d73426e6d6abd860e71ae83ab`. New requirement `TRE-V0-044`,
proposed (V1-0612). No decision file.

## Finding

The legacy `storybook-test-runner` profile accepted a non-empty `Target` and ignored it, so a
caller's skip selection was silently dropped. The `@storybook/test-runner` 0.26.0 package source
was read from the local npm cache with its integrity verified. `--skipTags` is a variadic runner
option that is not forwarded to Jest. It sets `STORYBOOK_SKIP_TAGS`, which is split on `,`,
overrides configured `tags.skip`, and turns each matching story into a skipped test.

## Change

- A non-empty `Target` must be 1..16 distinct comma-separated tags matching
  `[A-Za-z0-9][A-Za-z0-9_-]{0,63}`. It is appended as one final fixed `--skipTags <Target>` pair.
- Every other value is refused before a plan exists.
- An empty `Target` leaves the historical argv byte-identical.

## Evidence

- `TestStorybookSkipTagsFixedArgv` failed before the change (no `--skipTags`) and passes after it.
- It covers: the historical argv; single and multiple tags; ordering after `--url`; 16 admitted tags;
  and refusals for option-like, empty, duplicate, whitespace, path, shell, `=`, newline, non-ASCII,
  65-character and 17-tag values.

## Limits

- The real Storybook 10.6.1 / test-runner 0.26.0 / Jest 30.5.2 run is NOT_RUN. Its earlier
  qualification tree was purged and reinstalling needs a network download that was not approved.
- The optional-config `module.register` startup failure therefore stands as the retained
  unsupported result, and the profile stays BLOCKED.
- Also NOT_RUN: pass/fail/tagged-skip, native report inventory, no-tests/setup failure, and browser
  descendant cleanup.
- Independent review is NOT_RUN in this lane.
- Toolchain: tests ran with local Go 1.27.2, not the 1.27.1 named in `AGENTS.md`.
