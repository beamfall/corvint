# Agent evidence routes

This is a reading aid, not accepted intent or a delivery claim. Use native Corvint first under the
installed skill and [dogfood contract](DOGFOOD.md#1-orient-with-corvint). Retain its omissions,
uncertainty and unsupported results. A ranked document is a lead, not the governing answer.
Read the matching row, then expand only the relevant original sources.

| Question | First source and expansion | Boundary / next action |
|---|---|---|
| Select features for Corvint development | [Shared stage guide](SELF-DEVELOPMENT.md) → owning spec and actual CLI help. | Corvint itself or explicit repository adoption only; select applicable routes and preserve actual receipts/exclusions, not an all-command loop or FULL claim. |
| Starting a change | [AGENTS.md](../AGENTS.md) → [dogfood loop](DOGFOOD.md#required-loop-for-substantive-changes) and [authority order](SPEC-DRIVEN-DEVELOPMENT.md#authority) | Freeze the base and measurement before context. For an enrolled local change follow [local completion](DOGFOOD.md#enrolled-local-completion-policy); final CEM binding/checking remains post-commit under DOGFOOD §4. |
| What capability exists? | Select matching records from [INDEX.json](specs/INDEX.json); read that spec's Agent digest and its named sections. | Keep intent, delivery, dependencies, supersession and promotion evidence separate. An accepted spec can have no executable delivery. |
| Which experimental verb answers this? | [Decision 0089](decisions/0089-agentic-completeness-wave-2026-09-11.md) names the eight wave verbs and their owning specs; read that spec's Agent digest, then `corvint VERB --help`. | Every one is `proposed` intent and `experimental` delivery: read its receipt, do not let another capability consume it. `depsource` abstains with `module-not-required` in this repository, `calibrate` returns empty buckets until `internal/trace/record.go` carries a packet stance, and `surprise` refuses on a dirty worktree. |
| Does implementation meet intent? | Spec requirement → implementation/evidence table → exact definition and test named there. Use [REQUIREMENTS.tsv](specs/REQUIREMENTS.tsv) for the clause location. | Follow acceptance provenance to the owner decision when status conflicts. Code/tests cannot accept intent; a test mention is not a passing run. |
| Why did a command fail? | Preserve the exact error/receipt; search its code with `rg -n -F 'ERROR_CODE' internal cmd`, then read the matching implementation, tests and owning requirement. | Check profile, revision and worktree preconditions before changing code. Unsupported-by-design is a boundary; never discard local work to satisfy a read command. |
| Review a change | [CEM non-claims](CHANGE-EVIDENCE-MAP.md#positioning-and-non-claims), [OCM digest](specs/ocm-v0-dogfood.md#agent-digest), then [reviewer reports](DOGFOOD.md#6-review-what-another-reviewer-sees). | Inspect mapped hunks, every scoped requirement, unknowns and test witnesses. Structural closure does not establish semantic support, test adequacy or a passing gate. |
| Resume or select work | If this checkout has an initialized `.taskman` store, read its native queue and current receipt first; otherwise route to a workspace with that store. Use [ROADMAP history](../ROADMAP.md#current-build-queue-indispensable-context-and-cumulative-development-savings), [0.6 portfolio](PORTFOLIO-0.6.md), `corvint-tasks roadmap` / `corvint-tasks ticket search --label agent-memory` (then `ticket show`), and the owning spec for intent. | Native store presence makes it operative for execution status; never initialize a replacement or treat AT/E/U Markdown as live state. Search `docs/BUILD-LOG.md` and `docs/build-log/` by heading or ID, then read matching entries. Re-pin status and coordinate owners; preserve pending gates and owner-goal gaps. |
| Verify documentation | [Focused checks](#focused-documentation-checks), then the owning spec's test/measurement matrix. | Focused checks do not replace `make gate`, frozen evaluations, independent review or dogfood. Coordinate full gates with an existing runner. |

## A task names a requirement

For file discovery when the task names a requirement ID, use the experimental task-context packet:

```sh
corvint context --task "THE ORIGINAL TASK INCLUDING ITS REQUIREMENT ID" --limit 5
```

Inspect `spec-mentioned` rows and `critical_missing`/unexamined scope. Start at the returned
path and clause line with a bounded original-source window; extend it when needed to read the
complete clause and relevant qualifications. Use [REQUIREMENTS.tsv](specs/REQUIREMENTS.tsv) when a
clause location is unresolved, rather than repeating a locator the packet already supplied.
When binding an expansion to evidence, compare the packet's `revision` with the captured commit's
Git **tree** identity, then verify the path's blob identity; `revision` is not a commit ID.

A named requirement does not establish implementation or execution. Follow its witnesses to the
necessary code, tests and actual run records, retaining unknowns. Scope searches to the named
witness paths before widening. For large JSON/tool results, project relevant status, counts, fields
and bounded excerpts before forwarding them into the conversation, when supported. Requested
per-item limits do not guarantee a bound on the whole result; narrow the request when safe
projection is unavailable. Preserve the original through existing evidence handling, including
commit/sample identity, checks and outcomes, failures, timeouts, incomplete capture and unknowns.
State omissions and return to the original immutable bytes when they matter. A projection is a
reading aid, not a new authenticated receipt or proof of completeness.

Use `impact` when an affected-path question remains; another search or impact packet is not
required merely because the command exists. This route is discovery and reading guidance, not a
claim that the packet covers every task constraint or that fewer bytes prove lower task cost.

## After a trace-state refusal

`query` can return `unsupported-query-trace-state` on a clean repository after a successful
`record`. The authority-start profile intentionally excludes a present local trace store under
`GPK-V0-028`; repository/agent-tooling queries have a separate trace-consuming profile under
`GPK-V0-043/044`. This refusal does not show that the trace is corrupt. Preserve the task, error,
exit status and measurement; do not remove traces, dirty the tree, or rewrite the task to change
the selected profile. See [decision 0011](decisions/0011-standalone-query-trace-state-acceptance-2026-09-01.md).

For workflow/queue questions, open [AGENTS.md](../AGENTS.md), the
[dogfood loop](DOGFOOD.md#required-loop-for-substantive-changes), and the
[historical queue](../ROADMAP.md#current-build-queue-indispensable-context-and-cumulative-development-savings); use an initialized native `.taskman` store for current execution status.
Those original sources establish the workflow; the failed query stays failed. When file discovery
is also useful, the separate experimental command can use the same task unchanged:

```sh
corvint context --task "THE SAME TASK" --limit 5
```

Keep its syntax/relationship evidence and uncertainty separate from governing owner prose.
`context` does not consume local trace learning and its success does not qualify authority-start
query support. Charge the refusal, fallback, and subsequent source reads to the same task.

## Token-efficient process without model or cache changes

Optimize the complete verified task, including retrieval, reasoning, failed attempts, review,
setup and verification. Smaller packets or fewer visible commands alone do not establish savings.
Keep model selection, reasoning settings, prompt framing/serialization and cache keys, lifetimes
and automatic cache behavior unchanged when refining this reading process. Do not compress source
into invented shorthand or drop qualifications to meet a byte target.

Before another lookup, check whether the current packet or retained notes already contain the
needed immutable handle. In the same agent, reuse instruction text already supplied or read while
its identity and relevance remain valid; a visible `@` reference does not prove its body was read.
Verify pinned source identities when asserting immutable evidence, and reopen originals for
identity/relevance changes, conflicts or missing qualifications. Fresh independent reviewers still
read the relevant originals. Batch bounded independent reads, retain short notes, and stop when
the scoped answer is supported; record unresolved evidence instead of seeking reassurance.

Before another gate or review, check the existing result, reviewed scope and covered source
identities. Reuse applicable evidence, run focused checks for the actual delta, and run the
canonical gate when required by the repository contract. A later documentation-only addendum
must name the earlier code-gate revision and its own checks; do not relabel a previous run as a
new one. Freeze bounded experiments before execution and retain losing runs. Report observed
bytes as bytes, and leave tokens, cache use and complete cost unknown when host evidence is absent.

## From index to exact evidence

Filter the index instead of reading every record; replace the search term for the task:

```sh
jq '.[] | select([.path,.title,.reqPrefix] | join(" ") | test("checkpoint|falsifiable"; "i"))' docs/specs/INDEX.json
rg -n '^FPK-V0-023\t' docs/specs/REQUIREMENTS.tsv
```

The TSV is a locator, not an authority or obligation-completeness check. Read the original clause
and relevant non-goals/failure modes. For OCM, confirm the spec uses the executable
[requirement form](SPEC-DRIVEN-DEVELOPMENT.md#required-capability-spec-shape); index presence alone
does not prove that `ocm prepare` enumerates a clause. Locate named tests with `rg -n -F` inside the
owning implementation directory, then open their assertions. If a pointer is stale, use `rg --files`
in that directory before guessing a filename. Record missing evidence or unresolved conflicts.

Return the scoped answer, owner clause/decision, implementation/test witness, observed gate state,
remaining uncertainty and next action. Use stable headings or requirement IDs for navigation;
pin Git identities when asserting evidence. A short answer must still expose an open obligation.

## Focused documentation checks

```sh
nice -n 15 make spec-requirements-check requirement-definitions-check traceability-tests-check decision-numbers-check line-citations-check
nice -n 15 env GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go test -count=1 ./internal/specindex
```

`internal/specindex` checks index/header/digest/README consistency; the Make targets check
requirement locators, definitions, named tests, decision numbering and line-citation shape.
Neither proves link meaning, semantic agreement, owner acceptance or exhaustive OCM enumeration.
After moving clauses, regenerate the TSV with `script/gen-spec-requirements.sh` and inspect its
diff. Keep IDs and historical decisions stable. Full verification remains [Makefile](../Makefile)
and [DOGFOOD.md](DOGFOOD.md); report anything not run with its reason.
