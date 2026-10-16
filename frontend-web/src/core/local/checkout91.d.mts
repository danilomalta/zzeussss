import type {SaleInput,SaleReceipt,LocalSession} from './localClient.mjs';
export type Method91='cash'|'debit'|'credit'|'pix';
export type Fiscal91='none'|'cpf'|'cnpj';
export interface Tender91{version:1;sale_id:string;operation_id:string;cash_session_id:string;total_cents:number;received_cents:number}
export const checkoutMethods91:Method91[];
export function canSubmit91(input:{method:Method91;fiscal:Fiscal91;total:number;received:number|null}):boolean;
export function checkoutScope91(scope:LocalSession):string;
export function tenderStore91(storage:Pick<Storage,'getItem'|'setItem'>,scope:LocalSession):{get:(operation:string)=>Tender91|null;save:(input:SaleInput,received:number)=>Tender91};
export function confirmedChange91(receipt:SaleReceipt|null,tender:Tender91|null):{total:number;received:number;change:number}|null;
