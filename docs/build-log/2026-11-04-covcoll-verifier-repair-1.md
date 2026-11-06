# Native collector verifier repair 1

Human intent: repair only the host verifier failures for lane covcoll against
base `39c03198086681fb24f509fbd00b34696d3268c4`.

The new `internal/nativelinecov` package inherited an unbounded-reader classification
through `testrunner` -> `testvaliditydoc` -> `jstestprovider`. Its tests instead use
in-memory reports and private temporary fixture/configuration paths; they do not execute
the imported repository-root locator or read other repository directories. Declare
`internal/nativelinecov: []` in `.corvint/test-read-scopes.json` under AFP-V0-023.
No unbounded-reader exception is needed. Import dependency selection remains intact.
This applies the existing contract; no spec or product behavior changes.

Both context golden failures reported the same fixture tree and JSON except for the
commit ID: `05d3142624a106eafdac3a4f644b6f44eacaf3eb` instead of the frozen
`9ab69ef47d196e1a757a3de3319527eb3686ff8b`. The existing fixture sets `user.name=t`,
`user.email=t@x` and fixed dates, but inherits Git identity environment variables.
The host verification inherited the exported seal identity, which overrides that config.
Running the unchanged frozen checks with GIT_AUTHOR_NAME, GIT_AUTHOR_EMAIL,
GIT_COMMITTER_NAME and GIT_COMMITTER_EMAIL unset passes. Export Russell Lewis
<me@russe11.com> again before every dogfood bind or repository commit. No product code,
golden, fixture, assertion, error text or wire contract was changed for this failure.
Read-only `git hash-object -t commit --stdin` over the fixture tree, pinned dates
and message independently reproduces both IDs by changing only author/committer
identity from `t <t@x>` to `Russell Lewis <me@russe11.com>`.

Focused verification through `script/heavy.sh` passed the requested
`Freeze|GPKV0002|Golden` selection in `cmd/corvint` and the whole existing
`internal/nativelinecov` package. Raw logs and affected-plan output remain under
`/home/russell/lanes/covcoll-evidence/repair1-*`. Independent review remains the
orchestrator's responsibility. All previous missing-native-toolchain and missing-audit-
collector limitations remain; this repair does not qualify those capabilities.
