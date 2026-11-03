import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {SalesReport,SalesWindow} from './model.mjs';
export interface SalesAnalysisSource{scope:string;assertCurrent():void;report(window:SalesWindow,offset?:number,signal?:AbortSignal):Promise<SalesReport>;csv(window:SalesWindow,offset?:number,signal?:AbortSignal):Promise<string>}
export function createSalesAnalysis(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;fetcher?:typeof fetch}):SalesAnalysisSource;
