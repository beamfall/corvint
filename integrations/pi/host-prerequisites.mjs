// SPDX-License-Identifier: Apache-2.0
import {spawnSync} from 'node:child_process';
import {existsSync} from 'node:fs';

// Missing optional hosts are visible locally; qualification jobs must install them.
export function hostAvailable(t, command, args=['--version']) {
 const result=spawnSync(command,args,{encoding:'utf8',timeout:10000});
 if(result.error?.code==='ENOENT')return missingHost(t,`${command} is not installed`);
 if(result.error)throw result.error;
 if(result.status!==0)throw Error(`${command} prerequisite failed: ${result.stderr}`);
 return true;
}
export function missingHost(t, reason) {
 if(process.env.CORVINT_REQUIRE_PI==='1')throw Error(`required Pi qualification prerequisite: ${reason}`);
 t.skip(reason);return false;
}
export function hostFile(t,path) {
 return path&&existsSync(path)||missingHost(t,`host artifact is not installed: ${path??'unset candidate path'}`);
}
export function taskWritesAvailable(t) {
 return process.env.CORVINT_PI_TASK_WRITES!=='0'||missingHost(t,'native Tasks fixture writes disabled by operator');
}
