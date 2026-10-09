import test from 'node:test';
import assert from 'node:assert/strict';
import {createStockRead} from '../src/core/local/stockRead.mjs';
const b={product_id:'p',location_id:'l',unit:'kg',physical_milli:10000,reserved_milli:9000,free_milli:1000};
const reply=v=>async()=>new Response(JSON.stringify(v));
test('stock checks the selected context and physical = reserved + free',async()=>{
 assert.deepEqual(await createStockRead(reply(b)).balance('t','p','l'),b);
 for(const delta of [{product_id:'foreign'},{location_id:'other'},{free_milli:1001},{reserved_milli:-1},{physical_milli:Number.MAX_SAFE_INTEGER+1},{unit:'box'}])await assert.rejects(createStockRead(reply({...b,...delta})).balance('t','p','l'));
});
test('global balances are preserved when only a subset of reserves is paginated',async()=>{
 const r={reservation_id:'r',order_id:'o',version_id:'v',responsible_id:'actor',created_at:'2026-10-09',unit:'kg',quantity_milli:1000};
 const page={balance:b,total_count:51,offset:50,limit:50,has_more:false,items:[r]};
 const v=await createStockRead(reply(page)).reservations('t','p','l',50);assert.equal(v.balance.reserved_milli,9000);assert.equal(v.items[0].quantity_milli,1000);
 for(const delta of [{offset:0},{items:[{...r,quantity_milli:9001}]},{items:[{...r,unit:'g'}]},{items:[r,r]}])await assert.rejects(createStockRead(reply({...page,...delta})).reservations('t','p','l',50));
});
test('stock no writes, exact URL and negative offsets rejected before network',async()=>{
 let count=0;const c=createStockRead(async(path,opt)=>{count++;assert.equal(opt.method,'GET');assert.equal(opt.body,undefined);assert.equal(path,'/local/v1/stock/reservations?product_id=p&location_id=l&offset=0');return new Response(JSON.stringify({balance:{...b,reserved_milli:0,free_milli:10000},total_count:0,offset:0,limit:50,has_more:false,items:[]}));});
 assert.equal((await c.reservations('t','p','l')).items.length,0);await assert.rejects(c.reservations('t','p','l',-1));assert.equal(count,1);
});
test('server refusal is visible and cannot invent available stock',async()=>{
 await assert.rejects(createStockRead(async()=>new Response('',{status:403})).balance('t','p','l'),e=>e.status===403);
});
