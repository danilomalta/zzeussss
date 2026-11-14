'use strict';
module.exports=async({b,evaluate,wait,mark,go,click,field,src,q})=>{
 const path=require('node:path'),{pathToFileURL}=require('node:url'),w=(await import(pathToFileURL(path.join(src,'core/cashVaults/words.mjs')).href)).cashVaultWords('pt-BR'),money=(await import(pathToFileURL(path.join(src,'core/cashJournal/model.mjs')).href)).journalMoney,area='.official-cash-vaults';
 await go('/local/cash/register');await wait(`!!document.querySelector('${area}')`,'vault supply screen');
 if(b.tradePhase==='payment-closing-supply-recover'){
  mark('explicit supply receipt recovery after native restart');await click(w.check,area+' > aside[role=status]');await wait(`document.querySelector('${area} > p[role=status]')?.textContent===${q(w.confirmed)}`,'original supply receipt confirmed');
 }
 await click(w.query,area);await wait(`document.querySelector('${area} tbody tr')?.textContent.includes('Supply reserve vault')`,'queried supply vault');await evaluate(`document.querySelector('${area} tbody button').click()`);
 if(b.tradePhase==='payment-closing-supply-submit'){
  mark('owner supplies open drawer from counted vault money');await click(w.supply,area+' article nav');await field(area,w.cash_session_id,'supply-target-turn');await field(area,w.amount_cents,'20,00');await field(area,w.reason,'Explicit drawer replenishment from counted vault source');await click(w.review,area+' fieldset');await click(w.confirm,area);
  await wait(`!!document.querySelector('${area} > aside[role=status] button')&&!document.querySelector('${area} > aside[role=status] button').disabled`,'uncertain supply retained');
 }else{
  mark('vault debit and original target drawer visible');await wait(`document.querySelector('${area} article h3')?.textContent.includes(${q(money(4800,'pt-BR'))})&&[...document.querySelectorAll('${area} article ol li')].some(e=>e.textContent.includes('supply-target-turn')&&e.textContent.includes(${q(money(-2000,'pt-BR'))}))`,'exact vault debit and linked cash supply');
 }
 return {businessPhase:'trade',passed:true,packagedApp:true,commercialData:false};
};
