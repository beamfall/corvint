import json,statistics,pathlib,sys
s=pathlib.Path('/home/russell/lanes/pk-tstore.fix')
summary={}
for label in sys.argv[1:]:
 rows=[json.loads(line) for line in (s/(label+'.jsonl')).read_text().splitlines()]
 assert len(rows)==40,(label,len(rows))
 expect=[(n,rep,v,verb) for n in [13500,40000] for rep in range({'full':0,'narrow':5,'final':10}[label],{'full':0,'narrow':5,'final':10}[label]+5) for v in ['BASE','HEAD'] for verb in ['claim','complete']]
 assert [(r['Receipts'],r['Repetition'],r['Version'],r['Verb']) for r in rows]==expect
 loads=[float(r[k].split()[0]) for r in rows for k in ['LoadBefore','LoadAfter']]
 groups=[]
 for n in [13500,40000]:
  for verb in ['claim','complete']:
   data={v:[r['HoldMS'] for r in rows if r['Receipts']==n and r['Verb']==verb and r['Version']==v] for v in ['BASE','HEAD']}
   med={v:statistics.median(ds) for v,ds in data.items()}
   g=dict(receipts=n,verb=verb,baseMedianMS=med['BASE'],headMedianMS=med['HEAD'],ratio=med['HEAD']/med['BASE'],medianPairedRatio=statistics.median([b/a for a,b in zip(data['BASE'],data['HEAD'])]),baseSamplesMS=data['BASE'],headSamplesMS=data['HEAD'])
   groups.append(g)
 result=dict(status='NOT_QUALIFIED' if max(loads)>16 else 'QUALIFIED',loadOneMinuteRange=[min(loads),max(loads)],samples=len(rows),groups=groups)
 summary[label]=result
 print(label,result['status'],'loads',result['loadOneMinuteRange'])
 for g in groups:print(g['receipts'],g['verb'],'BASE',round(g['baseMedianMS'],3),'HEAD',round(g['headMedianMS'],3),'ratio',round(g['ratio'],3))
(s/'paired-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
