# Local evidence-provider authoring kit 0.1.0

Experimental implementation of V1-0027. V1-0013's portable-proof freeze and owner acceptance
remain prerequisites for promotion. This kit adds no Core option, installation, service or authority.

`main.go` is a complete, dependency-free Go provider. Copy it, change `providerID` and
`providerRevision`, and replace the example declared relation with your evidence. The sample only
supports provider `kit-example` revision `0.1.0`. It requires an exact `--profile`; it never chooses
the newest profile. It emits a single record to stdout, diagnostics to stderr, and performs no
Git calls, downloads, network access, environment reads or writes. Inputs are caller declarations;
Corvint subsequently verifies repository identity, ancestry and path references.

## Compatibility and pins

The experimental compatibility window is exactly `external-evidence-provider/0`, `/1` and `/2`
under the current checkout's consumer. Retained `/0` records and current `/1` and `/2` records are
tested together. This is record compatibility, not arbitrary historical engine execution or a
promoted long-term window. `/0` lacks repository identity; use `/1` or `/2` to pin a root commit.
`/2` also admits path-to-path relations, which the minimal provider does not need to demonstrate.

Build the `check` consumer from this checkout. It strictly decodes the existing record profiles,
requires exact schema, provider ID, provider revision and full repository commit pins, and requires
repository ID plus root commit for `/1` and `/2`. It emits the original record bytes only after
agreement; every failure exits nonzero with no record bytes. Other repositories declared by a
record still need Corvint's normal checkout binding and verification. Pin equality is not freshness,
truth, checkout verification, governing authority, or test-selection permission.

For commands it additionally requires the lowercase SHA-256 of the executable **before launch**.
Keep a reviewed source commit, its built binary digest, exact argv and record pins together.
Save that invocation; do not recalculate its expected digest when upgrading. An upgrade needs a new
explicit invocation and conformance result. Keep the old executable at its original absolute path
to retain the old invocation. A changed executable or provider/profile version fails closed.
Trusted local operators must keep the executable and its parent directories immutable between
digest checking and launch; this is not protection against malicious concurrent path replacement.

## Author, build, exercise

Prerequisites: a clean Corvint checkout, Git, a locally installed Go **1.27.1**, and Darwin or Linux
for contained commands. No modules beyond the Go standard library are downloaded. The standalone
producer builds from its copied single source file without Corvint or a module cache. The checker
imports Corvint internals and builds inside this checkout. The complete reproducible conformance
invocation from the repository root is:

```sh
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test -count=1 -timeout 30m \
  ./examples/evidence-provider/v0/... ./internal/extevidence ./internal/procgroup ./cmd/corvint \
  -run '^(TestProviderKit|TestCommandTransport|TestRunProcessInterruptionLeavesNoDescendant|TestSupervisorSignalReapsNestedOwnedGroup|TestImpactProviderSectionSeparation|TestImpactProviderCommandEndToEnd)'
```

This edits a copy of the provider's identity/version in a fresh scratch directory, builds only that
source offline, and exercises its `/0`, `/1`, `/2` output through both transports. The command also
replays the existing labelled fixtures and process containment regressions. It is not a substitute
for the repository's full and interoperability gates.

For a manual `/1` example from the clean checkout, use a scratch path without JSON metacharacters
and retain the printed digest when saving your invocation. `shasum` is needed for this shell example.
The traps remove only this example's scratch directory; the provider itself spawns no descendants.

