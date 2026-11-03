import assert from 'node:assert/strict';
import {createCashBags} from '../src/core/cashBags/source.mjs';
const session=JSON.parse(process.env.TITAN_BRIDGE_SESSION),root=process.env.TITAN_BRIDGE_URL;
const store=new Map(),storage={getItem:k=>store.get(k)??null,setItem:(k,v)=>store.set(k,v),removeItem:k=>store.delete(k)},locks={request:async(k,o,fn)=>fn({})};
let postCount=0,lose=false;
function client(receiver=false){const s={...session,...(receiver?{identity_id:'receiver'}:{})},capabilities={...s,permissions:['manage_cash'],license:{state:'active',modules:['core','inventory','pos']}};return createCashBags({token:receiver?process.env.TITAN_BRIDGE_RECEIVER:process.env.TITAN_BRIDGE_TOKEN,session:s,capabilities,isCurrent:()=>true,storage,locks,fetcher:async(p,o)=>{if(o.method==='POST')postCount++;const r=await fetch(root+p,o);if(lose&&o.method==='POST'){await r.arrayBuffer();lose=false;throw Error('lost after actual response');}return r;}});}
const sender=client(),receiver=client(true),body={operation_id:'prepare-ui',bag_id:'bag-ui',withdrawal_operation_id:'withdraw-one',seal_id:'seal-ui'};
const command=sender.prepare('create','bag-ui',body);lose=true;await assert.rejects(sender.submit(command));assert.equal(sender.pending().uncertain,true);const renewed=client();assert.equal((await renewed.recover()).state,'confirmed');assert.equal(postCount,1);
let b=await renewed.read('bag-ui');assert.equal(b.declared_cents,300);await renewed.submit(renewed.prepare('dispatch',b.id,{operation_id:'dispatch-ui'},b));
b=await receiver.read('bag-ui');assert.equal(b.declared_cents,null);const result=await receiver.submit(receiver.prepare('receive',b.id,{operation_id:'receive-ui',seal_id:'seal-ui',counted_cents:290},b));assert.equal(result.result.bag.difference_cents,-10);
const page=await renewed.list({state:'received',session_id:'turn-one',offset:0});assert.equal(page.total_count,1);assert.equal(page.items[0].history.length,3);
const coverage=await renewed.coverage('turn-one');assert.equal(coverage.withdrawn_cents,300);assert.equal(coverage.received_counted_cents,290);assert.equal(coverage.difference_cents,-10);assert.equal(postCount,3);
console.log('HTTP custody passed: independent blind count; original receipt recovery; exactly 3 POSTs; 300 declared, 290 counted, -10 difference.');
