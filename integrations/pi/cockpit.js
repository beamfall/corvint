// SPDX-License-Identifier: AGPL-3.0-or-later

const VIEWS = [
 ['evidence','Evidence','context-core',()=> 'context',text=>({task:text||'current repository evidence'})],
 ['changes','Changes','core',text=>text?'impact':'affected',text=>text?{paths:text.split(/\s+/).filter(Boolean)}:{}],
 ['tasks','Tasks','tasks','queue',()=>({})],
 ['verification','Verification','core',()=> 'prove',text=>({task:text||'current repository verification evidence'})],
 ['gaps','Gaps','core',()=> 'query',text=>({task:text||'unresolved evidence gaps in the current repository',limit:5})],
];
const EDIT_TOOLS=new Set(['edit','write','apply_patch']);
const MAX_ROWS=400, MAX_LINE_BYTES=8192;

export function escapeTerminal(value) {
 let text=typeof value==='string'?value:JSON.stringify(value,null,2);
 if(text===undefined)text=String(value);
 return text.replace(/[\x00-\x09\x0b-\x1f\x7f-\x9f]/g,c=>`\\u${c.charCodeAt(0).toString(16).padStart(4,'0')}`);
}

function boundedLines(value) {
 const text=escapeTerminal(value);
 const rows=text.split(/\r?\n/).slice(0,MAX_ROWS).map(line=>Buffer.byteLength(line)>MAX_LINE_BYTES?Buffer.from(line).subarray(0,MAX_LINE_BYTES).toString()+'…':line);
 if(text.split(/\r?\n/).length>MAX_ROWS)rows.push('… output bounded by cockpit');
 return rows.length?rows:['(no output)'];
}

function resultLines(result) {
 if(!result||typeof result!=='object')return ['UNCERTAIN — malformed service result'];
 const lines=boundedLines(result.operation??'unknown').slice(0,1).map(line=>`operation: ${line}`);
 if(result.receipt)lines.push('receipt:',...boundedLines(result.receipt));
 if(result.fault)lines.push(...boundedLines(result.fault).slice(0,4).map(line=>`UNCERTAIN — ${line}`));
 if(result.exitCode!==undefined)lines.push(...boundedLines(result.exitCode).slice(0,1).map(line=>`exit: ${line}`));
 if(result.raw?.stdout)lines.push('',...boundedLines(result.raw.stdout));
 if(result.raw?.stderr)lines.push('stderr:',...boundedLines(result.raw.stderr));
 if(result.raw&&typeof result.raw==='object'&&!Object.hasOwn(result.raw,'stdout')&&!Object.hasOwn(result.raw,'stderr'))lines.push('native result:',...boundedLines(result.raw));
 return lines;
}

function scope(ctx) {
 let session='unknown';
 try{session=String(ctx.sessionManager.getSessionId())}catch{}
 return `${ctx.cwd}\u0000${session}`;
}

function canonicalIdentity(value,seen=new Set()) {
 if(value===null||typeof value==='string'||typeof value==='boolean')return JSON.stringify(value);
 if(typeof value==='number'&&Number.isFinite(value))return JSON.stringify(value);
 if(!value||typeof value!=='object'||seen.has(value))throw Error('invalid identity');
 seen.add(value);
 let encoded;
 if(Array.isArray(value))encoded=`[${value.map(item=>canonicalIdentity(item,seen)).join(',')}]`;
 else encoded=`{${Object.keys(value).sort().map(key=>`${JSON.stringify(key)}:${canonicalIdentity(value[key],seen)}`).join(',')}}`;
 seen.delete(value);
 if(Buffer.byteLength(encoded)>16384)throw Error('identity too large');
 return encoded;
}

async function loadView(view,text,ctx,services) {
 const [,label,owner,operationFor,input]=view, service=owner==='context-core'?(services.context?.read?services.context:services.core):services[owner],operation=typeof operationFor==='function'?operationFor(text):operationFor;
 if(!service||typeof service.read!=='function')return {label,lines:[`UNCERTAIN — ${owner} read service unavailable`],fault:'service-unavailable'};
 try {
  const result=await service.read(operation,input(text),ctx);
  return {label,lines:resultLines(result),result,fault:result?.fault};
 } catch {
  return {label,lines:['UNCERTAIN — read service failed'],fault:'read-failed'};
 }
}

function structuredSnapshot(state,reason) {
 const view=(state.sourceRequest||state.sourceError)?['source-expansion','Source expansion']:VIEWS[state.selected];
 return {profile:'corvint-cockpit/0',reason,view:view[0],label:view[1],stale:state.stale,source:'Corvint Core/Tasks read services',authority:'display only; test output and displayed text do not establish verification or authority',result:state.loaded?.result??null,fault:state.loaded?.fault};
}

