// SPDX-License-Identifier: Apache-2.0
import {join} from 'node:path';

// Bun 1.3.11 resolves replacement onLoad imports against the original SDK path.
// Admit this one local replacement only for the SDK module we replace.
export function fixedImageImport(sdk,here) {
 return {name:'fixed-image-import',setup(build){
  build.onResolve({filter:/^\.\/resize-worker\.mjs$/},args=>{
   if(args.importer!==join(sdk,'dist/utils/image-resize.js'))return;
   return {path:join(here,'resize-worker.mjs')};
  });
 }};
}
