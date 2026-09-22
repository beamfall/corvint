# GPK-V0-020 repaired Go archive assessment — 2026-08-31

This assessment binds the repaired canonical archive gate to commit
`7638cf336de935bb106ebe37f13488d8b831fd16` and tree
`c85a59c55fad0d6ff32d09b1cbe5dd1558d4a711`. It is local technical evidence, not a retained
candidate, publication receipt, signature, tag, or promotion decision.

`make go-archive-gate` ran from that exact clean revision under a private external temporary parent.
For every manifest target it first ran and consumed the existing loose reproducibility gate's two
cold-cache binaries, then compared them with two separately exported immutable-tree builds. It
assembled each archive twice in separate directories and passed the result through the separately
compiled offline verifier. That verifier independently bound the existing loose-gate binary,
archive binary, inner checksum, legal bytes, build information, pinned target, canonical container,
and double-assembly evidence. The retained output and its temporary parent were deleted by the gate's
bounded cleanup after validation.

The canonical private witness was a `0600` file beneath the invoking user's real `0700` Git-private
directory and reported `PASS` with the following exact archive rows:

| Archive | SHA-256 | Bytes |
|---|---|---:|
| `corvint_darwin_amd64.tar.gz` | `58a6fdb131ee649f40901c84ec18e0cc868cc7083aa1d25d9c153e0ae4dd727b` | 5,524,075 |
| `corvint_darwin_arm64.tar.gz` | `3a7124ca2a6f8dc92f7318136315c6ad6e6e12a583e3f1025953320e66520b90` | 5,155,488 |
| `corvint_linux_amd64.tar.gz` | `8c4fd3e18a347d8d14057e45e64cde10f96c57acaca7aadcd98190fff29ff8ba` | 5,482,238 |
| `corvint_linux_arm64.tar.gz` | `f3fadf5f015eb70e9edcd41a0df2da77a97f4c2751bce7cf7cf1090cfd065010` | 4,992,231 |
| `corvint_windows_amd64.zip` | `fc9e41cccb0a3892d7dc2f3750b69a6774c155e30bc2136d0e5a921980db735e` | 9,824,144 |

Each archive contained exactly its target binary, `LICENSE`, `LICENSE-APACHE-2.0`, `LICENSING.md`,
`PROVENANCE.md`, and `SHA256SUMS` beneath the canonical root. The archive-notice obligation of
`GPK-V0-020` is therefore `PASS` for this exact revision. Commercial terms and legal conclusions
remain outside this evidence. Packet 5, wheel publication, tag, publication, promotion, signing,
native execution on cross-built targets, and cross-builder reproducibility remain unproved and
unauthorized.
