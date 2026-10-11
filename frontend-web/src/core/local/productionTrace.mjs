import {id,integer,requireValid,readAPI} from './operationsRead.mjs';
import {validExecutionTrace} from './productionExecution.mjs';
import {validLot} from './productionLots.mjs';
import {validLotQuality,sameLotMetadata} from './productionQuality.mjs';
export function validProductionTrace(v,orderID,offset=0){
 if(!integer(offset)||!validExecutionTrace(v,orderID))return false;
 if(v.order.status!=='completed')return v.losses===null&&v.lots===null;
 if(v.stages&&v.stages.items.some(s=>s.status!=='completed'))return false;
 const r=v.result,l=v.losses,p=v.lots;
 if(!l||![l.recorded_milli,l.unclassified_milli].every(integer)||BigInt(l.recorded_milli)+BigInt(l.unclassified_milli)!==BigInt(r.shortfall_milli))return false;
 if(!p||![p.produced_milli,p.assigned_milli,p.unassigned_milli,p.total_count,p.offset].every(integer)||p.produced_milli!==r.produced_milli||BigInt(p.assigned_milli)+BigInt(p.unassigned_milli)!==BigInt(p.produced_milli)||p.offset!==offset||typeof p.has_more!=='boolean'||!Array.isArray(p.items)||p.items.length!==Math.min(50,Math.max(0,p.total_count-offset))||p.has_more!==(offset+p.items.length<p.total_count)||new Set(p.items.map(i=>i?.lot?.id)).size!==p.items.length)return false;
 if(r.unit==='unit'&&[l.recorded_milli,l.unclassified_milli,p.assigned_milli,p.unassigned_milli].some(n=>n%1000!==0))return false;
 let assigned=0n;
 for(const i of p.items){if(!i||!validLot(i.lot)||i.lot.result_id!==r.result_id||i.lot.product_id!==r.product_id||i.lot.unit!==r.unit||i.lot.quantity_milli>r.produced_milli||!validLotQuality(i.quality,i.lot.id)||i.quality.lot_status!==i.lot.status||i.quality.latest&&!sameLotMetadata(i.lot,i.quality.latest.lot_snapshot))return false;if(i.lot.status==='recorded')assigned+=BigInt(i.lot.quantity_milli);}
 return assigned<=BigInt(p.assigned_milli);
}
export function createProductionTrace(fetcher){const get=readAPI(fetcher);return {async trace(token,orderID,offset=0){requireValid(id(orderID)&&new TextEncoder().encode(orderID).length<=128&&integer(offset));return get(`/production/orders/${encodeURIComponent(orderID)}/trace?lot_offset=${offset}`,token,v=>validProductionTrace(v,orderID,offset));}};}
