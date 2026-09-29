#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Controlled observation clocks and unreaped-owner signal regressions, never qualification."""
import importlib.util
from pathlib import Path
import signal
import subprocess

spec=importlib.util.spec_from_file_location('harness',Path(__file__).resolve().parents[2]/'script/qualify-lsp-editors.py')
h=importlib.util.module_from_spec(spec);spec.loader.exec_module(h)
root=Path('/private/tmp/corvint-owned-cleanup-seam')
command='/Applications/Visual Studio Code.app/Electron --user-data-dir '+str(root)
def fact(parent=1,cmd=command,start='generation-1',state='S'):
    return {'parent':parent,'command':cmd,'start':start,'state':state}
def observe(snapshot):
    clock=[0.0];signals=[];budgets=[]
    def inventory(timeout):
        budgets.append(timeout)
        return snapshot(clock[0],timeout)
    h.inventory=inventory;h.time.monotonic=lambda:clock[0];h.time.sleep=lambda dt:clock.__setitem__(0,clock[0]+dt)
    h.os.kill=lambda *args:signals.append(('pid',args));h.os.killpg=lambda *args:signals.append(('group',args))
    report={};h.cleanup_owned(None,root,report)
    assert not signals,'indirect processes must never be signaled'
    assert clock[0]<=5.000001 and all(0<b<=5 for b in budgets)
    return report,clock[0]
# Previous three-sample/0.2-second cleanup stops before this actual disappearance.
r,t=observe(lambda t,b:{101:fact(cmd=command if t<.2 else '(Code)',state='Z' if t>=.2 else 'S')} if t<.8 else {})
assert r['cleanup']=='OWNED_EDITOR_PROCESSES_RETIRED' and .8<=t<=1
r,t=observe(lambda t,b:{101:fact()});assert r['cleanup']=='OWNED_EDITOR_PROCESS_REMAINS' and t==5
r,t=observe(lambda t,b:{101:fact(),**({102:fact(parent=101,cmd='late-child')} if t>=4.8 else {})})
assert r['cleanup']=='OWNED_EDITOR_PROCESS_REMAINS' and 102 in r['remainingOwnedProcesses']
r,t=observe(lambda t,b:{101:fact(start='generation-1' if t<.1 else 'generation-2')})
assert r['cleanup']=='UNKNOWN'
r,t=observe(lambda t,b:{101:fact()} if t<.1 else {102:fact(parent=101,cmd='ambiguous-orphan')})
assert r['cleanup']=='UNKNOWN' and 102 not in r['ownedCleanupPids']
r,t=observe(lambda t,b:{303:fact(cmd='/unrelated')});assert r['cleanup']=='OWNED_EDITOR_PROCESSES_RETIRED'
def timeout(t,b):
    h.time.sleep(b)
    raise subprocess.TimeoutExpired('ps',b)
r,t=observe(timeout);assert r['cleanup']=='UNKNOWN' and t==1
for invalid in ['',7,None]:
    r,t=observe(lambda t,b,invalid=invalid:{101:fact(start=invalid)})
    assert r['cleanup']=='UNKNOWN'
class Owner:
    pid=1001
    returncode=None
    def __init__(self,reaped=False,held=False):self.reaped=reaped;self.held=held;self.calls=0
    def poll(self):return 0 if self.reaped else None
    def communicate(self,timeout):
        self.calls+=1
        if self.calls==1:
            if self.held:self.reaped=True
            raise subprocess.TimeoutExpired('owned',timeout)
        if self.held:raise subprocess.TimeoutExpired('held-pipe',timeout)
        self.reaped=True;self.returncode=-9;return b'',b''
signals=[];h.os.killpg=lambda pid,sig:signals.append((pid,sig));h.inventory=lambda timeout:{}
owner=Owner();h.cleanup_owned(owner,root,{})
assert signals==[(1001,signal.SIGTERM),(1001,signal.SIGKILL)]
signals.clear();owner=Owner(reaped=True);h.cleanup_owned(owner,root,{})
assert not signals
owner=Owner(held=True)
try:h.cleanup_owned(owner,root,{})
except subprocess.TimeoutExpired:pass
else:raise AssertionError('held pipe must preserve failure')
assert signals==[(1001,signal.SIGTERM)]
# Same helper guards the outer exception fallback after the leader was reaped.
assert h.signal_owned_group(owner,signal.SIGKILL) is False
assert signals==[(1001,signal.SIGTERM)]
print('delayed/late/live/ambiguous/timeout cleanup, zero indirect signals, and unreaped-owner escalation guards passed')

# Capture initial descendants before TERM removes their parent/command attribution.
for disappears in [False,True]:
    phase=[False];clock=[0.0];signals=[]
    class Parent:
        pid=1001;returncode=0
        def poll(self):return 0 if phase[0] else None
        def communicate(self,timeout):phase[0]=True;return b'',b''
    def snapshots(timeout):
        if not phase[0]:return {1001:fact(cmd='/direct-owner'),1002:fact(parent=1001)}
        if disappears and clock[0]>=.8:return {}
        return {1002:fact(parent=1,cmd='(Code)')}
    h.inventory=snapshots;h.time.monotonic=lambda:clock[0];h.time.sleep=lambda dt:clock.__setitem__(0,clock[0]+dt)
    h.os.killpg=lambda pid,sig:signals.append((pid,sig))
    r={};h.cleanup_owned(Parent(),root,r)
    assert 1002 in r['cleanupPreSignalPids']
    assert r['cleanup']==('OWNED_EDITOR_PROCESSES_RETIRED' if disappears else 'OWNED_EDITOR_PROCESS_REMAINS')
    assert signals==[(1001,signal.SIGTERM)]
# Failed setup cannot be cleared by permitted direct retirement or a later empty inventory.
phase=[False];calls=[]
def failed_setup(timeout):
    calls.append(timeout)
    raise subprocess.TimeoutExpired('initial-ps',timeout)
h.inventory=failed_setup;r={};h.cleanup_owned(Parent(),root,r)
assert r['cleanup']=='UNKNOWN' and calls==[1] and r['cleanupHold']==str(root)
print('pre-signal orphan/changed-command retention and sticky setup UNKNOWN passed')
