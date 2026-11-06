// SPDX-License-Identifier: Apache-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,mkdtempSync,mkdirSync,writeFileSync,rmSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {spawnSync} from 'node:child_process';
import {hostAvailable} from './host-prerequisites.mjs';

test('PWV-V0-8101 documented Pi MCP registration changes only the project config',t=>{
 if(!hostAvailable(t,process.env.PI_BIN??'pi'))return;
 const readme=readFileSync(new URL('./README.md',import.meta.url),'utf8');
 const command=readme.match(/```sh\n(pi mcp add corvint[^\n]+)\n```/)?.[1];
 assert.ok(command,'a copyable project-scoped recipe is required');
 assert.match(command,/--local -- /);
 const dir=mkdtempSync(join(tmpdir(),'pi-mcp-scope-'));t.after(()=>rmSync(dir,{recursive:true,force:true}));
 const home=join(dir,'home'),repo=join(dir,'repo'),agent=join(home,'.pi/agent');
 mkdirSync(agent,{recursive:true});mkdirSync(repo);
 const global=join(agent,'mcp.json'),bytes='{"mcpServers":{}}\n';writeFileSync(global,bytes);
 const version=spawnSync(process.env.PI_BIN??'pi',['--version'],{encoding:'utf8'});assert.ifError(version.error);assert.equal(version.stdout.trim(),'0.99.1');
 const args=command.replace('"$PWD"',repo).split(' ').slice(1);
 const run=spawnSync(process.env.PI_BIN??'pi',args,{cwd:repo,env:{...process.env,HOME:home,PI_CODING_AGENT_DIR:agent},encoding:'utf8',timeout:15000});
 assert.ifError(run.error);assert.equal(run.status,0,run.stderr);assert.match(run.stdout,/project MCP server/);
 assert.equal(readFileSync(global,'utf8'),bytes,'the global file stays byte-identical');
 const local=JSON.parse(readFileSync(join(repo,'.pi/mcp.json'),'utf8'));
 assert.deepEqual(local.mcpServers.corvint,{command:'corvint-mcp',args:['--protocol-version','2025-11-25','--root',repo]});
});
