// Local business-workspace drafts. No network dispatch, stock posting or payment.
export const statuses=['open','triaged','approved','in_service','waiting_parts','inspection','released','cancelled'];
export const priorities=['normal','urgent','emergency'];
export const costKinds=['fuel','parts','labor','tires','insurance','documents','toll','other'];
const end=new Set(['released','cancelled']);
const assert=(ok,key='invalid')=>{if(!ok)throw Error(key);};
const text=(v,max=500)=>typeof v==='string'&&v.trim().length>0&&v.length<=max&&!/[\x00-\x08\x0b\x0c\x0e-\x1f]/.test(v);
const integer=v=>Number.isSafeInteger(v)&&v>=0;
function time(at){assert(typeof at==='string'&&Number.isFinite(Date.parse(at)));return at;}
export function maintenanceAccess(cap){return !!cap&&cap.license?.state==='active'&&cap.license.modules?.includes('logistics')&&cap.permissions?.includes('view_orders');}
function actorCheck(actor){assert(text(actor?.id,128)&&actor.id===actor.cap?.identity_id&&maintenanceAccess(actor.cap),'permission');}
const manager=actor=>['owner','manager'].includes(actor.cap.role);
export function cents(value){assert(typeof value==='string'&&/^\d{1,12}([.,]\d{1,2})?$/.test(value),'amount');const [a,b='']=value.replace(',','.').split('.');const n=BigInt(a)*100n+BigInt(b.padEnd(2,'0'));assert(n<=BigInt(Number.MAX_SAFE_INTEGER),'amount');return Number(n);}
export function exactSum(values){const total=values.reduce((n,v)=>{assert(integer(v),'amount');return n+BigInt(v);},0n);assert(total<=BigInt(Number.MAX_SAFE_INTEGER),'amount');return Number(total);}
export function emptyMaintenance(){return {version:1,equipment:[],requests:[],costs:[],profiles:[]};}
function unique(rows){return new Set(rows.map(v=>v.id)).size===rows.length;}
export function validMaintenance(v){
 try{assert(v?.version===1);for(const key of ['equipment','requests','costs','profiles'])assert(Array.isArray(v[key])&&v[key].length<=500&&unique(v[key]));
  for(const e of v.equipment)assert(text(e.id,128)&&text(e.name,120)&&text(e.reference,120)&&['machine','forklift','other'].includes(e.kind)&&text(e.location,240)&&integer(e.meter)&&integer(e.nextService));
  for(const r of v.requests){assert(text(r.id,128)&&text(r.assetId,150)&&text(r.issue)&&text(r.location,240)&&typeof r.impact==='string'&&r.impact.length<=240&&priorities.includes(r.priority)&&statuses.includes(r.status)&&typeof r.blocks==='boolean'&&text(r.requester,128)&&typeof r.assigned==='string'&&r.assigned.length<=128&&integer(r.revision)&&r.revision>0&&Array.isArray(r.history)&&r.history.length===r.revision&&r.history.length<=200);time(r.createdAt);time(r.updatedAt);assert(typeof r.delivery==='string'&&r.delivery.length<=120);assert(r.history.every((h,i)=>h.revision===i+1&&text(h.actor,128)&&text(h.note)&&statuses.includes(h.status)&&Number.isFinite(Date.parse(h.at))));}
  for(const c of v.costs)assert(text(c.id,128)&&text(c.assetId,150)&&costKinds.includes(c.kind)&&integer(c.amount)&&c.amount>0&&text(c.note)&&text(c.actor,128)&&Number.isFinite(Date.parse(c.at)));
  for(const p of v.profiles)assert(text(p.id,150)&&integer(p.acquisition)&&integer(p.residual)&&p.residual<=p.acquisition&&Number.isSafeInteger(p.months)&&p.months>0&&p.months<=1200&&integer(p.elapsed)&&p.elapsed<=p.months&&integer(p.revenue)&&integer(p.distance)&&integer(p.hours)&&integer(p.downtime)&&integer(p.estimatedLoss)&&Number.isFinite(Date.parse(p.at)));
  return true;
 }catch{return false;}
}
function stateCheck(state){assert(validMaintenance(state),'corrupt');}
export function addEquipment(state,input,actor){stateCheck(state);actorCheck(actor);assert(manager(actor),'permission');assert(state.equipment.length<500,'limit');assert(text(input.id,128)&&text(input.name,120)&&text(input.reference,120)&&text(input.location,240)&&['machine','forklift','other'].includes(input.kind)&&integer(input.meter)&&integer(input.nextService));assert(!state.equipment.some(v=>v.id===input.id||v.reference.trim().toLowerCase()===input.reference.trim().toLowerCase()),'duplicate');return {...state,equipment:[...state.equipment,structuredClone(input)]};}
export function openRequest(state,input,actor,assetIds,at){stateCheck(state);actorCheck(actor);time(at);assert(state.requests.length<500,'limit');assert(text(input.id,128)&&!state.requests.some(v=>v.id===input.id),'duplicate');assert(assetIds.includes(input.assetId),'asset');assert(text(input.issue)&&text(input.location,240)&&priorities.includes(input.priority)&&typeof input.blocks==='boolean'&&typeof input.impact==='string'&&input.impact.length<=240&&typeof input.delivery==='string'&&input.delivery.length<=120);
 const r={...structuredClone(input),requester:actor.id,assigned:'',status:'open',revision:1,createdAt:at,updatedAt:at,history:[{revision:1,actor:actor.id,note:input.issue,status:'open',at}]};return {...state,requests:[...state.requests,r]};}
