import test from 'node:test';import assert from 'node:assert/strict';
import {checkoutFiscalView} from '../src/core/local/checkoutFiscal.mjs';
test('provider contingency remains visibly pending, never authorized by a local receipt',()=>{
 const plan={checkout_id:'sale',completion:{sale_id:'sale',operation_id:'complete',received_cents:100},fiscal:{state:'pending',operation_id:'fiscal',error_code:'focus_contingency_pending'}};
 const view=checkoutFiscalView(plan,'sale');assert.equal(view.state,'pending');assert.equal(view.contingencyPending,true);
});

import fs from 'node:fs';
test('accountant export offers a customer-selected destination without external transmission or storage',()=>{
 const source=fs.readFileSync(new URL('../src/modules/tenant/pages/enterprise89/FiscalAccountingExport.tsx',import.meta.url),'utf8');
 assert.match(source,/showSaveFilePicker/);assert.match(source,/\/local\/v1\/fiscal\/accounting-export/);assert.doesNotMatch(source,/localStorage|sessionStorage|console\.|sendBeacon|https?:\/\//);assert.match(source,/generation.current/);
});
