import type {LocalCapabilities} from '../local/localClient.mjs';
export interface Context83 {tenant_id:string;store_id:string;identity_id:string;device_id:string}
export type Source83<T>={state:'ready';rows:T[];asOf:string;demo:boolean}|{state:'unavailable'|'forbidden'|'error';rows:[];demo:false};
export interface Record83 {id:string;name:string;status:string;amount?:number;quantity?:number;reference?:string}
export interface Adapter83 {read(context:Context83,cap:LocalCapabilities,signal:AbortSignal,demo:boolean):Promise<Source83<Record83>>}
export interface Draft83 {version:1;scope:string;updated:string;values:Record<string,string>;events:{id:string;area:string;action:string;at:string}[]}
// Integration contract only: endpoint paths are deliberately not guessed.
export interface OperationsAPI83 {
 read(area:string,context:Context83,signal:AbortSignal):Promise<Source83<Record83>>;
 submit(area:string,context:Context83,request:{operationId:string;expectedVersion:number;payload:unknown}):Promise<{operationId:string;state:string;version:number}>;
}
