import type {StockBalance} from './stockRead.mjs';
import type {StockNeed,Coverage} from './stockPlanning.mjs';
export interface StockListRequest {product_id:string;quantity:string;unit:string}
export interface StockListCoverage {location_id:string;product_count:number;sufficient_count:number;sufficient:boolean;items:{product_id:string;need:StockNeed;balance:StockBalance;coverage:Coverage;remaining_numerator:string|null}[]}
export function stockListSelection(locationID:string,requests:StockListRequest[]):{location_id:string;items:{product_id:string;need:StockNeed}[]};
export function stockListCoverage(locationID:string,requests:StockListRequest[],balances:StockBalance[]):StockListCoverage;
export function createStockList(fetcher?:typeof fetch):{coverage(token:string,locationID:string,requests:StockListRequest[]):Promise<StockListCoverage>};
