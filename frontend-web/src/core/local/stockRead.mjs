import {id,text,integer,positive,unit,array,requireValid,offsetValue,query,readAPI} from './operationsRead.mjs';
const balance=(v,p,l)=>v && v.product_id===p && v.location_id===l && unit(v.unit) && integer(v.physical_milli) && integer(v.reserved_milli) && integer(v.free_milli) && BigInt(v.reserved_milli)+BigInt(v.free_milli)===BigInt(v.physical_milli);
export function createStockRead(fetcher){
 const get=readAPI(fetcher);
 return {
  async balance(token,product,location){requireValid(id(product)&&id(location));return get(`/stock/availability?${query({product_id:product,location_id:location})}`,token,v=>balance(v,product,location));},
  async reservations(token,product,location,offset=0){
   requireValid(id(product)&&id(location));offsetValue(offset);
   return get(`/stock/reservations?${query({product_id:product,location_id:location,offset})}`,token,v=>v && balance(v.balance,product,location) && integer(v.total_count) && v.offset===offset && v.limit===50 && typeof v.has_more==='boolean' && array(v.items) && v.items.length<=50 && v.items.every(r=>r && id(r.reservation_id) && id(r.order_id) && id(r.version_id) && id(r.responsible_id) && text(r.created_at) && r.unit===v.balance.unit && positive(r.quantity_milli)) && new Set(v.items.map(r=>r.reservation_id)).size===v.items.length && v.items.reduce((sum,r)=>sum+BigInt(r.quantity_milli),0n)<=BigInt(v.balance.reserved_milli));
  },
 };
}
