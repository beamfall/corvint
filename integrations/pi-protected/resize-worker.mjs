// SPDX-License-Identifier: Apache-2.0
// Upstream resizeImage resolves ResizedImage|null and never rejects; its callers
// omit an image on null. The fixed worker has no in-process fallback, so every
// worker failure, exit, malformed result or deadline resolves null (V1-0448).
export const RESIZE_DEADLINE_MS=10000;
const BASE64=/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/;
const dimension=v=>Number.isSafeInteger(v)&&v>0;
// Only the upstream ResizedImage shape passes; anything else is a failed
// resize. The copy drops unknown fields so nothing else reaches upstream.
export function resizedImage(v) {
 if(!v||typeof v!=='object'||Array.isArray(v))return null;
 const {data,mimeType,originalWidth,originalHeight,width,height,wasResized}=v;
 if(typeof data!=='string'||!data||!BASE64.test(data)||typeof mimeType!=='string'||!/^image\/[a-z0-9.+-]+$/.test(mimeType)||typeof wasResized!=='boolean')return null;
 if(![originalWidth,originalHeight,width,height].every(dimension)||width>originalWidth||height>originalHeight)return null;
 if(!wasResized&&(width!==originalWidth||height!==originalHeight))return null;
 return {data,mimeType,originalWidth,originalHeight,width,height,wasResized};
}
export function resizeInWorker(worker,inputBytes,mimeType,options,timers={setTimeout,clearTimeout}) {
 return new Promise(resolve=>{
  const timer=timers.setTimeout(()=>resolve(null),RESIZE_DEADLINE_MS);
  const finish=value=>{timers.clearTimeout(timer);resolve(value)};
  worker.once('message',message=>finish(message&&typeof message==='object'&&!message.error?resizedImage(message.result):null));
  worker.once('error',()=>finish(null));
  worker.once('exit',()=>finish(null));
  const bytes=new Uint8Array(inputBytes);
  worker.postMessage({inputBytes:bytes,mimeType,options},[bytes.buffer]);
 });
}
