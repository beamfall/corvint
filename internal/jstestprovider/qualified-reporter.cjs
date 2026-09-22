const fs = require('node:fs');
const crypto = require('node:crypto');
const childProcess = require('node:child_process');
const path = require('node:path');
const playwright = require(require.resolve('playwright', {paths: [process.cwd()]}));

const identityKeys = ['browserName', 'defaultBrowserType', 'channel', 'headless', 'connectOptions', 'viewport', 'screen', 'userAgent', 'isMobile', 'hasTouch', 'deviceScaleFactor', 'locale', 'timezoneId', 'colorScheme', 'permissions', 'contextOptions', 'launchOptions'];
const qualifiedBundledBrowser = {
  executableName: 'chromium-headless-shell', revision: '1243', manifestVersion: '153.0.8010.12',
  observedVersion: 'Google Chrome for Testing 153.0.8010.12',
  executableSha256: 'a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282',
  pathSuffix: '/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell'
};
const bundledBrowsers = new Map();
const redactionMarker = '[REDACTED]';
const defaultSensitiveActions = ['fill', 'type', 'inserttext', 'insert-text', 'insert text', 'presssequentially', 'press sequentially'];
const defaultSensitiveFields = ['value', 'text', 'inputvalue', 'input-value'];
const sensitiveLimits = {depth: 32, steps: 4096, stringBytes: 64 * 1024};

function sensitivePolicy(additions) {
  if (!additions) return null;
  const invalid = () => { throw sensitiveInputError('sensitive-input-policy-invalid'); };
  if (typeof additions !== 'object' || Array.isArray(additions) || Object.keys(additions).some(key => !['additionalActionPatterns', 'additionalSensitiveFields'].includes(key))) invalid();
  const actionInput = additions.additionalActionPatterns ?? [];
  const fieldInput = additions.additionalSensitiveFields ?? [];
  if (!Array.isArray(actionInput) || !Array.isArray(fieldInput) || actionInput.length > 16 || fieldInput.length > 32) invalid();
  const normalize = value => {
    if (typeof value !== 'string') invalid();
    const normalized = simpleLower(value).replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '');
    if (!normalized || Buffer.byteLength(normalized) > 128) invalid();
    return normalized;
  };
  const actions = actionInput.map(normalize).map(normalizedWords);
  if (actions.some(action => !action)) invalid();
  const fields = new Set(defaultSensitiveFields.concat(fieldInput.map(normalize)));
  return {actions, fields};
}

function sensitiveInputError(code) {
  const error = new Error(code);
  error.code = code;
  return error;
}

function normalizedWords(value) {
  return (simpleLower(value).match(/[\p{L}\p{N}\p{M}]+/gu) || []).join(' ');
}

function simpleLower(value) {
  // Go's per-rune simple lowercasing does not expand U+0130 or use final sigma.
  return Array.from(String(value), rune => Array.from(rune.toLowerCase())[0]).join('');
}

const wordRune = rune => /[\p{L}\p{N}\p{M}]/u.test(rune);
const spaceRune = rune => /\p{White_Space}/u.test(rune);
const delimiterRune = rune => !wordRune(rune);

function patternEnd(title, start, pattern) {
  const words = normalizedWords(pattern).split(' ');
  if (!words[0]) return -1;
  for (let i = 0; i < words.length; i++) {
    if (i > 0) while (start < title.length && delimiterRune(title[start])) start++;
    let end = start;
    while (end < title.length && wordRune(title[end])) end++;
    if (simpleLower(title.slice(start, end).join('')) !== words[i]) return -1;
    start = end;
  }
  return start;
}

function sensitiveActionMatch(title, policy) {
  const runes = Array.from(String(title));
  const defaults = [
    {patterns: ['fill'], display: 'Fill'}, {patterns: ['type'], display: 'Type'},
    {patterns: ['inserttext', 'insert text', 'insert-text'], display: 'InsertText'},
    {patterns: ['presssequentially', 'press sequentially'], display: 'PressSequentially'}
  ];
  let start = 0;
  while (start < runes.length && delimiterRune(runes[start])) start++;
  while (start < runes.length) {
    for (const action of defaults) for (const pattern of action.patterns) {
      const end = patternEnd(runes, start, pattern);
      if (end >= 0) return {display: action.display, tail: runes.slice(end).join('')};
    }
    for (const pattern of policy.actions) {
      const end = patternEnd(runes, start, pattern);
      if (end >= 0) return {display: normalizedWords(pattern), tail: runes.slice(end).join('')};
    }
    let end = start;
    while (end < runes.length && (wordRune(runes[end]) || runes[end] === '_' || runes[end] === '$')) end++;
    if (end === start) break;
    let separator = end;
    while (separator < runes.length && spaceRune(runes[separator])) separator++;
    if (separator === runes.length || (runes[separator] !== '.' && !(runes[separator] === '/' && separator === end))) break;
    start = separator + 1;
    while (start < runes.length && delimiterRune(runes[start])) start++;
  }
  return {display: '', tail: ''};
}

