# RCF qualification currency gate: V1-5692

Date: 2026-11-18
Requirements: RCF-V1-003, RCF-V1-010, RCF-V1-013.
Status: PARTIAL pending host workflow integration and candidate evidence.

## Decision and implementation

A selected package pass does not prove that `TestRCFV1008Qualification` ran. Add the release
support tool `tools/rcf-currency`; leave the opt-in test and all product contracts unchanged.
The charter's last qualification is recorded in `docs/specs/rcf-qualification.json`: commit
`0bc3087bd1ef475c832eb7cfbb59f2a5ce35d949`, report SHA-256
`51800e0dcdfb250e23acf0deab35e9a75a533283022b7f6ffb3f69bab92138bd`.
This is the owner's supplied historical evidence pin, not a newly reproduced qualification.

Derive patterns from both immutable revisions' selection owners naming the qualification test.
Use the existing affected owner matcher and disable Git rename detection so both names count.
Any selection-file change conservatively requires qualification, including removal of an owner.
No trigger change reuses the committed pin. Otherwise require a clean candidate checkout and
absolute live configuration, refuse an existing report destination, observe the named uncached
live test and package pass, and check the fresh report's frozen-query digest and repository results.
Two uncached ranking-test passes are also required; the existing test compares repeated ranks.
The retained gate output binds the fresh report digest to the candidate commit. This avoids adding
a candidate field to the frozen qualification JSON format. Configured tests run via `heavy.sh`.

This gate runs only on release dispatch. Hosted runners cannot supply the private proof corpora,
so the integration deliberately leaves live configuration empty: changed triggers fail closed.
The owner runs the tool at the clean candidate with the existing private configuration, retains
its candidate-bound log and report, then updates the committed pin to that qualified commit and
report digest. A following commit that only updates the pin can reuse that qualification.
A failed or skipped run must never refresh the pin. No new qualification is claimed by this lane.

## Acceptance evidence

`TestCurrency` exercises reuse, current live success, trigger additions/deletions/renames, removed
owners, absent/invalid pins and selection, missing configuration, skipped/unselected/failed tests,
missing/stale/failing reports, incorrect query digest, incomplete repositories, ranking failure,
and dirty or mismatched candidates. `TestNamedTestPass` rejects skipped or absent named tests even
when a package passes. `TestTriggerPatterns` rejects malformed trigger declarations.

Focused verification passed as recorded below. The live three-repository
qualification is NOT_RUN: its private configuration and corpora were not supplied to this lane.
The readiness record `docs/build-log/2026-11-18-rc4-readiness.md` and qualified Git object
are absent at base `55decc6a041a1df3f8ae43e95ba21cea669e7d80`. AC3 remains pending host
integration.
When that record is available, append only this enforcement citation to row 13, retaining its HOLD:
`Enforced by go run ./tools/rcf-currency --candidate FULL_SHA; see the rcf-currency gate run.`
Replace the run reference with the actual dispatched job URL after execution; none is invented here.

## Exact host integration patch

The charter prohibits `.github/` edits. Apply this patch on the host; no workflow file was edited.
The existing workflow anchors provide full history, exact checkout identity and Go 1.27.4.
The job's nonzero exit blocks the dispatched workflow and always retains its log and digest.

```diff
--- a/.github/workflows/release-gates.yml
+++ b/.github/workflows/release-gates.yml
@@ -205,4 +205,31 @@
           if-no-files-found: error

+  # RCF-V1-010/013: release dispatch only; ordinary PRs and nightlies do not run this job.
+  rcf-currency:
+    if: github.event_name == 'workflow_dispatch'
+    runs-on: ubuntu-24.04
+    timeout-minutes: 120
+    env:
+      RUNNER_LABEL: ubuntu-24.04
+      # Hosted runners reuse the committed pin only when trigger inputs are unchanged.
+      # A trigger diff fails closed; refresh the pin after an owner-run live qualification.
+      CORVINT_RCF_QUALIFY: ""
+    steps:
+      - *check-inputs
+      - *checkout
+      - *verify-checkout
+      - *setup-go
+      - *provision
+      - *gate-log-helper
+      - name: Enforce RCF qualification currency
+        run: |
+          "$RUNNER_TEMP/run-gate" rcf-currency go run ./tools/rcf-currency --candidate "$SHA"
+      - if: always()
+        uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1
+        with:
+          name: rcf-currency-${{ inputs.release }}-ubuntu-24.04
+          path: ${{ runner.temp }}/evidence/rcf-currency-*
+          if-no-files-found: error
+
   # linux/amd64 Core platform evidence (decision 0420 item 2, PRS-V1-004, PRS-V1-006): release
   # runbook steps 5, 6 and 8, run natively on this sha's own
```

