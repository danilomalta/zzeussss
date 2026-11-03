export interface ReturnItem{sale_item_id:string;quantity_milli:number;disposition:'restock'|'no_restock'}
export interface ReturnLine extends ReturnItem{product_id:string;location_id:string;original_movement_id:string;return_movement_id:string|null;refund_cents:number}
export interface ReturnQuote{sale_id:string;cash_session_id:string;refund_cents:number;items:ReturnLine[]}
export interface ReturnBody{operation_id:string;reason:string;expected_refund_cents:number;items:ReturnItem[]}
export interface CustomerReturn extends ReturnQuote{id:string;operation_id:string;actor_id:string;reason:string;cash_movement_id:string;created_at:string;repeated:boolean;fiscal_authorized:false}
export interface BalanceLine{sale_item_id:string;product_id:string;location_id:string;sold_milli:number;returned_milli:number;remaining_milli:number;original_cents:number;refunded_cents:number;remaining_cents:number;cancelled_milli:number;cancelled_cents:number}
export interface ReturnHistory{sale_id:string;sale_status:'committed'|'cancelled';cash_session_id:string;original_total_cents:number;refunded_cents:number;remaining_cents:number;cancelled_cents:number;complete:true;items:BalanceLine[];returns:CustomerReturn[]}
export interface ReturnCommand{version:1;scope:string;actor:string;sale:string;body:ReturnBody;quote:ReturnQuote;uncertain:boolean}
export function quantity(text:string):number;
export function quantityText(n:number):string;
export function validateQuote(v:unknown,sale:string,items:ReturnItem[]):ReturnQuote;
export function history(v:unknown,sale:string):ReturnHistory;
export function receipt(v:unknown,sale:string,command?:ReturnCommand):CustomerReturn;
export function command(v:unknown,scope:string,actor:string):ReturnCommand;
