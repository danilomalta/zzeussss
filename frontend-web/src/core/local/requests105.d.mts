import type {Request94} from './till94.mjs';
export interface Input105{kind:'withdrawal'|'deposit'|'discount';amount:number;reason:string;reference:string;gross?:number}
export const kinds105:string[];
export function createInput105(kind:string,amount:string,reason:string,reference:string,gross?:number):Input105;
export function snapshot105(r:Request94):string;
export function filter105(rows:Request94[],filter?:{query?:string;state?:string;kind?:string}):Request94[];
export function sameRequest105(rows:Request94[],id:string,stamp:string):Request94;
export function events105(events:import('./till94.mjs').Till94['journal'],id:string):import('./till94.mjs').Till94['journal'];
