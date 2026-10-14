import {localClient,useLocalSession} from '../local/useLocalSession';
import type {SaleReceipt,SaleSummary} from '../local/localClient.mjs';
import {sum} from './engine.mjs';
export async function salesPage(token:string,offset:number):Promise<SaleSummary[]>{const rows=await localClient.history(token,offset);if(rows.some(r=>!Number.isFinite(Date.parse(r.committed_at))))throw Error('invalid');return rows;}
export async function paymentsForPage(token:string,rows:SaleSummary[]):Promise<{receipts:SaleReceipt[];total:number}>{
 const receipts:SaleReceipt[]=[];for(const row of rows){if(useLocalSession.getState().token!==token)throw Error('session');receipts.push(await localClient.receipt(token,row.sale_id));}
 if(useLocalSession.getState().token!==token)throw Error('session');
 return {receipts,total:sum(receipts.filter(r=>r.status==='committed').map(r=>r.total_cents))};
}
// No speculative POST route or fake PSP/SEFAZ response is used by this release.
