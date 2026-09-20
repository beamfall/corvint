const fs = require('node:fs');
const crypto = require('node:crypto');

const identityKeys = ['browserName', 'defaultBrowserType', 'channel', 'viewport', 'screen', 'userAgent', 'isMobile', 'hasTouch', 'deviceScaleFactor', 'locale', 'timezoneId', 'colorScheme', 'permissions', 'contextOptions', 'launchOptions'];

// Qualified against 1.60.0's in-process reporter objects. Serialization or a
// custom executable fixture can erase effective options; that is unknown.
function effectiveUse(test, project, version) {
  if (version !== '1.60.0' || !Array.isArray(test._testType?.fixtures)) return null;
  const use = {};
  const assign = (fixtures, builtin) => {
    if (!builtin && ['browser', 'context', 'page', 'playwright', '_combinedContextOptions'].some(k => Object.hasOwn(fixtures, k))) return false;
    for (const key of identityKeys) {
      if (!Object.hasOwn(fixtures, key)) continue;
      let value = fixtures[key];
      if (Array.isArray(value) && value.length === 2 && value[1] && typeof value[1] === 'object' && ('option' in value[1] || 'scope' in value[1] || 'auto' in value[1])) value = value[0];
      if (typeof value === 'function') {
        if (builtin) continue;
        return false;
      }
      if (value === undefined) { if (!builtin) return false; continue; }
      use[key] = value;
    }
    return true;
  };
  for (const fixture of test._testType.fixtures) {
    if (!fixture.fixtures || !fixture.location) return null;
    if (!assign(fixture.fixtures, fixture.location.file.endsWith('/playwright/lib/index.js'))) return null;
  }
  if (!assign(project.use, false)) return null;
  const parents = [];
  for (let suite = test.parent; suite; suite = suite.parent) parents.unshift(suite);
  for (const suite of parents) {
    if (!Array.isArray(suite._use)) return null;
    for (const override of suite._use) if (!override.fixtures || !assign(override.fixtures, false)) return null;
  }
  const context = use.contextOptions || {};
  const resolved = {viewport: {width:1280,height:720}, isMobile:false, hasTouch:false, locale:'en-US', colorScheme:'light', ...context, ...use};
  resolved.browserName = use.browserName || use.defaultBrowserType || 'chromium';
  if (resolved.channel === undefined && use.launchOptions?.channel !== undefined) resolved.channel = use.launchOptions.channel;
  return resolved;
}

// The config supplies only a private output path. Nothing is read from stdout.
class Reporter {
  constructor(options) { this.path = options.output; this.tests = new Map(); this.errors = []; }
  onBegin(config, suite) {
    this.version = config.version;
    this.files = {};
    this.configFiles = {};
    for (const file of Object.keys(require.cache)) {
      if (file.includes('/node_modules/') || file.startsWith(require('node:path').dirname(this.path) + '/')) continue;
      if (fs.statSync(file).isFile()) this.configFiles[file] = crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
    }
    for (const test of suite.allTests()) {
      const file = test.location.file;
      this.files[file] = crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
    }
  }
  onError(error) { this.errors.push(error.message || String(error)); }
  onTestEnd(test, result) {
    const project = test.parent.project();
    const use = project ? effectiveUse(test, project, this.version) : null;
    const errors = result.errors || [];
    const message = errors.map(e => e.message || '').join('\n');
    const failedFixture = (steps) => steps.some(s => ((s.category === 'fixture' || s.category === 'hook') && s.error) || failedFixture(s.steps || []));
    const browserFailure = /browserType\.launch:|browser\.newContext:|browser has been closed|Browser closed|Target page, context or browser has been closed/.test(message);
    const infrastructure = browserFailure || failedFixture(result.steps || []);
    let state = result.status;
    if (infrastructure) state = 'infrastructure';
    const previous = this.tests.get(test.id);
    const attempts = previous ? previous.attempts : [];
    attempts.push({state, retry: result.retry, failureKind: infrastructure ? 'browser-or-fixture' : state === 'failed' ? 'assertion-or-test' : state === 'timedOut' ? 'test-timeout' : 'none'});
    this.tests.set(test.id, {
      name: test.title, fullName: test.titlePath().join(' > '), state,
      project: {name: project ? project.name : '', browser: use ? use.browserName : '', device: project && typeof project.metadata?.device === 'string' ? project.metadata.device : 'unknown', use},
      retries: result.retry, durationMs: result.duration,
      anchor: {file: test.location.file, line: test.location.line},
      failureMessage: message, attempts,
      artifacts: (result.attachments || []).filter(a => a.path).map(a => ({name: a.name, path: a.path}))
    });
  }
  onEnd(result) {
    const tests = Array.from(this.tests.values());
    for (const test of tests) {
      if (test.state === 'passed' && test.attempts.some(a => a.state !== 'passed')) test.state = 'flaky';
    }
    fs.writeFileSync(this.path, JSON.stringify({version: this.version, files: this.files, configFiles: this.configFiles, status: result.status, tests, errors: this.errors}));
  }
}
module.exports = Reporter;
