// Invoked by the real Go/SQLite integration; never connects to installed databases.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {once} from 'node:events';
const base=process.env.TITAN_BRIDGE_URL,token=process.env.TITAN_BRIDGE_TOKEN,session=JSON.parse(process.env.TITAN_BRIDGE_SESSION||'null');
assert.ok(base&&new URL(base).hostname==='127.0.0.1'&&token&&session);
const profile=await mkdtemp(join(tmpdir(),'titan-ar-browser-'));
let browser,ws;
async function close(){if(ws)ws.close();if(browser&&browser.exitCode===null){const done=once(browser,'exit');browser.kill('SIGTERM');await done;}browser=null;}
async function start(){
 browser=spawn(process.env.TITAN_TEST_CHROME||'google-chrome',['--headless','--no-sandbox','--disable-gpu','--no-first-run','--no-default-browser-check','--disable-background-networking','--disable-sync','--disable-extensions','--remote-debugging-port=0','--user-data-dir='+profile,'about:blank'],{stdio:['ignore','ignore','pipe']});
 const endpoint=await new Promise((resolve,reject)=>{let text='';const timeout=setTimeout(()=>reject(Error('Chrome startup timeout')),10000);browser.stderr.on('data',chunk=>{text+=chunk;const m=text.match(/DevTools listening on (ws:\/\/[^\s]+)/);if(m){clearTimeout(timeout);resolve(m[1]);}});browser.once('error',()=>{clearTimeout(timeout);reject(Error('Chrome unavailable'));});});
 ws=new WebSocket(endpoint);await once(ws,'open');let sequence=0;const waiting=new Map();
 ws.addEventListener('message',e=>{const value=JSON.parse(e.data);if(value.id){const p=waiting.get(value.id);waiting.delete(value.id);value.error?p.reject(Error('CDP request failed')):p.resolve(value.result);}});
 const call=(method,params={},sid)=>new Promise((resolve,reject)=>{const id=++sequence;waiting.set(id,{resolve,reject});ws.send(JSON.stringify({id,method,params,...(sid?{sessionId:sid}:{})}));});
 async function page(){const {targetId}=await call('Target.createTarget',{url:base+'/ar-recovery/'});const {sessionId}=await call('Target.attachToTarget',{targetId,flatten:true});await call('Runtime.enable',{},sessionId);
  const evaluate=async expression=>{const r=await call('Runtime.evaluate',{expression,replMode:true,awaitPromise:true,returnByValue:true},sessionId);if(r.exceptionDetails)throw Error('Browser assertion failed: '+(r.exceptionDetails.exception?.description||r.exceptionDetails.text));return r.result.value;};
  // wait for the target origin before importing same-origin ES modules.
  for(let n=0;n<100;n++){if(await evaluate('location.origin')===base)break;await new Promise(r=>setTimeout(r,10));}
  await evaluate(`globalThis.context=${JSON.stringify({token,session})};globalThis.mod=await import('/ar-recovery/client.mjs');globalThis.journal=(await import('/ar-recovery/recovery.mjs')).createRecoveryJournal();globalThis.posts=0;globalThis.lose=false;globalThis.client=mod.createReceivablesClient({pending:new Map(),getCurrent:()=>context,fetcher:async(p,o)=>{const r=await fetch(p,o);if(o.method==='POST'){posts++;if(lose){lose=false;await r.arrayBuffer();throw Error('synthetic lost response after real commit');}}return r;}}).bind(context);await client.ready();true`);
  return evaluate;
 }
 return page;
}
const input={operation_id:'browser-create',id:'browser-document',customer_reference:'NEVER-PERSIST-CUSTOMER',document_reference:'NEVER-PERSIST-REFERENCE',description:'NEVER-PERSIST-DESCRIPTION',issued_on:'2026-11-01',total_cents:1000,installments:[{id:'one',due_on:'2026-11-05',amount_cents:500},{id:'two',due_on:'2026-11-10',amount_cents:500}]};
const cash={operation_id:'browser-cash-one',installment_id:'one',session_id:'ar-turn',amount_cents:500,expected_received_cents:0};
try{
 let pages=await start(),evaluate=await pages();
 await evaluate(`lose=true;await client.create(${JSON.stringify(input)}).then(()=>{throw Error('expected lost response')},()=>{});true`);
 assert.equal(await evaluate('client.pending().operation_id'),'browser-create');
 const stored=await evaluate(`JSON.stringify(await journal.read(JSON.stringify(['${session.tenant_id}','${session.store_id}','${session.identity_id}','${session.device_id}'])))`);
 assert.deepEqual(JSON.parse(stored),{op:'browser-create',id:'browser-document',action:'create'});
 for(const secret of [token,input.customer_reference,input.document_reference,input.description,'amount_cents','session_id'])assert.ok(!stored.includes(secret));
 await close(); // process exit: neither module memory nor an open IDB connection survives.
 pages=await start();evaluate=await pages();assert.equal(await evaluate('client.pending().operation_id'),'browser-create');
 assert.equal(await evaluate('(await client.recover()).document.id'),'browser-document');assert.equal(await evaluate('posts'),0);
 await evaluate(`lose=true;await client.receive('browser-document',${JSON.stringify(cash)}).catch(()=>{});true`);
 await close();pages=await start();evaluate=await pages();
 assert.equal(await evaluate('(await client.recover()).document.received_cents'),500);assert.equal(await evaluate('posts'),0);
 await evaluate(`await client.receive('browser-document',${JSON.stringify({...cash,operation_id:'browser-cash-two',installment_id:'two'})});true`);
 assert.equal(await evaluate("(await client.read('browser-document')).received_cents"),1000);
 // Atomic IndexedDB reservation also protects independent tabs and module instances.
 const other=await pages(),scope=JSON.stringify(['scope','store','user','device']);
 const compete=async(e,op)=>e(`await journal.reserve(${JSON.stringify(scope)},{op:${JSON.stringify(op)},id:'document',action:'create'})`);
 assert.deepEqual((await Promise.all([compete(evaluate,'first'),compete(other,'second')])).sort(),[false,true]);
 assert.equal(await evaluate(`await journal.read(${JSON.stringify(JSON.stringify(['another','store','user','device']))})`),null);
 // A pointer without a committed receipt survives a restart and cannot be resent.
 const actualScope=JSON.stringify([session.tenant_id,session.store_id,session.identity_id,session.device_id]);
 await evaluate(`await journal.reserve(${JSON.stringify(actualScope)},{op:'not-sent',id:'never-created',action:'create'});true`);
 await close();pages=await start();evaluate=await pages();
 assert.equal(await evaluate('await client.recover()'),null);
 assert.equal(await evaluate(`await client.recover(true).then(()=>false,e=>e.message==='recoveryOnly')`),true);
 assert.equal(await evaluate('posts'),0);assert.equal(await evaluate('client.pending().operation_id'),'not-sent');
 // Explicit reconciliation commits a server fence; lose its reply and restart again.
 await evaluate(`lose=true;await client.reconcile().then(()=>{throw Error('expected lost response')},()=>{});true`);
 assert.equal(await evaluate('client.pending().operation_id'),'not-sent');
 await close();pages=await start();evaluate=await pages();
 assert.equal(await evaluate('await client.recover()'),null);assert.equal(await evaluate('client.pending()'),null);assert.equal(await evaluate('posts'),0);
 const delayed={...input,operation_id:'not-sent',id:'never-created',document_reference:'retired-document'};
 assert.equal(await evaluate(`(await fetch('/local/v1/finance/receivables',{method:'POST',headers:{Authorization:'Bearer '+context.token,'Content-Type':'application/json'},body:JSON.stringify(${JSON.stringify(delayed)})})).status`),409);
 console.log(JSON.stringify({explicitReconciliation:true,lateCommandBlocked:true,lostReconciliationRecovered:true,realChromeProcessRestarts:4,realHTTP:true,realIndexedDB:true,onlyIdentifiersPersisted:true,crossTabReservation:true,noDuplicateCash:true,missingReceiptNeverResent:true}));
}finally{await close();await rm(profile,{recursive:true,force:true});}
