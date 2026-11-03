import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {StockSource,StockProduct} from '../stockWorkspace/source.mjs';
import type {ReviewedDetail} from '../reviewedInventory/model.mjs';
import type {PositionReport,VarianceReport} from './model.mjs';
export interface StockReportsSource{scope:string;canValue:boolean;assertCurrent(financial?:boolean):void;positions(location:string,product?:string,offset?:number,valued?:boolean,signal?:AbortSignal):Promise<PositionReport>;csv(location:string,product?:string,offset?:number,valued?:boolean,signal?:AbortSignal):Promise<string>;variance(detail:ReviewedDetail,signal?:AbortSignal):Promise<VarianceReport>;products(query?:string,offset?:number,signal?:AbortSignal):Promise<{items:StockProduct[];total:number}>;locations(signal?:AbortSignal):Promise<{id:string;name:string}[]>}
export function createStockReports(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;fetcher?:typeof fetch;directory?:StockSource}):StockReportsSource;
