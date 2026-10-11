import {LocalAPIError} from './localClient.mjs';
import {id,integer,positive,unit,readAPI,requireValid} from './operationsRead.mjs';
const ident=v=>id(v)&&new TextEncoder().encode(v).length<=128;
export function validProductUsage(v,productID,storeID,kind,offset){
 if(!['versions','orders'].includes(kind)||!v||!v.product||v.product.id!==productID||typeof v.product.name!=='string'||!unit(v.product.unit)||v.store_id!==storeID||v.kind!==kind||v.offset!==offset||v.limit!==50||!['recipe_version_count','latest_recipe_version_count','order_count'].every(k=>integer(v[k]))||v.latest_recipe_version_count>v.recipe_version_count||!Array.isArray(v.items))return false;
 const total=kind==='versions'?v.recipe_version_count:v.order_count;
 if(v.items.length!==Math.min(50,Math.max(0,total-offset))||v.has_more!==(offset+v.items.length<total))return false;
 const ids=new Set();
 for(const x of v.items){if(!x||x.kind!==kind||!ident(x.id)||ids.has(x.id)||!ident(x.recipe_id)||!ident(x.version_id)||!['ingredient','output'].includes(x.role)||!unit(x.unit)||!positive(x.quantity_milli)||x.quantity_basis!==(kind==='versions'?'per_batch':'planned_total')||x.unit_matches_catalog!==(x.unit===v.product.unit))return false;
  if(kind==='versions'){if(x.id!==x.version_id||typeof x.latest_version!=='boolean')return false;}else if(!['planned','approved','cancelled','completed'].includes(x.order_status)||!ident(x.location_id))return false;
  ids.add(x.id);
 }
 if(kind==='versions'&&v.items.filter(x=>x.latest_version).length>v.latest_recipe_version_count)return false;
 return true;
}
export function capacitySelection(locationID,versions){requireValid(ident(locationID)&&Array.isArray(versions)&&versions.length>=1&&versions.length<=20&&versions.every(ident)&&new Set(versions).size===versions.length);return {location_id:locationID,version_ids:[...versions]};}
function ratio(stock,recipe){if(stock===recipe)return [1n,1n];if((stock==='kg'&&recipe==='g')||(stock==='liter'&&recipe==='ml'))return [1000n,1n];if((stock==='g'&&recipe==='kg')||(stock==='ml'&&recipe==='liter'))return [1n,1000n];return null;}
export function validAlternatives(v,input){
 try{capacitySelection(input.location_id,input.version_ids);}catch{return false;}
 if(!v||v.location_id!==input.location_id||typeof v.measured_at!=='string'||!Number.isFinite(Date.parse(v.measured_at))||v.basis!=='local_available_balance_after_reservations'||v.alternatives_independent!==true||v.simultaneous_total_available!==false||!Array.isArray(v.alternatives)||v.alternatives.length!==input.version_ids.length)return false;
 const seen=new Set(),shared=new Map();
 for(const c of v.alternatives){
  if(!c||!input.version_ids.includes(c.version_id)||seen.has(c.version_id)||!ident(c.recipe_id)||!ident(c.output_product_id)||!positive(c.revision)||c.revision>2147483647||!unit(c.output_unit)||!positive(c.yield_per_batch_milli)||!integer(c.possible_batches)||!integer(c.possible_output_milli)||BigInt(c.possible_batches)*BigInt(c.yield_per_batch_milli)!==BigInt(c.possible_output_milli)||!Array.isArray(c.materials)||!c.materials.length||!Array.isArray(c.limiting_product_ids))return false;
  seen.add(c.version_id);const products=new Set();let minimum=BigInt(Number.MAX_SAFE_INTEGER);const limiting=[];
  for(const m of c.materials){if(!m||!ident(m.product_id)||products.has(m.product_id)||!unit(m.recipe_unit)||!unit(m.stock_unit)||!positive(m.required_milli)||!integer(m.stock_milli)||!positive(m.conversion_numerator)||!positive(m.conversion_denominator)||!integer(m.possible_batches)||typeof m.limiting!=='boolean')return false;
   products.add(m.product_id);const r=ratio(m.stock_unit,m.recipe_unit);if(!r||BigInt(m.conversion_numerator)!==r[0]||BigInt(m.conversion_denominator)!==r[1])return false;
   const possible=BigInt(m.stock_milli)*r[0]/(BigInt(m.required_milli)*r[1]);if(possible!==BigInt(m.possible_batches))return false;if(possible<minimum)minimum=possible;
   const prior=shared.get(m.product_id);if(prior&&(prior.stock_unit!==m.stock_unit||prior.stock_milli!==m.stock_milli))return false;shared.set(m.product_id,m);
  }
  if(minimum!==BigInt(c.possible_batches))return false;
  for(const m of c.materials){if(m.limiting!==(m.possible_batches===c.possible_batches))return false;if(m.limiting)limiting.push(m.product_id);}
  if(new Set(c.limiting_product_ids).size!==c.limiting_product_ids.length||c.limiting_product_ids.length!==limiting.length||!limiting.every(x=>c.limiting_product_ids.includes(x)))return false;
 }
 return true;
}
export function createCatalogPlanning(fetcher=globalThis.fetch){const get=readAPI(fetcher);return {
 async usage(token,productID,storeID,kind='versions',offset=0){requireValid(ident(productID)&&ident(storeID)&&['versions','orders'].includes(kind)&&integer(offset));return get(`/catalog/products/${encodeURIComponent(productID)}/production-usage?kind=${kind}&offset=${offset}`,token,v=>validProductUsage(v,productID,storeID,kind,offset));},
 async alternatives(token,locationID,versions){const input=capacitySelection(locationID,versions);requireValid(typeof token==='string'&&token.length>0);const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),30000);try{
  const r=await fetcher('/local/v1/production/capacity/alternatives',{method:'POST',signal:controller.signal,credentials:'omit',cache:'no-store',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(input)});
  if(!r.ok)throw new LocalAPIError(r.status,({401:'Sessão expirada.',403:'Consulta não autorizada.',404:'Versão ou local não encontrado.',409:'Saldo, conversão ou receita inconsistente.'})[r.status]||'Não foi possível consultar as alternativas.');const v=await r.json();requireValid(validAlternatives(v,input));return v;
 }catch(e){if(e instanceof LocalAPIError)throw e;throw new LocalAPIError(0,'Consulta de capacidade não confirmada. Nenhum estoque foi reservado.');}finally{clearTimeout(timer);}}
};}
