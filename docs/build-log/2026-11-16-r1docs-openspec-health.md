# 2026-11-16: pinned OpenSpec validation and flat-spec health (V1-5549)

Requirements: OSC-V1-001 through OSC-V1-012; PRS-V1-018 carry-over unchanged.
Base: `df315e35056d15c7933b5ff97c605cedf5ef4919`.

## Decision and changes

The owner authorizes the missing health headers in this lane. Add only the missing
headers: 125 RETIRE for already-retired delivery headers, 18 MERGE for the cleanup
inventory's merge rows, and 145 KEEP for the remaining live/pending flat documents.
Existing Health lines are unchanged. KEEP preserves the present document; it does not
promote delivery or override a pending removal decision. MERGE does not perform a
merge. No delivery header, requirement body or acceptance marker is changed. The
proposed cleanup inventory is a disposition aid, not new accepted product intent.
Swift and Kotlin remain in 1.0 under decision 0600; neither is newly retired here.

The 288 OSC-V1-008 entries leave `tools/openspec-check/baseline.txt`. The checker now
reports **PASS, 0 failures, 65 baselined findings in 29 subjects, 0 reports**. The
remaining findings belong to other change folders; no new baseline entry is added.
`make spec-requirements` regenerates INDEX.json and README.md. REQUIREMENTS.tsv is
byte-identical to the lane base.

Tasks 2.10 and 3.1 are ticked with this proof; tasks 1.2 through 2.9 were already
ticked and their fixture package passes again. Task 2.11 retains its ID, unchecked
state and proof, and moves after the repository work in execution order: the ticket
explicitly excludes its separate CI PR. This preserves the tick-order rule without
claiming CI installation. The build-log portion of 3.2 is done; its native Tasks
completion stays unchecked because the owner prohibits task-store writes in this lane.
The broader spec-set-cleanup task 2.4 is not ticked: it also requires archive markings
and earlier cleanup tasks outside V1-5549.

## Commands and observed results

- The first npm install failed because `/home/agent/.npm` is read-only. Retried with
  `npm_config_cache=/home/agent/.cache/npm`, installing outside the repository at
  `/home/agent/lanes/r1docs-evidence/openspec`.
- `git grep -n '1.17.0' -- Makefile docs/specs/openspec-check-v1.md` agrees with the
  installed command's `--version`: **1.17.0**. No other version ran.
- `OPENSPEC=/home/agent/lanes/r1docs-evidence/openspec/node_modules/.bin/openspec make openspec-validate`:
  **PASS**, normal validation 44 passed / 0 failed. Advisory `--strict`: 3 passed /
  41 failed, 196 warning lines. The normal pass does not turn strict failures into passes.
- With that bin directory on PATH, `make openspec-validate-test`: **ok**; the real
  command refused a fixture requirement with no scenario.
- With the original PATH, `make openspec-validate`: **NOT_RUN**, command not installed,
  exit 0 outside CI. The fixture script also exercises absent/wrong-version CI refusals.
- `corvint affected --base df315e35056d15c7933b5ff97c605cedf5ef4919`: selects
  tools/openspec-check and internal/specindex; scope UNKNOWN, 4 unknowns retained.
- `script/heavy.sh go test -count=1 -timeout 30m ./tools/openspec-check`: **PASS**,
  whole existing package, 2.735 s. `script/heavy.sh go vet ./tools/openspec-check`: **PASS**.
- `make openspec-check`: **PASS** with the counts above, including the final task order.

No checker code, frozen wire field, golden, core-freeze output or `.github/` file
changes. No new requirement IDs. Revert the headers, generated projections and task
ticks together to roll back; restore only the corresponding removed baseline findings.

## Evidence bindings

Raw logs are under `/home/agent/lanes/r1docs-evidence/`; SHA-256:

- `openspec-validate.log`: `8e1fcba37c20e9656e72629845d1500176e745d7006050464dd7ef037106fb3c`.
- `openspec-validate-test-live.log`: `6070241d5c6da4448d3115d7e25a780ccbbad885b827fe44dab1271b1ccc92c5`.
- `openspec-absent.log`: `9720ee2c141771cdea9dbc71fc678007715b463fb127d2b35d7af48a70eaba18`.
- `openspec-check-final.log`: `b09dfe00b7b4a73548f5f3c701ed6a069bbd58a6fa657abd05173e37bc9e7d4c`.
- `openspec-package.log`: `10eec3b666c9f3ffe03878606e3855063e00ea6b6f3c45fd2440a9b663d1a953`.
- `openspec-vet.log`: `7630608b5c7e9f61662ec4914c955057e1d2f235fcffd66eef07de532520c6f1`.

## Citation repair

The Health insertions move 76 existing line citations. Automatic repinning preserves
their content. Two generated README anchors change with the added health cells. The
NOT_RUN examples still support the glossary and are rehashed at their original rows.
The other anchor already named Test Consolidation Planner at the lane base, despite
the glossary calling it “Unknown Frontier outcome gate”. No matching programme row
exists in this tree. Remove that unsupported programme-name claim and retain the
repository-frontier definition with its existing ARCHITECTURE evidence, rather than
rehashing a row that says something else. This is a citation repair, not product intent.

The required `make focused-docs` run found that adding Health moved the Human
Documentation Compiler digest from line 40 to line 41. Joining two existing
authority-input lines restores the unchanged digest to line 40; no assertion or
40-line bound is weakened. The focused-docs attempt is retained as failed; its
remaining component checks and the repaired specindex package are run separately.

After that layout repair, `make -s spec-requirements-check line-citations-check`
passed, including the whole `internal/specindex` package (1.818 s). The remaining
focused-docs components were run directly: requirement definitions, traceability
tests, decision numbers and openspec-check all passed. The original focused-docs
invocation remains failed; it was not rerun or relabelled as a pass.
