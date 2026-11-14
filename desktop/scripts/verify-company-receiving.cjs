"use strict";
// Disposable acceptance; real existing order/receiving forms, no business mocks.
module.exports=async function({evaluate,wait,input,mark}){
 const path=require('node:path'),q=JSON.stringify;
 const w=(await import(require('node:url').pathToFileURL(path.join(__dirname,'../../frontend-web/src/core/deliveryCycle/words.mjs')).href)).deliveryWords('pt-BR');
 const click=async(text,area='.delivery-cycle')=>{
  const e=`[...document.querySelectorAll(${q(area+' button')})].find(e=>e.textContent.trim()===${q(text)})`;
  await wait(`!!(${e})&&!(${e}).disabled`,'authenticated receiving action');await evaluate(`(${e}).click()`);
 };
 let seq=0;
 const field=async(text,value)=>{
  const e=`[...document.querySelectorAll('.delivery-cycle label')].find(e=>[...e.childNodes].filter(n=>n.nodeType===3).map(n=>n.textContent).join('').trim()===${q(text)})?.querySelector('input,textarea,select')`;
  // Async location lookup must finish before selecting: setting a missing option
  // silently clears the field and native required validation blocks submission.
  await wait(`!!(${e})&&!(${e}).disabled&&((${e}).tagName!=='SELECT'||[...(${e}).options].some(o=>o.value===${q(value)}))`,'authenticated receiving field');const id=String(++seq);
  await evaluate(`(${e}).setAttribute('data-authenticated-field',${q(id)})`);await input('[data-authenticated-field="'+id+'"]',value);
 };
 const commit=async()=>{await click(w.review);await click(w.send);await wait('!document.querySelector(".delivery-cycle .supplier-decision-review")','authenticated receiving saved')};
 mark('authenticated receiving route');await evaluate(`history.pushState({},'', '/local/orders');window.dispatchEvent(new PopStateEvent('popstate'))`);
 await wait('!!document.querySelector(".o115-table tbody button")','authenticated order row');await evaluate('document.querySelector(".o115-table tbody button").click()');
 await wait('!!document.querySelector(".delivery-cycle")','authenticated delivery');
 const now=new Date(),at=new Date(now.getTime()-now.getTimezoneOffset()*60000).toISOString().slice(0,19),today=at.slice(0,10);
 mark('authenticated delivery plan');await click(w.plan);
 for(const [key,value] of Object.entries({delivery_id:'authenticated-trip',delivery_reference:'Synthetic signed-order delivery',planned_milli:'3',reason:'Explicit delivery under authenticated supplier decision'}))await field(w[key],value);
 await commit();
 await wait('document.querySelector(".delivery-cycle > label select")?.options.length>1','authenticated delivery selection');
 await input('.delivery-cycle > label select','authenticated-trip');
 for(const action of ['load','arrive']){
  mark('authenticated delivery '+action);await click(w[action]);await field(w.evidence_reference,'Synthetic physical observation');await field(w.observedLocal,at);await field(w.reason,'Explicit observation, not automatic supplier evidence');await commit();
 }
 mark('authenticated partial receiving');await click(w.receive);
 for(const [key,value] of Object.entries({location_id:'receiving-room',delivered_milli:'3',accepted_milli:'2.5',reason:'Half unit refused due to observed damage'}))await field(w[key],value);
 await commit();
 mark('authenticated receiving obligation');await click(w.finance);
 for(const [key,value] of Object.entries({payable_id:'authenticated-payable',document_reference:'SYNTHETIC-RECEIPT',source_reference:'AUTHENTICATED-TRIP',description:'Explicit document for accepted quantity only',issued_on:today,total_cents:'31,50'}))await field(w[key],value);
 await field(w.due_on+' 1',today);await field(w.amount_cents+' 1','31,50');await commit();
 // Remount the actual screens: reads must use the persisted server result.
 mark('authenticated persisted receipt');await evaluate(`history.pushState({},'', '/local/home');window.dispatchEvent(new PopStateEvent('popstate'))`);
 await wait('!document.querySelector(".delivery-cycle")','receiving unmounted');
 await evaluate(`history.pushState({},'', '/local/orders');window.dispatchEvent(new PopStateEvent('popstate'))`);
 await wait('!!document.querySelector(".o115-table tbody button")','persisted authenticated order');await evaluate('document.querySelector(".o115-table tbody button").click()');
 await wait('document.querySelector(".delivery-cycle > label select")?.options.length>1','persisted authenticated delivery');await input('.delivery-cycle > label select','authenticated-trip');
 await field(w.originQuery,'authenticated-payable');await evaluate(`[...document.querySelectorAll('.delivery-cycle form')].find(f=>f.querySelector('input[maxlength="128"]')).requestSubmit()`);
 await wait(`[...document.querySelectorAll('.delivery-cycle dl dd')].some(e=>e.textContent==='authenticated-payable')&&[...document.querySelectorAll('.delivery-cycle dl dd')].some(e=>e.textContent==='authenticated-trip')`,'persisted payable source');
 return {businessPhase:'trade',passed:true,packagedApp:true,authenticatedReceiving:true,commercialData:false};
};
