import {createAgenda114} from '../payables114/client.mjs';
import {window114,filterAgenda114} from '../payables114/agenda.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {LocalAPIError} from '../local/localClient.mjs';
import {sourceAllowed} from './model.mjs';
import {sheetFinanceWords} from './financeWords.mjs';
export function sheetFinanceQuery(filters){
 try{if(typeof filters?.q!=='string'||filters.q.length>240)throw Error();return window114({from:filters.from,to:filters.to,as_of:filters.as_of});}catch{throw Error('financeSheetRange');}
}
export async function sheetFinanceSnapshot(context,filters,fetcher=globalThis.fetch){
 const input=structuredClone(filters),query=sheetFinanceQuery(input),{token,session,capabilities,signal,isCurrent}=context;
 if(typeof token!=='string'||!token||/[\r\n]/.test(token))throw new LocalAPIError(401,'permission');
 const scope=contextKey(contextOf(session,capabilities));
 function check(){signal.throwIfAborted();if(typeof isCurrent!=='function'||!isCurrent()||contextKey(contextOf(session,capabilities))!==scope)throw new LocalAPIError(401,'permission');if(!sourceAllowed('finance',capabilities))throw new LocalAPIError(403,'permission');}
 check();
 const guarded=async(url,init)=>{check();init.signal.throwIfAborted();const response=await fetcher(url,init);check();init.signal.throwIfAborted();
  if(response.ok&&!response.headers?.get('content-type')?.toLowerCase().startsWith('text/csv'))throw Error('unavailable');
  if(response.status===422)throw Error('financeSheetLimit');
  return {ok:response.ok,status:response.status,text:async()=>{const text=await response.text();check();init.signal.throwIfAborted();return text;}};
 };
 const report=await createAgenda114(token,guarded)(query,signal);check();
 const items=filterAgenda114(report.items,{query:input.q});
 if(items.length>500)throw Error('financeSheetRows');
 const words=sheetFinanceWords(context.locale||'pt-BR');
 return {columns:['finance_document_id','finance_supplier_id','supplier','finance_reference','finance_state','finance_state_label','finance_installment_id','finance_due_on','finance_principal_cents','finance_applied_cents','finance_balance_cents','finance_timing','finance_timing_label','finance_currency','finance_as_of','finance_from','finance_to'],rows:items.map(r=>[r.payable_id,r.supplier_id,r.supplier_name,r.document_reference,r.state,words[r.state],r.installment_id,r.due_on,r.amount_cents,r.cash_applied_cents,r.outstanding_cents,r.timing,words[r.timing],'BRL',query.as_of,query.from,query.to]),partial:false,mock:false,at:Date.now()};
}
