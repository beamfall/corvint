// SPDX-License-Identifier: Apache-2.0
import {readFileSync} from 'node:fs';
import {join} from 'node:path';
import {browserHighlight} from './highlight-import.mjs';

export async function fixedAssets(sdk,names) {
 const assets={};
 for(const name of names){
  const bytes=name==='dist/core/export-html/vendor/highlight.min.js'?Buffer.from(await browserHighlight(sdk)):readFileSync(join(sdk,name));
  assets[name]=bytes.toString('base64');
 }
 return assets;
}
