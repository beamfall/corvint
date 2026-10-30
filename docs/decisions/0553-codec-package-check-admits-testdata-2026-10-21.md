# Decision 0553 — The protected codec package check admits `testdata`

Date: 2026-10-21. Status: accepted under the owner's standing auto-accept of lane requirements
(2026-10-11). V1-2036 permits this outcome: its first criterion is met either by a real enrollment
or by narrowing `onlyCodecPackage` with a decision. This is not an owner release or promotion
decision.

## Problem

V1-2036 reported that the protected local-authority command refuses any target tree that is not
codec-only. Checked against the code at `ef2a01db5`, that is not quite what happens.
`onlyCodecPackage` (`tools/local-authority/capsule.go`) reads only the target's `internal/wp3codec`
directory, and the rest of the tree is unconstrained. Inside that directory it admitted only
`codec.go` and regular `*_test.go` files. This repository's real package also holds a `testdata`
directory, so a Corvint-shaped target was refused with `unsupported package entry` before any
evidence was published.

## Decision

The check stays scoped to the package directory. It now also admits one `testdata` entry of tree
mode `40000`, whose subtree it never reads (`PLE-V0-013`). The Go toolchain never compiles
`testdata` into a package, and the protected recipe compiles only the admitted `codec.go` with its
protected wrapper. So the reason for the check still holds: no undeclared production source can
disappear from the attested subject. Every other entry is still refused, including:

- another `.go` file, assembly or `syso`;
- an executable or symlinked test file;
- a `testdata` entry that is a file, a symlink or a submodule.

## Not decided here

A live protected enrollment over a real map stays NOT_RUN. It needs the root-owned authority store
and accounts, which a lane does not create. This repository's own tree also exceeds the capsule's
1,024-object and 8,192-entry closure bounds, so a real enrollment needs a smaller repository.
V1-2036's second criterion therefore remains open.

## Rollback

Remove the `testdata` branch from `onlyCodecPackage`, `PLE-V0-013` and
`TestPLEV0013CodecPackageAdmitsTestdata`. No enrollment, store or published evidence depends on it.
