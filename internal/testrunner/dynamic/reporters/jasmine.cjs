// SPDX-License-Identifier: AGPL-3.0-or-later
// Jasmine native reporter-event recorder. Jasmine 7 ships no machine-readable
// file reporter, so this fixed profile file copies named fields of Jasmine's own
// reporter events and writes them once, at jasmineDone, beside itself in the
// fresh report directory. It decides no outcome: the Go parser owns validation.
const fs = require('node:fs');
const path = require('node:path');

const expectations = list => (Array.isArray(list) ? list : []).map(e => ({
  matcherName: String(e && e.matcherName || ''),
  message: String(e && e.message || ''),
  globalErrorType: String(e && e.globalErrorType || ''),
}));
const node = r => ({
  id: String(r.id || ''),
  description: String(r.description || ''),
  fullName: String(r.fullName || ''),
  parentSuiteId: r.parentSuiteId == null ? null : String(r.parentSuiteId),
  filename: String(r.filename || ''),
  status: String(r.status || ''),
  pendingReason: String(r.pendingReason || ''),
  notApplicableReason: String(r.notApplicableReason || ''),
  failedExpectations: expectations(r.failedExpectations),
});

module.exports = class CorvintJasmineReporter {
  constructor() {
    this.started = null;
    this.suites = [];
    this.specs = [];
  }
  jasmineStarted(info) {
    this.started = {
      totalSpecsDefined: Number.isInteger(info && info.totalSpecsDefined) ? info.totalSpecsDefined : null,
      parallel: typeof (info && info.parallel) === 'boolean' ? info.parallel : null,
    };
  }
  suiteDone(result) { this.suites.push(node(result)); }
  specDone(result) { this.specs.push(node(result)); }
  jasmineDone(info) {
    const done = {
      overallStatus: String(info && info.overallStatus || ''),
      incompleteCode: String(info && info.incompleteCode || ''),
      incompleteReason: String(info && info.incompleteReason || ''),
      failedExpectations: expectations(info && info.failedExpectations),
    };
    fs.writeFileSync(path.join(__dirname, 'jasmine.json'), JSON.stringify({ profile: 'corvint-jasmine/0',
      started: this.started, suites: this.suites, specs: this.specs, done }), { flag: 'wx', mode: 0o600 });
  }
};
