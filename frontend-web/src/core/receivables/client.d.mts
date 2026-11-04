import type {LocalSession} from '../local/localClient.mjs';
export interface Installment{id:string;due_on:string;amount_cents:number;received_cents:number}
export interface CustomerBillingOrder{customer_tenant_id:string;customer_store_id:string;message_id:string;decision_id:string;order_id:string;customer_reference:string;product_id:string;unit:string;accepted_milli:number;offer_sha256:string}
export interface CustomerOrderInput{message_id:string;terms_reference:string;confirmed:boolean}
export interface Document{credit_sale_id?:string;reversed_cents?:number;reversals?:{operation_id:string;original_operation_id:string;original_device_id:string;amount_cents:number}[];customer_order?:{source:CustomerBillingOrder;terms_reference:string};id:string;customer_reference:string;document_reference:string;description:string;issued_on:string;total_cents:number;received_cents:number;outstanding_cents:number;state:'open'|'settled'|'cancelled';created_by:string;created_at:string;installments:Installment[];receipts:{device_id?:string;operation_id:string;amount_cents:number;actor_id:string;created_at:string;movement_id:string}[];history:{operation_id:string;action:string;actor_id:string;device_id:string;reason:string;created_at:string}[]}
export interface Page{items:Document[];total:number;limit:number;offset:number}
export interface Result{operation_id:string;action:string;repeated:boolean;document:Document}
export interface Input{credit_sale?:{sale_id:string;session_id:string;items:{product_id:string;location_id:string;quantity_milli:number}[]};customer_order?:CustomerOrderInput;operation_id:string;id:string;customer_reference:string;document_reference:string;description:string;issued_on:string;total_cents:number;installments:{id:string;due_on:string;amount_cents:number}[]}
export interface Client{ready():Promise<ReturnType<Client["pending"]>>;pending():{operation_id:string;document_id:string;action:string}|null;dispose():void;list(offset?:number):Promise<Page>;customerOrder(id:string):Promise<CustomerBillingOrder>;read(id:string):Promise<Document>;create(input:Input):Promise<Result>;receive(id:string,input:{operation_id:string;installment_id:string;session_id:string;amount_cents:number;expected_received_cents:number}):Promise<Result>;reverse(id:string,input:{operation_id:string;original_operation_id:string;original_device_id:string;session_id:string;amount_cents:number;expected_reversed_cents:number;reason:string}):Promise<Result>;cancel(id:string,input:{operation_id:string;reason:string}):Promise<Result>;reconcile():Promise<Result|null>;recover(retry?:boolean):Promise<Result|null>}
export class ReceivableError extends Error{status:number;constructor(code:string,status?:number)}
export function parseCents(value:string):number;
export function validateDocument(value:unknown):Document;
export function createReceivablesClient(options:{fetcher?:typeof fetch;getCurrent:()=>{token:string|null;session:LocalSession|null};pending?:Map<string,unknown>;journal?:{read(scope:string):Promise<any>;reserve(scope:string,command:any):Promise<boolean>;remove(scope:string,command:any):Promise<any>}|null}):{bind(context:{token:string;session:LocalSession}):Client};

export function validateCreateInput(value:unknown):Input;

export function validateCustomerOrder(value:unknown):CustomerBillingOrder;
