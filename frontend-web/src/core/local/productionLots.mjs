import {id,integer,positive,unit,array,requireValid,readAPI} from './operationsRead.mjs';
const bytes=v=>typeof v==='string'?new TextEncoder().encode(v).length:Infinity;
export function calendarDate(v){if(typeof v!=='string'||!/^\d{4}-\d{2}-\d{2}$/.test(v))return false;const [y,m,d]=v.split('-').map(Number);const leap=y%4===0&&(y%100!==0||y%400===0);return y>=1&&m>=1&&m<=12&&d>=1&&d<=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31][m-1];}
export function recordedTime(v){return typeof v==='string'&&/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(v)&&calendarDate(v.slice(0,10))&&Number(v.slice(11,13))<24&&Number(v.slice(14,16))<60&&Number(v.slice(17,19))<60;}
export const reasonText=v=>typeof v==='string'&&v.trim()===v&&bytes(v)>=1&&bytes(v)<=255;
export function validLot(v){return !!v&&['id','result_id','product_id','created_by'].every(k=>id(v[k]))&&unit(v.unit)&&positive(v.quantity_milli)&&(v.unit!=='unit'||v.quantity_milli%1000===0)&&typeof v.code==='string'&&v.code.trim()===v.code&&bytes(v.code)>=1&&bytes(v.code)<=64&&!/\p{Cc}/u.test(v.code)&&calendarDate(v.manufactured_on)&&(v.expires_on===''||calendarDate(v.expires_on)&&v.expires_on>=v.manufactured_on)&&reasonText(v.reason)&&recordedTime(v.created_at)&&recordedTime(v.updated_at)&&((v.status==='recorded'&&v.revision===1&&v.created_at===v.updated_at)||(v.status==='voided'&&v.revision===2));}
export function validLotSummary(v,resultID){
 if(!v||!id(resultID)||v.result_id!==resultID||!id(v.product_id)||!unit(v.unit)||![v.produced_milli,v.assigned_milli,v.unassigned_milli].every(integer)||BigInt(v.assigned_milli)+BigInt(v.unassigned_milli)!==BigInt(v.produced_milli)||!(v.items===null||array(v.items)&&v.items.length<=50))return false;
 if(v.unit==='unit'&&[v.produced_milli,v.assigned_milli,v.unassigned_milli].some(n=>n%1000))return false;
 const items=v.items||[];return new Set(items.map(l=>l?.id)).size===items.length&&items.every(l=>validLot(l)&&l.result_id===resultID&&l.product_id===v.product_id&&l.unit===v.unit&&l.quantity_milli<=v.produced_milli)&&items.filter(l=>l.status==='recorded').reduce((s,l)=>s+BigInt(l.quantity_milli),0n)<=BigInt(v.assigned_milli);
}
export function createProductionLots(fetcher){const get=readAPI(fetcher);return {
 async summary(token,resultID,offset=0){requireValid(id(resultID)&&integer(offset));const v=await get(`/production/results/${encodeURIComponent(resultID)}/lots?offset=${offset}`,token,v=>validLotSummary(v,resultID));return {...v,items:v.items||[]};},
 async lot(token,lotID){requireValid(id(lotID));return get(`/production/lots/${encodeURIComponent(lotID)}`,token,v=>validLot(v)&&v.id===lotID);}
};}
