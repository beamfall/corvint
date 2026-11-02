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

test('PWP-V3-002 retains ordered detail and the flaky first attempt', () => {
  const Reporter = loadReporter();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-attempts-'));
  try {
    const output = path.join(dir, 'report.json');
    const reporter = new Reporter({output, retainAttemptDetails: true});
    const project = {name: 'chromium', use: {}, metadata: {}};
    const item = {id:'retry', title:'retry', retries:1, repeatEachIndex:0, location:{file:'/repo/test.cjs',line:8}, _testType:{fixtures:[]}, parent:{project:()=>project}, titlePath:()=>['retry']};
    reporter.onTestEnd(item, {status:'failed',retry:0,duration:7,errors:[{message:'first assertion failed',location:{file:'/repo/test.cjs',line:12,column:5}}],attachments:[{name:'screen',path:'/tmp/first.png'}],steps:[]});
    reporter.onTestEnd(item, {status:'passed',retry:1,duration:3,errors:[],attachments:[],steps:[]});
    reporter.onEnd({status:'passed'});
    const result = JSON.parse(fs.readFileSync(output,'utf8')).tests[0];
    assert.equal(result.state,'flaky');
    assert.deepEqual(result.attemptDetails.map(a=>[a.state,a.retry,a.durationMs]),[['failed',0,7],['passed',1,3]]);
    assert.equal(result.attemptDetails[0].failureMessage,'first assertion failed');
    assert.equal(result.attemptDetails[0].artifacts[0].path,'/tmp/first.png');
    assert.equal(result.attemptDetails[1].failureMessage,result.failureMessage);
    // PWP-V3-008: the failed attempt names its error location; the anchor stays the declaration.
    assert.deepEqual(result.attemptDetails[0].failureLocation,{file:'/repo/test.cjs',line:12});
    assert.deepEqual(result.attemptDetails[0].anchor,{file:'/repo/test.cjs',line:8});
    assert.equal(result.attemptDetails[1].failureLocation,undefined);
    assert.throws(()=>new Reporter({output,retainAttemptDetails:true,sensitiveInputPolicy:{}}),/composition-unsupported/);
  } finally { fs.rmSync(dir,{recursive:true,force:true}); }
});

test('PWP-V3-008 failure location prefers the innermost frame in the test file', () => {
  const Reporter = loadReporter();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-attempts-'));
  try {
    const output = path.join(dir, 'report.json');
    const reporter = new Reporter({output, retainAttemptDetails: true});
    const project = {name: 'chromium', use: {}, metadata: {}};
    const item = (id, repeatEachIndex) => ({id, title:'t', retries:0, repeatEachIndex, location:{file:'/repo/e2e/a.spec.ts',line:4}, _testType:{fixtures:[]}, parent:{project:()=>project}, titlePath:()=>['t']});
    // The page object throws; its own frame comes first, the scenario's cleanup call second.
    const stack = 'Error: cleanup failed /repo/e2e/a.spec.ts:2:1\n    at ClubPage.cleanup (/repo/e2e/pages/club.page.ts:30:11)\n    at /repo/e2e/a.spec.ts:19:5\n    at /repo/e2e/a.spec.ts:7:3';
    reporter.onTestEnd(item('own', 0), {status:'failed',retry:0,duration:1,errors:[{message:'cleanup failed',stack,location:{file:'/repo/e2e/pages/club.page.ts',line:30,column:11}}],attachments:[],steps:[]});
    // No frame in the test file: the error's own location stands.
    reporter.onTestEnd(item('elsewhere', 1), {status:'failed',retry:0,duration:1,errors:[{message:'x',stack:'Error: x\n    at helper (/repo/lib/h.ts:3:1)',location:{file:'/repo/lib/h.ts',line:3,column:1}}],attachments:[],steps:[]});
    // No location at all: the member is absent rather than the anchor.
    reporter.onTestEnd(item('none', 2), {status:'timedOut',retry:0,duration:1,errors:[{message:'Test timeout'}],attachments:[],steps:[]});
    reporter.onEnd({status:'failed'});
    const tests = JSON.parse(fs.readFileSync(output,'utf8')).tests;
    assert.deepEqual(tests.map(t=>t.attemptDetails[0].failureLocation),[{file:'/repo/e2e/a.spec.ts',line:19},{file:'/repo/lib/h.ts',line:3},undefined]);
  } finally { fs.rmSync(dir,{recursive:true,force:true}); }
});

