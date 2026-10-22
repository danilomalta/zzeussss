import type {Suspended94} from './till94.mjs';import type {LocalProduct} from './localClient.mjs';
export interface Preview104{lines:{product:string;name:string;unit:string;qty:number;oldPrice:number;currentPrice:number|null;old:number;current:number|null;loaded:boolean;changed:boolean}[];old:number;current:number|null;complete:boolean;changed:boolean;fingerprint:string}
export function preview104(row:Suspended94,products:LocalProduct[]):Preview104;
export function suspendedList104(rows:Suspended94[],query?:string,state?:string):Suspended94[];
