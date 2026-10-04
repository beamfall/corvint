# Experimental test runner companion

This optional command executes trusted local test code through explicit runner
profiles and records native outcomes. It is not included in the Core release and
is not a CEM1.0 promotion or a Tasks completion command.

Build with `go build ./cmd/corvint-test-runner`. `runners` lists current profile
IDs. Prepare a JSON request with an absolute source `root`, a **new** absolute
`reportDir`, runner, project/config/reporter fields required by that profile,
`timeoutSeconds` and `inputFiles` mapping source-relative names to SHA256. An
`expectedTests` array names the native identities whose presence must be checked;
runner-specific `selectors` choose execution. Full dependency closure remains
`NOT_OBSERVED`.

```sh
corvint-test-runner plan --request request.json \
  --executable /absolute/pinned/runner --executable-sha256 SHA256 \
  --out /absolute/new/plan.json
corvint-test-runner run --plan /absolute/new/plan.json --approve PLAN_SHA256 \
  --executable /absolute/pinned/runner --executable-sha256 SHA256 \
  --experimental --trusted-local --out /absolute/new/receipt.json
```

The plan digest is printed to stderr. Review the entire request and derived
invocation before approving that exact digest. Profiles using auxiliary tools
also require `--tools /absolute/operator-tools.json` on both operations. Its Tool
map must match the reviewed request and contains independently configured
executable paths and SHA256 values; a plan cannot choose those executables.
Configuration and caller reporter bytes are pinned before and after execution.
Profile-generated reporter templates are checked throughout execution.

`run` reserves a new receipt file outside the source root before launching tests.
Native failure, flaky, incomplete or refused execution returns nonzero while
retaining its receipt when execution started. Raw bounded phase output and fresh
reports remain in `reportDir`. The receipt distinguishes observed complete
inventory from expected inventory, semantic adequacy, coverage and criterion
acceptance. Missing/contradictory reports and execution drift become UNKNOWN.

Timeout/cancellation retires ordinary process-group descendants on qualified
Darwin/Linux profiles. This is not an operating-system sandbox. Executables and
tests may perform trusted project actions. PATH contains declared tool
directories; other files in those directories and undeclared external
runtime/device/dependency state are not attested. Other operating systems refuse
this executor pending containment qualification. Each adapter README retains
its actual tool/version/runtime qualification and unavailable rows.

Native Tasks capture/verification and CEM hunk binding are separate experimental
contracts. This companion's execution receipt alone grants no live gate,
integration, mutation-kill or release authority.
