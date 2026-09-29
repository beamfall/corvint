# SPDX-License-Identifier: AGPL-3.0-or-later
"""Fixture-owned Git evidence validation for one optional actual editor request."""
import hashlib
import os
from pathlib import Path
import re
import subprocess

TASK = 'CTX-V0-001 pkg/main.go pkg/AGENTS.md SPEC.md pkg/main_test.go governing requirements tests'
OVERLAY = 'package p\n/*😀*/ var x int\nvar y = x\n'
FILES = {'go.mod': 'module example.com/editorcontext\n\ngo 1.27.1\n',
         'AGENTS.md': '# Agent contract\nRead SPEC.md before changing pkg/main.go.\n',
         'pkg/AGENTS.md': '# Nested agent contract\nCTX-V0-001 governs this package; read SPEC.md and pkg/main_test.go.\n',
         'SPEC.md': '# Fixture executable specification\n\n## Requirements\n\n- `CTX-V0-001`: pkg/main.go MUST preserve the disk variable; pkg/main_test.go witnesses it.\n',
         'pkg/main.go': 'package p\nvar disk int\n',
         'pkg/main_test.go': 'package p\nimport "testing"\nfunc TestDisk(t *testing.T) { _ = disk }\n'}

def git(root, *args):
    env = {k:v for k,v in os.environ.items() if not k.startswith('GIT_')}
    env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull, GIT_TERMINAL_PROMPT='0', GIT_OPTIONAL_LOCKS='0')
    return subprocess.check_output(['/usr/bin/git', '-C', str(root), '-c', 'core.hooksPath=/dev/null', *args], env=env, timeout=10)

