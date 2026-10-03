import type {LocalSession} from './localClient.mjs';
export interface SupplierInput {operation_id:string;id:string;name:string;status:'active'|'inactive'}
export interface PurchaseInput {operation_id:string;order_id:string;supplier_id:string;suggestion_id:string}
export interface Supplier {id:string;name:string;status:string}
export interface Approval {suggestion_id:string;product_id:string;name:string;unit:string;quantity_milli:number}
export interface PurchaseOrder {id:string;operation_id:string;supplier_id:string;supplier_name:string;suggestion_id:string;status:string;created_at:string;approved_by:string;approved_at:string;items:{product_id:string;sku:string;name:string;unit:string;quantity_milli:number}[]}
export type PendingPurchase={kind:'order';input:PurchaseInput}|{kind:'supplier';input:SupplierInput};
export function createPurchaseClient(fetcher?:typeof fetch):{
 suppliers(token:string,offset?:number):Promise<Supplier[]>;
 approvals(token:string,offset?:number):Promise<Approval[]>;
 orders(token:string,offset?:number):Promise<PurchaseOrder[]>;
 order(token:string,id:string):Promise<PurchaseOrder>;
 createOrder(token:string,input:PurchaseInput):Promise<{id:string;repeated:boolean}>;
 createSupplier(token:string,input:SupplierInput):Promise<{id:string;repeated:boolean}>;
};
export function validPurchaseInput(v:unknown):boolean;
export function validSupplierInput(v:unknown):boolean;
export function pendingPurchaseKey(session:LocalSession):string;
export function readPurchasePending(storage:Storage,key:string):PendingPurchase|null;
export function persistPurchasePending(storage:Storage,key:string,v:PendingPurchase):void;
export function resolvePurchasePending(client:ReturnType<typeof createPurchaseClient>,token:string,storage:Storage,key:string,send?:boolean):Promise<{state:string;order?:PurchaseOrder}|null>;
