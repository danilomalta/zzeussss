import type {Suspended94} from './till94.mjs';import type {LocalProduct} from './localClient.mjs';import type {Preview104} from './suspended104.mjs';
export interface CartView111{row:Suspended94;preview:Preview104;missing:number;changed:number;difference:number|null;signature:string}
export function inspectCarts111(rows:Suspended94[],products:LocalProduct[]):CartView111[];
export function filterCarts111(views:CartView111[],filters?:{query?:string;state?:string;issues?:boolean}):CartView111[];
export function confirmCart111(row:Suspended94,products:LocalProduct[],signature:string,ack:boolean,blocked:boolean):string;
