// SPDX-License-Identifier: Apache-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,writeFile,rm,realpath} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {spawnSync} from 'node:child_process';

test('PPI-V0-009 Bun resolves the fixed worker only for the exact replaced SDK importer',async t=>{
 const dir=await realpath(await mkdtemp(join(tmpdir(),'corvint-fixed-image-')));
 t.after(()=>rm(dir,{recursive:true,force:true}));
 const sdk=join(dir,'sdk'),here=join(dir,'local'),utils=join(sdk,'dist/utils');
 await mkdir(utils,{recursive:true});await mkdir(here);
 await writeFile(join(here,'resize-worker.mjs'),'export const worker="fixed-worker";');
 const exact=join(utils,'image-resize.js'),other=join(dir,'other.js');
 const source='import {worker} from "./resize-worker.mjs";console.log(worker);';
 await writeFile(exact,source);await writeFile(other,source);
 const script=join(dir,'check.mjs');
 await writeFile(script,`import {fixedImageImport} from ${JSON.stringify(new URL('./fixed-image-import.mjs',import.meta.url).href)};
if(Bun.version!=='1.3.11')throw Error('pinned-bun-required');
const plugin=fixedImageImport(${JSON.stringify(sdk)},${JSON.stringify(here)});
const result=await Bun.build({entrypoints:[process.argv[2]],target:'node',plugins:[plugin]});
if(!result.success)throw Error(result.logs.join('\\n'));
if(!(await result.outputs[0].text()).includes('fixed-worker'))throw Error('exact-importer-not-resolved');
`);
 const result=spawnSync('bun',[script,exact],{encoding:'utf8',timeout:30000});
 assert.ifError(result.error);assert.equal(result.status,0,result.stdout+result.stderr);
 const negative=spawnSync('bun',[script,other],{encoding:'utf8',timeout:30000});
 assert.ifError(negative.error);assert.equal(negative.status,1,negative.stdout+negative.stderr);
 assert.match(negative.stderr,/Could not resolve:.*resize-worker\.mjs/);
});
