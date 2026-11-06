// SPDX-License-Identifier: Apache-2.0
import {createRequire} from 'node:module';
import {join,dirname} from 'node:path';
import {readFileSync} from 'node:fs';

// The pinned SDK uses v10's .js subpaths. v11 exports extensionless subpaths.
// Resolve only the SDK highlighter through the installed, locked dependency;
// no package edits or fallback to a global installation are permitted.
export function highlightImport(sdk) {
 const importer=join(sdk,'dist/utils/syntax-highlight.js');
 const require=createRequire(join(sdk,'package.json'));
 return {name:'highlight-v11-imports',setup(build){
  build.onResolve({filter:/^highlight\.js\/lib\/(?:core|index|languages\/[a-z0-9-]+)\.js$/},args=>{
   if(args.importer!==importer)return;
   const name=args.path==='highlight.js/lib/index.js'?'highlight.js':args.path.slice(0,-3);
   return {path:require.resolve(name)};
  });
 }};
}

// Pi also embeds an older browser copy in exported HTML. Build that asset from
// the same locked v11 closure instead of shipping the SDK's stale vendored file.
export async function browserHighlight(sdk) {
 const require=createRequire(join(sdk,'package.json'));
 const main=require.resolve('highlight.js'),pkg=require('highlight.js/package.json');
 if(pkg.version!=='11.12.0')throw Error('pinned-highlight-required');
 const license=readFileSync(join(dirname(require.resolve('highlight.js/package.json')),'LICENSE'),'utf8');
 const result=await Bun.build({entrypoints:['corvint:highlight-browser'],target:'browser',format:'iife',minify:true,
  banner:`/*! highlight.js ${pkg.version}\n${license.replaceAll('*/','* /')}\n*/`,
  plugins:[{name:'locked-browser-highlight',setup(build){
   build.onResolve({filter:/^corvint:highlight-browser$/},()=>({path:'entry',namespace:'corvint-highlight'}));
   build.onLoad({filter:/^entry$/,namespace:'corvint-highlight'},()=>({contents:`import hljs from ${JSON.stringify(main)};globalThis.hljs=hljs;`,loader:'js'}));
  }}]});
 if(!result.success||result.outputs.length!==1)throw Error('browser-highlight-build-failed');
 return result.outputs[0].text();
}
