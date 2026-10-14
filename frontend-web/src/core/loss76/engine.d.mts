import type {LocalCapabilities} from '../local/localClient.mjs';
export interface Lot {id:string;name:string;unit:'unit'|'kg'|'litre';stock_milli:number;cost_cents:number;price_cents:number;sold_milli:number;window_days:number;minimum_milli:number;lead_days:number;loss_milli:number;expiry:string|null;}
export interface Policy {expiry_days:number;slow_days:number;margin_bps:number;discount_bps:number;}
export interface Analysis {stock:number;days:number|null;coverage:number|null;expired:boolean;near:boolean;slow:boolean;floor:number|null;suggested:number|null;reorder:number;warning:boolean;loss_cents:number;}
export interface Recipe {output_unit?:'unit'|'kg'|'litre';yield_milli:number;waste_bps:number;ingredients:{id:string;required_milli:number}[];}
export interface Decision {product_id:string;original_cents:number;proposed_cents:number;state:'planned';}
export interface Journal {id:string;product_id:string;quantity_milli:number;reason:string;date:string;}
export interface Scenario {schema:string;context:string;rows:Lot[];decisions:Decision[];recipe:Recipe;policy:Policy;journal:Journal[];comparisons:{name:string;recipe:Recipe}[];}
export function permitted(c:LocalCapabilities|null):boolean;
export function validate(rows:unknown):Lot[];
export function validDate(s:unknown):boolean;
export function analyze(row:Lot,policy:Policy,today:string):Analysis;
export function production(rows:Lot[],recipe:Recipe):{batches:number;output_milli:number;yield_milli:number;batch_cost_cents:number;unit_cost_cents:number|null;limiting:string[]};
export function recordLoss(rows:Lot[],input:{id:string;quantity_milli:number;reason:string}):Lot[];
export function approve(row:Lot,result:Analysis,price:number):Decision;
export function scenarioFile(context:string,rows:Lot[],decisions:Decision[],recipe:Recipe,policy:Policy,journal:Journal[],comparisons?:{name:string;recipe:Recipe}[]):Scenario;
export function readScenario(value:unknown,context:string):Scenario;
export const defaultPolicy:Policy;
export function examples(today:string):Lot[];
