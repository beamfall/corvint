# Design: jest-provider

## Decisions

1. **Built-in, not a provider.** Jest is a runner of the compiled-in contract (RNC-V1-001). The
   change adds no executable and no `providers.jest` configuration entry (owner decision R14). The
   change name keeps the word "provider" because owner decision R7 names it so.

2. **Discovery.** The test inventory lists Jest test files with `jest --listTests` and test names
   with Jest's own collection run, marked as an executing lister (TIV-V1-004). A lister failure
   makes the instance absent from the inventory, so it runs in full (TIV-V1-011).

3. **Selection.** For selected tests, Corvint runs the instance command with `--runTestsByPath` and
   the selected files, and one `-t` pattern anchored with `^` and `$` that matches the selected full
   names, escaped as regular expression literals. `exact` is true only when no other test in those
   files has the same full name; otherwise it is false and the plan says so. Selecting a whole file
   passes the file only.

4. **Results.** Corvint adds `--json --outputFile=<private path>` and reads `testResults[].
   assertionResults[]`: `ancestorTitles` and `title` give the canonical test ID (CTI-V1-004),
   `status` gives the outcome and `duration` the per-test duration (PTD-V1-002). A selected test not
   in the report is NOT_RUN and the run is INCOMPLETE. A report that cannot be parsed makes the run
   INCOMPLETE and keeps Jest's exit code.

5. **Default command and instances.** With no declared command, the default is `npx jest` run from
   the instance root (RNC-V1-013). Each Jest root and config is an instance (TIV-V1-003);
   `CORVINT_SELECTION=full:jest` applies to every Jest instance.

6. **Unsupported setups.** A Jest major version below the minimum (Open 1), a config that cannot be
   read, or a project that sets a custom test runner or reporter that removes the JSON report runs the
   instance's full command with reason `jest-unsupported-config`.

7. **End-to-end check.** A Jest fixture with two roots is planned and run through `corvint affected`,
   `corvint test`, `corvint watch` and the MCP affected tool, and the receipt is compared with the
   inventory. This closes the G23 open item and is the evidence for RNC-V1-010 in change
   `runner-contract`.

## Failure modes

| Failure | Behaviour |
| --- | --- |
| `npx` or `jest` not found | Run INCOMPLETE with the launch error; exit code kept; no narrowing claimed. |
| Lister fails | Instance absent from inventory; runs in full, `inventory-absent`. |
| Duplicate full names in a file | `exact` false; the plan prints it. |
| Selected test missing from report | NOT_RUN; run INCOMPLETE. |
| Report missing or unparsable | Run INCOMPLETE; exit code kept. |
| Pattern exceeds argument limit | Argument file or file level, `argv-limit` (LEV-V1-014). |
| Unsupported version or config | Full command, `jest-unsupported-config`. |
| Unknown `full:` runner name | Per owner decision R13: exit 2 in CLI and CI, all tests in hooks and MCP. |

## Acceptance evidence

- JST-V1-009 end-to-end fixture result through all four consumers.
- Unit tests for name escaping, anchoring and duplicate detection.
- A report fixture with nested `describe` blocks mapping to canonical IDs with durations.
- The release binary list without a Jest provider (JST-V1-010).
- Jest's G4 measurement on held-out repositories, recorded by the narrowing measurement work.

## Rollback

Revert the change's commits. Jest projects then run their declared full command through `corvint
test` as at integ 9ec3a29d0. Receipts written with Jest canonical IDs stay readable as data.
JST-V1-001 to JST-V1-010 stay listed as retired if the change is withdrawn.

## Open

1. The minimum supported Jest major version.
2. Whether the JST-V1-009 fixture also serves as the RNC-V1-010 fixture, or the runner-contract
   change keeps its own.
3. Filing the G23 ticket for the Jest end-to-end check.
