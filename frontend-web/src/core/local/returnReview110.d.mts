import type {Receipt96} from './checkout96.mjs';import type {Request94} from './till94.mjs';
export interface ReturnInput110{receipt:Receipt96;requests:Request94[];mode:string;item:string;quantity:string;unit:string;outcome:string;reason:string;scope:string;cash:string;actor:string}
export interface ReturnReview110{amount:number;quantity:number|null;mode:string;outcome:string;reason:string;signature:string}
export function returnReview110(input:ReturnInput110):ReturnReview110;
export function confirmReturn110(input:ReturnInput110,signature:string,ack:boolean):ReturnReview110;
export function returnQueue110(requests:Request94[]):{requested:number;approved:number;denied:number;withdrawn:number;pendingAmount:number;refund:number;exchange:number;credit:number;cancel:number};
