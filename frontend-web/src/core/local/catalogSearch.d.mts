import type {LocalProduct} from './localClient.mjs';
export interface CatalogSearchProduct extends LocalProduct{minimum_configured:boolean;approximate:boolean;status?:'active'|'inactive';state_revision?:number}
export interface CatalogSearchResult{items:CatalogSearchProduct[];total:number;offset:number;limit:number;cost_visible:boolean;status?:'all'|'active'|'inactive'}
export function createCatalogSearch(fetcher?:typeof fetch):(token:string,options?:{q?:string;unit?:string;pending?:string;offset?:number;status?:'all'|'active'|'inactive'},signal?:AbortSignal)=>Promise<CatalogSearchResult>;
