# 2026-10-20 Docs batch: Pi catalog routing decision (V1-2022)

Decision (delegated in-ticket fork, conservative option): the Spec Kit command texts now link
`docs/CAPABILITIES.md` and the Spec Kit extension and OpenSpec skill metadata versions advance from
0.1.0 to 0.1.1 because their shipped content changed (AHI-020 spirit; `check-host-package-versions.sh`
does not yet cover the `protocol/integrations` packages). Pi is not routed in this change: its host
tuple is pinned, so any shipped Pi content change would move the tuple and need a version bump and
re-qualification owned by the Pi lane. Pi users still reach the catalog through the root README and
`docs/README.md`. Revisit when the Pi tuple next moves.
