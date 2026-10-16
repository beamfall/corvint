# 2026-10-16: Test consolidation repository qualification and V1-1023/V1-1024 audit

## Intent

The lane prompt for V1-1023 (GitHub #681) asked for a spec section, an implementation, hostile tests
and a real-repository qualification of a test-consolidation projection. For V1-1024 (GitHub #682)
it asked for the step-negation negative-control spec, with browser-free parts only.

## Audit: both contracts were already delivered

PR #697 ("Batch J", merged 2026-10-09) delivered both, and GitHub closed #681 and #682 through it.

- V1-1023 is covered by `docs/specs/test-consolidation-planner-v0.md` (TCN-V0-001..012, accepted by
  decision 0457, with MCP interpretations in decision 0466). The code is `internal/testplan`,
  `corvint test-plan consolidate|check` and `corvint-corpus-mcp --consolidation`
  (`corvint.consolidate_tests`). See build logs 2026-10-08-v1-1023-consolidation-planner and
  2026-10-08-test-consolidation-planner-spec.
- V1-1024 is covered by LPCV-V0-057..070 in `docs/specs/live-proof-carrying-verification-v0.md`
  (accepted by decision 0458). The spec covers fault derivation, the single-step-failure pass rule,
  revision binding, `--all-steps`, `UNPROVEN` abstention, non-goals, failure modes and rollback. The
  code is `internal/stepnegation`, `internal/jstestprovider` negation and `corvint test-validity
  negate`. The join is compiled but gated off, and the LPCV-V0-070 live Playwright matrix is
  `NOT_RUN`. See build logs 2026-10-08-v1-1024-negative-controls and
  2026-10-08-v1-1024-step-negative-controls-spec.

Decision: do not write a second spec or a parallel projection. Duplicate requirement IDs for an
accepted contract would split authority, and changing accepted TCN-V0 or LPCV-V0 text is an
amendment the standing auto-accept does not cover. This change adds no requirement IDs and amends
no requirement text. The remaining open gap in V1-1023 is the real-repository qualification. This
entry records it and pins it with a maintained test.

## Repository qualification (V1-1023)

The inventory is this repository's console tests at `a0b4c4108`: `internal/console/console_test.go`,
`roadmap_test.go` and `security_test.go`.

Selection rule: one variation for each subtest or table case that sends at least one HTTP request
to a console `Server`. Pure unit subtests are excluded. Each variation records the following:

| Field | Value |
| --- | --- |
| `spec` | The test file. |
| `app`, `flow_id` | `corvint-console`. |
| `screen` | `screen:corvint-console:<route>`, taken from the first request. |
| `setup` | The stub, snapshot or repository fixture the subtest builds, for example `stub:help+list`, `snapshot:document`, `repo:corvint-head` or `stub:marker-tool`. |
| `user` | `operator`. The nine `TestBrowserBoundaryBeforeTools` cases use `untrusted-browser`. |
| `org` | `""`. |
| `action` | Method and path. `?`, `&` and `#` become `@`, `+` and `@` to fit the identifier charset. |
| `assertion` | The checked facts. |
| `changes` | Present only where a request writes. |

No `witnesses` were recorded: the console tests are Go tests, which have no test-validity identity
(V1-1031). The input is committed as `internal/testplan/testdata/console-qualification-input.json`.

Results from `corvint test-plan consolidate` (the CLI built from `a0b4c4108`, Corvint 1.0.0-rc.3):

| Measure | Value |
| --- | --- |
| Input digest | `sha256:b32eeb43ba17b93cbe32a4b8c4744a9fdaf49048a8ba991c8c01dce45c2e9f7d` |
| Table digest | `sha256:6315943581297a42c0e94898b8294e6c8309784ac1a026014b4e96d368ead086` |
| Variations | 47 |
| New tests | 33 |
| Grouped | 20 |
| Isolated | 27 (all `unique-context`) |
| Reused, duplicates, abstained | 0, 0, 0 |
| Saving against `baseline_one_per_row` | 14 of 47 (29.8 %) |
| Status | `COMPLETE` |
| Anchors | `NOT_RUN` (no AMAP-V0 map of the console exists) |

The existing structure has 47 subtests with 39 server setups, because the nine boundary cases share
one server.

Table excerpt:

```
<!-- corvint test-consolidation-table/0 input=sha256:b32eeb43ba17b93cbe32a4b8c4744a9fdaf49048a8ba991c8c01dce45c2e9f7d tests=none maps=none revision=none anchors=NOT_RUN max-steps=8 status=COMPLETE -->
| test | spec | variation ids | isolated reason |
| --- | --- | --- | --- |
| T001 | internal/console/console_test.go | con.axis.invocation-boundary, con.boundary.standalone, con.render.columns | - |
| T002 | internal/console/console_test.go | con.boundary.no-state | unique-context |
| T030 | internal/console/security_test.go | sec.boundary.foreign-host, sec.boundary.foreign-origin, sec.boundary.null-origin, sec.boundary.write-read-route, sec.boundary.wrong-port | - |
| T031 | internal/console/security_test.go | sec.boundary.missing-token, sec.boundary.oversized, sec.boundary.token-query-insufficient, sec.boundary.unreviewed | - |
```

The other multi-step groups are T008 (three evidence routes), T015 (two listing routes) and T022
(three roadmap routes).

Checks on the real CLI:

- Determinism: a rerun produced identical bytes. A shuffled, re-encoded input (seed 1023) also
  produced identical bytes.
- `check` exit 0: the exact table, and the table inside Markdown.
- `check` exit 1 with `test-plan-mismatch`: an edited reason, reordered rows, a moved variation, a
  dropped row ("variation sec.structured-forms is missing"), and `--max-steps 2`.
- `check` exit 1 with `test-plan-header-missing`: a doubled header.
- `check` exit 1 with `test-plan-incomplete`: an input with `changes` removed from
  `con.mutate.post-only`, which abstains with `missing-anchor`.
- Exit 2 with `test-plan-invalid-arguments`: `--revision` without `--map` or `--tests`.
- Read-only: the worktree status hash was the same before and after.

Findings:

- Singletons dominate, because almost every subtest builds its own stub fixture.
- A variation that spans several routes is placed only by its first screen.
- The 29.8 % saving is above the 20 % kill line. However, this is the lane's own labelled set, not
  the adopter's 3,614-row set, and no grouped test was written or run. The promotion criteria are
  therefore not met.

Maintained evidence: `TestConsoleQualificationGolden` (`internal/testplan/qualification_test.go`).
It asserts the counts and the `COMPLETE` status, pins the table digest, compares the table with
`testdata/console-qualification.table`, and checks that the table passes `Check` against its own plan.

- Mutation: dropping `Setup` from `contextOf` made the test fail with counts of 27 new tests, 29
  grouped and 18 isolated. The change was then reverted.
- Run: `go test -race -run TestConsoleQualificationGolden ./internal/testplan` passed. Load averages
  were 62.28/62.55/61.30 before and 64.76/63.00/61.55 after.

Baseline focused packages at `a0b4c4108`, all passing:

| Package | Time |
| --- | --- |
| `internal/testplan` | 439.3 s |
| `internal/stepnegation` | 10.1 s |
| `cmd/corvint-js-test-provider` | 17.8 s |
| `cmd/corvint-corpus-mcp`, `-run 'Consolidat\|TCNV0'` | 166.9 s |
| `internal/jstestprovider`, `-run Negat` | 4.2 s |

Load averages were 58.74/56.90/58.22 at the start and 70.31/64.37/61.48 at the end.

## Not run

- `--tests` reuse: `REUSED` cannot be reached from real provider documents (V1-1031).
- `--map`: the console has no application map.
- The owner-run qualification on the adopter's labelled set.
- The LPCV-V0-070 live Playwright matrix for V1-1024: Playwright browser and Chrome installs are not
  approved on this host.
- `make gate`.

## Rollback

Revert this change. It removes one test, two fixtures and status text in the spec. Nothing else
reads the fixtures, and no wire or stored state changes.
