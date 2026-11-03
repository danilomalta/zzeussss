import {createStockReports} from '../stockReports/source.mjs';
import {reportID} from '../stockReports/model.mjs';
import {sourceAllowed} from './model.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {LocalAPIError} from '../local/localClient.mjs';
function access(context){
 const {token,session,capabilities,isCurrent,signal}=context;
 if(typeof token!=='string'||!token||/[\r\n]/.test(token))throw new LocalAPIError(401,'permission');
 const scope=contextKey(contextOf(session,capabilities));
 function check(){signal.throwIfAborted();if(typeof isCurrent!=='function'||!isCurrent()||contextKey(contextOf(session,capabilities))!==scope)throw new LocalAPIError(401,'permission');if(!sourceAllowed('stock',capabilities))throw new LocalAPIError(403,'permission');}
 check();return check;
}
export async function sheetStockLocations(context,fetcher=globalThis.fetch){
 const check=access(context);const result=await fetcher('/local/v1/locations',{method:'GET',credentials:'omit',cache:'no-store',headers:{Authorization:'Bearer '+context.token},signal:context.signal});check();
 const raw=await result.text();check();if(!result.ok)throw new LocalAPIError(result.status,'stock');if(raw.length>1000000)throw Error('unavailable');
 const value=JSON.parse(raw),items=value?.items===null?[]:value?.items;
 if(!Array.isArray(items)||items.length>500||new Set(items.map(i=>i?.ID)).size!==items.length||items.some(i=>!reportID(i?.ID)||typeof i.name!=='string'||!i.name.trim()||i.name.length>255||!['shelf','backroom','receiving','production'].includes(i.kind)))throw Error('unavailable');
 return items.map(i=>({id:i.ID,name:i.name}));
}
// Physical rows only; never infer global availability or valuation. Pagination
// is bounded and each row retains the server's own observation time.
export async function sheetStockSnapshot(context,filters,fetcher=globalThis.fetch){
 const check=access(context);if(!reportID(filters?.location_id)||typeof filters.q!=='string'||filters.q.length>240)throw Error('unavailable');
 const {location_id:location,q}=structuredClone(filters),needle=q.toLocaleLowerCase();
 const source=createStockReports({...context,fetcher,isCurrent:()=>{try{check();return true;}catch{return false;}}});
 const rows=[],seen=new Set();let scanned=0,total=0;
 for(let offset=0;offset<500;offset+=50){
  check();const page=await source.positions(location,'',offset,false,context.signal);check();
  if(offset&&page.total!==total)throw Error('unavailable');total=page.total;
  for(const item of page.items){if(seen.has(item.product_id))throw Error('unavailable');seen.add(item.product_id);scanned++;
   if(!needle||[item.product_id,item.sku,item.name].some(v=>v.toLocaleLowerCase().includes(needle)))rows.push([item.product_id,item.sku,item.name,item.unit,location,item.physical_milli,item.reserved_milli,item.free_milli,page.measured_at]);
  }
  if(scanned>=total)break;
 }
 check();return {columns:['stock_product_id','sku','product','unit','stock_location_id','stock_physical_milli','stock_reserved_milli','stock_free_milli','stock_measured_at'],rows,partial:scanned<total,mock:false,at:Date.now()};
}
