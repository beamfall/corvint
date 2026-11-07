# Fullplan verifier repair 1

Authority: the owner's fullplan Verifier repair 1 request, 2026-11-05. Base
`9db7a94169821d9b28701eaa88866aee2fec1083`; failed verifier head
`6c27e3902020a30c528cef835193b7edbab4b5f4`. Requirements: AZU-V0-9920,
AZU-V0-9922, STR-V0-069, AFP-V0-023 and AFP-V0-025. No native ticket was
supplied; native ticket writes and publication remain orchestrator-owned.

## Decision and repair

The graph's four newly indexed `.github` Go packages were absent from the
runner's `go list ./...` inventory. Discovery now independently enumerates
`.github` child directories when the planned universe includes helpers and
adds their explicit recursive patterns to the ordinary listing. Even
`./.github/...` skips its dot-named root: an initial real-probe regression
failed with the same missing-helper diagnostic and rejected that approach.
A directly indexed `.github` root package is listed explicitly too.

`TestHiddenHelperUniverse_AZU_V0_9920` uses the real Go probe, checking exact
agreement, a root helper package, an extra helper and a missing helper. The
existing difference comparison, diagnostics, exit behavior, JSON profiles,
manifest fields, nested-module handling and committed output goldens remain
unchanged. STR-V0-069 and AZU-V0-9920 record the inventory extension under the
owner's repair authorization; the existing spec registry entries are reused.

The two newly visible unbounded readers are bounded by empty external read
scopes for `.github/cishards` and `.github/testconfine/cmd`. Their complete
existing suites pass under the real Landlock wrapper with those declarations.
No unbounded-reader exemption is added. The static read-scope check also
mistook cishards' historical wall-time fixture package names for reads:
its retained cost literals now carry the existing fixture-path annotation.
All fixture string literals are unchanged (literal-sequence SHA256
`3a6c9dab76edd903a59e484b10c8b105b06f40ee66120a98aeff0d8a107fcad0`),
and no existing assertion was removed, weakened or rewritten.

The whole affected-package run then exposed another discovery consumer: the
standard-library gate selector still skipped `.github`, so it rejected the
new scoped keys. Its bounded index and both mirrored committed-scope readers
now admit only the root `.github` exception. `TestHiddenHelperScopes_AZU_V0_9920`
checks scoped helper selection, test-only versus production import closure,
and continued exclusion of other hidden paths and unlisted nested modules.
Added committed-scope parity cases exercise both readers against the same
Git tree, including nested and hidden refusal controls. Existing assertions
are retained. The initial whole-package failure (167.082 s) remains evidence;
the unchanged assertions pass in the final whole-package rerun (116.916 s)
after integrating the selector.

## Evidence and limits

Whole-package tests through `script/heavy.sh`, without an exported commit
identity: `internal/sessiontest` PASS (3.800 s); confined `.github/cishards`
PASS (6.256 s); confined `.github/testconfine/cmd` PASS (0.016 s);
`internal/liveverify/affected/golang` PASS (10.301 s after the mirrored-scope
repair); `internal/gateselect` PASS (2.084 s); Python plugin PASS (0.483 s). The focused
`unbounded-readers-check` and `test-read-scopes-check` pass. The handback
retains the complete command results, remaining original-lane package reruns,
spec and citation checks, raw-log digests, and final CEM check.

The initial read-scope check failed on the historical fixture literals before
their annotations; that failure log remains outside the repository. The first
spec-index regeneration refused unstaged specs; staging the two specs and
rerunning succeeded. Heavy admission waited 35 s for host load to fall from
53 to the configured 40; its semaphore configuration was preserved.

The owner's repair rules prohibit `corvint test`, `doc-gates`, selection
baseline checks, repository-wide tests, sealing and a local model reviewer.
Those checks are NOT_RUN here; host integration and independent review remain
orchestrator-owned. Native Darwin execution and general AZU zero-unknown
qualification are NOT_PRODUCED. This repair claims inventory and declaration
integration, not a passing host selection receipt or a performance improvement.

Rollback: revert this repair's inventory extension, two scoped declarations,
fixture annotations and matching spec amendments. Reverting the original
helper discovery remains the conservative fallback; do not change a golden
or weaken the universe comparison to hide missing packages.

Raw logs are retained under `/home/russell/lanes/fullplan-evidence/`. SHA256
bindings for the results above:

| Evidence | Log | SHA256 |
|---|---|---|
| sessiontest | `repair1-sessiontest-complete.log` | `16e2c0e5324de3593a43c68bf7fe344e3b41c26b504a6b741cee9bb4151b6cdb` |
| confined cishards | `repair1-cishards-final.log` | `8aa615a5c68e292e12c3e120529697c788250813b7b9062005d3a087f06f8d13` |
| confined wrapper command | `repair1-confine-cmd.log` | `4279fbb197caebdd395552600e8c4090c2b7aa82b129a68fb51797868d6a7632` |
| gate selector | `repair1-gateselect-terminal.log` | `a02c3585010ff49b1434bacc6c279a68acd85b33f95c6cbdcb0ecc51913fd549` |
| Go plugin | `repair1-golang-terminal.log` | `aea52b32a73dcc25ce5eb97a8243dc7545107af74e0e5d27aad826f34c266ed9` |
| Python plugin | `repair1-python-whole.log` | `d6d55e91475f3fc1982ccc085a6fecb1294d9cf7ec8f23febb030d06240a4505` |
| affected graph | `repair1-affected-terminal.log` | `7c04e2d5972cff3adfc9a5bbf803939ca210a23e58116fa63664401b932fb642` |
| read-scope checks | `repair1-read-checks-complete.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| spec/citation checks | `repair1-spec-lines-terminal.log` | `1a9d26fdeb1beb25cc01a99bdd44db1fa940851d56f890f0ff0db8cd49d36029` |
