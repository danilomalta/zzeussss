"use strict";
// Disposable acceptance only. Every business write is made by the actual form.
module.exports=async function({b,evaluate,wait,input,mark}){
 if(b.tradePhase==='receive')return require('./verify-company-receiving.cjs')({b,evaluate,wait,input,mark});
 const path=require('node:path'),fs=require('node:fs');
 const w=JSON.parse(fs.readFileSync(path.join(__dirname,'../../frontend-web/src/i18n/companyTrade66/pt-BR.json')));
 const q=JSON.stringify;
 const click=async(text,area='.trade66')=>{
  const e=`[...document.querySelectorAll(${q(area+' button')})].find(e=>e.textContent.trim()===${q(text)})`;
  await wait(`!!(${e})&&!(${e}).disabled`,'trade action');await evaluate(`(${e}).click()`);
 };
 const field=async(text,value)=>{
  const e=`[...document.querySelectorAll('.trade66 label')].find(e=>[...e.childNodes].filter(n=>n.nodeType===3).map(n=>n.textContent).join('').trim()===${q(text)})?.querySelector('input,textarea,select')`;
  await wait(`!!(${e})`,'trade field');await evaluate(`(${e}).setAttribute('data-trade-field','current')`);
  await input('[data-trade-field="current"]',value);await evaluate(`document.querySelector('[data-trade-field="current"]').removeAttribute('data-trade-field')`);
 };
 const kind=async value=>{
  await input('.trade66 > section > form > label select',value);
 };
 const commit=async uncertain=>{
  await click(w.prepare);await wait('!!document.querySelector(".trade66-review input[type=checkbox]")','trade review');
  await evaluate('document.querySelector(".trade66-review input[type=checkbox]").click()');
  await click(w.submit,'.trade66-review');
  await wait(uncertain?'!!document.querySelector(".trade66-pending")&&!document.querySelector(".trade66[aria-busy=true]")':'!document.querySelector(".trade66-review")&&!document.querySelector(".trade66-pending")','trade result');
 };
 mark('trade route');await evaluate(`location.hash='';history.pushState({},'', '/local/company-trade');window.dispatchEvent(new PopStateEvent('popstate'))`);
 await wait('!!document.querySelector(".trade66 > section > form > label select")','trade page');
 if(b.tradePhase==='recover'){
  mark('trade recovery');await wait('!!document.querySelector(".trade66-pending")','trade pending');
  await click(w.recover,'.trade66-pending');await wait('!document.querySelector(".trade66-pending")','trade recovered');
 }else{
  if(b.remoteIdentity){
   mark('trade trust');await kind('trust');await field(w.peerID,b.peer);await field(w.role,b.partyRole);
   if(b.partyRole==='supplier')await field(w.supplierID,'supplier');
   await field(w.remoteIdentity,JSON.stringify(b.remoteIdentity));await click(w.verify);
   await wait('!!document.querySelector(".trade66 > section form pre")','trade fingerprint');
   await field(w.reference,'Synthetic independent owner verification');await commit(false);
  }
  mark('trade peers');await click(w.refresh);await wait('!!document.querySelector(".trade66-scroll tbody tr")','trade peer list');
  await kind(b.tradePhase==='export'?'export':b.tradePhase==='decide'?'import':'apply');
  await field(w.peer,b.peer);
  if(b.tradePhase==='export'){
   mark('trade export');await field(w.target,'order');await field(w.reason,'Synthetic explicitly approved order');await commit(false);
  }else{
   await field(w.envelope,JSON.stringify(b.envelope));
   if(b.tradePhase==='apply')await field(w.target,'order');
   mark('trade import');await commit(b.tradePhase==='apply');
   if(b.tradePhase==='decide'){
    mark('trade inspect received order');await field(w.inspect,b.messageID);await click(w.load,'.trade66-actions:has(input[maxlength="128"])');
    await wait(`document.querySelector('.trade66 > section > form > label select')?.value==='decide'`,'trade selected order');
    await field(w.quantity,'3');await field(w.promised,b.promised);await field(w.reason,'Synthetic supplier explicitly confirmed partial quantity');
    mark('trade supplier decision');await commit(false);
   }
  }
 }
 return {businessPhase:'trade',passed:true,packagedApp:true,authenticatedCompanies:true,commercialData:false};
};
