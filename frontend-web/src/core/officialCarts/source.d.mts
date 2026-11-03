import type {SaleReceipt,createLocalClient} from '../local/localClient.mjs';
export interface InputItem {product_id:string;location_id:string;quantity_milli:number}
export interface QuoteItem extends InputItem {unit:string;unit_price_cents:number;total_cents:number;available_milli:number}
export interface Cart {id:string;revision:number;state:'suspended'|'claimed'|'completed'|'cancelled';label:string;sale_id:string;payment_id:string;cash_session_id:string;items:QuoteItem[];total_cents:number;created_at:string;updated_at:string;sale_operation_id:string;received_cents:number;change_cents:number}
export type CartAction='save'|'claim'|'release'|'cancel'|'finalize_cash';
export interface CartResult {operation_id:string;action:CartAction;reason:string;repeated:boolean;cart:Cart;sale?:SaleReceipt}
export interface CartCommand {version:1;scope:string;kind:'save'|'action';id:string;body:Record<string,unknown>;before:Cart|null;origin:string;uncertain:boolean}
export interface CartResolution {state:string;result?:CartResult;command?:CartCommand}
export interface Page<T> {items:T[];total:number;limit:number;offset:number;has_more:boolean}
export interface Reconciliation {cart_id:string;revision:number;state:string;status:string;blockers:string[];checked_at:string;saved_total_cents:number;sale_id:string;reserved:false;current_quote?:{cash_session_id:string;items:QuoteItem[];total_cents:number;payment_methods:string[];discounts_supported:false;loyalty_supported:false;reserved:false}}
export interface OfficialCarts {scope:string;key:string;legacyKey:string;readonly canWrite:boolean;assertCurrent():void;pending():CartCommand|null;subscribe(fn:()=>void):()=>void;dispose():void;prepare(kind:'save'|'action',id:string,body:Record<string,unknown>,before?:Cart|null,origin?:string):CartCommand;submit(c:CartCommand):Promise<CartResolution|null>;recover(send?:boolean,discard?:boolean):Promise<CartResolution|null>;list(state?:string,offset?:number):Promise<Page<Cart>>;read(id:string):Promise<Cart>;reconcile(c:Cart,cash:string):Promise<Reconciliation>;history(id:string,offset?:number):Promise<Page<CartResult>>}
export function createOfficialCarts(options:{token:string;session:unknown;capabilities:unknown;isCurrent:()=>boolean;storage:Storage;locks:LockManager;saleReader:Pick<ReturnType<typeof createLocalClient>,'receipt'>;fetcher?:typeof fetch}):OfficialCarts;
