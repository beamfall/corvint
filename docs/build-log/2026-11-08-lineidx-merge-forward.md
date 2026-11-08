# lineidx merge-forward to 16ba0021

The owner requested a merge of mainline `16ba0021ab0aa92d244ad1b8d54b607d12bc4ed3`
into `claude/lineidx` at `842f1ff4f194e53d0eb59581da407371a4623f06`, preserving
both sides, followed by a whole-lane rebind and seal against that mainline commit.
No pre-existing commit is rewritten. The merge is `eddbd5f8be5d84849a31fb5036040cb269757c07`.

The collector composition retains mainline's independently rebuilt child-binary
provenance and cumulative before/after coverage windows together with the lane's
environment audit, quiet collector reads, bounded package concurrency and suite
union. Reader selection retains both the lane's `Readers` entry point and
mainline's hidden-package and typed-reach helpers. CI retains the line-narrowed
FULL-record exclusion and mainline's revised control-plane policy. Prose conflicts
combine the lane's added test citation with mainline's newer waiver behavior;
line-only citation conflicts are repinned after inspecting their subjects.
There were no colliding new requirement IDs. Generated spec and capability data
and the changed use-case receipt are regenerated from the merged inputs.

The first focused `internal/linecov` package run failed: the new build-proof
helper's collector-internal `os.Environ` call was counted as a test's inherited
whole-environment read. This caused conservative `whole(environment)` fallbacks
and broke the existing environment-binding and negative-control assertions.
The helper now uses the same quiet environment accessor as the launch audit.
Child-build proof requirements and the environment completeness checks remain.
The existing package tests exercise their composition; no assertion is weakened.

The handback outside the repository retains the final focused test results,
bind/check/seal identities, each conflict resolution and the numbered friction
list. Pre-change receipts remain stale at their original base; guarding tests,
fresh live mutation qualification and external outcomes are not claimed by this
merge-forward round. Broad selections and host gates are reserved for the host
verifier by the owner's instruction.

The new-base binding preflight refused two inherited read-scope gaps: mainline's
self-use benchmark builds `cmd/corvint`, whose merged import closure now includes
`internal/linecov`, and the lane's workflow qualification test reads the CI
workflow. The existing read-scope declarations now name those exact inputs;
no path is suppressed or marked as a fixture to evade a real read.
