#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Development-only three-arm LSP baseline on frozen public source witnesses."""
import argparse
import hashlib
import json
import math
import re
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


def strict_json(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError('duplicate JSON key')
            result[key] = value
        return result
    def finite(raw_number):
        value = float(raw_number)
        if not math.isfinite(value):
            raise ValueError('nonfinite JSON number')
        return value
    def constant(value):
        raise ValueError('nonstandard JSON constant')
    return json.loads(raw, object_pairs_hook=pairs, parse_float=finite, parse_constant=constant)


def source_path(uri, root):
    # Reject malformed spelling before urlparse can discard literal controls.
    if type(uri) is not str or any(c.isspace() or ord(c) < 32 or ord(c) == 127 for c in uri):
        return None
    if re.search(r'%(?![0-9A-Fa-f]{2})', uri):
        return None
    parsed = urlparse(uri)
    if parsed.scheme != 'file' or parsed.netloc or parsed.query or parsed.fragment:
        return None
    try:
        decoded = unquote(parsed.path, errors='strict')
        if any(ord(c) < 32 or ord(c) == 127 for c in decoded):
            return None
        path = Path(decoded)
        if uri != path.as_uri():
            return None
        return str(path.relative_to(root))
    except (ValueError, UnicodeError):
        return None


def percentile_nearest(values, numerator, denominator):
    ordered = sorted(values)
    return ordered[max(0, (len(ordered)*numerator + denominator-1)//denominator-1)]


def integral(value):
    return type(value) is int and value >= 0


def point_offset(data, point):
    if type(point) is not dict or set(point) != {'line', 'column', 'offset'}:
        raise ValueError('invalid point schema')
    if not all(integral(v) for v in point.values()) or point['line'] < 1 or point['column'] < 1:
        raise ValueError('invalid point types')
    lines = data.splitlines(keepends=True)
    line, column = point['line'], point['column']
    if line > len(lines) or column > len(lines[line-1]) + 1:
        raise ValueError('point outside source')
    offset = sum(len(x) for x in lines[:line-1]) + column - 1
    if offset != point['offset'] or offset > len(data):
        raise ValueError('point offset disagreement')
    return offset


def identifier_span(data, span, identifier):
    if type(identifier) is not str or not identifier.isascii() or not identifier.isidentifier():
        raise ValueError('invalid identifier')
    if type(span) is not dict or set(span) != {'start', 'end'}:
        raise ValueError('invalid span schema')
    start, end = point_offset(data, span['start']), point_offset(data, span['end'])
    if data[start:end] != identifier.encode() or end <= start:
        raise ValueError('source identifier mismatch')
    word = lambda b: chr(b).isascii() and (chr(b).isalnum() or b == 95)
    if (start and word(data[start-1])) or (end < len(data) and word(data[end])):
        raise ValueError('identifier is not a complete token')


def validate_case(case, read_blob):
    # LQP-V0-015/016: gold is frozen source evidence, never inferred from candidate output.
    query, target = case['queryGold'], case['definitionGold']
    for path in (case['subject'], case['expectedDefinition']):
        if type(path) is not str or not path or Path(path).is_absolute() or '..' in Path(path).parts or ':' in path:
            raise ValueError('invalid repository path')
    if query['identifier'] != target['identifier']:
        raise ValueError('query/target identifier mismatch')
    if set(query) != {'identifier', 'start', 'end'} or set(target) != {'identifier', 'start', 'end'}:
        raise ValueError('exact gold required')
    path, line, column = case['position'].rsplit(':', 2)
    if path != case['subject'] or int(line) != query['start']['line'] or not query['start']['column'] <= int(column) < query['end']['column']:
        raise ValueError('query position mismatch')
    identifier_span(read_blob(path), {k: query[k] for k in ('start', 'end')}, query['identifier'])
    identifier_span(read_blob(case['expectedDefinition']), {k: target[k] for k in ('start', 'end')}, target['identifier'])


def score_definition(packet, case, root, target_bytes):
    try:
        span = packet['span']
        if type(span) is not dict or set(span) != {'uri', 'start', 'end'} or type(span['uri']) is not str:
            return False
        if source_path(span['uri'], root) != case['expectedDefinition']:
            return False
        actual = {k: span[k] for k in ('start', 'end')}
        identifier_span(target_bytes, actual, case['definitionGold']['identifier'])
        return actual == {k: case['definitionGold'][k] for k in ('start', 'end')}
    except (KeyError, TypeError, ValueError, AttributeError):
        return False


def self_check():
    # LQP-V0-015/016: nearby same-file symbol and coordinate/type drift must not score.
    import copy
    data = b'func Expand() {}\ntype Request struct {}\n'
    case = {'expectedDefinition': 'p.go', 'definitionGold': {'identifier': 'Expand',
            'start': {'line': 1, 'column': 6, 'offset': 5}, 'end': {'line': 1, 'column': 12, 'offset': 11}}}
    root = Path('/fixture')
    packet = {'span': {'uri': 'file:///fixture/p.go', **{k: case['definitionGold'][k] for k in ('start', 'end')}}}
    assert score_definition(packet, case, root, data)
    bad = copy.deepcopy(packet); bad['span'].update(start={'line': 2, 'column': 6, 'offset': 22}, end={'line': 2, 'column': 13, 'offset': 29})
    assert source_path(bad['span']['uri'], root) == case['expectedDefinition']  # old oracle false pass
    assert not score_definition(bad, case, root, data)
    for key in ('line', 'column', 'offset'):
        for value in (True, 5.0, '5', -1, None):
            bad = copy.deepcopy(packet); bad['span']['start'][key] = value
            assert not score_definition(bad, case, root, data)
    for uri in ('file:///fixture/other.go', 'file://foreign/fixture/p.go', 'file:///fixture/p.go?x'):
        bad = copy.deepcopy(packet); bad['span']['uri'] = uri
        assert not score_definition(bad, case, root, data)
    bad = copy.deepcopy(packet); bad['span']['end']['offset'] += 1
    assert not score_definition(bad, case, root, data)
    assert not score_definition(packet, case, root, data.replace(b'Expand', b'Otherx'))
    try:
        validate_case({'position': 'p.go:1:6', 'subject': 'p.go', **case}, lambda _: data)
        raise AssertionError('path-only manifest accepted')
    except KeyError:
        pass
    valid = {'position': 'p.go:1:6', 'subject': 'p.go', **case, 'queryGold': copy.deepcopy(case['definitionGold'])}
    validate_case(valid, lambda _: data)
    valid['position'] = 'p.go:1:12'
    try:
        validate_case(valid, lambda _: data)
        raise AssertionError('query coordinate drift accepted')
    except ValueError:
        pass
    corpus_root = Path(__file__).resolve().parents[1]
    corpus = strict_json((corpus_root / 'benchmarks/lsp-quality/public-v0.json').read_bytes())
    for actual_case in corpus['cases']:
        blob = subprocess.check_output(['git', 'show', corpus['sourceBase']+':'+actual_case['expectedDefinition']], cwd=corpus_root)
        pkt = {'span': {'uri': 'file:///fixture/'+actual_case['expectedDefinition'],
                       **{k: actual_case['definitionGold'][k] for k in ('start', 'end')}}}
        raw = json.dumps(pkt)
        assert score_definition(strict_json(raw), actual_case, root, blob)
        duplicate = raw.replace('"uri":', '"uri":"file:///wrong.go","uri":', 1)
        for malformed in (duplicate, '{"x":NaN}', '{"x":Infinity}', '{"x":-Infinity}', '{"x":1e999}'):
            try:
                strict_json(malformed)
                raise AssertionError('malformed raw JSON accepted')
            except ValueError:
                pass
        for suffix in ('\n', '\t', '\r', ' ', '%0A', '%09', '%', '%GG', '%FF'):
            bad = copy.deepcopy(pkt)
            bad['span']['uri'] = bad['span']['uri'].replace('/internal/', '/internal/'+suffix)
            assert not score_definition(strict_json(json.dumps(bad)), actual_case, root, blob)
    assert source_path('file:///fixture/a%20b.go', root) == 'a b.go'
    assert source_path('file:///fixture/p%2Ego', root) is None  # noncanonical alias
    print('exact-symbol gold self-check PASS; qualification UNQUALIFIED')


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--self-check', action='store_true')
    parser.add_argument('--manifest',type=Path)
    parser.add_argument('--root',type=Path)
    parser.add_argument('--corvint',type=Path)
    parser.add_argument('--gopls',type=Path)
    parser.add_argument('--output',type=Path)
    parser.add_argument('--repeat',type=int,default=3)
    args=parser.parse_args()
    if args.self_check:
        self_check(); return
    if not all((args.manifest, args.root, args.corvint, args.gopls, args.output)):
        parser.error('manifest, root, corvint, gopls and output required')
    if not 1 <= args.repeat <= 20:
        parser.error('--repeat must be 1..20')
    root=args.root.resolve()
    if not root.is_dir() or not args.corvint.is_file() or not args.gopls.is_file():
        parser.error('root or executable missing')
    if args.output.resolve().is_relative_to(root):
        parser.error('--output must be outside the repository')
    manifest_bytes=args.manifest.read_bytes()
    try:
        corpus=strict_json(manifest_bytes)
    except (ValueError, UnicodeError) as exc:
        parser.error('invalid corpus JSON: '+str(exc))
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
    def read_blob(path):
        return subprocess.run(['git', 'show', corpus['sourceBase']+':'+path], cwd=root, check=True, stdout=subprocess.PIPE).stdout
    try:
        for case in corpus['cases']:
            validate_case(case, read_blob)
    except (KeyError, TypeError, ValueError, subprocess.CalledProcessError) as exc:
        parser.error('invalid exact-symbol corpus: '+str(exc))
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
                'goldScoring':'exact-symbol-span/1; combined relations remain path-level',
                'corvintSha256':digest(args.corvint.read_bytes()),'goplsSha256':digest(args.gopls.read_bytes()),
                'go':subprocess.check_output(['go','version'],env=env).decode().strip(),
                'gopls':gopls_version['stdout'].decode(errors='replace').strip() if gopls_version['exit']==0 else None,
                'goplsVersionState':{'exit':gopls_version['exit'],'timedOut':gopls_version['timedOut'],
                                     'startError':gopls_version.get('startError')},
                'cacheTopology':'shared Go build cache; direct gopls uses retained private cache; Corvint provider creates a private cache per call',
                'repeats':args.repeat,'cases':[],'limitations':['Public source witnesses only','Arm timing is descriptive and not cross-arm comparable because gopls cache states differ','Each arm starts a process; editor warm session not measured','No agent task outcome or held-out claims','Exact-symbol upstream gold; combined relation gold remains path-level']}
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
                            packet=strict_json(run['stdout'])
                            if arm=='upstream':
                                got=source_path(packet['span']['uri'],root)
                                sample['definition']=got
                                sample['goldDefinition']=score_definition(packet, case, root, read_blob(case['expectedDefinition']))
                                sample['definitionSpan']=packet.get('span')
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
                                    'expectedRelation':case['expectedRelation'],'queryGold':case['queryGold'],
                                    'definitionGold':case['definitionGold'],'samples':samples,'summary':summary})
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