test('PWP-V3-009 a /3 run keeps each run-level error location; other profiles emit none', () => {
  const Reporter = loadReporter();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-attempts-'));
  try {
    const output = path.join(dir, 'report.json');
    const attempt = new Reporter({output, retainAttemptDetails: true});
    attempt.onError({message: 'worker fixture teardown boom', location: {file: '/repo/e2e/a.spec.ts', line: 4, column: 51}});
    attempt.onError({message: 'unlocated'});
    attempt.onEnd({status: 'failed'});
    const report = JSON.parse(fs.readFileSync(output, 'utf8'));
    assert.deepEqual(report.errors, ['worker fixture teardown boom', 'unlocated']);
    assert.deepEqual(report.errorLocations, [{file: '/repo/e2e/a.spec.ts', line: 4}, null]);
    const legacy = new Reporter({output});
    legacy.onError({message: 'boom', location: {file: '/repo/e2e/a.spec.ts', line: 4}});
    legacy.onEnd({status: 'failed'});
    assert.equal(JSON.parse(fs.readFileSync(output, 'utf8')).errorLocations, undefined);
    const clean = new Reporter({output, retainAttemptDetails: true});
    clean.onEnd({status: 'passed'});
    assert.equal(JSON.parse(fs.readFileSync(output, 'utf8')).errorLocations, undefined);
  } finally { fs.rmSync(dir,{recursive:true,force:true}); }
});

test('PWP-V0-011 PWP-V0-014 keep mode reports kept entries after the first provider entry with order-independent option digests', () => {
  const Reporter = loadReporter();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-kept-'));
  const output = path.join(dir, 'report.json');
  const digest = text => require('node:crypto').createHash('sha256').update(text).digest('hex');
  const run = (reporter, keepReporters) => {
    const instance = new Reporter({output, keepReporters});
    instance.onBegin({workers: 1, version: '1.60.0', reporter}, {allTests: () => []});
    instance.onEnd({status: 'passed'});
    return JSON.parse(fs.readFileSync(output, 'utf8'));
  };
  const provider = [path.join(dir, 'reporter.cjs'), {output, keepReporters: true}];
  const kept = run([provider, ['list'], ['/repo/project.cjs', {b: [1, {d: true, c: null}], a: 'x'}], ['/repo/fn.cjs', {f: () => 1}], ['json', {outputFile: '/repo/r.json'}]], true);
  assert.deepEqual(kept.projectReporters, [
    {name: 'list', options: 'absent'},
    {name: '/repo/project.cjs', options: 'bound', optionsDigest: digest('{"a":"x","b":[1,{"c":null,"d":true}]}')},
    {name: '/repo/fn.cjs', options: 'unknown'},
    {name: 'json', options: 'bound', optionsDigest: digest('{"outputFile":"/repo/r.json"}')},
  ]);
  const reordered = run([provider, ['/repo/project.cjs', {a: 'x', b: [1, {c: null, d: true}]}]], true);
  assert.equal(reordered.projectReporters[0].optionsDigest, kept.projectReporters[1].optionsDigest);
  const cyclic = {}; cyclic.self = cyclic;
  for (const options of [cyclic, {n: Infinity}, {date: new Date(0)}, [1, , 3]]) {
    assert.equal(run([provider, ['/repo/x.cjs', options]], true).projectReporters[0].options, 'unknown');
  }
  assert.equal(run([provider, ['list']], false).projectReporters, undefined);
  // The retired provider-last order is never reported as kept entries.
  assert.equal(run([['list'], provider], true).projectReporters, undefined);
  assert.equal(run([[provider[0], {output: path.join(dir, 'other.json')}], ['list']], true).projectReporters, undefined);
  assert.equal(run(undefined, true).projectReporters, undefined);
});

