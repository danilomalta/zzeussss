import type {SaleSummary} from './localClient.mjs';
export interface OwnerSummary {total_cents:number;completed:number;cancelled:number;loaded:number;days:{date:string;cents:number}[];}
export function summarizeOwnerSales(rows:SaleSummary[]):OwnerSummary;
export function formatOwnerMoney(cents:number,locale?:string):string;
