# CEM 0.3 packet: the zero-byte `json-empty` map is untracked

Date: 2026-10-11. Ticket context: V1-0632 batch (#808), V1-0609.

## Finding

Binding the #808 batch refused `malformed-patch` ("expected --- file header") at the stage that
adds `protocol/cem-0.3/vectors/maps/json-empty.json`. Git emits a new empty file with no `---`/`+++`
headers. The CEM 0.2/0.3 patch grammar requires those headers, so the creation cannot be bound.
V1-0609 records this as a known capability limit. The product fix (metadata-capable binding in
CEM 1.0) stays open there.

## Decision

Follow the V1-0609 fixture precedent and keep the case without a tracked zero-byte file. The
manifest bytes, their pinned digest, the 221 cases and `artifactSha256` are unchanged. A pinned
artifact whose digest is the SHA-256 of zero bytes is not tracked. Both consumers rebuild it as
zero bytes from the pin:

- `readPacket03` (interop): `TestCEMSM016PortablePacketConformance`.
- `TestCEMSM016NativePortable03Vectors` (native workflow). It also fails if the file reappears.

`TestCEMSM014PacketReproducible` still generates every artifact byte for byte. Its write mode skips
zero-byte artifacts, and its check mode requires them to be absent. The `protocol/cem-0.3/README.md`
layout row states the rule.

## Evidence

- go1.27.2: interop CEM-SM tests and `internal/cem/workflow` Portable03 tests PASS. Both run
  `json-empty`, which refuses wire `invalid-json` as declared.
- go1.27.1: `TestCEMSM014PacketReproducible` PASS.
- `go vet` is clean.

## Rollback

Revert the commit. This restores the tracked zero-byte file, which then has to be bound some other
way.
