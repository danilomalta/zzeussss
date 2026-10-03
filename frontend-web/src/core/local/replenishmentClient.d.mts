import type {LocalSession} from './localClient.mjs';
export type RestockKind='policy'|'suggest'|'review';
export type RestockInput={operation_id:string;product_id?:string;minimum_milli?:number;target_milli?:number;suggestion_id?:string;decision?:string;reason?:string};
export interface RestockProduct{id:string;name:string;unit:string;balance_milli:number;minimum_milli:number;target_milli:number;revision:number}
export interface RestockSuggestion{id:string;product_id:string;name:string;unit:string;observed_milli:number;recommended_milli:number;policy_revision:number;status:string;stale:boolean;reason:string;order_id:string;created_at:string}
export interface RestockPending{kind:RestockKind;input:RestockInput;uncertain:boolean}
export function validRestockInput(kind:string,v:unknown):boolean;
export function parseThreshold(value:string,unit:string):number;
export function createReplenishmentClient(fetcher?:typeof fetch):{products(token:string,offset?:number):Promise<RestockProduct[]>;suggestions(token:string,offset?:number):Promise<RestockSuggestion[]>;operation(token:string,kind:RestockKind,input:RestockInput):Promise<any>;write(token:string,kind:RestockKind,input:RestockInput):Promise<any>};
export function restockKey(session:LocalSession):string;
export function readRestockPending(storage:Storage,key:string):RestockPending|null;
export function persistRestockPending(storage:Storage,key:string,kind:RestockKind,input:RestockInput):void;
export function resolveRestockPending(client:ReturnType<typeof createReplenishmentClient>,token:string,storage:Storage,key:string,send?:boolean,discard?:boolean):Promise<{state:string;result?:any}|null>;
