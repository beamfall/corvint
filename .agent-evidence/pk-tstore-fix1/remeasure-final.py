import os, subprocess, pathlib, json
s=pathlib.Path('/home/russell/lanes/pk-tstore.fix')
out=s/'final.jsonl'
if out.exists():raise SystemExit('refuse overwrite samples')
prior=[json.loads(line) for line in (s/'narrow.jsonl').read_text().splitlines()]
assert len(prior)==40,'first paired phase must be complete'
with (s/'final.log').open('w') as log:
 log.write('Remeasure same fixtures, no reset or journal rewrite; repetitions 10..14 use the next five tickets after full repetitions 0..4 and preliminary narrowing repetitions 5..9. Nominal history n; actual pre-call seq n+1+4*rep (claim), n+4+4*rep (complete). Same cache/checkpoint clearing.\n');log.flush()
 for n in [13500,40000]:
  for rep in range(10,15):
   for version,binary in [('BASE',s/'base.test'),('HEAD',s/'head-final.test')]:
    env=dict(os.environ,PK_TSTORE_PAIR_DIR=str(s/f'full-{n}-{version}'),PK_TSTORE_PAIR_SIZE=str(n),PK_TSTORE_PAIR_REP=str(rep),PK_TSTORE_PAIR_VERSION=version,PK_TSTORE_PAIR_OUTPUT=str(out))
    print(f'final sample {n} {rep} {version}',flush=True)
    subprocess.run(['script/heavy.sh','--admit-only',str(binary),'-test.run=^TestATRV07203_PairedHoldSample$','-test.timeout=30m','-test.v'],env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
print('PASS final',flush=True)
