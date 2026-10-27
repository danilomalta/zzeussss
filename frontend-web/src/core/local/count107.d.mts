export interface CountSummary107 {total:number;notes:number;coins:number;pieces:number;signature:string;rows:{index:number;value:number;quantity:number;subtotal:number;kind:string}[];}
export function normalizeCount107(value:string):string;
export function countSummary107(counts:string[]):CountSummary107;
export function updateCount107(counts:string[],index:number,value:string):string[];
export function stepCount107(counts:string[],index:number,delta:number):string[];
export function confirmCount107(counts:string[],signature:string,reviewed:boolean):number;
