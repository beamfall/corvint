// Optional live-provider observations extend the existing host profile; old vectors are unchanged.
const keys = (value, allowed) => value !== null && typeof value === 'object' && !Array.isArray(value)
  && Object.keys(value).every((key) => allowed.includes(key));
const canonical = (value) => Array.isArray(value) ? value.map(canonical)
  : value !== null && typeof value === 'object' ? Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonical(value[key])])) : value;
const equal = (a, b) => JSON.stringify(canonical(a)) === JSON.stringify(canonical(b));
const bounded = (value) => typeof value === 'string' && Buffer.byteLength(value) <= 4096;
const phases = ['disabled', 'pending', 'running', 'completed', 'unavailable'];

export function checkLiveObservation(testCase, observation) {
  const expected = testCase.expect.liveTests;
  if (!expected) return [];
  if (observation.result === 'not-run') {
    return observation.liveTests === undefined && typeof observation.notRunReason === 'string' && observation.notRunReason.trim()
      ? [] : ['live NOT_RUN must have a reason and no fabricated live observations'];
  }
  if (!keys(expected, ['checkpoints', 'events', 'generationAdvances', 'minimumDelays'])) return ['unknown live expectation'];
  if (!Array.isArray(expected.checkpoints) || !Array.isArray(expected.events)) return ['invalid live expectation arrays'];
  const live = observation.liveTests;
  const failures = [];
  const check = (condition, message) => { if (!condition) failures.push(message); };
  if (!keys(live, ['trusted', 'checkpoints', 'events'])) return ['invalid closed liveTests observation'];
  check(typeof live.trusted === 'boolean' && live.trusted === testCase.setup.trusted, 'live workspace trust was not observed as requested');
  if (!Array.isArray(live.checkpoints) || live.checkpoints.length > 64) return ['invalid live checkpoints'];
  if (!Array.isArray(live.events) || live.events.length > 256) return ['invalid live events'];
  check(live.checkpoints.length === expected.checkpoints.length, 'live checkpoint count differs');
  for (const [index, point] of live.checkpoints.entries()) {
    if (!keys(point, ['step', 'phase', 'generation', 'items', 'diagnostics', 'output', 'documentDigest', 'inputIdentity', 'omitted', 'snapshotAbsent'])) {
      failures.push('unknown live checkpoint member');
      continue;
    }
    check(bounded(point.step), 'invalid live step');
    const absent = point.snapshotAbsent === true;
    check(!('snapshotAbsent' in point) || typeof point.snapshotAbsent === 'boolean', 'invalid snapshot absence flag');
    if (absent) {
      check(expected.checkpoints[index]?.snapshotAbsent === true, 'present snapshot reported absent');
      check(['phase', 'generation', 'documentDigest', 'inputIdentity', 'omitted'].every((key) => !(key in point)), 'absent snapshot carries snapshot fields');
      check(Array.isArray(point.items) && point.items.length === 0, 'absent snapshot carries items');
    } else {
      check(phases.includes(point.phase), 'invalid live phase');
      check(Number.isSafeInteger(point.generation) && point.generation >= 0, 'invalid live generation');
    }
    check(!('documentDigest' in point) || point.documentDigest === null || typeof point.documentDigest === 'string' && /^sha256:[a-f0-9]{64}$/u.test(point.documentDigest), 'invalid live document digest');
    check(!('inputIdentity' in point) || point.inputIdentity === null || bounded(point.inputIdentity), 'invalid live input identity');
    check(!('omitted' in point) || Number.isSafeInteger(point.omitted) && point.omitted >= 0, 'invalid live omission count');
    check(Array.isArray(point.items) && point.items.length <= 2048, 'invalid live items');
    check(Array.isArray(point.diagnostics) && point.diagnostics.length <= 2048, 'invalid live diagnostics');
    check(Array.isArray(point.output) && point.output.length <= 64 && point.output.every(bounded), 'invalid live output');
    for (const item of Array.isArray(point.items) ? point.items : []) check(keys(item, ['id', 'state']) && bounded(item.id) && ['passed', 'failed', 'skipped', 'errored', 'enqueued', 'started'].includes(item.state), 'invalid live item');
    for (const diagnostic of Array.isArray(point.diagnostics) ? point.diagnostics : []) check(keys(diagnostic, ['file', 'line', 'message']) && bounded(diagnostic.file) && Number.isSafeInteger(diagnostic.line) && diagnostic.line >= 0 && bounded(diagnostic.message), 'invalid live diagnostic');
    const want = expected.checkpoints[index];
    for (const [key, value] of Object.entries(want ?? {})) check(equal(point[key], value), `live checkpoint ${index} ${key} differs`);
  }
  // Events are observations from public actions, fixture IO and OS probes, never private callbacks.
  let prior = -1;
  for (const event of live.events) {
    if (!keys(event, ['kind', 'label', 'atMs', 'argv', 'env', 'groupAbsent'])) {
      failures.push('unknown live event member');
      continue;
    }
    check(['spawn', 'save', 'stdout', 'stdin-eof', 'exit', 'group-probe', 'capture', 'configure', 'shutdown', 'remove-root'].includes(event.kind) && bounded(event.label), 'invalid live event');
    check(Number.isFinite(event.atMs) && event.atMs >= 0 && event.atMs >= prior, 'live events are not monotonic');
    check(!('argv' in event) || Array.isArray(event.argv) && event.argv.length <= 128 && event.argv.every(bounded), 'invalid live argv');
    check(!('env' in event) || keys(event.env, Object.keys(event.env ?? {})) && Object.keys(event.env).length <= 32 && Object.entries(event.env).every(([key, value]) => bounded(key) && bounded(value)), 'invalid live environment');
    check(!('groupAbsent' in event) || typeof event.groupAbsent === 'boolean', 'invalid group absence observation');
    prior = event.atMs;
  }
  check(live.events.length === expected.events.length, 'live event count differs');
  for (const [index, event] of expected.events.entries()) {
    for (const [key, value] of Object.entries(event)) check(equal(live.events[index]?.[key], value), `live event ${index} ${key} differs`);
  }
  for (const [before, after] of expected.generationAdvances ?? []) {
    check(live.checkpoints[after]?.generation > live.checkpoints[before]?.generation, 'live generation did not advance');
  }
  for (const delay of expected.minimumDelays ?? []) {
    check(live.events[delay.to]?.atMs - live.events[delay.from]?.atMs >= delay.ms, 'live debounce interval was shortened');
  }
  return failures;
}
