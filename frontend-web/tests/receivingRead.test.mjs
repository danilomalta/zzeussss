import test from 'node:test';
import assert from 'node:assert/strict';
import {createReceivingRead} from '../src/core/local/receivingRead.mjs';
import {exactQuantity} from '../src/core/local/operationsRead.mjs';
import {areaForRoute} from '../src/core/local/accessModel.mjs';
const reply=v=>async()=>new Response(JSON.stringify(v));
const voided={receipt_id:'receipt',order_id:'o',product_id:'p',quantity_milli:2000,unit:'g',location_id:'l',actor_id:'person',reason:'Erro de lançamento',created_at:'2026-10-09T12:00:00Z'};
const receipt={id:'receipt',order_id:'o',product_id:'p',delivery_reference:'doc',delivered_milli:3000,accepted_milli:2000,unit:'g',location_id:'l',actor_id:'person',reason:'Recebimento conferido',created_at:'2026-10-09T11:00:00Z',void:voided};
const history={order_id:'o',product_id:'p',unit:'g',commercial_status:'local_not_sent',receiving_status:'authorized',total_count:52,offset:50,limit:50,has_more:false,totals:{planned_milli:5000,effective_accepted_milli:0,effective_delivered_milli:0,partial_rejected_milli:0,remaining_milli:5000,fully_rejected_milli_exact:'18014398509482029',recorded_accepted_milli_exact:'2000',voided_accepted_milli_exact:'2000'},items:[{kind:'received',id:'receipt',created_at:receipt.created_at,receipt},{kind:'voided',id:'receipt',created_at:voided.created_at,void:voided}]};
test('original and void entries remain separate, totals exact above safe range',async()=>{
 const v=await createReceivingRead(reply(history)).history('t','o',50);
 assert.equal(v.items[0].receipt.accepted_milli,2000);assert.equal(v.totals.effective_accepted_milli,0);
 assert.equal(exactQuantity(v.totals.fully_rejected_milli_exact),'18.014.398.509.482,029');
});
test('reject wrong order, unit, event context, rounding and corrupted totals',async()=>{
 for(const delta of [{order_id:'other'},{unit:'kg'},{offset:0},{totals:{...history.totals,remaining_milli:4999}},{totals:{...history.totals,recorded_accepted_milli_exact:'2001'}},{totals:{...history.totals,fully_rejected_milli_exact:18014398509482029}},{items:[{...history.items[0],receipt:{...receipt,accepted_milli:3001}}]},{items:[history.items[0],history.items[0]]}])await assert.rejects(createReceivingRead(reply({...history,...delta})).history('t','o',50));
});
test('complete refusal does not count as accepted or reduce remaining',async()=>{
 const rejection={id:'rej',order_id:'o',product_id:'p',delivery_reference:'delivery',unit:'g',delivered_milli:500000,actor_id:'person',reason:'Entrega imprópria',created_at:'2026-10-09'};
 const v=await createReceivingRead(reply({...history,offset:0,total_count:1,items:[{kind:'rejected',id:'rej',created_at:rejection.created_at,rejection}]})).history('t','o');
 assert.equal(v.totals.remaining_milli,5000);assert.equal(v.items[0].kind,'rejected');
});
test('filtered purchase pages accept cancelled snapshots and refuse wrong supplier',async()=>{
 const o={id:'o',supplier_id:'s',supplier_name:'Fornecedor',status:'cancelled',receiving_status:'not_authorized',created_at:'2026-10-09',items:[{product_id:'p',sku:'sku',name:'Farinha',unit:'g',quantity_milli:5000}]};
 const page={filters:{supplier_id:'s',product_id:'p'},total_count:1,offset:0,limit:50,has_more:false,items:[o]};
 assert.equal((await createReceivingRead(reply(page)).orders('t','s','p')).items[0].status,'cancelled');
 await assert.rejects(createReceivingRead(reply({...page,items:[{...o,supplier_id:'foreign'}]})).orders('t','s','p'));
});
test('history GET encodes ID without credentials, refuses missing token and 409',async()=>{
 let count=0;const c=createReceivingRead(async(url,opt)=>{count++;assert.equal(url,'/local/v1/purchase-orders/o%2F%3F/receiving-history?offset=50');assert.equal(opt.method,'GET');assert.equal(opt.body,undefined);assert.equal(opt.credentials,'omit');return new Response(JSON.stringify({...history,order_id:'o/?',items:[]}));});
 await c.history('t','o/?',50);await assert.rejects(c.history('','o'));assert.equal(count,1);
 await assert.rejects(createReceivingRead(async()=>new Response('',{status:409})).history('t','o'),e=>e.status===409);
 assert.equal(areaForRoute('/local/receiving'),'orders');
});
