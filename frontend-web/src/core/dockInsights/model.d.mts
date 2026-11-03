import type {InsightKind,InsightFilters,InsightReports} from './source.mjs';
import type {ExactSeconds} from './contracts';
export function insightFail():never;
export function exactRatio(v:unknown):{n:bigint;d:bigint};
export function ratioText(v:ExactSeconds,locale?:string,percent?:boolean):string;
export function insightFilters(kind:InsightKind,input:InsightFilters):InsightFilters;
export function insightReport<K extends InsightKind>(kind:K,v:unknown,input:InsightFilters,offset?:number):InsightReports[K];
export function insightCSV(rows:unknown[][]):string;
