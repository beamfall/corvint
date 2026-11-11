# Design: `non-code-map`

## Decisions

- Only Corvint configuration files under `.corvint/` force a full run (owner decision R31,
  2026-11-09). NCM-V1-006 now lists the exclusions: the flow file and its link records select the
  tests linked to the changed flows, and the two private ledgers (`self-observations.jsonl`,
  `unplanned-reads.jsonl`) add nothing. Declared documentation paths keep `DECLARED_DOCUMENTATION`
  (`internal/liveverify/affected/select.go`, integ 9ec3a29d0) and add no test.
- A path no runner owns, and a removed or unavailable runner, run all tests with a named reason
  (owner decision R32, 2026-11-09). NCM-V1-001 states this. Orchestrator ruling 2026-11-11 (under
  the owner's 2026-11-02 delegation, `docs/build-log/2026-11-11-ncm-owned-paths.md`): a non-code path
  a Go rule already attributes (its enclosing package directory, a package's testdata, a go:embed
  input, a declared reader) is owned and narrows to that attribution; only a path no runner
  attributes runs everything, with reason `unowned-path`. The selector that holds the Go index
  decides this, so `UNOWNED_DIRTY_PATH` stays on the allowlist in
  `internal/gateselect/allowlist.go`. Earlier text took it off that allowlist.
- The map profile is `corvint-data-map/0`. Every other declaration new in 1.0 starts at `/0`
  (`corvint-flows/0`, `corvint-test-inventory/0`), and nothing reads an earlier data map, so there
  is no reason to start at `/1` (review docs F17). Earlier text proposed `corvint-data-map/1` and
  left the choice to the build task. A later incompatible change takes `/1`; a 1.0 reader refuses
  it (NCM-V1-007).

## Failure modes

- A file under `.corvint/` that is neither a listed exclusion nor a known declaration (for example
  `changes/`, `deferred-intents/` or an operator marker such as `unplanned-reads.enabled`): it is
  treated as configuration and selects everything with reason `corvint-config`. Whether some of these
  should add nothing is an open question for the owner.
- A flow file edit whose changed flows cannot be computed (unreadable or invalid file): every test
  is selected with a named reason, never an empty selection.
- The map file is malformed, oversized or has an unknown profile version: refuse with
  `data-map-invalid`, exit 2; never fall back to an empty map that would narrow nothing silently.
- An entry has no proving fault case or no acceptance record: it is reported as `proposed` and does
  not narrow.
- A test cited by an entry no longer exists, or the mapped path's evidence commit is no longer an
  ancestor: the entry is `stale`, does not narrow, and is reported.
- A path is both on the never-narrow list and in an entry: the entry is ignored and reported.

## Acceptance evidence

- Fixture repositories for Go, one JS runner, pytest and one Ruby runner, each with an unmapped
  fixture, a proven entry, an unproven entry, a stale entry and a never-narrowed path.
- Go tests per requirement; the never-narrow list asserted path by path.
- At least one proven entry per blocking runner on its PRR-V1 repository, with its FIH-V1 fault case.
- Planner tests for the R31 cases (map edit, flow file edit, ledger append, documentation-only
  edit) and the R32 cases (unowned path, removed runner), each asserting the printed reason.

## Open questions

- Classification of `.corvint/changes/`, `.corvint/deferred-intents/`, the committed inventory file
  and the `unplanned-reads.enabled` marker under NCM-V1-006 (today: configuration, full run).
- Whether the existing hand-written read declarations later fold into the map.

## Rollback

Delete the map file to return a repository to full runs for every non-code change. To withdraw the
change, revert its commits; IDs are retired, not reused.
