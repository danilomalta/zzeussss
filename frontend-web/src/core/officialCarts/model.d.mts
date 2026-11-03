import type {Cart,CartAction,InputItem} from './source.mjs';
export function draftItems(items:{product:{id:string};location_id:string;quantity_milli:number}[]):InputItem[];
export function cartItems(cart:Cart):InputItem[];
export function actions(cart:Cart):CartAction[];
export const states:string[];
export function equal(a:unknown,b:unknown):boolean;
export function matchesDraftOrigin(c:import('./source.mjs').CartCommand,reference:string,items:InputItem[]):boolean;
