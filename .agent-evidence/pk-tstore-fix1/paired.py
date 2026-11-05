import os, subprocess, pathlib, json, sys, hashlib
s=pathlib.Path('/home/russell/lanes/pk-tstore.fix')
label=sys.argv[1]; head=sys.argv[2]; reps=int(sys.argv[3])
log=(s/(label+'.log')).open('w'); out=s/(label+'.jsonl')
if out.exists():raise SystemExit('refuse overwrite samples')
for n in [13500,40000]:
 for version,binary in [('BASE',s/'base.test'),('HEAD',s/head)]:
  env=dict(os.environ,PK_TSTORE_PAIR_DIR=str(s/f'{label}-{n}-{version}'),PK_TSTORE_PAIR_SIZE=str(n),PK_TSTORE_PAIR_SETUP='1',PK_TSTORE_PAIR_VERSION=version,PK_TSTORE_PAIR_OUTPUT=str(out))
  print(f'setup {n} {version}',flush=True)
  subprocess.run(['script/heavy.sh','--admit-only',str(binary),'-test.run=^TestATRV07203_PairedHoldSample$','-test.timeout=30m','-test.v'],env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
 for rep in range(reps):
  for version,binary in [('BASE',s/'base.test'),('HEAD',s/head)]:
   env=dict(os.environ,PK_TSTORE_PAIR_DIR=str(s/f'{label}-{n}-{version}'),PK_TSTORE_PAIR_SIZE=str(n),PK_TSTORE_PAIR_REP=str(rep),PK_TSTORE_PAIR_VERSION=version,PK_TSTORE_PAIR_OUTPUT=str(out))
   print(f'sample {n} {rep} {version}',flush=True)
   subprocess.run(['script/heavy.sh','--admit-only',str(binary),'-test.run=^TestATRV07203_PairedHoldSample$','-test.timeout=30m','-test.v'],env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
log.close()
print('PASS',label,flush=True)
