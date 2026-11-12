# Historical benchmark

This harness is historical after decision 0594 (2026-11-09). It runs commands that decision
removed (`corvint context`, `corvint dogfood` and `make dogfood-change`), so it no longer runs
end to end. The receipts and runs are retained as evidence, and the frozen preregistration is
not edited. A new benchmark needs a new preregistered revision.

Decision 0597 (2026-11-11) also removed `corvint context --task`, the treatment of the
`orientation` job (`UC-TASK-ORIENTATION`). No kept surface takes task text and names files, so that
job is retired rather than retargeted; `harness.py` still names it because each preregistration
seals the harness digest (`harnessSha256`) and the harness is not edited. A new run needs a new
preregistered revision without the orientation job.
