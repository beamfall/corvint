# 2026-10-19 V1-2010: RSpec example selection, runtime identity and shared-example origin

Ticket: V1-2010 (GitHub #1271). Requirements: `LPCV-V0-131` to `LPCV-V0-135` in
`docs/specs/live-proof-carrying-verification-v0.md`. They are accepted under the owner's standing
auto-accept of lane requirements (2026-10-11).

## Problem

The RSpec provider (`LPCV-V0-090..095`) accepted only path `--spec` values and recorded no
selection. Under `--exec-prefix`, it recorded nothing about the Ruby that ran inside the prefix.
It also dropped RSpec's shared-group metadata. As a result, an agent could not tell whether
`spec/x_spec.rb:12` had matched an example, which Ruby and Bundler a container run used, or
where a shared example was included.

## Decisions

- **Optional members, same profile.** `selection`, `selectionWithheld`, `runtime`, `sharedExample`
  and `sharedExampleAbsent` are optional members of `corvint-ruby-test-provider/0`. Decoding stays
  closed. No member moves a projected axis.
- **Selectors.**
  - `path:line[:line...]` and `path[scoped-id,...]` are parsed strictly. A value whose suffix is
    not a well-formed list stays a plain path, which is how RSpec reads it.
  - Path-only values accepted before are still accepted.
  - New refusals: more than 256 values, a value over 1,024 bytes, and a line or id form that names
    only the project directory.
- **Outcomes come from the report.** Outcomes use the report's `id` (the rerun path and scoped id)
  and the declared `file_path`/`line_number`.
  - `UNATTRIBUTED` is separate from `NO_EXAMPLE`. A line inside an example, or a line naming a
    group, runs examples that the report cannot attribute to that line.
  - `NOT_EVALUATED` is recorded when there is no report.
- **Runtime identity comes from a separate probe of the same prefix.**
  - The probe is `ruby -e` with a fixed script that reads no project file. A run's own output
    cannot carry it without changing the user's command.
  - The container ID is named `probeContainer`, not the run's container. With
    `docker compose run --rm`, each invocation gets a sibling container.
  - The ID is taken from the `/etc/hostname` mount in `/proc/self/mountinfo`. That mount is
    specific to the container, whereas shm mounts on a Docker host can name other containers.
  - The image digest is always `unknown: not-observable-inside-prefix`. Nothing on the host is
    inspected.
- **Shared-example origin.**
  - It is read only from `shared_group_inclusion_backtrace` frames (`inclusion_location`).
  - The stock JSON formatter does not emit them, so stock runs record `not-reported`. This is
    distinct from `not-shared`, which means an empty array.
  - The provider ships no Ruby formatter.

## Byte identity

Retained documents are never rewritten. Legacy documents decode and project byte-identically:
`TestLPCVV0135RubyOptionalMembersDecodeClosed` compares the projection JSON of a legacy document
with the same document carrying every new member.

Newly produced documents do differ from those produced before:

- Every example now carries `sharedExampleAbsent` (or `sharedExample`). `LPCV-V0-134` requires
  this.
- Runs with `--spec` or `--exec-prefix` carry the new members.

No committed corpus, fixture or golden file contains a Ruby provider document, so no committed
bytes change.

## Measurements and evidence

Focused tests, run through the host semaphore one package at a time, all pass:

- `internal/rubytestprovider`, the whole package including `TestLPCVV0131..0134`. The real probe
  script ran under the host's `/usr/bin/ruby` 2.6.10 through a pass-through prefix and reported
  that version.
- `internal/testvaliditydoc`, the whole package including `TestLPCVV0135`.
- `cmd/corvint-ruby-test-provider` and `internal/rubyprofile`.

Fail-without-fix proof:

- `TestLPCVV0131..0134` all fail when the fix's runner calls are replaced by no-ops. The removed
  calls are the selection recording and evaluation, `probeRuntime`, and `observeShared`.
- `TestLPCVV0135` fails ("open shared absence accepted") when `Validate` stops calling the new
  validators.

Other checks:

- `go vet` on the three touched packages and `gofmt -l` are clean.
- `corvint affected` selected 106 units through `testvaliditydoc`'s reverse imports. Only the
  direct dependents above were run.

## NOT_RUN

- A real RSpec run, which needs the rspec gem. Installing it was not authorized.
- A real `docker compose` prefix, and therefore a real `probeContainer` value and its
  `mountinfo` shape.
- A project formatter that emits `shared_group_inclusion_backtrace`.
- The rest of the `corvint affected` selection, and `make gate`.

## Residuals

- A provider-supplied or documented JSON formatter that copies `shared_group_inclusion_backtrace`,
  so that `LPCV-V0-134` yields an origin on stock projects.
- A live container qualification of the runtime probe.

## Rollback

Revert the V1-2010 change. Documents that carry the new members are then refused by closed
decoding and skipped by discovery. Documents without them are unaffected.
