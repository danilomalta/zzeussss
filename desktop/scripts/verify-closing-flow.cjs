'use strict';
// UI acceptance only. No business POSTs are made by this controller.
module.exports=async({b,evaluate,wait,input,mark,go,click,field,get,src,q})=>{
 const fs=require('node:fs'),path=require('node:path');
 const bw=(await import(path.join(src,'core/cashBags/words.mjs'))).cashBagWords('pt-BR'),area='.official-cash-bags';
 await go('/local/cash/register');
 if(b.business==='close-bag'){
  mark('withdrawal for custody');await wait('!!document.querySelector(".pos-page details.finance116")','cash movement');await evaluate('document.querySelector(".pos-page details.finance116").open=true');await input('.pos-page details.finance116 input[inputmode=decimal]','10,00');await input('.pos-page details.finance116 input[maxlength="500"]','Malote sintetico para conferencia independente');await evaluate('document.querySelector(".pos-page details.finance116 form").requestSubmit()');await wait('!!document.querySelector(".pos-page details.finance116 section[aria-label]")','withdrawal review');await evaluate('document.querySelector(".pos-page details.finance116 section[aria-label] button").click()');await wait('!!document.querySelector(".pos-page details.finance116 section[role=status] code")','withdrawal receipt');const withdrawal=await evaluate('document.querySelector(".pos-page details.finance116 section[role=status] code").textContent');
  mark('blind drawer count');await wait('!!document.querySelector(".count107")','count');if(await evaluate('!!document.querySelector(".pos-closing-report")'))throw Error('official report exposed before count');
  // Enter exact denomination quantities; do not set or bypass a total.
  const values=[20000,10000,5000,2000,1000,500,200,100,50,25,10,5,1];let remaining=3198;
  for(let i=0;i<values.length;i++){const quantity=Math.floor(remaining/values[i]);remaining-=quantity*values[i];if(quantity){await evaluate(`document.querySelectorAll('.count107 input[inputmode=numeric]')[${i}].setAttribute('data-pilot-denom',${q(String(i))})`);await input('[data-pilot-denom="'+i+'"]',String(quantity))}}
  const cw=JSON.parse(fs.readFileSync(path.join(src,'i18n/count107/pt-BR.json'))),cl=JSON.parse(fs.readFileSync(path.join(src,'i18n/closing92/pt-BR.json')));await click(cw.next107,'.count107');await evaluate('document.querySelector(".count107-check input").click()');await click(cw.confirm107,'.count107');await wait('!!document.querySelector(".close92-actions")','payment review');await click(cl.reviewClose,'.close92');
  await wait('!!document.querySelector(".final108 > label.final108-check input:not(:disabled)")','final review');await evaluate('document.querySelector(".final108 > label.final108-check input").click()');await wait('!document.querySelector(".final108-submit button").disabled','final acknowledgement');await evaluate('document.querySelector(".final108-submit button").click()');await wait('!!document.querySelector(".final108 dialog[open]")','close confirmation');await evaluate('document.querySelector(".final108 dialog button:last-child").click()');await wait('!!document.querySelector(".pos-closing-report")','official closure');
  const report=await evaluate('document.querySelector(".pos-closing-report").textContent');if(!report.includes('31,98')||!report.includes('0,00'))throw Error('official closure totals missing');
  mark('sealed bag dispatch');await field(area,bw.withdrawal,withdrawal);await field(area,bw.seal,'LACRE-DESCARTAVEL-001');await click(bw.review,area+' fieldset');await click(bw.confirm,area);await wait(`document.querySelector('.official-cash-bags article nav')?.textContent.includes(${q(bw.dispatch)})`,'bag prepared');await click(bw.dispatch,area);await click(bw.review,area+' fieldset');await click(bw.confirm,area);await wait(`document.querySelector('.official-cash-bags article h3')?.textContent.includes(${q(bw.in_transit)})`,'bag dispatched');
 }else{
  mark(b.business==='receive-bag'?'independent blind bag count':'restored closing and custody');await click(bw.list,area);await wait('!!document.querySelector(".official-cash-bags tbody button")','bag row');
  if(b.business==='receive-bag'){
   // GET uses this receiver's own session, never the owner's observer token.
   const page=await get('/cash/bags?limit=50&offset=0');if(page.items.length!==1||page.items[0].declared_cents!==null||page.items[0].counted_cents!==null)throw Error('receiver saw declared amount');
  }
  await evaluate('document.querySelector(".official-cash-bags tbody button").click()');
  if(b.business==='receive-bag'){await field(area,bw.seal,'LACRE-DESCARTAVEL-001');await field(area,bw.counted_cents,'10,00');await click(bw.review,area+' fieldset');await click(bw.confirm,area);}
  await wait(`document.querySelector('.official-cash-bags article h3')?.textContent.includes(${q(bw.received)})`,'bag received');
 }
};
