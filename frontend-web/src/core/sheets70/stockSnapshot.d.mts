import type {Context,Filters,DataSet} from './contracts';
export function sheetStockLocations(context:Context,fetcher?:typeof fetch):Promise<{id:string;name:string}[]>;
export function sheetStockSnapshot(context:Context,filters:Filters,fetcher?:typeof fetch):Promise<DataSet>;
