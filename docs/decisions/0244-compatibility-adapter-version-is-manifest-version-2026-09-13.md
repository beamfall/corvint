# Decision 0244 — compatibility records: the adapter version is the package manifest version

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Every `AHI-010` compatibility record named adapter `0.1.0`: the four rows of
`integrations/compatibility.json`, the Claude Code and Codex `compatibility.json` declarations, the
Gemini CLI `compatibility.json`, and OpenCode's `package.json` `corvintIntegration.adapterVersion`.
Meanwhile the `AHI-020` manifests had advanced to claude-code `0.1.6`, codex
`0.1.0+codex.20260913082622`, gemini-cli `0.1.3` and opencode `0.1.2`. No clause said what
"adapter version" meant, and the existing `AHI-010` test only bound each row to its shipped
declaration, so both sides could stay stale together.

The call:

- The adapter version identifies the host package the record describes. It MUST equal that
  package's `AHI-020` manifest version, compared as an exact string. For Codex the
  `+codex.<timestamp>` build metadata is part of the string. Semantic-version precedence ignores
  build metadata, but the timestamp is how the Codex package advances (`AHI-020`). Dropping it would
  let one record name every Codex build.
- Host-version evidence from a native host validator ran against one package build, and a later
  bump does not re-validate it. Changing `0.1.0` to the current version must not silently claim that
  the current build was validated. The build a run exercised is therefore recorded separately:
  `hostVersionEvidenceAdapterVersion` on a matrix row whose evidence exercised a package,
  `lastValidation.adapterVersion` in the Claude Code declaration, and
  `host.staticallyValidatedAdapterVersion` in the Codex declaration. The value is `unknown` when no
  committed evidence ties the run to a build (invariant 2).
  - Claude Code 2.1.267: `0.1.4`. The only documented run is the 2026-09-12 first-user walkthrough
    of `5561b5bf` (`docs/BUILD-LOG.md`), whose plugin manifest was `0.1.4`. The record's
    `lastValidation.date` says 2026-09-13, the day `f71bc8f1` recorded the run.
  - Codex 0.149.0: `unknown`. The 2026-08-23 validation (`docs/build-log/2026-08-23.md`) predates the
    squashed history, and no record kept the build's timestamp.
  - Gemini CLI 0.54.0: `0.1.0`. Both the 2026-08-23 run and the 2026-09-12 walkthrough ran against a
    `0.1.0` extension.
  - OpenCode carries no field. Its evidence observed the host binary with the plugin not installed.
  Gemini CLI's and OpenCode's shipped declarations list no tested host versions, so they need no
  field.
- `support`/`status` stays `FALLBACK` and conformance stays `NOT_RUN`. Under this spec an untested
  release is `FALLBACK`, so both remain true for the new builds.

Implementation. Three declarations live inside their packages' shipped sets, so republishing them
is a content change under `AHI-020`. The same commit bumps claude-code to `0.1.7` (plugin and
marketplace entry), codex to `0.1.0+codex.20260913091153`, and gemini-cli to `0.1.4`. OpenCode's
declaration is in `package.json`, outside its shipped `files`, so it stays `0.1.2`. The records now
name `0.1.7`, `0.1.0+codex.20260913091153`, `0.1.4` and `0.1.2`. The `AHI-010` test in
`integrations/host-adapters.test.mjs` also asserts each row's adapter version equals its package
manifest version. From now on, every package bump restates the record in the same commit.

Consequences:

- With the new assertion and the old records, `TestHostAdapterJavaScriptHosts` failed:
  `claude-code: adapter version must equal the package manifest version`, expected `0.1.6`, actual
  `0.1.0`.
- A package change that forgets the record now fails that test instead of drifting unnoticed.

Rollback: revert the commit. That restores the `0.1.0` records, the prior manifest versions, the
test and `AHI-010` without the clause, and brings back the fixes entry.
