import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {checkoutFiscalView} from '../src/core/local/checkoutFiscal.mjs';
test('missing XML archive warns without changing fiscal authorization or enabling retry',()=>{
 const plan={checkout_id:'sale',completion:{sale_id:'sale',operation_id:'cash',received_cents:100,sale_status:'committed'},fiscal:{state:'authorized_homologation',key:'3'.repeat(44),intent_id:'intent',operation_id:'fiscal',archive_state:'pending'}};
 assert.equal(checkoutFiscalView(plan,'sale').state,'authorized');assert.equal(checkoutFiscalView(plan,'sale').archivePending,true);
 plan.fiscal.archive_state='unavailable';assert.equal(checkoutFiscalView(plan,'sale').archivePending,true);
 plan.fiscal.archive_state='archived';assert.equal(checkoutFiscalView(plan,'sale').archivePending,undefined);
 for(const locale of ['pt-BR','en','es','fr','zh-CN'])for(const [module,key]of[['checkout96','checkoutFiscalArchivePending'],['fiscalCustomer','nfeArchivePending']]){
  const words=JSON.parse(fs.readFileSync(new URL(`../src/i18n/${module}/${locale}.json`,import.meta.url),'utf8'));assert.ok(words[key]?.length>20);
 }
});
