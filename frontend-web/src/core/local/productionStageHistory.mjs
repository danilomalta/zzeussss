import {id,integer,positive,text,array,requireValid,readAPI} from './operationsRead.mjs';
import {validProductionInput} from './productionMutations.mjs';
export function validStageHistoryEvent(e,orderID){
 if(!e||!positive(e.sequence)||!['operation_id','device_id','actor_id'].every(k=>id(e[k]))||!text(e.reason)||!text(e.created_at)||!e.created_at||!e.result||!['configured','running','completed'].includes(e.kind))return false;
 const kind=e.kind==='configured'?'stage_plan':'stage_state',i=e.request,r=e.result;
 if(!validProductionInput(kind,i)||i.operation_id!==e.operation_id||i.order_id!==orderID||i.reason!==e.reason||r.order_id!==orderID||r.status!==e.kind||r.repeated!==false)return false;
 return kind==='stage_plan'?r.revision===1&&!r.stage_id:r.stage_id===i.stage_id&&r.revision===i.expected_revision+1&&i.status===e.kind;
}
export function createProductionStageHistory(fetcher){const get=readAPI(fetcher);return {async history(token,orderID,offset=0){
 requireValid(id(orderID)&&integer(offset));const v=await get(`/production/orders/${encodeURIComponent(orderID)}/stages/history?offset=${offset}`,token,v=>v&&array(v.items)&&v.items.length<=50&&v.items.every((e,n)=>validStageHistoryEvent(e,orderID)&&(n===0||e.sequence>v.items[n-1].sequence)));return v.items;
 }};}