function unsupportedSensitiveAction(title, policy) {
  if (sensitiveActionMatch(title, policy).display) return false;
  const runes = Array.from(String(title));
  let start = 0;
  while (start < runes.length && delimiterRune(runes[start])) start++;
  let end = start;
  while (end < runes.length && (wordRune(runes[end]) || runes[end] === '_' || runes[end] === '$')) end++;
  if (end === start) return false;
  while (end < runes.length && spaceRune(runes[end])) end++;
  if (end === runes.length || !'.(['.includes(runes[end])) return false;
  let quote = '', quoteStart = 0, property = false, escaped = false;
  for (let i = end; i < runes.length; i++) {
    const rune = runes[i];
    if (quote) {
      if (escaped) { escaped = false; continue; }
      if (rune === '\\') { escaped = true; continue; }
      if (rune === quote) {
        if (property) {
          let raw = runes.slice(quoteStart, i).join('');
          if (quote === "'") raw = raw.replace(/\\'/g, "'");
          try { raw = JSON.parse(`"${raw}"`); } catch {}
          if (defaultSensitiveActions.concat(policy.actions).some(pattern => normalizedWords(raw) === normalizedWords(pattern))) return true;
        }
        quote = '';
      }
      continue;
    }
    if (rune === '"' || rune === "'" || rune === '`') {
      quote = rune; quoteStart = i + 1;
      let previous = i - 1;
      while (previous >= end && spaceRune(runes[previous])) previous--;
      property = previous >= end && runes[previous] === '[';
      continue;
    }
    if (wordRune(rune) && (i === 0 || !wordRune(runes[i - 1]))) {
      if (defaultSensitiveActions.concat(policy.actions).some(pattern => patternEnd(runes, i, pattern) >= 0)) return true;
    }
  }
  return quote !== '';
}

function collectTailCandidates(tail, values) {
  const add = raw => {
    const value = String(raw).replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '').replace(/^["']+|["']+$/g, '');
    if (value && value !== redactionMarker) values.add(value);
  };
  tail = String(tail).replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '');
  add(tail);
  add(tail.replace(/^[^\p{L}\p{N}\p{M}]+|[^\p{L}\p{N}\p{M}]+$/gu, ''));
  tail = tail.replace(/^[\p{White_Space}):=_-]+/u, '');
  if (tail.startsWith('(')) tail = tail.slice(1).replace(/\)$/, '');
  add(tail);
  const runes = Array.from(tail);
  let start = 0, quoteStart = 0, depth = 0, quote = '', escaped = false;
  for (let i = 0; i < runes.length; i++) {
    const rune = runes[i];
    if (quote) {
      if (escaped) { escaped = false; continue; }
      if (rune === '\\') { escaped = true; continue; }
      if (rune === quote) {
        let raw = runes.slice(quoteStart, i).join('');
        add(raw);
        if (quote === "'") raw = raw.replace(/\\'/g, "'");
        try { add(JSON.parse(`"${raw}"`)); } catch {}
        quote = '';
      }
      continue;
    }
    if (rune === '"' || rune === "'") { quote = rune; quoteStart = i + 1; continue; }
    if (rune === '(') { depth++; continue; }
    if (rune === ')') { if (depth > 0) depth--; continue; }
    if (rune === ',') {
      if (depth === 0) { add(runes.slice(start, i).join('')); start = i + 1; }
      continue;
    }
    if (depth === 0 && wordRune(rune) && (i === 0 || !wordRune(runes[i - 1]))) {
      let end = i;
      while (end < runes.length && wordRune(runes[end])) end++;
      if (['with', 'value', 'text'].includes(simpleLower(runes.slice(i, end).join('')))) add(runes.slice(end).join(''));
    }
  }
  add(runes.slice(start).join(''));
}

