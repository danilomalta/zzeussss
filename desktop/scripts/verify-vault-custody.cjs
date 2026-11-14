'use strict';
module.exports=async({b,evaluate,wait,input,mark,go,click,field,src,q})=>{
 const path=require('node:path'),{pathToFileURL}=require('node:url');
 const w=(await import(pathToFileURL(path.join(src,'core/cashVaults/words.mjs')).href)).cashVaultWords('pt-BR'),money=(await import(pathToFileURL(path.join(src,'core/cashJournal/model.mjs')).href)).journalMoney,area='.official-cash-vaults';
 const selectVault=async()=>{if(await evaluate(`document.querySelector('${area} button[aria-expanded]')?.getAttribute('aria-expanded')!=='true'`))await click(w.query,area);await wait(`document.querySelector('${area} tbody tr')?.textContent.includes('Custody deposit vault')`,'queried vault row');await evaluate(`document.querySelector('${area} tbody button').click()`);};
 await go('/local/cash/register');await wait(`!!document.querySelector('${area}')`,'vault screen');
 if(b.tradePhase==='payment-closing-vault-create'){
  mark('create empty physical vault');await click(w.createVault,area);await field(area,w.name,'Custody deposit vault');await click(w.review,area+' fieldset');await click(w.confirm,area);
  await wait(`document.querySelector('${area} > p[role=status]')?.textContent===${q(w.confirmed)}`,'vault creation confirmed');await selectVault();
  await wait(`document.querySelector('${area} article h3')?.textContent.includes(${q(money(0,'pt-BR'))})`,'empty vault persisted');
 }else if(b.tradePhase==='payment-closing-vault-recover'){
  mark('explicit vault receipt recovery after native restart');await click(w.check,area+' > aside[role=status]');
  await wait(`document.querySelector('${area} > p[role=status]')?.textContent===${q(w.confirmed)}`,'original vault receipt confirmed');await selectVault();
  await wait(`document.querySelector('${area} article h3')?.textContent.includes(${q(money(6800,'pt-BR'))})`,'original counted amount recovered');
 }else{
  await click(w.query,area);await wait(`!!document.querySelector('${area} table tbody')`,'vault listing');
  if(b.tradePhase==='payment-closing-vault-deposit'){
   mark('receiver deposits original counted bag, not investigation recount');await click(w.pendingBags,area+' > nav');
   await wait(`document.querySelector('${area} tbody tr')?.textContent.includes('AUTH-CUSTODY-001')`,'received bag pending deposit');
   const id=await evaluate(`document.querySelector('${area} tbody tr td').textContent`);
   await click(w.vaults,area+' > nav');await wait(`document.querySelector('${area} tbody tr')?.textContent.includes('Custody deposit vault')`,'vault destination');await evaluate(`document.querySelector('${area} tbody button').click()`);
   await field(area,w.bag_id,id);await click(w.review,area+' fieldset');await click(w.confirm,area);
   await wait(`!!document.querySelector('${area} > aside[role=status] button')&&!document.querySelector('${area} > aside[role=status] button').disabled`,'uncertain deposit retained without resend');
  }else{
   mark('restored vault balance and immutable deposit statement');await wait(`document.querySelector('${area} tbody tr')?.textContent.includes('Custody deposit vault')`,'restored vault');await evaluate(`document.querySelector('${area} tbody button').click()`);
   await wait(`document.querySelector('${area} article h3')?.textContent.includes(${q(money(6800,'pt-BR'))})&&document.querySelector('${area} article ol li')?.textContent.includes(${q(money(6800,'pt-BR'))})`,'restored counted deposit and statement');
  }
 }
 return {businessPhase:'trade',passed:true,packagedApp:true,commercialData:false};
};
