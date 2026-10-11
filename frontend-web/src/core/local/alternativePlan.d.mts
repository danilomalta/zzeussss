import type {Alternatives} from './catalogPlanning.mjs';
export interface AlternativePlan {version_id:string;location_id:string;measured_at:string;batches:number;possible_batches:number;output_product_id:string;output_unit:string;planned_output_milli:string;sufficient:boolean;materials:{product_id:string;recipe_unit:string;stock_unit:string;required_milli:string;stock_free_milli:number;sufficient:boolean;limiting:boolean;missing_recipe_numerator:string;missing_recipe_denominator:string;remaining_stock_numerator:string;remaining_stock_denominator:string}[]}
export function plannedBatches(value:string):number;
export function planAlternative(snapshot:Alternatives,versionID:string,value:string):AlternativePlan;