function redactStep(step, policy, values) {
  const title = String(step.title || '');
  const action = sensitiveActionMatch(title, policy);
  let display = action.display;
  let sensitive = display !== '' || step.redacted === true;
  const metadata = {};
  for (const [key, raw] of Object.entries(step.metadata || {})) {
    if (policy.fields.has(simpleLower(key))) {
      sensitive = true;
      if (String(raw) !== redactionMarker) values.add(String(raw));
      metadata[key] = redactionMarker;
    } else metadata[key] = String(raw);
  }
  if (sensitive) {
    if (display) collectTailCandidates(action.tail, values);
    if (!display) display = 'Input';
  }
  const nestedResults = (step.steps || []).map(child => redactStep(child, policy, values));
  const nestedSensitive = nestedResults.some(result => result.sensitive);
  const protectedBranch = sensitive || nestedSensitive;
  const attachments = (step.attachments || []).map(attachment => protectedBranch ? {name: redactionMarker, path: redactionMarker} : {name: attachment.name || '', path: attachment.path || ''});
  return {sensitive: protectedBranch, step: {
    title: sensitive ? `${display} "${redactionMarker}"` : title, category: step.category || '',
    ...(Object.keys(metadata).length ? {metadata} : {}),
    ...(step.error ? {error: protectedBranch ? redactionMarker : (step.error.message || String(step.error))} : {}),
    ...(attachments.length ? {attachments} : {}), ...(nestedResults.length ? {steps: nestedResults.map(result => result.step)} : {}),
    ...(sensitive ? {redacted: true} : {})
  }};
}

function scrubRiskText(value, values, protectedTest = false) {
  let safe = String(value || '');
  if (protectedTest) return safe ? redactionMarker : '';
  if (safe === redactionMarker) return safe;
  for (const sensitive of values) if (sensitive) safe = safe.split(sensitive).join(redactionMarker);
  return safe;
}

function scrubStepRiskFields(steps, values, protectedTest) {
  for (const step of steps || []) {
    if (step.error) step.error = scrubRiskText(step.error, values, protectedTest);
    for (const attachment of step.attachments || []) {
      attachment.name = scrubRiskText(attachment.name, values, protectedTest);
      attachment.path = scrubRiskText(attachment.path, values, protectedTest);
      if (attachment.body) attachment.body = scrubRiskText(attachment.body, values, protectedTest);
    }
    scrubStepRiskFields(step.steps, values, protectedTest);
  }
}

function scrubReportRiskFields(report, values) {
  const sensitiveSteps = steps => (steps || []).some(step => step.redacted === true || sensitiveSteps(step.steps));
  const protectedReport = (report.tests || []).some(test => (test.attempts || []).some(attempt => sensitiveSteps(attempt.steps)));
  for (const test of report.tests || []) {
    test.failureMessage = scrubRiskText(test.failureMessage, values, protectedReport);
    for (const artifact of test.artifacts || []) {
      artifact.name = scrubRiskText(artifact.name, values, protectedReport);
      artifact.path = scrubRiskText(artifact.path, values, protectedReport);
    }
    for (const attempt of test.attempts || []) scrubStepRiskFields(attempt.steps, values, protectedReport);
  }
  report.errors = (report.errors || []).map(error => scrubRiskText(error, values, protectedReport));
  return report;
}

function checkStepBounds(steps, priorCount, policy) {
  const stack = [{steps: steps || [], depth: 1}];
  let count = priorCount;
  while (stack.length) {
    const current = stack.pop();
    if (current.depth > sensitiveLimits.depth) throw new Error('sensitive-input-depth-exceeded');
    count += current.steps.length;
    if (count > sensitiveLimits.steps) throw new Error('sensitive-input-step-bound-exceeded');
    for (const step of current.steps) {
      const strings = [step.title, step.category, step.error?.message || step.error];
      for (const [key, value] of Object.entries(step.metadata || {})) strings.push(key, value);
      for (const attachment of step.attachments || []) strings.push(attachment.name, attachment.path);
      if (strings.some(value => Buffer.byteLength(String(value || '')) > sensitiveLimits.stringBytes)) throw new Error('sensitive-input-string-bound-exceeded');
      if (unsupportedSensitiveAction(step.title || '', policy)) throw sensitiveInputError('sensitive-input-action-syntax-unsupported');
      if ((step.steps || []).length) stack.push({steps: step.steps, depth: current.depth + 1});
    }
  }
  return count;
}

function fileSha256(file) {
  const hash = crypto.createHash('sha256');
  const descriptor = fs.openSync(file, 'r');
  const buffer = Buffer.allocUnsafe(1024 * 1024);
  try {
    for (let bytes = fs.readSync(descriptor, buffer); bytes > 0; bytes = fs.readSync(descriptor, buffer)) hash.update(buffer.subarray(0, bytes));
  } finally {
    fs.closeSync(descriptor);
  }
  return hash.digest('hex');
}

function observedVersion(executablePath) {
  const stdout = childProcess.spawnSync(executablePath, ['--version'], {encoding: 'utf8'}).stdout;
  return typeof stdout === 'string' ? stdout.trim() : '';
}