def fixture(root):
    for name, content in FILES.items():
        p = root / name; p.parent.mkdir(parents=True, exist_ok=True); p.write_text(content)
    git(root, 'init', '-q')
    git(root, 'add', '--', *FILES)
    git(root, '-c', 'user.name=Corvint fixture', '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '-q', '-m', 'Fixed offline editor fixture')
    return snapshot(root)

def snapshot(root):
    commit = git(root, 'rev-parse', 'HEAD').decode().strip()
    tracked = git(root, 'ls-tree', '-rz', '--name-only', commit).decode().split('\0')
    tracked = [name for name in tracked if name]
    return {'commit': commit, 'tree': git(root, 'rev-parse', 'HEAD^{tree}').decode().strip(),
            'status': git(root, 'status', '--porcelain=v1', '--untracked-files=all').decode(),
            'files': {name:{'blob':git(root, 'rev-parse', commit+':'+name).decode().strip(),
                           'contentSha256':hashlib.sha256(git(root, 'show', commit+':'+name)).hexdigest(),
                           'content':git(root, 'show', commit+':'+name).decode(),
                           'diskSha256':hashlib.sha256((root/name).read_bytes()).hexdigest()} for name in tracked}}

def validate(rows, observation, uri, before, after):
    errors = []
    marker = observation.get('initializeResult', {}).get('capabilities', {}).get('experimental', {}).get('corvintContext')
    if marker != {'method':'corvint/context','schema':'corvint-editor-context/0'}: errors.append('context-marker-invalid')
    def events(direction, method):
        return [(i,r['message']) for i,r in enumerate(rows) if r.get('direction')==direction and isinstance(r.get('message'),dict) and r['message'].get('method')==method]
    opened = [(i,m) for i,m in events('client-to-server','textDocument/didOpen') if m.get('params',{}).get('textDocument',{}).get('uri')==uri]
    changed = [(i,m) for i,m in events('client-to-server','textDocument/didChange') if m.get('params',{}).get('textDocument',{}).get('uri')==uri]
    fixture_changes = [(i,m) for i,m in changed if m.get('params',{}).get('contentChanges')==[{'text':OVERLAY}]]
    queries = [(i,m) for i,m in events('client-to-server','corvint/context') if m.get('params')=={'textDocument':{'uri':uri},'task':TASK,'limit':20}]
    result = observation.get('context',{}).get('result')
    if len(opened)!=1 or len(fixture_changes)!=1 or len(queries)!=1:
        errors.append('open-change-query-missing-or-duplicate')
    else:
        oi,om=opened[0];ci,cm=fixture_changes[0];qi,qm=queries[0]
        initial=om['params']['textDocument']
        if set(initial) not in ({'uri','version','text'},{'uri','version','text','languageId'}) or ('languageId' in initial and initial['languageId']!='go') or type(initial.get('version')) is not int or initial['version']<0 or initial.get('text')!=FILES['pkg/main.go']:errors.append('context-initial-open-invalid')
        replies=[(i,r['message']) for i,r in enumerate(rows) if r.get('direction')=='server-to-client' and isinstance(r.get('message'),dict) and 'method' not in r['message'] and r['message'].get('id')==qm.get('id')]
        if len(replies)!=1 or not oi<ci<qi<replies[0][0] or replies[0][1].get('result')!=result:errors.append('context-wire-order-or-result-mismatch')
        elif any(qi<i<replies[0][0] for i,_ in changed): errors.append('context-changed-before-result')
        version=cm['params']['textDocument'].get('version')
        if type(version) is not int or type(om['params']['textDocument'].get('version')) is not int or version<=om['params']['textDocument'].get('version',version) or observation.get('context',{}).get('version')!=version:errors.append('context-snapshot-version-mismatch')
        if not changed or changed[-1][0]!=ci:errors.append('context-not-latest-fixture')
    closed = [i for i,m in events('client-to-server','textDocument/didClose') if m.get('params',{}).get('textDocument',{}).get('uri')==uri]
    if queries:
        query_id=queries[0][1].get('id')
        response_indexes=[i for i,r in enumerate(rows) if r.get('direction')=='server-to-client' and isinstance(r.get('message'),dict) and 'method' not in r['message'] and r['message'].get('id')==query_id]
        if opened and response_indexes and any(opened[0][0]<i<response_indexes[0] for i in closed):errors.append('context-source-closed-before-result')
    if not observation.get('context',{}).get('bufferModified'):errors.append('context-buffer-not-unsaved')
    if not isinstance(result,dict):return {'valid':False,'errors':errors+['context-result-missing']}
    if set(result)!={'schema','core','overlayObservation','inclusionReason'} or result.get('schema')!='corvint-editor-context/0' or result.get('inclusionReason')!='requested-open-document-subject':errors.append('context-envelope-invalid')
    overlay=result.get('overlayObservation',{})
    if set(overlay)!={'sessionID','captureID','uri','version','digestAlgorithm','digest'}:errors.append('overlay-closed-shape-invalid')
    if not isinstance(overlay.get('sessionID'),str) or not re.fullmatch('[0-9a-f]{32}',overlay.get('sessionID','')) or not isinstance(overlay.get('captureID'),str) or not re.fullmatch('[1-9][0-9]{0,19}',overlay.get('captureID','')) or int(overlay['captureID'])>18446744073709551615:errors.append('overlay-session-capture-invalid')
    if type(overlay.get('version')) is not int or overlay.get('uri')!=uri or not fixture_changes or overlay.get('version')!=fixture_changes[0][1]['params']['textDocument'].get('version') or overlay.get('digestAlgorithm')!='sha256' or overlay.get('digest')!=hashlib.sha256(OVERLAY.encode()).hexdigest():errors.append('overlay-binding-invalid')
    core=result.get('core',{})
    required={'schema','tool','mutates','state','epistemicClass','authorityClass','repository','receipt','abstention'}
    if set(core)!=required or core.get('schema')!='corvint-mcp-bridge-result/0' or core.get('tool')!='corvint.context' or core.get('mutates') is not False:errors.append('core-bridge-envelope-invalid')
    if core.get('state') == 'READY' and (core.get('epistemicClass')!='OBSERVED' or core.get('authorityClass')!='REPOSITORY_EVIDENCE'): errors.append('core-authority-class-invalid')
    if core.get('state') not in ['READY','ABSTAINED']:errors.append('core-state-invalid')
    if core.get('abstention')!={'active':False,'reason':'NONE'} or core.get('abstention',{}).get('active') is not False:errors.append('core-abstention-shape-invalid')
    if core.get('state')=='ABSTAINED' or core.get('abstention',{}).get('active') is not False:errors.append('core-abstained-not-fixture-proof')
    binding=core.get('repository') or {}
    if binding.get('commitRevision')!=before['commit'] or binding.get('treeRevision')!=before['tree'] or binding.get('objectFormat')!='sha1':errors.append('core-git-binding-invalid')
    receipt=core.get('receipt') or {}
    if set(binding)!={'commitRevision','treeRevision','objectFormat','profileId','worktreeState','dirtyPathCount','dirtyPathsSha256'}:errors.append('core-repository-shape-invalid')
    if type(binding.get('dirtyPathCount')) is not int or binding.get('dirtyPathCount') != 0 or binding.get('worktreeState') != 'CLEAN' or binding.get('dirtyPathsSha256') != hashlib.sha256(b'').hexdigest(): errors.append('unexpected-core-dirty-state')
    if binding.get('profileId')!='generic':errors.append('fixture-core-profile-invalid')
    if receipt.get('tool')!='context':errors.append('core-receipt-tool-invalid')
    if receipt.get('revision')!=before['tree']:errors.append('core-receipt-revision-invalid')
    evidence=[]
    for row in receipt.get('results',[]): evidence.extend(row.get('evidence',[]))
    for name in ['pkg/AGENTS.md','SPEC.md','pkg/main_test.go']:
        matches=[e for e in evidence if e.get('path')==name and e.get('blob_hash')==before['files'][name]['blob'] and e.get('reason')]
        if not matches:errors.append('missing-or-forged-governing-evidence:'+name)
    for name, content in FILES.items():
        fact=before['files'][name]
        data=content.encode()
        expected=hashlib.sha1(b'blob '+str(len(data)).encode()+b'\0'+data).hexdigest()
        if fact['blob']!=expected or fact['contentSha256']!=hashlib.sha256(data).hexdigest():errors.append('fixture-object-content-invalid:'+name)
    def facts(value):
        if isinstance(value, dict):
            if 'path' in value: yield value
            for nested in value.values(): yield from facts(nested)
        elif isinstance(value, list):
            for nested in value: yield from facts(nested)
    for e in evidence:
        name=e.get('path')
        if name not in before['files']:errors.append('untracked-claimed-path:'+str(name))
        elif not e.get('blob_hash') or e['blob_hash']!=before['files'][name]['blob']:errors.append('forged-or-unbound-fixture-blob:'+name)
    for fact in facts(receipt):
        name=fact.get('path')
        if name not in before['files'] or fact.get('blob_hash')!=before['files'][name]['blob']:
            errors.append('unverified-packet-path-blob:'+str(name));continue
        content=before['files'][name]['content'];lines=content.splitlines(keepends=True)
        if 'line' in fact and (type(fact['line']) is not int or not 1<=fact['line']<=len(lines)):errors.append('packet-line-outside-blob:'+name)
        has_span='start_line' in fact or 'end_line' in fact
        if has_span:
            start,end=fact.get('start_line'),fact.get('end_line')
            if type(start) is not int or type(end) is not int or not 1<=start<=end<=len(lines):
                errors.append('packet-span-outside-blob:'+name);continue
            content=''.join(lines[start-1:end])
        if 'content' in fact and fact['content']!=content:errors.append('packet-content-not-git-bytes:'+name)
    if before!=after or after['status']:errors.append('fixture-disk-or-repository-changed')
    return {'valid':not errors,'errors':errors,'coreState':core.get('state'),'coreWorktreeState':binding.get('worktreeState'),'overlaySeparateFromGit':not any(e.startswith('overlay-') for e in errors),'governingPaths':['pkg/AGENTS.md','SPEC.md','pkg/main_test.go'],'qualification':'UNQUALIFIED'}
