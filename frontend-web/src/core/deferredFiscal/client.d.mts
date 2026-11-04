export type Result={state:string;operation_id:string;key:string;error_code:string;message:string;archive_state:string};
export type Client={dispose():void;emit(id:string,incoming:{payment_code:string;nature:string;presence:string;confirmed:boolean}):Promise<Result>;query(id:string):Promise<Result>};
export function deferredFiscalClient(options:{token:string;session:object;getCurrent:()=>unknown}):Client;
