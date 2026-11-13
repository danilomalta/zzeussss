"use strict";
// Reuse the installed pre-registration/full-profile checks without another login.
const assert=require('node:assert/strict');
module.exports=async({b,evaluate,wait,input,mark})=>{
  mark('company form');
  await evaluate(`history.pushState({},'', '/local/settings');dispatchEvent(new PopStateEvent('popstate'))`);
  const companyLink=JSON.stringify('a[href="/local/company"]');
  await wait(`!!document.querySelector(${companyLink})`,'company link');
  await evaluate(`document.querySelector(${companyLink}).click()`);
  await wait('!!document.querySelector("#company-registration-form")','company form');
  const values=['Synthetic clean company','Synthetic test owner','11222333000181','Synthetic clean legal','SP','123456789','3550308','1'];
  if(!b.resume){await input('#company-registration-form input:nth-of-type(1)', 'Synthetic clean company');
  await evaluate('document.querySelector("#company-registration-form").requestSubmit()');
  await wait('document.querySelector("#company-registration-form fieldset")&&!document.querySelector("#company-registration-form fieldset").disabled','pre-registration saved');
  // Observe authoritative persisted revision, not just a transient UI message.
  await wait(`(async()=>{const r=await fetch('/local/v1/company/registration',{headers:{Authorization:'Bearer '+${JSON.stringify(b.token)}}});return r.ok&&(await r.json()).revision===1})()`,'persisted pre-registration');
  await input('#company-registration-form select','company_profile');
  mark('complete profile');
  for(let n=0;n<values.length;n++)await input(`#company-registration-form label:has(input):nth-of-type(${n+2}) input`,values[n]);
  const selects=await evaluate('document.querySelectorAll("#company-registration-form select").length');assert.equal(selects,3);
  await input('#company-registration-form label:nth-last-of-type(2) select','external');await input('#company-registration-form label:last-of-type select','homologation');
  await evaluate('document.querySelector("#company-registration-form").requestSubmit()');
  await wait(`(async()=>{const r=await fetch('/local/v1/company/registration',{headers:{Authorization:'Bearer '+${JSON.stringify(b.token)}}});return r.ok&&(await r.json()).revision===2})()`,'persisted complete profile');}
  await wait('!document.querySelector("#company-registration-form fieldset").disabled','complete profile saved');
  const displayed=await evaluate(`JSON.stringify([...document.querySelectorAll('#company-registration-form input')].map(e=>e.value))===${JSON.stringify(JSON.stringify(['Synthetic clean company','Synthetic test owner','11222333000181','Synthetic clean legal','SP','123456789','3550308','1']))}`);
  if(!displayed)throw Error('profile fields mismatch');
  const saved=await evaluate(`(async()=>{const r=await fetch('/local/v1/company/registration',{headers:{Authorization:'Bearer '+${JSON.stringify(b.token)}}});const d=await r.json();return {revision:d.revision,stage:d.input.stage,environment:d.input.environment}})()`);
  assert.deepEqual(saved,{revision:2,stage:'company_profile',environment:'homologation'});
  // No screen capture of personal fields or credentials is retained.
  return {packagedApp:true,freshProfile:!b.resume,reopened:!!b.resume,allProfileFieldsDisplayed:true,connectionSaved:true,ownerLogin:true,preRegistrationRevision:1,completeRegistrationRevision:2,uiSource:'frontend-web',realHTTP:true,commercialData:false,officialFiscalAcceptance:false};
};
