#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
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

# The keeper-pinned group owner shared by the qualification scripts (V1-1387).
sys.path.insert(0, str(Path(__file__).resolve().parent))
import process_group  # noqa: E402

ACTIVE = None


def retire(owned):
    """SIGTERM to a running group, 30 s grace, then the bounded SIGKILL
    retirement, which also catches same-group descendants of an exited
    command. Every signal precedes the keeper reap (V1-1387)."""
    if owned.process.poll() is None:
        owned.signal(signal.SIGTERM)
        try:
            owned.communicate(timeout=30)
        except subprocess.TimeoutExpired:
            pass
    return owned.retire()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def command(argv, root, env, timeout=90):
    global ACTIVE
    started = time.monotonic()
    try:
        owned = process_group.OwnedGroup(argv, cwd=root, env=env, stdout=subprocess.PIPE,
                                         stderr=subprocess.PIPE)
    except OSError as exc:
        return {'exit': None, 'seconds': round(time.monotonic()-started, 4), 'bytes': 0,
                'sha256': digest(b''), 'stdout': b'', 'stderrBytes': 0,
                'stderrSha256': digest(b''), 'timedOut': False,
                'startError': type(exc).__name__}
    ACTIVE = owned
    try:
        timed_out = False
        try:
            owned.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
        stdout, stderr = retire(owned)
        return {'exit': owned.process.returncode, 'seconds': round(time.monotonic()-started, 4),
                'bytes': len(stdout), 'sha256': digest(stdout),
                'stdout': stdout, 'stderrBytes': len(stderr), 'stderrSha256': digest(stderr),
                'timedOut': timed_out}
    finally:
        ACTIVE = None


def interrupt(signum, frame):
    if ACTIVE and not ACTIVE.retired:
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


def relation_gold(section, case):
    """LQP-V0-022: classify the combined expected relation. Only `present` can
    pass; a bound that may have dropped it is stated uncertainty, and a
    projection that breaks TCP-V0-054 subject-first order is a defect."""
    expected, subject = case['expectedRelation'], case['subject']
    rows = [x.get('relation', {}) for x in section.get('path_relations', [])]
    # Like TCP-V0-054, only endpoints in the bound root repository count.
    roots = [p.get('root_repository') for p in section.get('providers', []) if p.get('state') == 'loaded']
    root = roots[0] if len(roots) == 1 and roots[0] else None
    def path(row, side):
        endpoint = row.get(side, {})
        return endpoint.get('path') if root is not None and endpoint.get('repository') == root else None
    def touches(row):
        return path(row, 'from') == subject or path(row, 'to') == subject
    # The order check comes first: a misordered packet is a defect even when
    # it happens to carry the gold relation.
    flags = [touches(r) for r in rows]
    if any(later and not earlier for earlier, later in zip(flags, flags[1:])):
        return 'projection-not-subject-first'
    if any(r.get('type') == expected['type'] and path(r, 'from') == expected['from']
           and path(r, 'to') == expected['to'] for r in rows):
        return 'present'
    omitted = section.get('omitted', {}).get('path_relations', 0)
    gold_touches = subject in (expected['from'], expected['to'])
    # Subject-first order kept every subject row when any non-subject row
    # survived, so then only a non-subject gold can have been cut here.
    if omitted and (not gold_touches or all(flags)):
        return 'omitted-by-packet-bound'
    query = section.get('query') or {}
    if query.get('omitted_rows') or query.get('stopped'):
        return 'omitted-by-provider-bound'
    return 'absent'


def combined_gold_passed(sample):
    return (sample['exit']==0 and not sample['timedOut'] and
            sample.get('goldRelation') is True and sample.get('corePacketParity') is True)


