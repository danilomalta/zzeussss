import type {LocalCapabilities} from '../local/localClient.mjs';
export const sections83:readonly string[];
export function cents83(value:string|number):number;
export function quantity83(value:string|number,signed?:boolean):number;
export function scope83(context:{tenant_id:string;store_id:string;identity_id:string;device_id:string}):string;
export function can83(cap:LocalCapabilities|null,area:string):boolean;
export function approves83(cap:LocalCapabilities|null):boolean;
export interface Sale83 {id:string;total:number;payments:{id:string;method:string;amount:number}[]}
export interface Evidence83 {paymentId:string;provider:string;terminal:string;reference:string;method:string;amount:number;confirmed:boolean}
export function reconciliation83(sales:Sale83[],evidence:Evidence83[]):{id:string;total:number;confirmed:number;pending:number;balanced:boolean;issues:string[]}[];
export function countCash83(counts:Record<string,number>):number;
export function inventoryAdjustment83(expected:number,counted:number,movements:number):{difference:number;proposed:number;expectedNow:number};
export function refundImpact83(refund:number,cost:number,recovered?:number,fees?:number):{refund:number;loss:number};
export function production83(stock:number,perBatch:number,output:number,loss?:number):{batches:number;gross:number;net:number};
export function supplierAction83(rejections:number):string;
export function compareCohort83(a:any,b:any):boolean;
export function transfer83(remaining:number,quantity:number,originLocked:boolean):{source:number;destination:number;state:string};
export function append83<T extends {id:string}>(events:T[],event:T):T[];
export function transition83(kind:string,state:string,action:string,manager?:boolean):string;
