// SPDX-License-Identifier: Apache-2.0
import {Worker} from 'node:worker_threads';
import workerSource from 'corvint:fixed-image-worker';
import {resizeInWorker} from './resize-worker.mjs';
export async function resizeImage(inputBytes,mimeType,options) {
 let worker;
 try {
  worker=new Worker(workerSource,{eval:true,execArgv:[],resourceLimits:{maxOldGenerationSizeMb:128}});
  return await resizeInWorker(worker,inputBytes,mimeType,options);
 } catch {return null} finally {await worker?.terminate().catch(()=>undefined)}
}
export function formatDimensionNote(result) {
 if(!result.wasResized)return undefined;
 const scale=result.originalWidth/result.width;
 return `[Image: original ${result.originalWidth}x${result.originalHeight}, displayed at ${result.width}x${result.height}. Multiply coordinates by ${scale.toFixed(2)} to map to original image.]`;
}
