# Six experimental change-evidence workflows

These owner-requested additions are available in the source build. Their technical contracts
remain proposed or experimental; the qualification below does not promote Core 1.0 or close
the broader release backlog. Build with `make build` and use that candidate executable for
new flags. Installed Core 1.0.0-rc.1 build 163 predates these additions.

## Review one exact change

```sh
corvint cem report --map MAP --format json --ocm OCM \
  --expected-base FULL_BASE --target FULL_TARGET
```

The JSON hunk denominator comes from the original patch. Each hunk carries its immutable
source location, cited basis, explicitly linked requirements, supplied coverage/discrimination
witnesses and unknowns. Test claims and structural links do not mean tests ran or passed.
Omit `--ocm` to retain an explicit missing-obligation view. Invalid OCM inputs remain visible
and produce no joins. JSON output is read-only stdout; it rejects `--output`.

The optional standalone `corvint-console` `/chain` pane follows a hunk to original pinned
source, requirement/citation detail and test/gap state. Start that loopback console explicitly
when useful, and stop the owned process afterward. See [CEM pilot kit](specs/cem-pilot-kit.md)
and [console contract](specs/local-admin-console-v0.md).

## Attack tests for changed Go conditions

```sh
corvint prove --base FULL_BASE --mutate --attack-tests
```

This explicit committed-range mode continues through at most eight selected mutants per row.
It exposes survivors even after an earlier mutant was killed. A legacy `PASS` may coexist with
survivors. `COMPLETED` describes this bounded selected set; `PARTIAL` and `NOT_RUN` preserve
budget, baseline, sandbox and unsupported-language limits. It never runs from an automatic
hook. See [test attack](specs/test-attack-v0.md).

## Hand an exact task to another agent

```sh
corvint dogfood handoff --task-state STATE_JSON --session-key ORIGINAL_KEY > BUNDLE_JSON
corvint dogfood handoff --bundle BUNDLE_JSON --session-key ORIGINAL_KEY
```

The first command adds bounded caller scope, decisions, review feedback, unknowns and next
actions to the existing receipt. The receiver chooses the original session key explicitly.
Clean HEAD/tree, checkout, enrollment and anchor drift withhold both context and task state.
Caller text is inert and cannot grant authority or run commands. A separate Codex receiving
session was exercised; cross-provider consumption remains unobserved. The original
`--receipt` mode is preserved. State schema and bounds:
[SESSION-V0-020..022](specs/session-context-dividend-v0.md).

## Inspect relationships around a selected API

```sh
corvint breakage --manifest MANIFEST_JSON --api api:api.go:Changed \
  --repository api=/absolute/api-checkout \
  --repository consumer=/absolute/consumer-checkout --base FULL_BASE
```

Supply an explicit `corvint-breakage-manifest/0` with repository origin/commit/tree identities,
exact source path/blob/spans and optional original EEP V1/V2 declarations. Checkout locations
are supplied separately. Go syntax calls, test references and declared flow/documentation
associations remain distinct. A shared file does not establish dependency on the selected
symbol. Every report retains incomplete scope, unresolved edges and unsupported sources.
No checkout discovery, fetching, provider execution or tests occur. See the
[manifest and map contract](specs/cross-repository-breakage-map-v0.md).

## Inspect conditional project instruction loading

```sh
corvint context --task "inspect project instructions" --subject app/source.go \
  --instruction-host codex --instruction-host-version 0.153.2 \
  --instruction-cwd app --instruction-profile default
```

The explicit default assumptions predict committed project filename selection, shadowing,
byte truncation and bounded Unicode-control warnings using exact Codex 0.153.2 source rules.
Actual global instructions, trust/configuration and session load state remain `UNKNOWN`.
Other versions and hosts remain unknown. Host selection never changes project authority;
semantic instruction conflicts are unresolved. See [instruction doctor](specs/agent-instruction-doctor-v0.md).

## Run the disposable proof tour

```sh
script/proof-tour.sh --out /private/tmp/new-proof-tour \
  --verifier /absolute/path/to/reviewed-cem01-go --verifier-sha256 LOWERCASE_SHA256
script/proof-tour.sh --resume /private/tmp/new-proof-tour --ack /absolute/reviewer.ack
```

The fresh run records orientation, a failing constant test, a 600-to-900 repair, missing-witness
refusal and a cited CEM. It exits 3 awaiting an independent reviewer. The reviewer supplies the
exact seven-line ACK described in [proof tour](specs/proof-tour-v0.md); the tour never fabricates
an accepted ACK. Resume rechecks the original fixture and digests, then strict readiness, Go
tests and a copied checksum-pinned verifier. Output collisions and inspection failures refuse.
The actual tour used a separately supplied independent ACK. Its example proves a constant-only
fixture; token expiration/timing, third-party interoperability and general usefulness remain
unobserved. The portable verifier is a second Corvint-authored implementation.

## Verification and rollback

[The build log](build-log/2026-09-29-six-change-evidence-workflows.md) retains candidates, failed
reviews, repairs, actual use and qualification limits. Focused checks and independent review
do not imply the repository-wide release gate. Each owning spec records rollback. Preserve
receipt/fixture directories when removing optional features; no store or authority migration
is needed. Native tickets remain open until required landing and completion writes succeed.
