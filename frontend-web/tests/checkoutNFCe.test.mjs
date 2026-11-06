import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {createRequire} from 'node:module';
import {createNFCeCheckoutAdapter} from '../src/core/local/checkoutNFCe.mjs';
import {validateFinal96} from '../src/core/local/checkout96.mjs';

function fixture(){
 const session={tenant_id:'tenant',store_id:'store',identity_id:'owner',device_id:'device'},current={token:'private-test-session',session};
 const item={product_id:'product',location_id:'shelf',quantity_milli:1000},calls=[];
 const setup={nature:'VENDA',presence:'1',policies:{0:'policy'},unitPrices:{0:1599},issuerAddress:{Street:'Private street',Number:'1',District:'Private district',City:'Private city'}};
 const profile={revision:2,input:{profile_id:'issuer',environment:'homologation',model:'65',crt:'1',state:'SP',timezone:'America/Sao_Paulo',require_tax_pricing:true}};
 const policy={revision:3,input:{profile_id:'issuer',profile_revision:2,active:true,rule:{id:'policy',product_id:'product',model:'65',crt:'1',origin_uf:'SP',destination_uf:'SP'},case:{operation:'sale',final_consumer:'yes'},review:{accountant_approved:false}}};
 const input={checkout:'checkout',cash:'shift',items:[item],gross:1599,discount:0,points:0,reason:'',customer:{cpf:'',club:'skip'},fiscal:'nfce'};
 const receipt={sale_id:'checkout',operation_id:'checkout',cash_session_id:'shift',status:'committed',total_cents:1599,fiscal_authorized:false,items:[{...item,item_id:'line',unit_price_cents:1599,total_cents:1599}],payments:[{payment_id:'cash',method:'cash',amount_cents:1599,status:'confirmed'}]};
 const server={checkout_id:'checkout',cash_session_id:'shift',total_cents:1599,completion:{sale_id:'checkout',operation_id:'checkout',sale_status:'committed',received_cents:2000,change_cents:401},fiscal:{state:'authorized_homologation',operation_id:'fiscal-attempt',intent_id:'intent',key:'1'.repeat(44)}};
 const state={plan:server,policy,profile,setup,current,calls,input,receipt};
 state.fetcher=async(path,options)=>{
  const body=options.body?JSON.parse(options.body):undefined;calls.push({path,...options,body});
  if(state.hook)await state.hook(path,body);
  let out;
  if(path.includes('/tax-policies/'))out=policy;
  else if(path.includes('/profiles/'))out=profile;
  else if(path.endsWith('/tax-quotes'))out={input:body,result:{total:'15.99',goods_xml:{version:1}}};
  else if(path.endsWith('/sales/quote'))out={cash_session_id:'shift',reserved:false,discounts_supported:false,loyalty_supported:false,payment_methods:['cash'],total_cents:1599,items:[{...item,unit:'unit',unit_price_cents:1599,total_cents:1599,available_milli:2000}]};
  else if(path.endsWith('/sales/checkout'))out=receipt;
  else out=state.plan;
  if(state.transform)out=state.transform(path,out);
  return new Response(JSON.stringify(out),{status:200,headers:{'Content-Type':'application/json'}});
 };
 state.adapter=createNFCeCheckoutAdapter({getCurrent:()=>current,getSetup:()=>setup,fetcher:state.fetcher,uuid:()=> 'quote',now:()=>new Date('2026-11-06T01:00:00Z')});
 state.prepare=async()=>{const quote=await state.adapter.quote(current.token,input,new AbortController().signal);return {version:1,id:'checkout',cash:'shift',source:'cart',mode:'live',total:1599,state:'quoted',parts:[{id:'cash',method:'cash',amount:1599,state:'approved',reference:''}],received:2000,fiscal:'nfce',quote,items:[item]};};
 return state;
}

test('initial cash NFC-e calls existing policies, tax quote, authoritative quote, durable plan and fiscal finalization',async()=>{
 const f=fixture(),plan=await f.prepare();assert.equal(plan.quote.accountantApproved,false);
 const response=await f.adapter.finalize(f.current.token,{plan,items:plan.items},new AbortController().signal);
 assert.equal(validateFinal96(response,plan,plan.items,'nfce').receipt.fiscal_authorized,true);
 const tax=f.calls.find(c=>c.path.endsWith('/tax-quotes'));assert.equal(tax.body.context.date,'2026-11-05');assert.equal(tax.body.policy_revision,3);assert.equal(tax.body.item.unit_price,'15.99');
 const quote=f.calls.find(c=>c.path.endsWith('/sales/quote'));assert.deepEqual(quote.body.tax_quote_operations,['quote-0']);
 const durable=f.calls.find(c=>c.path.endsWith('/checkout/plans'));assert.deepEqual(durable.body.tax_quote_operations,['quote-0']);assert.equal(durable.body.expected_total_cents,1599);
 const final=f.calls.find(c=>c.path.endsWith('/finalize-cash'));assert.equal(final.body.operation_id,plan.id);assert.equal(final.body.fiscal_invoice.Environment,2);assert.equal(final.body.received_cents,2000);assert.equal(final.body.fiscal_invoice.Key,undefined);
 assert.ok(f.calls.every(c=>!c.path.startsWith('http')));
});

