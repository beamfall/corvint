# SPDX-License-Identifier: AGPL-3.0-or-later
"""LEQ-V0-006: synthetic oracle checks, never actual editor evidence."""
import copy
import hashlib
import importlib.util
from pathlib import Path
import tempfile

spec=importlib.util.spec_from_file_location('freshness',Path(__file__).with_name('freshness-probe.py'))
f=importlib.util.module_from_spec(spec);spec.loader.exec_module(f)
c=f.context
with tempfile.TemporaryDirectory() as directory:
    before=c.fixture(Path(directory));uri=(Path(directory)/'pkg/main.go').as_uri();after=c.snapshot(Path(directory))
    core={'schema':'corvint-mcp-bridge-result/0','tool':'corvint.context','mutates':False,'state':'READY','epistemicClass':'OBSERVED','authorityClass':'REPOSITORY_EVIDENCE','repository':{'commitRevision':before['commit'],'treeRevision':before['tree'],'objectFormat':'sha1','profileId':'generic','worktreeState':'CLEAN','dirtyPathCount':0,'dirtyPathsSha256':hashlib.sha256(b'[]').hexdigest()},'receipt':{'tool':'context','revision':before['tree'],'results':[{'evidence':[{'path':name,'blob_hash':before['files'][name]['blob'],'line':1,'reason':'fixture governing witness'}]} for name in ['pkg/AGENTS.md','SPEC.md','pkg/main_test.go']]},'abstention':{'active':False,'reason':'NONE'}}
    def packet(text,version,capture):return {'schema':'corvint-editor-context/0','core':copy.deepcopy(core),'overlayObservation':{'sessionID':'1'*32,'captureID':capture,'uri':uri,'version':version,'digestAlgorithm':'sha256','digest':hashlib.sha256(text.encode()).hexdigest()},'inclusionReason':'requested-open-document-subject'}
    def row(direction,**m):return {'direction':direction,'message':dict(jsonrpc='2.0',**m)}
    def edit(text,version):return row('client-to-server',method='textDocument/didChange',params={'textDocument':{'uri':uri,'version':version},'contentChanges':[{'text':text}]})
    def query(i):return row('client-to-server',id=i,method='corvint/context',params={'textDocument':{'uri':uri},'task':c.TASK,'limit':20})
    a,b=packet(c.OVERLAY,2,'1'),packet(f.NEWEST,3,'2');cancel={'code':-32800,'message':'Request cancelled','data':{'reason':'CANCELLED'}}
    rows=[row('client-to-server',method='textDocument/didOpen',params={'textDocument':{'uri':uri,'version':1,'text':c.FILES['pkg/main.go']}}),edit(c.OVERLAY,2),query(5),row('server-to-client',id=5,result=a),query(6),edit(f.NEWEST,3),row('server-to-client',id=6,error=cancel),query(7),row('server-to-client',id=7,result=b)]
    obs={'initializeResult':{'capabilities':{'experimental':{'corvintContext':{'method':'corvint/context','schema':'corvint-editor-context/0'}}}},'freshness':{'baseline':{'result':a},'rapid':{'error':cancel},'retry':{'result':b},'pendingBeforeEdit':True,'bufferModified':True,'versions':[2,3]}}
    def run(r=rows,o=obs,af=after):return f.validate(r,o,uri,before,af)
    assert run()['valid'] and run()['outcome']=='EDIT_CANCELLED_WITNESSED'
    # Old single-request wrapper must not pretend this new transcript is its witness.
    old={'initializeResult':obs['initializeResult'],'context':{'result':a,'version':2,'bufferModified':True}}
    assert not c.validate(rows,old,uri,before,after)['valid']
    count=0
    def negative(change):
        global count
        r,o=copy.deepcopy(rows),copy.deepcopy(obs);change(r,o)
        result=run(r,o);assert not result['valid'],(count,result);count+=1
    negative(lambda r,o:r.pop())
    negative(lambda r,o:r.append(copy.deepcopy(r[6])))
    negative(lambda r,o:r[4]['message'].update(id=True))
    negative(lambda r,o:r[4]['message'].update(id=None))
    negative(lambda r,o:r[4]['message'].update(id=5))
    negative(lambda r,o:r[6]['message'].update(result=a))
    negative(lambda r,o:r[5]['message']['params']['contentChanges'].append({'text':f.NEWEST}))
    negative(lambda r,o:r[5]['message']['params']['textDocument'].update(uri='file:///other.go'))
    negative(lambda r,o:r[5]['message']['params']['textDocument'].update(version=True))
    negative(lambda r,o:o['freshness'].update(pendingBeforeEdit=1))
    negative(lambda r,o:o['freshness'].pop('pendingBeforeEdit'))
    negative(lambda r,o:r.insert(5,row('client-to-server',method='$/cancelRequest',params={'id':6})))
    negative(lambda r,o:r.insert(5,row('client-to-server',method='textDocument/didOpen',params={'textDocument':{'uri':'file:///other.go','version':1,'text':'x'}})))
    negative(lambda r,o:r.insert(6,row('client-to-server',method='textDocument/didClose',params={'textDocument':{'uri':uri}})))
    negative(lambda r,o:r.insert(6,query(8)))
    negative(lambda r,o:r.insert(6,row('client-to-server',id=9,method='textDocument/definition',params={})))
    negative(lambda r,o:r.insert(6,row('client-to-server',id=9,method='shutdown')))
    negative(lambda r,o:r.insert(6,row('client-to-server',method='exit')))
    negative(lambda r,o:r.insert(5,r.pop(7)))
    def result_change(change):
        def mutate(r,o):change(r[8]['message']['result']);o['freshness']['retry']={'result':r[8]['message']['result']}
        negative(mutate)
    result_change(lambda p:p['overlayObservation'].update(captureID='1'))
    result_change(lambda p:p['overlayObservation'].update(captureID=2))
    result_change(lambda p:p['overlayObservation'].update(captureID='18446744073709551616'))
    result_change(lambda p:p['overlayObservation'].update(sessionID='2'*32))
    result_change(lambda p:p['overlayObservation'].update(digest='0'*64))
    result_change(lambda p:p['core']['receipt']['results'][0]['evidence'][0].update(blob_hash='f'*40))
    result_change(lambda p:p['core']['receipt']['results'][0]['evidence'][0].update(line=10000))
    result_change(lambda p:p['core']['receipt'].update(coverage={'critical':'governing'}))
    def wrong_error(r,o):r[6]['message']['error']['data']['reason']='DEADLINE';o['freshness']['rapid']={'error':r[6]['message']['error']}
    negative(wrong_error)
    assert not run(af=dict(after,status=' M pkg/main.go'))['valid']
    # Complete races are distinct from malformed evidence and never successful.
    miss=copy.deepcopy(rows);miss[5],miss[6]=miss[6],miss[5]
    m=run(miss);assert m['outcome']=='NOT_WITNESSED' and not m['valid'] and m['healthyNewestRetry']
    miss=copy.deepcopy(rows);miss[4],miss[5]=miss[5],miss[4]
    assert run(miss)['outcome']=='NOT_WITNESSED'
    pending=copy.deepcopy(obs);pending['freshness']['pendingBeforeEdit']=False
    assert run(o=pending)['outcome']=='NOT_WITNESSED'
    stale=copy.deepcopy(rows);so=copy.deepcopy(obs);error={'code':-32801,'message':'Context stale','data':{'reason':'CONTENT_CHANGED'}};stale[6]['message']['error']=error;so['freshness']['rapid']={'error':error}
    assert run(stale,so)['outcome']=='STALE_REJECTION_OBSERVED' and not run(stale,so)['valid']
    # READY scheduling misses bind text to the independently observed request snapshot.
    ready=copy.deepcopy(rows);ready[6]['message']={'jsonrpc':'2.0','id':6,'result':packet(c.OVERLAY,2,'2')};ready[5],ready[6]=ready[6],ready[5]
    ro=copy.deepcopy(obs);ro['freshness']['rapid']={'result':ready[5]['message']['result']}
    assert run(ready,ro)['outcome']=='NOT_WITNESSED'
    forged=copy.deepcopy(ready);fo=copy.deepcopy(ro);forged[5]['message']['result']['overlayObservation']['digest']='f'*64;fo['freshness']['rapid']={'result':forged[5]['message']['result']}
    assert run(forged,fo)['outcome']=='FAILED'
    before_request=copy.deepcopy(rows);before_request[4],before_request[5]=before_request[5],before_request[4];before_request[6]['message']={'jsonrpc':'2.0','id':6,'result':packet(f.NEWEST,3,'2')}
    bo=copy.deepcopy(obs);bo['freshness']['rapid']={'result':before_request[6]['message']['result']}
    assert run(before_request,bo)['outcome']=='NOT_WITNESSED'
    wrong=copy.deepcopy(before_request);wo=copy.deepcopy(bo);wrong[6]['message']['result']=packet(c.OVERLAY,2,'2');wo['freshness']['rapid']={'result':wrong[6]['message']['result']}
    assert run(wrong,wo)['outcome']=='FAILED'
    negative(lambda r,o:r[6]['message'].update(id='6'))
    negative(lambda r,o:r[4]['message'].update(id='x'*4097))
    negative(lambda r,o:r[4]['message'].update(jsonrpc='1.0'))
    negative(lambda r,o:r.insert(6,edit(f.NEWEST,4)))
    negative(lambda r,o:r.insert(6,row('client-to-server',method='textDocument/didChange',params={'textDocument':{'uri':'file:///other','version':2},'contentChanges':[{'text':'x'}]})))
    negative(lambda r,o:r.insert(6,row('client-to-server',id=8,method='initialize',params={})))
    negative(lambda r,o:r.insert(6,row('client-to-server',method='initialized',params={})))
    negative(lambda r,o:r[6]['message']['error'].update(message='wrong'))
    negative(lambda r,o:r[6]['message']['error']['data'].update(extra=True))
    result_change(lambda p:p['core'].update(extra=True))
    result_change(lambda p:p['overlayObservation'].update(version=2))
    negative(lambda r,o:r.insert(6,row('client-to-server',method='textDocument/didSave',params={'textDocument':{'uri':uri}})))
    print('LEQ-V0-006 whole-transcript cancellation/retry checks passed; negatives:',count,'actual editors NOT_RUN')
