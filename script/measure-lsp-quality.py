#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Development-only three-arm LSP baseline on frozen public source witnesses."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time
import sys
from urllib.parse import unquote, urlparse

ACTIVE = None


def kill_group(process, sig):
    try:
        os.killpg(process.pid, sig)
    except ProcessLookupError:
        pass


def retire(process):
    if process.poll() is None:
        kill_group(process, signal.SIGTERM)
    try:
        output = process.communicate(timeout=30)
    except subprocess.TimeoutExpired:
        kill_group(process, signal.SIGKILL)
        output = process.communicate()
    # Catch any same-group descendants after the leader exits.
    kill_group(process, signal.SIGKILL)
    return output


def digest(data):
    return hashlib.sha256(data).hexdigest()


def command(argv, root, env, timeout=90):
    global ACTIVE
    started = time.monotonic()
    try:
        process = subprocess.Popen(argv, cwd=root, env=env, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, start_new_session=True)
    except OSError as exc:
        return {'exit': None, 'seconds': round(time.monotonic()-started, 4), 'bytes': 0,
                'sha256': digest(b''), 'stdout': b'', 'stderrBytes': 0,
                'stderrSha256': digest(b''), 'timedOut': False,
                'startError': type(exc).__name__}
    ACTIVE = process
    try:
        timed_out = False
        try:
            stdout, stderr = process.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
            stdout, stderr = retire(process)
        kill_group(process, signal.SIGKILL)
        return {'exit': process.returncode, 'seconds': round(time.monotonic()-started, 4),
                'bytes': len(stdout), 'sha256': digest(stdout),
                'stdout': stdout, 'stderrBytes': len(stderr), 'stderrSha256': digest(stderr),
                'timedOut': timed_out}
    finally:
        ACTIVE = None


def interrupt(signum, frame):
    if ACTIVE:
        retire(ACTIVE)
    raise KeyboardInterrupt()


def source_path(uri, root):
    parsed = urlparse(uri)
    if parsed.scheme != 'file':
        return None
    try:
        return str(Path(unquote(parsed.path)).relative_to(root))
    except ValueError:
        return None


