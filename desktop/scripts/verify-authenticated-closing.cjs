'use strict';
module.exports=async({b,evaluate,wait,input,mark,go,click,field,src,q})=>{
 const fs=require('node:fs'),path=require('node:path'),{pathToFileURL}=require('node:url');
 const bw=(await import(pathToFileURL(path.join(src,'core/cashBags/words.mjs')).href)).cashBagWords('pt-BR'),area='.official-cash-bags';
 await go('/local/cash/register');await wait('!!document.querySelector(".official-cash-bags")','custody screen');
 if(b.tradePhase==='payment-closing-close'){
  mark('remaining cash separate custody withdrawal');await wait('!!document.querySelector(".pos-page details.finance116")','cash movement');await evaluate('document.querySelector(".pos-page details.finance116").open=true');
  await input('.pos-page details.finance116 input[inputmode=decimal]','68,50');await input('.pos-page details.finance116 input[maxlength="500"]','Remaining cash transferred to distinct sealed bag, not supplier payment');await evaluate('document.querySelector(".pos-page details.finance116 form").requestSubmit()');
  await wait('!!document.querySelector(".pos-page details.finance116 section[aria-label]")','custody withdrawal review');await evaluate('document.querySelector(".pos-page details.finance116 section[aria-label] button").click()');await wait('!!document.querySelector(".pos-page details.finance116 section[role=status] code")','custody withdrawal receipt');
  const withdrawal=await evaluate('document.querySelector(".pos-page details.finance116 section[role=status] code").textContent');
  mark('blind empty drawer after transfer');await wait('!!document.querySelector(".count107")','blind count');
  if(await evaluate('!!document.querySelector(".pos-closing-report")'))throw Error('official report exposed before count');
  await input('.count107 input[inputmode=numeric]','0');
  const cw=JSON.parse(fs.readFileSync(path.join(src,'i18n/count107/pt-BR.json'))),cl=JSON.parse(fs.readFileSync(path.join(src,'i18n/closing92/pt-BR.json')));
  await click(cw.next107,'.count107');await evaluate('document.querySelector(".count107-check input").click()');await click(cw.confirm107,'.count107');await wait('!!document.querySelector(".close92-actions")','closing review');await click(cl.reviewClose,'.close92');
  await wait('!!document.querySelector(".final108 > label.final108-check input:not(:disabled)")','final drawer review');await evaluate('document.querySelector(".final108 > label.final108-check input").click()');await wait('!document.querySelector(".final108-submit button").disabled','drawer acknowledgement');await evaluate('document.querySelector(".final108-submit button").click()');await wait('!!document.querySelector(".final108 dialog[open]")','drawer confirmation');await evaluate('document.querySelector(".final108 dialog button:last-child").click()');await wait('!!document.querySelector(".pos-closing-report")','drawer closed');
  mark('custody preparation and dispatch');await field(area,bw.withdrawal,withdrawal);await field(area,bw.seal,'AUTH-CUSTODY-001');await click(bw.review,area+' fieldset');await click(bw.confirm,area);await wait(`document.querySelector('${area} article nav')?.textContent.includes(${q(bw.dispatch)})`,'custody prepared');await click(bw.dispatch,area);await click(bw.review,area+' fieldset');await click(bw.confirm,area);await wait(`document.querySelector('${area} article h3')?.textContent.includes(${q(bw.in_transit)})`,'custody dispatched');
 }else if(b.tradePhase==='payment-closing-recover'){
  mark('explicit custody receipt recovery');await click(bw.check,area+' > aside[role=status]');
  await wait(`document.querySelector('${area} article h3')?.textContent.includes(${q(bw.received)})`,'custody recovered');
 }else{
  mark(b.tradePhase==='payment-closing-receive'?'independent blind count and review':'restored custody and review');await click(bw.list,area);await wait(`!!document.querySelector('${area} tbody button')`,'custody row');await evaluate(`document.querySelector('${area} tbody button').click()`);
  if(b.tradePhase==='payment-closing-receive'){
   await wait(`!!document.querySelector('${area} article nav')`,'independent custody actions');
   await field(area,bw.seal,'AUTH-CUSTODY-001');await field(area,bw.counted_cents,'68,00');await field(area,bw.receiveReason,'Independent physical count: fifty cents missing; discrepancy retained, not repaired');await click(bw.review,area+' fieldset');await click(bw.confirm,area);
   await wait(`!!document.querySelector('${area} > aside[role=status] button')&&!document.querySelector('${area} > aside[role=status] button').disabled`,'uncertain custody retained');
  }else{
   await wait(`document.querySelector('${area} article h3')?.textContent.includes(${q(bw.received)})&&document.querySelector('${area} article').textContent.includes('Independent physical count: fifty cents missing; discrepancy retained, not repaired')`,'restored custody explanation');
   mark('owner investigation and compensating recount through installed forms');
   const iw=(await import(pathToFileURL(path.join(src,'core/cashBags/reviewWords.mjs')).href)).reviewWords('pt-BR'),investigation=area+' section[aria-label="'+iw.title+'"]';
   await click(iw.query,investigation);await wait(`document.querySelector(${q(investigation+' [role=status]')})?.textContent===${q(iw.pending)}`,'pending custody investigation');
   await field(investigation,iw.reason,'Independent evidence reviewed by owner');await click(iw.submit,investigation);
   await wait(`document.querySelector(${q(investigation+' [role=status]')})?.textContent===${q(iw.investigating)}`,'investigation persisted');
   await input(investigation+' select','recount');await field(investigation,iw.count,'68,50');await field(investigation,iw.reason,'Disposable controlled recount: original record retained');await click(iw.submit,investigation);
   await wait(`document.querySelectorAll(${q(investigation+' ol li')}).length===2`,'compensating recount persisted');
   await input(investigation+' select','resolve');await field(investigation,iw.reason,'Owner explicitly acknowledges reviewed custody evidence');await click(iw.submit,investigation);
   await wait(`document.querySelector(${q(investigation+' [role=status]')})?.textContent===${q(iw.resolved)}&&document.querySelectorAll(${q(investigation+' ol li')}).length===3`,'investigation closed without rewriting original cash');

  }
 }
 return {businessPhase:'trade',passed:true,packagedApp:true,commercialData:false};
};