test('PWP-V0-055 provider output files are never config inputs when the output path runs through a symlink', () => {
  const Reporter = loadReporter();
  const real = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-scratch-real-')));
  const link = path.join(fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-scratch-link-'))), 'scratch');
  fs.symlinkSync(real, link);
  const scratchModule = path.join(real, 'playwright.config.cjs');
  fs.writeFileSync(scratchModule, 'module.exports = {};\n');
  try {
    require(path.join(link, 'playwright.config.cjs'));
    assert.ok(Object.hasOwn(require.cache, scratchModule), 'module cache keys are real paths');
    const output = path.join(link, 'report.json');
    const reporter = new Reporter({output});
    reporter.onBegin({workers: 1, version: '1.64.0', reporter: []}, {allTests: () => []});
    reporter.onEnd({status: 'passed'});
    const report = JSON.parse(fs.readFileSync(path.join(real, 'report.json'), 'utf8'));
    assert.deepEqual(Object.keys(report.configFiles).filter(file => file.startsWith(real + '/')), []);
  } finally {
    delete require.cache[scratchModule];
    fs.rmSync(real, {recursive: true, force: true});
    fs.rmSync(path.dirname(link), {recursive: true, force: true});
  }
});

// PWP-V3-013..014 fixtures: Playwright caches each fixture function's parameter names under a
// symbol described 'signature'; the reporter reads that cache, so the doubles carry one.
function signed(names, fn = async () => {}) {
  fn[Symbol('signature')] = names;
  return fn;
}

function loadReporterWithBrowser(qualified) {
  const filename = path.join(__dirname, 'qualified-reporter.cjs');
  const source = fs.readFileSync(filename, 'utf8');
  const module = {exports: {}};
  const fakeRequire = name => name === 'playwright' ? {chromium: {executablePath: () => ''}} : require(name);
  fakeRequire.resolve = name => name === 'playwright' ? 'playwright' : require.resolve(name);
  fakeRequire.cache = require.cache;
  const browser = {platform: 'darwin', arch: 'arm64', nodeVersion: process.version, browserType: 'chromium', browserVersion: 'v', channel: '', executableSource: 'playwright-bundled', executableName: 'chromium-headless-shell', executablePath: '/b/chrome-headless-shell', executableSha256: 'd', browserRevision: '1248', manifestBrowserVersion: 'm', headlessShellAvailable: true};
  const stub = 'bundledBrowserIdentity = () => (qualified ? browser : null); qualifiedBundledIdentity = () => qualified;';
  return vm.runInThisContext(`(function(require,module,exports,__filename,__dirname,qualified,browser){${source}\n${stub}\nreturn module.exports;})`, {filename})(fakeRequire, module, module.exports, filename, __dirname, qualified, browser);
}

function builtinRegistration() {
  return {location: {file: '/repo/node_modules/playwright/lib/index.js'}, fixtures: {
    playwright: [signed([]), {scope: 'worker'}], browserName: ['chromium', {option: true}], headless: [true, {option: true}], storageState: [undefined, {option: true}],
    browser: [signed(['playwright']), {scope: 'worker'}], context: signed(['browser']), page: signed(['context']),
    request: signed(['playwright'])
  }};
}

function fixtureTest(id, fnNames, registrations, extra = {}) {
  const project = {name: 'chromium', use: {}, metadata: {}};
  const suite = {title: 'file', _use: [], _hooks: [], _modifiers: [], parent: undefined, project: () => project};
  return {id, title: id, retries: 0, repeatEachIndex: 0, location: {file: '/repo/a.spec.ts', line: 1}, fn: signed(fnNames),
    _testType: {fixtures: [builtinRegistration(), ...registrations]}, parent: suite, titlePath: () => [id], ...extra};
}

function runFixtureTests(Reporter, cases, version = '1.64.0') {
  const output = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-fixtures-')), 'report.json');
  const reporter = new Reporter({output});
  reporter.version = version;
  for (const [testCase, result] of cases) reporter.onTestEnd(testCase, {retry: 0, workerIndex: 0, duration: 1, errors: [], attachments: [], ...result});
  reporter.onEnd({status: 'passed'});
  return Object.fromEntries(JSON.parse(fs.readFileSync(output, 'utf8')).tests.map(t => [t.name, t.project]));
}

const browserSteps = (extra = []) => [{category: 'hook', title: 'Before Hooks', steps: [
  {category: 'fixture', title: 'Fixture "browser"', steps: [{category: 'pw:api', title: 'Launch browser'}]},
  {category: 'fixture', title: 'Fixture "context"', steps: [{category: 'pw:api', title: 'Create context'}]},
  {category: 'fixture', title: 'Fixture "page"', steps: [{category: 'pw:api', title: 'Create page'}, ...extra]}]},
  {category: 'pw:api', title: 'Set content'}];
const user = fixtures => ({location: {file: '/repo/fixtures.ts'}, fixtures});

test('PWP-V3-013 page and context overrides keep the launch identity; browser overrides stay unknown', () => {
  const Reporter = loadReporterWithBrowser(true);
  const projects = runFixtureTests(Reporter, [
    [fixtureTest('page override', ['page'], [user({page: signed(['context', 'storageState'])})]), {status: 'passed', steps: browserSteps()}],
    [fixtureTest('context override', ['page'], [user({context: signed(['browser'])})]), {status: 'passed', steps: browserSteps()}],
    [fixtureTest('browser override', ['page'], [user({browser: [signed(['playwright']), {scope: 'worker'}]})]), {status: 'passed', steps: browserSteps()}],
    [fixtureTest('private options override', ['page'], [user({_browserOptions: [signed([]), {scope: 'worker'}]})]), {status: 'passed', steps: browserSteps()}],
    [fixtureTest('combined options override', ['page'], [user({_combinedContextOptions: signed([])})]), {status: 'passed', steps: browserSteps()}],
    [fixtureTest('playwright page', ['page'], [user({page: signed(['playwright'])})]), {status: 'passed', steps: browserSteps()}],
    [fixtureTest('boxed page', ['page'], [user({page: [signed(['context']), {box: true}]})]), {status: 'passed', steps: browserSteps()}],
  ]);
  for (const name of ['page override', 'context override']) {
    assert.equal(projects[name].browser, 'chromium', name);
    assert.equal(projects[name].use.corvintBrowser.browserType, 'chromium', name);
  }
  assert.deepEqual(projects['page override'].use.corvintFixtureOverrides, ['page']);
  assert.deepEqual(projects['context override'].use.corvintFixtureOverrides, ['context']);
  for (const name of ['browser override', 'private options override', 'combined options override', 'playwright page', 'boxed page']) {
    assert.equal(projects[name].browser, '', name);
    assert.equal(projects[name].use, null, name);
  }
});

test('PWP-V3-013 an override in a worker that launched a browser outside the browser fixture is unknown', () => {
  const Reporter = loadReporterWithBrowser(true);
  const projects = runFixtureTests(Reporter, [
    [fixtureTest('launching override', ['page'], [user({page: signed(['context'])})]), {status: 'passed', steps: browserSteps([{category: 'pw:api', title: 'Launch browser'}])}],
    [fixtureTest('later override', ['page'], [user({page: signed(['context'])})]), {status: 'passed', steps: [{category: 'pw:api', title: 'Set content'}]}],
    [fixtureTest('other worker', ['page'], [user({page: signed(['context'])})]), {status: 'passed', workerIndex: 1, steps: browserSteps()}],
  ]);
  assert.equal(projects['launching override'].use, null);
  assert.equal(projects['later override'].use, null);
  assert.equal(projects['later override'].browser, '');
  assert.equal(projects['other worker'].browser, 'chromium');
});

test('PWP-V3-014 a test that used no browser records browser none with a closed reason', () => {
  const Reporter = loadReporterWithBrowser(false);
  const api = [{category: 'hook', title: 'Before Hooks', steps: [{category: 'fixture', title: 'Fixture "request"', steps: [{category: 'pw:api', title: 'Create request context'}]}]},
    {category: 'pw:api', title: 'GET'}, {category: 'expect', title: 'Expect "toBeOK"'}, {category: 'expect', title: 'Expect "toEqual"'},
    {category: 'hook', title: 'Worker Cleanup', steps: [{category: 'fixture', title: 'Fixture "browser"'}]}];
  const projects = runFixtureTests(Reporter, [
    [fixtureTest('api', ['request'], []), {status: 'passed', steps: api}],
    [fixtureTest('runtime skip', ['request'], []), {status: 'skipped', steps: api.slice(0, 1)}],
    [fixtureTest('serial skipped', ['page'], []), {status: 'skipped', workerIndex: -1, steps: []}],
    [fixtureTest('fixture failed', ['auth', 'page'], [user({auth: signed([])})]), {status: 'failed', workerIndex: 1, errors: [{message: 'auth down'}],
      steps: [{category: 'hook', title: 'Before Hooks', error: {message: 'auth down'}, steps: [{category: 'fixture', title: 'Fixture "auth"', error: {message: 'auth down'}}]}]}],
    [fixtureTest('browser used', ['page'], []), {status: 'passed', steps: browserSteps()}],
    [fixtureTest('page skipped after open', ['page'], []), {status: 'skipped', steps: browserSteps()}],
    [fixtureTest('api reaching page', ['request', 'page'], []), {status: 'passed', steps: api}],
    [fixtureTest('api in launch worker', ['request'], []), {status: 'passed', workerIndex: 2, steps: api}],
    [fixtureTest('test launch', [], []), {status: 'passed', workerIndex: 2, steps: [{category: 'pw:api', title: 'Launch browser'}]}],
    [fixtureTest('unsigned hook', ['request'], [], {parent: {title: 'f', _use: [], _hooks: [{type: 'beforeEach', fn: async () => {}}], _modifiers: [], project: () => ({name: 'chromium', use: {}, metadata: {}})}}), {status: 'passed', workerIndex: 3, steps: api}],
  ]);
  assert.deepEqual(projects.api, {...projects.api, browser: 'none', use: {corvintBrowserless: 'no-browser-use'}});
  assert.deepEqual(projects['runtime skip'].use, {corvintBrowserless: 'no-browser-use'});
  assert.deepEqual(projects['serial skipped'].use, {corvintBrowserless: 'not-run'});
  assert.deepEqual(projects['fixture failed'].use, {corvintBrowserless: 'fixture-failed-before-browser'});
  for (const name of ['browser used', 'page skipped after open', 'api in launch worker', 'test launch', 'unsigned hook']) {
    assert.equal(projects[name].browser, '', name);
    assert.equal(projects[name].use, null, name);
  }
  // A passing test whose closure reaches a page is not no-browser-use even with browserless steps.
  assert.equal(projects['api reaching page'].use, null);
  // Runner versions outside 1.63.0 and 1.64.0 never classify a browserless test.
  const legacy = runFixtureTests(Reporter, [[fixtureTest('api', ['request'], []), {status: 'passed', steps: api}]], '1.61.1');
  assert.equal(legacy.api.browser, '');
});

test('PWP-V3-013 a project fixture with a custom title is unknown, so a retitled launch cannot pass as the browser fixture', () => {
  const Reporter = loadReporterWithBrowser(true);
  const firefox = [{category: 'hook', title: 'Before Hooks', steps: [
    {category: 'fixture', title: 'Fixture "browser"', steps: [{category: 'pw:api', title: 'Launch browser'}]},
    {category: 'fixture', title: 'Fixture "context"', steps: [{category: 'pw:api', title: 'Create context'}]}]}];
  const projects = runFixtureTests(Reporter, [
    [fixtureTest('retitled page', ['page'], [user({page: [signed(['browser']), {title: 'browser'}]})]), {status: 'passed', steps: firefox}],
    [fixtureTest('own title', ['page'], [user({page: [signed(['context']), {title: 'page'}]})]), {status: 'passed', workerIndex: 1, steps: browserSteps()}],
  ]);
  assert.equal(projects['retitled page'].browser, '');
  assert.equal(projects['retitled page'].use, null);
  assert.deepEqual(projects['own title'].use.corvintFixtureOverrides, ['page']);
});

test('PWP-V3-014 a project option value applies before a later test.extend override, as in Playwright', () => {
  const Reporter = loadReporterWithBrowser(false);
  const withProject = (id, fnNames, registrations, use) => {
    const project = {name: 'chromium', use, metadata: {}};
    return fixtureTest(id, fnNames, registrations, {parent: {title: 'f', _use: [], _hooks: [], _modifiers: [], project: () => project}});
  };
  const truthy = [{category: 'expect', title: 'Expect "toBeTruthy"'}];
  const projects = runFixtureTests(Reporter, [
    [withProject('option then browser override', ['session'], [user({session: [null, {option: true}]}), user({session: signed(['browser'])})], {session: 'literal'}), {status: 'passed', steps: truthy}],
    [withProject('configured option', ['session'], [user({session: [null, {option: true}]})], {session: 'literal'}), {status: 'passed', workerIndex: 1, steps: truthy}],
    [withProject('configured non-option', ['session'], [user({session: signed([])})], {session: 'literal'}), {status: 'passed', workerIndex: 2, steps: truthy}],
  ]);
  assert.equal(projects['option then browser override'].browser, '');
  assert.equal(projects['option then browser override'].use, null);
  assert.deepEqual(projects['configured option'].use, {corvintBrowserless: 'no-browser-use'});
  assert.equal(projects['configured non-option'].use, null);
});

test('PWP-V3-014 a test whose launch identity is known keeps it', () => {
  const projects = runFixtureTests(loadReporterWithBrowser(true), [[fixtureTest('api', ['request'], []), {status: 'passed', steps: []}]]);
  assert.equal(projects.api.browser, 'chromium');
});

// V1-3636: test.fail() declares expectedStatus 'failed'. A body that fails as declared is a
// pass, and a body that passes is an unexpected outcome, the way Playwright counts them.
test('expectedStatus failed inverts the recorded state of a test.fail() test', () => {
  const Reporter = loadReporter();
  const output = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-expected-')), 'report.json');
  const reporter = new Reporter({output});
  const project = {name: 'chromium', use: {}, metadata: {}};
  const item = (id, expectedStatus) => ({id, title: id, retries: 0, repeatEachIndex: 0, expectedStatus, location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => project}, titlePath: () => [id]});
  const run = status => ({status, retry: 0, workerIndex: 0, duration: 1, errors: status === 'failed' ? [{message: 'expected failure'}] : [], attachments: [], steps: []});
  reporter.onTestEnd(item('fails-as-declared', 'failed'), run('failed'));
  reporter.onTestEnd(item('passes-unexpectedly', 'failed'), run('passed'));
  reporter.onTestEnd(item('ordinary-pass', 'passed'), run('passed'));
  reporter.onTestEnd(item('ordinary-fail', 'passed'), run('failed'));
  reporter.onEnd({status: 'failed'});
  const states = Object.fromEntries(JSON.parse(fs.readFileSync(output, 'utf8')).tests.map(t => [t.name, t]));
  assert.equal(states['fails-as-declared'].state, 'passed');
  assert.equal(states['passes-unexpectedly'].state, 'failed');
  assert.match(states['passes-unexpectedly'].failureMessage, /expected to fail/);
  assert.equal(states['passes-unexpectedly'].attempts[0].failureKind, 'assertion-or-test');
  assert.equal(states['ordinary-pass'].state, 'passed');
  assert.equal(states['ordinary-fail'].state, 'failed');
});