```sh
set -eu
kit_tmp=$(mktemp -d /tmp/corvint-provider-kit.XXXXXX)
trap 'rm -rf "$kit_tmp"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
go build -trimpath -o "$kit_tmp/provider" examples/evidence-provider/v0/main.go
go build -trimpath -o "$kit_tmp/check" ./examples/evidence-provider/v0/check
go build -trimpath -o "$kit_tmp/corvint" ./cmd/corvint
revision=$(git rev-parse HEAD)
origin=$(git rev-list --max-parents=0 HEAD)
digest=$(shasum -a 256 "$kit_tmp/provider" | cut -d ' ' -f 1)
printf '%s\n' "$digest"
"$kit_tmp/provider" --provider-version 0.1.0 --profile external-evidence-provider/1 \
  --revision "$revision" --origin "$origin" --path internal/extevidence/record.go > "$kit_tmp/record.json"
"$kit_tmp/check" --file "$kit_tmp/record.json" --profile external-evidence-provider/1 \
  --provider-id kit-example --provider-version 0.1.0 --revision "$revision" \
  --repository-id app --origin "$origin" > "$kit_tmp/checked-file.json"
argv=$(printf '["%s","--provider-version","0.1.0","--profile","external-evidence-provider/1","--revision","%s","--origin","%s","--path","internal/extevidence/record.go"]' "$kit_tmp/provider" "$revision" "$origin")
"$kit_tmp/check" --command "$argv" --executable-sha256 "$digest" \
  --profile external-evidence-provider/1 --provider-id kit-example --provider-version 0.1.0 \
  --revision "$revision" --repository-id app --origin "$origin" > "$kit_tmp/checked-command.json"
cmp "$kit_tmp/checked-file.json" "$kit_tmp/checked-command.json"
"$kit_tmp/corvint" impact --provider "$kit_tmp/checked-command.json" internal/extevidence/record.go
```

The checker launches the command once and retains its complete checked bytes. Pass those bytes as
a file to Core; launching the provider again would be a different observation. A failed shell
redirection may leave an empty file; consume it only after the checker exits zero. For a direct
unpinned transport comparison the existing `impact --provider-command "$argv"` remains available;
that Core option does not enforce this kit's extra identity pins.

## Fixtures and limits

The fixtures below live under `internal/extevidence/testdata`; their placeholder revisions are filled
by the existing two-repository test harness. They are synthetic labelled records, not adopter data.

| Case | Existing witness |
|---|---|
| Valid `/0` | `mock-provider.json` |
| Stale | same `/0` record at its real ancestor; `conformance-selection/stale-provider.json` |
| Malformed | unknown authority member mutation; `TestCommandTransportFailuresAreClosed` malformed and trailing-document cases |
| Ambiguous | `conformance-v1/ambiguous.json`, `conformance-selection/ambiguous-root.json` |
| Repository mismatch | `conformance-selection/cases.json` checkout mismatch; `TestCheckoutBinding` |
| Unsupported | `conformance-selection/unsupported.json`; unsupported schema and provider pin tests |
| Current `/1`, `/2` | `conformance-v1/two-repository.json`, `conformance-path/*.json` |
| Core separation | `TestImpactProviderSectionSeparation`, `TestImpactProviderCommandEndToEnd` |

`ReadPinned` uses the existing strict decoders and contained command runner: 10 seconds per launch,
1 MiB stdout/record, 64 KiB stderr, empty stdin, repository cwd, only `PATH`, `TMPDIR`, `LANG=C` and
`LC_ALL=C`. Timeout, cancellation, overflow, failed exit or unproven cleanup returns no record bytes.
SIGINT/SIGTERM cancel the checker; the owned process group is killed and reaped. The procgroup
interruption and nested-group regressions prove no surviving owned descendants. As in the existing
transport, this is process containment, not a sandbox against a malicious operator-selected binary.
File input must be regular and reads at most 1 MiB plus the overflow sentinel. Executable hashing
precedes the runner's timeout and relies on the trusted local regular artifact.

External evidence remains in `context.external`, labelled `external-provider`. It never enters Core
ranking, CEM/OCM authority or governing intent. No runtime download, remote transport, daemon,
general plugin host, n8n source or default-product change belongs to the kit. Kit MCP integration
is proposed/out of scope; existing separately accepted MCP and remote profiles are unchanged.

## Licensing and rollback

The template, checker, implementation and tests are **AGPL-3.0-or-later** under `LICENSE` and
`LICENSING.md`. Copying this template preserves its terms. Protocol/interoperability paths retain
their enumerated Apache-2.0 boundary; any new Apache material belongs in `protocol/**`. This kit
does not widen that boundary or relicense the provider template as protocol material.

Rollback removes `examples/evidence-provider/v0`, `internal/extevidence/pin.go` and its tests, and
the kit requirements/index additions. Existing file/command transports and records keep working.
Do not mutate a saved pin or an old artifact to make an upgrade pass. Freeze/promotion and owner
acceptance remain open even when all local conformance tests pass.
