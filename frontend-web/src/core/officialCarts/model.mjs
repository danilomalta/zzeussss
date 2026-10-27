import {journalID as id} from '../cashJournal/model.mjs';
import {keys,equal} from '../cashBags/model.mjs';
import {isCents,lineCents} from '../local/posModel.mjs';
export {id,keys,equal};
export const fail=()=>{throw Error('officialCartInvalid');};
const text=(v,n)=>typeof v==='string'&&!!v&&v===v.trim()&&new TextEncoder().encode(v).length<=n&&!/[\x00-\x1f\x7f]/.test(v);
const time=v=>typeof v==='string'&&/^\d{4}-\d\d-\d\dT.*(?:Z|[+-]\d\d:\d\d)$/.test(v)&&Number.isFinite(Date.parse(v));
export const states=['suspended','claimed','completed','cancelled'];
export const cartItems=cart=>cart.items.map(({product_id,location_id,quantity_milli})=>({product_id,location_id,quantity_milli}));
export const draftItems=items=>items.map(({product,location_id,quantity_milli})=>({product_id:product.id,location_id,quantity_milli}));
export function itemInputs(items){if(!Array.isArray(items)||items.length<1||items.length>500||!items.every(i=>keys(i,['product_id','location_id','quantity_milli'])&&id(i.product_id)&&id(i.location_id)&&Number.isSafeInteger(i.quantity_milli)&&i.quantity_milli>0))fail();return items;}
export function quoteItems(items){
 if(!Array.isArray(items)||items.length<1||items.length>500)fail();let total=0n;for(const i of items){if(!keys(i,['product_id','location_id','quantity_milli','unit','unit_price_cents','total_cents','available_milli'])||!id(i.product_id)||!id(i.location_id)||!Number.isSafeInteger(i.quantity_milli)||i.quantity_milli<1||!text(i.unit,32)||i.unit==='unit'&&i.quantity_milli%1000!==0||!isCents(i.unit_price_cents)||!isCents(i.total_cents)||!isCents(i.available_milli)||lineCents(i.unit_price_cents,i.quantity_milli)!==i.total_cents)fail();total+=BigInt(i.total_cents);}if(total<1n||total>BigInt(Number.MAX_SAFE_INTEGER))fail();return Number(total);
}
export function cartRecord(c){
 if(!keys(c,['id','revision','state','label','sale_id','payment_id','cash_session_id','items','total_cents','created_at','updated_at','sale_operation_id','received_cents','change_cents'])||!id(c.id)||!Number.isSafeInteger(c.revision)||c.revision<1||c.revision>4096||!states.includes(c.state)||!text(c.label,160)||!id(c.sale_id)||!id(c.payment_id)||!time(c.created_at)||!time(c.updated_at)||quoteItems(c.items)!==c.total_cents)fail();
 if(c.state==='completed'){if(!id(c.sale_operation_id)||!id(c.cash_session_id)||!isCents(c.received_cents)||c.received_cents<c.total_cents||!isCents(c.change_cents)||BigInt(c.received_cents)-BigInt(c.total_cents)!==BigInt(c.change_cents))fail();}
 else if(c.sale_operation_id!==''||c.received_cents!==0||c.change_cents!==0||(c.state==='claimed'?!id(c.cash_session_id):c.cash_session_id!==''))fail();return c;
}
export function actions(c){cartRecord(c);if(c.revision>=4096)return [];return c.state==='suspended'?['save','claim','cancel']:c.state==='claimed'?['release','finalize_cash']:[];}
export function cartPage(v,state,offset){if(!keys(v,['items','total','limit','offset','has_more'])||!Array.isArray(v.items)||!isCents(v.total)||v.limit!==50||v.offset!==offset||v.items.length!==Math.min(50,Math.max(0,v.total-offset))||v.has_more!==(offset+v.items.length<v.total))fail();const seen=new Set();for(const c of v.items){cartRecord(c);if(seen.has(c.id)||state&&c.state!==state)fail();seen.add(c.id);}return v;}
export function result(r){if(!keys(r,['operation_id','action','reason','repeated','cart'])||!id(r.operation_id)||!['save','claim','release','cancel','finalize_cash'].includes(r.action)||typeof r.repeated!=='boolean'||(r.action==='save'?r.reason!=='':!text(r.reason,255)))fail();const c=cartRecord(r.cart);if(c.state!==({save:'suspended',claim:'claimed',release:'suspended',cancel:'cancelled',finalize_cash:'completed'})[r.action]||r.action==='finalize_cash'&&c.sale_operation_id!==r.operation_id)fail();return r;}
export function command(c,scope){
 if(!keys(c,['version','scope','kind','id','body','before','origin','uncertain'])||c.version!==1||c.scope!==scope||!id(c.id)||typeof c.uncertain!=='boolean'||typeof c.origin!=='string'||c.origin!==''&&!id(c.origin))fail();const b=c.body;
 if(c.kind==='save'){
  if(!keys(b,['operation_id','cart_id','expected_revision','label','items'])||b.cart_id!==c.id||!text(b.label,160))fail();itemInputs(b.items);
 }else if(c.kind==='action'){
  if(!keys(b,['operation_id','expected_revision','action','cash_session_id','received_cents','reason'])||!['claim','release','cancel','finalize_cash'].includes(b.action)||!text(b.reason,255)||!isCents(b.received_cents)||(b.action==='claim'?!id(b.cash_session_id):b.cash_session_id!=='')||b.action!=='finalize_cash'&&b.received_cents!==0||c.origin!=='')fail();
 }else fail();
 if(!id(b.operation_id)||!Number.isSafeInteger(b.expected_revision)||b.expected_revision<0||b.expected_revision>=4096)fail();
 if(c.before===null){if(c.kind!=='save'||b.expected_revision!==0)fail();}
 else{const old=cartRecord(c.before);if(old.id!==c.id||old.revision!==b.expected_revision||!actions(old).includes(c.kind==='save'?'save':b.action)||c.kind==='action'&&b.action==='finalize_cash'&&b.received_cents<old.total_cents)fail();}if(new TextEncoder().encode(JSON.stringify(c.body)).length>(c.kind==='save'?128*1024:4096)||new TextEncoder().encode(JSON.stringify(c)).length>1024*1024)fail();return c;
}
export function receipt(r,c){result(r);const v=r.cart,b=c.body;if(r.operation_id!==b.operation_id||r.action!==(c.kind==='save'?'save':b.action)||r.reason!==(c.kind==='save'?'':b.reason)||v.id!==c.id||v.revision!==b.expected_revision+1)fail();
 if(c.before&&!['id','sale_id','payment_id','created_at'].every(k=>v[k]===c.before[k]))fail();
 if(c.kind==='save'){if(v.label!==b.label||!equal(cartItems(v),b.items))fail();}
 else{if(v.label!==c.before.label||v.total_cents!==c.before.total_cents||!equal(v.items,c.before.items)||b.action==='claim'&&v.cash_session_id!==b.cash_session_id||b.action==='finalize_cash'&&(v.cash_session_id!==c.before.cash_session_id||v.received_cents!==b.received_cents))fail();}return r;
}
export function reconciliation(v,c,cash){
 const required=['cart_id','revision','state','status','blockers','checked_at','saved_total_cents','sale_id','reserved'];
 if(!keys(v,Object.hasOwn(v||{},'current_quote')?[...required,'current_quote']:required)||v.cart_id!==c.id||v.revision!==c.revision||v.state!==c.state||v.saved_total_cents!==c.total_cents||v.sale_id!==c.sale_id||v.reserved!==false||!time(v.checked_at)||!Array.isArray(v.blockers)||new Set(v.blockers).size!==v.blockers.length||v.blockers.some(k=>!['claimed_in_another_shift','cash_closed','insufficient_stock','invalid_catalog_or_quantity','inactive_product','saved_prices_or_units_changed'].includes(k)))fail();
 if(['completed','cancelled'].includes(c.state)){if(v.status!=='terminal'||v.blockers.length||Object.hasOwn(v,'current_quote'))fail();}
 else{if(!['ready','blocked'].includes(v.status)||(v.status==='ready')!==(v.blockers.length===0))fail();if(v.current_quote){const q=v.current_quote;if(!keys(q,['cash_session_id','items','total_cents','payment_methods','discounts_supported','loyalty_supported','reserved'])||q.cash_session_id!==cash||q.total_cents!==quoteItems(q.items)||!equal(cartItems({items:q.items}),cartItems(c))||!equal(q.payment_methods,['cash'])||q.discounts_supported!==false||q.loyalty_supported!==false||q.reserved!==false)fail();if(v.status==='ready'&&(q.total_cents!==c.total_cents||q.items.some((i,n)=>i.unit!==c.items[n].unit||i.unit_price_cents!==c.items[n].unit_price_cents)))fail();}else if(v.status==='ready')fail();}return v;
}

// A historical confirmation may clear only its own unchanged local draft.
export function matchesDraftOrigin(c,reference,items){return c?.kind==='save'&&!!c.origin&&c.origin===reference&&equal(c.body.items,items);}
