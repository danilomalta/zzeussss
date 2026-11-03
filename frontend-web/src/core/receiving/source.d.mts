import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {ReceivingAccess,ReceivingCommand,ReceivingHistory,ReceivingTrace,ReceivingReceipt} from './model.mjs';
export interface ReceivingResolution{state:'confirmed'|'missing'|'discarded';receipt?:ReceivingReceipt}
export interface ReceivingSource{scope:string;access:ReceivingAccess;canLock:boolean;pending():ReceivingCommand|null;history(order:string,offset?:number,signal?:AbortSignal):Promise<{trace:ReceivingTrace;history:ReceivingHistory}>;locations(signal?:AbortSignal):Promise<{id:string;name:string;kind:string}[]>;submit(command:ReceivingCommand):Promise<ReceivingResolution|null>;recover(send?:boolean,discard?:boolean):Promise<ReceivingResolution|null>}
export function createReceiving(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;storage:Storage;locks?:LockManager;isCurrent:()=>boolean;fetcher?:typeof fetch}):ReceivingSource;
