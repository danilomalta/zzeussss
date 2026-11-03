import type {Bag,BagAction,Coverage} from './source.mjs';
export const states:string[];
export function bagRecord(v:unknown,actor:string):Bag;
export function allowedBagActions(b:Bag,actor:string):BagAction[];
export function text(v:unknown,n?:number,empty?:boolean):boolean;
export function bagCoverage(v:unknown,cash:string):Coverage;
