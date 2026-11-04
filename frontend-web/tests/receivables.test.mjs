import test from 'node:test';
import assert from 'node:assert/strict';
import {createReceivablesClient,parseCents,validateDocument,validateCreateInput} from '../src/core/receivables/client.mjs';
import {dictionaries} from '../src/core/receivables/words.mjs';
const input=()=>({operation_id:'create',id:'doc',customer_reference:'customer',document_reference:'external-1',description:'External commercial document',issued_on:'2026-11-01',total_cents:1000,installments:[{id:'one',due_on:'2026-11-05',amount_cents:1000}]});
const document=(received=0)=>({...input(),state:received===1000?'settled':'open',created_by:'owner',created_at:'2026-11-01T00:00:00Z',received_cents:received,outstanding_cents:1000-received,installments:[{...input().installments[0],received_cents:received}],receipts:received?[{operation_id:'receipt',movement_id:'movement',amount_cents:received,actor_id:'owner',created_at:'2026-11-01T00:00:00Z'}]:[],history:[{operation_id:'create',action:'create',actor_id:'owner',device_id:'device',reason:'',created_at:'2026-11-01T00:00:00Z'}]});
const result=(op='create',action='create',received=0)=>({operation_id:op,action,repeated:false,document:document(received)});
function fixture(handler){
 let context={token:'private-test-token',session:{tenant_id:'tenant',store_id:'store',identity_id:'owner',device_id:'device'}};const calls=[],pending=new Map();
 const factory=createReceivablesClient({getCurrent:()=>context,pending,fetcher:async(path,options)=>{calls.push({path,...options});const value=await handler(path,options);return value instanceof Response?value:new Response(JSON.stringify(value),{headers:{'Content-Type':'application/json'}});}});
 const client=factory.bind(context);return {client,calls,pending,change(patch){context={...context,...patch};},bind(){return factory.bind(context);}};
}
test('exact cents and validated installment schedules refuse float, overflow and inconsistent totals',()=>{
 assert.equal(parseCents('90071992547409,91'),Number.MAX_SAFE_INTEGER);assert.equal(parseCents('0.01'),1);
 for(const s of ['0','-1','1.001','90071992547409.92','1e2','NaN'])assert.throws(()=>parseCents(s));
 assert.deepEqual(validateCreateInput(input()),input());for(const edit of [v=>v.total_cents=999,v=>v.installments.push({...v.installments[0]}),v=>v.issued_on='2026-02-30',v=>v.installments[0].due_on='2026-10-01',v=>v.customer_reference='']){const v=input();edit(v);assert.throws(()=>validateCreateInput(v));}
});
test('server responses never silently lose a receipt or show inconsistent zero',()=>{
 assert.equal(validateDocument(document(500)).received_cents,500);
 for(const edit of [v=>v.receipts=[],v=>v.outstanding_cents=0,v=>v.installments[0].received_cents=1001,v=>v.state='official',v=>v.total_cents=-1]){const v=document(500);edit(v);assert.throws(()=>validateDocument(v));}
});
test('real API paths use no-store, bearer only in headers and exact original body',async()=>{
 const f=fixture(()=>result());await f.client.create(input());const call=f.calls[0];assert.equal(call.path,'/local/v1/finance/receivables');assert.equal(call.method,'POST');assert.equal(call.cache,'no-store');assert.equal(call.credentials,'omit');assert.equal(call.headers.Authorization,'Bearer private-test-token');assert.ok(!call.path.includes('private-test-token'));assert.deepEqual(JSON.parse(call.body),input());assert.equal(f.client.pending(),null);
});
test('lost response retains the operation; read-only recovery never sends a second POST',async()=>{
 let first=true;const f=fixture((path,options)=>{if(first&&options.method==='POST'){first=false;throw Error('lost response');}return {...result(),repeated:true};});
 const original=input();await assert.rejects(f.client.create(original));original.description='changed later';assert.equal(f.client.pending().operation_id,'create');
 assert.equal((await f.client.recover()).repeated,true);assert.equal(f.calls.filter(v=>v.method==='POST').length,1);assert.equal(f.client.pending(),null);
});
test('missing receipt keeps uncertainty; explicit retry uses original operation and body',async()=>{
 let first=true;const f=fixture((path,options)=>{if(options.method==='GET')return new Response('',{status:404});if(first){first=false;throw Error('lost');}return result();});
 const original=input();await assert.rejects(f.client.create(original));original.total_cents=1;
 assert.equal(await f.client.recover(),null);assert.equal(f.calls.filter(v=>v.method==='POST').length,1);assert.ok(f.client.pending());
 await f.client.recover(true);const posts=f.calls.filter(v=>v.method==='POST');assert.equal(posts.length,2);assert.equal(posts[0].body,posts[1].body);
});
test('logout/context changes forbid old requests and do not discard uncertain operations',async()=>{
 for(const field of ['tenant_id','store_id','identity_id','device_id']){
 const f=fixture(()=>result());f.change({session:{tenant_id:'tenant',store_id:'store',identity_id:'owner',device_id:'device',[field]:'another'}});await assert.rejects(f.client.create(input()),/context/);assert.equal(f.calls.length,0);
 }
 let release;const f=fixture(()=>new Promise(r=>release=r));const pending=f.client.create(input());await new Promise(r=>setImmediate(r));f.change({token:'new-login'});release(result());await assert.rejects(pending,/context/);assert.ok(f.client.pending());
 await assert.rejects(f.client.recover(true),/context/);assert.equal(f.calls.filter(v=>v.method==='POST').length,1);
 const recovered=f.bind(); // A new explicit read remains necessary; no automatic action is fired on bind.
 assert.equal(f.calls.length,1);assert.ok(recovered.pending());
});
test('parallel writes are blocked before a second HTTP mutation',async()=>{
 let release;const f=fixture(()=>new Promise(r=>release=r));const first=f.client.create(input());await new Promise(r=>setImmediate(r));await assert.rejects(f.client.create({...input(),operation_id:'other'}),/busy/);release(result());await first;assert.equal(f.calls.length,1);
});
test('strict client input rejection occurs before HTTP',async()=>{
 const f=fixture(()=>result());await assert.rejects(async()=>f.client.create({...input(),total_cents:1}));await assert.rejects(async()=>f.client.receive('doc',{operation_id:'receive',session_id:'turn',installment_id:'one',amount_cents:-1,expected_received_cents:0}));assert.equal(f.calls.length,0);
});
test('five dictionaries cover all labels and totals render exact received/outstanding amounts',async()=>{
 const keys=Object.keys(dictionaries['pt-BR']).sort();for(const lang of ['pt-BR','en','es','fr','zh-CN']){assert.deepEqual(Object.keys(dictionaries[lang]).sort(),keys);assert.ok(Object.values(dictionaries[lang]).every(v=>typeof v==='string'&&v.trim()));}
 const {createRequire}=await import('node:module'),fs=await import('node:fs');const require=createRequire(import.meta.url),ts=require('typescript'),React=require('react'),{renderToStaticMarkup}=require('react-dom/server');
 const source=fs.readFileSync(new URL('../src/modules/financial/pages/LocalReceivables.tsx',import.meta.url),'utf8');
 const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX,target:ts.ScriptTarget.ES2022}}).outputText;
 const {money113}=await import('../src/core/payables113/model.mjs');
 const mod={exports:{}};Function('require','exports','module',js)(name=>name.startsWith('react')?require(name):name.includes('words.mjs')?{receivableWords:lang=>dictionaries[lang]}:name.includes('payables113/model.mjs')?{money113}:{},mod.exports,mod);
 for(const locale of Object.keys(dictionaries)){const html=renderToStaticMarkup(React.createElement(mod.exports.ReceivableTotals,{document:document(600),locale}));const money=n=>new Intl.NumberFormat(locale,{style:'currency',currency:'BRL'}).format(n/100);assert.ok(html.includes(dictionaries[locale].received));assert.ok(html.includes(money(600)));assert.ok(html.includes(money(400)));}
});