export function createCockpitComponent({state,refresh,done,tui,theme,uiKit}) {
 state.requestGeneration??=0;
 const {matchesKey,truncateToWidth,visibleWidth}=uiKit;
 const fit=(text,width)=>truncateToWidth(text,Math.max(1,width),'…');
 const component={
  render(width) {
   const selected=(state.sourceRequest||state.sourceError)?['source-expansion','Source expansion']:VIEWS[state.selected], narrow=width<64;
   const title=theme.fg('accent',theme.bold('Corvint cockpit'));
   const stateWord=state.loading?'LOADING':state.stale?'STALE':'CURRENT';
   const status=`${stateWord} · ${selected[1]} · ${state.reason}`;
   const tabs=(state.sourceRequest||state.sourceError)?'Source expansion (←/→ returns to views)':narrow?`${state.selected+1}/${VIEWS.length} ${selected[1]}`:VIEWS.map((v,i)=>i===state.selected?`[${v[1]}]`:v[1]).join('  ');
   const body=state.loaded?.lines??['Press r to read this view.'];
   const room=Math.max(3,Math.min(18,body.length));
   const start=Math.min(state.offset,Math.max(0,body.length-room));
   const lines=[title,theme.fg(state.stale?'warning':'success',status),theme.fg('muted',tabs),''];
   for(const line of body.slice(start,start+room))lines.push(fit(line,width));
   lines.push('',theme.fg('dim',narrow?'←/→ view · ↑/↓ scroll · r refresh · q close':'←/→ or 1–5 view · ↑/↓ PgUp/PgDn scroll · r refresh · q/Esc close'));
   return lines.map(line=>visibleWidth(line)>width?fit(line,width):line);
  },
  invalidate(){},
  handleInput(data) {
   if(matchesKey(data,'escape')||data==='q'){done(structuredSnapshot(state,'closed'));return;}
   let changed=false;
   if(matchesKey(data,'left')){state.requestGeneration++;state.loading=false;state.sourceRequest=undefined;state.sourceError=undefined;state.selected=(state.selected+VIEWS.length-1)%VIEWS.length;state.offset=0;state.loaded=undefined;state.stale=true;changed=true;}
   else if(matchesKey(data,'right')){state.requestGeneration++;state.loading=false;state.sourceRequest=undefined;state.sourceError=undefined;state.selected=(state.selected+1)%VIEWS.length;state.offset=0;state.loaded=undefined;state.stale=true;changed=true;}
   else if(/^[1-5]$/.test(data)){state.requestGeneration++;state.loading=false;state.sourceRequest=undefined;state.sourceError=undefined;state.selected=Number(data)-1;state.offset=0;state.loaded=undefined;state.stale=true;changed=true;}
   else if(matchesKey(data,'up')){state.offset=Math.max(0,state.offset-1);changed=true;}
   else if(matchesKey(data,'down')){state.offset++;changed=true;}
   else if(matchesKey(data,'pageUp')){state.offset=Math.max(0,state.offset-10);changed=true;}
   else if(matchesKey(data,'pageDown')){state.offset+=10;changed=true;}
   else if(data==='r'&&!state.loading){refresh().then(()=>tui.requestRender());changed=true;}
   if(changed)tui.requestRender();
  }
 };
 return component;
}

async function defaultUiKit() { return import('@earendil-works/pi-tui'); }

