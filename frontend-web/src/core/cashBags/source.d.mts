export type BagState='prepared'|'in_transit'|'received'|'rejected'|'cancelled';
export interface BagEvent {operation_id:string;actor_id:string;action:string;from_state:string;to_state:string;reason:string;created_at:string}
export interface Bag {id:string;session_id:string;withdrawal_operation_id:string;movement_id:string;sender_id:string;operator_id:string;seal_id:string;state:BagState;declared_cents:number|null;counted_cents:number|null;difference_cents:number|null;receiver_id:string|null;received_at:string|null;created_at:string;updated_at:string;history:BagEvent[]}
export type BagAction='create'|'dispatch'|'cancel'|'receive'|'reject';
export interface BagCommand {version:1;scope:string;action:BagAction;id:string;body:Record<string,string|number>;before:Bag|null;uncertain:boolean}
export interface BagPage {items:Bag[];limit:number;offset:number;total_count:number;has_more:boolean}
export interface Coverage {session_id:string;scope:string;complete:boolean;[key:string]:string|number|boolean}
export interface BagResolution {state:string;result?:{operation_id:string;repeated:boolean;bag:Bag}}
export interface CashBags {scope:string;key:string;actor:string;readonly canWrite:boolean;assertCurrent():void;subscribe(fn:()=>void):()=>void;dispose():void;pending():BagCommand|null;prepare(action:BagAction,id:string,body:Record<string,string|number>,before?:Bag|null):BagCommand;submit(c:BagCommand):Promise<BagResolution|null>;recover(send?:boolean,discard?:boolean):Promise<BagResolution|null>;read(id:string):Promise<Bag>;list(filters:{state:string;session_id:string;offset:number}):Promise<BagPage>;coverage(id:string):Promise<Coverage>}
export function createCashBags(options:{token:string;session:unknown;capabilities:unknown;isCurrent:()=>boolean;storage:Storage;locks:LockManager;fetcher?:typeof fetch}):CashBags;
