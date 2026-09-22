const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

function loadReporter(includeGrammar = false) {
  const filename = path.join(__dirname, 'qualified-reporter.cjs');
  const source = fs.readFileSync(filename, 'utf8');
  const module = {exports: {}};
  const fakeRequire = name => name === 'playwright' ? {chromium: {executablePath: () => ''}} : require(name);
  fakeRequire.resolve = name => name === 'playwright' ? 'playwright' : require.resolve(name);
  fakeRequire.cache = require.cache;
  const runtime = vm.runInThisContext(`(function(require,module,exports,__filename,__dirname){${source}\nreturn {Reporter: module.exports, sensitiveActionMatch, collectTailCandidates, sensitivePolicy};})`, {filename})(fakeRequire, module, module.exports, filename, __dirname);
  return includeGrammar ? runtime : runtime.Reporter;
}

test('reporter removes input values from the complete emitted report', () => {
  const Reporter = loadReporter();
  const output = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-redaction-')), 'report.json');
  const reporter = new Reporter({output, sensitiveInputPolicy: {additionalActionPatterns: ['custom entry'], additionalSensitiveFields: ['value']}});
  reporter.onError({message: 'global hunter2 p"ass metadata-secret'});
  const project = {name: 'chromium', use: {}, metadata: {}};
  const testCase = {
    id: 'id', title: 'assertion title', retries: 1, repeatEachIndex: 0,
    location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []},
    parent: {project: () => project}, titlePath: () => ['suite', 'assertion title']
  };
  const result = {
    status: 'failed', retry: 1, workerIndex: 0, duration: 1,
    errors: [{message: 'assertion mentions hunter2 and p"ass and metadata-secret'}],
    attachments: [{name: 'hunter2 screenshot', path: '/tmp/p"ass.png'}],
    steps: [{
      title: 'parent', error: {message: 'parent hunter2'}, attachments: [{name: 'parent hunter2', path: '/tmp/hunter2'}],
      steps: [
        {title: ' Fill "#password" with hunter2', category: 'pw:api', error: {message: 'hunter2 failed'}},
        {title: 'TYPE "p\\"ass"', category: 'pw:api'},
        {title: 'Type unquoted-secret', category: 'pw:api'},
        {title: 'Expect visible', category: 'expect', error: {message: 'actual unquoted-secret'}, attachments: [{name: 'unquoted-secret screenshot', path: '/tmp/unquoted-secret.png'}]},
        {title: 'provider/custom-entry(metadata-secret)', metadata: {value: 'metadata-secret', selector: '#safe'}}
      ]
    }]
  };
  reporter.onTestEnd(testCase, {...result, retry: 0});
  reporter.onTestEnd(testCase, result);
  reporter.onEnd({status: 'failed'});
  const raw = fs.readFileSync(output, 'utf8');
  for (const leaked of ['hunter2', 'p"ass', 'p\\"ass', 'unquoted-secret', 'metadata-secret']) assert.equal(raw.includes(leaked), false, raw);
  const report = JSON.parse(raw);
  const steps = report.tests[0].attempts[0].steps[0].steps;
	assert.equal(report.tests[0].attempts.length, 2);
  assert.equal(steps[0].title, 'Fill "[REDACTED]"');
  assert.equal(steps[1].title, 'Type "[REDACTED]"');
  assert.equal(steps[2].title, 'Type "[REDACTED]"');
  assert.equal(steps[3].title, 'Expect visible');
  assert.match(steps[3].error, /\[REDACTED\]/);
  assert.equal(steps[4].title, 'custom entry "[REDACTED]"');
  assert.equal(steps[4].metadata.selector, '#safe');
  assert.equal(report.tests[0].failureMessage, '[REDACTED]');
  assert.equal(report.errors[0], '[REDACTED]');
});

