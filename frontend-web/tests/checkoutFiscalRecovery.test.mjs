import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {createRequire} from 'node:module';
import {createCheckoutFiscalClient} from '../src/core/local/checkoutFiscal.mjs';

const plan=(state='recovery_required')=>({checkout_id:'original-sale',completion:{sale_id:'original-sale',operation_id:'original-finalization',received_cents:2000,sale_status:'committed'},fiscal:{state,operation_id:'original-fiscal',intent_id:'original-intent',key:'3'.repeat(44)}});
function fixture(reply){
 let current={token:'synthetic-session',session:{tenant_id:'t',store_id:'s',identity_id:'owner',device_id:'d'}};
 const calls=[];
 const client=createCheckoutFiscalClient({getCurrent:()=>current,fetcher:async(url,options)=>{
  calls.push({url,...options});return {ok:true,status:200,json:async()=>reply(url,options)};
 }}).bind(current);
 return {client,calls,change:patch=>{current={...current,...patch};}};
}
const tick=()=>new Promise(resolve=>setImmediate(resolve));

test('recovery is explicit, server-owned and ID-only; reading never sends a command',async()=>{
 const f=fixture((url,options)=>plan(options.method==='POST'?'authorized_homologation':'recovery_required'));
 const view=await f.client.read('original-sale');assert.equal(view.canRecover,true);assert.equal(view.canRetry,false);
 assert.equal(f.calls.filter(c=>c.method==='POST').length,0);
 assert.equal((await f.client.recover('original-sale')).state,'authorized');
 const posts=f.calls.filter(c=>c.method==='POST');assert.equal(posts.length,1);
 assert.equal(posts[0].url,'/local/v1/checkout/plans/original-sale/fiscal/reprocess');assert.equal(posts[0].body,'{}');
 assert.equal(posts[0].credentials,'omit');assert.equal(posts[0].cache,'no-store');
 assert.ok(!posts[0].body.includes('fiscal_invoice'));assert.ok(!posts[0].body.includes('operation_id'));
});

test('other pending states, cancellation and uncommitted sales cannot recover',async()=>{
 for(const state of ['pending','uncertain','reserved','sending','response_received','not_sent','recovery_pending','protocol_received_homologation','failed','cancelled_homologation','cancellation_failed','authorized_homologation']){
  const f=fixture(()=>plan(state));await f.client.recover('original-sale');
  assert.ok(f.calls.every(c=>c.method==='GET'),state);
 }
 for(const status of ['cancelled','pending']){
  const f=fixture(()=>{const p=plan();p.completion.sale_status=status;return p;});
  await f.client.recover('original-sale');assert.ok(f.calls.every(c=>c.method==='GET'));
 }
});

test('context or login changing during the prerequisite read prevents recovery POST',async()=>{
 for(const patch of [{token:'another-session'},{session:{tenant_id:'other',store_id:'s',identity_id:'owner',device_id:'d'}}]){
  let release;const f=fixture(()=>new Promise(resolve=>release=resolve));
  const pending=f.client.recover('original-sale');await tick();f.change(patch);release(plan());
  await assert.rejects(pending,/context/);assert.equal(f.calls.filter(c=>c.method==='POST').length,0);
 }
});

test('aborting prerequisite read prevents the recovery command',async()=>{
 let release;const f=fixture(()=>new Promise(resolve=>release=resolve)),abort=new AbortController();
 const pending=f.client.recover('original-sale',abort.signal);await tick();abort.abort();release(plan());
 await assert.rejects(pending,/aborted/);assert.ok(f.calls.every(c=>c.method==='GET'));
});

test('concurrent clicks share the checkout lock and completed recovery is not repeated',async()=>{
 let release,state='recovery_required';
 const f=fixture((url,options)=>options.method==='POST'?new Promise(resolve=>release=()=>{state='authorized_homologation';resolve(plan(state));}):plan(state));
 const first=f.client.recover('original-sale');await tick();
 await assert.rejects(f.client.recover('original-sale'),/busy/);
 await assert.rejects(f.client.retry('original-sale'),/busy/);
 release();assert.equal((await first).state,'authorized');
 await f.client.recover('original-sale');assert.equal(f.calls.filter(c=>c.method==='POST').length,1);
});

test('lost recovery response never causes automatic retry; next click reads the new state',async()=>{
 let state='recovery_required';const f=fixture((url,options)=>{
  if(options.method==='POST'){state='authorized_homologation';throw Error('lost-response');}
  return plan(state);
 });
 await assert.rejects(f.client.recover('original-sale'),/lost-response/);
 assert.equal(f.calls.filter(c=>c.method==='POST').length,1);
 assert.equal((await f.client.recover('original-sale')).state,'authorized');
 assert.equal(f.calls.filter(c=>c.method==='POST').length,1);
});

test('recovery controls are explicit, translated and absent for ordinary pending/demo',()=>{
 const require=createRequire(import.meta.url),ts=require('typescript'),React=require('react'),{renderToStaticMarkup}=require('react-dom/server');
 const source=fs.readFileSync(new URL('../src/modules/pos/pages/CheckoutFiscalStatus.tsx',import.meta.url),'utf8');
 const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX,target:ts.ScriptTarget.ES2022}}).outputText;
 const mod={exports:{}};Function('require','exports','module',js)(name=>name.startsWith('react')?require(name):{},mod.exports,mod);
 for(const locale of ['pt-BR','en','es','fr','zh-CN']){
  const w=JSON.parse(fs.readFileSync(new URL(`../src/i18n/checkout96/${locale}.json`,import.meta.url)));
  assert.ok(w.checkoutFiscalRecover&&w.checkoutFiscalRecoverHelp);
  const render=(extra={})=>renderToStaticMarkup(React.createElement(mod.exports.CheckoutFiscalStatusView,{view:{state:'pending',canRecover:true},busy:false,unavailable:false,demo:false,connected:true,w,onRetry(){},onRefresh(){},onRecover(){},...extra}));
  const html=render();assert.ok(html.includes(w.checkoutFiscalRecover));assert.ok(html.includes(w.checkoutFiscalRecoverHelp));
  assert.ok(!html.includes('>'+w.checkoutFiscalRetry+'</button>'));
  for(const extra of [{view:{state:'pending'}},{demo:true},{canReprocess:false},{onRecover:undefined}])assert.ok(!render(extra).includes('>'+w.checkoutFiscalRecover+'</button>'));
  for(const extra of [{busy:true},{connected:false}])assert.match(render(extra),/disabled=""/);
 }
});
