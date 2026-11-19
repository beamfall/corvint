# V1-5694 merged command catalog and fixture ownership

The verifier at `60004668d` found a stale watch refusal, a command count from before the
line-evidence merge, and a fixture owner that still named the ownership audit.

Both merge parents independently identify the expected core routes. The B2 parent
`2560ca70a` supplies 50. Adding the `line-evidence` topic and its `refresh` verb
under LEV-V1-001 gives the same set as taking the 72 routes at `6fe6ffc0c` and
removing the 20 routes under the command families retired by RMV-V1-008.
The CAP-V0-047 regression checks the exact line-evidence routes and the original B2 count.
It also substitutes an unknown child for `refresh` in memory to exercise rejection at
the same total count.

LEV-V1-012 requires watch to read supported non-Go indexes. The catalog now describes
the shared planner and restricts the package and wrapper flags to legacy Go indexes.
The interactive-alpha fixture owner names the retained-evidence regression that exercises
the fixture under RMV-V1-009. AFP-V0-10329 reserves the standalone audit owner for paths
without an exercising test.
