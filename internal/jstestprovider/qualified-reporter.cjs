const fs = require('node:fs');
const crypto = require('node:crypto');
const childProcess = require('node:child_process');
const path = require('node:path');
const playwright = require(require.resolve('playwright', {paths: [process.cwd()]}));

const identityKeys = ['browserName', 'defaultBrowserType', 'channel', 'headless', 'connectOptions', 'viewport', 'screen', 'userAgent', 'isMobile', 'hasTouch', 'deviceScaleFactor', 'locale', 'timezoneId', 'colorScheme', 'permissions', 'contextOptions', 'launchOptions'];
// PWP-V0-032: each qualified runner version pins its own bundled headless shell.
const qualifiedBundledBrowsers = {
  '1.63.0': {
    executableName: 'chromium-headless-shell', revision: '1243', manifestVersion: '153.0.8010.12',
    observedVersion: 'Google Chrome for Testing 153.0.8010.12',
    executableSha256: 'a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282',
    pathSuffix: '/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell'
  },
  '1.64.0': {
    executableName: 'chromium-headless-shell', revision: '1248', manifestVersion: '156.0.8078.4',
    observedVersion: 'Google Chrome for Testing 156.0.8078.4',
    executableSha256: '77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634',
    pathSuffix: '/chromium_headless_shell-1248/chrome-headless-shell-mac-arm64/chrome-headless-shell'
  }
};
// PWP-V0-008, PWP-V0-037: each runner version's own configured system-Chrome version.
const qualifiedSystemBrowsers = {'1.63.0': 'Google Chrome 153.0.8010.48', '1.64.0': 'Google Chrome 155.0.8059.85'};
// PWP-V0-054: the external Node pre-filter, per runner and browser source, is a
// copy of the shared Go table's external-profile rows; a Go test fails on drift.
// These two declarations are the reporter's only Node release literals.
const externalBundledNodes = {'1.63.0': ['v22.23.2', 'v24.11.1'], '1.64.0': ['v22.23.2', 'v24.11.1']};
const externalSystemNodes = {'1.63.0': ['v22.23.2'], '1.64.0': ['v22.23.2']};
const bundledBrowsers = new Map();
const observedVersions = new Map();
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
    for (const attempt of test.attemptDetails || []) {
      attempt.failureMessage = scrubRiskText(attempt.failureMessage, values, protectedReport);
      for (const artifact of attempt.artifacts || []) {
        artifact.name = scrubRiskText(artifact.name, values, protectedReport);
        artifact.path = scrubRiskText(artifact.path, values, protectedReport);
      }
    }
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

// failureLocation is PWP-V3-008's location of a failed attempt: the innermost stack frame of the
// first located error that lies in the test's own file, else that error's reported location.
function failureLocation(errors, testFile) {
  for (const e of errors) {
    if (!e) continue;
    if (typeof e.stack === 'string' && typeof testFile === 'string' && testFile !== '') {
      for (const frame of e.stack.split('\n').slice(0, 256)) {
        if (!/^\s+at /.test(frame)) continue;
        const i = frame.lastIndexOf(testFile + ':');
        const m = i < 0 ? null : /^(\d+):\d+\)?\s*$/.exec(frame.slice(i + testFile.length + 1));
        if (m && Number(m[1]) > 0) return {file: testFile, line: Number(m[1])};
      }
    }
    const l = e.location;
    if (l && typeof l.file === 'string' && l.file !== '' && Number.isInteger(l.line) && l.line > 0) return {file: l.file, line: l.line};
  }
  return undefined;
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
  if (observedVersions.has(executablePath)) return observedVersions.get(executablePath);
  const stdout = childProcess.spawnSync(executablePath, ['--version'], {encoding: 'utf8'}).stdout;
  const version = typeof stdout === 'string' ? stdout.trim() : '';
  observedVersions.set(executablePath, version);
  return version;
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

