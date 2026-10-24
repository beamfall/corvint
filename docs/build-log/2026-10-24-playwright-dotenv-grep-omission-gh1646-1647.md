# Playwright dotenv inputs and verified grep omissions

Date: 2026-10-24. Lane: claude/playwright-gh1646-1647 (base 368b80d43). Issues: GitHub #1646
and #1647, which are what #1641 left of `corvint affected --playwright-config`. Requirements
`AFP-V0-405..414` (`docs/specs/affected-plan-v0.md`) are accepted under the owner's standing
auto-accept (2026-10-11). They amend AFP-V0-386 and TJAA-V0-021.

## Problem

- **#1646:** a config with a top-level `require('dotenv').config()` blocked browser-executable
  resolution even when no `.env` existed. Any dotenv use counted as a writer.
- **#1647:** a regex-literal `grep`/`grepInvert` omission was never verified, because a describe or
  config tag with whitespace is only partly visible in the listing. Two details were also too
  thin:
  - the unverified detail named one unit;
  - `BINDING_MISMATCH` did not say which binding differed.

## Decisions

- **Which dotenv uses are modelled.** Only a prelude load is modelled, meaning one in the leading
  top-level statements before any other config code. Such a load runs before the config reads
  `process.env`. Its file is statically known when the argument is absent or an object literal
  with a literal `path`. `override` is accepted only as literal `false`, because dotenv releases
  before 14.1 ignore `override: true`.
- **Line grammar.** It is deliberately the intersection that the dotenv majors in use parse alike. No
  `export`, inline `#`, backslash or backtick is allowed, and nothing may follow a closing quote.
  Version-specific forms stay a frontier instead of guessing which dotenv is installed.
- **`.vault` files.** An existing `<file>.vault` is a frontier, because `DOTENV_KEY` would switch
  dotenv to decrypting it.
- **Resolution.** It is recorded beside the caller's observation (`resolved`). It never replaces
  the observation, so the plan shows both what the caller saw and what the config reads.
- **When an omission is verified.** It needs literal source tags plus listing tags drawn from
  them.
  - Exactness is a closure check. The config, the test file and the relative static import
    closure must hold only literal `@\S+` tag properties. They must also contain no construct that
    can build or reach a tag without naming it: computed keys, computed member access or writes,
    `Object`/`Reflect`/`JSON`, or a `'tag'` string.
  - The listing's explicit tags must also be among those literals. This is a consistency check
    that keeps a forged or stale listing from verifying.

## Review repairs (Codex, PR #1648)

- **Tag closure soundness.** Escaped keys, identifier escapes and `constructor`-style routes to
  `Object` keep a file inexact, as do enumerating helpers and in-place array mutators.
- **Ambiguous imports.** An import with two existing extension candidates is ambiguous.
- **Bound dependencies.** A dependency outside the bound source set (`dist/`, hidden or skipped
  trees) keeps the omission unverified, so every proof file is under the receipt's source digest.
- **Dotenv paths.** A dotenv `path` must be unchanged by path cleaning apart from one leading `./`.
- **Dotenv values.** Backslashes and backticks are now rejected anywhere in a value, matching
  AFP-V0-406.

## Evidence

Focused tests, all passing:

- `internal/liveverify/affected/typescript`:
  - `TestPlaywrightDotenv*_GH1646`
  - `TestPlaywrightGrepOmission*_GH1647`
  - `TestPlaywrightUnitListBound_GH1647`
  - `TestPlaywrightBindingMismatchNamesBinding_GH1647`
  - updated `TestPlaywrightDiscoveryGrepOmission_TJAA_V0_021`: the GitHub #790 repro is now
    verified, and a computed-tag variant stays unverified.
- `cmd/corvint` Affected/Playwright tests.

## Limits

The following are not modelled and stay a frontier with the site named:

- `dotenv/config`, `dotenv-expand`, `@dotenvx` and `process.loadEnvFile`;
- named `config` imports;
- runtime `DOTENV_CONFIG_*`.

The following are not evaluated, so the omission stays unverified:

- tags reached through a package outside the repository;
- computed patterns, titles or tags.

Receipts written before this change keep their `unverifiedUnits` until they are regenerated.