export function changeRequest(state,id,expected,action,note,actor,at){stateCheck(state);actorCheck(actor);time(at);const r=state.requests.find(v=>v.id===id);assert(r,'missing');assert(r.revision===expected,'stale');assert(text(note),'reason');assert(r.history.length<200,'limit');assert(Date.parse(at)>=Date.parse(r.updatedAt),'invalid');const privileged=manager(actor),assigned=r.assigned===actor.id;let status=r.status,owner=r.assigned;
 if(action==='triage'){assert(r.status==='open','transition');assert(!owner||assigned||privileged,'permission');status='triaged';owner=owner||actor.id;}
 else if(action==='approve'){assert(privileged,'permission');assert(r.status==='triaged','transition');status='approved';}
 else if(action==='start'){assert(privileged||assigned,'permission');assert(r.status==='approved'||r.status==='waiting_parts','transition');status='in_service';}
 else if(action==='wait'){assert(privileged||assigned,'permission');assert(r.status==='in_service','transition');status='waiting_parts';}
 else if(action==='inspect'){assert(privileged||assigned,'permission');assert(r.status==='in_service','transition');status='inspection';}
 else if(action==='release'){assert(privileged,'permission');assert(r.status==='inspection','transition');status='released';}
 else if(action==='cancel'){assert(privileged||(r.requester===actor.id&&['open','triaged'].includes(r.status)),'permission');assert(!end.has(r.status),'transition');status='cancelled';}
 else if(action==='reopen'){assert(privileged,'permission');assert(end.has(r.status),'transition');status='open';owner='';}
 else if(action==='note'){assert(!end.has(r.status),'transition');assert(privileged||assigned||r.requester===actor.id,'permission');}
 else throw Error('transition');
 const next={...r,status,assigned:owner,revision:r.revision+1,updatedAt:at,history:[...r.history,{revision:r.revision+1,actor:actor.id,note,status,at}]};return {...state,requests:state.requests.map(v=>v.id===id?next:v)};
}
export function addCost(state,input,actor,assetIds,at){stateCheck(state);actorCheck(actor);assert(manager(actor)&&actor.cap.permissions.includes('view_accounting'),'permission');time(at);assert(state.costs.length<500,'limit');assert(text(input.id,128)&&!state.costs.some(v=>v.id===input.id),'duplicate');assert(assetIds.includes(input.assetId),'asset');assert(costKinds.includes(input.kind)&&integer(input.amount)&&input.amount>0,'amount');assert(text(input.note),'reason');return {...state,costs:[...state.costs,{...structuredClone(input),actor:actor.id,at}]};}
export function setProfile(state,input,actor,assetIds,at){stateCheck(state);actorCheck(actor);assert(manager(actor)&&actor.cap.permissions.includes('view_accounting'),'permission');assert(assetIds.includes(input.id),'asset');time(at);const profiles=[...state.profiles.filter(v=>v.id!==input.id),{...structuredClone(input),at}];const next={...state,profiles};assert(validMaintenance(next),'amount');return next;}
export function assetStatus(state,id,base='available',running=false){stateCheck(state);const active=state.requests.filter(r=>r.assetId===id&&!end.has(r.status));if(active.some(r=>['in_service','waiting_parts','inspection'].includes(r.status)))return 'maintenance';if(active.some(r=>r.blocks))return 'blocked';if(base==='maintenance')return 'maintenance';if(running)return 'operating';if(active.length)return 'review';return 'available';}
export function costSummary(state,id,legacy=[]){stateCheck(state);const operating=exactSum([...legacy,...state.costs.filter(c=>c.assetId===id).map(c=>c.amount)]),p=state.profiles.find(p=>p.id===id);if(!p)return {operating,profile:null};const base=BigInt(p.acquisition-p.residual),depreciation=Number(base*BigInt(p.elapsed)/BigInt(p.months));const known=exactSum([operating,depreciation]);return {operating,profile:{...p,depreciation,bookEstimate:p.acquisition-depreciation,contribution:p.revenue-known,costPerKm:p.distance?{numerator:operating,denominator:p.distance}:null,costPerHour:p.hours?{numerator:operating,denominator:p.hours}:null}};}
