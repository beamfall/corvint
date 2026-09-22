# CEM 0.1 independent-result submission

A local PASS is necessary but insufficient for an independent implementation-matrix entry. Submit
only after reviewing the owner-only observation and deciding explicitly to publish it.

## Required evidence

- public implementation repository and exact commit;
- implementation owner/team and language;
- dependency tree;
- packet manifest SHA-256;
- observation `packetSha256`, `corvintCommit`, and `corvintTreeState`;
- proof that the public commit has `corvintTreeState: clean` and recomputes the same framed packet
  digest over the eight files named in `START-HERE.md`;
- observation file SHA-256 and the reviewed observation;
- first and final run outcomes, including failures and unsupported cells;
- any runner-level aborts recorded separately by the submitter, because incomplete matrices are not
  written into the observation;
- elapsed human-active, agent, and wall time reported separately; and
- every clarification requested while implementing the contract.

## Independence attestation

State that the implementation:

- was authored by a person/team independent of Corvint and the other consumer;
- read only this packet plus standard language/library/Git documentation;
- did not inspect, copy, translate, link, import, or subprocess a Corvint verifier or the
  Corvint-authored Go portability probe;
- does not branch on fixture names or copy expected decisions; and
- does not share a protocol implementation library with another matrix entry.

Agent-generated work may qualify only when an unrelated external team owns and reviews it and the
agent received only the clean-room inputs above. Record the agent/tool and its elapsed time. A
wrapper, generated port of Corvint code, or second CLI over one implementation does not qualify.
The manifest-derived passing executable in Corvint's runner tests branches on public fixture digests;
it is harness plumbing only and can never qualify as independent semantic or conformance evidence.

## Non-claims and privacy

The observation includes no source bodies or absolute paths, but implementation and fixture digests
remain identifiers. Do not submit private-repository data, prompts, raw logs, environment captures,
or command output. No result is uploaded automatically. The owner-only sibling lock is advisory:
use an OS sandbox for an untrusted implementation or hostile same-UID process.

Public expected outcomes make fixture special-casing possible. Until a committed, non-public,
byte-distinct qualification set exists, an external source review is mandatory and the result must
say `sealedAudit: NOT_BUILT`. This packet establishes reproducibility evidence, not resistance to
gaming.

Producer execution is also `NOT_BUILT`; do not claim `P1`. Consumer `C1` and `C2` remain separate
implementations by separate owners, and both must later accept the same independent producer output
before the complete interoperability matrix can pass.

The frozen public drift suite contains one evidence item per case. A local PASS therefore does not
establish multi-evidence ordering or strict full-surface conformance. That claim requires a versioned
next suite with a multi-evidence vector; do not infer it from adapter prose.
