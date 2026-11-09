import test from 'node:test';
import assert from 'node:assert/strict';
import {navigationPolicy} from '../navigation-policy.mjs';
const origin='http://127.0.0.1:3000';
function policy(max='read',disposable=false){const receipt={traffic:[],gaps:[]};return {receipt,p:navigationPolicy({manifest:{origin},maxEffect:max,origins:{disposable:disposable?[origin]:[]}},receipt)};}
const step={flow_id:'goal',step_id:'click',effect_class:'read'};
test('NEX-V0-003 grant and disposable are independent pre-dispatch checks',()=>{
 for(const [grant,listed,allowed] of [['read',true,false],['write-irreversible',false,false],['write-irreversible',true,true]]){
  const {p,receipt}=policy(grant,listed);p.activate(step);assert.equal(p.request(origin+'/write','POST'),allowed);assert.equal(receipt.traffic[0].effect,'write-irreversible');
 }
});
test('NEX-V0-003 hidden write and every form require an active admitted transition',()=>{
 const {p}=policy('write-irreversible',true);assert.equal(p.request(origin+'/write','POST'),false);
 for(const method of ['GET','FORM']){const {p}=policy();p.activate(step);assert.equal(p.request(origin+'/write',method,true),false);}
});
test('NEX-V0-003 origin and traffic budget fail closed',()=>{
 for(const url of ['http://localhost:3000/','http://127.0.0.1:3001/','https://127.0.0.1:3000/','http://user@127.0.0.1:3000/']){const {p}=policy();p.activate(step);assert.equal(p.request(url,'GET'),false);}
 const {p,receipt}=policy();for(let i=0;i<8192;i++)assert.equal(p.request(origin,'GET'),true);assert.equal(p.request(origin,'GET'),false);assert.equal(receipt.traffic.length,8192);assert.deepEqual(receipt.gaps,['traffic-budget']);
});
test('NEX-V0-004 recovery cannot revive blocked policy or exceed grant',()=>{
 const {p}=policy();p.activate(step);p.request(origin,'POST');assert.throws(()=>p.activate(step));
 assert.throws(()=>policy().p.activate({...step,effect_class:'write-irreversible'}));
});