test('a delayed old recovery query never permits POST after a new login',async()=>{
 let release;const f=fixture((path,options)=>{if(options.method==='POST')throw Error('lost response');return new Promise(r=>release=r);});
 await assert.rejects(f.client.create(input()));const recovery=f.client.recover(true);await new Promise(r=>setImmediate(r));
 f.change({token:'new-login'});release(new Response('',{status:404}));await assert.rejects(recovery,/context/);
 assert.equal(f.calls.filter(v=>v.method==='POST').length,1);assert.ok(f.client.pending());
});

function durableFixture(handler,initial=null){
 const saved=new Map(),scope=JSON.stringify(['tenant','store','owner','device']);if(initial)saved.set(scope,initial);
 const journal={read:async key=>saved.get(key)||null,reserve:async(key,c)=>{if(saved.has(key))return false;saved.set(key,{op:c.op,id:c.id,action:c.action});return true;},remove:async(key)=>{saved.delete(key);}};
 let context={token:'original-session',session:{tenant_id:'tenant',store_id:'store',identity_id:'owner',device_id:'device'}};const calls=[],pending=new Map();
 const factory=createReceivablesClient({getCurrent:()=>context,pending,journal,fetcher:async(path,options)=>{calls.push({path,...options});const result=await handler(path,options);return result instanceof Response?result:new Response(JSON.stringify(result));}});
 return {client:factory.bind(context),calls,saved,pending,journal,bind:()=>factory.bind(context),change(patch){context={...context,...patch};}};
}
test('durable pointer hydrates before mutations and recovers only its original receipt',async()=>{
 const f=durableFixture(()=>result(),{op:'create',id:'doc',action:'create'});
 assert.equal((await f.client.ready()).operation_id,'create');
 await assert.rejects(f.client.create({...input(),operation_id:'new'}),/pending/);assert.equal(f.calls.length,0);
 assert.equal((await f.client.recover()).document.id,'doc');assert.equal(f.calls[0].method,'GET');assert.equal(f.saved.size,0);
});
test('reopened pointer not found remains durable and refuses explicit retry without the original body',async()=>{
 const f=durableFixture(()=>new Response('',{status:404}),{op:'not-sent',id:'doc',action:'create'});
 await f.client.ready();assert.equal(await f.client.recover(),null);
 await assert.rejects(f.client.recover(true),/recoveryOnly/);assert.equal(f.calls.filter(c=>c.method==='POST').length,0);assert.equal(f.saved.size,1);assert.ok(f.client.pending());
});
test('a new login may consult but never resend a command retained by the old session',async()=>{
 const f=durableFixture((path,o)=>{if(o.method==='POST')throw Error('lost');return new Response('',{status:404});});
 await assert.rejects(f.client.create(input()));f.change({token:'another-session'});const newClient=f.bind();
 await assert.rejects(newClient.recover(true),/recoveryOnly/);assert.equal(f.calls.filter(c=>c.method==='POST').length,1);assert.equal(f.saved.size,1);
});
test('durable storage failure refuses financial POST and leaves no pretend confirmation',async()=>{
 const f=durableFixture(()=>result());f.journal.reserve=async()=>{throw Error('storage');};
 await assert.rejects(f.client.create(input()),/storage/);assert.equal(f.calls.length,0);
});
test('session changed while committing a pointer prevents POST without removing the pointer',async()=>{
 const f=durableFixture(()=>result()),reserve=f.journal.reserve;f.journal.reserve=async(...args)=>{const saved=await reserve(...args);f.change({token:'other-session'});return saved;};
 await assert.rejects(f.client.create(input()),/context/);assert.equal(f.calls.length,0);assert.equal(f.saved.size,1);
 assert.ok(await f.bind().ready());
});
test('failure to remove a confirmed pointer retains uncertainty and subsequent GET confirms it',async()=>{
 const f=durableFixture(()=>result()),remove=f.journal.remove;let fail=true;
 f.journal.remove=async(...args)=>{if(fail){fail=false;throw Error('storage');}return remove(...args);};
 await assert.rejects(f.client.create(input()),/storage/);assert.ok(f.client.pending());assert.equal(f.saved.size,1);
 await f.client.recover();assert.equal(f.calls.filter(c=>c.method==='POST').length,1);assert.equal(f.saved.size,0);
});

