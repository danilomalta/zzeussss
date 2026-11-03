const fail=()=>{throw Error('reviewedInvalid');};
export const reviewedStatuses=['open','submitted','approved','rejected','cancelled'];
const units=['unit','kg','g','liter','ml','meter'];
const int=(v,min=0,max=Number.MAX_SAFE_INTEGER)=>Number.isSafeInteger(v)&&v>=min&&v<=max;
const bytes=s=>new TextEncoder().encode(s).length;
const exact=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
export const reviewedID=v=>typeof v==='string'&&v===v.trim()&&bytes(v)>0&&bytes(v)<=128&&!/[\x00-\x20\x7f]/.test(v);
const reference=v=>typeof v==='string'&&v.trim().length>0&&v.length<=128;
const text=(v,max)=>typeof v==='string'&&v===v.trim()&&v.length>0&&bytes(v)<=max&&!/[\x00-\x1f\x7f]/.test(v);
const time=v=>{if(typeof v!=='string')return false;const m=/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(?:Z|([+-])(\d{2}):(\d{2}))$/.exec(v);if(!m)return false;const y=Number(m[1]),month=Number(m[2]),day=Number(m[3]),leap=y%4===0&&(y%100!==0||y%400===0);return y>=1&&month>=1&&month<=12&&day>=1&&day<=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31][month-1]&&Number(m[4])<=23&&Number(m[5])<=59&&Number(m[6])<=59&&(!m[7]||Number(m[8])<=23&&Number(m[9])<=59)&&Number.isFinite(Date.parse(v));};
const sortItems=items=>items.map(i=>({...i})).sort((a,b)=>a.product_id<b.product_id?-1:a.product_id>b.product_id?1:0);
const canonical=v=>JSON.stringify(Array.isArray(v)?v.map(o=>JSON.parse(canonical(o))):v&&typeof v==='object'?Object.fromEntries(Object.keys(v).sort().map(k=>[k,JSON.parse(canonical(v[k]))])):v);
export function reviewedQuantity(value,unit){if(!units.includes(unit)||typeof value!=='string'||value.length>24||!/^\d+(?:[.,]\d{1,3})?$/.test(value))fail();const [whole,fraction='']=value.replace(',','.').split('.'),n=BigInt(whole)*1000n+BigInt(fraction.padEnd(3,'0'));if(n>BigInt(Number.MAX_SAFE_INTEGER)||unit==='unit'&&n%1000n!==0n)fail();return Number(n);}
export function reviewedQuantityText(value,locale){if(!int(value,-Number.MAX_SAFE_INTEGER))fail();const n=BigInt(value),absolute=n<0n?-n:n,nf=new Intl.NumberFormat(locale,{maximumFractionDigits:0}),decimal=new Intl.NumberFormat(locale).formatToParts(1.1).find(p=>p.type==='decimal')?.value||'.';return (n<0n?'-':'')+nf.format(absolute/1000n)+decimal+(absolute%1000n).toString().padStart(3,'0');}
export function reviewedInput(kind,inventoryID,value){
 if(!reviewedID(inventoryID)||!value||!reviewedID(value.operation_id))fail();
 if(kind==='open'){
  if(!exact(value,['operation_id','inventory_id','location_id','title','items'])||value.inventory_id!==inventoryID||!reviewedID(value.location_id)||!text(value.title,160)||!Array.isArray(value.items)||value.items.length<1||value.items.length>200||!value.items.every(i=>exact(i,['product_id','unit'])&&reviewedID(i.product_id)&&units.includes(i.unit))||new Set(value.items.map(i=>i.product_id)).size!==value.items.length)fail();
  return {...value,items:sortItems(value.items)};
 }
 if(!int(value.expected_revision,1,Number.MAX_SAFE_INTEGER-1))fail();
 if(kind==='count'){
  if(!exact(value,['operation_id','expected_revision','items'])||!Array.isArray(value.items)||value.items.length<1||value.items.length>200||!value.items.every(i=>exact(i,['product_id','counted_milli'])&&reviewedID(i.product_id)&&int(i.counted_milli))||new Set(value.items.map(i=>i.product_id)).size!==value.items.length)fail();
  return {...value,items:sortItems(value.items)};
 }
 if(kind!=='state'||!exact(value,['operation_id','expected_revision','action','reason'])||!['submit','reopen','approve','reject','cancel'].includes(value.action)||!text(value.reason,255))fail();
 return {...value};
}
export function reviewedCommand(value,scope){if(!exact(value,['version','scope','kind','inventory_id','body','uncertain'])||value.version!==1||value.scope!==scope||typeof value.uncertain!=='boolean')fail();return {...value,body:reviewedInput(value.kind,value.inventory_id,value.body)};}
export function reviewedResult(value,command,original=true){
 const action=command.kind==='state'?command.body.action:command.kind,expected={open:'open',count:'open',submit:'submitted',reopen:'open',approve:'approved',reject:'rejected',cancel:'cancelled'}[action];
 if(!expected||!exact(value,['operation_id','inventory_id','action','revision','status','changed_lines','occurred_at','repeated'])||value.operation_id!==command.body.operation_id||value.inventory_id!==command.inventory_id||value.action!==action||value.status!==expected||value.revision!==(action==='open'?1:command.body.expected_revision+1)||!int(value.changed_lines,0,200)||(action!=='approve'&&value.changed_lines!==0)||!time(value.occurred_at)||typeof value.repeated!=='boolean'||original&&value.repeated)fail();
 return value;
}
export function reviewedSession(value){
 if(!exact(value,['inventory_id','location_id','title','status','revision','created_by','created_at','updated_at'])||!reviewedID(value.inventory_id)||!reviewedID(value.location_id)||!text(value.title,160)||!reviewedStatuses.includes(value.status)||!int(value.revision,1)||!reference(value.created_by)||!time(value.created_at)||!time(value.updated_at))fail();return value;
}
export function reviewedDetail(value,inventoryID){
 if(!exact(value,['inventory','lines'])||reviewedSession(value.inventory).inventory_id!==inventoryID||!Array.isArray(value.lines)||value.lines.length<1||value.lines.length>200||new Set(value.lines.map(i=>i?.product_id)).size!==value.lines.length)fail();
 for(const line of value.lines){
  if(!exact(line,['product_id','unit','counted_milli','previous_milli','difference_milli','counted_by','counted_at','movement_id'])||!reviewedID(line.product_id)||!units.includes(line.unit))fail();
  if(line.counted_milli===null){if(line.previous_milli!==null||line.difference_milli!==null||line.counted_by!==null||line.counted_at!==null||line.movement_id!==null||['submitted','approved','rejected'].includes(value.inventory.status))fail();}
  else{if(!int(line.counted_milli)||!int(line.previous_milli)||!int(line.difference_milli,-Number.MAX_SAFE_INTEGER)||BigInt(line.difference_milli)!==BigInt(line.counted_milli)-BigInt(line.previous_milli)||line.unit==='unit'&&line.counted_milli%1000!==0||!reference(line.counted_by)||!time(line.counted_at))fail();if(value.inventory.status==='approved'){if(line.difference_milli===0?line.movement_id!==null:!reviewedID(line.movement_id))fail();}else if(line.movement_id!==null)fail();}
 }
 return value;
}
export function reviewedCanAction(detail,actor,reviewer,action){
 const {inventory:s,lines}=reviewedDetail(detail,detail?.inventory?.inventory_id);if(!reference(actor))fail();
 if(action==='count')return s.status==='open';
 if(action==='submit')return s.status==='open'&&lines.every(l=>l.counted_milli!==null);
 if(action==='reopen')return s.status==='submitted';
 if(action==='approve'||action==='reject')return s.status==='submitted'&&reviewer&&lines.every(l=>l.counted_by!==null&&l.counted_by!==actor);
 if(action==='cancel')return ['open','submitted'].includes(s.status)&&(s.created_by===actor||reviewer);
 return false;
}
export function reviewedPage(value,offset,limit=50){if(!exact(value,['items','total','limit','offset'])||!int(offset,0,1000000)||!int(limit,1,100)||value.offset!==offset||value.limit!==limit||!int(value.total)||!Array.isArray(value.items)||value.items.length!==Math.min(limit,Math.max(0,value.total-offset)))fail();return value;}
export function reviewedList(value,offset,status=''){reviewedPage(value,offset);if(status!==''&&!reviewedStatuses.includes(status)||new Set(value.items.map(i=>i?.inventory_id)).size!==value.items.length)fail();for(const row of value.items){reviewedSession(row);if(status&&row.status!==status)fail();}return value;}
export function reviewedHistory(value,inventoryID,offset,limit=50){
 reviewedPage(value,offset,limit);if(!reviewedID(inventoryID)||new Set(value.items.map(e=>e?.operation_id)).size!==value.items.length)fail();
 for(const [index,event] of value.items.entries()){
  if(!exact(event,['revision','operation_id','actor_id','device_id','action','reason','occurred_at','request','result'])||event.revision!==offset+index+1||!reviewedID(event.operation_id)||!reference(event.actor_id)||!reference(event.device_id)||!time(event.occurred_at))fail();
  const kind=event.action==='open'||event.action==='count'?event.action:'state',body=reviewedInput(kind,inventoryID,event.request),reason=kind==='open'?body.title:kind==='count'?'':body.reason;
  reviewedResult(event.result,{kind,inventory_id:inventoryID,body});
  if(event.operation_id!==body.operation_id||event.revision!==event.result.revision||event.action!==event.result.action||event.reason!==reason||event.occurred_at!==event.result.occurred_at)fail();
 }
 return value;
}
export function reviewedOriginal(receipt,event,command,actor,device){
 reviewedResult(receipt,command);reviewedHistory({items:[event],total:event?.revision,offset:event?.revision-1,limit:1},command.inventory_id,event?.revision-1,1);
 if(event.actor_id!==actor||event.device_id!==device||canonical(event.result)!==canonical(receipt)||canonical(reviewedInput(command.kind,command.inventory_id,event.request))!==canonical(command.body))fail();return receipt;
}
