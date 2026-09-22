// SPDX-License-Identifier: AGPL-3.0-or-later
import {Worker} from 'node:worker_threads';
import workerSource from 'corvint:fixed-image-worker';
export async function resizeImage(inputBytes,mimeType,options) {
 const worker=new Worker(workerSource,{eval:true,execArgv:[],resourceLimits:{maxOldGenerationSizeMb:128}});
 try {
  return await new Promise((resolve,reject)=>{
   const timer=setTimeout(()=>reject(Error('image-resize-deadline')),10000);
   const finish=fn=>value=>{clearTimeout(timer);fn(value)};
   worker.once('message',finish(message=>message.error?reject(Error('image-resize-failed')):resolve(message.result??null)));
   worker.once('error',finish(reject));
   worker.once('exit',finish(()=>reject(Error('image-resize-exited'))));
   const bytes=new Uint8Array(inputBytes);
   worker.postMessage({inputBytes:bytes,mimeType,options},[bytes.buffer]);
  });
 } finally {await worker.terminate()}
}
export function formatDimensionNote(result) {
 if(!result.wasResized)return undefined;
 const scale=result.originalWidth/result.width;
 return `[Image: original ${result.originalWidth}x${result.originalHeight}, displayed at ${result.width}x${result.height}. Multiply coordinates by ${scale.toFixed(2)} to map to original image.]`;
}
