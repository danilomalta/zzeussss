export interface RestockHypothesis{product_id:string;from:string;to:string;lead_days:number;coverage_days:number;safety_milli:number}
export interface PreviewProduct{id:string;unit:string}
export interface RestockPreview{product_id:string;unit:string;from:string;to:string;measured_at:string;basis:string;demand_basis:string;sold_milli:number;daily_demand_milli_numerator:string;daily_demand_milli_denominator:string;physical_milli:number;reserved_milli:number;free_milli:number;policy_revision:number;policy_minimum_milli:number;policy_target_milli:number;suggested_minimum_milli:number;suggested_target_milli:number;suggested_order_milli:number;review_required:true;order_created:false}
export function restockPreviewInput(value:unknown,product?:PreviewProduct):RestockHypothesis;
export function restockPreviewReport(value:unknown,input:RestockHypothesis,product:PreviewProduct):RestockPreview;
export function restockDays(value:string,min:number,max:number):number;