// PWP-V0-009, PWP-V0-033, PWP-V0-036: the external pre-filter admits a runner's
// bundled Node releases from externalBundledNodes. The Go predicate stays the
// authority for which profile may project it as passing.
// PWP-V0-052: the freshness (runner, Node) rows are not repeated here; the
// provider passes them from its one shared table as freshnessTuples.
function qualifiedBundledIdentity(browser, version, freshnessProfile = false, freshnessTuples = []) {
  const qualified = Object.hasOwn(qualifiedBundledBrowsers, version) ? qualifiedBundledBrowsers[version] : null;
  const nodes = Object.hasOwn(externalBundledNodes, version) ? externalBundledNodes[version] : [];
  return qualified && browser && browser.platform === 'darwin' && browser.arch === 'arm64' && (freshnessProfile ? freshnessTuples.some(row => row[0] === version && row[1] === browser.nodeVersion) : nodes.includes(browser.nodeVersion)) &&
    browser.browserType === 'chromium' && browser.browserVersion === qualified.observedVersion && browser.channel === '' &&
    browser.executableSource === 'playwright-bundled' && browser.executableName === qualified.executableName &&
    browser.executablePath.endsWith(qualified.pathSuffix) && browser.executableSha256 === qualified.executableSha256 &&
    browser.browserRevision === qualified.revision && browser.manifestBrowserVersion === qualified.manifestVersion &&
    browser.headlessShellAvailable === true;
}

// PWP-V3-013, PWP-V3-014: Playwright caches each fixture, hook and test function's
// parameter names on the function under a 'signature' symbol when it builds the fixture
// pool. A function without one (a deserialized or stub object) has unknown dependencies.
const builtinFixtureFile = '/playwright/lib/index.js';
const browserFixtureNames = ['browser', 'context', 'page'];
function fixtureSignature(fn) {
  if (typeof fn !== 'function') return null;
  const symbol = Object.getOwnPropertySymbols(fn).find(s => s.description === 'signature');
  const names = symbol ? fn[symbol] : null;
  return Array.isArray(names) && names.every(name => typeof name === 'string') ? names : null;
}

// fixtureRegistry mirrors the fixture pool a test resolves against: each name's registrations
// in order, test-type registrations first, then describe-level overrides. A project option value
// takes effect right after the option's last defining registration, so a later test.extend
// override still wins, as in Playwright; a project value for a registered non-option fixture is
// unknown. Any override of a browser, context, page or playwright fixture, or of a private '_'
// fixture, from configuration or test.use() is unknown; so is a test type without Playwright's
// own fixtures, and a project fixture with a custom title, because the reporter recognizes
// fixtures by their step titles.
function fixtureRegistry(test, project) {
  const registrations = test._testType?.fixtures;
  if (!Array.isArray(registrations) || !project || !project.use || typeof project.use !== 'object') return null;
  const registry = new Map();
  let retitled = false;
  const entry = (name, value, builtin) => {
    const tuple = Array.isArray(value) && value[1] !== null && typeof value[1] === 'object' ? value : [value, {}];
    if (!builtin && tuple[1].title !== undefined && tuple[1].title !== name) retitled = true;
    return {name, fn: typeof tuple[0] === 'function' ? tuple[0] : null, options: tuple[1], builtin};
  };
  const add = (name, value, builtin) => {
    if (!registry.has(name)) registry.set(name, []);
    registry.get(name).push(entry(name, value, builtin));
  };
  let builtinSeen = false;
  for (const registration of registrations) {
    if (!registration.fixtures || typeof registration.location?.file !== 'string') return null;
    const builtin = registration.location.file.endsWith(builtinFixtureFile);
    builtinSeen ||= builtin;
    for (const [name, value] of Object.entries(registration.fixtures)) add(name, value, builtin);
  }
  if (!builtinSeen) return null;
  for (const [name, value] of Object.entries(project.use)) {
    if (browserFixtureNames.includes(name) || name === 'playwright' || name.startsWith('_')) return null;
    const stack = registry.get(name);
    if (!stack) { add(name, value, false); continue; }
    const at = stack.findLastIndex(registered => registered.options.option);
    if (at < 0) return null;
    stack.splice(at + 1, 0, entry(name, value, false));
  }
  const overrides = [];
  const parents = [];
  for (let suite = test.parent; suite; suite = suite.parent) parents.unshift(suite);
  for (const suite of parents) {
    if (!Array.isArray(suite._use)) return null;
    for (const override of suite._use) overrides.push(override?.fixtures);
  }
  for (const fixtures of overrides) {
    if (!fixtures || typeof fixtures !== 'object') return null;
    for (const [name, value] of Object.entries(fixtures)) {
      if (browserFixtureNames.includes(name) || name === 'playwright' || name.startsWith('_')) return null;
      add(name, value, false);
    }
  }
  return retitled ? null : registry;
}

