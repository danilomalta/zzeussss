import type {LocalCapabilities} from '../local/localClient.mjs';
export type Area84='closing'|'journal'|'returns'|'inventory'|'quality'|'supply'|'opportunities'|'performance'|'devices';
export type Method84='cash'|'debit'|'credit'|'pix';
export interface Context84 {tenant_id:string;store_id:string;identity_id:string;device_id:string}
export interface Payment84 {id:string;method:Method84;amount:number;provider:string;terminal:string;reference:string;settled:boolean}
export interface Item84 {id:string;sku:string;label:string;qty:number;unit:string;unitPrice:number;unitCost:number;returned:number}
export interface Sale84 {id:string;at:string;operator:string;store:string;total:number;items:Item84[];payments:Payment84[]}
export interface Evidence84 {id:string;saleId:string;paymentId:string;method:Method84;amount:number;at:string;provider:string;terminal:string;reference:string;status:'observed'|'provider_confirmed';superseded?:boolean}
export interface Movement84 {id:string;at:string;operator:string;kind:'opening'|'supply'|'withdrawal'|'refund'|'correction';amount:number;method:Method84;reference:string;reason:string;reverses?:string}
export interface Custody84 {id:string;stage:string;person:string;at:string;count?:number;reason?:string}
export interface Bag84 {id:string;state:'sealed'|'in_transit'|'received'|'verified';amount:number;counts:Record<string,number>;operator:string;reason:string;shift:string;chain:Custody84[];difference?:number}
export interface Refund84 {id:string;paymentId:string;method:Method84;amount:number;state:'requested'|'confirmed';reference:string}
export interface Return84 {id:string;saleId:string;at:string;purchaseChannel:string;reason:string;state:'requested'|'approved'|'rejected'|'refunding'|'refunded';items:{itemId:string;qty:number;disposition:string}[];amount:number;costLost:number;recovered:number;refunds:Refund84[]}
export interface CountLine84 {id:string;sku:string;unit:string;location:string;expected:number;counted:number;movements:number;reason:string}
export interface Count84 {id:string;name:string;at:string;schedule:string;state:'draft'|'submitted'|'approved'|'recount';lines:CountLine84[];approver?:string}
export interface Ingredient84 {id:string;sku:string;unit:string;required:number;available:number;cost:number;lot:string}
export interface Formula84 {id:string;name:string;version:number;output:number;unit:string;lossBps:number;ingredients:Ingredient84[];approved:boolean;at:string}
export interface Test84 {id:string;name:string;value:string;min:string;max:string;unit:string;report:string;state:'pending'|'pass'|'fail'}
export interface Batch84 {id:string;formulaId:string;formulaVersion:number;state:'quarantine'|'released'|'rejected'|'recalled';at:string;expiry:string;planned:number;actual:number;actualConsumed:Record<string,number>;tests:Test84[];documents:{id:string;name:string;type:string}[];customers:{id:string;name:string;qty:number;order:string}[];reason:string}
export interface OrderLine84 {id:string;sku:string;unit:string;ordered:number;received:number;price:number;dispatchChecked:boolean;arrivalChecked:boolean}
export interface Order84 {id:string;supplier:string;state:'suggested'|'accepted'|'confirmed'|'dispatched'|'received'|'rejected';minimum:number;maximum:number;demand:string;rejections:number;method:string;dueDate:string;policy:string;lines:OrderLine84[];reason:string;driver:string}
export interface Slot84 {id:string;orderId:string;route:string;dock:string;from:string;to:string;source:'titan'|'manual';state:'proposed'|'agreed';travelMinutes:number}
export interface Collection84 {id:string;orderId:string;sku:string;qty:number;reason:string;policy:string;state:'requested'|'approved'|'rejected'|'scheduled'|'collected'|'supplier_received';date:string;relationshipEnded:boolean;driver:string;credit:number}
export interface Offer84 {id:string;product:string;supplier:string;band:string;activity:string;region:string;period:string;sample:number;consent:boolean;units:number;marginBps:number;trialSupplier:boolean;trialBuyer:boolean;buyerAccept:boolean;slotDate:string;state:'review'|'accepted'|'rejected'}
export interface Metric84 {sku:string;label:string;store:string;period:string;units:number;revenue:number;cogs:number;labor:number;laborMinutes:number;workers:number;waste:number;holding:number;averageStock:number;days:number}
export interface Device84 {id:string;name:string;kind:'pc'|'phone';state:'connected'|'isolated'|'revoked'|'restore_required';lastSeen:string;pending:number;offlineAllowed:boolean}
export interface Quota84 {id:string;deviceId:string;sku:string;unit:string;remaining:number;frozen:number;expired:boolean}
export interface Transfer84 {id:string;from:string;to:string;sku:string;qty:number;state:'awaiting_origin'|'locked'|'credited'|'reconciled';quotaId:string;at:string}
export interface Queue84 {id:string;deviceId:string;kind:string;state:'pending'|'received'|'confirmed'|'review';attempts:number}
export interface Audit84 {id:string;area:Area84;action:string;reference:string;at:string;actor:string;detail:string}
export interface Workspace84 {version:1;scope:string;mode:'draft'|'demo';revision:number;updated:string;forms:Record<string,Record<string,string>>;sales:Sale84[];evidence:Evidence84[];movements:Movement84[];bags:Bag84[];returns:Return84[];counts:Count84[];formulas:Formula84[];batches:Batch84[];orders:Order84[];slots:Slot84[];collections:Collection84[];offers:Offer84[];metrics:Metric84[];devices:Device84[];quotas:Quota84[];pool:{sku:string;unit:string;remaining:number}[];transfers:Transfer84[];queue:Queue84[];audit:Audit84[]}
export interface SourceAdapter84<T> {read(context:Context84,cap:LocalCapabilities,signal:AbortSignal):Promise<{state:'unavailable'|'forbidden'|'ready';rows:T[];asOf?:string}>}