## Limits and rollback

AC1's executable check and AC2's regressions are implemented. AC1's hosted invocation and AC3's
readiness gate-run citation are NOT_PRODUCED until the host applies the patch and runs the gate.
Ordinary main PR behavior is unchanged. No frozen product output, golden or qualification report
shape changes. Independent review is assigned to the orchestrator, not performed in this lane.
Revert the tool, pin and workflow integration together; restore the manual RCF-V1-010 readiness
hold. Removing enforcement cannot be counted as qualification evidence.

## Verification and friction

All commands used `TMPDIR=/home/russell/lanes/rcf-enforce.tmp`, without exported Git identity.

- PASS: `script/heavy.sh go test -count=1 -timeout 30m ./tools/rcf-currency`.
  Whole package, no filter; package time 1.468 seconds. Includes the subprocess adapter regression
  `TestLiveTest`, which rejects skip/fail events from a temporary wrapper.
- PASS: `script/heavy.sh go vet ./tools/rcf-currency`.
- PASS: `make -s spec-requirements-check line-citations-check openspec-check`.
  OpenSpec reports 65 baselined findings; OSC-V1-003/004 are NOT_RUN without a base argument.
  The citation check skips 184 references whose basenames are not tracked at repository root.
- PASS: `make spec-requirements`; `make requirement-id-collisions` found one new ID, no collision.
  `git grep` on `origin/main` found no RCF-V1-013 through RCF-V1-016 definitions before editing.
- PASS: `script/check-line-citations.sh --repin` (no unchanged citation target moved).
- PASS: `git apply --check` on the exact workflow patch above, without applying it.

Raw logs are outside the repository at `~/lanes/rcf-enforce-evidence/`. SHA-256 digests:


- `package-test.log`:
  `f967e13e67f6af2adbe0f2c6bde4ddf7635f79d907dd0ec84470b9ec3dcdc1c0`.

- `vet.log`:
  `8b39f1bee84aae368082987ad89ea1a6d10d067b635b6ab752d8092031d1aadf`.

- `doc-checks-final.log`:
  `cfc53cb0451d061b79ae9cc2bf665af0e2f32b811fcbd37e2199fb9ac67f7a85`.

Friction and repros:

1. `corvint affected --base 55decc6a041a1df3f8ae43e95ba21cea669e7d80` exited 2 with
   `unsupported-affected-graph`: `.corvint/test-selection.json` declares tree reader
   `script/github-split-check.sh`, but no driver unit was observed. Selection coverage is UNKNOWN;
   the touched package was run directly through the required heavy wrapper.
2. `git cat-file -t 0bc3087bd1ef475c832eb7cfbb59f2a5ce35d949` cannot find the qualified object
   in this lane. The actual candidate reuse verdict is NOT_RUN until the host supplies history.
3. `git ls-files docs/build-log/2026-11-18-rc4-readiness.md` returns no file at this base.
   Row 13's narrowly scoped citation must be applied after host integration of that record.
4. The charter prohibits `.github/` edits. Workflow activation is NOT_PRODUCED here; apply the
   exact patch above on the host and retain the dispatched job URL in readiness row 13.

Dogfood, CEM and seal are NOT_RUN under the charter's retirement override. Repository-wide gates,
Windows vet (no push), model-backed review and live qualification are NOT_RUN under this lane's
scope or because their required evidence is unavailable. No push or task-store write was performed.
