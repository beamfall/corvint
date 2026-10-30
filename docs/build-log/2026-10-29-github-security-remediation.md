# GitHub security remediation — first slice

Owner request (2026-10-29): fix the repository security findings with subagents and
appropriate effort. Native ticket `V1-3178`; immutable base
`9477995d776ddf88d7f6a82790dd9c22d51da765`.

## Findings and decisions

The initial dashboard contained 55 Dependabot alerts in three npm lockfiles,
14 JavaScript/Python CodeQL alerts and eight secret alerts. The subsequently completed
Go scan added 88 findings, tracked separately in `V1-3179`, not fixed by this slice.

Two distinct adversarial inputs to the transcript metrics Git-write regex each exceeded
a three-second timeout. A single-pass token classifier removes its ambiguous repeated
option forms. Executable CLI regressions and a 17-command corpus retain the behavior in
`docs/LANE-ORCHESTRATION.md` section 7. It remains an approximate metrics classifier,
not a shell parser or command-authorization mechanism.

JavaScript fixtures now pass dynamic values as data instead of embedding them in source.
Malformed-envelope tests prove a valid baseline and the intended duplicate-key or newline
corruption. The browser fixture uses constant script source and a validated numeric port.
CodeQL alert 7 follows `process.execPath` into argv-only `spawn` without shell evaluation;
explicit `shell: false` and hostile executable/argument regressions document that boundary.
Independent review supports a false-positive disposition if a new scan still reports it.

VS Code transitive updates fit upstream ranges. The interactive fixture advances Vitest
and its mocker to 5.0.3 with matching qualification pins. Protected Pi retains SDK 0.85.1.
Its published shrinkwrap prevents older npm clients from honoring the patched override;
exact npm 12.2.1, its documented compatible Node range and nested install strategy admit
Undici 8.12.2 and brace-expansion 5.0.12 while preserving the build's fixed dependency paths.
Clean installation, the resolved tree and audits verify the closure.

Rebuilding exposed Bun resolving the substituted image module's worker import against the
SDK directory. The independently reviewed repair maps only the exact import from the exact
SDK importer to the fixed repository worker. Real Bun positive and wrong-importer negative
builds verify this boundary; there is no generic fallback or runtime module discovery.

The native protected test also expected context injection contrary to `PWV-V0-016`, which
requires the embedded 0.85.1 host to visibly refuse the ordinary 0.99.1 tuple. The corrected
oracle uses both version constants, preserves the supported-version positive assertion,
and requires no context plus the actual typed RPC warning when unsupported. The protected
TUI driver requires a fresh refusal notice alongside its response token and settlement
delay for each prompt phase. Hostile resources, session replacement, image-worker and
startup-injection controls remain; final image-tool requests also check context behavior.

All eight secret values were independently compared against every reported source location
and verified as deterministic test fixtures. Readback found them already resolved as
`used_in_tests`; duplicate comment updates were refused. No new resolution by this change,
credential revocation, fixture removal or history rewrite is claimed.

## Evidence and limits

Baseline npm audits reported 8, 3 and 4 vulnerable packages; all three updated lockfiles
report zero. Focused evidence includes VS Code lint/typecheck and 91 tests, Vitest's fixture,
Python CLI regressions, JavaScript process/envelope tests, actual Bun controls, the rebuilt
protected image tests, and focused TUI driver tests. The final candidate's dogfood checks
retain exact-target observations, CEM/OCM reports and independent review. Windows vet is
required for the Go TUI driver. The owner's scoped-work preference does not select `make gate`.

The official Node 22.23.2 archive matched the existing pinned SHA-256 before execution.
The image still declares `EXPERIMENTAL_UNQUALIFIED`; source/image tests do not establish
protected installation, independent admission or formal FULL support. The dependency
closure invalidates earlier image identity and qualification under `PPI-V0-009`.

Corvint pre-change context and impact were used, retaining lexical omissions and incomplete
reverse-import coverage. Core and Tasks report local build 790; the official release list
returned no releases, so updater freshness remains UNKNOWN and no update is claimed.
Native receipt audits observed structural consistency and projection agreement; historical
acceptance, actor authentication, liveness and runtime qualification remain NOT_OBSERVED,
and semantic coverage remains UNKNOWN. Raw local evidence is retained under
`/private/tmp/corvint-security-20261029/`; behavioral proof is the executable regression suite.

Routing: Sol/medium for dependencies and synthetic-fixture verification, Astra/high for
CodeQL implementation and Go analysis, Sol/high for independent review. Billing/cache
telemetry is NOT_OBSERVED. A separate Go boundary review stopped on a provider hardware-key
authentication error and stays pending, without rerouting around that requirement.

## Rollback

Reverting this slice restores the previous dependency closure and its known vulnerabilities.
Preserve prior manifests and never reuse image admission across either closure. The optional
protected image remains uninstalled and unpromoted. Unrelated primary-checkout work and the
native backlog remain intact. The operator admission pause was restored immediately after
the approved TUI scope reservation.