test('committed sale with fiscal failure stays committed and independently reprocessable',async()=>{
 const f=fixture(),plan=await f.prepare();f.plan.fiscal={state:'failed',operation_id:'attempt',can_reprocess:true};
 const response=await f.adapter.finalize(f.current.token,{plan,items:plan.items});
 assert.equal(validateFinal96(response,plan,plan.items,'nfce').state,'committed');assert.equal(response.receipt.fiscal_authorized,false);assert.equal(response.receipt.fiscal_state,'failed');
 assert.throws(()=>validateFinal96({...response,receipt:{...response.receipt,fiscal_state:undefined}},plan,plan.items,'nfce'));
});

test('status after client restart reads original sale without invoice cache, secret or POST',async()=>{
 const f=fixture();const response=await f.adapter.status(f.current.token,'checkout');assert.equal(response.state,'committed');assert.ok(f.calls.every(c=>c.method==='GET'));
});

test('production, foreign/stale profiles, absent policy and unsupported tax mapping refuse before sale creation',async()=>{
 for(const mutate of [f=>f.profile.input.environment='production',f=>f.profile.input.model='55',f=>f.policy.input.profile_revision=1,f=>f.policy.input.rule.product_id='foreign-product',f=>f.setup.policies={},f=>f.policy.input.rule.origin_uf='RJ']){
  const f=fixture();mutate(f);await assert.rejects(()=>f.prepare());assert.equal(f.calls.some(c=>c.path.includes('/checkout/plans')),false);
 }
});

test('discount, loyalty and unconfirmed noncash parts never turn into a cash fiscal sale',async()=>{
 for(const patch of [{discount:1},{points:1},{customer:{cpf:'',club:'join'}},{fiscal:'none'}]){const f=fixture();Object.assign(f.input,patch);await assert.rejects(()=>f.prepare());assert.equal(f.calls.length,0);}
 const f=fixture(),plan=await f.prepare();plan.parts[0].method='credit';await assert.rejects(()=>f.adapter.finalize(f.current.token,{plan,items:plan.items}));assert.equal(f.calls.some(c=>c.path.endsWith('/finalize-cash')),false);
});

test('session changes between quote and finalization, including same-token new session, prevent POST',async()=>{
 const f=fixture(),plan=await f.prepare(),before=f.calls.length;f.current.session={...f.current.session};
 await assert.rejects(()=>f.adapter.finalize(f.current.token,{plan,items:plan.items}));assert.equal(f.calls.length,before);
});

test('lost finalization response never retries POST; explicit status recovers the same committed sale',async()=>{
 const f=fixture(),plan=await f.prepare();f.hook=async path=>{if(path.endsWith('/finalize-cash'))throw Error('network lost');};
 await assert.rejects(()=>f.adapter.finalize(f.current.token,{plan,items:plan.items}));f.hook=null;
 const result=await f.adapter.status(f.current.token,'checkout');assert.equal(result.receipt.sale_id,'checkout');assert.equal(f.calls.filter(c=>c.path.endsWith('/finalize-cash')).length,1);
});

test('double confirmation does not issue a second concurrent finalization',async()=>{
 const f=fixture(),plan=await f.prepare();let release;const waiting=new Promise(r=>release=r);f.hook=async path=>{if(path.endsWith('/checkout/plans'))await waiting;};
 const first=f.adapter.finalize(f.current.token,{plan,items:plan.items});await assert.rejects(()=>f.adapter.finalize(f.current.token,{plan,items:plan.items}),/busy/);release();await first;assert.equal(f.calls.filter(c=>c.path.endsWith('/finalize-cash')).length,1);
});

test('missing XML tax mapping and added tax cannot silently change the displayed total',async()=>{
 for(const result of [{total:'15.99'},{total:'16.00',goods_xml:{version:1}}]){
  const f=fixture();f.transform=(path,out)=>path.endsWith('/tax-quotes')?{...out,result}:out;
  await assert.rejects(()=>f.prepare(),/invalidReply/);assert.equal(f.calls.some(c=>c.path.includes('/checkout/plans')),false);
 }
});