// V1-4066 (not reproduced): the suspicion was that `let state = result.status` lets a passing
// test.fail() count as a pass. It does not: the expectedStatus inversion right after it (V1-3636)
// applies on every attempt, so a test.fail() body that passes on each retry stays a failure, one
// whose retry fails as declared reads flaky, and a test.fail() body that times out is never a pass.
test('V1-4066 a test.fail() body that passes is never a pass on any attempt', () => {
  const Reporter = loadReporter();
  const output = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-expected-retry-')), 'report.json');
  const reporter = new Reporter({output});
  const project = {name: 'chromium', use: {}, metadata: {}};
  const item = id => ({id, title: id, retries: 1, repeatEachIndex: 0, expectedStatus: 'failed', location: {file: '/repo/test.cjs', line: 1}, _testType: {fixtures: []}, parent: {project: () => project}, titlePath: () => [id]});
  const run = (status, retry) => ({status, retry, workerIndex: 0, duration: 1, errors: status === 'failed' ? [{message: 'expected failure'}] : [], attachments: [], steps: []});
  reporter.onTestEnd(item('always-passes'), run('passed', 0));
  reporter.onTestEnd(item('always-passes'), run('passed', 1));
  reporter.onTestEnd(item('fails-on-retry'), run('passed', 0));
  reporter.onTestEnd(item('fails-on-retry'), run('failed', 1));
  reporter.onTestEnd(item('times-out'), run('timedOut', 0));
  reporter.onEnd({status: 'failed'});
  const states = Object.fromEntries(JSON.parse(fs.readFileSync(output, 'utf8')).tests.map(t => [t.name, t]));
  assert.equal(states['always-passes'].state, 'failed');
  assert.deepEqual(states['always-passes'].attempts.map(a => a.state), ['failed', 'failed']);
  assert.match(states['always-passes'].failureMessage, /expected to fail/);
  assert.equal(states['fails-on-retry'].state, 'flaky');
  assert.deepEqual(states['fails-on-retry'].attempts.map(a => a.state), ['failed', 'passed']);
  assert.notEqual(states['times-out'].state, 'passed');
  assert.equal(states['times-out'].attempts[0].failureKind, 'test-timeout');
});
