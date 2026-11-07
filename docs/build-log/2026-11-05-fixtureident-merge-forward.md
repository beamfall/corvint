# Fixture identity lane: merge mainline forward

The owner requested a merge of mainline
`c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514` into the fixture identity lane at
`908fcda019ecc07a48f4e19396e97db69d2f9157`, preserving both behaviours and all
existing commits. Merge commit: `64ce1d9834bab6b8936ea26aef316ec861c48b3c`.

## Conflict dispositions

- `cmd/corvint/context_summary_test.go`: keep the lane's shared filtered Git
  environment (CCF-V1-9800) and mainline's explicit fixture author/committer
  identity and dates (TCP-V0-024, TCP-V0-150). Fixture commit bytes stay pinned.
- `docs/specs/README.md`: retain mainline's Protected Pi Runtime status and the
  lane's CCF-V1-9800 range. Keep all other merged rows.

Generated `REQUIREMENTS.tsv` and `INDEX.json` were taken from mainline before the
merge commit. `make spec-requirements` owns both files and the README projection;
its post-merge regeneration restored the lane's requirement row. No requirement
ID collision was observed: CCF-V1-9800 was absent from mainline.

## Evidence boundary

The new binding uses the requested mainline base, not the lane's old base. The
owner explicitly requested stopping after dogfood-check, without a new seal,
and delegated broader verification to the host verifier and batch CI. Only the
conflict-resolved Go package and the requested spec and line-citation checks run
in this round. Earlier qualification remains historical evidence at its original
base; it does not establish fresh qualification of this composition.

No publication, Tasks mutation, history rewrite or repository-wide gate is
part of this round. Exact command results and final binding identifiers are
retained in the lane's external merge handback and private evidence directory.
