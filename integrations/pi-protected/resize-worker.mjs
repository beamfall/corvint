// SPDX-License-Identifier: AGPL-3.0-or-later
// Upstream resizeImage resolves ResizedImage|null and never rejects; its callers
// omit an image on null. The fixed worker has no in-process fallback, so every
// worker failure, exit or deadline resolves null (V1-0448).
export const RESIZE_DEADLINE_MS=10000;
export function resizeInWorker(worker,inputBytes,mimeType,options,timers={setTimeout,clearTimeout}) {
 return new Promise(resolve=>{
  const timer=timers.setTimeout(()=>resolve(null),RESIZE_DEADLINE_MS);
  const finish=value=>{timers.clearTimeout(timer);resolve(value)};
  worker.once('message',message=>finish(message&&typeof message==='object'&&!message.error?message.result??null:null));
  worker.once('error',()=>finish(null));
  worker.once('exit',()=>finish(null));
  const bytes=new Uint8Array(inputBytes);
  worker.postMessage({inputBytes:bytes,mimeType,options},[bytes.buffer]);
 });
}
