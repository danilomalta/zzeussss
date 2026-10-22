import type {LocalCapabilities} from '../local/localClient.mjs';import type {Supplier,Approval,PurchaseOrder} from '../local/purchaseClient.mjs';
export interface Access115{read:boolean;stock:boolean;manage:boolean;write:boolean;calculate:boolean}
export function access115(cap:LocalCapabilities|null):Access115;
export function quantity115(value:number,locale?:string):string;
export function normalize115(value:unknown):string;
export function filter115<T>(rows:T[],query:string,fields:(keyof T)[]):T[];
export function page115<T>(rows:T[]):T[];
export function error115(error:unknown):string;
export function orderPreview115(suppliers:Supplier[],approvals:Approval[],supplierID:string,suggestionID:string):{supplier:Supplier;approval:Approval};
export function ordersCSV115(rows:PurchaseOrder[],labels:{headers:string[];local:string;cancelled?:string}):string;