test('authoritative price change is refused rather than silently charged',async()=>{
 const f=fixture();f.transform=(path,out)=>path.endsWith('/sales/quote')?{...out,total_cents:1600,items:out.items.map(i=>({...i,unit_price_cents:1600,total_cents:1600}))}:out;
 await assert.rejects(()=>f.prepare(),/checkoutNFCePriceChanged/);assert.equal(f.calls.some(c=>c.path.endsWith('/finalize-cash')),false);
});

test('context change during a response and abort after plan creation prevent finalization POST',async()=>{
 const f=fixture();f.hook=path=>{if(path.includes('/tax-policies/'))f.current.session={...f.current.session,store_id:'other'};};
 await assert.rejects(()=>f.prepare(),/context/);assert.equal(f.calls.length,1);
 const g=fixture(),plan=await g.prepare(),c=new AbortController();g.hook=path=>{if(path.endsWith('/checkout/plans'))c.abort();};
 await assert.rejects(()=>g.adapter.finalize(g.current.token,{plan,items:plan.items},c.signal));assert.equal(g.calls.some(v=>v.path.endsWith('/finalize-cash')),false);
});

test('a failed fiscal receipt never passes as authorized or as a different sale',async()=>{
 const f=fixture(),plan=await f.prepare();f.plan.fiscal={state:'failed',operation_id:'attempt',can_reprocess:true};
 f.receipt.operation_id='other-original';await assert.rejects(()=>f.adapter.finalize(f.current.token,{plan,items:plan.items}),/invalidReply/);
});

test('an interrupted prepared plan remains uncharged and can only be replaced through explicit cancellation',async()=>{
 const f=fixture();f.plan={checkout_id:'checkout',state:'prepared',cash_session_id:'shift',total_cents:1599};
 assert.equal((await f.adapter.status(f.current.token,'checkout')).state,'rejected');assert.ok(f.calls.every(c=>c.method==='GET'));
 f.transform=(path,out)=>path.endsWith('/cancel')?{...out,state:'cancelled'}:out;
 await f.adapter.cancelPlan(f.current.token,'checkout','Operator explicitly requests revision');
 const cancel=f.calls.find(c=>c.path.endsWith('/cancel'));assert.equal(cancel.body.operation_id,'checkout-edit');
 assert.equal(f.calls.some(c=>c.path.endsWith('/finalize-cash')),false);
});

test('explicit editing never cancels an already committed sale or fiscal document',async()=>{
 const f=fixture();await assert.rejects(()=>f.adapter.cancelPlan(f.current.token,'checkout','Review'),/conflict/);
 assert.ok(f.calls.every(c=>c.method==='GET'));
});

test('Focus issuer data comes from its customer registration without re-entering or overriding the address',async()=>{
 const f=fixture();f.setup.issuerAddress={};
 f.transform=(path,out)=>path.includes('/fiscal/onboarding/')?{configuration_valid:true,profile_revision:2,provider_configuration:{provider:'external',environment:'homologation',certificate_type:'A1'}}:out;
 const plan=await f.prepare();await f.adapter.finalize(f.current.token,{plan,items:plan.items});
 assert.equal(f.calls.find(c=>c.path.endsWith('/finalize-cash')).body.fiscal_invoice.Issuer,undefined);
});

test('rendered setup has no prefilled fiscal values, escapes product text and shows homologation in all five languages',()=>{
 const require=createRequire(import.meta.url),ts=require('typescript'),React=require('react'),{renderToStaticMarkup}=require('react-dom/server');
 const source=fs.readFileSync(new URL('../src/modules/pos/pages/CheckoutNFCeSetup.tsx',import.meta.url),'utf8');
 const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX,target:ts.ScriptTarget.ES2022}}).outputText;
 const mod={exports:{}};Function('require','exports','module',js)(require,mod.exports,mod);
 for(const locale of ['pt-BR','en','es','fr','zh-CN']){
  const w=JSON.parse(fs.readFileSync(new URL(`../src/i18n/checkout96/${locale}.json`,import.meta.url)));
  const props={value:{nature:'',presence:'',policies:{},issuerAddress:{}},cart:[{product:{name:'<img src=x onerror=attack>'}}],disabled:false,w,onChange(){}};
  const html=renderToStaticMarkup(React.createElement(mod.exports.default,props));
  for(const key of ['checkoutNFCeBanner','checkoutNFCeNature','checkoutNFCePolicy','checkoutNFCePresence'])assert.ok(html.includes(w[key]));
  assert.ok(html.includes('&lt;img'));assert.ok(!html.includes('<img'));
  assert.ok(!html.includes('value="VENDA"'));assert.match(html,/<option value="" selected=""/);
  const blocked=renderToStaticMarkup(React.createElement(mod.exports.default,{...props,disabled:true}));assert.match(blocked,/<fieldset disabled=""/);
 }
 assert.doesNotMatch(source,/localStorage|sessionStorage|console\.|dangerouslySetInnerHTML/);
});
