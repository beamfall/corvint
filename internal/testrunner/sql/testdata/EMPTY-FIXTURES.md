# Empty native fixture packaging

`empty-fixtures.json` retains the named zero-byte captures as explicit empty JSON
strings with their original SHA-256 digests. The test loader reconstructs only
these entries, validates their empty bytes and digest, and refuses a declaration
that shadows a file. Missing undeclared fixtures remain file-read errors.

This packaging avoids metadata-only empty-file additions, which the current
canonical CEM 0.2 patch reader rejects with `expected --- header`. It changes no
native output bytes, parser behavior, report provenance or wire contract. The
original empty files remain in the expanded build checkout and private capture
evidence. All nonempty native fixtures and provenance records remain unchanged.
