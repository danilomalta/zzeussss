import test from 'node:test';
import assert from 'node:assert/strict';
import {createSalesAnalysis} from '../src/core/salesAnalysis/source.mjs';

const legacy='product_id,sku,name,unit,quantity_milli,revenue_cents,previous_quantity_milli,previous_revenue_cents,revenue_delta_cents,change_basis,abc,from_inclusive,to_exclusive,compare_from_inclusive,compare_to_exclusive,basis';
const current=legacy+',row_type,page_limit,page_offset,page_products,total_products,source_rows,period_revenue_cents,previous_period_revenue_cents,page_complete';
test('CSV client accepts exact legacy and coverage formats but rejects unknown fields',async()=>{
 for(const header of [legacy,current,current+',unknown']){
  const session={tenant_id:'T',store_id:'S',identity_id:'U',device_id:'D',expires_unix:Math.floor(Date.now()/1000)+3600};
  const raw=header+'\n';
  const source=createSalesAnalysis({token:'synthetic-test-only',session,capabilities:{...session,permissions:['view_accounting']},isCurrent:()=>true,fetcher:async()=>new Response(raw,{headers:{'content-type':'text/csv'}})});
  const request=source.csv({from:'2026-10-01T00:00:00Z',to:'2026-10-02T00:00:00Z'});
  if(header.endsWith(',unknown'))await assert.rejects(request,/salesReportInvalid/);else assert.equal(await request,raw);
 }
});
