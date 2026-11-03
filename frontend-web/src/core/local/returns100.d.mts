import type {Receipt96} from './checkout96.mjs';import type {Request94} from './till94.mjs';
export function validateReceipt100(r:unknown,id:string):Receipt96;
export function availability100(receipt:Pick<Receipt96,'sale_id'|'items'>,requests:Request94[]):(Receipt96['items'][number]&{used:number;requested:number;remaining:number;availableCents:number;blocked:boolean})[];
export function estimateReturn100(receipt:Pick<Receipt96,'sale_id'|'status'|'items'>,itemID:string,qty:number,requests:Request94[]):{amount:number;remaining:number;availableCents:number};
export function quantityText100(n:number):string;
export function receiptFingerprint100(r:Receipt96):string;
export function parseReturnQuantity100(value:string,unit:string):number;
export function filterRequests100(requests:Request94[],filters?:{query?:string;state?:string}):(Request94&{display:number})[];
