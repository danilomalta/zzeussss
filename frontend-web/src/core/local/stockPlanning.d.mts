import type {StockBalance,Reservations} from './stockRead.mjs';
export interface StockNeed {quantity_milli:number;unit:string}
export interface Coverage {sufficient:boolean;required_numerator:string;denominator:string;missing_numerator:string;stock_unit:string}
export interface StockComparison {location_id:string;balance:StockBalance|null;error:string}
export function stockLocations(values:string[]):string[];
export function stockNeed(value:string,u:string):StockNeed;
export function stockCoverage(balance:StockBalance,need:StockNeed):Coverage;
export function fractionQuantity(n:string,d:string):string;
export function reservationGroups(page:Reservations,kind:'order'|'responsible'):{id:string;count:number;quantity_milli:string;unit:string;version_ids:string[];responsible_ids:string[]}[];
export function createStockPlanning(fetcher?:typeof fetch):{compare(token:string,product:string,locations:string[]):Promise<StockComparison[]>};
