# SPDX-License-Identifier: AGPL-3.0-or-later
"""Whole-transcript oracle for one experimental edit-cancellation witness."""
import hashlib
import importlib.util
import json
import math
from pathlib import Path

spec = importlib.util.spec_from_file_location('context_probe', Path(__file__).with_name('context-probe.py'))
context = importlib.util.module_from_spec(spec); spec.loader.exec_module(context)
NEWEST = 'package p\n/*😀*/ var newest int\nvar y = newest\n'

def identity(value):
    if type(value) not in (int, str) or len(json.dumps(value, ensure_ascii=False).encode()) > 4096:
        raise ValueError('invalid request ID')
    return type(value).__name__, value

def json_equal(left, right):
    """Compare JSON structure without numeric aliases or nonfinite values."""
    if type(left) is not type(right):return False
    if type(left) is dict:
        return all(type(key) is str for key in left) and all(type(key) is str for key in right) and left.keys()==right.keys() and all(json_equal(left[key],right[key]) for key in left)
    if type(left) is list:
        return len(left)==len(right) and all(json_equal(a,b) for a,b in zip(left,right))
    if type(left) is float:return math.isfinite(left) and math.isfinite(right) and left==right
    if type(left) in (str,int,bool,type(None)):return left==right
    return False

def valid_error(error):
    # Equality alone admits Python float/bool aliases for integer wire fields.
    if not isinstance(error,dict) or set(error)!={'code','message','data'} or type(error['code']) is not int or type(error['message']) is not str:
        return False
    data=error['data']
    if not isinstance(data,dict) or set(data)!={'reason'} or type(data['reason']) is not str:
        return False
    return (error['code'],error['message'],data['reason']) in (
        (-32800,'Request cancelled','CANCELLED'),(-32801,'Context stale','CONTENT_CHANGED'))