def relation_states(samples):
    states={}
    for sample in samples:
        state=sample.get('goldRelationState','not-scored')
        states[state]=states.get(state,0)+1
    return dict(sorted(states.items()))


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
    # V1-2536: a lexically ordered projection (pre-TCP-V0-054 binary) cut the
    # gold subject definition at --limit 20; it must fail as a defect, and a
    # legitimate bound miss must stay uncertainty, never a pass.
    rel_case = {'subject': 's.go', 'expectedRelation': {'type': 'gopls:uses-definition', 'from': 's.go', 'to': 'd.go'}}
    def row(source, target, kind='gopls:referenced-by', repository='root'):
        return {'relation': {'type': kind, 'from': {'repository': repository, 'path': source},
                             'to': {'repository': repository, 'path': target}}}
    gold = row('s.go', 'd.go', 'gopls:uses-definition')
    loaded = [{'state': 'loaded', 'root_repository': 'root'}]
    lexical = {'providers': loaded, 'path_relations': [row('a.go', 'b.go'), row('s.go', 'c.go')],
               'omitted': {'path_relations': 1}, 'query': {'omitted_rows': 64, 'stopped': 'query-budget'}}
    assert relation_gold(lexical, rel_case) == 'projection-not-subject-first'
    assert relation_gold({**lexical, 'path_relations': [gold, row('a.go', 'b.go')]}, rel_case) == 'present'
    # Gold after a non-subject row still breaks subject-first order and never passes.
    misordered_gold = {**lexical, 'path_relations': [row('a.go', 'b.go'), gold]}
    assert relation_gold(misordered_gold, rel_case) == 'projection-not-subject-first'
    subject_full = {**lexical, 'path_relations': [row('s.go', 'c.go'), row('x.go', 's.go')]}
    assert relation_gold(subject_full, rel_case) == 'omitted-by-packet-bound'
    kept_all = {**lexical, 'path_relations': [row('s.go', 'c.go'), row('a.go', 'b.go')]}
    assert relation_gold(kept_all, rel_case) == 'omitted-by-provider-bound'
    assert relation_gold({'providers': loaded, 'path_relations': kept_all['path_relations'], 'omitted': {'path_relations': 1},
                          'query': {'omitted_rows': 0, 'stopped': ''}}, rel_case) == 'absent'
    # A same-path endpoint in another repository neither matches gold nor gets subject priority.
    foreign_gold = {**kept_all, 'path_relations': [row('s.go', 'c.go'), row('a.go', 'b.go'),
                                                   row('s.go', 'd.go', 'gopls:uses-definition', 'other')]}
    assert relation_gold(foreign_gold, rel_case) == 'omitted-by-provider-bound'
    assert relation_gold({**lexical, 'providers': [], 'path_relations': [gold]}, rel_case) != 'present'
    for section in (lexical, misordered_gold, subject_full, kept_all):
        assert not combined_gold_passed({'exit': 0, 'timedOut': False, 'corePacketParity': True,
                                         'goldRelation': relation_gold(section, rel_case) == 'present'})
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
                                    sample['goldRelationState']=relation_gold(section, case)
                                    sample['goldRelation']=sample['goldRelationState']=='present'
                                    sample['pathRelationsOmitted']=section.get('omitted',{}).get('path_relations')
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
                                           combined_gold_passed(sample)))
            summary={}
            for arm in arms:
                times=[s['seconds'] for s in samples[arm] if s['exit']==0]
                summary[arm]={'processSuccesses':len(times),'goldPasses':sum(s['goldPassed'] for s in samples[arm]),
                              **({'goldRelationStates':relation_states(samples[arm])} if arm=='combined' else {}),
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
        states=relation_states([s for row in report['cases'] for s in row['samples']['combined']])
        print(json.dumps({'output':str(args.output),'revision':revision,'cases':len(report['cases']),
                          'allExecutionSucceeded':execution_succeeded,'allGoldPassed':gold_passed,
                          'combinedRelationStates':states,
                          'combinedRelationUncertain':sum(v for k,v in states.items() if k.startswith('omitted-by-'))}))
        if not execution_succeeded or not gold_passed:
            sys.exit(1)


if __name__=='__main__':
    signal.signal(signal.SIGINT,interrupt)
    signal.signal(signal.SIGTERM,interrupt)
    main()
