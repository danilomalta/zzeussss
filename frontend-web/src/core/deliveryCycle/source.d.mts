import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {PurchaseOrder} from '../local/purchaseClient.mjs';
import type {OrderContext} from '../supplyDecisions/source.mjs';
import type {Cycle,Delivery,History,Origin,Result,PlanInput,ActionInput,ReceiveInput,FullRejectionInput,FinanceInput} from './contracts';
export type DeliveryKind='plan'|'action'|'receive'|'reject'|'finance';
export type DeliveryBody=PlanInput|ActionInput|ReceiveInput|FullRejectionInput|FinanceInput;
export interface DeliveryCommand {version:1;scope:string;kind:DeliveryKind;order:OrderContext;cycle:Omit<Cycle,'deliveries'|'limit'|'offset'|'has_more'>|null;before:Delivery|null;body:DeliveryBody;uncertain:boolean}
export interface DeliveryResolution {state:'confirmed'|'missing'|'discarded';command:DeliveryCommand;receipt?:Result}
export interface DeliveryCycle {scope:string;assertCurrent():void;canWrite(kind:DeliveryKind):boolean;dispose():void;pending():DeliveryCommand|null;cycle(order:PurchaseOrder,offset?:number,signal?:AbortSignal):Promise<Cycle>;detail(id:string,order?:PurchaseOrder,signal?:AbortSignal):Promise<Delivery>;history(id:string,offset?:number,signal?:AbortSignal):Promise<History>;origin(id:string,signal?:AbortSignal):Promise<Origin>;locations(signal?:AbortSignal):Promise<{id:string;name:string;kind:string}[]>;prepare(kind:DeliveryKind,order:PurchaseOrder,page:Cycle|null,before:Delivery|null,body:DeliveryBody):DeliveryCommand;submit(c:DeliveryCommand):Promise<DeliveryResolution|null>;recover(send?:boolean,discard?:boolean):Promise<DeliveryResolution|null>}
export function createDeliveryCycle(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;storage:Pick<Storage,'getItem'|'setItem'|'removeItem'>;locks:LockManager;fetcher?:typeof fetch}):DeliveryCycle;
