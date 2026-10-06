import type {LocalSession} from './localClient.mjs';
export interface ComparisonSite {id:string;name:string;origin:string;search_template:string;status:'active'|'inactive';revision:number;changed_at:string;mode:'external_link'}
export interface SiteInput {operation_id:string;id:string;expected_revision:number;name:string;origin:string;search_template:string;status:'active'|'inactive'}
export interface SiteResult {operation_id:string;site:ComparisonSite;repeated:boolean}
export interface ComparisonClient {sites(token:string,signal?:AbortSignal):Promise<{items:ComparisonSite[];limit:32;automatic_state:'not_configured'}>;save(token:string,input:SiteInput):Promise<SiteResult>;operation(token:string,id:string):Promise<SiteResult>}
export function createComparisonClient(fetcher?:typeof fetch):ComparisonClient;
export function validComparisonOrigin(origin:string):boolean;
export function validComparisonTemplate(origin:string,template:string):boolean;
export function comparisonLink(site:ComparisonSite,query:string):string;
export function validSiteInput(v:unknown):boolean;
export function siteResultMatches(v:SiteResult,input:SiteInput):boolean;
export function sitePendingKey(session:LocalSession):string;
export interface PendingSite {input:SiteInput;state:'prepared'|'uncertain'|'refused'}
export function readSitePending(storage:Storage,key:string):PendingSite|null;
export function persistSitePending(storage:Storage,key:string,input:SiteInput):void;
export function resolveSitePending(client:ComparisonClient,token:string,storage:Storage,key:string,send?:boolean):Promise<null|{state:'not_found'}|{state:'confirmed';result:SiteResult}>;
export interface ManualComparison {price_cents:number;freight_cents:number|null;total_cents:number|null;availability:'available'|'unavailable'|'unknown';identical:boolean;eligible:boolean;source:'manual'}
export function manualComparison(price:number,freight:number|null,available:string,identical:boolean):ManualComparison;

export function releaseRefusedSite(client:ComparisonClient,token:string,storage:Storage,key:string):Promise<{state:string;result?:SiteResult}>;

export function comparisonQuery(name:string,variant:string):string;