const retired={operation_id:'not-sent',document_id:'doc',action:'create',repeated:false,resolution:'not_applied'};
test('explicit reconciliation queries first and removes a pointer only after a matching durable fence',async()=>{
 const f=durableFixture((path,o)=>o.method==='GET'?new Response('',{status:404}):retired,{op:'not-sent',id:'doc',action:'create'});
 await f.client.ready();assert.equal(f.calls.length,0);assert.equal(await f.client.reconcile(),null);
 assert.deepEqual(f.calls.map(c=>c.method),['GET','POST']);assert.ok(f.calls[1].path.endsWith('/operations/not-sent/reconcile'));
 assert.deepEqual(JSON.parse(f.calls[1].body),{document_id:'doc',action:'create',confirmed:true});assert.equal(f.saved.size,0);assert.equal(f.client.pending(),null);
});
test('reconciling an existing receipt only reads it, without reversing or resending money',async()=>{
 const f=durableFixture(()=>result(),{op:'create',id:'doc',action:'create'});await f.client.ready();
 assert.equal((await f.client.reconcile()).document.id,'doc');assert.deepEqual(f.calls.map(c=>c.method),['GET']);
});
test('lost reconciliation response recovers the terminal fence by GET without another POST',async()=>{
 let fenced=false;const f=durableFixture((path,o)=>{if(o.method==='GET')return fenced?retired:new Response('',{status:404});fenced=true;throw Error('lost reconciliation response');},{op:'not-sent',id:'doc',action:'create'});
 await f.client.ready();await assert.rejects(f.client.reconcile());assert.equal(f.saved.size,1);
 assert.equal(await f.client.recover(),null);assert.equal(f.saved.size,0);assert.equal(f.calls.filter(c=>c.method==='POST').length,1);
});
test('forged or denied reconciliation never clears an uncertain operation',async()=>{
 for(const response of [{...retired,document_id:'other'},{...retired,action:'receive_cash'},new Response('',{status:403})]){
 const f=durableFixture((path,o)=>o.method==='GET'?new Response('',{status:404}):response,{op:'not-sent',id:'doc',action:'create'});await f.client.ready();
 await assert.rejects(f.client.reconcile());assert.equal(f.saved.size,1);assert.ok(f.client.pending());
 }
});
test('session switch during reconciliation lookup prevents even the explicit fence POST',async()=>{
 let release;const f=durableFixture(()=>new Promise(r=>release=r),{op:'not-sent',id:'doc',action:'create'});await f.client.ready();
 const work=f.client.reconcile();await new Promise(r=>setImmediate(r));f.change({token:'other-login'});release(new Response('',{status:404}));
 await assert.rejects(work,/context/);assert.equal(f.calls.filter(c=>c.method==='POST').length,0);assert.equal(f.saved.size,1);
});

