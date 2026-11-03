import type {RecipeVersion} from './source.mjs';
export interface Rational{numerator:string;denominator:string}
export interface MaterialCost{source:'current_catalog_material_estimate';valued_at:string;version_id:string;order_id?:string;batches:number;output_unit:string;output_quantity_milli:number;lines:{product_id:string;unit:string;quantity_milli:number;catalog_cost_cents:number;cost_cents_exact:Rational;requires_cost_review:boolean}[];material_cost_cents_exact:Rational;material_cost_cents:number;material_unit_cost_cents_exact:Rational;rounding:'half_up_on_aggregate';labor_cost_cents:null;overhead_cost_cents:null;is_complete:false}
export interface ScenarioInput{batches:number;labor?:{minutes:number;workers:number;hourly_cost_cents:number};overhead_cents?:number;sale_price_cents?:number}
export interface CostScenario{materials:MaterialCost;basis:string;labor_cents_exact:Rational|null;overhead_cents:number|null;known_cost_cents_exact:Rational;known_cost_cents:number;known_unit_cost_cents_exact:Rational;scenario_revenue_cents_exact:Rational|null;scenario_contribution_cents_exact:Rational|null;scenario_margin_basis_points:number|null;all_requested_components_provided:boolean;historical_cost:false;profit:false}
export interface PlannedRecipe{version:RecipeVersion;batches:number}
export interface SimultaneousPlan{location_id:string;measured_at:string;basis:string;feasible:boolean;reserved:false;outputs:{version_id:string;product_id:string;unit:string;batches:number;output_milli:number}[];materials:{product_id:string;unit:string;required_milli:number;physical_milli:number;reserved_milli:number;free_milli:number;shortfall_milli:number;remaining_milli:number}[]}
export function exactRational(value:unknown,signed?:boolean):[bigint,bigint];
export function formatProductionRational(value:Rational,locale:string):string;
export function productionInteger(value:string,min?:number,max?:number):number;
export function productionCents(value:string):number;
export function costScenarioInput(value:unknown):ScenarioInput;
export function materialCostEstimate(value:unknown,version:RecipeVersion,batches:number,orderID?:string):MaterialCost;
export function productionCostScenario(value:unknown,version:RecipeVersion,input:ScenarioInput):CostScenario;
export function simultaneousInput(locationID:string,items:PlannedRecipe[]):{location_id:string;recipes:{version_id:string;batches:number}[]};
export function productionSimultaneous(value:unknown,locationID:string,items:PlannedRecipe[]):SimultaneousPlan;
