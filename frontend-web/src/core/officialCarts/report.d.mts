export interface CartReportRow {cart_id:string;state:string;revision:number;updated_at:string;age_seconds:number;line_count:number;sale_id:string}
export interface CartReport {from:string;to:string;generated_at:string;source_complete:true;financial_totals_included:false;total:number;summary:Record<string,number>;items:CartReportRow[]}
export function cartPeriod(from:string,to:string):{first:number;last:number};
export function cartReport(v:unknown,from:string,to:string):CartReport;
export function cartReportCSV(data:Uint8Array,from:string,to:string):{data:Uint8Array;row_count:number};
export const cartReportColumns:string[];
