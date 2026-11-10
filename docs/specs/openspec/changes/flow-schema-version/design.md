# Design: `flow-schema-version`

## Decisions

- Version field. The existing `profile` member carries the version as `corvint-flows/N`. No second
  version member is added, so a file written today stays valid as version 0.
- Supported list. Each reader build holds a closed list of supported N values in one constant in
  `internal/appflows`. The highest supported value is the one the writer (`corvint flows` commands
  that write the file) emits.
- When N increments. Unknown members are refused today (`flowplan.go` line 102, integ 9ec3a29d0),
  so adding, removing or renaming a member, or changing a member's meaning, increments N. The member
  changes planned by `affected-plan-kinds` therefore ship as a new N in the same change that adds
  the members.
- Member lists. Each released N has a committed member list and a golden file under
  `internal/appflows/testdata/`. A test fails if the reader's accepted members for an existing N
  differ from that N's recorded list, so a member change cannot ship without a new N.
- One reader. `internal/appflows` exposes one function that reads, checks the version and returns a
  typed result. A test in `internal/appflows` fails if any other package opens `.corvint/flows.json`
  directly (a source scan for the path constant outside the package).
- Reasons. Three outcomes are distinct and typed:
  - `FLOWS_SCHEMA_UNSUPPORTED`: the profile has the `corvint-flows/` prefix and a well-formed N that
    is not in the supported list; the reason names the found N and the supported list;
  - `FLOWS_DECLARATION_INVALID`: the file is malformed, has an unknown member, or has a profile
    without the `corvint-flows/` prefix;
  - no file: flows are absent and the planner behaves as it does today.
- Planner behaviour. On `FLOWS_SCHEMA_UNSUPPORTED` the affected plan selects every E2E test, reports
  the reason on each such selection and in the plan summary, and exits as it does for any fail-closed
  E2E plan. Unit and integration selection does not depend on the flow file and is unchanged.
- Flow reports. `corvint flows` reports and the documentation product's feature map refuse with the
  same reason and a non-zero exit; they never print a partial report.
- Smoke test. A release job extracts only the `corvint` binary from the candidate archive into an
  empty `PATH` directory and runs, on a pinned fixture repository:
  - `corvint affected --base <fixture base>` with a version-0 flow file;
  - the flow reports of PROD-DOCS-011 (`corvint flows status` and `corvint flows gaps`);
  - the same commands on a copy whose profile is one above the highest supported N, expecting
    `FLOWS_SCHEMA_UNSUPPORTED` and an all-E2E plan.

## Failure modes

- A writer emits an N the reader does not support: blocked by a unit test that writes with the
  writer and reads with the reader of the same build.
- A profile such as `corvint-flows/01` or `corvint-flows/-1`: malformed, so
  `FLOWS_DECLARATION_INVALID`.
- Both `corvint` and `corvint-docs` installed at different versions: each refuses independently; no
  product silently reads a newer file.
- The smoke job finds a second binary on `PATH`: the job fails, since the test is "installed alone".
- `corvint-docs` vendors its own parser: caught by the source scan test.

## Acceptance evidence

- Unit tests in `internal/appflows` for supported, unsupported, malformed and missing versions.
- An affected-plan test where an unsupported version yields every E2E test with the reason.
- The source scan test for a single reader.
- The release smoke job log on the RC SHA.

## Rollback

- Revert the reader change; the existing exact-profile refusal at `flowplan.go` lines 105-106 returns
  and every non-zero version reads as `FLOWS_DECLARATION_INVALID`.
- Remove the smoke job from the release workflow.
- `FLOWS_SCHEMA_UNSUPPORTED` stays reserved in the catalogue; it is never reused for another meaning.
