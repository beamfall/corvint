// SPDX-License-Identifier: AGPL-3.0-or-later
// Mocha native event reporter. Each reporter instance writes its own
// fresh artifact, so parallel specs never overwrite an earlier report.
const fs = require('node:fs');
const path = require('node:path');
const { randomUUID } = require('node:crypto');

module.exports = class CorvintMochaReporter {
  constructor(runner, options) {
    const outputDir = options.reporterOptions.outputDir;
    const rows = new Map();
    const problems = [];
    const key = test => `${test.file || ''}::${test.fullTitle()}`;
    const record = (test, state, error) => {
      if (test.type === 'hook') {
        problems.push({ Code: 'hook-error', Detail: error && error.message || test.title });
        return;
      }
      const id = key(test);
      let row = rows.get(id);
      if (!row) {
        row = { ID: id, Name: test.title, File: test.file || '', State: state, Attempts: [] };
        rows.set(id, row);
      }
      const retry = test.currentRetry();
      if (retry !== row.Attempts.length) {
        problems.push({ Code: 'attempt-sequence', Detail: id });
      }
      row.Attempts.push({ State: state, FailureKind: state === 'FAILED' ? 'UNKNOWN' : '',
        Message: error && error.message || '' });
      row.State = state === 'PASSED' && retry > 0 ? 'FLAKY' : state;
    };
    runner.on('pass', test => record(test, 'PASSED'));
    runner.on('pending', test => record(test, 'SKIPPED'));
    runner.on('retry', (test, error) => record(test, 'FAILED', error));
    runner.on('fail', (test, error) => record(test, 'FAILED', error));
    runner.once('end', () => {
      const tests = Array.from(rows.values());
      const output = path.join(outputDir, `mocha-${randomUUID()}.json`);
      fs.writeFileSync(output, JSON.stringify({ Profile: 'corvint-mocha/0', Complete: true,
        Count: tests.length, Tests: tests, Problems: problems }), { flag: 'wx', mode: 0o600 });
    });
  }
};
