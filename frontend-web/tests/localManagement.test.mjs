import test from 'node:test';
import assert from 'node:assert/strict';
import {createLocalClient} from '../src/core/local/localClient.mjs';
import {cashCount} from '../src/core/local/cashCount.mjs';
const json=body=>new Response(JSON.stringify(body),{headers:{'Content-Type':'application/json'}});
test('cash denomination count is exact and rejects fractions, negatives and exponent notation',()=>{
 assert.equal(cashCount(['1','2','3']),55000);
 assert.equal(cashCount(['','','','','','','','','1','1','1','1','1']),91);
 for(const bad of ['-1','1.5','1e3','x','9999999999']) assert.throws(()=>cashCount([bad]));
});
test('management rejects unsafe prices before sending and accepts actual creation IDs',async()=>{
 const calls=[];const client=createLocalClient(async(url,options)=>{calls.push([url,JSON.parse(options.body)]);return json({id:'created'});});
 const input={sku:'A',name:'Produto',barcode:'',unit:'kg',price_cents:1234,cost_cents:500};
 assert.deepEqual(await client.createProduct('token',input),{id:'created'});
 assert.deepEqual(calls[0],['/local/v1/products',input]);
 await assert.rejects(client.createProduct('token',{...input,price_cents:1.25}));
 await assert.rejects(client.createProduct('token',{...input,name:''}));
 assert.equal(calls.length,1);
 await client.createLocation('token',{name:'Gôndola',kind:'shelf'});
 assert.equal(calls[1][0],'/local/v1/locations');
});
test('stock retries preserve the original operation identity and rejected or lost responses never trigger automatic posts',async()=>{
 const payload={operation_id:'original',kind:'entry',product_id:'p',to_location_id:'l',quantity_milli:1000,reason:'Entrada'};
 const bodies=[];const client=createLocalClient(async(_url,o)=>{bodies.push(o.body);if(bodies.length===1)throw new Error('lost');return json({operation_id:'original',repeated:true});});
 await assert.rejects(client.stockEntry('token',payload));assert.equal(bodies.length,1);
 await client.stockEntry('token',payload);assert.equal(bodies[0],bodies[1]);
 await assert.rejects(client.stockEntry('token',{...payload,quantity_milli:0.5}));assert.equal(bodies.length,2);
});
test('history is paginated and malformed totals or status cannot be displayed as sales',async()=>{
 const row={sale_id:'sale',cash_session_id:'cash',status:'committed',committed_at:'now',total_cents:100};
 let url;const client=createLocalClient(async path=>{url=path;return json({items:[row]});});
 assert.deepEqual(await client.history('token',20),[row]);assert.equal(url,'/local/v1/sales?limit=20&offset=20');
 await assert.rejects(client.history('token',-1));
 for(const invalid of [{...row,total_cents:1.1},{...row,status:'invented'}]) await assert.rejects(createLocalClient(async()=>json({items:[invalid]})).history('token'));
});
test('empty location lists do not block a new installation',async()=>{
 assert.deepEqual(await createLocalClient(async()=>json({items:null})).locations('token'),[]);
});
