export type StockKind='entry'|'loss'|'transfer';
export interface StockInput{operation_id:string;kind:StockKind;product_id:string;expected_unit?:string;from_location_id?:string;to_location_id?:string;quantity_milli:number;reason:string}
export interface StockCommand{version:1;body:StockInput;uncertain:boolean;legacy:boolean}
export interface Availability{product_id:string;location_id:string;unit:string;physical_milli:number;reserved_milli:number;free_milli:number}
export interface StockReceipt{input:StockInput;unit:string;actor_id:string;device_id:string;occurred_at:string}
export interface StockHistory{product_id:string;location_id:string;items:StockReceipt[];total_count:number;offset:number;limit:50;has_more:boolean}
export {quantityInput,quantityText} from '../receiving/model.mjs';
export function stockInput(v:StockInput,legacy?:boolean):StockInput;
export function stockCommand(c:StockCommand):StockCommand;
export function availability(v:unknown,product:string,location:string):Availability;
export function stockReceipt(v:unknown,body?:StockInput,actor?:string,device?:string):StockReceipt;
export function stockHistory(v:unknown,product:string,location:string,offset?:number):StockHistory;
