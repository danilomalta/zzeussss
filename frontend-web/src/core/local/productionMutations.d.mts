import type {LocalSession} from './localClient.mjs';
import type {RecipeVersion,ProductionOrder} from './productionRead.mjs';
export interface RecipeInput {operation_id:string;recipe_id:string;version_id:string;expected_revision:number;name:string;output_product_id:string;output_unit:string;yield_milli:number;ingredients:{product_id:string;unit:string;quantity_milli:number}[]}
export interface OrderInput {operation_id:string;order_id:string;version_id:string;location_id:string;responsible_id:string;planned_batches:number}
export interface StateInput {operation_id:string;order_id:string;expected_revision:number;status:'approved'|'cancelled';reason:string}
export type ProductionOperation={kind:'recipe';input:RecipeInput}|{kind:'order';input:OrderInput}|{kind:'state';input:StateInput};
export type PendingProduction=ProductionOperation & {uncertain:boolean};
export interface MutationResult {revision:number;repeated:boolean;recipe_id?:string;version_id?:string;order_id?:string;status?:string}
export function validProductionInput(kind:string,v:unknown):boolean;
export function parseProductionQuantity(v:string):number;
export function parseProductionInteger(v:string,allowZero?:boolean):number;
export function plannedPreview(recipe:RecipeVersion,batches:number):{output_milli:number;ingredients:{product_id:string;unit:string;quantity_milli:number;planned_milli:number}[]};
export function createProductionMutations(fetcher?:typeof fetch):{version(token:string,id:string):Promise<RecipeVersion>;order(token:string,id:string):Promise<ProductionOrder>;operation(token:string,kind:ProductionOperation['kind'],input:ProductionOperation['input']):Promise<MutationResult>;write(token:string,kind:ProductionOperation['kind'],input:ProductionOperation['input']):Promise<MutationResult>};
export function productionPendingKey(s:LocalSession):string;
export function readProductionPending(storage:Storage,key:string):PendingProduction|null;
export function persistProductionPending(storage:Storage,key:string,operation:ProductionOperation):void;
export function resolveProductionPending(client:ReturnType<typeof createProductionMutations>,token:string,storage:Storage,key:string,send?:boolean,discard?:boolean):Promise<{state:string;result?:MutationResult}|null>;
