// SPDX-License-Identifier: Apache-2.0
// LVT-V0-009: private test scratch, with conservative recovery after SIGKILL.
import { constants, closeSync, fstatSync, lstatSync, mkdtempSync, openSync, readFileSync, readdirSync, readlinkSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const profile='corvint-pi-host-scratch/1', prefix='corvint-pi-host-', marker='.harness-owner.json';
const owned=new Set();
const privateDir=stat=>stat.isDirectory() && stat.uid===process.getuid() && (stat.mode&0o077)===0;
const pidNamespace=()=>{
 if(process.platform!=='linux')return null;
 try{return readlinkSync('/proc/self/ns/pid')}catch{return undefined}
};

export function createHostScratch(root=tmpdir()) {
 const path=mkdtempSync(join(root,prefix));
 try {
  writeFileSync(join(path,marker),JSON.stringify({profile,pid:process.pid,created:Date.now(),pidNamespace:pidNamespace()}),{flag:'wx',mode:0o600});
  const stat=lstatSync(path);owned.add({path,dev:stat.dev,ino:stat.ino});return path;
 } catch(error) {rmSync(path,{recursive:true,force:true});throw error}
}

export function cleanupHostScratch() {
 for(const entry of owned) {
  try {
   const stat=lstatSync(entry.path);
   if(privateDir(stat)&&stat.dev===entry.dev&&stat.ino===entry.ino)rmSync(entry.path,{recursive:true,force:true});
  } catch {}
 }
 owned.clear();
}

export function installHostScratchCleanup(before=()=>{}) {
 const cleanup=()=>{try{before()}finally{cleanupHostScratch()}};
 process.once('exit',cleanup);
 for(const [signal,code] of [['SIGHUP',129],['SIGINT',130],['SIGTERM',143]])
  process.once(signal,()=>{cleanup();process.exit(code)});
}

export function sweepHostScratch(root=tmpdir(),now=Date.now()) {
 let names;try{names=readdirSync(root).filter(name=>name.startsWith(prefix)).sort().slice(0,512)}catch{return []}
 const removed=[];
 for(const name of names) {
  const path=join(root,name);let fd;
  try {
   const stat=lstatSync(path);
   if(!privateDir(stat)||stat.mtimeMs>now-86400000)continue;
   // O_NOFOLLOW plus a bounded regular owner file avoids trusting symlinks or arbitrary files.
   fd=openSync(join(path,marker),constants.O_RDONLY|constants.O_NOFOLLOW|constants.O_NONBLOCK);
   const ownerStat=fstatSync(fd);
   if(!ownerStat.isFile()||ownerStat.uid!==process.getuid()||(ownerStat.mode&0o077)!==0||ownerStat.size>512)continue;
   const owner=JSON.parse(readFileSync(fd,'utf8'));
   if(owner.profile!==profile||!Number.isSafeInteger(owner.pid)||owner.pid<=1||!Number.isSafeInteger(owner.created)||owner.created<=0||owner.created>now-86400000)continue;
   // ESRCH only proves absence inside the caller's PID namespace. Older or
   // foreign-namespace markers remain ambiguous even when their PID is absent here.
   if(process.platform==='linux') {
    const namespace=pidNamespace();if(!namespace||owner.pidNamespace!==namespace)continue;
   }
   try{process.kill(owner.pid,0);continue}catch(error){if(error.code!=='ESRCH')continue}
   const current=lstatSync(path);
   if(!privateDir(current)||current.dev!==stat.dev||current.ino!==stat.ino)continue;
   rmSync(path,{recursive:true,force:true});removed.push(path);
  } catch {} finally {if(fd!==undefined)closeSync(fd)}
 }
 return removed;
}
