# Supervisor effort and wall policy: squashed current-main re-integration

PR456 (issue 354, CAL-V0-062/063) conflicted with public MAIN after its DD8
composition. This branch is based on MAIN
`4620e3436ceddc7e9b1ccb392a93f157155a7154`. The owner
declined a status override for that head and asked for an ordinary re-integration
that passes its checks normally. This is that re-integration. It is not a
qualification of non-low effort, longer stages or V1-0475 completion.

## Composition decision

The new branch starts at MAIN and carries one squashed copy of the net PR456
change, taken as the diff from the merge base
`dd8cc0ca918a80faaa041c98a783de11550b2bcc` to the sealed head
`59a1f59702da611f764adba42af289812601a320`. The six Go files, the supervision
guide and the four earlier build-log entries are byte-identical to that head.
The earlier entries stay as history; the commits they name are not ancestors of
this branch.

Only the requirement catalog conflicted. It was regenerated from the staged
spec, so it equals MAIN's catalog plus the CAL-V0-062/063 rows, with later
rows' line numbers shifted. The spec, INDEX and README edits applied without
conflict, and MAIN's newer issue 503 and issue 494 text remains. MAIN's
reservation sentence and slices table now name S13 as defined, matching the
earlier S14 sentence correction. The sealed DD8 archive bound a commit that
this branch does not contain, so it is not carried. Every MAIN archive under
`.corvint/changes` stays byte-identical.

The PR page listed `.github/testconfine/**` and `.github/workflows/ci.yml` only
because its recorded base `c2c7fc12988d3e957d51fba66c7033b9eb53376b` predates
the MAIN commits `67171e06`, `022eaa46`, `d22fbe14` and `73ec646e`, which
made those edits. They
are already in DD8 and MAIN. The issue 354 change needs no `.github` edit, and
this branch does not touch `.github`.

## Evidence boundary

The 14 checks and the seal from the DD8 head bound content that this branch
does not contain. They are not reused here. Fresh focused tests, a fresh CEM
and a local outcome are required for this branch's own commits. Provider-applied effort, stages
beyond one hour and multi-host/multi-repository supervision remain NOT_RUN or
NOT_OBSERVED. V1-0475 and issue 354 remain OPEN/PARTIAL.

Rollback reverts the branch's commits. Recorded program configs keep their
digests because `stageEfforts` is omitted when absent.
