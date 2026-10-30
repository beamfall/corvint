import { readFile, mkdir, lstat, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { run, cleanup, treeSHA, sha, fileSHA, save, assert } from './core-process.mjs';
const [tasks, source, root, output] = process.argv.slice(2);
for (const path of [tasks, source, root, output]) assert(path?.startsWith('/'), 'planning proof needs absolute paths');
const seed = join(source, 'script/seed-planning-store.sh'), data = JSON.parse(await readFile(join(source, 'script/seed-planning-store-data.json')));
const sourceSha256 = await fileSHA(join(source, 'docs/plans/integrated-product-roadmap-2026-09-12.md'));
const state = join(root, '.git/taskman');
async function retain(name, bytes) { await writeFile(join(output, name + '.json'), bytes, { flag: 'wx', mode: 0o600 }); }
function result(raw, command, outcome = 'OK') { const v = JSON.parse(raw); assert(v.profile === 'taskman-command-result/0' && JSON.stringify(v.command) === JSON.stringify(command) && v.outcome === outcome && v.mutation === null && !v.page?.truncated, 'Tasks result differs from native read contract'); return v; }
try {
  await mkdir(output, { recursive: true });
  const fixtureFiles = ['docs/plans/integrated-product-roadmap-2026-09-12.md', 'script/atm-fixture-store.sh', 'script/seed-planning-store-data.json', 'script/seed-planning-store.sh'];
  const fixtureRows = await Promise.all(fixtureFiles.map(async name => [name, (await lstat(join(source, name))).mode & 0o111 ? 'executable' : 'file', await fileSHA(join(source, name))]));
  await save(join(output, 'fixture-planning.json'), { name: 'planning', treeSha256: sha(JSON.stringify(fixtureRows)), packageSha256: '', lockSha256: '', configSha256: await fileSHA(join(source, 'script/seed-planning-store-data.json')), testSha256: '' });
  const first = await run(['/bin/sh', seed, tasks, root], source);
  const after = await treeSHA(state);
  await save(join(output, 'tasks-seed-first.json'), { profile: 'corvint-core-tasks-seed/0', phase: 'first', sourceSha256, storeBeforeSha256: sha('[]'), storeAfterSha256: after, stdout: first.toString() });
  await run(['/usr/bin/git', '-C', root, 'init', '-q'], root);
  await writeFile(join(root, 'README.md'), '# Disposable core planning fixture\n');
  await run(['/usr/bin/git', '-C', root, 'add', 'README.md'], root);
  await run(['/usr/bin/git', '-C', root, '-c', 'user.name=Core qualification', '-c', 'user.email=core@example.invalid', 'commit', '-qm', 'Core planning fixture'], root);
  const beforeReplay = await treeSHA(state), replay = await run(['/bin/sh', seed, tasks, root], source), afterReplay = await treeSHA(state);
  assert(beforeReplay === afterReplay, 'seed replay changed native store');
  await save(join(output, 'tasks-seed-replay.json'), { profile: 'corvint-core-tasks-seed/0', phase: 'replay', sourceSha256, storeBeforeSha256: beforeReplay, storeAfterSha256: afterReplay, stdout: replay.toString() });
  const roadmap = await run([tasks, 'roadmap', '--limit', '50'], root), mapped = result(roadmap, ['roadmap']);
  assert(mapped.items.length === 11 && mapped.page.total === '11', 'roadmap did not return eleven complete tickets');
  const ids = data.tickets.map(t => 'ticket:corvint:planning:' + t.localToken).sort();
  assert(JSON.stringify(mapped.items.map(t => t.ticketId).sort()) === JSON.stringify(ids), 'roadmap IDs differ from real seed');
  await retain('tasks-roadmap', roadmap);
  const responses = [];
  for (const expected of [...data.tickets].sort((a, b) => a.localToken.localeCompare(b.localToken))) {
    const raw = await run([tasks, 'ticket', 'show', expected.localToken], root), v = result(raw, ['ticket', 'show']); assert(v.items.length === 1, 'missing ticket detail');
    const r = v.items[0].record;
    assert(r.executionClass === 'MANUAL' && r.milestone === expected.milestone && r.order === expected.order && r.source.sourceRevisionSha256 === sourceSha256, 'planning execution/source/order differs');
    assert(JSON.stringify(r.acceptanceCriteria) === JSON.stringify(expected.acceptanceCriteria), 'acceptance criteria changed');
    assert(JSON.stringify([...r.requirementRefs].sort()) === JSON.stringify([...expected.requirementRefs].sort()), 'requirement references changed');
    assert(JSON.stringify(r.dependencies.map(d => d.ticketId).sort()) === JSON.stringify(expected.dependsOn.map(id => 'ticket:corvint:planning:' + id).sort()), 'dependency mapping changed');
    responses.push(raw.toString());
  }
  await save(join(output, 'tasks-ticket-details.json'), { profile: 'corvint-core-tasks-details/0', responses });
  const queue = await run([tasks, 'queue', 'status'], root), q = result(queue, ['queue', 'status']).items[0];
  assert(q.fixture === true && q.executionCutover === false && q.tickets === '11' && q.attempts === 'NOT_OBSERVED' && q.publication === 'NOT_OBSERVED', 'fixture execution boundary changed');
  const beforeAttempt = await treeSHA(state), notRun = await run([tasks, 'attempt'], root, 1), afterAttempt = await treeSHA(state); result(notRun, ['attempt'], 'NOT_RUN');
  assert(beforeAttempt === afterAttempt, 'omitted attempt command changed store');
  for (const directory of ['attempts', 'effects', 'worktrees']) { let absent = false; try { await lstat(join(state, directory)); } catch (error) { if (error.code === 'ENOENT') absent = true; else throw error; } assert(absent, 'execution lane artifact appeared'); }
  await save(join(output, 'tasks-no-dispatch.json'), { profile: 'corvint-core-tasks-no-dispatch/0', queueStatusRaw: queue.toString(), attemptNotRunRaw: notRun.toString(), stateBeforeSha256: beforeAttempt, stateAfterSha256: afterAttempt, laneArtifactsAbsent: true });
  console.log(JSON.stringify({ status: 'PASS', tickets: ids, nativeExecutionObservation: 'NOT_OBSERVED', sourceSha256 }));
} finally { await cleanup(); }
