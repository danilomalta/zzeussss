import type {LocalSession} from './localClient.mjs';
export interface ProductValues {sku:string;name:string;unit:string;barcode:string;price_cents:number;cost_cents:number}
export interface EditingDetails {product_id:string;revision:number;product:ProductValues}
export interface ProductState {product_id:string;revision:number;status:'active'|'inactive';repeated:boolean}
export interface EditInput extends ProductValues {operation_id:string;product_id:string;expected_revision:number}
export interface StateInput {operation_id:string;product_id:string;expected_revision:number;status:'active'|'inactive';reason:string}
export interface EditForm {sku:string;name:string;unit:string;barcode:string;price:string;cost:string}
export type CatalogOperation={kind:'edit';input:EditInput}|{kind:'state';input:StateInput};
export type PendingCatalog=CatalogOperation & {uncertain:boolean};
export interface CatalogResult {product_id:string;revision:number;repeated:boolean;status?:'active'|'inactive'}
export function validEditingDetails(v:unknown,id:string):boolean;
export function validProductState(v:unknown,id:string):boolean;
export function validCatalogInput(kind:string,v:unknown):boolean;
export function editMoneyValue(v:number):string;
export function prepareCatalogEdit(v:EditingDetails,values:EditForm,operationID:string):EditInput;
export function prepareProductState(v:ProductState,status:'active'|'inactive',reason:string,operationID:string):StateInput;
export function createCatalogMutations(fetcher?:typeof fetch):{details(token:string,id:string):Promise<EditingDetails>;state(token:string,id:string):Promise<ProductState>;operation(token:string,kind:CatalogOperation['kind'],input:CatalogOperation['input']):Promise<CatalogResult>;write(token:string,kind:CatalogOperation['kind'],input:CatalogOperation['input']):Promise<CatalogResult>};
export function catalogPendingKey(s:LocalSession):string;
export function readCatalogPending(storage:Storage,key:string):PendingCatalog|null;
export function persistCatalogPending(storage:Storage,key:string,operation:CatalogOperation):void;
export function resolveCatalogPending(client:ReturnType<typeof createCatalogMutations>,token:string,storage:Storage,key:string,send?:boolean,discard?:boolean):Promise<{state:string;result?:CatalogResult}|null>;
