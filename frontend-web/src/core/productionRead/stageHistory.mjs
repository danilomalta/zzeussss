import {executionInput,executionResult} from './execution.mjs';
const fail=()=>{throw Error('productionInvalid');};
const id=s=>typeof s==='string'&&s===s.trim()&&s.length>0&&new TextEncoder().encode(s).length<=128&&!/[\x00\r\n]/.test(s);
export function productionStageHistory(v,orderID){
 const rows=v?.items;
 if(!id(orderID)||!Array.isArray(rows)||rows.length>50)fail();
 let previous=0;const operations=new Set();
 for(const row of rows){
  if(!row||!Number.isSafeInteger(row.sequence)||row.sequence<=previous||!['operation_id','device_id','actor_id'].every(k=>id(row[k]))||operations.has(row.operation_id)||!['configured','running','completed'].includes(row.kind)||typeof row.created_at!=='string'||!Number.isFinite(Date.parse(row.created_at)))fail();
  const kind=row.kind==='configured'?'stage_plan':'stage_state',input=executionInput(kind,row.request);
  if(input.order_id!==orderID||input.operation_id!==row.operation_id||input.reason!==row.reason||(kind==='stage_state'&&input.status!==row.kind))fail();
  executionResult(kind,input,row.result);
  if(row.result.repeated!==false)fail();
  previous=row.sequence;operations.add(row.operation_id);
 }
 return rows;
}
