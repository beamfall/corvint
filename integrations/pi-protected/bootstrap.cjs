// SPDX-License-Identifier: AGPL-3.0-or-later
// This prelude executes before SDK module initializers. Asset bytes are part of the signed image.
var __corvintModuleURL=require('node:url').pathToFileURL(process.execPath).href;
var __corvintAssetRoot=process.execPath+'.assets';
{
 const fs=require('node:fs'),path=require('node:path'),module=require('node:module');
 const assets=CORVINT_EMBEDDED_ASSETS;
 const read=fs.readFileSync.bind(fs),exists=fs.existsSync.bind(fs);
 const key=file=>typeof file==='string'&&file.startsWith(__corvintAssetRoot+'/')?path.relative(__corvintAssetRoot,file):undefined;
 fs.readFileSync=(file,options)=>{
  const name=key(file);if(name===undefined)return read(file,options);
  if(!Object.hasOwn(assets,name))throw Error('protected-asset-unavailable');
  const bytes=Buffer.from(assets[name],'base64');
  const encoding=typeof options==='string'?options:options?.encoding;
  return encoding?bytes.toString(encoding):bytes;
 };
 fs.existsSync=file=>{const name=key(file);return name===undefined?exists(file):Object.hasOwn(assets,name)||Object.keys(assets).some(p=>p.startsWith(name+'/'))};
 const create=module.createRequire;
 module.createRequire=filename=>{
  const original=create(filename);
  const builtin=id=>{if(!module.isBuiltin(id))throw Error('external-module-unavailable');return original(id)};
  builtin.resolve=id=>{if(!module.isBuiltin(id))throw Error('external-module-unavailable');return original.resolve(id)};
  return builtin;
 };
 process.dlopen=()=>{throw Error('native-addon-unavailable')};
 // Provider credentials come from the explicitly selected data store. Ambient provider config
 // (including cloud executable credential helpers) cannot alter this runtime's code boundary.
 const keep=new Set(['PATH','HOME','USER','LOGNAME','LANG','LC_ALL','LC_CTYPE','TMPDIR','TERM','COLORTERM','TERM_PROGRAM','TERM_PROGRAM_VERSION','NO_COLOR']);
 for(const name of Object.keys(process.env))if(!keep.has(name))delete process.env[name];
 process.env.PI_OFFLINE='1';
 module.syncBuiltinESMExports();
}
