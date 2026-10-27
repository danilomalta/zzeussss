import type {LocalCapabilities} from '../local/localClient.mjs';
export type ReceivingKind='authorize'|'receipt'|'reject'|'void';
export type ReceivingInput={operation_id:string;order_id:string;reason:string;reference?:string;delivery_reference?:string;location_id?:string;unit?:string;delivered_milli?:number;accepted_milli?:number;receipt_id?:string};
export interface ReceivingCommand{version:1;kind:ReceivingKind;body:ReceivingInput;uncertain:boolean}
export interface ReceivingAccess{read:boolean;history:boolean;authorize:boolean;write:boolean}
export interface ReceivingEvent extends ReceivingInput{device_id:string;actor_id:string;created_at:string;id?:string;product_id?:string;stock_operation_id?:string;movement_id?:string;quantity_milli?:number;void?:ReceivingEvent}
export interface ReceivingTrace{order_id:string;commercial_status:'local_not_sent'|'cancelled';receiving_status:'not_authorized'|'authorized'|'partially_received'|'received';authorization:null|ReceivingEvent;product_id:string;unit:string;planned_milli:number;delivered_milli:number;accepted_milli:number;rejected_milli:number;remaining_milli:number;total_count:number;offset:number;limit:50;has_more:boolean;voided_count:number;voided_accepted_milli_exact:string;items:ReceivingEvent[]}
export interface ReceivingHistory{order_id:string;commercial_status:'local_not_sent'|'cancelled';receiving_status:ReceivingTrace['receiving_status'];product_id:string;unit:string;totals:{planned_milli:number;effective_accepted_milli:number;effective_delivered_milli:number;partial_rejected_milli:number;remaining_milli:number;fully_rejected_milli_exact:string;voided_accepted_milli_exact:string;recorded_accepted_milli_exact:string};total_count:number;offset:number;limit:50;has_more:boolean;items:{kind:'received'|'rejected'|'voided';id:string;created_at:string;receipt?:ReceivingEvent;rejection?:ReceivingEvent;void?:ReceivingEvent}[]}
export interface ReceivingReceipt{kind:ReceivingKind;input:ReceivingInput;result:{repeated:boolean;order_id?:string;authorization?:ReceivingEvent;receipt?:ReceivingEvent;rejection?:ReceivingEvent;void?:ReceivingEvent}}
export function receivingID(v:unknown):boolean;
export function quantityInput(v:string):number;
export function quantityText(v:number|string,locale:string):string;
export function receivingAccess(cap:LocalCapabilities|null):ReceivingAccess;
export function receivingCommand(c:ReceivingCommand):ReceivingCommand;
export function receivingTrace(v:unknown,order:string):ReceivingTrace;
export function receivingHistory(v:unknown,order:string,offset?:number):ReceivingHistory;
export function receivingResult(v:unknown,c:ReceivingCommand,actor:string,device:string):ReceivingReceipt['result'];
export function receivingReceipt(v:unknown,c:ReceivingCommand,actor:string,device:string):ReceivingReceipt;