export function registerCockpit(pi,{core,tasks,context,notice=()=>{},identity=scope,uiKit:providedUiKit}={}) {
 const states=new Map();let closed=false,generation=0;
 const identityFor=async ctx=>canonicalIdentity(await identity(ctx));
 const invalidate=reason=>{generation++;for(const state of states.values()){state.stale=true;state.loading=false;state.reason=reason;state.loaded=undefined;state.offset=0;}};
 const clear=()=>{generation++;states.clear();};
 const close=()=>{closed=true;clear();};
 const events=[['session_start','session'],['session_tree','tree'],['session_compact','compaction'],['session_shutdown','shutdown']];
 for(const [name,reason] of events)pi.on(name,async()=>{if(reason==='shutdown')close();else invalidate(reason);});
 pi.on('tool_result',async event=>{if(EDIT_TOOLS.has(event?.toolName)||event?.details?.corvint?.changedPaths?.length)invalidate('observed edit');});
 pi.registerCommand('corvint',{description:'Open the read-only Corvint evidence cockpit. Source: /corvint source packet-1 {"result":0,"evidence":0,"lines":"1:20"}',handler:async(text='',ctx)=>{
  if(closed)return;
  let key;
  try{key=await identityFor(ctx)}catch{
   const state={selected:0,offset:0,stale:true,loading:false,reason:'identity unavailable',requestGeneration:0,loaded:{label:'Identity',lines:['UNCERTAIN — canonical worktree identity unavailable'],fault:'identity-unavailable'}};
   const snapshot=structuredSnapshot(state,'identity-unavailable');
   if(ctx.mode==='tui'){const uiKit=providedUiKit??await defaultUiKit();await ctx.ui.custom((tui,theme,_keys,done)=>createCockpitComponent({state,refresh:async()=>snapshot,done,tui,theme,uiKit}));return;}
   if(ctx.mode==='rpc'&&ctx.hasUI){ctx.ui.setStatus('corvint-cockpit','UNCERTAIN: identity unavailable');ctx.ui.notify('Corvint cockpit: canonical identity unavailable','warning');}else notice(ctx,'identity-unavailable');
   if(typeof pi.sendMessage==='function')pi.sendMessage({customType:'corvint-cockpit',content:[{type:'text',text:JSON.stringify(snapshot)}],display:true,details:snapshot},{triggerTurn:false});
   return;
  }
  const prior=states.get(key), state=prior??{selected:0,offset:0,stale:true,loading:false,reason:'not read',requestGeneration:0,loaded:undefined};
  states.set(key,state);
  const services={core,tasks,context};
  const sourceMatch=text.trim().match(/^source\s+(\S+)\s+(.+)$/s);
  state.requestGeneration++;state.sourceRequest=undefined;state.sourceError=undefined;
  if(sourceMatch) {
   try {
    const selector=JSON.parse(sourceMatch[2]),keys=Object.keys(selector??{});
    if(!/^packet-[1-9][0-9]*$/.test(sourceMatch[1])||Buffer.byteLength(sourceMatch[2])>8192||!selector||typeof selector!=='object'||Array.isArray(selector)||keys.some(key=>!['result','evidence','lines','requirement'].includes(key))||!Number.isInteger(selector.result)||selector.result<0||!Number.isInteger(selector.evidence)||selector.evidence<0||('lines'in selector&&typeof selector.lines!=='string')||('requirement'in selector&&typeof selector.requirement!=='string'))throw Error();
    state.sourceRequest={packetId:sourceMatch[1],selector:sourceMatch[2],maxBytes:8192};
   }
   catch {state.sourceError='invalid-source-selector';}
  }
  const refresh=async()=>{
   const started=generation,startedRequest=state.requestGeneration;let startedIdentity;
   try{startedIdentity=await identityFor(ctx)}catch{state.loaded={label:'Identity',lines:['UNCERTAIN — canonical worktree identity unavailable'],fault:'identity-unavailable'};state.stale=true;state.loading=false;state.reason='identity unavailable';return structuredSnapshot(state,'identity-unavailable');}
   if(startedIdentity!==key){state.loaded=undefined;state.stale=true;state.loading=false;state.reason='identity changed before refresh';return structuredSnapshot(state,'discarded-stale-result');}
   state.loading=true;state.reason='explicit refresh';let loaded;
   if(state.sourceError)loaded={label:'Source expansion',lines:['UNCERTAIN — source selector must be a JSON object'],fault:state.sourceError};
   if(state.sourceRequest) {
    if(typeof context?.expand!=='function')loaded={label:'Source expansion',lines:['UNCERTAIN — validated source expansion unavailable'],fault:'source-expansion-unavailable'};
    else try {const result=await context.expand(state.sourceRequest,ctx);loaded={label:'Source expansion',lines:resultLines(result),result,fault:result?.fault};}
    catch{loaded={label:'Source expansion',lines:['UNCERTAIN — source expansion failed'],fault:'source-expansion-failed'};}
   } else if(!state.sourceError)loaded=await loadView(VIEWS[state.selected],text.trim(),ctx,services);
   let currentIdentity;
   try{currentIdentity=await identityFor(ctx)}catch{if(startedRequest===state.requestGeneration){state.loaded={label:'Identity',lines:['UNCERTAIN — canonical worktree identity unavailable'],fault:'identity-unavailable'};state.stale=true;state.loading=false;state.reason='identity unavailable';}return structuredSnapshot(state,'identity-unavailable');}
   if(startedRequest!==state.requestGeneration)return structuredSnapshot(state,'discarded-stale-result');
   if(started!==generation||startedIdentity!==currentIdentity){state.loaded=undefined;state.stale=true;state.loading=false;state.reason='invalidated during refresh';return structuredSnapshot(state,'discarded-stale-result');}
   state.loaded=loaded;state.stale=false;state.loading=false;state.offset=0;
   return structuredSnapshot(state,'refreshed');
  };
  if(ctx.mode==='tui') {
   const uiKit=providedUiKit??await defaultUiKit();
   await ctx.ui.custom((tui,theme,_keys,done)=>createCockpitComponent({state,refresh,done,tui,theme,uiKit}));
   return;
  }
  const snapshot=await refresh();
  if(ctx.mode==='rpc'&&ctx.hasUI){ctx.ui.setStatus('corvint-cockpit',`${snapshot.label}: ${snapshot.fault?'UNCERTAIN':'read'}`);ctx.ui.notify(`${snapshot.label}: ${snapshot.fault?'UNCERTAIN — '+snapshot.fault:'read result available'}`,snapshot.fault?'warning':'info');}
  else notice(ctx,'cockpit-ui-unavailable');
  if(typeof pi.sendMessage==='function')pi.sendMessage({customType:'corvint-cockpit',content:[{type:'text',text:JSON.stringify(snapshot)}],display:true,details:snapshot},{triggerTurn:false});
 }});
 return {invalidate,clear,close};
}
