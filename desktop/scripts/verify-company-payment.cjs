'use strict';
// Explicit disposable installed acceptance. Business writes use existing forms.
module.exports=async({b,evaluate,wait,input,mark})=>{
 const fs=require('node:fs'),path=require('node:path'),q=JSON.stringify;
 const src=path.resolve(__dirname,'../../frontend-web/src');
 const fw={...JSON.parse(fs.readFileSync(path.join(src,'i18n/payables113/pt-BR.json'))),...JSON.parse(fs.readFileSync(path.join(src,'i18n/payables116/pt-BR.json')))};
 const money=(await import(require('node:url').pathToFileURL(path.join(src,'core/payables113/model.mjs')).href)).money113;
 const settled=async()=>{
  await wait(`!!document.querySelector('.p116-desk > button.p116-primary')&&document.querySelector('.p116-desk > button.p116-primary').disabled&&document.querySelectorAll('.p116-desk .p113-metrics strong')[2]?.textContent===${q(money(3150,'pt-BR'))}&&document.querySelectorAll('.p116-desk .p113-metrics strong')[3]?.textContent===${q(money(0,'pt-BR'))}&&[...document.querySelectorAll('.p116-desk td')].some(e=>e.textContent==='AUTHENTICATED-PAYMENT')`,'persisted paid balance and original receipt');
 };
 const go=async route=>{await evaluate(`history.pushState({},'',${q(route)});dispatchEvent(new PopStateEvent('popstate'))`)};
 const click=async(text,area)=>{
  const e=`[...document.querySelectorAll(${q(area+' button')})].find(e=>e.textContent.trim()===${q(text)})`;
  await wait(`!!(${e})&&!(${e}).disabled`,'payment action');await evaluate(`(${e}).click()`);
 };
 let seq=0;
 const field=async(area,text,value)=>{
  const e=`[...document.querySelectorAll(${q(area+' label')})].find(e=>[...e.childNodes].filter(n=>n.nodeType===3).map(n=>n.textContent).join('').trim()===${q(text)})?.querySelector('input,textarea,select')`;
  await wait(`!!(${e})&&!(${e}).disabled`,'payment field');const id=String(++seq);
  await evaluate(`(${e}).setAttribute('data-payment-field',${q(id)})`);await input('[data-payment-field="'+id+'"]',value);
 };
 if(b.tradePhase==='payment-approve'){
  mark('independent authenticated payable approval');await go('/local/payables');
  await wait('!!document.querySelector(".payable-documents")','payable approval screen');await evaluate('document.querySelector(".payable-documents").open=true');
  await wait('!!document.querySelector(".payable-documents tbody button")','payable approval row');await evaluate('document.querySelector(".payable-documents tbody button").click()');
  await click('Aprovar','.payable-documents');await field('.payable-documents','Motivo','Independent review of authenticated source and accepted quantity');
  await click('Revisar','.payable-documents form');await wait('!!document.querySelector(".payable-documents form input[type=checkbox]")','approval explicit review');
  await evaluate('document.querySelector(".payable-documents form input[type=checkbox]").click()');await click('Confirmar registro','.payable-documents form');
  await wait('!document.querySelector(".payable-documents form")','approval persisted');
 }else if(b.tradePhase==='payment-submit'){
  mark('authenticated payable cash opening');await go('/local/pos');
  await wait('!!document.querySelector(".pos-cash form input")&&!document.querySelector(".pos-cash form input").disabled','cash opening');await input('.pos-cash form input','100,00');
  await evaluate('document.querySelector(".pos-cash form").requestSubmit()');await wait('!!document.querySelector(".checkout91-close-register")','cash opened');
  mark('authenticated payable explicit withdrawal');await wait('!!document.querySelector(".pos-page details.finance116")','cash movement');await evaluate('document.querySelector(".pos-page details.finance116").open=true');
  await input('.pos-page details.finance116 input[inputmode=decimal]','31,50');await input('.pos-page details.finance116 input[maxlength="500"]','Explicit payment of accepted goods');
  await evaluate('document.querySelector(".pos-page details.finance116 form").requestSubmit()');await wait('!!document.querySelector(".pos-page details.finance116 section[aria-label]")','withdrawal reviewed');
  await evaluate('document.querySelector(".pos-page details.finance116 section[aria-label] button").click()');await wait('!!document.querySelector(".pos-page details.finance116 section[role=status]")','withdrawal persisted');
  mark('authenticated payable explicit payment');await go('/local/payables?document=authenticated-payable');
  await wait('!!document.querySelector(".p116-desk > button.p116-primary")&&!document.querySelector(".p116-desk > button.p116-primary").disabled','cash payment');await evaluate('document.querySelector(".p116-desk > button.p116-primary").click()');
  await wait('!!document.querySelector(".p116-desk dialog input[type=radio]")','withdrawal selection');await evaluate('document.querySelector(".p116-desk dialog input[type=radio]").click()');
  await field('.p116-desk dialog',fw.recipient,'Synthetic supplier representative');await field('.p116-desk dialog',fw.reference,'AUTHENTICATED-PAYMENT');
  await evaluate('document.querySelector(".p116-desk dialog form").requestSubmit()');await wait('!!document.querySelector(".p116-desk dialog input[type=checkbox]")','payment explicit review');
  await evaluate('document.querySelector(".p116-desk dialog input[type=checkbox]").click()');await evaluate('document.querySelector(".p116-desk dialog form").requestSubmit()');
  await wait('!!document.querySelector(".finance116 > .p113-warning")','uncertain payment retained');
 }else if(b.tradePhase==='payment-recover'){
  mark('authenticated payment explicit recovery');await go('/local/payables?document=authenticated-payable');
  await wait('!!document.querySelector(".finance116 > .p113-warning")','original payment retained');await click(fw.consult,'.finance116 > .p113-warning');
  await wait('!document.querySelector(".finance116 > .p113-warning")','original receipt recovered');await settled();
 }else if(b.tradePhase==='payment-restored'){
  mark('authenticated payment restored screen');await go('/local/payables?document=authenticated-payable');
  await wait('!!document.querySelector(".p116-desk")','restored payable');
  await settled();
 }else throw Error('unsupported payment phase');
 return {businessPhase:'trade',passed:true,packagedApp:true,commercialData:false};
};
