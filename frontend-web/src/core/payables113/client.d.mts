import type {Document113,CashPage113,Input113,Result113} from './model.mjs';
export interface PayablesSource113{documents(signal?:AbortSignal):Promise<Document113[]>;payments(document:string,offset?:number,signal?:AbortSignal):Promise<CashPage113>;submit(document:string,kind:string,body:Input113,paymentID?:string,signal?:AbortSignal):Promise<Result113>;receipt(document:string,kind:string,body:Input113,paymentID?:string,signal?:AbortSignal):Promise<Result113>}
export function createPayables113(token:string,fetcher?:typeof fetch):PayablesSource113;
