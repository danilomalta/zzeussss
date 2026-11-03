import type {ReceiptRow108} from './receipt108.mjs';
import {csv97} from './closingReview97.mjs';
import {print95} from './print95';
export const receiptStatus108=(p:ReceiptRow108,w:Record<string,string>)=>w[p.status+'R108'];
export function receiptTable108(rows:ReceiptRow108[],w:Record<string,string>,money:(n:number)=>string,date:(at:string)=>string){return [[w.at,w.method,w.amount,w.reference,w.provider,w.terminal,w.statusR108],...rows.map(p=>[date(p.at),w[p.method],money(p.amount),p.reference,p.provider,p.terminal,receiptStatus108(p,w)])];}
export function downloadReceipts108(rows:ReceiptRow108[],w:Record<string,string>,money:(n:number)=>string,date:(at:string)=>string){const url=URL.createObjectURL(new Blob([csv97(receiptTable108(rows,w,money,date))],{type:'text/csv;charset=utf-8'})),a=document.createElement('a');a.href=url;a.download='titan-comprovantes.csv';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}
export function printReceipt108(p:ReceiptRow108,w:Record<string,string>,money:(n:number)=>string,date:(at:string)=>string,lang:string){print95(w.detailsR108,[[w.method,w[p.method]],[w.amount,money(p.amount)],[w.at,date(p.at)],[w.reference,p.reference],[w.provider,p.provider],[w.terminal,p.terminal],[w.statusR108,receiptStatus108(p,w)],[w.warning97,p.issue?w[p.issue]||w.pendingR108:w.noSettlementR108]],w.noSettlementR108,{lang});}
