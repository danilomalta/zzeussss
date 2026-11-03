import type {Document113,Summary113,Part113,Payment113,Input113,Result113} from '../payables113/model.mjs';
export interface BalanceItem116{installment:Part113;paid_cents:number;returned_cents:number;applied_cents:number;outstanding_cents:number}
export interface Balances116{payable_id:string;state:Document113['state'];currency:'BRL';generated_at:string;complete:true;summary:Summary113;items:BalanceItem116[]}
export interface Candidate116{operation_id:string;session_id:string;movement_id:string;actor_id:string;kind:'withdrawal'|'supply';amount_cents:number;occurred_at:string}
export interface Candidates116{purpose:'pay'|'return';target_id:string;installment_id:string;currency:'BRL';generated_at:string;complete:true;reservation_created:false;write_authorization_checked:false;amount_rule:string;amount_limit_cents:number;blocked_reason:string;items:Candidate116[];total_count:number;limit:number;offset:number;has_more:boolean}
export interface Command116{version:1;document:string;kind:'pay'|'return';installment:string;amount_cents:number;paymentID:string|null;body:Input113;uncertain:boolean}
export function stamp116(v:unknown):boolean;
export function balances116(v:unknown,document:Document113):Balances116;
export const blocked116:string[];
export function candidates116(v:unknown,expected:{kind:'pay'|'return';target:string;part:string;actor:string;offset?:number;payment?:Payment113}):Candidates116;
export function command116(v:unknown):Command116;
export function receipt116(v:unknown,p:Command116):Result113;
export function queueKey116(scope:string):string;
export function read116(storage:Storage,key:string):Command116|null;
export function save116(storage:Storage,key:string,v:Command116):void;
export function selection116(page:Candidates116|null,id:string,now?:number,loadedAt?:number):Candidate116;
