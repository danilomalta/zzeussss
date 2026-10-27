const invalid=()=>{throw Error('productionInvalid');};
const exact=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const size=s=>new TextEncoder().encode(s).length;
const id=s=>typeof s==='string'&&s===s.trim()&&size(s)>0&&size(s)<=128&&!/[\x00\r\n]/.test(s);
export function recipeStateInput(v){
 if(!exact(v,['operation_id','recipe_id','expected_revision','status','reason'])||!id(v.operation_id)||!id(v.recipe_id)||!Number.isSafeInteger(v.expected_revision)||v.expected_revision<0||v.expected_revision>2147483646||!['active','inactive'].includes(v.status)||typeof v.reason!=='string'||v.reason!==v.reason.trim()||!v.reason||size(v.reason)>255||/[\x00\r\n]/.test(v.reason))invalid();
 return {...v};
}
export function productionRecipeState(v,recipeID){
 if(!exact(v,['recipe_id','revision','status','repeated'])||!id(recipeID)||v.recipe_id!==recipeID||!Number.isSafeInteger(v.revision)||v.revision<0||v.revision>2147483647||!['active','inactive'].includes(v.status)||typeof v.repeated!=='boolean'||v.revision===0&&v.status!=='active')invalid();
 return v;
}
export function recipeStateResult(v,b){
 productionRecipeState(v,b.recipe_id);
 if(v.revision!==b.expected_revision+1||v.status!==b.status)invalid();
 return v;
}