def validate(rows, observation, uri, before, after):
    errors = []
    def failed(reason):
        return {'valid':False, 'completeWitness':False, 'outcome':'FAILED', 'errors':errors+[reason], 'qualification':'UNQUALIFIED'}
    if observation.get('initializeResult',{}).get('capabilities',{}).get('experimental',{}).get('corvintContext') != {'method':'corvint/context','schema':'corvint-editor-context/0'}:
        return failed('context-marker-invalid')
    frontend=observation.get('freshness')
    if not isinstance(frontend,dict) or set(frontend)!={'baseline','rapid','retry','pendingBeforeEdit','bufferModified','versions'} or type(frontend['pendingBeforeEdit']) is not bool or frontend['bufferModified'] is not True:
        return failed('frontend-observation-invalid')
    queries=[];opens=[];changes=[];responses={}
    # Inspect every row, including document/cancel events outside the three queries.
    for i,row in enumerate(rows):
        if not isinstance(row,dict) or set(row)!={'direction','message'} or not isinstance(row.get('message'),dict):return failed('invalid-wire-row')
        if row['direction'] in ('provenance','lifecycle'):continue
        if row['direction'] not in ('client-to-server','server-to-client'):return failed('invalid-wire-direction')
        m=row['message'];direction=row['direction']
        if m.get('jsonrpc')!='2.0':return failed('invalid-jsonrpc')
        method=m.get('method')
        if direction=='client-to-server':
            if method=='textDocument/didOpen':opens.append((i,m))
            if method=='textDocument/didChange':changes.append((i,m))
            if method=='corvint/context':
                if set(m)!={'jsonrpc','id','method','params'} or not isinstance(m['params'],dict) or type(m['params'].get('limit')) is not int or m['params']!={'textDocument':{'uri':uri},'task':context.TASK,'limit':20}:return failed('context-request-invalid')
                try:key=identity(m['id'])
                except ValueError:return failed('context-request-id-invalid')
                queries.append((i,m,key))
        elif method is None:
            if 'id' not in m:return failed('response-id-missing')
            try:key=identity(m['id'])
            except ValueError:return failed('response-id-invalid')
            responses.setdefault(key,[]).append((i,m))
    if len(queries)!=3 or len({q[2] for q in queries})!=3 or len(opens)!=1 or len(changes)!=2:return failed('open-edits-queries-missing-or-extra')
    initial=opens[0][1]['params']['textDocument']
    if set(initial) not in ({'uri','version','text'},{'uri','version','text','languageId'}) or initial.get('uri')!=uri or initial.get('text')!=context.FILES['pkg/main.go'] or type(initial.get('version')) is not int or initial['version']<0 or initial.get('languageId','go')!='go':return failed('initial-open-invalid')
    versions=[]
    for (_,m),text in zip(changes,(context.OVERLAY,NEWEST)):
        p=m.get('params',{});d=p.get('textDocument',{})
        if set(p)!={'textDocument','contentChanges'} or set(d)!={'uri','version'} or d['uri']!=uri or type(d['version']) is not int or p['contentChanges']!=[{'text':text}]:return failed('full-edit-invalid')
        versions.append(d['version'])
    if not initial['version']<versions[0]<versions[1] or frontend['versions']!=versions or any(type(v) is not int for v in frontend['versions']):return failed('version-invalid')
    replies=[]
    for n,(_,_,key) in enumerate(queries):
        found=responses.get(key,[])
        if len(found)!=1:return failed('matching-response-missing-or-duplicate')
        i,m=found[0]
        if set(m) not in ({'jsonrpc','id','result'},{'jsonrpc','id','error'}):return failed('response-envelope-invalid')
        reported=frontend[('baseline','rapid','retry')[n]]
        expected={'result':m['result']} if 'result' in m else {'error':m['error']}
        if not isinstance(reported,dict) or set(reported)!=set(expected):return failed('frontend-response-shape-invalid')
        if 'error' in m and (not valid_error(m['error']) or not valid_error(reported['error'])):return failed('response-error-schema-invalid')
        if not json_equal(reported,expected):return failed('frontend-response-mismatch')
        replies.append((i,m))
    oi=opens[0][0];ai=changes[0][0];bi=changes[1][0];aq,rq,bq=[q[0] for q in queries];ar,rr,br=[r[0] for r in replies]
    if not oi<ai<aq<ar<rq<rr<bq<br or not ar<bi<bq:return failed('baseline-or-retry-order-invalid')
    for i,row in enumerate(rows):
        m=row['message'];method=m.get('method')
        if oi<i<br and row['direction']=='client-to-server':
            if method in ('$/cancelRequest','textDocument/didOpen','textDocument/didClose','initialize','initialized','shutdown','exit'):return failed('competing-transition:'+method)
            if 'id' in m and method!='corvint/context':return failed('competing-request')
            if method=='textDocument/didChange' and i not in (ai,bi):return failed('competing-edit')
            if isinstance(method,str) and method.startswith('textDocument/') and method!='textDocument/didChange':return failed('competing-document-notification')
    packets=[]
    for n,text in ((0,context.OVERLAY),(2,NEWEST)):
        m=replies[n][1]
        if 'result' not in m:return failed('successful-packet-missing')
        p=context.validate_packet(m['result'],uri,versions[0 if n==0 else 1],text,before,after)
        if not p['valid']:errors.extend(p['errors'])
        packets.append(m['result'])
    if errors:return failed('packet-invalid')
    a,b=[p['overlayObservation'] for p in packets]
    if a['sessionID']!=b['sessionID'] or int(b['captureID'])<=int(a['captureID']) or not json_equal(packets[0]['core'],packets[1]['core']):return failed('capture-or-core-binding-drift')
    witness=rq<bi<rr and frontend['pendingBeforeEdit'] is True
    rapid=replies[1][1]
    if witness:
        error=rapid.get('error')
        if error=={'code':-32800,'message':'Request cancelled','data':{'reason':'CANCELLED'}}:outcome='EDIT_CANCELLED_WITNESSED'
        elif error=={'code':-32801,'message':'Context stale','data':{'reason':'CONTENT_CHANGED'}}:outcome='STALE_REJECTION_OBSERVED'
        else:return failed('rapid-response-not-edit-cancellation')
    else:
        # A complete scheduling miss is retained; malformed rapid evidence still fails.
        if 'result' in rapid:
            expected_text=context.OVERLAY if rq<bi else NEWEST
            expected_version=versions[0] if rq<bi else versions[1]
            p=context.validate_packet(rapid['result'],uri,expected_version,expected_text,before,after)
            if not p['valid']:return failed('scheduling-miss-packet-invalid')
            snapshot=packets[0] if rq<bi else packets[1]
            rapid_overlay=rapid['result']['overlayObservation']
            if rapid_overlay['sessionID']!=snapshot['overlayObservation']['sessionID'] or rapid_overlay['captureID']!=snapshot['overlayObservation']['captureID'] or not json_equal(rapid['result']['core'],snapshot['core']):return failed('scheduling-miss-packet-lineage-invalid')
        elif rapid.get('error') not in ({'code':-32800,'message':'Request cancelled','data':{'reason':'CANCELLED'}},{'code':-32801,'message':'Context stale','data':{'reason':'CONTENT_CHANGED'}}):return failed('scheduling-miss-error-invalid')
        outcome='NOT_WITNESSED'
    return {'valid':outcome=='EDIT_CANCELLED_WITNESSED','completeWitness':outcome=='EDIT_CANCELLED_WITNESSED','outcome':outcome,'errors':[], 'healthyNewestRetry':True,'baselineCaptureID':a['captureID'],'retryCaptureID':b['captureID'],'qualification':'UNQUALIFIED'}
