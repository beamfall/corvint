# The co-author trailer check runs through a make target — V1-3227

The first `doc-gates` trailer step (CIN-V0-006) carried its skip logic as inline shell. Batch-precheck
refused it: its sync check mirrors every `doc-gates` run command locally and accepts only `make
<targets>` or `go run ./path`. Decision (lane, within the ticket): the logic moves to the
`ai-coauthor-trailers-check` Makefile target and the step runs `make -s ai-coauthor-trailers-check`
with `BASE_SHA` from the pull request's base. The target runs the check script on a pull request,
fails a pull request run that has no `BASE_SHA`, and says so and passes on a merge-queue run, a
push, batch-precheck's local mirror, or a tree without the script.

Evidence: `TestCINV0006DocGatesRefusesAICoauthorTrailersOnPullRequests` runs the target against a
stub script for each case (PASS, with `TestAFPV0670`); `script/batch-precheck.sh --sync-check`
rc 0; `make doc-gates` rc 0. The step reached batch-precheck only after the first dogfood seal, so
the lane bound and sealed a second time.