const orderSource={customer_tenant_id:'buyer',customer_store_id:'buyer-store',message_id:'order-message',decision_id:'decision',order_id:'purchase-order',customer_reference:'customer',product_id:'product',unit:'un',accepted_milli:3000,offer_sha256:'a'.repeat(64)};
test('customer order preview validates the authenticated source and posts only explicit documented terms',async()=>{
 const value={...document(),customer_order:{source:orderSource,terms_reference:'agreed-terms-42'}};
 const f=fixture((path,options)=>options.method==='GET'?orderSource:{...result(),document:value});
 assert.deepEqual(await f.client.customerOrder('order-message'),orderSource);
 const inValue={...input(),customer_order:{message_id:'order-message',terms_reference:'agreed-terms-42',confirmed:true}};
 assert.equal((await f.client.create(inValue)).document.customer_order.source.order_id,'purchase-order');
 assert.deepEqual(JSON.parse(f.calls[1].body),inValue);assert.equal(f.calls[0].method,'GET');assert.ok(!f.calls[1].body.includes('offer_sha256'));
});
test('unsupported or unconfirmed order terms are refused before any HTTP write',async()=>{
 for(const change of [v=>v.customer_order.confirmed=false,v=>v.customer_order.terms_reference='',v=>v.customer_order.message_id='',v=>v.customer_order=false]){
 const f=fixture(()=>result());const value={...input(),customer_order:{message_id:'order-message',terms_reference:'terms',confirmed:true}};change(value);await assert.rejects(async()=>f.client.create(value));assert.equal(f.calls.length,0);
 }
});
test('foreign or inconsistent source metadata is never accepted as this customer document',()=>{
 const value={...document(),customer_order:{source:orderSource,terms_reference:'terms'}};assert.equal(validateDocument(value),value);
 for(const change of [v=>v.customer_order.source={...orderSource,customer_reference:'someone-else'},v=>v.customer_order.source={...orderSource,accepted_milli:0},v=>v.customer_order.terms_reference='']){const invalid=structuredClone(value);change(invalid);assert.throws(()=>validateDocument(invalid));}
});
test('a delayed source query cannot be used in another login or store',async()=>{
 let release;const f=fixture(()=>new Promise(r=>release=r));const work=f.client.customerOrder('order-message');await new Promise(r=>setImmediate(r));
 f.change({token:'new-session'});release(orderSource);await assert.rejects(work,/context/);assert.equal(f.calls.filter(c=>c.method==='POST').length,0);
});
