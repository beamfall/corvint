# Source removal B1: verifier repair 1

Ticket: V1-5694. Repair parent: `a40193eacf7a023d330f85ce69dbc11b02e5b8cd`.

The host found stale requirement-token exceptions and unresolved traceability citations after
the B1 removals (RMV-V1-007, RMV-V1-031). Remove the two TUI row-label exceptions whose only
source was the deleted fixture. Mark the self-use process-buffer test and plan-delta test as
removed under TTG-V0-952. Keep the historical names and the surviving GPK-V0-9930 test citations.
The TSP-V0-666 row records why it is untested under TTG-V0-963: B1 retired its tool and tests.
No requirement definition, runtime code, golden, assertion or checker policy changes.

This is stale evidence bookkeeping, not an analysis soundness gap. No new form allowlist or
unknown-form regression applies. RDG-V0-9980 continues to reject stale exceptions, and the
traceability gate continues to reject unresolved test names without an explicit disposition.

## Verification

Commands used `TMPDIR=/home/russell/lanes/sr-b13.tmp`, with no exported commit identity.

- `make -s requirement-definitions-check traceability-tests-check`: PASS; 240 retired-only IDs
  remain informational, with 885 removed-test markers, one planned test and two checked subtests.
- `make -s spec-requirements`: PASS, generated files unchanged.
- `script/check-line-citations.sh --repin`: PASS, no moved citations; 184 basename-only skips.
- `make -s spec-requirements-check line-citations-check`: PASS; the existing whole
  `internal/specindex` package ran through `script/heavy.sh`, 1.108 seconds.
- `script/heavy.sh go test -count=1 -timeout 30m ./tools/declaration-compat-check`: PASS,
  existing whole package, 0.276 seconds.
- `script/heavy.sh go test -count=1 -timeout 30m ./tools/import-ratchet`: PASS,
  existing whole package, 0.841 seconds.

Before testing, `corvint affected --base 55decc6a041a1df3f8ae43e95ba21cea669e7d80 --format json`
selected those three packages. Its scope remains UNKNOWN: 339 selected, 350 excluded,
80 unknowns (58 LANGUAGE_FRONTIER, five UNINDEXED_SOURCE_PATH, 17 UNOWNED_DIRTY_PATH), and
16 unresolved packages. The broad fallback was not run. The raw 289241-byte plan stays outside
the repository at `/home/russell/lanes/sr-b13-evidence/repair1-affected.json`, SHA-256
`5d0821c8ea6b88faac8c3105eaa28e73526792fcda06cc0b1029fc6ed8782dd4`.

The requested CEM rebind/check cannot run: decision 0594 retired the workflow, as recorded in
`docs/DOGFOOD.md`, and `make -n dogfood-check` exits 2 with no rule for that target. No workflow
was restored and no capture or seal was attempted. Independent review and the broad host gates
remain host-owned. The earlier B1 reference-proof blockers and B2/B3 disposition are unchanged.

Rollback: revert this repair commit to restore only the prior citation and exception state.
