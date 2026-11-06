// SPDX-License-Identifier: Apache-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {spawnSync} from 'node:child_process';

function bunAvailable(t) {
 const result=spawnSync('bun',['--version'],{encoding:'utf8',timeout:10000});
 if(result.error?.code==='ENOENT'&&process.env.CORVINT_REQUIRE_BUN!=='1'){
  t.skip('bun is not installed');return false;
 }
 assert.ifError(result.error);
 assert.equal(result.status,0,result.stderr);
 assert.equal(result.stdout.trim(),'1.3.11','pinned-bun-required');
 return true;
}

test('PPI-V0-8100 protected dependency closure excludes vulnerable highlight.js',()=>{
 const lock=JSON.parse(readFileSync(new URL('./package-lock.json',import.meta.url)));
 const entries=Object.entries(lock.packages).filter(([path])=>path.endsWith('/highlight.js'));
 assert.ok(entries.length);
 for(const [,pkg] of entries)assert.equal(pkg.version,'11.12.0');
});

test('PPI-V0-8100 the bundled SDK highlighter loads v11 and highlights C and C++',async t=>{
 if(!bunAvailable(t))return;
 const {mkdtemp,writeFile,rm}=await import('node:fs/promises');
 const {tmpdir}=await import('node:os');
 const {join,resolve}=await import('node:path');
 const dir=await mkdtemp(join(tmpdir(),'pi-dependencies-'));t.after(()=>rm(dir,{recursive:true,force:true}));
 const sdk=resolve('integrations/pi-protected/node_modules/@earendil-works/pi-coding-agent');
 const entry=join(dir,'entry.mjs'),bundle=join(dir,'bundle.cjs'),script=join(dir,'build.mjs');
 await writeFile(entry,`import {highlight,supportsLanguage,loadAllHighlightLanguages} from ${JSON.stringify(join(sdk,'dist/utils/syntax-highlight.js'))};
import assert from 'node:assert/strict';
(async()=>{await loadAllHighlightLanguages();for(const language of ['c','cpp','arduino','javascript']){assert.ok(supportsLanguage(language),language);assert.match(highlight('int main() { return 42; }',{language,theme:{keyword:s=>'KEYWORD('+s+')'}}),/KEYWORD/)}console.log('bundled-highlighter-pass')})().catch(e=>{console.error(e);process.exitCode=1});`);
 await writeFile(script,`import {highlightImport} from ${JSON.stringify(new URL('./highlight-import.mjs',import.meta.url).href)};
if(Bun.version!=='1.3.11')throw Error('pinned-bun-required');
const r=await Bun.build({entrypoints:[${JSON.stringify(entry)}],target:'node',format:'cjs',plugins:[highlightImport(${JSON.stringify(sdk)})]});
if(!r.success)throw Error(r.logs.join('\\n'));await Bun.write(${JSON.stringify(bundle)},r.outputs[0]);`);
 const build=spawnSync('bun',[script],{encoding:'utf8',timeout:30000});assert.ifError(build.error);assert.equal(build.status,0,build.stderr);
 const run=spawnSync(process.execPath,[bundle],{encoding:'utf8',timeout:30000});assert.ifError(run.error);assert.equal(run.status,0,run.stderr);assert.match(run.stdout,/bundled-highlighter-pass/);
});

test('PPI-V0-8101 protected dependency closure excludes yaml 2.9.0',()=>{
 const lock=JSON.parse(readFileSync(new URL('./package-lock.json',import.meta.url)));
 const entries=Object.entries(lock.packages).filter(([path])=>path.endsWith('/yaml'));
 assert.ok(entries.length);
 for(const [,pkg] of entries)assert.equal(pkg.version,'2.9.1');
});

test('PPI-V0-8101 bundled SDK frontmatter retains mapping, alias and body parsing',async t=>{
 if(!bunAvailable(t))return;
 const {mkdtemp,writeFile,rm}=await import('node:fs/promises');
 const {tmpdir}=await import('node:os');
 const {join,resolve}=await import('node:path');
 const dir=await mkdtemp(join(tmpdir(),'pi-yaml-'));t.after(()=>rm(dir,{recursive:true,force:true}));
 const sdk=resolve('integrations/pi-protected/node_modules/@earendil-works/pi-coding-agent');
 const entry=join(dir,'entry.mjs'),bundle=join(dir,'bundle.cjs');
 await writeFile(entry,`import {parseFrontmatter} from ${JSON.stringify(join(sdk,'dist/utils/frontmatter.js'))};
import assert from 'node:assert/strict';
const value=parseFrontmatter('---\\nname: &name Corvint\\nalias: *name\\n---\\nBody');
assert.deepEqual(value,{frontmatter:{name:'Corvint',alias:'Corvint'},body:'Body'});console.log('bundled-yaml-pass');`);
 const build=spawnSync('bun',['build',entry,'--target=node','--format=cjs','--outfile='+bundle],{encoding:'utf8',timeout:30000});assert.ifError(build.error);assert.equal(build.status,0,build.stderr);
 const run=spawnSync(process.execPath,[bundle],{encoding:'utf8',timeout:30000});assert.ifError(run.error);assert.equal(run.status,0,run.stderr);assert.match(run.stdout,/bundled-yaml-pass/);
});

test('PPI-V0-8100 exported HTML embeds the locked patched browser highlighter with its license',async t=>{
 if(!bunAvailable(t))return;
 const {mkdtemp,writeFile,readFile,rm}=await import('node:fs/promises');
 const {tmpdir}=await import('node:os');
 const {join,resolve}=await import('node:path');
 const {runInNewContext}=await import('node:vm');
 const dir=await mkdtemp(join(tmpdir(),'pi-browser-highlight-'));t.after(()=>rm(dir,{recursive:true,force:true}));
 const sdk=resolve('integrations/pi-protected/node_modules/@earendil-works/pi-coding-agent');
 const script=join(dir,'browser.mjs'),output=join(dir,'highlight.js');
 await writeFile(script,`import {fixedAssets} from ${JSON.stringify(new URL('./assets.mjs',import.meta.url).href)};const assets=await fixedAssets(${JSON.stringify(sdk)},['package.json','dist/core/export-html/vendor/highlight.min.js']);if(JSON.parse(Buffer.from(assets['package.json'],'base64')).version!=='0.85.1')throw Error('wrong-sdk');await Bun.write(${JSON.stringify(output)},Buffer.from(assets['dist/core/export-html/vendor/highlight.min.js'],'base64'));`);
 const build=spawnSync('bun',[script],{encoding:'utf8',timeout:30000});assert.ifError(build.error);assert.equal(build.status,0,build.stderr);
 const source=await readFile(output,'utf8'),context={};runInNewContext(source,context,{timeout:5000});
 const upstream={};runInNewContext(readFileSync(join(sdk,'dist/core/export-html/vendor/highlight.min.js'),'utf8'),upstream,{timeout:5000});
 assert.equal(upstream.hljs.versionString,'11.9.0','the pinned SDK browser asset would retain the old version without replacement');
 assert.equal(context.hljs.versionString,'11.12.0');assert.ok(context.hljs.getLanguage('arduino'));
 assert.match(context.hljs.highlight('int main() { return 42; }',{language:'cpp'}).value,/hljs-keyword/);
 assert.match(source,/Redistribution and use in source and binary forms/);
});
