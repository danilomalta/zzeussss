import type {ProductState} from './catalogMutations.mjs';
export interface CatalogStateEvent {revision:number;store_id:string;operation_id:string;device_id:string;actor_id:string;before_status:'active'|'inactive';after_status:'active'|'inactive';reason:string;created_at:string}
export interface CatalogStateHistory {current:ProductState;offset:number;limit:number;total_count:number;has_more:boolean;items:CatalogStateEvent[]}
export function validCatalogStateHistory(v:unknown,id:string,offset:number):boolean;
export function createCatalogStateHistory(fetcher?:typeof fetch):(token:string,id:string,offset?:number)=>Promise<CatalogStateHistory>;
