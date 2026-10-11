import type {LocalSession} from './localClient.mjs';
export type ReceivingKind='authorize'|'receipt'|'reject'|'void';
export interface AuthorizationInput {operation_id:string;order_id:string;reference:string;reason:string}
export interface ReceiptInput {operation_id:string;order_id:string;delivery_reference:string;location_id:string;unit:string;delivered_milli:number;accepted_milli:number;reason:string}
export interface RejectionInput {operation_id:string;order_id:string;delivery_reference:string;unit:string;delivered_milli:number;reason:string}
export interface VoidInput {operation_id:string;order_id:string;receipt_id:string;reason:string}
export type ReceivingInput=AuthorizationInput|ReceiptInput|RejectionInput|VoidInput;
export interface Authorization {operation_id:string;device_id:string;actor_id:string;reference:string;reason:string;created_at:string}
export interface OriginalReceipt extends ReceiptInput {id:string;product_id:string;actor_id:string;device_id:string;stock_operation_id:string;movement_id:string;created_at:string;void?:OriginalVoid}
export interface OriginalVoid extends VoidInput {quantity_milli:number;unit:string;product_id:string;location_id:string;actor_id:string;device_id:string;stock_operation_id:string;movement_id:string;created_at:string}
export interface OriginalRejection extends RejectionInput {id:string;product_id:string;actor_id:string;device_id:string;created_at:string}
export interface ReceivingResult {repeated:boolean;order_id?:string;authorization?:Authorization;receipt?:OriginalReceipt;rejection?:OriginalRejection;void?:OriginalVoid}
export interface ReceivingOperation {kind:ReceivingKind;input:ReceivingInput}
export type PendingReceiving=ReceivingOperation&{uncertain:boolean};
export interface ReceivingEnvelope extends ReceivingOperation {result:ReceivingResult}
export interface ReceivingOrder {id:string;supplier_id:string;status:string;receiving_status:string;items:{product_id:string;unit:string;quantity_milli:number}[]}
export interface ReceivingSnapshot {order_id:string;product_id:string;unit:string;commercial_status:string;receiving_status:string;authorization:Authorization|null;planned_milli:number;delivered_milli:number;accepted_milli:number;rejected_milli:number;remaining_milli:number;total_count:number;offset:number;limit:number;has_more:boolean;voided_count:number;voided_accepted_milli_exact:string;items:OriginalReceipt[]}
export interface DeliveryValues {reference:string;location:string;delivered:string;accepted:string;reason:string}
export function validReceivingInput(kind:string,v:unknown):boolean;
export function validReceivingOrder(v:unknown,id:string):boolean;
export function validReceivingSnapshot(v:unknown,id:string,offset?:number):boolean;
export function prepareReceivingAuthorization(order:ReceivingOrder,reference:string,reason:string,op:string):AuthorizationInput;
export function prepareReceivingDelivery(snapshot:ReceivingSnapshot,kind:'receipt'|'reject',values:DeliveryValues,op:string):ReceiptInput|RejectionInput;
export function prepareReceivingVoid(snapshot:ReceivingSnapshot,receiptID:string,reason:string,op:string):VoidInput;
export function createReceivingMutations(fetcher?:typeof fetch):{order(token:string,id:string):Promise<ReceivingOrder>;snapshot(token:string,id:string,offset?:number):Promise<ReceivingSnapshot>;receipt(token:string,kind:ReceivingKind,id:string):Promise<ReceivingEnvelope>;operation(token:string,kind:ReceivingKind,input:ReceivingInput):Promise<ReceivingResult>;write(token:string,kind:ReceivingKind,input:ReceivingInput):Promise<ReceivingResult>};
export function receivingPendingKey(s:LocalSession):string;
export function readReceivingPending(storage:Storage,key:string):PendingReceiving|null;
export function persistReceivingPending(storage:Storage,key:string,operation:ReceivingOperation):void;
export function resolveReceivingPending(client:ReturnType<typeof createReceivingMutations>,token:string,storage:Storage,key:string,send?:boolean,discard?:boolean):Promise<{state:string;result?:ReceivingResult}|null>;
