import type {LocalProduct} from './localClient.mjs';
export interface CatalogSearchProduct extends LocalProduct{minimum_configured:boolean;approximate:boolean}
export interface CatalogSearchResult{items:CatalogSearchProduct[];total:number;offset:number;limit:number;cost_visible:boolean}
export function createCatalogSearch(fetcher?:typeof fetch):(token:string,options?:{q?:string;unit?:string;pending?:string;offset?:number},signal?:AbortSignal)=>Promise<CatalogSearchResult>;
