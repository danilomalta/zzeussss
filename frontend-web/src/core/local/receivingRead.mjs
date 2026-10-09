import {id,text,integer,positive,unit,array,requireValid,offsetValue,query,readAPI} from './operationsRead.mjs';
const statuses=['not_authorized','authorized','partially_received','received'];
const decimal=v=>typeof v==='string' && /^(0|[1-9][0-9]*)$/.test(v);
const order=v=>v && id(v.id) && id(v.supplier_id) && text(v.supplier_name) && text(v.created_at) && ['local_not_sent','cancelled'].includes(v.status) && statuses.includes(v.receiving_status) && array(v.items) && v.items.length===1 && v.items.every(i=>id(i.product_id) && text(i.sku) && text(i.name) && unit(i.unit) && positive(i.quantity_milli));
function ledgerItem(e,orderID,productID,units){
 if(!e || !id(e.id) || !text(e.created_at) || !['received','rejected','voided'].includes(e.kind))return false;
 const r=e.kind==='received'?e.receipt:e.kind==='rejected'?e.rejection:e.void;
 if(!r || r.order_id!==orderID || r.product_id!==productID || r.unit!==units || !id(r.actor_id) || !text(r.reason) || r.created_at!==e.created_at)return false;
 if(e.kind==='voided')return r.receipt_id===e.id && positive(r.quantity_milli) && id(r.location_id);
 if(r.id!==e.id || !id(r.delivery_reference) || !positive(r.delivered_milli))return false;
 if(e.kind==='rejected')return !e.receipt && !e.void;
 if(!positive(r.accepted_milli) || r.accepted_milli>r.delivered_milli || !id(r.location_id))return false;
 return !r.void || r.void.receipt_id===r.id && r.void.order_id===orderID && r.void.product_id===productID && r.void.unit===units && r.void.quantity_milli===r.accepted_milli && text(r.void.reason);
}
export function createReceivingRead(fetcher){
 const get=readAPI(fetcher);
 return {
  async orders(token,supplier='',product='',offset=0){requireValid((!supplier||id(supplier))&&(!product||id(product)));offsetValue(offset);
   return get(`/purchase-order-search?${query({supplier_id:supplier,product_id:product,offset})}`,token,v=>v && integer(v.total_count) && v.offset===offset && v.limit===50 && typeof v.has_more==='boolean' && (v.filters?.supplier_id||'')===supplier && (v.filters?.product_id||'')===product && array(v.items) && v.items.length<=50 && v.items.every(order) && v.items.every(o=>(!supplier||o.supplier_id===supplier)&&(!product||o.items.some(i=>i.product_id===product))));
  },
  async history(token,orderID,offset=0){requireValid(id(orderID));offsetValue(offset);
   return get(`/purchase-orders/${encodeURIComponent(orderID)}/receiving-history?${query({offset})}`,token,v=>{
    if(!v || v.order_id!==orderID || !id(v.product_id) || !unit(v.unit) || !['local_not_sent','cancelled'].includes(v.commercial_status) || !statuses.includes(v.receiving_status) || !integer(v.total_count) || v.offset!==offset || v.limit!==50 || typeof v.has_more!=='boolean' || !array(v.items) || v.items.length>50)return false;
    const t=v.totals;
    if(!t || !positive(t.planned_milli) || !['effective_accepted_milli','effective_delivered_milli','partial_rejected_milli','remaining_milli'].every(k=>integer(t[k])) || !['fully_rejected_milli_exact','voided_accepted_milli_exact','recorded_accepted_milli_exact'].every(k=>decimal(t[k])))return false;
    if(BigInt(t.effective_accepted_milli)+BigInt(t.remaining_milli)!==BigInt(t.planned_milli) || BigInt(t.effective_accepted_milli)+BigInt(t.partial_rejected_milli)!==BigInt(t.effective_delivered_milli) || BigInt(t.recorded_accepted_milli_exact)-BigInt(t.voided_accepted_milli_exact)!==BigInt(t.effective_accepted_milli))return false;
    return v.items.every(e=>ledgerItem(e,orderID,v.product_id,v.unit)) && new Set(v.items.map(e=>`${e.kind}:${e.id}`)).size===v.items.length;
   });
  },
 };
}
