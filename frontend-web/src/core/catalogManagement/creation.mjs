import {contextOf,contextKey} from '../context74/model.mjs';
import {LocalAPIError} from '../local/localClient.mjs';
import {catalogProduct} from './model.mjs';
// The legacy creation endpoint has no operation receipt: never automatically retry.
export function createCatalogCreator({token,session,capabilities,isCurrent,send}){
 const scope=contextKey(contextOf(session,capabilities));let busy=false;
 function check(){let valid=false;try{valid=typeof token==='string'&&!!token&&!/[\r\n]/.test(token)&&isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}if(!valid)throw new LocalAPIError(401,'catalog');if(!['view_catalog','manage_stock'].every(p=>capabilities.permissions?.includes(p))||capabilities.license?.state!=='active'||!['core','inventory'].every(m=>capabilities.license.modules.includes(m)))throw new LocalAPIError(403,'catalog');}
 return {async create(input){check();if(busy)throw Error('catalogPending');const body=structuredClone(catalogProduct(input));busy=true;try{check();const result=await send(token,body);check();return result;}finally{busy=false;}}};
}
