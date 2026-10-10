import {id,integer,array,requireValid,readAPI} from './operationsRead.mjs';
import {validProductState} from './catalogMutations.mjs';
const statusFor=r=>r%2?'inactive':'active';
export function validCatalogStateHistory(v,productID,offset){
 if(!id(productID)||!integer(offset)||!v||!validProductState(v.current,productID)||v.offset!==offset||v.limit!==50||!integer(v.total_count)||v.total_count!==v.current.revision||v.current.status!==statusFor(v.total_count)||typeof v.has_more!=='boolean'||!array(v.items)||v.items.length!==Math.min(50,Math.max(0,v.total_count-offset))||v.has_more!==(offset+v.items.length<v.total_count))return false;
 return new Set(v.items.map(e=>JSON.stringify([e?.device_id,e?.operation_id]))).size===v.items.length&&v.items.every((e,n)=>e&&e.revision===offset+n+1&&['store_id','operation_id','device_id','actor_id'].every(k=>id(e[k]))&&e.before_status===statusFor(e.revision-1)&&e.after_status===statusFor(e.revision)&&typeof e.reason==='string'&&e.reason.trim()===e.reason&&e.reason.length>0&&new TextEncoder().encode(e.reason).length<=255&&!/[\x00\r\n]/.test(e.reason)&&typeof e.created_at==='string'&&/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(e.created_at)&&Number.isFinite(Date.parse(e.created_at)));
}
export function createCatalogStateHistory(fetcher){const get=readAPI(fetcher);return async(token,productID,offset=0)=>{requireValid(id(productID)&&integer(offset));return get(`/catalog/products/${encodeURIComponent(productID)}/state-history?offset=${offset}`,token,v=>validCatalogStateHistory(v,productID,offset));};}
