import type {ClosingDraft92,ClosingSale92,Paper92} from './closing92.mjs';
export interface ReceiptRow108 extends Paper92 {status:string;issue:string;saleNumber:number|null;signature:string;}
export interface ReceiptLedger108 {rows:ReceiptRow108[];total:number;active:number;void:number;linked:number;pending:number;}
export interface ReceiptFilter108 {query?:string;method?:string;status?:string;from?:string;to?:string;}
export function receiptStamp108(p:Paper92):string;
export function receiptLedger108(d:ClosingDraft92,sales:ClosingSale92[],complete:boolean):ReceiptLedger108;
export function filterReceipts108(rows:ReceiptRow108[],filter?:ReceiptFilter108):ReceiptRow108[];
export function sameReceipt108(d:ClosingDraft92,id:string,signature:string):Paper92;
