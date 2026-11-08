# Close startup, open and mapping provenance gaps

V1-5106 repair; RSP-V0-006, RSP-V0-007, RSP-V0-021 and RSP-V0-022.
The isolated backend remains unqualified and unintegrated; this is no runner admission.

## Decisions and closed classes

- Create provenance requires an ordinary successful open with a supported flag
  combination. The closed bit mask admits access mode, create, exclusive, noctty,
  truncate, append, nonblock, dsync, direct, largefile, directory, nofollow, noatime,
  cloexec, sync and path-only. Path-only opens only record a dependency: Linux
  ignores their create/exclusive/truncate/access flags. Ordinary opens additionally
  require access mode 0, 1 or 2, exclusive only with create, truncate only with write
  access, and no directory/create combination. Unknown bits/combinations mark the
  trace incomplete with the existing unsupported-syscall reason and retain reads;
  they cannot populate the created-path map. Unnamed temporary-file creation is
  outside this allowlist.
- Initial executable and working-directory paths now use the same closed path
  walker as traced pathname operations, before resuming user instructions. The
  walker admits ordinary directories, regular files, textual symlinks (40 hops),
  and the previously specified missing-suffix grammar. It retains intermediate
  links and applies the existing filesystem-matching allowlist at every traversed
  prefix as well as the initial lexical and resolved names. Magic links, unknown
  filesystem matching and other unsupported path forms remain incomplete.
- Only private mappings with the understood anonymous, fixed, denywrite and
  executable modifier bits and read/write/execute protection bits are certified
  as lacking a shared-memory peer. Shared and shared-validate mappings mark external
  incomplete with the existing descriptor-peer-unknown reason, even under a declared
  temporary root. Every other unknown mapping form also fails closed. File mappings
  still retain their reads. No ownership or peer-provenance proof is claimed.

These decisions repair existing requirements without changing shapes, field names,
error/exit codes, diagnostic strings, goldens or existing test assertions. Rollback
is to revert this repair and keep the isolated tracing service unqualified; reverting
must not promote the original incomplete dependency model to selection authority.

## Evidence

`TestRSPV0007OpenProvenance` changes an existing file's byte between runs after
O_PATH plus create/exclusive/truncate; reads remain present with no invented write.
Its unknown-bit live open and unsupported-combination checks cannot establish ownership.
`TestRSPV0007StartupLinks` changes only the middle executable or directory link;
both change the exit result and retain both links. `TestRSPV0022UnknownStartupPaths`
requires incomplete for procfs magic-link startup forms.
`TestRSPV0021SharedMemoryPeer` exchanges repository-dependent bytes between the
pre-existing test process and the tracee through shared/shared-validate mappings of
an outside temporary file; the repository read is deliberately outside the traced
process tree and external remains incomplete. Its private-map control cannot publish
writes to the peer. `TestRSPV0022UnknownMappingForms` checks unknown type and flag bits.

The first whole-package run failed because host /bin/true introduced an unrelated
futex syscall in the startup fixture. Tiny compiled exit-code executables replace
that fixture, retaining the completeness assertions. The next whole-package run
passed all existing and new tests in 3.802 seconds; focused vet passed. Both commands
used script/heavy.sh with the configured semaphore and no exported Git identity.
Raw evidence is outside Git under /home/russell/lanes/rspt4-evidence/self-review-*.
Affected selection against 8b80e602 selected proctrace; overall scope UNKNOWN,
74 unknown units (73 LANGUAGE_FRONTIER, 1 NO_SELECTABLE_TEST), retained for the host.

Independent review, full-corpus qualification/integration, broad gates and native
completion remain NOT_RUN under the repair-round boundary. Prechange evidence is
reused; final binding/checking is recorded in the external lane handback.

Evidence SHA-256 digests (raw logs are not committed):

- Passing package log: `44bbfc01559a9b2463ab56c1d78dd3a15804c5892004c1ae14d2ba8c796f5feb`.
- Failed fixture run: `50f09d294556f1a3ad6c70d1193acdf1d046b9b9f80577d617570d8e45765cc4`.
- Focused vet: `05281b836735cbf8cd7fc3f11922caaa6e24469b068224ce2ab671fef4742461`.
