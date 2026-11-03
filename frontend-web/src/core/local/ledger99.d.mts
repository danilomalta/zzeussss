import type {ClosingSale92} from './closing92.mjs';import type {Till94,Bag94,Actor94} from './till94.mjs';
export interface LedgerEntry99{key:string;at:string;origin:'sale'|'request'|'audit';kind:string;method:string;amount:number;state:string;reference:string;label:number;part:number;reason:string;actor:string}
export function ledger99(sales:ClosingSale92[],data:Till94,complete:boolean):{entries:LedgerEntry99[];cash:number|null;total:number|null;complete:boolean;sales:number;requests:number};
export function filterLedger99(entries:LedgerEntry99[],filters?:{origin?:string;method?:string;state?:string;query?:string;from?:number|null;to?:number|null;order?:string}):LedgerEntry99[];
export function range99(from:string,to:string):{from:number|null;to:number|null};
export function bag99(b:Bag94,cash:string):Bag94;
export function bagAction99(b:Bag94,actor:Actor94):{next:string;reason:string};
