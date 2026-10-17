# V1-1581: `flows appmap refs` independent-review fixes

The independent review of PR #1052 (AMAP-V0-090..099) arrived after the PR entered the merge
queue. It reported seven findings, all confirmed by a test that fails before the fix. The wording
amendments to AMAP-V0-090, -091, -092 and -097 in `docs/specs/application-map-v0.md` are accepted
under the owner's standing auto-accept of lane requirements (2026-10-11). Delivery remains
experimental.

Fixes:

- AMAP-V0-097 total bound: the 128 MiB corpus total now counts every source path. The shared blob
  reader counts each unique blob once, but every path is tokenised. Previously 129 paths sharing
  one 1 MiB blob passed (`TestAMAPV0097DuplicateBlobsBound`).
- AMAP-V0-097 corpus exclusion: every map test path of `--root` stays out of the corpus whether or
  not it was read. Previously a test rejected for an anchor mismatch under `--source` became
  application evidence (`TestAMAPV0097UnreadTestNotCorpus`).
- AMAP-V0-091 CSS whitespace: whitespace around the attribute name and operator is skipped, so
  `[data-state = "deleted"]` checks `deleted` (`TestAMAPV0094NameLevelCheck`).
- AMAP-V0-091 numeric values: the leading-digit exclusion applies only to `#id` and `.class`, so
  `[data-status="404"]` checks `404`.
- AMAP-V0-092/095 regex label: a regular-expression value no longer enters URL-glob handling, so an
  escaped `\*` stays literal and the `regular-expression` label is kept (`TestAMAPV0092ValueModes`).
- AMAP-V0-090/096 budget: an explicit `--budget 0` is out of range rather than the default, and
  with `--full` it still conflicts (`TestAMAPV0098RefsCLI`).
- AMAP-V0-090 zero sources: the CLI no longer pre-empts this case with a generic usage error, so
  `CheckRefs` refuses it with `appmap-invalid-query`. The CLI test now asserts each refusal code.

Rollback: revert this change. The PR #1052 behavior returns with the seven defects above.
