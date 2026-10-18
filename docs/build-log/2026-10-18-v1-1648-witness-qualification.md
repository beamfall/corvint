# V1-1648: adopter version qualification and declared test repositories

Owner input: [issue 1070](https://github.com/beamfall/corvint/issues/1070). An adopter running
Playwright 1.64 in a test repository separate from the store could not credit any obligation:

- the shipped qualified-version list is empty, so every report refused `UNSUPPORTED_VERSION`;
- there was no supported way for an operator to qualify a version locally;
- witness commit resolution and the TOL-V0-012 source check ran only in the store's worktree.

TOL-V0-060..067 were added to `docs/specs/corvint-tasks-obligation-ledger-v0.md`, building on
TOL-V0-055..059 (V1-1649, PR #1090). No CAL-V0 ID was added. They are accepted under the owner's
standing auto-accept of lane requirements (2026-10-11), and delivery stays experimental. The
highest TOL ID on `origin/main` was TOL-V0-059, and no open PR used TOL IDs, so there was no
collision.

## Design

- **Fixture (TOL-V0-060).** One embedded spec file, `obligation-qualification.spec.ts`, has nine
  `TOLQ` ids. They cover these behaviours:
  - a passing step;
  - a soft-failed step and a passing sibling after it;
  - a parent of a soft-failed nested step;
  - a soft failure outside steps;
  - a test that fails at retry 0 and passes at retry 1;
  - a plain pass;
  - a `test.fail` test.

  `--print-fixture` prints the fixture, its digest and the expected classes, and writes nothing.
- **Check (TOL-V0-061).** `qualify-version --from-playwright-report` reads the report with the
  TOL-V0-009 reader, admitting any version label. It classifies the report against a synthetic
  OPEN ledger with the unchanged TOL-V0-010..012 and TOL-V0-022 rules. A match binds only in a file
  named like the fixture, read as the embedded bytes, and no Git object is read. Any class
  difference refuses `GATE_FAILED` with `OBLIGATION_QUALIFICATION_FAILED:` naming the first id.
- **Allow-list (TOL-V0-062, TOL-V0-063).** The allow-list is the closed
  `taskman-obligation-qualifications/0` file in the Git common directory, outside the journal and
  archive, like the run records.
  - It retains each version's report and fixture digests, its classes and `qualifiedAt`.
  - It holds at most 64 versions and 1 MiB, and is written by atomic rename.
  - A later profile refuses `UNSUPPORTED_VERSION`, and any other defect refuses `MALFORMED`.
  - Witness and run-batch admit the shipped list plus this file. A version on neither list keeps
    the old refusal.
  - It is classified under CAL-V0-131 (`LiveFormats` and the newer-version table).
- **Test repositories (TOL-V0-064..067).**
  - The policy's `obligations.testRepositories` maps 1..16 labels to paths. A relative path is
    resolved against the primary worktree.
  - `witness --test-repository NAME` resolves `--commit` and the source check in that worktree.
    The name is retained as an optional `testRepository` member of the payload and evidence, so a
    witness without the flag keeps its exact bytes.
  - The writer binds the name in its recomputation. Both the command and the writer refuse an
    undeclared name as `MALFORMED` with `OBLIGATION_TEST_REPOSITORY_UNDECLARED:`.
  - Receipt audit re-checks the matches in the repository the policy posted at that receipt
    declares.

## Evidence

- Focused tests trace each ID; see the spec's traceability row.
- AC3 is `TestTOLV0065_SeparateTestRepository1_64`:
  - version `1.64.0` is qualified through a fixture-shaped report;
  - a separate Git repository is declared;
  - its row that passed at retry 0 is credited, while a row that failed at retry 0 is not;
  - receipt audit is consistent.
- `NOT_RUN`: a live Playwright run of the fixture. This lane installs no browsers or Playwright,
  so the synthetic report follows the Playwright 1.61.1 shape observed under TOL-V0-010. An
  operator's first live `qualify-version` run is the remaining qualification evidence.
- `NOT_RUN`: run-batch with a store-local version, which is exercised only through the shared
  admission helper.

## Limits

- `qualify-version` takes no store lock. Two concurrent runs are last-writer-wins on the
  allow-list file, and each run's own entry is atomic.
- run-batch, preflight and qualify still run only in the store's worktree. Only the witness verb
  takes `--test-repository`.
