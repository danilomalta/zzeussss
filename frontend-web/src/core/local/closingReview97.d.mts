import type {ClosingDraft92,ClosingSale92,ClosingPayment92,Paper92} from './closing92.mjs';
export const methods97:string[];
export interface ReviewPart97 extends ClosingPayment92{key:string;proof:number;paper:string;confirmed:boolean}
export interface ReviewRow97{id:string;index:number;at:string;total:number;cash:number;parts:ReviewPart97[];manual:Paper92[];documented:number;provisional:number;confirmed:number;verified:number;covered:number;pending:number;excess:number}
export interface Review97{rows:ReviewRow97[];methods:{method:string;parts:number;expected:number|null;confirmed:number;documented:number;verified:number;pending:number|null}[];issues:{id:string;kind:string}[];total:number|null;pending:number|null;unverified:number|null;unassigned:number;complete:boolean}
export function paymentKey97(sale:string,payment:string):string;
export function paperFingerprint97(p:Paper92):string;
export function validateSales97(sales:unknown):ClosingSale92[];
export function reviewClosing97(sales:ClosingSale92[],papers:Paper92[],complete:boolean,mode?:'papers'|'integrated'):Review97;
export function gate97(d:ClosingDraft92,sales:ClosingSale92[],complete:boolean,manager:boolean):string|null;
export function freshPaperForm97(previous:{provider:string;terminal:string},parsed:{method:string;amount:string;reference:string;at:string}):{method:string;amount:string;reference:string;at:string;provider:string;terminal:string};
export function csv97(rows:unknown[][]):string;
