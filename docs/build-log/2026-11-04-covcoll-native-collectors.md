# Native per-test collectors and audit measurement dependency

Human intent: lane covcoll, base 39c03198086681fb24f509fbd00b34696d3268c4,
V1-4369, V1-4370, V1-4371, V1-4375, V1-4376, V1-4664.
New obligations LPCV-V0-8200..8207 use the owner's standing approval; none claims owner read-back.

V1-4664 was investigated first. Its exported body references `internal/linecov/audit.go` at
bb590a7e8, but that path does not exist at this lane's required base. The referenced commit
also changes collection and test-window attribution. Importing that other lane's implementation
would exceed this lane's scope and alter existing behavior. The rejection rate is NOT_OBSERVED,
not zero, and the conditional >20% allowlist decision cannot be made. External blocker: integrate
the audit collector before measuring its Whole fallback rate. No allowlist is safe merely because
the executable is named git or sh: either can execute repository programs. The charter's short
V1-4664 description and its full ticket have different goals; original ticket evidence is retained
outside the repo in the lane's supplied ticket JSON.

Design: share orchestration and native report conversion across the five collector families;
reuse the existing testrunner native/platform adapters and unchanged linecov Record/Profile.
Execution and native coverage export are explicit caller inputs, with complete process-tree
capture required before accepting a record. Neither absence of counters nor successful exit
proves subprocess instrumentation. Process-isolated JaCoCo collection resets counters by a fresh
VM (no shared reset race); JAVA_TOOL_OPTIONS propagates the agent. Unqualified families always
retain whole-package/static selection. Native build flags and export documentation were checked
against JaCoCo agent docs, the Clang source-based coverage guide and the rustc coverage book.

Dogfood routes used: context, query and impact receipts bound to base, then affected before tests.
The initial prechange impact requested directories and refused unsupported-impact-path-suffix;
the same original paths were resolved to collector source files and impact rerun. Optional analyzers,
live providers, mutation services and all-feature workflows are inapplicable to this internal slice.
Independent review is reserved for the orchestrator, per the charter.

Host tool probe: command -v java dotnet cmake cargo swift finds none. Live criteria and pinned
real-repository fault qualification are PARTIAL/NOT_RUN for missing toolchains. Fixture reports
are development inputs, never presented as live output. No family is enabled for exclusion.

Verification results and digests are retained in the lane handback and outside-repo evidence.

Local verification: whole-package `internal/nativelinecov` and `internal/langplugin` tests
passed through `script/heavy.sh`; `internal/langplugin/decl` compiled with no test files.
The collector fixture matrix covers 17 runner profiles, 51 suite/isolated invocations, and
40 candidate line-selection cases across five families. This is not a live mutation kill count.
The first fixture run failed because synthetic TRX omitted the native adapter identity and used
an invalid completed counter; fixtures were corrected without changing native parsers or assertions.
Existing plugin conformance caught missing provider-free directory declarations and then their
Swift sort order; both production declarations were repaired, with original assertions unchanged.
`make spec-requirements`, collision and capacity checks passed; citation repinning moved three
existing anchors in two docs without changing their cited byte digests. Full repository gate,
Windows publication vet, native live qualification and independent review are NOT_RUN under the
lane's scoped/no-publication contract, missing tools, and orchestrator-owned review respectively.

Reproducible friction: the configured heavy lock and fallback log under
`/home/russell/bench/rt/.locks` are not writable in this sandbox. The wrapper refused before any
test ran; using its documented shared `/tmp/corvint-heavy-1000` default with lane-owned log paths
allowed tests. Host load admission waited 120 seconds before the first two test rounds.

Verification log digests (raw files outside the repository):
- `test-native-3.log`: SHA-256 `5ab2b76469541ec5b8d0b4eb362a05bcc9a268ee62e9feae2b357e820c6cf9ae`.
- `test-plugin-final2.log`: SHA-256 `60f6ac275cb705b3028d086c76cd67007c4b25c28204c042eb7256eb8daad714`.
- `test-decl-final2.log`: SHA-256 `0dd6895078ef182741b4f861bb1258575e2ed2863124d1b57ad0ca0148373ad4`.
- `spec-requirements.log`: SHA-256 `1943c1bf5c1cb16e2878ea8274356245b4520fe11e695cbc752cded6537ce034`.
- `line-citations.log`: SHA-256 `eb174638418dda7a9967afbcef04f2985b11cd7a6ed2d655370c9834fcbe78f8`.

Native instrumentation references: [JaCoCo agent](https://www.jacoco.org/jacoco/trunk/doc/agent.html),
[Clang coverage](https://clang.llvm.org/docs/SourceBasedCodeCoverage.html),
[rustc coverage](https://doc.rust-lang.org/rustc/instrument-coverage.html),
[MTP coverage](https://learn.microsoft.com/en-us/dotnet/core/testing/microsoft-testing-platform-code-coverage),
[coverlet VSTest](https://github.com/coverlet-coverage/coverlet/blob/master/Documentation/VSTestIntegration.md).

`spec-check-final.log` SHA-256: `e834b12f03b5130bf4527a55ad437133160ef6d40ef284c3ebd4b0d4ef1111f2`.

`tool-probes.json` SHA-256: `56a6f2aa139c0f50edb39149ea21b3e67c8fb9ea9ddb7ec431f70c22ba259a68`.

Final spec verification passed: generated requirements check, whole `internal/specindex`
and focused `internal/lrfrepo` obligation/enumeration checks. Its first invocation inherited
the same unwritable host lock configuration and was rerun with the existing writable overrides.
The affected plan retained UNKNOWN scope (118 selected; 81 unknown entries, including 77 language
frontiers, one declaration-only package with no selectable tests, one unowned dirty path and two
unselected binary consumers). These are not exhaustive repository validation. No missing native
qualification or unavailable audit-rate measurement is represented as passed.
