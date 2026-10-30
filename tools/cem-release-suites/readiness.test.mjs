// Independent Node check of the committed cem/1.0 release-readiness inputs
// (docs/specs/cem-stable-v1.md CEM-V1-605, V1-2323). It reads the bytes the
// Go gate reads, without the Go code: the declaration's closed shape and
// suite coverage, MIGRATION.json A02 out of OPEN, and the sealed manifest's
// pin of MIGRATION.json recomputed with node:crypto.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';

const root = new URL('../../', import.meta.url);
const read = (path) => readFileSync(new URL(path, root));
const json = (path) => JSON.parse(read(path).toString('utf8'));

test('readiness declaration is one closed cem-release-readiness/1 document', () => {
  const d = json('tools/cem-release-suites/READINESS.json');
  assert.deepEqual(Object.keys(d), ['profile', 'runner', 'platform', 'external']);
  assert.equal(d.profile, 'cem-release-readiness/1');
  for (const name of ['runner', 'platform', 'external']) {
    const rows = d[name];
    assert.ok(Array.isArray(rows) && rows.length >= 1 && rows.length <= 64, `${name} row count`);
    assert.equal(new Set(rows).size, rows.length, `${name} rows repeat`);
    for (const row of rows) assert.match(row, /^[A-Za-z0-9._/:-]{1,128}$/);
  }
});

test('readiness declaration requires every sealed suite on both binaries', () => {
  const d = json('tools/cem-release-suites/READINESS.json');
  for (const binary of ['corvint', 'cem01-go']) {
    for (const suite of ['tamper-matrix', 'citation-vectors', 'neutral-criterion', 'receipt-authentication', 'repository-envelope']) {
      assert.ok(d.runner.includes(`${binary}:${suite}`), `${binary}:${suite} is not required`);
    }
  }
  assert.ok(d.runner.includes('source:repository-envelope'));
  assert.ok(d.external.length >= 1, 'external interoperability must stay a required row');
});

test('MIGRATION.json A02 has left OPEN and names the integrating gate', () => {
  const m = json('protocol/cem-1.0/stable/MIGRATION.json');
  const a02 = m.laterAdditions.filter((entry) => entry.id === 'A02');
  assert.equal(a02.length, 1);
  assert.notEqual(a02[0].status, 'OPEN');
  assert.match(a02[0].status, /^IMPLEMENTED_EXPERIMENTAL: /);
  assert.match(a02[0].status, /make cem-release-gate/);
  assert.match(a02[0].status, /not promoted\.$/);
});

test('sealed stable manifest pins the current MIGRATION.json bytes', () => {
  const bytes = read('protocol/cem-1.0/stable/MIGRATION.json');
  const pins = json('protocol/cem-1.0/stable/manifest.json').files.filter((f) => f.path === 'MIGRATION.json');
  assert.equal(pins.length, 1);
  assert.equal(pins[0].bytes, bytes.length);
  assert.equal(pins[0].sha256, createHash('sha256').update(bytes).digest('hex'));
});