function bundledBrowserIdentity(headless) {
  const name = headless ? 'chromium-headless-shell' : 'chromium';
  if (bundledBrowsers.has(name)) return bundledBrowsers.get(name);
  const registry = require(require.resolve('playwright-core/lib/coreBundle', {paths: [process.cwd()]})).registry?.registry;
  const entry = registry?.findExecutable(name);
  const executablePath = entry?.executablePath();
  if (!entry || !executablePath || !fs.existsSync(executablePath)) {
    bundledBrowsers.set(name, null);
    return null;
  }
  const browser = {
    platform: process.platform, arch: process.arch, nodeVersion: process.version,
    browserType: entry.browserName, browserVersion: observedVersion(executablePath), channel: '',
    executableSource: 'playwright-bundled', executableName: entry.name,
    executablePath, executableSha256: fileSha256(executablePath), browserRevision: String(entry.revision),
    manifestBrowserVersion: entry.browserVersion, headlessShellAvailable: entry.name === 'chromium-headless-shell'
  };
  bundledBrowsers.set(name, browser);
  return browser;
}

function qualifiedBundledIdentity(browser) {
  const qualified = qualifiedBundledBrowser;
  return browser && browser.platform === 'darwin' && browser.arch === 'arm64' && browser.nodeVersion === 'v22.23.2' &&
    browser.browserType === 'chromium' && browser.browserVersion === qualified.observedVersion && browser.channel === '' &&
    browser.executableSource === 'playwright-bundled' && browser.executableName === qualified.executableName &&
    browser.executablePath.endsWith(qualified.pathSuffix) && browser.executableSha256 === qualified.executableSha256 &&
    browser.browserRevision === qualified.revision && browser.manifestBrowserVersion === qualified.manifestVersion &&
    browser.headlessShellAvailable === true;
}

