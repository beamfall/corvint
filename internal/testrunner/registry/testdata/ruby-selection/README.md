# Ruby selection fixtures

Hand-authored fixtures for RSP-V0-001 through RSP-V0-005 and RSP-V0-048.
These are native report-shaped parser inputs, not captured runtime qualification.
`rspec.json` uses RSpec's stock JSON dry-run shape. The other files reuse the
existing owned reporter profiles, including the existing Test.File field, as
emitted by the discovery adapter. Tests derive malformed/count/skip/failure
variants in memory. No Ruby executable or Rails application ran to produce these
fixtures; live discovery parity and the Rails qualification corpus are NOT_RUN.
