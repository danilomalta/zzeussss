import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {JournalPage} from './contracts';
export interface CashJournal {scope:string;assertCurrent():void;cancel():void;dispose():void;read(cash:string,offset?:number,signal?:AbortSignal):Promise<JournalPage>;exportCSV(cash:string,signal?:AbortSignal):Promise<{data:Uint8Array;name:string;total:number}>}
export function createCashJournal(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;fetcher?:typeof fetch}):CashJournal;
