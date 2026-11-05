import os, subprocess, pathlib, json
s=pathlib.Path('/home/russell/lanes/pk-tstore.fix')
out=s/'full.jsonl';rows=[json.loads(line) for line in out.read_text().splitlines()]
assert len(rows)==20 and all(r['Receipts']==13500 for r in rows)
with (s/'full.log').open('a') as log:
 log.write('\nRESUME: interrupted only heavy admission before the first 40000 sample; all four fixture setups completed, no measured call interrupted. Same binaries and fixtures; raw interleaved sample series continues. HEAVY_MAX_LOAD=100 HEAVY_LOAD_WAIT=0; NOT_QUALIFIED.\n');log.flush()
 for rep in range(5):
  for version,binary in [('BASE',s/'base.test'),('HEAD',s/'head-full.test')]:
   env=dict(os.environ,PK_TSTORE_PAIR_DIR=str(s/f'full-40000-{version}'),PK_TSTORE_PAIR_SIZE='40000',PK_TSTORE_PAIR_REP=str(rep),PK_TSTORE_PAIR_VERSION=version,PK_TSTORE_PAIR_OUTPUT=str(out))
   print(f'sample 40000 {rep} {version}',flush=True)
   subprocess.run(['script/heavy.sh','--admit-only',str(binary),'-test.run=^TestATRV07203_PairedHoldSample$','-test.timeout=30m','-test.v'],env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
print('PASS full',flush=True)
