import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {Availability,StockCommand,StockReceipt,StockHistory} from './model.mjs';
export interface StockProduct{id:string;name:string;sku:string;unit:string}
export interface StockResolution{state:'confirmed'|'missing'|'discarded';receipt?:StockReceipt}
export interface StockSource{scope:string;canLock:boolean;canWrite:boolean;pending():StockCommand|null;products(q?:string,offset?:number,signal?:AbortSignal):Promise<{items:StockProduct[];total:number}>;locations(signal?:AbortSignal):Promise<{id:string;name:string}[]>;availability(product:string,location:string,signal?:AbortSignal):Promise<Availability>;history(product:string,location:string,offset?:number,signal?:AbortSignal):Promise<StockHistory>;submit(command:StockCommand):Promise<StockResolution|null>;recover(send?:boolean,discard?:boolean):Promise<StockResolution|null>}
export function createStockWorkspace(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;storage:Storage;locks?:LockManager;isCurrent:()=>boolean;fetcher?:typeof fetch}):StockSource;