test('short input values do not corrupt structural report strings', () => {
  const Reporter = loadReporter();
  const output = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-redaction-short-')), 'report.json');
  const reporter = new Reporter({output, sensitiveInputPolicy: {}});
  const project = {name: 'chromium', use: {}, metadata: {}};
  reporter.onTestEnd({id: 'stable-id', title: 'assertion title', retries: 0, repeatEachIndex: 0, location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => project}, titlePath: () => ['assertion title']}, {status: 'passed', retry: 0, workerIndex: 0, duration: 1, errors: [], attachments: [], steps: [{title: 'Fill "a"'}, {title: 'Expect navigation'}]});
  reporter.onEnd({status: 'passed'});
  const report = JSON.parse(fs.readFileSync(output, 'utf8'));
  assert.equal(report.status, 'passed');
  assert.equal(report.tests[0].state, 'passed');
  assert.equal(report.tests[0].name, 'assertion title');
  assert.equal(report.tests[0].fullName, 'assertion title');
  assert.equal(report.tests[0].attempts[0].steps[1].title, 'Expect navigation');
});

test('reporter enforces the aggregate retry step bound before recursion', () => {
  const Reporter = loadReporter();
  const reporter = new Reporter({output: path.join(os.tmpdir(), 'unused-report.json'), sensitiveInputPolicy: {}});
  const testCase = {id: 'id', title: 'bounded', retries: 0, repeatEachIndex: 0, location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => ({name: 'p', use: {}, metadata: {}})}, titlePath: () => ['bounded']};
  const result = {status: 'failed', retry: 0, workerIndex: 0, duration: 1, errors: [], attachments: [], steps: Array.from({length: 4097}, () => ({title: 'navigation'}))};
  assert.throws(() => reporter.onTestEnd(testCase, result), /sensitive-input-step-bound-exceeded/);
});

test('reporter rejects over-depth raw steps before fixture recursion', () => {
  const Reporter = loadReporter();
  const reporter = new Reporter({output: path.join(os.tmpdir(), 'unused-depth-report.json'), sensitiveInputPolicy: {}});
  let step = {title: 'leaf'};
  for (let i = 0; i < 33; i++) step = {title: 'parent', steps: [step]};
  const testCase = {id: 'id', title: 'bounded', retries: 0, repeatEachIndex: 0, location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => ({name: 'p', use: {}, metadata: {}})}, titlePath: () => ['bounded']};
  const result = {status: 'failed', retry: 0, workerIndex: 0, duration: 1, errors: [], attachments: [], steps: [step]};
  assert.throws(() => reporter.onTestEnd(testCase, result), /sensitive-input-depth-exceeded/);
});

test('receiver-prefixed unquoted actions protect sibling and other-test errors', () => {
  for (const title of ['keyboard.insertText unquoted-secret', 'Ⱥ. InSeRtText: unquoted-secret', 'keyboard/insert \t text unquoted-secret', 'locator. press   sequentially(unquoted-secret)']) {
    const Reporter = loadReporter();
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-receiver-'));
    try {
      const output = path.join(dir, 'report.json');
      const reporter = new Reporter({output, sensitiveInputPolicy: {}});
      const testCase = {id: 'id', title: 'assertion title', retries: 1, repeatEachIndex: 0, location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => ({name: 'p', use: {}, metadata: {}})}, titlePath: () => ['assertion title']};
      const result = {status: 'failed', retry: 0, workerIndex: 0, duration: 1, errors: [], attachments: [], steps: [{title}, {title: 'Expect visible', error: {message: 'actual unquoted-secret'}}]};
      reporter.onTestEnd(testCase, result);
      reporter.onTestEnd({...testCase, id: 'sibling'}, {...result, steps: [], errors: [{message: 'actual unquoted-secret'}]});
      reporter.onEnd({status: 'failed'});
      const raw = fs.readFileSync(output, 'utf8');
      assert.equal(raw.includes('unquoted-secret'), false, raw);
      assert.equal(JSON.parse(raw).tests[0].attempts[0].steps[1].title, 'Expect visible');
    } finally { fs.rmSync(dir, {recursive: true}); }
  }
});

