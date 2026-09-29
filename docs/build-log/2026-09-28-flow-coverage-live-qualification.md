# Frozen flow coverage qualification

Issue340 source `bbe7abd904b7` was compiled into the native compiler and provider. Subsequent
`0b1164cb` changed only six shifted documentation citations and the obsolete tool count; the
requirements and line-citation checks then passed. No source changed during the browser experiments.
The two disposable fixtures ran Node22.23.2, Playwright1.63.0 and bundled Chromium1243, with
an explicitly unnamed project and configured/observed retries0.

- Missing case: source `885e4a86af6330ef32d500c3196c88902f24fe85`, committed coverage inputs
  `7b94623f93ce10c96bf1f50647bbbe7ed433b92e`; PROVEN/MISSING_TEST, exit1.
- Complete case: source `eda2406bb7e8ac344b1ef0a42021412ac3a837f6`, committed coverage inputs
  `96ed962f37e7635ab45a8fb031aced57e7045caa`; PROVEN/PROVEN, exit0.
- Both wrote eight Markdown pages, matched byte-for-byte read-only checks and refused a second
  write with exit2. Original receipt, generated override, reporter, package/lock and served-build
  bytes were retained and bound through immutable path mappings.
- Each real SIGTERM experiment observed eight runner/browser processes and zero survivors.
  The fixture-owned loopback server was closed by its EXIT/INT/TERM cleanup handler.
- Provider SHA256: `4074b4a467ec47980fc8cc4ad786213f24541a94585757f7161f206ce23b17a0`.
  Missing receipt SHA256: `5905768ce0d43e20ddcf54f437fa3727f5e6426032151d9e0a5c8dec47222171`.
  Complete receipt SHA256: `61f5b56eb12cec94e7e39bdef96242931a8ba767078760da446b96017a5b48d7`.

Independent review repaired complete-inventory freshness, cross-context control pairing, exact
source-bound exclusions, proposed intent and entry validation. A second focused correction binds
comparable history to actual mapped bytes: metadata-only source/test commits and app relabels cannot
hide failed manual runs. Changed build/test bytes can requalify while historical observations remain
visible. Seven history cases passed in20.439s; the earlier regression and failed citation check are
retained, not overwritten by final success.

| Requirement | Acceptance evidence |
|---|---|
| AFU-V1-049 | InventoryFreshness and DenominatorOmissions regressions; complete two-variation fixture |
| AFU-V1-050 | RetryAndInfrastructure, GroupHistoryAndPaging, ActualRetryAndAPI; original live pass/control receipts |
| AFU-V1-051 | ExclusionAndEntryAuthority, StagedAndCitedSource; current/historical byte-group regressions |
| AFU-V1-052 | CLI, MCPParity, DefaultCoverageAbsent; native exit1/exit0/exit2 witnesses |
| AFU-V1-053 | BoundProofAndWriteback; native eight-page write/check/no-clobber witnesses |
| PWP-V3-007 | ProfileAttemptEmptyProject; both actual unnamed-project browser runs and interruptions |

Test names above share the `TestFlowCoverage` prefix except `TestDefaultCoverageAbsent` and
`TestProfileAttemptEmptyProject`. Enrolled terminal checks and native completion retain their own
post-commit evidence under the worktree Git directory, `corvint/adoption-340`; this entry does not
substitute for those receipts. Repository-wide gate remains NOT_RUN under scoped-issue policy.
The base OCM cannot assess these newly introduced requirements; unassessed older requirements remain
explicit rather than presenting the entire AFU or provider capability as newly complete.

The fixtures establish bounded local browser evidence, not external migration parity. PROVEN binds
immutable bytes and caller-declared application identity; deployment attestation, clean execution-tree
observation and negative-control assertion localization remain unavailable. API_PROVEN is covered by
unit admission fixtures, not a newly qualified live API application. No ETS exclusion authority is
added. Safe rollback removes this additive compiler/tool and regenerates documentation from retained
source-generation data; accepted intents and older evidence remain unchanged.
