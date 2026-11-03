// Launched only by the Go disposable HTTP fixture, never a commercial service.
import assert from 'node:assert/strict';
import {createMaintenance} from '../src/core/internalMaintenance/client.mjs';
const base=process.env.TITAN_BRIDGE_URL,token=process.env.TITAN_BRIDGE_TOKEN,session=JSON.parse(process.env.TITAN_BRIDGE_SESSION||'null');
if(!base?.startsWith('http://127.0.0.1:')||!token||!session)throw Error('Disposable Go fixture required');
const fetcher=(path,options)=>fetch(new URL(path,base),options);
const cr=await fetcher('/local/v1/capabilities',{headers:{Authorization:'Bearer '+token}});assert.equal(cr.status,200);const capabilities=await cr.json();
const map=new Map(),storage={getItem:k=>map.get(k)??null,setItem:(k,v)=>map.set(k,v),removeItem:k=>map.delete(k)},locks={request:async(k,o,fn)=>fn({name:k})};
const api=createMaintenance({token,session,capabilities,isCurrent:()=>true,fetcher,storage,locks});
async function create(resource,id,data){const c=api.prepare(resource,{operation_id:'bridge-'+resource,id,expected_revision:0,action:'create',reason:'Disposable HTTP contract verification',data});assert.equal((await api.submit(c)).state,'confirmed');assert.equal(api.pending(),null);}
await create('specialties','electrical',{name:'Elétrica',description:'Machines and installations'});
await create('technicians',session.identity_id,{identity_id:session.identity_id,specialty_ids:['electrical'],contact:'Ramal 2',note:''});
await create('assets','oven',{asset_id:'oven',asset_type:'installation',sector:'Padaria',location:'Quadro 2',note:''});
await create('classifications','electric-repair',{request_id:'electric-repair',expected_request_revision:1,specialty_id:'electrical',note:''});
assert.equal((await api.read('specialties')).total,1);
assert.ok((await api.read('candidates')).items.some(v=>v.id===session.identity_id));
const queue=await api.read('queue',{technician_id:session.identity_id});assert.equal(queue.total,1);assert.equal(queue.items[0].specialty_name,'Elétrica');assert.equal(queue.items[0].asset_profile.data.sector,'Padaria');assert.equal(queue.push_delivered,false);
assert.equal((await api.history('specialties','electrical')).total,1);
const command=api.prepare('specialties',{operation_id:'bridge-specialties',id:'electrical',expected_revision:0,action:'create',reason:'Disposable HTTP contract verification',data:{name:'Elétrica',description:'Machines and installations'}});
assert.equal((await api.submit(command)).state,'confirmed');assert.equal((await api.read('specialties')).total,1);
api.dispose();console.log('PASS real disposable HTTP: signed test contract, real session/middleware, 4 registry resources, receipt recovery/idempotency, candidates, validated queue and history. No browser, push or commercial database.');
