import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {PurchaseOrder} from '../local/purchaseClient.mjs';
import type {Cycle,DecisionInput,Result} from './contracts';
export interface OrderContext {id:string;supplier_id:string;product_id:string;unit:string;planned_milli:number}
export interface DecisionCommand {version:1;scope:string;order:OrderContext;cycle:Omit<Cycle,'deliveries'|'limit'|'offset'|'has_more'>;body:DecisionInput;uncertain:boolean}
export interface DecisionResolution {state:'confirmed'|'missing'|'discarded';command:DecisionCommand;receipt?:Result}
export interface SupplyDecisions {scope:string;readonly canWrite:boolean;assertCurrent():void;dispose():void;pending():DecisionCommand|null;cycle(order:PurchaseOrder,offset?:number,signal?:AbortSignal):Promise<Cycle>;prepare(order:PurchaseOrder,page:Cycle,body:DecisionInput):DecisionCommand;submit(command:DecisionCommand):Promise<DecisionResolution|null>;recover(send?:boolean,discard?:boolean):Promise<DecisionResolution|null>}
export function createSupplyDecisions(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;storage:Pick<Storage,'getItem'|'setItem'|'removeItem'>;locks:LockManager;fetcher?:typeof fetch}):SupplyDecisions;
