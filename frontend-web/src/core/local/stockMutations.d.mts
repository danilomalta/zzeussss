import type {LocalSession} from './localClient.mjs';
export type StockKind='entry'|'transfer'|'loss';
export interface StockInput {operation_id:string;kind:StockKind;expected_unit:string;product_id:string;from_location_id:string;to_location_id:string;quantity_milli:number;reason:string}
export interface StockReceipt {input:StockInput;actor_id:string;device_id:string;occurred_at:string;unit:string}
export interface StockHistory {product_id:string;location_id:string;items:StockReceipt[];offset:number;limit:number;total_count:number;has_more:boolean}
export interface StockOperation {kind:StockKind;input:StockInput}
export type PendingStock=StockOperation&{uncertain:boolean};
export function validStockInput(kind:string,v:unknown):boolean;
export function prepareStockOperation(product:{id:string;unit:string},values:{kind:StockKind;from:string;to:string;quantity:string;reason:string},operationID:string):StockInput;
export function createStockMutations(fetcher?:typeof fetch):{receipt(token:string,id:string):Promise<StockReceipt>;operation(token:string,kind:StockKind,input:StockInput):Promise<StockReceipt>;history(token:string,product:string,location:string,offset?:number):Promise<StockHistory>;write(token:string,kind:StockKind,input:StockInput):Promise<{operation_id:string;repeated:boolean}>};
export function stockPendingKey(s:LocalSession):string;
export function readStockPending(storage:Storage,key:string):PendingStock|null;
export function persistStockPending(storage:Storage,key:string,operation:StockOperation):void;
export function resolveStockPending(client:ReturnType<typeof createStockMutations>,token:string,storage:Storage,key:string,send?:boolean,discard?:boolean):Promise<{state:string;result?:StockReceipt}|null>;
