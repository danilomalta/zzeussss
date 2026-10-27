import {fleetID,fleetSchema,fleetRecord,fleetFail} from './model.mjs';
const text=(v,max,empty=false)=>typeof v==='string'&&v.trim()===v&&(empty||!!v)&&new TextEncoder().encode(v).length<=max&&!/[\u0000-\u0008\u000b-\u001f\u007f\uD800-\uDFFF]/u.test(v);
export function providerActions(record,cap){if(record?.kind!=='provider'||!['owner','manager'].includes(cap.role))return [];return ['update',...(record.state==='active'?['disable']:record.state==='inactive'?['enable']:[])];}
export function quoteCents(value){if(typeof value!=='string')fleetFail();const m=/^(0|[1-9]\d*)(?:[.,](\d{1,2}))?$/.exec(value);if(!m)fleetFail();const n=BigInt(m[1])*100n+BigInt((m[2]||'').padEnd(2,'0'));if(n>BigInt(Number.MAX_SAFE_INTEGER))fleetFail();return Number(n);}
export function quoteTotal(data){let sum=0n;for(const k of ['parts_cents','labor_cents','other_cents']){if(!Number.isSafeInteger(data[k])||data[k]<0)fleetFail();sum+=BigInt(data[k]);}if(sum>BigInt(Number.MAX_SAFE_INTEGER))fleetFail();return Number(sum);}
export function providerQuoteCommand(c){const {body:b,before,resource}=c,create=b?.action==='create';
 fleetSchema(resource==='providers'?(create?'ProvidersCreate':'ProvidersAction'):'QuotesCreate',b);
 if(create){if(before!==null)fleetFail();}else{fleetRecord('providers',before);if(b.id!==before.id||b.expected_revision!==before.revision||!['update','enable','disable'].includes(b.action))fleetFail();}
 if(resource==='providers'){
  if(create||b.action==='update'){const d=b.data;fleetSchema('Provider',d);if(!text(d.name,160)||!text(d.contact,240)||!text(d.services,500)||!text(d.note,1000,true))fleetFail();}
  else if(Object.keys(b.data).length)fleetFail();
 }else{
  const d=b.data;fleetRecord('requests',c.quoteRequest);if(!fleetID(d.asset_id)||!fleetID(d.request_id)||d.provider_id!==''&&!fleetID(d.provider_id)||!text(d.note,1000)||c.quoteRequest.id!==d.request_id||c.quoteRequest.asset_id!==d.asset_id||c.quoteRequest.state!=='triaged')fleetFail();quoteTotal(d);
 }
 return c;
}
