import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {FreeWindows,Performance,PendingPage,UtilizationReport} from './contracts';
export type InsightKind='availability'|'performance'|'pending'|'utilization';
export interface InsightFilters {from?:string;to?:string;dock_id?:string;supplier_id?:string;duration_minutes?:number}
export interface InsightReports {availability:FreeWindows;performance:Performance;pending:PendingPage;utilization:UtilizationReport}
export interface DockInsightsSource {scope:string;assertCurrent():void;dispose():void;query<K extends InsightKind>(kind:K,input:InsightFilters,offset?:number,signal?:AbortSignal):Promise<InsightReports[K]>}
export function createDockInsights(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;fetcher?:typeof fetch}):DockInsightsSource;
