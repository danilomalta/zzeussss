import type {ProductionLot} from './productionLots.mjs';
import type {LotQuality} from './productionQuality.mjs';
export interface LotSearchFilters {status?:string;quality?:string;product_id?:string;location_id?:string;expiry?:string;expires_from?:string;expires_to?:string}
export interface LotSearchPage {filters:LotSearchFilters&{offset:number};total_count:number;limit:number;has_more:boolean;items:{lot:ProductionLot;order_id:string;location_id:string;quality:LotQuality}[]}
export function lotSearchFilters(value?:LotSearchFilters):LotSearchFilters;
export function validLotSearchPage(value:unknown,filters:LotSearchFilters,offset:number):boolean;
export function createProductionLotSearch(fetcher?:typeof fetch):{search(token:string,filters?:LotSearchFilters,offset?:number):Promise<LotSearchPage>};