def percentile_nearest(values, numerator, denominator):
    ordered = sorted(values)
    return ordered[max(0, (len(ordered)*numerator + denominator-1)//denominator-1)]


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--manifest',type=Path,required=True)
    parser.add_argument('--root',type=Path,required=True)
    parser.add_argument('--corvint',type=Path,required=True)
    parser.add_argument('--gopls',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--repeat',type=int,default=3)
    args=parser.parse_args()
    if not 1 <= args.repeat <= 20:
        parser.error('--repeat must be 1..20')
    root=args.root.resolve()
    if not root.is_dir() or not args.corvint.is_file() or not args.gopls.is_file():
        parser.error('root or executable missing')
    if args.output.resolve().is_relative_to(root):
        parser.error('--output must be outside the repository')
    manifest_bytes=args.manifest.read_bytes()
    corpus=json.loads(manifest_bytes)
    if corpus.get('profile')!='corvint-lsp-public-corpus/0' or not corpus.get('cases'):
        parser.error('unsupported corpus')
    def git(*argv):
        return subprocess.run(['git',*argv],cwd=root,check=True,stdout=subprocess.PIPE).stdout.decode().strip()
    revision=git('rev-parse','HEAD')
    if git('status','--porcelain'):
        parser.error('repository must be clean for baseline')
    code_delta=subprocess.run(['git','diff','--quiet',corpus['sourceBase'],revision,'--','*.go','go.mod','go.sum','go.work'],cwd=root)
    if code_delta.returncode:
        parser.error('Go sources differ from pinned corpus sourceBase')
    env=dict(os.environ)
    env.update(GOPROXY='off',GOSUMDB='off',GOTOOLCHAIN='local',
               PATH=str(args.gopls.parent)+os.pathsep+env['PATH'])
    if not env.get('GOCACHE'):
        env['GOCACHE']=subprocess.check_output(['go','env','GOCACHE']).decode().strip()
    with tempfile.TemporaryDirectory(prefix='corvint-lsp-baseline-') as temp:
        env['GOPLSCACHE']=str(Path(temp)/'gopls-cache')
        gopls_version = command([str(args.gopls), 'version'], root, env, timeout=10)
        report={'profile':'corvint-lsp-public-baseline/0','corpusSha256':digest(manifest_bytes),
                'sourceBase':corpus['sourceBase'],'revision':revision,
                'corvintSha256':digest(args.corvint.read_bytes()),'goplsSha256':digest(args.gopls.read_bytes()),
                'go':subprocess.check_output(['go','version'],env=env).decode().strip(),
                'gopls':gopls_version['stdout'].decode(errors='replace').strip() if gopls_version['exit']==0 else None,
                'goplsVersionState':{'exit':gopls_version['exit'],'timedOut':gopls_version['timedOut'],
                                     'startError':gopls_version.get('startError')},
                'cacheTopology':'shared Go build cache; direct gopls uses retained private cache; Corvint provider creates a private cache per call',
                'repeats':args.repeat,'cases':[],'limitations':['Public source witnesses only','Arm timing is descriptive and not cross-arm comparable because gopls cache states differ','Each arm starts a process; editor warm session not measured','No agent task outcome or held-out claims']}
        for case in corpus['cases']:
            arms=['upstream','core','combined']
            samples={arm:[] for arm in arms}
            for repetition in range(args.repeat):
                order=arms[repetition%3:]+arms[:repetition%3]
                packets={}
                for arm in order:
                    if arm=='upstream':
                        argv=[str(args.gopls),'definition','-json',case['position']]
                    else:
                        argv=[str(args.corvint),'--root',str(root),'context','--task',case['task'],
                              '--subject',case['subject'],'--limit','20','--lsp','gopls' if arm=='combined' else 'off']
                    run=command(argv,root,env)
                    sample={k:run[k] for k in ('exit','seconds','bytes','sha256','stderrBytes','stderrSha256','timedOut')}
                    if 'startError' in run:
                        sample['startError']=run['startError']
                    if run['exit']==0:
                        try:
                            packet=json.loads(run['stdout'])
                            if arm=='upstream':
                                got=source_path(packet['span']['uri'],root)
                                sample['definition']=got
                                sample['goldDefinition']=got==case['expectedDefinition']
                            else:
                                sample['expectedPathRank']=next((i for i,x in enumerate(packet.get('results',[]),1)
                                                                 if x.get('id')==case['expectedDefinition']),None)
                                packets[arm]={k:v for k,v in packet.items() if k!='external'}
                                if arm=='combined':
                                    section=packet.get('external',{})
                                    relations=[x.get('relation',{}) for x in section.get('path_relations',[])]
                                    sample['relationCount']=len(relations)
                                    sample['goldRelation']=any(r.get('type')==case['expectedRelation']['type']
                                                               and r.get('from',{}).get('path')==case['expectedRelation']['from']
                                                               and r.get('to',{}).get('path')==case['expectedRelation']['to'] for r in relations)
                                    sample['query']=section.get('query')
                                    sample['providerStates']=[p.get('state') for p in section.get('providers',[])]
                        except (ValueError,KeyError,TypeError) as exc:
                            sample['parseError']=type(exc).__name__
                    samples[arm].append(sample)
                if 'core' in packets and 'combined' in packets:
                    samples['combined'][-1]['corePacketParity']=packets['core']==packets['combined']
                for arm in arms:
                    sample=samples[arm][-1]
                    sample['goldPassed']=(sample['exit']==0 and not sample['timedOut'] and
                                          (sample.get('goldDefinition') is True if arm=='upstream' else
                                           sample.get('expectedPathRank') is not None if arm=='core' else
                                           sample.get('goldRelation') is True and sample.get('corePacketParity') is True))
            summary={}
            for arm in arms:
                times=[s['seconds'] for s in samples[arm] if s['exit']==0]
                summary[arm]={'processSuccesses':len(times),'goldPasses':sum(s['goldPassed'] for s in samples[arm]),
                              'p50Seconds':percentile_nearest(times,1,2) if times else None,
                              'p95Seconds':percentile_nearest(times,95,100) if times else None,
                              'bytesMedian':percentile_nearest([s['bytes'] for s in samples[arm] if s['exit']==0],1,2) if times else None}
            report['cases'].append({'id':case['id'],'expectedDefinition':case['expectedDefinition'],
                                    'expectedRelation':case['expectedRelation'],'samples':samples,'summary':summary})
        args.output.parent.mkdir(parents=True,exist_ok=True)
        args.output.write_text(json.dumps(report,indent=2,sort_keys=True)+'\n')
        execution_succeeded=(gopls_version['exit']==0 and not gopls_version['timedOut'] and
                             all(all(s['exit']==0 and not s['timedOut'] for s in row['samples'][arm])
                                 for row in report['cases'] for arm in ('upstream','core','combined')))
        gold_passed=all(all(s['goldPassed'] for s in row['samples'][arm])
                        for row in report['cases'] for arm in ('upstream','core','combined'))
        print(json.dumps({'output':str(args.output),'revision':revision,'cases':len(report['cases']),
                          'allExecutionSucceeded':execution_succeeded,'allGoldPassed':gold_passed}))
        if not execution_succeeded or not gold_passed:
            sys.exit(1)


if __name__=='__main__':
    signal.signal(signal.SIGINT,interrupt)
    signal.signal(signal.SIGTERM,interrupt)
    main()
