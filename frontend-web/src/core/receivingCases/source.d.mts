import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
export interface CaseEvidence {product_id:string;unit:string;planned_milli:number;accepted_milli:number;remaining_milli:number;review_flags:string[];measured_at:string}
export interface ReceivingCase {case_id:string;order_id:string;supplier_id:string;kind:string;requested_outcome:string;reference:string;status:string;assigned_to:string;due_at:string;revision:number;opening_evidence:CaseEvidence;created_by:string;created_at:string;updated_at:string;last_reason:string}
export interface CaseDetail {case:ReceivingCase;overdue:boolean;measured_at:string}
export interface CaseResult {operation_id:string;case:ReceivingCase;repeated:boolean}
export interface CaseHistory {items:{action:string;actor_id:string;device_id:string;result:CaseResult}[];total:number;limit:number;offset:number}
export interface CaseFilters {order_id?:string;assigned_to?:string;status?:string}
export interface CasePage {items:CaseDetail[];total:number;limit:number;offset:number;measured_at:string}
export interface CaseCommand {version:1;scope:string;order_id:string;case_id:string;before:ReceivingCase|null;body:Record<string,unknown>&{operation_id:string};uncertain:boolean}
export interface CaseResolution {state:'confirmed'|'missing'|'discarded';command:CaseCommand;receipt?:CaseResult}
export interface ReceivingCases {scope:string;readonly canWrite:boolean;assertCurrent():void;dispose():void;pending():CaseCommand|null;detail(id:string,signal?:AbortSignal):Promise<CaseDetail>;history(id:string,offset?:number,signal?:AbortSignal):Promise<CaseHistory>;list(filters?:CaseFilters,offset?:number,signal?:AbortSignal):Promise<CasePage>;prepare(order:string,body:CaseCommand['body'],before?:ReceivingCase|null):CaseCommand;submit(command:CaseCommand):Promise<CaseResolution|null>;recover(send?:boolean,discard?:boolean):Promise<CaseResolution|null>}
export function createReceivingCases(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;storage:Pick<Storage,'getItem'|'setItem'|'removeItem'>;locks:LockManager;fetcher?:typeof fetch}):ReceivingCases;
