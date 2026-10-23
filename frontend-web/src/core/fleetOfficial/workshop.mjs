import {LocalAPIError} from '../local/localClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {createFleetRead} from './source.mjs';
import {fleetEqual} from './commands.mjs';
import {fleetID,fleetStamp,fleetSchema,fleetRecord,fleetFail} from './model.mjs';
const names={templates:'Templates',requirements:'Requirements',inspections:'Inspections',labor:'Labor'};
const keys=(v,n)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===n.length&&n.every(k=>Object.hasOwn(v,k));
const text=(v,max,empty=false)=>typeof v==='string'&&v.trim()===v&&(empty||!!v)&&new TextEncoder().encode(v).length<=max&&!/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f\uD800-\uDFFF]/u.test(v);
const revision=v=>Number.isSafeInteger(v)&&v>0&&v<Number.MAX_SAFE_INTEGER;
export function workshopTemplate(v){fleetSchema('CheckTemplate',v);if(!text(v.name,160)||!text(v.note,1000,true)||v.items.length<1||v.items.length>50)fleetFail();const seen=new Set();for(const item of v.items){if(!fleetID(item.id)||seen.has(item.id)||!text(item.label,240))fleetFail();seen.add(item.id);}return v;}
export function workshopSeconds(start,end,now=Infinity){if(![start,end].every(v=>fleetStamp(v)&&!v.includes('.')&&Number(v.slice(0,4))>=2000&&Number(v.slice(0,4))<=2200))fleetFail();const seconds=(Date.parse(end)-Date.parse(start))/1000;if(!Number.isSafeInteger(seconds)||seconds<1||seconds>57600||Date.parse(end)>now)fleetFail();return seconds;}
export function workshopPassed(items,answers){if(!Array.isArray(answers)||answers.length!==items.length)fleetFail();let passed=true;for(let i=0;i<items.length;i++){const item=items[i],a=answers[i];if(!a||!keys(a,['item_id','outcome','note'])||a.item_id!==item.id||!['pass','fail','not_applicable'].includes(a.outcome)||!text(a.note,1000,a.outcome==='pass'))fleetFail();if(a.outcome==='fail'||item.required&&a.outcome!=='pass')passed=false;}return passed;}
export function workshopActions(before,request,cap){const manager=['owner','manager'].includes(cap.role);if(before?.kind==='template')return manager&&before.state==='active'?['disable']:[];if(before?.kind!=='labor'||!request)return [];const result=[];if(manager&&before.state==='declared')result.push('confirm','reject');if(before.state!=='voided'&&(manager||before.data.worker_id===cap.identity_id&&before.state==='declared'&&['in_service','waiting_parts','inspection'].includes(request.state)))result.push('void');return result;}
export function workshopCommand(c,scope){
 if(!keys(c,['version','scope','resource','before','references','body','uncertain'])||c.version!==1||c.scope!==scope||!Object.hasOwn(names,c.resource)||typeof c.uncertain!=='boolean'||!keys(c.references,['request','template','binding','epoch']))fleetFail();
 const b=c.body,ref=c.references,create=b?.action==='create';fleetSchema(names[c.resource]+(create?'Create':'Action'),b);
 if(!fleetID(b.id)||!fleetID(b.operation_id)||!text(b.reason,1000)||new TextEncoder().encode(JSON.stringify(b)).length>24576||new TextEncoder().encode(JSON.stringify(b.data)).length>16384)fleetFail();
 if(create){if(c.before!==null)fleetFail();}else{fleetRecord('workshop/'+c.resource,c.before);if(b.id!==c.before.id||b.expected_revision!==c.before.revision)fleetFail();}
 if(c.resource==='templates'){if(ref.request!==null||ref.template!==null||ref.binding!==null||ref.epoch!==0)fleetFail();if(create)workshopTemplate(b.data);return c;}
 fleetRecord('requests',ref.request);if(!revision(ref.request.revision)||!Number.isSafeInteger(ref.epoch)||ref.epoch<0)fleetFail();
 if(c.resource==='requirements'){
  fleetRecord('workshop/templates',ref.template);workshopTemplate(ref.template.data);
  if(ref.template.state!=='active'||b.id!==ref.request.id||b.data.request_id!==ref.request.id||b.data.expected_request_revision!==ref.request.revision||b.data.template_id!==ref.template.id||ref.epoch!==0||!['open','triaged','approved'].includes(ref.request.state))fleetFail();
  if(ref.binding!==null){fleetRecord('workshop/requirements',ref.binding);if(!fleetEqual(ref.binding,c.before))fleetFail();}else if(c.before!==null)fleetFail();
 }else{
  if(ref.template!==null)fleetFail();
  if(create&&(b.data.request_id!==ref.request.id||b.data.expected_request_revision!==ref.request.revision))fleetFail();
  if(!create&&c.before.request_id!==ref.request.id)fleetFail();
  if(c.resource==='inspections'){
   fleetRecord('workshop/requirements',ref.binding);const policy=ref.binding.data;workshopTemplate(policy.template);
   if(ref.binding.id!==ref.request.id||ref.binding.request_id!==ref.request.id||b.data.expected_binding_revision!==ref.binding.revision||!revision(ref.epoch)||ref.epoch>ref.request.revision||!['in_service','inspection'].includes(ref.request.state)||!text(b.data.note,1000))fleetFail();
   workshopPassed(policy.template.items,b.data.answers);
  }else{if(ref.binding!==null||ref.epoch!==0)fleetFail();if(create){if(!text(b.data.description,1000)||!['in_service','waiting_parts','inspection'].includes(ref.request.state))fleetFail();workshopSeconds(b.data.started_at,b.data.ended_at);}}
 }
 return c;
}
export function createWorkshopCommands({token,session,capabilities,isCurrent,storage,locks,fetcher=globalThis.fetch}){
 const ctx=contextOf(session,capabilities),scope=contextKey(ctx),peerKey='titan-fleet-commands:v1:'+scope,key='titan-workshop-commands:v1:'+scope,reads=createFleetRead({token,session,capabilities,isCurrent,fetcher});let disposed=false,loadVersion=0;
 const manager=()=>['owner','manager'].includes(capabilities.role);
 function authorize(write=false,c){reads.assertCurrent();if(disposed)throw new LocalAPIError(401,'workshop');if(!write)return;
  if(capabilities.license?.state!=='active'||!['core','inventory','orders','logistics'].every(m=>capabilities.license.modules.includes(m)))throw new LocalAPIError(403,'workshop');
  if(c){if(['templates','requirements'].includes(c.resource)&&!manager()||c.before&&['templates','labor'].includes(c.resource)&&!workshopActions(c.before,c.references.request,capabilities).includes(c.body.action))throw new LocalAPIError(403,'workshop');if(['inspections','labor'].includes(c.resource)&&!c.before&&!manager()&&c.references.request.assigned_to!==ctx.identity_id)throw new LocalAPIError(403,'workshop');}}
 function pending(){authorize();try{const raw=storage.getItem(key);if(raw===null)return null;if(raw.length>150000)throw Error();return workshopCommand(JSON.parse(raw),scope);}catch{throw Error('fleetStorage');}}
 function peer(){authorize();try{if(storage.getItem(peerKey)!==null)throw Error('fleetPending');}catch(e){if(e?.message==='fleetPending')throw e;throw Error('fleetStorage');}}
 function save(c){authorize();try{const raw=JSON.stringify(workshopCommand(c,scope));storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('fleetStorage');}}
 function clear(c){authorize();if(!fleetEqual(pending(),c))throw Error('fleetStorage');try{storage.removeItem(key);if(storage.getItem(key)!==null)throw Error();}catch{throw Error('fleetStorage');}}
 async function locked(fn){if(!locks?.request)throw Error('fleetLocks');return locks.request(peerKey,{ifAvailable:true},async lock=>{authorize();if(!lock)throw Error('fleetLocked');return fn();});}
 async function request(path,c,post=false){authorize(post,c);const ctrl=new AbortController(),timer=setTimeout(()=>ctrl.abort(),15000);try{const response=await fetcher('/local/v1/fleet/workshop/'+path,{method:post?'POST':'GET',credentials:'omit',cache:'no-store',headers:{Authorization:'Bearer '+token,...(post?{'Content-Type':'application/json'}:{})},...(post?{body:JSON.stringify(c.body)}:{}),signal:ctrl.signal});authorize();ctrl.signal.throwIfAborted();const raw=await response.text();authorize();ctrl.signal.throwIfAborted();if(raw.length>2000000)fleetFail();if(!response.ok)throw new LocalAPIError(response.status,'workshop');try{return JSON.parse(raw);}catch{fleetFail();}}finally{clearTimeout(timer);}}
 function result(v,c){fleetRecord('workshop/'+c.resource,v);const b=c.body,ref=c.references;if(v.id!==b.id||v.operation_id!==b.operation_id||v.revision!==b.expected_revision+1||v.actor_id!==ctx.identity_id||v.device_id!==ctx.device_id)fleetFail();
  let data,state,requestID='';
  if(c.resource==='templates'){data=c.before?c.before.data:b.data;state=c.before?'inactive':'active';}
  if(c.resource==='requirements'){data={...b.data,template:ref.template.data};state='required';requestID=ref.request.id;}
  if(c.resource==='inspections'){const passed=workshopPassed(ref.binding.data.template.items,b.data.answers);data={...b.data,template_id:ref.binding.data.template_id,epoch_revision:ref.epoch,passed};state=passed?'passed':'failed';requestID=ref.request.id;}
  if(c.resource==='labor'){data=c.before?c.before.data:{...b.data,worker_id:ctx.identity_id,seconds:workshopSeconds(b.data.started_at,b.data.ended_at)};state=c.before?{confirm:'confirmed',reject:'rejected',void:'voided'}[b.action]:'declared';requestID=ref.request.id;}
  if(v.state!==state||v.request_id!==requestID||!fleetEqual(v.data,data))fleetFail();return v;
 }
 async function receipt(c){let raw;try{raw=await request('operations/'+encodeURIComponent(c.body.operation_id),c);}catch(e){if(e instanceof LocalAPIError&&e.status===404)return null;throw e;}const out=result(raw,c),offset=Math.floor(c.body.expected_revision/50)*50,history=await reads.history('workshop/'+c.resource,c.body.id,offset);authorize();const event=history.items.find(e=>e.result.operation_id===c.body.operation_id);if(!event||event.action!==c.body.action||event.reason!==c.body.reason||!fleetEqual({...event.result,repeated:false},{...out,repeated:false}))fleetFail();return out;}
 async function epoch(record){let value=0;const first=await reads.history('requests',record.id);authorize();if(first.total>1000)throw new LocalAPIError(422,'workshop');if(first.total!==record.revision)throw new LocalAPIError(409,'workshop');const seen=new Set();const consume=(items,offset)=>{for(let i=0;i<items.length;i++){const e=items[i];if(e.result.record.revision!==offset+i+1||seen.has(e.result.operation_id))throw new LocalAPIError(409,'workshop');seen.add(e.result.operation_id);if(['start','assign','reopen'].includes(e.action))value=Math.max(value,e.result.record.revision);}};consume(first.items,0);for(let offset=50;offset<first.total;offset+=50){const page=await reads.history('requests',record.id,offset);authorize();if(page.total!==first.total)throw new LocalAPIError(409,'workshop');consume(page.items,offset);}if(!value)throw new LocalAPIError(409,'workshop');return value;}
 async function same(resource,before){const current=await reads.record(resource,before.id);authorize();if(!fleetEqual(current,before))throw new LocalAPIError(409,'workshop');}
 async function absent(resource,id){try{await reads.record(resource,id);throw new LocalAPIError(409,'workshop');}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}authorize();}
 async function preflight(c){authorize(true,c);peer();if(c.resource==='labor'&&!c.before)workshopSeconds(c.body.data.started_at,c.body.data.ended_at,Date.now());const ref=c.references;if(c.before)await same('workshop/'+c.resource,c.before);else await absent('workshop/'+c.resource,c.body.id);authorize(true,c);
  if(ref.request)await same('requests',ref.request);if(ref.template)await same('workshop/templates',ref.template);
  if(ref.binding)await same('workshop/requirements',ref.binding);else if(c.resource==='requirements')await absent('workshop/requirements',ref.request.id);
  if(c.resource==='inspections'&&await epoch(ref.request)!==ref.epoch)throw new LocalAPIError(409,'workshop');authorize(true,c);peer();}
 async function resolve(send,discard){let c=pending();if(!c)return null;const out=await receipt(c);if(out){clear(c);return {state:'confirmed',receipt:out,command:c};}if(discard){if(c.uncertain)throw Error('fleetUncertain');clear(c);return {state:'discarded',command:c};}if(!send)return {state:'missing',command:c};await preflight(c);authorize(true,c);c={...c,uncertain:true};save(c);authorize(true,c);peer();result(await request(c.resource+(c.body.action==='create'?'':'/'+encodeURIComponent(c.body.id)+'/actions'),c,true),c);const confirmed=await receipt(c);if(!confirmed)throw Error('fleetUncertain');clear(c);return {state:'confirmed',receipt:confirmed,command:c};}
 return {scope,pending,assertCurrent(){authorize();},dispose(){disposed=true;loadVersion++;reads.dispose();},get canWrite(){try{authorize(true);peer();return !!locks?.request;}catch{return false;}},
  async context(resource,requestID,templateID=''){authorize();const version=++loadVersion;if(!['requirements','inspections','labor'].includes(resource)||!fleetID(requestID)||resource==='requirements'&&!fleetID(templateID))fleetFail();const ref={request:await reads.record('requests',requestID),template:null,binding:null,epoch:0};authorize();if(resource==='requirements')ref.template=await reads.record('workshop/templates',templateID);if(['requirements','inspections'].includes(resource)){try{ref.binding=await reads.record('workshop/requirements',requestID);}catch(e){if(resource==='inspections'||!(e instanceof LocalAPIError)||e.status!==404)throw e;}}if(ref.template)workshopTemplate(ref.template.data);if(ref.binding)workshopTemplate(ref.binding.data.template);if(resource==='inspections')ref.epoch=await epoch(ref.request);authorize();if(version!==loadVersion)throw Error('fleetSuperseded');return ref;},
  prepare(resource,before,references,body){const c=workshopCommand(structuredClone({version:1,scope,resource,before,references,body,uncertain:false}),scope);authorize(true,c);if(resource==='labor'&&!before)workshopSeconds(body.data.started_at,body.data.ended_at,Date.now());return c;},
  async submit(value){const c=workshopCommand(structuredClone(value),scope);authorize(true,c);if(c.uncertain)fleetFail();return locked(async()=>{authorize(true,c);peer();if(pending())throw Error('fleetPending');save(c);return resolve(true,false);});},
  async recover(send=false,discard=false){if(typeof send!=='boolean'||typeof discard!=='boolean'||send&&discard)fleetFail();return locked(()=>resolve(send,discard));}
 };
}
