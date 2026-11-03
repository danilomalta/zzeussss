// Only the disposable Go fixture may launch this integration check.
import assert from 'node:assert/strict';
import {createDeliveryCycle} from '../src/core/deliveryCycle/source.mjs';
const base=process.env.TITAN_BRIDGE_URL,token=process.env.TITAN_BRIDGE_TOKEN,session=JSON.parse(process.env.TITAN_BRIDGE_SESSION||'null');
if(!/^http:\/\/127\.0\.0\.1:\d+$/.test(base||'')||!token||!session)throw Error('Disposable Go HTTP fixture required');
const direct=(path,options)=>fetch(new URL(path,base),options),res=await direct('/local/v1/capabilities',{headers:{Authorization:'Bearer '+token}});assert.equal(res.status,200);const capabilities=await res.json();
const memory=new Map(),storage={getItem:k=>memory.get(k)??null,setItem:(k,v)=>memory.set(k,v),removeItem:k=>memory.delete(k)},locks={request:async(k,o,fn)=>fn({name:k})};let loss=true,rejectPosts=0;
const fetcher=async(path,options)=>{const r=await direct(path,options);if(options.method==='POST'&&path.endsWith('/reject')){rejectPosts++;if(loss&&r.ok)throw Error('Synthetic loss after real server response');}return r;};
const options={token,session,capabilities,storage,locks,isCurrent:()=>true,fetcher},api=createDeliveryCycle(options);
const before=await api.detail('trip-one');
const order={id:'order',supplier_id:before.supplier_id,items:[{product_id:before.product_id,unit:'unit',quantity_milli:5000}]};
const body={operation_id:'bridge-full-refusal',expected_revision:before.revision,delivered_milli:3000,existing_rejection_id:'',evidence_reference:'Synthetic disposable unloading check',observed_at:new Date().toISOString(),reason:'Full load refused after inspection'};
const command=api.prepare('reject',order,null,before,body);await assert.rejects(api.submit(command));assert.equal(api.pending().uncertain,true);api.dispose();
const resumed=createDeliveryCycle({...options,token});const recovered=await resumed.recover();assert.equal(recovered.state,'confirmed');assert.equal(recovered.receipt.delivery.state,'rejected');assert.equal(recovered.receipt.rejection_evidence.recorded_by,session.identity_id);assert.equal(recovered.command.body.operation_id,body.operation_id);assert.equal(rejectPosts,1);assert.equal(resumed.pending(),null);
const cycle=await resumed.cycle(order);assert.equal(cycle.allocated_milli,0);assert.equal(cycle.effective_accepted_milli,0);assert.equal(cycle.remaining_order_milli,5000);assert.equal((await resumed.history('trip-one')).total,4);assert.equal((await resumed.detail('trip-one',order)).receipt_id,'');
resumed.dispose();console.log('PASS real Go HTTP + frontend source: atomic full refusal, original IDs, immutable history, scoped recovery after lost response, one POST, zero stock/financial receipt. Disposable SQLite and synthetic signed contract only.');
