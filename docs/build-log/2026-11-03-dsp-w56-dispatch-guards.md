# Lane dispatch evidence and host readiness

Lane: dsp-w56. Base: `5d1ed52b17d61147bd8c04b08afa6da39fa80afb`.
Tickets: V1-3494 (including V1-3654 screen criteria), V1-3867, V1-3761.

The owner requested a dispatch refusal for already assigned/delivered work, stale charter
references, unpublished in-flight refs and stale host binaries. Requirements `ASD-V0-2200`
through `ASD-V0-2203` use the owner's standing acceptance and remain experimentally delivered.
The owning spec had 244 obligations before editing; generation counts 248 afterwards, below 256.

The screen reads Git objects and explicit/registered JSON or Markdown charters. It retains
the original commit subject, including partial AC labels, and explicit duplicate evidence;
neither becomes proof of full acceptance. Overrides name tickets and remain visible. Missing
charter packages, pins and occupied ID ranges require charter repair. The registry and fetched
refs define the observation boundary: undisclosed remote work is not claimed observed. Ticket
bodies are screened together to avoid repeating the branch scan for each ticket.

Ref publication is an explicit non-forced push of every declared source tip to every declared
host, followed by exact-tip checks. Dispatch checks before issuing its receipt; partial host
publication remains a recoverable refusal, with no atomicity or rewind claim. The binary check
retains AFP-V0-846 source-equivalence semantics and now checks fetched origin/main before lane
creation, so an older lane base cannot conceal a stale host tool.

Evidence: `make lane-screen-test` passes 57 assertions using ticket fixtures, two local bare
host repositories and fake corvint binaries. Existing binary-check and dispatch/base-check
fixture groups pass separately. `make spec-requirements`, `make spec-requirements-check`,
`make line-citations-check`, `make flow-guide-check` and `bash script/check-bash32_test.sh`
pass in an isolated verification clone with its own staged index. No Go files changed; gofmt and Windows vet have
no changed inputs. The first fixture run failed two refusal exit-code assertions; explicit
exit 2 fixed both, and subsequent final runs pass. No whole repository gate was requested.
The staged Bash 3.2 scanner also caught three unguarded array expansions; the compatible
array-presence idiom repaired them. Its final scan passes 233 scripts, with its optional
Git-directory pass stamp initially unretained because the shared metadata is read-only; the
verification clone retains its own stamp. Exporting the private index into Go fixture tests
contaminated that scratch index and produced 578 false missing-file citations. Restoring it
and rerunning the gates in a clone with ordinary independent Git metadata fixes the isolation
problem; the implementation and owning spec did not change for that repair.

Dogfood start was attempted with the installed build 49 binary and refused at a read-only
shared Git directory. Pre-change `corvint affected` selected nothing on the clean base;
the initial changed-tree plan reports UNKNOWN, 75 selected units and 78 unknowns; the final
plan including handback/build-log files selects 76 with 79 unknowns. Focused dispatch
fixtures and the mandated document/spec checks were selected; the broad Makefile/document
readers do not imply exhaustive test execution. The full plan and local command logs are in
`/tmp/dsp-w56-*`; the acceptance mapping and retained limits are in `dsp-w56.handback.md`.

Limits: independent review, committed CEM/reviewer report, post-commit binding/check/seal and
native task completion are NOT_PRODUCED under this session's command restrictions and
read-only shared Git metadata. The post-merge wsl-lanes run is NOT_MET-pending-operator;
the handback gives its exact command. No live host publication, ticket mutation, repository
commit or repository push was performed. Git commits/pushes in the tests are confined to
temporary fixture repositories. The ID allocator refuses four-digit assigned ranges;
a direct scan found ASD-V0-2200..2209 free at the base and all 676 open refs.

Rollback: revert the new helper, its dispatch integrations and origin/main precheck together;
retain the existing HEAD binary check. No persisted screen state or queue migration exists.

## Review fixes

An independent review found two P2 defects; both reproduced against HEAD and are fixed.

- Malformed Markdown declarations (`ASD-V0-2201`): the per-field regexes skipped a recognized
  field line whose value did not parse, so `Pin: go.mod@bogus` or `ID-block: ASD-V0 nope 2209`
  left the constraint undeclared and the screen could pass. Every recognized field line
  (`Tickets:`, `Package:`, `Pin:`, `ID-block:`, `Branch:`, `Host-repo:`, `Status:`) now must parse
  or the screen refuses with exit 2. Prose without a field prefix is still ignored.
- Relative host paths (`ASD-V0-2202`): every Git call runs under `-C REPO`, so a relative
  `--host-repo` or charter `Host-repo:` path resolved beneath the source repository. A fixture
  with a decoy bare repository under `--repo` showed publication to the decoy and a passing
  verification against it. Filesystem host paths are now made absolute against the invocation
  directory first. URLs (`scheme://`), scp-like `HOST:PATH` and slash-free configured remote
  names of `--repo` pass through unchanged, and the resolved host is what `HOST_REF` lines report.

Evidence: `script/lane-screen_test.pl` has 78 assertions, all passing. 17 of them fail with the
HEAD script: 14 Markdown refusals, plus the resolved host name, the invocation-relative host's
refs and the empty decoy. Limit: Git `url.*.insteadOf` rewrites of a bare path and
Windows drive-letter paths are not modelled.
