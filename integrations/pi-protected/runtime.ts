// SPDX-License-Identifier: AGPL-3.0-or-later
import {dirname,join,resolve} from 'node:path';
import {mkdirSync} from 'node:fs';
import {homedir} from 'node:os';
import {createAgentSession,createAgentSessionRuntime,ModelRuntime,SettingsManager,SessionManager,createEventBus,createExtensionRuntime,InteractiveMode,runRpcMode,initTheme} from '@earendil-works/pi-coding-agent';
import {loadExtensionFromFactory} from '@corvint/pi-internal/core/extensions/loader.js';
import {FileAuthStorageBackend} from '@corvint/pi-internal/core/auth-storage.js';
import {loadAnthropicOAuth,loadOpenAICodexOAuth,loadGitHubCopilotOAuth,loadOpenRouterOAuth,loadKimiCodingOAuth,loadXaiOAuth,loadRadiusOAuth} from '@corvint/pi-ai-internal/auth/oauth/load.js';
import * as bedrock from '@corvint/pi-ai-internal/api/bedrock-converse-stream.js';
import {setBedrockProviderModule} from '@corvint/pi-ai-internal/api/bedrock-converse-stream.lazy.js';
import registerProtected from './extension.mjs';
import {createQualifiedRunner} from './qualified-runner.mjs';
import {createRunner} from '../pi/runtime.js';
import {createCredentials,parseArguments,readSettings} from './settings.mjs';

class FixedResources {
 result;
 constructor(readonly cwd:string,readonly consumer:string){}
 async reload(){
  const runtime=createExtensionRuntime();
  const installed=/^\/Library\/CorvintAuthority\/versions\/[0-9a-f]{64}\/pi-protected$/.test(process.execPath);
  const extension=await loadExtensionFromFactory(pi=>registerProtected(pi,{ordinary:createRunner({binary:this.consumer}),qualified:createQualifiedRunner({binary:this.consumer,consumerSHA256:CORVINT_CONSUMER_SHA256}),installed,version:'0.85.1'}),this.cwd,createEventBus(),runtime,'<compiled-corvint>');
  this.result={extensions:[extension],errors:[],runtime};
 }
 getExtensions(){return this.result}
 getSkills(){return {skills:[],diagnostics:[]}}
 getPrompts(){return {prompts:[],diagnostics:[]}}
 getThemes(){return {themes:[],diagnostics:[]}}
 getAgentsFiles(){return {agentsFiles:[]}}
 getSystemPrompt(){return undefined}
 getSystemPromptSource(){return undefined}
 getAppendSystemPrompt(){return []}
 getAppendSystemPromptSources(){return []}
 extendResources(){throw Error('executable-resources-closed')}
}

async function main(){
 const cli=parseArguments(process.argv.slice(2));
 // Fail before accepting a prompt if any built-in provider's sealed code is unavailable.
 const oauth=await Promise.all([loadAnthropicOAuth(),loadOpenAICodexOAuth(),loadGitHubCopilotOAuth(),loadOpenRouterOAuth(),loadKimiCodingOAuth(),loadXaiOAuth(),loadRadiusOAuth({name:'Runtime code check',gateway:'https://runtime.invalid'})]);
 if(oauth.some(flow=>typeof flow.login!=='function'||typeof flow.refresh!=='function'||typeof flow.toAuth!=='function')||typeof bedrock.stream!=='function'||typeof bedrock.streamSimple!=='function')throw Error('provider-code-unavailable');
 setBedrockProviderModule(bedrock);
 const dataDir=resolve(cli.dataDir??join(homedir(),'.corvint','pi-protected'));
 mkdirSync(dataDir,{recursive:true,mode:0o700});
 const settings={...readSettings(join(dataDir,'settings.json')),...Object.fromEntries(Object.entries(cli).filter(([k])=>['provider','model','thinkingLevel'].includes(k)))};
 const credentials=createCredentials(new FileAuthStorageBackend(join(dataDir,'auth.json')));
 const consumer=join(dirname(process.execPath),'corvint');
 const factory=async options=>{
  const loader=new FixedResources(options.cwd,consumer);await loader.reload();
  const models=await ModelRuntime.create({credentials,modelsPath:null,allowModelNetwork:false,refreshOnCreate:false});
  if(settings.endpoint)models.registerProvider(settings.provider,{baseUrl:settings.endpoint.baseUrl,api:settings.endpoint.api,models:[{id:settings.model,name:settings.model,reasoning:false,input:settings.endpoint.input??['text'],cost:{input:0,output:0,cacheRead:0,cacheWrite:0},contextWindow:settings.endpoint.contextWindow,maxTokens:settings.endpoint.maxTokens}]});
  const model=settings.provider&&settings.model?models.getModel(settings.provider,settings.model):undefined;
  if(settings.provider&&settings.model&&!model)throw Error('protected-model-unavailable');
  const manager=SettingsManager.inMemory({defaultProvider:settings.provider,defaultModel:settings.model,defaultThinkingLevel:settings.thinkingLevel??'medium',theme:settings.theme??'dark',quietStartup:true});
  const result=await createAgentSession({cwd:options.cwd,agentDir:dataDir,modelRuntime:models,model,thinkingLevel:settings.thinkingLevel,settingsManager:manager,sessionManager:options.sessionManager,resourceLoader:loader,sessionStartEvent:options.sessionStartEvent});
  return {...result,services:{cwd:options.cwd,agentDir:dataDir,modelRuntime:models,settingsManager:manager,resourceLoader:loader,diagnostics:[]},diagnostics:[]};
 };
 const sessions=join(dataDir,'sessions');mkdirSync(sessions,{recursive:true,mode:0o700});
 const sessionManager=cli.session?SessionManager.open(resolve(cli.session),sessions):SessionManager.create(process.cwd(),sessions);
 const host=await createAgentSessionRuntime(factory,{cwd:sessionManager.getCwd(),agentDir:dataDir,sessionManager});
 if(cli.mode==='rpc')await runRpcMode(host);
 else {initTheme(settings.theme??'dark',false);await new InteractiveMode(host,{verbose:false}).run();}
}
main().catch(()=>{process.stderr.write('Protected Pi unavailable: runtime-or-configuration-invalid.\n');process.exitCode=1});
