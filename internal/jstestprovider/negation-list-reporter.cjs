'use strict';
// Corvint step-negation test lister (LPCV-V0-057). It runs only under
// `playwright test --list` from the provider's controlled config and writes
// each collected test's location, title path and project to a private scratch
// file so the provider can refuse an ambiguous selection before any run.
const fs = require('node:fs');

class NegationListReporter {
  constructor(options) {
    this.output = options.output;
  }

  printsToStdio() {
    return false;
  }

  onBegin(config, suite) {
    const tests = suite.allTests().map(test => ({
      file: test.location.file,
      line: test.location.line,
      titlePath: test.titlePath(),
      project: test.parent.project()?.name ?? '',
    }));
    fs.writeFileSync(this.output, JSON.stringify(tests), {mode: 0o600, flag: 'wx'});
  }
}

module.exports = NegationListReporter;