test('already-redacted input protects arbitrary risk fields across retries', () => {
  const Reporter = loadReporter();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-pre-redacted-'));
  try {
    const output = path.join(dir, 'report.json');
    const reporter = new Reporter({output, sensitiveInputPolicy: {}});
    const testCase = {id: 'id', title: 'assertion title', retries: 1, repeatEachIndex: 0, location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => ({name: 'p', use: {}, metadata: {}})}, titlePath: () => ['assertion title']};
    const result = {status: 'failed', retry: 0, workerIndex: 0, duration: 1, errors: [], attachments: [], steps: [{title: 'Fill "[REDACTED]"', redacted: true, error: {message: 'hunter2'}}, {title: 'Expect visible', error: {message: 'hunter2'}}]};
    reporter.onTestEnd(testCase, result);
    reporter.onTestEnd(testCase, {...result, retry: 1, steps: [{title: 'Navigate /account', error: {message: 'hunter2'}, attachments: [{name: 'hunter2', path: '/tmp/hunter2'}]}], errors: [{message: 'hunter2'}], attachments: [{name: 'hunter2', path: '/tmp/hunter2'}]});
    reporter.onError({message: 'hunter2'});
    reporter.onEnd({status: 'failed'});
    const raw = fs.readFileSync(output, 'utf8');
    assert.equal(raw.includes('hunter2'), false, raw);
    const report = JSON.parse(raw);
    assert.equal(report.tests[0].attempts[0].steps[1].title, 'Expect visible');
    assert.equal(report.tests[0].attempts[1].steps[0].title, 'Navigate /account');
  } finally { fs.rmSync(dir, {recursive: true}); }
});

test('Unicode action grammar and custom arguments protect the entire report', () => {
  for (const title of ['(Fill) unquoted-secret', '\u00a0Fill unquoted-secret', 'custom entry unquoted-secret', 'Fill(#password, unquoted-secret)', '\u0085(Ⱥ.\u2003InSeRt\u00a0TeXt)(unquoted-secret)', 'provider/custom-entry(unquoted-secret)', '“Fill” unquoted-secret', 'FİLL unquoted-secret', 'Fill "[REDACTED]"']) {
    const Reporter = loadReporter();
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-grammar-'));
    try {
      const output = path.join(dir, 'report.json');
      const reporter = new Reporter({output, sensitiveInputPolicy: {additionalActionPatterns: ['custom entry']}});
      const testCase = {id: 'id', title: 'input', retries: 1, repeatEachIndex: 0, location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => ({name: 'p', use: {}, metadata: {}})}, titlePath: () => ['input']};
      const result = {status: 'failed', retry: 0, workerIndex: 0, duration: 1, errors: [], attachments: [], steps: [{title}]};
      reporter.onTestEnd(testCase, result);
      reporter.onTestEnd({...testCase, id: 'other', title: 'other'}, {...result, errors: [{message: 'actual unquoted-secret'}], attachments: [{name: 'unquoted-secret', path: '/tmp/unquoted-secret'}], steps: [{title: 'Expect visible', error: {message: 'actual unquoted-secret'}}]});
      reporter.onError({message: 'actual unquoted-secret'});
      reporter.onEnd({status: 'failed'});
      const raw = fs.readFileSync(output, 'utf8');
      assert.equal(raw.includes('unquoted-secret'), false, raw);
      const report = JSON.parse(raw);
      assert.equal(report.tests[1].attempts[0].steps[0].title, 'Expect visible');
      assert.equal(report.tests[1].name, 'other');
      assert.equal(report.tests[1].state, 'failed');
    } finally { fs.rmSync(dir, {recursive: true}); }
  }
});

test('assertion and navigation titles do not enter the action grammar', () => {
  const Reporter = loadReporter();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-nonactions-'));
  try {
    const output = path.join(dir, 'report.json');
    const reporter = new Reporter({output, sensitiveInputPolicy: {additionalActionPatterns: ['custom entry']}});
    const titles = ['Expect input type to be text', 'Expect locator.fill to pass', 'Navigate /fill', 'Navigate https://example.test/type', 'Refill account', 'Fillable field', 'Expect custom entry to succeed', 'Expect page.getByLabel("Password").fill to fail', 'Navigate to page.getByLabel("Password").fill', 'page.goto("/fill")', 'page.getByText("fill").click()'];
    const testCase = {id: 'id', title: 'controls', retries: 0, repeatEachIndex: 0, location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => ({name: 'p', use: {}, metadata: {}})}, titlePath: () => ['controls']};
    reporter.onTestEnd(testCase, {status: 'failed', retry: 0, workerIndex: 0, duration: 1, errors: [], attachments: [], steps: titles.map(title => ({title, error: {message: 'ordinary assertion'}}))});
    reporter.onEnd({status: 'failed'});
    const steps = JSON.parse(fs.readFileSync(output, 'utf8')).tests[0].attempts[0].steps;
    assert.deepEqual(steps.map(step => step.title), titles);
    assert.ok(steps.every(step => step.error === 'ordinary assertion'));
  } finally { fs.rmSync(dir, {recursive: true}); }
});

