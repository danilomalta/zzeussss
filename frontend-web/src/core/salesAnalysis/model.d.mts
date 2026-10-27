export interface SalesWindow{from:string;to:string;compare_from?:string;compare_to?:string}
export interface ProductSales{product_id:string;sku:string;name:string;unit:string;quantity_milli:number;revenue_cents:number;previous_quantity_milli:number;previous_revenue_cents:number;revenue_delta_cents:number;revenue_change_basis_points:number|null;change_basis:'different_duration_totals_only'|'no_baseline'|'new_activity'|'change_out_of_range'|'comparable_totals';abc_category:'A'|'B'|'C'|'no_current_revenue'}
export interface SalesReport{from:string;to:string;compare_from:string;compare_to:string;current_seconds:number;previous_seconds:number;equal_duration:boolean;basis:string;abc_basis:string;payment_confirmed:false;profit:false;source_rows:number;items:ProductSales[];total_products:number;limit:50;offset:number;revenue_cents:number;previous_revenue_cents:number}
export function salesInstant(value:string):bigint;
export function salesLocalTime(value:string):string;
export function salesWindow(value:SalesWindow):Required<SalesWindow>;
export function salesReport(value:unknown,window:SalesWindow,offset?:number):SalesReport;
export function salesBasisPoints(value:number,locale:string):string;
