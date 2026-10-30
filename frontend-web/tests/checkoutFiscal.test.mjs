import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {createRequire} from 'node:module';
import {createCheckoutFiscalClient,checkoutFiscalView} from '../src/core/local/checkoutFiscal.mjs';

const original={operation_id:'same-finish',received_cents:2000,fiscal_invoice:{Environment:2,Nature:'Approved original invoice'}};
const plan=state=>({checkout_id:'checkout',completion:{sale_id:'checkout',operation_id:'same-finish',received_cents:2000,sale_status:'committed'},fiscal:{state,can_reprocess:true,operation_id:'original-fiscal-operation',intent_id:'intent',key:'3'.repeat(44)}});
function fixture(fetcher){
 const session={tenant_id:'company',store_id:'store',identity_id:'operator',device_id:'device'};let current={token:'private-session',session};
 const data=new Map(),calls=[];
 const client=createCheckoutFiscalClient({getCurrent:()=>current,cache:{async read(k){return data.get(k)||null;},async write(k,v){data.set(k,structuredClone(v));}},fetcher:async(url,input)=>{calls.push({url,...input});return {ok:true,status:200,json:async()=>fetcher(url,input)};}}).bind(current);
 return {client,calls,data,change:patch=>{current={...current,...patch};}};
}
test('fiscal statuses are separate from payment and only homologation authorization is accepted',()=>{
 for(const [raw,expected] of [['authorized_homologation','authorized'],['failed','failed'],['pending','pending'],['recovery_required','pending'],['uncertain','pending']])assert.equal(checkoutFiscalView(plan(raw),'checkout').state,expected);
 assert.equal(checkoutFiscalView({checkout_id:'checkout'},'checkout').state,'none');
 assert.throws(()=>checkoutFiscalView(plan('authorized'),'checkout'));
 assert.throws(()=>checkoutFiscalView({...plan('failed'),checkout_id:'foreign'},'checkout'));
 assert.throws(()=>checkoutFiscalView({...plan('authorized_homologation'),fiscal:{state:'authorized_homologation',operation_id:'op'}},'checkout'));
});
test('cleared browser storage still allows ID-only retry with no invoice or new operation',async()=>{
 let state='failed';const f=fixture((url,input)=>{if(url.endsWith('/fiscal/reprocess'))state='authorized_homologation';return plan(state);});
 const first=await f.client.finalize('checkout',original);assert.equal(first.state,'failed');assert.equal(first.canRetry,true);
 f.data.clear();assert.equal((await f.client.read('checkout')).canRetry,true);
 const last=await f.client.retry('checkout');assert.equal(last.state,'authorized');
 const posts=f.calls.filter(v=>v.method==='POST');assert.equal(posts.length,2);assert.deepEqual(JSON.parse(posts[0].body),original);
 assert.equal(posts[1].url,'/local/v1/checkout/plans/checkout/fiscal/reprocess');assert.equal(posts[1].body,'{}');
 assert.ok(posts.every(v=>v.cache==='no-store'&&v.credentials==='omit'));
 assert.equal(f.data.size,0);
 await f.client.retry('checkout');assert.equal(f.calls.filter(v=>v.method==='POST').length,2);
});
test('pending, missing server request and cancelled sale never issue a retry POST',async()=>{
 for(const state of ['pending','recovery_required']){const f=fixture(()=>plan(state));await f.client.retry('checkout');assert.equal(f.calls.filter(v=>v.method==='POST').length,0);}
 const f=fixture(()=>({...plan('failed'),fiscal:{...plan('failed').fiscal,can_reprocess:false}}));
 assert.equal((await f.client.read('checkout')).canRetry,false);await assert.rejects(f.client.retry('checkout'),/missingRequest/);assert.ok(f.calls.every(v=>v.method==='GET'));
 const cancelled=fixture(()=>({...plan('failed'),completion:{...plan('failed').completion,sale_status:'cancelled'}}));assert.equal((await cancelled.client.read('checkout')).canRetry,false);
});
test('production is rejected locally without a POST',async()=>{
 const f=fixture(()=>plan('failed'));
 await assert.rejects(f.client.finalize('checkout',{...original,fiscal_invoice:{Environment:1}}),/invalidReply/);
 assert.equal(f.calls.length,0);
});
test('session or context change before/during a request refuses the reply and prevents POST',async()=>{
 const f=fixture(()=>plan('failed'));f.change({token:'new-login'});await assert.rejects(f.client.retry('checkout'),/context/);assert.equal(f.calls.length,0);
 let resolve;const delayed=fixture(()=>new Promise(r=>{resolve=r;}));const pending=delayed.client.read('checkout');
 await new Promise(r=>setImmediate(r));delayed.change({session:{tenant_id:'other',store_id:'store',identity_id:'operator',device_id:'device'}});resolve(plan('failed'));await assert.rejects(pending,/context/);
});
test('concurrent retry is blocked while the original request is in flight',async()=>{
 let release,hold=false;const f=fixture((url,input)=>hold&&input.method==='POST'?new Promise(r=>{release=r;}):plan('failed'));
 await f.client.finalize('checkout',original);hold=true;
 const one=f.client.retry('checkout');await new Promise(r=>setImmediate(r));
 await assert.rejects(f.client.retry('checkout'),/busy/);release(plan('authorized_homologation'));assert.equal((await one).state,'authorized');assert.equal(f.calls.filter(v=>v.method==='POST').length,2);
});
test('UI renders actual status, accessible buttons and homologation label in all five languages',()=>{
 const require=createRequire(import.meta.url),ts=require('typescript'),React=require('react'),{renderToStaticMarkup}=require('react-dom/server');
 const source=fs.readFileSync(new URL('../src/modules/pos/pages/CheckoutFiscalStatus.tsx',import.meta.url),'utf8');
 const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX,target:ts.ScriptTarget.ES2022}}).outputText;
 const mod={exports:{}};Function('require','exports','module',js)(name=>name.startsWith('react')?require(name):{},mod.exports,mod);
 for(const locale of ['pt-BR','en','es','fr','zh-CN']){
  const w=JSON.parse(fs.readFileSync(new URL(`../src/i18n/checkout96/${locale}.json`,import.meta.url)));
  const render=(view,extra={})=>renderToStaticMarkup(React.createElement(mod.exports.CheckoutFiscalStatusView,{view,busy:false,unavailable:false,demo:false,connected:true,w,onRetry(){},onRefresh(){},...extra}));
  const authorized=render({state:'authorized'});assert.ok(authorized.includes(w.checkoutFiscalAuthorized));assert.ok(authorized.includes(w.checkoutFiscalHomologation));assert.ok(!authorized.includes('>'+w.checkoutFiscalRetry+'</button>'));
  assert.match(render({state:'failed',canRetry:true}),/role="status"/);assert.ok(render({state:'failed',canRetry:true}).includes(w.checkoutFiscalRetry));
  assert.ok(render({state:'failed',canRetry:false}).includes(w.checkoutFiscalMissingRequest));assert.match(render({state:'failed',canRetry:false}),/disabled=""/);
  assert.ok(render({state:'pending'}).includes(w.checkoutFiscalPending));assert.ok(!render({state:'pending'}).includes('>'+w.checkoutFiscalRetry+'</button>'));
  assert.match(render({state:'failed',canRetry:true},{busy:true}),/disabled=""/);
 }
});

test('unavailable browser storage does not participate in ID-only retry',async()=>{
 const context={token:'session',session:{tenant_id:'t',store_id:'s',identity_id:'u',device_id:'d'}};let posts=0,state='failed';
 const client=createCheckoutFiscalClient({getCurrent:()=>context,cache:{async read(){throw Error('storage');},async write(){throw Error('storage');}},fetcher:async(url,options)=>{if(options.method==='POST'){posts++;assert.ok(url.endsWith('/fiscal/reprocess'));assert.equal(options.body,'{}');state='authorized_homologation';}return {ok:true,status:200,json:async()=>plan(state)};}}).bind(context);
 const view=await client.read('checkout');assert.equal(view.state,'failed');assert.equal(view.canRetry,true);assert.equal((await client.retry('checkout')).state,'authorized');assert.equal(posts,1);
});
