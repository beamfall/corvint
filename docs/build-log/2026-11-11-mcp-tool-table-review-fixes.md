# MCP tool table review fixes (V1-5447)

Date: 2026-11-11 (America/Toronto)
Intent: `docs/specs/mcp-tool-table-v1.md`, MTT-V1-003, MTT-V1-005, MTT-V1-007, MTT-V1-008 and
MTT-V1-009, after the independent review of the delivery in
`docs/build-log/2026-11-11-mcp-tool-table.md` (no P1; three P2 and four P3 findings, all handled
here).
Outcome: each finding is fixed with a test, or recorded in the spec where the finding was a gap
in the stated contract; the focused tests pass.

## Findings and dispositions

- Grammar parity (P2, MTT-V1-008): no command line parses `CORVINT_SELECTION`; the only reader is
  the `affected` tool through `selectionswitch.Parse`. There is no second parser to keep in step,
  so the spec's delivered-state note says so and requires the change `narrowing-measurement` CLI
  reader to call the same `Parse`. `TestMTTV1008AffectedToolAgreesWithTheOneParser` pins the
  tool's reason to `Parse` for `full`, `full:go`, `full:a,a`, `fastest`, ` full`, `full:Go` and
  the empty value.
- `full:<runner>` widens to every test (P2, MTT-V1-008): recorded as a known fail-closed limit;
  `TestMTTV1008RunnerSwitchWidensAndEchoesItsRunners` asserts the runners are echoed in order and
  the detail says the answer was widened.
- Unbounded echo (P2, MTT-V1-008): `selection.value` and the value quoted in `detail` keep at most
  256 bytes, cut at a UTF-8 boundary, with the marker `...(truncated, N bytes)`;
  `selection.runners` lists only runners wholly inside the kept bytes.
  `TestMTTV1008EchoedValueIsBounded` uses a 10 KB multibyte value and a 10 KB runner list.
- Drift test depth (P3, MTT-V1-003): the table records each input's name and description but no
  type or required flag, so comparing either would need new columns on about eighty inputs,
  several of them string-or-list unions. The requirement is narrowed to input property names,
  with the earlier wording kept in the spec.
- `--corpus-artifact` as a flag value (P3, MTT-V1-005): `removedGroupReason` skips the value of
  each value-taking flag; `TestMTTV1005RemovedGroupMatchesOnlyFlagPosition`.
- Old flow names (P3, MTT-V1-007): a call to a name in a row's `replaces` column stays the
  MCPV0-952 `unknown-tool` refusal, so the closed reason set is unchanged, and its `rule` names
  the replacing tool; `TestMTTV1007OldFlowsNameNamesTheNewTool`.
- Parity test lookup (P3, MTT-V1-009): `MissingParityTests` parses each `_test.go` file with
  `go/parser` and looks for a top-level function of that name;
  `TestMTTV1009MissingParityTestIsNamed` covers a renamed parameter, a method, a block comment, a
  raw string and a file that does not parse.

## Limits

The change folder's own spec delta (`docs/specs/openspec/changes/mcp-tool-table/specs/`) still
reads "input schemas" for MTT-V1-003; the canonical spec carries the narrowed wording.
