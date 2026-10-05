## 2026-10-04 V1-0545: reference pipeline runs `corvint delta`

Human-owned intent: the ticket V1-0545 (#398) asks for reference CI host templates and a host
adapter contract for the post-merge workflow. The delta job in `postmerge.yml` still printed
`delta NOT_PRODUCED corvint-delta-not-yet-published`, and the graph listed `corvint delta` as a
pending command. `corvint delta` has since been delivered (#525).

### Change

Specified as the extended `PCH-V0-001`/`PCH-V0-003` and the new `PCH-V0-015`.

- **Delta job.** It runs with `contents: read`, no secret and no persisted credentials. It checks
  out the merged change with full history as data, builds the pinned Corvint, resolves the first
  parent with `git rev-parse --verify`, and runs `corvint delta --base PARENT --head CHANGE`. It
  uploads only the canonical `corvint-delta/0` record.
- **Graph.** The delta step lists `corvint delta` under `commands` and has no pending command.
- **Audit table.** Four new single mutations, now 90 in total, are each refused:
  - an undocumented Corvint command in the delta job;
  - a Corvint call before the pinned install;
  - a secret in the delta job;
  - a write permission in the delta job.
- **Local conformance.** `TestTemplateDeltaReplayConformance` runs the shipped `resolve` and
  delta scripts unmodified under bash and POSIX sh against a fixture `--no-ff` merge, using a
  locally built `corvint`. Each step starts in `GITHUB_WORKSPACE`, as on a hosted runner.
  - The record must be byte-equal to a direct `corvint delta` of the merge's first parent and head.
  - A root commit and an absent change must fail the step and leave no record.

### Evidence

- `go test ./internal/postmergehost ./cmd/corvint-postmerge-host-launcher` passes.
- Two mutants of the delta script were killed by the new test:
  - `^1` changed to `^2` (fails the merge case in both shells);
  - `|| true` after the parent lookup (fails the root and absent cases).

### Limits

- No hosted run. The checkout, setup-go and upload actions did not run.
- No later step consumes the record yet, and the record's `build` field is the binary's default
  label because `install-pinned.sh` sets no link-time flags.
- Physical isolation, a hosted dry-run of the #395 replay set, and owner acceptance of PCH-V0
  remain pending.

### Rollback

Revert the delta job, the graph entry and the tests. This restores the `NOT_PRODUCED` placeholder
and the pending `corvint delta`. Operators who copied the template revert their copy.
