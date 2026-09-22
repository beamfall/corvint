// SPDX-License-Identifier: AGPL-3.0-or-later
import {readFileSync,writeFileSync,mkdirSync,copyFileSync,chmodSync} from 'node:fs';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createHash} from 'node:crypto';
import {run} from './process.mjs';
const here=dirname(fileURLToPath(import.meta.url)),root=resolve(here,'../..');
const sdk=join(here,'node_modules/@earendil-works/pi-coding-agent');
const out=join(here,'build'),release=join(out,'release');
const digest=bytes=>createHash('sha256').update(bytes).digest('hex');
const nodeVersion='22.23.2',archiveSHA='61130f394c1630d211dd50aecc4353d379480f36d3ac913cd85dbba1aed585c6';
async function checked(command,args,options={}) {
 const result=await run(command,args,{cwd:here,...options});
 if(result.code!==0)throw Error(`${command} failed: ${result.stderr||result.stdout}`);
 return result;
}
async function bundle(entry,plugins,banner='') {
 const result=await Bun.build({entrypoints:[entry],target:'node',format:'cjs',banner,plugins,define:{'import.meta.resolve':'undefined','import.meta.url':'__corvintModuleURL','PI_BUNDLED_NODE':'true','process.env.PI_PACKAGE_DIR':'__corvintAssetRoot'}});
 if(!result.success)throw Error(result.logs.join('\n'));
 if(result.outputs.length!==1)throw Error('unexpected-bundle-splitting');
 return result.outputs[0].text();
}
async function main(){
 if(process.platform!=='darwin'||process.arch!=='arm64'||Bun.version!=='1.3.11')throw Error('build-requires-macos-arm64-bun-1.3.11');
 const archive=process.env.CORVINT_PI_NODE_ARCHIVE;
 if(!archive||digest(readFileSync(archive))!==archiveSHA)throw Error('pinned-node-archive-required');
 if(JSON.parse(readFileSync(join(sdk,'package.json'))).version!=='0.85.1')throw Error('pinned-pi-required');
 mkdirSync(release,{recursive:true});
 const node=join(out,'node');
 await checked('/usr/bin/python3',['-c',`import tarfile,sys\nwith tarfile.open(sys.argv[1]) as t:\n m=t.getmember('node-v${nodeVersion}-darwin-arm64/bin/node')\n assert m.isfile()\n with open(sys.argv[2],'wb') as f:f.write(t.extractfile(m).read())`,archive,node]);
 chmodSync(node,0o755);
 const photon=join(sdk,'node_modules/@silvia-odwyer/photon-node');
 const wasm=readFileSync(join(photon,'photon_rs_bg.wasm')).toString('base64');
 const basePlugin={name:'fixed-dependencies',setup(b){
  b.onResolve({filter:/^@corvint\/pi-internal\//},a=>({path:join(sdk,'dist',a.path.slice('@corvint/pi-internal/'.length))}));
  b.onResolve({filter:/^@corvint\/pi-ai-internal\//},a=>({path:join(sdk,'node_modules/@earendil-works/pi-ai/dist',a.path.slice('@corvint/pi-ai-internal/'.length))}));
  b.onResolve({filter:/^jiti(?:\/.*)?$/},()=>({path:'closed-jiti',namespace:'corvint'}));
  b.onLoad({filter:/^closed-jiti$/,namespace:'corvint'},()=>({contents:"export function createJiti(){throw Error('external-extensions-unavailable')}",loader:'js'}));
  // Upstream deliberately hides these Node-only imports from general-purpose bundlers.
  // This distribution is Node-only: expose only the pinned module set to static bundling.
  b.onLoad({filter:/\/(auth\/oauth\/load|api\/bedrock-converse-stream\.lazy)\.js$/},a=>{
   const source=readFileSync(a.path,'utf8'),needle='return import(__rewriteRelativeImportExtension(runtimeSpecifier));';
   if(source.split(needle).length!==2)throw Error('provider-loader-version-drift');
   const names=a.path.endsWith('/load.js')?['anthropic','openai-codex','github-copilot','openrouter','kimi-coding','xai','radius']:['bedrock-converse-stream'];
   const closed='switch(specifier){'+names.map(name=>`case './${name}.ts': return import('./${name}.js');`).join('')+"default: throw Error('provider-module-unavailable')}";
   return {contents:source.replace(needle,closed),loader:'js',resolveDir:dirname(a.path)};
  });
  b.onLoad({filter:/\/utils\/photon\.js$/},()=>({contents:`import * as photon from ${JSON.stringify(join(photon,'photon_rs.js'))};export async function loadPhoton(){return photon}`,loader:'js'}));
  b.onLoad({filter:/\/photon_rs\.js$/},a=>{
   const source=readFileSync(a.path,'utf8'),needle="const bytes = require('fs').readFileSync(path);";
   if(source.split(needle).length!==2)throw Error('photon-version-drift');
   return {contents:source.replace(needle,`const bytes = Buffer.from(${JSON.stringify(wasm)},'base64');`),loader:'js'};
  });
 }};
 const worker=await bundle(join(sdk,'dist/utils/image-resize-worker.js'),[basePlugin]);
 const assets={};
 const assetNames=['package.json','README.md','CHANGELOG.md','dist/modes/interactive/theme/dark.json','dist/modes/interactive/theme/light.json','dist/modes/interactive/assets/clankolas.png','dist/core/export-html/template.html','dist/core/export-html/template.css','dist/core/export-html/template.js','dist/core/export-html/vendor/marked.min.js','dist/core/export-html/vendor/highlight.min.js'];
 for(const name of assetNames)assets[name]=readFileSync(join(sdk,name)).toString('base64');
 const banner=readFileSync(join(here,'bootstrap.cjs'),'utf8').replace('CORVINT_EMBEDDED_ASSETS',JSON.stringify(assets));
 const mainPlugin={name:'fixed-image-worker',setup(b){
  b.onResolve({filter:/^corvint:fixed-image-worker$/},()=>({path:'image-worker',namespace:'corvint'}));
  b.onLoad({filter:/^image-worker$/,namespace:'corvint'},()=>({contents:`export default ${JSON.stringify(worker)}`,loader:'js'}));
  b.onLoad({filter:/\/utils\/image-resize\.js$/},()=>({contents:readFileSync(join(here,'image-resize.mjs'),'utf8'),loader:'js',resolveDir:here}));
 }};
 const source=await bundle(join(here,'runtime.ts'),[basePlugin,mainPlugin],banner);
 const main=join(out,'runtime.cjs'),blob=join(out,'runtime.blob'),binary=join(release,'pi-protected');
 writeFileSync(main,source);
 writeFileSync(join(out,'sea.json'),JSON.stringify({main,output:blob,disableExperimentalSEAWarning:true,useCodeCache:true,execArgvExtension:'none',execArgv:['--disable-sigusr1','--openssl-config=/dev/null','--']}));
 await checked(node,['--experimental-sea-config',join(out,'sea.json')]);
 copyFileSync(node,binary);chmodSync(binary,0o755);
 await checked('/usr/bin/codesign',['--remove-signature',binary]);
 await checked(node,[join(here,'node_modules/postject/dist/cli.js'),binary,'NODE_SEA_BLOB',blob,'--sentinel-fuse','NODE_SEA_FUSE_fce680ab2cc467b6e072b8b5df1996b2','--macho-segment-name','NODE_SEA']);
 await checked('/usr/bin/codesign',['--sign','-','--options','runtime','--entitlements',join(here,'runtime-entitlements.plist'),binary]);
 await checked('/usr/bin/codesign',['--verify','--strict',binary]);
 await checked('go',['build','-trimpath','-o',join(release,'corvint'),'./cmd/corvint'],{cwd:root,env:{...process.env,GOTOOLCHAIN:'local',GOCACHE:process.env.CORVINT_GOCACHE??'/tmp/corvint-go-build-cache'},timeout:180000});
 const manifest={schema:'corvint-pi-protected-build/0',status:'EXPERIMENTAL_UNQUALIFIED',pi:'0.85.1',node:nodeVersion,bun:Bun.version,nodeArchiveSHA256:archiveSHA,lockSHA256:digest(readFileSync(join(here,'package-lock.json'))),bundleSHA256:digest(source),workerSHA256:digest(worker),photonSHA256:digest(Buffer.from(wasm,'base64')),entitlementsSHA256:digest(readFileSync(join(here,'runtime-entitlements.plist'))),assets:Object.fromEntries(Object.entries(assets).map(([k,v])=>[k,digest(Buffer.from(v,'base64'))])),images:Object.fromEntries(['pi-protected','corvint'].map(k=>[k,digest(readFileSync(join(release,k)))]))};
 writeFileSync(join(release,'manifest.json'),JSON.stringify(manifest,null,2)+'\n');
 console.log(JSON.stringify({release,sha256:manifest.images['pi-protected'],status:manifest.status}));
}
await main();
