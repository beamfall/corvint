# Bugfix repair version scope correction

This corrects the package-version statement in
`2026-11-05-bugfix-self-review-repair.md`. AHI-020 and the Pi shipped-path set in
`script/check-host-package-versions.sh` exclude this round's README and test
changes. The initial 0.3.15 bump was unnecessary. Restore all seven synchronized
locations (including the CLI constant) to the repair input's 0.3.14 before final
binding. The final repair changes only the requested Pi contract, README and
negative test, with no Tasks runtime change. The attempted full CLI check remains
failed environment evidence; this correction does not turn it into a pass.
The final passing Pi run disables native Tasks write fixtures. The first run's
fixture-write restriction mistake remains recorded in the earlier entry and
handback. No native project-ticket completion or publication is authorized.

The full Pi suite rerun on the final 0.3.14 tree passes: 166 passed, eleven
skipped, native Tasks fixtures disabled. Final log SHA-256: `6788e7103d4afd17ffd815c84dd9ffdd98a92aa06b32ced62bcd6c91dcb4f1ac`.
