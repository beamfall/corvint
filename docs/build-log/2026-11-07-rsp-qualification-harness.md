# Runner qualification harness: retained experiments and core execution seam

Ticket: V1-5108. Requirements: RSP-V0-033..036, RSP-V0-056.
Lane base: `89372490721bb960ff9261a930c94f8514a2056e`.
Status: experimental implementation; tests written, NOT_RUN under the build-only charter.

The harness consumes the existing evidence identities, selector boundary classes,
language mutation declarations, and runner inventories/results. A core-owned Driver
verifies the pinned corpus, enumerates complete populations and captures FULL
experiments. It retains unchanged negative controls per claimed class, merge replay,
traced/untraced result parity, every independently seeded draw, misses, exact one-sided
95% upper bounds, and measured inventory/collection times. Unknown or capped inputs
remain unqualified with reasons. The output is a candidate, not admission authority.

File operators enumerate byte flips, truncation, listed/lookup additions, deletion,
rename, and tracked executable/configuration byte changes. Syntax-aware body,
declaration and server operators must name the language's mutation piece. The harness
does not copy existing language engines or invent syntax operators in the selector.
Samples are drawn with replacement from a canonically ordered complete population;
the record binds that population's digest. Unkilled samples never enter the trial
denominator. A zero observed miss count is accompanied by a positive upper bound.

The operator CLI evaluates retained captures against locally verified merge parent
pins, comparing the first parent with the merged result, not the topic parent.
It requires an explicit checkout, new output file, and cost ceiling. Retained
replay never duplicates one capture to manufacture multiple observed trials. It does
not execute tests or claim that a replay invocation is live qualification.

The pure requalification planner consumes only core-admitted records, considers the
newest admitted record for the runner, compares exact declarations and identities,
and emits a bounded job description while its plan remains FULL/requalifying.
The caller owns CI execution, deduplication and admission. No CI files or production
selection behavior change in this lane. An unset CI ceiling cannot schedule a job.

## External integration blocker and claim limits

At this base, `internal/testrunner/registry/runner_plugin.go` exposes Inventory,
Command, Parse and Evidence; it has no qualification declaration or mutation
generator method. `langplugin.SubPlugin` records mutation names, paths and suffixes.
The existing language mutation generators are private to their engine packages.
Consequently no production live Driver or registered mutation-provider adapter is
available to call from this lane without changing parallel-owned interfaces or
duplicating those engines. Live runner qualification and its CI execution remain
NOT_PRODUCED. The operator entry presently supports retained replay only. This
ticket is PARTIAL until those adapters and CI integration are supplied and qualified.

The accepted contract permits an untraced execution model, but this slice's parity
path supports traced experiments only; untraced declarations are explicitly
unsupported and remain FULL. No runner is promoted by these changes.

The V1-2766 preregistration was consulted for boundary-mutation provenance. Its
Go-specific private generators and 40-hour study budget are not silently substituted
for a runner's declared population or an unset CI ceiling. The host must provide
the shared pinned replay corpus and live captures before external outcome claims.

## Verification and rollback

Only formatting, package vet and spec/line-citation checks are authorized in this
build-only lane. Harness/CLI runtime tests, frozen evaluations, live measurements, independent
review, CEM binding/checking/sealing and ticket writes are deferred to integration.
Exact check return codes and requirement-to-test mappings are in the lane handback.
Remove this package and tool to roll back; no existing record store, wire output,
production plan, committed golden or release manifest was modified.
