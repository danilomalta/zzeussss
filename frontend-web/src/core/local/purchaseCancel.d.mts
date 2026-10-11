import type {LocalSession} from './localClient.mjs';
export interface OrderItem {product_id:string;sku:string;name:string;unit:string;quantity_milli:number}
export interface Cancellation {operation_id:string;device_id:string;actor_id:string;reason:string;created_at:string}
export interface OrderTrace {order:{id:string;operation_id:string;supplier_id:string;supplier_name:string;suggestion_id:string;status:string;receiving_status:string;created_at:string;approved_by:string;approved_at:string;items:OrderItem[]};creation:{kind:string;operation_id:string;device_id:string;actor_id:string;created_at:string};approval:{suggestion_id:string;operation_id:string;device_id:string;reviewer_id:string;decision:string;reason:string;decided_at:string};cancellation?:Cancellation;receiving_authorization?:Cancellation & {reference:string}}
export interface CancelInput {operation_id:string;order_id:string;expected_status:'local_not_sent';reason:string}
export interface CancelResult {order_id:string;status:'cancelled';repeated:boolean;cancellation:Cancellation}
export interface CancelReceipt {input:CancelInput;result:CancelResult}
export type CancelOperation={kind:'cancel';input:CancelInput};
export type PendingCancel=CancelOperation & {uncertain:boolean};
export function validOrderTrace(v:unknown,id:string):boolean;
export function validCancelInput(kind:string,v:unknown):boolean;
export function validCancelReceipt(v:unknown,opID:string):boolean;
export function preparePurchaseCancel(v:OrderTrace,reason:string,opID:string):CancelInput;
export function createPurchaseCancelClient(fetcher?:typeof fetch):{trace(token:string,id:string):Promise<OrderTrace>;receipt(token:string,opID:string):Promise<CancelReceipt>;operation(token:string,kind:'cancel',input:CancelInput):Promise<CancelResult>;write(token:string,kind:'cancel',input:CancelInput):Promise<CancelResult>};
export function purchaseCancelPendingKey(s:LocalSession):string;
export function readPurchaseCancelPending(storage:Storage,key:string):PendingCancel|null;
export function persistPurchaseCancelPending(storage:Storage,key:string,op:CancelOperation):void;
export function resolvePurchaseCancelPending(client:ReturnType<typeof createPurchaseCancelClient>,token:string,storage:Storage,key:string,send?:boolean,discard?:boolean):Promise<{state:string;result?:CancelResult}|null>;
