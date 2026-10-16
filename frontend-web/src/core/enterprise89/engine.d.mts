import type {Data89,Finance89,Settlement89,Campaign89,Order89,Area89} from './types';
import type {LocalCapabilities} from '../local/localClient.mjs';
export const areas89:Area89[];
export function can89(cap:LocalCapabilities|null,area:Area89):boolean;
export function empty89(scope:string):Data89;
export function validate89(data:unknown,scope:string):Data89;
export function remaining89(row:Finance89):number;
export function settle89(row:Finance89,s:Settlement89):void;
export function promotion89(row:Campaign89):{price:number;margin:number};
export function finance89(rows:Finance89[],from?:string,to?:string):{pay:number;receive:number;incoming:number;outgoing:number;net:number;result:number};
export function match89(data:Data89,bankID:string,settlementID:string):void;
export function transition89(row:Order89,next:Order89['state']):void;
export function parseOFX89(raw:string):{reference:string;date:string;kind:'pay'|'receive';amount:number;name:string}[];
export function money(v:string):number;
export function quantity(v:string,scale?:number):number;
export function validDate(v:string):boolean;
export function csv(rows:(string|number)[][]):string;

export function cashback89(program:{rate:number;minimum:number},amount:number):number;
