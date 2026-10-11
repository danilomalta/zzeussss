import {id,text,integer,positive,unit,array,requireValid,offsetValue,query,readAPI} from './operationsRead.mjs';
const ingredient=v=>v && id(v.product_id) && unit(v.unit) && positive(v.quantity_milli);
const version=v=>v && id(v.recipe_id) && id(v.version_id) && positive(v.revision) && text(v.name) && id(v.output_product_id) && unit(v.output_unit) && positive(v.yield_milli) && array(v.ingredients) && v.ingredients.length>0 && v.ingredients.every(ingredient);
const order=v=>v && id(v.id) && id(v.version_id) && id(v.location_id) && id(v.responsible_id) && positive(v.planned_batches) && positive(v.planned_output_milli) && positive(v.revision) && text(v.created_at) && ['planned','approved','cancelled','completed'].includes(v.status) && version(v.recipe) && v.recipe.version_id===v.version_id && BigInt(v.planned_batches)*BigInt(v.recipe.yield_milli)===BigInt(v.planned_output_milli);
export function productionOrderFilters(value={}){requireValid(value&&typeof value==='object'&&!Array.isArray(value)&&[Object.prototype,null].includes(Object.getPrototypeOf(value))&&Object.keys(value).every(k=>['version_id','location_id','responsible_id'].includes(k)));const out={};for(const k of ['version_id','location_id','responsible_id']){const v=value[k]??'';requireValid(typeof v==='string'&&(v===''||id(v)&&new TextEncoder().encode(v).length<=128));if(v)out[k]=v;}return out;}
export function createProductionRead(fetcher) {
  const get=readAPI(fetcher);
  return {
    async versions(token,offset=0,recipeID='') {
      requireValid(recipeID==='' || id(recipeID));offsetValue(offset);
      const r=await get(`/production/recipe-versions?${query({limit:50,offset,recipe_id:recipeID})}`,token,v=>v && (v.items===null || array(v.items) && v.items.length<=50 && v.items.every(version) && (!recipeID || v.items.every(i=>i.recipe_id===recipeID))));
      return r.items || [];
    },
    async orders(token,status='',offset=0,filters={}) {
      requireValid(['','planned','approved','cancelled','completed'].includes(status));offsetValue(offset);const selected=productionOrderFilters(filters);
      return get(`/production/order-search?${query({status,offset,...selected})}`,token,v=>v && integer(v.total_count) && v.limit===50 && typeof v.has_more==='boolean' && v.filters?.offset===offset && (v.filters.status || '')===status && ['version_id','location_id','responsible_id'].every(k=>(v.filters[k]||'')===(selected[k]||'')) && array(v.items) && v.items.length===Math.min(50,Math.max(0,v.total_count-offset)) && v.has_more===(offset+v.items.length<v.total_count) && new Set(v.items.map(i=>i.id)).size===v.items.length && v.items.every(order) && v.items.every(i=>new Set(i.recipe.ingredients.map(x=>x.product_id)).size===i.recipe.ingredients.length) && (!status || v.items.every(i=>i.status===status)) && Object.entries(selected).every(([k,value])=>v.items.every(i=>i[k]===value)));
    },
    async capacity(token,versionID,locationID) {
      requireValid(id(versionID) && id(locationID));
      return get(`/production/capacity?${query({version_id:versionID,location_id:locationID})}`,token,v=>v && v.location_id===locationID && text(v.measured_at) && v.basis==='local_available_balance_after_reservations' && v.alternatives_independent===true && v.simultaneous_total_available===false && array(v.alternatives) && v.alternatives.length===1 && v.alternatives.every(c=>c.version_id===versionID && positive(c.revision) && unit(c.output_unit) && positive(c.yield_per_batch_milli) && integer(c.possible_batches) && integer(c.possible_output_milli) && BigInt(c.possible_batches)*BigInt(c.yield_per_batch_milli)===BigInt(c.possible_output_milli) && array(c.limiting_product_ids) && c.limiting_product_ids.every(id) && array(c.materials) && c.materials.length>0 && c.materials.every(m=>id(m.product_id) && unit(m.recipe_unit) && unit(m.stock_unit) && positive(m.required_milli) && integer(m.stock_milli) && positive(m.conversion_numerator) && positive(m.conversion_denominator) && integer(m.possible_batches) && typeof m.limiting==='boolean')));
    },
  };
}

export {version as validRecipeVersion,order as validProductionOrder};
