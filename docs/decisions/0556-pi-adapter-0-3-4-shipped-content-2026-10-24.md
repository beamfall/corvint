# Decision 0556 — Pi adapter 0.3.4 for the PWV-V0-013..017 shipped content

Date: 2026-10-24. Status: accepted by the orchestrator. It applies the existing `AHI-020` and
`AHI-071` rules and adds no new requirement.

## Context

Decision 0550 bumped the Pi adapter from 0.3.2 to 0.3.3 with the `AHI-071` schema change. The Pi
host-integration lane (beamfall/corvint#1654, PWV-V0-013..017) then changed shipped files in
`integrations/pi/` after that bump: the four shipped skills, tool annotations, the Tasks
`search`/`list` selectors and the overview/features/witness reads. Commit `1c314c308` was the last
of those changes. `AHI-020` requires the package manifest version to move with shipped content, so
`make host-package-versions-check` failed.

## Decision

The Pi adapter is bumped from 0.3.3 to 0.3.4. That supersedes the 0.99.1 / 0.3.3 tuple (decision
0550) in AHI-024, AHI-071, LCP-V0-008/009, the PWV-V0 inventory, the package, both compatibility
manifests, the native `adapter pi` and `dogfood event` admission, and the runtime and workflow
guards. Adapter 0.3.3 now refuses visibly. The host version stays 0.99.1 and support stays
FALLBACK only. Recorded evidence fields (`lastConformance`, `hostVersionEvidenceAdapterVersion`)
keep naming 0.3.3, the adapter version that evidence was produced on; a live-host run on 0.3.4 is
not part of this decision.

## Rollback

Revert the change. Restoring 0.3.3 requires replacing the Pi package and the Corvint binary as a
pair, and reverting the shipped-content commits so `AHI-020` holds again.
