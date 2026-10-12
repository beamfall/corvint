# V1-1304: frontier admits a verified cem/1.0 map as a stable view

Date: 2026-10-12. Base: `9deae4838efab45075ad3280fdae8ff484404bf7`. Branch:
`claude/v1-1304-frontier-stable`. Ticket: V1-1304.

## Intent

`corvint frontier` over the default `cem/1.0` change map must answer through the stable verifier
`ACCEPT` instead of refusing `unsupported-frontier-context`. It must refuse a tampered map and a
wrong base. The host-lifecycle conformance frontier case must exercise the default map instead of
the explicit `cem prepare --spec cem/0.2`.

## Decisions

- Contract conflict, decided fail-closed. The ticket asks for "the frontier view". Accepted
  `CEM-V1-121` keeps `frontier/0` refusing `cem/1.0`, and `CEM-V1-126` records that no stable
  frontier document exists (`NOT_PRODUCED`, never `EMPTY`). The new route therefore emits a
  distinct profile, `frontier-stable-view/0`. Its state is always `NOT_PRODUCED` and it exits 1,
  the same as an open frontier, which closes nothing. `frontier.Compute` and its `cem/1.0` refusal
  are unchanged, so `TestCEMV1121LegacyRoutesRefuseStable` still holds.
- The CLI routes by the map's declared spec (`wire.DeclaresStable`). The `cem/0.2` path keeps
  working unchanged until V1-1278 removes it.
- The artifact root is explicit (`--artifacts`, resolved against `--root`), with no default, as in
  `CEM-V1-125`. `--artifacts` given with a non-stable map is `invalid-frontier-input`.
- Admission order: call shape, raw ceilings, profiles (the map must declare `cem/1.0` and the OCM
  must be bound to `cem/1.0`), revisions, artifact root, then the viewer. Viewer refusals pass
  through only from closed allowlists: the stable verifier codes, the frontier/0 verifier codes and
  the TCQ passthrough codes. Any other code becomes `frontier-internal-error`. A returned view is
  rechecked before encoding: state, spec, `ACCEPT`, `EXACT`, the TCQ profile and identity prefix,
  and map/OCM digests equal to the digests of the request bytes. A view that fails the recheck never
  surfaces.
- The view also lists the hunks marked `unknown`. Without them, a map with no OCM claims would
  render as an empty-looking view.
- The conformance case commits the map so that the sidecar is `EXACT`, prepares the OCM against
  that commit, and passes the clone as `--artifacts`. It then checks a wrong base
  (`base-revision-mismatch`) and a tampered working-tree map (closed exit-2 code).

## Failure modes

The closed exit-2 envelope covers:

- tampered map bytes (`excluded-artifact-mismatch`);
- wrong expected base (`base-revision-mismatch`);
- tampered artifact (`artifact-digest-mismatch`);
- a missing artifact root, or `--artifacts` with a `cem/0.2` map (`invalid-frontier-input`);
- a `cem/0.2` map or an OCM bound to 0.2 on the stable route, or a verifier without the stable
  route (`unsupported-frontier-context`);
- an undeclared verifier code, or a forged or non-`ACCEPT` view (`frontier-internal-error`).

## Acceptance evidence

- `internal/frontier`: `TestStableViewAcceptsVerifiedMap`, `TestStableViewAdmissionOrder`,
  `TestStableViewRefusalsAreClosed` and `TestStableVerifierCodesCoverTheResultSchema`: PASS. The last
  test pins the stable allowlist to every `code`/`issueCodes` member of
  `protocol/cem-1.0/stable/RESULT.schema.json`.
- `internal/frontierrepo`: `TestV1304StableViewAccepts` and `TestV1304StableViewRefusals` (tampered
  map, wrong base, missing root, 0.2 map, tampered artifact) over a real Git fixture: PASS.
- `cmd/corvint`: `TestRunFrontierStableMapRoute` (JSON and human exit 1, artifact root forwarded,
  repository bytes unchanged, four exit-2 refusals) and the existing frontier tests: PASS.
  `TestParseFrontierOptions` now covers `--artifacts`.
- `go run ./conformance/host-lifecycle-v1 --host cli` with the branch binary: `SUMMARY status=PASS
  passed=9`, and the frontier case reports the stable view plus both refusals.
  `go test ./conformance/host-lifecycle-v1/...`: PASS. Both refusals require no stdout, exit 2 and
  exactly the two-field `frontier-error/0` envelope with the named code.
- Direct importers of `internal/frontier`: `conformance/frontier-v0` and
  `tools/gate-affected-select`: PASS.
- `cmd/corvint` with `-run 'Frontier|CLIReadVerbs'`: PASS.
- Full `cmd/corvint` package: FAIL, `NOT_ATTRIBUTED`. The run hit the 30-minute timeout while
  several other lanes loaded the host. Every failing test was a hook deadline or a 10-second probe
  (`dogfood-event-deadline`, `index-snapshot-stale`, `QLF-V0-006` Git probe). None was a frontier
  test. CI is the authoritative run.
- Codex round 1 (one round, `codex exec -s read-only`): CONCERNS, both fixed. (1) The stable
  allowlist omitted schema-declared codes such as `duplicate-reference` and
  `artifact-resource-limit`, so those became `frontier-internal-error`. It now admits the full
  schema enums, and the schema test above pins them. (2) The conformance tampered-map control
  accepted any code and parsed the combined streams. It now requires `excluded-artifact-mismatch`
  and the exact envelope.
- `go vet` on the touched packages: clean.

## Retained limits

- No requirement IDs. `docs/specs/REQUIREMENTS.tsv`, `INDEX.json` and `README.md` were reserved by
  another lane, so the behavior is recorded as a dated section appended to
  `docs/specs/change-frontier-v0.md`.
- `--artifacts` is not in the `help frontier` topic, because `cmd/corvint/help.go` is reserved.
  This is follow-up work.
- No stable frontier document. Promoting `frontier-stable-view/0` to a closing profile needs a
  stable LRF profile and its own decision.
- Full gate (`make gate`): `NOT_RUN`, per the owner's standing preference for focused tests.
- No real-adapter regression for a duplicated criterion binding. The pass-through is pinned at the
  translation boundary, against the schema enums.
- `docs/specs/host-lifecycle-qualification-v1.md` still describes the frontier case as
  `frontier/0` `OPEN`. That spec is outside the claimed scope, so the row needs a follow-up edit.
- The use-case receipt `UC-EVIDENCE-CARRYING-COMPLETION` pins `cmd/corvint/frontier.go`, so it is
  repinned in its own commit with `script/repin-use-case-receipts.sh`. That path is outside the
  claimed scope; the edit is a mechanical consequence of this change.

## Rollback

Revert this change. That deletes `internal/frontier/stable.go`, `frontierrepo.Adapter.StableView`
and the CLI branch, and restores the conformance case to `cem prepare --spec cem/0.2`. A `cem/1.0`
map is then refused with `unsupported-frontier-context` as before.
