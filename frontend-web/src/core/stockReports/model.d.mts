import type {Availability} from '../stockWorkspace/model.mjs';
import type {ReviewedDetail,ReviewedLine,ReviewedSession} from '../reviewedInventory/model.mjs';
export interface StockPosition extends Availability{sku:string;name:string;catalog_cost_cents?:number;value_cents_numerator?:string;value_cents_denominator?:string;cost_requires_review?:boolean}
export interface PositionReport{measured_at:string;basis:string;items:StockPosition[];total:number;limit:50;offset:number;historical_cost:false;page_value_cents_numerator?:string;page_value_cents_denominator?:string}
export interface VarianceLine extends ReviewedLine{current_catalog_cost_cents:number;cost_requires_review:boolean;difference_value_cents_numerator:string|null;difference_value_cents_denominator:string|null}
export interface VarianceReport{inventory:ReviewedSession;basis:string;lines:VarianceLine[];counted_lines:number;uncounted_lines:number;shortage_lines:number;surplus_lines:number;equal_lines:number;shortage_value_cents_numerator:string;shortage_value_cents_denominator:string;surplus_value_cents_numerator:string;surplus_value_cents_denominator:string;net_value_cents_numerator:string;net_value_cents_denominator:string;posted_to_finance:false}
export function reportID(value:unknown):boolean;
export function stockValueText(numerator:string,denominator:string,locale:string):string;
export function stockPositions(value:unknown,location:string,product?:string,offset?:number,valued?:boolean):PositionReport;
export function inventoryVariance(value:unknown,expected:ReviewedDetail):VarianceReport;
