import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {DocumentCommand,DocumentPage,DocumentReceipt,PayableDocument} from './model.mjs';
export interface DocumentResolution{state:'confirmed'|'missing'|'discarded';receipt?:DocumentReceipt}
export interface DocumentSource{scope:string;actor:string;canLock:boolean;pending():DocumentCommand|null;read(document:string,signal?:AbortSignal):Promise<PayableDocument>;page(state?:string,offset?:number,signal?:AbortSignal):Promise<DocumentPage>;suppliers(offset?:number,signal?:AbortSignal):Promise<{id:string;name:string;status:string}[]>;submit(command:DocumentCommand):Promise<DocumentResolution|null>;recover(send?:boolean,discard?:boolean):Promise<DocumentResolution|null>}
export function createPayableDocuments(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;storage:Storage;locks?:LockManager;fetcher?:typeof fetch;isCurrent:()=>boolean}):DocumentSource;