// Qualified against 1.60.0 and 1.63.0 in-process reporter objects. Serialization or a
// custom executable fixture can erase effective options; that is unknown.
function effectiveUse(test, project, version) {
  if (!['1.60.0', '1.63.0'].includes(version) || !Array.isArray(test._testType?.fixtures)) return null;
  const use = {};
  const assign = (fixtures, builtin) => {
    if (!builtin && ['browser', 'context', 'page', 'playwright', '_browserOptions', '_combinedContextOptions', '_optionConnectOptions'].some(k => Object.hasOwn(fixtures, k))) return false;
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
  if (version === '1.63.0') {
    resolved.headless = Object.hasOwn(resolved, 'headless') ? resolved.headless : resolved.launchOptions?.headless ?? true;
    if (Object.hasOwn(resolved, 'connectOptions') || process.env.PW_TEST_CONNECT_WS_ENDPOINT) return null;
    const executablePath = resolved.launchOptions?.executablePath || '';
    const executableVersion = executablePath ? observedVersion(executablePath) : '';
    const bundledChromium = playwright.chromium.executablePath();
    const bundledMatch = bundledChromium.match(/^(.*)\/chromium-(\d+)\//);
    const headlessShell = bundledMatch ? path.join(bundledMatch[1], `chromium_headless_shell-${bundledMatch[2]}`, 'chrome-headless-shell-mac-arm64', 'chrome-headless-shell') : '';
    resolved.channel = resolved.channel || '';
    resolved.corvintBrowser = executablePath ? {platform: process.platform, arch: process.arch, nodeVersion: process.version, browserType: resolved.browserName, browserVersion: executableVersion, channel: resolved.channel, executablePath, headlessShellAvailable: headlessShell !== '' && fs.existsSync(headlessShell)} : bundledBrowserIdentity(resolved.headless);
    const systemQualified = resolved.corvintBrowser?.platform === 'darwin' && resolved.corvintBrowser?.arch === 'arm64' && resolved.corvintBrowser?.nodeVersion === 'v22.23.2' && resolved.corvintBrowser?.browserType === 'chromium' && resolved.corvintBrowser?.browserVersion === 'Google Chrome 153.0.8010.48' && resolved.corvintBrowser?.channel === '' && resolved.corvintBrowser?.executablePath === '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' && resolved.corvintBrowser?.headlessShellAvailable === true;
    if (resolved.corvintBrowser?.browserType !== resolved.browserName || (!systemQualified && !qualifiedBundledIdentity(resolved.corvintBrowser))) return null;
  } else {
    delete resolved.headless;
    delete resolved.connectOptions;
  }
  return resolved;
}

// The config supplies only a private output path. Nothing is read from stdout.
class Reporter {
  constructor(options) { this.path = options.output; this.tests = new Map(); this.errors = []; this.sensitiveInputPolicy = options.sensitiveInputPolicy || null; this.policy = sensitivePolicy(this.sensitiveInputPolicy); this.hasSensitiveInput = false; this.sensitiveValues = new Set(); this.sensitiveStepCount = 0; }
  onBegin(config, suite) {
	this.schedule = {workers: config.workers, starts: []};
    this.version = config.version;
    this.files = {};
    this.configFiles = {};
    for (const file of Object.keys(require.cache)) {
      if (file.includes('/node_modules/') || file.startsWith(require('node:path').dirname(this.path) + '/')) continue;
      // A module moved or removed since it was loaded is skipped rather than aborting onBegin.
      try { if (fs.statSync(file).isFile()) this.configFiles[file] = crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'); } catch { continue; }
    }
    for (const test of suite.allTests()) {
      const file = test.location.file;
      this.files[file] = crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
    }
  }
  onError(error) { this.errors.push(error.message || String(error)); }
  onTestBegin(test, result) {
    const project = test.parent.project();
    const repeat = test.repeatEachIndex > 0 ? ` > repeat ${test.repeatEachIndex}` : '';
    this.schedule.starts.push({fullName: test.titlePath().join(' > ') + repeat, file: test.location.file, line: test.location.line, project: project?.name || '', retry: result.retry, retries: test.retries, worker: result.workerIndex, fullyParallel: project?.fullyParallel === true});
  }
  onTestEnd(test, result) {
    const project = test.parent.project();
    const use = project ? effectiveUse(test, project, this.version) : null;
    const errors = result.errors || [];
    let message = errors.map(e => e.message || '').join('\n');
    if (this.policy) {
      try { this.sensitiveStepCount = checkStepBounds(result.steps || [], this.sensitiveStepCount, this.policy); }
      catch (error) { this.sensitiveRejection = error; throw error; }
    }
    const failedFixture = (steps) => steps.some(s => ((s.category === 'fixture' || s.category === 'hook') && s.error) || failedFixture(s.steps || []));
    const browserFailure = /browserType\.launch:|browser\.newContext:|browser has been closed|Browser closed|Target page, context or browser has been closed/.test(message);
    const infrastructure = browserFailure || failedFixture(result.steps || []);
    let state = result.status;
    if (infrastructure) state = 'infrastructure';
    const previous = this.tests.get(test.id);
    const attempts = previous ? previous.attempts : [];
    const values = this.sensitiveValues;
    const redactedSteps = this.policy ? (result.steps || []).map(step => redactStep(step, this.policy, values)) : [];
    const sensitive = redactedSteps.some(result => result.sensitive);
    const steps = redactedSteps.map(result => result.step);
    if (sensitive && message) message = redactionMarker;
    this.hasSensitiveInput ||= sensitive;
    attempts.push({state, retry: result.retry, failureKind: infrastructure ? 'browser-or-fixture' : state === 'failed' ? 'assertion-or-test' : state === 'timedOut' ? 'test-timeout' : 'none', ...(this.policy ? {steps} : {})});
    const repeat = test.repeatEachIndex > 0 ? ` > repeat ${test.repeatEachIndex}` : '';
    this.tests.set(test.id, {
      name: test.title, fullName: test.titlePath().join(' > ') + repeat, state,
      project: {name: project ? project.name : '', browser: use ? use.browserName : '', device: project && typeof project.metadata?.device === 'string' ? project.metadata.device : 'unknown', use},
      retries: result.retry, durationMs: result.duration,
      anchor: {file: test.location.file, line: test.location.line},
      failureMessage: message, attempts,
      artifacts: (result.attachments || []).filter(a => a.path).map(a => sensitive ? {name: redactionMarker, path: redactionMarker} : {name: a.name, path: a.path})
    });
  }
  onEnd(result) {
    if (this.sensitiveRejection) throw this.sensitiveRejection;
    const tests = Array.from(this.tests.values());
    for (const test of tests) {
      if (test.state === 'passed' && test.attempts.some(a => a.state !== 'passed')) test.state = 'flaky';
    }
    if (this.hasSensitiveInput) this.errors = this.errors.map(() => redactionMarker);
    const report = {version: this.version, files: this.files, configFiles: this.configFiles, status: result.status, tests, errors: this.errors, schedule: this.schedule, ...(this.sensitiveInputPolicy ? {sensitiveInputPolicy: this.sensitiveInputPolicy} : {})};
    fs.writeFileSync(this.path, JSON.stringify(scrubReportRiskFields(report, this.sensitiveValues)));
  }
}
module.exports = Reporter;
