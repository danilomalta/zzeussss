import {id,integer,positive,unit,array,text,requireValid,readAPI} from './operationsRead.mjs';
import {validProductionOrder} from './productionRead.mjs';
import {parseProductionQuantity,validProductionInput} from './productionMutations.mjs';
const item=v=>v&&id(v.product_id)&&unit(v.unit)&&positive(v.quantity_milli);
function resultValid(r,o,current){return r&&['result_id','order_id','reservation_id','product_id','location_id','operation_id','actor_id','device_id'].every(k=>id(r[k]))&&r.order_id===o.id&&r.product_id===o.recipe.output_product_id&&r.location_id===o.location_id&&r.unit===o.recipe.output_unit&&r.planned_milli===o.planned_output_milli&&integer(r.produced_milli)&&integer(r.shortfall_milli)&&BigInt(r.produced_milli)+BigInt(r.shortfall_milli)===BigInt(r.planned_milli)&&r.revision===o.revision&&r.status==='completed'&&r.repeated===false&&text(r.reason)&&text(r.created_at)&&(r.unit!=='unit'||r.produced_milli%1000===0)&&current?.status==='consumed'&&r.reservation_id===current.id;}
export function validExecutionTrace(v,orderID){
 if(!v||!validProductionOrder(v.order)||v.order.id!==orderID||v.order.revision>2147483647||!array(v.ingredients)||v.ingredients.length!==v.order.recipe.ingredients.length||!v.materials)return false;
 const o=v.order,m=v.materials;
 if(![m.active_count,m.consumed_count,m.released_count].every(integer)||m.active_count+m.consumed_count>1)return false;
 const current=m.current;if(m.active_count+m.consumed_count===0){if(current!==null)return false;}else{
  if(!current||!id(current.id)||current.order_id!==o.id||current.location_id!==o.location_id||current.status!==(m.active_count===1?'active':'consumed')||!id(current.created_by)||!text(current.created_at)||!text(current.updated_at)||!array(current.items)||current.items.length!==v.ingredients.length||!current.items.every(item)||new Set(current.items.map(i=>i.product_id)).size!==current.items.length)return false;
 }
 if(new Set(v.ingredients.map(i=>i?.product_id)).size!==v.ingredients.length)return false;
 for(const i of v.ingredients){const recipe=o.recipe.ingredients.find(r=>r.product_id===i?.product_id);if(!recipe||i.unit!==recipe.unit||!positive(i.planned_milli)||BigInt(i.planned_milli)!==BigInt(recipe.quantity_milli)*BigInt(o.planned_batches)||!integer(i.reserved_milli)||!integer(i.consumed_milli)||i.reserved_milli!==(m.active_count?i.planned_milli:0)||i.consumed_milli!==(m.consumed_count?i.planned_milli:0))return false;
  if(current&&!current.items.some(c=>c.product_id===i.product_id&&c.unit===i.unit&&c.quantity_milli===i.planned_milli))return false;
 }
 if(v.stages!==null){const s=v.stages;if(!s||s.order_id!==o.id||!array(s.items)||s.items.length<1||s.items.length>100||new Set(s.items.map(i=>i?.stage_id)).size!==s.items.length||!s.items.every((i,n)=>i&&id(i.stage_id)&&id(i.responsible_id)&&text(i.name)&&i.position===n+1&&['pending','running','completed'].includes(i.status)&&i.revision===({pending:1,running:2,completed:3})[i.status]))return false;}
 return o.status==='completed'?resultValid(v.result,o,current):v.result===null;
}
export function executionChoices(v){if(!validExecutionTrace(v,v?.order?.id))return [];if(v.order.status!=='approved')return [];return v.materials.current?.status==='active'?['release','consume']:v.materials.current?[]:['reserve'];}
export function completionReady(v){return validExecutionTrace(v,v?.order?.id)&&v.order.status==='approved'&&v.order.revision<2147483647&&v.materials.current?.status==='consumed'&&(v.stages===null||v.stages.items.every(i=>i.status==='completed'));}
export function prepareCompletion(v,value,reason,op,resultID){
 requireValid(completionReady(v));const produced=parseProductionQuantity(value,true);requireValid(produced<=v.order.planned_output_milli&&(v.order.recipe.output_unit!=='unit'||produced%1000===0));
 const input={operation_id:op,result_id:resultID,order_id:v.order.id,expected_revision:v.order.revision,produced_milli:produced,reason:reason.trim()};requireValid(validProductionInput('result',input));return input;
}
export function createProductionExecution(fetcher){const get=readAPI(fetcher);return {async trace(token,orderID){requireValid(id(orderID));return get(`/production/orders/${encodeURIComponent(orderID)}/trace?lot_offset=0`,token,v=>validExecutionTrace(v,orderID));}};}
