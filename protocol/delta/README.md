# Delta record, experimental version 0

SPDX-License-Identifier: Apache-2.0

`schema.json` defines the proposed source-content-free output. Repository paths are relative; OIDs
are full lowercase SHA-1 or SHA-256. Digests and opaque IDs are lowercase SHA-256. Internal digests
use `corvint-delta/0:` followed by their domain and a NUL separator. Provider input digests are raw
SHA-256 over captured bytes. The changed-path digest hashes the complete sorted path/mode/object
entry delta, including CEM paths, with the `paths` domain. Renames remain delete/add pairs.

Output carries no source or ticket text and confers no execution authority. Confidence stays
`unscored`; test execution and runtime coverage are unknown. Complete input bindings include
explicit immutable revisions and captured provider/prior-generation records. Named checkout
freshness is observed read-only; unreadable or dirty counterparts forbid narrowing.

HTML documentation is currently withheld because the reused flowdocs reader cannot meet this
profile's pre-allocation bound. Native lexical exclusions and unsupported languages retain explicit
incomplete evidence. CLI dispatch/integration and full completion evidence are separate gates.
