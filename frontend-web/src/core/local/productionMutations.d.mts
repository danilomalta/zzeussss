import type {LocalSession} from './localClient.mjs';
import type {RecipeVersion,ProductionOrder} from './productionRead.mjs';
export interface RecipeInput {operation_id:string;recipe_id:string;version_id:string;expected_revision:number;name:string;output_product_id:string;output_unit:string;yield_milli:number;ingredients:{product_id:string;unit:string;quantity_milli:number}[]}
export interface OrderInput {operation_id:string;order_id:string;version_id:string;location_id:string;responsible_id:string;planned_batches:number}
export interface StateInput {operation_id:string;order_id:string;expected_revision:number;status:'approved'|'cancelled';reason:string}
export interface RecipeStateInput {operation_id:string;recipe_id:string;expected_revision:number;status:'active'|'inactive';reason:string}
export interface ReserveInput {operation_id:string;reservation_id:string;order_id:string;reason:string}
export interface MaterialsInput {operation_id:string;reservation_id:string;action:'release'|'consume';reason:string}
export interface ResultInput {operation_id:string;result_id:string;order_id:string;expected_revision:number;produced_milli:number;reason:string}
export interface RecipeState {recipe_id:string;status:'active'|'inactive';revision:number;repeated:boolean}
export type ProductionOperation={kind:'recipe';input:RecipeInput}|{kind:'order';input:OrderInput}|{kind:'state';input:StateInput}|{kind:'recipe_state';input:RecipeStateInput}|{kind:'reserve';input:ReserveInput}|{kind:'materials';input:MaterialsInput}|{kind:'result';input:ResultInput};
export type PendingProduction=ProductionOperation & {uncertain:boolean};
export interface MutationResult {revision?:number;reservation_id?:string;result_id?:string;unit?:string;product_id?:string;location_id?:string;planned_milli?:number;produced_milli?:number;shortfall_milli?:number;repeated:boolean;recipe_id?:string;version_id?:string;order_id?:string;status?:string}
export function validProductionInput(kind:string,v:unknown):boolean;
export function parseProductionQuantity(v:string,allowZero?:boolean):number;
export function parseProductionInteger(v:string,allowZero?:boolean):number;
export function plannedPreview(recipe:RecipeVersion,batches:number):{output_milli:number;ingredients:{product_id:string;unit:string;quantity_milli:number;planned_milli:number}[]};
export function createProductionMutations(fetcher?:typeof fetch):{recipeState(token:string,id:string):Promise<RecipeState>;version(token:string,id:string):Promise<RecipeVersion>;order(token:string,id:string):Promise<ProductionOrder>;operation(token:string,kind:ProductionOperation['kind'],input:ProductionOperation['input']):Promise<MutationResult>;write(token:string,kind:ProductionOperation['kind'],input:ProductionOperation['input']):Promise<MutationResult>};
export function productionPendingKey(s:LocalSession):string;
export function readProductionPending(storage:Storage,key:string):PendingProduction|null;
export function persistProductionPending(storage:Storage,key:string,operation:ProductionOperation):void;
export function resolveProductionPending(client:ReturnType<typeof createProductionMutations>,token:string,storage:Storage,key:string,send?:boolean,discard?:boolean):Promise<{state:string;result?:MutationResult}|null>;
