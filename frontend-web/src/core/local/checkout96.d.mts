import type {SaleReceipt} from './localClient.mjs';
export type Receipt96=Omit<SaleReceipt,'fiscal_authorized'>&{fiscal_authorized:boolean};
export interface Part96{id:string;method:string;amount:number;state:'draft'|'pending'|'unknown'|'approved'|'declined'|'cancelled';reference:string;manual?:boolean}
export interface Quote96{id:string;gross:number;discount:number;pointsDiscount?:number;pointsSpent?:number;member?:{name:string;points:number};net:number;expires:string}
export interface Plan96{version:1;id:string;cash:string;source:string;mode:'live'|'demo';total:number;state:'editing'|'quoted'|'finalizing'|'unknown'|'completed';parts:Part96[];received:number|null;quote?:Quote96;fiscal?:string;items?:{product_id:string;location_id:string;quantity_milli:number}[]}
export interface Charge96{ id:string;method:string;amount:number;state:Part96['state'];reference:string;qr?:string;copy?:string;expires?:string }
export interface Final96{checkout:string;state:'pending'|'committed'|'rejected';receipt?:Receipt96}
export interface QuoteInput96{checkout:string;cash:string;items:{product_id:string;location_id:string;quantity_milli:number}[];gross:number;discount:number;reason:string;supervisor?:{username:string;password:string};customer:{cpf:string;club:string};points:number;fiscal:string}
export interface CheckoutAdapter96{recordManual?:(token:string,input:{checkout:string;part:Part96;proof:{reference:string;provider:string;terminal:string;at:string}},signal:AbortSignal)=>Promise<Charge96>;receipt?:(token:string,id:string,signal:AbortSignal)=>Promise<Receipt96>;sales?:(token:string,cash:string,signal:AbortSignal)=>Promise<{sales:import('./closing92.mjs').ClosingSale92[];complete:boolean}>;quote:(token:string,input:QuoteInput96,signal:AbortSignal)=>Promise<Quote96>;charge:(token:string,input:{checkout:string;quote:string;part:Part96;customer:string;installments:number;due:string},signal:AbortSignal)=>Promise<Charge96>;chargeStatus:(token:string,checkout:string,part:Part96,signal:AbortSignal)=>Promise<Charge96>;cancelCharge:(token:string,checkout:string,part:Part96,signal:AbortSignal)=>Promise<Charge96>;finalize:(token:string,input:{plan:Plan96;items:QuoteInput96['items']},signal:AbortSignal)=>Promise<Final96>;status:(token:string,checkout:string,signal:AbortSignal)=>Promise<Final96>}
export const paymentMethods96:string[];
export function balance96(parts:Part96[],total:number,received:number|null):{allocated:number;remaining:number;cash:number;change:number|null;covered:boolean};
export function validatePlan96(p:unknown):Plan96;
export function validateQuote96(q:unknown,gross:number,discount:number,points?:number):Quote96;
export function validateCharge96(r:unknown,part:Part96):Charge96;
export function validateFinal96(r:unknown,plan:Plan96,items:QuoteInput96['items'],fiscal:string):Final96;
