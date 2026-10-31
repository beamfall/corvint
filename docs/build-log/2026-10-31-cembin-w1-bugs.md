# cembin-w1: binary test fixtures in a dogfood range (V1-2752)

`cem prepare` refused every range that committed a binary file, even when the range also changed text:
the canonical patch carried a `Binary files … differ` section and the native parser refused it with
`binary-patch` (CEM-V1-037). A range that adds a test fixture could therefore never be dogfooded. The
contract change is `CEM-V1-980..986` in `docs/specs/cem-stable-v1.md`, committed on its own before
the implementation.

## Decisions

- **Native grammar extension, no wire change.** The parser accepts exactly Git's canonical
  full-index binary section (`diff --git`, the creation/deletion mode line when present, `index`
  with two full object IDs of the same width, and the exact `Binary files <old> and <new> differ`
  marker) as a binary group with no hunks. The map stays `cem/1.0`: no hunk, evidence or profile
  member is added, and the binary content is still bound, because `patchSha256` covers the
  section's object IDs through the CEM-V1-110 patch proof. Two alternatives were rejected:
  - An additive profile (as for `cem-non-regular/1`). It would need a flag through prepare, cite,
    verify, OCM and every dogfood script, which is not proportionate for a section the
    patch digest already binds.
  - Teaching the portable consumer and frozen packet. `protocol/cem-1.0/stable/` is digest-pinned,
    and changing it is a release-level act outside this lane.
- **Fail closed on the portable side (CEM-V1-986).** `interop/cem01-go` is unchanged. It refuses
  any binary marker with its `patch` code, so a map over a binary section is refused, never
  silently accepted, by the portable verifier. The dogfood scripts run the native verifier only.
  This divergence is the recorded cost of not changing the frozen packet. Teaching the portable
  consumer the same grammar needs its own packet revision.
- **Blob identity, checked against the base (CEM-V1-982).** For a modify or delete, simulation
  requires the base blob at the path to hash to the named object, using SHA-1 for 40 hex digits
  and SHA-256 for 64, and its mode to match. A create requires the destination to be absent. The
  target side is bound by the patch digest that verify re-derives from the target tree.
- **Size is reported, not wired (CEM-V1-983).** The prepare result gains a `binary` array, present
  only when the range has a binary section. Each row holds the path, change, mode, both object IDs,
  Git's object-header sizes, and `disposition:"excluded"`; the absent side is null. AC1 allows
  either blob hash and size or an explicit bounded exclusion. Sizes in the wire map would be a
  `cem/1.0` change for no verification gain, because the object ID already fixes the size.
- **A binary-only patch still refuses (CEM-V1-981).** A `cem/1.0` map needs at least one hunk.
  The refusal stays `binary-patch` and names the first binary path.
- **Every other binary shape keeps the CEM-V1-037 refusal.** That covers abbreviated or mixed-width
  IDs, a missing `index`, a rename, `GIT binary patch` and a marker naming other paths. The
  message now says that cem/1.0 binds only the canonical full-index section.
- **Cite refuses binary evidence (CEM-V1-984).** A span in a changed binary path refuses
  `cite-span-not-stable`, because binary content has no line classification. Citations into the
  text files of the same map work as before.
- **Bounds are unchanged (CEM-V1-985).** The existing patch (8 MiB) and blob (64/128 MiB) limits
  apply, and the header read uses the existing bounded `BlobHeaders` call.

## Evidence

The tests are named in the spec's traceability table. They pass under `script/heavy.sh` on the lane
worktree:

- `internal/cem/patch` and `internal/cem/sim` as whole packages.
- `internal/cem/workflow` with `-run 'TestCEMV198|TestPrepareBinaryRefusalNamesPath|TestCAXV0020|Prepare|Cite'`.
- `internal/cem/verify` with `-run 'Stable|Candidate|Structural|Binary|Create'`.
- The `interop/cem01-go` module as a whole.

`corvint test --dry-run` narrowed the change to 202 units, which is wide, so only focused packages ran.

`TestCEMV1983PrepareBindsBinaryFixture` drives the real producer and the native stable verifier over
a Git range. The range modifies one binary fixture, adds a second and edits a text file. The verifier
reaches stage `complete` with outcome `ACCEPT`.

## Follow-ups

- The dogfood refusal guidance does not yet reflect this change. It sits in
  `internal/dogfoodflow/propose.go` (default case) and `docs/DOGFOOD.md` §4 step 1, and still tells
  the caller to drop any binary file. That fix still works, but only a binary-only or non-canonical
  binary change now needs it.
- `CEM-V1-986` portable parity needs a packet revision before external verifiers accept such maps.

## Review fixes

The independent review found two gaps, and both are fixed with regression tests that fail on the
earlier code.

- **Over-bound binaries (CEM-V1-985).** Binary bodies are never read, so the existing blob bound
  never fired. A 64 MiB + 1 binary creation beside a text change prepared, and `verify-stable`
  accepted it. `verify.BinaryBlobSizes` now reads every binary object header, base and target
  side, and refuses `repository-object-unavailable` naming the path. Prepare, the candidate
  verifier and `verify-stable` share it and run it before base simulation.
- **Empty rename lines (CEM-V1-980).** The parser refused rename metadata only when it had a
  value, so `rename from ` with an empty value let a binary section through. It now tracks the
  metadata's presence. Copy and similarity lines already refused. A textual group still parses
  exactly as before. Whether the textual grammar should also refuse empty rename values is left
  as a follow-up, because the shared tamper matrix pins textual verdicts for both consumers.