// fixtureClosure walks the fixtures the named roots depend on, resolving a fixture's dependency on
// its own name to the registration it overrides, as Playwright does. It reports whether the walk
// reached any browser, context or page registration, whether it reached Playwright's own browser
// fixture, and whether the closure is unknown: an unregistered name, a project fixture that is
// boxed (its steps are hidden from reporters) or has no signature, or a project fixture that
// depends on the playwright fixture or a private '_' fixture (and so can launch or connect its
// own browser).
function fixtureClosure(registry, roots) {
  const closure = {unknown: false, browserFixture: false, builtinBrowser: false};
  const seen = new Set();
  const visit = (name, index, fromProject) => {
    if (closure.unknown) return;
    if (fromProject && (name === 'playwright' || name.startsWith('_'))) { closure.unknown = true; return; }
    const stack = registry.get(name);
    if (!stack) { closure.unknown = true; return; }
    const at = index === undefined ? stack.length - 1 : index;
    if (at < 0) { closure.unknown = true; return; }
    if (seen.has(`${at}:${name}`)) return;
    seen.add(`${at}:${name}`);
    const entry = stack[at];
    if (browserFixtureNames.includes(name)) closure.browserFixture = true;
    if (entry.builtin && name === 'browser') { closure.builtinBrowser = true; return; }
    if (!entry.fn) return;
    if (!entry.builtin && entry.options.box) { closure.unknown = true; return; }
    const names = fixtureSignature(entry.fn);
    if (!names) { closure.unknown = true; return; }
    for (const dep of names) visit(dep, dep === name ? at - 1 : undefined, !entry.builtin);
  };
  for (const root of roots) visit(root.name, undefined, root.fromProject);
  return closure;
}

// testFixtureRoots lists every fixture the test can resolve: the test function's, each hook's and
// each modifier's parameters in its describe chain, and every auto fixture. It is null when one of
// those functions has no signature.
function testFixtureRoots(test, registry) {
  const roots = [];
  const addFunction = fn => {
    const names = fixtureSignature(fn);
    if (!names) return false;
    for (const name of names) roots.push({name, fromProject: true});
    return true;
  };
  if (!addFunction(test.fn)) return null;
  for (let suite = test.parent; suite; suite = suite.parent) {
    if (!Array.isArray(suite._hooks) || !Array.isArray(suite._modifiers)) return null;
    for (const hook of suite._hooks) if (!addFunction(hook?.fn)) return null;
    for (const modifier of suite._modifiers) if (typeof modifier?.fn === 'function' && !addFunction(modifier.fn)) return null;
  }
  for (const [name, stack] of registry) if (stack.some(entry => entry.options.auto)) roots.push({name, fromProject: false});
  return roots;
}

// PWP-V3-014: steps a browserless attempt may report. Hooks, test steps, attachments and
// fixtures other than browser, context and page are containers; an expect step must be a generic
// or API-response matcher, and a Playwright API step must be an API request. Anything else,
// including a step category this list does not name, is browser use. The Worker Cleanup hook
// tears down worker fixtures that earlier tests set up and reported; it is not this test's use.
const browserlessExpect = /^Expect "(not )?(toBe|toBeCloseTo|toBeDefined|toBeFalsy|toBeGreaterThan|toBeGreaterThanOrEqual|toBeInstanceOf|toBeLessThan|toBeLessThanOrEqual|toBeNaN|toBeNull|toBeTruthy|toBeUndefined|toContain|toContainEqual|toEqual|toHaveLength|toHaveProperty|toMatch|toMatchObject|toStrictEqual|toThrow|toThrowError|toBeOK)"$/;
const browserlessApi = /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|\{method\}|Create request context|Get response body)$/;
function browserlessSteps(steps) {
  for (const step of steps || []) {
    if (step.category === 'hook' && step.title === 'Worker Cleanup') continue;
    if (step.category === 'fixture') { if (browserFixtureNames.some(name => step.title === `Fixture "${name}"`)) return false; }
    else if (step.category === 'expect') { if (!browserlessExpect.test(step.title)) return false; }
    else if (step.category === 'pw:api') { if (!browserlessApi.test(step.title)) return false; }
    else if (!['hook', 'test.step', 'test.attach'].includes(step.category)) return false;
    if (!browserlessSteps(step.steps)) return false;
  }
  return true;
}

