import test from 'node:test';
import assert from 'node:assert/strict';
import {createLocalClient, LocalAPIError} from '../src/core/local/localClient.mjs';
import {quoteCart,confirmCashQuote} from '../src/core/local/quotedCart.mjs';
import {makeSale} from '../src/core/local/posModel.mjs';

const input = {cash_session_id:'shift',items:[{product_id:'p',location_id:'shelf',quantity_milli:1500}]};
const response = () => ({cash_session_id:'shift',items:[{...input.items[0],unit:'kg',unit_price_cents:299,total_cents:449,available_milli:5000}],total_cents:449,payment_methods:['cash'],discounts_supported:false,loyalty_supported:false,reserved:false});
const json = (data,status=200) => new Response(JSON.stringify(data),{status,headers:{'Content-Type':'application/json'}});
const cart = [{product:{id:'p',sku:'SKU',name:'Produto',unit:'kg',price_cents:199},location_id:'shelf',quantity_milli:1500}];

test('quote posts only shift and item quantities with session and no-store',async()=>{
  let calls=0;
  const client=createLocalClient(async(url,options)=>{
    calls++;
    assert.equal(url,'/local/v1/sales/quote');
    assert.equal(options.method,'POST');
    assert.equal(options.headers.Authorization,'Bearer token');
    assert.equal(options.cache,'no-store');
    assert.deepEqual(JSON.parse(options.body),input);
    return json(response());
  });
  const quote=await client.quoteSale('token',input);
  assert.equal(quote.total_cents,449);
  assert.equal(calls,1);
});

test('changed prices refresh the cart without mutating its cached source',async()=>{
  const client=createLocalClient(async()=>json(response()));
  const result=await quoteCart(client,'token',cart,'shift');
  assert.equal(result.changed,true);
  assert.equal(result.total_cents,449);
  assert.equal(result.cart[0].product.price_cents,299);
  assert.equal(cart[0].product.price_cents,199);
  assert.equal((await quoteCart(client,'token',result.cart,'shift')).changed,false);
});

test('invalid quote input never reaches HTTP',async()=>{
  let calls=0;
  const client=createLocalClient(async()=>{calls++;return json(response());});
  for(const bad of [null,{...input,items:[]},{...input,items:[{...input.items[0],quantity_milli:1.5}]}]){
    await assert.rejects(client.quoteSale('token',bad),LocalAPIError);
  }
  await assert.rejects(client.quoteSale('',input),LocalAPIError);
  assert.equal(calls,0);
});

test('inexact totals, cross-context items and invented capabilities are blocked',async()=>{
  for(const change of [
    q=>q.total_cents++,q=>q.items[0].total_cents++,q=>q.items[0].product_id='foreign',
    q=>q.cash_session_id='other',q=>q.items[0].quantity_milli=1000,q=>q.items[0].available_milli=1000,
    q=>q.items[0].unit_price_cents=Number.MAX_SAFE_INTEGER+1,q=>q.reserved=true,
    q=>q.discounts_supported=true,q=>q.loyalty_supported=true,q=>q.payment_methods=['cash','pix'],
    q=>q.items[0].unit='unknown',q=>q.items=[],
  ]){
    const data=response();change(data);
    await assert.rejects(createLocalClient(async()=>json(data)).quoteSale('token',input),LocalAPIError);
  }
});

test('repeated lines must fit cumulative available stock, not each fit separately',async()=>{
  const data=response();data.items.push({...data.items[0]});data.total_cents=898;
  data.items.forEach(item=>item.available_milli=2000);
  const duplicate={...input,items:[...input.items,...input.items]};
  await assert.rejects(createLocalClient(async()=>json(data)).quoteSale('token',duplicate),LocalAPIError);
});

test('quote failure propagates and does not fall back to cached catalog or write sale',async()=>{
  const calls=[];
  const client=createLocalClient(async url=>{calls.push(url);return json({},409);});
  await assert.rejects(quoteCart(client,'token',cart,'shift'),e=>e.status===409);
  assert.deepEqual(calls,['/local/v1/sales/quote']);
});

test('quantity mismatch from server cannot silently change order',async()=>{
  const data=response();data.items[0].quantity_milli=1000;data.items[0].total_cents=299;data.total_cents=299;
  await assert.rejects(createLocalClient(async()=>json(data)).quoteSale('token',input),LocalAPIError);
});

test('checkout96 cash bridge stops before tender/finalization when quote changes',async()=>{
  const client=createLocalClient(async()=>json(response()));
  let updated=null,confirmed=0;
  await assert.rejects(confirmCashQuote(client,'token',cart,'shift',500,c=>{updated=c;},async()=>{confirmed++;}),/catálogo mudou/);
  assert.equal(updated[0].product.price_cents,299);
  assert.equal(confirmed,0);
});

test('checkout96 cash bridge preserves its stable draft ID and authoritative values',async()=>{
  const client=createLocalClient(async()=>json(response()));
  const current=[{...cart[0],product:{...cart[0].product,price_cents:299}}];
  let confirmed=0;
  const result=await confirmCashQuote(client,'token',current,'shift',500,()=>assert.fail('unexpected cart refresh'),async c=>{
    confirmed++;
    const input=makeSale(c,'shift',()=> 'stable-draft96');
    assert.equal(input.operation_id,'stable-draft96');
    assert.equal(input.sale_id,'stable-draft96');
    assert.equal(input.payments[0].amount_cents,449);
    return input;
  });
  assert.equal(confirmed,1);
  assert.equal(result.cash_session_id,'shift');
});

test('checkout96 cash bridge never confirms with failed quote or insufficient tender',async()=>{
  let confirmed=0;
  const confirm=async()=>{confirmed++;};
  const current=[{...cart[0],product:{...cart[0].product,price_cents:299}}];
  const blocked=createLocalClient(async()=>json({},409));
  await assert.rejects(confirmCashQuote(blocked,'token',current,'shift',500,()=>{},confirm));
  const client=createLocalClient(async()=>json(response()));
  await assert.rejects(confirmCashQuote(client,'token',current,'shift',448,()=>{},confirm),/valor recebido/);
  assert.equal(confirmed,0);
});
