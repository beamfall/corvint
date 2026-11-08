# Rust runner self-review repair (V1-5113, V1-4375)

This repair keeps `RSP-V0-002`, `RSP-V0-004` and `RSP-V0-005` within the
accepted runner-selection contract. It changes no default qualification,
record wire, CLI error text, committed golden or existing test assertion.
The original pre-change capture is retained; no new pre-change capture runs.

## Decisions and regression evidence

- LCOV zero counts have line granularity, not LLVM region columns. Their
  ranges now end on the same source line, so they cannot certify a complete
  unexecuted line under `LPCV-V0-952`. The unchanged selector consults the
  enclosing body. Positive line counts remain conservative executed ranges.
  `TestRSPV0005RustLCOVMatchPatternSelectsEnclosingBody` changes the pattern
  of an unexecuted match arm that would affect a call with the new value. It
  exercises actual core selection after record union, including adjacent
  zero-count lines, with synthetic core admission to reach rule (b). This
  synthetic admission does not qualify the Rust plugin.
- The lexer tracks signature angle brackets before accepting a body brace.
  Return-type const expressions, including nested generics and function types
  with return arrows, refuse the region profile. The existing signature
  refusal remains unchanged. `TestRSPV0005RustReturnTypeConstExpressionRefused`
  covers multiline return-type expressions whose value changes type size.
- Nextest's testcase `kind` is decoded as optional: absent/null supports older
  listings; `test` is admitted; unsupported strings/types refuse discovery.
  `TestRSPV0002RustNextestNativeCaseKind` uses the retained native-format
  `testdata/rust-nextest-list.json` fixture. It is a schema-representative
  fixture, not evidence of a live nextest run. Schema authority:
  https://github.com/nextest-rs/nextest/blob/main/nextest-metadata/src/test_list.rs.
- Collection accepts a copied, core-captured compiler environment in the
  internal Go factory options. Explicit encoded flags take precedence over
  plain flags, including an explicitly empty encoded value. Coverage flags
  are appended to the effective flags and pinned in the encoded environment
  so an inherited encoded value cannot suppress instrumentation. Config-only
  or uncaptured flags refuse collection with the existing unsupported-purpose
  error; core must resolve Cargo configuration before collecting. Discovery
  and ordinary runs require no flag capture. The existing fixture now captures
  an explicitly empty compiler environment. No SPI/JSON field changes.
  `TestRSPV0005RustCollectionCompilerFlagPrecedence` covers both runner types,
  plain/encoded precedence, empty encoded precedence, immutable capture and
  refusal without capture. Cargo authority:
  https://doc.rust-lang.org/cargo/reference/config.html#buildrustflags.

The first whole registry run exposed two initialization blockers on these paths:
nextest requests omitted the native builder's required executable, and the test
component decoder merged omitted optional reasons into `New`'s default missing
reasons. The requests now supply `cargo`; the fixture decodes complete components
into zero values. Existing assertions and native wire bytes are unchanged.

## Verification and scope

The existing whole packages `internal/analyzerrust` and
`internal/testrunner/registry` pass through `script/heavy.sh`, without a `-run`
filter, using the Mac semaphore configuration and an empty remote host.
The affected receipt selects both packages with conservative additive fallbacks
`PACKAGE_EMBEDS` and `IMPORT_NOT_ALREADY_IN_PACKAGE`; the overall comparison
against `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734` includes predecessor work
and remains UNKNOWN. It is not a complete impact proof.

Corvint context and affected routes were used. Context retains omitted lexical
matches; release checks are unavailable because sandbox process-group cleanup
cannot be proved. Installed Core and Tasks remain build 127; available versions
remain UNKNOWN. No verification bypass or update was attempted. Qualification,
release gates and model-backed review remain deferred to the orchestrator. No
Tasks writes, push, PR, seal, or background job is authorized in this lane.

Raw receipts and logs are outside the repository in
`/Users/russelllewis/lanes/rspt11rs-evidence/`; the external handback records
commands, final binding/check results, digests and remaining friction.
Rollback is a revert of this repair commit and its CEM binding commit; no store
migration, public contract change or qualification rollback is needed.