// foreignLaunch reports a Playwright API launch or connect step outside a fixture named browser:
// a browser the test, a hook or a project fixture started itself.
function foreignLaunch(steps, underBrowser = false) {
  return (steps || []).some(step => (!underBrowser && step.category === 'pw:api' && /launch|connect/i.test(step.title)) ||
    foreignLaunch(step.steps, underBrowser || (step.category === 'fixture' && step.title === 'Fixture "browser"')));
}

// Qualified against 1.60.0, 1.63.0 and 1.64.0 in-process reporter objects. Serialization or a
// custom executable fixture can erase effective options; that is unknown.
// PWP-V3-013: on 1.63.0 and 1.64.0 a project fixture that overrides only page and/or context
// keeps the launch identity when it is not boxed, has a signature, depends on no playwright or
// private '_' fixture, and resolves through Playwright's own browser fixture; the override names
// are recorded as corvintFixtureOverrides. Overrides of browser, playwright, _browserOptions,
// _combinedContextOptions or _optionConnectOptions, and page or context overrides from
// configuration or test.use(), stay unknown.
function effectiveUse(test, project, version, freshnessProfile = false, freshnessTuples = []) {
  if (!['1.60.0', '1.63.0', '1.64.0'].includes(version) || !Array.isArray(test._testType?.fixtures)) return null;
  const use = {};
  const pageOverrides = new Set();
  const assign = (fixtures, builtin, registration = false) => {
    if (!builtin && ['browser', 'playwright', '_browserOptions', '_combinedContextOptions', '_optionConnectOptions'].some(k => Object.hasOwn(fixtures, k))) return false;
    for (const key of ['context', 'page']) {
      if (builtin || !Object.hasOwn(fixtures, key)) continue;
      if (!registration || version === '1.60.0') return false;
      pageOverrides.add(key);
    }
    for (const key of (freshnessProfile ? [...identityKeys, 'baseURL'] : identityKeys)) {
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
    if (!assign(fixture.fixtures, fixture.location.file.endsWith(builtinFixtureFile), true)) return null;
  }
  if (pageOverrides.size) {
    const registry = fixtureRegistry(test, project);
    if (!registry) return null;
    for (const name of pageOverrides) {
      const closure = fixtureClosure(registry, [{name, fromProject: false}]);
      if (closure.unknown || !closure.builtinBrowser) return null;
    }
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
  if (pageOverrides.size) resolved.corvintFixtureOverrides = [...pageOverrides].sort();
  if (resolved.channel === undefined && use.launchOptions?.channel !== undefined) resolved.channel = use.launchOptions.channel;
  if (version === '1.63.0' || version === '1.64.0') {
    resolved.headless = Object.hasOwn(resolved, 'headless') ? resolved.headless : resolved.launchOptions?.headless ?? true;
    if (Object.hasOwn(resolved, 'connectOptions') || process.env.PW_TEST_CONNECT_WS_ENDPOINT) return null;
    const executablePath = resolved.launchOptions?.executablePath || '';
    const executableVersion = executablePath ? observedVersion(executablePath) : '';
    const bundledChromium = playwright.chromium.executablePath();
    const bundledMatch = bundledChromium.match(/^(.*)\/chromium-(\d+)\//);
    const headlessShell = bundledMatch ? path.join(bundledMatch[1], `chromium_headless_shell-${bundledMatch[2]}`, 'chrome-headless-shell-mac-arm64', 'chrome-headless-shell') : '';
    resolved.channel = resolved.channel || '';
    resolved.corvintBrowser = executablePath ? {platform: process.platform, arch: process.arch, nodeVersion: process.version, browserType: resolved.browserName, browserVersion: executableVersion, channel: resolved.channel, executablePath, headlessShellAvailable: headlessShell !== '' && fs.existsSync(headlessShell)} : bundledBrowserIdentity(resolved.headless);
    const systemQualified = Object.hasOwn(qualifiedSystemBrowsers, version) && resolved.corvintBrowser?.platform === 'darwin' && resolved.corvintBrowser?.arch === 'arm64' && Object.hasOwn(externalSystemNodes, version) && externalSystemNodes[version].includes(resolved.corvintBrowser?.nodeVersion) && resolved.corvintBrowser?.browserType === 'chromium' && resolved.corvintBrowser?.browserVersion === qualifiedSystemBrowsers[version] && resolved.corvintBrowser?.channel === '' && resolved.corvintBrowser?.executablePath === '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' && resolved.corvintBrowser?.headlessShellAvailable === true && (version !== '1.64.0' || resolved.headless === true);
    if (resolved.corvintBrowser?.browserType !== resolved.browserName || (!(systemQualified && !freshnessProfile) && !qualifiedBundledIdentity(resolved.corvintBrowser, version, freshnessProfile, freshnessTuples))) return null;
  } else {
    delete resolved.headless;
    delete resolved.connectOptions;
  }
  return resolved;
}

// canonicalReporterOptions renders plain JSON data with sorted keys; anything
// else (functions, class instances, cycles, non-finite numbers) is unknown.
function canonicalReporterOptions(value, depth = 0, seen = new Set()) {
  if (value === null || typeof value === 'boolean' || typeof value === 'string') return JSON.stringify(value);
  if (typeof value === 'number' && Number.isFinite(value)) return JSON.stringify(value);
  if (typeof value !== 'object' || depth >= 32 || seen.has(value)) throw new Error('reporter-options-unknown');
  seen.add(value);
  try {
    if (Array.isArray(value)) return '[' + Array.from(value, item => canonicalReporterOptions(item, depth + 1, seen)).join(',') + ']';
    const prototype = Object.getPrototypeOf(value);
    if (prototype !== Object.prototype && prototype !== null) throw new Error('reporter-options-unknown');
    return '{' + Object.keys(value).sort().map(key => JSON.stringify(key) + ':' + canonicalReporterOptions(value[key], depth + 1, seen)).join(',') + '}';
  } finally { seen.delete(value); }
}

// keptReporterEntries reports every reporter Playwright received after this
// provider entry, which must be first and carry this run's private output path
// (PWP-V0-014): the provider copies each observation before a kept reporter
// receives the same callback arguments.
function keptReporterEntries(reporters, output) {
  const own = Array.isArray(reporters) ? reporters[0] : undefined;
  if (!Array.isArray(own) || own[1]?.output !== output) return undefined;
  return reporters.slice(1).map(entry => {
    const [name, options] = Array.isArray(entry) ? entry : [entry];
    if (typeof name !== 'string') return {name: '', options: 'unknown'};
    if (options === undefined) return {name, options: 'absent'};
    try { return {name, options: 'bound', optionsDigest: crypto.createHash('sha256').update(canonicalReporterOptions(options)).digest('hex')}; }
    catch { return {name, options: 'unknown'}; }
  });
}

// The config supplies only a private output path. Nothing is read from stdout.
class Reporter {
  constructor(options) { this.keepReporters = options.keepReporters === true; this.freshnessProfile = options.freshnessProfile === true; this.freshnessTuples = this.freshnessProfile && Array.isArray(options.freshnessTuples) ? options.freshnessTuples.filter(row => Array.isArray(row) && row.length === 2 && row.every(v => typeof v === 'string')) : []; this.path = options.output; this.tests = new Map(); this.errors = []; this.sensitiveInputPolicy = options.sensitiveInputPolicy || null; this.policy = sensitivePolicy(this.sensitiveInputPolicy); this.hasSensitiveInput = false; this.sensitiveValues = new Set(); this.sensitiveStepCount = 0; this.retainAttemptDetails = options.retainAttemptDetails === true; this.browserUse = new Map(); this.launchWorkers = new Set(); if (this.retainAttemptDetails && this.policy) throw new Error('external-attempt-details-composition-unsupported'); }
  onBegin(config, suite) {
	this.schedule = {workers: config.workers, starts: []};
    if (this.keepReporters) this.projectReporters = keptReporterEntries(config.reporter, this.path);
    this.version = config.version;
    this.files = {};
    this.configFiles = {};
    for (const file of Object.keys(require.cache)) {
      // Module cache keys are real paths, so the private output root is compared as one too: on a host
      // whose temp directory is a symlink (macOS /var) the provider's own files were bound as config inputs (PWP-V0-055).
      const outputRoot = fs.realpathSync(path.dirname(this.path));
      if ((!this.freshnessProfile && file.includes('/node_modules/')) || file.startsWith(outputRoot + '/')) continue;
      // A module moved or removed since it was loaded is skipped rather than aborting onBegin.
      try { if (fs.statSync(file).isFile()) this.configFiles[file] = crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'); } catch { continue; }
    }
    for (const test of suite.allTests()) {
      const file = test.location.file;
      this.files[file] = crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
    }
  }
  // PWP-V3-009: a /3 run keeps each run-level error's reported location, so a worker teardown
  // failure can be placed like a test failure instead of reading as infrastructure.
  onError(error) {
    this.errors.push(error.message || String(error));
    if (this.retainAttemptDetails) {
      const l = error && error.location;
      if (!this.errorLocations) this.errorLocations = [];
      this.errorLocations.push(l && typeof l.file === 'string' && l.file !== '' && Number.isInteger(l.line) && l.line > 0 ? {file: l.file, line: l.line} : null);
    }
  }
  onTestBegin(test, result) {
    const project = test.parent.project();
    const repeat = test.repeatEachIndex > 0 ? ` > repeat ${test.repeatEachIndex}` : '';
    this.schedule.starts.push({fullName: test.titlePath().join(' > ') + repeat, file: test.location.file, line: test.location.line, project: project?.name || '', retry: result.retry, retries: test.retries, worker: result.workerIndex, fullyParallel: project?.fullyParallel === true});
  }
  onTestEnd(test, result) {
    const project = test.parent.project();
    let use = project ? effectiveUse(test, project, this.version, this.freshnessProfile, this.freshnessTuples) : null;
    // PWP-V3-013: a page or context override that launched or connected a browser of its own in
    // any attempt has no launch identity; onEnd extends that to every worker that ran it.
    const launched = foreignLaunch(result.steps);
    const prior = this.browserUse.get(test.id);
    const overrides = Array.isArray(use?.corvintFixtureOverrides);
    const overrideLaunched = (prior?.overrideLaunched || false) || (launched && overrides);
    if (overrideLaunched) use = null;
    if (launched) this.launchWorkers.add(result.workerIndex);
    // PWP-V3-014: on 1.63.0 and 1.64.0, browserSteps is false while every attempt reported only
    // browserless steps; closureBrowserless is whether the test's fixture closure is known and
    // reaches no browser, context or page fixture. onEnd classifies the test once every worker
    // that started a browser outside a browser fixture is known.
    const tracked = ['1.63.0', '1.64.0'].includes(this.version);
    let closureBrowserless = prior ? prior.closureBrowserless : false;
    if (!prior && tracked) {
      const registry = project ? fixtureRegistry(test, project) : null;
      const roots = registry ? testFixtureRoots(test, registry) : null;
      const closure = roots ? fixtureClosure(registry, roots) : null;
      closureBrowserless = closure !== null && !closure.unknown && !closure.browserFixture;
    }
    const browserSteps = !tracked || (prior?.browserSteps || false) || !browserlessSteps(result.steps);
    this.browserUse.set(test.id, {closureBrowserless, browserSteps, overrides: (prior?.overrides || false) || overrides, overrideLaunched, workers: [...(prior?.workers || []), result.workerIndex]});
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
    // V1-3636: test.fail() declares expectedStatus 'failed'. As Playwright counts it, a body that
    // fails as declared passed, and a body that passed is an unexpected failure.
    if (test.expectedStatus === 'failed' && !infrastructure) {
      if (state === 'failed') { state = 'passed'; message = ''; }
      else if (state === 'passed') { state = 'failed'; message = message || 'test was expected to fail (test.fail()) but passed'; }
    }
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
    const artifacts = (result.attachments || []).filter(a => a.path).map(a => sensitive ? {name: redactionMarker, path: redactionMarker} : {name: a.name, path: a.path});

    let importedFiles;
    if (this.freshnessProfile) {
      const imports = (result.stdout || []).map(value => value.toString()).join('').split('\n').filter(line => line.startsWith('CORVINT-PTF-IMPORTS '));
      if (imports.length === 1 && imports[0].length <= 128 * 1024) {
        try { const parsed = JSON.parse(imports[0].slice('CORVINT-PTF-IMPORTS '.length));
          const outputRoot=fs.realpathSync(path.dirname(this.path));for (const name of ['playwright.config.cjs','reporter.cjs']) delete parsed?.[path.join(outputRoot,name)];
          if (parsed && !Array.isArray(parsed) && typeof parsed === 'object' && Object.keys(parsed).length <= 512 && Object.entries(parsed).every(([file,digest]) => path.isAbsolute(file) && typeof digest === 'string' && /^[a-f0-9]{64}$/.test(digest))) importedFiles=parsed;
        } catch {}
      }
    }
    const anchor = {file: test.location.file, line: test.location.line};
    const attemptDetails = previous?.attemptDetails || [];
    if (this.retainAttemptDetails) {
      if (result.retry !== attemptDetails.length || attemptDetails.length >= 32) throw new Error('external-attempt-details-invalid');
      // PWP-V3-008: where the failure happened, distinct from the test declaration anchor.
      const located = state === 'passed' ? undefined : failureLocation(errors, test.location.file);
      attemptDetails.push({state, retry: result.retry, durationMs: result.duration, failureMessage: message, anchor, artifacts: artifacts.map(a => ({...a})), ...(located ? {failureLocation: located} : {})});
    }
    this.tests.set(test.id, {
      name: test.title, fullName: test.titlePath().join(' > ') + repeat, state,
      project: {name: project ? project.name : '', browser: use ? use.browserName : '', device: project && typeof project.metadata?.device === 'string' ? project.metadata.device : 'unknown', use},
      retries: result.retry, durationMs: result.duration,
      anchor,
      failureMessage: message, attempts,
      artifacts, ...(this.freshnessProfile ? {importedFiles} : {}), ...(this.retainAttemptDetails ? {attemptDetails} : {})
    });
  }
  onEnd(result) {
    if (this.sensitiveRejection) throw this.sensitiveRejection;
    // PWP-V3-013: an override that ran in a worker where a browser was started outside a browser
    // fixture may have used that browser; its launch identity is unknown.
    // PWP-V3-014: a test whose identity is unknown and whose every attempt observed no browser use
    // records browser 'none' with one closed reason; a test whose identity is known keeps it, and a
    // test that observed browser use without a known identity stays unknown.
    for (const [id, test] of this.tests) {
      const observed = this.browserUse.get(id);
      if (!observed) continue;
      const launchWorker = observed.workers.some(worker => this.launchWorkers.has(worker));
      if (observed.overrides && launchWorker) { test.project.browser = ''; test.project.use = null; }
      if (test.project.use !== null || observed.browserSteps || launchWorker) continue;
      const reason = observed.closureBrowserless ? 'no-browser-use'
        : test.attempts.every(a => a.state === 'skipped') ? 'not-run'
        : test.attempts.every(a => a.state === 'infrastructure') ? 'fixture-failed-before-browser' : '';
      if (reason) { test.project.browser = 'none'; test.project.use = {corvintBrowserless: reason}; }
    }
    const tests = Array.from(this.tests.values());
    for (const test of tests) {
      if (test.state === 'passed' && test.attempts.some(a => a.state !== 'passed')) test.state = 'flaky';
    }
    if (this.hasSensitiveInput) this.errors = this.errors.map(() => redactionMarker);
    const report = {version: this.version, files: this.files, configFiles: this.configFiles, status: result.status, tests, errors: this.errors, ...(this.errorLocations ? {errorLocations: this.errorLocations} : {}), schedule: this.schedule, ...(this.sensitiveInputPolicy ? {sensitiveInputPolicy: this.sensitiveInputPolicy} : {}), ...(this.keepReporters && this.projectReporters ? {projectReporters: this.projectReporters} : {})};
    fs.writeFileSync(this.path, JSON.stringify(scrubReportRiskFields(report, this.sensitiveValues)));
  }
}
module.exports = Reporter;
