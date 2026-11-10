# ACR-V1-012 — Edit `AGENTS.md` product invariants 3, 4, 5 and 7; pin them with invariant 8

Ticket V1-5272, change `acceptance-record`, task 3.2. Base: `origin/main`
`2913fdc7e73d9e73524ef30f5ef03a69c371cc86`, branch `claude/agents-invariants-acr012`. Authority:
decision 0596 (owner-merged 2026-11-10), section 7 superseded-text table, and owner decisions H4
and H5 (2026-11-10). No key signs the record (H14), so the signed tag of task 3.1 does not apply
and the owner-merged decision file is the precondition this edit waited for.

## What changed

- Invariant 3 is now "Project-owned authority outranks syntax and history."
- Invariant 4 gains one sentence: agent hook session state kept in the user cache directory is not
  repository or trace state (decision D17, needed by AGT-V1-022). The `--learn-slot-weights`
  clause the table quotes was already absent on the base; nothing was left to drop.
- Invariant 5 is now "Corvint 1.0 learns nothing from use. Any later learning must be explicit,
  local, bounded, secret-screened and evaluation-gated under its own decision."
- Invariant 7 keeps its citations of `go-production-kernel-migration-v0.md` and of decision 0591
  with `core-bundled-apps-v0.md`, and drops the citations of
  `deployment-neutral-index-platform-v0.md` and of decision 0081 with
  `local-admin-console-v0.md`. No other word of invariant 7 changed.
- Invariant 8 is byte-identical to the base (SHA-256 of its block before and after:
  `d6d029e73c0db3a311ea5867a6bf2e619c17c2d9d5f1ee62464017710035bb55`).

## Decision made in the lane

The record gives no literal new text for invariant 7, only the rule "citations of active
documents stay and those of archived documents are dropped". The choice of which two citations
count as archived is this lane's reading, not owner wording: the local admin console spec is
retired by decision 0594, and plan row G22 (accepted with the plan) archives the
deployment-neutral index spec. Both files still exist on the base until the removal batches move
them. If the owner reads the rule differently, the fix is one edit in this change plus a new pin.

## The check

`script/agents-invariants-check.sh` reads the "Product invariants" section, fails when invariant
8 lacks "human-owned intent", and fails when the text of invariants 3, 4, 5, 7 and 8 differs
from a SHA-256 pinned in the script, naming the last commit on the file. `ACR-V1-012` asks the
audit to refuse an invariant edit from another change. A commit does not record which change it
belongs to, so the pin stands in for that: any edit fails until the pin moves, and the pin lives
in a script whose header names the owning change. Rejected: matching the change name in commit
messages, which a foreign commit can copy and an honest one can forget.

`script/agents-invariants-check_test.sh` covers the committed file, the dropped phrase, an edit
of each of invariants 3, 4, 5 and 7, an edit of invariant 1 and of another section (passes), a
missing invariant, and an absent file. It is registered in `GATE_STEPS` and `shell-test`, so the
pin is enforced wherever the shell tests run.

## Verification and retained limits

- `script/agents-invariants-check_test.sh`: pass, macOS bash 3.2. Linux: `NOT_RUN` locally.
- `make unbounded-readers-check shell-test-registration-check`: pass.
- Retained limit: no workflow runs the check by itself; it runs only as a shell test. A direct
  doc-gates step would be a `.github` change and belongs to the CI trim (B0).
- Retained limit: the pin proves the text is unchanged, not who changed it. A change that edits
  an invariant and moves the pin in one commit passes; review of the pin line is the control.
- Tasks 1.2, 1.3, 2.1, 2.6, 2.7, 2.8 and 3.3 of the change are still open under V1-5272. Tasks
  2.2 to 2.5 were delivered by decision 0596 and `docs/specs/openspec/acceptance-log.tsv`
  (PR 172); task 3.0 is the recorded "no key" answer.

## Rollback

Revert this commit. The four invariants return to the base text, which contradicts decision 0596
until the edit is made again; the check and its test leave with it.
