import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
export interface RecipeIngredient{product_id:string;unit:string;quantity_milli:number}
export interface RecipeVersion{operation_id:string;recipe_id:string;version_id:string;expected_revision:number;revision:number;name:string;output_product_id:string;output_unit:string;yield_milli:number;ingredients:RecipeIngredient[];created_at:string;created_by:string}
export interface CapacityMaterial{product_id:string;recipe_unit:string;stock_unit:string;required_milli:number;stock_milli:number;conversion_numerator:number;conversion_denominator:number;possible_batches:number;limiting:boolean}
export interface CapacityResult{location_id:string;measured_at:string;basis:string;alternatives_independent:true;simultaneous_total_available:false;alternatives:{recipe_id:string;version_id:string;revision:number;output_product_id:string;output_unit:string;yield_per_batch_milli:number;possible_batches:number;possible_output_milli:number;limiting_product_ids:string[];materials:CapacityMaterial[]}[]}
export interface ProductionRead{scope:string;versions(recipe?:string,offset?:number,signal?:AbortSignal):Promise<RecipeVersion[]>;version(id:string,signal?:AbortSignal):Promise<RecipeVersion>;locations(signal?:AbortSignal):Promise<{id:string;name:string}[]>;capacity(version:RecipeVersion,location:string,signal?:AbortSignal):Promise<CapacityResult>}
export function recipeVersion(value:unknown):RecipeVersion;
export function capacityResult(value:unknown,version:RecipeVersion,location:string):CapacityResult;
export function createProductionRead(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;fetcher?:typeof fetch}):ProductionRead;
