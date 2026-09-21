# Decision 0318 — A remote provider transport is NO-GO on the default local path

Date: 2026-09-19. Status: accepted (owner call on Beamfall/corvint#11); the issue stays open for a
remote profile.

Decision 0325 accepts the separately built opt-in HTTPS profile on 2026-09-20. The separate-profile
gap is resolved; this decision's default-binary network prohibition remains binding.

## Decision

`EEP-TR-010`: no option, environment variable, or record member may cause the default Corvint
binary to open a network connection to fetch a provider record. Invariant 7 keeps the default
local product free of network dependency, and a network fetch would make a read command's result
depend on a remote party at run time.

An accepted remote profile needs its own spec and decision, and must provide: a path the default
binary cannot reach (separately built or separately enabled); TLS with a pinned server identity; a
credential policy that never places a secret in argv, a receipt, or a record; an explicit
per-invocation opt-in; the same byte, time, and record bounds as the command transport; the same
transport-neutral decode (`EEP-TR-005`); and closed failure (`EEP-TR-006`). Until then an operator
can fetch a record with their own tool and pass it as `--provider FILE` or through a local command.

## Rollback

Nothing ships; superseding this record requires the remote profile's own accepted decision.
