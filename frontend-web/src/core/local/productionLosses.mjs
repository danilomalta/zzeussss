import {id,integer,positive,unit,array,text,requireValid,readAPI} from './operationsRead.mjs';
import {parseProductionQuantity,validProductionInput} from './productionMutations.mjs';
export function validLoss(v){return v&&['id','result_id','product_id','created_by'].every(k=>id(v[k]))&&unit(v.unit)&&positive(v.quantity_milli)&&text(v.reason)&&!!v.reason&&text(v.created_at)&&text(v.updated_at)&&((v.status==='recorded'&&v.revision===1)||(v.status==='voided'&&v.revision===2))&&(v.unit!=='unit'||v.quantity_milli%1000===0);}
export function validLossSummary(v,resultID){
 if(!v||v.result_id!==resultID||!id(resultID)||!id(v.product_id)||!unit(v.unit)||!positive(v.planned_milli)||![v.produced_milli,v.shortfall_milli,v.recorded_loss_milli,v.unclassified_shortfall_milli].every(integer)||BigInt(v.produced_milli)+BigInt(v.shortfall_milli)!==BigInt(v.planned_milli)||BigInt(v.recorded_loss_milli)+BigInt(v.unclassified_shortfall_milli)!==BigInt(v.shortfall_milli)||!(v.items===null||array(v.items)&&v.items.length<=50))return false;
 if(v.unit==='unit'&&[v.planned_milli,v.produced_milli,v.shortfall_milli,v.recorded_loss_milli,v.unclassified_shortfall_milli].some(n=>n%1000!==0))return false;
 const items=v.items||[];if(new Set(items.map(i=>i?.id)).size!==items.length||!items.every(i=>validLoss(i)&&i.result_id===resultID&&i.product_id===v.product_id&&i.unit===v.unit&&i.quantity_milli<=v.shortfall_milli))return false;
 return items.filter(i=>i.status==='recorded').reduce((n,i)=>n+BigInt(i.quantity_milli),0n)<=BigInt(v.recorded_loss_milli);
}
export function prepareLoss(summary,value,reason,op,lossID){requireValid(validLossSummary(summary,summary?.result_id));const quantity=parseProductionQuantity(value);requireValid(quantity<=summary.unclassified_shortfall_milli&&(summary.unit!=='unit'||quantity%1000===0));const input={operation_id:op,loss_id:lossID,result_id:summary.result_id,quantity_milli:quantity,reason:reason.trim()};requireValid(validProductionInput('loss',input));return input;}
export function prepareVoidLoss(loss,reason,op){requireValid(validLoss(loss)&&loss.status==='recorded');const input={operation_id:op,loss_id:loss.id,expected_revision:loss.revision,reason:reason.trim()};requireValid(validProductionInput('loss_void',input));return input;}
export function validLossHistory(items){return array(items)&&items.length>0&&items.length<=2&&items.every((e,n)=>e&&['operation_id','device_id','actor_id'].every(k=>id(e[k]))&&e.revision===n+1&&e.kind===(n===0?'recorded':'voided')&&text(e.reason)&&!!e.reason&&text(e.created_at)&&!!e.created_at);}
export function createProductionLosses(fetcher){const get=readAPI(fetcher);return {
 async summary(token,resultID,offset=0){requireValid(id(resultID)&&integer(offset));const v=await get(`/production/results/${encodeURIComponent(resultID)}/losses?offset=${offset}`,token,v=>validLossSummary(v,resultID));return {...v,items:v.items||[]};},
 async loss(token,lossID){requireValid(id(lossID));return get(`/production/losses/${encodeURIComponent(lossID)}`,token,v=>validLoss(v)&&v.id===lossID);},
 async history(token,lossID){requireValid(id(lossID));const v=await get(`/production/losses/${encodeURIComponent(lossID)}/history`,token,v=>v&&validLossHistory(v.items));return v.items;}
 };}
