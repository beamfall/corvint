// SPDX-License-Identifier: AGPL-3.0-or-later
// Cypress supplies reconstructed tests on test:after:run for every attempt.
// Its retry event is a plain object, unlike Mocha. Each instance writes its own
// fresh artifact, so parallel specs never overwrite an earlier report.
const fs = require('node:fs');
const path = require('node:path');
const { randomUUID } = require('node:crypto');

module.exports = class CorvintCypressReporter {
  constructor(runner, options) {
    const outputDir = options.reporterOptions.outputDir;
    const rows = new Map();
    const problems = [];
    const source = test => test.invocationDetails && test.invocationDetails.absoluteFile || '';
    const key = test => `${source(test)}::${test.fullTitle()}`;
    const finalized = new Map();
    const record = (test, state, error) => {
      if (test.type === 'hook' || test.failedFromHookId) {
        problems.push({ Code: 'hook-error', Detail: error && error.message || test.title });
        return;
      }
      const id = key(test);
      if (error && state !== 'FAILED') problems.push({ Code: 'case-error-conflict', Detail: id });
      if (!source(test)) {
        // Synthetic root tests originate in the Cypress runner, not a spec.
        // Other missing source metadata remains unresolved too.
        const invocation = test.invocationDetails || {};
        const synthetic = test.parent && test.parent.root && /\/__cypress\/runner\//.test(invocation.fileUrl || '');
        problems.push({ Code: synthetic ? 'collection-error' : 'missing-native-source', Detail: id });
      }
      finalized.set(id, test.final === true);
      let row = rows.get(id);
      if (!row) {
        row = { ID: id, Name: test.title, File: source(test), State: state, Attempts: [] };
        rows.set(id, row);
      }
      const retry = test.currentRetry();
      if (!Number.isInteger(retry) || retry < 0 || retry >= 32) { problems.push({ Code: 'attempt-bound', Detail: id }); return; }
      if (retry !== row.Attempts.length) {
        problems.push({ Code: 'attempt-sequence', Detail: id });
      }
      row.Attempts.push({ State: state, FailureKind: state === 'FAILED' ? 'UNKNOWN' : '',
        Message: error && error.message || '' });
      row.State = state === 'PASSED' && retry > 0 ? 'FLAKY' : state;
      const info = test._cypressTestStatusInfo;
      if (test.final && info && (info.attempts !== row.Attempts.length ||
          !['passed', 'failed'].includes(info.outerStatus) ||
          (info.outerStatus === 'failed' && state !== 'FAILED') ||
          (info.outerStatus === 'passed' && state !== 'PASSED'))) {
        problems.push({ Code: 'native-attempt-conflict', Detail: id });
      }
    };
    runner.on('test:after:run', test => {
      const state = { passed: 'PASSED', failed: 'FAILED', pending: 'SKIPPED' }[test.state] || 'UNKNOWN';
      record(test, state, test.err);
    });
    runner.once('end', () => {
      const tests = Array.from(rows.values());
      for (const [id, final] of finalized) { if (!final) problems.push({ Code: 'missing-final-attempt', Detail: id }); }
      const output = path.join(outputDir, `cypress-${randomUUID()}.json`);
      fs.writeFileSync(output, JSON.stringify({ Profile: 'corvint-cypress/0', Complete: true,
        Count: tests.length, Tests: tests, Problems: problems }), { flag: 'wx', mode: 0o600 });
    });
  }
};
