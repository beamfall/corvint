# GPK-V0-020 Go archive assessment — 2026-08-31

This assessment binds the first complete real archive run to commit
`6a4db962f4e1b39c638699bf475930dc8737cd92` and tree
`b5fa0b092109a84e8442fe3afb69950301f21828`. It is historical evidence, not a retained candidate,
publication receipt, signature, tag, or promotion decision.

The canonical command ran `conformance/release-artifact-v0 archive --revision HEAD` with an absent
destination under an invoking-user-owned `0700` external parent. It returned `PASS` after two raw
committed-tree exports, two cold builds per target, two assemblies per target, and independent
offline verification. The path-free report recorded manifest SHA-256
`e09dee701e771c8938a68a7cdb9aef1b62acc120a6c3050fcecfb678fa2ab17b`, six members per archive,
a derived maximum archive bound of 10,871,871 bytes, and a derived five-archive bound of 52,731,185
bytes.

| Target | Binary SHA-256 / bytes | Archive SHA-256 / bytes |
|---|---|---|
| darwin/amd64 | `d098d33ec608ff89d7307d2aab87a778527a67a1475041b8285b1a615d9a42b8` / 9,755,952 | `8333a206dcb27485dfffbed4bf29ab365c7c1fb6e41d148a22f19ac44c1a465d` / 5,524,073 |
| darwin/arm64 | `9e51ecad9e626d30b6d43c099cb2fa77217bf61e0e7e475a88d9c2c86275c909` / 9,047,682 | `dc1cb9e5cac75d451de12a44194958787f6cb45ac8b64b4f162f22f62528b372` / 5,155,445 |
| linux/amd64 | `d286f84d8c0be7cab50f9dbb988e98a979987d6610af4e702dd55ae76dfb193b` / 9,703,593 | `8862c6e84f57105caf4558355cb586f52e71fbaa423e22ed09525b15200cbafb` / 5,482,236 |
| linux/arm64 | `17d606bfcc2470b5b3c2e1abf3df2001b25840817c8107c8446471cb748f1a90` / 8,950,683 | `0f623fe5e0ced77a3c6351b44e2c39fa1e0096e2b28587ce1291135352c4fd5c` / 4,992,231 |
| windows/amd64 | `c976e56226a96996d3580b20b419d355c70b1ace48d6ce697eecb550dbc9bc12` / 9,771,520 | `f2061aa7dc62a8bf92db348713cb24a8eac3953dc2af479626b51de77a888324` / 9,824,144 |

Each archive contained exactly its target binary, `LICENSE`, `LICENSE-APACHE-2.0`, `LICENSING.md`,
`PROVENANCE.md`, and `SHA256SUMS` beneath the canonical root. The verifier compared every legal
member with the exact raw blob exported from the named commit and with the manifest digest. The
archive-notice obligation of `GPK-V0-020` is therefore `PASS` for this exact revision. Commercial
terms and legal conclusions remain outside this technical evidence.

The private witness was a canonical `0600` file beneath the invoking user's real `0700` Git-private
directory, and the read-only checklist reported the Go-archive row `PASS`. Packet 5, wheel, tag,
publication, promotion, signing, native execution on cross-built targets, and cross-builder
reproducibility are not proved by this assessment.
