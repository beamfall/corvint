# Captured native fixture packaging

`report-fixtures.json` retains exact captured report bytes as explicit base64
strings with their original SHA-256 digests, including the nine named empty
captures previously held in `empty-fixtures.json`. The test-only loader requires
a declared name, decodable bytes and matching hash. Missing names and declarations
that shadow physical files fail.

This lossless packaging preserves the canonical CEM admitted-path bound. It
changes no native output, production parser, provenance or wire contract. Original
raw files remain in the expanded build checkout and private capture evidence.
SQL source fixtures and provenance remain separate and unchanged.