test('argument candidates respect quoted commas, escapes and nested expressions', () => {
  const runtime = loadReporter(true);
  const policy = runtime.sensitivePolicy({additionalActionPatterns: ['custom entry']});
  const cases = [
    ['Fill("#pass,word", "secret,with(paren)")', ['#pass,word', 'secret,with(paren)'], ['secret', 'with(paren)']],
    ['Fill(select("#password", "label"), unquoted-secret)', ['select("#password", "label")', 'unquoted-secret'], ['select("#password"']],
    ['custom entry "p\\"ass"', ['p"ass'], []],
    ["Type 'p\\'ass'", ["p'ass"], []],
    ['Fill "#password" with hunter2', ['hunter2'], []]
  ];
  for (const [title, required, forbidden] of cases) {
    const action = runtime.sensitiveActionMatch(title, policy);
    assert.ok(action.display, title);
    const candidates = new Set();
    runtime.collectTailCandidates(action.tail, candidates);
    for (const value of required) assert.ok(candidates.has(value), title);
    for (const value of forbidden) assert.equal(candidates.has(value), false, title);
  }
});

test('every admitted additive pattern has matching Unicode separator semantics', () => {
  const runtime = loadReporter(true);
  for (const pattern of ['custom+entry', 'custom💠entry', 'custom\u200bentry', 'custom\u00a0entry']) {
    const policy = runtime.sensitivePolicy({additionalActionPatterns: [pattern]});
    assert.equal(runtime.sensitiveActionMatch(pattern + ' unquoted-secret', policy).display, 'custom entry');
  }
  for (const pattern of ['***', '+💠\u200b', '', 'x'.repeat(129)]) {
    assert.throws(() => runtime.sensitivePolicy({additionalActionPatterns: [pattern]}), error => error.message === 'sensitive-input-policy-invalid');
  }
});

test('unsupported sensitive receiver calls reject without writing any report', () => {
  const Reporter = loadReporter();
  for (const title of ['page.getByLabel("Password").fill("unquoted-secret")', 'page.locator("#password").first().type("unquoted-secret")', 'page["getByLabel"]("Password")["fill"]("unquoted-secret")', 'page.locator("#password").custom+entry("unquoted-secret")']) {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-unsupported-action-'));
    try {
      const output = path.join(dir, 'report.json');
      const reporter = new Reporter({output, sensitiveInputPolicy: {additionalActionPatterns: ['custom+entry']}});
      const testCase = {id: 'id', title: 'input', retries: 0, repeatEachIndex: 0, location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => ({name: 'p', use: {}, metadata: {}})}, titlePath: () => ['input']};
      const result = {status: 'failed', retry: 0, workerIndex: 0, duration: 1, errors: [{message: 'unquoted-secret'}], attachments: [], steps: []};
      reporter.onTestEnd({...testCase, id: 'prior'}, result);
      assert.throws(() => reporter.onTestEnd(testCase, {...result, steps: [{title}]}), error => error.code === 'sensitive-input-action-syntax-unsupported' && !error.message.includes('unquoted-secret'));
      assert.throws(() => reporter.onEnd({status: 'failed'}), error => error.code === 'sensitive-input-action-syntax-unsupported');
      assert.equal(fs.existsSync(output), false);
    } finally { fs.rmSync(dir, {recursive: true}); }
  }
});

test('a cached module that moved since load is skipped instead of aborting onBegin', () => {
  const Reporter = loadReporter();
  const output = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-moved-')), 'report.json');
  const moved = path.join(os.tmpdir(), 'corvint-moved-module-' + process.pid + '.cjs');
  require.cache[moved] = {id: moved, filename: moved, loaded: true, exports: {}};
  try {
    const reporter = new Reporter({output});
    reporter.onBegin({workers: 1, version: '1.60.0'}, {allTests: () => []});
    assert.equal(reporter.configFiles[moved], undefined);
    assert.ok(Object.keys(reporter.configFiles).includes(__filename));
  } finally {
    delete require.cache[moved];
  }
});
