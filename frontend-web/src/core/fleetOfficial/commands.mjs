import {LocalAPIError} from '../local/localClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {createFleetRead} from './source.mjs';
import {fleetID,fleetSchema,fleetRecord,fleetFail} from './model.mjs';
const keys=(v,names)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===names.length&&names.every(k=>Object.hasOwn(v,k));
export function fleetEqual(a,b){if(a===null||b===null||typeof a!=='object'||typeof b!=='object')return a===b;if(Array.isArray(a)||Array.isArray(b))return Array.isArray(a)&&Array.isArray(b)&&a.length===b.length&&a.every((v,i)=>fleetEqual(v,b[i]));const ka=Object.keys(a),kb=Object.keys(b);return ka.length===kb.length&&ka.every(k=>Object.hasOwn(b,k)&&fleetEqual(a[k],b[k]));}
const text=(v,max,empty=false)=>typeof v==='string'&&v.trim()===v&&(empty||!!v)&&new TextEncoder().encode(v).length<=max&&!/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(v);
export function requestActions(record,cap){
 if(record?.kind!=='request')return [];const manager=['owner','manager'].includes(cap.role),mine=record.assigned_to===cap.identity_id,own=record.data.requester_id===cap.identity_id,state=record.state,closed=['released','cancelled'].includes(state),result=[];
 if(state==='open'&&(!record.assigned_to||mine||manager))result.push('triage');
 if(manager&&!closed&&state!=='cancelled_blocked')result.push('assign');
 if(manager&&cap.permissions?.includes('view_accounting')&&state==='triaged'&&record.assigned_to)result.push('approve');
 if((manager||mine)&&['approved','waiting_parts'].includes(state))result.push('start');
 if((manager||mine)&&state==='in_service')result.push('wait','inspect');
 if(manager&&state==='inspection')result.push('release');
 if(!closed&&state!=='cancelled_blocked'&&(manager||own&&['open','triaged'].includes(state)))result.push('cancel');
 if(manager&&state==='cancelled_blocked')result.push('clear_hold');
 if(manager&&closed)result.push('reopen');
 if(!closed&&(manager||mine||own))result.push('note');return result;
}
export function fleetCommand(value,scope){
 if(!keys(value,['version','scope','resource','before','body','uncertain'])||value.version!==1||value.scope!==scope||typeof value.uncertain!=='boolean'||!['assets','requests'].includes(value.resource))fleetFail();
 const b=value.body,create=b?.action==='create';fleetSchema(value.resource==='assets'?'AssetsCreate':create?'RequestsCreate':'RequestsAction',b);
 if(!fleetID(b.operation_id)||!fleetID(b.id)||!text(b.reason,1000)||new TextEncoder().encode(JSON.stringify(b)).length>24576||new TextEncoder().encode(JSON.stringify(b.data)).length>16384)fleetFail();
 if(create){if(value.before!==null)fleetFail();if(value.resource==='requests'){const d=b.data;if(!fleetID(d.asset_id)||!text(d.issue,1000)||!text(d.location,240)||!text(d.impact,500,true)||!text(d.reference,128,true)||!['normal','urgent','emergency'].includes(d.priority))fleetFail();}else{const d=b.data;if(!text(d.reference,120)||!text(d.name,160)||!text(d.plate,32,true)||!text(d.location,240,true)||!text(d.note,1000,true)||d.meter_basis==='none'&&d.meter_milli!==0)fleetFail();}}
 else{fleetRecord('requests',value.before);if(b.id!==value.before.id||b.expected_revision!==value.before.revision||!['triage','assign','approve','start','wait','inspect','release','cancel','clear_hold','reopen','note'].includes(b.action)||b.action!=='assign'&&b.data.assigned_to!==''||b.action!=='approve'&&b.data.quote_id!==''||b.action==='assign'&&!fleetID(b.data.assigned_to)||b.action==='approve'&&!fleetID(b.data.quote_id))fleetFail();}
 return value;
}
export function createFleetCommands({token,session,capabilities,isCurrent,storage,locks,fetcher=globalThis.fetch}){
 const ctx=contextOf(session,capabilities),scope=contextKey(ctx),key='titan-fleet-commands:v1:'+scope;
 const reads=createFleetRead({token,session,capabilities,isCurrent,fetcher});let disposed=false;
 function authorize(write=false,command){reads.assertCurrent();if(disposed)throw new LocalAPIError(401,'fleet');if(command?.body.action==='approve'&&!capabilities.permissions.includes('view_accounting'))throw new LocalAPIError(403,'fleet');
  if(write){if(capabilities.license?.state!=='active'||!['core','inventory','orders','logistics'].every(m=>capabilities.license.modules.includes(m)))throw new LocalAPIError(403,'fleet');
   if(command?.resource==='assets'&&!['owner','manager'].includes(capabilities.role)||command?.before&&!requestActions(command.before,capabilities).includes(command.body.action))throw new LocalAPIError(403,'fleet');}}
 function pending(){authorize();try{const raw=storage.getItem(key);if(raw===null)return null;if(raw.length>60000)throw Error();return fleetCommand(JSON.parse(raw),scope);}catch{throw Error('fleetStorage');}}
 function save(c){authorize(false,c);try{const raw=JSON.stringify(fleetCommand(c,scope));storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('fleetStorage');}}
 function clear(c){authorize(false,c);if(!fleetEqual(pending(),c))throw Error('fleetStorage');try{storage.removeItem(key);if(storage.getItem(key)!==null)throw Error();}catch{throw Error('fleetStorage');}}
 function peer(){authorize();try{if(storage.getItem('titan-workshop-commands:v1:'+scope)!==null)throw Error('fleetPending');}catch(e){if(e?.message==='fleetPending')throw e;throw Error('fleetStorage');}}
 async function locked(fn){if(!locks?.request)throw Error('fleetLocks');return locks.request(key,{ifAvailable:true},async lock=>{authorize();if(!lock)throw Error('fleetLocked');return fn();});}
 async function request(path,c,post=false){authorize(post,c);const ctrl=new AbortController(),timer=setTimeout(()=>ctrl.abort(),15000);try{
  const response=await fetcher('/local/v1/fleet/'+path,{method:post?'POST':'GET',credentials:'omit',cache:'no-store',headers:{Authorization:'Bearer '+token,...(post?{'Content-Type':'application/json'}:{})},...(post?{body:JSON.stringify(c.body)}:{}),signal:ctrl.signal});authorize(false,c);ctrl.signal.throwIfAborted();const raw=await response.text();authorize(false,c);ctrl.signal.throwIfAborted();if(raw.length>2000000)fleetFail();if(!response.ok)throw new LocalAPIError(response.status,'fleet');try{return JSON.parse(raw);}catch{fleetFail();}
 }finally{clearTimeout(timer);}}
 function result(raw,c){fleetSchema('Result',raw);const r=fleetRecord(c.resource,raw.record);if(raw.ack||raw.operation_id!==c.body.operation_id||r.id!==c.body.id||r.revision!==c.body.expected_revision+1)fleetFail();
  if(c.body.action==='create'){if(r.created_by!==ctx.identity_id||r.state!==(c.resource==='assets'?'available':'open'))fleetFail();const expected=c.resource==='assets'?c.body.data:{...c.body.data,requester_id:ctx.identity_id,quote_id:''};if(!fleetEqual(r.data,expected))fleetFail();}else{const b=c.before,a=c.body.action,states={triage:'triaged',assign:b.state,approve:'approved',start:'in_service',wait:'waiting_parts',inspect:'inspection',release:'released',cancel:['open','triaged'].includes(b.state)?'cancelled':'cancelled_blocked',clear_hold:'cancelled',reopen:'open',note:b.state},expected={...b.data,quote_id:a==='approve'?c.body.data.quote_id:a==='reopen'?'':b.data.quote_id},assigned=a==='triage'?ctx.identity_id:a==='assign'?c.body.data.assigned_to:a==='reopen'?'':a==='start'&&!b.assigned_to?ctx.identity_id:b.assigned_to;if(r.state!==states[a]||r.asset_id!==b.asset_id||r.assigned_to!==assigned||r.created_by!==b.created_by||r.created_at!==b.created_at||!fleetEqual(r.data,expected))fleetFail();}return raw;}
 async function receipt(c){let raw;try{raw=await request('operations/'+encodeURIComponent(c.body.operation_id),c);}catch(e){if(e instanceof LocalAPIError&&e.status===404)return null;throw e;}const out=result(raw,c),offset=Math.floor(c.body.expected_revision/50)*50,history=await reads.history(c.resource,c.body.id,offset);authorize(false,c);
  const event=history.items.find(e=>e.result.operation_id===c.body.operation_id);if(!event||event.actor_id!==ctx.identity_id||event.device_id!==ctx.device_id||event.action!==c.body.action||event.reason!==c.body.reason||!fleetEqual(event.result.record,out.record))fleetFail();return out;}
 async function preflight(c){authorize(true,c);if(c.before){const fresh=await reads.record(c.resource,c.body.id);authorize(true,c);if(!fleetEqual(fresh,c.before))throw new LocalAPIError(409,'fleet');if(['inspect','release'].includes(c.body.action)){const summary=await reads.report('workshop',c.body.id);authorize(true,c);if(summary.request_revision!==c.before.revision||summary.checklist_required&&!summary.inspection_satisfied)throw new LocalAPIError(409,'fleet');}}
  else{try{await reads.record(c.resource,c.body.id);throw new LocalAPIError(409,'fleet');}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}authorize(true,c);if(c.resource==='requests'){const asset=await reads.record('assets',c.body.data.asset_id);authorize(true,c);if(asset.state==='retired')throw new LocalAPIError(409,'fleet');}}}
 async function resolve(send,discard){let c=pending();if(!c)return null;authorize(false,c);const out=await receipt(c);
  if(out){clear(c);return {state:'confirmed',receipt:out,command:c};}
  if(discard){if(c.uncertain)throw Error('fleetUncertain');clear(c);return {state:'discarded',command:c};}
  if(!send)return {state:'missing',command:c};
  peer();await preflight(c);authorize(true,c);peer();c={...c,uncertain:true};save(c);authorize(true,c);
  result(await request(c.resource+(c.body.action==='create'?'':'/'+encodeURIComponent(c.body.id)+'/actions'),c,true),c);
  const confirmed=await receipt(c);if(!confirmed)throw Error('fleetUncertain');clear(c);return {state:'confirmed',receipt:confirmed,command:c};
 }
 return {scope,pending,assertCurrent(){authorize();},dispose(){disposed=true;reads.dispose();},get canWrite(){try{authorize(true);peer();return !!locks?.request;}catch{return false;}},
  prepare(resource,before,body){const c=fleetCommand(structuredClone({version:1,scope,resource,before,body,uncertain:false}),scope);authorize(true,c);return c;},
  async submit(value){const c=fleetCommand(structuredClone(value),scope);authorize(true,c);if(c.uncertain)fleetFail();return locked(async()=>{authorize(true,c);peer();if(pending())throw Error('fleetPending');save(c);return resolve(true,false);});},
  async recover(send=false,discard=false){if(typeof send!=='boolean'||typeof discard!=='boolean'||send&&discard)fleetFail();return locked(()=>resolve(send,discard));}
 };
}
