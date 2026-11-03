import type {Till94,Actor94,Bag94} from './till94.mjs';import type {LedgerEntry99} from './ledger99.mjs';
export type BagRow109=Omit<Bag94,'difference'>&{difference:number|null;last:string;next:{next:string;reason:string}};
export function bags109(data:Till94,actor:Actor94,filters?:{query?:string;state?:string;variance?:boolean}):{rows:BagRow109[];total:number;open:number;verified:number;variances:number;amount:number;difference:number};
export function ledgerSummary109(entries:LedgerEntry99[],complete:boolean):{sales:number;requests:number;audit:number;total:number|null;methods:{method:string;parts:number;amount:number|null}[];requested:number;approved:number};
export function entryStamp109(entry:LedgerEntry99):string;
