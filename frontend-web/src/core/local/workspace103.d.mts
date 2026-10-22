import type {Summary95} from './report95.mjs';import type {ClosingSale92} from './closing92.mjs';import type {Request94} from './till94.mjs';
export interface Filter103{query?:string;from?:string;to?:string;state?:string;method?:string}
export interface Receipt103{sale_id:string;operation_id:string;cash_session_id:string;status:'committed'|'cancelled';fiscal_authorized:boolean;total_cents:number;items:{item_id:string;product_id:string;location_id:string;quantity_milli:number;unit_price_cents:number;total_cents:number}[];payments:{payment_id:string;method:string;status:string;amount_cents:number}[]}
export const methods103:string[];
export function range103(from?:string,to?:string):number[];
export function archives103(rows:Summary95[],scope:string,mode:'live'|'demo',filter?:Filter103):Summary95[];
export function compare103(a:Summary95,b:Summary95):{expected:number;counted:number;difference:number};
export function sales103(rows:ClosingSale92[],filter?:Filter103):(ClosingSale92&{total:number})[];
export function receipt103(v:unknown,id:string,cash:string):Receipt103;
export function queue103(rows:Request94[],filter?:Filter103):Request94[];
