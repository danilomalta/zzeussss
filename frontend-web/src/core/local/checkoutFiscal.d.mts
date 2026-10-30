import type {LocalSession} from './localClient.mjs';
export interface CheckoutFiscalRequest{operation_id:string;received_cents:number;fiscal_invoice:Record<string,unknown>&{Environment:2}}
export interface CheckoutFiscalView{state:'authorized'|'failed'|'pending'|'none';canRetry?:boolean;plan:unknown}
export function checkoutFiscalScope(session:LocalSession):string;
export function checkoutFiscalView(plan:unknown,checkout:string):CheckoutFiscalView;
export function createCheckoutFiscalClient(options:{fetcher?:typeof fetch;cache:{read:(key:string)=>Promise<CheckoutFiscalRequest|null>;write:(key:string,body:CheckoutFiscalRequest)=>Promise<void>};getCurrent:()=>{token:string|null;session:LocalSession|null}}):{bind:(context:{token:string;session:LocalSession})=>{read:(checkout:string,signal?:AbortSignal)=>Promise<CheckoutFiscalView>;finalize:(checkout:string,input:CheckoutFiscalRequest,signal?:AbortSignal)=>Promise<CheckoutFiscalView>;retry:(checkout:string,signal?:AbortSignal)=>Promise<CheckoutFiscalView>}};
