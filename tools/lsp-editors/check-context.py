#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Adversarial validator fixtures using actual committed Git object bytes, not server qualification."""
import copy
import hashlib
import importlib.util
from pathlib import Path
import tempfile

spec = importlib.util.spec_from_file_location('context_probe', Path(__file__).with_name('context-probe.py'))
probe = importlib.util.module_from_spec(spec); spec.loader.exec_module(probe)
with tempfile.TemporaryDirectory(prefix='editor-context-check-') as td:
    root=Path(td).resolve();before=probe.fixture(root);after=probe.snapshot(root);uri=(root/'pkg/main.go').as_uri()
    core={'schema':'corvint-mcp-bridge-result/0','tool':'corvint.context','mutates':False,'state':'READY','epistemicClass':'OBSERVED','authorityClass':'REPOSITORY_EVIDENCE','repository':{'commitRevision':before['commit'],'treeRevision':before['tree'],'objectFormat':'sha1','profileId':'fixture-profile','worktreeState':'CLEAN','dirtyPathCount':0,'dirtyPathsSha256':hashlib.sha256(b'').hexdigest()},'receipt':{'revision':before['tree'],'results':[{'evidence':[{'path':name,'blob_hash':before['files'][name]['blob'],'reason':'fixture governing witness'}]} for name in ['pkg/AGENTS.md','SPEC.md','pkg/main_test.go']]},'abstention':{'active':False,'reason':'none'}}
    result={'schema':'corvint-editor-context/0','core':core,'overlayObservation':{'sessionID':'1'*32,'captureID':1,'uri':uri,'version':2,'digestAlgorithm':'sha256','digest':hashlib.sha256(probe.OVERLAY.encode()).hexdigest()},'inclusionReason':'requested-open-document-subject'}
    observation={'initializeResult':{'capabilities':{'experimental':{'corvintContext':{'method':'corvint/context','schema':'corvint-editor-context/0'}}}},'context':{'result':result,'version':2,'bufferModified':True}}
    def row(direction, message):return {'direction':direction,'message':message}
    rows=[row('client-to-server',{'method':'textDocument/didOpen','params':{'textDocument':{'uri':uri,'version':1,'text':probe.FILES['pkg/main.go']}}}),row('client-to-server',{'method':'textDocument/didChange','params':{'textDocument':{'uri':uri,'version':2},'contentChanges':[{'text':probe.OVERLAY}]}}),row('client-to-server',{'method':'corvint/context','id':5,'params':{'textDocument':{'uri':uri},'task':probe.TASK,'limit':20}}),row('server-to-client',{'id':5,'result':result})]
    assert probe.validate(rows,observation,uri,before,after)['valid']
    assert not probe.validate(rows[:3],observation,uri,before,after)['valid']
    assert not probe.validate([rows[2],rows[3],rows[0],rows[1]],observation,uri,before,after)['valid']
    def changed_result(change):
        bad_rows=copy.deepcopy(rows);bad_obs=copy.deepcopy(observation);change(bad_obs['context']['result']);bad_rows[-1]['message']['result']=bad_obs['context']['result'];return probe.validate(bad_rows,bad_obs,uri,before,after)
    assert not changed_result(lambda r:r['core']['repository'].update(commitRevision='f'*40))['valid']
    assert not changed_result(lambda r:r['core']['receipt']['results'][0]['evidence'][0].update(blob_hash='f'*40))['valid']
    assert not changed_result(lambda r:r['overlayObservation'].update(digest='0'*64))['valid']
    assert not changed_result(lambda r:r['overlayObservation'].update(commitRevision=before['commit']))['valid']
    assert not changed_result(lambda r:r['overlayObservation'].update(version=1))['valid']
    assert not changed_result(lambda r:r['core'].update(state='ABSTAINED',abstention={'active':True,'reason':'unsupported'}))['valid']
    bad=copy.deepcopy(observation);bad['initializeResult']['capabilities']['experimental']['corvintContext']['schema']='invented'
    assert not probe.validate(rows,bad,uri,before,after)['valid']
    (root/'SPEC.md').write_text('tampered')
    assert not probe.validate(rows,observation,uri,before,probe.snapshot(root))['valid']
print('context incomplete/order/forged Git/tampered overlay/marker/disk checks passed; actual corvint/context clients NOT_RUN')
