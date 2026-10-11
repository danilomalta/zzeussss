import type {LocalSession} from './localClient.mjs';
export interface SupplierState {id:string;name:string;status:'active'|'inactive';revision:number;repeated:boolean}
export interface SupplierInput {operation_id:string;supplier_id:string;expected_revision:number;name:string;status:'active'|'inactive';reason:string}
export type SupplierOperation={kind:'edit';input:SupplierInput};
export type PendingSupplier=SupplierOperation & {uncertain:boolean};
export interface SupplierChange {revision:number;operation_id:string;device_id:string;actor_id:string;before_name:string;before_status:string;name:string;status:string;reason:string;created_at:string}
export interface SupplierHistory {current:SupplierState;origin:{id:string;name:string;status:string;creation:{operation_id:string;actor_id:string;device_id:string;created_at:string}};offset:number;limit:number;total_count:number;has_more:boolean;items:SupplierChange[]}
export function validSupplierState(v:unknown,id:string):boolean;
export function validSupplierInput(kind:string,v:unknown):boolean;
export function validSupplierHistory(v:unknown,id:string,offset:number):boolean;
export function prepareSupplierEdit(v:SupplierState,name:string,reason:string,operationID:string):SupplierInput;
export function prepareSupplierStatus(v:SupplierState,status:'active'|'inactive',reason:string,operationID:string):SupplierInput;
export function createSupplierMutations(fetcher?:typeof fetch):{state(token:string,id:string):Promise<SupplierState>;history(token:string,id:string,offset?:number):Promise<SupplierHistory>;operation(token:string,kind:'edit',input:SupplierInput):Promise<SupplierState>;write(token:string,kind:'edit',input:SupplierInput):Promise<SupplierState>};
export function supplierPendingKey(s:LocalSession):string;
export function readSupplierPending(storage:Storage,key:string):PendingSupplier|null;
export function persistSupplierPending(storage:Storage,key:string,operation:SupplierOperation):void;
export function resolveSupplierPending(client:ReturnType<typeof createSupplierMutations>,token:string,storage:Storage,key:string,send?:boolean,discard?:boolean):Promise<{state:string;result?:SupplierState}|null>;
